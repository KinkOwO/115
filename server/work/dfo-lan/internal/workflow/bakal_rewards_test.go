package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/savecontract"
)

type bakalTestBoxes struct{}

func (bakalTestBoxes) RewardBox(uint32) (loot.RewardBox, bool) { return loot.RewardBox{}, false }
func (bakalTestBoxes) Item(id uint32) bool                     { return id == 10418036 }
func (bakalTestBoxes) Container(uint32) bool                   { return false }

// bakalTestRules mirrors the locked catalog truth of bakal.etc (the values
// bakal_raid_test.go pins: weekly limits 1/1, [MINIMAL DUNGEON CLEAR COUNT] 3,
// one channel-slot expected item and the two monster-piece ranks) with one
// substitution: the item templates use the stackable the configs can actually
// grant, so the Claim path exercises the real awarder.
func bakalTestRules() *catalog.BakalRaidRules {
	return &catalog.BakalRaidRules{
		WeeklyClearLimit:         1,
		WeeklyRewardLimit:        1,
		MinimalDungeonClearCount: 3,
		ChannelSlotRewardItems:   []uint32{10418036},
		MonsterPieceRewards:      []uint32{0, 10418036, 1, 10418036},
		Rewards:                  []catalog.BakalRewardEntry{{Category: "party_card", Template: 10418036, Amount: 1, Weight: 100}, {Category: "squad_item", Template: 10418036, Amount: 1, Weight: 100}},
	}
}

