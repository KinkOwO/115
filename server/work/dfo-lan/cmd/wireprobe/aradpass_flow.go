package main

// 战令（阿拉德通行证 / Arad Pass，活动 516/10005）—— 会话侧流程与接线。
//
// ## 帧
//
//	0x08A1 (2209)  账号级状态：等级 / 累计经验 / 满级经验 / 购票经验 + 两条任务列表（正文 760B）
//	0x08A2 (2210)  角色槽级状态：4B 头 + 20×55B（正文 1104B）
//
// 两者都是**纯 S2C**，登录必须下发，否则客户端面板空白（抓包：登录期各下发 18 / 21 次）。
// 字节契约见 internal/game/protocol/aradpass115.go 的头部注释（18/21 份样本交叉校验）。
//
// ## 领奖（CMD680/681，与签到 331 / 在线 313 / 直升 662 同一族）
//
// 抓包实证的 16B 正文：`<赛季 516> <轨道> <奖励槽> <保留>`。回包沿用活动族的
// `EventReply115`（`01 <事件号> <十格值>`），第一格 = 请求的第二个字（轨道）。
//
// ★ 帧序**必须先推后答**（与 331/313 同一条已实机验证过的硬约束）：
//
//	ch1 680 请求
//	  → ch0 2210 + ch0 2209（刷新面板：等级/已领/皇家锁）   ← 推送先于应答
//	  → ch0 14（奖励落袋的行刷新）
//	  → ch1 680 应答（清在途标志）
//
// 来源实现把应答排在状态帧之前，但它自己的注释写的抓包序就是"重推在应答之前"
// （21:34:51.829/837 vs 51.938）—— 这里按抓包序，不按那份代码的字面顺序。
//
// ## 680 vs 681
//
// 680 = `ENUM_CMDPACKET_EVENT_REWARD`（领奖）、681 = 查询。**只有 680 且带奖励槽
// （16B）才真发奖**；8B / 681 一律只重推状态 + 应答。抓包只有 680 一个样本，
// 681 的语义是推断 ⇒ 宁可少做（不凭空发奖），也不猜。
//
// ## 皇家通行证：拥有即激活
//
// 源里通行证是**点券商城商品**（`[pass item index]`，`[item category] event`），
// 本仓的点券商城是 PVF 直读、没有上架它，购买报文也没有实证。所以按"**拥有即激活**"
// 收口：背包里出现那件物品就置 Royal + 加购票赠送经验 + 发源里的首发 4 件。
// 幂等键 `arad-pass-activate-v1`；登录时收口一次，会话内再定时复核（GM/外部脚本
// 是直写数据库的，运行中的会话看不见）。
import (
	"context"
	"dfolan/internal/aradpass"
	"dfolan/internal/attendance"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// aradPassCharacInfoNotiID / aradPassEventInfoNotiID 是两条状态帧在**会话侧**的别名。
//
// 与 `honeyTimeNotiID = 1294` 同一取舍：entry_flow.go 不引协议包（它只拼帧列表），
// 通知号在那边按名字引用。opcode 的唯一定义仍在 internal/game/protocol。
const (
	aradPassCharacInfoNotiID = protocol.AradPassCharacInfoOpcode
	aradPassEventInfoNotiID  = protocol.AradPassEventInfoOpcode
)

// aradPassPayloads 组装战令的两帧状态。
//
// 0x08A2 的槽位**保持全空**（charID = 0xFFFFFFFF）：抓包 21 份样本在玩家本人正在
// 游戏的情况下每一条都是空槽，说明这 20 槽并不是"账号角色列表"；非空槽里那 46 字节
// 更没有实证。照抄抓包的空白态比凭空发明字段安全。（来源实现把当前角色登记进第 0 槽，
// 那是它自己的判断，无实证 —— 这里不跟。）
func aradPassPayloads(c *aradpass.Catalog, role database.Character) (event, charac []byte, err error) {
	if c == nil {
		return nil, nil, fmt.Errorf("arad pass catalog is not prepared")
	}
	state, err := aradpass.ReadState(role.State)
	if err != nil {
		return nil, nil, err
	}
	progress := protocol.AradPassProgress{
		Level:       state.Level(c),
		Exp:         state.Exp,
		MaxExp:      c.MaxExp(),
		PurchaseExp: c.PurchaseExp,
		// 已购皇家：面板 ROYAL 轨的解锁位（候选字节见 protocol.AradPassProgress.Royal）。
		Royal: state.Royal,
		// 已购皇家 ⇒ 2209 正文头 = 2（★ ROYAL 轨解锁的开关，实机双向验证过）。
		Purchased: state.Royal,
		// 已领档位图（面板"已领"状态的唯一来源；bit0 = 普通轨第 1 档，实机双向验证过）。
		ClaimedSlots: aradPassClaimedSlots(state, c),
		// 每日礼包"今天已领"：面板那行的绿勾 + 客户端**发送前**的门禁（实机 + IDA 双证）。
		// 口径与领取同一把钥匙（aradPassRoyalDailyKey，09:00 UTC），
		// 所以"领了却还说能领"这种不一致从结构上不可能发生。
		RoyalDailyDay: aradPassRoyalDailyDay(state, time.Now()),
		// 两条任务列表的逐行计数（面板 Mission 页签的 `n / m` 与 `k time(s)`）。
		// 不给这两份时客户端整列表显示 0/N —— 就是 2026-10-11 实机看到的样子。
		NormalMissions: aradPassMissionRows(state, c, aradpass.MissionTrackNormal, len(c.NormalMissions)),
		SeasonMissions: aradPassMissionRows(state, c, aradpass.MissionTrackSeason, len(c.SeasonMissions)),
	}
	if rows, ok := aradPassMissionProbe(); ok {
		// 判别探针：把三个候选位置写成互不相同的哨兵，一次重启就能看出
		// 面板的 `n / m` / `k time(s)` 到底读哪一格（见 aradPassMissionProbe 的注释）。
		progress.NormalMissions, progress.SeasonMissions = rows, rows
	}
	event, err = protocol.AradPassEventInfo(progress)
	if err != nil {
		return nil, nil, err
	}
	aradPassBodyPoke(event)
	if charac, err = protocol.AradPassCharacInfo(aradPassCharacHeader(), []protocol.AradPassCharSlot{aradPassCharSlot(role)}); err != nil {
		return nil, nil, err
	}
	aradPassCharacPoke(charac)
	return event, charac, nil
}

// aradPassClaimedSlots 把角色状态里的"已领档"折成 0x08A1 的**逐档标记数组**（每档 1 字节）。
//
// 索引 = **该轨的序号**（不是等级）：`0..24` = 普通轨 slot 1..25、`25..49` = 皇家轨 slot 1..25。
//
// ★★ 2026-10-11 实机两轮对照定案（这是"ROYAL 格子一直显示 Get、点了没反应"的根因）：
//
//	发出 0x1C2 = 01 00 …  ⇒ 面板只有**第 1 格** Complete
//	发出 0x1C2 = ff 01 …  ⇒ 面板只有**第 1、2 格** Complete，第 3、4 格仍是 Get
//
// 若是位图，0xff 该点亮 8 格；而且第 2 个亮的是 **Free 第 2 格**（不是 Royal 第 1 格）
// ⇒ 它是"每档 1 字节"的数组且按每轨序号排。之前按"bit = 等级−1"折位图，
// 皇家那 25 个字节恒为 0 ⇒ 皇家 4 格永远是可点的 Get。
func aradPassClaimedSlots(st aradpass.State, c *aradpass.Catalog) []byte {
	out := make([]byte, aradPassClaimedSlotCount)
	if c == nil {
		return out
	}
	for key, claimed := range st.Claimed {
		if !claimed {
			continue
		}
		track, slot, ok := splitAradPassClaimKey(key)
		if !ok {
			continue
		}
		if _, found := c.RewardAt(track, slot); !found {
			continue
		}
		index := slot - 1
		if track == aradpass.TrackRoyal {
			index += aradPassNormalSlotCount
		}
		if index < 0 || index >= len(out) {
			continue
		}
		out[index] = aradPassClaimedMark
	}
	return out
}

// aradPassMissionTotalExpScale 是任务行 `Total` 字段的口径：**字段 = 经验 / 10**
// （面板显示时 ×10，实测锚点见 protocol.AradPassMissionRow.TotalExp）。
const aradPassMissionTotalExpScale = 10

// aradPassClaimedMark 是"已领"标记的取值（非 0 即可；取 1 与实机验证过的
// `0x1C2 = 1 → 第 1 格 Complete` 一致）。
const aradPassClaimedMark byte = 1

// 普通轨 25 档 + 皇家轨 25 档；逐档 1 字节 ⇒ 数组 50 字节。
const (
	aradPassNormalSlotCount  = 25
	aradPassClaimedSlotCount = 50
)
const aradPassClaimBitCount = 50

// splitAradPassClaimKey 解析 `"<轨道>:<槽>"`。
func splitAradPassClaimKey(key string) (track, slot int, ok bool) {
	parts := strings.Split(key, ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	t, e1 := strconv.Atoi(parts[0])
	s, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil || s <= 0 {
		return 0, 0, false
	}
	return t, s, true
}

// aradPassMissionProbe 读 `DFO_ARAD_PASS_MISSION_PROBE`：诊断用，把**每一行**的三个候选
// 位置写成互不相同的哨兵（`@+1 = 41`、`@+3 = 42`、`@+5 = 43`）。
//
// 为什么需要它：`@+3` / `@+5` 里哪个是 `n / m` 的 n、哪个是 `k time(s)` 的 k，
// 反汇编只证明了**这两个 u16 被客户端读**（一个拿去比"变了没有"、一个被带进 UI 记录），
// 但没证明谁是谁；`@+1` 更是完全没有消费者证据。
// 一轮探针就能定性：面板上出现 41 的那一栏是 `@+1`、42 的是 `@+3`、43 的是 `@+5`。
// **缺省（非探针）**口径 = `@+3 = 进度`、`@+5 = 次数`、`@+1 = 0`。
func aradPassMissionProbe() ([]protocol.AradPassMissionRow, bool) {
	if !envFlagOn("DFO_ARAD_PASS_MISSION_PROBE") {
		return nil, false
	}
	rows := make([]protocol.AradPassMissionRow, 16)
	for i := range rows {
		rows[i] = protocol.AradPassMissionRow{
			Progress: aradPassProbeProgress, // @+1 → 面板 `n / m` 的 n
			TotalExp: aradPassProbeTotal,    // @+3 → 面板 `Total`
			Times:    aradPassProbeTimes,    // @+5 → 面板 `k time(s)`
		}
	}
	return rows, true
}

// 探针哨兵：三个值必须互不相同、形状明显，且**小到不会把面板的排版撑坏**。
//
// 实机读法（2026-10-11 已用它定案）：面板 `n / m` 列出现 41 ⇒ 进度在 `@+1`；
// `Total` 列出现 42 ⇒ 累计经验在 `@+3`；`k time(s)` 列出现 43 ⇒ 次数在 `@+5`。
const (
	aradPassProbeProgress = 41
	aradPassProbeTotal    = 42
	aradPassProbeTimes    = 43
)

// aradPassMissionRows 把角色的任务计数映射成 0x08A1 的行（纯搬运，语义在
// internal/aradpass 的 MissionProgressList）。
func aradPassMissionRows(st aradpass.State, c *aradpass.Catalog, track, count int) []protocol.AradPassMissionRow {
	rows := st.MissionProgressList(c, track, count)
	out := make([]protocol.AradPassMissionRow, 0, len(rows))
	for _, r := range rows {
		// `TotalExp` = 该任务累计给过的经验（面板 `Total` 列）。
		// ★ 口径是**十分之一经验**：面板显示值 = 该字段 × 10（实测锚点见
		// protocol.AradPassMissionRow.TotalExp）⇒ 通关 1 次 300 经验要写 30。
		out = append(out, protocol.AradPassMissionRow{
			Progress: r.Progress,
			TotalExp: uint16(uint32(r.Times) * uint32(r.Exp) / aradPassMissionTotalExpScale),
			Times:    r.Times,
		})
	}
	return out
}

// aradPassCharSlot 组装 0x08A2 里属于**当前角色**的那一条槽。
//
// ⚠ 这里与最初移植时的取舍**相反**：最初照抓包"21 份样本全是空槽"把 20 槽全空，
// 但 2026-10-11 实机的面板把 Free 第 1 档显示成**绿勾 Complete（不可点）**，
// 而 `sub_1422A1770`（2210 的解析函数）证明：
//
//	`if (*v6 != 0 || *(u32*)(v6+1) != 0 || *(u32*)(v6+17) != 0)` 才认这一条记录；
//	记录里 `u32@+5 = charID`，解析成 0 就 `sub_14021ADF0(.., -1)` 失败 ⇒
//	**客户端自己发 CMD681 `(516, 7, u8@0, u32@1)` 向服务端要这一条**。
//
// 20 槽全空 = 客户端手上**一条本角色的通行证记录都没有** ⇒ 面板只能拿缺省态渲染
// （观察到的就是那个绿勾）。所以登记当前角色是必需的，不是可选的美化。
//
// 字段划分同样来自 `sub_1422A1770` 的反汇编：
//
//	u8  @+0  记录键低位（与 u32@+1 一起做红黑树键）
//	u32 @+1  记录键高位
//	u32 @+5  **charID**（0xFFFFFFFF = 空槽）
//	u32 @+9  种类：1 / 2 / 3（1 或 3 → 列表B；2 或 3 且 u8@+15 != 0 → 列表A）
//	u8  @+15 标志（列表A 要求非 0）
//	u32 @+17 参与"非空"判据
func aradPassCharSlot(role database.Character) protocol.AradPassCharSlot {
	charID := uint32(role.WireID)
	// 诊断：把 charID 写成"解析不出来"的值。这时客户端会**主动发 681
	// `(516, 7, u8@0, u32@1)`** 向服务端要这一条记录（`sub_1422A1770` 里的
	// `else` 分支），日志里就能看到它到底认不认我们的键 —— 用来验证"记录是否被接受"。
	if envFlagOn("DFO_ARAD_PASS_CHARAC_UNRESOLVED") {
		charID = 0xFFFFFFFF
	}
	return protocol.AradPassCharSlot{
		CharID: charID,
		Lead:   aradPassCharSlotByte("DFO_ARAD_PASS_CHARAC_LEAD", 1),
		Key:    aradPassCharSlotU32("DFO_ARAD_PASS_CHARAC_KEY", 1),
		Kind:   aradPassCharSlotU32("DFO_ARAD_PASS_CHARAC_KIND", 3),
		Flag:   aradPassCharSlotByte("DFO_ARAD_PASS_CHARAC_FLAG", 1),
		// ★ @+16：`sub_14229FF90` 的返回值就是这一格 ⇒ 非 0 = 该角色的皇家通行证生效。
		State: aradPassCharSlotByte("DFO_ARAD_PASS_CHARAC_STATE", 1),
		Extra: aradPassCharSlotU32("DFO_ARAD_PASS_CHARAC_EXTRA", 0),
	}
}

// 0x08A2 记录槽的五个字段全部可用环境变量热换，方便一轮一轮试而**不必重编**：
//
//	DFO_ARAD_PASS_CHARAC_LEAD   @+0  缺省 1（源 `[royal registable count] 1 10` 的下界）
//	DFO_ARAD_PASS_CHARAC_KEY    @+1  缺省 1（**必须非 0**，否则整条被跳过）
//	DFO_ARAD_PASS_CHARAC_KIND   @+9  缺省 3（位掩码：bit0 普通、bit1 皇家）
//	DFO_ARAD_PASS_CHARAC_FLAG   @+15 缺省 1（bit1 那条列表要求非 0）
//	DFO_ARAD_PASS_CHARAC_STATE  @+16 缺省 1（★ 判定函数返回的那一格，非 0 = 皇家生效）
//	DFO_ARAD_PASS_CHARAC_EXTRA  @+17 缺省 0（语义未定）
//	DFO_ARAD_PASS_CHARAC_HEADER 正文前 4 字节缺省 **2**（判定函数的第一句就要求 == 2）
const aradPassCharSlotKey uint32 = 1

func aradPassCharSlotU32(name string, def uint32) uint32 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	v, e := strconv.ParseUint(raw, 0, 32)
	if e != nil {
		return def
	}
	return uint32(v)
}

// aradPassBodyPoke 是**诊断用**的缓冲探针：按 `DFO_ARAD_PASS_BODY_POKE` 把 2209 正文的
// 指定格写成指定值，用一次重启就能判别"某个保留字节是不是某个开关"。
//
// 格式：`<偏移>:<值>[,<偏移>:<值>…]`。
//
//	**偏移一律按十六进制读**（写成 `8B`/`140`/`1C9` 就行，也接受 `0x` 前缀）；
//	**值按十进制读**（0..255，也接受 `0x` 前缀）。
//
// 例：`DFO_ARAD_PASS_BODY_POKE=8B:1,140:1,1C9:1`
//
// **保护段**（写进去会破坏已取证的布局，直接拒绝）：
//
//	0x04..0x17  等级 / 累计经验 / 各项 ×256 定点
//	0x1A..0x8A  普通任务条数 + 16 行任务
//	0xED..0x141 赛季任务条数 + 12 行任务
//	0x1C0..0x1C1 每日礼包"已领日"（实机 + IDA 双证，已是功能字段）
//	0x1C2..0x1C8 已领档位图（实机双向验证过的那 7 字节）
//
// 这样 0x00..0x03（u32 头）、0x18/0x19、0x8B..0xEC、0x142..0x1BF、0x1C9..0x2F7
// 这些**保留区**都可以试 —— 面板读缓冲是已被 0x1C2 证明的事实。
//
// ★ 2026-10-11 起 `0x1C0..0x1C1` 也进保护段：它已定案为「每日礼包已领日」
// （protocol.aradPassOffsetRoyalDailyDay），是**功能字段**而不是保留区。
func aradPassBodyPoke(body []byte) {
	applyPokeSpec("DFO_ARAD_PASS_BODY_POKE", len(body), body, func(off int) bool {
		switch {
		case off >= 0x04 && off <= 0x17:
			return true
		case off >= 0x1A && off <= 0x8A:
			return true
		case off >= 0xED && off <= 0x141:
			return true
		case off == 0x1C0 || off == 0x1C1:
			// 「每日礼包已领日」——功能字段，探针不许再写。
			return true
		case off >= 0x1C2 && off <= 0x1C8:
			return true
		}
		return false
	})
}

// aradPassCharacPoke 是 0x08A2 的**每角色记录**探针（诊断用）。
//
// 偏移相对**第 0 条记录的起点**（= 正文 +4），范围 0..54。
// 用途：这条 55 字节的记录里 `@+21..@+54` 那 34 字节我们从来没写过；
// 而"Royal Pass Character"本身是**按角色**的设计 ⇒ 皇家轨的已领标记很可能就在这里。
// 例：`DFO_ARAD_PASS_CHARAC_POKE=21-3D:1`（把 25 个字节全置 1）。
func aradPassCharacPoke(body []byte) {
	const rec = aradPassCharacRecordOffset
	if len(body) < rec+aradPassCharacSlotSize {
		return
	}
	view := body[rec : rec+aradPassCharacSlotSize]
	applyPokeSpec("DFO_ARAD_PASS_CHARAC_POKE", len(view), view, func(int) bool { return false })
}

// aradPassCharacSlotSize 是单条记录的字节数（55）。
const aradPassCharacSlotSize = 55

// aradPassCharacRecordOffset 是第 0 条记录在正文里的起点（4 字节正文头之后）。
const aradPassCharacRecordOffset = 4

// applyPokeSpec 解析 `<偏移>:<值>[,<偏移>:<值>…]` 并写进 dst。
//
// 偏移**按十六进制**读，支持**闭区间**写法 `A-B:值`（例：`21-3D:1` 写 0x21..0x3D）；
// 值按十进制读（0..255，接受 `0x` 前缀）。`protected` 返回 true 的偏移会被拒绝。
func applyPokeSpec(env string, size int, dst []byte, protected func(int) bool) {
	raw := strings.TrimSpace(os.Getenv(env))
	if raw == "" {
		return
	}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, ":", 2)
		if len(parts) != 2 {
			log.Printf("arad pass body poke: 忽略格式不对的 %q（要 <偏移>:<值>）", item)
			continue
		}
		// ★ 偏移一律按**十六进制**读（写 `8B` 而不是 `0x8B` 才顺手），可写闭区间 `A-B`；
		// 值按十进制读。第一版两个都用 ParseInt(..., 0, 32)，`8B` 直接解析失败被当"越界"跳过。
		rangeStr, valStr := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		var val64 int64
		var valErr error
		if strings.HasPrefix(valStr, "0x") || strings.HasPrefix(valStr, "0X") {
			val64, valErr = strconv.ParseInt(valStr[2:], 16, 32)
		} else {
			val64, valErr = strconv.ParseInt(valStr, 10, 32)
		}
		loStr, hiStr := rangeStr, rangeStr
		if k := strings.Index(rangeStr, "-"); k > 0 {
			loStr, hiStr = strings.TrimSpace(rangeStr[:k]), strings.TrimSpace(rangeStr[k+1:])
		}
		lo, e1 := strconv.ParseInt(strings.TrimPrefix(strings.TrimPrefix(loStr, "0x"), "0X"), 16, 32)
		hi, e3 := strconv.ParseInt(strings.TrimPrefix(strings.TrimPrefix(hiStr, "0x"), "0X"), 16, 32)
		if e1 != nil || e3 != nil || valErr != nil || lo < 0 || hi < lo || hi >= int64(size) || val64 < 0 || val64 > 255 {
			log.Printf("%s: 忽略越界的 %q", env, item)
			continue
		}
		for off := int(lo); off <= int(hi); off++ {
			if protected != nil && protected(off) {
				log.Printf("%s: 拒绝写保护段 0x%X（会破坏已取证的布局）", env, off)
				continue
			}
			dst[off] = byte(val64)
		}
		log.Printf("%s: [0x%X..0x%X] = %d", env, lo, hi, val64)
	}
}

