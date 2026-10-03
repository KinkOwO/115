package charactercheck

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"dfolan/internal/workflow"
	"encoding/json"
	"fmt"
	"sync"
)

func lootCheck(ctx context.Context, s, reopened *storage.Store, role storage.Character, other int64) error {
	c, e := loadNativeLootCatalog()
	if e != nil {
		return e
	}
	rules, e := loot.LoadRules("configs/drop.compat90.json")
	if e != nil {
		return e
	}
	bagRules, e := inventory.LoadBagRules("configs/inventory.compat90.json", c.Source.Checksum)
	if e != nil {
		return e
	}
	tables, e := loot.Parse(c)
	if e != nil {
		return e
	}
	dc, e := loadNativeDungeonCatalog()
	if e != nil {
		return e
	}
	run, e := dungeon.Select(dc, protocol.DungeonSelection{ID: 3, Quest: 3145, Party: 65535}, 1, map[uint16]bool{3145: true})
	if e != nil {
		return e
	}
	run.Loaded = true
	session := loot.NewSession(c, tables, rules, nil, run.RunID, role.AccountID, role.ID, role.WireID)
	// Durable grant check uses one PVF level3 gold amount and an actual imported
	// throwable; source random selection is covered independently by loot tests.
	var item uint32
	for id, v := range c.Items {
		if v.StackableType == "[throw]" && (item == 0 || id < item) {
			item = id
		}
	}
	if item == 0 {
		return fmt.Errorf("missing source throwable")
	}
	for i, award := range []loot.Award{{Template: 0, Amount: uint32(tables.Gold[7])}, {Template: item, Amount: 1}} {
		object := uint32(32768 + i)
		session.Objects[object] = loot.Drop{Run: run.RunID, Map: run.Room.Map, Owner: role.WireID, Object: object, Slot: uint16(i + 1), Award: award}
	}
	domain := loot.Service{Catalog: c, Rules: rules, BagRules: bagRules, Tables: tables}
	service := workflow.LootService{Store: s, Loot: &domain}
	foreign := role
	foreign.AccountID = other
	if _, _, _, e = service.Pickup(ctx, foreign, session, run, protocol.PickupRequest{Object: 32768}); e == nil {
		return fmt.Errorf("pickup crossed owner")
	}
	changedRoom := *run
	changedRoom.Room.Map++
	if _, _, _, e = service.Pickup(ctx, role, session, &changedRoom, protocol.PickupRequest{Object: 32768}); e == nil {
		return fmt.Errorf("pickup crossed room")
	}
	if _, _, _, e = service.Pickup(ctx, role, session, run, protocol.PickupRequest{Object: 32768, DropX: 65535}); e == nil {
		return fmt.Errorf("distant pickup accepted")
	}
	type result struct {
		role    storage.Character
		receipt loot.PickupReceipt
		applied bool
		err     error
	}
	for _, object := range []uint32{32768, 32769} {
		results := make(chan result, 12)
		var wg sync.WaitGroup
		for i := 0; i < 12; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				r, receipt, a, e := service.Pickup(ctx, role, session, run, protocol.PickupRequest{Object: object})
				results <- result{r, receipt, a, e}
			}()
		}
		wg.Wait()
		close(results)
		count := 0
		for res := range results {
			if res.err != nil {
				return res.err
			}
			if res.applied {
				count++
			}
			b, e := inventory.ReadBag(res.role.State)
			if e != nil {
				return e
			}
			if b.Gold != 34 {
				return fmt.Errorf("gold grant replay: %d", b.Gold)
			}
			if object == 32769 && (len(b.Items) != 1 || b.Items[0].Amount != 1 || b.Items[0].Template != item) {
				return fmt.Errorf("duplicate stackable grant")
			}
			var state character.State
			if e = json.Unmarshal(res.role.State, &state); e != nil {
				return e
			}
			if state.Experience != 2490 || state.Level != 3 {
				return fmt.Errorf("inventory overwrote experience")
			}
		}
		if count != 1 {
			return fmt.Errorf("pickup applied %d times", count)
		}
	}
	service.Store = reopened
	saved, _, applied, e := service.Pickup(ctx, role, session, run, protocol.PickupRequest{Object: 32769})
	if e != nil || applied {
		return fmt.Errorf("pickup reopen replay: %v", e)
	}
	items := inventory.ItemService{Catalog: domain.Catalog}
	p, e := items.Bootstrap(workflow.InventoryRole(saved))
	if e != nil || len(p) != 367 {
		return fmt.Errorf("reopened bag wire: %d %v", len(p), e)
	}
	fmt.Println("LOOT_STORAGE_PASS gold_and_source_stackable=true concurrent_pickup_once=true owner_and_room_checked=true experience_preserved=true reopen=true")
	return nil
}
