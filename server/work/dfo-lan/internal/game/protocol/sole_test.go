package protocol

import (
	"encoding/binary"
	"testing"
)

// 实机样本（2026-09-30，三个会话的 10 帧之一）：24 字节负载，含 13 字节信封。
var soleSample = []byte{
	0x50, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x80, 0x94, 0x3a, 0x70, 0x01, // 13 字节信封（令牌每局都变）
	0x03,       // [13] container = 3（已穿戴）
	0x16, 0x00, // [14..15] slot = 22（Diregie 100346156）
	0x00, 0x00, 0x00, 0x00, // [16..19] selector
	0x00, 0x00, 0x00, 0x00, // [20..23] 补零
}

func TestDecodeSoleQualityFromRealSample(t *testing.T) {
	r, err := DecodeSoleQuality(soleSample)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if r.Container != 3 || r.Slot != 22 || r.Selector != 0 {
		t.Fatalf("decoded = %+v, want container 3 slot 22 selector 0", r)
	}
	if r.PayloadOffset != 13 {
		t.Fatalf("payload offset = %d, want 13", r.PayloadOffset)
	}
}

// 带信封帧（37 字节 = 13 信封 + 24 负载）：结构自校验后字段整体后移 13 字节（与 CMD2258 同一约定）。
func TestDecodeSoleQualityWithEnvelope(t *testing.T) {
	frame := []byte{0x01, 0xF0, 0x08}          // 01 | opcode(2288 = 0x08F0) | …
	frame = append(frame, make([]byte, 10)...) // …凑满 13 字节信封
	frame = append(frame, soleSample...)       // 再加 24 字节负载（worn / 槽 22 Diregie）
	if len(frame) != 37 {
		t.Fatalf("fixture length = %d, want 37", len(frame))
	}
	if binary.LittleEndian.Uint16(frame[1:3]) != SoleQualityOpcode {
		t.Fatalf("fixture opcode = %d", binary.LittleEndian.Uint16(frame[1:3]))
	}
	r, err := DecodeSoleQuality(frame)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if r.PayloadOffset != 26 || r.Container != 3 || r.Slot != 22 {
		t.Fatalf("decoded = %+v, want offset 26 container 3 slot 22", r)
	}
}

func TestDecodeSoleQualityRejectsShortAndForeignContainer(t *testing.T) {
	if _, err := DecodeSoleQuality(soleSample[:20]); err == nil {
		t.Fatal("a 20-byte body must be rejected")
	}
	bad := append([]byte(nil), soleSample...)
	bad[13] = 7
	if _, err := DecodeSoleQuality(bad); err == nil {
		t.Fatal("container 7 must be rejected")
	}
}

func TestSoleQualityReplyShape(t *testing.T) {
	got := SoleQualityReply(3, 22)
	want := []byte{0x01, 0x03, 0x16, 0x00}
	if len(got) != len(want) {
		t.Fatalf("reply = %x, want %x", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("reply = %x, want %x", got, want)
		}
	}
}

// 材料组的选择来自请求的 selector，且**与面板显示一一对应**。
//
// 业主受控实验（2026-10-02）：打开窗口默认显示「巡礼之印（实物）」⇒ 提交 selector=0；
// 切到「金币」⇒ 提交 selector=1。判定依据必须是"面板显示 ↔ 请求字段"的对照，
// **不能**用扣料结果反推（那会被旧映射污染 —— 最初就是这么把映射搞反的）。
func TestSoleMaterialGroupSelectorMapping(t *testing.T) {
	if g, ok := SoleMaterialGroupForSelector(0); !ok || g != 0 {
		t.Fatalf("selector 0 (面板显示实物) -> group %d (ok=%v), want 0", g, ok)
	}
	if g, ok := SoleMaterialGroupForSelector(1); !ok || g != 1 {
		t.Fatalf("selector 1 (面板显示金币) -> group %d (ok=%v), want 1", g, ok)
	}
	for _, bad := range []uint32{2, 3, 0xFFFFFFFF} {
		if _, ok := SoleMaterialGroupForSelector(bad); ok {
			t.Fatalf("selector %d must be refused", bad)
		}
	}
}
