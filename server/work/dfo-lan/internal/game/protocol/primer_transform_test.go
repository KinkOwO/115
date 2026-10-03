package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"
)

// 实机 C2S2381 样本（2026-10-03 23:03:23，会话
// `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261003_225033_612748_next37`）。
//
// 明文 208 字节 = 客户端 `sub_146D75B10(..., 203)` 的 203 字节 + 封包层补零 5 字节。
// 期望值由 `sub_1404A7B60`（初始化器）+ `sub_141518820`（行↔槽）+ 发送侧字段归属给出：
//
//	[0:13]   栈残留（客户端从不写）⇒ 必须忽略
//	[13:17]  窗口对象 win+0x19E4（构造器写 1）
//	[17:32]  行 0 = 誓约核心槽 47 的记录（本场为空）
//	[32:36]  发送侧 id（实机 1）
//	[36:201] 11 条记录 = 晶体槽 36..46（本场只有前两条非空）
//	[201]    u8 = 0，[202] u8 = 调用方第 4 实参 = 0
const primerTransformSampleHex = "402ecfee01000000ac00000000010000002e0000ffffffff00000000ffffffff010000002e0000d101fc0500000000ffffffff2e0000b801fc0500000000ffffffff2e0000ffffffff00000000ffffffff2e0000ffffffff00000000ffffffff2e0000ffffffff00000000ffffffff2e0000ffffffff00000000ffffffff2e0000ffffffff00000000ffffffff2e0000ffffffff00000000ffffffff2e0000ffffffff00000000ffffffff2e0000ffffffff00000000ffffffff2e0000ffffffff00000000ffffffff00000000000000"

func primerTransformSample(t *testing.T) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.TrimSpace(primerTransformSampleHex))
	if err != nil {
		t.Fatalf("sample hex: %v", err)
	}
	if len(b) != PrimerTransformWireSize {
		t.Fatalf("sample is %d bytes, want %d", len(b), PrimerTransformWireSize)
	}
	return b
}

// 实机帧逐字段对位。任一处偏移写错，模板号就会落到别的字段上 —— 这条用例的价值就在这里：
// 两个非空记录必须是请求里真实出现的 100401617 / 100401592，且它们落在槽 36 / 37。
func TestDecodePrimerTransformRequestLiveSample(t *testing.T) {
	r, err := DecodePrimerTransformRequest(primerTransformSample(t))
	if err != nil {
		t.Fatal(err)
	}
	// [0:13] 是栈残留：解出来只为长度对齐，**不得当字段用**（这里连同值一起钉住，
	// 免得以后有人把它当"上下文/面板"去分支）。
	if binary.LittleEndian.Uint32(r.Residue[0:4]) != 0xEECF2E40 {
		t.Fatalf("residue[0:4] = %#x (栈残留，仅供对位)", binary.LittleEndian.Uint32(r.Residue[0:4]))
	}
	if r.WindowKey != 1 {
		t.Fatalf("window key = %d, want 1 (win+0x19E4 构造器写 1)", r.WindowKey)
	}
	if r.SelectID != 1 {
		t.Fatalf("select id = %d, want 1", r.SelectID)
	}
	if r.Tail != 0 || r.CallerArg != 0 {
		t.Fatalf("tail/caller = %d/%d, want 0/0", r.Tail, r.CallerArg)
	}
	// 行 0 = 誓约核心槽 47：本场为空。
	if !r.Oath.Empty() || r.Oath.Space != 0x2e {
		t.Fatalf("oath row = %+v, want empty with space 0x2e", r.Oath)
	}
	idx, entries, templates := r.Wanted()
	if len(templates) != 2 || idx[0] != 0 || idx[1] != 1 {
		t.Fatalf("wanted indexes = %v (templates %v), want [0 1]", idx, templates)
	}
	if templates[0] != 100401617 || templates[1] != 100401592 {
		t.Fatalf("wanted templates = %v, want [100401617 100401592]", templates)
	}
	if slot, ok := r.CrystalSlot(idx[0]); !ok || slot != 36 {
		t.Fatalf("record 0 slot = %d ok=%v, want 36", slot, ok)
	}
	if slot, ok := r.CrystalSlot(idx[1]); !ok || slot != 37 {
		t.Fatalf("record 1 slot = %d ok=%v, want 37", slot, ok)
	}
	if _, ok := r.CrystalSlot(PrimerTransformCrystalSlotCount); ok {
		t.Fatal("out-of-range record index must not map to a slot")
	}
	// 记录字段：+0 space=46、+1 slot=0、+7=0、+11=-1（发送侧第二遍填充用的行重映射值）。
	for i, e := range entries {
		if e.Space != 0x2e || e.Slot != 0 || e.Extra != 0 || e.RowIndex != PrimerTransformEmptyTemplate {
			t.Fatalf("entry %d = %+v, want space 46 slot 0 extra 0 rowindex -1", i, e)
		}
	}
	empty := 0
	for _, e := range r.Entries {
		if e.Empty() {
			empty++
		}
	}
	if empty != PrimerTransformEntryCount-2 {
		t.Fatalf("%d empty entries, want %d", empty, PrimerTransformEntryCount-2)
	}
}

