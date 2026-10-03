package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/game/wire"
	"dfolan/internal/legion"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestIspinsTimeoutRetryKeepsProgressAndRebuiltPartyCanEnter(t *testing.T) {
	t.Setenv("DFO_ISPINS_MODE", "unlimited")
	dungeons := catalog.LoadNativeDungeons(t, legion.IspinsStageDungeons[:]...)
	for _, leaveFirst := range []bool{false, true} {
		client, _, _ := newDispatchTestClient()
		client.selectedCharacterID = 7
		w := client.worldState
		w.channelType = 81
		w.dungeons = &dungeons
		w.level = 140
		w.role = storage.Character{ID: 7, WireID: 7, Name: "001", State: []byte(`{"level":115,"advancement":5,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`)}
		w.characters = &character.Service{ChannelContext: [2]byte{3, 86}}
		w.state = storage.WorldState{Position: storage.WorldPosition{Town: 146, Area: 2, X: 600, Y: 300}}
		w.ispins = &ispinsRun{stage: 3, cleared: [4]bool{true, true, true, false}, confirmed: true, operationIndex: 11}
		enter := make([]byte, 24)
		enter[13] = 101
		enter[17] = 3
		if _, _, err := w.enterIspinsStage(enter); err != nil {
			t.Fatal(err)
		}
		if _, err := w.ispinsTimeout(w.ispins.deadline); err != nil {
			t.Fatal(err)
		}
		if !w.ispinsRetryPending {
			t.Fatal("timeout did not arm post-town recovery")
		}
		if leaveFirst {
			handled, plan, err := w.ispinsStandbyPartyHandle(13, make([]byte, 8))
			if !handled || err != nil || len(plan) < 5 || plan[0].ID != 9 || plan[1].ID != 32 || w.ispinsRetryPending || w.ispins != nil {
				t.Fatal("leave before town readiness failed to restore actor/quota", err)
			}
			create, _ := hex.DecodeString(ispinsStandbyPartyRequestHex)
			if _, _, err := w.ispinsStandbyPartyHandle(12, create); err != nil {
				t.Fatal(err)
			}
			start := make([]byte, 24)
			start[13] = 101
			if _, _, err := w.startIspins(start); err != nil {
				t.Fatal(err)
			}
			enter[17] = 0
		} else {
			plan, err := w.ispinsRetryRestorePackets(true)
			wait, _ := legion.IspinsInfoPayload("wait3", [5]byte{})
			if err != nil || len(plan) != 2 || plan[0].ID != 32 || len(plan[0].Payload) != 8 || plan[0].Payload[2] != 1 || !bytes.Equal(plan[1].Payload, wait) {
				t.Fatal("missing native alive/wait3 recovery", err)
			}
			ready := &clientRequest{frame: wire.Frame{Type: 1, ID: 35}, plaintext: make([]byte, 8), verified: true}
			if client.dispatchIspins(ready) != dispatchNext || w.ispinsRetryPending {
				t.Fatal("town readiness did not send/clear recovery")
			}
			if w.ispins.cleared != [4]bool{true, true, true, false} {
				t.Fatal("retry erased completed stages")
			}
		}
		confirm := make([]byte, 32)
		confirm[13] = 2
		binary.LittleEndian.PutUint16(confirm[17:], 11)
		if _, _, err := w.ispinsOperation(confirm); err != nil {
			t.Fatal(err)
		}
		if _, _, err := w.enterIspinsStage(enter); err != nil {
			t.Fatalf("retry leaveFirst=%v cannot enter: %v", leaveFirst, err)
		}
		if w.activeDungeon == nil || w.ispins.deadline.IsZero() || !w.ispins.confirmed {
			t.Fatal("retry did not create a fresh combat clock")
		}
	}
}
