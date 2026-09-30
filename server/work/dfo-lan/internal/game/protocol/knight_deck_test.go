package protocol

import (
	"bytes"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestDecodeKnightDeckSourceShape(t *testing.T) {
	// Source-derived bodies, NOT captured native vectors. Static sender proof
	// lives in analysis/knight-shield-evidence/0x14505dab0.c. Captured native
	// requests are checked separately below.
	want := [KnightDeckSize]uint32{113370003, 113370008, 113370026, 113370029, 113370040}
	p := KnightDeckInfo(want)
	for _, body := range [][]byte{p, append(append([]byte(nil), p...), 0, 0, 0, 0)} {
		got, err := DecodeKnightDeck(body)
		if err != nil || got != want {
			t.Fatalf("decode=%v err=%v", got, err)
		}
	}
	for _, size := range []int{0, 19, 21, 23, 25, 28} {
		if _, err := DecodeKnightDeck(make([]byte, size)); err == nil {
			t.Fatalf("accepted %d bytes", size)
		}
	}
	bad := append(append([]byte(nil), p...), 0, 0, 0, 1)
	if _, err := DecodeKnightDeck(bad); err == nil {
		t.Fatal("accepted nonzero padding")
	}
}

func TestNativeKnightShieldRequests20260930(t *testing.T) {
	raw, err := os.ReadFile("testdata/native_knight_shield_requests_20260930.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Samples []struct {
			ID         uint16 `json:"id"`
			ChecksumOK bool   `json:"checksum_ok"`
			Plain      string `json:"plain_hex"`
		} `json:"samples"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Samples) != 6 {
		t.Fatalf("native sample count: %d", len(fixture.Samples))
	}
	for i, sample := range fixture.Samples {
		p, e := hex.DecodeString(sample.Plain)
		if e != nil || !sample.ChecksumOK {
			t.Fatalf("sample %d integrity: %v", i, e)
		}
		if sample.ID == 19 {
			r, e := DecodeItemMove(p)
			if e != nil || r.SourceList != 32 || r.DestinationList != 31 || r.DestinationSlot != 0 || r.SourceItem != 113370003 || r.Count != 0 || r.Selection != 0xffffffff {
				t.Fatalf("native shelf request: %+v, %v", r, e)
			}
			continue
		}
		if sample.ID != 649 || len(p) != 24 {
			t.Fatalf("sample %d shape", i)
		}
		var want [KnightDeckSize]uint32
		if i <= 3 {
			want[i+1] = 113370003
		}
		got, e := DecodeKnightDeck(p)
		if e != nil || got != want {
			t.Fatalf("native upload %d: %v != %v, %v", i, got, want, e)
		}
	}
}

func TestKnightDeckClientReaderBudgets(t *testing.T) {
	for _, tc := range []struct {
		kind  byte
		id    uint16
		body  []byte
		reads int
	}{
		{1, 649, KnightDeckAck(), 3},
		{0, 567, KnightDeckInfo([KnightDeckSize]uint32{113370003}), 20},
	} {
		if len(tc.body) != tc.reads {
			t.Fatalf("id %d body=%d reader=%d", tc.id, len(tc.body), tc.reads)
		}
		encrypted, err := wire.EncryptPayload(make([]byte, wire.SessionKeyBytes), tc.id, tc.body)
		if err != nil {
			t.Fatal(err)
		}
		frame, err := wire.ServerFrame(tc.kind, tc.id, encrypted)
		if err != nil {
			t.Fatal(err)
		}
		if tc.reads > len(frame)-16 {
			t.Fatalf("id %d reader exceeds frame: %d>%d", tc.id, tc.reads, len(frame)-16)
		}
	}
	if !bytes.Equal(KnightDeckAck(), []byte{0, 0, 0}) {
		t.Fatal("ack must stay inert")
	}
	r := ItemMoveRequest{SourceList: 31, DestinationList: 31}
	if binary.LittleEndian.Uint16(ItemMoveRefused(r, 5)[1:3]) != 5 {
		t.Fatal("silent refusal changed")
	}
}
