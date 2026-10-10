package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
)

// Verified C12 at2026-10-05T14:05:21.5464264Z, channel99. The old
// capacity4-only check emitted Refusal8, rendered as the daily entry limit.
const venusSoloLivePartyRequestHex = "000001000000310100000000000000002201000101020407070707ffffffff0000000000000000000000000000000000"

func TestVenusStandbyCapacityRoundTripAndBounds(t *testing.T) {
	p, err := hex.DecodeString(venusSoloLivePartyRequestHex)
	if err != nil {
		t.Fatal(err)
	}
	for _, capacity := range []uint32{1, 2, 3, 4, 0, 5, 256, ^uint32(0)} {
		req := bytes.Clone(p)
		binary.LittleEndian.PutUint32(req[7:], capacity)
		decoded, err := protocol.DecodeVenusStandbyParty(req)
		if capacity < 1 || capacity > 4 {
			if err == nil {
				t.Fatal("invalid capacity accepted", capacity)
			}
			continue
		}
		if err != nil || decoded.Capacity != byte(capacity) || string(decoded.Name) != "1" {
			t.Fatal(capacity, decoded, err)
		}
		reply, err := protocol.VenusStandbyPartyReply(decoded.Name, 7, [2]byte{3, 86}, decoded.Capacity)
		if err != nil || len(reply) != 108 {
			t.Fatal(capacity, err)
		}
		q := reply[len(decoded.Name):]
		if q[20] != byte(capacity) || binary.LittleEndian.Uint16(reply[4:]) != 1 || q[29] != 0x22 || q[95] != 0x22 {
			t.Fatal("requested capacity, actual member count or Venus type lost", capacity)
		}
		req[6] = '2'
		if string(decoded.Name) != "1" {
			t.Fatal("request retained caller-owned name")
		}
	}
	for _, capacity := range []byte{0, 5} {
		if _, err := protocol.VenusStandbyPartyReply([]byte("1"), 7, [2]byte{3, 86}, capacity); err == nil {
			t.Fatal("reply accepted invalid capacity", capacity)
		}
	}
}

func TestVenusLiveSoloPartyCanStartWithoutMutatingRole(t *testing.T) {
	p, err := hex.DecodeString(venusSoloLivePartyRequestHex)
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{channelType: 99,
		role:       database.Character{ID: 7, WireID: 7, Name: "VenusCap", State: []byte(`{"level":115,"advancement":5,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`)},
		characters: &character.Service{ChannelContext: [2]byte{3, 86}}}
	state := bytes.Clone(w.role.State)
	handled, plan, err := w.venusStandbyPartyHandle(12, p)
	if !handled || err != nil || len(plan) != 3 || !w.soloPartyReady {
		t.Fatal("live single-player party refused", handled, plan, err)
	}
	if plan[2].ID != 9 || plan[2].Payload[21] != 1 || binary.LittleEndian.Uint16(plan[2].Payload[4:]) != 1 {
		t.Fatal("solo NOTI9 capacity/member count differs")
	}
	start, err := hex.DecodeString(venusLiveStartRequestHex)
	if err != nil {
		t.Fatal(err)
	}
	packets, _, err := w.startVenus(start)
	if err != nil || len(packets) != 2 || packets[0].ID != legion.CmdStart || packets[1].ID != legion.NotiVenusInfo || w.venus == nil {
		t.Fatal("solo party cannot enter the existing Venus start chain", packets, err)
	}
	if !bytes.Equal(state, w.role.State) {
		t.Fatal("party creation/start changed persisted role state")
	}
}
