package inventory

import (
	"encoding/json"
)

// Extended equipment-slot unlock bits, as stored in Bag.ExpandEquipFlags.
//
// Two paths award them: quest [slot expansion] rewards (internal/quest) and
// Odyssey dungeon clears (internal/character, see OdysseyExpandEquipMask).
// USERINFO1 / EntryAddition then projects the byte back to the armoury padlocks.
// Slot numbers: support 22, magic stone 23, earring 25.
const (
	ExpandSupport    byte = 1 << 0 // equipment slot 22
	ExpandMagicStone byte = 1 << 1 // equipment slot 23
	ExpandEarring    byte = 1 << 4 // equipment slot 25
)

// UnlockEquipSlots accumulates extended equipment-slot unlock bits into the
// saved bag and returns the rewritten character state.
//
// The mask is OR-ed into a byte the armoury rebuilds its padlocks from, so
// repeated awards and out-of-order quest completion are both safe: finishing
// 650 before 649 leaves 3, and a third award leaves 19. A zero mask is a
// no-op and returns the state untouched rather than rewriting it.
//
// ReadBag/SaveBag bracket the change the same way Awarder.Grant does, so the
// new bits land in the caller's character transaction and roll back with it.
func UnlockEquipSlots(raw json.RawMessage, mask byte) (json.RawMessage, error) {
	if mask == 0 {
		return raw, nil
	}
	b, e := ReadBag(raw)
	if e != nil {
		return nil, e
	}
	b.ExpandEquipFlags |= mask
	return SaveBag(raw, b)
}
