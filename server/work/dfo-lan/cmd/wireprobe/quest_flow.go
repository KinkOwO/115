package main

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/quest"
	"dfolan/internal/storage"
	"encoding/binary"
	"fmt"
	"time"
)

// unlockRefresh re-publishes the actor's detail packets after a reward that
// changed the extended-slot unlock byte.
//
// The armoury draws its padlocks from the byte the native USERINFO1 reader
// stores at character +0x198 (14563d692), so the new value has to arrive in a
// fresh mode 1 addition - no other packet carries it, and the NOTI328 refresh
// only re-reads a state the client already holds. A mode 0 must precede that
// addition, and it resets the client's worn containers, so the worn rows
// follow immediately. This is the same trio dungeonEntryPlan sends on every
// mid-session actor rebuild.
func (w *worldSession) unlockRefresh(role storage.Character) ([]outboundPacket, error) {
	if w.characters == nil {
		return nil, nil
	}
	visual, e := w.characters.EntryBasicProbe(role, [2]byte{})
	if e != nil {
		return nil, e
	}
	addition, e := w.characters.EntryAddition(role)
	if e != nil {
		return nil, e
	}
	plan := []outboundPacket{
		{"reward_actor_appearance_sent", 0, 2, visual},
		{"reward_actor_addition_sent", 0, 2, addition},
	}
	if worn, e := inventory.WornSpaceUpdate(role.State); e == nil && len(worn) > 0 {
		plan = append(plan, outboundPacket{"reward_worn_visuals_sent", 0, 14, worn})
	}
	return plan, nil
}

func (w *worldSession) finishQuest(r protocol.QuestSubmitRequest) ([]outboundPacket, error) {
	if w == nil || w.quests == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("quest submit before character selection")
	}
	if w.answeredQuests[r.ID] {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, e := w.quests.Finish(ctx, w.role, r)
	if e != nil {
		return nil, e
	}
	result.Role.WireID = w.role.WireID
	active, e := w.quests.Active(ctx, result.Role)
	if e != nil {
		return nil, e
	}
	done, e := w.quests.Completed(ctx, result.Role)
	if e != nil {
		return nil, e
	}
	triggers, e := protocol.QuestTriggers(active)
	if e != nil {
		return nil, e
	}
	completed, e := protocol.CompletedQuests(done)
	if e != nil {
		return nil, e
	}
	experience, e := character.ExperiencePayload(result.Role)
	if e != nil {
		return nil, e
	}
	var plan []outboundPacket
	// The native terminal ACK mutates a live quest object. After a reconnect,
	// restore saved state without dereferencing that already-completed object.
	if result.Applied {
		ack, err := protocol.QuestFinishedNoItems(r.ID, result.Receipt.Experience)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"quest_finished", 1, 34, ack})
	}
	// Resend the ordinary bag whenever the reward changed it - items OR gold.
	// The gold balance rides the bag as the slot-0/template-0 row (Bag.Rows),
	// so a quest that pays only gold (its [job] item tuples do not match this
	// character) must still refresh it; otherwise the wallet never ticks up in
	// town even though the balance is committed. Live capture 20260911T215854
	// shows quest 3149 crediting 4300 gold with no items and no NOTI13, so the
	// on-screen number stayed put until the next relog.
	if len(result.Receipt.Items) > 0 || result.Receipt.Gold > 0 {
		accountMaterial := false
		for _, item := range result.Receipt.Items {
			if _, ok := inventory.AccountMaterialSlot(item.Template); ok {
				accountMaterial = true
				break
			}
		}
		if accountMaterial {
			// Account-shared materials never stay in the bag: sweep them into
			// the account storage and precede the list0 snapshot with the
			// list35 storage snapshot so the client harvest adopts them.
			var materials inventory.AccountMaterials
			result.Role, materials, e = sweepAccountMaterials(ctx, w.quests.Store, result.Role)
			if e != nil {
				return nil, e
			}
			storageBody, e := accountMaterialSnapshot(materials)
			if e != nil {
				return nil, e
			}
			plan = append(plan, outboundPacket{"quest_account_materials_committed", 0, 13, storageBody})
		}
		bag, e := inventory.ReadBag(result.Role.State)
		if e != nil {
			return nil, e
		}
		body, e := protocol.InventoryRestore(bag.Rows())
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"quest_inventory_committed", 0, 13, body})
	}
	// A reward that opened an extended equipment slot has to be republished:
	// the client keeps showing the padlock until the new unlock byte arrives
	// in a USERINFO1 addition, even though the slot is already saved.
	if result.Receipt.UnlockedEquipment != 0 {
		refresh, e := w.unlockRefresh(result.Role)
		if e != nil {
			return nil, e
		}
		plan = append(plan, refresh...)
	}
	plan = append(plan, outboundPacket{"quest_experience", 0, 37, experience}, outboundPacket{"quest_triggers_updated", 0, 291, triggers}, outboundPacket{"completed_quests_updated", 0, 342, completed})
	w.role, w.level = result.Role, experience[0]
	available, e := w.availableQuestPayload(ctx)
	if e != nil {
		return nil, e
	}
	plan = append(plan, outboundPacket{"available_quests_updated", 0, 21, available})
	return plan, nil
}

