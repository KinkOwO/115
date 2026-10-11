package protocol

import (
	"encoding/binary"
	"fmt"
	"sort"
)

// 装备库（装备图鉴）协议。三个包，全部由客户端 2.38.3.25 的二进制定案：
//
//	CMD 2264 EQUIPMENT_SET_JOURNAL_FAVORITE      C→S 32B / S→C **必须非空**（见下）
//	CMD 2265 EQUIPMENT_SET_JOURNAL_CREATE        S→C 6B  （handler 固定读 6 字节）
//	NOTI 2610 EQUIPMENT_SET_JOURNAL_CHARAC_INFO  S→C **16444B**（客户端 sub_145304380 里 mov edx/ecx,403Ch 两处）
//
// ⚠️ 三条硬约束，都有二进制证据，别改：
//  1. 2610 必须**恰好 16444** 字节。客户端用 sub_146EA0BE0(&buf,0x403C) 精确读取，
//     少一个字节会走进该函数的空写陷阱（崩客户端），不是报错。
//  2. 2264 的**成功应答体不能是空的** —— 服务端的 preparePackets 会丢掉没有正文的帧，
//     客户端于是永远等不到回执。handler 本身不读载荷，所以这里按本项目既有惯例发 1 字节 0x01。
//  3. 2610 尾部 60 B = 5 类 × 3 槽 × u32；模板 0 的行整行跳过（不是结束），份数 0 的行必须保留。
const (
	EquipmentJournalOpcode uint16 = 2610
	// EquipmentJournalBodySize 是 2610 解密后正文的固定长度。
	EquipmentJournalBodySize = 16444
	// EquipmentJournalTableSlots 是模板数量表的行数（每行 u32 模板 + u32 份数）。
	EquipmentJournalTableSlots = 2048
	// EquipmentJournalCategoryCount 是收藏类别数（源里 [set mark] 取 0..4）。
	EquipmentJournalCategoryCount = 5
	// EquipmentJournalCategorySlots 是每类在尾部占的槽位数。
	EquipmentJournalCategorySlots = 3

	// EquipmentFavoriteRequestSize 是 CMD2264 请求的正文长度（实测 8/8 样本均为 32）。
	EquipmentFavoriteRequestSize = 32

	// EquipmentJournalCreateReplySize 是 6 字节应答的长度。⚠️ 它属于 **2259**（见下文
	// EquipmentCraftOpcode 的纠错说明）；保留旧名只为不改既有调用方。
	EquipmentJournalCreateReplySize = EquipmentCraftReplySize
)

// EquipmentFavoriteRequest 是 CMD2264（收藏）的解析结果。
//
// 偏移来自实机 8 条样本（服主在 4 个栏目各收藏一次 + 若干取消）：
//
//	+0  u32  上下文 A（同一次"栏目会话"内不变，语义未定 —— 原样保留）
//	+4  u32  恒 1
//	+8  u32  上下文 B（与 A 成对变化，语义未定 —— 原样保留）
//	+12 u8   恒 1
//	+13 u32  ★ 收藏类别（0..4，低字节；高 3 字节恒 0）
//	+17 u32  槽位 0 ┐ 三个槽位，**升序**、每次发**全量**（不是增量）
//	+21 u32  槽位 1 ├ 与 2610 尾部「每类 3 槽」数量一致
//	+25 u32  槽位 2 ┘
//	+29 3B   尾部填充（32 字节装不下第 4 槽，见下）
//
// ⚠️ 槽位起点是 **+17**，不是 +16。两条互相独立的证据：
//  1. 同族的 2265 构包代码（`sub_141390D80` case 2）显式写
//     `*(_DWORD *)&v74[4 * v50 + 17] = …`（v50 = 0..11）—— 数组起点 +17、步长 4；
//     两个命令共享同一「13 字节包头 + u32@13」前缀。
//  2. 8 条实机样本按 +17 读得 16201/16203/16211/16212，全部落在源表 `[part set index]`
//     （16201..16213）内；按 +16 读得 16201<<8 之类的值，**不在任何源表里**。
//
// 早先按 +16 的版本把测试期望钉成了 0x3f4b00（= 16203<<8）—— 那是把错形状固化了，
// 已随本次修复一并改正（教训：实测样本必须能对回源表，否则测试只是在复述实现）。
type EquipmentFavoriteRequest struct {
	ContextA uint32
	ContextB uint32
	Kind     byte
	Category byte
	Slots    [3]uint32
}

