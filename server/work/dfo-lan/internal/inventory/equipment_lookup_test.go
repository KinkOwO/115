package inventory

import (
	"bytes"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"testing"
)

func TestEquipmentUpgradesPreserveContainerSelection(t *testing.T) {
	loadRefineRulesForTest(t)
	loadAmplifyUpgradeRulesForTest(t)
	oldTickets, oldRoll := amplifyTickets, amplifyUpgradeRandomInt
	amplifyTickets = map[uint32]reinforcementTicket{50024309: {Fields: map[string][]pvf.Token{
		amplifyTicketSection: {{Value: 7}, {Value: 100}, {Type: 6, Text: "fixed"}, {Value: -1}},
	}}}
	amplifyUpgradeRandomInt = func(int) (int, error) { return 0, nil }
	t.Cleanup(func() { amplifyTickets, amplifyUpgradeRandomInt = oldTickets, oldRoll })
	const template = 900000001
	s := &WearService{Catalog: &EquipmentCatalog{index: map[uint32]EquipmentDefinition{template: {
		ID: template, Fields: map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: "[weapon]"}},
			"[minimum level]":  {{Value: 100}},
			"[rarity]":         {{Value: 3}},
		},
	}}}}
	operations := []struct {
		name  string
		apply func(storage.Character, byte, uint32) (json.RawMessage, byte, error)
	}{
		{"refine", func(role storage.Character, space byte, target uint32) (json.RawMessage, byte, error) {
			next, out, err := s.applyRefine(role, protocol.RefineRequest{EquipmentSpace: space, EquipmentSlot: 12, EquipmentTemplate: target, MaterialSlot: 79})
			return next, out.EquipmentSpace, err
		}},
		{"amplify", func(role storage.Character, space byte, target uint32) (json.RawMessage, byte, error) {
			next, out, err := s.applyAmplifyUpgrade(role, protocol.ReinforcementRequest{EquipmentSpace: space, EquipmentSlot: 12, EquipmentTemplate: target, TicketSlot: 78})
			return next, out.EquipmentSpace, err
		}},
		{"ticket", func(role storage.Character, space byte, target uint32) (json.RawMessage, byte, error) {
			next, out, err := s.applyAmplifyTicket(role, protocol.ReinforcementRequest{EquipmentSpace: space, EquipmentSlot: 12, EquipmentTemplate: target, TicketSlot: 77})
			return next, out.EquipmentSpace, err
		}},
		{"grimoire", func(role storage.Character, _ byte, target uint32) (json.RawMessage, byte, error) {
			next, out, err := s.applyAmplifyGrimoire(role, protocol.AmplifyOptionRequest{EquipmentSlot: 12, EquipmentTemplate: target, BookSlot: 77, BookTemplate: 50024309, Type: 3}, 5, false, false)
			return next, out.EquipmentSpace, err
		}},
	}
	for _, operation := range operations {
		for _, location := range []struct {
			name            string
			requested, want byte
			bag, worn       bool
		}{
			{"bag", 0, 0, true, false},
			{"worn", 3, 3, false, true},
			{"fallback-to-worn", 0, 3, false, true},
			{"fallback-to-bag", 3, 0, true, false},
			{"prefer-bag", 0, 0, true, true},
			{"prefer-worn", 3, 3, true, true},
		} {
			t.Run(operation.name+"/"+location.name, func(t *testing.T) {
				row := EquipmentRow(BagEquipment{Slot: 12, Template: template})
				row[amplifyTypeOffset] = 1
				gear := BagEquipment{Slot: 12, Template: template, Record: row[:]}
				bag := Bag{Version: "ordinary-bag-v1", Items: []BagItem{
					{Slot: 77, Template: 50024309, Amount: 1}, {Slot: 78, Template: 3242, Amount: 1}, {Slot: 79, Template: 3326, Amount: 7},
				}}
				if location.bag {
					bag.Equipment = []BagEquipment{gear}
				}
				if location.worn {
					bag.Worn = []BagEquipment{gear}
				}
				state, err := SaveBag(json.RawMessage(`{"level":100,"future_field":{"keep":true}}`), bag)
				if err != nil {
					t.Fatal(err)
				}
				role := storage.Character{State: state}
				if next, _, err := operation.apply(role, location.requested, template+1); err == nil || next != nil {
					t.Fatalf("mismatched template accepted: err=%v", err)
				}
				next, space, err := operation.apply(role, location.requested, template)
				want := location.want
				if operation.name == "grimoire" && location.bag {
					want = 0 // CMD205 always prefers the bag.
				}
				if err != nil || space != want {
					t.Fatalf("space=%d want=%d err=%v", space, want, err)
				}
				after, err := ReadBag(next)
				if err != nil {
					t.Fatal(err)
				}
				for sp, items := range map[byte][]BagEquipment{0: after.Equipment, 3: after.Worn} {
					for _, item := range items {
						if changed := !bytes.Equal(item.Record, row[:]); changed != (sp == want) {
							t.Fatalf("container %d changed=%v; only container %d should change", sp, changed, want)
						}
					}
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(next, &fields); err != nil || string(fields["future_field"]) != `{"keep":true}` {
					t.Fatalf("unrelated save field changed: %s (%v)", next, err)
				}
			})
		}
	}
}
