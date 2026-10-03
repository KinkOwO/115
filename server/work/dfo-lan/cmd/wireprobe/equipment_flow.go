package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"fmt"
	"time"
)

type equipmentSession struct {
	nonce       [16]byte
	initialized bool
	// 增幅摧毁装备后，客户端会把金币**显示**清成 0（存档里的金币是对的，重登即恢复）。
	// 单独补发一次金币包没用 —— 客户端是在「装备破坏」动画之后才刷 UI 的，
	// 所以这里把金币包挂起来，等主循环到点后再补发一次（同一 goroutine 串行发送，不并发写 socket）。
	pendingGoldBody []byte
	pendingGoldDue  time.Time
}

// requestKey shares the session nonce; callers retain their operation prefixes.
func (s *equipmentSession) requestKey(raw []byte) (string, error) {
	if !s.initialized {
		if _, e := rand.Read(s.nonce[:]); e != nil {
			return "", e
		}
		s.initialized = true
	}
	return fmt.Sprintf("%x:%x", s.nonce, sha256.Sum256(raw)), nil
}

// takePendingGold 到点则取出待补发的金币包（未到点或没有则返回 nil）。
func (s *equipmentSession) takePendingGold(now time.Time) []byte {
	if len(s.pendingGoldBody) == 0 || now.Before(s.pendingGoldDue) {
		return nil
	}
	body := s.pendingGoldBody
	s.pendingGoldBody = nil
	s.pendingGoldDue = time.Time{}
	return body
}