// aradPassCharacHeader 取 0x08A2 正文前 4 字节。
//
// ★ 缺省 **2**：`sub_14229FF90` 第一句 `if (*(u32*)(passObj + 0xAC8) != 2) return 0;`
// —— 这一格不对，"已购买"判定永远为假（ROYAL 上锁、Royal Pass Character 恒 0）。
// `DFO_ARAD_PASS_CHARAC_HEADER` 可覆盖（试 0/1/2/3）。
func aradPassCharacHeader() uint32 {
	return aradPassCharSlotU32("DFO_ARAD_PASS_CHARAC_HEADER", protocol.AradPassCharacHeader)
}

func aradPassCharSlotByte(name string, def byte) byte {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	v, e := strconv.ParseUint(raw, 0, 8)
	if e != nil {
		return def
	}
	return byte(v)
}

// ensureAradPassPassItem 实现"账号级一次性发放通行证物品"。
//
// ⚠ **业主 2026-10-11 决定不再发它**（原话：「ROYAL PASS 本身不是一个可以右键打开的道具，
// 建议你不用发送它，而是应该静默的为每个角色解开 ROYAL 轨」）⇒ 缺省**关闭**
// （`DFO_ARAD_PASS_GRANT_ITEM=1` 才开）。
// 机制保留是因为它同时是"账号级幂等发放"的参考实现，将来接点券商城购买时可以直接借用。
//
// 键里带**模板号**：通行证是**每季一样**的商品（源 `[pass item index]`），换季就该发新的那一件，
// 所以"一件物品每账号一次"才是正确粒度（同一季重复登录不会重复发）。
func aradPassPassItemGrantID(template uint32, account int64) string {
	return fmt.Sprintf("arad-pass-item:%d:%d", template, account)
}

