package charactercheck

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"encoding/json"
	"fmt"
	"sync"
)

func cardCheck(ctx context.Context, s *database.TestFixture, reopened *database.Store, role database.Character, other int64) error {
	storedState, e := s.CharacterState(ctx, role.ID)
	if e != nil {
		return e
	}
	beforeBag, e := inventory.ReadBag(storedState)
	if e != nil {
		return e
	}
	role.State = storedState
	c, e := loadNativeLootCatalog()
	if e != nil {
		return e
	}
	tables, e := loot.Parse(c)
	if e != nil {
		return e
	}
	bag, e := inventory.LoadBagRules("configs/inventory.compat90.json", c.Source.Checksum)
	if e != nil {
		return e
	}
	policy, e := loot.LoadCardRules("configs/cards.compat90.json")
	if e != nil {
		return e
	}
	equipment, e := loadNativeEquipmentCatalog(c.Source.Checksum)
	if e != nil {
		return e
	}
	domain := loot.Service{Catalog: c, Tables: tables, BagRules: bag, CardPolicy: &policy, Equipment: equipment}
	service := workflow.LootService{Store: s.Storage(), Loot: &domain}
	dc, e := loadNativeDungeonCatalog()
	if e != nil {
		return e
	}
	run, e := dungeon.Select(dc, protocol.DungeonSelection{ID: 3, Quest: 3145, Party: 65535}, 1, map[uint16]bool{3145: true})
	if e != nil {
		return e
	}
	if _, e = service.FreezeCards(ctx, role, run, policy, 123); e == nil {
		return fmt.Errorf("unfinished card plan accepted")
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
			run, e = run.Move(dc, path[i])
			if e != nil {
				return e
			}
		}
	}
	if e = run.BossCheck(protocol.BossCheckRequest{Actor: role.WireID, Target: run.Monsters[0].Entity}, role.WireID); e != nil {
		return e
	}
	p, e := service.FreezeCards(ctx, role, run, policy, 123)
	if e != nil {
		return e
	}
	expectedEquipment := map[uint32]int{}
	for _, item := range beforeBag.Equipment {
		expectedEquipment[item.Template]++
	}
	for _, item := range p.Items {
		if item != (loot.Award{}) {
			expectedEquipment[item.Template] += int(item.Amount)
		}
	}
	if p2, e := service.FreezeCards(ctx, role, run, policy, 999999); e != nil || p2 != p {
		return fmt.Errorf("card plan rerolled: %v", e)
	}
	foreign := role
	foreign.AccountID = other
	if _, _, _, e = service.PickCard(ctx, foreign, run, p, 0); e == nil {
		return fmt.Errorf("card crossed owner")
	}
	changed := p
	changed.Gold++
	if _, _, _, e = service.PickCard(ctx, role, run, changed, 0); e == nil {
		return fmt.Errorf("forged card amount")
	}
	if _, _, _, e = service.PickCard(ctx, role, run, p, 4); e == nil {
		return fmt.Errorf("invalid card index")
	}
	type result struct {
		role    database.Character
		receipt loot.CardReceipt
		applied bool
		err     error
	}
	ch := make(chan result, 12)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(index byte) {
			defer wg.Done()
			r, c, a, e := service.PickCard(ctx, role, run, p, index)
			ch <- result{r, c, a, e}
		}(byte(i % 4))
	}
	wg.Wait()
	close(ch)
	applied := 0
	index := byte(255)
	for r := range ch {
		if r.err != nil {
			return r.err
		}
		if r.applied {
			applied++
		}
		if index == 255 {
			index = r.receipt.Index
		}
		if r.receipt.Index != index {
			return fmt.Errorf("card choice changed")
		}
		b, e := inventory.ReadBag(r.role.State)
		if e != nil {
			return e
		}
		var state character.State
		if e = json.Unmarshal(r.role.State, &state); e != nil {
			return e
		}
		if uint64(b.Gold) != uint64(beforeBag.Gold)+uint64(p.Gold) || state.Experience != 2490 || len(b.Items) != len(beforeBag.Items) {
			return fmt.Errorf("card lost or duplicated inventory/EXP")
		}
		actualEquipment := map[uint32]int{}
		for _, item := range b.Equipment {
			actualEquipment[item.Template]++
		}
		if len(actualEquipment) != len(expectedEquipment) {
			return fmt.Errorf("card equipment lost or duplicated")
		}
		for id, count := range expectedEquipment {
			if actualEquipment[id] != count {
				return fmt.Errorf("card equipment %d count=%d, want=%d", id, actualEquipment[id], count)
			}
		}
	}
	if applied != 1 {
		return fmt.Errorf("card applied %d times", applied)
	}
	service.Store = reopened
	if _, c, a, e := service.PickCard(ctx, role, run, p, (index+1)%4); e != nil || a || c.Index != index {
		return fmt.Errorf("card reopen replay: %v", e)
	}
	fmt.Println("CARD_STORAGE_PASS frozen_plan=true concurrent_choice_once=true changed_choice_returns_receipt=true owner_checked=true forged_amount_refused=true inventory_and_exp_preserved=true reopen=true")
	return nil
}
