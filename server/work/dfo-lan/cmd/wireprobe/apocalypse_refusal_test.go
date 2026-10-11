package main

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/legion"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestApocalypseRejectedRequestAnswersWithoutChangingRun(t *testing.T) {
	for _, id := range []uint16{legion.CmdEnterDungeon, legion.CmdFail} {
		for _, broken := range []bool{false, true} {
			s, w := navigationEntryFixture(t)
			s.plan = nil // actual live C2045 arrived before difficulty confirmation
			party, session, before := s.party, *s.session, append([]byte(nil), s.projection...)
			body := navigationRequest()
			if id == legion.CmdFail {
				body = failPayload(0)
			}
			a, peer := net.Pipe()
			peer.SetDeadline(time.Now().Add(5 * time.Second))
			var notes []map[string]any
			log := func(note map[string]any) { notes = append(notes, note) }
			keys := make([]byte, wire.SessionKeyBytes)
			c := &gameConnection{worldState: w, legionState: *s, bootstrapped: true, event: log, output: newConnectionOutput(a, keys, "test", log)}
			done := make(chan dispatchAction, 1)
			go func() {
				done <- c.dispatchLegion(&clientRequest{frame: wire.Frame{Type: 1, ID: id}, verified: true, plaintext: body})
			}()
			if broken {
				peer.Close()
			} else {
				frame := readDispatchServerFrame(t, peer)
				plain, err := wire.DecryptPayload(keys, id, frame[wire.ServerHeaderSize:])
				if err != nil || frame[0] != 1 || binary.LittleEndian.Uint16(frame[1:]) != id || len(plain) < 3 || !bytes.Equal(plain[:3], protocol.Refusal(4)) {
					t.Fatal("rejected request did not receive its native failure envelope", err)
				}
			}
			action := <-done
			if broken && action != dispatchClose || !broken && action != dispatchHandled {
				t.Fatal("wrong refusal dispatch action", action)
			}
			if c.legionState.party != party || *c.legionState.session != session || c.legionState.plan != nil || !bytes.Equal(c.legionState.projection, before) || w.activeDungeon != nil || w.apocalypseRun != nil {
				t.Fatal("failure acknowledged a run or changed its owner")
			}
			if !broken {
				selected, err := c.legionState.handle(w, apocalypseOperationBody(2, 1), legion.CmdOperationSelect)
				if err != nil {
					t.Fatal(err)
				}
				c.legionState.adoptProjection(selected.Packets)
				entry, err := c.legionState.handle(w, navigationRequest(), legion.CmdEnterDungeon)
				if err != nil || entry.Navigation == nil {
					t.Fatal("confirmed retry could not prepare navigation", err)
				}
				if notes[len(notes)-1]["kind"] != "apocalypse_request_refused_ack" {
					t.Fatal("native refusal missing from audit")
				}
			}
			a.Close()
			peer.Close()
		}
	}
}

func TestOtherLegionRefusalKeepsExistingDispatch(t *testing.T) {
	w := townSession(7)
	w.channelType = 99
	c := &gameConnection{worldState: w, bootstrapped: true, event: func(map[string]any) {}}
	for _, id := range []uint16{legion.CmdEnterDungeon, legion.CmdFail, legion.CmdRewardEnd} {
		if got := c.dispatchLegion(&clientRequest{frame: wire.Frame{Type: 1, ID: id}, verified: true}); got != dispatchHandled {
			t.Fatal("other content refusal changed")
		}
	}
}