// ensureAradPassPassItem 用现成的 `database.Store.ApplyGrant`（`grant_id` 主键 = 幂等键）
// 给**第一个登录的角色**发一件通行证物品，之后同账号不再发。缺省关闭，见上。
func (w *worldSession) ensureAradPassPassItem(ctx context.Context) (bool, error) {
	if w == nil || w.aradPass == nil || w.loot == nil || w.store == nil || w.role.ID == 0 {
		return false, nil
	}
	if len(w.aradPass.PassItem) == 0 {
		return false, nil
	}
	// 缺省**关**（业主口径）。envFlagOn 只在显式 1/on/true/yes 时为真。
	if !envFlagOn("DFO_ARAD_PASS_GRANT_ITEM") {
		return false, nil
	}
	template := w.aradPass.PassItem[0]
	// 已经自己拿到了（GM 发过 / 商城买过 / 上一轮发过）⇒ 什么都不做。
	if ownsAradPassItem(w.role.State, template) {
		return false, nil
	}
	grant := database.Grant{
		ID:        aradPassPassItemGrantID(template, w.account),
		AccountID: w.account,
		Character: w.role.ID,
		Items:     []database.GrantItem{{Template: template, Amount: 1}},
		Operator:  "system",
		Reason:    "arad pass: first-login account activation grant",
	}
	result, err := w.store.ApplyGrant(ctx, grant, func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		return w.loot.PrepareAradPassItemGrant(workflow.LootRole(current), template, 1)
	})
	if err != nil {
		return false, err
	}
	if result.Character.ID != 0 {
		saved := result.Character
		saved.WireID = w.role.WireID
		w.role = saved
	}
	if result.Applied {
		log.Printf("arad pass pass-item granted role=%d account=%d template=%d", w.role.ID, w.account, template)
	}
	return result.Applied, nil
}

