package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
)

type BagRules struct {
	Model             string               `json:"model"`
	Source            string               `json:"source"`
	Slots             map[string][2]uint16 `json:"slots"`
	MissingStackLimit uint32               `json:"missing_stack_limit"`
	EquipmentSlots    [2]uint16            `json:"equipment_slots,omitempty"`
	// QuickSlots is the belt the client drags consumables onto, below the
	// equipment range. Live capture 20260912T004320 shows CMD19 asking to put
	// the 6003 stack from slot 65 into slot 3; every earlier build refused it
	// because list 0 modelled nothing but the equipment range, and the client
	// then showed its generic "target inventory is full" notice. Slot 0 is a
	// legal belt slot, so this range is not subject to the nonzero check the
	// type ranges use.
	QuickSlots [2]uint16 `json:"quick_slots,omitempty"`
}

// Quick reports whether a slot is part of the quick-use belt.
func (r BagRules) Quick(slot uint16) bool {
	return r.QuickSlots != [2]uint16{} && slot >= r.QuickSlots[0] && slot <= r.QuickSlots[1]
}

func LoadBagRules(p string) (BagRules, error) {
	var r BagRules
	b, e := os.ReadFile(p)
	if e != nil {
		return r, e
	}
	if e = json.Unmarshal(b, &r); e != nil {
		return r, e
	}
	if r.Model != "reference90-bag-v1" || len(r.Source) != 64 || r.MissingStackLimit == 0 {
		return r, fmt.Errorf("invalid bag policy")
	}
	seen := map[uint16]bool{}
	if r.QuickSlots != [2]uint16{} {
		v := r.QuickSlots
		if v[0] > v[1] || v[1] > 65534 {
			return r, fmt.Errorf("invalid quick slot range")
		}
		if r.EquipmentSlots != [2]uint16{} && v[1] >= r.EquipmentSlots[0] {
			return r, fmt.Errorf("quick slots overlap the equipment bag")
		}
		for n := uint32(v[0]); n <= uint32(v[1]); n++ {
			seen[uint16(n)] = true
		}
	}
	if r.EquipmentSlots != [2]uint16{} {
		v := r.EquipmentSlots
		if v[0] == 0 || v[0] > v[1] || v[1] > 65534 {
			return r, fmt.Errorf("invalid equipment bag range")
		}
		for n := uint32(v[0]); n <= uint32(v[1]); n++ {
			seen[uint16(n)] = true
		}
	}
	for _, v := range r.Slots {
		if v[0] == 0 || v[0] > v[1] || v[1] > 65534 {
			return r, fmt.Errorf("invalid bag slot range")
		}
		for n := uint32(v[0]); n <= uint32(v[1]); n++ {
			if seen[uint16(n)] {
				return r, fmt.Errorf("overlapping bag ranges")
			}
			seen[uint16(n)] = true
		}
	}
	return r, nil
}

type BagItem struct {
	Slot             uint16 `json:"slot"`
	Template, Amount uint32
	ExpireTime       uint32 `json:"expire_time,omitempty"`
}
type Bag struct {
	Expansion byte                    `json:"expansion,omitempty"`
	Version   string                  `json:"version"`
	Gold      uint32                  `json:"gold"`
	Coin      uint32                  `json:"coin,omitempty"`
	Items     []BagItem               `json:"items"`
	Equipment []BagEquipment          `json:"equipment,omitempty"`
	Worn      []BagEquipment          `json:"worn,omitempty"`
	Special   map[byte][]BagEquipment `json:"special_equipment,omitempty"`
	// CreatureExperience is keyed by the creature instance key stored in its
	// equipment record. Older saves omit it and start at zero experience.
	CreatureExperience map[uint32]uint32 `json:"creature_experience,omitempty"`
	// ExpandEquipFlags carries the extended equipment-slot unlock bits the
	// armoury draws its padlocks from: support 1<<0, magic stone 1<<1 and
	// earring 1<<4. Quests 649/650/2636 award one bit each and every award
	// must accumulate, so this field is only ever OR-ed - assigning it would
	// relock whatever an earlier quest opened. The client reads the byte from
	// the USERINFO1 unlock slot (protocol entry_addition, native 14563d692)
	// and gates equipment slots 22/23/25 on bits 1/2/16. Absent in saves
	// written before 2026-09-22, which reads back as 0 (nothing unlocked).
	ExpandEquipFlags byte `json:"expand_equip_flags,omitempty"`
}

