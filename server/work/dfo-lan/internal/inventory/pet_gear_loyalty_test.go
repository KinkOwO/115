package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"testing"
)

func TestPetGearPlacementAndOldSaveSweep(t *testing.T) {
	gear := &EquipmentCatalog{index: map[uint32]EquipmentDefinition{
		63501: {ID: 63501, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[artifact red]"}}}},
	}}
	bag := Bag{Version: "ordinary-bag-v1", Equipment: []BagEquipment{{Slot: 9, Template: 63501, Durability: 4}},
		Special: map[byte][]BagEquipment{7: {{Slot: 1, Template: 63501, Durability: 5}, {Slot: 0, Template: 63009}}}}
	got, moved, err := SweepPetGear(bag, gear)
	if err != nil || !moved || len(got.Equipment) != 0 || len(got.Special[7]) != 3 {
		t.Fatalf("sweep: %+v, moved=%v, err=%v", got, moved, err)
	}
	if got.Special[7][1].Slot != 320 || got.Special[7][2].Slot != 321 || got.Special[7][1].Durability != 4 || got.Special[7][2].Durability != 5 {
		t.Fatalf("pet gear slots or instances changed: %+v", got.Special[7])
	}
	again, moved, err := SweepPetGear(got, gear)
	if err != nil || moved || len(again.Special[7]) != 3 {
		t.Fatalf("sweep is not idempotent: %+v, %v, %v", again, moved, err)
	}
	if err := WearableBy(map[string][]pvf.Token{"[usable job]": {{Type: 6, Text: "[all]"}}}, "[artifact red]", "[fighter]", 0, 1); err != nil {
		t.Fatalf("artifact without minimum level refused: %v", err)
	}
}

func TestPetGearMovesFromPetPageToBodySlot(t *testing.T) {
	service, role := wearFixture(t)
	service.Catalog.index[63501] = EquipmentDefinition{ID: 63501, Fields: map[string][]pvf.Token{
		"[equipment type]": {{Type: 6, Text: "[artifact red]"}},
		"[usable job]":     {{Type: 6, Text: "[all]"}},
	}}
	bag, err := ReadBag(role.State)
	if err != nil {
		t.Fatal(err)
	}
	bag.Special = map[byte][]BagEquipment{7: {{Slot: PetGearFirst, Template: 63501}}}
	role.State, err = SaveBag(role.State, bag)
	if err != nil {
		t.Fatal(err)
	}
	state, err := service.MoveOrdinary(role, protocol.ItemMoveRequest{SourceList: 7, SourceSlot: PetGearFirst,
		SourceItem: 63501, DestinationList: 3, DestinationSlot: 27, Count: 1, Selection: 0xffffffff})
	if err != nil {
		t.Fatal(err)
	}
	bag, err = ReadBag(state)
	if err != nil || len(bag.Special[7]) != 0 || len(bag.Worn) != 1 || bag.Worn[0].Slot != 27 || bag.Worn[0].Template != 63501 {
		t.Fatalf("pet gear not worn: %+v, %v", bag, err)
	}
}

func TestPetFeedConsumptionUsesPetContainer(t *testing.T) {
	items := catalog.LootCatalog{Items: map[uint32]catalog.LootItem{24: {ID: 24, Kind: "stackable", StackableType: "[feed]"}}}
	bag := Bag{Version: "ordinary-bag-v1", Items: []BagItem{{Slot: 65, Template: 24, Amount: 9}}, PetItems: []BagItem{{Slot: 376, Template: 24, Amount: 2}}}
	got, remaining, err := bag.ConsumePet(items, 376, 24)
	if err != nil || remaining != 1 || got.PetItems[0].Amount != 1 || got.Items[0].Amount != 9 {
		t.Fatalf("pet consume touched the wrong container: %+v, %d, %v", got, remaining, err)
	}
	if _, _, err := bag.ConsumePet(items, 65, 24); err == nil {
		t.Fatal("ordinary bag slot accepted as a pet slot")
	}
	if IsPetFeed("[creature expitem]") {
		t.Fatal("experience item classified as feed")
	}
}

func TestCreatureLoyaltyOfficialHourlyRatesAndAutoFeed(t *testing.T) {
	items := catalog.LootCatalog{Items: map[uint32]catalog.LootItem{24: {ID: 24, Kind: "stackable", StackableType: "[feed]"}}}
	bag := Bag{Version: "ordinary-bag-v1", Worn: []BagEquipment{{Slot: 26, Template: 63000}}, PetItems: []BagItem{{Slot: 376, Template: 24, Amount: 1}}}
	state, err := SaveBag(json.RawMessage(`{"other":7}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	state, _, _, err = AdvanceCreatureLoyalty(state, 1000, true, items)
	if err != nil {
		t.Fatal(err)
	}
	state, changed, fed, err := AdvanceCreatureLoyalty(state, 1060, true, items)
	if err != nil || !changed || fed {
		t.Fatalf("dungeon minute: changed=%v fed=%v err=%v", changed, fed, err)
	}
	got, _ := ReadBag(state)
	if creatureSatiety(got, 1) != 99 || got.CreatureExperience[1] != 0 {
		t.Fatalf("dungeon minute changed experience or wrong loyalty: %+v", got)
	}
	state, changed, fed, err = AdvanceCreatureLoyalty(state, 6400, false, items) // 90 dungeon minutes total
	if err != nil || !changed || !fed {
		t.Fatalf("automatic feed: changed=%v fed=%v err=%v", changed, fed, err)
	}
	got, _ = ReadBag(state)
	if creatureSatiety(got, 1) != 100 || len(got.PetItems) != 0 {
		t.Fatalf("feed did not refill and consume exactly one: %+v", got)
	}
	state, _, _, err = AdvanceCreatureLoyalty(state, 6760, false, items)
	if err != nil {
		t.Fatal(err)
	}
	got, _ = ReadBag(state)
	if creatureSatiety(got, 1) != 100 {
		t.Fatalf("town recovery exceeded cap: %+v", got)
	}
	got.CreatureSatiety[1] = 50
	state, err = SaveBag(state, got)
	if err != nil {
		t.Fatal(err)
	}
	state, _, _, err = AdvanceCreatureLoyalty(state, 7120, false, items)
	if err != nil {
		t.Fatal(err)
	}
	got, _ = ReadBag(state)
	if creatureSatiety(got, 1) != 51 {
		t.Fatalf("town did not restore 10 per hour: %+v", got)
	}
	got.Worn = nil
	got.Special = map[byte][]BagEquipment{7: {{Slot: 0, Template: 63000}}}
	state, err = SaveBag(state, got)
	if err != nil {
		t.Fatal(err)
	}
	state, _, _, err = AdvanceCreatureLoyalty(state, 7120, true, items)
	if err != nil {
		t.Fatal(err)
	}
	state, _, _, err = AdvanceCreatureLoyalty(state, 7480, true, items)
	if err != nil {
		t.Fatal(err)
	}
	got, _ = ReadBag(state)
	if creatureSatiety(got, 1) != 52 {
		t.Fatalf("unequipped creature did not recover in dungeon: %+v", got)
	}
}

func TestCreatureLoyaltyLongIntervalConsumesEachNeededFeed(t *testing.T) {
	items := catalog.LootCatalog{Items: map[uint32]catalog.LootItem{24: {ID: 24, Kind: "stackable", StackableType: "[feed]"}}}
	bag := Bag{Version: "ordinary-bag-v1", Worn: []BagEquipment{{Slot: 26, Template: 63000}}, PetItems: []BagItem{{Slot: 376, Template: 24, Amount: 2}}}
	state, err := SaveBag(json.RawMessage(`{}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	state, _, _, err = AdvanceCreatureLoyalty(state, 1000, true, items)
	if err != nil {
		t.Fatal(err)
	}
	state, changed, fed, err := AdvanceCreatureLoyalty(state, 11800, true, items)
	if err != nil || !fed {
		t.Fatalf("long dungeon interval: changed=%v fed=%v err=%v", changed, fed, err)
	}
	got, err := ReadBag(state)
	if err != nil || creatureSatiety(got, 1) != 100 || len(got.PetItems) != 0 {
		t.Fatalf("two feed threshold crossings not settled: %+v, %v", got, err)
	}
}