// envFlagDefaultOn 读一个 `DFO_*` 布尔开关：**缺省开**，`0/off/false/no` 关。
func envFlagDefaultOn(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "0", "off", "false", "no":
		return false
	}
	return true
}

// envFlagOn 读一个 `DFO_*` 布尔开关（缺省 = 关；`1/on/true` = 开）。
func envFlagOn(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "on", "true", "yes":
		return true
	}
	return false
}

// dispatchAradPass 是入站派发链里的一站：只管 CMD680/681 上事件号为 516/10005 的帧。
func (client *gameConnection) dispatchAradPass(requestData *clientRequest) dispatchAction {
	w := client.worldState
	if requestData.frame.Type != 1 || !requestData.verified || !client.bootstrapped || w == nil || w.aradPass == nil {
		return dispatchNext
	}
	if requestData.frame.ID != 680 && requestData.frame.ID != 681 {
		return dispatchNext
	}
	event, ok := protocol.EventRequestEventID115(requestData.plaintext)
	if !ok || !aradpass.IsSchedule(event) {
		// 别的活动（331/313/662/665）的 680/681 —— 不接管。
		return dispatchNext
	}
	// ★ 680 的 8B 形态**不都是**自动帧：`516, 3` 是「**领取 Royal Pass 每日登录礼包**」——
	// 实机 2026-10-11：Royal Pass 页签点 `Get (1)`，客户端连发 `516, 3`（8B，无奖励槽）。
	// 此前一律忽略 ⇒ 业主点几次都领不下来。这一种必须放行到 aradPassRequest。
	track, hasTrack := protocol.EventRequestTrack115(requestData.plaintext)
	if requestData.frame.ID == 680 && len(requestData.plaintext) < 16 &&
		(!hasTrack || track != protocol.AradPassRoyalDailyTrack) {
		// 其余 8B（331/313 的同款形态：只有活动号 + 动作类型）仍是**进城自动帧**，
		// 不回应答 —— 回了会在每次进城时给玩家弹一个无意义的提示。
		client.event(map[string]any{
			"kind":         "arad_pass_request_ignored",
			"character_id": w.role.ID,
			"reason":       "no reward slot (auto frame on entry)",
			"request_hex":  hex.EncodeToString(requestData.plaintext),
		})
		return dispatchNext
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	plan, err := w.aradPassRequest(ctx, requestData.plaintext, requestData.frame.ID)
	if err != nil {
		client.event(map[string]any{
			"kind":         "arad_pass_request_refused",
			"id":           requestData.frame.ID,
			"character_id": w.role.ID,
			"reason":       err.Error(),
			"request_hex":  hex.EncodeToString(requestData.plaintext),
		})
	}
	// 拒绝帧本身就在 plan 里（kind=1 回执），有错也要先把包发出去再收日志。
	if client.sendPlan(plan, client.logWorldResponseBody) != nil {
		return dispatchClose
	}
	return dispatchHandled
}

// aradPassRequest 处理战令的 C2S：CMD680（领奖）/ CMD681（查询）上事件号 516 或 10005。
//
// 派发那一站已经把「680 的 8B 自动帧」挡掉了，所以这里的 680 一定是 16B 的点击帧。
// **只有 680 真发奖**；681 只重推状态 + 应答（抓包没有 681 样本 ⇒ 宁可少做，
// 不猜它能顺手领奖）。
func (w *worldSession) aradPassRequest(ctx context.Context, p []byte, id uint16) ([]outboundPacket, error) {
	if w == nil || w.aradPass == nil || w.loot == nil || w.store == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("arad pass requires a persisted character and the source catalog")
	}
	claim, err := protocol.DecodeAradPassClaim(p)
	if err != nil {
		return nil, err
	}
	if !aradpass.IsSchedule(claim.Event) {
		return nil, fmt.Errorf("arad pass: unexpected event %d", claim.Event)
	}
	track, level := int(claim.Track), int(claim.Slot)
	if track != aradpass.TrackNormal && track != aradpass.TrackRoyal {
		track = aradpass.TrackNormal
	}
	svc := &workflow.LootService{Store: w.store, Loot: w.loot}

	var receipt loot.AradPassRewardReceipt
	applied := false
	royalDaily := false
	if id == 680 {
		if claim.Track == protocol.AradPassRoyalDailyTrack {
			// ★ 「Royal Pass 每日登录礼包」：请求是 8B 的 `516, 3`（没有奖励槽字段）。
			// 每天一次（面板写 Reset daily at 9:00 UTC），发源 `[daily reward list]`。
			svc := &workflow.LootService{Store: w.store, Loot: w.loot}
			role, daily, wasApplied, claimErr := svc.ClaimAradPassRoyalDaily(
				ctx, w.role, w.aradPass, aradPassRoyalDailyKey(time.Now()))
			if claimErr != nil {
				return nil, claimErr
			}
			w.role, applied, royalDaily = role, wasApplied, true
			receipt.Slots = daily.Slots
			if len(daily.Items) != 0 {
				receipt.Item = daily.Items[0]
			}
			log.Printf("arad pass royal daily role=%d day=%s applied=%v items=%d slots=%d",
				w.role.ID, daily.Day, applied, len(daily.Items), len(daily.Slots))
		} else if level <= 0 {
			// ★ `slot == 0` = 面板的 **Receive All**（领取当前等级下所有可领档）。
			//
			// 证据（2026-10-11 01:30:36 实机会话）：客户端发了
			// `680 : 04020000 00000000 00000000 00000000` = `516, 轨道0, 槽0, 保留0`，
			// 而我们按 `RewardAt(0, 0)` 找不到档、回了拒绝
			// （事件 `arad_pass_request_refused: no reward for track=0 slot=0`）。
			// 抓包里那次单档领取是 `516, 0, 1, 0`（槽=1），所以槽 0 是**通配**而不是某一档。
			//
			// 口径：把**两条轨**上"等级已到、还没领"的档全部领掉（皇家轨仍要求 `Royal`）。
			// 逐档走同一条 `CommitCharacterEvent`，所以天然幂等、可重复点。
			plan, err := w.claimAllAradPassTiers(ctx)
			if err != nil {
				return nil, err
			}
			return plan, nil
		} else {
			role, r, wasApplied, claimErr := svc.ClaimAradPassReward(ctx, w.role, w.aradPass, track, level)
			if claimErr != nil {
				return nil, claimErr
			}
			w.role, receipt, applied = role, r, wasApplied
		}
	}
	// 应答的第一个 u32 = 请求的第二个 u32（轨道），照抓包。
	values := [10]uint32{}
	values[0] = claim.Track

	plan := make([]outboundPacket, 0, 4)
	// ① 状态帧先于应答（见文件头）：等级/已领/皇家锁全靠它刷新。
	if event, charac, pErr := aradPassPayloads(w.aradPass, w.role); pErr != nil {
		fmt.Printf("arad pass: state frame encode failed: %v\n", pErr)
	} else {
		plan = append(plan,
			outboundPacket{"arad_pass_charac_info_restored", 0, protocol.AradPassCharacInfoOpcode, charac},
			outboundPacket{"arad_pass_event_info_restored", 0, protocol.AradPassEventInfoOpcode, event})
	}
	// ② 奖励真的落袋了就补行刷新（首领发增量行；重放发整包，避免把已被消耗的格子复原）。
	if applied || len(receipt.Slots) > 0 {
		if bag, bagErr := inventory.ReadBag(w.role.State); bagErr == nil {
			bodyID := uint16(14)
			rows, rowErr := protocol.InventoryUpdate(stackSlotRows(bag, receipt.Slots...))
			if !applied {
				bodyID = 13
				rows, rowErr = w.loot.Bootstrap(workflow.LootRole(w.role))
			}
			if rowErr != nil {
				fmt.Printf("arad pass: inventory refresh failed: %v\n", rowErr)
			} else {
				plan = append(plan, outboundPacket{"arad_pass_inventory_refresh", 0, bodyID, rows})
			}
		}
	}
	// ③ 最后才是应答（清客户端那个按钮的在途标志）。
	plan = append(plan, outboundPacket{"arad_pass_reply", 1, id, protocol.EventReply115(claim.Event, values)})
	if !royalDaily {
		fmt.Printf("arad pass request id=%d event=%d track=%d level=%d applied=%v item=%d slots=%d exp=%d royal=%v\n",
			id, claim.Event, track, level, applied, receipt.Item, len(receipt.Slots),
			mustAradPassExp(w.role.State), mustAradPassRoyal(w.role.State))
	}
	return plan, nil
}

