package protocol

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestCurrentNativeSettlementPackets(t *testing.T) {
	for _, name := range []string{"play_result", "clear_reward"} {
		file := "testdata/native_" + name + "_current.json"
		if name == "play_result" {
			file = "testdata/native_play_result_record30.json"
		}
		b, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		var f struct {
			Payload   string `json:"payload_hex"`
			Consumed  int
			Truncated string `json:"truncation_failure"`
		}
		if e = json.Unmarshal(b, &f); e != nil {
			t.Fatal(e)
		}
		var p []byte
		if name == "play_result" {
			p, e = PlayResultNotice(3, 50, 50, 60000, true)
		} else {
			p, e = ClearReward(ClearRewardState{BaseExperience: 500, ScoreExperience: 50, MonsterExperience: 1234})
		}
		if e != nil || hex.EncodeToString(p) != f.Payload || len(p) != f.Consumed || f.Truncated == "" {
			t.Fatal(name, "native current result mismatch", e)
		}
	}
	b, e := os.ReadFile("testdata/native_play_result_request.json")
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
	if e != nil || r.Actor != 3 || r.RankPoint != 50 {
		t.Fatal(r, e)
	}
	full := append(append(append([]byte{}, p...), requestDigest(p)...), make([]byte, 128-len(p)-4)...)
	if _, e = DecodePlayResult(full); e != nil {
		t.Fatal(e)
	}
	if _, e = DecodePlayResult(p[:len(p)-1]); e == nil {
		t.Fatal("truncated result accepted")
	}
	p[75]++
	if _, e = DecodePlayResult(p); e == nil {
		t.Fatal("conflicting solo rank accepted")
	}
}
