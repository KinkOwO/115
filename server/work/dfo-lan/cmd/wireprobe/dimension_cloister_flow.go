package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
)

// 次元回廊（Dimension Cloister，频道 Type 84「Hall of Dimensions」/ 内容号 0x0d）
//
// ★ 官服流程（2026-10-09 抓包 session_s30 + 业主 2026-10-10 实机多轮校准）：
//
//	① 客户端 CMD2043「开始作战」
//	   → 3-2-1 倒计时 → 横幅（第X界 - …）+ 右上角 UI
//	② 点右上角 UI → 客户端发 **CMD2080**（MYRES_DIMENSION_CLOISTER_OPERATION_SELECT）
//	   → 服务端回**同族操作窗应答**（见下）
//	③ 客户端 CMD2045（**带着选中的关卡号**）→ 服务端这才发进图帧列（N28 = 该关副本号）
//	④ 击杀 BOSS → 横幅「第X界 …记忆阅览完毕」→ 翻牌 → 返回城镇
//	⑤ **每一界都是独立的一次 BOSS 战**（不是连战）：回集结区自己再挑一界
//	⑥ 最后一关（第0界 - 崩坏覆亡的世界）击杀 → 放视频 → 横幅 + UI 关闭 → 翻牌 → 返回城镇
//
// ★ 2026-10-10 第二次转向（业主：「点击开始作战没有任何反应……最好是参考伊斯，
// 因为流程差不多」）：把这一族**完全按伊斯那条已验证的路重排**。
//
// 伊斯（`ispins_flow.go`）的开战链是「先推权威状态帧、再回 ACK」，顺序在官服 s4
// 抓包里是 N2254 → N2255(initial) → ACK2043 →（约 2.7 秒后）N2255(waiting)。
// 次元回廊在官服 s30 抓包里是**同一个形状**：
//
//	#752 N2254 入场账本 → #754 N2314 @3=01 → #755 ACK2043
//	（倒计时结束）#758 N2314 @3=06 → 客户端画面才出现右上角 UI
//
// 本仓此前是「只回 ACK，2.2 秒后补两帧 N2314」，等于把伊斯那条链的头尾都砍了：
// 开战应答之前没有账本、没有横幅初态。业主实机 trace 里客户端确实起了横幅
// （`[RaidTitleDrawer::setState]0/1/2`），但几秒后 `exit=0xC0000005` —— N2314 是
// **权威状态帧**，只发其中两帧、其余按语义省略，客户端下面要读的字段就是空的。
//
// 所以本文件的口径：
//
//  1. N2314 一律**整帧照抄官服原文**（`dimension_cloister_info.generated.go`），
//     服务端只决定「该发哪一帧」，不重建、不省略字节；
//  2. 顺序照官服：账本 + 横幅初态 + ACK 一起发，UI 态留到倒计时结束后再推；
//  3. 每一次「开窗」（客户端点右上角 UI / 清关之后）都补上本界进度态，
//     与官服在 2080 之前推的那两帧一致。
//
// 2026-10-10 项目 opcode 表里这一族的名字（**不要改名**）：
//
//	cmd 2080  MYRES_DIMENSION_CLOISTER_OPERATION_SELECT   难度/关卡选择请求
//	cmd 2081  MYRES_DIMENSION_CLOISTER_OPERATION_CLEAR    关窗
//	noti 2314 MYRES_DIMENSION_CLOISTER_INFO               横幅/UI 权威状态
//	noti 2254 LEGION_ENTRY_CHARAC_INFO                    入场账本（军团家族共用）
const dimCloisterChannelType uint32 = legion.DimCloisterChannelType

// dimCloisterPendingEvent 是次元回廊的一个待发事件（有序队列，见 legionSession）。
type dimCloisterPendingEvent struct {
	at      time.Time
	kind    string
	packets []outboundPacket
	events  []map[string]any
}

// dimCloisterWindowDelay 是「开始作战 → 下发右上角 UI 状态帧（@3=06）」的间隔。
//
// ★ 2026-10-10 15:0x 实机修正：这个值必须是 **2.4 秒**（= 客户端本地 3-2-1 倒计时的长度）。
//
// 官服 s30：#754 N2314 @3=01 + #755 ACK2043 → #758 N2314 @3=06（右上角 UI）之间的
// 字节偏移只隔了 0x100（一帧的工夫）。业主实机口径也一致：
// **倒计时结束时横幅与右上角 UI 应该同时出现**。
//
// 之前为了让「点 UI 就崩」那段有多一点准备时间，这里被推到 12 秒；崩溃根因查明后
// （见 docs/protocol/dimension-cloister-memory-records-20261010.md）副作用就露出来了：
// 业主 2026-10-10 15:0x 实测「横幅先出现、右上角 UI 等好几秒才出现」。
// 现在按官服口径收回 2.4 秒。
const dimCloisterWindowDelay = 2400 * time.Millisecond

// dimCloisterIdleCloseDelay 是「开窗帧发出去之后，玩家一直不点」的兜底等待时长。
//
// 客户端那扇难度窗自带 25 秒倒计时（官服截图口径），所以兜底必须**晚于**它，
// 否则服务端的收尾帧会在玩家还在挑的时候插进来。取 30 秒。
const dimCloisterIdleCloseDelay = 30 * time.Second

// dimCloisterLeaveStateName 是「收尾/离开」那一帧在生成文件里的名字（@3=05）。
const dimCloisterLeaveStateName = "dungeon2"

// dimCloisterMemoryRecordFrom / To 是 N2314 正文里「本角色已选记忆」那 4 条记录的跨度
// （每条 10B：u16 记忆文本 id + u16 参数 + u16 数值 + u16 槽位 + u16 保留）。
//
// ★ 2026-10-10 定位（本文件的第二次转向）：这 4 条记录**不是内容常量，而是账号自己的状态**。
// 同一份 s30 抓包里它们不只随进度变（(1,4,7) → (1,3,7) → (1,3,6,9)），
// 同一进度下还会变（同一次抓包的 #379 是 (1,3,8)、#758 是 (1,4,7)）——
// 也就是**玩家当时选好的「记忆」（105LvAbility 能力件）**。
//
// 本仓此前逐字节回放抓包，等于把**别人账号的记忆**塞给客户端：
// `DimensionCloisterMainInfoWindow::updateControl` 会拿记录里的 id 去查自己的记忆文本表
// （客户端 `sub_1479F8A60` → 管理器 +672 的 map），查到的结构里带着没被写过的字符串；
// 窗口构造接着调那条记录的文本 getter（`sub_142092C60` → 虚表 +0x58），
// 里面 `_stdio_common_vswprintf` → `wcsnlen` 去量一个坏指针 —— 实机表现正是
// 「点右上角 UI 先卡死后 0xC0000005」，且客户端一个包都不发（卡在开窗构造里）。
//
// 本仓还没有记忆系统，所以对任何角色一律发「一件记忆都没选」的形态（该区全 0），
// 与官服 @3=01（集结区倒计时那一帧）逐字节一致 —— 那正是玩家还没选记忆时的窗口初态。
const (
	dimCloisterMemoryRecordFrom = 63
	dimCloisterMemoryRecordTo   = 103
)

// dimCloisterMemoryRecords 返回这条连接此刻该发的记忆组（诊断入口优先，否则按已清界数）。
func (s *legionSession) dimCloisterMemoryRecords() []dimCloisterMemoryRecord {
	return dimCloisterMemoryRecordsFor(s.dimCloisterCleared)
}

// dimCloisterMemoryRecord 是一条 10 字节记忆记录，字段语义由客户端数据
// `Contents/2022/DimensionCloister/Etc/MyresDimensionCloister.etc` 的
// `[operation data set]` 定死（官服三组记录逐字节对照得到）：
//
//	@0 u16 操作号 [index] 1..9
//	@2 u16 [type fixed value]（总表里没写这项的操作就是 0）
//	@4 u16 [random range value] 掷出的值（min..max 步进 step 里取一个）
//	@6 u16 槽位 1..4
//	@8 u16 保留
type dimCloisterMemoryRecord struct {
	OpIndex uint16 // [index] 1..9
	Fixed   uint16 // [type fixed value]
	Value   uint16 // [random range value] 掷出的值
	Slot    uint16 // 槽位 1..4
}

// dimCloisterMemoryRecordsEnvVar 是**诊断入口**（只用于复现与取证，不是玩法开关）。
//
// DFO_CLOISTER_MEMORY_RECORDS="操作号,固定值,掷出值,槽位;…"（最多 4 条，槽位 1..4）。
// 注意：一键启动器那层不继承环境变量，测试时必须从 shell 里起（见 docs §6.3）。
// 记忆系统落地、记录改成按角色状态生成之后，这个入口应当删除。
const dimCloisterMemoryRecordsEnvVar = "DFO_CLOISTER_MEMORY_RECORDS"

func dimCloisterMemoryRecordsFromEnv() []dimCloisterMemoryRecord {
	raw := strings.TrimSpace(os.Getenv(dimCloisterMemoryRecordsEnvVar))
	if raw == "" {
		return nil
	}
	out := make([]dimCloisterMemoryRecord, 0, 4)
	for _, item := range strings.Split(raw, ";") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		fields := strings.Split(item, ",")
		if len(fields) != 4 {
			continue
		}
		var vals [4]uint16
		ok := true
		for i, f := range fields {
			v, err := strconv.ParseUint(strings.TrimSpace(f), 10, 16)
			if err != nil {
				ok = false
				break
			}
			vals[i] = uint16(v)
		}
		if !ok || vals[3] == 0 || vals[3] > 4 {
			continue
		}
		out = append(out, dimCloisterMemoryRecord{OpIndex: vals[0], Fixed: vals[1], Value: vals[2], Slot: vals[3]})
		if len(out) == 4 {
			break
		}
	}
	return out
}

// dimCloisterApplyMemoryRecords 把 records 写进 N2314 正文 @63..102（未给的槽位清零）。
func dimCloisterApplyMemoryRecords(body []byte, records []dimCloisterMemoryRecord) []byte {
	out := append([]byte(nil), body...)
	for i := dimCloisterMemoryRecordFrom; i < dimCloisterMemoryRecordTo && i < len(out); i++ {
		out[i] = 0
	}
	for _, rec := range records {
		if rec.Slot == 0 || rec.Slot > 4 {
			continue
		}
		base := dimCloisterMemoryRecordFrom + 10*int(rec.Slot-1)
		if base+10 > len(out) {
			continue
		}
		binary.LittleEndian.PutUint16(out[base:], rec.OpIndex)
		binary.LittleEndian.PutUint16(out[base+2:], rec.Fixed)
		binary.LittleEndian.PutUint16(out[base+4:], rec.Value)
		binary.LittleEndian.PutUint16(out[base+6:], rec.Slot)
	}
	return out
}

