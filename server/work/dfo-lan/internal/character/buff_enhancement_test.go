package character

import (
	"bytes"
	"context"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func buffFixture(t *testing.T) (*Service, storage.Character) {
	t.Helper()
	const sum = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	catalog := inventory.EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: sum}, Rows: []inventory.EquipmentDefinition{
		{ID: 100, Path: "title.equ", SHA256: sum, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[title name]"}}}},
		{ID: 101, Path: "other-title.equ", SHA256: sum, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[title name]"}}}},
	}}
	raw, _ := json.Marshal(catalog)
	path := filepath.Join(t.TempDir(), "equipment.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	equipment, err := inventory.LoadEquipmentCatalog(path, sum)
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{Equipment: equipment, WearRules: inventory.WearRules{Slots: map[string]uint16{"[title name]": 13}}}
	state := json.RawMessage(`{"level":10,"learned_skills":[{"306":1},null],"future":{"keep":true},"quest_state":{"keep":1}}`)
	state, err = inventory.SaveBag(state, inventory.Bag{Version: "ordinary-bag-v1", Gold: 999, Equipment: []inventory.BagEquipment{{Slot: 57, Template: 100}}})
	if err != nil {
		t.Fatal(err)
	}
	return s, storage.Character{AccountID: 1, ID: 1, Profession: 9, ConfigVersion: sum, State: state}
}

func TestBuffEnhancementPersistenceAndIdentity(t *testing.T) {
	s, role := buffFixture(t)
	original := append([]byte(nil), role.State...)
	raw, err := s.applyBuffEnhancement(role, protocol.BuffEnhancementRequest{Skill: 306, Kind: 13, List: 0, Slot: 57})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(role.State, original) {
		t.Fatal("apply mutated source state")
	}
	role.State = raw
	got, err := s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 1, 1, 13, 0, 57, 0}) {
		t.Fatalf("restore %x %v", got, err)
	}
	var fields map[string]json.RawMessage
	json.Unmarshal(raw, &fields)
	if string(fields["future"]) != `{"keep":true}` || string(fields["quest_state"]) != `{"keep":1}` {
		t.Fatal("unrelated state lost")
	}
	var state State
	json.Unmarshal(raw, &state)
	merged, err := mergeSkillState(raw, state)
	if err != nil {
		t.Fatal(err)
	}
	role.State = merged
	got, err = s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 1, 1, 13, 0, 57, 0}) {
		t.Fatal("skill merge lost registration")
	}
	b, err := inventory.ReadBag(role.State)
	if err != nil {
		t.Fatal(err)
	}
	b.Equipment[0].Slot = 80
	role.State, err = inventory.SaveBag(role.State, b)
	if err != nil {
		t.Fatal(err)
	}
	got, err = s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 1, 1, 13, 0, 80, 0}) {
		t.Fatalf("move did not follow owned identity: %x %v", got, err)
	}
	b.Equipment[0].Template = 101
	role.State, _ = inventory.SaveBag(role.State, b)
	got, err = s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 0}) {
		t.Fatal("reused slot incorrectly restored")
	}
	b.Equipment = []inventory.BagEquipment{{Slot: 80, Template: 100}, {Slot: 81, Template: 100}}
	role.State, _ = inventory.SaveBag(role.State, b)
	got, err = s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 0}) {
		t.Fatal("ambiguous duplicate restored")
	}
}

func TestBuffEnhancementSelectionAndClear(t *testing.T) {
	s, role := buffFixture(t)
	got, err := s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0, 0, 0}) {
		t.Fatal("legacy save not cleared")
	}
	for _, req := range []protocol.BuffEnhancementRequest{{Skill: 306, Kind: 13, List: 0, Slot: 57}, {Skill: 0, Kind: 48, List: 46, Slot: 65535}, {Skill: 306, Kind: 48, List: 46, Slot: 65535}} {
		role.State, err = s.applyBuffEnhancement(role, req)
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err = s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 1, 1, 13, 0, 57, 0}) {
		t.Fatal("skill selection discarded items")
	}
	role.State, err = s.applyBuffEnhancement(role, protocol.BuffEnhancementRequest{Skill: 306, Kind: 13, List: 46, Slot: 65535})
	if err != nil {
		t.Fatal(err)
	}
	got, err = s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 0}) {
		t.Fatal("clear registration failed")
	}
	for _, req := range []protocol.BuffEnhancementRequest{{Skill: 999, Kind: 13, List: 0, Slot: 57}, {Skill: 306, Kind: 12, List: 0, Slot: 57}, {Skill: 306, Kind: 13, List: 0, Slot: 58}} {
		if _, err = s.applyBuffEnhancement(role, req); err == nil {
			t.Fatalf("accepted invalid %+v", req)
		}
	}
}

