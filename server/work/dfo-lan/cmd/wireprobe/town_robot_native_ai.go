package main

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Explicit local client adapter. No new game protocol or player persistence.
// Once handed over, the server never resumes coordinate following in the same
// room: a lost/failed adapter halts, avoiding concurrent controllers or snaps.
type robotNativeAI struct {
	offerPath, ackPath string
	generation         uint64
	phase              uint32
	started, lastAck   time.Time
	wroteDrive         bool
	deaths             [3]robotNativeDeath
}

func (t *townRobotTransport) resetNativeAILocked() {
	t.nativeAI = robotNativeAI{}
	if os.Getenv("DFO_LOCAL_ROBOT_AI") != "r23-native-input-v1" || t.authorityPath == "" {
		return
	}
	offer, ack := os.Getenv("DFO_ROBOT_AI_OFFER"), os.Getenv("DFO_ROBOT_AI_ACK")
	dir := filepath.Dir(t.authorityPath)
	if !filepath.IsAbs(offer) || !filepath.IsAbs(ack) || filepath.Dir(offer) != dir || filepath.Dir(ack) != dir || filepath.Base(offer) != "ai-offer.bin" || filepath.Base(ack) != "ai-ack.bin" {
		return
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return
	}
	id := binary.LittleEndian.Uint64(nonce[:])
	if id == 0 {
		return
	}
	t.nativeAI = robotNativeAI{offerPath: offer, ackPath: ack, generation: id, phase: 1}
}

func nativeAIMessage(party [64]byte, generation uint64, phase uint32) [80]byte {
	var b [80]byte
	copy(b[:64], party[:])
	binary.LittleEndian.PutUint64(b[64:], generation)
	binary.LittleEndian.PutUint32(b[72:], phase)
	return b
}
func validNativeAIAck(ack []byte, offer [80]byte, now time.Time) (uint32, bool) {
	if (len(ack) != 80 && len(ack) != 104) || !bytes.Equal(ack[:16], offer[:16]) || !bytes.Equal(ack[24:72], offer[24:72]) || binary.LittleEndian.Uint32(ack[76:]) == 0 {
		return 0, false
	}
	issued := binary.LittleEndian.Uint64(ack[16:24])
	ms := uint64(now.UnixMilli())
	if issued > ms || ms-issued > 2500 {
		return 0, false
	}
	phase := binary.LittleEndian.Uint32(ack[72:])
	return phase, phase >= 1 && phase <= 3
}
func writeNativeAIFile(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".robot-ai-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	_, err = f.Write(b)
	if e := f.Close(); err == nil {
		err = e
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	return err
}

// Caller holds transport and authority-publisher locks. Only the current party
// publisher may write offers; retired transports cannot revoke a newer party.
func (t *townRobotTransport) publishNativeAILocked(now time.Time, party [64]byte) {
	a := &t.nativeAI
	if a.offerPath == "" {
		return
	}
	phase := a.phase
	active := binary.LittleEndian.Uint32(party[28:]) == 1
	if !active {
		phase = 3
	} else {
		placed := len(t.peers) > 0
		for _, p := range t.peers {
			placed = placed && len(p.history) > 0
		}
		// Loading can pause actor frames for much longer than the AI setup
		// timeout. Start that timeout only after every initial room placement.
		if phase == 1 && placed && a.started.IsZero() {
			a.started = now
		}
		offer := nativeAIMessage(party, a.generation, phase)
		ack, err := os.ReadFile(a.ackPath)
		ackPhase, valid := validNativeAIAck(ack, offer, now)
		if err == nil && valid {
			if ackPhase == 3 {
				phase = 3
			} else {
				a.lastAck = now
				if phase == 2 && ackPhase == 2 {
					t.observeNativeDeathsLocked(ack, now)
				}
				if phase == 1 && ackPhase == 1 && placed {
					phase = 2
				}
			}
		}
		if phase == 1 && !a.started.IsZero() && now.Sub(a.started) > 10*time.Second {
			phase = 3
		}
		if phase == 2 && now.Sub(a.lastAck) > 2500*time.Millisecond {
			phase = 3
		}
	}
	offer := nativeAIMessage(party, a.generation, phase)
	if err := writeNativeAIFile(a.offerPath, offer[:]); err != nil {
		// If DRIVE was ever published, leave the server fenced even if a later
		// write fails. The client's existing lease will expire independently.
		if a.phase != 3 {
			a.phase = 3
			t.event(map[string]any{"kind": "town_robot_native_ai_error", "error": fmt.Sprint(err)})
		}
		return
	}
	if phase == 2 {
		a.wroteDrive = true
	}
	if phase != a.phase {
		t.event(map[string]any{"kind": "town_robot_native_ai_phase", "phase": phase, "epoch": t.observeEpoch, "generation": a.generation})
	}
	a.phase = phase
}
func (t *townRobotTransport) nativeAIControlsLocked() bool {
	return t.nativeAI.offerPath != "" && (t.nativeAI.phase == 2 || t.nativeAI.phase == 3)
}
