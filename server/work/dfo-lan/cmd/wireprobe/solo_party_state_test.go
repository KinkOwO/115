package main

import (
	"bytes"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestSoloPartyUsesIntroducedActorContext(t *testing.T) {
	w := &worldSession{role: database.Character{WireID: 2}, characters: &character.Service{ChannelContext: [2]byte{1, 30}}}
	for _, override := range []*[2]byte{nil, {3, 86}} {
		identity := w.characters.ChannelContext
		if override != nil {
			identity = *override
		}
		got, err := w.soloPartyInfo(override)
		if err != nil {
			t.Fatal(err)
		}
		want, err := protocol.PartyRosterSeats115(1, identity, [4]uint16{2}, 2, 4)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatal("solo roster does not use the introduced actor identity")
		}
	}
}
