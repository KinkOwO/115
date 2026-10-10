package main

import (
	"dfolan/internal/dungeon"
	"encoding/binary"
	"encoding/hex"
	"reflect"
	"testing"
	"time"
)

func TestForestMelodyResetAfterRetreatPreservesProgress(t *testing.T) {
	request, _ := hex.DecodeString("000000000000000070f15f000004000000ffff00000000000000000000000000")
	for _, hard := range []bool{false, true} {
		for stage := 0; stage < 3; stage++ {
			run := &forestRun{hard: hard, choice: 2, target: 6, stage: stage}
			for i := 0; i < stage; i++ {
				run.cleared[i] = true
			}
			run.stageClock[stage] = time.Unix(1791310000, 0)
			run.potionUsed[stage] = 3
			before := *run
			w := &worldSession{forest: run}
			plan, notes, err := w.forestOperation(request)
			if err != nil || len(plan) != 2 || len(notes) != 1 {
				t.Fatalf("reset hard=%t stage=%d: %v", hard, stage, err)
			}
			if plan[0].ID != 2563 || plan[0].Payload[2] != 0xff || plan[0].Payload[3] != 2 || binary.LittleEndian.Uint32(plan[0].Payload[11:]) != uint32(stage) {
				t.Fatal("wrong resumed stage")
			}
			ack := plan[1].Payload
			if plan[1].Kind != 1 || plan[1].ID != 2226 || ack[0] != 1 || binary.LittleEndian.Uint32(ack[1:]) != 4 || binary.LittleEndian.Uint16(ack[5:]) != 0xffff || ack[7] != 0 || binary.LittleEndian.Uint32(ack[8:]) != 0 || ack[16] != 0 {
				t.Fatal("native reader cannot reopen unselected dialog")
			}
			before.choice, before.target = 0xff, 0
			if !reflect.DeepEqual(before, *run) {
				t.Fatal("reset changed progress/mode/timer/consumption")
			}
			again, _, err := w.forestOperation(request)
			if err != nil || !reflect.DeepEqual(plan, again) {
				t.Fatal("reset replay unstable")
			}
			w.activeDungeon = &dungeon.Session{}
			if _, _, err = w.forestOperation(request); err == nil {
				t.Fatal("in-dungeon reset accepted")
			}
		}
	}
}

func TestForestMelodyResetRejectsInvalidTargetWithoutMutation(t *testing.T) {
	request, _ := hex.DecodeString("000000000000000070f15f000004000000060000000000000000000000000000")
	w := &worldSession{forest: &forestRun{choice: 2, target: 6}}
	before := *w.forest
	if _, _, err := w.forestOperation(request); err == nil || !reflect.DeepEqual(before, *w.forest) {
		t.Fatal("invalid reset changed selection")
	}
}
