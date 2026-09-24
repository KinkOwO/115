package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"fmt"
	"time"
)

type equipmentSession struct {
	nonce       [16]byte
	initialized bool
}

func (s *equipmentSession) handle(service *inventory.WearService, w *worldSession, p, raw []byte) ([]outboundPacket, error) {
	if service == nil || w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("equipment move requires owned character")
	}
	r, e := protocol.DecodeItemMove(p)
	if e != nil {
		return nil, e
	}
	if r.SourceList == 12 || r.DestinationList == 12 {
		if !s.initialized {
			if _, e = rand.Read(s.nonce[:]); e != nil {
				return nil, e
			}
			s.initialized = true
		}
		return w.moveAccountVault(service, r, fmt.Sprintf("account-vault-move:%x:%x", s.nonce, sha256.Sum256(raw)))
	}
	// CMD19 carries every bag move, not only equipment. A move involving
	// the personal vault (list 2) belongs to the vault path. A stack going onto
	// the quick-use belt belongs to the stackable path; anything neither
	// recognises falls through to the equipment move unchanged.
	if plan, handled, e := w.moveVault(service.BagRules, r); handled {
		return plan, e
	}
	if !s.initialized {
		if _, e = rand.Read(s.nonce[:]); e != nil {
			return nil, e
		}
		s.initialized = true
	}
	hash := sha256.Sum256(raw)
	// A stack going onto the quick-use belt belongs to the stackable path; anything it does not
	// recognise falls through to the equipment move unchanged.
	if plan, handled, e := w.moveStack(service.BagRules, r, fmt.Sprintf("bagmove:%x:%x", s.nonce, hash)); handled {
		return plan, e
	}
	// (20260918: the live-catalog warm block was removed together with the
	// move-path PVF validation itself - the move no longer reads the gear
	// catalog, so there is nothing to preheat.)
	key := fmt.Sprintf("equipment:%x:%x", s.nonce, hash)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, applied, e := service.Move(ctx, w.role, key, r)
	if e != nil {
		return nil, e
	}
	if !applied {
		w.role = saved
		return nil, nil
	}
	b, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	// Two channels, two jobs. The id-13 restores re-feed the bag grid and the
	// worn model (same channel as entry and quest rewards). The id-14 frames
	// re-feed the worn-slot WINDOW: the 20260918 live contrast showed the
	// window emptying when only id-13 was sent, so both go out together. (The
	// earlier note claiming id-14 parses no rows misread the dispatcher front
	// half as the whole receiver; the row walk lives in its 0x1452a1210 body.)
	plan := []outboundPacket{{"equipment_move_committed", 1, 19, protocol.ItemMoveSuccess(r, 1)}}
	bagBody, e := protocol.InventoryRestore(b.Rows(), b.Expansion)
	if e != nil {
		return nil, e
	}
	plan = append(plan, outboundPacket{"equipment_bag_resynced", 0, 13, bagBody})
	wornBody, e := inventory.WornPayload(saved.State)
	if e != nil {
		return nil, e
	}
	if len(wornBody) > 0 {
		plan = append(plan, outboundPacket{"equipment_worn_resynced", 0, 13, wornBody})
	}
	for _, space := range []byte{0, 1, 3, 7} {
		var rows []inventory.BagEquipment
		for _, loc := range []struct {
			space byte
			slot  uint16
		}{{r.SourceList, r.SourceSlot}, {r.DestinationList, r.DestinationSlot}} {
			if loc.space != space {
				continue
			}
			row := inventory.BagEquipment{Slot: loc.slot, Template: 0xFFFFFFFF}
			items := b.Equipment
			if space == 3 {
				items = b.WornBaseItems()
			}
			if space == 1 || space == 7 {
				items = b.Special[space]
			}
			for _, item := range items {
				if item.Slot == loc.slot {
					row = item
					break
				}
			}
			rows = append(rows, row)
		}
		if len(rows) > 0 {
			body, e := inventory.EquipmentPayload(space, rows, false)
			if e != nil {
				return nil, e
			}
			plan = append(plan, outboundPacket{"equipment_slots_updated", 0, 14, body})
		}
	}
	if r.SourceList == 7 || r.DestinationList == 7 || r.SourceSlot == 26 || r.DestinationSlot == 26 {
		clPayload, err := inventory.CreatureListPayload(saved.State)
		if err == nil {
			plan = append(plan, outboundPacket{"creature_list_updated", 0, 105, clPayload})
		}
		if (r.DestinationList == 3 && r.DestinationSlot == 26) || (r.SourceList == 3 && r.SourceSlot == 26) {
			if inventory.HasEquippedCreature(saved.State) {
				growth, err := inventory.CreatureGrowthPayload(saved.State)
				if err == nil {
					plan = append(plan, outboundPacket{"creature_growth_updated", 0, 102, growth})
				}
			}
		}
	}
	wornUpdate, e := inventory.WornSpaceUpdate(saved.State)
	if e != nil {
		return nil, e
	}
	if len(wornUpdate) > 0 {
		plan = append(plan, outboundPacket{"equipment_worn_window_refreshed", 0, 14, wornUpdate})
	}
	// Appearance refresh (C9): when the move touched the worn set, re-send the
	// mode0 userinfo with the equipped-appearance block bound to the new state.
	// The client's CMD19 apply routine updates its per-slot model table from
	// this block (0x145639840 rows land at [actor+slot*4+0x405], the slot set
	// 0x145a8a780 covers), which is what makes the world model follow the
	// change. It follows this handler's id13/id14 rows so the rebuild sees
	// fresh item objects. Bag-to-bag moves change no visible slot and skip it.
	if shouldSendEquipmentAppearanceRebuild(w.activeDungeon != nil, r) && w.characters != nil {
		probe, e := w.characters.AppearanceProbe(saved, [2]byte{})
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"equipment_appearance_refreshed", 0, 2, probe})
	}
	w.role = saved
	return plan, nil
}

// A mode-0 appearance probe reconstructs the live actor. Keep the CMD19 bag
// and worn-slot updates in a dungeon, but defer this actor rebuild until the
// next town/entry refresh. In the 2026-09-23 live trace, a worn move emitted
// NOTI2 during map 58591; afterward the client acknowledged door interaction
// (CMD38) but stopped issuing the room transition (CMD45). Before that move,
// the same run had advanced rooms normally.
func shouldSendEquipmentAppearanceRebuild(inDungeon bool, r protocol.ItemMoveRequest) bool {
	return !inDungeon && (r.SourceList == 3 || r.DestinationList == 3)
}

func cloneAvatarRemoval(r protocol.ItemMoveRequest, catalog *inventory.EquipmentCatalog) bool {
	// The observed unequip request swaps an empty bag source with the Clone
	// currently in the worn destination. Restrict this experiment to that path.
	if r.SourceList != 1 || r.SourceItem != 0 || r.DestinationList != 3 || r.DestinationSlot > 11 || catalog == nil {
		return false
	}
	if r.DestinationItem == 0 || r.DestinationItem == 0xFFFFFFFF {
		return false
	}
	definition, err := catalog.Definition(r.DestinationItem)
	return err == nil && definition.IsCloneAvatar()
}
