package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"time"
)

// Both ordinary completion and Ispins' dedicated settlement must submit the
// confirmed source maps. RecordQuestMapClear is keyed by run/map, so a retry
// cannot count a completed stage twice. Content-clear guide objectives retain
// their existing progress contract; a single dragon is not a full legion run.
func (w *worldSession) mapClearQuestPlan() ([]outboundPacket, error) {
	if w.quests == nil || w.dungeons == nil {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	active, err := w.quests.MapClear(ctx, w.role, w.activeDungeon, w.dungeons.Source.SaveIdentity())
	if err != nil {
		return nil, err
	}
	triggers, err := protocol.QuestTriggers(active)
	if err != nil {
		return nil, err
	}
	return []outboundPacket{{"map_clear_quest_triggers", 0, 291, triggers}}, nil
}