// Optional real PostgreSQL exercise, confined to a new temporary schema.
func TestBuffEnhancementPostgres(t *testing.T) {
	config := os.Getenv("BUFF_INTEGRATION_CONFIG")
	if config == "" {
		t.Skip("isolated PostgreSQL integration not requested")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cfg, err := storage.LoadConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("buff_check_%d", time.Now().UnixNano())
	if _, err = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		clean, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		if _, e := admin.DB.Exec(clean, "DROP SCHEMA "+schema+" CASCADE"); e != nil {
			t.Error(e)
		}
	}()
	cfg.PostgresSchema, cfg.RedisPrefix = schema, schema+":"
	store, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = store.MigrateCharacterEvents(ctx); err != nil {
		t.Fatal(err)
	}
	s, fixture := buffFixture(t)
	s.Store = store
	account, err := store.DevelopmentAccount(ctx, "buff-fixture")
	if err != nil {
		t.Fatal(err)
	}
	fixture.AccountID, fixture.ID, fixture.Name, fixture.Request = account, 0, "BuffFixture", []byte{0}
	role, err := store.CreateCharacter(ctx, fixture, 24)
	if err != nil {
		t.Fatal(err)
	}
	before := role.State
	role, _, err = s.SaveBuffEnhancement(ctx, role, "edit-1", protocol.BuffEnhancementRequest{Skill: 306, Kind: 13, List: 0, Slot: 57})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.SaveBuffEnhancement(ctx, role, "bad", protocol.BuffEnhancementRequest{Skill: 306, Kind: 13, List: 0, Slot: 58}); err == nil {
		t.Fatal("missing item accepted")
	}
	other := role
	other.AccountID++
	if _, _, err = s.SaveBuffEnhancement(ctx, other, "wrong-owner", protocol.BuffEnhancementRequest{Skill: 306, Kind: 13, List: 46, Slot: 65535}); err == nil {
		t.Fatal("wrong owner accepted")
	}
	for i, skill := range []uint16{0, 306, 0, 306} {
		role, _, err = s.SaveBuffEnhancement(ctx, role, fmt.Sprintf("select-%d", i), protocol.BuffEnhancementRequest{Skill: skill, Kind: 48, List: 46, Slot: 65535})
		if err != nil {
			t.Fatal(err)
		}
	}
	second, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	var restored storage.Character
	rows, err := second.Characters(ctx, account)
	if err != nil || len(rows) != 1 {
		t.Fatalf("reconnect: %v", err)
	}
	restored = rows[0]
	got, err := s.BuffEnhancementRestore(restored)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 1, 1, 13, 0, 57, 0}) {
		t.Fatalf("reconnect restore %x %v", got, err)
	}
	var old, next map[string]json.RawMessage
	json.Unmarshal(before, &old)
	json.Unmarshal(restored.State, &next)
	for _, key := range []string{"inventory", "future", "quest_state", "learned_skills"} {
		var a, b any
		json.Unmarshal(old[key], &a)
		json.Unmarshal(next[key], &b)
		ar, _ := json.Marshal(a)
		br, _ := json.Marshal(b)
		if !bytes.Equal(ar, br) {
			t.Fatalf("changed %s", key)
		}
	}
	role, _, err = s.SaveBuffEnhancement(ctx, restored, "clear", protocol.BuffEnhancementRequest{Skill: 306, Kind: 13, List: 46, Slot: 65535})
	if err != nil {
		t.Fatal(err)
	}
	got, err = s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 0}) {
		t.Fatal("database clear failed")
	}
	// Same event key must not reapply a conflicting request after a successful clear.
	role, changed, err := s.SaveBuffEnhancement(ctx, role, "clear", protocol.BuffEnhancementRequest{Skill: 306, Kind: 13, List: 0, Slot: 57})
	if err != nil || changed {
		t.Fatal("receipt replay changed state")
	}
	if !strings.Contains(string(role.State), "buff_enhancement") {
		t.Fatal("save missing")
	}
}
