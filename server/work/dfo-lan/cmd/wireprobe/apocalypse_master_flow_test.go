package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
	"time"
)

func masterFlowWorld(t *testing.T) *worldSession {
	t.Helper()
	w := apocalypseTownSession(t, 7)
	s := apocalypseSession(t)
	w.legion, s.world = s, w
	f := ispinsSQLiteRewardWorld(t)
	w.store, w.account, w.role, w.loot = f.store, f.account, f.role, f.loot
	w.dungeons.Source = w.loot.Catalog.Source
	w.dungeons.LegionContents = f.dungeons.LegionContents
	w.dungeons.LegionContents.SHA256 = strings.Repeat("b", 64)
	// Keep the real CTP route and operation identity; only inventory products
	// use a two-item SQLite fixture.
	op := s.catalog.Operation(3)
	op.Reward.Values = []float64{22, 3, 1000000, 1, 1}
	c := catalog.LegionContent{Name: "Apocalypse"}
	c.Rewards = []catalog.LegionRewardRow{{PartyType: "solo", Difficulty: "master", Kind: "basic", Values: []int64{1000000, 77, 1}}}
	w.dungeons.LegionContents.Contents[c.Name] = c
	w.loot.RewardBoxes = apocalypseTestBoxes{77: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 21, Count: 8, Weight: 1}}}}}}
	return w
}

func clearMasterRoom(t *testing.T, w *worldSession, elapsed time.Duration) {
	t.Helper()
	d := w.activeDungeon
	stage := legion.ApocalypseStageOfDungeon(d.Definition.ID)
	d.Loaded = true
	w.apocalypse.StageClock[stage] = time.Now().Add(-elapsed)
	for _, m := range d.Monsters {
		if _, err := d.ConfirmDeath(uint32(m.Entity), w.role.WireID, w.role.WireID); err != nil {
			t.Fatal(err)
		}
	}
	w.apocalypseStageProjection(func(map[string]any) {})
}

func TestApocalypseMasterBranchMechanismAndSQLiteSettlement(t *testing.T) {
	for _, target := range []byte{2, 3, 4, 5} {
		t.Run(string(rune('0'+target)), func(t *testing.T) {
			w := masterFlowWorld(t)
			s := w.legion
			for _, req := range []struct {
				id uint16
				p  []byte
			}{{legion.CmdStart, startPayload(107)}, {legion.CmdOperationSelect, operationPayload(2, 2, 107)}, {legion.CmdEnterDungeon, enterPayload(107, 0)}} {
				if _, e := s.handle(w, req.p, req.id); e != nil {
					t.Fatal(e)
				}
			}
			clearMasterRoom(t, w, time.Second)
			if plan, _ := w.apocalypseAdvanceDue(time.Now().Add(time.Second)); len(plan) == 0 {
				t.Fatal("navigation did not advance")
			}
			if _, e := s.ApocalypseDirectMove(w, legion.ApocalypseStageDungeons[target]); e == nil {
				t.Fatal("living first boss bypassed")
			}
			clearMasterRoom(t, w, time.Second)
			if !w.apocalypse.MasterAwaitingGate || w.apocalypseAdvancePending != nil && !w.apocalypseAdvancePending.done {
				t.Fatal("branch was automatically chosen")
			}
			if _, e := s.ApocalypseDirectMove(w, legion.ApocalypseStageDungeons[target]); e != nil {
				t.Fatal(e)
			}
			for {
				stage := legion.ApocalypseStageOfDungeon(w.activeDungeon.Definition.ID)
				if limit, ok := w.apocalypseCoinLimit(); !ok || limit != 0 {
					t.Fatal("master enabled revives")
				}
				clearMasterRoom(t, w, time.Second)
				if stage == 5 {
					break
				}
				if plan, _ := w.apocalypseAdvanceDue(time.Now().Add(4 * time.Second)); len(plan) == 0 {
					t.Fatal("selected branch stuck", stage)
				}
			}
			if _, e := w.completeDungeon(); e != nil {
				t.Fatal(e)
			}
			plan, _ := w.apocalypseSettlementDue(time.Now().Add(time.Second))
			if !w.apocalypse.Rewarded || !hasPacket(plan, 2252) || !hasPacket(plan, 2253) {
				t.Fatal("master not settled")
			}
			before, _ := inventory.ReadBag(w.role.State)
			if len(before.Items) != 2 || before.Items[0].Amount != 3 || before.Items[1].Amount != 8 {
				t.Fatal("master products", before)
			}
			end := make([]byte, 22)
			binary.LittleEndian.PutUint32(end[13:], 107)
			binary.LittleEndian.PutUint32(end[17:], 5)
			for i := 0; i < 2; i++ {
				if _, e := s.handle(w, end, legion.CmdRewardEnd); e != nil {
					t.Fatal(e)
				}
			}
			after, _ := inventory.ReadBag(w.role.State)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("repeated master award")
			}
			story, _, e := w.apocalypseStoryPause([]byte{1, 0})
			if e != nil || len(story) != 2 || binary.LittleEndian.Uint32(story[1].Payload[13:]) != 5 {
				t.Fatal("master movie return stage", e)
			}
			if _, exit, e := w.settlementExit([]byte{1, 2, 1}); e != nil || len(exit) == 0 || w.apocalypse.Choice != 255 {
				t.Fatal("master town return", e)
			}
		})
	}
}

func TestApocalypseMasterRejectsClosedBranch(t *testing.T) {
	for _, elapsed := range []time.Duration{46 * time.Second, 61 * time.Second, 121 * time.Second} {
		w := masterFlowWorld(t)
		s := w.legion
		for _, req := range []struct {
			id uint16
			p  []byte
		}{{legion.CmdStart, startPayload(107)}, {legion.CmdOperationSelect, operationPayload(2, 2, 107)}, {legion.CmdEnterDungeon, enterPayload(107, 0)}} {
			if _, e := s.handle(w, req.p, req.id); e != nil {
				t.Fatal(e)
			}
		}
		clearMasterRoom(t, w, time.Second)
		w.apocalypseAdvanceDue(time.Now().Add(time.Second))
		clearMasterRoom(t, w, elapsed)
		for target := byte(2); target <= 5; target++ {
			if w.apocalypse.MasterGateBoard[target-2] != 0 {
				continue
			}
			before := w.activeDungeon
			if _, e := s.ApocalypseDirectMove(w, legion.ApocalypseStageDungeons[target]); e == nil || w.activeDungeon != before || !w.apocalypse.MasterAwaitingGate {
				t.Fatal("closed branch entered", elapsed, target, e)
			}
		}
		if events := w.apocalypseAdvanceSweep(time.Now()); len(events) != 0 {
			t.Fatal("sweep selected a branch")
		}
	}
}
