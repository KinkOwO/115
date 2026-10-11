package main

import (
	"dfolan/internal/game/wire"
	"dfolan/internal/legion"
	"testing"
)

func TestForestDispatcherPassesOtherContentStarts(t *testing.T) {
	for _, channel := range []uint32{81, 96, 99, 119} {
		w := townSession(7)
		w.channelType = channel
		w.forestPartyHard = true // A stale mode flag cannot claim another content.
		client := &gameConnection{worldState: w, bootstrapped: true}
		for _, content := range []uint32{101, 106, 107, 999} {
			request := &clientRequest{frame: wire.Frame{Type: 1, ID: legion.CmdStart}, verified: true, plaintext: startPayload(content)}
			if got := client.dispatchForest(request); got != dispatchNext {
				t.Fatalf("channel=%d content=%d swallowed: %d", channel, content, got)
			}
		}
	}
}

func TestForestSharedFamilyRecognizesBothModesOnly(t *testing.T) {
	for _, id := range []uint16{legion.CmdStart, legion.CmdEnterDungeon, legion.CmdRewardEnd} {
		for _, content := range []uint32{101, 104, 105, 106, 107} {
			want := content == 104 || content == 105
			if got := isForestRequest(id, startPayload(content)); got != want {
				t.Fatalf("id=%d content=%d owned=%t", id, content, got)
			}
		}
	}
}