// DecodeEquipmentFavoriteRequest 解析 CMD2264 正文。
func DecodeEquipmentFavoriteRequest(p []byte) (EquipmentFavoriteRequest, error) {
	var out EquipmentFavoriteRequest
	if len(p) != EquipmentFavoriteRequestSize {
		return out, fmt.Errorf("equipment favorite requires an exact %d-byte body, got %d",
			EquipmentFavoriteRequestSize, len(p))
	}
	out.ContextA = binary.LittleEndian.Uint32(p[0:4])
	out.Kind = p[12]
	out.Category = p[13]
	out.ContextB = binary.LittleEndian.Uint32(p[8:12])
	if out.Category > EquipmentJournalCategoryCount-1 {
		return out, fmt.Errorf("equipment favorite category %d is out of range", out.Category)
	}
	// Kind(+12) 实测 8/8 恒 1，但**不在解码器里硬拒**：它是未定字段，假拒绝的代价是
	// 2264 无回包 = 玩家"点了没反应"，而放行只是多一条诊断。由调用方按 Kind 决定怎么记。
	for i := 0; i < 3; i++ {
		out.Slots[i] = binary.LittleEndian.Uint32(p[17+4*i : 21+4*i])
	}
	return out, nil
}

// EquipmentFavoriteReply 是 CMD2264 的成功应答。
//
// handler（sub_145277BF0）**不读载荷**，它只按框架给的"成功位"刷新窗口 2154；
// 但正文不能为空（空正文的帧会被服务端丢弃），所以按本项目惯例发 1 字节通用成功前缀。
// 收藏的真实效果由随后下发的 2610 权威快照体现。
func EquipmentFavoriteReply() []byte { return []byte{1} }

// ---------------------------------------------------------------------------
// CMD2259：装备库「制作 / 变换」
// ---------------------------------------------------------------------------
//
// ⚠️ 编号纠错。`next113` 曾把「6 字节应答」记在 **2265** 名下 —— 那是
// `journal-handlers-cmd` 那次 dump 把 2264/2265 两个文件名**互换了**。
// `sub_1452A1F90`（装备库家族 init）的原始注册序列是：
//
//	0x1452A3B9D  mov edx, 8D8h  (2264)  → handler sub_145277C70
//	0x1452A3BB4  mov edx, 8D9h  (2265)  → handler sub_145277BF0   (127 B，**不读载荷**)
//	0x1452A3BCB  mov edx, 8D3h  (2259)  → handler sub_145277D00   (246 B，**读 6 字节**) ★
//
// 所以「读 6 字节并据此打开窗口 3937 / 2145」属于 **2259**，不是 2265。
const EquipmentCraftOpcode uint16 = 2259

