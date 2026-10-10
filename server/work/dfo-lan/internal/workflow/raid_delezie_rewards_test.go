package workflow

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func delezieRewardServiceFixture(t *testing.T) (*DelezieRewardService, *bakalRewardStoreFake, *catalog.DelezieRaidRules) {
	t.Helper()
	hash := strings.Repeat("a", 64)
	store := &bakalRewardStoreFake{role: database.Character{ID: 7, AccountID: 2, WireID: 9, ConfigVersion: (pvf.ArchiveSnapshot{}).SaveIdentity(), State: json.RawMessage(`{"unrelated":{"keep":true},"bakal_raid_rewards":{"keep":"other raid"}}`)}, receipts: map[string]json.RawMessage{}}
	boxes := bakalWrapperSource{
		11: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 21, Count: 4, Weight: 1}}}}},
		12: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 22, Count: 3, Weight: 1}}}}},
		13: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 21, Count: 1, Weight: 1}}}}},
	}
	svc := &DelezieRewardService{Store: store, Boxes: boxes, Awarder: &inventory.Awarder{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: hash}, Items: map[uint32]catalog.LootItem{21: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}, 22: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}}}, Rules: inventory.BagRules{Source: hash, Slots: map[string][2]uint16{"[etc]": {68, 69}}}}}
	header, err := pvf.NewCTPTree(&pvf.CTPTable{Path: "fixture.ctp", Records: []pvf.CTPRecord{
		{Index: 0, Parent: -1, Name: "[WEEKLY CLEAR COUNT]", Cells: []pvf.CTPCell{{Kind: "float", Float: 1}}},
		{Index: 1, Parent: -1, Name: "[WEEKLY REWARD COUNT]", Cells: []pvf.CTPCell{{Kind: "float", Float: 1}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	rules := &catalog.DelezieRaidRules{Source: pvf.ArchiveSnapshot{Checksum: hash}, Header: header, Modes: map[catalog.DelezieDifficulty]*catalog.DelezieModeRules{}, Rewards: &catalog.DelezieRewardRules{Path: "fixture.etc", Kind: 14, Modes: map[catalog.DelezieDifficulty]catalog.DelezieModeRewards{}}}
	for d := catalog.DelezieNormal; d <= catalog.DelezieMatch; d++ {
		base := 100 * int(d)
		rules.Modes[d] = &catalog.DelezieModeRules{Difficulty: d}
		program := catalog.DelezieModeRewards{Rewards: []catalog.DelezieRewardRow{
			{Row: base + 1, Kind: "party_card", Group: -1, Weight: 100, Template: 11},
			{Row: base + 2, Kind: "squad_item", Group: -1, Weight: 100, Template: 12},
			{Row: base + 3, Kind: "pcroom_card", Group: -1, Weight: 100, Template: 13},
		}}
		if d == catalog.DelezieNormal || d == catalog.DelezieHard {
			program.Rewards = append(program.Rewards, catalog.DelezieRewardRow{Row: base + 4, Kind: "special_item", Group: -1, Weight: 999, Template: -1}, catalog.DelezieRewardRow{Row: base + 5, Kind: "special_item", Group: -1, Weight: 1, Template: 21})
			program.Common = []catalog.DelezieCommonRewardRow{{Row: base + 6, Kind: "squad_item", Weight: 999, Template: -1}, {Row: base + 7, Kind: "squad_item", Weight: 1, Template: 22}}
		}
		if d == catalog.DelezieHard {
			program.Rewards = append(program.Rewards, catalog.DelezieRewardRow{Row: base + 8, Kind: "bidding_item", Group: 0, Weight: 1, Template: 999})
		}
		rules.Rewards.Modes[d] = program
	}
	return svc, store, rules
}

func delezieFixtureSelection(d catalog.DelezieDifficulty) DelezieRewardSelection {
	base := int(d) * 100
	selection := DelezieRewardSelection{Rows: []int{base + 1, base + 2}}
	if d == catalog.DelezieNormal || d == catalog.DelezieHard {
		selection.Rows = append(selection.Rows, base+4)
		selection.CommonRows = []int{base + 6}
	}
	return selection
}

func TestDelezieRewardFreezeClaimRecoveryAllModes(t *testing.T) {
	for d := catalog.DelezieNormal; d <= catalog.DelezieMatch; d++ {
		t.Run(d.Name(), func(t *testing.T) {
			svc, store, rules := delezieRewardServiceFixture(t)
			ctx := context.Background()
			now := time.Date(2026, 10, 5, 1, 0, 0, 0, time.UTC)
			selection := delezieFixtureSelection(d)
			saved, plan, err := svc.FreezeResolvedAwards(ctx, store.role, rules, rules.Modes[d], "fixture", selection, now)
			if err != nil || len(plan.Products) != 2 || plan.Mode != d || plan.AuctionPending != (d == catalog.DelezieHard) {
				t.Fatal(plan, err)
			}
			// Two occupied slots make the whole grant fail. No partial first
			// item, consumed receipt or lost pending parcel is allowed.
			bag, _ := inventory.ReadBag(store.role.State)
			for _, occupied := range [][]inventory.BagItem{
				{{Slot: 68, Template: 13, Amount: 1}, {Slot: 69, Template: 13, Amount: 1}},
				{{Slot: 69, Template: 13, Amount: 1}}, // first item fits, second fails
			} {
				bag.Items = occupied
				store.role.State, _ = inventory.SaveBag(store.role.State, bag)
				before := string(store.role.State)
				if _, _, err = svc.ClaimPersonal(ctx, saved, "fixture"); err == nil || string(store.role.State) != before || len(store.receipts) != 1 {
					t.Fatal("failed grant was not atomic", err)
				}
			}
			// Mutating the source resolver after freeze cannot change retry.
			svc.Boxes.(bakalWrapperSource)[11] = loot.RewardBox{Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 21, Count: 99, Weight: 1}}}}}
			_, replay, err := svc.FreezeResolvedAwards(ctx, store.role, rules, rules.Modes[d], "fixture", selection, now)
			if err != nil || !reflect.DeepEqual(replay.Products, plan.Products) {
				t.Fatal("rerolled pending rewards", replay, err)
			}
			bag.Items = nil
			store.role.State, _ = inventory.SaveBag(store.role.State, bag)
			saved, err = svc.RecoverPersonal(ctx, store.role)
			if err != nil {
				t.Fatal(err)
			}
			bag, err = inventory.ReadBag(saved.State)
			amounts := map[uint32]uint32{}
			for _, item := range bag.Items {
				amounts[item.Template] += item.Amount
			}
			if err != nil || amounts[21] != 4 || amounts[22] != 3 || len(store.receipts) != 2 {
				t.Fatal(bag, err)
			}
			again, receipts, err := svc.ClaimPersonal(ctx, saved, "fixture")
			if err != nil || len(receipts) != 2 || string(again.State) != string(saved.State) {
				t.Fatal("claim replay changed bag", receipts, err)
			}
			_, p, err := readDelezieProgress(saved.State)
			if err != nil || p.Clears != 1 || p.Rewards != 1 {
				t.Fatal(p, err)
			}
			if d == catalog.DelezieHard {
				if len(p.Pending) != 1 || !p.Pending["fixture"].PersonalClaimed || !p.Pending["fixture"].AuctionPending {
					t.Fatal("lost pending auction obligation", p)
				}
			} else if len(p.Pending) != 0 {
				t.Fatal("personal rewards not closed", p)
			}
			if !strings.Contains(string(saved.State), `"unrelated":{"keep":true}`) || !strings.Contains(string(saved.State), `"bakal_raid_rewards":{"keep":"other raid"}`) {
				t.Fatal("unrelated old save fields changed")
			}
		})
	}
}

