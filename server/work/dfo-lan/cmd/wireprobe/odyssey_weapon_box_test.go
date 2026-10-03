package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestCapturedOdysseyWeaponSelection(t *testing.T) {
	role, wear := odysseyRewardFixture(t)
	c, e := loadOdysseyWeaponChoices("../../configs/odyssey-weapon-box-release.json")
	if e != nil {
		t.Fatal(e)
	}
	p, _ := hex.DecodeString("4200010000000000d8c2050600000000")
	r, e := protocol.DecodeWeaponBoxSelection(p)
	if e != nil || r.Slot != 66 || r.Template != 101040856 || !c.allows(r) {
		t.Fatal(r, e)
	}
	b, _ := inventory.ReadBag(role.State)
	b.Items = append(b.Items, inventory.BagItem{Slot: 66, Template: 10417789, Amount: 1})
	role.State, _ = inventory.SaveBag(role.State, b)
	raw, _, e := applyOdysseyWeaponChoice(role, wear, c, r)
	if e != nil {
		t.Fatal(e)
	}
	result, e := inventory.ReadBag(raw)
	if e != nil || len(result.Items) != 0 || len(result.Equipment) != 1 || result.Equipment[0].Template != r.Template {
		t.Fatal(result, e)
	}
	ack := protocol.WeaponBoxSuccess(c.Template, r)
	if len(ack) != 30 || hex.EncodeToString(ack) != "0100007df69e004200000000000100d8c205060100000000000000000000" {
		t.Fatalf("native response mismatch: %x", ack)
	}
	invalid := r
	invalid.Template = 1
	if _, _, e = applyOdysseyWeaponChoice(role, wear, c, invalid); e == nil {
		t.Fatal("non-source selection")
	}
	wear.BagRules.EquipmentSlots = [2]uint16{9, 9}
	b.Equipment = []inventory.BagEquipment{{Slot: 9, Template: r.Template}}
	role.State, _ = inventory.SaveBag(role.State, b)
	if out, _, e := applyOdysseyWeaponChoice(role, wear, c, r); e == nil || out != nil {
		t.Fatal("full bag consumed box")
	}
	for _, bad := range [][]byte{p[:15], append(append([]byte{}, p[:12]...), 1, 0, 0, 0)} {
		if _, e := protocol.DecodeWeaponBoxSelection(bad); e == nil {
			t.Fatal("malformed selection")
		}
	}
	t.Log("captured CMD160 exact source selection; one box->one chosen weapon; full bag and invalid selections retain box")
}

func TestOdysseyWeaponSelectionFollowsSourceBoxTemplate(t *testing.T) {
	role, wear := odysseyRewardFixture(t)
	choices, err := loadOdysseyWeaponChoices("../../configs/odyssey-weapon-box-release.json")
	if err != nil {
		t.Fatal(err)
	}
	choices.Template = 99999
	req := protocol.WeaponBoxSelection{Slot: 66, Category: choices.Categories[0].Category, Template: 101040856}
	bag, _ := inventory.ReadBag(role.State)
	bag.Items = append(bag.Items, inventory.BagItem{Slot: req.Slot, Template: choices.Template, Amount: 1})
	role.State, _ = inventory.SaveBag(role.State, bag)
	if _, _, err := applyOdysseyWeaponChoice(role, wear, choices, req); err != nil {
		t.Fatal(err)
	}
	ack := protocol.WeaponBoxSuccess(choices.Template, req)
	if binary.LittleEndian.Uint32(ack[3:7]) != choices.Template {
		t.Fatalf("box identity does not follow source: %x", ack)
	}
}
