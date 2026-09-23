package quest

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"fmt"
)

// SceneClearObjective validates a client SET_QUEST_TRIGGER (CMD33) raised
// from inside an active story scene. A [clear map] scene quest owns no rank-3
// boss monster, so the client never sends a boss check: it walks the layered
// boss room to its final map, plays the closing cinematic and sets the quest
// trigger directly (quest 3191, palaceofload maze 6, layer maps
// 100008695/100008694/100008684/100008683, live 2026-09-22). The run's
// presence on the objective map is the server-owned evidence: ordered scene
// transitions are the only way onto a layered final map, and every preceding
// room had to be cleared first. The trigger value itself is never trusted.
//
// applies=false means the quest is not a single-map [clear map] scene
// objective and the caller must silently ignore the request, exactly like the
// town path ignores non-[meet npc] quests. An error means the quest does own
// a scene objective but the run is not standing on it.
func SceneClearObjective(d catalog.QuestDefinition, run *dungeon.Session) (objective uint32, applies bool, err error) {
	if len(d.Pending) != 0 || d.Kind != "[clear map]" || len(d.ObjectiveCells) != 1 || d.ObjectiveCells[0].Type != 0 || d.ObjectiveCells[0].Value <= 0 {
		return 0, false, nil
	}
	if run == nil || !run.Loaded {
		return 0, true, fmt.Errorf("scene trigger requires the owned loaded run")
	}
	objective = uint32(d.ObjectiveCells[0].Value)
	if run.Room.Map != objective {
		return 0, true, fmt.Errorf("quest %d scene trigger outside its source objective map", d.ID)
	}
	return objective, true, nil
}

// SceneTrigger settles a [clear map] objective from inside its own story
// scene and returns the active quest list for the NOTI291 trigger sync, the
// same response the town [meet npc] path produces. The clear evidence is
// recorded against the owned run, so a replayed or foreign trigger cannot
// mint progress, and a duplicate scene trigger stays idempotent through the
// store's run-keyed evidence table.
func (s *Service) SceneTrigger(ctx context.Context, role storage.Character, run *dungeon.Session, id uint16) ([]protocol.ActiveQuest, error) {
	d, ok := s.Catalog.Quests[uint32(id)]
	if !ok {
		return nil, nil
	}
	objective, applies, err := SceneClearObjective(d, run)
	if err != nil || !applies {
		return nil, err
	}
	x := s.Index()
	if _, err := s.Store.RecordQuestMapClear(ctx, role.AccountID, role.ID, run.RunID, objective, s.Catalog.Source.Checksum, SingleClearMap, x.ByClearMap[objective]); err != nil {
		return nil, err
	}
	return s.Active(ctx, role)
}
