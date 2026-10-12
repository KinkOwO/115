// Package protocol 里本文件是活动 685「奥拉蔻的装备模块研究」面板的 S→C 帧编码。
//
// ## 载荷几何来自 IDA（L0），字段语义未证
//
// 反编译（`analysis/tasks/next201-…md §9`，探针 `ida_probe_685_notis.py`）：
//
//	2806 SIMULATION_RESEARCH_EVENT_ETC             sub_1404243D0
//	     sub_146EA0BE0(v4, 21) → qmemcpy 到 obj+1240，随后刷新**窗口 3956**
//	2809 SIMULATION_RESEARCH_EVENT_CREATION_BOARD  sub_1404242F0
//	     handler 先按「10 格 × 5 字节」把 `02 00 00 00 00` 铺好，再整块读 50 字节
//
// ⚠️ 2807 `…_INVEN_ALL` 与 2808 `…_INVEN_ITEM` 是**变长**（先读一个 u32 = 条数，再逐条），
// 本轮不实现 —— 它们要的是"玩家背包里的模块清单"，而模块从哪来还没闭环。
//
// ## 2809 的"默认值"为什么可以直接照抄
//
// 那个 `02 00 00 00 00` × 10 是**客户端 handler 自己铺的默认**（读到正文之前的状态）。
// 我们送同样的形状，就是"空格子"的中性态 —— 比自造一个不知道对不对的值安全。
package protocol

import "encoding/binary"

const (
	// SimulationResearchEtcBytes 是 NOTI2806 的载荷长度（handler 里的读流长度）。
	SimulationResearchEtcBytes = 21
	// SimulationResearchCreationBoardBytes 是 NOTI2809 的载荷长度（10 格 × 5 字节）。
	SimulationResearchCreationBoardBytes = 50
	// simulationCreationBoardCells 是 `[combine]` 的格子数（50 / 5）。
	simulationCreationBoardCells = 10
)

// SimulationResearchCreationBoardEmpty115 编 NOTI2809 的**空格子**载荷。
//
// 每个格子 5 字节：1 字节 tag + 4 字节小端值；tag=2 是客户端 handler 自己铺的默认
// （见文件头注），值 0 = 空。⇒ 10 格全空，与"还没放任何模块"一致。
func SimulationResearchCreationBoardEmpty115() []byte {
	out := make([]byte, SimulationResearchCreationBoardBytes)
	for i := 0; i < simulationCreationBoardCells; i++ {
		out[i*5] = 2
		binary.LittleEndian.PutUint32(out[i*5+1:i*5+5], 0)
	}
	return out
}

// SimulationResearchEtc115 编 NOTI2806 的载荷（21 字节，整块下发给 obj+1240）。
//
// ⚠️ **字段语义未证**（只知道长度与落点）。所以本函数只负责"按给定字节编"，取值策略
// （含 `DFO_ORACLE_ETC` 覆盖）在 `cmd/wireprobe/simresearch_flow.go` —— 与本仓对未证字段的
// 既定做法一致：做成启动期开关让业主实机 A/B，而不是在代码里赌一种解释。
func SimulationResearchEtc115(raw []byte) []byte {
	out := make([]byte, SimulationResearchEtcBytes)
	copy(out, raw)
	return out
}

// ---- 三条动作命令的 **S→C 应答** ----
//
// 长度取自 cmd handler 的"服务端方向"分支（`a2 != 0`，探针 `ida_probe_685_cmd.py`）：
//
//	2412 PRODUCTION           sub_146EA0BE0(&v11, 10)   10 B；取第 2 个 u32 喂 sub_140613BE0(窗口 3956, 值)
//	2413 COMBINE              sub_146EA0BE0(&v12, 4)     4 B
//	2414 DISJOINT_STACKABLE   sub_146EA0BE0(&v19, 12)   12 B（另清 obj+2192 链）
//
// ★ 这三个 handler 的公共头都会 `*(obj+2114) = 0`（清"进行中"标志）、并在结尾刷新**窗口 3956**
// ⇒ **服务端不回，客户端的按钮就一直停在在途状态**（实机 2026-10-12：「点了没反应」）。
const (
	// SimulationResearchProductionReplyBytes 是 CMD2412 应答的载荷长度。
	SimulationResearchProductionReplyBytes = 10
	// SimulationResearchCombineReplyBytes 是 CMD2413 应答的载荷长度。
	SimulationResearchCombineReplyBytes = 4
	// SimulationResearchDisjointReplyBytes 是 CMD2414 应答的载荷长度。
	SimulationResearchDisjointReplyBytes = 12
)

