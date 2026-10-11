package main

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"testing"
)

func TestBlackPurgatoryPartyRestoresOwnedEquipmentAfterCaptainRebuild(t *testing.T) {
	weapon := protocol.OrdinaryItem(12, 101000013, 0)
	weapon[60] = 0x7a // Rich instance data must survive the owner's NOTI13.
	state, err := inventory.SaveBag(json.RawMessage(`{"future":{"keep":true}}`), inventory.Bag{
		Version: "ordinary-bag-v1",
		Worn: []inventory.BagEquipment{
			{Slot: 12, Template: 101000013, Record: append([]byte(nil), weapon[:]...)},
			{Slot: 47, Template: 100610096},
			{Slot: 3, Template: 517500000},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	before := bytes.Clone(state)
	plan, err := blackPurgatoryPartyPlan([]byte{1}, []byte{2}, []byte{3}, state, nil)
	if err != nil || !bytes.Equal(state, before) {
		t.Fatal("party projection changed saved equipment", err)
	}
	want, err := inventory.WornPayload(state)
	if err != nil {
		t.Fatal(err)
	}
	rebuild, worn, visual, party := -1, -1, -1, -1
	for i, p := range plan {
		switch {
		case p.ID == 2:
			rebuild = i
		case p.ID == 13 && len(p.Payload) > 0 && p.Payload[0] == 3:
			worn = i
			if !bytes.Equal(p.Payload, want) || !bytes.Contains(p.Payload, weapon[:]) {
				t.Fatal("captain inventory lost an instance or oath slot")
			}
		case p.ID == 14:
			visual = i
		case p.ID == 9:
			party = i
		}
	}
	if rebuild < 0 || worn <= rebuild || visual <= worn || party <= visual {
		t.Fatal("captain rebuild must precede inventory, visuals and party", rebuild, worn, visual, party)
	}
}

func TestBlackPurgatoryPartyRejectsInvalidInventoryBeforePublishing(t *testing.T) {
	plan, err := blackPurgatoryPartyPlan(nil, nil, nil, json.RawMessage(`{"inventory":{"version":"invalid"}}`), nil)
	if err == nil || len(plan) != 0 {
		t.Fatal("invalid inventory must not publish a partial party")
	}
}
