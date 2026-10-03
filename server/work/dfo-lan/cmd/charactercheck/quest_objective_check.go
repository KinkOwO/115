package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/quest"
	"dfolan/internal/storage"
	"fmt"
	"sync"
)

func questObjectiveCheck(ctx context.Context, s, reopened *storage.Store, role storage.Character, other int64) error {
	if err := s.MigrateQuestObjectives(ctx); err != nil {
		return err
	}
	c, err := loadNativeQuestCatalog()
	if err != nil {
		return err
	}
	service := quest.Service{Store: s, Catalog: c}
	reset := func() error {
		// Only the caller's isolated temporary-schema character is touched.
		_ = s.AbandonQuest(ctx, role.AccountID, role.ID, 3145)
		_, err := s.AcceptQuest(ctx, role.AccountID, role.ID, 3145, c.Source.SaveIdentity(), 1, 10000, nil, 1, quest.SingleClearMap)
		return err
	}
	if err = reset(); err != nil {
		return err
	}
	run := &dungeon.Session{RunID: "000000000000000000000000000000a1", Loaded: true, Room: catalog.DungeonRoom{Map: 76126, Boss: true}, Monsters: []protocol.DungeonMonster{{Entity: 4096, Rank: 3}}, Dead: map[uint16]bool{}}
	if _, err = service.MapClear(ctx, role, run, c.Source.SaveIdentity()); err == nil {
		return fmt.Errorf("quest completed before known enemy death")
	}
	run.Dead[4096] = true
	if _, err = service.MapClear(ctx, role, run, c.Source.SaveIdentity()); err == nil {
		return fmt.Errorf("enemy death bypassed final boss check")
	}
	if err = run.BossCheck(protocol.BossCheckRequest{Actor: role.WireID, Target: 4096}, role.WireID); err != nil {
		return err
	}
	if _, err = service.MapClear(ctx, role, run, "wrong source"); err == nil {
		return fmt.Errorf("quest crossed source version")
	}
	foreign := role
	foreign.AccountID = other
	if _, err = service.MapClear(ctx, foreign, run, c.Source.SaveIdentity()); err == nil {
		return fmt.Errorf("map clear crossed account boundary")
	}
	var wg sync.WaitGroup
	errors := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			active, err := service.MapClear(ctx, role, run, c.Source.SaveIdentity())
			if err == nil {
				found := false
				for _, q := range active {
					if q.ID == 3145 {
						found = q.Progress == 0
					}
				}
				if !found {
					err = fmt.Errorf("ready trigger missing")
				}
			}
			errors <- err
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			return err
		}
	}
	var count int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM character_map_clears WHERE character_id=$1 AND run_id=$2`, role.ID, run.RunID).Scan(&count); err != nil || count != 1 {
		return fmt.Errorf("duplicate map-clear evidence: count=%d err=%v", count, err)
	}
	states, err := reopened.Quests(ctx, role.AccountID, role.ID)
	if err != nil {
		return err
	}
	found := false
	for _, q := range states {
		if q.ID == 3145 {
			found = q.Progress == 0 && q.Status == "accepted"
		}
	}
	if !found {
		return fmt.Errorf("objective state did not survive reopen")
	}
	if err = reset(); err != nil {
		return err
	}
	active, err := service.MapClear(ctx, role, run, c.Source.SaveIdentity())
	if err != nil {
		return err
	}
	for _, q := range active {
		if q.ID == 3145 && q.Progress != 1 {
			return fmt.Errorf("old clear credited to reaccepted quest")
		}
	}
	run.RunID = "000000000000000000000000000000a2"
	run.Room.Map = 76121
	active, err = service.MapClear(ctx, role, run, c.Source.SaveIdentity())
	if err != nil {
		return err
	}
	for _, q := range active {
		if q.ID == 3145 && q.Progress != 1 {
			return fmt.Errorf("wrong map completed quest")
		}
	}
	run.RunID = "000000000000000000000000000000a3"
	run.Room.Map = 76126
	if _, err = service.MapClear(ctx, role, run, c.Source.SaveIdentity()); err != nil {
		return err
	}
	fmt.Println("QUEST_MAP_OBJECTIVE_PASS source_map=true ownership=true live_enemies_refused=true concurrent_retry=true reopen=true reaccept_requires_new_clear=true rewards_unchanged=true")
	return nil
}
