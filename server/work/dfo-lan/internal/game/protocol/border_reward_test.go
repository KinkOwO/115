package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

// borderDropSlots 是客户端四格掉落演出表的大小：值 − 40 = 下标
// （Unique / Legendary / Epic / Primeval）。
const borderDropSlots = 4

func TestBorderRewardInfoSplitsTheAnimationSlotFromTheRunGrade(t *testing.T) {
	for slot := uint32(40); slot < 40+borderDropSlots; slot++ {
		grade := slot + 2 // 本仓档位阶梯比演出表晚两格：42 unique → 40，45 primeval → 43
		p, err := BorderRewardInfo(slot, grade)
		if err != nil {
			t.Fatalf("slot %d: %v", slot, err)
		}
		if len(p) != 12 {
			t.Fatalf("slot %d: %d bytes", slot, len(p))
		}
		// 三段各有语义：word0 = 演出格（40..43）、word1 = 本场最高品质（40..45，
		// getHellDungeonItemMaxRarity 与副本里那颗球的阶段分支都读它）、word2 = 常量下界。
		// 2026-10-08 曾把三格统一填成演出格，于是 45 从来没有上过线（实机 2026-10-12）。
		if got := binary.LittleEndian.Uint32(p[0:4]); got != slot {
			t.Fatalf("slot %d: word0 = %d, want the animation slot", slot, got)
		}
		if got := binary.LittleEndian.Uint32(p[4:8]); got != grade {
			t.Fatalf("slot %d: word1 = %d, want the run grade %d", slot, got, grade)
		}
		if got := binary.LittleEndian.Uint32(p[8:12]); got != 40 {
			t.Fatalf("slot %d: word2 = %d, want 40", slot, got)
		}
	}
	// 越界一律拒绝：-1 与 72 是模块构造初值（历史 bug 的两个来源），而两段的值域不同。
	for _, slot := range []uint32{0, 39, 44, 45, 72, 0xffffffff} {
		if _, err := BorderRewardInfo(slot, 44); err == nil {
			t.Fatalf("out-of-window slot %d accepted", slot)
		}
	}
	for _, grade := range []uint32{0, 39, 46, 70, 72, 0xffffffff} {
		if _, err := BorderRewardInfo(42, grade); err == nil {
			t.Fatalf("out-of-window grade %d accepted", grade)
		}
	}
}

// 原生向量是 L0 证据（2026-10-07 从客户端 handler 0x1406b18d0 读出）：它记下了
// 三个位移，以及 `member58`（= +0x58，word0 落点）的构造初值 -1。
//
// 那个 -1 是**陷阱**而不是「不覆盖」：三格整帧校验，-1 越界 ⇒ 整帧退化成
// PrimevalDrop，这就是「大深渊恒定播太初动画」的成因。所以这里把它当反例钉住。
func TestBorderRewardInfoNeverEmitsTheNativeConstructorState(t *testing.T) {
	b, err := os.ReadFile("testdata/border-reward-native.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Vectors []struct {
			Name     string
			Payload  string
			Member58 int32 `json:"member58"`
		}
	}
	if err = json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Vectors) == 0 {
		t.Fatal("native vectors missing")
	}
	for _, v := range doc.Vectors {
		if v.Member58 != -1 {
			t.Fatalf("native vector %s no longer records the -1 constructor state", v.Name)
		}
		raw, err := hex.DecodeString(v.Payload)
		if err != nil || len(raw) != 12 {
			t.Fatalf("native vector %s: %v (%d bytes)", v.Name, err, len(raw))
		}
		if got := int32(binary.LittleEndian.Uint32(raw)); got != -1 {
			t.Fatalf("native vector %s: word0 = %d, want the recorded -1", v.Name, got)
		}
	}
	for slot := uint32(40); slot < 40+borderDropSlots; slot++ {
		p, err := BorderRewardInfo(slot, 44)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 3; i++ {
			if got := int32(binary.LittleEndian.Uint32(p[i*4:])); got == -1 {
				t.Fatalf("slot %d word %d reproduced the native constructor state", slot, i)
			}
		}
	}
}

// noti 2755 的 handler `sub_145310880` 是 `sub_146EA0BE0(&v4, 4)` —— **定长读 4 字节**。
// 长度不对不会报错，只会让客户端从流里多读或少读，把后面那一帧整体错位
// （2838 的 8 字节护栏在 2026-09-27 用 2839 撞过一次），所以这里把宽度钉死。
func TestRewardBoostTingInfoIsOneLittleEndianU32(t *testing.T) {
	for _, dungeon := range []uint32{100005066, 100005067, 100005068, 100004768} {
		p := RewardBoostTingInfo(dungeon)
		if len(p) != 4 {
			t.Fatalf("dungeon %d: %d bytes, want the handler's fixed 4-byte read", dungeon, len(p))
		}
		if got := binary.LittleEndian.Uint32(p); got != dungeon {
			t.Fatalf("dungeon %d encoded as %d", dungeon, got)
		}
	}
}
