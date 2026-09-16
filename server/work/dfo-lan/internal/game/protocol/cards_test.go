package protocol

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestCurrentNativeCardPackets(t *testing.T) {
	check := func(name string, p []byte) {
		t.Helper()
		b, e := os.ReadFile("testdata/native_" + name + ".json")
		if e != nil {
			t.Fatal(e)
		}
		var f struct {
			Payload  string `json:"payload_hex"`
			Consumed int
		}
		if e = json.Unmarshal(b, &f); e != nil {
			t.Fatal(e)
		}
		if hex.EncodeToString(p) != f.Payload || len(p) != f.Consumed {
			t.Fatal(name, "native cursor mismatch")
		}
	}
	check("card_scroll", nil)
	check("card_layout", CardLayout()[1:])
	for i := 0; i < 4; i++ {
		p, e := CardSelected(i)
		if e != nil {
			t.Fatal(e)
		}
		check(fmt.Sprintf("card_free_%d", i), p[1:])
	}
	for s := byte(1); s <= 2; s++ {
		for o := byte(0); o <= 2; o++ {
			check(fmt.Sprintf("card_exit_%d_%d", s, o), SettlementExitSuccess(SettlementExit{s, o})[1:])
		}
	}
	r := ClearRewardState{BaseExperience: 500, ScoreExperience: 50, MonsterExperience: 1234}
	r.Cards[0] = []CardReward{{Template: 0, Amount: 5}, {Template: 1012, Amount: 1}}
	p, e := ClearReward(r)
	if e != nil {
		t.Fatal(e)
	}
	check("clear_reward_cards", p)
	for _, p := range [][]byte{nil, {0}, {2, 0}, {0, 4}, {0, 0, 1}, {0, 0, 0, 0, 0, 0, 0, 1}} {
		if _, e := DecodeCardSelection(p); e == nil {
			t.Fatal("malformed card selection", p)
		}
	}
	for _, p := range [][]byte{{0, 0}, {3, 0}, {1, 4, 1}, {2}, {1, 2, 0, 0, 0, 0, 0, 0}, {1, 2, 1, 1, 0, 0, 0, 0}} {
		if _, e := DecodeSettlementExit(p); e == nil {
			t.Fatal("malformed exit", p)
		}
	}
	for _, raw := range []string{"01020100000000000000000000000000", "01030100000000000000000000000000"} {
		p, _ := hex.DecodeString(raw)
		r, e := DecodeSettlementExit(p)
		if e != nil || r.State != 1 || r.Option != p[1] {
			t.Fatal("captured exit refused", r, e)
		}
	}
}
