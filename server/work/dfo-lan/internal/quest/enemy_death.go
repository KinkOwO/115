package quest

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/storage"
)

// EnemyDeath advances an accepted single-target hunt after the current run
// confirms the source monster's death in that quest's dedicated maze. A
// scripted death counts as a death for the objective even when killer FFFF
// leaves it unowned for drops and experience.
func (s *Service) EnemyDeath(ctx context.Context, role storage.Character, run *dungeon.Session, entity uint16) (bool, error) {
	if run == nil || run.Maze.Quest == 0 {
		return false, nil
	}
	qid := run.Maze.Quest
	en := s.Index().Entries[uint32(qid)]
	if en == nil {
		return false, nil
	}
	var target uint32
	switch en.Model {
	case SingleHuntEnemy:
		target = en.HuntEnemy
	case SingleHuntMonster:
		target = en.HuntMonster
	default:
		return false, nil
	}
	if !singleKillMatch(en, run, entity, target) {
		return false, nil
	}
	return s.Store.CompleteQuestObjective(ctx, role.AccountID, role.ID, qid, s.Catalog.Source.SaveIdentity(), en.Model)
}

func singleHuntMatch(en *Entry, run *dungeon.Session, entity uint16) bool {
	return en != nil && en.Model == SingleHuntEnemy && singleKillMatch(en, run, entity, en.HuntEnemy)
}

func singleKillMatch(en *Entry, run *dungeon.Session, entity uint16, target uint32) bool {
	if en == nil || !en.Implemented || target == 0 ||
		run == nil || !run.Loaded || !run.Dead[entity] ||
		en.ID != uint32(run.Maze.Quest) || run.Definition.ID != en.HuntDungeon {
		return false
	}
	for _, monster := range run.Monsters {
		if monster.Entity == entity && monster.Template == target {
			return true
		}
	}
	return false
}