// dimCloisterProgressionMemories 返回「已清 cleared 界」时该角色身上那组记忆。
//
// ★ 数据来源：官服 s30 同一条会话里三类窗口帧的 @63..102（逐帧原文见
// docs/protocol/dimension-cloister-memory-records-20261010.md §3/§6/§6.1）：
//
//	开第 1 界前 (#758): (1,0,37,1,0) (4,0,40,2,0) (7,0,150,3,0) (0,…)
//	清完第 1 界 (#833): (1,0,37,1,0) (3,0,35,2,0) (7,0,150,3,0) (0,…)
//	清完第 2 界 (#910): (1,0,37,1,0) (3,0,40,2,0) (6,0,4,3,0) (9,30,400,4,0)
//
// 每组里的操作号/固定值/掷出值都能在客户端数据
// `Contents/2022/DimensionCloister/Etc/MyresDimensionCloister.etc` 里对上：
//
//	op1  range 37 37    → 37      op3  range 30 40 step5 → 35 / 40
//	op4  range 30 40 step5 → 40   op6  range 4 4      → 4
//	op7  range 150 150  → 150     op9  fixed 30, range 400 400 → (30,400)
//
// 也就是「记忆随清界数变多」——这是官服自己的口径，不是本仓发明的规则。
//
// 跨周是否保留、第一次进本要不要先白送一组 —— 属于业主经营口径，等拍板后再改；
// 现在按"本次会话已清界数"给，等于把官服那三组原样接上。
func dimCloisterProgressionMemories(cleared int) []dimCloisterMemoryRecord {
	sets := [][][4]uint16{
		{{1, 0, 37, 1}, {4, 0, 40, 2}, {7, 0, 150, 3}},
		{{1, 0, 37, 1}, {3, 0, 35, 2}, {7, 0, 150, 3}},
		{{1, 0, 37, 1}, {3, 0, 40, 2}, {6, 0, 4, 3}, {9, 30, 400, 4}},
	}
	if cleared < 0 {
		cleared = 0
	}
	if cleared >= len(sets) {
		cleared = len(sets) - 1
	}
	out := make([]dimCloisterMemoryRecord, 0, len(sets[cleared]))
	for _, v := range sets[cleared] {
		out = append(out, dimCloisterMemoryRecord{OpIndex: v[0], Fixed: v[1], Value: v[2], Slot: v[3]})
	}
	return out
}

// dimCloisterPartyMemorySlots 是记忆窗口要渲染的槽位数 = **队伍成员数**。
//
// ★ 2026-10-10 第五轮实机证据（会话 ..._20261010_150519_619864，client.log exit=0xC0000005）：
// 客户端在崩之前打出的最后一行是 `[ETC] party member count : 1`，而官服那份记录
// 是三名队员的（槽位 1/2/3）⇒ 窗口按队员渲染记忆，第 2/3 个槽位查不到成员，
// 文本构造里拿到空指针就崩（与 docs §1 的调用链一致）。
// 官服抓包里槽位数随队伍长：第 1/2 界 3 条、第 3 界 4 条，正好是队伍人数。
//
// 本仓是单机（单人队伍），所以恒为 1；以后做多人组队时改成真实成员数。
const dimCloisterPartyMemorySlots = 1

// dimCloisterPartyMemoryRecords 按队伍成员数截断一组记忆：窗口是按队员渲染的，
// 只该给"队里真有的人"发槽位（官服槽位数随队伍长：3 → 3 → 4）。
//
// ⚠️ 现在**不是默认口径**：客户端数据缺口没补之前，任何非零记录都会崩
// （见 dimCloisterMemoryRecordsFor 的注释）。补上 `[string data]` 之后，
// 默认口径应当切到 `dimCloisterPartyMemoryRecords(cleared, dimCloisterPartyMemorySlots)`。
func dimCloisterPartyMemoryRecords(cleared, members int) []dimCloisterMemoryRecord {
	if members <= 0 {
		return nil
	}
	records := dimCloisterProgressionMemories(cleared)
	if len(records) > members {
		records = records[:members]
	}
	return records
}

// dimCloisterMemoryRecordsFor 是「这条连接当前该发哪组记忆」的唯一入口。
//
// ★ 2026-10-10 第五/六轮实机结论（会话 ..._150519_619864 与随后那次 1 条记录版本）：
// 官服那份 `(1,0,37,1)(4,0,40,2)(7,0,150,3)` 会崩；**只发 1 条（槽位 1）同样崩**
// （两次都是 `client.log exit=0xC0000005`，都在点右上角 UI 那一刻）⇒ 与条数/队伍人数无关。
//
// IDA 追到的崩点：operation 的文本 getter（如 type 1 的 `sub_1420A2910`）把表项里的
// 字符串拼成**格式串**再喂 `vswprintf`，格式串是脏指针 ⇒ CRT `wcsnlen` 崩。
// 而本客户端数据 `Contents/2022/DimensionCloister/Etc/MyresDimensionCloister.etc` 里
// 9 个 operation 的 `[string data]` **全是空的**（国服那份不是），解析器
// `sub_14209E7A0` 就是按这份字符串写进那张 16 位键 map 的 ⇒ **任何非零记录都会崩**。
//
// 结论：这是**客户端数据缺口**，服务端任何帧都补不上。所以在业主决定是否补客户端
// 数据之前，这里**一律发全 0**（内容可用、不崩）；官服那三组只作为诊断入口
// （DFO_CLOISTER_MEMORY_RECORDS）与文档目标保留。
func dimCloisterMemoryRecordsFor(cleared int) []dimCloisterMemoryRecord {
	if override := dimCloisterMemoryRecordsFromEnv(); override != nil {
		return override
	}
	_ = cleared
	return nil
}

// dimCloisterInfoPackets 返回按名字取出的 N2314 状态帧（整帧官服原文，
// 只把那 4 条记忆记录换成本角色当前的形态）。
func dimCloisterInfoPackets(records []dimCloisterMemoryRecord, names ...string) ([]outboundPacket, error) {
	out := make([]outboundPacket, 0, len(names))
	for _, name := range names {
		body, err := legion.DimCloisterInfoBody(name)
		if err != nil {
			return nil, err
		}
		out = append(out, outboundPacket{
			Name: "dim_cloister_info_" + name, Kind: 0, ID: legion.NotiDimCloisterInfo,
			Payload: dimCloisterApplyMemoryRecords(body, records),
		})
	}
	return out, nil
}

// dimCloisterProgressPackets 返回「本场已打到第 stage 界」那一组状态帧：
// 本界记录（dungeon0/1/2）+ 开窗态（@3=06），顺序与官服 #897 → #910 一致。
func dimCloisterProgressPackets(records []dimCloisterMemoryRecord, stage int) ([]outboundPacket, string, error) {
	body, name, err := legion.DimCloisterDungeonInfoBody(stage)
	if err != nil {
		return nil, "", err
	}
	options, err := legion.DimCloisterInfoBody(legion.DimCloisterInfoHallOptions)
	if err != nil {
		return nil, "", err
	}
	return []outboundPacket{
		{Name: "dim_cloister_info_" + name, Kind: 0, ID: legion.NotiDimCloisterInfo,
			Payload: dimCloisterApplyMemoryRecords(body, records)},
		{Name: "dim_cloister_info_" + legion.DimCloisterInfoHallOptions, Kind: 0, ID: legion.NotiDimCloisterInfo,
			Payload: dimCloisterApplyMemoryRecords(options, records)},
	}, name, nil
}

// dimCloisterOperationCandidatePackets 返回按已清界数取的那一组难度帧。
//
// ★ 2026-10-10 第七轮改造（业主指路「伊斯/苏醒之森/维纳斯/末世录都有难度选择框，可以参考」）：
// 这两帧不是「两条候选项」，而是**同族的 A/B 应答**（本仓 `IspinsOperationAckA/B` 是现成可用实现，
// 形状逐字节对得上）：
//
//	A 帧：开窗（@5..6 = ffff「还没选」），@12..15 = LE unix 秒（同族口径）
//	B 帧：已选（@5 = 选择值；第 1/2 界 7，第 3 界 9）
//
// 官服这两帧相隔 3.7 秒（#771 22:47:28.354 → #773 22:47:32.096），即"先开窗、随后确认"。
// 本仓原先两帧挤在同一批发出，于是业主看到「难度框一闪而过、被自动选了难度」。
// 现在：第一次请求只发 A（把窗口真正交给玩家，服务端兜底关窗），
// 玩家选完（客户端第二次请求）才发 B。
func dimCloisterOperationOpenPackets(cleared int) []outboundPacket {
	return []outboundPacket{{
		Name: "dim_cloister_operation_open_ack", Kind: 1, ID: legion.CmdDimCloisterOperationSelect,
		Payload: legion.DimCloisterOperationOpenFrame(cleared, uint32(time.Now().Unix())),
	}}
}

func dimCloisterOperationChoicePackets(cleared int) []outboundPacket {
	return []outboundPacket{{
		Name: "dim_cloister_operation_choice_ack", Kind: 1, ID: legion.CmdDimCloisterOperationSelect,
		Payload: legion.DimCloisterOperationChoiceFrame(cleared),
	}}
}

// dimCloisterQueueInfo 把一批状态帧排进待发队列（延迟 after 到期后由精确定时器下发）。
func (s *legionSession) dimCloisterQueueInfo(after time.Duration, kind string, packets []outboundPacket, extra map[string]any) {
	note := map[string]any{
		"kind":     kind,
		"frames":   len(packets),
		"delay_ms": after.Milliseconds(),
	}
	for k, v := range extra {
		note[k] = v
	}
	s.dimCloisterEvents = append(s.dimCloisterEvents, dimCloisterPendingEvent{
		at:      time.Now().Add(after),
		kind:    kind,
		packets: packets,
		events:  []map[string]any{note},
	})
}

// dimCloisterNextEventDue 返回待发队列里**最近**一条事件的到期时刻（空队列返回零值）。
//
// 用途：dispatch 那层以前是拿一个固定时长去唤醒精确定时器（"按窗口长度 30 秒"），
// 结果排在 3.7 秒的"自动确认难度"要等到 30 秒才轮询到 —— 业主实机就是
// 「难度选择界面要等很久才会自动选择」。改成按**真正最近的到期时刻**唤醒。
func (s *legionSession) dimCloisterNextEventDue() time.Time {
	if s == nil {
		return time.Time{}
	}
	var earliest time.Time
	for _, ev := range s.dimCloisterEvents {
		if earliest.IsZero() || ev.at.Before(earliest) {
			earliest = ev.at
		}
	}
	return earliest
}

