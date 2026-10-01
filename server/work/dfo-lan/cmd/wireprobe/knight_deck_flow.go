package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"dfolan/internal/workflow"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// The launcher runs the gateway from the pack root and supplies absolute
// wear-rule paths. Resolve the optional side-car beside that loaded config.
func knightShieldCatalogPath(configured, wearRulesPath string) string {
	if configured == "" || filepath.IsAbs(configured) {
		return configured
	}
	return filepath.Join(filepath.Dir(wearRulesPath), configured)
}

func (s *equipmentSession) handleKnightDeck(service *workflow.WearService, w *worldSession, p, raw []byte) ([]outboundPacket, error) {
	deck, e := protocol.DecodeKnightDeck(p)
	if e != nil {
		return nil, e
	}
	if service == nil || w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("knight deck requires owned character")
	}
	key, e := s.requestKey(raw)
	if e != nil {
		return nil, e
	}
	before, e := inventory.ReadBag(w.role.State)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, applied, e := service.CommitKnightDeck(ctx, w.role, "knight-deck:"+key, deck)
	if e != nil {
		return nil, e
	}
	w.role = saved
	if !applied {
		return nil, nil
	}
	after, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	if before.KnightDeck()[0] == after.KnightDeck()[0] {
		return nil, nil
	}
	return knightShieldUpdates(w, saved, true)
}

func (s *equipmentSession) handleKnightShieldMove(service *workflow.WearService, w *worldSession, r protocol.ItemMoveRequest, raw []byte) ([]outboundPacket, error) {
	key, e := s.requestKey(raw)
	if e != nil {
		return nil, e
	}
	before, e := inventory.ReadBag(w.role.State)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, e := service.Move(ctx, w.role, "equipment:"+key, r)
	if e != nil {
		return nil, e
	}
	w.role = saved
	after, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	changed := before.KnightDeck()[0] != after.KnightDeck()[0]
	plan := []outboundPacket{{"knight_shield_move_committed", 1, 19, protocol.ItemMoveSuccess(r, 1)}}
	// Shelf moves carry item identity and the client already paints them.
	if changed {
		repaint := r.SourceList == inventory.ShieldActiveSpace && r.DestinationList == inventory.ShieldActiveSpace
		updates, e := knightShieldUpdates(w, saved, repaint)
		if e != nil {
			return nil, e
		}
		plan = append(plan, updates...)
	}
	return plan, nil
}

func knightShieldUpdates(w *worldSession, saved storage.Character, repaint bool) ([]outboundPacket, error) {
	var plan []outboundPacket
	worn, e := inventory.WornPayload(saved.State)
	if e != nil {
		return nil, e
	}
	if len(worn) > 0 {
		plan = append(plan, outboundPacket{"equipment_worn_resynced", 0, 13, worn})
	}
	if repaint && os.Getenv("DFO_EQUIP_AVATAR_REFRESH") != "0" && w != nil && w.characters != nil && w.activeDungeon == nil {
		return knightShieldRepaint(w, saved, plan)
	}
	rebound, e := knightShieldWornUpdate(saved)
	if e != nil {
		return nil, e
	}
	return append(plan, outboundPacket{"knight_shield_worn_window_rebound", 0, 14, rebound}), nil
}

// Preserve the current branch's live dungeon restriction: rebuilding the
// actor with mode0 during a dungeon previously stopped CMD45 room transitions.
// Town uses both halves, with the window rebind strictly after them.
func knightShieldRepaint(w *worldSession, saved storage.Character, plan []outboundPacket) ([]outboundPacket, error) {
	appearance, e := w.characters.AppearanceProbe(saved, [2]byte{})
	if e != nil {
		return nil, e
	}
	basic, e := w.characters.EntryBasicProbe(saved, [2]byte{})
	if e != nil {
		return nil, e
	}
	rebound, e := knightShieldWornUpdate(saved)
	if e != nil {
		return nil, e
	}
	return append(plan,
		outboundPacket{"knight_shield_appearance_refreshed", 0, 2, appearance},
		outboundPacket{"knight_shield_worn_basic_refreshed", 0, 2, basic},
		outboundPacket{"knight_shield_worn_window_rebound", 0, 14, rebound},
	), nil
}

func knightShieldWornUpdate(saved storage.Character) ([]byte, error) {
	bag, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	rows := bag.WornBaseItems()
	if bag.KnightDeck()[0] == 0 {
		rows = append(rows, inventory.BagEquipment{Slot: 24, Template: 0xffffffff})
	}
	return inventory.EquipmentPayload(3, rows, false)
}

func knightShieldObservation(w *worldSession, id uint16, p []byte, old uint32, err error) map[string]any {
	event := map[string]any{"kind": "knight_deck_upload", "id": id, "refusal_code": uint16(0), "refuse_reason": "", "worn_changed": false, "equipped": uint32(0)}
	if id == 19 {
		event["kind"] = "knight_shield_move"
		if r, e := protocol.DecodeItemMove(p); e == nil {
			event["source_space"] = r.SourceList
			event["source_slot"] = r.SourceSlot
			event["source_item"] = r.SourceItem
			event["destination_space"] = r.DestinationList
			event["destination_slot"] = r.DestinationSlot
			event["destination_item"] = r.DestinationItem
		}
	} else if deck, e := protocol.DecodeKnightDeck(p); e == nil {
		event["deck"] = deck
	}
	if w != nil {
		event["character_id"] = w.role.ID
		if bag, e := inventory.ReadBag(w.role.State); e == nil {
			deck := bag.KnightDeck()
			event["saved_deck"] = deck
			event["equipped"] = deck[0]
			event["worn_changed"] = deck[0] != old
			if id == 19 {
				event["deck"] = deck
			}
		}
		event["appearance_deferred_in_dungeon"] = w.activeDungeon != nil
	}
	if err != nil {
		event["refuse_reason"] = err.Error()
		event["refusal_code"] = inventory.MoveRefusalCode(err)
	}
	return event
}
