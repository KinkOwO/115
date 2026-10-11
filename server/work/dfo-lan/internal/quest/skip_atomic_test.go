package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"encoding/json"
	"reflect"
	"testing"
)

func ticketTestService() *Service {
	return &Service{Skip: &catalog.QuestSkip{Items: []uint32{20, 21}}, Inventory: &inventory.Awarder{
		Catalog: catalog.LootCatalog{Items: map[uint32]catalog.LootItem{
			20: {Kind: "stackable", StackableType: "[waste]", StackLimit: 1}, 21: {Kind: "stackable", StackableType: "[waste]", StackLimit: 1},
		}}, Rules: inventory.BagRules{Slots: map[string][2]uint16{"default": {65, 66}, "[waste]": {65, 66}}, MissingStackLimit: 1},
	}}
}

func TestTicketItemsFailureReturnsNoPartialStateOrReceipt(t *testing.T) {
	s := ticketTestService()
	s.Inventory.Rules.Slots["[waste]"] = [2]uint16{65, 65}
	s.Inventory.Rules.Slots["default"] = [2]uint16{65, 65}
	before := json.RawMessage(`{"level":115,"untouched":{"keep":true}}`)
	state, items, receipt, err := s.prepareTicketItems(before, nil, "test", "retry")
	if err == nil || state != nil || items != nil || receipt != nil {
		t.Fatalf("partial grant escaped rollback: %s %v %s %v", state, items, receipt, err)
	}
	s.Inventory.Rules.Slots["[waste]"] = [2]uint16{65, 66}
	state, items, receipt, err = s.prepareTicketItems(before, nil, "test", "retry")
	if err != nil || !reflect.DeepEqual(items, []uint32{20, 21}) {
		t.Fatalf("retry: %v %v", items, err)
	}
	bag, _ := inventory.ReadBag(state)
	if len(bag.Items) != 2 {
		t.Fatalf("bag %v", bag.Items)
	}
	repeated, items, next, err := s.prepareTicketItems(state, receipt, "test", "repeat")
	if err != nil || len(items) != 0 || next != nil || string(repeated) != string(state) {
		t.Fatal("completed receipt regranted items")
	}
}

func TestTicketItemsLegacyPartialReceiptRetriesOnlyFailures(t *testing.T) {
	s := ticketTestService()
	state, _, err := s.Inventory.Grant(json.RawMessage(`{}`), 20, 1)
	if err != nil {
		t.Fatal(err)
	}
	state, items, receipt, err := s.prepareTicketItems(state, json.RawMessage(`{"granted":[20],"failed":{"21":"full"}}`), "test", "repair")
	if err != nil || !reflect.DeepEqual(items, []uint32{21}) {
		t.Fatalf("legacy retry %v %v", items, err)
	}
	bag, _ := inventory.ReadBag(state)
	if len(bag.Items) != 2 {
		t.Fatalf("duplicated successful key: %v", bag.Items)
	}
	var record struct {
		Granted []uint32
		Failed  map[uint32]string
	}
	_ = json.Unmarshal(receipt, &record)
	if !reflect.DeepEqual(record.Granted, []uint32{20, 21}) || len(record.Failed) > 0 {
		t.Fatalf("receipt %+v", record)
	}
}

func TestSkipClosureKeepsMutuallyExclusiveBranches(t *testing.T) {
	a, b := epicEntry(1), epicEntry(2)
	a.Collisions = []uint32{2}
	b.Collisions = []uint32{1}
	s := closureService(closureIndex(a, b), nil)
	ids, _, _ := s.skipClosure("[mage]", closureState, map[uint32]string{}, MainlineGrades(), 0)
	if len(ids) != 0 {
		t.Fatalf("unaccepted branch was chosen automatically: %v", ids)
	}
	ids, _, _ = s.skipClosure("[mage]", closureState, map[uint32]string{1: "accepted"}, MainlineGrades(), 0)
	if !reflect.DeepEqual(ids, []uint32{1}) {
		t.Fatalf("accepted branch not preserved: %v", ids)
	}
	ids, _, _ = s.skipClosure("[mage]", closureState, map[uint32]string{2: "completed"}, MainlineGrades(), 0)
	if len(ids) != 0 {
		t.Fatalf("completed collision bypassed: %v", ids)
	}
}
