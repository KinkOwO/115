package inventory

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"testing"
)

func portableTestRule(t *testing.T, mode byte) reinforcementTicket {
	t.Helper()
	action := "[portable upgrade]"
	if mode == 1 {
		action = "[portable amplify]"
	}
	s := catalog.ItemScript{Exact: true, Item: catalog.ItemIndexEntry{ID: 700, Kind: "stackable", Path: "stackable/portable-fixture.stk"}, Cells: []pvf.Token{
		{Type: 3, Text: "[action type]"}, {Type: 6, Text: action}, {Type: 0, Value: 0}, {Type: 0, Value: 9},
		{Type: 3, Text: "[check usable itemlevel]"}, {Type: 0, Value: 100}, {Type: 0, Value: 115},
		{Type: 3, Text: "[usable job]"}, {Type: 6, Text: "[all]"},
		{Type: 3, Text: "[action usable place]"}, {Type: 6, Text: "[village]"}, {Type: 6, Text: "[seria room]"},
	}}
	b := &EnhancementItemImport{c: &EnhancementCatalog{}}
	if err := b.Consume(s); err != nil {
		t.Fatal(err)
	}
	return b.c.Portable[700]
}

func TestPortableEnhancementConsumesToolWithoutNormalMaterialsOrGold(t *testing.T) {
	loadGoldRulesForTest(t)
	loadAmplifyUpgradeRulesForTest(t)
	previous, oldGold, oldRoll := portableEnhancements, goldRules, amplifyUpgradeRandomInt
	t.Cleanup(func() { portableEnhancements, goldRules, amplifyUpgradeRandomInt = previous, oldGold, oldRoll })
	for _, mode := range []byte{0, 1} {
		for _, success := range []bool{true, false} {
			for _, space := range []byte{0, 3} {
				rule := portableTestRule(t, mode)
				portableEnhancements = map[uint32]reinforcementTicket{700: rule}
				goldCopy := *oldGold
				rate, roll := 0, 99
				if success {
					rate, roll = 100, 0
				}
				goldCopy.Failure.PlayerMeasured.SuccessRatePercentByLevel = map[string]int{"default": rate}
				goldRules = &goldCopy
				amplifyUpgradeRandomInt = func(int) (int, error) { return roll, nil }
				d := EquipmentDefinition{ID: 701, Fields: map[string][]pvf.Token{
					"[equipment type]": {{Type: 6, Text: "[weapon]"}}, "[minimum level]": {{Value: 115}}, "[rarity]": {{Value: 4}},
				}}
				s := &WearService{Catalog: &EquipmentCatalog{index: map[uint32]EquipmentDefinition{701: d}}}
				row := EquipmentRow(BagEquipment{Slot: 12, Template: 701})
				row[10], row[amplifyTypeOffset] = 0x60|7, 1
				gear := BagEquipment{Slot: 12, Template: 701, Record: row[:]}
				bag := Bag{Version: "ordinary-bag-v1", Gold: 0, Items: []BagItem{{Slot: 78, Template: 700, Amount: 2}}}
				if space == 0 {
					bag.Equipment = []BagEquipment{gear}
				} else {
					bag.Worn = []BagEquipment{gear}
				}
				state, err := SaveBag(json.RawMessage(`{"level":115,"future":{"keep":42}}`), bag)
				if err != nil {
					t.Fatal(err)
				}
				original := append([]byte(nil), state...)
				r := protocol.ReinforcementRequest{Mode: mode, EquipmentSpace: space, EquipmentSlot: 12, EquipmentTemplate: 701, TicketSlot: 78, MaterialSlot: 0xffff, ProtectionSlot: 0xffff}
				var next json.RawMessage
				var spent, gold uint32
				var ack []byte
				if mode == 0 {
					var out GoldReinforcementReceipt
					next, _, out, err = s.ApplyGoldReinforcement(Role{State: state}, nil, "portable", r)
					spent, gold = out.MaterialSpent, out.GoldSpent
					if err == nil {
						ack, err = protocol.ReinforcementGoldReply(r, out.MaterialRemaining, out.Old, out.Level, out.Result)
					}
					if out.Mode != "portable_upgrade" {
						t.Fatal(out)
					}
				} else {
					var out AmplifyUpgradeReceipt
					next, out, err = s.ApplyAmplifyUpgrade(Role{State: state}, r)
					spent, gold = out.MaterialSpent, out.GoldSpent
					if err == nil {
						ack, err = protocol.AmplifyUpgradeReply(r, out.MaterialRemaining, out.LevelBefore, out.LevelAfter, out.Result)
					}
					if !out.Portable {
						t.Fatal(out)
					}
				}
				if err != nil {
					t.Fatalf("mode=%d success=%t space=%d: %v", mode, success, space, err)
				}
				after, err := ReadBag(next)
				if err != nil {
					t.Fatal(err)
				}
				items := after.Equipment
				if space == 3 {
					items = after.Worn
				}
				got := EquipmentRow(items[0])
				if spent != 1 || gold != 0 || after.Gold != 0 || len(after.Items) != 1 || after.Items[0].Amount != 1 || got[10]>>5 != 3 || got[amplifyTypeOffset] != 1 || len(ack) != 35 {
					t.Fatalf("incorrect portable result: mode=%d spent=%d gold=%d bag=%+v ack=%x", mode, spent, gold, after, ack)
				}
				if success && got[10]&31 != 8 {
					t.Fatalf("portable action must advance exactly one level: %d", got[10]&31)
				}
				if !bytes.Equal(state, original) || !bytes.Contains(next, []byte(`"future"`)) {
					t.Fatal("input or unknown state changed")
				}
			}
		}
	}
}