// dimCloisterEventsDue 由 connectionSession.cloisterWindow 的精确定时器驱动：
// 把到期的待发事件按队列顺序下发（主循环独占 worldSession，无并发）。
func (client *gameConnection) dimCloisterEventsDue(now time.Time) ([]outboundPacket, []map[string]any) {
	if client == nil || client.worldState == nil {
		return nil, nil
	}
	s := &client.legionState
	if len(s.dimCloisterEvents) == 0 {
		return nil, nil
	}
	var packets []outboundPacket
	var events []map[string]any
	rest := s.dimCloisterEvents[:0]
	for _, ev := range s.dimCloisterEvents {
		if now.Before(ev.at) {
			rest = append(rest, ev)
			continue
		}
		if client.worldState.activeDungeon != nil {
			events = append(events, map[string]any{
				"kind": "dim_cloister_event_aborted", "event": ev.kind, "reason": "dungeon already active",
			})
			continue
		}
		packets = append(packets, ev.packets...)
		events = append(events, ev.events...)
		// 自动确认（B 帧）之后就不该再有关窗兜底了：窗口已经按官服节奏确认过。
		if ev.kind == dimCloisterAutoChoiceKind {
			keep := rest[:0]
			for _, pend := range rest {
				if pend.kind == dimCloisterTimeoutCloseKind {
					continue
				}
				keep = append(keep, pend)
			}
			rest = keep
		}
	}
	s.dimCloisterEvents = rest
	return packets, events
}

// isDimCloister reports whether this connection speaks the 次元回廊 flow.
//
// 判据是**内容身份**（队伍类型 / 内容号 0x0d）**或实机那一行频道**：
// 客户端 clientchannelinfo 里 50（Evildom）与 84（Hall of Dimensions）两行都指向
// 本内容，实机走的是 84（2026-10-10 事件 `apocalypse_party_probe channel_type=84`）。
//
// ★ 为什么 84 也要在里面：登录期推入场账本（`ispinsPostSelection`）时客户端**还没
// 建队**、也还没发过任何本内容的包 —— 那一刻唯一能回答「这条连接是不是次元回廊」
// 的就是频道号。漏了它就会把伊斯那一族的 N2254 发给次元回廊频道。
func (s *legionSession) isDimCloister() bool {
	return s.channelType == dimCloisterChannelType ||
		s.channelType == legion.DimCloisterHallChannelType ||
		s.dimCloisterPartyActive
}

// isDimCloisterChannel 报告这条世界连接是不是次元回廊频道。
//
// 给**不依赖军团会话**的调用点用（例如 `monsterDeath` 里判断"军团 BOSS 不许
// roll 地面掉落"那一处）：那时只认频道号，与 `isDimCloister()` 的第一项同口径。
// 军团会话可能还没建（还没建队），所以不能写成 `w.legion != nil && w.legion.isDimCloister()`。
func (w *worldSession) isDimCloisterChannel() bool {
	if w == nil {
		return false
	}
	if w.channelType == dimCloisterChannelType || w.channelType == legion.DimCloisterHallChannelType {
		return true
	}
	return w.legion != nil && w.legion.dimCloisterPartyActive
}

// handleDimCloister routes the legion family for 次元回廊（家族派发层那一份）。
func (s *legionSession) handleDimCloister(w *worldSession, p []byte, id uint16) (legionResult, bool, error) {
	if !s.isDimCloister() {
		return legionResult{}, false, nil
	}
	switch id {
	case legion.CmdStart:
		result, _, err := s.dimCloisterStart(w, p)
		return result, true, err
	case legion.CmdEnterDungeon:
		result, err := s.dimCloisterLoadStage(w, p)
		return result, true, err
	case legion.CmdRewardEnd:
		result, err := s.dimCloisterRewardEnd(w, p)
		return result, true, err
	default:
		return legionResult{}, false, nil
	}
}

// dimCloisterCreateParty 处理次元回廊待机区的 CMD12（建队）。
//
// 走**与伊斯/森林完全相同的那条路**：CMD12 → [N2 队长资料, N2 队长详细资料, N9]。
// 应答模板是 2.38.2 原生语法（legionStandbyPartyReply），只换队伍类型 0x0d。
func (s *legionSession) dimCloisterCreateParty(w *worldSession, p []byte) (legionResult, error) {
	if w == nil || w.role.ID == 0 {
		return legionResult{}, fmt.Errorf("次元回廊建队需要已选定角色")
	}
	req, err := legion.DecodeEvildomParty(p)
	if err != nil {
		return legionResult{}, err
	}
	if w.characters == nil {
		return legionResult{}, fmt.Errorf("次元回廊建队：角色服务不可用")
	}
	party, err := protocol.EvildomPartyReply(req.NameReplyBytes, w.role.WireID, w.characters.ChannelContext, byte(req.Capacity))
	if err != nil {
		return legionResult{}, err
	}
	basic, err := w.characters.EntryBasicProbe(w.role, w.characters.ChannelContext)
	if err != nil {
		return legionResult{}, err
	}
	detail, err := w.characters.EntryAddition(w.role)
	if err != nil {
		return legionResult{}, err
	}
	w.soloPartyReady = true
	s.dimCloisterPartyName = req.Name
	s.dimCloisterPartyNameReply = append([]byte(nil), req.NameReplyBytes...)
	s.dimCloisterPartyActive = true
	// 业主 2026-10-11 定调：**恢复随机关卡**（抽定本场三关的顺序，见 dimCloisterRollStages）。
	s.dimCloisterRollStages()
	return legionResult{
		Packets: []outboundPacket{
			{Name: "dim_cloister_leader_basic", Kind: 0, ID: 2, Payload: basic},
			{Name: "dim_cloister_leader_detail", Kind: 0, ID: 2, Payload: detail},
			{Name: "dim_cloister_party_created", Kind: 0, ID: 9, Payload: party},
		},
		Events: []map[string]any{{
			"kind":           "dim_cloister_party_created",
			"character_id":   w.role.ID,
			"party_name":     req.Name,
			"party_type":     req.PartyType,
			"mode":           req.Mode,
			"capacity":       req.Capacity,
			"name_reply_hex": fmt.Sprintf("%x", req.NameReplyBytes),
			"reply_bytes":    len(party),
		}},
	}, nil
}

// dimCloisterLeaveParty 处理待机区的 CMD13（离队）。
func (s *legionSession) dimCloisterLeaveParty(w *worldSession, p []byte) (legionResult, error) {
	if w == nil || w.role.ID == 0 {
		return legionResult{}, fmt.Errorf("次元回廊离队需要已选定角色")
	}
	if len(p) != 0 && len(p) != 8 {
		return legionResult{}, fmt.Errorf("次元回廊离队请求长度 %d 无效", len(p))
	}
	for _, b := range p {
		if b != 0 {
			return legionResult{}, fmt.Errorf("次元回廊离队请求带非零选项")
		}
	}
	if w.activeDungeon != nil {
		return legionResult{}, fmt.Errorf("请先打本关或回城再解散次元回廊队伍")
	}
	w.soloPartyReady = false
	s.dimCloisterPartyName = ""
	s.dimCloisterPartyActive = false
	s.dimCloisterStage = -1
	s.dimCloisterCleared = 0
	var ctx [2]byte
	if w.characters != nil {
		ctx = w.characters.ChannelContext
	}
	return legionResult{
		Packets: []outboundPacket{{
			Name:    "dim_cloister_party_gone",
			Kind:    0,
			ID:      9,
			Payload: protocol.BlackPurgatoryPartyGone(ctx),
		}},
		Events: []map[string]any{{"kind": "dim_cloister_party_left", "character_id": w.role.ID}},
	}, nil
}

// dimCloisterLedgerMarks 是 N2254 内容行旗标的四种**官服实测形态**。
//
// 官服 s30 九帧 N2254 的差异只落在这四个字节上（行首 65/66/67/69 分别对应内容
// 101/102/103/105，标在各自行的 +5 处）；把九帧按 (21,45,69,117) 归并，只出现
// 这三种组合 —— 所以本仓也只发这三种，不发明新组合：
//
//	#246                    (7f, 7f, 00, 7f)   ← 登录/进场那一刻
//	#369 #752               (00, 7f, 00, 00)   ← 开战时刻（本仓 CMD2043 用这一组）
//	#434                    (00, 00, 7f, 00)
//	#822 #895 #979 #1027    (00, 00, 00, 00)   ← 内容内/已用尽
type dimCloisterLedgerMarks [4]byte

var (
	// dimCloisterLedgerLogin 是 #246 的形态（登录/进场）。
	dimCloisterLedgerLogin = dimCloisterLedgerMarks{0x7f, 0x7f, 0x00, 0x7f}
	// dimCloisterLedgerStart 是 #752 的形态（客户端点开始作战那一刻）。
	dimCloisterLedgerStart = dimCloisterLedgerMarks{0x00, 0x7f, 0x00, 0x00}
)

// dimCloisterEntryLedgerAt 返回次元回廊的 N2254 入场账本：
// 官服 #752 模板（272B）+ 指定的内容行旗标 + 当前时间戳（@256..260）。
func dimCloisterEntryLedgerAt(nowMillis uint64, marks dimCloisterLedgerMarks) ([]byte, error) {
	body, err := legion.DimCloisterEntryLedgerBody(nowMillis)
	if err != nil {
		return nil, err
	}
	for i, at := range []int{21, 45, 69, 117} {
		if at < len(body) {
			body[at] = marks[i]
		}
	}
	return body, nil
}

// dimCloisterLoginLedger 回答「这条连接登录期该不该发次元回廊的入场账本」，
// 该发时返回官服 #246 那一组旗标的 272B 账本。第二个返回值是「该不该发」。
//
// 判据只能是**频道号**：登录期客户端还没建队、也还没发过任何本内容的包。
// 实机 2026-10-10 实证：没有这条分支时，次元回廊频道收到的是伊斯那一族的 N2254
// （`ispins_login_entry_character_info_sent` 出现了两次），而客户端的难度窗会读
// 那几行旗标 —— 这是「点开右上角 UI 就 exit=0xC0000005」的头号嫌疑。
func dimCloisterLoginLedger(channelType uint32, nowMillis uint64) ([]byte, bool, error) {
	if channelType != legion.DimCloisterHallChannelType && channelType != legion.DimCloisterChannelType {
		return nil, false, nil
	}
	body, err := dimCloisterEntryLedgerAt(nowMillis, dimCloisterLedgerLogin)
	if err != nil {
		return nil, true, err
	}
	return body, true, nil
}

