package main

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"fmt"
	"time"
)

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
	if w == nil || w.role.ID == 0 || w.quests == nil || w.activeDungeon != nil {
		return nil, fmt.Errorf("NPC interaction requires an owned town character")
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
	d, ok := w.quests.Catalog.Quests[uint32(id)]
	if !ok || d.Kind != "[meet npc]" || len(d.ObjectiveCells) != 1 {
		return nil, nil
	}
	npc := uint32(d.ObjectiveCells[0].Value)
	if !w.service.HasNPC(w.state.Position, npc) {
		return nil, fmt.Errorf("quest NPC is absent from current source area")
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
