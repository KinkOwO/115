package main

import (
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"encoding/binary"
	"testing"
)

func TestForestHardRecreateResetsNativeStartGateWithoutEntering(t *testing.T) {
	w := &worldSession{channelType: 96, role: database.Character{ID: 7, WireID: 7, Name: "ForestRepeat", State: []byte(`{"level":115,"advancement":5,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`)}, characters: &character.Service{ChannelContext: [2]byte{3, 0x56}}}
	request := apocalypseStandbyPartyRequest("again", 4, protocol.ForestPartyTypeHard)
	for i := 0; i < 2; i++ {
		_, packets, err := w.forestStandbyPartyHandle(12, request)
		if err != nil {
			t.Fatal(err)
		}
		p := packets[len(packets)-1]
		if p.ID != legion.NotiForestHardInfo || binary.LittleEndian.Uint32(p.Payload[2:]) != 0 || int32(binary.LittleEndian.Uint32(p.Payload[10:])) != -1 {
			t.Fatal("native start gate remains blocked or phase starts", p)
		}
		if w.activeDungeon != nil || w.forest != nil || w.forestEntryPending != nil {
			t.Fatal("party creation started combat")
		}
		_, packets, err = w.forestStandbyPartyHandle(13, nil)
		if err != nil {
			t.Fatal(err)
		}
		p = packets[len(packets)-1]
		if p.ID != legion.NotiForestHardInfo || binary.LittleEndian.Uint32(p.Payload[2:]) != 14 {
			t.Fatal("leave must still reset previous native run")
		}
	}
}
