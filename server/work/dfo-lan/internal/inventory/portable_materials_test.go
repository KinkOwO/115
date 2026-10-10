package inventory

import (
	"bytes"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"testing"
)

func TestPortableSourceMaterialsAreAtomicAndExpiryAware(t *testing.T) {
	loadGoldRulesForTest(t)
	loadAmplifyUpgradeRulesForTest(t)
	previous, oldGold, oldRoll := portableEnhancements, goldRules, amplifyUpgradeRandomInt
	t.Cleanup(func() { portableEnhancements, goldRules, amplifyUpgradeRandomInt = previous, oldGold, oldRoll })
	for _, mode := range []byte{0, 1} {
		for _, success := range []bool{false, true} {
			for _, enough := range []bool{false, true} {
				t.Run(fmt.Sprintf("mode%d/success%v/enough%v", mode, success, enough), func(t *testing.T) {
					rule := portableTestRule(t, mode)
					rule.Fields["[need material]"] = []pvf.Token{{Value: 800}, {Value: 5}}
					portableEnhancements = map[uint32]reinforcementTicket{700: rule}
					goldCopy := *oldGold
					rate, roll := 0, 99
					if success {
						rate, roll = 100, 0
					}
					goldCopy.Failure.PlayerMeasured.SuccessRatePercentByLevel = map[string]int{"default": rate}
					goldRules = &goldCopy
					amplifyUpgradeRandomInt = func(int) (int, error) { return roll, nil }
					def := EquipmentDefinition{ID: 701, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[weapon]"}}, "[minimum level]": {{Value: 115}}, "[rarity]": {{Value: 4}}}}
					service := &WearService{Catalog: &EquipmentCatalog{index: map[uint32]EquipmentDefinition{701: def}}}
					gear := BagEquipment{Slot: 12, Template: 701}
					row := EquipmentRow(gear)
					row[10], row[amplifyTypeOffset] = 7, 1
					gear.Record = row[:]
					fee := uint32(4)
					if !enough {
						fee = 1
					}
					raw, e := SaveBag(json.RawMessage(`{"level":115,"unrelated":{"keep":42}}`), Bag{Version: "ordinary-bag-v1", Equipment: []BagEquipment{gear}, Items: []BagItem{{Slot: 78, Template: 700, Amount: 2}, {Slot: 79, Template: 800, Amount: 100, ExpireTime: 1667865600}, {Slot: 81, Template: 800, Amount: 2}, {Slot: 82, Template: 800, Amount: fee}}})
					if e != nil {
						t.Fatal(e)
					}
					before := append([]byte(nil), raw...)
					request := protocol.ReinforcementRequest{Mode: mode, EquipmentSlot: 12, EquipmentTemplate: 701, TicketSlot: 78, MaterialSlot: 0xffff, ProtectionSlot: 0xffff}
					var next json.RawMessage
					if mode == 0 {
						next, _, _, e = service.ApplyGoldReinforcement(Role{State: raw}, nil, "source-fee", request)
					} else {
						next, _, e = service.ApplyAmplifyUpgrade(Role{State: raw}, request)
					}
					if !bytes.Equal(raw, before) {
						t.Fatal("caller state mutated")
					}
					if !enough {
						if e == nil || next != nil {
							t.Fatal("expired materials paid a fee")
						}
						return
					}
					if e != nil {
						t.Fatal(e)
					}
					bag, e := ReadBag(next)
					if e != nil {
						t.Fatal(e)
					}
					if bag.Gold != 0 || len(bag.Items) != 3 || bag.Items[0].Amount != 1 || bag.Items[1].Amount != 100 || bag.Items[2].Amount != 1 || !bytes.Contains(next, []byte(`"unrelated"`)) {
						t.Fatalf("tool/fee settlement incorrect: %+v", bag)
					}
				})
			}
		}
	}
}
