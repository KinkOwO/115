package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/wire"
	"dfolan/internal/legion"
	"encoding/binary"
	"strings"
	"testing"
)

func apocalypseSelectionSession(t *testing.T) (*legionSession, *worldSession) {
	t.Helper()
	c, err := catalog.LoadApocalypseCatalog("../../configs/apocalypse.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	clock, err := legion.NewApocalypseClock(c)
	if err != nil {
		t.Fatal(err)
	}
	s, w := &legionSession{catalog: c, clock: clock}, townSession(9)
	if _, err := s.handle(w, startPayload(uint32(legion.ApocalypseContent)), legion.CmdStart); err != nil {
		t.Fatal(err)
	}
	return s, w
}

func apocalypseOperationBody(action uint32, choice byte) []byte {
	p := make([]byte, legion.EnvelopeSize+9)
	binary.LittleEndian.PutUint32(p[13:], action)
	p[17] = choice
	binary.LittleEndian.PutUint32(p[18:], legion.OperationChannelCode)
	return p
}

func TestApocalypseDifficultyPublishedBeforeConfirm(t *testing.T) {
	for _, choice := range []byte{0, 1, 2, 4} {
		s, w := apocalypseSelectionSession(t)
		result, err := s.handle(w, apocalypseOperationBody(2, choice), legion.CmdOperationSelect)
		if err != nil || len(result.Packets) != 2 {
			t.Fatalf("choice %d: %+v %v", choice, result, err)
		}
		if s.plan.OperationID != uint32(choice)+1 || s.session.Operation != s.plan.OperationID {
			t.Fatal("choice was not used as the source difficulty key")
		}
		if result.Packets[0].ID != legion.NotiLegionInfo || result.Packets[1].ID != legion.CmdOperationSelect {
			t.Fatal("confirmation raced selected difficulty publication")
		}
		keys := make([]byte, wire.SessionKeyBytes)
		encoded, err := preparePackets(keys, result.Packets)
		if err != nil {
			t.Fatal(err)
		}
		for i, packet := range encoded {
			body, err := wire.DecryptPayload(keys, packet.ID, packet.Raw[wire.ServerHeaderSize:])
			if err != nil {
				t.Fatal(err)
			}
			if i == 0 && (len(body) < legion.LegionInfoSize || body[4] != choice || binary.LittleEndian.Uint32(body[13:]) != 0) {
				t.Fatalf("incorrect selected choice/stage: %x", body)
			}
			if i == 1 && (len(body) < legion.OperationAckSize || body[0] != 1 || binary.LittleEndian.Uint32(body[3:]) != 2) {
				t.Fatalf("incorrect confirmation: %x", body)
			}
		}
		// Re-opening the screen must preserve the accepted difficulty.
		if _, err := s.handle(w, apocalypseOperationBody(1, 255), legion.CmdOperationSelect); err != nil {
			t.Fatal(err)
		}
		reopen, err := s.handle(w, startPayload(uint32(legion.ApocalypseContent)), legion.CmdStart)
		if err != nil || reopen.Packets[1].Payload[4] != choice {
			t.Fatal("reopening lost the confirmed difficulty", err)
		}
	}
}

func TestApocalypseInvalidSelectionDoesNotChangeAcceptedPlan(t *testing.T) {
	s, w := apocalypseSelectionSession(t)
	if _, err := s.handle(w, apocalypseOperationBody(2, 1), legion.CmdOperationSelect); err != nil {
		t.Fatal(err)
	}
	old := s.plan
	for _, p := range [][]byte{apocalypseOperationBody(2, 3), apocalypseOperationBody(2, 255), apocalypseOperationBody(3, 0), apocalypseOperationBody(1, 0)} {
		result, err := s.handle(w, p, legion.CmdOperationSelect)
		if err == nil || len(result.Packets) != 0 || s.plan != old || s.session.Operation != 2 {
			t.Fatal("invalid choice changed state or was acknowledged", err)
		}
	}
	result, err := s.handle(w, startPayload(104), legion.CmdStart)
	if err == nil || len(result.Packets) != 0 || s.plan != old {
		t.Fatal("another content was acknowledged as Apocalypse")
	}
	s.catalog = nil
	if _, err := s.handle(w, apocalypseOperationBody(2, 0), legion.CmdOperationSelect); err == nil || s.plan != old {
		t.Fatal("missing source accepted")
	}
}

func TestApocalypseStageIsIndependentOfDifficultyAndCannotFakeEntry(t *testing.T) {
	s, w := apocalypseSelectionSession(t)
	if _, err := s.handle(w, apocalypseOperationBody(2, 1), legion.CmdOperationSelect); err != nil {
		t.Fatal(err)
	}
	for stage, want := range map[uint32]string{0: "operation=2 stage=0", 1: "not the active stage", 6: "not declared by the source clock"} {
		p := make([]byte, legion.EnvelopeSize+8)
		binary.LittleEndian.PutUint32(p[13:], legion.OperationChannelCode)
		binary.LittleEndian.PutUint32(p[17:], stage)
		result, err := s.handle(w, p, legion.CmdEnterDungeon)
		if err == nil || !strings.Contains(err.Error(), want) || len(result.Packets) != 0 || w.activeDungeon != nil || s.session.Phase != 0 || s.session.Operation != 2 {
			t.Fatalf("stage=%d result=%+v error=%v", stage, result, err)
		}
	}
}
