package main

import (
	"encoding/binary"

	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/legion"
	"testing"
)

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
// deliberately answer "either": their call sites have not been read, and
// accepting them while logging the arriving side is what turns the first live
// run into the evidence (next64 §6.2, P5).
//
// CMD2355 answers "either" for the same reason, with live evidence: the
// apocalypse capture 20261008-105227 shows the client sending it twice inside
// the waiting room the client itself loaded with CMD2062, at a moment when the
// server has no activeDungeon because CMD2045 only confirms the operation. A
// dungeon-only guard rejects both packets (see apocalypseRole in
// apocalypse_run.go), which is a live bug, not a stricter check.
func TestLegionSideClassification(t *testing.T) {
	cases := []struct {
		id   uint16
		want string
	}{
		{legion.CmdStart, "town"},
		{legion.CmdOperationSelect, "town"},
		{legion.CmdEnterDungeon, "town"},
		{legion.CmdRoleSelect, "either"},
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

// The ack length is the whole contract: the client's handler reads a fixed
// number of bytes off the packet and dies on a short read (next65 §2.2).
func TestLegionFailAckMeetsClientReadSize(t *testing.T) {
	s := &legionSession{}
	w := townSession(7)
	if _, err := s.handle(w, startPayload(1), legion.CmdStart); err != nil {
		t.Fatalf("CMD2043: %v", err)
	}

	result, err := s.handle(w, failPayload(0x2A), legion.CmdFail)
	if err != nil {
		t.Fatalf("CMD2044: %v", err)
	}
	if len(result.Packets) != 1 {
		t.Fatalf("packets %d, want 1", len(result.Packets))
	}
	p := result.Packets[0]
	if p.ID != legion.CmdFail || len(p.Payload) < legion.FailAckSize {
		t.Fatalf("ack id=%d len=%d, want id=%d len>=%d", p.ID, len(p.Payload), legion.CmdFail, legion.FailAckSize)
	}
	if !s.session.Failed {
		t.Fatal("session not marked failed")
	}
	// The run keeps its operation: teardown is CMD2046's job, so the log can
	// still show which of the two the client chose.
	if len(result.Events) != 1 || result.Events[0]["kind"] != "legion_run_failed" {
		t.Fatalf("events %v, want one legion_run_failed", result.Events)
	}
	if result.Events[0]["arriving_side"] != "town" {
		t.Fatalf("arriving_side %v, want town", result.Events[0]["arriving_side"])
	}
}

func TestLegionRewardEndAckMeetsClientReadSize(t *testing.T) {
	s := &legionSession{}
	w := townSession(7)
	if _, err := s.handle(w, startPayload(1), legion.CmdStart); err != nil {
		t.Fatalf("CMD2043: %v", err)
	}

	result, err := s.handle(w, rewardEndPayload(11, 22, 3), legion.CmdRewardEnd)
	if err != nil {
		t.Fatalf("CMD2046: %v", err)
	}
	if len(result.Packets) != 1 {
		t.Fatalf("packets %d, want 1", len(result.Packets))
	}
	p := result.Packets[0]
	if p.ID != legion.CmdRewardEnd || len(p.Payload) < legion.RewardEndAckSize {
		t.Fatalf("ack id=%d len=%d, want id=%d len>=%d", p.ID, len(p.Payload), legion.CmdRewardEnd, legion.RewardEndAckSize)
	}
	// The reward screen being done ends the run.
	if s.session.Entered {
		t.Fatal("session still marked entered after CMD2046")
	}
	if len(result.Events) != 1 || result.Events[0]["kind"] != "legion_run_reward_end" {
		t.Fatalf("events %v, want one legion_run_reward_end", result.Events)
	}
	note := result.Events[0]
	if note["first"] != uint32(11) || note["second"] != uint32(22) || note["flag"] != byte(3) {
		t.Fatalf("decoded fields %v/%v/%v, want 11/22/3", note["first"], note["second"], note["flag"])
	}
	// No catalog was loaded, so the table columns must read as absent rather
	// than as an empty reward, which would look like a real answer.
	if note["table_reward_values"] != nil {
		t.Fatalf("table_reward_values %v, want nil without a catalog", note["table_reward_values"])
	}
}

// The side of CMD2044/CMD2046 is unestablished, so both sides have to work —
// otherwise the guard would silently decide the question. CMD2043 is town-side
// (the entry gate requires world state 1), so the dungeon case opens the run in
// town and then walks the player into the instance, which is the real order.
func TestLegionEndOfRunAcceptedOnEitherSide(t *testing.T) {
	for _, inDungeon := range []bool{false, true} {
		s := &legionSession{}
		w := townSession(7)
		if _, err := s.handle(w, startPayload(1), legion.CmdStart); err != nil {
			t.Fatalf("CMD2043: %v", err)
		}
		if inDungeon {
			w.activeDungeon = &dungeon.Session{}
		}
		result, err := s.handle(w, failPayload(0), legion.CmdFail)
		if err != nil {
			t.Fatalf("CMD2044 refused (inDungeon=%v): %v", inDungeon, err)
		}
		want := "town"
		if inDungeon {
			want = "dungeon"
		}
		if result.Events[0]["arriving_side"] != want {
			t.Fatalf("arriving_side %v, want %s", result.Events[0]["arriving_side"], want)
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
	if _, err := s.handle(townSession(7), startPayload(1), legion.CmdStart); err != nil {
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
	result, err := s.handle(w, startPayload(1), legion.CmdStart)
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
