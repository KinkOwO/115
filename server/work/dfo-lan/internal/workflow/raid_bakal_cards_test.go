package workflow

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func nativeBakalCardFixture() (*BakalRewardService, *bakalRewardStoreFake, *catalog.BakalRaidRules) {
	hash := strings.Repeat("e", 64)
	raw, _ := inventory.SaveBag(json.RawMessage(`{"keep":{"value":77}}`), inventory.Bag{Version: "ordinary-bag-v1", Gold: 555, Coin: 6})
	store := &bakalRewardStoreFake{role: database.Character{ID: 7, AccountID: 2, ConfigVersion: (pvf.ArchiveSnapshot{}).SaveIdentity(), State: raw}, receipts: map[string]json.RawMessage{}}
	service := &BakalRewardService{NativeCards: true, Store: store, Awarder: &inventory.Awarder{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: hash}, Items: map[uint32]catalog.LootItem{21: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}, 22: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}}}, Rules: inventory.BagRules{Source: hash, Slots: map[string][2]uint16{"[etc]": {68, 69}}}}}
	rules := &catalog.BakalRaidRules{Mode: catalog.BakalHard, RewardCardSlots: 12, MinimumClearCount: 3, WeeklyClearCount: 1, WeeklyRewardCount: 1, Rewards: []catalog.BakalRewardEntry{{Kind: "party_card", Template: 21, Weight: 1}, {Kind: "squad_item", Template: 22, Weight: 1}}}
	return service, store, rules
}

func TestBakalNativeCardsFreezeFullBagReconnectAndReplay(t *testing.T) {
	service, store, rules := nativeBakalCardFixture()
	ctx := context.Background()
	now := time.Now()
	role, plan, err := service.Freeze(ctx, store.role, rules, "native", 3, now)
	if err != nil || len(plan.Cards) != 2 || plan.Mode != catalog.BakalHard {
		t.Fatal(plan, err)
	}
	bag, _ := inventory.ReadBag(role.State)
	if len(bag.Items) != 0 {
		t.Fatal("freeze paid cards before selection")
	}
	if _, _, err = service.Claim(ctx, role, plan.Run); err == nil {
		t.Fatal("native cards bypassed selection")
	}
	if _, _, err = service.ClaimCard(ctx, role, plan.Run, 0); err == nil {
		t.Fatal("unselected card paid")
	}
	if _, _, err = service.SelectCard(ctx, role, plan.Run, 0, 12); err == nil {
		t.Fatal("foreign card index accepted")
	}
	if _, _, err = service.SelectCard(ctx, role, plan.Run, 2, 0); err == nil {
		t.Fatal("PC-room entitlement invented")
	}
	role, _, err = service.SelectCard(ctx, role, plan.Run, 1, 7)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = service.SelectCard(ctx, role, plan.Run, 1, 8); err == nil {
		t.Fatal("duplicate changed frozen choice")
	}
	bag.Items = []inventory.BagItem{{Slot: 68, Template: 99, Amount: 1}, {Slot: 69, Template: 99, Amount: 1}}
	store.role.State, _ = inventory.SaveBag(store.role.State, bag)
	before := append([]byte(nil), store.role.State...)
	if _, _, err = service.ClaimCard(ctx, store.role, plan.Run, 1); err == nil {
		t.Fatal("full bag paid")
	}
	if !reflect.DeepEqual(before, []byte(store.role.State)) {
		t.Fatal("failed grant changed player state")
	}
	_, progress, err := readBakalProgress(store.role.State)
	if err != nil || progress.Pending[plan.Run].Cards[1].Selection == nil || *progress.Pending[plan.Run].Cards[1].Selection != 7 {
		t.Fatal("full bag lost selected index", err)
	}
	bag.Items = nil
	store.role.State, _ = inventory.SaveBag(store.role.State, bag)
	// Same store with a new service simulates reconnect. Preserve existing
	// chosen7, default only the still-unselected card to native auto-pick0.
	reopened := *service
	role, err = reopened.Recover(ctx, store.role)
	if err != nil {
		t.Fatal(err)
	}
	bag, err = inventory.ReadBag(role.State)
	if err != nil {
		t.Fatal(err)
	}
	amounts := map[uint32]uint32{}
	for _, item := range bag.Items {
		amounts[item.Template] += item.Amount
	}
	if !reflect.DeepEqual(amounts, map[uint32]uint32{21: 1, 22: 1}) || bag.Gold != 555 || bag.Coin != 6 {
		t.Fatal("wrong recovered products/wallet", amounts, bag)
	}
	for _, kind := range []byte{1, 0} {
		replay, card, e := reopened.ClaimCard(ctx, role, plan.Run, kind)
		if e != nil || !card.Claimed || !reflect.DeepEqual(role.State, replay.State) {
			t.Fatal("replay paid twice", card, e)
		}
	}
	normal := *rules
	normal.Mode = catalog.BakalNormal
	if err = BakalRaidAdmission(role, &normal, now); err != ErrBakalWeeklyClearLimit {
		t.Fatal("switching modes bypassed shared weekly clear", err)
	}
	_, progress, err = readBakalProgress(role.State)
	if err != nil || len(progress.Pending) != 0 || progress.Rewards != 1 || progress.Clears != 1 {
		t.Fatal("wrong weekly/pending state", progress, err)
	}
	if !strings.Contains(string(role.State), `"value":77`) {
		t.Fatal("unrelated state lost")
	}
}

func TestBakalNativeEmptyCardStillCompletesWithoutItem(t *testing.T) {
	service, store, rules := nativeBakalCardFixture()
	rules.Rewards[0].Template = -1
	role, p, err := service.Freeze(context.Background(), store.role, rules, "empty", 3, time.Now())
	if err != nil || len(p.Cards[0].Products) != 0 {
		t.Fatal(p, err)
	}
	role, _, err = service.SelectCard(context.Background(), role, p.Run, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	role, card, err := service.ClaimCard(context.Background(), role, p.Run, 0)
	if err != nil || !card.Claimed || len(card.Products) != 0 {
		t.Fatal(card, err)
	}
	bag, _ := inventory.ReadBag(role.State)
	if len(bag.Items) != 0 {
		t.Fatal("empty face rerolled")
	}
}
