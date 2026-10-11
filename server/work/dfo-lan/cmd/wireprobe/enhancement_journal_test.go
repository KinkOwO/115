package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestEnhancementDestructionClearsWornSlotWithoutDeletingReplacement(t *testing.T) {
	ack := outboundPacket{"ack", 1, 80, []byte{1}}
	plan, err := appendEnhancementRemoval([]outboundPacket{ack}, inventory.Bag{}, 3, 12)
	if err != nil || len(plan) != 2 {
		t.Fatal(plan, err)
	}
	p := plan[1].Payload
	if plan[0].ID != 80 || plan[1].ID != 14 || p[0] != 3 || binary.LittleEndian.Uint16(p[1:]) != 1 || binary.LittleEndian.Uint16(p[3:]) != 12 || binary.LittleEndian.Uint32(p[5:]) != 0xffffffff {
		t.Fatalf("missing explicit worn deletion: %x", p)
	}
	for _, tc := range []struct {
		bag   inventory.Bag
		space byte
	}{{inventory.Bag{}, 0}, {inventory.Bag{Worn: []inventory.BagEquipment{{Slot: 12, Template: 999}}}, 3}} {
		got, err := appendEnhancementRemoval([]outboundPacket{ack}, tc.bag, tc.space, 12)
		if err != nil || len(got) != 1 {
			t.Fatal("unexpected delete", got, err)
		}
	}
}

func TestEnhancementJournalRefreshUsesSavedLedger(t *testing.T) {
	state, err := inventory.SaveEquipmentJournal(json.RawMessage(`{}`), inventory.EquipmentJournal{Counts: map[uint32]uint32{123: 1}})
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{role: database.Character{ID: 7, State: state}, items: &inventory.ItemService{Journal: &catalog.EquipmentJournalRules{Maximum: 99}}}
	ack := outboundPacket{"ack", 1, 80, []byte{1}}
	added := []inventory.JournalRegistration{{Template: 123, After: 1}}
	plan := w.appendEnhancementJournal([]outboundPacket{ack}, added, nil, nil)
	if len(plan) != 2 || plan[0].ID != 80 || plan[1].ID != protocol.EquipmentJournalOpcode || len(plan[1].Payload) != 16444 {
		t.Fatalf("refresh plan: %+v", plan)
	}
	for _, skipped := range [][]inventory.JournalSkip{nil, {{Template: 123, Reason: "cap reached"}}} {
		if got := w.appendEnhancementJournal([]outboundPacket{ack}, nil, skipped, nil); len(got) != 1 {
			t.Fatal("unchanged ledger refreshed")
		}
	}
	w.role.State = json.RawMessage(`{"equipment_journal":"bad"}`)
	if got := w.appendEnhancementJournal([]outboundPacket{ack}, added, nil, nil); len(got) != 1 || got[0].ID != 80 {
		t.Fatal("refresh error discarded committed ACK")
	}
}
