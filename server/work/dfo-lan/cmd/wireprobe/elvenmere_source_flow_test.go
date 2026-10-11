package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"dfolan/internal/workflow"
	"encoding/binary"
	"os"
	"strings"
	"testing"
	"time"
)

func TestElvenmereCurrentPVFZoneRoutingAndResume(t *testing.T) {
	path := os.Getenv("DFO_FAME_AUDIT_PVF")
	if path == "" {
		t.Skip("requires current PVF audit archive")
	}
	source, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: path})
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	c, err := source.Dungeons([]uint32{100003126})
	if err != nil {
		t.Fatal(err)
	}
	if c.Elvenmere == nil || len(c.Elvenmere.Areas) != 4 {
		t.Fatal("missing current source zones")
	}
	for _, area := range c.Elvenmere.Areas {
		s, err := dungeon.Select(c, protocol.DungeonSelection{ID: 100003126, Difficulty: 2, Extra: uint16(area.First), Party: 65535}, 115, nil)
		if err != nil || s.Room.Map != area.WaitingMap {
			t.Fatal("wrong source waiting map", area, s, err)
		}
	}
	w, _ := elvenmereRewardFixture(t, false)
	// The exact native source is tested above. Below rewards use the private
	// SQLite fixture's identity; no current player save or source alias is used.
	w.dungeons = &c
	ctx := context.Background()
	r, p, err := w.elvenmereSelection(ctx, protocol.DungeonSelection{ID: 100003126, Extra: 36})
	if err != nil || r.Extra != 36 || p.MaxCleared != 0 {
		t.Fatal(r, p, err)
	}
	g := workflow.ElvenmereFloorGrant{Run: strings.Repeat("a", 32), Floor: 36, StartFloor: 36, WeeklyItem: 10336354, WeeklyCount: 5}
	if _, _, err := (&workflow.LootService{Store: w.store, Loot: w.loot}).CommitElvenmereFloor(ctx, w.role, g, nil); err != nil {
		t.Fatal(err)
	}
	r, p, err = w.elvenmereSelection(ctx, protocol.DungeonSelection{ID: 100003126, Extra: 36})
	if err != nil || r.Extra != 37 || p.StartFloor != 36 || p.MaxCleared != 36 {
		t.Fatal("did not resume account progress", r, p, err)
	}
	if _, _, err := w.elvenmereSelection(ctx, protocol.DungeonSelection{ID: 100003126, Extra: 61}); err == nil {
		t.Fatal("changed zone after first clear")
	}
	g.Run, g.Floor = strings.Repeat("b", 32), 38
	if _, _, err := (&workflow.LootService{Store: w.store, Loot: w.loot}).CommitElvenmereFloor(ctx, w.role, g, nil); err == nil {
		t.Fatal("skipped an uncompleted floor")
	}
	t.Logf("source zones=%d original maze map=%d zone36 waiting=%d resume=%d", len(c.Elvenmere.Areas), c.Dungeons[100003126].Mazes[0].Rooms[0].Map, c.Elvenmere.Areas[1].WaitingMap, r.Extra)
}

func TestElvenmereExistingLoadAndBossHandlersClearConsecutiveFloors(t *testing.T) {
	w, f := elvenmereRewardFixture(t, false)
	w.dungeons.Elvenmere.Floors[6] = catalog.ElvenmereFloorRule{Weekly: catalog.ElvenmereItemReward{Template: 10336354, Count: 5}}
	w.activeDungeon.ElvenmereWeekStart = dungeon.ElvenmereWeekStart(time.Now())
	// Consume the fixture's completed fifth floor; all subsequent calls use
	// actual loading and boss handlers instead of MarkSceneCompleted.
	if _, err := w.elvenmereTeleport(make([]byte, 25)); err != nil {
		t.Fatal(err)
	}
	if _, err := w.finishDungeonLoading(make([]byte, 16)); err != nil {
		t.Fatal(err)
	}
	// The dispatcher marks Loaded only after it sends a successful C37 plan.
	w.activeDungeon.Loaded = true
	payload := make([]byte, 16)
	binary.LittleEndian.PutUint16(payload[0:2], w.role.WireID)
	binary.LittleEndian.PutUint16(payload[2:4], 99)
	if _, err := w.bossCheck(payload); err != nil {
		t.Fatal(err)
	}
	if _, err := w.elvenmereTeleport(make([]byte, 25)); err != nil {
		t.Fatal(err)
	}
	if w.activeDungeon.Extra != 7 {
		t.Fatal("consecutive floor not advanced")
	}
	saved, err := f.Storage().AdminCharacter(context.Background(), w.role.ID)
	if err != nil {
		t.Fatal(err)
	}
	if weekly, season := elvenmereHeldItems(t, saved); weekly != 10 || season != 1 {
		t.Fatal(weekly, season)
	}
	// A run retained across reset must not settle old progress in the new week.
	w.activeDungeon.ElvenmereWeekStart -= 7 * 24 * 3600
	if _, err := w.finishDungeonLoading(make([]byte, 16)); err == nil {
		t.Fatal("expired week load accepted")
	}
	if _, err := w.elvenmereTeleport(make([]byte, 25)); err == nil {
		t.Fatal("expired week teleport accepted")
	}
}

