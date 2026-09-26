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
