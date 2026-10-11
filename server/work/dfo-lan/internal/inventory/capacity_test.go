package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func capacityEquipmentCatalog() *EquipmentCatalog {
	return &EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)}, index: map[uint32]EquipmentDefinition{42: {ID: 42, Fields: map[string][]pvf.Token{
		"[rarity]": {{Type: 0, Value: 2}}, "[equipment type]": {{Type: 6, Text: "[coat]"}}, "[durability]": {{Type: 0, Value: 60}},
	}}}}
}
func capacityFullEquipment(tier byte) Bag {
	b := Bag{Version: "ordinary-bag-v1", Expansion: tier}
	for s := uint16(9); s < 9+40+8*uint16(tier); s++ {
		b.Equipment = append(b.Equipment, BagEquipment{Slot: s, Template: 42, Durability: 60})
	}
	return b
}
func TestEquipmentAwardHonorsExpansion(t *testing.T) {
	for _, tier := range []byte{0, 1, 2} {
		t.Run(fmt.Sprint(tier), func(t *testing.T) {
			full := capacityFullEquipment(tier)
			got, slots, err := full.AddEquipment(capacityEquipmentCatalog(), [2]uint16{9, 64}, 42, 1)
			if err == nil || len(slots) != 0 || !reflect.DeepEqual(got, full) {
				t.Fatalf("full tier %d accepted or mutated: %v %v", tier, slots, err)
			}
			rules := BagRules{EquipmentSlots: [2]uint16{9, 64}}
			if got, err := full.AddMailItem(catalog.LootCatalog{}, rules, capacityEquipmentCatalog(), MailItem{Equipment: &BagEquipment{Template: 42, Durability: 60}}); err == nil || !reflect.DeepEqual(got, full) {
				t.Fatal("equipment mail claim bypassed capacity")
			}
			raw, err := SaveBag(json.RawMessage(`{}`), full)
			if err != nil {
				t.Fatal(err)
			}
			a := Awarder{Rules: rules, Equipment: capacityEquipmentCatalog()}
			if out, _, err := a.Grant(raw, 42, 1); err == nil || len(out) != 0 {
				t.Fatal("generic equipment reward bypassed capacity")
			}
			last := full.Equipment[len(full.Equipment)-1].Slot
			b := full
			b.Equipment = append([]BagEquipment(nil), full.Equipment[:len(full.Equipment)-1]...)
			if got, slots, err = b.AddEquipment(capacityEquipmentCatalog(), [2]uint16{9, 64}, 42, 2); err == nil || !reflect.DeepEqual(got, b) {
				t.Fatal("partial multi-award was accepted or mutated")
			}
			got, slots, err = b.AddEquipment(capacityEquipmentCatalog(), [2]uint16{9, 64}, 42, 1)
			if err != nil || len(slots) != 1 || slots[0] != last || got.Equipment[len(got.Equipment)-1].Durability != 60 {
				t.Fatalf("last unlocked slot: %v %v", slots, err)
			}
			if tier < 2 {
				b = full
				b.Expansion++
				got, slots, err = b.AddEquipment(capacityEquipmentCatalog(), [2]uint16{9, 64}, 42, 1)
				if err != nil || slots[0] != last+1 {
					t.Fatalf("expanded tier did not unlock next slot: %v %v", slots, err)
				}
			}
		})
	}
}
func TestStackAwardsRespectUnlockedCategories(t *testing.T) {
	types := []string{"[waste]", "[material]", "[quest]", "[material expert job]"}
	for i, typ := range types {
		for _, tier := range []byte{0, 1, 2} {
			t.Run(fmt.Sprintf("%s/%d", typ, tier), func(t *testing.T) {
				start := uint16(65 + i*56)
				if typ == "[quest]" {
					start = 177
				}
				if typ == "[material expert job]" {
					start = 233
				}
				rules := BagRules{Source: "test", Slots: map[string][2]uint16{typ: {start, start + 55}}}
				c := catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: "test"}, Items: map[uint32]catalog.LootItem{42: {ID: 42, Kind: "stackable", StackableType: typ, StackLimit: 10}}}
				b := Bag{Version: "ordinary-bag-v1", Expansion: tier}
				for s := start; s < start+40+8*uint16(tier); s++ {
					b.Items = append(b.Items, BagItem{Slot: s, Template: 99, Amount: 1})
				}
				// A partly filled visible stack cannot absorb the whole award. Rejecting
				// must restore the partial merge as well as avoid allocating a locked slot.
				b.Items[0] = BagItem{Slot: start, Template: 42, Amount: 9}
				original := append([]BagItem(nil), b.Items...)
				got, _, err := b.Add(c, rules, 42, 2)
				if err == nil || !reflect.DeepEqual(got.Items, original) || !reflect.DeepEqual(b.Items, original) {
					t.Fatal("full-category award did not roll back")
				}
				shopBag := b
				shopBag.Items = append([]BagItem(nil), b.Items...)
				shopBag.Items[0].Template = 99
				got, _, err = shopBag.Buy(rules, 42, 1, 0, typ)
				if err == nil || !reflect.DeepEqual(got.Items, shopBag.Items) {
					t.Fatal("shop bypassed category capacity")
				}
				got, err = b.AddMailItem(c, rules, nil, MailItem{Stack: &BagItem{Template: 42, Amount: 2}})
				if err == nil || !reflect.DeepEqual(got.Items, original) {
					t.Fatal("mail attachment bypassed category capacity")
				}
			})
		}
	}
}
func TestLockedLegacyRowsRemainLossless(t *testing.T) {
	b := capacityFullEquipment(0)
	b.Equipment = append(b.Equipment, BagEquipment{Slot: 49, Template: 42, Durability: 60, Period: 123})
	b.Items = []BagItem{{Slot: 105, Template: 77, Amount: 3}}
	raw, err := SaveBag(json.RawMessage(`{"future":"keep"}`), b)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadBag(raw)
	if err != nil || !reflect.DeepEqual(got.Equipment, b.Equipment) || !reflect.DeepEqual(got.Items, b.Items) {
		t.Fatal("legacy rows lost", err)
	}
	c := catalog.LootCatalog{Items: map[uint32]catalog.LootItem{77: {ID: 77, Kind: "stackable", StackableType: "[waste]", StackLimit: 10}}}
	got, slot, err := got.Add(c, BagRules{}, 77, 1)
	if err != nil || slot != 65 || got.Items[0].Amount != 3 {
		t.Fatal("award merged into legacy locked stack", err)
	}
}
func TestLockedDestinationsRejected(t *testing.T) {
	rules := BagRules{Source: strings.Repeat("a", 64), EquipmentSlots: [2]uint16{9, 64}, Slots: map[string][2]uint16{"[waste]": {65, 120}}}
	b := Bag{Version: "ordinary-bag-v1", Equipment: []BagEquipment{{Slot: 9, Template: 42, Durability: 60}}}
	perm := make([]uint16, 380)
	for i := range perm {
		perm[i] = uint16(i)
	}
	perm[9], perm[49] = 49, 9
	if _, err := SortItems(b, rules, protocol.SortItemRequest{Slots: perm}); err == nil || b.Equipment[0].Slot != 9 {
		t.Fatal("sort accepted locked destination or mutated input")
	}
	v := Vault{Slots: 8, Items: []VaultItem{{Slot: 0, Template: 42, IsEquip: true, Durability: 60}}}
	if _, got, _, err := MoveVaultItem(b, v, rules, protocol.ItemMoveRequest{SourceList: 2, DestinationList: 0, DestinationSlot: 49}); err == nil || len(got.Items) != 1 {
		t.Fatal("vault withdrawal accepted locked slot")
	}
	gear := capacityEquipmentCatalog()
	svc := WearService{Catalog: gear, Professions: catalog.Characters{Source: gear.Source}, BagRules: rules}
	state, err := SaveBag(json.RawMessage(`{}`), b)
	if err != nil {
		t.Fatal(err)
	}
	role := Role{ConfigVersion: gear.Source.SaveIdentity(), State: state}
	req := protocol.ItemMoveRequest{SourceSlot: 49, DestinationSlot: 9, DestinationItem: 42, Selection: 0xffffffff}
	if _, err = svc.MoveOrdinary(role, req); err == nil {
		t.Fatal("equipment move accepted locked slot")
	}
	// Removing a historical hidden item into a visible empty slot is allowed.
	b.Equipment[0].Slot = 49
	role.State, err = SaveBag(json.RawMessage(`{}`), b)
	if err != nil {
		t.Fatal(err)
	}
	req.SourceSlot, req.DestinationSlot = 9, 49
	if _, err = svc.MoveOrdinary(role, req); err != nil {
		t.Fatal("legacy item cannot move out", err)
	}
	c := catalog.LootCatalog{Source: gear.Source, Items: map[uint32]catalog.LootItem{77: {ID: 77, Kind: "stackable", StackableType: "[waste]", StackLimit: 10}}}
	stacks := Bag{Items: []BagItem{{Slot: 65, Template: 77, Amount: 1}}}
	req = protocol.ItemMoveRequest{SourceSlot: 65, SourceItem: 77, DestinationSlot: 105, Count: 1, Selection: 0xffffffff}
	if _, err = stacks.MoveStackRequest(c, rules, req); err == nil {
		t.Fatal("stack move accepted locked slot")
	}
}