// dimCloisterPartyRefreshPacket 构造「队伍稳态刷新」（NOTI9），发在 @3=06 之后。
//
// 官服 s30 在 #758（@3=06）紧跟一帧 #759 N9；那份 192B（正文 176B）里除了队伍信息，
// 还带着 @48 起 7 项 `01/07` 阶梯与 @64 起 `3c 00 00 00` = 60 之类的**操作/难度列表**
// 数据。客户端的难度窗很可能要读它 —— 这是本仓与官服序列剩下的唯一结构差异。
//
// 所以默认发**官服原文**（只把队名换成本场的值，容量/队伍类型/模式都是本义的 0x0d）；
// `DFO_CLOISTER_NATIVE_N9=1` 时退回本客户端原生模板（建队那条已验证的 128B 形态），
// 用来做 A/B。走环境变量而不是 profile，是为了不动 launcher 的 profile 白名单。
func (s *legionSession) dimCloisterPartyRefreshPacket(w *worldSession) (outboundPacket, error) {
	if w == nil || w.characters == nil {
		return outboundPacket{}, fmt.Errorf("次元回廊队伍刷新：角色服务不可用")
	}
	if strings.TrimSpace(os.Getenv("DFO_CLOISTER_NATIVE_N9")) == "1" {
		return s.dimCloisterNativePartyPacket(w)
	}
	body, err := legion.DimCloisterOfficialPartyInfo(s.dimCloisterPartyName)
	if err != nil {
		return outboundPacket{}, err
	}
	return outboundPacket{Name: "dim_cloister_party_refresh", Kind: 0, ID: 9, Payload: body}, nil
}

// dimCloisterNativePartyPacket 是本客户端原生形态的队伍稳态（128B 帧），
// 与建队那次同一个模板、同一份名长字节 —— 只用于 `DFO_CLOISTER_NATIVE_N9=1` 的对照。
func (s *legionSession) dimCloisterNativePartyPacket(w *worldSession) (outboundPacket, error) {
	if len(s.dimCloisterPartyNameReply) == 0 {
		return outboundPacket{}, fmt.Errorf("次元回廊队伍刷新：还没有建队请求的名长字节")
	}
	body, err := protocol.EvildomPartyReply(s.dimCloisterPartyNameReply, w.role.WireID, w.characters.ChannelContext, 4)
	if err != nil {
		return outboundPacket{}, err
	}
	return outboundPacket{Name: "dim_cloister_party_refresh_native", Kind: 0, ID: 9, Payload: body}, nil
}

// dimCloisterStart 处理 CMD2043（开始作战）。
//
// 顺序照官服 s30 #752 → #754 → #755，也就是伊斯那条已验证的「先状态、后 ACK」：
//
//	[N2254 入场账本, N2314 @3=01 横幅初态, ACK2043(16B)]
//
// 右上角 UI 的 @3=06 由精确定时器在倒计时结束后补推（返回值 windowFrames 是排了
// 几帧，供派发层起定时器）。**必须推**：2026-10-10 第三次实机证明不推时 UI 永远不出现。
func (s *legionSession) dimCloisterStart(w *worldSession, p []byte) (legionResult, int, error) {
	if w == nil || w.role.ID == 0 {
		return legionResult{}, 0, fmt.Errorf("次元回廊开始作战需要已选定角色")
	}
	if w.activeDungeon != nil {
		return legionResult{}, 0, fmt.Errorf("次元回廊开始作战时已有副本会话在跑（先打完本关或撤离）")
	}
	req, err := legion.DecodeDimCloisterStart(p)
	if err != nil {
		return legionResult{}, 0, err
	}
	ledger, err := dimCloisterEntryLedgerAt(uint64(time.Now().UnixMilli()), dimCloisterLedgerStart)
	if err != nil {
		return legionResult{}, 0, err
	}
	banner, err := legion.DimCloisterInfoBody(legion.DimCloisterInfoHallInitial)
	if err != nil {
		return legionResult{}, 0, err
	}
	banner = dimCloisterApplyMemoryRecords(banner, s.dimCloisterMemoryRecords())
	options, err := dimCloisterInfoPackets(s.dimCloisterMemoryRecords(), legion.DimCloisterInfoHallOptions)
	if err != nil {
		return legionResult{}, 0, err
	}
	// 官服在 @3=06 之后紧跟一帧队伍稳态（#759 N9，192B）。那份正文里带着**操作/难度
	// 列表**（@48 起 7 项 `01/07` 阶梯、@64 起 `3c 00 00 00` = 60 之类），本客户端原生
	// 模板（128B）只承载队伍基本信息 —— 客户端的难度窗很可能要读前者，这就是「点开 UI
	// 就 exit=0xC0000005」剩下的唯一结构差异。
	//
	// 默认发官服原文（只把队名换成本场的值）；`DFO_CLOISTER_NATIVE_N9=1` 可切回原生
	// 模板做 A/B（诊断入口，走环境变量以免动 launcher 的 profile 白名单）。
	if refresh, refreshErr := s.dimCloisterPartyRefreshPacket(w); refreshErr == nil {
		options = append(options, refresh)
	}

	// 本场是新一轮：已清界数归零，上一场排队的帧一并作废。
	s.dimCloisterStage = -1
	s.dimCloisterCleared = 0
	s.dimCloisterWindowDeadline = time.Time{}
	s.dimCloisterEvents = nil

	// `DFO_CLOISTER_SKIP_WINDOW=1`：**跳过难度窗，直接进第 1 界**（诊断/兜底入口）。
	//
	// 为什么留它：本机 2.38.2 美服客户端在这个内容上「点右上角 UI」必 `exit=0xC0000005`，
	// 而服务端侧已经与两条独立抓包（官服 s30 + 低版本 110）逐字节对齐过：N2254 两种
	// 旗标形态、N2314 @3=01/@3=06 两帧原文、ACK2043、队伍稳态 N9（官服 176B 原文）、
	// 操作窗应答（40B，官服 #771 与低版本 2079 应答同形）全都对上了，客户端仍然崩在
	// 它自己的点击处理里且**一个包都不发**。
	//
	// 打开这个开关后：开始作战照常回 [账本 + 横幅 + ACK2043]，但跳过倒计时之后的
	// 开窗帧与难度窗，2.4 秒后直接下发第 1 界的进图帧列（N23 + N26…N1474），
	// 于是「击杀 BOSS → 横幅 → 翻牌 → 返回城镇 → 再挑下一界 → 最后一界放视频 + UI 关闭
	// → 翻牌 → 返回城镇」这整条循环可以正常走完，代价是没有难度选择界面。
	if os.Getenv("DFO_CLOISTER_SKIP_WINDOW") == "1" {
		plan, note, loadErr := w.dimCloisterLoadStagePlan(0, req.Party, s.dimCloisterStageDungeon(0))
		if loadErr != nil {
			return legionResult{}, 0, loadErr
		}
		note["kind"] = "dim_cloister_skip_window_stage_loaded"
		note["note"] = "DFO_CLOISTER_SKIP_WINDOW=1：跳过难度窗直接进第 1 界"
		enter := []outboundPacket{
			{Name: "dim_cloister_entry_ledger", Kind: 0, ID: legion.NotiEntryCharacterInfo, Payload: ledger},
			{Name: "dim_cloister_info_" + legion.DimCloisterInfoHallInitial, Kind: 0, ID: legion.NotiDimCloisterInfo, Payload: banner},
			{Name: "dim_cloister_start_ack", Kind: 1, ID: legion.CmdStart, Payload: legion.DimCloisterStartAck(req)},
		}
		s.dimCloisterStage = 0
		s.dimCloisterQueueInfo(2400*time.Millisecond, "dim_cloister_skip_window_enter", plan,
			map[string]any{"stage": 0, "dungeon": note["dungeon"]})
		return legionResult{Packets: enter, Events: []map[string]any{note}}, 1, nil
	}

	s.dimCloisterQueueInfo(dimCloisterWindowDelay, "dim_cloister_info_opened", options, map[string]any{
		"after_countdown": true,
	})
	// 到点（= 官服客户端自己开窗的那个时刻 + 15s）如果玩家一直没点，服务端兜底关窗。
	//
	// 用 `DimCloisterOperationClose()`（同族关窗 ACK，Action1 + close=1）而不是 N2314 的
	// @3=05：官服抓包里的 @3=05 只有 `place=02`（副本内）形态，拿来当集结区的关窗帧
	// 是**静默无效**的（2026-10-10 第 7 轮实测：推了 @3=05，UI 30 秒没消失）。
	//
	// 另一个作用是留一条可判读的时间线：不点击的那一轮里若客户端在收尾帧之前就崩，
	// 说明闪退是定时器驱动、与点击无关（第 7 轮实测结论：**不崩**，所以是点击驱动）。
	s.dimCloisterQueueInfo(dimCloisterWindowDelay+dimCloisterIdleCloseDelay,
		"dim_cloister_info_idle_closed", []outboundPacket{{
			Name:    "dim_cloister_operation_idle_close",
			Kind:    1,
			ID:      legion.CmdDimCloisterOperationSelect,
			Payload: legion.DimCloisterOperationClose(),
		}}, map[string]any{"reason": "player never opened the operation window"})

	return legionResult{
		Packets: []outboundPacket{
			{Name: "dim_cloister_entry_ledger", Kind: 0, ID: legion.NotiEntryCharacterInfo, Payload: ledger},
			{Name: "dim_cloister_info_" + legion.DimCloisterInfoHallInitial, Kind: 0, ID: legion.NotiDimCloisterInfo, Payload: banner},
			{Name: "dim_cloister_start_ack", Kind: 1, ID: legion.CmdStart, Payload: legion.DimCloisterStartAck(req)},
		},
		Events: []map[string]any{{
			"kind":          "dim_cloister_start_ack_sent",
			"character_id":  w.role.ID,
			"party":         req.Party,
			"content":       req.Content,
			"window_frames": len(options),
			"order": "N2254 entry ledger -> N2314 state01 banner -> ACK2043 (official s30 #752/#754/#755); " +
				"N2314 state06 follows after " + dimCloisterWindowDelay.String() + " to light up the top-right UI",
		}},
	}, len(options), nil
}

