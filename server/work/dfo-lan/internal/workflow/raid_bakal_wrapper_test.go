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

type bakalWrapperSource map[uint32]loot.RewardBox

func (s bakalWrapperSource) RewardBox(id uint32) (loot.RewardBox, bool) { b, ok := s[id]; return b, ok }
func (s bakalWrapperSource) Item(id uint32) bool                        { return id == 21 || id == 22 }
func (s bakalWrapperSource) Container(id uint32) bool                   { return id == 11 || id == 12 || id == 13 }

func TestBakalWrapperProductsFrozenAndRecoveredExactlyOnce(t *testing.T) {
	hash := strings.Repeat("a", 64)
	store := &bakalRewardStoreFake{role: database.Character{ID: 7, AccountID: 2, ConfigVersion: (pvf.ArchiveSnapshot{}).SaveIdentity(), State: json.RawMessage(`{"unrelated":{"keep":true}}`)}, receipts: map[string]json.RawMessage{}}
	boxes := bakalWrapperSource{
		11: {Pools: []loot.RewardBoxPool{{Draws: 2, Candidates: []loot.RewardBoxCandidate{{Template: 13, Weight: 1, Count: 1}}}}},
		13: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 21, Weight: 1, Count: 4}}}}},
		12: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 22, Weight: 1, Count: 3}}}}},
	}
	svc := &BakalRewardService{Store: store, Boxes: boxes, Awarder: &inventory.Awarder{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: hash}, Items: map[uint32]catalog.LootItem{21: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}, 22: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}}}, Rules: inventory.BagRules{Source: hash, Slots: map[string][2]uint16{"[etc]": {68, 69}}}}}
	rules := &catalog.BakalRaidRules{MinimumClearCount: 3, WeeklyRewardCount: 1, Rewards: []catalog.BakalRewardEntry{{Kind: "party_card", Template: 11, Weight: 1}, {Kind: "squad_item", Template: 12, Weight: 1}}}
	ctx := context.Background()
	now := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	role, plan, err := svc.Freeze(ctx, store.role, rules, "run", 3, now)
	if err != nil || len(plan.Products) == 0 {
		t.Fatal(plan, err)
	}
	amounts := map[uint32]uint32{}
	for _, p := range plan.Products {
		amounts[p.Template] += p.Amount
	}
	if !reflect.DeepEqual(amounts, map[uint32]uint32{21: 8, 22: 3}) {
		t.Fatal(amounts)
	}
	// Changing the source after Freeze must not reroll pending products.
	boxes[13] = loot.RewardBox{Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 21, Weight: 1, Count: 99}}}}}
	bag, _ := inventory.ReadBag(role.State)
	bag.Items = []inventory.BagItem{{Slot: 68, Template: 99, Amount: 1}, {Slot: 69, Template: 99, Amount: 1}}
	store.role.State, _ = inventory.SaveBag(store.role.State, bag)
	if _, _, err := svc.Claim(ctx, store.role, "run"); err == nil {
		t.Fatal("full bag consumed products")
	}
	if len(store.receipts) != 1 {
		t.Fatal("failed grant committed")
	}
	bag.Items = nil
	store.role.State, _ = inventory.SaveBag(store.role.State, bag)
	saved, err := svc.Recover(ctx, store.role)
	if err != nil {
		t.Fatal(err)
	}
	bag, err = inventory.ReadBag(saved.State)
	if err != nil {
		t.Fatal(err)
	}
	got := map[uint32]uint32{}
	for _, item := range bag.Items {
		got[item.Template] += item.Amount
	}
	if !reflect.DeepEqual(got, amounts) {
		t.Fatal(got)
	}
	replay, _, err := svc.Claim(ctx, saved, "run")
	if err != nil || !reflect.DeepEqual(saved.State, replay.State) {
		t.Fatal("replayed grant", err)
	}
	if !strings.Contains(string(saved.State), `"unrelated":{"keep":true}`) {
		t.Fatal("unrelated saved state changed")
	}
}

