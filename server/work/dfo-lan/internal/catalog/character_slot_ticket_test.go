package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"testing"
)

func TestCharacterSlotTicketFollowsSourceAndRefusesMissingAction(t *testing.T) {
	cells := []pvf.Token{{Type: 3, Text: "[stackable type]"}, {Type: 6, Text: "[town and dungeon]"}, {Type: 0, Value: 0}, {Type: 3, Text: "[int data]"}, {Type: 0, Value: 2}, {Type: 0, Value: 30}, {Type: 3, Text: "[/int data]"}}
	c := LootCatalog{Items: map[uint32]LootItem{NativeCharacterSlotTicket: {Kind: "stackable", Script: ScriptRecord{Cells: cells}}}}
	rule, found, err := c.CharacterSlotTicket(NativeCharacterSlotTicket)
	if err != nil || !found || rule.Amount != 2 || rule.Limit != 30 {
		t.Fatalf("source projection: %+v, %v", rule, err)
	}
	cells[1].Text = "[etc]"
	if _, found, err = c.CharacterSlotTicket(NativeCharacterSlotTicket); !found || err == nil {
		t.Fatal("unrepaired coupon accepted")
	}
	if _, found, err = c.CharacterSlotTicket(42); found || err != nil {
		t.Fatal("unrelated item treated as a coupon")
	}
}

func TestCharacterSlotTicketCandidatePVF(t *testing.T) {
	path := os.Getenv("DFO_SLOT_TICKET_CANDIDATE_PVF")
	if path == "" {
		t.Skip("explicit candidate archive required")
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: path, MaxBytes: 1 << 30})
	if err != nil {
		t.Fatal(err)
	}
	script, err := ReadScript(a, "stackable/cash/character_slotkit.stk")
	if err != nil {
		t.Fatal(err)
	}
	c := LootCatalog{Source: a.Snapshot(), Items: map[uint32]LootItem{NativeCharacterSlotTicket: {Kind: "stackable", Script: script}}}
	rule, found, err := c.CharacterSlotTicket(NativeCharacterSlotTicket)
	if err != nil || !found || rule.Amount != 1 || rule.Limit != 100 {
		t.Fatalf("actual candidate rule: %+v %v", rule, err)
	}
}
