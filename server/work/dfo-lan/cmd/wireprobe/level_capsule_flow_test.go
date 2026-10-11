package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/testfixture"
	"dfolan/internal/world"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCapsuleSQLiteAtomicUpgradeReplayAndRollback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	store, err := database.Open(ctx, database.Config{Driver: database.DriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "capsule.sqlite3"), MaxConnections: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	runCapsuleAtomicUpgrade(t, ctx, store)
}

func TestCapsulePostgresAtomicUpgradeReplayAndRollback(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("isolated PostgreSQL schema required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	fixture, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	runCapsuleAtomicUpgrade(t, ctx, fixture.Storage())
}

func runCapsuleAtomicUpgrade(t *testing.T, ctx context.Context, store *database.Store) {
	t.Helper()
	// PostgreSQL JSONB canonicalizes whitespace/key order on a read; compare
	// saved values rather than treating that serialization as a second grant.
	sameState := func(a, b []byte) bool {
		var x, y any
		if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
			return false
		}
		return reflect.DeepEqual(x, y)
	}
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	professions, err := catalog.LoadCharacters("../../configs/characters.skycastle-release.json")
	if err != nil {
		t.Fatal(err)
	}
	growth, err := catalog.LoadProgression(testfixture.ProgressionPath(t))
	if err != nil {
		t.Fatal(err)
	}
	prog := &character.ProgressionService{Store: store, Catalog: growth, Professions: professions, Rules: character.GrowthRules{LevelCap: 115}}
	account, err := store.DevelopmentAccount(ctx, "capsule-atomic-fixture")
	if err != nil {
		t.Fatal(err)
	}
	p := professions.Professions[0]
	s := character.State{Level: 110, Experience: growth.Thresholds[108], Advancement: 1, Attributes: maps.Clone(p.InitialAttributes), InitialSkills: p.InitialSkills, SourcePath: p.Path, SourceSHA256: p.RawSHA256, SkillPoints: [2]uint16{17, 23}}
	state, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	json.Unmarshal(state, &fields)
	fields["future_fixture"] = json.RawMessage(`{"keep":true}`)
	state, _ = json.Marshal(fields)
	state, err = inventory.SaveBag(state, inventory.Bag{Version: "ordinary-bag-v1", Gold: 321, Coin: 7, Items: []inventory.BagItem{{Slot: 66, Template: 50080657, Amount: 2}, {Slot: 67, Template: 1001, Amount: 5}}, Worn: []inventory.BagEquipment{{Slot: 13, Template: 2002, Durability: 7}}, TitleSkin: 2003, TitleBook: map[byte][]inventory.TitleBookEntry{0: {{Equipment: inventory.BagEquipment{Slot: 2, Template: 2003}}}}})
	if err != nil {
		t.Fatal(err)
	}
	makeRole := func(name string, state json.RawMessage) database.Character {
		t.Helper()
		r, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: name, Profession: 0, ConfigVersion: growth.Source.SaveIdentity(), Request: []byte{0, 255}, State: state}, 24)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	role, other := makeRole("CapsuleAtomic", state), makeRole("CapsuleUnchanged", state)
	tag := func(v string) pvf.Token { return pvf.Token{Type: 3, Text: v} }
	txt := func(v string) pvf.Token { return pvf.Token{Type: 6, Text: v} }
	num := func(v int32) pvf.Token { return pvf.Token{Type: 0, Value: v} }
	script := catalog.ScriptRecord{Path: "stackable/50080001/50080657.stk", SHA256: strings.Repeat("a", 64), Cells: []pvf.Token{
		tag("[usable job]"), txt("[all]"), tag("[/usable job]"), tag("[maximum level]"), num(114), tag("[action type]"), txt("[jumping lv]"), tag("[/action type]"), tag("[increase status type]"), txt("[levelUpByTable]"), num(1), num(114), num(115), tag("[/increase status type]"), tag("[action usable place]"), txt("[seria room]"), tag("[/action usable place]"), tag("[impossible contents]"), txt("ban redeemitem"), txt("hardcore mode"), tag("[/impossible contents]"),
	}}
	c := catalog.LootCatalog{Source: growth.Source, Items: map[uint32]catalog.LootItem{50080657: {ID: 50080657, Kind: "stackable", Script: script}}}
	w := &worldSession{account: account, role: role, level: 110, characters: &character.Service{Catalog: professions}, progression: prog, loot: &loot.Service{Catalog: c}, service: &world.Service{Catalog: catalog.WorldCatalog{Source: c.Source, Areas: map[string]catalog.WorldArea{"38/1": {Town: 38, Area: 1, SeriaReturnWarp: true}}}}, state: database.WorldState{Position: database.WorldPosition{Town: 38, Area: 1}}}
	w.characters.Store = store
	w.characters.ProfileFrames = func(ctx context.Context, r character.Character) ([2]uint32, error) {
		return loadProfileFrames(ctx, store, r, nil)
	}
	request, _ := hex.DecodeString("42000000000000a00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")
	use := func(raw []byte) ([]outboundPacket, error) {
		return w.useJumpLevelCapsule(ctx, request, raw, "native-fixture", nil)
	}
	w.state.Position.Area = 2
	if _, err := use(request); err == nil {
		t.Fatal("missing source room admitted")
	}
	w.state.Position.Area = 1
	plan, err := use(request)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 6 || plan[0].ID != 14 || plan[1].ID != 2 || plan[1].Payload[0] != 0 || plan[2].ID != 2 || plan[2].Payload[0] != 1 || plan[3].ID != 14 || plan[4].ID != 560 || plan[4].Kind != 1 || plan[5].ID != 37 || plan[5].Payload[0] != 115 {
		t.Fatal("wrong absolute result plan", plan)
	}
	// Independent native mode0 offsets: same live actor and name, new level.
	if binary.LittleEndian.Uint16(plan[1].Payload[165:]) != role.WireID || plan[1].Payload[173+len(role.Name)] != 115 {
		t.Fatal("actor detail retains old identity or level")
	}
	worn, err := inventory.WornSpaceUpdate(w.role.State)
	if err != nil || !reflect.DeepEqual(plan[3].Payload, worn) {
		t.Fatal("actor rebuild failed to restore worn instances")
	}
	if !reflect.DeepEqual(plan[4].Payload, protocol.TitleAppearanceSuccess(2003)) {
		t.Fatal("actor rebuild lost applied title-book appearance")
	}
	var after character.State
	json.Unmarshal(w.role.State, &after)
	var sp uint32
	for n := uint16(111); n <= 115; n++ {
		sp += uint32(growth.SkillPoints[n])
	}
	if after.Level != 115 || after.Experience != growth.Thresholds[113] || after.SkillPoints[0] != 17+uint16(sp) || after.SkillPoints[1] != 23+uint16(sp) {
		t.Fatal("source growth was not applied", after.Level, after.Experience, after.SkillPoints)
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil || len(bag.Items) != 2 || bag.Items[0].Amount != 1 || bag.Items[1].Amount != 5 || bag.Gold != 321 || bag.Coin != 7 || len(bag.Worn) != 1 || bag.Worn[0].Template != 2002 || bag.Worn[0].Durability != 7 || bag.TitleSkin != 2003 || !bag.OwnsBookTitle(2003) {
		t.Fatal("bad atomic inventory result", bag, err)
	}
	committed := append([]byte(nil), w.role.State...)
	if _, err := use(request); err != nil || !sameState(committed, w.role.State) {
		t.Fatal("native retry reapplied capsule", err)
	}
	if _, err := use(append(append([]byte(nil), request...), 1)); err == nil || !sameState(committed, w.role.State) {
		t.Fatal("new request spent capsule on ineligible level", err)
	}
	rows, err := store.Characters(ctx, account)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.ID == other.ID && !sameState(r.State, other.State) {
			t.Fatal("unrelated character changed")
		}
		if r.ID == role.ID && !sameState(r.State, committed) {
			t.Fatal("committed save and emitted state differ")
		}
	}
	// A growth validation failure occurs after reading the capsule but rolls
	// back both the character update and consumption on either storage engine.
	s.SkillPoints = [2]uint16{65535, 65535}
	overflow, _ := json.Marshal(s)
	overflow, _ = inventory.SaveBag(overflow, inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 66, Template: 50080657, Amount: 1}}})
	failRole := makeRole("CapsuleRollback", overflow)
	if _, _, err := prog.UseLevelCapsule(ctx, failRole, c, 66, true, "capsule-overflow", time.Now()); err == nil {
		t.Fatal("SP overflow accepted")
	}
	rows, err = store.Characters(ctx, account)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.ID == failRole.ID && !sameState(r.State, failRole.State) {
			t.Fatal("failed upgrade consumed item or changed save")
		}
	}
	if _, err := store.CharacterEventReceipt(ctx, account, failRole.ID, "capsule-overflow"); err == nil {
		t.Fatal("failed upgrade committed a success receipt")
	}
	if _, err := protocol.DecodeJumpLevelCapsule(request); err != nil {
		t.Fatal(err)
	}
}
