package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"encoding/json"
	"testing"
)

func TestEpicSeekingSourceForms(t *testing.T) {
	quests, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	x := BuildIndex(quests)
	want := map[uint32]struct {
		dungeon, items, rewards uint32
	}{
		3292: {46, 6, 9}, // exact duplicate source rows collapse
		3293: {41, 1, 1},
		3594: {93, 1, 1},
	}
	implemented := 0
	for id, d := range quests.Quests {
		if d.Kind != "[seeking]" {
			continue
		}
		en := x.Entries[id]
		if en == nil || !en.Implemented {
			continue
		}
		implemented++
		w, ok := want[id]
		if !ok || en.Model != SeekingItems || en.Initial != 1 || en.Seeking.Dungeon != w.dungeon ||
			uint32(len(en.Seeking.Items)) != w.items || uint32(len(en.Seeking.Rewards)) != w.rewards {
			t.Fatalf("unexpected seeking projection for quest %d: %+v", id, en)
		}
	}
	if implemented != len(want) {
		t.Fatalf("implemented %d epic seeking rows, want %d", implemented, len(want))
	}
	if en := x.Entries[3594]; len(en.PrerequisiteGroups) != 1 || len(en.PrerequisiteGroups[0]) != 1 || en.PrerequisiteGroups[0][0] != 3587 {
		t.Fatalf("quest 3594 prerequisite drift: %+v", en.PrerequisiteGroups)
	}
}

func TestSeekingConsumptionSpansStacksAndPreservesState(t *testing.T) {
	raw, err := inventory.SaveBag(json.RawMessage(`{"level":66,"marker":7}`), inventory.Bag{
		Version: "ordinary-bag-v1",
		Items: []inventory.BagItem{
			{Slot: 10, Template: 10164797, Amount: 1},
			{Slot: 11, Template: 10164797, Amount: 2},
			{Slot: 12, Template: 6001, Amount: 9},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, receipts, err := consumeSeekingItems(raw, []ItemNeed{{10164797, 2}})
	if err != nil {
		t.Fatal(err)
	}
	bag, err := inventory.ReadBag(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 1 || receipts[0].Amount != 2 || len(receipts[0].Slots) != 2 || itemCounts(bag)[10164797] != 1 || itemCounts(bag)[6001] != 9 {
		t.Fatalf("unexpected consume result: receipts=%+v bag=%+v", receipts, bag.Items)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(out, &fields); err != nil || string(fields["marker"]) != "7" {
		t.Fatalf("non-inventory state was not preserved: %s (%v)", out, err)
	}
	if _, _, err := consumeSeekingItems(out, []ItemNeed{{10164797, 2}}); err == nil {
		t.Fatal("insufficient quest items were consumed")
	}
}

func TestSeekingMonsterItemUsesDirectBagStackable(t *testing.T) {
	items, err := catalog.LoadLoot("../../configs/loot.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = items.SupplementStackables("../catalog/testdata/item-flow.json"); err != nil {
		t.Fatal(err)
	}
	rules, err := inventory.LoadBagRules("../../configs/inventory.compat90.json")
	if err != nil {
		t.Fatal(err)
	}
	if got := items.Items[10164797]; got.Kind != "stackable" || got.StackableType != "[quest]" {
		t.Fatalf("quest item source projection drift: %+v", got)
	}
	awarder := inventory.Awarder{Catalog: items, Rules: rules}
	raw, receipt, err := awarder.Grant(json.RawMessage(`{"level":66}`), 10164797, 1)
	if err != nil {
		t.Fatal(err)
	}
	bag, err := inventory.ReadBag(raw)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Template != 10164797 || receipt.Amount != 1 || len(receipt.Slots) != 1 || itemCounts(bag)[10164797] != 1 {
		t.Fatalf("quest item was not granted directly to the bag: receipt=%+v bag=%+v", receipt, bag.Items)
	}
}
