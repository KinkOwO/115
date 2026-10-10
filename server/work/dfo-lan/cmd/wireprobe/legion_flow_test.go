package main

import (
	"dfolan/internal/game/wire"
	"encoding/binary"
	"reflect"

	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/legion"
	"testing"
)

func TestVenusUnsupportedRequestsRemainObservableAfterRetries(t *testing.T) {
	var notes []map[string]any
	w := townSession(1)
	w.channelType = 99
	c := &gameConnection{worldState: w, bootstrapped: true, event: func(note map[string]any) { notes = append(notes, note) }}
	for _, id := range []uint16{legion.CmdVenusOperationSelect, legion.CmdVenusEndAtPhase4} {
		seen := map[uint16]int{}
		for i := 0; i < BodySampleLimit+2; i++ {
			if !retainRequestBody(id, seen) {
				t.Fatal("Venus retry lost its evidence body")
			}
			if got := c.dispatchLegion(&clientRequest{frame: wire.Frame{Type: 1, ID: id}, verified: true, plaintext: []byte{1, 2, 3}}); got != dispatchHandled {
				t.Fatal("unsupported operation disappeared")
			}
		}
	}
	if len(notes) != 2*(BodySampleLimit+2) || notes[0]["channel_type"] != uint32(99) || notes[0]["request_hex"] != "010203" || w.activeDungeon != nil {
		t.Fatalf("unsupported audit: %v", notes)
	}
}

// townSession is a connection with a selected character and no dungeon.
func townSession(characterID int64) *worldSession {
	return &worldSession{role: database.Character{ID: characterID}}
}

// dungeonSession is the same connection inside a dungeon (world state 3), which
// is where CMD2355 is sent from and where the end-of-run packets may arrive.
func dungeonSession(characterID int64) *worldSession {
	w := townSession(characterID)
	w.activeDungeon = &dungeon.Session{}
	return w
}

// sideOf drives a guard, so a wrong answer here either rejects a packet the
// client really sends or lets a town packet through mid-run. CMD2044/CMD2046
// deliberately answer "either": a side guard does not establish ownership.
// Both sides still refuse completion when the real run executor is unbound.
func TestLegionSideClassification(t *testing.T) {
	cases := []struct {
		id   uint16
		want string
	}{
		{legion.CmdStart, "town"},
		{legion.CmdOperationSelect, "town"},
		{legion.CmdEnterDungeon, "town"},
		{legion.CmdRoleSelect, "dungeon"},
		{legion.CmdFail, "either"},
		{legion.CmdRewardEnd, "either"},
	}
	for _, c := range cases {
		if got := sideOf(c.id); got != c.want {
			t.Fatalf("sideOf(%d) = %q, want %q", c.id, got, c.want)
		}
	}
}

// Entering the channel is the precondition for every other packet in the
// family; without it the end-of-run packets must refuse rather than answer.
func TestLegionEndOfRunRefusedBeforeEntry(t *testing.T) {
	s := &legionSession{}
	w := townSession(1)

	if _, err := s.handle(w, failPayload(0), legion.CmdFail); err == nil {
		t.Fatal("CMD2044 accepted before CMD2043")
	}
	if _, err := s.handle(w, rewardEndPayload(0, 0, 0), legion.CmdRewardEnd); err == nil {
		t.Fatal("CMD2046 accepted before CMD2043")
	}
}

// A source-confirmed choice still has no real run. End requests must preserve
// that choice and every tracker field, without publishing successful packets
// or completion events. An unrelated ordinary dungeon is not an owner either.
func TestLegionEndOfRunRefusesUnownedSessionWithoutMutation(t *testing.T) {
	for _, inDungeon := range []bool{false, true} {
		for _, currentCharacter := range []int64{9, 10} {
			for _, reset := range []bool{false, true} {
				for _, request := range []struct {
					id   uint16
					body []byte
				}{
					{legion.CmdFail, failPayload(107)},
					{legion.CmdRewardEnd, rewardEndPayload(107, 22, 3)},
				} {
					s, w := apocalypseSelectionSession(t)
					if _, err := s.handle(w, apocalypseOperationBody(2, 0), legion.CmdOperationSelect); err != nil {
						t.Fatal(err)
					}
					if reset {
						s.session.Reset()
					}
					if inDungeon {
						w.activeDungeon = &dungeon.Session{}
					}
					w.role.ID = currentCharacter
					before, plan, active := *s.session, s.plan, w.activeDungeon
					for retry := 0; retry < 2; retry++ {
						result, err := s.handle(w, request.body, request.id)
						if err == nil || len(result.Packets) != 0 || len(result.Events) != 0 {
							t.Fatalf("unowned completion accepted: %d %v %v", request.id, result, err)
						}
						if !reflect.DeepEqual(before, *s.session) || s.plan != plan || w.activeDungeon != active {
							t.Fatal("refusal changed source choice or dungeon")
						}
					}
				}
			}
		}
	}
}

