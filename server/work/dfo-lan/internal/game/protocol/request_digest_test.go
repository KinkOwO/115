package protocol

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestLive27SkillDragDigest(t *testing.T) {
	for _, h := range []string{"000100ffffffff78f9b72d0000000000", "000200ffffffffce77974c0000000000"} {
		p, _ := hex.DecodeString(h)
		r, e := DecodeSkillMove(p)
		if e != nil || r.To != 0 {
			t.Fatal(r, e)
		}
		p[7] ^= 1
		if _, e = DecodeSkillMove(p); e == nil {
			t.Fatal("corrupted native digest accepted")
		}
	}
}
func TestLive27ClearResultDigest(t *testing.T) {
	b, e := os.ReadFile("testdata/live27_play_result.json")
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
	r, e := DecodePlayResult(p)
	if e != nil || r.Actor != 1 || r.RankPoint != 90 {
		t.Fatal(r, e)
	}
	p[113] ^= 1
	if _, e = DecodePlayResult(p); e == nil {
		t.Fatal("forged settlement digest accepted")
	}
}