// dimCloisterSelect 处理 CMD2080（MYRES_DIMENSION_CLOISTER_OPERATION_SELECT，客户端点右上角 UI）。
//
// ★ 2026-10-10 15:0x 实机修正后的形状（**以本客户端自己的 build 为准**）：
//
//	官服 s30：#770 N2314 @3=02 → #771/#773 **两帧 32B 候选**（kind=1，同 opcode 2080）
//	          → #774 N1539（会员信息，与本内容无关，本仓不发）
//
// 之前这里回的是低版本（110 客户端）抓包里的 **40B「操作窗 ACK」**。那是另一个 build 的
// reader：业主 2026-10-10 15:0x 实机（抓包 20261010-135812）点「选择记忆之书」后，
// 客户端连发 5 次 CMD2080，窗口只把按钮文案换成「变更记忆」、一张难度卡都不显示 ——
// 那份 40B 的 @1=0 被当成了「序号 0 的候选」。现在按官服原文逐帧发候选
// （`internal/legion/dimension_cloister_operation.generated.go`）。
func (s *legionSession) dimCloisterSelect(w *worldSession, p []byte) (legionResult, error) {
	if w == nil || w.role.ID == 0 {
		return legionResult{}, fmt.Errorf("次元回廊选择难度需要已选定角色")
	}
	req, err := legion.DecodeDimCloisterOperation(p)
	if err != nil {
		return legionResult{}, err
	}
	// 截止值沿用同族口径记账（客户端侧不读它，但服务端到点要兜底关窗）。
	if !req.Close {
		s.dimCloisterWindowDeadline = time.Now().Add(legion.DimCloisterSelectionWindow)
	} else {
		s.dimCloisterWindowDeadline = time.Time{}
	}

	plan := make([]outboundPacket, 0, 4)
	if !req.Close {
		// 同族 A/B 口径（见 dimCloisterOperationOpenPackets 的注释）：
		// 窗口还没开 → 本界进度态 + A（开窗，把选择权交给玩家）；
		// 窗口已经开着 → 这一条就是玩家的选择 → 只回 B（已选），并撤掉兜底关窗。
		if s.dimCloisterWindowAcked {
			plan = append(plan, dimCloisterOperationChoicePackets(s.dimCloisterCleared)...)
			s.dimCloisterWindowAcked = false
			s.dimCloisterWindowDeadline = time.Time{}
			s.dimCloisterDropWindowEvents()
		} else {
			// 本界进度态：还没进过图就是集结区「已选定」态，进过图就是本界记录态。
			//
			// ★★ 2026-10-11 第二十七轮：**下标必须是"已清界数"，不是"已清界数-1"**。
			//
			// 实机日志（会话 `..._20261010_220400_862970`）：
			//
			//	22:06:04 清关 → 发 dungeon1（@11=2）→ 客户端横幅+翻牌 ✓、发了 CMD2046 ✓
			//	22:06:28 客户端回城后**打开了作战窗口**（CMD2080）
			//	         我们回的是 **dungeon0（@11=1）** ← 又把进度倒回"第 1 界已清"
			//
			// 清关那条路（`dimCloisterAcknowledgeStage`）已经按"已清界数"当下标取帧，
			// 而这里还传着 `cleared-1`（旧口径），于是回城开窗时客户端看到的记录
			// 退回上一界 ⇒ 第 2 界仍不可选。两处口径必须一致。
			if s.dimCloisterCleared > 0 {
				if progress, _, err := dimCloisterProgressPackets(s.dimCloisterMemoryRecords(), s.dimCloisterCleared); err == nil {
					plan = append(plan, progress...)
				}
			} else if picked, err := dimCloisterInfoPackets(s.dimCloisterMemoryRecords(), legion.DimCloisterInfoHallPicked); err == nil {
				plan = append(plan, picked...)
			}
			s.dimCloisterWindowAcked = true
			plan = append(plan, dimCloisterOperationOpenPackets(s.dimCloisterCleared)...)
			// 到点由同一条待发队列兜底关窗（维纳斯同款：客户端自己对截止值不做任何事）。
			// 玩家若先选好了关卡，进图后这条事件会在 dimCloisterEventsDue 里被跳过。
			s.dimCloisterQueueWindowClose()
			// 空卡兜底：照官服 s30 的 3.7 秒节奏自动按下本界选择值（B 帧），
			// 否则选择窗会一直停在那里、按钮又按不动，玩家进不了图（业主 2026-10-10 实机）。
			s.dimCloisterQueueChoice()
		}
	} else {
		// 关窗（CMD2081 / 兜底）仍走同族口径的 40B close 帧（这一支尚未在实机单独验证）。
		s.dimCloisterWindowAcked = false
		plan = append(plan, outboundPacket{
			Name: "dim_cloister_operation_ack", Kind: 1, ID: legion.CmdDimCloisterOperationSelect,
			Payload: legion.DimCloisterOperationAck(byte(req.Action), legion.DimCloisterOperationTicket),
		})
	}

	stage := "close"
	if !req.Close {
		if s.dimCloisterWindowAcked {
			stage = "open"
		} else {
			stage = "choice"
		}
	}
	return legionResult{
		Packets: plan,
		Events: []map[string]any{{
			"kind":         "dim_cloister_operation_window",
			"character_id": w.role.ID,
			"action":       req.Action,
			"close":        req.Close,
			"frames":       len(plan),
			"stage":        stage,
			"choice":       legion.DimCloisterOperationChoice(s.dimCloisterCleared),
			"request_hex":  fmt.Sprintf("%x", p),
		}},
	}, nil
}

// dimCloisterOperationClear 处理 CMD2081（MYRES_DIMENSION_CLOISTER_OPERATION_CLEAR，关窗）。
func (s *legionSession) dimCloisterOperationClear(w *worldSession, p []byte) (legionResult, error) {
	if w == nil || w.role.ID == 0 {
		return legionResult{}, fmt.Errorf("次元回廊关窗需要已选定角色")
	}
	ack := legion.DimCloisterOperationClose()
	return legionResult{
		Packets: []outboundPacket{{
			Name: "dim_cloister_operation_close_ack", Kind: 1, ID: legion.CmdDimCloisterOperationClear, Payload: ack,
		}},
		Events: []map[string]any{{
			"kind":         "dim_cloister_operation_window_closed",
			"character_id": w.role.ID,
			"request_hex":  fmt.Sprintf("%x", p),
			"ack_hex":      fmt.Sprintf("%x", ack),
		}},
	}, nil
}

// dimCloisterQueueWindowClose 在难度选择窗倒计时归 0 时推原生 close ACK 自动关窗。
//
// 与 `venusOperationClose` 同款：客户端自己对截止值不做任何事（走到 0 就停住），
// 由服务端在到期时刻下发 Action1 + close=1（原生 reader 的关闭分支判据）。
// 客户端主动路径是 CMD2081；这里是服务端兜底路径，排进同一条待发队列，
// 由 `connectionSession.cloisterWindow` 精确定时器到点唤醒主循环。
func (s *legionSession) dimCloisterQueueWindowClose() bool {
	if s == nil || s.dimCloisterWindowDeadline.IsZero() {
		return false
	}
	after := time.Until(s.dimCloisterWindowDeadline)
	if after <= 0 {
		return false
	}
	s.dimCloisterQueueInfo(after, dimCloisterTimeoutCloseKind, []outboundPacket{{
		Name:    "dim_cloister_operation_close_ack",
		Kind:    1,
		ID:      legion.CmdDimCloisterOperationSelect,
		Payload: legion.DimCloisterOperationClose(),
	}}, nil)
	return true
}

const (
	// dimCloisterTimeoutCloseKind / dimCloisterAutoChoiceKind 是队列里那两种窗口事件的 kind。
	dimCloisterTimeoutCloseKind = "dim_cloister_operation_window_timeout_closed"
	dimCloisterAutoChoiceKind   = "dim_cloister_operation_auto_choice"
	// dimCloisterChoiceDelay 是「开窗 A 帧 → 按本界选择值确认 B 帧」的窗口停留时长。
	//
	// 业主 2026-10-10 定调：**选择界面停留 3 秒**后自动按下最高难度。
	// （官服 s30 实测 3.7 秒，#771 22:47:28.354 → #773 22:47:32.096，可作参照。）
	//
	// ⚠️ 这一段是**给空卡兜底**：本客户端这条内容的卡片渲染缺客户端数据（见文档 §6.12），
	// 卡片没有内容、按钮也按不出选择值，所以只能由服务端按固定难度确认。
	dimCloisterChoiceDelay = 3 * time.Second
)

// dimCloisterQueueChoice 排「按本界选择值自动确认」那一帧（官服 #773 的节奏）。
func (s *legionSession) dimCloisterQueueChoice() bool {
	if s == nil {
		return false
	}
	choice := legion.DimCloisterOperationChoice(s.dimCloisterCleared)
	s.dimCloisterQueueInfo(dimCloisterChoiceDelay, dimCloisterAutoChoiceKind,
		dimCloisterOperationChoicePackets(s.dimCloisterCleared),
		map[string]any{"choice": choice, "cadence": "official s30 #771 → #773 = 3.7s"})
	return true
}

// dimCloisterDropWindowEvents 丢掉本界身上还没到期的窗口事件（玩家自己选了之后用）。
func (s *legionSession) dimCloisterDropWindowEvents() {
	if s == nil || len(s.dimCloisterEvents) == 0 {
		return
	}
	rest := s.dimCloisterEvents[:0]
	for _, ev := range s.dimCloisterEvents {
		if ev.kind == dimCloisterAutoChoiceKind || ev.kind == dimCloisterTimeoutCloseKind {
			continue
		}
		rest = append(rest, ev)
	}
	s.dimCloisterEvents = rest
}