func mustAradPassExp(state json.RawMessage) uint32 {
	st, err := aradpass.ReadState(state)
	if err != nil {
		return 0
	}
	return st.Exp
}

func mustAradPassRoyal(state json.RawMessage) bool {
	st, err := aradpass.ReadState(state)
	if err != nil {
		return false
	}
	return st.Royal
}

// claimAllAradPassTiers 处理 `680` 的 `slot == 0`（面板的 **Receive All**）：
// 把两条轨上"等级已到、尚未领取"的档一次领完。
//
// 逐档调用 `ClaimAradPassReward`（各自一个 `CommitCharacterEvent` 事务），所以：
//   - 天然幂等：已经领过的档会被"已领"校验挡住，重放读回原回执；
//   - 某一档因背包满而失败时，**只影响那一档**，不会把已经领到的回滚掉；
//   - 不需要新的批量接口，也不会绕过任何单档校验。
//
// 帧序与单档领奖一致（推送先于应答）：状态帧 + 库存刷新在前，应答在最后。
func (w *worldSession) claimAllAradPassTiers(ctx context.Context) ([]outboundPacket, error) {
	if w == nil || w.aradPass == nil || w.loot == nil || w.store == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("arad pass requires a persisted character and the source catalog")
	}
	svc := &workflow.LootService{Store: w.store, Loot: w.loot}
	st, err := aradpass.ReadState(w.role.State)
	if err != nil {
		return nil, err
	}
	level := st.Level(w.aradPass)
	tracks := []int{aradpass.TrackNormal}
	if st.Royal {
		tracks = append(tracks, aradpass.TrackRoyal)
	}
	claimed, slots := 0, []uint16{}
	for _, track := range tracks {
		for _, row := range w.aradPass.RewardsFor(track) {
			if uint32(row.Level) > level || st.IsClaimed(track, row.Slot) {
				continue
			}
			role, receipt, _, claimErr := svc.ClaimAradPassReward(ctx, w.role, w.aradPass, track, row.Level)
			if claimErr != nil {
				// 背包满/校验不过：记一笔、跳过这一档，别把整批拖死。
				log.Printf("arad pass receive-all skipped track=%d slot=%d: %v", track, row.Slot, claimErr)
				continue
			}
			w.role, claimed = role, claimed+1
			slots = append(slots, receipt.Slots...)
			st, err = aradpass.ReadState(w.role.State)
			if err != nil {
				return nil, err
			}
		}
	}
	log.Printf("arad pass receive-all role=%d level=%d claimed=%d", w.role.ID, level, claimed)

	plan := make([]outboundPacket, 0, 4)
	if event, charac, pErr := aradPassPayloads(w.aradPass, w.role); pErr == nil {
		plan = append(plan,
			outboundPacket{"arad_pass_charac_info_restored", 0, protocol.AradPassCharacInfoOpcode, charac},
			outboundPacket{"arad_pass_event_info_restored", 0, protocol.AradPassEventInfoOpcode, event})
	}
	if len(slots) > 0 {
		if bag, bagErr := inventory.ReadBag(w.role.State); bagErr == nil {
			if rows, rowErr := protocol.InventoryUpdate(stackSlotRows(bag, slots...)); rowErr == nil {
				plan = append(plan, outboundPacket{"arad_pass_inventory_refresh", 0, 14, rows})
			}
		}
	}
	// 应答仍在最后（清客户端 Receive All 按钮的在途标志）。第一格回显轨道，与单档一致。
	values := [10]uint32{}
	plan = append(plan, outboundPacket{"arad_pass_reply", 1, 680,
		protocol.EventReply115(aradpass.ClaimScheduleID, values)})
	return plan, nil
}

