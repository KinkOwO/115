package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/legion"
	"os"
	"testing"
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
