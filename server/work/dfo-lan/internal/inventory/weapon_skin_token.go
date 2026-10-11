package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"fmt"
)

// Row preserves the native per-instance weapon shape. N13/N14 raw181 +0x0e
// feeds 14576D8B0 temp+8, then 14592A8F0 -> 145013D70 instance+0x4a4.
// 1444EBFE0 reads the same value through info+0x44c (instance+0x58).
func (i BagItem) Row() [protocol.CurrentItemRecordSize]byte {
	r := protocol.OrdinaryItem(i.Slot, i.Template, i.Amount, i.ExpireTime)
	binary.LittleEndian.PutUint32(r[14:], i.WeaponShape)
	return r
}

// GenerateWeaponShape creates an unconsumed shape item. Owning the same skin
// does not prevent making another item for a different character.
func (b Bag) GenerateWeaponShape(slot uint16, equipment *EquipmentCatalog, rules BagRules, recipe catalog.WeaponSkinRecipe, actor ReplicationActor) (Bag, WeaponReplication, error) {
	if equipment == nil || recipe.Product == 0 || recipe.Material == 0 || recipe.Count == 0 {
		return b, WeaponReplication{}, fmt.Errorf("weapon shape source rules unavailable")
	}
	index := -1
	for i, gear := range b.Equipment {
		if gear.Slot == slot {
			index = i
			break
		}
	}
	if index < 0 {
		return b, WeaponReplication{}, fmt.Errorf("weapon shape source slot is empty")
	}
	skin := b.Equipment[index].Template
	def, e := equipment.definitionResolved(skin, 0)
	if e != nil {
		return b, WeaponReplication{}, e
	}
	kind := def.Fields["[equipment type]"]
	if len(kind) == 0 || kind[0].Text != weaponKind {
		return b, WeaponReplication{}, fmt.Errorf("only weapons can be replicated")
	}
	if recipe.Attach != "[free]" && recipe.Attach != "[account]" {
		if e := equipment.WeaponSkinUsable(skin, actor); e != nil {
			return b, WeaponReplication{}, e
		}
	}
	// Allocate the product before charging: a full consumable bag cannot eat a
	// weapon. Items are separate instances even if their product template matches.
	home := stackableSlotRange(rules, recipe.StackableType)
	used := map[uint16]bool{}
	for _, i := range b.Items {
		used[i.Slot] = true
	}
	for _, i := range b.Equipment {
		used[i.Slot] = true
	}
	var productSlot uint16
	for n := uint32(home[0]); n <= uint32(home[1]); n++ {
		if s := uint16(n); b.OrdinarySlotUnlocked(s) && !used[s] {
			productSlot = s
			break
		}
	}
	if productSlot == 0 {
		return b, WeaponReplication{}, fmt.Errorf("weapon shape consumable bag is full")
	}
	var available uint64
	for _, i := range b.Items {
		if i.Template == recipe.Material && i.WeaponShape == 0 {
			available += uint64(i.Amount)
		}
	}
	if available < uint64(recipe.Count) {
		return b, WeaponReplication{}, fmt.Errorf("selected weapon shape material is missing")
	}
	out := b
	out.Equipment = append([]BagEquipment(nil), b.Equipment...)
	out.Equipment = append(out.Equipment[:index], out.Equipment[index+1:]...)
	out.Items = make([]BagItem, 0, len(b.Items)+1)
	cost := WeaponReplication{Skin: skin, Slot: slot, Mold: recipe.Material, Product: recipe.Product, ProductSlot: productSlot}
	remaining := recipe.Count
	for _, i := range b.Items {
		if i.Template == recipe.Material && i.WeaponShape == 0 && remaining > 0 {
			n := min(remaining, i.Amount)
			remaining -= n
			i.Amount -= n
			cost.MoldSlot, cost.MoldAmount = i.Slot, i.Amount
		}
		if i.Amount > 0 {
			out.Items = append(out.Items, i)
		}
	}
	out.Items = append(out.Items, BagItem{Slot: productSlot, Template: recipe.Product, Amount: 1, WeaponShape: skin})
	out.WeaponSkinTokenSeq++
	return out, cost, nil
}

func (b Bag) RegisterWeaponShape(slot uint16, template uint32, equipment *EquipmentCatalog, actor ReplicationActor) (Bag, uint32, error) {
	idx := -1
	for i, item := range b.Items {
		if item.Slot == slot && item.Template == template {
			idx = i
			break
		}
	}
	if idx < 0 || b.Items[idx].WeaponShape == 0 {
		return b, 0, fmt.Errorf("weapon shape item has no weapon instance")
	}
	skin := b.Items[idx].WeaponShape
	if equipment == nil {
		return b, 0, fmt.Errorf("weapon shape equipment catalog unavailable")
	}
	def, e := equipment.definitionResolved(skin, 0)
	if e != nil {
		return b, 0, e
	}
	kind := def.Fields["[equipment type]"]
	if len(kind) == 0 || kind[0].Text != weaponKind {
		return b, 0, fmt.Errorf("weapon shape instance is not a weapon")
	}
	if e := equipment.WeaponSkinUsable(skin, actor); e != nil {
		return b, 0, e
	}
	for _, owned := range b.WeaponSkinStorage() {
		if owned == skin {
			return b, 0, fmt.Errorf("weapon shape already registered; item retained")
		}
	}
	out := b
	out.Items = append([]BagItem(nil), b.Items...)
	if out.Items[idx].Amount > 1 {
		out.Items[idx].Amount--
	} else {
		out.Items = append(out.Items[:idx], out.Items[idx+1:]...)
	}
	out.WeaponSkins = append(b.WeaponSkinStorage(), skin)
	out.WeaponSkinTokenSeq++
	return out, skin, nil
}