func ReadBag(state json.RawMessage) (Bag, error) {
	var fields map[string]json.RawMessage
	var b Bag
	if e := json.Unmarshal(state, &fields); e != nil {
		return b, e
	}
	if fields == nil {
		return b, fmt.Errorf("character state is not an object")
	}
	if p, ok := fields["inventory"]; ok {
		if e := json.Unmarshal(p, &b); e != nil {
			return b, e
		}
		if b.Version != "ordinary-bag-v1" {
			return b, fmt.Errorf("unsupported saved inventory")
		}
	} else {
		b.Version = "ordinary-bag-v1"
	}
	filtered := make([]BagItem, 0, len(b.Items))
	if b.Expansion > 2 {
		return b, fmt.Errorf("背包扩展档位超出客户端范围")
	}
	for _, i := range b.Items {
		if i.Template == 1 {
			if uint64(b.Coin)+uint64(i.Amount) <= math.MaxUint32 {
				b.Coin += i.Amount
			}
			continue
		}
		filtered = append(filtered, i)
	}
	b.Items = filtered
	seen := map[uint16]bool{}
	for _, i := range b.Items {
		if i.Slot == 0 || i.Slot == 1 || i.Template == 0 || i.Amount == 0 || seen[i.Slot] {
			return b, fmt.Errorf("invalid saved inventory row")
		}
		seen[i.Slot] = true
	}
	for _, i := range b.Equipment {
		if e := i.ValidateRecord(); e != nil {
			return b, e
		}
		if i.Slot == 0 || i.Template == 0 || seen[i.Slot] {
			return b, fmt.Errorf("invalid saved equipment")
		}
		seen[i.Slot] = true
	}
	wornSeen := map[uint32]bool{}
	for idx, i := range b.Worn {
		if e := i.ValidateRecord(); e != nil {
			return b, e
		}
		if !EquipmentBodySlot(i.Slot) || i.Template == 0 {
			return b, fmt.Errorf("invalid saved worn equipment")
		}
		if i.Group > 1 || (i.Group == 1 && i.Slot > 11) {
			return b, fmt.Errorf("invalid worn equipment group")
		}
		key := (uint32(i.Group) << 16) | uint32(i.Slot)
		if wornSeen[key] {
			return b, fmt.Errorf("invalid saved worn equipment")
		}
		wornSeen[key] = true
		if i.Slot == 26 {
			if hatched, ok := EggHatchOutputs[i.Template]; ok {
				b.Worn[idx].Template = hatched
			}
		}
	}
	for space, rows := range b.Special {
		if space != 1 && space != 7 {
			return b, fmt.Errorf("unsupported equipment inventory")
		}
		seen = map[uint16]bool{}
		for _, i := range rows {
			if e := i.ValidateRecord(); e != nil {
				return b, e
			}
			if i.Template == 0 || seen[i.Slot] {
				return b, fmt.Errorf("invalid special equipment")
			}
			seen[i.Slot] = true
		}
	}
	return b, nil
}
func SaveBag(state json.RawMessage, b Bag) (json.RawMessage, error) {
	if b.Expansion > 2 {
		return nil, fmt.Errorf("背包扩展档位超出客户端范围")
	}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(state, &fields); e != nil {
		return nil, e
	}
	if fields == nil {
		return nil, fmt.Errorf("character state is not an object")
	}
	v, e := json.Marshal(b)
	if e != nil {
		return nil, e
	}
	fields["inventory"] = v
	return json.Marshal(fields)
}
func (b Bag) Rows() [][protocol.CurrentItemRecordSize]byte {
	items := append([]BagItem(nil), b.Items...)
	sort.Slice(items, func(i, j int) bool { return items[i].Slot < items[j].Slot })
	rows := [][protocol.CurrentItemRecordSize]byte{
		protocol.OrdinaryItem(0, 0, b.Gold),
		protocol.OrdinaryItem(1, 1, b.Coin),
	}
	for _, i := range items {
		rows = append(rows, protocol.OrdinaryItem(i.Slot, i.Template, i.Amount, i.ExpireTime))
	}
	for _, i := range b.Equipment {
		rows = append(rows, EquipmentRow(i))
	}
	sort.Slice(rows, func(i, j int) bool {
		return uint16(rows[i][0])|uint16(rows[i][1])<<8 < uint16(rows[j][0])|uint16(rows[j][1])<<8
	})
	return rows
}

// RowAt returns the protocol row for one slot, matching the Rows() layout for
// that slot. It lets callers publish an incremental NOTI14 update so the
// client only marks the changed slot as newly obtained instead of flashing
// the whole bag's new-item highlight on every pickup.
func (b Bag) RowAt(slot uint16) ([protocol.CurrentItemRecordSize]byte, bool) {
	switch slot {
	case 0:
		return protocol.OrdinaryItem(0, 0, b.Gold), true
	case 1:
		return protocol.OrdinaryItem(1, 1, b.Coin), true
	}
	for _, i := range b.Items {
		if i.Slot == slot {
			return protocol.OrdinaryItem(i.Slot, i.Template, i.Amount, i.ExpireTime), true
		}
	}
	for _, e := range b.Equipment {
		if e.Slot == slot {
			return EquipmentRow(e), true
		}
	}
	return [protocol.CurrentItemRecordSize]byte{}, false
}

