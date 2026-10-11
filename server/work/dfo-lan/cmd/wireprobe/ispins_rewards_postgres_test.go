package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

type ispinsDirectRewardSource struct{}

func (ispinsDirectRewardSource) RewardBox(uint32) (loot.RewardBox, bool) {
	return loot.RewardBox{}, false
}
func (ispinsDirectRewardSource) Container(uint32) bool { return false }
func (ispinsDirectRewardSource) Item(id uint32) bool   { return id == 21 || id == 22 }

func TestIspinsPostgresSettlementDisplayMatchesCommittedInventory(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	t.Setenv("DFO_ISPINS_MODE", "unlimited")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	f, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	store := f.Storage()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = store.MigrateCharacterEvents(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := store.DevelopmentAccount(ctx, "ispins-source-fixture")
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	source := pvf.ArchiveSnapshot{Checksum: hash}
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "IspinsSourceFixture", ConfigVersion: source.SaveIdentity(), Request: []byte{0}, State: json.RawMessage(`{"keep":true}`)}, 24)
	if err != nil {
		t.Fatal(err)
	}
	role.WireID = 7
	content := catalog.LegionContent{Name: "Ispins", PhaseReward: 1, Dungeons: []catalog.LegionDungeon{{Dungeon: 100}}, Rewards: []catalog.LegionRewardRow{{PartyType: "solo", Difficulty: "normal", Kind: "basic", Values: []int64{1, 21, 8}}}}
	d := &dungeon.Session{Definition: catalog.DungeonDefinition{ID: 100}, RunID: "source-stage", StartedAt: time.Now().Add(-time.Second)}
	d.MarkSceneCompleted()
	w := &worldSession{store: store, account: account, role: role, activeDungeon: d, ispins: &ispinsRun{confirmed: true, operationIndex: 11}, dungeons: &catalog.DungeonCatalog{Source: source, LegionContents: &catalog.LegionContents{Source: source, Contents: map[string]catalog.LegionContent{"Ispins": content}}, IspinsOperations: map[uint16]catalog.IspinsOperationDefinition{11: {Type: 6, FixedValue: 4, Difficulty: "normal", Rewards: []catalog.LegionOperationReward{{Difficulty: "normal", Template: 22, Amount: 3, Weight: 1}}}}}, loot: &loot.Service{Catalog: catalog.LootCatalog{Source: source, Items: map[uint32]catalog.LootItem{21: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}, 22: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}}}, BagRules: inventory.BagRules{Source: hash, Slots: map[string][2]uint16{"[etc]": {68, 69}}}, RewardBoxes: ispinsDirectRewardSource{}}}
	packets, err := w.completeIspinsStage()
	if err != nil {
		t.Fatal(err)
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil || len(bag.Items) != 2 || bag.Items[0].Amount != 8 || bag.Items[1].Amount != 3 {
		t.Fatal("source products not committed", bag, err)
	}
	snapshot, err := w.loot.Bootstrap(workflow.LootRole(w.role))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, p := range packets {
		switch p.Name {
		case "ispins_saved_inventory_refreshed":
			if !bytes.Equal(p.Payload, snapshot) {
				t.Fatal("inventory notification preceded grant")
			}
			seen[p.Name] = true
		case "ispins_basic_clear_reward":
			if binary.LittleEndian.Uint32(p.Payload[1600:]) != 21 || binary.LittleEndian.Uint32(p.Payload[1604:]) != 8 {
				t.Fatal("wrong basic projection")
			}
			seen[p.Name] = true
		case "ispins_additional_clear_reward":
			if p.Payload[0] != 1 || binary.LittleEndian.Uint32(p.Payload[1:]) != 22 || binary.LittleEndian.Uint32(p.Payload[5:]) != 3 {
				t.Fatal("wrong operation projection")
			}
			seen[p.Name] = true
		}
	}
	if len(seen) != 3 {
		t.Fatal("settlement notifications missing", seen)
	}
	before := string(w.role.State)
	if _, err = w.completeIspinsStage(); err != nil {
		t.Fatal(err)
	}
	var first, second any
	if err = json.Unmarshal([]byte(before), &first); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(w.role.State, &second); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("settlement replay changed saved items", first, second)
	}
	if !strings.Contains(before, `"keep":true`) {
		t.Fatal("old save data lost")
	}
}
