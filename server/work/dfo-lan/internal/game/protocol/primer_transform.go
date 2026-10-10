package protocol

import (
	"encoding/binary"
	"fmt"
)

// ---------------------------------------------------------------------------
// CMD2381：装备库「誓约 / 晶体变换」（ENUM_CMDPACKET_PRIMER_TRANSFORM）
// ---------------------------------------------------------------------------
//
// 客户端文案（DSTR 101038946 / 101038833）叫 **Convert Oath/Crystal**，入口是装备库
// `[new peculiar group]` mark 4 / index 16227 `[button type] \`primer transform\“ 那一页。
//
// 取证（2026-10-03，权威 IDB 工作副本 + 实机帧，产物 analysis/tmp-primer-transform/ida/）：
//
//	发送方   sub_14150C4D0（与 2259 同一个"装备库变换窗口"发送函数；另一处 sub_1404AD130 同布局）
//	         `sub_146D746E0(w, 2381)` 写信封 → `sub_146D75B10(w, buf, 203)` 追加正文
//	初始化   sub_1404A7B60(buf)（正文几何的唯一真源，逐字节与实机帧吻合）
//	handler  sub_145287A40(a1, status, code)：固定读 **6 字节**（与 2259 的 sub_145277D00 同形）
//	行↔槽    sub_141518820(win, mode, sel)：**行 0 = 誓约核心 47**，行 k(1..11) = **槽 35+k = 36..46**
//	         （独立复证：sub_14150B940 对 space==3 的晶体调用 sub_14150F150(win, slot-35, item, 3, slot, 0)）
//
// ⚠️ **正文长度是 203，实机明文是 208**：封包层把正文补零到 8 字节边界（203 → 208；
// 2259 的 121 → 128 同规律）。这里按 203 布局解析、容忍 203..208，并要求补零部分全 0。
const PrimerTransformOpcode uint16 = 2381

const (
	// PrimerTransformBodySize 是客户端实际 append 的正文长度（`sub_146D75B10(...,203)`）。
	PrimerTransformBodySize = 203
	// PrimerTransformWireSize 是实机明文长度（203 + 5 字节封包层补零）。
	PrimerTransformWireSize = 208
	// PrimerTransformHeaderSize 是记录数组之前的头长度。
	//
	//	[0:13]   **客户端从不写**（穷举发送方全部栈操作数证明）⇒ 实机里的
	//	         40 2e cf ee / 01 00 00 00 / ac 00 00 00 / 00 是**栈残留**，服务端整段忽略。
	//	[13:17]  u32 = 窗口对象 `win+0x19E4`（构造器 sub_1414F1A00 写 1，客户端恒 1）
	//	[17:32]  **行 0 记录**（15 字节）= 誓约核心槽 47
	//	[32:36]  u32：发送侧写 `sub_14206BB60(...)` 或回退 `sub_145CD4300(...)`（实机 1）
	PrimerTransformHeaderSize = 36
	// PrimerTransformEntryCount / Size 是 11 条 15 字节记录（晶体槽 36..46）。
	PrimerTransformEntryCount = 11
	PrimerTransformEntrySize  = 15
	// PrimerTransformEmptyTemplate 是空槽哨兵（初始化器写 -1）。
	PrimerTransformEmptyTemplate = 0xFFFFFFFF
	// PrimerTransformPayloadSize 是 handler 固定读的字节数。
	PrimerTransformPayloadSize = 6
	// PrimerTransformReplySize 是应答正文总长 = 1 字节状态前缀 + 6 字节窗口指令。
	//
	// **状态字节是硬契约**：kind 1 的收包分发器先读 body[0] 当 status，
	// **只有 status == 0 时**才再读一个 u16 提示码；正文不足 7 字节会 `MEMORY[0]=0` 崩客户端。
	// status != 0 = 成功（不读提示码）；status == 0 = 服务端给提示码，走 sub_146ADFC80 消息表。
	PrimerTransformReplySize = PrimerTransformPayloadSize + 1
	// PrimerTransformOathSlot 是"行 0"对应的穿戴槽（誓约核心）。
	PrimerTransformOathSlot = 47
	// PrimerTransformCrystalSlotBase / Count 是行 1..11 对应的晶体槽（36..46）。
	PrimerTransformCrystalSlotBase  = 36
	PrimerTransformCrystalSlotCount = 11
)

// PrimerTransformEntry 是 2381 正文里的一条 15 字节记录。
//
// 字段语义由 `sub_1415141E0` / `sub_1414F7DA0` 闭环（不是按外形猜的）：
//
//	Space    u8   @0  **物品所在 space**（`mov [rcx], r8d`，即 sub_14150F150 的第 3 实参）
//	Slot     u16  @1  **该 space 内的槽号**（`mov [rcx+4], r9d`）
//	Template u32  @3  物品模板 id（发送侧 `sub_1421B2820`）
//	Extra    u32  @7  `*(u8*)((vtbl+1080)(item)+285)`
//	RowIndex u32  @11 客户端行重映射下标（`*(u32*)(elem+0x118)`）；第二遍填包按它搬行，实机 -1
type PrimerTransformEntry struct {
	Space    byte
	Slot     uint16
	Template uint32
	Extra    uint32
	RowIndex uint32
}

// Empty 报告这条记录是否为空槽。
func (e PrimerTransformEntry) Empty() bool {
	return e.Template == 0 || e.Template == PrimerTransformEmptyTemplate
}

