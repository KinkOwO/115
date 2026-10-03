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

// CMD2289 秘宝制作：**字段偏移是 13，不是 12**（本轮最容易踩的坑）。
//
// 两条实机明文样本（2026-10-02，多个会话逐字节一致）：偏移 13 读出的 u32 恰好等于源
// `[item index]`（Venus / Nabel）；按 12 读会把信封尾的 `ff` 当成模板首字节，拿到
// `0xFF8548FB` 这类垃圾值 ⇒ 查不到源 ⇒ 制作恒被拒。
var soleCreateSamples = []struct {
	name     string
	body     []byte
	template uint32
}{
	{"Venus", []byte{
		0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
		0xFE, 0xFF, 0xFF, 0xFF, 0xFF, // 13 字节信封（以 ff ff ff ff 收尾）
		0x85, 0x48, 0xFB, 0x05, // [13..16] 模板 = 0x05FB4885 = 100354181
		0x00, 0x00, 0x00, 0x00, // [17..20] selector
		0x00, 0x00, 0x00, // [21..23] 补零
	}, 100354181},
	{"Nabel", []byte{
		0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
		0xFE, 0xFF, 0xFF, 0xFF, 0xFF,
		0xE6, 0xD8, 0xFB, 0x05, // 0x05FBD8E6 = 100391142
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00,
	}, 100391142},
}

func TestDecodeSoleCreateFromRealSamples(t *testing.T) {
	for _, tc := range soleCreateSamples {
		if len(tc.body) != SoleCreatePayloadSize {
			t.Fatalf("%s: fixture length = %d, want %d", tc.name, len(tc.body), SoleCreatePayloadSize)
		}
		r, err := DecodeSoleCreate(tc.body)
		if err != nil {
			t.Fatalf("%s: decode: %v", tc.name, err)
		}
		if r.Template != tc.template || r.Selector != 0 || r.PayloadOffset != soleCreateFields {
			t.Fatalf("%s: decoded = %+v, want template %d selector 0 offset %d",
				tc.name, r, tc.template, soleCreateFields)
		}
		// 负面钉：按 12 读必须拿到不同的值 —— 这条断言把"偏移必须 13"钉死。
		if at12 := binary.LittleEndian.Uint32(tc.body[12:16]); at12 == tc.template {
			t.Fatalf("%s: 偏移 12 也读出了同一个模板，样本已失去区分度", tc.name)
		}
	}
}

// 带信封帧（37 字节 = 13 信封 + 24 负载）：结构自校验后字段整体后移 13（与 2258/2288 同一约定）。
func TestDecodeSoleCreateWithEnvelope(t *testing.T) {
	frame := []byte{0x01, 0xF1, 0x08}          // 01 | opcode(2289 = 0x08F1) | …
	frame = append(frame, make([]byte, 10)...) // …凑满 13 字节信封
	frame = append(frame, soleCreateSamples[0].body...)
	if binary.LittleEndian.Uint16(frame[1:3]) != SoleCreateOpcode {
		t.Fatalf("fixture opcode = %d", binary.LittleEndian.Uint16(frame[1:3]))
	}
	r, err := DecodeSoleCreate(frame)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if r.PayloadOffset != 26 || r.Template != soleCreateSamples[0].template {
		t.Fatalf("decoded = %+v, want offset 26 template %d", r, soleCreateSamples[0].template)
	}
}

func TestDecodeSoleCreateRejectsShortAndBadTemplate(t *testing.T) {
	if _, err := DecodeSoleCreate(soleCreateSamples[0].body[:16]); err == nil {
		t.Fatal("a 16-byte body must be rejected（最短 17 字节）")
	}
	for _, bad := range []uint32{0, 0xFFFFFFFF} {
		body := append([]byte(nil), soleCreateSamples[0].body...)
		binary.LittleEndian.PutUint32(body[13:17], bad)
		if _, err := DecodeSoleCreate(body); err == nil {
			t.Fatalf("template 0x%08X must be rejected", bad)
		}
	}
}

func TestSoleCreateReplyShape(t *testing.T) {
	got := SoleCreateReply(100354181)
	want := []byte{0x01, 0x85, 0x48, 0xFB, 0x05}
	if len(got) != len(want) {
		t.Fatalf("reply = %x, want %x", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("reply = %x, want %x", got, want)
		}
	}
}
