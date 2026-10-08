package main

// noti 2836 ENUM_NOTIPACKET_OMEN_OF_ORDER_PARTY_INFO：征兆（omen）的队伍状态。
//
// 这是征兆 UI 的唯一数据源。取证见
// docs/protocol/endkeeper-of-order-primer-20260926.md §34：
//
//	处理器 sub_140656A80（注册于 sub_140657920：`lea r8,<handler>` 在前、`mov edx,0B14h` 在后）
//	第一件事就是 sub_146EA0BE0(&buf, 69) —— **恰好 69 字节**，与旧记录一致。
//
// 载荷几何（由 69 这个长度反推、并被读到它的代码逐条证实）：
//
//	4 条记录 × 17 字节 + 1 个尾标志字节 = 68 + 1 = 69
//	每条记录 = 4 × u32 + 1 × u8
//
//	处理器把第 i 条记录写进单例 qword_14E6388B8 的第 i 个座位槽：
//
//	  记录 u32[0..3]   -> 槽 +0/+4/+8/+12           （sub_1406590A0）
//	  记录 byte[16]    -> 槽 +16                    （sub_140656A80 第三段循环）
//	  记录 byte[16]    -> 单例 +192（仅本人座位）
//	  记录 u32[0..3]   -> 单例 +176..191（仅本人座位，sub_140658D80）= 征兆 ID 数组
//	  尾标志字节 == 1  -> 先用「旧状态」刷一遍可视化再套用载荷（否则直接套用）
//
// 这些槽的位置与内置函数精确对上（槽基址 +96、步长 20）：
//
//	+16 的 u8       -> getEOOPartyOmenState(seat)      （2136，sub_1406578F0）
//	+0/+4/+8/+12    -> 前导非零 u32 的个数 = 持有数    （sub_1406578B0）
//	+176 + 4*seat   -> 征兆 ID 数组（sub_140658D80 把"本人那条记录"的 4 个 u32 拷进来）
//
// ⚠️ **档位不是 u32 的值**。脚本里的 grade（1..4）是「该座位槽里**非零 u32 的个数**」
// —— 也就是该座位**持有几个征兆**。这一条是实机否定出来的：本通道最初给每条记录写了
// 4 个非零 u32，客户端当场显示 4 档（四颗石头全出），而 u32 的值只是 1。
// 所以 record 的语义是：[4 × 征兆ID][状态字节]，ID 会在 sub_1406590A0 / sub_140658D80 里
// 拿去 sub_140283D60(qword_14E683B38, id, 1) 查表（奖励预览）。详见 docs §36。
//
// 脚本侧的用法（PVF 实证）坐实了值域：
//
//	effect_eoo/action/basic.act： getEOOPartyOmenGrade(t:seatIndex()) == 1..4
//	                            -> 播 omen_effect_1_start .. omen_effect_4_start
//	omen_drop_1/action/basic.act：state == 1 且 grade >= 1 -> 给该座位出征兆石
//	（omen_drop/omen_1..4 = unique / legendary / epic / primeval 四档石头）
//
// ⇒ **grade ∈ 1..4**。UI 那一侧是客户端自己的活在干：
// 进 EOO 副本（客户端把 `[dungeon type] = endkeeper of order` 编号成 212）时由
// sub_140658530 开两个窗口（0xF8C 个人、0xF8B 队伍），离开时 sub_140658690 关闭；
// 这两个函数只被 EOO 单例的 vftable（off_1492D76A0 +0x30/+0x38）引用
// ⇒ **服务端不需要（也无法）去"打开"UI，只需把状态喂进去**。
//
// 这条通道有两个来源：-omen-info 的显式载荷（诊断注入，用来实测字段语义），以及
// -omen-state 打开后按**角色存档**里的真实持有档数自动生成（正常路径，见
// omen_state.go）。两个都没配就什么都不发 —— 客户端进 EOO 副本时自己会把窗口开好，
// 没状态就是空格子。

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"
)

const (
	// omenInfoPacketID 是 noti 2836 的 id。
	omenInfoPacketID = 2836
	// omenInfoSeats 是队伍座位数（载荷里就是固定 4 条记录）。
	omenInfoSeats = 4
	// omenInfoRecordBytes 是单条记录的字节数：4 × u32 + 1 × u8。
	omenInfoRecordBytes = 4*4 + 1
	// omenInfoPayloadLen 是整条载荷长度：4 条记录 + 1 个尾标志字节。
	// 处理器 sub_146EA0BE0(&buf, 69) 写死的就是这个数。
	omenInfoPayloadLen = omenInfoSeats*omenInfoRecordBytes + 1
)

// omenInfoPayload 按线格式拼出 69 字节载荷。
//
// seats[i][j] 是第 i 个座位记录里的第 j 个 u32；states[i] 是第 i 条记录的 u8；
// flag 是尾标志字节（0 = 直接套用载荷，1 = 先按旧状态刷一遍可视化）。
func omenInfoPayload(seats [omenInfoSeats][4]uint32, states [omenInfoSeats]uint8, flag uint8) []byte {
	out := make([]byte, 0, omenInfoPayloadLen)
	for i := 0; i < omenInfoSeats; i++ {
		for j := 0; j < 4; j++ {
			out = binary.LittleEndian.AppendUint32(out, seats[i][j])
		}
		out = append(out, states[i])
	}
	return append(out, flag)
}

