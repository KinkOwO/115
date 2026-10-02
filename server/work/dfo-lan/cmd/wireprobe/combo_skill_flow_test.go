package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestEntryReplaysComboSkillInfoAsNotify433(t *testing.T) {
	// C2S body for six empty cells; the S2C notify body must be 00 01 00 +
	// (body without its leading 0).
	c2s := []byte{0x00, 0x06, 0x76, 0x00, 0x00, 0x77, 0x00, 0x00, 0x78, 0x00, 0x00, 0x79, 0x00, 0x00, 0x7a, 0x00, 0x00, 0x7b, 0x00, 0x00}
	s2c := []byte{0x00, 0x01, 0x00, 0x06, 0x76, 0x00, 0x00, 0x77, 0x00, 0x00, 0x78, 0x00, 0x00, 0x79, 0x00, 0x00, 0x7a, 0x00, 0x00, 0x7b, 0x00, 0x00}
	packets := (entryPayloads{
		Skills:         []byte{1},
		SkillPreset:    make([]byte, 28),
		ComboSkillInfo: s2c,
	}).packets()
	var saw bool
	skills, combo := -1, -1
	for i, p := range packets {
		if p.Kind == 0 && p.ID == 19 {
			skills = i
		}
		if p.Name == "combo_skill_info_restored" {
			combo = i
			saw = true
			if p.Kind != 0 || p.ID != 433 {
				t.Fatalf("combo replay kind=%d id=%d, want kind=0 id=433", p.Kind, p.ID)
			}
			if !bytes.Equal(p.Payload, s2c) {
				t.Fatalf("combo replay payload = %x, want S2C %x", p.Payload, s2c)
			}
			// PAGE must stay 0; a C2S echo would put 0x76 there.
			if p.Payload[2] != 0x00 {
				t.Fatalf("combo replay PAGE = %#x, want 0x00", p.Payload[2])
			}
		}
		// 500 must never appear on the notify side (it is MODULE_REQUEST).
		if p.Kind == 0 && p.ID == 500 {
			t.Fatalf("entry packet %q uses notify id 500 (MODULE_REQUEST)", p.Name)
		}
	}
	if !saw {
		t.Fatal("combo replay was not emitted")
	}
	if skills < 0 || combo <= skills || bytes.Equal(s2c, c2s) {
		t.Fatal("combo must follow skill tree using independent S2C layout")
	}
}

func TestComboRequestsRemainVerifiedAfterSampleLimit(t *testing.T) {
	for _, id := range []uint16{500, 502} {
		samples := map[uint16]int{}
		for i := 0; i < BodySampleLimit+3; i++ {
			if !retainRequestBody(id, samples) {
				t.Fatalf("CMD%d stopped decoding at request %d", id, i+1)
			}
		}
	}
}

func TestComboRepeatedEditsGetDistinctEventKeys(t *testing.T) {
	var a, b comboSkillSession
	seen := map[string]bool{}
	for _, session := range []*comboSkillSession{&a, &a, &a, &b} {
		key, err := session.nextKey()
		if err != nil {
			t.Fatal(err)
		}
		if seen[key] {
			t.Fatal("repeated/reconnected edit reuses an event key")
		}
		seen[key] = true
	}
}

func TestComboResetAcceptsLiveZeroPadding(t *testing.T) {
	var s comboSkillSession
	_, err := s.save(&character.Service{}, &worldSession{role: storage.Character{ID: 1, Profession: 9}}, 502, make([]byte, 16))
	if err == nil || s.sequence != 1 {
		t.Fatal("canonical encrypted empty body was rejected before persistence")
	}
	var invalid comboSkillSession
	_, err = invalid.save(&character.Service{}, &worldSession{role: storage.Character{ID: 1, Profession: 9}}, 502, []byte{1})
	if err == nil || invalid.sequence != 0 {
		t.Fatal("nonempty reset reached persistence")
	}
	_, err = invalid.save(&character.Service{}, &worldSession{role: storage.Character{ID: 1, Profession: 9}}, 502, make([]byte, 32))
	if err == nil || invalid.sequence != 0 {
		t.Fatal("oversized reset padding reached persistence")
	}
}

