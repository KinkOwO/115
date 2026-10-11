package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func moonAutoFixture(t *testing.T) (*worldSession, time.Time) {
	t.Helper()
	svc, db, role := awakeningSQLiteFixture(t, 1)
	role.WireID = 7
	now := time.Unix(1800000000, 0)
	c := catalog.DungeonCatalog{Source: svc.Catalog.Source, Dungeons: map[uint32]catalog.DungeonDefinition{
		100004136: {ID: 100004136, BasisLevel: 115, Mazes: []catalog.DungeonMaze{{Size: [2]byte{1, 1}, Rooms: []catalog.DungeonRoom{{Map: 1}}}}},
		100004137: {ID: 100004137, BasisLevel: 115, Mazes: []catalog.DungeonMaze{{Size: [2]byte{5, 3}, Rooms: []catalog.DungeonRoom{{X: 3, Map: 2}}}}},
	}, Maps: map[uint32]catalog.ScriptRecord{1: {}, 2: {}}}
	owner, err := dungeon.NewMoonSolo(c, 115, 7, 0, now)
	if err != nil {
		t.Fatal(err)
	}
	run := owner.Session()
	run.Definition.ID = 100004137
	run.MarkCompleted()
	ls := &loot.Service{Catalog: catalog.LootCatalog{Source: svc.Catalog.Source, Items: map[uint32]catalog.LootItem{101: {ID: 101, Kind: "stackable", StackableType: "material", StackLimit: 10}, 102: {ID: 102, Kind: "stackable", StackableType: "material", StackLimit: 10}}}, BagRules: inventory.BagRules{Source: role.ConfigVersion, Slots: map[string][2]uint16{"material": {121, 121}}, MissingStackLimit: 10}}
	p := loot.MoonRewardPlan{Source: role.ConfigVersion, Run: run.RunID, Rules: strings.Repeat("a", 64), Account: role.AccountID, Character: role.ID, Grants: []loot.MoonRewardGrant{{Template: 101, Count: 2}}}
	raw, _ := json.Marshal(p)
	_, _, err = db.CommitCharacterEvent(context.Background(), role.AccountID, role.ID, role.ConfigVersion, "moon-clear:"+run.RunID, loot.MoonRewardModel, func(cur database.Character) (json.RawMessage, json.RawMessage, error) { return cur.State, raw, nil })
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{role: role, account: role.AccountID, characters: svc, store: db, loot: ls, activeDungeon: run, moonConfig: &moonSoloConfig{}, resultSent: true, cardLayoutSent: true, cardScrolled: true, moon: moonSoloState{created: true, owner: owner, plan: &p, autoPickAt: now, autoPickRun: run.RunID, autoPickCharacter: role.ID}}
	return w, now
}
func TestMoonAutoPickCommitsOnceAndManualPickCancels(t *testing.T) {
	for _, manual := range []bool{false, true} {
		t.Run(fmt.Sprint(manual), func(t *testing.T) {
			w, now := moonAutoFixture(t)
			if packets, err := w.autoPickMoonCard(now.Add(-time.Second)); err != nil || len(packets) > 0 || w.moon.autoPickAt.IsZero() {
				t.Fatal("early tick", err)
			}
			var packets []outboundPacket
			var err error
			if manual {
				packets, err = w.moonClaim(1)
			} else {
				packets, err = w.autoPickMoonCard(now)
			}
			if err != nil || len(packets) != 3 || !w.moon.claimed || !w.moon.autoPickAt.IsZero() {
				t.Fatal("claim", err, len(packets))
			}
			saved, err := w.store.AdminCharacter(context.Background(), w.role.ID)
			if err != nil {
				t.Fatal(err)
			}
			bag, err := inventory.ReadBag(saved.State)
			if err != nil || len(bag.Items) != 1 || bag.Items[0].Amount != 2 || bag.Gold != 12345 {
				t.Fatal("grant", err, bag)
			}
			before := string(saved.State)
			if packets, err = w.autoPickMoonCard(now.Add(time.Minute)); err != nil || len(packets) > 0 {
				t.Fatal("repeat tick", err)
			}
			saved, err = w.store.AdminCharacter(context.Background(), w.role.ID)
			if err != nil || string(saved.State) != before {
				t.Fatal("duplicate mutation", err)
			}
		})
	}
}
func TestMoonAutoPickFullBagRetainsFrozenPlanAndRetries(t *testing.T) {
	w, now := moonAutoFixture(t)
	state, err := inventory.SaveBag(w.role.State, inventory.Bag{Version: "ordinary-bag-v1", Gold: 12345, Items: []inventory.BagItem{{Slot: 121, Template: 102, Amount: 10}}})
	if err != nil {
		t.Fatal(err)
	}
	role, _, err := w.store.CommitCharacterEvent(context.Background(), w.role.AccountID, w.role.ID, w.role.ConfigVersion, "fixture:full", "fixture", func(database.Character) (json.RawMessage, json.RawMessage, error) {
		return state, json.RawMessage(`{}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	role.WireID = 7
	w.role = role
	if packets, err := w.autoPickMoonCard(now); !errors.Is(err, loot.ErrMoonBagFull) || len(packets) > 0 || w.moon.claimed || !w.moon.autoPickAt.Equal(now.Add(5*time.Second)) {
		t.Fatal("full bag", err)
	}
	pending, err := w.store.PendingMoonRewardRuns(context.Background(), role.ID, loot.MoonRewardModel)
	if err != nil || len(pending) != 1 {
		t.Fatal("frozen reward lost", err, pending)
	}
	saved, _ := w.store.AdminCharacter(context.Background(), role.ID)
	if string(saved.State) != string(state) {
		t.Fatal("partial award")
	}
	state, err = inventory.SaveBag(role.State, inventory.Bag{Version: "ordinary-bag-v1", Gold: 12345})
	if err != nil {
		t.Fatal(err)
	}
	role, _, err = w.store.CommitCharacterEvent(context.Background(), role.AccountID, role.ID, role.ConfigVersion, "fixture:space", "fixture", func(database.Character) (json.RawMessage, json.RawMessage, error) {
		return state, json.RawMessage(`{}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	role.WireID = 7
	w.role = role
	if packets, err := w.autoPickMoonCard(now.Add(5 * time.Second)); err != nil || len(packets) != 3 || !w.moon.claimed {
		t.Fatal("retry", err)
	}
}
func TestMoonAutoPickRejectsStaleRunCharacterAndPlan(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*worldSession)
	}{
		{"run", func(w *worldSession) { w.moon.autoPickRun = "old" }},
		{"character", func(w *worldSession) { w.moon.autoPickCharacter++ }},
		{"foreign-plan", func(w *worldSession) { w.moon.plan.Account++ }},
		{"plan-run", func(w *worldSession) { w.moon.plan.Run = "other" }},
		{"source", func(w *worldSession) { w.moon.plan.Source = "other" }},
		{"no-plan", func(w *worldSession) { w.moon.plan = nil }},
		{"returned", func(w *worldSession) { w.activeDungeon = nil }},
		{"new-owner", func(w *worldSession) { v := *w.activeDungeon; w.activeDungeon = &v }},
		{"no-result", func(w *worldSession) { w.resultSent = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, now := moonAutoFixture(t)
			tc.change(w)
			before, _ := w.store.AdminCharacter(context.Background(), w.role.ID)
			if p, err := w.autoPickMoonCard(now); err != nil || len(p) > 0 || !w.moon.autoPickAt.IsZero() {
				t.Fatal("stale tick", err)
			}
			after, _ := w.store.AdminCharacter(context.Background(), w.role.ID)
			if string(after.State) != string(before.State) {
				t.Fatal("stale tick changed assets")
			}
		})
	}
}
func TestMoonCardLayoutReplayDoesNotPostponeDeadline(t *testing.T) {
	w, now := moonAutoFixture(t)
	w.cardLayoutSent = false
	w.moon.autoPickAt = time.Time{}
	if handled, _, err := w.moonHandle(70, nil, now, func(map[string]any) {}); err != nil || !handled {
		t.Fatal(err)
	}
	deadline := w.moon.autoPickAt
	if !deadline.Equal(now.Add(3 * time.Second)) {
		t.Fatal("layout did not schedule")
	}
	if _, _, err := w.moonHandle(70, nil, now.Add(time.Second), func(map[string]any) {}); err != nil || !w.moon.autoPickAt.Equal(deadline) {
		t.Fatal("replay postponed deadline", err)
	}
}
func TestMoonPreparedReplyFailureDoesNotCommitAward(t *testing.T) {
	w, _ := moonAutoFixture(t)
	before, _ := w.store.AdminCharacter(context.Background(), w.role.ID)
	reject := errors.New("reply preparation rejected")
	_, _, err := (&workflow.LootService{Store: w.store, Loot: w.loot}).ClaimMoonRewardPrepared(context.Background(), w.role, w.moon.plan.Run, func(database.Character) error { return reject })
	if !errors.Is(err, reject) {
		t.Fatal(err)
	}
	after, _ := w.store.AdminCharacter(context.Background(), w.role.ID)
	if string(after.State) != string(before.State) {
		t.Fatal("failed reply committed assets")
	}
	pending, err := w.store.PendingMoonRewardRuns(context.Background(), w.role.ID, loot.MoonRewardModel)
	if err != nil || len(pending) != 1 {
		t.Fatal("reward no longer recoverable", err)
	}
}
