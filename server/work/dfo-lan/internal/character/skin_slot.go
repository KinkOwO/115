package character

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

// ExpandSkinSlot consumes one aura/creature ticket and unlocks its USERINFO1 bit.
// An explicit requestedSlot is checked inside the transaction; absent/zero
// means the caller has no native slot and needs a matching owned ticket.
// Never spend a different slot than the inventory row published by the caller.
//
// The whole change - the spent ticket row and the new unlock byte - lands in one
// CommitCharacterEvent transaction keyed by the request, so a client retry
// replays the receipt instead of spending a second ticket. The returned bool is
// false for such a replay: the caller must then skip the bag refresh, because
// the bag it would report on is the one the first attempt already published.
//
// Callers re-publish USERINFO1 afterwards: no other packet carries the unlock
// byte (see worldSession.unlockRefresh).
func (s *Service) ExpandSkinSlot(ctx context.Context, role Character, key string, mask byte, source catalog.LootCatalog, requestedSlot ...uint16) (Character, bool, error) {
	if s == nil || s.Store == nil {
		return role, false, fmt.Errorf("skin slot expansion without a store")
	}
	if mask != inventory.ExpandAuraSkin && mask != inventory.ExpandCreatureSkin {
		return role, false, fmt.Errorf("skin slot expansion without a mask")
	}
	if key == "" {
		return role, false, fmt.Errorf("skin slot expansion without an event key")
	}
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "skin-slot-expand-v1",
		func(cur Character) (json.RawMessage, json.RawMessage, error) {
			b, err := inventory.ReadBag(cur.State)
			if err != nil {
				return nil, nil, err
			}
			slot, ok := inventory.SkinSlotTicketSlot(b, mask, source)
			if len(requestedSlot) > 0 && requestedSlot[0] != 0 {
				slot, ok = requestedSlot[0], false
				for _, item := range b.Items {
					if item.Slot == slot {
						m, recognized := inventory.SkinSlotMaskForTicket(item.Template, source)
						ok = recognized && m == mask && item.Amount > 0
						break
					}
				}
			}
			if !ok {
				return nil, nil, fmt.Errorf("no ticket for unlock mask %#02x in the main bag", mask)
			}
			index := -1
			for i, item := range b.Items {
				if item.Slot == slot {
					index = i
					break
				}
			}
			if b.ExpandEquipFlags&mask != 0 {
				return nil, nil, fmt.Errorf("skin slot already expanded")
			}
			ticket := b.Items[index].Template
			if b.Items[index].Amount <= 1 {
				b.Items = append(b.Items[:index], b.Items[index+1:]...)
			} else {
				b.Items[index].Amount--
			}
			b.ExpandEquipFlags |= mask
			next, err := inventory.SaveBag(cur.State, b)
			if err != nil {
				return nil, nil, err
			}
			receipt, _ := json.Marshal(map[string]any{"slot": slot, "template": ticket, "mask": mask})
			return next, receipt, nil
		})
	if e != nil {
		return role, false, e
	}
	saved.WireID = role.WireID
	return saved, applied, nil
}