const (
	// EquipmentCraftBodySize 是 2259 请求正文的**固定**长度（实机 5 帧全部 128）。
	EquipmentCraftBodySize = 128
	// EquipmentCraftHeaderSize 是正文头长度：u32 上下文 + u32 恒 0 + u32 面板 + u8 + u8 + 3 B 保留。
	EquipmentCraftHeaderSize = 17
	// EquipmentCraftEntryCount 是记录条数，等于「装备槽 12..25」这 14 个槽。
	EquipmentCraftEntryCount = 14
	// EquipmentCraftEntrySize 是单条记录长度：u24 分组 + u32 装备模板。
	EquipmentCraftEntrySize = 7
	// EquipmentCraftTrailerSize 是尾部长度（实机恒定 13 字节、以 00 01 开头，语义未定）。
	EquipmentCraftTrailerSize = 13
	// EquipmentCraftSlotBase 是记录 0 对应的装备槽号 ⇒ 记录 i ↔ 槽 (12+i)。
	//
	// 依据：`configs/equipment-wear.full-candidate.json` 的 slots 表
	// （[weapon]=12 [title name]=13 [coat]=14 [shoulder]=15 [pants]=16 [shoes]=17
	//   [waist]=18 [amulet]=19 [wrist]=20 [ring]=21 [support]=22 [magic stone]=23
	//   [support weapon]=24 [earring]=25），且实机三条填充精确落在
	// 槽 12 / 15 / 18（= 记录 0 / 3 / 6），与玩家点的巨剑 / 头肩 / 腰带逐一吻合。
	EquipmentCraftSlotBase = 12
	// EquipmentCraftEmptyTemplate 是空槽哨兵；实机空记录 = `2e 00 00 ff ff ff ff`。
	EquipmentCraftEmptyTemplate = 0xFFFFFFFF
	// EquipmentCraftPayloadSize 是 2259 应答里 handler 要读的字节数（handler 固定读 6）。
	EquipmentCraftPayloadSize = 6
	// EquipmentCraftReplySize 是 2259 应答**正文总长** = 1 字节成功前缀 + 6 字节窗口指令。
	//
	// ⚠️ 那 1 字节前缀不是可选的：type-1（cmd）回包的收包分发器会先把 body[0] 当成功标志
	// 并推进游标，再把手交给 handler。少写它 ⇒ handler 只剩 5 字节却要读 6 ⇒
	// 客户端 `sub_146EA0BE0` 执行 `MEMORY[0] = 0`（断言式崩溃，实机 0xC0000005）。
	EquipmentCraftReplySize = EquipmentCraftPayloadSize + 1
)

// EquipmentCraftEntry 是 2259 正文里的一条记录：`u24 分组 + u32 装备模板`。
//
// Group 的语义**未定案**：同栏目内不同槽（肩 15 与腰 18）同值、换栏目（装备→武器）才变
// ⇒ 更像「栏目/面板标识」而不是逐槽属性。当作不透明上下文原样收下即可，
// 真正决定行为的是 Template。
type EquipmentCraftEntry struct {
	Group    uint32
	Template uint32
}

// EquipmentCraftRequest 是 CMD2259 的解析结果（由 5 条不重复实机样本钉住）。
//
//	Panel     = u32@0  栏目/面板标识：装备栏目 = 164，武器栏目 = 197（实机）
//	Reserved  = u32@4  实测恒 0
//	Context   = u32@8  **来源窗口**（不是常量）：装备变换 = 0x46ece836，装备生成 = 0x5ff2f9
//	Action    = u8@12  动作：0 = 装备生成，1 = 装备变换
//	PayOption = u8@13  **付款方式序号**（1 起，对应 [create cost] 里 [cost] 行的顺序）
//
// ★ PayOption 曾被当成常量 1（老注释写「实测恒 1/1」）。2026-09-29 13:52 的实机帧把它推翻了：
// 同一会话两次「生成」分别是 1 与 2，玩家确认第二次点的就是另一支付法。
// 9 个套装档每档恰好两支：1 = 登记证 + 金币；2 = 登记证 + **巡礼之印**
// （`10401346` = `Pilgrimage Seal`；4/5/8/9 档没有登记证，只有金币/巡礼之印）。
// **服务端必须按它扣料** —— 忽略它就会把玩家选的「材料」那支错扣成金币
// （13:52 那笔实际就错扣了 35,000 金币）。
type EquipmentCraftRequest struct {
	Panel     uint32
	Reserved  uint32
	Context   uint32
	Action    byte
	PayOption byte
	Entries   [EquipmentCraftEntryCount]EquipmentCraftEntry
}

// Slot 返回记录 i 对应的装备槽号（记录 0 = 槽 12 `[weapon]`）。
func (r EquipmentCraftRequest) Slot(i int) uint32 {
	return EquipmentCraftSlotBase + uint32(i)
}

// Wanted 列出被点选的槽号与其装备模板（跳过空槽）。两者等长、一一对应。
func (r EquipmentCraftRequest) Wanted() (slots, templates []uint32) {
	for i, en := range r.Entries {
		if en.Template == EquipmentCraftEmptyTemplate || en.Template == 0 {
			continue
		}
		slots = append(slots, r.Slot(i))
		templates = append(templates, en.Template)
	}
	return slots, templates
}

