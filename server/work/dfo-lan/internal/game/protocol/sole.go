package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD2288 `ENUM_CMDPACKET_SOLE_EQUIPMENT_QUALITY`（秘宝精度提升）。
//
// 协议来源：业主提供的 AI 交接书 + 2026-09-30 三个会话共 10 帧实机抓包
// （见 analysis/tasks/next150-秘宝精度提升-直读PVF实现.md）。
//
// 请求负载固定 **24 字节**（含 13 字节信封）：
//
//	[0..12]  13 字节信封（`50 02` + 6×00 + 会话令牌 + `01`）—— 令牌每局都变，
//	         不是语义，必须整段跳过（与 CMD2258/2259 同口径，见 CLIENT-MECHANICS §5）
//	[13]     container  u8   秘宝容器（`03` = 已穿戴）
//	[14..15] slot       u16  秘宝槽位（小端；交接书实机判定：22 = Diregie、23 = Venus、25 = Nabel）
//	[16..19] selector   u32  配方 / 材料组选择（实机恒 0）
//	[20..23] u8[4]           补零
//
// 应答（kind 1，见 CLIENT-MECHANICS §4）：分派器先吃掉体首的 **u8 状态字节**，
// handler 从 body+1 读自己的负载 ⇒ 回显客户端自己报的定位字段：
//
//	body = 01 | container(u8) | slot(u16 LE)
//
// ⚠️ 客户端这一侧的 2288 S2C handler 没有反编译体（opcodes.tsv 只给了注册槽位），
// 所以这是「与 CMD272 EnchantByBeadReply 同形的最小回显」，**待实机确认**；
// 唯一硬约束是 §2 的期望回包树：请求登记了 2288，服务端不回同 id 包则窗口卡在等待态。
const (
	SoleQualityOpcode uint16 = 2288

	// SoleQualityPayloadSize 是客户端固定发送的负载长度（实测 24 字节，含 13 字节信封）。
	SoleQualityPayloadSize = 24

	soleQualityFields  = 13 // 字段区起点（相对负载）
	soleQualityMinBody = 24 // 负载最短长度（信封 13 + 字段 11）
	soleQualityEnvelope = 13
)

// SoleQualityRequest 是 CMD2288 的解析结果。
type SoleQualityRequest struct {
	// Container：秘宝容器（3 = 已穿戴；与强化/调适同一套空间编号）。
	Container byte
	// Slot：秘宝在 Container 里的槽位。
	Slot uint16
	// Selector：配方 / 材料组选择（实机恒 0，保留原值供日志与后续取证）。
	Selector uint32
	// PayloadOffset 记录字段区的实际起点（13 = 同口径，26 = 带信封帧），供日志定位。
	PayloadOffset int
}

// DecodeSoleQuality 解析 CMD2288 请求体。
func DecodeSoleQuality(p []byte) (SoleQualityRequest, error) {
	var r SoleQualityRequest
	if len(p) < soleQualityMinBody {
		return r, fmt.Errorf("秘宝精度请求长度不足：%d < %d", len(p), soleQualityMinBody)
	}
	offset := soleQualityFields
	// 带信封的帧：`01 | opcode(u16 LE) | 8×0 | 2×0` 打头（与 2258 同一结构自校验）。
	if len(p) >= soleQualityMinBody+soleQualityEnvelope &&
		p[0] == 1 && binary.LittleEndian.Uint16(p[1:3]) == SoleQualityOpcode {
		offset += soleQualityEnvelope
	}
	if len(p) < offset+11 {
		return r, fmt.Errorf("秘宝精度请求长度不足：%d 字节装不下字段区（偏移 %d）", len(p), offset)
	}
	r.PayloadOffset = offset
	r.Container = p[offset]
	r.Slot = binary.LittleEndian.Uint16(p[offset+1 : offset+3])
	r.Selector = binary.LittleEndian.Uint32(p[offset+3 : offset+7])
	if r.Container != 0 && r.Container != 3 {
		return r, fmt.Errorf("秘宝精度只支持背包(0)或穿戴栏(3)，收到容器 %d", r.Container)
	}
	return r, nil
}

// SoleMaterialGroupForSelector 把请求里的 selector 映射成源 `[quality need materials]` 的组号。
//
// **与面板显示一一对应**（2026-10-02 业主受控实验判定）：
//
//	selector = 0 → 组 0：面板默认显示「巡礼之印」这类实物，第三项扣 10401346 ×800
//	selector = 1 → 组 1：切到金币后，第三项扣 4,000,000 金币
//
// 也就是说：**玩家在面板上看到什么，服务端就该扣什么**。
//
// ⚠️ 这里踩过一次坑，记录下来免得重犯：最初按"日志里 selector=1 的那几次扣的是实物"反推成
// `1 → 组 0`，那是**把服务端自己的旧行为当成了事实标准** —— 于是面板显示实物时服务端却扣金币，
// 玩家看到的现象就是「材料没扣（金币反而在掉）」。判定这种映射**只能靠"面板显示 ↔ 请求字段"
// 的受控对照**（打开窗口不切换点一次、切换后再点一次），不能靠扣料结果反推，因为扣料结果
// 本身就被旧映射污染了。
func SoleMaterialGroupForSelector(selector uint32) (int, bool) {
	switch selector {
	case 0:
		return 0, true
	case 1:
		return 1, true
	default:
		return 0, false
	}
}

// SoleQualityReply 构造 CMD2288 应答体（kind 1）：`u8 1` + 容器 + 槽位。
//
// 为什么必须发：客户端的「期望回包树」以 opcode 为键，handler 收到包先把它清掉，
// **清不掉就整个 handler 跳过** ⇒ 不回同 id 的包，精度窗口会一直卡在等待态
// （交接书症状 1：提升一次后窗口无响应、需关窗重开）。
func SoleQualityReply(container byte, slot uint16) []byte {
	p := []byte{1, container}
	return binary.LittleEndian.AppendUint16(p, slot)
}
