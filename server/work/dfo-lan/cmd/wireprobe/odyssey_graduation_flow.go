package main

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"fmt"
	"time"
)

func returnedToTown(plan []outboundPacket) bool {
	for _, p := range plan {
		if p.Name == "dungeon_return_users" {
			return true
		}
	}
	return false
}

// Call after town-return packets are written and the run is detached, never
// during settlementExit's route preflight or the final dungeon cinematic.
func (w *worldSession) graduateOdysseyAtTown() ([]outboundPacket, error) {
	if w == nil || w.activeDungeon != nil || w.selectingDungeon || !character.CreatedAsOdyssey(w.role) {
		return nil, nil
	}
	if w.quests == nil || w.characters == nil {
		return nil, fmt.Errorf("graduation town services missing")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	role, applied, err := w.quests.GraduateOdyssey(ctx, w.role)
	if err != nil {
		return nil, err
	}
	role.WireID = w.role.WireID
	w.role = role
	w.odyssey = character.OdysseyMember(role)
	if !applied {
		return nil, nil
	}
	plan, err := w.unlockRefresh(role)
	if err != nil {
		return nil, err
	}
	active, err := w.quests.Active(ctx, role)
	if err != nil {
		return nil, err
	}
	done, err := w.quests.Completed(ctx, role)
	if err != nil {
		return nil, err
	}
	triggers, err := protocol.QuestTriggers(active)
	if err != nil {
		return nil, err
	}
	completed, err := protocol.CompletedQuests(done)
	if err != nil {
		return nil, err
	}
	available, err := w.availableQuestPayload(ctx)
	if err != nil {
		return nil, err
	}
	return append(plan,
		outboundPacket{"odyssey_graduation_triggers", 0, 291, triggers},
		outboundPacket{"odyssey_graduation_completed_quests", 0, 342, completed},
		outboundPacket{"odyssey_graduation_available_quests", 0, 21, available}), nil
}