// PrimerTransformRequest 是 CMD2381 的解析结果。
//
// `Oath` 是正文 `+17` 的行 0（誓约核心槽 47）；`Entries[i]` 是第 i 条晶体记录（槽 36+i）。
type PrimerTransformRequest struct {
	// The first 13 bytes are never written by this client; they are stack residue
	// and must not be interpreted. They are kept only so the offsets line up.
	Residue   [13]byte
	WindowKey uint32
	Oath      PrimerTransformEntry
	SelectID  uint32
	Entries   [PrimerTransformEntryCount]PrimerTransformEntry
	Tail      byte
	CallerArg byte
}

// PagePayloadVariant 是正文 `+12` 字节（参考实现里称 `f12`）的取值：**整页组合负载**。
//
// 取证来源：另一棵树的实机抓包定案（`equipment-journal-20261004/MERGE_REQUEST.md` 第 1 节）——
// 「f12=1 弹窗单件指派、f12=0 页面组合负载，两者都应用」。本仓此前的注释把 `+12` 当作
// "客户端从不写入的栈残留"，因此**从未解码过它**；成套替换（你的报告：A 套 8 件换成 B 套 4 件后
// 变成 4+4，而不是只剩 4 件）正需要这个变体来区分"单件指派"与"整页期望状态"。
//
// ⚠️ 目前只**解码并落日志**，不改变行为：本仓还没有一条实机 f12=0 的完整帧（需要用户点一次
// 「一键替换/成套替换」抓取），按 AGENTS §0.3 不据此猜包。
const PagePayloadVariant byte = 0

// Variant 返回正文 `+12` 的变体字节（0 = 整页组合负载，1 = 弹窗单件指派）。
func (r PrimerTransformRequest) Variant() byte { return r.Residue[12] }

// Wanted 列出被点选的非空晶体记录：返回记录下标、记录与模板。三者等长、一一对应。
//
// 下标是**记录位次 0..10**，对应穿戴槽 `36+i`（见 `PrimerTransformCrystalSlotBase`）。
func (r PrimerTransformRequest) Wanted() (indexes []int, entries []PrimerTransformEntry, templates []uint32) {
	for i, e := range r.Entries {
		if e.Empty() {
			continue
		}
		indexes = append(indexes, i)
		entries = append(entries, e)
		templates = append(templates, e.Template)
	}
	return indexes, entries, templates
}

// CrystalSlot 把记录位次翻成穿戴槽。
func (r PrimerTransformRequest) CrystalSlot(index int) (uint16, bool) {
	if index < 0 || index >= PrimerTransformCrystalSlotCount {
		return 0, false
	}
	return uint16(PrimerTransformCrystalSlotBase + index), true
}

// decodePrimerTransformEntry 读一条 15 字节记录。
func decodePrimerTransformEntry(p []byte) PrimerTransformEntry {
	return PrimerTransformEntry{
		Space:    p[0],
		Slot:     binary.LittleEndian.Uint16(p[1:3]),
		Template: binary.LittleEndian.Uint32(p[3:7]),
		Extra:    binary.LittleEndian.Uint32(p[7:11]),
		RowIndex: binary.LittleEndian.Uint32(p[11:15]),
	}
}

// DecodePrimerTransformRequest 解析 CMD2381 正文。
//
// 长度口径：客户端 append 的是 203 字节，封包层补零到 8 字节边界后实机看到 208。
// 这里两者都收，但补零部分必须全 0 —— 非零说明几何变了，宁可报错也不要读错字段。
func DecodePrimerTransformRequest(p []byte) (PrimerTransformRequest, error) {
	var out PrimerTransformRequest
	if len(p) < PrimerTransformBodySize || len(p) > PrimerTransformWireSize {
		return out, fmt.Errorf("primer transform requires a %d..%d-byte body, got %d",
			PrimerTransformBodySize, PrimerTransformWireSize, len(p))
	}
	for _, b := range p[PrimerTransformBodySize:] {
		if b != 0 {
			return out, fmt.Errorf("primer transform padding is not zero")
		}
	}
	copy(out.Residue[:], p[0:13])
	out.WindowKey = binary.LittleEndian.Uint32(p[13:17])
	out.Oath = decodePrimerTransformEntry(p[17:32])
	out.SelectID = binary.LittleEndian.Uint32(p[32:36])
	for i := 0; i < PrimerTransformEntryCount; i++ {
		off := PrimerTransformHeaderSize + i*PrimerTransformEntrySize
		out.Entries[i] = decodePrimerTransformEntry(p[off : off+PrimerTransformEntrySize])
	}
	out.Tail = p[201]
	out.CallerArg = p[202]
	return out, nil
}

// PrimerTransformReply 是 CMD2381 的应答正文（1 字节状态 + handler 读的 6 字节）。
//
// ★ 窗口选择字节（`payload[4]`）必须发 **0**：`0` ⇒ 窗口 **2145 = EquipmentTransformWindow**
// 本体（发送方 `sub_14150C4D0` 的唯一上层调用者就是它的 `setState`），成功时
// `sub_14151BAE0(win,1)` → `sub_14150F5A0(win,3)` = setState(3) → 弹
// **DSTR 101039076「Oath/Crystal conversion completed! Moving to the Oath settings.」**
// 并切到窗口 2144（Oath settings）。发非 0 会走窗口 3937（`sub_1404AE520`，弹的是
// DSTR 101038235「Extraction complete.」，属于另一簇窗口），誓约变换看起来就"没生效"。
//
// `variant`（payload[5]）只在**失败分支**被读，成功时不用。
func PrimerTransformReply(window, variant byte) []byte {
	out := make([]byte, PrimerTransformReplySize)
	out[0] = 1 // status != 0 = 成功（不读提示码）
	out[5] = window
	out[6] = variant
	return out
}
