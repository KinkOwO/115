package main

import (
	"dfolan/internal/catalog"
	"encoding/binary"
	"testing"
)

func ispinsCardFixtureCatalog() *catalog.DungeonCatalog {
	defs := map[uint16]catalog.IspinsOperationDefinition{}
	for pool := 0; pool < 3; pool++ {
		defs[uint16(pool+1)] = catalog.IspinsOperationDefinition{Type: 8, MemberLimit: "party", Difficulty: "normal", SelectPool: uint32(pool), Probability: 1, RandomRanges: []catalog.LegionOperationRange{{Party: "party", Difficulty: "normal", Minimum: 15, Maximum: 15}}}
	}
	return &catalog.DungeonCatalog{IspinsOperations: defs}
}

func TestIspinsPresentedCardSelectionAndRetry(t *testing.T) {
	w := &worldSession{dungeons: ispinsCardFixtureCatalog(), ispins: &ispinsRun{orderOwned: true, order: [4]byte{0, 1, 2, 3}}}
	wait, err := w.ispinsWaitInfo()
	if err != nil {
		t.Fatal(err)
	}
	for slot := 0; slot < 3; slot++ {
		if binary.LittleEndian.Uint16(wait[51+8*slot:]) != uint16(slot+1) {
			t.Fatal("source cards absent")
		}
	}
	confirm := make([]byte, 19)
	confirm[13] = 2
	confirm[17] = 99
	if _, _, err = w.ispinsOperation(confirm); err == nil || w.ispins.confirmed {
		t.Fatal("unpresented operation accepted")
	}
	for slot := 0; slot < 3; slot++ {
		confirm[17] = byte(slot + 1)
		if _, _, err = w.ispinsOperation(confirm); err != nil {
			t.Fatal(err)
		}
		packet, err := w.ispinsInfoPayload("dungeon0", [5]byte{})
		if err != nil || packet[2] != byte(slot) {
			t.Fatal("native selection mismatch", slot, err)
		}
		frozen := w.ispins.cards[0]
		w.ispins.confirmed = false
		w.ispins.operationIndex = 0
		if _, err = w.ispinsWaitInfo(); err != nil || w.ispins.cards[0] != frozen {
			t.Fatal("retry changed cards", err)
		}
	}
}
