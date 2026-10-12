package quest

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"fmt"
	"time"
)

type townArrivalCinematicObjectiveStore interface {
	CompleteTownArrivalCinematic(context.Context, int64, int64, uint16, string, string, time.Time) (bool, error)
}

type townArrivalCinematicMigrator interface {
	MigrateTownArrivalCinematic(context.Context, int64, int64, uint16, string, string, string, uint32, []uint32) error
}

// The old client-gated model started at zero before any scene was viewed.
// Preserve zero only when the same accepted lifetime has a committed clear
// receipt for this source dungeon and a map-clear receipt from that same run.
func (s *Service) refreshTownArrivalCinematics(ctx context.Context, role character.Character) error {
	migrator, ok := s.Store.(townArrivalCinematicMigrator)
	if !ok {
		return nil
	}
	states, err := s.Store.Quests(ctx, role.AccountID, role.ID)
	if err != nil {
		return err
	}
	for _, q := range states {
		if q.Status != "accepted" || q.ProgressModel != LookCinematic || q.ConfigVersion != s.Catalog.Source.SaveIdentity() {
			continue
		}
		d, err := s.Catalog.Definition(uint32(q.ID))
		if err != nil {
			continue
		}
		_, model, err := InitialProgress(d)
		if err != nil || model != TownArrivalCinematic {
			continue
		}
		scene, _ := catalog.QuestTownArrivalScene(d)
		var maps []uint32
		if s.Dungeons != nil {
			if dungeon, found := s.Dungeons.Dungeons[scene.DungeonID]; found {
				for _, maze := range dungeon.Mazes {
					if maze.Quest == q.ID {
						for _, room := range maze.Rooms {
							maps = append(maps, room.Map)
						}
					}
				}
			}
		}
		if err := migrator.MigrateTownArrivalCinematic(ctx, role.AccountID, role.ID, q.ID, q.ConfigVersion, LookCinematic, model, scene.DungeonID, maps); err != nil {
			return err
		}
	}
	return nil
}

// The caller verifies the imported source area and dungeon. The quest owner
// checks accepted state and records the loaded quest maze's CMD33 evidence.
func (s *Service) TownArrivalCinematicTrigger(ctx context.Context, role character.Character, run *dungeon.Session, id uint16) error {
	d, err := s.Catalog.Definition(uint32(id))
	if err != nil {
		return err
	}
	scene, ok := catalog.QuestTownArrivalScene(d)
	_, model, err := InitialProgress(d)
	if err != nil || !ok || model != TownArrivalCinematic || run == nil || !run.Loaded || run.Definition.ID != scene.DungeonID || run.Maze.Quest != id {
		return fmt.Errorf("cinematic confirmation requires owned loaded arrival quest maze")
	}
	owner, available := s.Store.(townArrivalCinematicObjectiveStore)
	if !available {
		return fmt.Errorf("arrival cinematic objective store unavailable")
	}
	active, err := s.Active(ctx, role)
	if err != nil {
		return err
	}
	for _, q := range active {
		if q.ID != id {
			continue
		}
		if _, err := owner.CompleteTownArrivalCinematic(ctx, role.AccountID, role.ID, id, s.Catalog.Source.SaveIdentity(), model, run.StartedAt); err != nil {
			return err
		}
		active, err = s.Active(ctx, role)
		if err != nil {
			return err
		}
		for _, saved := range active {
			if saved.ID == id && saved.Progress == 0 {
				return nil
			}
		}
		return fmt.Errorf("arrival cinematic confirmation did not commit objective")
	}
	return fmt.Errorf("arrival cinematic quest is not accepted")
}