// DecodeEquipmentCraftRequest 解析 CMD2259 正文。
//
// 布局（全部来自实机字节，见 analysis/tasks/next124 §8/§9 与 next125）：
//
//	[0:17]    头：u32 面板 / u32 恒 0 / u32 来源窗口 / u8 动作(0 生成 / 1 变换) /
//	          u8 付款方式(1 起) / 3 B 保留
//	[17:115]  14 条 7 字节记录：`u24 分组 + u32 模板`（0xFFFFFFFF = 空槽）
//	[115:128] 13 字节恒定尾
func DecodeEquipmentCraftRequest(p []byte) (EquipmentCraftRequest, error) {
	var out EquipmentCraftRequest
	if len(p) != EquipmentCraftBodySize {
		return out, fmt.Errorf("equipment craft requires an exact %d-byte body, got %d",
			EquipmentCraftBodySize, len(p))
	}
	out.Panel = binary.LittleEndian.Uint32(p[0:4])
	out.Reserved = binary.LittleEndian.Uint32(p[4:8])
	out.Context = binary.LittleEndian.Uint32(p[8:12])
	out.Action = p[12]
	out.PayOption = p[13]
	for i := 0; i < EquipmentCraftEntryCount; i++ {
		off := EquipmentCraftHeaderSize + i*EquipmentCraftEntrySize
		out.Entries[i].Group = uint32(p[off]) | uint32(p[off+1])<<8 | uint32(p[off+2])<<16
		out.Entries[i].Template = binary.LittleEndian.Uint32(p[off+3 : off+7])
	}
	return out, nil
}

// EquipmentCraftReply 是 CMD2259 的应答正文（handler 读的 **6 字节**）。
//
// ★ 语义已由 IDA 定案（`analysis/dumps/journal-craft-reply/`，2026-09-29）。
// handler `sub_145277D00(a1, success, notice)` 读 6 字节到 `v12(u32) + v13(u16)`，
// 于是 `v13` 的低字节 = payload[4]、高字节 = payload[5]：
//
//	payload[4] != 0 → 取窗口 **3937**（装备变换）→ `sub_1404AE520(win, success != 0)`
//	payload[4] == 0 → 取窗口 **2145**（装备生成）→ 再看 payload[5]：
//	    payload[5] != 0 → `sub_14150F090(win, success)`  = `win[13280] = success`（只落标志）
//	    payload[5] == 0 → `sub_14151BAE0(win, success)`  = `EquipmentTransformWindow::setState`
//	                       `(win, (success^1)+3)` ⇒ 成功 = 状态 3、失败 = 状态 4
//
// ⚠️ **两条分支都没有任何「关闭窗口」调用** —— 服务端**无法**用这个包关掉生成界面。
// 界面常驻是客户端自己的设计：变换成功那支（`sub_1404AE520`）也只是
// 「`sub_14668C520(ui, 2875, 字符串, 0)` 弹个提示 + 状态归零」，同样不关窗。
// 详见 analysis/tasks/next127 §10。
//
// `success == 0` 时 handler 还会调 `sub_146ADFC80(notice, …)` —— 那是一张**通用提示码
// 分派表**（`0x72`/`0x73` → 提示 2875；`0xE5`/`0xE6`/`0x88` → 对话框 3634…），将来要做
// 失败提示时的现成挂点。
//
// u32@0..@3 未被 handler 使用，按本仓惯例留 0。
//
// ⚠️ 正文总长是 **7**（`{1}` + 6）：见 EquipmentCraftReplySize 的说明。
func EquipmentCraftReply(window, variant byte) []byte {
	out := make([]byte, EquipmentCraftReplySize)
	out[0] = 1 // 成功前缀：收包分发器会吃掉它并推进游标
	out[5] = window
	out[6] = variant
	return out
}

// EquipmentJournalCreateReply 是 6 字节应答的**旧名**（历史误标为 2265）。
// 真实归属见 EquipmentCraftOpcode 的纠错说明 —— 它是 **2259** 的应答，
// 语义见 EquipmentCraftReply。保留只为兼容既有测试与调用方。