func TestBakalUnresolvedWrappersCannotConsumeClear(t *testing.T) {
	hash := strings.Repeat("b", 64)
	store := &bakalRewardStoreFake{role: database.Character{ID: 7, AccountID: 2, ConfigVersion: (pvf.ArchiveSnapshot{}).SaveIdentity(), State: json.RawMessage(`{"keep":1}`)}, receipts: map[string]json.RawMessage{}}
	svc := &BakalRewardService{Store: store, Boxes: bakalWrapperSource{}, Awarder: &inventory.Awarder{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: hash}}}}
	rules := &catalog.BakalRaidRules{Rewards: []catalog.BakalRewardEntry{{Kind: "party_card", Template: 11, Weight: 1}, {Kind: "squad_item", Template: 12, Weight: 1}}}
	before := append([]byte(nil), store.role.State...)
	if _, _, err := svc.Freeze(context.Background(), store.role, rules, "run", 1, time.Now()); err == nil {
		t.Fatal("unparsed wrapper paid")
	}
	if !reflect.DeepEqual(before, []byte(store.role.State)) || len(store.receipts) != 0 {
		t.Fatal("unresolved wrapper consumed clear")
	}
}

func TestBakalSourceEmptyFaceIsNotRerolledIntoAnItem(t *testing.T) {
	hash := strings.Repeat("c", 64)
	store := &bakalRewardStoreFake{role: database.Character{ID: 7, AccountID: 2, ConfigVersion: (pvf.ArchiveSnapshot{}).SaveIdentity(), State: json.RawMessage(`{"keep":1}`)}, receipts: map[string]json.RawMessage{}}
	svc := &BakalRewardService{Store: store, Awarder: &inventory.Awarder{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: hash}}}}
	rules := &catalog.BakalRaidRules{Rewards: []catalog.BakalRewardEntry{{Kind: "party_card", Template: -1, Weight: 1}, {Kind: "squad_item", Template: -1, Weight: 1}}}
	role, plan, err := svc.Freeze(context.Background(), store.role, rules, "empty", 1, time.Now())
	if err != nil || !plan.Eligible || len(plan.Items) != 0 || plan.Products == nil {
		t.Fatal(plan, err)
	}
	if _, got, err := svc.Claim(context.Background(), role, "empty"); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
}

func TestBakalLegacyPendingReceiptKeepsOriginalGrantContract(t *testing.T) {
	hash := strings.Repeat("d", 64)
	identity := (pvf.ArchiveSnapshot{}).SaveIdentity()
	// An existing v1 save has Items and no Products member at all.
	legacy := map[string]any{"bakal_raid_rewards": map[string]any{"Pending": map[string]any{"old-run": map[string]any{"Run": "old-run", "Source": identity, "Content": hash, "Eligible": true, "Items": []catalog.BakalRewardEntry{{Kind: "party_card", Template: 21, Weight: 1}}}}}}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	store := &bakalRewardStoreFake{role: database.Character{ID: 7, AccountID: 2, ConfigVersion: identity, State: raw}, receipts: map[string]json.RawMessage{}}
	service := &BakalRewardService{Store: store, Boxes: bakalWrapperSource{}, Awarder: &inventory.Awarder{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: hash}, Items: map[uint32]catalog.LootItem{21: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}}}, Rules: inventory.BagRules{Source: hash, Slots: map[string][2]uint16{"[etc]": {68, 69}}}}}
	saved, err := service.Recover(context.Background(), store.role)
	if err != nil {
		t.Fatal(err)
	}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil || len(bag.Items) != 1 || bag.Items[0].Template != 21 || bag.Items[0].Amount != 1 {
		t.Fatal("legacy pending plan was rerolled or lost", bag, err)
	}
	if _, err := service.Recover(context.Background(), saved); err != nil || len(store.receipts) != 1 {
		t.Fatal("legacy pending plan paid twice", err)
	}
}
