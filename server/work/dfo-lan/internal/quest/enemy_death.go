package quest

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/storage"
)

// EnemyDeath advances an accepted single-target hunt only after the owned run
// confirms the source monster's death in that quest's dedicated maze.
func (s *Service) EnemyDeath(ctx context.Context, role storage.Character, run *dungeon.Session, entity uint16) (bool, error) {
	if run == nil || run.Maze.Quest == 0 {
		return false, nil
	}
	qid := run.Maze.Quest
	en := s.Index().Entries[uint32(qid)]
	if !singleHuntMatch(en, run, entity) {
		return false, nil
	}
	return s.Store.CompleteQuestObjective(ctx, role.AccountID, role.ID, qid, s.Catalog.Source.Checksum, en.Model)
}

func singleHuntMatch(en *Entry, run *dungeon.Session, entity uint16) bool {
	if en == nil || !en.Implemented || en.Model != SingleHuntEnemy ||
		run == nil || !run.Loaded || !run.Dead[entity] || run.Unowned[entity] ||
		en.ID != uint32(run.Maze.Quest) || run.Definition.ID != en.HuntDungeon {
		return false
	}
	for _, monster := range run.Monsters {
		if monster.Entity == entity && monster.Template == en.HuntEnemy {
			return true
		}
	}
	return false
}
