package protocol

import (
	"encoding/binary"
	"testing"
)

func TestTournamentChampionSettlementNativeWidths(t *testing.T) {
	cards := [2][2]TournamentCard{{{Amount: 1600}, {Amount: 1600}}, {{Template: 3329, Amount: 1}, {Template: 3329, Amount: 3}}}
	p, err := TournamentClearReward(4, cards)
	if err != nil || len(p) != 48 || p[0] != 4 || p[6] != 2 || p[27] != 2 || binary.LittleEndian.Uint32(p[28:32]) != 3329 {
		t.Fatalf("NOTI374 body: length=%d err=%v", len(p), err)
	}
	if p := TournamentSelectState(); len(p) != 9 || p[1] != 1 || p[5] != 1 {
		t.Fatalf("CMD449 response %v", p)
	}
	if p := TournamentSelection([2]byte{1, 0}); len(p) != 7 || p[2] != 255 || p[3] != 0 || p[5] != 0 || p[6] != 255 {
		t.Fatalf("CMD450 response %v", p)
	}
}
