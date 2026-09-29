package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD 1722 = 装备继承（itemCarving / Inherit）确认。
//
// 客户端发包点（IDA，DFO.exe 115 级权威 IDB）：
//
//	sub_14138D120  继承窗口的确认处理：case 2 先过 sub_14138A210 的校验，
//	               再 sub_146D746E0(w, 1722) + sub_146D75B10(w, a1+1512, 466)
//	               + sub_146D75AF0() 发送。
//
// ⇒ 请求体是窗口对象 +1512 处的一块 466 字节结构体。
//
// 布局（由 sub_14138A210 的写字段逐条坐实）：
//
//	u8  @0..12   前缀，实机全 0
//	u8  @13      状态字节：sub_14138A210 入口写 4、清完记录后写 0；失败路径再写回 4。
//	             实机明文恒为 0（因为只有校验通过才会走到 case 2 发包）。
//	记录 @14     共 14 条 × 32 字节 = 448 字节（sub_14138A210 里 n14 = 14 的清零循环，
//	             步长 32，起点 a1+1526 = 体偏移 14）
//	u32 @462..465 尾部 u32（a1+1974）。sub_14138A210 只把它清 0，从不写入条数
//	             ⇒ 实机恒 0，**不能当记录数用**。
//
// 14 + 14×32 + 4 = 466，与发包长度逐位吻合。
// 实机明文是 480 字节：466 字节请求体 + 14 字节分组补位（补到 16 的倍数）。
const (
	InheritBodySize      = 466
	InheritPrefixSize    = 14
	InheritRecordSize    = 32
	InheritRecordCount   = 14
	InheritTrailerOffset = InheritPrefixSize + InheritRecordSize*InheritRecordCount // 462
	// InheritEmptySpace 是未使用记录里 +30/+31 的哨兵值（'.'，0x2e）。
	// sub_1471B45D0 只做了部分清零，所以这两个字节有残留。
	InheritEmptySpace = 0x2e
)

// InheritEntry 是请求体里的一条继承记录（32 字节）。
//
// ⚠️ A / B **只是窗口的两个 UI 槽，没有「源 / 目标」语义** —— 玩家两件装备怎么拖都行。
// 谁是材料件（消耗）、谁是基础件（保留并接收等级）由服务层按等级判定，
// 见 internal/inventory/inherit.go。这里只负责把两个槽如实解出来。
//
// 字段偏移全部来自 sub_14138A210：v18 = a1+1526 是记录起点，
// A 侧装备对象 v21 = *(entry+136)、B 侧装备对象 v22 = *(entry+8)。
//
//	记录内 +0   (体 +14)  u16  A 槽位      *(_WORD *)v18       = v24
//	记录内 +2   (体 +16)  u32  A 模板号    *(_DWORD *)(v18+2)  = *(vtable[152](v21)+24)
//	记录内 +18  (体 +32)  u16  B 槽位      *(_WORD *)(v18+18)  = v26
//	记录内 +20  (体 +34)  u32  B 模板号    *(_DWORD *)(v18+20) = *(vtable[152](v22)+24)
//	记录内 +24  (体 +38)  u16  常量 257    *(_WORD *)(v18+24)  = 257（硬编码）
//	记录内 +26  (体 +40)  u32  计数器      *(_DWORD *)(v18+26) = v38（实机恒 0）
//	记录内 +30  (体 +44)  u8   A 容器      *(_BYTE *)(v18+30)  = v43
//	记录内 +31  (体 +45)  u8   B 容器      *(_BYTE *)(v18+31)  = n3
//
// ★ `+2` 是**模板号，不是实例 UID**（2026-09-28 逐行对账坐实）：20:38 那场 2×2 交叉里
// 解出的 101040829 / 101040830 / 101040833 / 101021079 与同会话 NOTI13 背包快照里
// 槽 9 / 10 / 11 / 12 的模板号**四组全中**，且旧会话的槽 57 = 101021079 也对上。
// 若是实例 UID，不可能与存档模板号系统性一致。
// 这条很关键：能按模板号核对身份，服务端就可以安全地做跨容器回退查找，
// 不必像「只知道槽位」那样担心拿错装备。
//
// 容器取值与项目既有约定一致：0 = 背包，3 = 已穿戴（见 inventory.go 的容器表）。
// 槽位查询失败时客户端写 3 再回退查询（sub_145AD5C20 < 0 → sub_145AD5F80），
// 所以请求里报的容器不一定准 —— 但模板号一定准。
type InheritEntry struct {
	SlotA     uint16
	TemplateA uint32
	SlotB     uint16
	TemplateB uint32
	// Const 是客户端硬编码的 257 (0x0101)。实机 7 个样本无一例外。
	Const uint16
	// Counter 是记录内 +26 的 u32，实机恒 0（不是材料模板，语义未取证）。
	Counter uint32
	SpaceA  byte
	SpaceB  byte
}

