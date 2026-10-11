package inventory

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"testing"
)

func TestEnhancementDestructionRegistersJournal(t *testing.T) {
	loadGoldRulesForTest(t)
	loadAmplifyUpgradeRulesForTest(t)
	oldGold, oldRoll := goldRules, amplifyUpgradeRandomInt
	goldCopy := *goldRules
	goldCopy.Failure.PlayerMeasured.SuccessRatePercentByLevel = map[string]int{"default": 0}
	goldCopy.Failure.DestroyEnabled = true
	goldRules = &goldCopy
	amplifyUpgradeRandomInt = func(int) (int, error) { return 99, nil }
	t.Cleanup(func() { goldRules, amplifyUpgradeRandomInt = oldGold, oldRoll })
	const template = 900000001
	for _, mode := range []byte{0, 1} {
		for _, space := range []byte{0, 3} {
			for _, scenario := range []string{"destroy", "protected", "cap", "ineligible", "corrupt"} {
				t.Run(string(rune('0'+mode))+"/"+string(rune('0'+space))+"/"+scenario, func(t *testing.T) {
					level := int32(115)
					if scenario == "ineligible" {
						level = 110
					}
					service := &WearService{Journal: &catalog.EquipmentJournalRules{Maximum: 2}, Catalog: &EquipmentCatalog{index: map[uint32]EquipmentDefinition{template: {ID: template, Fields: map[string][]pvf.Token{
						"[equipment type]": {{Type: 6, Text: "[weapon]"}}, "[minimum level]": {{Value: level}}, "[rarity]": {{Value: 4}},
					}}}}}
					row := EquipmentRow(BagEquipment{Slot: 12, Template: template})
					row[10] = 12
					row[amplifyTypeOffset] = 1
					setAmplifyLevel(row[:], 12)
					gear := BagEquipment{Slot: 12, Template: template, Record: row[:]}
					material := uint32(3037)
					if mode == 1 {
						material = 3242
					}
					bag := Bag{Version: "ordinary-bag-v1", Gold: 1000000000, Items: []BagItem{{Slot: 78, Template: material, Amount: 10000}}}
					if space == 3 {
						bag.Worn = []BagEquipment{gear}
					} else {
						bag.Equipment = []BagEquipment{gear}
					}
					if scenario == "protected" {
						ticket := uint32(50022396)
						if mode == 1 {
							ticket = 50022395
						}
						bag.Items = append(bag.Items, BagItem{Slot: 79, Template: ticket, Amount: 1})
					}
					state, err := SaveBag(json.RawMessage(`{"level":115,"future_field":{"keep":true}}`), bag)
					if err != nil {
						t.Fatal(err)
					}
					if scenario == "cap" {
						state, err = SaveEquipmentJournal(state, EquipmentJournal{Counts: map[uint32]uint32{template: 2}})
						if err != nil {
							t.Fatal(err)
						}
					}
					if scenario == "corrupt" {
						state = append([]byte(`{"equipment_journal":"bad",`), state[1:]...)
					}
					original := append([]byte(nil), state...)
					r := protocol.ReinforcementRequest{Mode: mode, EquipmentSpace: space, EquipmentSlot: 12, EquipmentTemplate: template, TicketSlot: 78, MaterialSlot: 0xffff, ProtectionSlot: 0xffff}
					var next json.RawMessage
					var added []JournalRegistration
					var skipped []JournalSkip
					var destroyed bool
					if mode == 0 {
						var out GoldReinforcementReceipt
						next, _, out, err = service.ApplyGoldReinforcement(Role{State: state}, nil, "break-test", r)
						added, skipped, destroyed = out.JournalAdded, out.JournalSkipped, out.Destroyed
						if destroyed && out.Result != 3 {
							t.Fatal("destroyed reinforcement reported as ordinary failure")
						}
						if err == nil {
							stored, e := ReadGoldReinforcementReceipt(next, "break-test")
							if e != nil || len(stored.JournalAdded) != len(added) {
								t.Fatalf("stored receipt: %+v %v", stored, e)
							}
						}
					} else {
						var out AmplifyUpgradeReceipt
						next, out, err = service.ApplyAmplifyUpgrade(Role{State: state}, r)
						added, skipped, destroyed = out.JournalAdded, out.JournalSkipped, out.Destroyed
					}
					if !bytes.Equal(state, original) {
						t.Fatal("input state mutated")
					}
					if scenario == "corrupt" {
						if err == nil || next != nil {
							t.Fatalf("corrupt ledger must reject atomically: %v", err)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					after, err := ReadBag(next)
					if err != nil {
						t.Fatal(err)
					}
					ledger, err := ReadEquipmentJournal(next)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Contains(next, []byte(`"future_field"`)) {
						t.Fatal("unknown state lost")
					}
					if scenario == "protected" {
						if destroyed || len(after.Equipment)+len(after.Worn) != 1 || len(added) != 0 || ledger.Counts[template] != 0 {
							t.Fatal("protected gear registered or removed")
						}
					} else {
						if !destroyed || len(after.Equipment)+len(after.Worn) != 0 {
							t.Fatal("destruction missing")
						}
						if scenario == "destroy" {
							if len(added) != 1 || ledger.Counts[template] != 1 {
								t.Fatalf("registration missing: %+v", ledger)
							}
						} else if len(added) != 0 || len(skipped) != 1 {
							t.Fatalf("expected cap/eligibility skip: %v %v", added, skipped)
						}
						if scenario == "cap" && ledger.Counts[template] != 2 {
							t.Fatal("cap changed")
						}
					}
				})
			}
		}
	}
}
