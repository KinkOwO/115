//go:build sqlite

package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"testing"
)

func sourceAuraFixture(t *testing.T, template uint32, action string) *worldSession {
	t.Helper()
	w := skinSlotFixture(t)
	w.items.Catalog.Items[template] = catalog.LootItem{ID: template, Kind: "stackable", StackableType: "[etc]",
		Script: catalog.ScriptRecord{Path: "stackable/source-test.stk", Cells: []pvf.Token{
			{Type: 3, Text: "[action type]"}, {Type: 6, Text: action}, {Type: 3, Text: "[/action type]"},
		}}}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	bag.Items = []inventory.BagItem{{Slot: 65, Template: 1111, Amount: 3}, {Slot: 66, Template: template, Amount: 2}}
	state, err := inventory.SaveBag(w.role.State, bag)
	if err != nil {
		t.Fatal(err)
	}
	saved, _, err := w.store.CommitCharacterEvent(context.Background(), w.role.AccountID, w.role.ID, w.role.ConfigVersion, "source-fixture", "source-fixture",
		func(_ character.Character) (json.RawMessage, json.RawMessage, error) {
			return state, json.RawMessage(`{}`), nil
		})
	if err != nil {
		t.Fatal(err)
	}
	saved.WireID = w.role.WireID
	w.role = saved
	return w
}

func TestSourceAuraTicketSQLiteActualRequestsAndReplay(t *testing.T) {
	// 590714050 is the actual owned item. A different source ID verifies that
	// behavior follows the action rather than another fixed template list.
	for _, id := range []uint32{590714050, 987654321} {
		for _, entry := range []string{"action", "padlock"} {
			t.Run(fmt.Sprintf("%s-%d", entry, id), func(t *testing.T) {
				w := sourceAuraFixture(t, id, "[open aura skin slot]")
				var plan []outboundPacket
				var err error
				action := make([]byte, 64)
				binary.LittleEndian.PutUint16(action, 66)
				binary.LittleEndian.PutUint32(action[7:], 101)
				// Actual click request carries 0xffff when the native fixed
				// license is absent. It must find a source-defined owned ticket.
				click := []byte{255, 255, 11, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
				if entry == "action" {
					plan, err = w.stackableAction(action)
				} else {
					plan, err = w.openSkinSlot(click)
				}
				if err != nil {
					t.Fatal(err)
				}
				assertAuraSlot(t, w, plan)
				bag, _ := inventory.ReadBag(w.role.State)
				if len(bag.Items) != 2 || bag.Items[0].Amount != 3 || bag.Items[1].Template != id || bag.Items[1].Amount != 1 {
					t.Fatal("wrong decrement", bag.Items)
				}
				before := string(w.role.State)
				if entry == "action" {
					_, err = w.stackableAction(action)
				} else {
					_, err = w.openSkinSlot(click)
				}
				if err != nil || string(w.role.State) != before {
					t.Fatal("replay charged again", err)
				}
				roles, err := w.store.Characters(context.Background(), w.role.AccountID)
				if err != nil || len(roles) != 1 {
					t.Fatal("reload", err)
				}
				reloaded, _ := inventory.ReadBag(roles[0].State)
				if reloaded.ExpandEquipFlags&inventory.ExpandAuraSkin == 0 || reloaded.Items[1].Amount != 1 {
					t.Fatal("unlock not persisted")
				}
			})
		}
	}
}

func TestSourceAuraTicketSQLiteRejectsOtherActions(t *testing.T) {
	for _, action := range []string{"[open creature skin slot]", "[not a skin ticket]"} {
		w := sourceAuraFixture(t, 590714050, action)
		before := string(w.role.State)
		if _, err := w.openSkinSlot([]byte{255, 255, 11, 0}); err == nil || string(w.role.State) != before {
			t.Fatal("wrong source action accepted", action, err)
		}
	}
}
