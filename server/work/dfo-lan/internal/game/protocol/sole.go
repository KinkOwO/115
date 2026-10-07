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

	soleQualityFields   = 13 // 字段区起点（相对负载）
	soleQualityMinBody  = 24 // 负载最短长度（信封 13 + 字段 11）
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

// CMD2289 `ENUM_CMDPACKET_SOLE_EQUIPMENT_CREATE`（秘宝制作：把半成品做成成品）。
//
// 请求负载固定 **24 字节**（含 13 字节信封），字段区从 **偏移 13** 起：
//
//	[0..12]  13 字节信封（以 `ff ff ff ff` 收尾的会话前导）—— 逐局可变、无语义，整段跳过
//	[13..16] 模板     u32 LE  要制作的成品模板（= 源 `[item index]`）
//	[17..20] selector u32 LE  制作组选择（实测恒 0）
//	[21..23] u8[3]            补零
//
// ⚠️ 偏移是 **13，不是 12**：按 12 读会把信封尾的 `ff` 当成模板首字节，拿到 `0xFF8548FB`
// 这类垃圾值 ⇒ 查不到源 ⇒ 制作恒被拒。判据是"哪个偏移读出的 u32 恰好等于源里声明的
// `[item index]`"：实机两条帧在偏移 13 得到 `100354181`(Venus) / `100391142`(Nabel)，
// 偏移 12 两个都是垃圾值。同族 2258/2288 用的是同一判据。
//
// 应答（kind 1）：分派器先吃掉体首的 u8 状态字节，handler 从 body+1 读负载 ⇒
// `body = 01 | u32 模板`。硬约束只有"必须回同 id 包"（客户端期望回包树以 opcode 为键，
// 收不到就清不掉等待态 ⇒ 制作窗口卡死）。
const (
	SoleCreateOpcode uint16 = 2289

	// SoleCreatePayloadSize 是客户端固定发送的负载长度（实测 24 字节，含 13 字节信封）。
	SoleCreatePayloadSize = 24

	soleCreateFields   = 13 // 字段区起点（相对负载）
	soleCreateMinBody  = 17 // 负载最短长度（信封 13 + 模板 4）
	soleCreateEnvelope = 13
)

// SoleCreateRequest 是 CMD2289 的解析结果。
type SoleCreateRequest struct {
	// Template：要制作的成品秘宝模板（源 `[item index]`）。
	Template uint32
	// Selector：制作组选择（实测恒 0；与精度提升同一口径：面板显示什么就扣什么）。
	Selector uint32
	// PayloadOffset 记录字段区的实际起点（13 = 同口径，26 = 带信封帧），供日志定位。
	PayloadOffset int
}

// DecodeSoleCreate 解析 CMD2289 请求体。
func DecodeSoleCreate(p []byte) (SoleCreateRequest, error) {
	var r SoleCreateRequest
	if len(p) < soleCreateMinBody {
		return r, fmt.Errorf("秘宝制作请求长度不足：%d < %d", len(p), soleCreateMinBody)
	}
	offset := soleCreateFields
	// 带信封的帧：`01 | opcode(u16 LE) == 2289` 打头（与 2258/2288 同一结构自校验）。
	if len(p) >= soleCreateMinBody+soleCreateEnvelope &&
		p[0] == 1 && binary.LittleEndian.Uint16(p[1:3]) == SoleCreateOpcode {
		offset += soleCreateEnvelope
	}
	if len(p) < offset+4 {
		return r, fmt.Errorf("秘宝制作请求长度不足：%d 字节装不下模板（偏移 %d）", len(p), offset)
	}
	r.PayloadOffset = offset
	r.Template = binary.LittleEndian.Uint32(p[offset : offset+4])
	if r.Template == 0 || r.Template == 0xFFFFFFFF {
		return r, fmt.Errorf("秘宝制作请求没有有效的成品模板（0x%08X）", r.Template)
	}
	// selector 只在长度够时才读（短的合法帧不含它），缺省 0。
	if len(p) >= offset+8 {
		r.Selector = binary.LittleEndian.Uint32(p[offset+4 : offset+8])
	}
	return r, nil
}

// SoleCreateReply 构造 CMD2289 应答体（kind 1）：`u8 1` + 成品模板 u32 LE。
//
// 回模板的理由：2288 回显的是它的定位字段（容器 + 槽位），制作的定位字段就是**成品模板**
// （请求里带的就是它），所以按最小回显走。
//
// ⚠️ 这一形状是**按 2288 推导的，没有反编译依据**。唯一硬约束是"必须发同 id 包"。
// 若实机出现"回包到了但制作窗口仍卡/不刷新"，按 2288 的三种候选形状逐一试
// （纯 `01` / `01 + u32` / `01 + u16 错误码`），每次只改这一个函数。
func SoleCreateReply(template uint32) []byte {
	p := []byte{1}
	return binary.LittleEndian.AppendUint32(p, template)
}