// Add updates the whole bag in the caller's character transaction. It does
// not silently spill, drop or partially grant a stack when the bag is full.
func (b Bag) Add(c catalog.LootCatalog, r BagRules, id, amount uint32, expireTime ...uint32) (Bag, uint16, error) {
	if amount == 0 || r.Source != c.Source.Checksum {
		return b, 0, fmt.Errorf("invalid inventory award/source")
	}
	var exp uint32
	if len(expireTime) > 0 {
		exp = expireTime[0]
	}
	b.Items = append([]BagItem(nil), b.Items...)
	if id == 0 {
		if uint64(b.Gold)+uint64(amount) > math.MaxUint32 {
			return b, 0, fmt.Errorf("gold overflow")
		}
		b.Gold += amount
		return b, 0, nil
	}
	if id == 1 {
		if uint64(b.Coin)+uint64(amount) > math.MaxUint32 {
			return b, 0, fmt.Errorf("coin overflow")
		}
		b.Coin += amount
		return b, 1, nil
	}
	item, ok := c.Items[id]
	if !ok || item.Kind != "stackable" {
		return b, 0, fmt.Errorf("unsupported source item")
	}
	slots := stackableSlotRange(r, item.StackableType)
	limit := stackLimitFor(r, item.StackableType, item.StackLimit)
	if amount > limit {
		return b, 0, fmt.Errorf("award exceeds stack limit")
	}
	occupied := map[uint16]bool{}
	for _, i := range b.Equipment {
		occupied[i.Slot] = true
	}
	for i, row := range b.Items {
		occupied[row.Slot] = true
		// 新发放的限时物品不能继承另一堆的非零期限（尤其是已过期的旧堆）。
		if exp != 0 && row.ExpireTime != 0 && row.ExpireTime != exp {
			continue
		}
		if row.Template == id && row.Slot >= slots[0] && row.Slot <= slots[1] && uint64(row.Amount)+uint64(amount) <= uint64(limit) {
			b.Items[i].Amount += amount
			if exp != 0 && b.Items[i].ExpireTime == 0 {
				b.Items[i].ExpireTime = exp
			}
			return b, row.Slot, nil
		}
	}
	for n := uint32(slots[0]); n <= uint32(slots[1]); n++ {
		slot := uint16(n)
		if !occupied[slot] {
			b.Items = append(b.Items, BagItem{Slot: slot, Template: id, Amount: amount, ExpireTime: exp})
			return b, slot, nil
		}
	}
	return b, 0, fmt.Errorf("bag category is full")
}

// SweepStackSlots relocates saved stackables with known categories. It leaves
// quick slots and unknown types alone, and returns the original bag on any
// placement error so a failed migration cannot lose an item.
func SweepStackSlots(b Bag, c catalog.LootCatalog, r BagRules) (Bag, bool, error) {
	next := b
	next.Items = append([]BagItem(nil), b.Items...)
	moved := false
	for i := 0; i < len(next.Items); {
		row := next.Items[i]
		definition, ok := c.Items[row.Template]
		if !ok || definition.Kind != "stackable" || row.Slot <= 8 {
			i++
			continue
		}
		slots, known := classifyStackableSlot(r, definition.StackableType)
		if !known || row.Slot >= slots[0] && row.Slot <= slots[1] {
			i++
			continue
		}
		next.Items = append(next.Items[:i], next.Items[i+1:]...)
		placed, err := next.AddMailItem(c, r, nil, MailItem{Stack: &row})
		if err != nil {
			return b, false, fmt.Errorf("relocate stackable %d from slot %d: %w", row.Template, row.Slot, err)
		}
		next = placed
		moved = true
		// The relocated row is appended in its correct range; examine the
		// remaining original row at this index next.
	}
	return next, moved, nil
}

// WornBaseItems filters b.Worn down to at most one item per slot (0..47),
// preferring Group 0 (Clone or base equipment) over Group 1 (Appearance).
// This is used for NOTI 13/14 packets which require unique slot entries.
func (b Bag) WornBaseItems() []BagEquipment {
	bySlot := make(map[uint16]BagEquipment)
	for _, item := range b.Worn {
		if item.Group == 1 {
			bySlot[item.Slot] = item
		}
	}
	for _, item := range b.Worn {
		if item.Group == 0 {
			bySlot[item.Slot] = item
		}
	}
	res := make([]BagEquipment, 0, len(bySlot))
	for _, item := range bySlot {
		res = append(res, item)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Slot < res[j].Slot })
	return res
}
