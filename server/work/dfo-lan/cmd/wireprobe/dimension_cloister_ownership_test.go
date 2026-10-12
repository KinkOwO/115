package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/legion"
	"encoding/binary"
	"fmt"
	"testing"
)

func prepareCloisterCompletedTestStage(s *legionSession, w *worldSession) {
	s.dimCloisterCleared = s.dimCloisterStage
	w.activeDungeon = &dungeon.Session{Definition: catalog.DungeonDefinition{ID: s.dimCloisterStageDungeon(s.dimCloisterStage)}, RunID: fmt.Sprint("cloister-stage-", s.dimCloisterStage)}
	w.activeDungeon.MarkSceneCompleted()
}
func TestCloisterRewardEndRequiresOwnedSequentialBoss(t *testing.T) {
	s := &legionSession{channelType: legion.DimCloisterChannelType}
	w := &worldSession{}
	w.role.ID = 9
	if _, err := s.dimCloisterRewardEnd(w, nil); err == nil {
		t.Fatal("no dungeon advanced")
	}
	w.activeDungeon = &dungeon.Session{Definition: catalog.DungeonDefinition{ID: s.dimCloisterStageDungeon(0)}, RunID: "live"}
	if _, err := s.dimCloisterRewardEnd(w, nil); err == nil {
		t.Fatal("living boss advanced")
	}
	w.activeDungeon.MarkSceneCompleted()
	w.activeDungeon.Definition.ID++
	if _, err := s.dimCloisterRewardEnd(w, nil); err == nil {
		t.Fatal("foreign clear advanced")
	}
	if s.dimCloisterCleared != 0 {
		t.Fatal("failed requests changed progress")
	}
	prepareCloisterCompletedTestStage(s, w)
	if _, err := s.dimCloisterRewardEnd(w, nil); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		r, err := s.dimCloisterRewardEnd(w, nil)
		if err != nil || len(r.Packets) != 1 || s.dimCloisterCleared != 1 {
			t.Fatal("duplicate advanced", err)
		}
	}
}

func TestCloisterEntryDoesNotAdvanceBeforeBossDeath(t *testing.T) {
	for stage := 0; stage < 3; stage++ {
		body, err := dimCloisterActiveInfo(nil, stage)
		if err != nil {
			t.Fatal(err)
		}
		if binary.LittleEndian.Uint32(body[3:]) != 2 || binary.LittleEndian.Uint32(body[7:]) != 0 || binary.LittleEndian.Uint32(body[11:]) != uint32(stage) {
			t.Fatalf("stage%d entry already cleared/advanced: %x", stage, body[:23])
		}
	}
}
