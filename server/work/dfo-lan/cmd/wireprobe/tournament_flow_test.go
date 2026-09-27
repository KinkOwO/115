package main

import (
	"encoding/hex"
	"testing"
)

func TestTournamentChoiceCapturedSilverDragon(t *testing.T) {
	for _, sample := range []struct {
		hex             string
		cardType, index byte
	}{
		{"00010000000000000000000000000000", 0, 1},
		{"01000000000000000000000000000000", 1, 0},
	} {
		body, err := hex.DecodeString(sample.hex)
		if err != nil {
			t.Fatal(err)
		}
		cardType, index, err := decodeTournamentChoice(body)
		if err != nil || cardType != sample.cardType || index != sample.index {
			t.Fatalf("captured CMD450 %s: type=%d index=%d err=%v", sample.hex, cardType, index, err)
		}
	}
	if _, _, err := decodeTournamentChoice([]byte{0, 1}); err == nil {
		t.Fatal("truncated choice accepted")
	}
}
