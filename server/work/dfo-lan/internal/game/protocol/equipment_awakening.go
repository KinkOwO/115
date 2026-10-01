package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD2258 `ENUM_CMDPACKET_EQUIPMENT_AWAKENING`（装备调适）。
//
// IDA 证据（工作副本 IDB，脚本 analysis/dumps/ida_awakening_2258*.py）：
//
//	注册点    sub_14000A060 → sub_14599D450(qword_14E66C090, 2258, sub_140B899B0, 0)
//	发送器    sub_140B8AD40：sub_146D746E0(w, 2258) → sub_146D75B10(w, buf, 25) → 发送
//	          ⇒ 请求负载**固定 25 字节**（前 13 = 信封，后 12 = 字段）
//	发包点①   sub_141491BE0：+13 = 1、+14..17 = 0xFFFFFFFF、+18 = [a1+56]、+19..20 = [a1+60]、
//	          +21..24 = 0xFFFFFFFF（初始化 / 返还路径）
//	发包点②   sub_141491D10：+13 = 0、+14..17 = [a1+996]、+18 = [a1+56]、+19..20 = [a1+60]、
//	          +21..24 = 目标模板（正常调适路径）
//	应答处理  sub_140B899B0(handler, 状态, 错误码)：
//	          状态 != 0 → sub_141491360 → sub_1414921F0(窗口 3580, 3)（**只刷新面板**）
//	          状态 == 0 → 按 u16 错误码弹提示 → sub_1414921F0(窗口 3580, 4)（刷新 + 复位）
//	          ⇒ **状态 1 = 成功**（与 CMD2259 的成功前缀同一约定），错误码只在状态 0 时被读
//
// ⚠️ 字段相对**请求体**的偏移：`+13` = 模式、`+14..17` = 材料组、`+18` = 装备空间、
// `+19..20` = 装备槽位、`+21..24` = 目标装备模板。这里的"请求体"指客户端 writer 里
// 那 25 字节负载；服务端 `wire` 给到 handler 的 `p` 与它同口径（既有 5 个实机验证的
// 协议模块同样从 `p[13:]` 读字段，见 analysis/dumps/CLIENT-MECHANICS.md §5）。
//
// 保留一条**带信封**的兼容读法：若 `p` 长度 ≥ 38 且首字节为 1、次两字节正是本 opcode，
// 说明这一帧把 13 字节信封一起带进来了，字段整体后移 13 字节。这是结构自校验，不是猜包。
const (
	EquipmentAwakeningOpcode uint16 = 2258

	// EquipmentAwakeningPayloadSize 是客户端固定发送的负载长度（sub_140B8AD40 的 25）。
	EquipmentAwakeningPayloadSize = 25

	equipmentAwakeningFields  = 13 // 字段区起点（相对负载）
	equipmentAwakeningMinBody = 25 // 负载最短长度（信封 13 + 字段 12）
	equipmentAwakeningEnvelope = 13
)

// EquipmentAwakeningRequest 是 CMD2258 的解析结果。
type EquipmentAwakeningRequest struct {
	// Mode：0 = 调适，1 = 初始化 / 返还（源里 [refund materials]）。
	Mode byte
	// MaterialGroup：源 `[need materials]` 的 `[group] N`（本版本 1 / 2）。
	MaterialGroup uint32
	// Space：0 = 背包，3 = 穿戴栏（与强化同一套空间编号）。
	Space byte
	// Slot：装备在 Space 里的槽位。
	Slot uint16
	// Target：目标装备模板。阶段 < 上限时原生发包点写 0xFFFFFFFF（保持原模板）。
	Target uint32
	// PayloadOffset 记录字段区的实际起点（13 = 同口径，26 = 带信封帧），供日志定位。
	PayloadOffset int
}

// NoTarget 是"本阶段不换模板"的哨兵（客户端在 +21..24 写 0xFFFFFFFF）。
const EquipmentAwakeningNoTarget uint32 = 0xFFFFFFFF

