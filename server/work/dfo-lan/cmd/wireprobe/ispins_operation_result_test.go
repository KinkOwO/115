package main

import (
	"bytes"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"testing"
)

func TestIspinsSettlementReportsSelectedOperationEveryStage(t *testing.T) {
	for stage := 0; stage < 4; stage++ {
		for _, index := range []uint16{4, 11} {
			run := &dungeon.Session{ArenaBoss: true, Loaded: true, Dead: map[uint16]bool{},
				Monsters: []protocol.DungeonMonster{{Entity: 4096, Rank: 3, Team: 100}}}
			if _, err := run.ConfirmDeath(4096, 7, 7); err != nil {
				t.Fatal(err)
			}
			w := &worldSession{activeDungeon: run, role: database.Character{WireID: 7}, ispins: &ispinsRun{stage: stage, operationIndex: index}}
			plan, err := w.completeIspinsStage()
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			for _, packet := range plan {
				if packet.ID == legion.NotiIspinsOperation {
					seen++
					if !bytes.Equal(packet.Payload, []byte{byte(index), 0, 1}) {
						t.Fatalf("stage%d selected%d result=%x", stage, index, packet.Payload)
					}
				}
			}
			if seen != 1 {
				t.Fatalf("stage%d selected%d result notices=%d", stage, index, seen)
			}
		}
	}
}
