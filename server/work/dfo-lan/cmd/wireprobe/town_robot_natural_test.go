package main

import (
	"dfolan/internal/world"
	"encoding/binary"
	"testing"
)

func TestTownRobotIndependentWalkStop(t *testing.T) {
	w, f := townRobotWorld(t, 20)
	w.ensureTownRobots()
	stops, moves, mixed := 0, 0, false
	previous := map[uint16][2]uint16{}
	w.peer.send = func(_ byte, id uint16, p []byte) error {
		if id != 22 {
			return nil
		}
		actor := binary.LittleEndian.Uint16(p)
		xy := [2]uint16{binary.LittleEndian.Uint16(p[2:]), binary.LittleEndian.Uint16(p[4:])}
		if p[6]&4 != 0 {
			stops++
		} else {
			moves++
			if old, ok := previous[actor]; ok && old == xy {
				t.Fatalf("stationary walk actor=%d", actor)
			}
		}
		previous[actor] = xy
		return nil
	}
	stationaryPositions := map[uint16][2]uint16{}
	idleFrames, frames := 0, 0
	for _, b := range f.bots {
		if b.recordID%3 != 0 {
			stationaryPositions[b.peer.actorID] = [2]uint16{b.peer.x, b.peer.y}
		}
	}
	for n := 0; n < 1500; n++ {
		f.tick()
		walking := 0
		for _, b := range f.bots {
			frames++
			if b.walking {
				walking++
			} else {
				idleFrames++
			}
			if initial, ok := stationaryPositions[b.peer.actorID]; ok && (b.walking || initial != [2]uint16{b.peer.x, b.peer.y}) {
				t.Fatal("standing actor moved", b.recordID)
			}
			if !world.Contains(b.rect, b.peer.x, b.peer.y, 0) {
				t.Fatal("left PVF rectangle")
			}
		}
		if walking > 0 && walking < len(f.bots) {
			mixed = true
		}
	}
	if stops <= 20 || moves == 0 || !mixed {
		t.Fatalf("independent pause missing stops=%d moves=%d mixed=%t", stops, moves, mixed)
	}
	if idleFrames*100 < frames*97 || len(stationaryPositions) < 12 {
		t.Fatalf("town must be predominantly idle: idle=%d/%d stationary=%d", idleFrames, frames, len(stationaryPositions))
	}
	// Idle actors emit one stop, not a stream of stationary walk packets.
	for _, b := range f.bots {
		b.walking = false
		b.announced = true
		b.idleUntil = f.clock + 100
	}
	before := stops + moves
	for i := 0; i < 20; i++ {
		f.tick()
	}
	if stops+moves != before {
		t.Fatal("idle actors emitted movement")
	}
}
func TestTownRobotSmallRectangleNeverWalksInPlace(t *testing.T) {
	w, f := townRobotWorld(t, 1)
	w.ensureTownRobots()
	b := f.bots[0]
	b.recordID = 3
	b.rect = [4]int32{100, 200, 1, 1}
	b.peer.x = 100
	b.peer.y = 200
	w.peer.send = func(_ byte, id uint16, p []byte) error {
		if id == 22 && p[6]&4 == 0 {
			t.Fatal("tiny destination must finish with native stop")
		}
		return nil
	}
	for i := 0; i < 1000; i++ {
		f.tick()
	}
}