// dimCloisterLoadStage 处理 CMD2045：客户端选完关卡后发来的确认帧，**带着关卡号**。
//
// 这才是加载副本的时机：按 @16 的关卡下标取副本号，发整套进图帧列（N26 … N1474），
// 并把 N28 的副本号换成该关的目标号；进图帧列之后补一帧 N2314 本界进度态，
// 这样下一轮开窗时客户端手上的记录是新的（官服在 #897 推同一帧）。
func (s *legionSession) dimCloisterLoadStage(w *worldSession, p []byte) (legionResult, error) {
	if w == nil || w.role.ID == 0 {
		return legionResult{}, fmt.Errorf("次元回廊进图需要已选定角色")
	}
	if w.activeDungeon != nil {
		return legionResult{}, fmt.Errorf("次元回廊进图时已有副本会话在跑")
	}
	party, stage, err := legion.DecodeDimCloisterStageConfirm(p)
	if err != nil {
		return legionResult{}, err
	}
	if stage >= uint32(len(legion.DimCloisterStageDungeons)) {
		return legionResult{}, fmt.Errorf("次元回廊进图关卡 %d 越界（官服抓包只有 0..%d）",
			stage, len(legion.DimCloisterStageDungeons)-1)
	}
	// ★★ 2026-10-11 第二十三轮：照**已修好的三家军团本**补上顺序语义。
	//
	// 三家（伊斯 `enterIspinsStage` / 维纳斯 / 苏醒之森）都有同一道闸门：
	//
	//	伊斯: if run.cleared[stage]              → error("already cleared")
	//	      if stage != w.ispinsClearedCount() → error("follows the official default order")
	//	森林: if run.cleared[stage]              → error("already cleared")
	//	      if stage != cleared                → error("sequential")
	//
	// 也就是客户端**只能按顺序进下一界**，不能重进已通关的界。次元回廊此前没有这道闸门
	// （进图不校验顺序），凡是"进度"相关的问题都会表现成"客户端随便报个 stage、服务端照收"。
	//
	// 先**只记日志不拒绝**：本轮目的是看清"回城后客户端到底报的是第几界"
	// （`requested_stage` 与 `cleared`）—— 这是区分"客户端仍认为第 1 界没打"
	// 与"服务端帧没推进"的唯一直接证据。看清之后下一轮再决定是否照三家硬拒。
	log.Printf("dim cloister enter request: requested_stage=%d cleared=%d stage_field=%d",
		stage, s.dimCloisterCleared, s.dimCloisterStage)
	s.dimCloisterStage = int(stage)
	s.dimCloisterParty = party
	// ★★ 2026-10-10 第二十一轮：进图时**复位开窗状态**。
	//
	// 业主实机：「回到城镇后不能选下一界」。原因是 `dimCloisterWindowAcked` 在上一界
	// 开窗时被置 true 之后一直没复位，于是回城再开窗（CMD2080）时，
	// `dimCloisterOperationSelect` 的分支判成"窗口已经开着 ⇒ 这是玩家的选择"，
	// **只回一帧"已选"（B），不回开窗列表（A）+ 本界进度态** ⇒ 客户端手上没有
	// 下一界的关卡列表，就选不了下一界。
	//
	// 开窗状态是"一次窗口会话"的量，进图即结束，所以在这里和新一轮开始时都清掉。
	s.dimCloisterWindowAcked = false
	s.dimCloisterWindowDeadline = time.Time{}

	plan, note, err := w.dimCloisterLoadStagePlan(int(stage), party, s.dimCloisterStageDungeon(int(stage)))
	if err != nil {
		return legionResult{}, err
	}
	// ★ 2026-10-10 业主实机报「进本后装备全没、打怪没伤害」。
	//
	// 这条内容按设计会切到「记忆负载」，负载为空 ⇒ 角色在副本内没有装备 ⇒ 没有伤害。
	// 别的副本进图时服务端会**重发角色穿戴快照（N13）+ 穿戴栏刷新（N14）**
	// （见 `dungeon_flow.go` 的 `dungeon_worn_equipment_restored` / `dungeon_worn_visuals_restored`），
	// 而次元回廊走的是官服抓包那套进图帧列，**从来没发过这两帧** —— 这里按同一口径补上，
	// 让客户端在副本内重新拿到该角色的真实穿戴数据。
	//
	// 这是**纯服务端**手段（不动 DFO.exe / PVF）；实机是否因此恢复攻击力，由业主验证。
	if len(w.role.State) > 0 {
		if wornSnapshot, err := inventory.WornPayload(w.role.State); err == nil && len(wornSnapshot) > 0 {
			plan = append(plan, outboundPacket{"dim_cloister_worn_equipment_restored", 0, 13, wornSnapshot})
		}
		if wornUpdate, err := inventory.WornSpaceUpdate(w.role.State); err == nil && len(wornUpdate) > 0 {
			plan = append(plan, outboundPacket{"dim_cloister_worn_visuals_restored", 0, 14, wornUpdate})
		}
	}
	// ★ 2026-10-10 业主实机：「击败 BOSS 转阶段后锁住 1 条血不死、打在它身上没有伤害数字」。
	//
	// 官服 s30 抓包里，客户端进图（CMD2045 #476）之后 0.5 秒服务端发了
	// **#804 `N31 ENABLE_CLEAR_DUNGEON`**（正文 16B：`413a0000 6f13b397 3b000000 00000000`）
	// —— 名字就是"允许通关"。这正是那条闸门：没有它，BOSS 停在最后一条血。
	// 我们的次元回廊走的是官服抓包那套帧列，**恰恰漏了这一帧**（帧列里只有 N23/N26/781/782/
	// N27/N476/N1584/N28/N629/N29/N465/N475/ACK2045/N3/N390/N37/N1474 + N2314）。
	plan = append(plan, outboundPacket{
		Name: "dim_cloister_enable_clear_dungeon", Kind: 0, ID: legion.NotiEnableClearDungeon,
		Payload: legion.DimCloisterEnableClearDungeon(),
	})
	// ★ N1474（副本时限/右上角「剩余时间」）里的 @4 是**本轮开始的 unix 秒**。
	//
	// 官服三帧实测（#793/#868/#949）：@0 = 3600（时限秒）常量，@4 = 1791557311 →
	// 1791557360 → 1791557429（随帧递增 = 当时时间），@8 = 校验串，@12 = 小计数。
	// 我们照抄抓包 ⇒ @4 停在抓包那一刻 ⇒ 客户端算 `3600 − (now − @4)` 得负数
	// ⇒ 右上角显示「剩余时间 00:00」（业主 2026-10-10 截图实证）。这里换成当前时间。
	for i := range plan {
		if plan[i].ID != legion.NotiDungeonTimeoutTime || len(plan[i].Payload) < 8 {
			continue
		}
		body := append([]byte(nil), plan[i].Payload...)
		binary.LittleEndian.PutUint32(body[4:], uint32(time.Now().Unix()))
		plan[i].Payload = body
	}
	progress, name, err := dimCloisterProgressPackets(s.dimCloisterMemoryRecords(), int(stage))
	if err != nil {
		return legionResult{}, err
	}
	plan = append(plan, progress...)
	note["progress_info"] = name
	note["frames"] = len(plan)
	return legionResult{Packets: plan, Events: []map[string]any{note}}, nil
}

// dimCloisterClearSignal 是「本界 BOSS 已死 → 可以结算」那套信号的第一帧。
//
// 官服 s30 第 1 界清关序列（客户端 CMD2045 进图之后）：
//
//	#800 N2059 RAID_BIDDING_START        ← 翻牌/结算开场（16B：01000000002e0000245eaca836000000）
//	#820 N2253 LEGION_ADDITIONAL_CLEAR_REWARD
//	#821 N2316 MYRES_DIMENSION_OPERATION_REWARD(824B)
//
// 缺了这套信号，客户端在 BOSS 消失后**不会发 CMD2046**，实机表现就是
// 「BOSS 消失了但一直不通关」（业主 2026-10-10 实机）。这里先发第一帧做验证，
// N2253/N2316 随后按抓包正文补（都取自官方 #820/#821）。
func (s *legionSession) dimCloisterClearSignal() []outboundPacket {
	return []outboundPacket{{
		Name: "dim_cloister_raid_bidding_start", Kind: 0, ID: 2059,
		Payload: []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x2e, 0x00, 0x00,
			0x24, 0x5e, 0xac, 0xa8, 0x36, 0x00, 0x00, 0x00},
	}}
}

