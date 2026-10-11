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

func auctionFixture(t *testing.T, mode catalog.BakalMode) (*BakalAuctionService, *bakalRewardStoreFake, *catalog.BakalRaidRules, time.Time) {
	t.Helper()
	hash := strings.Repeat("a", 64)
	raw, e := inventory.SaveBag(json.RawMessage(`{"sentinel":4711}`), inventory.Bag{Version: "ordinary-bag-v1", Gold: 1000, Coin: 9})
	if e != nil {
		t.Fatal(e)
	}
	store := &bakalRewardStoreFake{role: database.Character{AccountID: 2, ID: 7, ConfigVersion: (pvf.ArchiveSnapshot{}).SaveIdentity(), State: raw}, receipts: map[string]json.RawMessage{}}
	boxes := bakalWrapperSource{11: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 21, Weight: 1, Count: 4}}}}}}
	s := &BakalAuctionService{Store: store, Boxes: boxes, Awarder: &inventory.Awarder{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: hash}, Items: map[uint32]catalog.LootItem{11: {Kind: "stackable", StackableType: "[virtual]"}, 21: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}}}, Rules: inventory.BagRules{Source: hash, Slots: map[string][2]uint16{"[etc]": {68, 69}}}}}
	r := &catalog.BakalRaidRules{Mode: mode, Rewards: []catalog.BakalRewardEntry{{Kind: "bidding_item", Template: 11, Weight: 1}}, Bidding: catalog.BakalBiddingRules{Count: []catalog.BakalWeightedNumber{{Value: 1, Weight: 1}}, WeeklyCount: []catalog.BakalWeightedNumber{{Value: 0, Weight: 1}}, WeeklyGroups: []catalog.BakalWeightedNumber{{Value: 1, Weight: 1}}, WeeklyItems: map[uint32][]catalog.BakalBiddingItem{1: {{Template: 11, Weight: 1}}}, MinimumPrice: 100, IncreaseRatio: 1.05, Time: 8, StartDelay: 10, BreakTime: 3, LowestTime: 4, DecreaseAfterBids: 1, DecreasePerBid: 1}}
	clear := BakalRewardPlan{Run: "fixture", Source: store.role.ConfigVersion, Content: hash, Mode: mode, Eligible: true}
	store.receipts["bakal-clear:fixture"], _ = json.Marshal(clear)
	return s, store, r, time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
}

func TestBakalAuctionGoldReservationFullBagReplayAndRecovery(t *testing.T) {
	for _, mode := range []catalog.BakalMode{catalog.BakalNormal, catalog.BakalHard} {
		s, store, r, now := auctionFixture(t, mode)
		ctx := context.Background()
		role, p, e := s.Freeze(ctx, store.role, r, "fixture", now)
		if e != nil || len(p.Lots) != 1 || p.Mode != mode {
			t.Fatal(p, e)
		}
		if _, _, e = s.Open(ctx, role, p.Run, 0, now); e == nil {
			t.Fatal("delay bypass")
		}
		role, p, e = s.Open(ctx, role, p.Run, 0, p.ReadyAt)
		if e != nil {
			t.Fatal(e)
		}
		started := p.Deadline.Add(-8 * time.Second)
		before := append([]byte(nil), store.role.State...)
		for _, amount := range []uint32{0, 99, 1001} {
			if _, _, e = s.Bid(ctx, role, p.Run, 0, amount, false, started); e == nil {
				t.Fatal("invalid gold bid accepted", amount)
			}
		}
		if !reflect.DeepEqual(before, []byte(store.role.State)) {
			t.Fatal("rejected bid changed wallet")
		}
		role, p, e = s.Bid(ctx, role, p.Run, 0, 100, false, started)
		if e != nil {
			t.Fatal(e)
		}
		role, _, e = s.Bid(ctx, role, p.Run, 0, 100, false, started)
		if e != nil {
			t.Fatal("replay", e)
		}
		bag, _ := inventory.ReadBag(role.State)
		if bag.Gold != 900 {
			t.Fatal("replay charged twice", bag.Gold)
		}
		if _, _, e = s.Bid(ctx, role, p.Run, 0, 104, false, started); e == nil {
			t.Fatal("source ratio bypass")
		}
		role, p, e = s.Bid(ctx, role, p.Run, 0, 200, false, started)
		if e != nil {
			t.Fatal(e)
		}
		bag, _ = inventory.ReadBag(role.State)
		if bag.Gold != 800 {
			t.Fatal("raise did not reserve difference", bag.Gold)
		}
		bag.Items = []inventory.BagItem{{Slot: 68, Template: 99, Amount: 1}, {Slot: 69, Template: 99, Amount: 1}}
		store.role.State, _ = inventory.SaveBag(role.State, bag)
		if _, _, e = s.Settle(ctx, store.role, p.Run, 0, p.Deadline); e == nil {
			t.Fatal("full bag consumed award")
		}
		_, plans, _ := readBakalAuctions(store.role.State)
		if plans[p.Run].Lots[0].Settled || plans[p.Run].Lots[0].Price != 200 {
			t.Fatal("full bag discarded bid")
		}
		bag.Items = nil
		store.role.State, _ = inventory.SaveBag(store.role.State, bag)
		role, e = s.Recover(ctx, store.role, p.Deadline.Add(time.Second))
		if e != nil {
			t.Fatal(e)
		}
		role, _, e = s.Settle(ctx, role, p.Run, 0, p.Deadline.Add(time.Second))
		if e != nil {
			t.Fatal("settlement replay", e)
		}
		bag, _ = inventory.ReadBag(role.State)
		if bag.Gold != 800 || bag.Coin != 9 || len(bag.Items) != 1 || bag.Items[0].Template != 21 || bag.Items[0].Amount != 4 {
			t.Fatal("wrong committed award", bag)
		}
		var state map[string]json.RawMessage
		_ = json.Unmarshal(role.State, &state)
		if string(state["sentinel"]) != "4711" {
			t.Fatal("unrelated state lost")
		}
	}
}

func TestBakalAuctionPassAndOwnerModeBoundaries(t *testing.T) {
	s, store, r, now := auctionFixture(t, catalog.BakalNormal)
	ctx := context.Background()
	r.Mode = catalog.BakalHard
	if _, _, e := s.Freeze(ctx, store.role, r, "fixture", now); e == nil {
		t.Fatal("wrong source mode accepted")
	}
	r.Mode = catalog.BakalNormal
	role, p, e := s.Freeze(ctx, store.role, r, "fixture", now)
	if e != nil {
		t.Fatal(e)
	}
	role, p, e = s.Open(ctx, role, p.Run, 0, p.ReadyAt)
	if e != nil {
		t.Fatal(e)
	}
	foreign := role
	foreign.AccountID++
	if _, _, e = s.Bid(ctx, foreign, p.Run, 0, 100, false, p.ReadyAt); e == nil {
		t.Fatal("foreign account accepted")
	}
	role, p, e = s.Bid(ctx, role, p.Run, 0, 0, true, p.ReadyAt)
	if e != nil {
		t.Fatal(e)
	}
	role, p, e = s.Settle(ctx, role, p.Run, 0, p.Deadline)
	if e != nil || !p.Finished {
		t.Fatal(p, e)
	}
	bag, _ := inventory.ReadBag(role.State)
	if bag.Gold != 1000 || len(bag.Items) != 0 {
		t.Fatal("pass granted free items", bag)
	}
}
