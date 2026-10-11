package workflow

import (
	"context"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"testing"
)

func TestPreparedLegionAwardsFreezeFullBagRecoverAndReplay(t *testing.T) {
	s, store, _ := legionRewardFixture()
	ctx := context.Background()
	draws := 0
	draw := func() ([]loot.Award, []loot.Award, error) {
		draws++
		return []loot.Award{{Template: 21, Amount: 8}}, []loot.Award{{Template: 22, Amount: 3}}, nil
	}
	role, frozen, err := s.FreezePrepared(ctx, store.role, "forest:run", "ForestOfAwakeningNormal", 100, draw)
	if err != nil {
		t.Fatal(err)
	}
	bag, _ := inventory.ReadBag(store.role.State)
	bag.Items = []inventory.BagItem{{Slot: 68, Template: 21, Amount: 100}, {Slot: 69, Template: 22, Amount: 100}}
	store.role.State, _ = inventory.SaveBag(store.role.State, bag)
	before := string(store.role.State)
	if _, _, err := s.Claim(ctx, role, frozen.Run); err == nil {
		t.Fatal("partial full-bag grant committed")
	}
	if string(store.role.State) != before || len(store.receipts) != 1 {
		t.Fatal("failed grant lost pending rewards")
	}
	// New handler instance and a different proposed roll must restore the receipt.
	_, again, err := s.FreezePrepared(ctx, store.role, "forest:run", "ForestOfAwakeningNormal", 100, func() ([]loot.Award, []loot.Award, error) { t.Fatal("frozen draw repeated"); return nil, nil, nil })
	if err != nil || draws != 1 || again.Basic[0].Amount != 8 {
		t.Fatal("freeze replay", again, err)
	}
	bag.Items = nil
	store.role.State, _ = inventory.SaveBag(store.role.State, bag)
	role, err = s.Recover(ctx, store.role)
	if err != nil {
		t.Fatal(err)
	}
	bag, _ = inventory.ReadBag(role.State)
	if len(bag.Items) != 2 || bag.Items[0].Amount != 8 || bag.Items[1].Amount != 3 {
		t.Fatal("recovery products", bag)
	}
	before = string(role.State)
	role, _, err = s.Claim(ctx, role, frozen.Run)
	if err != nil || string(role.State) != before {
		t.Fatal("replay granted twice", err)
	}
}
