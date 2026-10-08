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

func TestBorderRewardInfoPinsOneSlotInAllThreeWords(t *testing.T) {
	for slot := uint32(40); slot < 40+borderDropSlots; slot++ {
		p, err := BorderRewardInfo(slot)
		if err != nil {
			t.Fatalf("slot %d: %v", slot, err)
		}
		if len(p) != 12 {
			t.Fatalf("slot %d: %d bytes", slot, len(p))
		}
		for i := 0; i < 3; i++ {
			// 三格填同一个值有两个理由：实测只确认了「word0 是驱动格」，填同值就
			// 不必先知道客户端取哪一格；而三格是整帧校验的，任何一格越界都会让
			// 整帧退化成 PrimevalDrop。
			if got := binary.LittleEndian.Uint32(p[i*4:]); got != slot {
				t.Fatalf("slot %d: word %d = %d", slot, i, got)
			}
		}
	}
	// 越界一律拒绝：-1 与 72 是模块构造初值（历史 bug 的两个来源），
	// 44 / 45 是本仓档位阶梯的值，不是演出表的值。
	for _, slot := range []uint32{0, 39, 44, 45, 72, 0xffffffff} {
		if _, err := BorderRewardInfo(slot); err == nil {
			t.Fatalf("out-of-window slot %d accepted", slot)
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
		p, err := BorderRewardInfo(slot)
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