// ensureAradPassActivation 把皇家轨收口到"已解锁"。
//
// 业主口径（2026-10-11 第二轮）：「**ROYAL PASS 本身不是一个可以右键打开的道具**，
// 建议你不用发送它，而是应该**静默地**为每个角色解开 ROYAL 轨」。
// ⇒ 不再要求"背包里有通行证物品"，直接按开关解锁：
//
//	DFO_ARAD_PASS_ROYAL_FREE （缺省**开**）—— 开则每个角色登录时静默解锁皇家轨
//	                                          （置 Royal + 加购票赠送经验 + 发源里的首发 4 件）；
//	                                          设 0/off 关闭，退回"拥有物品才激活"的旧口径。
//
// 幂等仍由 workflow 侧的 `arad-pass-activate-v1` 键保证：每个角色每赛季只入账一次。
// 不发道具就不会在背包里多出一件既不能开、也不能交易的账号绑定物。
func (w *worldSession) ensureAradPassActivation(ctx context.Context) (bool, error) {
	if w == nil || w.aradPass == nil || w.loot == nil || w.store == nil || w.role.ID == 0 {
		return false, nil
	}
	st, err := aradpass.ReadState(w.role.State)
	if err != nil || st.Royal {
		return false, err
	}
	free := envFlagDefaultOn("DFO_ARAD_PASS_ROYAL_FREE")
	if !free {
		// 旧口径：只有背包里真有通行证物品才激活（GM 发放 / 将来接商城购买都走这条）。
		if len(w.aradPass.PassItem) == 0 || !ownsAradPassItem(w.role.State, w.aradPass.PassItem[0]) {
			return false, nil
		}
	}
	svc := &workflow.LootService{Store: w.store, Loot: w.loot}
	role, receipt, _, err := svc.ActivateAradPass(ctx, w.role, w.aradPass)
	if err != nil {
		return false, err
	}
	w.role = role
	log.Printf("arad pass activated role=%d purchase_exp=%d first_rewards=%d free=%v",
		w.role.ID, receipt.PurchaseExp, len(receipt.Items), free)
	return true, nil
}

