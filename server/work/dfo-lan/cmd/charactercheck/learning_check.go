package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"sync"
)

func learningCheck(ctx context.Context, s, reopened *storage.Store, other int64) error {
	c, e := catalog.LoadCharacters("configs/characters.next25.json")
	if e != nil {
		return e
	}
	cs, e := character.New(s, c, character.Rules{MaxCharacters: 2, InitialLevel: 1})
	if e != nil {
		return e
	}
	cs.Learning, e = character.LoadLearningCatalog("configs/skills.next27.json", c.Source.Checksum)
	if e != nil {
		return e
	}
	account, e := s.DevelopmentAccount(ctx, "temporary-learning")
	if e != nil {
		return e
	}
	role, e := cs.Create(ctx, account, request("SkillTest"))
	if e != nil {
		return e
	}
	var state character.State
	if e = json.Unmarshal(role.State, &state); e != nil {
		return e
	}
	state.Level = 5
	state.SkillPoints = [2]uint16{100, 100}
	raw, e := json.Marshal(state)
	if e != nil {
		return e
	}
	var doc map[string]json.RawMessage
	json.Unmarshal(raw, &doc)
	doc["unrelated_module"] = json.RawMessage(`{"kept":true}`)
	raw, e = json.Marshal(doc)
	if e != nil {
		return e
	}
	role.State = raw
	if _, e = s.DB.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, role.ID, raw); e != nil {
		return e
	}
	buy := protocol.SkillPurchase{Entries: []protocol.SkillPurchaseEntry{{ID: 46, Delta: 1}}}
	wrong := role
	wrong.AccountID = other
	if _, _, e = cs.Learn(ctx, wrong, "skill:wrong", buy); e == nil {
		return fmt.Errorf("learning crossed account")
	}
	var wg sync.WaitGroup
	results := make(chan error, 8)
	applied := make(chan bool, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, a, e := cs.Learn(ctx, role, "skill:once", buy); results <- e; applied <- a }()
	}
	wg.Wait()
	close(results)
	close(applied)
	for e := range results {
		if e != nil {
			return e
		}
	}
	n := 0
	for a := range applied {
		if a {
			n++
		}
	}
	if n != 1 {
		return fmt.Errorf("duplicate SP consumption: %d", n)
	}
	if _, _, e = cs.Learn(ctx, role, "skill:overlevel", buy); e == nil {
		return fmt.Errorf("rank3 below required level accepted")
	}
	if _, _, e = cs.Learn(ctx, role, "skill:othergrowth", protocol.SkillPurchase{Entries: []protocol.SkillPurchaseEntry{{ID: 37, Delta: 1}}}); e == nil {
		return fmt.Errorf("other growth skill accepted")
	}
	learned, _, e := cs.Learn(ctx, role, "skill:guard", protocol.SkillPurchase{Entries: []protocol.SkillPurchaseEntry{{ID: 1, Delta: 1}}})
	if e != nil {
		return e
	}
	var newlyLearned character.State
	if e = json.Unmarshal(learned.State, &newlyLearned); e != nil {
		return e
	}
	if slot, exists := newlyLearned.SkillSlots[0][1]; !exists || slot >= 14 {
		return fmt.Errorf("new active skill was not bound to a free shortcut")
	}
	moved, _, e := cs.MoveSkill(ctx, learned, "skill:drag", protocol.SkillMove{From: 1, To: 5})
	if e != nil {
		return e
	}
	if _, _, e = cs.MoveSkill(ctx, moved, "skill:drag", protocol.SkillMove{From: 1, To: 5}); e != nil {
		return e
	}
	cs.Store = reopened
	rows, e := reopened.Characters(ctx, account)
	if e != nil || len(rows) != 1 {
		return fmt.Errorf("skill reopen: %v", e)
	}
	if e = json.Unmarshal(rows[0].State, &state); e != nil {
		return e
	}
	if state.SkillPoints != [2]uint16{60, 100} || state.LearnedSkills[0][46] != 2 || state.LearnedSkills[0][1] != 1 || state.SkillSlots[0][46] != 5 {
		return fmt.Errorf("skill state mismatch: %+v", state)
	}
	if state.SkillSlots[0][1] != newlyLearned.SkillSlots[0][1] {
		return fmt.Errorf("new skill shortcut lost after reopen")
	}
	json.Unmarshal(rows[0].State, &doc)
	if string(doc["unrelated_module"]) != "{\"kept\": true}" && string(doc["unrelated_module"]) != "{\"kept\":true}" {
		return fmt.Errorf("learning lost other modules")
	}
	if _, e = cs.EntrySkills(rows[0]); e != nil {
		return e
	}
	if _, e = cs.LearningResponse(rows[0], buy); e != nil {
		return e
	}
	refund := protocol.SkillPurchase{Entries: []protocol.SkillPurchaseEntry{{ID: 1, Refund: 1, Delta: 1}}}
	refunded, first, e := cs.Learn(ctx, rows[0], "skill:refund-once", refund)
	if e != nil || !first {
		return fmt.Errorf("skill refund failed: %v", e)
	}
	refunded, again, e := cs.Learn(ctx, refunded, "skill:refund-once", refund)
	if e != nil || again {
		return fmt.Errorf("skill refund replay failed: %v", e)
	}
	state = character.State{}
	if e = json.Unmarshal(refunded.State, &state); e != nil {
		return e
	}
	if state.SkillPoints[0] != 80 || state.LearnedSkills[0][1] != 0 || state.SkillSlots[0][46] != 5 {
		return fmt.Errorf("refund duplicated SP or lost layout")
	}
	if _, e = cs.LearningResponse(refunded, refund); e != nil {
		return e
	}
	if _, _, e = cs.Learn(ctx, refunded, "skill:refund-empty", refund); e == nil {
		return fmt.Errorf("absent skill minted SP")
	}
	if _, _, e = cs.Learn(ctx, refunded, "skill:refund-initial", protocol.SkillPurchase{Entries: []protocol.SkillPurchaseEntry{{ID: 46, Refund: 1, Delta: 2}}}); e == nil {
		return fmt.Errorf("initial rank minted SP")
	}
	back, e := reopened.Characters(ctx, account)
	if e != nil || len(back) != 1 {
		return fmt.Errorf("refund reconnect: %v", e)
	}
	state = character.State{}
	if e = json.Unmarshal(back[0].State, &state); e != nil {
		return e
	}
	if state.SkillPoints[0] != 80 || state.LearnedSkills[0][1] != 0 {
		return fmt.Errorf("refund reconnect state changed")
	}
	fmt.Println("SKILL_REFUND_PASS saved_points=80 retry_once=true empty_refused=true initial_floor=true layout_preserved=true reconnect=true")
	fmt.Println("SKILL_STORAGE_PASS source_gates=true points=60/100 duplicate_atomic=true layout_reopen=true modules_preserved=true")
	return nil
}
