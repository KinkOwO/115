package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

func (b Bag) UseEmblems(c catalog.LootCatalog, eq *EquipmentCatalog, rules *EmblemInlayRules, req protocol.UseEmblemRequest, now int64) (Bag, error) {
	fail := func(err error) (Bag, error) { return b, err }
	if err := req.Validate(); err != nil {
		return fail(err)
	}
	if eq == nil || rules == nil || rules.Source != c.Source.Checksum || eq.Source.Checksum != c.Source.Checksum {
		return fail(fmt.Errorf("avatar emblem source unavailable or mismatched"))
	}
	target := -1
	for i, row := range b.Special[1] {
		if row.Slot == req.AvatarSlot {
			if target >= 0 {
				return fail(fmt.Errorf("duplicate avatar emblem target slot"))
			}
			target = i
		}
	}
	if target < 0 || b.Special[1][target].Template != req.Template {
		return fail(fmt.Errorf("missing or stale avatar emblem target"))
	}
	item := b.Special[1][target]
	if err := item.ValidateRecord(); err != nil {
		return fail(err)
	}
	d, err := eq.definitionResolved(item.Template, 0)
	if err != nil {
		return fail(err)
	}
	kind := d.Fields["[equipment type]"]
	if !d.IsAvatar() || len(kind) != 2 || kind[0].Text != "[skin avatar]" {
		return fail(fmt.Errorf("emblem insertion requires a skin avatar"))
	}
	selectCells := d.Fields["[avatar type select]"]
	n := len(selectCells)
	if n < 3 || selectCells[n-3].Type != 0 || selectCells[n-3].Value != 2 || selectCells[n-2].Text != "[M socket]" || selectCells[n-1].Text != "[M socket]" {
		return fail(fmt.Errorf("unsupported source skin avatar socket layout"))
	}
	if len(item.AvatarOptions) == 0 || len(item.AvatarOptions) > 30 || len(item.AvatarOptions)%6 != 0 || (len(item.AvatarSockets) != 0 && len(item.AvatarSockets) != 4) {
		return fail(fmt.Errorf("unsupported or unopened avatar emblem extension"))
	}
	options := make([]byte, 30)
	copy(options, item.AvatarOptions)
	used := map[uint16]uint32{}
	for _, in := range req.Inputs {
		offset := int(in.Socket) * 6
		socket := binary.LittleEndian.Uint16(options[offset:])
		if in.Socket >= 2 || socket != avatarMultiSocket {
			return fail(fmt.Errorf("avatar emblem socket is not open"))
		}
		mask := rules.Masks[in.Template]
		definition, ok := c.Items[in.Template]
		if !ok || definition.Kind != "stackable" || definition.StackableType != "[avatar emblem]" || mask&socket == 0 {
			return fail(fmt.Errorf("avatar emblem does not match socket color"))
		}
		if binary.LittleEndian.Uint32(options[offset+2:]) == in.Template {
			return fail(fmt.Errorf("avatar socket already contains this emblem"))
		}
		found := -1
		for i, row := range b.Items {
			if row.Slot == in.Slot {
				if found >= 0 {
					return fail(fmt.Errorf("duplicate avatar emblem inventory slot"))
				}
				found = i
			}
		}
		used[in.Slot]++
		if found < 0 || b.Items[found].Template != in.Template || b.Items[found].Amount < used[in.Slot] || protocol.StoredItemExpired(b.Items[found].ExpireTime, now) {
			return fail(fmt.Errorf("missing, stale, expired or insufficient avatar emblem stack"))
		}
		binary.LittleEndian.PutUint32(options[offset+2:], in.Template)
	}
	// All validation precedes any mutation. Clone both changed slices and keep
	// every other instance field, extension, currency and inventory space.
	next := b
	next.Items = make([]BagItem, 0, len(b.Items))
	for _, row := range b.Items {
		count := used[row.Slot]
		if count == 0 {
			next.Items = append(next.Items, row)
			continue
		}
		row.Amount -= count
		if row.Amount > 0 {
			next.Items = append(next.Items, row)
		}
	}
	next.Special = make(map[byte][]BagEquipment, len(b.Special))
	for space, rows := range b.Special {
		next.Special[space] = append([]BagEquipment(nil), rows...)
	}
	item.AvatarOptions = options
	next.Special[1][target] = item
	return next, nil
}

// SaveEmblemInlay writes only the three inventory fields owned by this
// operation. Other inventory fields (including newer avatar capacity fields)
// and other special spaces must survive a mixed-version source integration.
func SaveEmblemInlay(state json.RawMessage, b Bag) (json.RawMessage, error) {
	updated, err := SaveBag(state, b)
	if err != nil {
		return nil, err
	}
	var original, next map[string]json.RawMessage
	if err := json.Unmarshal(state, &original); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(updated, &next); err != nil {
		return nil, err
	}
	var oldBag, newBag map[string]json.RawMessage
	if err := json.Unmarshal(original["inventory"], &oldBag); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(next["inventory"], &newBag); err != nil {
		return nil, err
	}
	var oldSpaces, newSpaces map[string]json.RawMessage
	if err := json.Unmarshal(oldBag["special_equipment"], &oldSpaces); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(newBag["special_equipment"], &newSpaces); err != nil {
		return nil, err
	}
	if oldBag == nil || oldSpaces == nil || len(newSpaces["1"]) == 0 {
		return nil, fmt.Errorf("missing saved avatar emblem inventory")
	}
	oldBag["items"] = newBag["items"]
	oldBag["emblem_inlay_seq"] = newBag["emblem_inlay_seq"]
	oldSpaces["1"] = newSpaces["1"]
	oldBag["special_equipment"], err = json.Marshal(oldSpaces)
	if err != nil {
		return nil, err
	}
	original["inventory"], err = json.Marshal(oldBag)
	if err != nil {
		return nil, err
	}
	return json.Marshal(original)
}