// 203 字节（客户端真正 append 的长度）与 208 字节（封包层补零后）都必须能解析，
// 且结果逐字段相同：解析器不能把补零当字段读。
func TestDecodePrimerTransformAcceptsUnpaddedBody(t *testing.T) {
	full := primerTransformSample(t)
	short := full[:PrimerTransformBodySize]
	a, err := DecodePrimerTransformRequest(full)
	if err != nil {
		t.Fatal(err)
	}
	b, err := DecodePrimerTransformRequest(short)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("padded and unpadded bodies decode differently:\n%+v\n%+v", a, b)
	}
}

// 补零里出现非零 ⇒ 几何变了，必须报错而不是静默读偏。
func TestDecodePrimerTransformRejectsDirtyPadding(t *testing.T) {
	full := primerTransformSample(t)
	full[PrimerTransformBodySize] = 7
	if _, err := DecodePrimerTransformRequest(full); err == nil {
		t.Fatal("non-zero padding must be rejected")
	}
}

// 长度越界必须拒绝：短了会把零当字段，长了说明不是这个命令。
func TestDecodePrimerTransformRejectsBadLength(t *testing.T) {
	full := primerTransformSample(t)
	for _, n := range []int{0, 13, 202, 209} {
		var body []byte
		if n <= len(full) {
			body = full[:n]
		} else {
			body = make([]byte, n)
		}
		if _, err := DecodePrimerTransformRequest(body); err == nil {
			t.Fatalf("%d-byte body must be rejected", n)
		}
	}
}

// 回包几何：正文 7 字节、首字节是**成功状态**（非 0）、payload[4] 选窗口。
//
// ★ 誓约变换必须发 `window = 0`（窗口 2145 = EquipmentTransformWindow 本体）：
// 只有那一支会 setState(3) → 弹 DSTR 101039076 并切到 2144（Oath settings）。
func TestPrimerTransformReplyShape(t *testing.T) {
	out := PrimerTransformReply(0, 0)
	if len(out) != PrimerTransformReplySize {
		t.Fatalf("reply = %d bytes, want %d", len(out), PrimerTransformReplySize)
	}
	if out[0] != 1 {
		t.Fatalf("status prefix = %d, want non-zero (成功不读提示码)", out[0])
	}
	if out[5] != 0 || out[6] != 0 {
		t.Fatalf("window/variant = %d/%d, want 0/0 (窗口 2145)", out[5], out[6])
	}
	// 与 2259 的编码器形状一致（同样 7 字节、同样 payload[4]/[5] 位置）。
	if got, want := out, EquipmentCraftReply(0, 0); string(got) != string(want) {
		t.Fatalf("primer reply % x != equipment reply % x", got, want)
	}
}

// 初始化器的默认值本身就是几何的一部分：记录里 `+3` 是模板（-1 = 空）、`+11` 默认 -1，
// 而 `+0/+1` 是 (space, slot)。这条用例用"全默认正文"钉住偏移，防止以后有人按 2259 的
// 7 字节记录去套。
func TestDecodePrimerTransformDefaultGeometry(t *testing.T) {
	body := make([]byte, PrimerTransformBodySize)
	body[17] = 0x2e
	binary.LittleEndian.PutUint32(body[20:], PrimerTransformEmptyTemplate)
	binary.LittleEndian.PutUint32(body[28:], PrimerTransformEmptyTemplate)
	for i := 0; i < PrimerTransformEntryCount; i++ {
		off := PrimerTransformHeaderSize + i*PrimerTransformEntrySize
		body[off] = 0x2e
		binary.LittleEndian.PutUint32(body[off+3:], PrimerTransformEmptyTemplate)
		binary.LittleEndian.PutUint32(body[off+11:], PrimerTransformEmptyTemplate)
	}
	binary.LittleEndian.PutUint32(body[13:], 0xFFFFFFFF)
	r, err := DecodePrimerTransformRequest(body)
	if err != nil {
		t.Fatal(err)
	}
	if r.WindowKey != 0xFFFFFFFF {
		t.Fatalf("window key = %#x, want 0xFFFFFFFF (u32 field at +13)", r.WindowKey)
	}
	if _, _, templates := r.Wanted(); len(templates) != 0 {
		t.Fatalf("all-empty body yielded %v", templates)
	}
	if r.Oath.RowIndex != 0xFFFFFFFF {
		t.Fatalf("oath row rowindex = %#x, want 0xFFFFFFFF", r.Oath.RowIndex)
	}
}