// ⚠️ 2026-10-07 起**不再被生产路径调用**：官方口径是「中央珠子→星蕴石、天平→誓约」，
// 两条线独立，所以不再把誓约档映射进 2836 的档位。函数保留给单测与诊断。
//
// omenGradeForOathTier 把天平档位（40..45）映射成星蕴石档位（1..4）。
//
// 名字逐档对齐（§6 取证）：109137366 Unique / 367 Legendary / 368 Epic / 369 Primeval，
// 所以 42 unique → 1、43 legendary → 2、44 epic → 3、45 primeval → 4；
// normal(40)/rare(41) 在这四个石头里没有专属档，一律取最低档 1。
//
// ⚠️ 这条映射是**我们一起补的**（业主 2026-10-01 拍板）：源里星蕴石品质只由
// getEOOPartyOmenGrade() 决定，而 noti 2836 的 grade 按 §36 只是「记录里非零 u32 的个数」
// （= 征兆持有档数），与天平档位无关。实机验证：固定 oath=45（primeval、隐藏 BOSS 正常登场）
// 时掉出的仍是 Unique 档箱子 ⇒ 源里确实没有这条链路。
//
// tier == 0 表示本场还没下发过档位（独立调用 / 测试路径），返回 0 = 不设下限，保持原行为。
func omenGradeForOathTier(tier uint16) int {
	if tier == 0 {
		return 0
	}
	switch {
	case tier >= oathGradePrimeval:
		return 4 // 45 primeval → 109137369
	case tier >= 44:
		return 3 // 44 epic → 109137368
	case tier >= 43:
		return 2 // 43 legendary → 109137367
	default:
		return 1 // 42 unique 及以下 → 109137366
	}
}

// omenInfoPayloadForHeld 把**真实**的已激活征兆编成 69 字节载荷。
//
// 档位 = 记录里非零 u32 的个数（§36 实机两端验证：1 档一颗石头、亮第一格；4 档四颗、
// 亮第四格），所以持有 N 档就写前 N 个。每个值是该档奖励盒的模板号
// （loot.AttunementRewards.OmenStageIDs），客户端拿它做奖励预览。
//
// state 字节 = 1 才有征兆石与屏幕特效（omen_drop_1.act 的门槛是 `state == 1 且
// grade >= 1`）；没有征兆时整条记录留 0，客户端显示空格子。
//
// 四个座位写同一份：本机跑的是单机副本（soloPartyBootstrap），客户端认哪个座位号
// 不确定，四份写一样可以保证 t:seatIndex() 取哪一份都对。实机验证用的也是这个做法
// （§36.4 的注入表就是四段写一样）。
func omenInfoPayloadForHeld(activeIDs []uint32) []byte {
	if len(activeIDs) > 4 {
		activeIDs = activeIDs[:4]
	}
	var seats [omenInfoSeats][4]uint32
	var states [omenInfoSeats]uint8
	for i := 0; i < omenInfoSeats; i++ {
		for j := 0; j < len(activeIDs); j++ {
			if activeIDs[j] != 0 {
				seats[i][j] = activeIDs[j]
			}
		}
		if len(activeIDs) > 0 {
			states[i] = 1
		}
	}
	return omenInfoPayload(seats, states, 0)
}

// parseOmenInfo 解析 -omen-info 的值。
//
// 两种写法：
//
//	① 原始载荷：138 个十六进制字符（69 字节），直接采用；
//	② 可读写法：4 个座位段（必需），后面可以再跟 1 个尾标志段。
//	   座位段 = 5 个数字 u32,u32,u32,u32,u8（空格随意）；
//	   整段写成 0 或 - 表示该座位全零。
//
// 尾标志段**可以整段省略**（等价于写一个空的第 5 段，取 0）—— 少写一个 `;0` 不该
// 让服务端起不来，这是踩过的坑：默认值少了一段，服务端在玩家"进频道"那一刻
// log.Fatalf 掉，现象就是"进不去频道"。
//
// 空串表示不注入。
func parseOmenInfo(spec string) ([]byte, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	if isOmenInfoHex(spec) {
		raw, err := hex.DecodeString(strings.ReplaceAll(spec, " ", ""))
		if err != nil {
			return nil, err
		}
		if len(raw) != omenInfoPayloadLen {
			return nil, fmt.Errorf("raw payload is %d bytes, want %d", len(raw), omenInfoPayloadLen)
		}
		return raw, nil
	}

	parts := strings.Split(spec, ";")
	if len(parts) != omenInfoSeats && len(parts) != omenInfoSeats+1 {
		return nil, fmt.Errorf("want %d seat groups plus an optional trailing flag (%d or %d groups), got %d in %q",
			omenInfoSeats, omenInfoSeats, omenInfoSeats+1, len(parts), spec)
	}
	var seats [omenInfoSeats][4]uint32
	var states [omenInfoSeats]uint8
	for i := 0; i < omenInfoSeats; i++ {
		g := strings.TrimSpace(parts[i])
		if g == "" || g == "0" || g == "-" {
			continue // 该座位全零
		}
		fields := strings.Split(g, ",")
		if len(fields) != 5 {
			return nil, fmt.Errorf("seat %d wants 5 numbers (u32,u32,u32,u32,u8), got %d", i, len(fields))
		}
		for j := 0; j < 4; j++ {
			v, err := strconv.ParseUint(strings.TrimSpace(fields[j]), 10, 32)
			if err != nil {
				return nil, fmt.Errorf("seat %d field %d: %w", i, j, err)
			}
			seats[i][j] = uint32(v)
		}
		v, err := strconv.ParseUint(strings.TrimSpace(fields[4]), 10, 8)
		if err != nil {
			return nil, fmt.Errorf("seat %d state: %w", i, err)
		}
		states[i] = uint8(v)
	}
	flagField := ""
	if len(parts) > omenInfoSeats {
		flagField = strings.TrimSpace(parts[omenInfoSeats])
	}
	if flagField == "" {
		flagField = "0"
	}
	flag, err := strconv.ParseUint(flagField, 10, 8)
	if err != nil {
		return nil, fmt.Errorf("trailing flag: %w", err)
	}
	return omenInfoPayload(seats, states, uint8(flag)), nil
}

