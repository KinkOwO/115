package charactercheck

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

func clearRewardCheck(ctx context.Context, s, reopened *storage.Store, role storage.Character, other int64) error {
	c, e := loadNativeProgressionCatalog()
	if e != nil {
		return e
	}
	prof, e := loadNativeCharacterCatalog()
	if e != nil {
		return e
	}
	rules, e := character.LoadGrowthRules("configs/experience.compat90.json")
	if e != nil {
		return e
	}
	d, e := loadNativeDungeonCatalog()
	if e != nil {
		return e
	}
	service := character.ProgressionService{Store: s, Catalog: c, Professions: prof, Rules: rules}
	run, e := dungeon.Select(d, protocol.DungeonSelection{ID: 3, Party: 65535, Quest: 3145}, 1, map[uint16]bool{3145: true})
	if e != nil {
		return e
	}
	now := run.StartedAt.Add(time.Minute)
	if _, _, _, e = service.Clear(ctx, role, run, 50, now); e == nil {
		return fmt.Errorf("unfinished dungeon rewarded")
	}
	path := [][2]byte{{1, 1}, {1, 0}, {2, 0}, {3, 0}}
	for i := 0; i <= len(path); i++ {
		run.Loaded = true
		for _, m := range run.Monsters {
			if _, e = run.ConfirmDeath(uint32(m.Entity), role.WireID, role.WireID); e != nil {
				return e
			}
		}
		if i < len(path) {
			run, e = run.Move(d, path[i])
			if e != nil {
				return e
			}
		}
	}
	target := run.Monsters[0].Entity
	if e = run.BossCheck(protocol.BossCheckRequest{Actor: role.WireID, Target: target}, role.WireID); e != nil {
		return e
	}
	foreign := role
	foreign.AccountID = other
	if _, _, _, e = service.Clear(ctx, foreign, run, 50, now); e == nil {
		return fmt.Errorf("clear reward crossed owner")
	}
	type result struct {
		r       storage.Character
		c       character.ClearReceipt
		applied bool
		e       error
	}
	results := make(chan result, 12)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, c, a, e := service.Clear(ctx, role, run, 50, now)
			results <- result{r, c, a, e}
		}()
	}
	wg.Wait()
	close(results)
	applications := 0
	for res := range results {
		if res.e != nil {
			return res.e
		}
		if res.applied {
			applications++
		}
		if res.c.Base != 163 || res.c.Score != 24 || res.c.Elapsed != 60000 || res.c.BestElapsed != 60000 || !res.c.NewRecord || !res.c.AllClear {
			return fmt.Errorf("clear receipt mismatch: %+v", res.c)
		}
		var state character.State
		if e = json.Unmarshal(res.r.State, &state); e != nil {
			return e
		}
		if state.Experience != 2490 || state.Level != 3 {
			return fmt.Errorf("duplicate clear experience: %d", state.Experience)
		}
	}
	if applications != 1 {
		return fmt.Errorf("clear applied %d times", applications)
	}
	if _, _, _, e = service.Clear(ctx, role, run, 60, now); e == nil {
		return fmt.Errorf("changed rank replay accepted")
	}
	service.Store = reopened
	saved, _, applied, e := service.Clear(ctx, role, run, 50, now.Add(time.Hour))
	if e != nil || applied {
		return fmt.Errorf("clear reopen replay: %v", e)
	}
	var state character.State
	if e = json.Unmarshal(saved.State, &state); e != nil {
		return e
	}
	if state.Experience != 2490 {
		return fmt.Errorf("clear persistence mismatch")
	}
	var recordState struct {
		Best map[string]uint32 `json:"dungeon_best_times"`
	}
	if e = json.Unmarshal(saved.State, &recordState); e != nil {
		return e
	}
	if recordState.Best["3:normal:solo"] != 60000 {
		return fmt.Errorf("clear best time not persisted")
	}
	fmt.Println("DUNGEON_CLEAR_STORAGE_PASS source_formula=true boss_gate=true concurrent_reward_once=true owner_checked=true rank_conflict_refused=true reopen=true")
	return nil
}