func TestElvenmereServerHandlersReachFinalFloorOnce(t *testing.T) {
	w, f := elvenmereRewardFixture(t, false)
	// Synthetic reward sizes isolate handler sequencing and storage invariants.
	// The separate current-PVF audit validates all actual reward rows and zones.
	w.dungeons.Elvenmere.Areas = []catalog.ElvenmereArea{{First: 1, Last: 100, WaitingMap: 1}}
	for floor := byte(1); floor <= 100; floor++ {
		row := catalog.ElvenmereFloorRule{Weekly: catalog.ElvenmereItemReward{Template: 10336354, Count: 1}}
		if floor%5 == 0 {
			row.Season = catalog.ElvenmereItemReward{Template: 10333027, Count: 1}
		}
		w.dungeons.Elvenmere.Floors[floor] = row
	}
	w.activeDungeon = &dungeon.Session{RunID: strings.Repeat("d", 32), Definition: catalog.DungeonDefinition{ID: 100003126}, Extra: 1, ElvenmereStartFloor: 1, ElvenmereWeekStart: dungeon.ElvenmereWeekStart(time.Now())}
	payload := make([]byte, 16)
	binary.LittleEndian.PutUint16(payload[0:2], w.role.WireID)
	binary.LittleEndian.PutUint16(payload[2:4], 99)
	for floor := 1; floor <= 100; floor++ {
		if _, err := w.finishDungeonLoading(make([]byte, 16)); err != nil {
			t.Fatalf("load floor %d: %v", floor, err)
		}
		w.activeDungeon.Loaded = true // dispatcher's successful-send transition
		if _, err := w.bossCheck(payload); err != nil {
			t.Fatalf("boss floor %d: %v", floor, err)
		}
		if _, err := w.elvenmereTeleport(make([]byte, 25)); err != nil {
			t.Fatalf("settle floor %d: %v", floor, err)
		}
		if _, err := w.elvenmereTeleport(make([]byte, 25)); err != nil {
			t.Fatalf("retry floor %d: %v", floor, err)
		}
	}
	if n, err := f.EventCount(context.Background(), w.role.ID); err != nil || n != 100 {
		t.Fatal("floor receipts", n, err)
	}
	if weekly, season := elvenmereHeldItems(t, w.role); weekly != 100 || season != 20 {
		t.Fatal(weekly, season)
	}
	if _, err := w.finishDungeonLoading(make([]byte, 16)); err == nil {
		t.Fatal("final floor loaded after full clear")
	}
	if _, _, err := w.elvenmereSelection(context.Background(), protocol.DungeonSelection{ID: 100003126, Extra: 1}); err == nil {
		t.Fatal("fully cleared account entered again")
	}
	p, _, err := dungeon.ReadElvenmereProgress(w.role.State)
	if err != nil || p.MaxCleared != 100 || p.StartFloor != 1 || len(p.Weekly) != 100 || len(p.Season) != 20 {
		t.Fatal(p, err)
	}
	t.Log("100 actual load/boss/settlement handler iterations; 100 duplicate requests; 100 receipts; cap and re-entry rejection passed (private SQLite fixture)")
}