// SimulationResearchProductionReply115 编 CMD2412 的应答（10 B）。
func SimulationResearchProductionReply115(raw []byte) []byte {
	out := make([]byte, SimulationResearchProductionReplyBytes)
	copy(out, raw)
	return out
}

// SimulationResearchCombineReply115 编 CMD2413 的应答（4 B）。
func SimulationResearchCombineReply115(raw []byte) []byte {
	out := make([]byte, SimulationResearchCombineReplyBytes)
	copy(out, raw)
	return out
}

// SimulationResearchDisjointReply115 编 CMD2414 的应答（12 B）。
func SimulationResearchDisjointReply115(raw []byte) []byte {
	out := make([]byte, SimulationResearchDisjointReplyBytes)
	copy(out, raw)
	return out
}

// ---- 模块清单两帧（2807 / 2808）----
//
// 布局由**四个读流助手的实测宽度**反推（探针 `ida_probe_readers.py`）：
//
//	sub_146EA0BA0 = 4 B（u32）   sub_146EA0BE0 = N B（整块）
//	sub_146EA09F0 = 1 B（u8）    sub_146EA1920 = 2 B（u16）
//
// 对照 handler 的读序：
//
//	2807 INVEN_ALL  sub_140424460：u16 条数 + 每条 (u32 id, u8 flag, u16 值)      = 7 B/条
//	2808 INVEN_ITEM sub_140424650：u16 条数 + 每条 (u16 槽位, u32 id, u8 flag, u16 值) = 9 B/条
//
// ★ 两条的分工也由此确定：2807 报**整份清单**（只给 id，客户端自己用 sub_140425090 映射槽位）；
// 2808 报**单件变更**（带槽位）。handler 结尾都会刷新**窗口 3956**。
//
// ⚠️ `flag` 与 `值` 的语义**未证**（只知道宽度）。`值` 从形态上看最可能是堆叠数量，
// 所以这里按"数量"填；`flag` 先恒 1（"这一格有东西"），并留调用方覆盖的口子。

// SimulationResearchModuleEntry115 是 2807/2808 里的一条模块记录。
type SimulationResearchModuleEntry115 struct {
	// Slot 只被 2808 用（2807 不带槽位）。
	Slot uint16
	// ID 是模块物品号。
	ID uint32
	// Flag / Value 语义未证（宽度确定：u8 / u16）。
	Flag  byte
	Value uint16
}

// SimulationResearchInvenAll115 编 NOTI2807 的载荷（整份清单：u16 条数 + 每条 7 B）。
func SimulationResearchInvenAll115(entries []SimulationResearchModuleEntry115) []byte {
	out := make([]byte, 2, 2+7*len(entries))
	binary.LittleEndian.PutUint16(out[0:2], uint16(len(entries)))
	for _, e := range entries {
		var b [7]byte
		binary.LittleEndian.PutUint32(b[0:4], e.ID)
		b[4] = e.Flag
		binary.LittleEndian.PutUint16(b[5:7], e.Value)
		out = append(out, b[:]...)
	}
	return out
}

// SimulationResearchInvenItem115 编 NOTI2808 的载荷（单件明细：u16 条数 + 每条 9 B）。
func SimulationResearchInvenItem115(entries []SimulationResearchModuleEntry115) []byte {
	out := make([]byte, 2, 2+9*len(entries))
	binary.LittleEndian.PutUint16(out[0:2], uint16(len(entries)))
	for _, e := range entries {
		var b [9]byte
		binary.LittleEndian.PutUint16(b[0:2], e.Slot)
		binary.LittleEndian.PutUint32(b[2:6], e.ID)
		b[6] = e.Flag
		binary.LittleEndian.PutUint16(b[7:9], e.Value)
		out = append(out, b[:]...)
	}
	return out
}
