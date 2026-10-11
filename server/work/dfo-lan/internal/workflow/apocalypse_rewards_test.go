package workflow

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"encoding/json"
	"strings"
	"testing"
)

func apocalypseRewardFixture() (*ApocalypseRewardService, *bakalRewardStoreFake, ApocalypseRewardPlan) {
	hash := strings.Repeat("b", 64)
	source := pvf.ArchiveSnapshot{Checksum: hash}
	store := &bakalRewardStoreFake{role: database.Character{ID: 7, AccountID: 2, WireID: 9, ConfigVersion: source.SaveIdentity(), State: json.RawMessage(`{"future":{"keep":true}}`)}, receipts: map[string]json.RawMessage{}}
	svc := &ApocalypseRewardService{Store: store, Awarder: &inventory.Awarder{Catalog: catalog.LootCatalog{Source: source, Items: map[uint32]catalog.LootItem{17: {Kind: "stackable", StackableType: "[booster]", StackLimit: 100}}}, Rules: inventory.BagRules{Source: hash, Slots: map[string][2]uint16{"[booster]": {3, 4}}}}}
	plan := ApocalypseRewardPlan{Run: "navigation-run", Source: source.SaveIdentity(), Content: hash, Rules: strings.Repeat("c", 64), Operation: 1, Phase: 3, Reward: legion.ClearReward{Template: 17, Amount: 1}}
	return svc, store, plan
}

func TestApocalypseDistinctFullClearsRewardButSameClearIsIdempotent(t *testing.T) {
	svc, store, plan := apocalypseRewardFixture()
	ctx := context.Background()
	if _, _, err := svc.Claim(ctx, store.role, plan.Run); err == nil {
		t.Fatal("request without full clear awarded")
	}
	for _, run := range []string{"first", "first", "second", "second", "third", "fourth"} {
		plan.Run = run
		saved, _, err := svc.Freeze(ctx, store.role, plan)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err = svc.Claim(ctx, saved, run); err != nil {
			t.Fatal(err)
		}
		if got, err := svc.Granted(ctx, store.role, run); err != nil || got.Run != run {
			t.Fatal(got, err)
		}
	}
	bag, err := inventory.ReadBag(store.role.State)
	if err != nil || len(bag.Items) != 1 || bag.Items[0].Amount != 4 || len(store.receipts) != 8 {
		t.Fatal("per-room/replayed reward or weekly cap", bag, err)
	}
	if !strings.Contains(string(store.role.State), `"future":{"keep":true}`) {
		t.Fatal("unrelated save field lost")
	}
}

func TestApocalypseFullBagPreservesClearAndRetriesCurrentSavedState(t *testing.T) {
	svc, store, plan := apocalypseRewardFixture()
	ctx := context.Background()
	bag, _ := inventory.ReadBag(store.role.State)
	bag.Items = []inventory.BagItem{{Slot: 3, Template: 88, Amount: 1}, {Slot: 4, Template: 89, Amount: 1}}
	store.role.State, _ = inventory.SaveBag(store.role.State, bag)
	saved, _, err := svc.Freeze(ctx, store.role, plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = svc.Claim(ctx, saved, plan.Run); err == nil {
		t.Fatal("full bag committed")
	}
	_, pending, err := apocalypsePending(store.role.State)
	if err != nil || len(pending) != 1 || len(store.receipts) != 1 {
		t.Fatal("pending clear lost", err)
	}
	bag.Items = nil
	bag.Gold = 12345
	store.role.State, _ = inventory.SaveBag(store.role.State, bag)
	// The old role snapshot still has a full bag. The transaction must use
	// current locked storage, including unrelated changes since the clear.
	if _, err = svc.Recover(ctx, saved); err != nil {
		t.Fatal(err)
	}
	bag, err = inventory.ReadBag(store.role.State)
	if err != nil || bag.Gold != 12345 || len(bag.Items) != 1 || bag.Items[0].Template != plan.Reward.Template {
		t.Fatal("retry overwrote concurrent inventory", bag, err)
	}
	_, pending, _ = apocalypsePending(store.role.State)
	if len(pending) != 0 {
		t.Fatal("granted clear remained pending")
	}
	if _, err = svc.Recover(ctx, store.role); err != nil || len(store.receipts) != 2 {
		t.Fatal("reopening UI created a new clear", err)
	}
}

func TestApocalypseRecoveryRetainsPendingOnSourceChange(t *testing.T) {
	svc, store, plan := apocalypseRewardFixture()
	ctx := context.Background()
	saved, _, err := svc.Freeze(ctx, store.role, plan)
	if err != nil {
		t.Fatal(err)
	}
	svc.Awarder.Catalog.Source.Checksum = strings.Repeat("d", 64)
	if _, err = svc.Recover(ctx, saved); err == nil {
		t.Fatal("changed archive granted old source reward")
	}
	_, pending, err := apocalypsePending(store.role.State)
	bag, bagErr := inventory.ReadBag(store.role.State)
	if err != nil || bagErr != nil || len(pending) != 1 || len(bag.Items) != 0 || len(store.receipts) != 1 {
		t.Fatal("source change lost pending clear or granted", pending, bag, err, bagErr)
	}
}

func TestApocalypseBundleGrantIsAtomicAndFrozenAcrossRecovery(t *testing.T) {
	svc, store, plan := apocalypseRewardFixture()
	ctx := context.Background()
	svc.Awarder.Catalog.Items[18] = catalog.LootItem{Kind: "stackable", StackableType: "[material]", StackLimit: 1000}
	svc.Awarder.Catalog.Items[19] = catalog.LootItem{Kind: "stackable", StackableType: "[material]", StackLimit: 1000}
	svc.Awarder.Rules.Slots["[material]"] = [2]uint16{5, 5}
	plan.AdditionalRules = strings.Repeat("e", 64)
	plan.Additional = []legion.ClearReward{{Template: 18, Amount: 20}, {Template: 19, Amount: 96}}
	saved, _, err := svc.Freeze(ctx, store.role, plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = svc.Claim(ctx, saved, plan.Run); err == nil {
		t.Fatal("partial bundle committed into full material band")
	}
	bag, _ := inventory.ReadBag(store.role.State)
	if len(bag.Items) != 0 || len(store.receipts) != 1 {
		t.Fatal("earlier box/material leaked from failed transaction", bag)
	}
	// Altering a retry proposal cannot change an already frozen clear.
	plan.Additional[0].Amount = 999
	_, frozen, err := svc.Freeze(ctx, store.role, plan)
	if err != nil || frozen.Additional[0].Amount != 20 {
		t.Fatal(frozen, err)
	}
	svc.Awarder.Rules.Slots["[material]"] = [2]uint16{5, 6}
	if _, err = svc.Recover(ctx, store.role); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Recover(ctx, store.role); err != nil {
		t.Fatal(err)
	}
	bag, _ = inventory.ReadBag(store.role.State)
	if len(bag.Items) != 3 || len(store.receipts) != 2 {
		t.Fatal("recovery omitted or duplicated products", bag)
	}
	if got, err := svc.Granted(ctx, store.role, plan.Run); err != nil || got.Additional[0].Amount != 20 {
		t.Fatal(got, err)
	}
	var receipt ApocalypseAwardReceipt
	json.Unmarshal(store.receipts["apocalypse-grant:"+plan.Run], &receipt)
	receipt.Additional = receipt.Additional[:1]
	store.receipts["apocalypse-grant:"+plan.Run], _ = json.Marshal(receipt)
	if _, err := svc.Granted(ctx, store.role, plan.Run); err == nil {
		t.Fatal("partial receipt authorized clear return")
	}
}
