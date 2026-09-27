package protocol

import (
	"encoding/hex"
	"testing"
)

// CMD 72 option 5 is the EPLP seamless rechallenge the client's own dungeon
// module sends (body 01 05 01, the right-edge walk-in). The decoder used to
// reject every option above 3, so that request came back as a refusal and the
// entry never became usable.
func TestSeamlessRechallengeOptionIsAccepted(t *testing.T) {
	p, _ := hex.DecodeString("01050100000000000000000000000000")
	r, e := DecodeSettlementExit(p)
	if e != nil {
		t.Fatalf("captured seamless exit refused: %v", e)
	}
	if r.State != 1 || r.Option != SettlementExitSeamless {
		t.Fatalf("decoded %+v, want state 1 option 5", r)
	}
	// It must not be treated as "keep the dungeon selection open" - option 1 is
	// the one that does that, and the seamless path raises its own select UI.
	if r.KeepsDungeonSelection() {
		t.Fatal("option 5 must not keep the selection flow")
	}
}

// Widening the range must not turn into a blanket allowance. 4 and 6 were and
// remain invalid.
func TestSeamlessWideningDoesNotAcceptEverything(t *testing.T) {
	for _, raw := range []string{
		"01040100000000000000000000000000",
		"01060100000000000000000000000000",
	} {
		q, _ := hex.DecodeString(raw)
		if _, e := DecodeSettlementExit(q); e == nil {
			t.Fatalf("option %d must stay refused", q[1])
		}
	}
}

// NOTI 261 (ENUM_NOTIPACKET_EPLP_RECHALLENGE) carries exactly one byte, and the
// client switches on it: 9 lights the continue-challenge entry, 1 greys it.
func TestEplpRechallengeBody(t *testing.T) {
	if got := EplpRechallenge(EplpRechallengeReady); len(got) != 1 || got[0] != 9 {
		t.Fatalf("ready body = %v, want [9]", got)
	}
	if got := EplpRechallenge(EplpRechallengeBlocked); len(got) != 1 || got[0] != 1 {
		t.Fatalf("blocked body = %v, want [1]", got)
	}
}
