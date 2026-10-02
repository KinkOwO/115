package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/progression"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

// This exercises the real experience transaction and the death response plan,
// not just ConfirmDeath with progression disabled. All writes use a disposable
// schema; no existing character is read or changed.
func TestMonsterDeathAfterAdvancementIntegration(t *testing.T) {
	if os.Getenv("MONSTER_DEATH_INTEGRATION") != "1" {
		t.Skip("set MONSTER_DEATH_INTEGRATION=1 for isolated PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cfg, err := storage.LoadConfig("../../runtime/storage/local.json")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("monster_death_test_%d", time.Now().UnixNano())
	if _, err = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := admin.DB.Exec(cleanup, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	cfg.PostgresSchema = schema
	store, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents, store.MigratePremiums} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "monster-death-fixture")
	if err != nil {
		t.Fatal(err)
	}
	professions, err := catalog.LoadCharacters("../../configs/characters.skycastle-release.json")
	if err != nil {
		t.Fatal(err)
	}
	pc, err := catalog.LoadProgression("../../configs/progression.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	rules, err := progression.LoadRules("../../configs/experience.compat90.json")
	if err != nil {
		t.Fatal(err)
	}
	ps := &character.ProgressionService{Store: store, Catalog: pc, Professions: professions, Rules: rules}
	p := professions.Professions[12]
	state := character.State{Level: 1, Attributes: p.InitialAttributes, InitialSkills: p.InitialSkills, SourcePath: p.Path, SourceSHA256: p.RawSHA256}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	role := storage.Character{AccountID: account, Name: "DeathAfterAdvance", WireID: 10, Profession: 12, State: raw, Request: []byte{0}, ConfigVersion: professions.Source.SaveIdentity()}
	role, _, err = ps.ApplyGain(role, pc.Thresholds[13])
	if err != nil {
		t.Fatal(err)
	}
	role.State, err = (&character.Service{Catalog: professions}).ApplyAdvancement(role, 1)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(role.State, &fields); err != nil {
		t.Fatal(err)
	}
	fields["inventory"] = json.RawMessage(`{"version":"ordinary-bag-v1","gold":321,"coin":7}`)
	fields["future_death_field"] = json.RawMessage(`{"keep":true}`)
	role.State, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	role, err = store.CreateCharacter(ctx, role, 24)
	if err != nil {
		t.Fatal(err)
	}
	// The captured killer is a scene ID, not this test's durable roster ID.
	role.WireID = 10
	state = character.State{}
	if err = json.Unmarshal(role.State, &state); err != nil {
		t.Fatal(err)
	}
	if state.Level != 15 || state.Advancement != 1 || state.AllJobsPilot || state.SwordmasterPilot {
		t.Fatalf("not an ordinary advanced fixture: %+v", state)
	}
	initialExperience := state.Experience
	// Match channel_probe.py's runtime catalog. The older generated subset
	// has only 11 dungeons and does not include this captured dungeon ID 11.
	dc, err := catalog.LoadDungeons("../../configs/dungeons.full.json")
	if err != nil {
		t.Fatal(err)
	}
	run, err := dungeon.Select(dc, protocol.DungeonSelection{ID: 11, Difficulty: 1, Party: 65535}, state.Level, nil)
	if err != nil {
		t.Fatal(err)
	}
	if run.Room.Map != 58597 || len(run.Monsters) != 4 {
		t.Fatalf("wrong captured room: map=%d monsters=%d", run.Room.Map, len(run.Monsters))
	}
	run.Loaded = true
	w := &worldSession{account: account, role: role, level: state.Level, activeDungeon: run, progression: ps, deathSent: map[uint16]bool{}}
	data, err := os.ReadFile("testdata/monster_death_after_advancement_20260922.json")
	if err != nil {
		t.Fatal(err)
	}
	var capture struct {
		Frames []struct {
			Line     int    `json:"line"`
			Entity   uint32 `json:"entity"`
			Killer   uint16 `json:"killer"`
			PlainHex string `json:"plain_hex"`
		} `json:"frames"`
	}
	if err = json.Unmarshal(data, &capture); err != nil || len(capture.Frames) != 4 {
		t.Fatal("invalid captured deaths", err)
	}
	keys := make([]byte, wire.SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	for _, frame := range capture.Frames {
		body, err := hex.DecodeString(frame.PlainHex)
		if err != nil {
			t.Fatal(err)
		}
		report, err := protocol.DecodeMonsterDeath(body)
		if err != nil || report.Entity != frame.Entity || report.Killer != frame.Killer {
			t.Fatalf("capture line %d identity: %+v, %v", frame.Line, report, err)
		}
		plan, err := w.monsterDeath(body, func(map[string]any) {})
		if err != nil {
			t.Fatalf("capture line %d withheld death confirmation: %v", frame.Line, err)
		}
		if len(plan) != 3 || plan[0].ID != 39 || plan[0].Kind != 1 || !bytes.Equal(plan[0].Payload, []byte{1}) || plan[1].ID != 38 || plan[1].Kind != 0 || plan[2].ID != 37 || plan[2].Kind != 0 {
			t.Fatalf("missing death ACK/confirmation/experience: %+v", plan)
		}
		if !bytes.Equal(plan[1].Payload, protocol.MonsterDeathConfirmed(uint16(frame.Entity))) {
			t.Fatal("death confirmation names a different entity")
		}
		encoded, err := preparePackets(keys, plan)
		if err != nil {
			t.Fatal(err)
		}
		for _, packet := range encoded {
			if err = wire.ValidateServer(packet.Raw); err != nil {
				t.Fatal(err)
			}
		}
		var after character.State
		if err = json.Unmarshal(w.role.State, &after); err != nil {
			t.Fatal(err)
		}
		if binary.LittleEndian.Uint64(plan[2].Payload[1:]) != after.Experience || w.role.WireID != 10 {
			t.Fatal("experience response differs from committed scene character")
		}
		// Lost-send retry still returns the same confirmation but must not
		// apply experience a second time. Successful-send retries omit NOTI38.
		for retry := 0; retry < 2; retry++ {
			replay, err := w.monsterDeath(body, func(map[string]any) {})
			if err != nil || len(replay) != 3-retry {
				t.Fatal("death replay", len(replay), err)
			}
			var got character.State
			if err = json.Unmarshal(w.role.State, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, after) {
				t.Fatal("death retry duplicated growth or changed saved state")
			}
			w.deathSent[uint16(frame.Entity)] = true // main.go marks this only after sending NOTI38.
		}
	}
	if !run.RoomCleared() {
		t.Fatal("captured room still blocked after every death")
	}
	moved := false
	for _, room := range run.Maze.Rooms {
		dx, dy := int(room.X)-int(run.Room.X), int(room.Y)-int(run.Room.Y)
		if dx*dx+dy*dy == 1 {
			next, err := run.Move(dc, [2]byte{room.X, room.Y})
			if err != nil || next.Room.Map == run.Room.Map {
				t.Fatal("next room remains blocked", err)
			}
			moved = true
			break
		}
	}
	if !moved {
		t.Fatal("fixture has no adjacent room")
	}
	var receipts int
	if err = store.DB.QueryRow(ctx, "SELECT count(*) FROM character_events WHERE character_id=$1 AND event_key LIKE 'monster:%'", role.ID).Scan(&receipts); err != nil || receipts != 4 {
		t.Fatal("death receipt count", receipts, err)
	}
	roles, err := store.Characters(ctx, account)
	if err != nil || len(roles) != 1 {
		t.Fatal("reload fixture", err)
	}
	if err = json.Unmarshal(roles[0].State, &state); err != nil {
		t.Fatal(err)
	}
	if state.Experience <= initialExperience || state.Advancement != 1 || state.AllJobsPilot || state.SwordmasterPilot {
		t.Fatal("advanced experience did not persist")
	}
	var saved, expected map[string]any
	if err = json.Unmarshal(roles[0].State, &saved); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(role.State, &expected); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"inventory", "future_death_field"} {
		if !reflect.DeepEqual(saved[key], expected[key]) {
			t.Fatalf("saved %s was lost during growth", key)
		}
	}
	t.Log("PASS: 4 native CMD39 -> ACK39/NOTI38/NOTI37; room exit unblocked; 12 reports -> 4 receipts; saved inventory preserved; disposable schema only")
}
