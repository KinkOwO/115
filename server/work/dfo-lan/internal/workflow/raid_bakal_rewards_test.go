package workflow

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

type bakalRewardStoreFake struct {
	role     database.Character
	receipts map[string]json.RawMessage
}

func (s *bakalRewardStoreFake) CommitCharacterEvent(_ context.Context, account, id int64, version, key, model string, apply func(database.Character) (json.RawMessage, json.RawMessage, error)) (database.Character, bool, error) {
	if account != s.role.AccountID || id != s.role.ID || version != s.role.ConfigVersion {
		return database.Character{}, false, fmt.Errorf("owner mismatch")
	}
	if _, exists := s.receipts[key]; exists {
		return s.role, false, nil
	}
	state, receipt, err := apply(s.role)
	if err != nil {
		return database.Character{}, false, err
	}
	s.role.State = state
	s.receipts[key] = receipt
	return s.role, true, nil
}
func (s *bakalRewardStoreFake) CharacterEventReceipt(_ context.Context, _, _ int64, key string) (json.RawMessage, error) {
	return s.receipts[key], nil
}

func TestBakalRewardFreezeReplayWeeklyAndPendingBag(t *testing.T) {
	hash := strings.Repeat("a", 64)
	store := &bakalRewardStoreFake{role: database.Character{ID: 7, AccountID: 2, WireID: 9, ConfigVersion: (pvf.ArchiveSnapshot{}).SaveIdentity(), State: json.RawMessage(`{"unrelated":{"keep":true}}`)}, receipts: map[string]json.RawMessage{}}
	svc := &BakalRewardService{Store: store, Awarder: &inventory.Awarder{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: hash}, Items: map[uint32]catalog.LootItem{11: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}, 12: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}}}, Rules: inventory.BagRules{Source: hash, Slots: map[string][2]uint16{"[etc]": {68, 69}}}}}
	r := &catalog.BakalRaidRules{MinimumClearCount: 3, WeeklyRewardCount: 1, Rewards: []catalog.BakalRewardEntry{{Kind: "party_card", Weight: 100, Template: 11}, {Kind: "squad_item", Weight: 100, Template: 12}, {Kind: "pcroom_card", Weight: 100, Template: 999}, {Kind: "bidding_item", Weight: 100, Template: 998}}}
	ctx := context.Background()
	now := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	saved, p, err := svc.Freeze(ctx, store.role, r, "fixture", 3, now)
	if err != nil || !p.Eligible || len(p.Items) != 2 {
		t.Fatal(p, err)
	}
	// A full/unsupported bag must not discard the already drawn source plan.
	bag, _ := inventory.ReadBag(store.role.State)
	bag.Items = []inventory.BagItem{{Slot: 68, Template: 13, Amount: 1}, {Slot: 69, Template: 13, Amount: 1}}
	store.role.State, _ = inventory.SaveBag(store.role.State, bag)
	if _, _, err = svc.Claim(ctx, saved, "fixture"); err == nil {
		t.Fatal("unavailable bag accepted")
	}
	_, progress, _ := readBakalProgress(store.role.State)
	if len(progress.Pending) != 1 || len(store.receipts) != 1 {
		t.Fatal("failed grant lost plan or committed grant")
	}
	bag.Items = nil
	store.role.State, _ = inventory.SaveBag(store.role.State, bag)
	saved, err = svc.Recover(ctx, store.role)
	if err != nil {
		t.Fatal(err)
	}
	bag, err = inventory.ReadBag(saved.State)
	if err != nil || len(bag.Items) != 2 {
		t.Fatal(bag, err)
	}
	again, _, err := svc.Claim(ctx, saved, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if string(again.State) != string(saved.State) {
		t.Fatal("replay changed inventory")
	}
	_, replayed, err := svc.Freeze(ctx, again, r, "fixture", 3, now)
	if err != nil || len(replayed.Items) != 2 {
		t.Fatal(replayed, err)
	}
	_, second, err := svc.Freeze(ctx, again, r, "second", 3, now)
	if err != nil || second.Eligible {
		t.Fatal("weekly reward cap not enforced", second, err)
	}
	if !strings.Contains(string(store.role.State), `"unrelated":{"keep":true}`) {
		t.Fatal("unrelated character state changed")
	}
}