// completeDimCloisterStage 是次元回廊的"清房 → 可以结算"投影（`completeDungeon` 里那一支）。
//
// 官服 s30 第 1 界清关时服务端发的是一整段连续帧：
//
//	#800 N2059 RAID_BIDDING_START    → 翻牌开场
//	#801 N38 / #802 N14 / #803 N279 / #804 N31 / #805..#809 N2168 / #810 N14
//	#811 N2252 LEGION_BASIC_CLEAR_REWARD（翻牌第一排）
//	#812..#814 N2168 / #815..#818 N14
//	#820 N2253 LEGION_ADDITIONAL_CLEAR_REWARD（第二排）
//	#821 N2316 MYRES_DIMENSION_CLOISTER_OPERATION_REWARD
//	#823 N9 / #824 N2314 @3=02 / #825 N1658 / #826 N2168 / #827 N115
//
// 客户端收到这套信号后会发 CMD2046，由 dimCloisterRewardEnd 推「下一界可挑」的开窗帧。
//
// ★★ 2026-10-10 业主实机四轮之后定下的三条硬结论（每一条都是实机打出来的）：
//
//  1. **编号必须对齐**（第一轮）：客户端那只 BOSS 的实体号来自我们下发的 N29，
//     会话怪物表要对齐它，否则 `CMD39` 被拒、清关链一帧都发不出去。
//  2. **N14/N2 不许回放**（第二轮）：那 6 帧 N14 与 1 帧 N2 是官服那位玩家的物品/角色数据。
//  3. **N2252 的读取需要"前面还有 0x1E5C(7772) 字节"**（第四轮，见下）。
//
// 第 3 条的取证（IDA + 四轮实机）：
//
//	客户端 N2252 handler = sub_1424FDC30（反编译见 _cloister/ida/dec/）：
//	    __int64 v6[7776];              // 栈上 7776B
//	    sub_146EA0BE0(v6, 0x1E5C);     // ← 从**共享收包游标**里复制 7772 字节
//	    … 取 XUI 对象 → 虚表 +0xF8
//
//	sub_146EA0BE0 的边界检查：
//	    cmp  cs:dword_14F1BF878, esi   ; 游标剩余
//	    jl   loc_146EA0C30             ; 剩余 < 请求 ⇒ `mov dword ptr ds:0, 0`（故意空写 ⇒ 崩）
//
//	实机四轮的崩溃栈**完全一样**（0x146EA0C30 ← 0x1424FDD06），
//	包括第一轮"逐字节回放官服整段"那一版 —— 也就是说：**客户端在这一帧上要求
//	前面还有 7772 字节可读**，而官服那位玩家当时能通过，靠的是它前面那 7 帧
//	N14/N2（192×6 + 752 = 1904 字节）带来的缓冲量。
//
// 所以现在：**跳过 N14/N2（不许回放别人的数据），但把它们占用的字节量用
// 本仓自己的帧补回去**（N2168 是本族里的无副作用辅助帧，官服自己也连发 5 帧）。
// completeDimCloisterStage 是本界清关的结算投影（`completeDungeon` 里那一支）。
//
// ★★ 2026-10-10 第五轮定稿：**照抄同族已经实机验证过的形状**，不再逐帧回放官服清关段。
//
// 业主指路：「每个修好的军团本都可以参考」。四个能用的军团本都是**同一个形状**
// （`completeIspinsStage` / `completeVenusStage` / `completeForestStage`）：
//
//  1. 先在服务端**真的把奖励发进背包**（`inventory.Awarder.Grant` + 存档 commit）；
//  2. 紧接着下发 **N14**（`protocol.InventoryUpdate(ChangedItemRows(before, after))`）——
//     「显示与发放同源」，客户端靠它当场看到奖励物品；
//  3. **N31** 通关横幅（包名必须是 `dungeon_clear_enabled`，dispatch 的发送监视器按它置
//     `completionSent`；token 与 N2252 尾 @7760 呼应）；
//  4. **N2252** 第一排翻牌（本仓构造，见 dimension_cloister_clear_reward.go）；
//  5. **N2 ×2** 角色资料（`EntryBasicProbe` + `EntryAddition`，本地重建）；
//  6. **N2253** 第二排；
//  7. **N9** 队伍稳态（`protocol.SoloPartyInfo`，本地重建）；
//  8. 本族自己的状态帧（N2314 / N2316）。
//
// 本族此前是**逐帧回放官服清关段 #800..#827**，那条路踩了五个坑（都写在各轮记录里）：
// 编号没对齐、官服玩家物品被回放、官服那两帧是 zlib、模板号抄错、
// 以及 N2252 处理器要从共享收包游标里读 7772 字节（垫不够就空写崩）。
// 改成"本仓自己构造"之后，第 2/3/4/5 条坑全部不复存在 —— 这正是同族能跑通的原因。
func (w *worldSession) completeDimCloisterStage() ([]outboundPacket, error) {
	if w == nil || w.activeDungeon == nil {
		return nil, nil
	}
	if !w.activeDungeon.Completed() || w.completionSent {
		return nil, nil
	}
	// ★★ 2026-10-10 第十八轮（业主：「换回普通翻牌，只要能正常开启下一关就行」）：
	//
	// 这一支现在**只发通关横幅 N31**，然后交回 `completeDungeon()` 的通用流程：
	//
	//	N31 横幅 → 客户端 CMD46 → `dungeonResult` 通用结算（N34/N37/N35/N261）
	//	→ 玩家点返回城镇 CMD72 → `settlementExit` 次元回廊分支 → 回集结区
	//
	// **不再发军团翻牌帧**（N2252/N2253）—— 本客户端 build 取不到本内容的翻牌窗口，
	// 发了必崩（十六轮实机一致：崩在 `sub_1424FDC30`，地址在
	// `0x146EA0C30` 读取守卫与 `0x1424FDD35` 虚表调用之间摆动）。
	//
	// 「界数推进」不走客户端：官服是客户端收到翻牌那一串后才发 CMD2046，
	// 再由 `dimCloisterRewardEnd → dimCloisterStageCleared` 推进。我们没翻牌 ⇒
	// 客户端不会发 2046 ⇒ 由服务端在 `monsterDeath` 里直接下发同一组状态帧
	// （见 `noteDimCloisterStageCleared`）。
	w.completionSent = true
	return []outboundPacket{{
		Name: "dungeon_clear_enabled", Kind: 0, ID: legion.NotiEnableClearDungeon,
		Payload: legion.DimCloisterEnableClearDungeon(),
	}}, nil
}

// dimCloisterElapsedMS 是本局的通关耗时毫秒（N2252 尾 @7760 的高 6B）。
//
// 官服 #811 那一格是 26785270（≈7.4 小时，明显是"账号级"的累计值，不是本局耗时），
// 本仓发 0（界面显示 00:00）。这里保留成函数，等业主定了口径再改。
func (w *worldSession) dimCloisterElapsedMS() uint64 { return 0 }

// legionMemoryRecords 取本连接此刻该发的记忆记录（无军团会话时给空组）。
func (w *worldSession) legionMemoryRecords() []dimCloisterMemoryRecord {
	if w == nil || w.legion == nil {
		return nil
	}
	return w.legion.dimCloisterMemoryRecords()
}

// legionStageForClear 返回本界在官服三界序列里的下标（用于挑 N2252 的阶段 token）。
func (w *worldSession) legionStageForClear() int {
	if w == nil || w.legion == nil {
		return 0
	}
	stage := w.legion.dimCloisterStage
	if stage < 0 || stage >= len(legion.DimCloisterStageDungeons) {
		return 0
	}
	return stage
}

// dimCloisterRewardEnd 处理本界 CMD2046（结算请求）。
//
// 官服 s30 的清关序列（第 1 界）：#821 N2316 奖励 → #824 N2314 @3=02 →
// #833 N2314 @3=06 → 客户端再开一次难度窗 → CMD2045 选第 2 界。也就是说清关后
// 官服**确实重推这两帧**，客户端据此开窗；本界不是最后一界时就这么走。
//
// 最后一界（打完三界）走 dimCloisterFinale：@3=03 放视频 → @3=05 关闭 UI。
// 两种情况的 ACK2046 都用家族共享的 RewardEndAck（与官服抓包同长度）。
func (s *legionSession) dimCloisterRewardEnd(w *worldSession, p []byte) (legionResult, error) {
	if w == nil || w.role.ID == 0 {
		return legionResult{}, fmt.Errorf("次元回廊结算需要已选定角色")
	}
	stage := s.dimCloisterStage
	if stage < 0 {
		stage = 0
	}
	last := stage >= len(legion.DimCloisterStageDungeons)-1

	result := legionResult{Packets: []outboundPacket{{
		Name: "dim_cloister_reward_end_ack", Kind: 1, ID: legion.CmdRewardEnd,
		// ★★ 2026-10-11 第二十六轮：**回退到 14 字节的通用 ack**。
		//
		// 第二十四轮我按官服把这里改成 32 字节带界号（`DimCloisterRewardEndAck`），
		// 之后业主连测两轮都变成"回城后不能选下一界"，**而第二十三轮（还是 14 字节 ack 时）
		// 客户端已经能发出第二次进图请求**（日志：`enter request` 出现两次）。
		//
		// 也就是说那 32 字节反而把客户端顶回去了 —— 客户端的 CMD2046 处理器只读 13 字节
		// （见 `RewardEndAckSize` 的注释与 sub_1424FDB60 的 `mov edx,965h`/13B 读取），
		// 多出来的字节很可能被它当成别的字段解析，破坏了"本界结算完成"的记账。
		//
		// 所以回到已知可用的 14 字节；要再动它，先有 IDA 层面的字段语义证据。
		Payload: legion.RewardEndAck(),
	}}}
	var err error
	if last {
		var finale legionResult
		finale, err = s.dimCloisterFinale(w)
		if err == nil {
			result.Packets = append(result.Packets, finale.Packets...)
			result.Events = append(result.Events, finale.Events...)
		}
	} else {
		var cleared legionResult
		cleared, err = s.dimCloisterStageCleared(w)
		if err == nil {
			result.Packets = append(result.Packets, cleared.Packets...)
			result.Events = append(result.Events, cleared.Events...)
		}
	}
	if err != nil {
		return result, err
	}
	return result, nil
}

// dimCloisterAcknowledgeStage 是**幂等**的「第 stage 界已清」确认。
//
// ★★ 2026-10-10 第十九轮（业主实机：横幅→翻牌→横幅→翻牌 无限循环）：
//
// 循环的成因：`dimCloisterStageCleared` 原先每收到一次 CMD2046 就重发那对 N2314
// （先把 `cleared` 加一，下次检查又通过），而客户端**收到 N2314 就会再开一次
// 翻牌界面并再发 CMD2046** ⇒ 无限循环（实机日志里
// `stage_cleared(2046) → reward_end_ack → info_dungeon0/hallOptions` 重复了几十次）。
//
// 现在：**同一界只确认一次**（`dimCloisterCleared` 已经超过它就不再发帧）。
// 第二次以后的 CMD2046 只回一个 ack，让客户端收尾。
func (s *legionSession) dimCloisterAcknowledgeStage(w *worldSession, stage int) legionResult {
	if stage < 0 {
		stage = 0
	}
	if stage < s.dimCloisterCleared {
		// 本界已经确认过 —— 不再重发 N2314（重发会让客户端再开一次翻牌）。
		return legionResult{}
	}
	// ★★ 2026-10-10 第二十二轮（业主：「回城后不能选下一界」，日志证据见下）：
	//
	// 官服那两帧的进度位把答案写得很清楚：
	//
	//	dungeon0（官服 #897）@11 = 01   ← 官服在【第 1 界清关】之后发的
	//	dungeon1（官服 #981）@11 = 02   ← 官服在【第 2 界清关】之后发的
	//
	// 也就是**第 N 界清关后要发 @11 = N+1 的那一帧**（"已经推进到第 N+1 界"）。
	// 本仓此前传的是 `stage` 本身（第 1 界清完发 dungeon0/@11=1）——
	// 等于把"进度"原地写了一遍，客户端的关卡列表里第 1 界仍然算没打过 ⇒
	// 回城开窗时选不了下一界。
	//
	// 改成"用清关后的界数当帧下标"：第 1 界清完 `cleared`=1 ⇒ `dungeon1`（@11=2），
	// 第 2 界清完 `cleared`=2 ⇒ `dungeon2`（@3=05 收尾帧）。
	//
	// 注意：这与本仓**既有的**一处口径一致 —— 回集结区开窗时
	// （`dimCloisterOperationSelect` 里 `s.dimCloisterCleared > 0` 那一支）本来就是
	// `dimCloisterProgressPackets(..., s.dimCloisterCleared-1)` 传"已清界数"当下标，
	// 也就是把 `dungeon0` 当作"第 1 界已清"的记录。
	newCleared := stage + 1
	if newCleared > len(legion.DimCloisterStageDungeons) {
		newCleared = len(legion.DimCloisterStageDungeons)
	}
	infoStage := newCleared
	if infoStage >= len(legion.DimCloisterStageDungeons) {
		infoStage = len(legion.DimCloisterStageDungeons) - 1
	}
	progress, name, err := dimCloisterProgressPackets(s.dimCloisterMemoryRecords(), infoStage)
	if err != nil {
		return legionResult{Events: []map[string]any{{
			"kind": "dim_cloister_stage_cleared_failed", "stage": stage, "error": err.Error(),
		}}}
	}
	s.dimCloisterCleared = stage + 1
	// ★ 第二十一轮：本界已清 ⇒ 下一界的开窗会话要重新开始（否则回城开窗只会收到
	// "已选"帧、拿不到关卡列表，业主实机「不能选下一界」就是这么来的）。
	s.dimCloisterWindowAcked = false
	s.dimCloisterWindowDeadline = time.Time{}
	log.Printf("dim cloister stage cleared: stage=%d cleared=%d info=%s frames=%d",
		stage, s.dimCloisterCleared, name, len(progress))
	return legionResult{
		Packets: progress,
		Events: []map[string]any{{
			"kind":          "dim_cloister_stage_cleared",
			"character_id":  w.role.ID,
			"stage":         stage,
			"cleared":       s.dimCloisterCleared,
			"progress_info": name,
			"frames":        len(progress),
		}},
	}
}