func bakalTestService(t *testing.T, role database.Character) (*BakalRewardService, *equipmentEventFake) {
	t.Helper()
	c, err := catalog.LoadLoot("../../configs/loot.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = c.SupplementStackables("../catalog/testdata/item-flow.json"); err != nil {
		t.Fatal(err)
	}
	rules, err := inventory.LoadBagRules("../../configs/inventory.next29.json")
	if err != nil {
		t.Fatal(err)
	}
	gear, err := inventory.LoadEquipmentCatalog("../../configs/equipment.current35.json", c.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	store := &equipmentEventFake{role: role}
	// The production wiring passes *database.Store; the unit tests exercise the
	// same two event methods through the in-memory fake.
	svc := &BakalRewardService{
		Store:   store,
		Loot:    &loot.Service{Catalog: c, BagRules: rules, Equipment: gear, RewardBoxes: bakalTestBoxes{}},
		Rules:   bakalTestRules(),
		Content: "contents/2022/bakalraid",
	}
	return svc, store
}

func TestBakalRaidAdmissionWeeklyGate(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	rules := bakalTestRules()
	if err := BakalRaidAdmission(nil, database.Character{}, now); err == nil || err.Error() != "native raid rules missing" {
		t.Fatalf("nil rules: %v", err)
	}
	if err := BakalRaidAdmission(rules, database.Character{State: json.RawMessage(`{}`)}, now); err != nil {
		t.Fatalf("fresh ledger must admit: %v", err)
	}
	sameWeek := json.RawMessage(`{"bakal_raid_rewards":{"week":"` + bakalWeekOf(now) + `","clears":1,"rewards":0,"plans":{}}}`)
	err := BakalRaidAdmission(rules, database.Character{State: sameWeek}, now)
	if !errors.Is(err, ErrBakalWeeklyClearLimit) {
		t.Fatalf("exhausted week: %v", err)
	}
	if err.Error() != "普通巴卡尔本周通关次数已用完" {
		t.Fatalf("error text mutated: %q", err.Error())
	}
	// 2026-10-05 is a Monday; the raid week (UTC-9, aligned Tuesday 09:00) of
	// that moment started 2026-09-29. Anything older is a different week.
	stale := json.RawMessage(`{"bakal_raid_rewards":{"week":"2026-09-22T09:00:00Z","clears":9,"rewards":9,"plans":{}}}`)
	if err := BakalRaidAdmission(rules, database.Character{State: stale}, now); err != nil {
		t.Fatalf("stale week must admit: %v", err)
	}
	broken := json.RawMessage(`{"bakal_raid_rewards":[1]}`)
	if err := BakalRaidAdmission(rules, database.Character{State: broken}, now); err == nil || err.Error() != "invalid character state for raid rewards" {
		t.Fatalf("broken ledger: %v", err)
	}
}

func TestBakalRewardFreezeClaimRecover(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	role := database.Character{ID: 7, AccountID: 2, Name: "Lansmt", ConfigVersion: savecontract.Identity(), State: json.RawMessage(`{}`)}
	svc, store := bakalTestService(t, role)
	ctx := context.Background()

	// 1. A clear under the minimal dungeon count earns nothing but still
	//    advances the weekly clear counter.
	plan, saved, err := svc.Freeze(ctx, store.role, "run-short", 2, now)
	if err != nil || plan.Eligible {
		t.Fatalf("short clear: plan=%+v err=%v", plan, err)
	}
	if plan.Run != "run-short" || plan.Source != savecontract.Identity() || plan.Content != "contents/2022/bakalraid" || plan.Week != bakalWeekOf(now) {
		t.Fatalf("plan identity: %+v", plan)
	}
	if err := BakalRaidAdmission(svc.Rules, saved, now); !errors.Is(err, ErrBakalWeeklyClearLimit) {
		t.Fatalf("weekly clear limit must trip after the ineligible clear: %v", err)
	}

	// 2. A qualifying clear freezes an eligible plan and reserves the reward.
	plan, saved, err = svc.Freeze(ctx, saved, "run-full", 3, now)
	if err != nil || !plan.Eligible {
		t.Fatalf("full clear: plan=%+v err=%v", plan, err)
	}
	if len(plan.Items) != 2 {
		t.Fatalf("plan items: %+v", plan.Items)
	}
	if plan.Items[0].Category != "party_card" || plan.Items[1].Category != "squad_item" {
		t.Fatalf("category order mutated: %+v", plan.Items)
	}

	// 3. The claim pays the plan's Products (three rows, same stackable →
	//    one merged stack of 3) and consumes the plan; the event replay
	//    restores the persisted receipt without touching the state.
	receipts, claimed, err := svc.Claim(ctx, saved, "run-full", now)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(receipts) != 2 {
		t.Fatalf("claim receipts: %+v", receipts)
	}
	bag, err := inventory.ReadBag(claimed.State)
	if err != nil || len(bag.Items) != 1 || bag.Items[0].Amount != 2 {
		t.Fatalf("granted bag: %+v, err=%v", bag.Items, err)
	}
	store.replay = true
	_, replayed, err := svc.Claim(ctx, claimed, "run-full", now)
	if err != nil || string(replayed.State) != string(claimed.State) {
		t.Fatalf("claim replay: %v", err)
	}
	store.replay = false

	// 4. The weekly reward slot was reserved at freeze: a second qualifying
	//    clear the same week earns nothing and only burns a clear.
	plan, saved2, err := svc.Freeze(ctx, claimed, "run-second", 3, now)
	if err != nil || plan.Eligible {
		t.Fatalf("second clear must be rewardless: %+v err=%v", plan, err)
	}
	_ = saved2

	// 5. A claim for a run that never froze belongs to nobody.
	if _, _, err := svc.Claim(ctx, claimed, "run-ghost", now); err == nil || !strings.Contains(err.Error(), "no owned source raid reward plan") {
		t.Fatalf("ghost claim: %v", err)
	}
}

func TestBakalRewardWeekRolloverKeepsPendingPlans(t *testing.T) {
	week1 := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	week2 := week1.AddDate(0, 0, 8)
	role := database.Character{ID: 7, AccountID: 2, ConfigVersion: savecontract.Identity(), State: json.RawMessage(`{}`)}
	svc, store := bakalTestService(t, role)
	ctx := context.Background()

	plan, saved, err := svc.Freeze(ctx, store.role, "run-late", 3, week1)
	if err != nil || !plan.Eligible {
		t.Fatalf("freeze: %+v %v", plan, err)
	}
	// Next week the counters reset: the same weekly limits admit again.
	plan, saved, err = svc.Freeze(ctx, saved, "run-nextweek", 3, week2)
	if err != nil || !plan.Eligible {
		t.Fatalf("rollover freeze: %+v %v", plan, err)
	}
	if plan.Week != bakalWeekOf(week2) {
		t.Fatalf("rollover week stamp: %q", plan.Week)
	}
	// The plan frozen last week still pays out (Claim never re-checks the
	// week — A-layer evidence).
	if _, _, err := svc.Claim(ctx, saved, "run-late", week2); err != nil {
		t.Fatalf("cross-week claim: %v", err)
	}
}

func TestBakalRewardClaimSaveContractGate(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	role := database.Character{ID: 7, AccountID: 2, ConfigVersion: savecontract.Identity(), State: json.RawMessage(`{}`)}
	svc, store := bakalTestService(t, role)
	ctx := context.Background()
	if _, _, err := svc.Freeze(ctx, store.role, "run-id", 3, now); err != nil {
		t.Fatal(err)
	}
	// A character from another save-contract generation cannot claim. The fake
	// applies against its own stored role, so the aged identity has to be
	// stamped onto the store's copy, not just the call argument.
	store.role.ConfigVersion = "deadbeef"
	if _, _, err := svc.Claim(ctx, store.role, "run-id", now); err == nil || !strings.Contains(err.Error(), "no owned source raid reward plan") {
		t.Fatalf("foreign save identity: %v", err)
	}
	store.role.ConfigVersion = savecontract.Identity()
	// Recover claims everything pending; with nothing left it is a no-op.
	if _, err := svc.Recover(ctx, store.role, now); err != nil {
		t.Fatalf("recover pending: %v", err)
	}
	if _, _, err := svc.Claim(ctx, store.role, "run-id", now); err == nil || !strings.Contains(err.Error(), "no owned source raid reward plan") {
		t.Fatalf("recover must have claimed run-id: %v", err)
	}
	if _, err := svc.Recover(ctx, store.role, now); err != nil {
		t.Fatalf("recover after claim: %v", err)
	}
}
