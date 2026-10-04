package charactercheck

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"encoding/json"
	"fmt"
	"sync"
)

func wearCheck(ctx context.Context, s *database.TestFixture, reopened *database.Store, account, foreign int64) error {
	c, e := loadNativeCharacterCatalog()
	if e != nil {
		return e
	}
	cs, e := character.New(s, c, character.Rules{MaxCharacters: 30, InitialLevel: 1})
	if e != nil {
		return e
	}
	role, e := cs.Create(ctx, account, request("WearTest"))
	if e != nil {
		return e
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(role.State, &fields); e != nil {
		return e
	}
	fields["level"] = json.RawMessage("6")
	fields["unrelated_wear_test"] = json.RawMessage("true")
	raw, e := json.Marshal(fields)
	if e != nil {
		return e
	}
	role.State, e = inventory.SaveBag(raw, inventory.Bag{Version: "ordinary-bag-v1", Gold: 345, Equipment: []inventory.BagEquipment{{Slot: 9, Template: 20002}, {Slot: 10, Template: 24002}}})
	if e != nil {
		return e
	}
	if e = s.SeedCharacterState(ctx, role.ID, role.State); e != nil {
		return e
	}
	eq, e := loadNativeEquipmentCatalog(c.Source.Checksum)
	if e != nil {
		return e
	}
	rules, e := inventory.LoadWearRules("configs/equipment-wear.current35.json", c.Source.Checksum)
	if e != nil {
		return e
	}
	bagRules, e := inventory.LoadBagRules("configs/inventory.next29.json", c.Source.Checksum)
	if e != nil {
		return e
	}
	service := workflow.WearService{Store: s.Storage(), WearService: inventory.WearService{Catalog: eq, Professions: c, BagRules: bagRules, Rules: rules}}
	r := protocol.ItemMoveRequest{SourceSlot: 9, SourceItem: 20002, DestinationList: 3, DestinationSlot: 19, Count: 1, Selection: 0xffffffff}
	wrongOwner := role
	wrongOwner.AccountID = foreign
	if _, _, e = service.Move(ctx, wrongOwner, "wear:foreign", r); e == nil {
		return fmt.Errorf("wear crossed character owner")
	}
	type result struct {
		role    database.Character
		applied bool
		e       error
	}
	results := make(chan result, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, a, e := service.Move(ctx, role, "wear:once", r)
			results <- result{v, a, e}
		}()
	}
	wg.Wait()
	close(results)
	count := 0
	for v := range results {
		if v.e != nil {
			return v.e
		}
		if v.applied {
			count++
		}
		b, e := inventory.ReadBag(v.role.State)
		if e != nil || len(b.Worn) != 1 || len(b.Equipment) != 1 || b.Worn[0].Template != 20002 || b.Gold != 345 {
			return fmt.Errorf("wear duplicated or lost assets: %v", e)
		}
	}
	if count != 1 {
		return fmt.Errorf("wear applied %d times", count)
	}
	service.Store = reopened
	saved, applied, e := service.Move(ctx, role, "wear:once", r)
	if e != nil || applied {
		return fmt.Errorf("wear reopen replay: %v", e)
	}
	// Removing an item through the empty backpack source preserves both rows.
	r.SourceItem = 0
	r.DestinationItem = 20002
	saved, applied, e = service.Move(ctx, saved, "wear:remove", r)
	if e != nil || !applied {
		return fmt.Errorf("unequip: %v", e)
	}
	b, e := inventory.ReadBag(saved.State)
	if e != nil || len(b.Worn) != 0 || len(b.Equipment) != 2 || b.Gold != 345 {
		return fmt.Errorf("unequip asset mismatch: %v", e)
	}
	json.Unmarshal(saved.State, &fields)
	if string(fields["unrelated_wear_test"]) != "true" {
		return fmt.Errorf("wear lost another module")
	}
	r.SourceItem = 20002
	r.DestinationItem = 0
	r.DestinationSlot = 20
	if _, _, e = service.Move(ctx, saved, "wear:wrong-slot", r); e == nil {
		return fmt.Errorf("wrong equipment slot accepted")
	}
	fmt.Println("WEAR_STORAGE_PASS equip_unequip=true source_slot_and_level=true concurrent_once=true owner_checked=true reopen=true gold_and_other_modules_preserved=true")
	return nil
}
