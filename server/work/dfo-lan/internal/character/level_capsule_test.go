package character

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func capsuleTestScript(target byte) catalog.ScriptRecord {
	tag := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	txt := func(s string) pvf.Token { return pvf.Token{Type: 6, Text: s} }
	num := func(n int32) pvf.Token { return pvf.Token{Type: 0, Value: n} }
	return catalog.ScriptRecord{Path: "stackable/fixture-level-capsule.stk", SHA256: strings.Repeat("a", 64), Cells: []pvf.Token{
		tag("[usable job]"), txt("[all]"), tag("[/usable job]"),
		tag("[action type]"), txt("[jumping lv]"), tag("[/action type]"),
		tag("[maximum level]"), num(int32(target) - 1),
		tag("[increase status type]"), txt("[levelUpByTable]"), num(1), num(int32(target) - 1), num(int32(target)), tag("[/increase status type]"),
		tag("[action usable place]"), txt("[seria room]"), tag("[/action usable place]"),
		tag("[impossible contents]"), txt("ban redeemitem"), txt("hardcore mode"), tag("[/impossible contents]"),
	}}
}

func TestCapsuleReadsSourceTargetAndPreservesOtherState(t *testing.T) {
	s := advancementProgressionService(t)
	role := ordinaryAdvancedRole(t, s, 0, 1)
	bag := inventory.Bag{Version: "ordinary-bag-v1", Gold: 321, Coin: 7, Items: []inventory.BagItem{{Slot: 66, Template: 42, Amount: 2}, {Slot: 67, Template: 43, Amount: 5}}}
	var err error
	role.State, err = inventory.SaveBag(role.State, bag)
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), role.State...)
	stateBefore := decodeState(t, role.State)
	for _, target := range []byte{18, 19} {
		c := catalog.LootCatalog{Source: s.Catalog.Source, Items: map[uint32]catalog.LootItem{42: {ID: 42, Kind: "stackable", Script: capsuleTestScript(target)}}}
		got, receipt, err := s.ApplyLevelCapsule(role, c, 66, true, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		state := decodeState(t, got.State)
		if state.Level != target || state.Experience != s.Catalog.Thresholds[int(target)-2] || receipt.Target != target || receipt.Remaining != 1 {
			t.Fatal(state, receipt)
		}
		if state.Advancement != stateBefore.Advancement || state.Awakening != stateBefore.Awakening || state.AdventureEarnedExperience != stateBefore.AdventureEarnedExperience+receipt.Gain {
			t.Fatal("progression state incorrect")
		}
		var a, b map[string]json.RawMessage
		json.Unmarshal(before, &a)
		json.Unmarshal(got.State, &b)
		if !bytes.Equal(a["future_progression_field"], b["future_progression_field"]) {
			t.Fatal("unknown saved field lost")
		}
		after, err := inventory.ReadBag(got.State)
		if err != nil || after.Gold != bag.Gold || after.Coin != bag.Coin || len(after.Items) != 2 || after.Items[0].Amount != 1 || after.Items[1] != bag.Items[1] {
			t.Fatal("unrelated bag state changed", after, err)
		}
		if _, _, err := s.ApplyLevelCapsule(got, c, 66, true, time.Now()); err == nil {
			t.Fatal("already upgraded role spent a second capsule")
		}
	}
	if !bytes.Equal(role.State, before) {
		t.Fatal("pure capsule operation mutated input")
	}
}

func TestCapsuleRefusalNeverConsumes(t *testing.T) {
	s := advancementProgressionService(t)
	base := ordinaryAdvancedRole(t, s, 0, 1)
	base.State, _ = inventory.SaveBag(base.State, inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 66, Template: 42, Amount: 1}}})
	for _, name := range []string{"outside-seria", "empty-slot", "expired", "wrong-action", "unknown-place", "source-mismatch", "unadvanced", "unknown-mode", "above-cap", "overlapping-source"} {
		t.Run(name, func(t *testing.T) {
			role := base
			script := capsuleTestScript(18)
			c := catalog.LootCatalog{Source: s.Catalog.Source, Items: map[uint32]catalog.LootItem{42: {ID: 42, Kind: "stackable", Script: script}}}
			inSeria, slot := true, uint16(66)
			switch name {
			case "outside-seria":
				inSeria = false
			case "empty-slot":
				slot = 67
			case "expired":
				bag, _ := inventory.ReadBag(role.State)
				bag.Items[0].ExpireTime = 1
				role.State, _ = inventory.SaveBag(role.State, bag)
			case "wrong-action":
				script.Cells[4].Text = "[add fatigue]"
			case "unknown-place":
				script.Cells[15].Text = "[dungeon]"
			case "source-mismatch":
				c.Source.Checksum = strings.Repeat("b", 64)
			case "unadvanced":
				var m map[string]json.RawMessage
				json.Unmarshal(role.State, &m)
				m["advancement"] = json.RawMessage("0")
				role.State, _ = json.Marshal(m)
			case "unknown-mode":
				var m map[string]json.RawMessage
				json.Unmarshal(role.State, &m)
				m["creation_mode"] = json.RawMessage("2")
				role.State, _ = json.Marshal(m)
			case "above-cap":
				script = capsuleTestScript(116)
			case "overlapping-source":
				script.Cells = append(script.Cells[:13], append([]pvf.Token{{Type: 0, Value: 1}, {Type: 0, Value: 17}, {Type: 0, Value: 19}}, script.Cells[13:]...)...)
			}
			c.Items[42] = catalog.LootItem{ID: 42, Kind: "stackable", Script: script}
			before := append([]byte(nil), role.State...)
			got, _, err := s.ApplyLevelCapsule(role, c, slot, inSeria, time.Now())
			if err == nil || !bytes.Equal(before, got.State) || !bytes.Equal(before, role.State) {
				t.Fatal("refusal modified state", err)
			}
		})
	}
}
