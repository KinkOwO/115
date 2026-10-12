package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/legion"
	"encoding/binary"
	"testing"
)

func TestIspinsAllSourceOperationsEnterWithNativeCardAndPotionLimit(t *testing.T) {
	c := catalog.LoadNativeDungeons(t, legion.IspinsStageDungeons[:]...)
	if len(c.IspinsOperations) != 11 || c.StackableLimits == nil {
		t.Fatal("native source incomplete")
	}
	for index, def := range c.IspinsOperations {
		value, err := def.DrawParameter("party", def.Difficulty, func(n int64) (int64, error) { return n - 1, nil })
		if err != nil {
			t.Fatal(index, err)
		}
		for stage := 0; stage < 4; stage++ {
			run := &ispinsRun{sourceCards: true, confirmed: true, operationIndex: index}
			for before := 0; before < stage; before++ {
				run.cleared[before] = true
			}
			run.cardsOwned[stage] = true
			run.cards[stage][0] = catalog.IspinsCard{Index: index, Value: value, Grade: def.SelectPool + 1}
			w := &worldSession{dungeons: &c, level: 140, ispins: run}
			request := make([]byte, 24)
			request[13] = 101
			request[17] = byte(stage)
			packets, _, err := w.enterIspinsStage(request)
			if err != nil {
				t.Fatal(index, stage, err)
			}
			cardSeen, potionSeen := false, false
			for _, p := range packets {
				switch p.ID {
				case 2255:
					cardSeen = true
					if p.Payload[2] != 0 || binary.LittleEndian.Uint16(p.Payload[51:]) != index || binary.LittleEndian.Uint16(p.Payload[53:]) != value {
						t.Fatal("wrong selected native card", index, stage)
					}
				case 1584:
					potionSeen = true
					if len(p.Payload) != 4 || binary.LittleEndian.Uint32(p.Payload) != run.potionLimit {
						t.Fatal("wrong native cap")
					}
				}
			}
			if !cardSeen || !potionSeen {
				t.Fatal("entry bindings absent")
			}
			if def.Type == 13 && run.potionLimit != uint32(value) {
				t.Fatal("source consumable condition ignored")
			}
			if def.Type != 6 && int64(run.deadline.Sub(w.activeDungeon.StartedAt).Seconds()) < int64(w.activeDungeon.Definition.TimeoutSeconds)-1 {
				t.Fatal("non-time operation borrowed minute condition")
			}
		}
	}
}
