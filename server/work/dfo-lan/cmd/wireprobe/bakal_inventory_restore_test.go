package main

import (
	"encoding/binary"
	"testing"
)

func TestBakalLoadRestoresCurrentCommanderStockOnce(t *testing.T) {
	c, at := bakalNativeClient(t)
	w := c.worldState
	w.bakal.GrantBuffs(0, 0, 2, 4)
	expected := w.bakal.BuffInventorySnapshot().Body
	grid := bakalNativeBossGrid(t, w, "bakal")
	if _, err := w.bakalDungeonEntry(uint32(w.bakalRules.NormalPhase.EnterBakalDungeon), 0, &grid, at, false); err != nil {
		t.Fatal(err)
	}
	_, plan, err := c.bakalLoadingDone(nil)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, p := range plan {
		if p.ID != 2288 {
			continue
		}
		n++
		if len(p.Payload) != 13 {
			t.Fatal("stock snapshot lost native13B layout")
		}
		for i := 0; i < 5; i++ {
			if p.Payload[i] != expected[i] {
				t.Fatal("load reset or granted commander stock")
			}
		}
		if binary.LittleEndian.Uint32(p.Payload[5:]) != 25 || binary.LittleEndian.Uint32(p.Payload[9:]) != 0xffffffff {
			t.Fatal("inventory restore activated an effect or targeted a player")
		}
	}
	if n != 1 {
		t.Fatalf("load snapshots=%d", n)
	}
	_, repeat, err := c.bakalFinishLoading(make([]byte, 16), false)
	if err != nil || len(repeat) != 1 || repeat[0].ID != 37 {
		t.Fatalf("duplicate load repeated restoration: %v", err)
	}
	if !observedGameRequest(2072) || !retainRequestBody(2072, map[uint16]int{2072: BodySampleLimit}) {
		t.Fatal("commander-use evidence truncated after eight requests")
	}
}
