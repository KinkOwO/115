package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"strings"
	"testing"
)

func netTransformFixture(t *testing.T, oldGrades, targetGrades []int32) (*ItemService, Role, []uint32, []uint32) {
	t.Helper()
	source := pvf.ArchiveSnapshot{Checksum: strings.Repeat("e", 64)}
	var defs []EquipmentDefinition
	var slots, targets []uint32
	bag := Bag{Version: "ordinary-bag-v1", Gold: 1_000_000}
	journal := EquipmentJournal{Counts: map[uint32]uint32{}}
	for i := range oldGrades {
		old, target := uint32(700+i), uint32(800+i)
		for j, id := range []uint32{old, target} {
			rarity := oldGrades[i]
			if j == 1 {
				rarity = targetGrades[i]
			}
			defs = append(defs, EquipmentDefinition{ID: id, Path: "fixture.equ", SHA256: strings.Repeat("a", 64), Fields: map[string][]pvf.Token{
				"[grade]": {{Type: 0, Value: 115}}, "[rarity]": {{Type: 0, Value: rarity}},
				"[minimum level]": {{Type: 0, Value: 115}}, "[durability]": {{Type: 0, Value: 60}},
				"[equipment type]": {{Type: 6, Text: "[coat]"}},
			}})
		}
		slot := uint16(14 + i)
		bag.Worn = append(bag.Worn, BagEquipment{Slot: slot, Template: old, Refine: 3})
		journal.Counts[target] = 1
		slots = append(slots, uint32(slot))
		targets = append(targets, target)
	}
	eq, err := NewEquipmentCatalog(EquipmentCatalog{Source: source, Rows: defs}, source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	table, err := catalog.ParseEquipmentTransformSystem(transformSystemFixtureText)
	if err != nil {
		t.Fatal(err)
	}
	// Exactly the five soul rows from the current PVF; branch-specific star
	// refunds are covered separately below.
	for _, row := range []struct {
		name string
		soul uint32
	}{{"rare", 10361512}, {"unique", 10361513}, {"legendary", 10361514}, {"epic", 10361515}, {"primeval", 10361516}} {
		for _, branch := range []int{0, 1} {
			table.EquipmentRefund = append(table.EquipmentRefund, catalog.TransformRefundRow{Rarity: row.name, Branch: branch, Items: []catalog.TransformMaterial{{Template: row.soul, Count: 1}}})
		}
	}
	table.EquipmentRefund = table.EquipmentRefund[1:]
	s := &ItemService{Catalog: catalog.LootCatalog{Source: source}, Equipment: eq,
		CreateCost: &catalog.EquipmentCreateCost{Source: source}, Transform: &table, Journal: &catalog.EquipmentJournalRules{Maximum: 99}}
	raw, err := SaveBag(json.RawMessage(`{"level":115,"advancement":0,"untouched":42}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = SaveEquipmentJournal(raw, journal)
	if err != nil {
		t.Fatal(err)
	}
	return s, Role{State: raw, ConfigVersion: source.SaveIdentity()}, slots, targets
}

func TestEquipmentTransformBatchNetSouls(t *testing.T) {
	for _, sameRarity := range []bool{false, true} {
		old := []int32{6, 6, 6, 6, 6, 6, 6, 4, 4, 4, 4}
		target := []int32{4, 4, 4, 4, 4, 4, 4, 6, 6, 6, 6}
		if sameRarity {
			target = append([]int32(nil), old...)
		}
		s, role, slots, templates := netTransformFixture(t, old, target)
		account := NewAccountMaterials()
		if !sameRarity {
			account, _, _ = account.Add(10361515, 3)
		}
		raw, _ := account.Save()
		plan, err := s.PlanEquipmentTransform(role, slots, templates, 1)
		if err != nil {
			t.Fatal(err)
		}
		updated, paid, vault, receipt, err := s.PrepareEquipmentTransform(role, raw, 0, plan)
		if err != nil {
			t.Fatalf("sameRarity=%t: %v", sameRarity, err)
		}
		bag, _ := ReadBag(updated)
		store, _ := ReadAccountMaterials(paid)
		wantLegend, wantGold := uint32(3), uint32(580000)
		if sameRarity {
			wantLegend, wantGold = 0, 595000
		}
		if len(receipt.Pairs) != 11 || len(receipt.Refunds) != 11 || store.Count(10361515) != 0 || store.Count(10361514) != wantLegend || bag.Gold != wantGold || vault != 0 {
			t.Fatalf("receipt=%+v account=%+v gold=%d vault=%d", receipt, store, bag.Gold, vault)
		}
		for i, w := range bag.Worn {
			if w.Template != templates[i] || w.Refine != 3 {
				t.Fatalf("worn=%+v", w)
			}
		}
		journal, _ := ReadEquipmentJournal(updated)
		for i, id := range templates {
			if journal.Counts[id] != 0 || journal.Counts[uint32(700+i)] != 1 {
				t.Fatalf("journal=%v", journal.Counts)
			}
		}
		// Re-select reads the saved character document, independent of UI.
		reloaded, _ := ReadBag(append(json.RawMessage(nil), updated...))
		if reloaded.Worn[0].Template != templates[0] {
			t.Fatal("saved transform reverted")
		}
	}
}

func TestEquipmentTransformNetFailureLeavesAssetsUntouched(t *testing.T) {
	s, role, slots, templates := netTransformFixture(t, []int32{6, 6}, []int32{4, 4})
	account := NewAccountMaterials()
	account, _, _ = account.Add(10361515, 1)
	raw, _ := account.Save()
	before := string(role.State)
	plan, err := s.PlanEquipmentTransform(role, slots, templates, 1)
	if err != nil {
		t.Fatal(err)
	}
	state, mats, _, _, err := s.PrepareEquipmentTransform(role, raw, 0, plan)
	if err == nil || state != nil || mats != nil || string(role.State) != before || account.Count(10361515) != 1 {
		t.Fatalf("partial payment: err=%v state=%s mats=%s", err, state, mats)
	}
	_, err = s.PlanEquipmentTransform(role, []uint32{slots[0], slots[0]}, []uint32{templates[0], templates[0]}, 1)
	if err == nil {
		t.Fatal("duplicate source accepted")
	}
}

func TestEquipmentTransformAlternativePaymentAggregatesBagCosts(t *testing.T) {
	s, role, slots, templates := netTransformFixture(t, []int32{4, 4}, []int32{4, 4})
	bag, _ := ReadBag(role.State)
	bag.Items = []BagItem{{Slot: 9, Template: 10401346, Amount: 8}}
	role.State, _ = SaveBag(role.State, bag)
	plan, err := s.PlanEquipmentTransform(role, slots, templates, 2)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := NewAccountMaterials().Save()
	_, _, _, _, err = s.PrepareEquipmentTransform(role, raw, 0, plan)
	if err == nil {
		t.Fatal("eight seals paid a sixteen-seal batch")
	}
	bag.Items[0].Amount = 16
	role.State, _ = SaveBag(role.State, bag)
	plan, _ = s.PlanEquipmentTransform(role, slots, templates, 2)
	updated, paid, _, receipt, err := s.PrepareEquipmentTransform(role, raw, 0, plan)
	if err != nil {
		t.Fatal(err)
	}
	bag, _ = ReadBag(updated)
	store, _ := ReadAccountMaterials(paid)
	if len(bag.Items) != 0 || bag.Gold != 1_000_000 || store.Count(10361515) != 0 || receipt.Gold != 0 {
		t.Fatalf("bag=%+v account=%+v", bag, store)
	}
}

func TestEquipmentTransformRefundUsesCommonPVFRows(t *testing.T) {
	s, _, _, _ := netTransformFixture(t, []int32{8}, []int32{8})
	for i := range s.Transform.EquipmentRefund {
		row := &s.Transform.EquipmentRefund[i]
		if row.Rarity == "primeval" && row.Branch == 0 {
			row.Items = append(row.Items, catalog.TransformMaterial{Template: 10403609, Count: 1})
		}
	}
	refund := s.equipmentTransformSoulRefund(700)
	if len(refund) != 1 || refund[0].Template != 10361516 || refund[0].Count != 1 {
		t.Fatalf("refund=%v", refund)
	}
	// A source change in one branch must not silently select the larger value.
	for i := range s.Transform.EquipmentRefund {
		row := &s.Transform.EquipmentRefund[i]
		if row.Rarity == "primeval" && row.Branch == 1 {
			row.Items[0].Count = 2
		}
	}
	if got := s.equipmentTransformSoulRefund(700); len(got) != 0 {
		t.Fatalf("ambiguous refund=%v", got)
	}
}
