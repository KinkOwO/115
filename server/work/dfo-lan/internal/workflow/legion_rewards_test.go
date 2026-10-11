package workflow

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func legionRewardFixture() (*LegionRewardService, *bakalRewardStoreFake, catalog.LegionPhaseRewards) {
	hash := strings.Repeat("a", 64)
	store := &bakalRewardStoreFake{role: database.Character{ID: 7, AccountID: 2, WireID: 9, ConfigVersion: (pvf.ArchiveSnapshot{}).SaveIdentity(), State: json.RawMessage(`{"unrelated":{"keep":true}}`)}, receipts: map[string]json.RawMessage{}}
	boxes := bakalWrapperSource{11: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 21, Weight: 1, Count: 8}}}}}, 12: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 22, Weight: 1, Count: 3}}}}}}
	s := &LegionRewardService{Store: store, Boxes: boxes, Awarder: &inventory.Awarder{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: hash}, Items: map[uint32]catalog.LootItem{21: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}, 22: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}}}, Rules: inventory.BagRules{Source: hash, Slots: map[string][2]uint16{"[etc]": {68, 69}}}}}
	return s, store, catalog.LegionPhaseRewards{Content: "fixture", Checksum: hash, Dungeon: 100, WeeklyLimit: 1, Basic: []catalog.LegionOperationReward{{Template: 11, Amount: 1, Weight: 1}}, Additional: []catalog.LegionOperationReward{{Template: 12, Amount: 1, Weight: 1}}}
}
func TestLegionRewardsFrozenFullBagReplayWeekAndPhase(t *testing.T) {
	s, store, r := legionRewardFixture()
	ctx := context.Background()
	now := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	role, p, err := s.Freeze(ctx, store.role, r, "run", now)
	if err != nil || !p.Eligible {
		t.Fatal(p, err)
	}
	bag, _ := inventory.ReadBag(store.role.State)
	bag.Items = []inventory.BagItem{{Slot: 68, Template: 21, Amount: 100}, {Slot: 69, Template: 22, Amount: 100}}
	store.role.State, _ = inventory.SaveBag(store.role.State, bag)
	before := string(store.role.State)
	if _, _, err = s.Claim(ctx, role, "run"); err == nil {
		t.Fatal("full bag committed grant")
	}
	if string(store.role.State) != before || len(store.receipts) != 1 {
		t.Fatal("failed grant lost frozen state")
	}
	// Alter wrapper contents after freezing: recovery must retain the first draw.
	s.Boxes = bakalWrapperSource{11: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 21, Weight: 1, Count: 99}}}}}}
	bag.Items = nil
	store.role.State, _ = inventory.SaveBag(store.role.State, bag)
	role, err = s.Recover(ctx, store.role)
	if err != nil {
		t.Fatal(err)
	}
	bag, _ = inventory.ReadBag(role.State)
	if len(bag.Items) != 2 || bag.Items[0].Amount != 8 || bag.Items[1].Amount != 3 {
		t.Fatal("rerolled products", bag)
	}
	_, replayed, err := s.Freeze(ctx, role, r, "run", now)
	if err != nil || !reflect.DeepEqual(replayed, p) {
		t.Fatal("freeze replay", replayed, err)
	}
	again, _, err := s.Claim(ctx, role, "run")
	if err != nil || string(again.State) != string(role.State) {
		t.Fatal("grant replay", err)
	}
	_, exhausted, err := s.Freeze(ctx, role, r, "second", now)
	if err != nil || exhausted.Eligible {
		t.Fatal("weekly phase cap", exhausted, err)
	}
	original, _, _ := legionRewardFixture()
	s.Boxes = original.Boxes
	r.Dungeon++
	role, other, err := s.Freeze(ctx, store.role, r, "other-phase", now)
	if err != nil || !other.Eligible {
		t.Fatal("different phase rejected", err)
	}
	r.Dungeon--
	_, next, err := s.Freeze(ctx, role, r, "next-week", now.Add(7*24*time.Hour))
	if err != nil || !next.Eligible {
		t.Fatal("new week rejected", err)
	}
	if !strings.Contains(string(store.role.State), `"unrelated":{"keep":true}`) {
		t.Fatal("unrelated save changed")
	}
}
func TestLegionSourceMismatchAndUnlimitedPolicy(t *testing.T) {
	s, store, r := legionRewardFixture()
	ctx := context.Background()
	now := time.Now()
	wrong := r
	wrong.Checksum = strings.Repeat("b", 64)
	if _, _, err := s.Freeze(ctx, store.role, wrong, "wrong", now); err == nil || len(store.receipts) != 0 {
		t.Fatal("foreign source accepted")
	}
	s.Unlimited = true
	for _, run := range []string{"first", "second"} {
		role, p, err := s.Freeze(ctx, store.role, r, run, now)
		if err != nil || !p.Eligible {
			t.Fatal("unlimited quota changed", err)
		}
		if _, _, err = s.Claim(ctx, role, run); err != nil {
			t.Fatal(err)
		}
	}
}
