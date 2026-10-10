package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/workflow"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestElvenmereWeeklyResetKeepsSeasonAndRejectsClockRollback(t *testing.T) {
	w, f := elvenmereRewardFixture(t, false)
	ctx := context.Background()
	boundary := time.Unix(dungeon.ElvenmereWeekStart(time.Now()), 0).AddDate(0, 0, 7)
	now := boundary.Add(-time.Nanosecond)
	svc := &workflow.LootService{Store: w.store, Loot: w.loot, ElvenmereNow: func() time.Time { return now }}
	g := workflow.ElvenmereFloorGrant{Run: strings.Repeat("a", 32), Floor: 5, WeeklyItem: 10336354, WeeklyCount: 5, SeasonItem: 10333027, SeasonCount: 1}
	first, _, err := svc.CommitElvenmereFloor(ctx, w.role, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	now = boundary
	projection, err := svc.RestoreElvenmereProgress(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	p, _, err := dungeon.ReadElvenmereProgress(projection.State)
	if err != nil || p.MaxCleared != 0 || len(p.Weekly) != 0 || p.Season[5].EventKey == "" {
		t.Fatal("weekly reset changed the wrong scope", p, err)
	}
	// Same RunID also cannot pin rewards permanently to a past week.
	second, receipt, err := svc.CommitElvenmereFloor(ctx, first, g, nil)
	if err != nil || receipt.Grant.WeeklyCount != 5 || receipt.Grant.SeasonCount != 0 {
		t.Fatal(receipt, err)
	}
	if weekly, season := elvenmereHeldItems(t, second); weekly != 10 || season != 1 {
		t.Fatal("reset reward quantities", weekly, season)
	}
	reopened, err := f.Reopen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	svc.Store = reopened
	if again, _, err := svc.CommitElvenmereFloor(ctx, w.role, g, nil); err != nil || string(again.State) != string(second.State) {
		t.Fatal("reopen duplicated current period", err)
	}
	now = boundary.Add(-time.Second)
	g.Run = strings.Repeat("b", 32)
	if _, _, err := svc.CommitElvenmereFloor(ctx, w.role, g, nil); err == nil {
		t.Fatal("clock rollback reactivated prior week")
	}
	if count, err := f.EventCount(ctx, w.role.ID); err != nil || count != 2 {
		t.Fatal("rollback changed events", count, err)
	}
}

func TestElvenmereAuditedSeasonResetPreservesWeekAndIsAccountIdempotent(t *testing.T) {
	w, f := elvenmereRewardFixture(t, false)
	ctx := context.Background()
	now := time.Now().UTC().Add(time.Hour)
	svc := &workflow.LootService{Store: w.store, Loot: w.loot, ElvenmereNow: func() time.Time { return now }}
	g := workflow.ElvenmereFloorGrant{Run: strings.Repeat("a", 32), Floor: 5, WeeklyItem: 10336354, WeeklyCount: 5, SeasonItem: 10333027, SeasonCount: 1}
	first, _, err := svc.CommitElvenmereFloor(ctx, w.role, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	sibling := elvenmereSibling(t, w, w.account, "SeasonSibling")
	now = now.Add(time.Second)
	operation := strings.Repeat("f", 32)
	reset, err := svc.ResetElvenmereSeason(ctx, first, operation, "verified season transition fixture")
	if err != nil {
		t.Fatal(err)
	}
	p, _, err := dungeon.ReadElvenmereProgress(reset.State)
	if err != nil || p.MaxCleared != 5 || p.Weekly[5].EventKey == "" || len(p.Season) != 0 || p.SeasonID != operation {
		t.Fatal("season reset damaged weekly progress", p, err)
	}
	g.Run = strings.Repeat("b", 32)
	second, receipt, err := svc.CommitElvenmereFloor(ctx, first, g, nil)
	if err != nil || receipt.Grant.WeeklyCount != 0 || receipt.Grant.SeasonCount != 1 {
		t.Fatal(receipt, err)
	}
	if weekly, season := elvenmereHeldItems(t, second); weekly != 5 || season != 2 {
		t.Fatal("season reset duplicated weekly reward", weekly, season)
	}
	for _, role := range []database.Character{first, sibling} {
		if _, err := svc.ResetElvenmereSeason(ctx, role, operation, "verified season transition fixture"); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := f.EventCount(ctx, sibling.ID); err != nil || n != 0 {
		t.Fatal("same account operation written twice", n, err)
	}
	if _, err := svc.ResetElvenmereSeason(ctx, first, operation, "different reason"); err == nil {
		t.Fatal("operation identifier reused with different reason")
	}
	if saved, err := w.store.AdminCharacter(ctx, w.role.ID); err != nil || string(saved.State) != string(second.State) {
		t.Fatal("reset replay erased new claims", err)
	}
}

func TestElvenmereSettlementCannotCrossResetWhileWaitingForStorage(t *testing.T) {
	for _, switchAt := range []int{2, 3} {
		t.Run(fmt.Sprint(switchAt), func(t *testing.T) {
			w, f := elvenmereRewardFixture(t, false)
			boundary := time.Unix(dungeon.ElvenmereWeekStart(time.Now()), 0).AddDate(0, 0, 7)
			calls := 0
			svc := &workflow.LootService{Store: w.store, Loot: w.loot, ElvenmereNow: func() time.Time {
				calls++
				if calls >= switchAt {
					return boundary
				}
				return boundary.Add(-time.Second)
			}}
			g := workflow.ElvenmereFloorGrant{Run: strings.Repeat("c", 32), Floor: 5, WeeklyItem: 10336354, WeeklyCount: 5}
			if _, _, err := svc.CommitElvenmereFloor(context.Background(), w.role, g, nil); err == nil {
				t.Fatal("settled across reset")
			}
			if n, err := f.EventCount(context.Background(), w.role.ID); err != nil || n != 0 {
				t.Fatal(n, err)
			}
			saved, err := w.store.AdminCharacter(context.Background(), w.role.ID)
			if err != nil || string(saved.State) != string(w.role.State) {
				t.Fatal("cross-reset settlement changed save", err)
			}
		})
	}
}

func TestElvenmereStaleZoneAndExpiredRunCannotClaim(t *testing.T) {
	w, f := elvenmereRewardFixture(t, false)
	ctx := context.Background()
	now := time.Now().UTC()
	svc := &workflow.LootService{Store: w.store, Loot: w.loot, ElvenmereNow: func() time.Time { return now }}
	g := workflow.ElvenmereFloorGrant{Run: strings.Repeat("a", 32), Floor: 36, StartFloor: 36, WeeklyItem: 10336354, WeeklyCount: 5}
	if _, _, err := svc.CommitElvenmereFloor(ctx, w.role, g, nil); err != nil {
		t.Fatal(err)
	}
	sibling := elvenmereSibling(t, w, w.account, "StaleZone")
	g.Run, g.Floor, g.StartFloor = strings.Repeat("b", 32), 1, 1
	if _, _, err := svc.CommitElvenmereFloor(ctx, sibling, g, nil); err == nil {
		t.Fatal("stale first-zone run bypassed account binding")
	}
	g.Floor, g.StartFloor, g.ExpectedWeekStart = 37, 36, dungeon.ElvenmereWeekStart(now)-7*24*3600
	if _, _, err := svc.CommitElvenmereFloor(ctx, sibling, g, nil); err == nil {
		t.Fatal("old run settled current-week rewards")
	}
	if n, err := f.EventCount(ctx, sibling.ID); err != nil || n != 0 {
		t.Fatal(n, err)
	}
}
