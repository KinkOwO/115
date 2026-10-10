package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"encoding/binary"
	"reflect"
	"testing"
	"time"
)

func forestHardPhaseFixture(stage int) *worldSession {
	run := &forestRun{hard: true, stage: stage, choice: 2, target: 5}
	for i := 0; i < stage; i++ {
		run.cleared[i] = true
	}
	return &worldSession{forest: run, role: database.Character{ID: 2, WireID: 2}, activeDungeon: &dungeon.Session{Loaded: true, Definition: catalog.DungeonDefinition{ID: legion.ForestHardStageDungeons[stage]}}, state: database.WorldState{Position: database.WorldPosition{Town: 198, Area: 3, X: 644, Y: 163}}}
}

func TestForestHardEarlyBossAdvancesWithoutSettlementOrNormalInfo(t *testing.T) {
	for stage := 0; stage < 2; stage++ {
		w := forestHardPhaseFixture(stage)
		// No reward store/catalog is bound: any settlement attempt must fail.
		w.activeDungeon.MarkCompleted()
		plan, err := w.completeDungeon()
		if err != nil || len(plan) != 1 {
			t.Fatalf("phase%d: packets=%v error=%v", stage, plan, err)
		}
		p := plan[0]
		if p.ID != 2565 || len(p.Payload) != 54 || binary.LittleEndian.Uint32(p.Payload[2:]) != 2 || binary.LittleEndian.Uint32(p.Payload[6:]) != 0 || binary.LittleEndian.Uint32(p.Payload[10:]) != uint32(stage+1) {
			t.Fatalf("wrong hard transition: %+v", p)
		}
		if !w.forest.cleared[stage] {
			t.Fatal("progress not committed")
		}
		before := *w.forest
		active := w.activeDungeon
		if replay, err := w.completeDungeon(); err != nil || len(replay) != 0 || !reflect.DeepEqual(before, *w.forest) {
			t.Fatal("duplicate boss reissued transition", err)
		}
		if replay, err := w.dungeonResult(make([]byte, 141)); err != nil || len(replay) != 0 || w.activeDungeon != active || !reflect.DeepEqual(before, *w.forest) {
			t.Fatal("stale result closed hard phase", err)
		}
	}
}

func TestForestResultCannotAdvanceAnUnclearedBoss(t *testing.T) {
	for _, hard := range []bool{false, true} {
		w := forestHardPhaseFixture(0)
		w.forest.hard = hard
		before := *w.forest
		active := w.activeDungeon
		if plan, err := w.dungeonResult(make([]byte, 141)); err == nil || len(plan) != 0 || w.activeDungeon != active || !reflect.DeepEqual(before, *w.forest) {
			t.Fatal("premature result changed run", hard, err)
		}
	}
}

func TestForestHardLoadingUsesNativeModeWithoutRewindingClear(t *testing.T) {
	for phase := 0; phase < 3; phase++ {
		w := forestHardPhaseFixture(phase)
		now := time.Unix(1791319000, 0)
		plan := w.forestStageTimer(now)
		if len(plan) != 2 || plan[0].ID != 2565 || binary.LittleEndian.Uint32(plan[0].Payload[10:]) != uint32(phase) || plan[1].ID != 1474 {
			t.Fatal("loading did not activate hard manager", phase, plan)
		}
		if !w.forest.stageClock[phase].Equal(now) {
			t.Fatal("phase clock not frozen")
		}
		w.forestStageTimer(now.Add(time.Second))
		if !w.forest.stageClock[phase].Equal(now) {
			t.Fatal("loading replay reset phase clock")
		}
		w.activeDungeon.MarkCompleted()
		if plan := w.forestStageTimer(now.Add(time.Second)); len(plan) != 0 {
			t.Fatal("loading replay regressed cleared phase")
		}
	}
}

func TestForestHardFinalRequiresAllBossesAndReplaysWithoutReward(t *testing.T) {
	p := make([]byte, 21)
	binary.LittleEndian.PutUint32(p[13:], 105)
	binary.LittleEndian.PutUint32(p[17:], 2)
	w := forestHardPhaseFixture(2)
	if _, _, err := w.forestRewardEnd(p); err == nil {
		t.Fatal("uncleared final accepted")
	}
	w.forest.cleared[2] = true
	w.forest.cleared[0] = false
	if _, _, err := w.forestRewardEnd(p); err == nil {
		t.Fatal("missing first boss accepted at final")
	}
	w.forest.cleared[0] = true
	plan, _, err := w.forestRewardEnd(p)
	if err != nil || len(plan) != 2 || plan[1].ID != 2565 || binary.LittleEndian.Uint32(plan[1].Payload[2:]) != 3 || binary.LittleEndian.Uint32(plan[1].Payload[6:]) != 1 {
		t.Fatal("wrong final state", plan, err)
	}
	again, _, err := w.forestRewardEnd(p)
	if err != nil || len(again) != 1 || again[0].ID != 2046 {
		t.Fatal("final replay unstable", err)
	}
}

func TestForestRetryKeepsPotionUseAndPhaseClock(t *testing.T) {
	for _, direct := range []bool{false, true} {
		for phase := 0; phase < 3; phase++ {
			w := forestHardPhaseFixture(phase)
			w.activeDungeon = nil
			id := legion.ForestHardStageDungeons[phase]
			w.dungeons = &catalog.DungeonCatalog{
				Dungeons:        map[uint32]catalog.DungeonDefinition{id: {ID: id, Mazes: []catalog.DungeonMaze{{Rooms: []catalog.DungeonRoom{{Map: 1, Boss: true}}}}}},
				Maps:            map[uint32]catalog.ScriptRecord{1: {}},
				StackableLimits: &catalog.StackableDungeonLimits{Dungeons: map[uint32]uint32{id: 4}},
			}
			started := time.Unix(1791319000, 0)
			w.forest.stageClock[phase] = started
			w.forest.potionUsed[phase] = 3
			if direct {
				if _, plan, err := w.enterForestStageDirectMove(protocol.DungeonDirectMove{ID: id}); err != nil || len(plan) == 0 {
					t.Fatal("retry direct entry failed", phase, err)
				}
			} else {
				p := make([]byte, 21)
				binary.LittleEndian.PutUint32(p[13:], 105)
				binary.LittleEndian.PutUint32(p[17:], uint32(phase))
				if plan, _, err := w.enterForestStage(p); err != nil || len(plan) == 0 {
					t.Fatal("retry entry failed", phase, err)
				}
			}
			if w.forest.potionUsed[phase] != 3 || !w.forest.stageClock[phase].Equal(started) {
				t.Fatal("retry reset consumption or clock", direct, phase)
			}
		}
	}
}

func TestForestHardTimeoutRetainsModeAndClosesHardManager(t *testing.T) {
	w := forestHardPhaseFixture(0)
	now := time.Unix(1791319000, 0)
	w.forest.stageClock[0] = now.Add(-time.Hour)
	var notes []map[string]any
	plan := w.forestStageTimeout(now, func(e map[string]any) { notes = append(notes, e) })
	if len(plan) == 0 || len(notes) != 1 || w.forest == nil || !w.forest.hard || w.activeDungeon != nil {
		t.Fatal("hard timeout lost mode or failed to leave", plan, notes)
	}
	last := plan[len(plan)-1]
	if last.ID != 2565 || binary.LittleEndian.Uint32(last.Payload[2:]) != 14 || binary.LittleEndian.Uint32(last.Payload[10:]) != ^uint32(0) {
		t.Fatal("timeout reopened normal window", last)
	}
}