// DecodeEquipmentAwakening 解析 CMD2258 请求体。
func DecodeEquipmentAwakening(p []byte) (EquipmentAwakeningRequest, error) {
	var r EquipmentAwakeningRequest
	if len(p) < equipmentAwakeningMinBody {
		return r, fmt.Errorf("装备调适请求长度不足：%d < %d", len(p), equipmentAwakeningMinBody)
	}
	offset := equipmentAwakeningFields
	// 带信封的帧：`01 | opcode(u16 LE) | 8×0 | 2×0` 打头。
	if len(p) >= equipmentAwakeningMinBody+equipmentAwakeningEnvelope &&
		p[0] == 1 && binary.LittleEndian.Uint16(p[1:3]) == EquipmentAwakeningOpcode {
		offset += equipmentAwakeningEnvelope
	}
	if len(p) < offset+12 {
		return r, fmt.Errorf("装备调适请求长度不足：%d 字节装不下字段区（偏移 %d）", len(p), offset)
	}
	r.PayloadOffset = offset
	r.Mode = p[offset]
	r.MaterialGroup = binary.LittleEndian.Uint32(p[offset+1 : offset+5])
	r.Space = p[offset+5]
	r.Slot = binary.LittleEndian.Uint16(p[offset+6 : offset+8])
	r.Target = binary.LittleEndian.Uint32(p[offset+8 : offset+12])
	if r.Mode > 1 {
		return r, fmt.Errorf("装备调适模式 %d 不受支持", r.Mode)
	}
	if r.Space != 0 && r.Space != 3 {
		return r, fmt.Errorf("装备调适只支持背包(0)或穿戴栏(3)，收到空间 %d", r.Space)
	}
	return r, nil
}

// EquipmentAwakeningReply 构造 CMD2258 应答体。
//
// 客户端 `sub_140B899B0(handler, 状态, 错误码)` 的语义（IDA 第五轮 + 实机 2026-10-01 22:46）：
//
//	状态 != 0 → sub_141491360 → sub_1414921F0(window, 3)
//	状态 == 0 → 按 u16 码弹提示 → sub_1414921F0(window, 4)
//
// 而 `sub_1414921F0(window, state)` 的两支：
//
//	state 3 → 只调 sub_1414928D0(window)：**重查材料并刷新面板**（按钮可用性/材料行）
//	state 4 → 调 sub_1414928D0 后把 window+1512 清 0，再 sub_141494520(window, 0) 复位
//
// ⇒ **状态 1（非 0）= 成功**、状态 0 = 失败。这与姊妹协议 CMD2259 的
// `EquipmentCraftReply`（`out[0] = 1 // 成功前缀`，已实机验证）同一约定。
//
// 实机教训（2026-10-01 22:46，角色 11 的 100261128 从 0→1→2→3 三次均成功）：
// 状态发 0 时服务端数据是对的（装备行 +170 已涨），但客户端走的是"刷新 + 复位"，
// 玩家看到的是「调适面板本身不刷新、也没有成功提示」。
//
// 状态非 0 时客户端**不读**错误码；状态为 0 时读的 u16 码决定弹哪条文案
// （1/3/119/217 各有原生物品提示，其余走 sub_146ADFC80 的消息表，0 = 无文案）。
func EquipmentAwakeningReply(status byte, code uint16) []byte {
	return []byte{status, byte(code), byte(code >> 8)}
}

// EquipmentAwakeningSuccess 是成功应答：状态 1（触发客户端刷新调适面板）。
func EquipmentAwakeningSuccess() []byte { return EquipmentAwakeningReply(1, 0) }

// EquipmentAwakeningFailure 是失败应答：状态 0（客户端按码弹提示并复位面板）。
func EquipmentAwakeningFailure() []byte { return EquipmentAwakeningReply(0, 0) }