func TestComboRestoreAfterSkillRefreshUsesStoredLayout(t *testing.T) {
	info := protocol.ComboSkillInfo{Cells: []protocol.ComboCell{{Skill: 118, Chain: []uint16{46}}}}
	raw, err := protocol.EncodeComboSkillInfo(info)
	if err != nil {
		t.Fatal(err)
	}
	state, err := json.Marshal(character.State{ComboSkillInfo: raw})
	if err != nil {
		t.Fatal(err)
	}
	cs := &character.Service{}
	role := storage.Character{Profession: 9, State: state}
	plan, err := appendComboSkillRestore([]outboundPacket{{"skills", 0, 19, []byte{1}}}, cs, role)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 || plan[1].ID != 433 || !bytes.Equal(plan[1].Payload[:3], []byte{0, 1, 0}) {
		t.Fatalf("missing correct combo restore: %+v", plan)
	}
	role.Profession = 0
	plan, err = appendComboSkillRestore(plan[:1], cs, role)
	if err != nil || len(plan) != 1 {
		t.Fatal("other profession received dark knight combo restore")
	}
}

func TestDefaultCatalogDarkKnightComboShortcuts(t *testing.T) {
	c, err := catalog.LoadCharacters("../../configs/characters.skycastle-release.json")
	if err != nil {
		t.Fatal(err)
	}
	prof := c.Professions[9]
	for i := uint16(0); i < 6; i++ {
		if slot, ok := prof.InitialSkillSlots[118+i]; !ok || slot != i {
			t.Fatalf("combo %d slot=%d present=%t", 118+i, slot, ok)
		}
	}
}

// Real PostgreSQL check in a disposable schema. No production characters are
// created or modified; exercise editing A -> B -> A, reset, and old-save fields.
func TestComboSkillPersistenceIntegration(t *testing.T) {
	if os.Getenv("COMBO_INTEGRATION") != "1" {
		t.Skip("COMBO_INTEGRATION=1 requires local storage")
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
	schema := fmt.Sprintf("combo_check_%d", time.Now().UnixNano())
	if _, err = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.DB.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	cfg.PostgresSchema = schema
	db, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = db.MigrateCharacterEvents(ctx); err != nil {
		t.Fatal(err)
	}
	role := storage.Character{Profession: 9, ConfigVersion: strings.Repeat("a", 64)}
	if err = db.DB.QueryRow(ctx, "INSERT INTO accounts(username) VALUES('combo-test') RETURNING id").Scan(&role.AccountID); err != nil {
		t.Fatal(err)
	}
	role.State = json.RawMessage(`{"level":1,"unknown_future_field":{"keep":true},"skill_slots":[{"118":0},null]}`)
	if err = db.DB.QueryRow(ctx, `INSERT INTO characters(account_id,wire_id,name,profession,create_request,config_version,state) VALUES($1,1,'combo-test',9,''::bytea,$2,$3) RETURNING id`, role.AccountID, role.ConfigVersion, role.State).Scan(&role.ID); err != nil {
		t.Fatal(err)
	}
	cs := &character.Service{Store: db}
	w := &worldSession{role: role}
	var session comboSkillSession
	for _, skill := range []uint16{46, 108, 46} {
		info := protocol.ComboSkillInfo{Cells: []protocol.ComboCell{{Skill: 118, Chain: []uint16{skill}}}}
		body, err := protocol.EncodeComboSkillInfo(info)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = session.save(cs, w, 500, body); err != nil {
			t.Fatal(err)
		}
		var persisted json.RawMessage
		if err = db.DB.QueryRow(ctx, "SELECT state FROM characters WHERE id=$1", role.ID).Scan(&persisted); err != nil {
			t.Fatal(err)
		}
		check := w.role
		check.State = persisted
		got, err := cs.EntryComboSkillInfo(check)
		if err != nil || !bytes.Equal(got, body) {
			t.Fatalf("edit %d did not persist: %x %v", skill, got, err)
		}
	}
	session.lastNotify = []byte{1}
	if _, err = session.save(cs, w, 502, make([]byte, 16)); err != nil {
		t.Fatal(err)
	}
	if len(session.lastNotify) != 0 {
		t.Fatal("reset retained notification dedupe")
	}
	var state map[string]json.RawMessage
	if err = json.Unmarshal(w.role.State, &state); err != nil {
		t.Fatal(err)
	}
	if string(state["combo_skill_info"]) != "null" || string(state["unknown_future_field"]) != `{"keep":true}` {
		t.Fatalf("reset corrupted old-save state: %s", w.role.State)
	}
	var count int
	if err = db.DB.QueryRow(ctx, "SELECT count(*) FROM character_events WHERE character_id=$1", role.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatalf("distinct edit/reset receipts=%d", count)
	}
	foreign := w.role
	foreign.AccountID++
	if _, _, err = cs.ClearComboSkillInfo(ctx, foreign, "foreign-reset"); err == nil {
		t.Fatal("foreign account cleared character")
	}
}
