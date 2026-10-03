package protocol

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// 公共 CMD 分发器先读成功字节，CMD72 handler 再读 state/option。
func TestSettlementExitAckWidth(t *testing.T) {
	for s := byte(1); s <= 2; s++ {
		for _, o := range []byte{0, 1, 2, 3, SettlementExitSeamless} {
			got := SettlementExitSuccess(SettlementExit{State: s, Option: o})
			if len(got) != 3 || got[0] != 1 || got[1] != s || got[2] != o {
				t.Fatalf("ack %d/%d missing CMD envelope or handler bytes: %v", s, o, got)
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
			check(fmt.Sprintf("card_exit_%d_%d", s, o), SettlementExitSuccess(SettlementExit{State: s, Option: o})[1:])
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
	// source=3 与 token 外非零 padding 仍拒绝；`{1,2,0,...}` 撤退体自
	// [ISPINS-ARENA-BOSS] 起合法（见 TestIspinsSettlementExitSources）。
	for _, p := range [][]byte{{0, 0}, {3, 0}, {1, 4, 1}, {2}, {1, 2, 3, 0, 0, 0, 0, 0}, {1, 2, 1, 1, 0, 0, 0, 0}} {
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

// [ISPINS-ARENA-BOSS] 伊斯大陆会话的 CMD72 三种发送器：撤退对话框
// （2.38.2 实测 5× `01 02 00`+token）、官服奖励后退出（s4 帧 498
// `01 01 02`+token）、官服结算退出（s4 帧 342/402 `01 02 01`+token）。
// 16B 体在 p[3:8] 携带官服/私服逐字节一致的常量回执 c5 20 24 76 3f。
func TestIspinsSettlementExitSources(t *testing.T) {
	for _, raw := range []string{
		"010200c52024763f0000000000000000", // 2.38.2 实测：撤退对话框 source=0
		"010102c52024763f0000000000000000", // 官服 s4 帧 498：奖励后退出 source=2
		"010201c52024763f0000000000000000", // 官服 s4 帧 342/402：结算退出 source=1
	} {
		p, _ := hex.DecodeString(raw)
		r, e := DecodeSettlementExit(p)
		if e != nil || r.State != p[0] || r.Option != p[1] {
			t.Fatal("ispins exit refused", raw, r, e)
		}
	}
	for _, raw := range []string{
		"010200c52024763f0100000000000000", // token 后 padding 非零
		"010200c520247600",                 // token 不匹配且 padding 非零
	} {
		p, _ := hex.DecodeString(raw)
		if _, e := DecodeSettlementExit(p); e == nil {
			t.Fatal("malformed ispins exit accepted", raw)
		}
	}
	p, _ := hex.DecodeString("010200c52024763f") // 8B：state+option+source+token，无 padding
	if r, e := DecodeSettlementExit(p); e != nil || r.Option != 2 {
		t.Fatal("compact retreat body refused", r, e)
	}
	p, _ = hex.DecodeString("0102000000000000")
	if r, e := DecodeSettlementExit(p); e != nil || r.Option != 2 {
		t.Fatal("short retreat body refused", r, e)
	}
}
