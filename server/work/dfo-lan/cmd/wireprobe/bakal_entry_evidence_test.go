package main

import (
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestBakalEvidenceCommandsRetainedAfterSampleLimit(t *testing.T) {
	for _, id := range []uint16{656, 657, 2089, 2069, 2070, 2073, 2074, 2261, 1134} {
		seen := map[uint16]int{id: BodySampleLimit}
		if !observedGameRequest(id) || !retainRequestBody(id, seen) || seen[id] != BodySampleLimit {
			t.Fatalf("Bakal command %d lost evidence after sampling cap", id)
		}
	}
	if retainRequestBody(2127, map[uint16]int{2127: BodySampleLimit}) {
		t.Fatal("unrelated telemetry lost its sampling cap")
	}
}

// 023721 live: C2062 enters the right room; C45 enters the source arena.
// Exercise the real portal path rather than selecting a boss grid directly.
func TestBakalFirstPhaseLivePortalThenArena(t *testing.T) {
	c, at := bakalNativeClient(t)
	w := c.worldState
	p, _ := hex.DecodeString("0300000000000000ffffffffff4dedf505000000000200000001000000bb000000210100000500000005000000000000")
	if handled, _, err := c.enterBakalPortal(2062, p, at); err != nil || !handled {
		t.Fatalf("portal: handled=%v err=%v", handled, err)
	}
	if [2]byte{w.activeDungeon.Room.X, w.activeDungeon.Room.Y} != [2]byte{2, 1} {
		t.Fatal("native entrance moved from requested grid")
	}
	if _, _, err := c.bakalLoadingDone(nil); err != nil {
		t.Fatal(err)
	}
	if len(w.activeDungeon.LivingMonsters()) != 0 {
		t.Fatal("boss created in entrance room")
	}
	p, _ = hex.DecodeString("0101150000005301000000aae3000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000cb0035a202001500000053010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000005ffffffffffff000000000000004dedf5050000000000")
	r, err := protocol.DecodeDungeonRoomTransition(p)
	if err != nil {
		t.Fatal(err)
	}
	next, _, err := w.moveDungeonRoomDecoded(r)
	if err != nil {
		t.Fatal(err)
	}
	w.activeDungeon = next
	_, plan, err := c.bakalLoadingDone(nil)
	if err != nil {
		t.Fatal(err)
	}
	creates := 0
	for _, packet := range plan {
		if packet.ID == 2194 {
			creates++
			if packet.Payload[1] != next.Room.X || packet.Payload[2] != next.Room.Y || binary.LittleEndian.Uint32(packet.Payload[7:]) != w.bakalRules.Monsters["bakal"].Template {
				t.Fatal("boss create does not belong to loaded player room")
			}
		}
	}
	if creates != 1 || len(next.LivingMonsters()) != 1 {
		t.Fatalf("arena creates=%d living=%d", creates, len(next.LivingMonsters()))
	}
	if extra, err := w.bakalLoadedBoss(); err != nil || len(extra) != 0 {
		t.Fatalf("boss duplicated after load: %v", err)
	}
}
