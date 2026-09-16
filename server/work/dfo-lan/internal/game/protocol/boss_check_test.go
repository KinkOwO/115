package protocol

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestBossCompletionNativePackets(t *testing.T) {
	load := func(name string) []byte {
		t.Helper()
		b, e := os.ReadFile("testdata/native_" + name + ".json")
		if e != nil {
			t.Fatal(e)
		}
		var f struct {
			Payload string `json:"payload_hex"`
		}
		if e = json.Unmarshal(b, &f); e != nil {
			t.Fatal(e)
		}
		p, e := hex.DecodeString(f.Payload)
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	req := load("boss_check_request")
	p := append(append([]byte{}, req...), make([]byte, 4)...)
	r, e := DecodeBossCheck(p)
	if e != nil || r.Actor != 3 || r.Target != 0x1234 || r.Check != 0x12345678 {
		t.Fatal(r, e)
	}
	res, e := BossCheckConfirmed(r.Target)
	if e != nil || hex.EncodeToString(res) != hex.EncodeToString(load("boss_check_response")) {
		t.Fatal("native115 mismatch", e)
	}
	if hex.EncodeToString(DungeonClearEnabled()) != hex.EncodeToString(load("clear_enabled")) {
		t.Fatal("native31 mismatch")
	}
	for _, bad := range [][]byte{req, make([]byte, 39), make([]byte, 40), make([]byte, 16)} {
		if _, e = DecodeBossCheck(bad); e == nil {
			t.Fatal("malformed/90 request accepted")
		}
	}
	p[4] = 1
	if _, e = DecodeBossCheck(p); e == nil {
		t.Fatal("reserved field accepted")
	}
}
