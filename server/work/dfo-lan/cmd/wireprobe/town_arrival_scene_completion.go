package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/quest"
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
	if err != nil || model != quest.LookCinematic {
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
	for _, packet := range plan {
		if packet.Name == "dungeon_clear_enabled" {
			w.completionSent = true
		}
	}
	return plan, nil
}
