package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func nativePartyResult(t *testing.T, count int) ([]byte, []uint16) {
	t.Helper()
	raw, err := os.ReadFile(fmt.Sprintf("testdata/native_party_result115_%d.json", count))
	if err != nil {
		t.Fatal(err)
	}
	var native struct {
		Payload string `json:"payload_hex"`
		Actors  []uint16
		SHA     string `json:"client_sha256"`
	}
	if err = json.Unmarshal(raw, &native); err != nil {
		t.Fatal(err)
	}
	if native.SHA != "515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb" {
		t.Fatal("wrong client writer")
	}
	p, err := hex.DecodeString(native.Payload)
	if err != nil {
		t.Fatal(err)
	}
	return p, native.Actors
}

func TestPartyPlayResult115FromNativeWriter(t *testing.T) {
	for _, count := range []int{1, 2, 4, 8} {
		p, actors := nativePartyResult(t, count)
		for _, actor := range actors {
			result, err := DecodePartyPlayResult115(p, actor, actors)
			if err != nil || result.Actor != actor || result.RankPoint != 50 {
				t.Fatal(count, actor, result, err)
			}
			full := append(append([]byte{}, p...), requestDigest(p)...)
			full = append(full, make([]byte, ((len(full)+15)/16)*16-len(full))...)
			if _, err := DecodePartyPlayResult115(full, actor, actors); err != nil {
				t.Fatal(count, err)
			}
			full[len(p)] ^= 1
			if _, err := DecodePartyPlayResult115(full, actor, actors); err == nil {
				t.Fatal("bad checksum accepted")
			}
		}
		if _, err := DecodePartyPlayResult115(p[:len(p)-1], actors[0], actors); err == nil {
			t.Fatal("truncated native result accepted")
		}
	}
}

func TestPartyPlayResult115RejectsForgedMembership(t *testing.T) {
	p, actors := nativePartyResult(t, 2)
	if _, err := DecodePartyPlayResult115(p, 999, actors); err == nil {
		t.Fatal("foreign requester accepted")
	}
	if _, err := DecodePartyPlayResult115(p, actors[0], actors[:1]); err == nil {
		t.Fatal("incomplete roster accepted")
	}
	for _, actor := range []uint16{actors[0], 0, 65535, 999} {
		forged := append([]byte{}, p...)
		binary.LittleEndian.PutUint16(forged[11:], actor)
		if _, err := DecodePartyPlayResult115(forged, actors[0], actors); err == nil {
			t.Fatal("forged result row accepted", actor)
		}
	}
}

func TestPartyPlayResult115UsesRequestersOwnRow(t *testing.T) {
	p, actors := nativePartyResult(t, 2)
	p[10], p[17] = 41, 73
	for i, actor := range actors {
		got, err := DecodePartyPlayResult115(p, actor, actors)
		if err != nil || got.Actor != actor || got.RankPoint != []byte{41, 73}[i] {
			t.Fatal("result used another member's rank", actor, got, err)
		}
	}
	// Native row order does not authorize identity: the server-owned set does.
	first := append([]byte(nil), p[4:11]...)
	copy(p[4:11], p[11:18])
	copy(p[11:18], first)
	got, err := DecodePartyPlayResult115(p, actors[0], actors)
	if err != nil || got.RankPoint != 41 {
		t.Fatal("row reorder changed reward owner", got, err)
	}
}
