package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/legion"
	"os"
	"testing"
	"time"
)

func TestApocalypseThreeSourceChoicesLoadNavigation(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("native PVF required")
	}
	a, err := catalog.OpenTestArchiveCached(p, "")
	if err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.ImportApocalypse(a)
	if err != nil {
		t.Fatal(err)
	}
	clock, err := legion.NewApocalypseClock(cat)
	if err != nil {
		t.Fatal(err)
	}
	dc, err := catalog.ImportRuntimeDungeons(a, legion.ApocalypseStageDungeons[:])
	if err != nil {
		t.Fatal(err)
	}
	defer dc.CloseMapSource()
	for choice := byte(0); choice < 3; choice++ {
		s := &legionSession{channelType: apocalypseChannelType, catalog: cat, clock: clock}
		w := apocalypseTownSession(t, 7)
		w.role.WireID = 7
		w.dungeons = &dc
		w.legion = s
		s.world = w
		if _, err = s.handle(w, startPayload(107), legion.CmdStart); err != nil {
			t.Fatal(choice, err)
		}
		if _, err = s.handle(w, operationPayload(2, choice, legion.OperationChannelCode), legion.CmdOperationSelect); err != nil {
			t.Fatal(choice, err)
		}
		result, err := s.handle(w, enterPayload(legion.OperationChannelCode, 0), legion.CmdEnterDungeon)
		if err != nil {
			t.Fatal(choice, err)
		}
		if w.activeDungeon == nil || w.activeDungeon.Definition.ID != legion.ApocalypseNavigationDungeon || !hasPacket(result.Packets, 28) || !hasPacket(result.Packets, 29) {
			t.Fatalf("choice%d accepted without real entry: %+v", choice, result.Events)
		}
		if s.apocalypseRun().Choice != choice {
			t.Fatal("difficulty changed")
		}
	}
}

func TestApocalypseMasterActualSourceRoutes(t *testing.T) {
	p := os.Getenv("DFO_REPAIR_PVF")
	if p == "" {
		t.Skip("native PVF required")
	}
	a, e := catalog.OpenTestArchiveCached(p, "")
	if e != nil {
		t.Fatal(e)
	}
	cat, e := catalog.ImportApocalypse(a)
	if e != nil {
		t.Fatal(e)
	}
	clock, e := legion.NewApocalypseClock(cat)
	if e != nil {
		t.Fatal(e)
	}
	contents, e := catalog.ImportLegionContents(a)
	if e != nil {
		t.Fatal(e)
	}
	dc, e := catalog.ImportRuntimeDungeons(a, legion.ApocalypseStageDungeons[:])
	if e != nil {
		t.Fatal(e)
	}
	defer dc.CloseMapSource()
	dc.LegionContents = &contents
	for target := byte(2); target <= 5; target++ {
		w := apocalypseTownSession(t, 7)
		s := &legionSession{channelType: apocalypseChannelType, catalog: cat, clock: clock, world: w}
		w.legion = s
		w.dungeons = &dc
		w.role.WireID = 7
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
			t.Fatal("source navigation failed")
		}
		clearMasterRoom(t, w, time.Second)
		if _, e := s.ApocalypseDirectMove(w, legion.ApocalypseStageDungeons[target]); e != nil {
			t.Fatal(target, e)
		}
		for count := 0; count < 5; count++ {
			stage := legion.ApocalypseStageOfDungeon(w.activeDungeon.Definition.ID)
			clearMasterRoom(t, w, time.Second)
			if stage == 5 {
				break
			}
			if plan, _ := w.apocalypseAdvanceDue(time.Now().Add(4 * time.Second)); len(plan) == 0 {
				t.Fatal("source route stuck", target, stage)
			}
		}
		if w.apocalypse.Cleared != 5 || !w.activeDungeon.Completed() {
			t.Fatal("source master terminal missing", target)
		}
	}
}
