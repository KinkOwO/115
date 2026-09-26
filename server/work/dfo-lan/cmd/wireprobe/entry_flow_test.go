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

func TestEntryPacketsDelayAvatarRowsUntilAfterWorldInitialization(t *testing.T) {
	p := entryPayloads{
		Avatars:      []byte{1, 0, 0, 0, 0},
		AvatarReady:  []byte{1, 0, 0, 1, 0},
		Creatures:    []byte{7, 0, 0},
		CreatureList: []byte{0},
		WornUpdate:   []byte{3, 0, 0},
	}
	packets := p.packets()
	var initial, ready int
	var list, inventory int
	for i, packet := range packets {
		switch packet.Name {
		case "avatar_inventory_initialized":
			initial = i
		case "avatar_inventory_restored":
			ready = i
		case "creature_list_restored":
			list = i
		case "creature_inventory_restored":
			inventory = i
		}
	}
	if initial == 0 || ready <= initial {
		t.Fatalf("avatar packet order is not delayed: initial=%d ready=%d", initial, ready)
	}
	if !bytes.Equal(packets[initial].Payload, p.Avatars) || !bytes.Equal(packets[ready].Payload, p.AvatarReady) {
		t.Fatalf("avatar payloads were swapped: initial=%x ready=%x", packets[initial].Payload, packets[ready].Payload)
	}
	if list == 0 || inventory <= list {
		t.Fatalf("pet list must precede pet inventory: list=%d inventory=%d", list, inventory)
	}
}

func TestEntrySkillPresetFollowsSkillTree(t *testing.T) {
	packets := (entryPayloads{Skills: []byte{1}, SkillPreset: make([]byte, 28)}).packets()
	skills, preset := -1, -1
	for i, p := range packets {
		if p.ID == 19 {
			skills = i
		}
		if p.ID == 2758 {
			preset = i
		}
	}
	if skills < 0 || preset != skills+1 {
		t.Fatalf("skill preset order skills=%d preset=%d", skills, preset)
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
	// Packets with non-empty payloads are sent in sequence
	var nonZeroPayloads []outboundPacket
	for _, pkt := range p.packets() {
		if len(pkt.Payload) > 0 {
			nonZeroPayloads = append(nonZeroPayloads, pkt)
		}
	}
	if len(prepared) != len(nonZeroPayloads) {
		t.Fatalf("entry frame count=%d want=%d", len(prepared), len(nonZeroPayloads))
	}
	var sent bytes.Buffer
	if err = writePackets(&sent, prepared, nil); err != nil {
		t.Fatal(err)
	}
	raw := sent.Bytes()
	for i, expectedPkt := range nonZeroPayloads {
		id := expectedPkt.ID
		if len(raw) < 16 {
			t.Fatal("truncated entry")
		}
		n := int(binary.LittleEndian.Uint32(raw[3:]))
		if n > len(raw) || n < 16 {
			t.Fatal("invalid frame length")
		}
		kind := expectedPkt.Kind
		if raw[0] != kind || binary.LittleEndian.Uint16(raw[1:]) != id {
			t.Fatalf("wrong entry sequence at %d: got id=%d kind=%d, want id=%d kind=%d", i, binary.LittleEndian.Uint16(raw[1:]), raw[0], id, kind)
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