func TestDelezieRewardRejectsUnownedIncompleteAndConflictingResults(t *testing.T) {
	svc, store, rules := delezieRewardServiceFixture(t)
	ctx, now := context.Background(), time.Date(2026, 10, 5, 1, 0, 0, 0, time.UTC)
	for _, selection := range []DelezieRewardSelection{
		{Rows: []int{1, 2, 4}}, // missing common result
		{Rows: []int{1, 2, 4}, CommonRows: []int{999}},
		{Rows: []int{1, 1, 2, 4}, CommonRows: []int{6}},
		{Rows: []int{201, 202}, CommonRows: []int{6}},   // other mode
		{Rows: []int{1, 2, 3, 4}, CommonRows: []int{6}}, // no PC-room eligibility
	} {
		if _, _, err := svc.FreezeResolvedAwards(ctx, store.role, rules, rules.Modes[catalog.DelezieNormal], "fixture", selection, now); err == nil || len(store.receipts) != 0 {
			t.Fatal("invalid result committed", selection, err)
		}
	}
	selection := delezieFixtureSelection(catalog.DelezieNormal)
	saved, _, err := svc.FreezeResolvedAwards(ctx, store.role, rules, rules.Modes[catalog.DelezieNormal], "fixture", selection, now)
	if err != nil {
		t.Fatal(err)
	}
	before := string(store.role.State)
	selection.Rows[2] = 5
	if _, _, err := svc.FreezeResolvedAwards(ctx, saved, rules, rules.Modes[catalog.DelezieNormal], "fixture", selection, now); err == nil || string(store.role.State) != before {
		t.Fatal("changed draw accepted on replay", err)
	}
	if _, _, err := svc.FreezeResolvedAwards(ctx, saved, rules, rules.Modes[catalog.DelezieSingle], "fixture", delezieFixtureSelection(catalog.DelezieSingle), now); err == nil || string(store.role.State) != before {
		t.Fatal("changed mode accepted on replay", err)
	}
	if _, _, err := svc.FreezeResolvedAwards(ctx, saved, rules, rules.Modes[catalog.DelezieSingle], "second", delezieFixtureSelection(catalog.DelezieSingle), now); !errors.Is(err, ErrDelezieWeeklyLimit) {
		t.Fatal("mode changed weekly cap", err)
	}
	if err := DelezieRaidAdmission(saved, rules, now); !errors.Is(err, ErrDelezieWeeklyLimit) {
		t.Fatal("admission ignored cap", err)
	}
	// The new week keeps old unclaimed parcels while allowing its own clear.
	saved, _, err = svc.FreezeResolvedAwards(ctx, saved, rules, rules.Modes[catalog.DelezieSingle], "next-week", delezieFixtureSelection(catalog.DelezieSingle), now.Add(7*24*time.Hour))
	_, progress, _ := readDelezieProgress(saved.State)
	if err != nil || len(progress.Pending) != 2 || progress.Clears != 1 {
		t.Fatal(progress, err)
	}
}

