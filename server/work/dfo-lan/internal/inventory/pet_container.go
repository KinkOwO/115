package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const PetConsumableFirst uint16 = 376
const PetConsumableLast uint16 = 431
const PetGearFirst uint16 = 320
const PetGearLast uint16 = 375

func IsPetGear(kind string) bool {
	switch kind {
	case "[artifact red]", "[artifact blue]", "[artifact green]":
		return true
	}
	return false
}

func IsPetFeed(stackableType string) bool {
	return strings.HasPrefix(normalizeStackableType(stackableType), "[feed]")
}

func (b Bag) AddPetGear(item BagEquipment) (Bag, uint16, error) {
	occupied := make(map[uint16]bool, len(b.Special[7])+len(b.PetItems))
	for _, row := range b.Special[7] {
		occupied[row.Slot] = true
	}
	for _, row := range b.PetItems {
		occupied[row.Slot] = true
	}
	for slot := PetGearFirst; slot <= PetGearLast; slot++ {
		if occupied[slot] {
			continue
		}
		next := b
		next.Special = make(map[byte][]BagEquipment, len(b.Special)+1)
		for space, rows := range b.Special {
			next.Special[space] = rows
		}
		item.Slot = slot
		next.Special[7] = append(append([]BagEquipment(nil), b.Special[7]...), item)
		return next, slot, nil
	}
	return b, 0, fmt.Errorf("pet equipment container is full")
}

// SweepPetGear relocates old rewards without changing any item instance.
func SweepPetGear(b Bag, equipment *EquipmentCatalog) (Bag, bool, error) {
	if equipment == nil {
		return b, false, fmt.Errorf("pet equipment catalog unavailable")
	}
	next := b
	next.Equipment = append([]BagEquipment(nil), b.Equipment...)
	moved := false
	for i := 0; i < len(next.Equipment); {
		item := next.Equipment[i]
		kind, err := equipment.EquipmentKind(item.Template)
		if err != nil || !IsPetGear(kind) {
			i++
			continue
		}
		placed, _, err := next.AddPetGear(item)
		if err != nil {
			return b, false, err
		}
		next = placed
		next.Equipment = append(next.Equipment[:i], next.Equipment[i+1:]...)
		moved = true
	}
	rows := append([]BagEquipment(nil), next.Special[7]...)
	for _, item := range rows {
		if item.Slot >= PetGearFirst && item.Slot <= PetGearLast {
			continue
		}
		kind, err := equipment.EquipmentKind(item.Template)
		if err != nil || !IsPetGear(kind) {
			continue
		}
		// Remove the old slot before finding a destination.
		kept := make([]BagEquipment, 0, len(next.Special[7])-1)
		for _, row := range next.Special[7] {
			if row.Slot != item.Slot {
				kept = append(kept, row)
			}
		}
		next.Special[7] = kept
		placed, _, err := next.AddPetGear(item)
		if err != nil {
			return b, false, err
		}
		next = placed
		moved = true
	}
	return next, moved, nil
}

func IsPetConsumable(stackableType string) bool {
	t := normalizeStackableType(stackableType)
	return strings.HasPrefix(t, "[feed]") || strings.HasPrefix(t, "[creature]")
}

func (b Bag) addPetStack(r BagRules, template, amount, expire, explicit uint32) (Bag, uint16, error) {
	limit := stackLimitFor(r, "", explicit)
	if amount == 0 || amount > limit {
		return b, 0, fmt.Errorf("invalid pet consumable amount")
	}
	original := b
	b.PetItems = append([]BagItem(nil), b.PetItems...)
	occupied := map[uint16]bool{}
	for _, row := range b.Special[7] {
		occupied[row.Slot] = true
	}
	for i, row := range b.PetItems {
		occupied[row.Slot] = true
		if row.Template != template || row.ExpireTime != expire || row.Amount >= limit {
			continue
		}
		added := min(amount, limit-row.Amount)
		b.PetItems[i].Amount += added
		amount -= added
		if amount == 0 {
			return b, row.Slot, nil
		}
	}
	for slot := PetConsumableFirst; slot <= PetConsumableLast; slot++ {
		if occupied[slot] {
			continue
		}
		b.PetItems = append(b.PetItems, BagItem{Slot: slot, Template: template, Amount: amount, ExpireTime: expire})
		return b, slot, nil
	}
	return original, 0, fmt.Errorf("pet consumable container is full")
}

// SweepPetConsumables preserves old character saves while relocating pet
// consumables that were formerly awarded into the ordinary bag.
func SweepPetConsumables(b Bag, c catalog.LootCatalog, r BagRules) (Bag, bool, error) {
	next := b
	next.Items = append([]BagItem(nil), b.Items...)
	moved := false
	for i := 0; i < len(next.Items); {
		row := next.Items[i]
		definition, ok := c.Items[row.Template]
		if !ok || definition.Kind != "stackable" || !IsPetConsumable(definition.StackableType) {
			i++
			continue
		}
		next.Items = append(next.Items[:i], next.Items[i+1:]...)
		placed, _, err := next.addPetStack(r, row.Template, row.Amount, row.ExpireTime, definition.StackLimit)
		if err != nil {
			return b, false, err
		}
		next = placed
		moved = true
	}
	return next, moved, nil
}

// List 7 contains both equipment rows and ordinary pet consumable rows.
func PetContainerBody(b Bag, restore bool) ([]byte, error) {
	body, err := EquipmentPayload(7, b.Special[7], restore)
	if err != nil {
		return nil, err
	}
	items := append([]BagItem(nil), b.PetItems...)
	sort.Slice(items, func(i, j int) bool { return items[i].Slot < items[j].Slot })
	if len(items)+len(b.Special[7]) > 65535 {
		return nil, fmt.Errorf("too many pet container rows")
	}
	binary.LittleEndian.PutUint16(body[1:3], uint16(len(items)+len(b.Special[7])))
	for _, item := range items {
		row := protocol.OrdinaryItem(item.Slot, item.Template, item.Amount, item.ExpireTime)
		body = append(body, row[:]...)
	}
	return body, nil
}

func PetContainerRestorePayload(state json.RawMessage) ([]byte, error) {
	b, err := ReadBag(state)
	if err != nil {
		return nil, err
	}
	return PetContainerBody(b, true)
}