// CMD2355 stays dungeon-side: its only client caller requires world state 3, so
// a town-side copy means our classification — or the client's — is wrong, and
// that has to be visible in the log.
func TestLegionRoleSelectStaysDungeonOnly(t *testing.T) {
	s := &legionSession{}
	if _, err := s.handle(townSession(7), rolePayload(1, legion.OperationChannelCode), legion.CmdRoleSelect); err == nil {
		t.Fatal("CMD2355 accepted in town")
	}
	if _, err := s.handle(dungeonSession(7), rolePayload(1, legion.OperationChannelCode), legion.CmdRoleSelect); err == nil {
		t.Fatal("CMD2355 refused in a dungeon")
	}
}

// P6: leaving the dungeon closes the run, once, and only when one was open.
func TestLegionAbandonOnLeave(t *testing.T) {
	s := &legionSession{}
	if _, closed := s.abandonOnLeave("never entered", 7); closed {
		t.Fatal("abandon reported a run that never started")
	}
	if _, err := s.handle(townSession(7), startPayload(uint32(legion.ApocalypseContent)), legion.CmdStart); err != nil {
		t.Fatalf("CMD2043: %v", err)
	}
	note, closed := s.abandonOnLeave("CMD42 dungeon leave", 7)
	if !closed {
		t.Fatal("abandon did not report the open run")
	}
	if note["kind"] != "legion_run_abandoned" || note["reason"] != "CMD42 dungeon leave" {
		t.Fatalf("note %v, want legion_run_abandoned/CMD42 dungeon leave", note)
	}
	if _, again := s.abandonOnLeave("CMD42 dungeon leave", 7); again {
		t.Fatal("abandon fired twice for one run")
	}
}

// startPayload mirrors the client's CMD2043 sender: envelope + u32 @13.
func startPayload(argument uint32) []byte {
	p := make([]byte, legion.EnvelopeSize+4)
	putU32(p, legion.EnvelopeSize, argument)
	return p
}

// failPayload mirrors the client's sender: 13-byte envelope, int32 @13.
func failPayload(argument uint32) []byte {
	p := make([]byte, legion.EnvelopeSize+4)
	putU32(p, legion.EnvelopeSize, argument)
	return p
}

// rewardEndPayload mirrors the client's sender: int32 @13, int32 @17, char @21.
func rewardEndPayload(first, second uint32, flag byte) []byte {
	p := make([]byte, legion.EnvelopeSize+9)
	putU32(p, legion.EnvelopeSize, first)
	putU32(p, legion.EnvelopeSize+4, second)
	p[legion.EnvelopeSize+8] = flag
	return p
}

// rolePayload mirrors the client's sender: int32 @13 (the role), int32 @17 (107).
func rolePayload(role, channel uint32) []byte {
	p := make([]byte, legion.EnvelopeSize+8)
	putU32(p, legion.EnvelopeSize, role)
	putU32(p, legion.EnvelopeSize+4, channel)
	return p
}

func putU32(p []byte, at int, v uint32) {
	p[at] = byte(v)
	p[at+1] = byte(v >> 8)
	p[at+2] = byte(v >> 16)
	p[at+3] = byte(v >> 24)
}

// The entry answer is two frames: the acknowledgement the client is blocked on,
// then NOTI2895. The order matters because the info packet is read into the
// content object the acknowledgement has just raised, and the server had never
// sent a 2895 at all - the entry screen was showing the client's constructor
// values. A short body makes the client's reader write past what arrived.
func TestLegionStartAnswersWithAckThenInfo(t *testing.T) {
	s := &legionSession{channelType: 119}
	w := townSession(7)
	result, err := s.handle(w, startPayload(uint32(legion.ApocalypseContent)), legion.CmdStart)
	if err != nil {
		t.Fatalf("CMD2043: %v", err)
	}
	if len(result.Packets) != 2 {
		t.Fatalf("packets %d, want 2", len(result.Packets))
	}
	ack, info := result.Packets[0], result.Packets[1]
	if ack.ID != legion.CmdStart || ack.Kind != 1 || len(ack.Payload) < legion.StartAckSize {
		t.Fatalf("first frame id=%d kind=%d len=%d, want the CMD2043 acknowledgement", ack.ID, ack.Kind, len(ack.Payload))
	}
	if info.ID != legion.NotiLegionInfo || info.Kind != 0 || len(info.Payload) != legion.LegionInfoSize {
		t.Fatalf("second frame id=%d kind=%d len=%d, want NOTI2895 with %d bytes", info.ID, info.Kind, len(info.Payload), legion.LegionInfoSize)
	}
	if got := binary.LittleEndian.Uint16(info.Payload); got != legion.OperationChannelCode {
		t.Fatalf("info channel code %d, want %d", got, legion.OperationChannelCode)
	}
	// The event has to record which channel the entry arrived on and that the
	// three variable fields are still the client's own defaults, so a live log
	// cannot be read as "the counters were sent".
	if len(result.Events) != 1 {
		t.Fatalf("events %v, want one", result.Events)
	}
	note := result.Events[0]
	if note["kind"] != "legion_entered_channel" || note["channel_type"] != uint32(119) {
		t.Fatalf("event %v", note)
	}
	if note["info_values"] == nil {
		t.Fatal("event does not record which values the info packet carried")
	}
	if s.session == nil || !s.session.Entered {
		t.Fatal("session not entered after CMD2043")
	}
}