// InheritItem 是请求里被选中的一件装备（由某条记录的 A 侧或 B 侧解出）。
type InheritItem struct {
	Slot     uint16
	Template uint32
	// Space 是客户端声明的容器：0 = 背包、3 = 已穿戴。
	Space byte
	// Kind 标记它来自记录的哪一侧：'A' 或 'B'（仅用于日志与排查）。
	Kind byte
}

// InheritRequest 是 CMD 1722 的请求。
//
// 只保留模板号非 0 的条目：尾部 u32 恒 0 不能当记录数，而未使用的记录里
// 容器字段有 0x2E 残留，按「模板号 == 0 即未使用」过滤是唯一可靠的判据。
type InheritRequest struct {
	Entries []InheritEntry
	// Items 是展平后的选中装备，正常情况恰好 2 件。
	Items []InheritItem
}

// DecodeInheritRequest 解 CMD 1722 的明文。
//
// 明文长度是 480（466 + 补位），所以只要求不少于 InheritBodySize，尾部补位忽略。
func DecodeInheritRequest(p []byte) (InheritRequest, error) {
	var r InheritRequest
	if len(p) < InheritBodySize {
		return r, fmt.Errorf("继承请求长度不足: %d < %d", len(p), InheritBodySize)
	}
	for k := 0; k < InheritRecordCount; k++ {
		base := InheritPrefixSize + k*InheritRecordSize
		rec := p[base : base+InheritRecordSize]
		// 未使用记录：两侧容器都是 0x2e 哨兵（sub_1471B45D0 的部分清零残留）。
		if rec[30] == InheritEmptySpace && rec[31] == InheritEmptySpace {
			continue
		}
		e := InheritEntry{
			SlotA:     binary.LittleEndian.Uint16(rec[0:]),
			TemplateA: binary.LittleEndian.Uint32(rec[2:]),
			SlotB:     binary.LittleEndian.Uint16(rec[18:]),
			TemplateB: binary.LittleEndian.Uint32(rec[20:]),
			Const:     binary.LittleEndian.Uint16(rec[24:]),
			Counter:   binary.LittleEndian.Uint32(rec[26:]),
			SpaceA:    rec[30],
			SpaceB:    rec[31],
		}
		if e.TemplateA == 0 && e.TemplateB == 0 {
			continue
		}
		r.Entries = append(r.Entries, e)
		if e.TemplateA != 0 {
			r.Items = append(r.Items, InheritItem{Slot: e.SlotA, Template: e.TemplateA, Space: e.SpaceA, Kind: 'A'})
		}
		if e.TemplateB != 0 {
			r.Items = append(r.Items, InheritItem{Slot: e.SlotB, Template: e.TemplateB, Space: e.SpaceB, Kind: 'B'})
		}
	}
	return r, nil
}

// ★★ CMD 1722 在客户端**双向都没有继承结果的接收通道** —— 服务端唯一正确的动作
// 就是「落库 + 发 kind=0 的 id14 槽位行刷新」，**任何方向、任何形态的 1722 出站包
// 都禁止**。
//
// 原因：客户端 opcode 表是两套独立命名空间，`1722` 这个编号在两个方向上的含义都与
// 继承结果无关：
//
//   - `kind=1` → CMD 表（`qword_14E6836F8`）：1722 是客户端自己发出去的继承确认命令，
//     没有接收 handler。服务端回 `kind=1 + 1722` 会被当成「自己刚发的继承命令」解析，
//     命令体 466 字节而负载只有几字节 ⇒ 格式不符、无对应消费路径。
//
//   - `kind=0` → NOTI 表（`qword_14E683700`）：1722 的 handler 是 `sub_143348A90`
//     （读 8 字节：i32 v1 + u32 v2），会弹**小游戏道具使用计数通知**
//     （`0xA88` 是小游戏弹窗参数、`v2` 是弹窗计数），与继承结果毫无关系。
//
// ⇒ 继承完成 / 被拒都**不回任何 1722 包**（成功也不回、失败也不回）。客户端窗口的
// 数值更新由 id14 槽位行刷新驱动（与第三方已验证实现、强化 / 增幅路径同一套路）。
// 若未来观察到「窗口需要额外收尾包」，必须先有新的 IDA / 抓包证据，禁止再拿 1722
// 的任何一个方向当回包通道。
