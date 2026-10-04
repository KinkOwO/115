package charactercheck

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/quest"
	"dfolan/internal/workflow"
	"encoding/json"
	"fmt"
	"sync"
)

func questRewardCheck(ctx context.Context, s *database.TestFixture, reopened *database.Store, role database.Character, other int64) error {
	if e := s.MigrateQuestRewards(ctx); e != nil {
		return e
	}
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
	q, e := loadNativeQuestCatalog()
	if e != nil {
		return e
	}
	p := &character.ProgressionService{Store: s.Storage(), Catalog: c, Professions: prof, Rules: rules}
	service := quest.Service{Store: s.Storage(), Catalog: q, Professions: prof, Progression: p}
	req := protocol.QuestSubmitRequest{ID: 3145, RewardSelection: 65535, Option: 1}
	foreign := role
	foreign.AccountID = other
	if _, e = (&workflow.QuestService{Store: s.Storage(), Quest: &service}).Finish(ctx, foreign, req); e == nil {
		return fmt.Errorf("quest reward crossed account")
	}
	// Keep every mutation inside the established throwaway test schema.
	if e = s.SeedQuestProgress(ctx, role.ID, 3145, 1); e != nil {
		return e
	}
	if _, e = (&workflow.QuestService{Store: s.Storage(), Quest: &service}).Finish(ctx, role, req); e == nil {
		return fmt.Errorf("unfinished objective rewarded")
	}
	if e = s.SeedQuestProgress(ctx, role.ID, 3145, 0); e != nil {
		return e
	}
	rows, e := s.Characters(ctx, role.AccountID)
	if e != nil || len(rows) != 1 {
		return fmt.Errorf("quest test role missing: %v", e)
	}
	saved := rows[0].State
	var state character.State
	if e = json.Unmarshal(saved, &state); e != nil {
		return e
	}
	state.Advancement = 1
	advanced, e := json.Marshal(state)
	if e != nil {
		return e
	}
	if e = s.SeedCharacterState(ctx, role.ID, advanced); e != nil {
		return e
	}
	if _, e = (&workflow.QuestService{Store: s.Storage(), Quest: &service}).Finish(ctx, role, req); e == nil {
		return fmt.Errorf("matching asset rewards silently discarded")
	}
	if e = s.SeedCharacterState(ctx, role.ID, saved); e != nil {
		return e
	}
	// Completion now also credits source-defined wallet gold. Keep the nil
	// awarder refusal above, then configure the real inventory for settlement.
	lc, e := loadNativeLootCatalog()
	if e != nil {
		return e
	}
	br, e := inventory.LoadBagRules("configs/inventory.next29.json", lc.Source.Checksum)
	if e != nil {
		return e
	}
	service.Inventory = &inventory.Awarder{Catalog: lc, Rules: br}
	beforeBag, e := inventory.ReadBag(saved)
	if e != nil {
		return e
	}
	d, e := q.Definition(uint32(req.ID))
	if e != nil {
		return e
	}
	expectedGold, e := character.GrowthQuestGold(c, d, state.Level)
	if e != nil {
		return e
	}
	type result struct {
		r quest.FinishResult
		e error
	}
	results := make(chan result, 12)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := (&workflow.QuestService{Store: s.Storage(), Quest: &service}).Finish(ctx, role, req)
			results <- result{r, e}
		}()
	}
	wg.Wait()
	close(results)
	applied := 0
	for res := range results {
		if res.e != nil {
			return res.e
		}
		if res.r.Applied {
			applied++
		}
		if res.r.Receipt.Experience != 1200 {
			return fmt.Errorf("quest source experience mismatch")
		}
		bag, err := inventory.ReadBag(res.r.Role.State)
		if err != nil || res.r.Receipt.Gold != expectedGold || uint64(bag.Gold) != uint64(beforeBag.Gold)+uint64(expectedGold) || len(res.r.Receipt.Items) != 0 {
			return fmt.Errorf("quest source gold/job filter mismatch: gold=%d receipt=%d: %v", bag.Gold, res.r.Receipt.Gold, err)
		}
		if e = json.Unmarshal(res.r.Role.State, &state); e != nil {
			return e
		}
		if state.Experience != 2303 || state.Level != 3 || state.SkillPoints != [2]uint16{60, 60} {
			return fmt.Errorf("quest retry state mismatch exp=%d level=%d SP=%v", state.Experience, state.Level, state.SkillPoints)
		}
	}
	if applied != 1 {
		return fmt.Errorf("quest duplicate reward applications=%d", applied)
	}
	var count int64
	if count, e = s.QuestRewardCount(ctx, role.ID, 0); e != nil || count != 1 {
		return fmt.Errorf("quest receipts=%d: %v", count, e)
	}
	service.Store = reopened
	p.Store = reopened
	rows, e = reopened.Characters(ctx, role.AccountID)
	if e != nil || len(rows) != 1 {
		return fmt.Errorf("reopened quest role missing: %v", e)
	}
	if e = json.Unmarshal(rows[0].State, &state); e != nil {
		return e
	}
	if state.Experience != 2303 || state.Level != 3 {
		return fmt.Errorf("quest reward reopen failed")
	}
	active, e := service.Active(ctx, role)
	if e != nil {
		return e
	}
	for _, a := range active {
		if a.ID == 3145 {
			return fmt.Errorf("completed quest remained active")
		}
	}
	completed, e := service.Completed(ctx, role)
	if e != nil {
		return e
	}
	completedTarget := 0
	for _, id := range completed {
		if id == 3145 {
			completedTarget++
		}
	}
	if completedTarget != 1 {
		return fmt.Errorf("completed quest bitmap mismatch: target occurrences=%d", completedTarget)
	}
	if _, e = service.Accept(ctx, role, 3145); e == nil {
		return fmt.Errorf("completed quest accepted twice")
	}
	fmt.Println("QUEST_REWARD_STORAGE_PASS concurrent_reward_once=true atomic_completion_exp=true source_job_filter=true unfinished_refused=true owner_checked=true reopen=true")
	return nil
}
