package main

import (
	"dfolan/internal/catalog"
	"encoding/binary"
	"testing"
)

func TestIspinsAllOrdersKeepUIEntryAndRewardOwnerAligned(t *testing.T) {
	c := &catalog.LegionContents{Contents: map[string]catalog.LegionContent{"Ispins": {LastPhase: 3, Dungeons: []catalog.LegionDungeon{{Dungeon: 10}, {Dungeon: 20}, {Dungeon: 30}, {Dungeon: 40}}}}}
	count := 0
	var walk func([4]byte, int)
	walk = func(order [4]byte, at int) {
		if at < 4 {
			for i := at; i < 4; i++ {
				next := order
				next[at], next[i] = next[i], next[at]
				walk(next, at+1)
			}
			return
		}
		count++
		w := &worldSession{dungeons: ispinsCardFixtureCatalog(), ispins: &ispinsRun{order: order, orderOwned: true}}
		for phase := 0; phase < 4; phase++ {
			w.ispins.stage = phase
			w.ispins.cleared[phase] = true
			p, err := w.ispinsInfoPayload("clear0", [5]byte{})
			if err != nil {
				t.Fatal(err)
			}
			for i, index := range order {
				if binary.LittleEndian.Uint32(p[19+8*i:]) != uint32(index) {
					t.Fatal("UI order mismatch")
				}
			}
			id, err := catalog.IspinsDefaultStageDungeon(c, int(w.ispins.dungeonIndex(phase)))
			if err != nil || id != uint32(order[phase]+1)*10 {
				t.Fatal("entry mismatch", id, err)
			}
			req := make([]byte, 22)
			binary.LittleEndian.PutUint32(req[13:], 101)
			binary.LittleEndian.PutUint32(req[17:], uint32(phase))
			req[21] = (order[phase] + 1) % 4
			if plan, _, err := w.ispinsRewardEnd(req); err == nil || len(plan) != 0 {
				t.Fatal("foreign dragon accepted")
			}
			req[21] = order[phase]
			plan, _, err := w.ispinsRewardEnd(req)
			if err != nil {
				t.Fatal(order, phase, err)
			}
			for _, packet := range plan {
				if packet.ID == 2046 && packet.Payload[13] != order[phase] {
					t.Fatal("reward owner mismatch")
				}
			}
			if w.ispins.order != order {
				t.Fatal("request changed challenge order")
			}
		}
	}
	walk([4]byte{0, 1, 2, 3}, 0)
	if count != 24 {
		t.Fatal(count)
	}
}

func TestIspinsNextChallengeAlwaysChangesOrder(t *testing.T) {
	previous := [4]byte{0, 1, 2, 3}
	for run := 0; run < 100; run++ {
		next, err := newIspinsOrder(previous, true)
		if err != nil || next == previous {
			t.Fatal(next, err)
		}
		seen := [4]bool{}
		for _, index := range next {
			if index >= 4 || seen[index] {
				t.Fatal("invalid permutation", next)
			}
			seen[index] = true
		}
		previous = next
	}
}
