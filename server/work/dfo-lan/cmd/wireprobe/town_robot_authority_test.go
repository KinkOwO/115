package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRobotAuthorityLeaseLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authority.bin")
	tr := &townRobotTransport{ownerActor: 2, authorityPath: path, event: func(map[string]any) {}, peers: []*townRobotUDP{{actor: 65534}}}
	read := func(active byte) []byte {
		t.Helper()
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if len(b) != 64 || string(b[:8]) != "TRAUTH16" || b[28] != active {
			t.Fatalf("invalid lease: %x", b)
		}
		return b
	}
	// Room attachment without completed native peer negotiation stays inactive.
	tr.setRoom(&[2]uint16{1, 2})
	read(0)
	tr.completed = true
	tr.publishAuthorityLocked(false, true)
	b := read(1)
	if binary.LittleEndian.Uint16(b[32:]) != 2 || binary.LittleEndian.Uint16(b[38:]) != 65534 || binary.LittleEndian.Uint16(b[52:]) != 1 {
		t.Fatal("wrong identity/room")
	}
	if binary.LittleEndian.Uint64(b[56:]) != 0x3130763631524154 {
		t.Fatal("IPC footer mismatch")
	}
	tr.setRoom(nil)
	read(0)
	tr.setRoom(&[2]uint16{2, 2})
	b = read(1)
	if binary.LittleEndian.Uint32(b[24:]) != 3 {
		t.Fatal("room epoch did not advance")
	}
	tr.closed = true
	tr.publishAuthorityLocked(false, true)
	read(0)
}

func TestRobotAuthorityRetiredTransportCannotClearNewParty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authority.bin")
	makeTr := func(actor uint16) *townRobotTransport {
		return &townRobotTransport{ownerActor: 2, authorityPath: path, completed: true, event: func(map[string]any) {}, peers: []*townRobotUDP{{actor: actor}}}
	}
	old, next := makeTr(65534), makeTr(65533)
	old.setRoom(&[2]uint16{0, 0})
	next.setRoom(&[2]uint16{1, 0})
	old.closed = true
	old.publishAuthorityLocked(false, true)
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if b[28] != 1 || binary.LittleEndian.Uint16(b[38:]) != 65533 {
		t.Fatal("old transport overwrote new owner")
	}
}

func TestRobotAuthorityRejectsInvalidIdentityAndRequiresOptIn(t *testing.T) {
	t.Setenv("DFO_LOCAL_ROBOT_AUTHORITY", "")
	t.Setenv("DFO_ROBOT_AUTHORITY_LEASE", filepath.Join(t.TempDir(), "authority.bin"))
	if robotAuthorityPath() != "" {
		t.Fatal("missing opt-in accepted")
	}
	t.Setenv("DFO_LOCAL_ROBOT_AUTHORITY", "r16-object-authority-v1")
	if robotAuthorityPath() == "" {
		t.Fatal("valid path rejected")
	}
	t.Setenv("DFO_ROBOT_AUTHORITY_LEASE", "authority.bin")
	if robotAuthorityPath() != "" {
		t.Fatal("relative path accepted")
	}
	for _, ids := range [][]uint16{{}, {0}, {2}, {65534, 65534}, {1, 3, 4, 5}} {
		tr := &townRobotTransport{ownerActor: 2, completed: true, room: &[2]uint16{}}
		for _, id := range ids {
			tr.peers = append(tr.peers, &townRobotUDP{actor: id})
		}
		if tr.authorityBody(time.Now())[28] != 0 {
			t.Fatal("invalid identities active", ids)
		}
	}
}