// dimCloisterStageCleared 推「下一界可挑」的权威状态帧（本界记录态 + 开窗列表态）。
//
// 幂等由 `dimCloisterAcknowledgeStage` 保证：同一界重复收到 CMD2046 时不再重发 N2314。
func (s *legionSession) dimCloisterStageCleared(w *worldSession) (legionResult, error) {
	stage := s.dimCloisterCleared
	if s.dimCloisterStage >= 0 {
		stage = s.dimCloisterStage
	}
	return s.dimCloisterAcknowledgeStage(w, stage), nil
}

// dimCloisterFinale 处理最后一界的收尾：放视频（@3=03）→ 关闭右上角 UI（@3=05）。
//
// 官服 s30 第三界打完：#981 N2314 @3=02 → #985 N2314 @3=03 → #993 N2314 @3=05
// → 客户端翻牌 → CMD72 返回城镇。@3=03 是「终局」态（客户端据此放视频），
// @3=05 是「离开」态（UI 关闭）。
func (s *legionSession) dimCloisterFinale(w *worldSession) (legionResult, error) {
	finale, err := dimCloisterInfoPackets(s.dimCloisterMemoryRecords(), legion.DimCloisterInfoFinale)
	if err != nil {
		return legionResult{}, err
	}
	leave, leaveName, err := legion.DimCloisterDungeonInfoBody(len(legion.DimCloisterStageDungeons) - 1)
	if err != nil {
		return legionResult{}, err
	}
	packets := append(finale, outboundPacket{
		Name: "dim_cloister_info_" + leaveName, Kind: 0, ID: legion.NotiDimCloisterInfo,
		Payload: dimCloisterApplyMemoryRecords(leave, s.dimCloisterMemoryRecords()),
	})
	s.dimCloisterCleared = len(legion.DimCloisterStageDungeons)
	return legionResult{
		Packets: packets,
		Events: []map[string]any{{
			"kind":         "dim_cloister_finale",
			"character_id": w.role.ID,
			"frames":       len(packets),
			"note":         "@3=03 放视频 → @3=05 关闭右上角 UI",
		}},
	}, nil
}

// dimCloisterStageCount 是本场要打的界数（官服口径 = 3）。
const dimCloisterStageCount = 3

// dimCloisterRollStages 抽定本场的关卡顺序（业主 2026-10-11 定调：**恢复随机关卡**）。
//
// 池子 = `legion.DimCloisterStageDungeons`（**有官服进图帧列的那 3 个界**）。
// 为什么不用 `DimCloisterAllStageDungeons`（5 个界）：Moros / LightWoman
// **没有任何官服帧**，只随机副本号却仍回放别的界的怪物/状态帧，实机会出现
// 「选择界面写第1界、地图里刷第1界的 BOSS、右上角写第5界」三处打架。
// 要扩到真正的"5 选 3"，必须先补那两界的官方帧列（业主再抓两份包）。
//
// 抽的是**顺序**（3 个界全用，打乱次序），因为进图帧列与进度记忆体都是按
// 「第几关」索引的，三关必须齐。
func (s *legionSession) dimCloisterRollStages() {
	if s == nil {
		return
	}
	pool := append([]uint32(nil), legion.DimCloisterStageDungeons[:]...)
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	if len(pool) > dimCloisterStageCount {
		pool = pool[:dimCloisterStageCount]
	}
	s.dimCloisterStages = pool
}

// dimCloisterStageDungeon 返回本场第 stage 关的副本号。
//
// 有抽定顺序就用它（业主 2026-10-11 恢复随机）；没抽过则退回官服固定顺序。
func (s *legionSession) dimCloisterStageDungeon(stage int) uint32 {
	if s != nil && stage >= 0 && stage < len(s.dimCloisterStages) {
		return s.dimCloisterStages[stage]
	}
	if stage >= 0 && stage < len(legion.DimCloisterStageDungeons) {
		return legion.DimCloisterStageDungeons[stage]
	}
	return 0
}

// dimCloisterLoadStagePlan 载入第 stage 关并按官服顺序拼出进图帧列。
//
// dungeonID 由调用方按**本场抽定的三关**给出（单机随机口径，见 dimCloisterRollStages）。
func (w *worldSession) dimCloisterLoadStagePlan(stage int, party byte, dungeonID uint32) ([]outboundPacket, map[string]any, error) {
	if w.dungeons == nil {
		return nil, nil, fmt.Errorf("次元回廊第 %d 关载入失败：没有副本目录", stage)
	}
	sel := protocol.DungeonSelection{ID: dungeonID, Difficulty: 0, Party: 65535}
	s, err := dungeon.Select(*w.dungeons, sel, w.level, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("次元回廊第 %d 关（%d）载入失败：%w", stage, dungeonID, err)
	}
	// [FOREST-ARENA-BOSS] 同款：次元回廊每一关都是「进图房间即 boss 房」，
	// 清房即通关（ArenaBoss 完成路径不依赖 CMD117）。
	s.ArenaBoss = true
	noteMazeEntry(s)

	vectors, err := legion.GetDimCloisterEntryVectorsForDungeon(dungeonID)
	if err != nil {
		return nil, nil, err
	}
	// ★★ 2026-10-10 业主实机「击败 BOSS 后不出横幅、不出翻牌界面」的根因修复。
	//
	// 这一关的怪物**不在本会话里**：客户端那只 BOSS 是它自己按我们下发的 N29
	// （START_MAP）生成的，编号取自 N29 记录。而会话这张
	// `.dgn` 怪物表原先自己编号（4096 起），于是客户端上报的 CMD39/CMD117
	// 在这张表里**查不到**：
	//
	//	monsterDeath → ConfirmDeath 报 "monster absent from current source room"
	//	             → 里面的 completeDungeon() 走不到 → Completed() 恒假
	//	             → 清关链（N31/N2059/N2252/N2253/N2316）一帧都不发
	//	             ⇒ 横幅 / 翻牌界面 / 下一界 全部不出现
	//
	// 实机那一场（第 5 界）客户端报的就是 2957 = N29 记录里的 0x0b8d，
	// 而旁路日志 `idx :109015480` 与会话怪物的 template 完全一致 —— 同一只怪、编号没对齐。
	//
	// 这里把会话怪物的编号对齐成 N29 给的编号（同一房间、同一批怪，按位置对应），
	// 客户端的死亡上报与 boss 确认就能命中，清关链自然由既有分支发出。
	// 证据链见 internal/legion/dimension_cloister_entity.go 顶部。
	clientEntities, entityErr := legion.DimCloisterStartMapEntities(vectors)
	if entityErr != nil {
		return nil, nil, entityErr
	}
	aligned, alignErr := legion.DimCloisterAlignArenaEntities(s, clientEntities, s.NextEntity)
	if alignErr != nil {
		return nil, nil, alignErr
	}
	// 安全检查：对齐后表里必须还站着一只**可上报的领主**（rank3 / 5..8 APC）。
	// 本族每一关都是 boss 战，对齐落错位置就意味着后面会把「清关」认到别的怪身上。
	if !s.LegionArenaRosterHasBoss() {
		return nil, nil, fmt.Errorf("次元回廊第 %d 关（%d）编号对齐后会话怪物表里没有领主：%v",
			stage, dungeonID, aligned)
	}
	// N23 + area=0xff：真正进副本时把角色切进副本区域态。
	areaNotice, err := w.forestDungeonAreaNotice()
	if err != nil {
		return nil, nil, err
	}
	plan := make([]outboundPacket, 0, len(vectors)+1)
	plan = append(plan, outboundPacket{Name: "dim_cloister_dungeon_area_entered", Kind: 0, ID: 23, Payload: areaNotice})
	for _, v := range vectors {
		payload := v.Body
		// CMD2045 应答里的队伍字节按请求回显（官服 #789 的 @5 = 请求 @13）。
		if v.Kind == 1 && v.ID == legion.CmdEnterDungeon {
			payload = legion.DimCloisterStageAck(party, uint32(stage))
		}
		plan = append(plan, outboundPacket{Name: "dim_cloister_entry_" + v.Name, Kind: v.Kind, ID: v.ID, Payload: payload})
	}
	// ★ N475（CHARACTER_BUFF_DUNGEON，角色·副本 buff）改用**我们自己的中性构造**。
	//
	// 业主 2026-10-10 实机：BOSS 出现「超越之战 4阶段」形态（三阶段），
	// 而正常 BOSS 只有两阶段 —— 我们这一帧原是**回放官服那次进图的正文**，
	// 带着那位玩家的 buff 状态。其它副本走 `dungeon_flow.go` 时发的是
	// `protocol.CharacterBuffDungeon()`（官服每次进图都发的中性形态），这里对齐它。
	for i := range plan {
		if plan[i].ID == 475 {
			plan[i].Payload = protocol.CharacterBuffDungeon()
			plan[i].Name = "dim_cloister_character_buff_neutral"
		}
	}

	// 与军团其它进图路径共用字段清理：上一场的掉落/死亡计数/结算标志不许渗进来。
	w.deathSent = map[uint16]bool{}
	w.drops = nil
	w.resetCards()
	w.completionSent = false
	w.completionErr = nil
	w.resultSent = false
	w.selectingDungeon = false
	w.approvedDungeonGate = 0
	w.pendingTownArrival = nil
	w.leaveScene()
	w.activeDungeon = s

	return plan, map[string]any{
		"kind":             "dim_cloister_stage_loaded",
		"character_id":     w.role.ID,
		"stage":            stage,
		"dungeon":          dungeonID,
		"maze":             s.Maze.Index,
		"map":              s.Room.Map,
		"monsters":         len(s.Monsters),
		"monster_entities": aligned,
		"party":            party,
		"frames":           len(plan),
	}, nil
}