func (w *worldSession) availableQuestPayload(ctx context.Context) ([]byte, error) {
	ids, e := w.quests.Available(ctx, w.role)
	if e != nil {
		return nil, e
	}
	return protocol.AvailableQuests(w.level, ids)
}

func (w *worldSession) questInteraction(p []byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.quests == nil {
		return nil, fmt.Errorf("quest check requires an owned character")
	}
	if len(p) != 16 || binary.LittleEndian.Uint16(p) != 33 {
		return nil, fmt.Errorf("unsupported quest check request")
	}
	for _, v := range p[4:] {
		if v != 0 {
			return nil, fmt.Errorf("unsupported quest check fields")
		}
	}
	id := binary.LittleEndian.Uint16(p[2:])
	if w.activeDungeon != nil {
		// A story scene ends with SET_QUEST_TRIGGER from its final layer map
		// instead of a boss check (quest 3191, live 2026-09-22): settle the
		// [clear map] objective against the owned run and sync triggers.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		active, e := w.quests.SceneTrigger(ctx, w.role, w.activeDungeon, id)
		if e != nil || active == nil {
			return nil, e
		}
		body, e := protocol.QuestTriggers(active)
		if e != nil {
			return nil, e
		}
		return []outboundPacket{{"quest_scene_trigger", 0, 291, body}}, nil
	}
	d, ok := w.quests.Catalog.Quests[uint32(id)]
	if !ok || d.Kind != "[meet npc]" || len(d.ObjectiveCells) != 1 {
		return nil, nil
	}
	npc := uint32(d.ObjectiveCells[0].Value)
	// A [sub type] 1 quest is settled by the conversation the client already
	// validated against its own resolved target, which for the three
	// extended-slot quests (649/650/2636) is an NPC that stands in a different
	// town than the raw objective. The positional test stays for the ordinary
	// form, so a request naming a quest whose NPC is genuinely elsewhere is
	// still refused, and the passive ProximityProgress walk keeps its own
	// in-area requirement (it never consults this path).
	if !quest.AllowsRemoteNPCInteraction(d) &&
		(w.service == nil || !w.service.HasNPC(w.state.Position, npc)) {
		return nil, fmt.Errorf("quest %d NPC %d is absent from current source area %d/%d", id, npc, w.state.Position.Town, w.state.Position.Area)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if e := w.quests.MeetNPC(ctx, w.role, id, npc); e != nil {
		return nil, e
	}
	active, e := w.quests.Active(ctx, w.role)
	if e != nil {
		return nil, e
	}
	body, e := protocol.QuestTriggers(active)
	if e != nil {
		return nil, e
	}
	return []outboundPacket{{"quest_npc_objective", 0, 291, body}}, nil
}
