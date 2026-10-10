package workflow

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
)

func TestBakalWrapperPostgresRollbackReconnectConcurrentReplay(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("explicit disposable PostgreSQL database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	f, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})
	store := f.Storage()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "bakal-wrapper-fixture")
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("b", 64)
	initial, err := inventory.SaveBag(json.RawMessage(`{"unrelated":{"keep":true},"sentinel":4711}`), inventory.Bag{Version: "ordinary-bag-v1", Gold: 12345, Coin: 9})
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "BakalWrapperFixture", ConfigVersion: (pvf.ArchiveSnapshot{}).SaveIdentity(), Request: []byte{0, 255}, State: initial}, 24)
	if err != nil {
		t.Fatal(err)
	}
	boxes := bakalWrapperSource{
		11: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 21, Weight: 1, Count: 8}}}}},
		12: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 22, Weight: 1, Count: 3}}}}},
	}
	service := &BakalRewardService{Store: store, Boxes: boxes, Awarder: &inventory.Awarder{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: hash}, Items: map[uint32]catalog.LootItem{21: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}, 22: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}}}, Rules: inventory.BagRules{Source: hash, Slots: map[string][2]uint16{"[etc]": {68, 69}}}}}
	rules := &catalog.BakalRaidRules{MinimumClearCount: 3, WeeklyRewardCount: 1, Rewards: []catalog.BakalRewardEntry{{Kind: "party_card", Template: 11, Weight: 1}, {Kind: "squad_item", Template: 12, Weight: 1}}}
	frozen, plan, err := service.Freeze(ctx, role, rules, "fixture-run", 3, time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC))
	if err != nil || len(plan.Products) != 2 {
		t.Fatal(plan, err)
	}
	if err := f.SeedInventoryItems(ctx, role.ID, json.RawMessage(`[{"slot":68,"Template":99,"Amount":1},{"slot":69,"Template":99,"Amount":1}]`)); err != nil {
		t.Fatal(err)
	}
	before, err := f.CharacterState(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Claim(ctx, frozen, "fixture-run"); err == nil {
		t.Fatal("full bag committed a grant")
	}
	after, err := f.CharacterState(ctx, role.ID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("failed claim changed persisted state", err)
	}
	if n, err := f.EventCount(ctx, role.ID); err != nil || n != 1 {
		t.Fatal("failed grant consumed receipt", n, err)
	}
	if err := f.SeedInventoryItems(ctx, role.ID, json.RawMessage(`[]`)); err != nil {
		t.Fatal(err)
	}
	reopened, err := f.Reopen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	roles, err := reopened.Characters(ctx, account)
	if err != nil || len(roles) != 1 {
		t.Fatal(roles, err)
	}
	service.Store = reopened
	boxes[11] = loot.RewardBox{Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 21, Weight: 1, Count: 99}}}}}
	saved, err := service.Recover(ctx, roles[0])
	if err != nil {
		t.Fatal(err)
	}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		t.Fatal(err)
	}
	amounts := map[uint32]uint32{}
	for _, item := range bag.Items {
		amounts[item.Template] += item.Amount
	}
	if !reflect.DeepEqual(amounts, map[uint32]uint32{21: 8, 22: 3}) {
		t.Fatal("reconnect changed frozen products", amounts)
	}
	if bag.Gold != 12345 || bag.Coin != 9 {
		t.Fatal("reward changed wallet", bag.Gold, bag.Coin)
	}
	persisted, err := f.CharacterState(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errors := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _, err := service.Claim(ctx, frozen, "fixture-run"); errors <- err }()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	final, err := f.CharacterState(ctx, role.ID)
	if err != nil || !reflect.DeepEqual(persisted, final) {
		t.Fatal("parallel replay changed persisted state", err)
	}
	if n, err := f.EventCount(ctx, role.ID); err != nil || n != 2 {
		t.Fatal("parallel replay committed twice", n, err)
	}
	var state map[string]json.RawMessage
	if err := json.Unmarshal(final, &state); err != nil {
		t.Fatal(err)
	}
	if string(state["sentinel"]) != "4711" || string(state["unrelated"]) != `{"keep": true}` && string(state["unrelated"]) != `{"keep":true}` {
		t.Fatal("unrelated save fields changed", string(final))
	}
	if saved.ConfigVersion != role.ConfigVersion || !reflect.DeepEqual(saved.Request, role.Request) {
		t.Fatal("save identity or creation request changed")
	}
}