// isOmenInfoHex 判断这串是不是「原始载荷」写法：去掉空格后正好是 69 字节的十六进制。
func isOmenInfoHex(spec string) bool {
	compact := strings.ReplaceAll(spec, " ", "")
	if len(compact) != omenInfoPayloadLen*2 {
		return false
	}
	for _, r := range compact {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}

// omenInfoPackets 在副本加载时把征兆队伍状态发下去。
//
// 时机与 oathInfoPackets 一致：都在 `dungeon_loading_ack` 之后、客户端开始读 getter 之前。
// 客户端进 EOO 副本时自己已经把两个征兆窗开好了，所以这里只要把状态填进去。
//
// 两个来源，**显式注入优先**：
//
//   - -omen-info 给了载荷 -> 原样发（诊断通道，用来实测字段语义）；
//   - 否则 -omen-state 开着且这个副本带征兆阶段表 -> 按本场刚读出的真实持有档数编码
//     （loadOmenRunState 已经把存档读进 w.omenHeldRun，所以这里不再碰库）。
//
// 都不是就不发 —— 没状态就是空格子，客户端自己会清。
func (w *worldSession) omenInfoPackets() ([]outboundPacket, error) {
	if len(w.omenInfo) > 0 {
		return []outboundPacket{{"omen_of_order_party_info", 0, omenInfoPacketID, w.omenInfo}}, nil
	}
	if !w.omenState || !w.omenHeldReady {
		return nil, nil
	}
	dungeon := w.oathProgressDungeon()
	stages := w.omenStagesCount(dungeon)
	if stages == 0 {
		return nil, nil
	}
	ids := w.omenStageIDs(dungeon)
	held := int(w.omenHeldRun)
	if held > stages-1 {
		held = stages - 1
	}
	// ⚠️ 2026-10-07 撤除「天平档位压进本轮档位」那层：官方口径是
	// **中央珠子 → 星蕴石的稀有度**、**天平 → 誓约的稀有度**（业主 2026-10-07 提供），
	// 两条线各自独立（客户端 `primer_00_normal_loop.act` 里 `PRIMER_UP_*` 与
	// `OATH_UP_TO_*` 是两组动作）。原来的 `grade = max(held, omenGradeForOathTier(oath))`
	// 把**誓约档**灌进了星蕴石/征兆这一格 —— 副作用正是业主实机反馈的
	// 「每次进入必定保底一阶征兆」（任何非 0 的天平档都会把 grade 抬到 ≥1）。
	// 现在 2836 的档位是**纯粹的征兆持有数**；珠子那条线走 2838 的 primer。
	grade := held
	// 诊断：档位 = 征兆持有数本身（2026-10-07 撤除天平档下限后不再有 floor）。
	// 珠子那条线（primer）走 2838，不再进这里。
	log.Printf("omen party info: held=%d oathTier=%d primerTier=%d preview=%v",
		held, w.oathTierRun, w.attunementRunTiers.Primer, omenActiveIDs(ids, grade))
	return []outboundPacket{{"omen_of_order_party_info", 0, omenInfoPacketID,
		omenInfoPayloadForHeld(omenActiveIDs(ids, grade))}}, nil
}

// omenActiveIDs 从整张阶段表里摘出「当前持有档数」对应的那几个预览模板。
//
// ids[0] 是行 0（「有概率激活第一个征兆」，本身没有任何条目），所以真正激活的第 k 档
// 是 ids[k]，k = 1..held。抽成纯函数是为了能单独测这条下标关系 —— 它在实机上的表现是
// 「UI 亮错格子」，而错一格从截图上很难看出来。
func omenActiveIDs(ids []uint32, held int) []uint32 {
	if held <= 0 || held >= len(ids) {
		return nil
	}
	return ids[1 : held+1]
}
