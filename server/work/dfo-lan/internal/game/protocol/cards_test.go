package protocol

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// 出站 ACK 的合法形状就是原生 reader 消费的两个 u8 (state, option)；旧的三字节
// {1, state, option} 多出来的那个前导 1 是入站请求体才有的 literal。
func TestSettlementExitAckWidth(t *testing.T) {
	for s := byte(1); s <= 2; s++ {
		for o := byte(0); o <= 3; o++ {
			got := SettlementExitSuccess(SettlementExit{State: s, Option: o})
			if len(got) != 2 || got[0] != s || got[1] != o {
				t.Fatalf("ack %d/%d not the native two-byte body: %v", s, o, got)
			}
		}
	}
}

// 旧的 selectingDungeon 是「读旧三字节 ACK 的 byte 2」；byte 2 恰好是 Option，
// 所以语义等价于 option == 1。现在由解码后的请求推导，这条用例钉住两者等价。
func TestSettlementExitSelectionFlagMatchesLegacyAckByte(t *testing.T) {
	want := map[byte]bool{0: false, 1: true, 2: false, 3: false}
	for o, expected := range want {
		r := SettlementExit{State: 1, Option: o}
		legacy := []byte{1, r.State, r.Option} // 收窄之前的出站 ACK
		if got := r.KeepsDungeonSelection(); got != expected {
			t.Fatalf("option %d: flag %v want %v", o, got, expected)
		}
		if got, legacyValue := r.KeepsDungeonSelection(), legacy[2] == 1; got != legacyValue {
			t.Fatalf("option %d: flag %v != legacy byte 2 value %v", o, got, legacyValue)
		}
	}
}

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
			check(fmt.Sprintf("card_exit_%d_%d", s, o), SettlementExitSuccess(SettlementExit{State: s, Option: o}))
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
