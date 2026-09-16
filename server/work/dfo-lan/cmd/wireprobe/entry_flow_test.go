package main

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestQuestBatchAfterPreviousWriteDeadlineExpired(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	if err := server.SetWriteDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	want := []byte{1, 34, 0, 1, 9, 19}
	result := make(chan error, 1)
	go func() { result <- writePackets(server, []preparedPacket{{Raw: want}}, nil) }()
	client.SetReadDeadline(time.Now().Add(time.Second))
	got := make([]byte, len(want))
	if _, err := io.ReadFull(client, got); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("quest reply changed")
	}
}

func TestCompleteEntryPreflight(t *testing.T) {
	must := func(p []byte, err error) []byte {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	user := protocol.AreaUser{ActorServerID: 3, X: 544, Y: 311, Flags: [3]byte{0, 1, 1}}
	p := entryPayloads{
		Select:   must(protocol.SelectProbeSuccess(protocol.SelectProbeState{ActorServerID: 3, Fatigue: [3]uint16{0, 156, 0}, ActiveQuests: []protocol.ActiveQuest{{ID: 3145, Progress: 1}}})),
		Basic:    must(protocol.UserInfoBasicProbe(protocol.EntryBasicProbe{ActorServerID: 3, Character: protocol.CharacterRow{Name: "LanTest01", Level: 1}})),
		Addition: must(protocol.UserInfoAdditionProbe(protocol.EntryAdditionProbe{ActorServerID: 3, Stats: protocol.PackedEntryStats{HP: 5200, MP: 4800, BasePercent: 100}})),
		Skills:   must(protocol.SkillInfo(1, []protocol.LearnedSkill{{ID: 46, Level: 1, Slot: 0}})),
		Vault:    must(protocol.EmptyPersonalVault(8)),
		UserArea: must(protocol.UserArea(38, 1, user)),
		Area:     must(protocol.AreaUsers(38, 1, []protocol.AreaUser{user})),
		Fatigue:  must(protocol.Fatigue(0, 156, 0)),
		Complete: protocol.EnterGameworldComplete(),
	}
	keys := make([]byte, wire.SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	prepared, err := preparePackets(keys, p.packets())
	if err != nil {
		t.Fatal(err)
	}
	ids := []uint16{4, 2, 2, 19, 13, 23, 24, 36, 124}
	if len(prepared) != len(ids) {
		t.Fatalf("entry frame count=%d", len(prepared))
	}
	var sent bytes.Buffer
	if err = writePackets(&sent, prepared, nil); err != nil {
		t.Fatal(err)
	}
	raw := sent.Bytes()
	for i, id := range ids {
		if len(raw) < 16 {
			t.Fatal("truncated entry")
		}
		n := int(binary.LittleEndian.Uint32(raw[3:]))
		if n > len(raw) || n < 16 {
			t.Fatal("invalid frame length")
		}
		kind := byte(0)
		if i == 0 {
			kind = 1
		}
		if raw[0] != kind || binary.LittleEndian.Uint16(raw[1:]) != id {
			t.Fatalf("wrong entry sequence at %d", i)
		}
		cipher := raw[16:n]
		if raw[11] != wire.Checksum(cipher) {
			t.Fatalf("checksum at %d", i)
		}
		decoded, err := wire.DecryptPayload(keys, id, cipher)
		want := prepared[i].Payload
		if err != nil || len(decoded) < len(want) || !bytes.Equal(decoded[:len(want)], want) {
			t.Fatalf("frame %d decode: %v", id, err)
		}
		for _, b := range decoded[len(want):] {
			if b != 0 {
				t.Fatal("nonzero padding")
			}
		}
		raw = raw[n:]
	}
	if len(raw) != 0 {
		t.Fatal("trailing frames")
	}
	// A future missing cipher or invalid frame must return no partially
	// prepared sequence; the caller never writes a successful SELECT first.
	for _, bad := range []outboundPacket{{"invalid", 2, 4, []byte{1}}} {
		plan := append(p.packets(), bad)
		out, err := preparePackets(keys, plan)
		if err == nil || out != nil || !strings.Contains(err.Error(), bad.Name) {
			t.Fatalf("partial entry escaped preflight: %v", err)
		}
	}
}
