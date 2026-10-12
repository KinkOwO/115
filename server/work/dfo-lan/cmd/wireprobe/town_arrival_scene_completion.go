package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/quest"
	"fmt"
	"time"
)

// completeTownArrivalCinematic is called only after CMD33 has matched an
// accepted active quest. Its zero progress makes the client confirmation legal,
// but only a loaded run belonging to that quest's imported town-arrival rule
// may turn that confirmation into dungeon completion.
func (w *worldSession) completeTownArrivalCinematic(q protocol.ActiveQuest) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.quests == nil || q.Progress != 0 {
		return nil, nil
	}
	run := w.activeDungeon
	if run == nil || !run.Loaded || w.completionSent {
		return nil, nil
	}
	scene, ok := w.townArrivalScenes[run.Definition.ID]
	if !ok || scene.DungeonID != run.Definition.ID || scene.QuestID != uint32(q.ID) || run.Maze.Quest != q.ID ||
		w.state.Position.Town != scene.Town || w.state.Position.Area != scene.Area {
		return nil, nil
	}
	definition, ok := w.quests.Catalog.Quests[scene.QuestID]
	if !ok {
		return nil, nil
	}
	_, model, err := quest.InitialProgress(definition)
	if err != nil || model != quest.TownArrivalCinematic {
		return nil, nil
	}
	run.MarkSceneCompleted()
	plan, err := w.completeDungeon()
	if err != nil {
		return nil, err
	}
	// Quest dispatch has no dungeon-plan flag consumer. Record the same clear
	// enable emitted by completeDungeon so CMD46 keeps its normal state gate;
	// CMD72 still waits for the owned settlement and card plan.
	filtered := plan[:0]
	for _, packet := range plan {
		// NOTI291 destroys/recreates accepted quests and rearms their town
		// arrival events. CMD33 updates this quest in place, as the GF trace
		// does, while MapClear's durable effects above remain committed.
		if packet.ID == 291 {
			continue
		}
		if packet.Name == "dungeon_clear_enabled" {
			w.completionSent = true
		}
		filtered = append(filtered, packet)
	}
	return filtered, nil
}

func (w *worldSession) townArrivalCinematicInteraction(id uint16) ([]outboundPacket, bool, error) {
	if w == nil || w.quests == nil {
		return nil, false, nil
	}
	d, found := w.quests.Catalog.Quests[uint32(id)]
	_, model, err := quest.InitialProgress(d)
	if !found || err != nil || model != quest.TownArrivalCinematic {
		return nil, false, nil
	}
	run := w.activeDungeon
	if run == nil || !run.Loaded {
		return nil, true, fmt.Errorf("arrival cinematic confirmation requires loaded source run")
	}
	scene, ok := w.townArrivalScenes[run.Definition.ID]
	if !ok || scene.QuestID != uint32(id) || scene.DungeonID != run.Definition.ID || run.Maze.Quest != id ||
		w.state.Position.Town != scene.Town || w.state.Position.Area != scene.Area {
		return nil, true, fmt.Errorf("arrival cinematic confirmation outside owned source")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := w.quests.TownArrivalCinematicTrigger(ctx, w.role, run, id); err != nil {
		return nil, true, err
	}
	active, err := w.quests.Active(ctx, w.role)
	if err != nil {
		return nil, true, err
	}
	for _, q := range active {
		if q.ID != id {
			continue
		}
		plan, err := w.completeTownArrivalCinematic(q)
		if err != nil {
			return nil, true, err
		}
		ack, err := protocol.QuestTriggerUpdated(q)
		return append(plan, outboundPacket{"quest_interaction_accepted", 1, 33, ack}), true, err
	}
	return nil, true, fmt.Errorf("arrival cinematic quest is not owned")
}

func (w *worldSession) validateTownArrivalQuest(sceneID uint32) error {
	scene := w.townArrivalScenes[sceneID]
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	active, err := w.quests.Active(ctx, w.role)
	if err != nil {
		return err
	}
	d, ok := w.quests.Catalog.Quests[scene.QuestID]
	_, model, modelErr := quest.InitialProgress(d)
	for _, q := range active {
		if uint32(q.ID) != scene.QuestID {
			continue
		}
		if ok && modelErr == nil && model == quest.TownArrivalCinematic && q.Progress == 0 {
			return fmt.Errorf("town arrival cinematic objective already completed; quest reward pending")
		}
		return nil
	}
	return fmt.Errorf("town arrival scene %d requires accepted source quest %d", sceneID, scene.QuestID)
}