func (s *equipmentSession) handle(service *workflow.WearService, w *worldSession, p, raw []byte) ([]outboundPacket, error) {
	if service == nil || w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("equipment move requires owned character")
	}
	r, e := protocol.DecodeItemMove(p)
	if e != nil {
		return nil, e
	}
	if inventory.IsKnightShieldMove(r) {
		return s.handleKnightShieldMove(service, w, r, raw)
	}
	if r.SourceList == 12 || r.DestinationList == 12 {
		key, e := s.requestKey(raw)
		if e != nil {
			return nil, e
		}
		return w.moveAccountVault(service, r, "account-vault-move:"+key)
	}
	// CMD19 carries every bag move, not only equipment. A move involving
	// the personal vault (list 2) belongs to the vault path. A stack going onto
	// the quick-use belt belongs to the stackable path; anything neither
	// recognises falls through to the equipment move unchanged.
	if plan, handled, e := w.moveVault(service.BagRules, r); handled {
		return plan, e
	}
	key, e := s.requestKey(raw)
	if e != nil {
		return nil, e
	}
	if plan, handled, e := w.movePetStack(service.BagRules, r, "petmove:"+key); handled {
		return plan, e
	}
	// A stack going onto the quick-use belt belongs to the stackable path; anything it does not
	// recognise falls through to the equipment move unchanged.
	if plan, handled, e := w.moveStack(service.BagRules, r, "bagmove:"+key); handled {
		return plan, e
	}
	// (20260918: the live-catalog warm block was removed together with the
	// move-path PVF validation itself - the move no longer reads the gear
	// catalog, so there is nothing to preheat.)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, applied, e := service.Move(ctx, w.role, "equipment:"+key, r)
	if e != nil {
		return nil, e
	}
	if !applied {
		w.role = saved
		return nil, nil
	}
	// 剑帝主副手互换，attempt 2/3（2026-09-25）。attempt 1 仅回 CMD19，
	// 实机仍保留旧城镇模型；重选角色正确。原生装备对象与城镇显示对象独立，
	// 城镇显示对象由 145BEFD60 消费外观表的首个模板字段。
	// 交换应答之后补发已修正主副手模板的外观；副本仍保留原生交换路径。
	if r.SourceList == 3 && r.DestinationList == 3 &&
		r.SourceItem != 0 && r.DestinationItem != 0 &&
		((r.SourceSlot == 12 && r.DestinationSlot == 24) ||
			(r.SourceSlot == 24 && r.DestinationSlot == 12)) {
		plan := []outboundPacket{{"equipment_weapon_swap_committed", 1, 19, protocol.ItemMoveSuccess(r, 1)}}
		if shouldSendEquipmentAppearanceRebuild(w.activeDungeon != nil, r) && w.characters != nil {
			probe, err := w.characters.AppearanceProbe(saved, [2]byte{})
			if err != nil {
				return nil, err
			}
			plan = append(plan, outboundPacket{"equipment_weapon_swap_appearance_refreshed", 0, 2, probe})
		}
		w.role = saved
		return plan, nil
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
	if moveTouchesWorn(r) {
		wornBody, e := inventory.WornPayload(saved.State)
		if e != nil {
			return nil, e
		}
		if len(wornBody) > 0 {
			plan = append(plan, outboundPacket{"equipment_worn_resynced", 0, 13, wornBody})
		}
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
	if moveTouchesCreature(r) {
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
	if moveTouchesWorn(r) {
		wornUpdate, e := inventory.WornSpaceUpdate(saved.State)
		if e != nil {
			return nil, e
		}
		if len(wornUpdate) > 0 {
			plan = append(plan, outboundPacket{"equipment_worn_window_refreshed", 0, 14, wornUpdate})
		}
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
	if (r.SourceList == 3 && r.SourceSlot == 26) || (r.DestinationList == 3 && r.DestinationSlot == 26) {
		loyaltyCtx, loyaltyCancel := context.WithTimeout(context.Background(), 5*time.Second)
		loyaltyPackets, loyaltyErr := w.refreshCreatureLoyalty(loyaltyCtx, time.Now(), w.activeDungeon != nil)
		loyaltyCancel()
		if loyaltyErr == nil {
			plan = append(plan, loyaltyPackets...)
		}
	}
	return plan, nil
}

// A mode-0 appearance probe reconstructs the live actor. Keep the CMD19 bag
// and worn-slot updates in a dungeon, but defer this actor rebuild until the
// next town/entry refresh. In the 2026-09-23 live trace, a worn move emitted
// NOTI2 during map 58591; afterward the client acknowledged door interaction
// (CMD38) but stopped issuing the room transition (CMD45). Before that move,
// the same run had advanced rooms normally.
func shouldSendEquipmentAppearanceRebuild(inDungeon bool, r protocol.ItemMoveRequest) bool {
	return !inDungeon && moveTouchesWorn(r)
}

func moveTouchesWorn(r protocol.ItemMoveRequest) bool {
	return r.SourceList == 3 || r.DestinationList == 3
}

func moveTouchesCreature(r protocol.ItemMoveRequest) bool {
	return r.SourceList == 7 || r.DestinationList == 7 ||
		(r.SourceList == 3 && r.SourceSlot == 26) ||
		(r.DestinationList == 3 && r.DestinationSlot == 26)
}

func moveNeedsCreatureActorAppearance(r protocol.ItemMoveRequest) bool {
	return !moveTouchesWorn(r) && moveTouchesCreature(r) &&
		(r.SourceSlot == 26 || r.DestinationSlot == 26)
}

// The confirmed CMD37 repair must run again after a successful in-dungeon
// worn move: the move's NOTI14 rows rebuild item visuals, including Clone
// objects whose cover had already been attached at room load. A bag-only move
// does not touch those objects. Keep the same detach/reattach/ordinary-gear
// order as finishDungeonLoading; this is a new live timing to verify manually.
func dungeonCloneEquipmentRefresh(w *worldSession, r protocol.ItemMoveRequest) ([]outboundPacket, bool, error) {
	if w == nil || w.activeDungeon == nil || w.characters == nil || !moveTouchesWorn(r) {
		return nil, false, nil
	}
	reset, full, enabled, err := w.characters.CloneReattachPackets(w.role)
	if err != nil || !enabled {
		return nil, false, err
	}
	restore, err := inventory.NonAvatarWornSpaceUpdate(w.role.State)
	if err != nil {
		return nil, false, err
	}
	plan := []outboundPacket{
		{"equipment_dungeon_clone_detached", 0, 2, reset},
		{"equipment_dungeon_clone_reattached", 0, 2, full},
	}
	if len(restore) > 0 {
		plan = append(plan, outboundPacket{"equipment_dungeon_nonavatar_worn_restored", 0, 14, restore})
	}
	return plan, true, nil
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