// refreshAradPassActivation 是会话内的**定时复核**：还有没有角色没解锁皇家轨。
//
// 为什么需要它：GM 工具/外部脚本是**直写数据库**的，运行中的会话看不见；只在登录收口
// 会让玩家觉得"发了没反应"。另外角色状态也可能被别处改写。
//
// 幂等：`st.Royal` 已为真时零写入、零回包；只有真的激活了才补发状态帧 + 一份库存刷新。
func (w *worldSession) refreshAradPassActivation(ctx context.Context) ([]outboundPacket, error) {
	if w == nil || w.aradPass == nil || w.loot == nil || w.store == nil || w.role.ID == 0 {
		return nil, nil
	}
	if st, err := aradpass.ReadState(w.role.State); err != nil || st.Royal {
		return nil, err
	}
	// 用**库里那一份**角色（外部直写只有落库才可见），WireID 从会话补回来。
	roles, err := w.store.Characters(ctx, w.role.AccountID)
	if err != nil {
		return nil, err
	}
	var fresh database.Character
	for _, role := range roles {
		if role.ID == w.role.ID {
			role.WireID = w.role.WireID
			fresh = role
			break
		}
	}
	if fresh.ID == 0 {
		return nil, nil
	}
	w.role = fresh
	activated, err := w.ensureAradPassActivation(ctx)
	if err != nil || !activated {
		return nil, err
	}
	event, charac, err := aradPassPayloads(w.aradPass, w.role)
	if err != nil {
		return nil, err
	}
	plan := []outboundPacket{
		{"arad_pass_charac_info_restored", 0, protocol.AradPassCharacInfoOpcode, charac},
		{"arad_pass_event_info_restored", 0, protocol.AradPassEventInfoOpcode, event},
	}
	if bag, bagErr := inventory.ReadBag(w.role.State); bagErr == nil {
		if body, bErr := protocol.InventoryRestore(bag.Rows(), bag.Expansion); bErr == nil {
			plan = append(plan, outboundPacket{"arad_pass_inventory_refreshed", 0, 13, body})
		}
	}
	return plan, nil
}

// ownsAradPassItem 查角色背包（道具区 + 装备区）里有没有这件通行证物品。
func ownsAradPassItem(state json.RawMessage, template uint32) bool {
	bag, err := inventory.ReadBag(state)
	if err != nil {
		return false
	}
	for _, item := range bag.Items {
		if item.Template == template && item.Amount > 0 {
			return true
		}
	}
	for _, item := range bag.Equipment {
		if item.Template == template {
			return true
		}
	}
	return false
}

// 已接线的战令任务（源 `newaradpassevent.evt` 里的稳定序号）。
//
// 源一共给了 16 条 `[normal mission]` + 12 条 `[season mission]`，每条都带
// 「要重复多少次」和「做满给多少经验」。**只有服务端真能观察到的事件才登记**，
// 其余等对应钩子接上再加 —— 加一行常量 + 一个调用点即可，不需要改引擎。
const (
	// `[season mission] 1 proper_dungeon_clear 100 800`
	aradPassMissionSeasonDungeonClear = 1
	// `[normal mission] 3 normal_dungeon_2 3 100`（只数源 `[normal dungeon index list]` 里的图）
	aradPassMissionNormalDungeon2 = 3
	// `[normal mission] 6 clear_moon_lake 1 300`（沉月湖）
	aradPassMissionNormalMoonLake = 6
	// `[normal mission] 7 clear_azure_main 1 300`（蔚蓝号）
	aradPassMissionNormalAzureMain = 7
)

// 上面两条任务对应的**副本号**（源里只有键名，没有号 ⇒ 号取自本仓已取证的常量）。
const (
	// 沉月湖：第一层 100004136（单人选的入口）/ 第二层 100004137。
	// 出处：internal/dungeon/moon_solo.go（SelectMoon 用 100004136）、
	// internal/loot/moon_rewards.go（MoonRunID 认 100004137 的结算）。
	moonLakeDungeonFirst  uint32 = 100004136
	moonLakeDungeonSecond uint32 = 100004137
	// 蔚蓝号：主层 = 内部常量 loot.AzureMainDungeonID（100004131），伴生层 100004134。
	// 出处：internal/loot/azure_clear_reward.go:13、cmd/wireprobe/azure_main_flow.go 的
	// azureMainCompanionDungeon。
	azureMainDungeonCompanion = azureMainCompanionDungeon // 100004134（同包常量）
)

// aradPassNormalMissionsForDungeon 返回"通关这张图该记哪几条**普通轨**任务"。
//
// ⚠ 源里 `[normal mission]` 只给了**键名**（`clear_moon_lake`、`clear_azure_main`…），
// **没有副本号**；副本号属于执行契约，逐条给出出处（见上面的常量注释）。
// 只登记**本仓有副本号证据**的那几条：
//
//	源 `[normal dungeon index list]` 的 14 张图 → 任务 3 `normal_dungeon_2`
//	沉月湖 100004136 / 100004137            → 任务 6 `clear_moon_lake`
//	蔚蓝号 100004131 / 100004134            → 任务 7 `clear_azure_main`
//
// 其余（`daily_mission` / `scenario_clear` / `legion_clear` / `raid_clear` /
// `start world boss` / `clear_temple_of_death` / `clear_disciple_of_doom` /
// `clear_endkeeper_of_order` / `clear_bleeding_mine` / `clear_infinite_inflorescence` /
// `clear_unshackled_nightmare_normal` / 图书馆 / 叛教者城堡）源里没有副本号、
// 本仓也没有对应证据 ⇒ **故意不接**：宁可显示 0，也不拿猜出来的 id 去凑数。
func aradPassNormalMissionsForDungeon(c *aradpass.Catalog, dungeon uint32) []int {
	out := make([]int, 0, 3)
	if c != nil && c.IsNormalDungeon(dungeon) {
		out = append(out, aradPassMissionNormalDungeon2)
	}
	switch dungeon {
	case moonLakeDungeonFirst, moonLakeDungeonSecond:
		out = append(out, aradPassMissionNormalMoonLake)
	case loot.AzureMainDungeonID, azureMainDungeonCompanion:
		out = append(out, aradPassMissionNormalAzureMain)
	}
	return out
}

// aradPassMissionEntry 是一次任务推进。
type aradPassMissionEntry struct {
	Track, Index int
	Count        uint32
}

// aradPassDayKey 是任务进度/经验池归属的**游戏日**键，形如 `2026-10-11`。
//
// ★ 换日规则：**06:00 UTC**，与签到、在线时长共用同一套（直接取
// `attendance.ResetOffsetSeconds`）—— 等价于把 UTC 时间往回挪 6 小时再取日期。
//
// 业主 2026-10-11 明确要求：本机单机修复按**每天**上限、每天换日重置任务与经验池
// （原版是按周），这样玩家每天都能重复刷任务经验。
func aradPassDayKey(now time.Time) string {
	shifted := now.UTC().Add(-time.Duration(attendance.ResetOffsetSeconds) * time.Second)
	return shifted.Format("2006-01-02")
}

