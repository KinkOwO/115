package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/progression"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"sync"
)

func progressionCheck(ctx context.Context, s, reopened *storage.Store, role storage.Character, other int64) error {
	if e := s.MigrateCharacterEvents(ctx); e != nil {
		return e
	}
	if e := s.MigrateCharacterNotices(ctx); e != nil {
		return e
	}
	c, e := catalog.LoadProgression("configs/progression.next25.json")
	if e != nil {
		return e
	}
	prof, e := catalog.LoadCharacters("configs/characters.next25.json")
	if e != nil {
		return e
	}
	rules, e := progression.LoadRules("configs/experience.compat90.json")
	if e != nil {
		return e
	}
	d, e := catalog.LoadDungeons("configs/dungeons.generated.json")
	if e != nil {
		return e
	}
	service := character.ProgressionService{Store: s, Catalog: c, Professions: prof, Rules: rules}
	run, e := dungeon.Select(d, protocol.DungeonSelection{ID: 3, Party: 65535, Quest: 3145}, 1, map[uint16]bool{3145: true})
	if e != nil {
		return e
	}
	run.Loaded = true
	m := run.Monsters[0]
	if _, _, e = service.Monster(ctx, role, run, m.Entity); e == nil {
		return fmt.Errorf("rewarded living monster")
	}
	_, e = run.ConfirmDeath(uint32(m.Entity), role.WireID, role.WireID)
	if e != nil {
		return e
	}
	wrong := role
	wrong.AccountID = other
	if _, _, e = service.Monster(ctx, wrong, run, m.Entity); e == nil {
		return fmt.Errorf("experience crossed character owner")
	}
	var before character.State
	if e = json.Unmarshal(role.State, &before); e != nil {
		return e
	}
	before.Experience = 999
	before.Level = 1
	encoded, e := json.Marshal(before)
	if e != nil {
		return e
	}
	// This role belongs to the preverified temporary schema, never live data.
	if _, e = s.DB.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, role.ID, encoded); e != nil {
		return e
	}
	type result struct {
		role    storage.Character
		applied bool
		err     error
	}
	results := make(chan result, 12)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, b, e := service.Monster(ctx, role, run, m.Entity)
			results <- result{a, b, e}
		}()
	}
	wg.Wait()
	close(results)
	applied := 0
	for r := range results {
		if r.err != nil {
			return r.err
		}
		if r.applied {
			applied++
		}
		var state character.State
		if e = json.Unmarshal(r.role.State, &state); e != nil {
			return e
		}
		if state.Experience != 1103 || state.Level != 2 || state.SkillPoints != [2]uint16{30, 30} {
			return fmt.Errorf("experience/SP retry mismatch: exp=%d level=%d SP=%v", state.Experience, state.Level, state.SkillPoints)
		}
		if state.Attributes["[hp max]"] != before.Attributes["[hp max]"]+prof.Professions[role.Profession].BaseGrowth["[hp max]"] {
			return fmt.Errorf("level growth missing")
		}
	}
	if applied != 1 {
		return fmt.Errorf("duplicate monster rewards: %d", applied)
	}
	roles, e := reopened.Characters(ctx, role.AccountID)
	if e != nil {
		return e
	}
	var saved character.State
	if len(roles) != 1 {
		return fmt.Errorf("isolated character count changed")
	}
	if e = json.Unmarshal(roles[0].State, &saved); e != nil {
		return e
	}
	if saved.Experience != 1103 || saved.Level != 2 {
		return fmt.Errorf("experience reopen failed")
	}
	// A refused domain operation must leave both receipt and character intact.
	_, _, e = s.CommitCharacterEvent(ctx, role.AccountID, role.ID, c.Source.SaveIdentity(), "rejected-test", rules.Model, func(storage.Character) (json.RawMessage, json.RawMessage, error) {
		return nil, nil, fmt.Errorf("intentional rollback")
	})
	if e == nil {
		return fmt.Errorf("domain failure committed")
	}
	var receipts int
	if e = s.DB.QueryRow(ctx, `SELECT count(*) FROM character_events WHERE character_id=$1`, role.ID).Scan(&receipts); e != nil || receipts != 1 {
		return fmt.Errorf("reward receipts=%d error=%v", receipts, e)
	}
	fmt.Println("PROGRESSION_STORAGE_PASS concurrent_reward_once=true atomic_exp_level_sp=true source_growth=true reopen=true owner_checked=true rollback=true")
	return nil
}