func TestDelezieRewardMissingWrappersAndSourceMismatchDoNotCommit(t *testing.T) {
	svc, store, rules := delezieRewardServiceFixture(t)
	ctx, now := context.Background(), time.Date(2026, 10, 5, 1, 0, 0, 0, time.UTC)
	svc.Boxes = nil
	before := string(store.role.State)
	if _, _, err := svc.FreezeResolvedAwards(ctx, store.role, rules, rules.Modes[catalog.DelezieSingle], "fixture", delezieFixtureSelection(catalog.DelezieSingle), now); err == nil || len(store.receipts) != 0 || string(store.role.State) != before {
		t.Fatal("unresolved wrapper consumed clear", err)
	}
	rules.Source.Checksum = strings.Repeat("b", 64)
	if _, _, err := svc.FreezeResolvedAwards(ctx, store.role, rules, rules.Modes[catalog.DelezieSingle], "fixture", delezieFixtureSelection(catalog.DelezieSingle), now); err == nil || len(store.receipts) != 0 {
		t.Fatal("mixed source accepted", err)
	}
}

func TestDelezieResolvedRewardExtraProductsAndAuctionOwnership(t *testing.T) {
	svc, store, rules := delezieRewardServiceFixture(t)
	ctx, now := context.Background(), time.Date(2026, 10, 5, 1, 0, 0, 0, time.UTC)
	selection := DelezieRewardSelection{Rows: []int{1, 2, 3, 5}, CommonRows: []int{7}, PCRoom: true}
	saved, plan, err := svc.FreezeResolvedAwards(ctx, store.role, rules, rules.Modes[catalog.DelezieNormal], "fixture", selection, now)
	if err != nil || len(plan.Rows) != 4 || len(plan.Common) != 1 {
		t.Fatal(plan, err)
	}
	saved, _, err = svc.ClaimPersonal(ctx, saved, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	bag, _ := inventory.ReadBag(saved.State)
	amounts := map[uint32]uint32{}
	for _, item := range bag.Items {
		amounts[item.Template] += item.Amount
	}
	if amounts[21] != 6 || amounts[22] != 4 {
		t.Fatal("resolved source extras lost", amounts)
	}

	svc, store, rules = delezieRewardServiceFixture(t)
	selection = delezieFixtureSelection(catalog.DelezieHard)
	selection.Rows = append(selection.Rows, 108)
	if _, _, err := svc.FreezeResolvedAwards(ctx, store.role, rules, rules.Modes[catalog.DelezieHard], "fixture", selection, now); err == nil || len(store.receipts) != 0 {
		t.Fatal("auction granted as a personal reward", err)
	}
	program := rules.Rewards.Modes[catalog.DelezieNormal]
	program.Common[0].Kind = "bidding_item"
	rules.Rewards.Modes[catalog.DelezieNormal] = program
	if _, _, err := svc.FreezeResolvedAwards(ctx, store.role, rules, rules.Modes[catalog.DelezieNormal], "fixture", delezieFixtureSelection(catalog.DelezieNormal), now); err == nil || len(store.receipts) != 0 {
		t.Fatal("unrecognized common reward silently granted", err)
	}
}