// ensureAradPassDailyRoll 在**换日之后**把任务进度与经验池清零（登录时收口一次）。
//
// 幂等键 `arad-pass-daily:<游戏日>`：同一天重复登录不会重复清。
// 只有在真的换过日时才会落库（`RollDaily` 自己判断），所以平时是零写入。
func (w *worldSession) ensureAradPassDailyRoll(ctx context.Context) (bool, error) {
	if w == nil || w.aradPass == nil || w.store == nil || w.role.ID == 0 {
		return false, nil
	}
	day := aradPassDayKey(time.Now())
	st, err := aradpass.ReadState(w.role.State)
	if err != nil {
		return false, err
	}
	if st.DailyKey == day {
		return false, nil
	}
	saved, _, err := w.store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion,
		"arad-pass-daily:"+day, "arad-pass-daily-v1",
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			cur, e := aradpass.ReadState(current.State)
			if e != nil {
				return nil, nil, e
			}
			next := cur.RollDaily(day)
			raw, e := aradpass.WriteState(current.State, next)
			if e != nil {
				return nil, nil, e
			}
			receipt, e := json.Marshal(map[string]any{
				"daily_key": next.DailyKey, "reset": next.DailyKey != cur.DailyKey,
			})
			return raw, receipt, e
		})
	if err != nil {
		return false, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	log.Printf("arad pass daily roll role=%d day=%s", w.role.ID, day)
	return true, nil
}

// aradPassRoyalDailyKey 是「Royal Pass 每日登录礼包」的换日键。
//
// ★ 这里是 **09:00 UTC**，与任务行 `aradPassDayKey`（06:00 UTC）**不同** ——
// 两边都是客户端自己写的规则：任务行跟随官方口径，而每日礼包的面板明写
// `Reset daily at 9:00 UTC`。所以状态里也分成两个字段（`RoyalDailyKey` / `DailyKey`）。
func aradPassRoyalDailyKey(now time.Time) string {
	return now.UTC().Add(-9 * time.Hour).Format("2006-01-02")
}

// aradPassRoyalDailyDay 返回 0x08A1 正文 0x1C0 该写什么：今天已领 ⇒ 饱和值，否则 0。
//
// ★ 判定口径与领取**完全同一把钥匙**（aradPassRoyalDailyKey = 09:00 UTC 换日），
// 所以"服务端已经发过、面板却还说能领"这种不一致从结构上不可能发生。
//
// 取值用饱和值而不是官方那个日序号：客户端比的是
// `表值(word) + 正文[0x2EE] <= 正文[0x1C0]`，只要左边恒 ≤ 0xFFFF 就等价于"已领"，
// 这样不必知道 `word` 的刻度（见 protocol 里 aradPassOffsetRoyalDailyDay 的取证）。
func aradPassRoyalDailyDay(state aradpass.State, now time.Time) uint16 {
	if state.RoyalDailyKey == "" || state.RoyalDailyKey != aradPassRoyalDailyKey(now) {
		return 0
	}
	return protocol.AradPassRoyalDailyClaimed
}

// recordAradPassMissions 把一次事件计入战令任务，并在**经验真的变了**时补发 0x08A1。
//
// key 必须在同一事件上稳定（例如带 run id）：它同时是任务推进的幂等键，
// 重放不会把计数加两次。没有任务做满时不发任何包（状态里的计数仍会落库）。
func (w *worldSession) recordAradPassMissions(ctx context.Context, key string, entries []aradPassMissionEntry) ([]outboundPacket, error) {
	if w == nil || w.aradPass == nil || w.store == nil || w.role.ID == 0 || len(entries) == 0 {
		return nil, nil
	}
	dayKey := aradPassDayKey(time.Now())
	var gained uint32
	saved, _, err := w.store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion,
		key, "arad-pass-mission-v1",
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			st, err := aradpass.ReadState(current.State)
			if err != nil {
				return nil, nil, err
			}
			total := uint32(0)
			for _, e := range entries {
				next, got := st.RecordMission(w.aradPass, dayKey, e.Track, e.Index, e.Count)
				st, total = next, total+got
			}
			gained = total
			raw, err := aradpass.WriteState(current.State, st)
			if err != nil {
				return nil, nil, err
			}
			receipt, err := json.Marshal(map[string]any{
				"exp": st.Exp, "daily_exp": st.DailyExp, "gained": total, "day": st.DailyKey,
			})
			return raw, receipt, err
		})
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	if gained == 0 {
		return nil, nil
	}
	event, _, pErr := aradPassPayloads(w.aradPass, w.role)
	if pErr != nil {
		return nil, pErr
	}
	return []outboundPacket{{"arad_pass_event_info_restored", 0, protocol.AradPassEventInfoOpcode, event}}, nil
}

// recordAradPassDungeonClear 把一次通关计入战令任务。
//
// 赛季轨固定记 `proper_dungeon_clear`（任意正经通关）；普通轨按副本号查表，
// 见 `aradPassNormalMissionsForDungeon`（`normal_dungeon_2` / 沉月湖 / 蔚蓝号）。
// 教学图（活动 662 的教学区）不算通关。
func (w *worldSession) recordAradPassDungeonClear(ctx context.Context) []outboundPacket {
	if w == nil || w.aradPass == nil || w.activeDungeon == nil || w.isBoostGuideRun() {
		// 诊断：这条路径**静默早退过一次**（钩子挂错分支，2026-10-11），
		// 花了三轮实机才定位 ⇒ 现在把"为什么没记"直接打出来。
		if w != nil {
			log.Printf("arad pass dungeon clear skipped role=%d catalog=%v active_dungeon=%v guide=%v",
				w.role.ID, w.aradPass != nil, w.activeDungeon != nil, w.isBoostGuideRun())
		}
		return nil
	}
	dungeon := w.activeDungeon.Definition.ID
	entries := []aradPassMissionEntry{{Track: aradpass.MissionTrackSeason, Index: aradPassMissionSeasonDungeonClear, Count: 1}}
	for _, index := range aradPassNormalMissionsForDungeon(w.aradPass, dungeon) {
		entries = append(entries, aradPassMissionEntry{Track: aradpass.MissionTrackNormal, Index: index, Count: 1})
	}
	plan, err := w.recordAradPassMissions(ctx, fmt.Sprintf("arad-pass-mission:clear:%s", w.activeDungeon.RunID), entries)
	if err != nil {
		log.Printf("arad pass dungeon mission role=%d dungeon=%d: %v", w.role.ID, dungeon, err)
		return nil
	}
	indices := make([]int, 0, len(entries))
	for _, entry := range entries {
		indices = append(indices, entry.Index)
	}
	log.Printf("arad pass dungeon clear recorded role=%d dungeon=%d entries=%v frames=%d",
		w.role.ID, dungeon, indices, len(plan))
	return plan
}
