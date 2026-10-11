package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Local IPC identity lease, not a game packet or persisted player data.
// Lock order is transport.mu then publisher.Mutex; the publisher never calls back.
var robotAuthorityPublisher struct {
	sync.Mutex
	owner *townRobotTransport
}

func robotAuthorityPath() string {
	if os.Getenv("DFO_LOCAL_ROBOT_AUTHORITY") != "r16-object-authority-v1" {
		return ""
	}
	p := os.Getenv("DFO_ROBOT_AUTHORITY_LEASE")
	if !filepath.IsAbs(p) || filepath.Base(p) != "authority.bin" {
		return ""
	}
	return p
}

func (t *townRobotTransport) authorityBody(now time.Time) [64]byte {
	var b [64]byte
	copy(b[:8], "TRAUTH16")
	binary.LittleEndian.PutUint32(b[8:], 1)
	binary.LittleEndian.PutUint32(b[12:], uint32(os.Getpid()))
	binary.LittleEndian.PutUint64(b[16:], uint64(now.UnixMilli()))
	binary.LittleEndian.PutUint32(b[24:], t.observeEpoch)
	valid := !t.closed && t.completed && t.room != nil && t.ownerActor != 0 && len(t.peers) > 0 && len(t.peers) <= 3
	seen := map[uint16]bool{t.ownerActor: true}
	for _, p := range t.peers {
		if p.actor == 0 || seen[p.actor] {
			valid = false
		}
		seen[p.actor] = true
	}
	if valid {
		binary.LittleEndian.PutUint32(b[28:], 1)
		binary.LittleEndian.PutUint16(b[32:], t.ownerActor)
		binary.LittleEndian.PutUint16(b[34:], uint16(len(t.peers)))
		for i, p := range t.peers {
			binary.LittleEndian.PutUint16(b[36+2*(i+1):], p.actor)
		}
		binary.LittleEndian.PutUint16(b[52:], t.room[0])
		binary.LittleEndian.PutUint16(b[54:], t.room[1])
	}
	// The per-launch unique directory isolates old game/server processes.
	binary.LittleEndian.PutUint64(b[56:], 0x3130763631524154)
	return b
}

// Caller holds t.mu. A new room claims the publisher; retired transports may
// never clear or refresh a newer party's lease.
func (t *townRobotTransport) publishAuthorityLocked(claim, force bool) {
	path := t.authorityPath
	if path == "" {
		return
	}
	now := time.Now()
	if !force && now.Sub(t.authorityLast) < 500*time.Millisecond {
		return
	}
	robotAuthorityPublisher.Lock()
	defer robotAuthorityPublisher.Unlock()
	if claim {
		robotAuthorityPublisher.owner = t
	}
	if robotAuthorityPublisher.owner != t {
		return
	}
	b := t.authorityBody(now)
	t.publishNativeAILocked(now, b)
	tmp, err := os.CreateTemp(filepath.Dir(path), ".authority-*")
	if err == nil {
		name := tmp.Name()
		_, err = tmp.Write(b[:])
		if e := tmp.Close(); err == nil {
			err = e
		}
		if err == nil {
			err = os.Rename(name, path)
		}
		if err != nil {
			_ = os.Remove(name)
		}
	}
	t.authorityLast = now
	if err != nil && !t.authorityError {
		t.authorityError = true
		t.event(map[string]any{"kind": "town_robot_authority_lease_error", "error": fmt.Sprint(err)})
	}
	if force && err == nil {
		t.event(map[string]any{"kind": "town_robot_authority_lease", "active": b[28] == 1, "epoch": t.observeEpoch, "members": len(t.peers)})
	}
}
