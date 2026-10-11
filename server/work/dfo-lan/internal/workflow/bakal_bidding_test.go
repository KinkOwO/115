package workflow

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

type biddingEventFake struct {
	role     database.Character
	receipts map[string]json.RawMessage
	writes   int
}

func (s *biddingEventFake) CommitCharacterEvent(_ context.Context, account, id int64, version, key, model string, apply func(database.Character) (json.RawMessage, json.RawMessage, error)) (database.Character, bool, error) {
	if account != s.role.AccountID || id != s.role.ID || version != s.role.ConfigVersion {
		return s.role, false, fmt.Errorf("owner/source mismatch")
	}
	if _, ok := s.receipts[key]; ok {
		return s.role, false, nil
	}
	state, receipt, e := apply(s.role)
	if e != nil {
		return s.role, false, e
	}
	s.role.State = state
	s.receipts[key] = receipt
	s.writes++
	return s.role, true, nil
}
func (s *biddingEventFake) CharacterEventReceipt(_ context.Context, account, id int64, key string) (json.RawMessage, error) {
	if account != s.role.AccountID || id != s.role.ID {
		return nil, fmt.Errorf("wrong owner")
	}
	r, ok := s.receipts[key]
	if !ok {
		return nil, fmt.Errorf("receipt missing")
	}
	return r, nil
}

func biddingFixture(t *testing.T) (*BakalRewardService, *biddingEventFake, time.Time) {
	t.Helper()
	state, e := inventory.SaveBag(json.RawMessage(`{"unrelated":{"keep":true}}`), inventory.Bag{Version: "ordinary-bag-v1", Gold: 1000})
	if e != nil {
		t.Fatal(e)
	}
	role := database.Character{ID: 7, AccountID: 2, WireID: 11, ConfigVersion: savecontract.Identity(), State: state}
	svc, _ := bakalTestService(t, role)
	store := &biddingEventFake{role: role, receipts: map[string]json.RawMessage{}}
	svc.Store = store
	svc.Rules.Bidding = catalog.BakalBiddingRules{StartDelaySecs: 10, Hards: []catalog.BakalBiddingHard{{Hard: 0, Rates: []catalog.BakalBiddingRate{{Grade: 2, Weight: 1}}}}, Items: []catalog.BakalRewardEntry{{Template: 10418036, Amount: 1, Weight: 1}}, WeeklyCounts: []catalog.BakalBiddingHard{{Hard: 0, Rates: []catalog.BakalBiddingRate{{Grade: 0, Weight: 1}}}}}
	at := time.Unix(1800000000, 0)
	if _, _, e := svc.Freeze(context.Background(), role, "owned-run", 3, at); e != nil {
		t.Fatal(e)
	}
	return svc, store, at
}

func TestBakalBiddingFreezesOnceAndPurchasesAtomically(t *testing.T) {
	s, store, at := biddingFixture(t)
	ctx := context.Background()
	plan, role, e := s.FreezeBidding(ctx, store.role, "owned-run", false, at)
	if e != nil || len(plan.Lots) != 2 {
		t.Fatalf("freeze: %+v %v", plan, e)
	}
	writes := store.writes
	s.Rules.Bidding.Hards[0].Rates[0].Grade = 9
	prior, role, e := s.FreezeBidding(ctx, role, "owned-run", false, at.Add(time.Hour))
	if e != nil || len(prior.Lots) != 2 || store.writes != writes {
		t.Fatal("replay redrew lots")
	}
	if _, _, e = s.PurchaseBiddingLot(ctx, role, "owned-run", 0, 100, at); e == nil {
		t.Fatal("purchased before source start delay")
	}
	if _, _, e = s.PurchaseBiddingLot(ctx, role, "owned-run", 0, 1001, plan.ReadyAt); e == nil {
		t.Fatal("insufficient currency accepted")
	}
	bag, _ := inventory.ReadBag(store.role.State)
	if bag.Gold != 1000 || store.writes != writes {
		t.Fatal("failed purchase mutated gold/state")
	}
	role, sale, e := s.PurchaseBiddingLot(ctx, role, "owned-run", 0, 100, plan.ReadyAt)
	if e != nil || !sale.Sold || sale.Price != 100 {
		t.Fatalf("purchase: %+v %v", sale, e)
	}
	bag, _ = inventory.ReadBag(role.State)
	if bag.Gold != 900 {
		t.Fatal("gold not debited")
	}
	count := bag.Items[0].Amount
	role, _, e = s.PurchaseBiddingLot(ctx, role, "owned-run", 0, 100, plan.ReadyAt.Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	bag, _ = inventory.ReadBag(role.State)
	if bag.Gold != 900 || bag.Items[0].Amount != count {
		t.Fatal("replay charged/granted twice")
	}
	var top map[string]json.RawMessage
	json.Unmarshal(role.State, &top)
	if string(top["unrelated"]) != `{"keep":true}` {
		t.Fatal("unrelated save data lost")
	}
	if _, _, e = s.FreezeBidding(ctx, role, "owned-run", true, at); e == nil {
		t.Fatal("normal clear authenticated hard auction")
	}
}

func TestBakalBiddingPurchaseFailureRollsBackAndRejectsForeignRun(t *testing.T) {
	s, store, at := biddingFixture(t)
	ctx := context.Background()
	_, role, e := s.FreezeBidding(ctx, store.role, "owned-run", false, at)
	if e != nil {
		t.Fatal(e)
	}
	top, plans, _ := readBakalBidding(role.State)
	plan := plans["owned-run"]
	plan.Lots[0].Award.Template = 999999999
	plans["owned-run"] = plan
	state, _ := saveBakalBidding(top, plans)
	store.role.State = state
	before := string(state)
	if _, _, e = s.PurchaseBiddingLot(ctx, store.role, "owned-run", 0, 100, at.Add(time.Hour)); e == nil {
		t.Fatal("invalid award accepted")
	}
	if string(store.role.State) != before {
		t.Fatal("failed award committed gold decrement")
	}
	if _, _, e = s.FreezeBidding(ctx, store.role, "foreign-run", false, at); e == nil {
		t.Fatal("unowned clear generated auction")
	}
}
