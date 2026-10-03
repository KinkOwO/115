// charactercheck exercises persistence and concurrent slot allocation in a
// new temporary schema. It never creates the user's requested game character.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"os"
	"sync"
	"time"
)

func request(name string) []byte {
	p := []byte{0}
	p = binary.LittleEndian.AppendUint32(p, uint32(len(name)))
	p = append(p, []byte(name)...)
	return append(p, 0, 0, 0, 0, 0, 0, 255, 0, 0, 0)
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, e := storage.LoadConfig("runtime/storage/local.json")
	if e != nil {
		return e
	}
	root, e := storage.Open(ctx, cfg)
	if e != nil {
		return e
	}
	defer root.Close()
	var token [8]byte
	if _, e = rand.Read(token[:]); e != nil {
		return e
	}
	schema := "charactercheck_" + hex.EncodeToString(token[:])
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, e = root.DB.Exec(ctx, "CREATE SCHEMA "+quoted); e != nil {
		return e
	}
	defer func() {
		cleanup, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		if _, e := root.DB.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); e != nil {
			fmt.Fprintln(os.Stderr, "temporary schema cleanup failed:", schema, e)
		}
	}()
	cfg.PostgresSchema = schema
	s, e := storage.Open(ctx, cfg)
	if e != nil {
		return e
	}
	defer s.Close()
	var actualSchema string
	if e = s.DB.QueryRow(ctx, "SELECT current_schema()").Scan(&actualSchema); e != nil {
		return e
	}
	if actualSchema != schema {
		return fmt.Errorf("test isolation failed: %s", actualSchema)
	}
	if e = s.Migrate(ctx); e != nil {
		return e
	}
	c, e := catalog.LoadCharacters("configs/characters.generated.json")
	if e != nil {
		return e
	}
	service, e := character.New(s, c, character.Rules{MaxCharacters: 1, InitialLevel: 1})
	if e != nil {
		return e
	}
	account, e := s.DevelopmentAccount(ctx, "temporary-test")
	if e != nil {
		return e
	}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, n := range []string{"ConcurrencyA", "ConcurrencyB"} {
		wg.Add(1)
		go func(name string) { defer wg.Done(); _, e := service.Create(ctx, account, request(name)); results <- e }(n)
	}
	wg.Wait()
	close(results)
	success := 0
	var failures []string
	for e := range results {
		if e == nil {
			success++
		} else {
			failures = append(failures, e.Error())
		}
	}
	if success != 1 {
		return fmt.Errorf("concurrent slot limit: successes=%d failures=%v", success, failures)
	}
	rows, e := s.Characters(ctx, account)
	if e != nil {
		return e
	}
	if len(rows) != 1 || rows[0].WireID != 1 {
		return fmt.Errorf("unexpected saved role count or id")
	}
	var state character.State
	if e = json.Unmarshal(rows[0].State, &state); e != nil {
		return e
	}
	if state.Attributes["[hp max]"] != c.Professions[0].InitialAttributes["[hp max]"] || rows[0].ConfigVersion != c.Source.SaveIdentity() {
		return fmt.Errorf("saved source configuration mismatch")
	}
	second, e := s.DevelopmentAccount(ctx, "temporary-other")
	if e != nil {
		return e
	}
	if _, e = service.Create(ctx, second, request(rows[0].Name)); e == nil {
		return fmt.Errorf("duplicate global name was accepted")
	}
	modern, _ := hex.DecodeString("100400000036363636000000000000ff0001000000000000")
	modernRole, e := service.Create(ctx, second, modern)
	if e != nil {
		return fmt.Errorf("current naming window replay: %w", e)
	}
	if modernRole.Profession != 16 || modernRole.Name != "6666" {
		return fmt.Errorf("modern role fields were lost")
	}
	slot, e := service.RosterSlot(ctx, second, modernRole.ID)
	if e != nil || slot != 0 || modernRole.WireID != 1 {
		return fmt.Errorf("storage key leaked into roster position: slot=%d id=%d err=%v", slot, modernRole.WireID, e)
	}
	if _, e = service.RosterSlot(ctx, account, modernRole.ID); e == nil {
		return fmt.Errorf("another account's character resolved in roster")
	}
	selectRequest := make([]byte, 16)
	selected, e := service.Select(ctx, second, selectRequest)
	if e != nil || selected.ID != modernRole.ID {
		return fmt.Errorf("selected wrong account character: %v", e)
	}
	selected, e = service.Select(ctx, account, selectRequest)
	if e != nil || selected.ID != rows[0].ID {
		return fmt.Errorf("same slot crossed account boundary: %v", e)
	}
	selectRequest[0] = 1
	if _, e = service.Select(ctx, second, selectRequest); e == nil {
		return fmt.Errorf("nonexistent roster slot accepted")
	}
	beforeRetry, e := s.Characters(ctx, second)
	if e != nil || len(beforeRetry) != 1 {
		return fmt.Errorf("missing saved modern role before retry: %v", e)
	}
	retry, e := service.Create(ctx, second, modern)
	if e != nil || retry.ID != modernRole.ID || !bytes.Equal(retry.State, beforeRetry[0].State) {
		return fmt.Errorf("creation retry changed persisted role: %v", e)
	}
	// Reopen storage to verify that state is not only an in-memory response.
	reopened, e := storage.Open(ctx, cfg)
	if e != nil {
		return e
	}
	defer reopened.Close()
	persisted, e := reopened.Characters(ctx, account)
	if e != nil {
		return e
	}
	if len(persisted) != 1 || persisted[0].ID != rows[0].ID {
		return fmt.Errorf("reopened persistence mismatch")
	}
	payload, e := service.List(ctx, account)
	if e != nil {
		return e
	}
	if e = s.MigrateWorld(ctx); e != nil {
		return e
	}
	if e = s.MigrateQuests(ctx); e != nil {
		return e
	}
	spawn := storage.WorldPosition{Town: 38, Area: 0, X: 561, Y: 234}
	world, e := s.LoadWorld(ctx, account, rows[0].ID, 0, spawn, c.Source.SaveIdentity())
	if e != nil {
		return e
	}
	if _, e = s.LoadWorld(ctx, second, rows[0].ID, 0, spawn, c.Source.SaveIdentity()); e == nil {
		return fmt.Errorf("world crossed account boundary")
	}
	next := spawn
	next.X = 600
	changed, e := s.SaveWorld(ctx, account, rows[0].ID, 0, world, next)
	if e != nil {
		return e
	}
	if _, e = s.SaveWorld(ctx, account, rows[0].ID, 0, world, spawn); !errors.Is(e, storage.ErrWorldConflict) {
		return fmt.Errorf("stale world overwrite accepted: %v", e)
	}
	loaded, e := reopened.LoadWorld(ctx, account, rows[0].ID, 0, spawn, c.Source.SaveIdentity())
	if e != nil || loaded.Position.X != 600 || loaded.Revision != changed.Revision {
		return fmt.Errorf("saved world did not survive reopen: %v", e)
	}
	q, e := s.AcceptQuest(ctx, account, rows[0].ID, 3145, c.Source.SaveIdentity(), 1, 10000, nil, 1, "single-clear-map-remaining-v1")
	if e != nil {
		return e
	}
	repeated, e := s.AcceptQuest(ctx, account, rows[0].ID, 3145, c.Source.SaveIdentity(), 1, 10000, nil, 1, "single-clear-map-remaining-v1")
	if e != nil || q != repeated {
		return fmt.Errorf("quest retry changed saved progress: %v", e)
	}
	if _, e = s.AcceptQuest(ctx, account, rows[0].ID, 3146, c.Source.SaveIdentity(), 1, 10000, []uint32{3145}, 1, "single-clear-map-remaining-v1"); e == nil {
		return fmt.Errorf("unfinished prerequisite accepted")
	}
	act2Groups := [][]uint32{{3232}, {3237}}
	if _, e = s.AcceptQuestGroups(ctx, account, rows[0].ID, 3240, c.Source.SaveIdentity(), 1, 10000, act2Groups, 1, "single-clear-map-remaining-v1"); e == nil {
		return fmt.Errorf("alternative prerequisite accepted before either branch completed")
	}
	if _, e = s.DB.Exec(ctx, `INSERT INTO character_quests(character_id,quest_id,status,progress,config_version,progress_model) VALUES($1,3232,'completed',0,$2,$3)`, rows[0].ID, c.Source.SaveIdentity(), "single-clear-map-remaining-v1"); e != nil {
		return e
	}
	if _, e = s.AcceptQuestGroups(ctx, account, rows[0].ID, 3240, c.Source.SaveIdentity(), 1, 10000, act2Groups, 1, "single-clear-map-remaining-v1"); e != nil {
		return fmt.Errorf("completed Act 2 alternative did not unlock successor: %w", e)
	}
	if _, e = s.AcceptQuest(ctx, second, rows[0].ID, 3145, c.Source.SaveIdentity(), 1, 10000, nil, 1, "single-clear-map-remaining-v1"); e == nil {
		return fmt.Errorf("quest crossed account boundary")
	}
	if e = s.AbandonQuest(ctx, second, rows[0].ID, 3145); e == nil {
		return fmt.Errorf("other account abandoned quest")
	}
	if e = s.AbandonQuest(ctx, account, rows[0].ID, 3145); e != nil {
		return e
	}
	if e = moduleCheck(ctx, s, reopened, rows[0], second, c); e != nil {
		return e
	}
	if e = tutorialCheck(ctx, s, reopened, rows[0], second); e != nil {
		return e
	}
	if e = birthCheck(ctx, s, reopened, rows[0], second); e != nil {
		return e
	}
	if e = grantCheck(ctx, s, reopened, rows[0], second); e != nil {
		return e
	}
	if e = fatigueChargeCheck(ctx, s, reopened, rows[0], second); e != nil {
		return e
	}
	if e = questObjectiveCheck(ctx, s, reopened, rows[0], second); e != nil {
		return e
	}
	if e = progressionCheck(ctx, s, reopened, rows[0], second); e != nil {
		return e
	}
	if e = questRewardCheck(ctx, s, reopened, rows[0], second); e != nil {
		return e
	}
	if e = clearRewardCheck(ctx, s, reopened, rows[0], second); e != nil {
		return e
	}
	if e = lootCheck(ctx, s, reopened, rows[0], second); e != nil {
		return e
	}
	if e = cardCheck(ctx, s, reopened, rows[0], second); e != nil {
		return e
	}
	if e = deleteCheck(ctx, s, reopened, second, c); e != nil {
		return e
	}
	if e = learningCheck(ctx, s, reopened, second); e != nil {
		return e
	}
	if e = questChainCheck(ctx, s, second); e != nil {
		return e
	}
	if e = wearCheck(ctx, s, reopened, second, account); e != nil {
		return e
	}
	fmt.Println("WORLD_QUEST_STORAGE_PASS ownership=true stale_write_refused=true reopen=true quest_retry_idempotent=true prerequisite_enforced=true")
	b, e := json.MarshalIndent(map[string]any{"status": "POSTGRES_CHARACTER_CHECK_PASS", "concurrent_slot_limit": true, "global_name_unique": true, "reopen_persistence": true, "current_naming_packet_replay": true, "source_attributes_preserved": true, "vault_contents_preserved": true, "fatigue_reconnect_and_rollover": true, "quest_pending_restore_and_audited_repair": true, "world_quest_ownership_and_revision": true, "list_payload_bytes": len(payload), "temporary_schema_only": true, "real_client_character_created": false}, "", "  ")
	if e != nil {
		return e
	}
	if e = os.WriteFile("runtime/character_validation.json", b, 0600); e != nil {
		return e
	}
	fmt.Println(string(b))
	return nil
}