func TestPortableEnhancementSourceLimitsRefuseBeforeConsumption(t *testing.T) {
	loadGoldRulesForTest(t)
	previous := portableEnhancements
	t.Cleanup(func() { portableEnhancements = previous })
	for _, scenario := range []string{"action-level", "equipment-level", "equipment-list", "extra-material", "event-context", "expired", "wrong-mode", "multiple"} {
		t.Run(scenario, func(t *testing.T) {
			rule := portableTestRule(t, 0)
			s := &WearService{Catalog: &EquipmentCatalog{index: map[uint32]EquipmentDefinition{701: {ID: 701, Fields: map[string][]pvf.Token{
				"[equipment type]": {{Type: 6, Text: "[weapon]"}}, "[minimum level]": {{Value: 115}}, "[rarity]": {{Value: 4}},
			}}}}}
			item := BagItem{Slot: 78, Template: 700, Amount: 1}
			r := protocol.ReinforcementRequest{EquipmentSlot: 12, EquipmentTemplate: 701, TicketSlot: 78, MaterialSlot: 0xffff, ProtectionSlot: 0xffff}
			switch scenario {
			case "action-level":
				rule.Fields["[action type]"][1].Value = 8
			case "equipment-level":
				rule.Fields["[check usable itemlevel]"][1].Value = 110
			case "equipment-list":
				rule.Fields["[reinforcement usable item list]"] = []pvf.Token{{Value: 702}}
			case "extra-material":
				rule.Fields["[need material]"] = []pvf.Token{{Value: 800}, {Value: 1}}
			case "event-context":
				rule.Fields["[impossible contents]"] = []pvf.Token{{Type: 6, Text: "gift"}}
			case "expired":
				item.ExpireTime = 1667865600
			case "wrong-mode":
				rule.Fields["[action type]"][0].Text = "[portable amplify]"
			case "multiple":
				r.Multiple = 1
			}
			portableEnhancements = map[uint32]reinforcementTicket{700: rule}
			row := EquipmentRow(BagEquipment{Slot: 12, Template: 701})
			row[10] = 7
			state, err := SaveBag(json.RawMessage(`{"level":115}`), Bag{Version: "ordinary-bag-v1", Items: []BagItem{item}, Equipment: []BagEquipment{{Slot: 12, Template: 701, Record: row[:]}}})
			if err != nil {
				t.Fatal(err)
			}
			before := append([]byte(nil), state...)
			next, _, _, err := s.ApplyGoldReinforcement(Role{State: state}, nil, "invalid", r)
			if err == nil || next != nil || !bytes.Equal(state, before) {
				t.Fatal("unsupported tool consumed inventory", err)
			}
		})
	}
}

func TestPortableEnhancementImportRejectsMalformedAction(t *testing.T) {
	c := &EnhancementCatalog{}
	for _, v := range [][]pvf.Token{
		{{Type: 6, Text: "[portable upgrade]"}},
		{{Type: 6, Text: "[portable amplify]"}, {Value: 10}, {Value: 9}},
		{{Type: 6, Text: "[portable amplify]"}, {Value: 0}, {Value: 32}},
	} {
		if err := c.importPortable(700, "stackable/tool.stk", map[string][]pvf.Token{"[action type]": v}); err == nil {
			t.Fatal("malformed portable action accepted")
		}
	}
}