// 客户端 handler（sub_145277D00）固定读 6 字节：u32 保留 + u8 窗口选择 + u8 子选择。
// 两个字节决定打开哪个窗口类（3937 / 2145 两族）。当前服务端尚未实现 2265 的写侧，
// 这个编码器先把几何钉住。
func EquipmentJournalCreateReply(window, variant byte) []byte {
	out := make([]byte, EquipmentJournalCreateReplySize)
	out[4] = window
	out[5] = variant
	return out
}

// EquipmentJournalBody 组 NOTI2610 的解密正文：恰好 16444 字节。
//
// 布局：
//
//	[0, 16384)      2048 行 × (u32 模板, u32 绝对份数)；模板 0 = 空行
//	[16384, 16444)  5 类 × 3 槽 × u32 收藏分组 ID
//
// counts 里**任何非零模板**都会写进表里（哪怕份数是 0 —— "已登记但 0 份"与"从未登记"在客户端是两种状态）；
// 模板号 0 不允许作为键。favorites 每类最多取前 3 项（多出来的由调用方记诊断，见 inventory.ExtraFavoriteSlots）。
func EquipmentJournalBody(counts map[uint32]uint32, favorites map[uint32][]uint32) ([]byte, error) {
	templates := make([]uint32, 0, len(counts))
	for t := range counts {
		if t == 0 {
			return nil, fmt.Errorf("equipment journal: template 0 is not a valid table key")
		}
		templates = append(templates, t)
	}
	if len(templates) > EquipmentJournalTableSlots {
		return nil, fmt.Errorf("equipment journal: %d registered templates exceed the %d-slot table",
			len(templates), EquipmentJournalTableSlots)
	}
	sort.Slice(templates, func(i, j int) bool { return templates[i] < templates[j] })

	out := make([]byte, EquipmentJournalBodySize)
	for i, t := range templates {
		off := 8 * i
		binary.LittleEndian.PutUint32(out[off:off+4], t)
		binary.LittleEndian.PutUint32(out[off+4:off+8], counts[t])
	}
	tail := EquipmentJournalTableSlots * 8
	for category := 0; category < EquipmentJournalCategoryCount; category++ {
		slots, ok := favorites[uint32(category)]
		if !ok {
			continue
		}
		if len(slots) > EquipmentJournalCategorySlots {
			slots = slots[:EquipmentJournalCategorySlots]
		}
		for i, v := range slots {
			off := tail + (category*EquipmentJournalCategorySlots+i)*4
			binary.LittleEndian.PutUint32(out[off:off+4], v)
		}
	}
	return out, nil
}

// DecodeEquipmentJournalBody 是 EquipmentJournalBody 的反向解析（自检与测试用）。
// 它按同一套规则还原：跳过模板 0 的行，保留份数 0 的行。
func DecodeEquipmentJournalBody(body []byte) (map[uint32]uint32, map[uint32][]uint32, error) {
	if len(body) != EquipmentJournalBodySize {
		return nil, nil, fmt.Errorf("equipment journal body requires an exact %d-byte payload, got %d",
			EquipmentJournalBodySize, len(body))
	}
	counts := map[uint32]uint32{}
	for i := 0; i < EquipmentJournalTableSlots; i++ {
		off := 8 * i
		t := binary.LittleEndian.Uint32(body[off : off+4])
		if t == 0 {
			continue
		}
		if _, dup := counts[t]; dup {
			return nil, nil, fmt.Errorf("equipment journal body repeats template %d", t)
		}
		counts[t] = binary.LittleEndian.Uint32(body[off+4 : off+8])
	}
	tail := EquipmentJournalTableSlots * 8
	favorites := map[uint32][]uint32{}
	for category := 0; category < EquipmentJournalCategoryCount; category++ {
		var slots []uint32
		for i := 0; i < EquipmentJournalCategorySlots; i++ {
			off := tail + (category*EquipmentJournalCategorySlots+i)*4
			if v := binary.LittleEndian.Uint32(body[off : off+4]); v != 0 {
				slots = append(slots, v)
			}
		}
		if len(slots) > 0 {
			favorites[uint32(category)] = slots
		}
	}
	return counts, favorites, nil
}
