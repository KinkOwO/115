// Package honeytime 是活动 313 HoneyTime（每日在线时长奖励）的**服务端进度**。
//
// 内容全部来自 PVF：脚本路径**从 `list/event.lst` 的索引解析**（活动 313 的绑定），
// 不是从目录名/日期猜的 —— 见 `ResolveScriptPath`。
// （`[reward data]` 每行 = `序号 物品 数量 0 毫秒阈值` ⇒ 在线 10/20/30/60 分钟各一档；
// `[reward postal type] 1` ⇒ 奖励走**邮件**）。
//
// ## 为什么需要服务端记进度
//
// 客户端的在线累计是**会话内、本地**的：换角色/重登就从头开始。业主实机确认过这一点
// （切换角色后阶段回到 1）。所以要像签到那样**按角色持久化**：
//
//	登录时读回「今天的累计秒数 + 它属于哪一天」，推到 NOTI1294 让面板从那里续上；
//	若存档里的那一天不是今天（**06:00 UTC 换日**，与签到同一条客户端规则），
//	从头开始（Seconds=0、Day=今天）。
//
// ## 与 1294 的对应（L0）
//
// `sub_143CC72A0(obj, 轨, u32)` 里第一句是 `v8 = 1000 * a3` —— 输入是**秒**，客户端自己
// 乘 1000 变毫秒，再用 .evt 每行（21 字节行表的 `+12`）的阈值算出"走到第几档"。
// ⇒ 1294 里每个轨道那个 u32 就是**该轨道的累计秒数**。本文件只管这一个值。
package honeytime

import (
	"dfolan/internal/attendance"
	"encoding/json"
	"fmt"
	"math/bits"
)

// EventID 是活动号（同时是 CMD680 载荷里的 u32，实机样本 `39010000` = 313）。
const EventID uint32 = 313

// EventListPath 是「活动号 → 脚本路径」的索引 —— **客户端加载哪一份 evt 由它决定**，
// 不是由目录名或日期决定。
//
// ★ 2026-10-11 实测：索引里 `313` 唯一绑定到
//
//	Live/Event/Kor/2023/1026_HoneyTime/HoneyTime.evt      ← 客户端读的（窗口 2026-07-09→08-04）
//
// 而仓库里另有一份 `live/event/kor/2026/0326_honeytime/honeytime.evt`
// （窗口 2026-03-26→05-07），**它没有被任何索引引用**。写死后者会让
// 「面板（客户端按索引那份画）」与「发奖（我们从 2026 那份取）」用两套物品号。
const EventListPath = "list/event.lst"

// CycleStart 是本期的起点 = **索引版脚本**（`list/event.lst` 的 313 绑定那份）
// `[control panel]` 的开始时刻：`2026-07-09 09:00:00`，按韩服本地时区 UTC+9 折成
// unix 秒 = 2026-07-09 00:00 UTC。
//
// ⚠️ 时刻必须来自**我们从索引解析出来的那一份**脚本。2026-10-11 之前这里用的是另一份
// （没被任何索引引用的 `2026/0326_honeytime`，窗口 `2026-03-26 06:00` KST）⇒ 与 op108 一起
// 订正，见 `analysis/tasks/next197-313读错脚本-根因与修复-20261011.md`。
//
// 它同时是**存档 state 里的期号**：换期时改这一行（+ NOTI108 里那条 313 行），
// 玩家的进度会自动从头算（见 `State.Normalize`），不需要运维清档。
// ⇒ **改这一行会清掉所有角色在本活动上的进度**；本次换值后已用
// `dfo-tool honeytime -set-seconds/-set-period-seconds` 把测试角色的进度推回。
//
// ★ 这里是**唯一定义**：`cmd/wireprobe` 的 op108 行与网关的 Normalize 都引用它；
// `dfo-tool honeytime` 也引用它。两处手抄同一个数字迟早会漂。
const CycleStart int64 = 1783555200

// MaxDailySeconds 是每日轨的**封顶**。
//
// ★ 2026-10-10 订正（原来写的 3600 是**按绝对阈值**读脚本得出的，错了）：
// `sub_143CC72A0` 算"走到第几档"时是**逐行相减**（`v10 -= row[+12]`），
// 所以 `.evt` 里那四个数（600000/1200000/1800000/3600000 ms）是**每档的时长**，
// 不是绝对门槛 ⇒ 四档的**累计**门槛是 **10 / 30 / 60 / 120 分钟**（600/1800/3600/7200 s）。
// 实机也对得上：进城 17 分 35 秒时客户端发的是第 1 档（10 分钟档）的领奖请求。
//
// 封顶取 4 小时（= 索引版脚本日档 4 行的累计 7200 秒的两倍）：**宁可存大数也别把合法值截掉**
// （截掉之后重登会推一个偏小的值，第 4 档就再也点不亮了）。客户端自己对越界的档做了夹取。
const MaxDailySeconds = 14400

// State 是一个角色在本期活动上的进度。
type State struct {
	Version int `json:"version"`
	// EventID 冗余存一份，防止同一 state 键下混进别的活动。
	EventID uint32 `json:"event_id"`
	// CycleStart 是本期的起点（运营窗口的开始，换期即重置）。见 cmd/wireprobe 的常量。
	CycleStart int64 `json:"cycle_start"`
	// Day 是 Seconds 所归属的"签到日号"（以 **06:00 UTC** 为界，与签到同一条规则）。
	// 它和 Seconds 必须一起读：Day 不是今天 ⇒ 当天从头开始。
	Day int64 `json:"day"`
	// Seconds 是**今天**已累计的在线秒数（0..MaxDailySeconds）。
	Seconds uint32 `json:"seconds"`
	// PeriodSeconds 是**期间轨**（面板下面那行"累计在线时长"）已累计的秒数。
	//
	// ★ 它**跨天不清零**：期间轨统计的是整期累计在线（本季累计门槛 5 / 20 / 50 / 80 小时），
	// 与日档"每天重来"不是一回事 ⇒ `Normalize` 跨天时只重置日档那部分（见 Normalize）。
	PeriodSeconds uint32 `json:"period_seconds,omitempty"`
	// PeriodClaimed 是期间轨已领档的位图（位 = `Tier.Index`）。与日档同理跨天保留。
	PeriodClaimed uint32 `json:"period_claimed_mask,omitempty"`
	// Claimed 是**今天**已经领过的档的位图：第 i 位 = 脚本里行首值为 i 的那一档已领。
	//
	// 用位图而不是 `[]int`：`State` 必须保持**可比较**（`honeyTimeRestore` 里
	// `if st != stored` 是"归一化后真的变了才落库"的判据），带 slice 就直接编译不过。
	// 位图还顺带把"同一个 Index 记两次"这类错挡在门外。
	Claimed uint32 `json:"claimed_mask,omitempty"`
}

const stateKey = "event_honeytime115"

// MaxPeriodSeconds 是期间轨的封顶**上界**（索引版脚本 = 5h + 15h + 30h + 30h = 80 小时 = 288000 秒）。
//
// 为什么这里再写一份常量：`dfo-tool honeytime` **不读 PVF**（要能离线跑），
// 它只能拿一个编译期上界来校验 `-set-period-seconds`；而网关侧用目录算出来的
// `Catalog.PeriodCapacity()`。两者本机一致（288000），脚本换形态时以目录为准。
const MaxPeriodSeconds = 288000

// MaxClaimedTiers 是可领档数的上限（位图按 u32 存，只允许 0..31 的档号）。
// 日档只有几档，超过它一定是脚本换了形态 —— 宁可拒绝，也不要拿一个越界的位去算奖。
const MaxClaimedTiers = 32

// ReadState 从角色 state JSON 里读出进度。没有这一段就是「本期全新」。
func ReadState(raw json.RawMessage) (State, error) {
	fresh := State{Version: 1, EventID: EventID, Day: -1}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return State{}, err
	}
	if fields == nil {
		return State{}, fmt.Errorf("character state is not an object")
	}
	body, ok := fields[stateKey]
	if !ok {
		return fresh, nil
	}
	s := fresh
	if err := json.Unmarshal(body, &s); err != nil {
		return State{}, err
	}
	if s.Version != 1 || s.EventID != EventID || s.Seconds > MaxDailySeconds {
		return State{}, fmt.Errorf("invalid honeytime state")
	}
	return s, nil
}

// WriteState 把进度写回角色 state JSON（保留其它键）。
func WriteState(raw json.RawMessage, s State) (json.RawMessage, error) {
	if s.Version != 1 || s.EventID != EventID || s.Seconds > MaxDailySeconds {
		return nil, fmt.Errorf("invalid honeytime state")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("character state is not an object")
	}
	body, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	fields[stateKey] = body
	return json.Marshal(fields)
}

// Today 把某一时刻折算成"签到日号"。
//
// 直接复用 `attendance.DayNumber`：**06:00 UTC 换日**是客户端自己的规则
// （`floor(t/86400)*86400 + 21600`），两个活动必须同口径 —— 各写一份迟早漂移。
func Today(unixSeconds int64) int64 {
	return attendance.DayNumber(unixSeconds)
}

// Normalize 把进度对齐到「本期 + 今天」。
//
//	换期（CycleStart 变了）—— **整份重来**（日档 + 期间轨都清）；
//	跨天（Day != today）    —— **只清日档**：期间轨是整期累计，跨天必须留着。
func (s State) Normalize(cycleStart, today int64) State {
	if s.Version != 1 || s.EventID != EventID || s.CycleStart != cycleStart {
		return State{Version: 1, EventID: EventID, CycleStart: cycleStart, Day: today}
	}
	if s.Day != today {
		out := s
		out.Day = today
		out.Seconds = 0
		out.Claimed = 0
		return out
	}
	return s
}

// AddSeconds 把 n 秒累计进去（封顶 MaxDailySeconds）。返回是否发生变化。
func (s *State) AddSeconds(n uint32) bool {
	if s == nil || n == 0 || s.Seconds >= MaxDailySeconds {
		return false
	}
	before := s.Seconds
	s.Seconds += n
	if s.Seconds > MaxDailySeconds {
		s.Seconds = MaxDailySeconds
	}
	return s.Seconds != before
}

// AddPeriodSeconds 把 n 秒累计进**期间轨**（cap 由目录给出：本机脚本 5 档 × 20 小时）。
// 返回是否发生变化。与 `AddSeconds` 同构，只是走另一条轨、且**不随跨天清零**。
func (s *State) AddPeriodSeconds(n, capacity uint32) bool {
	if s == nil || n == 0 || capacity == 0 || s.PeriodSeconds >= capacity {
		return false
	}
	before := s.PeriodSeconds
	s.PeriodSeconds += n
	if s.PeriodSeconds > capacity {
		s.PeriodSeconds = capacity
	}
	return s.PeriodSeconds != before
}

// PeriodClaimedTier 判断期间轨某一档（脚本每行的第一个值）是否已经领过。
func (s State) PeriodClaimedTier(index int) bool {
	if index < 0 || index >= MaxClaimedTiers {
		return false
	}
	return s.PeriodClaimed&(uint32(1)<<uint(index)) != 0
}

// PeriodClaimedCount 是期间轨已领的档数。
func (s State) PeriodClaimedCount() int {
	return bits.OnesCount32(s.PeriodClaimed)
}

// ClaimPeriod 在内存里记一次期间轨领取（**不落库**）。与 `Claim` 同构。
func (s State) ClaimPeriod(index int) (State, error) {
	if index < 0 || index >= MaxClaimedTiers {
		return s, fmt.Errorf("honeytime period tier index %d out of range", index)
	}
	if s.PeriodClaimedTier(index) {
		return s, fmt.Errorf("honeytime period tier %d is already claimed", index)
	}
	out := s
	out.PeriodClaimed |= uint32(1) << uint(index)
	return out, nil
}

// ClaimedTier 判断某一档（脚本每行的第一个值）今天是否已经领过。
func (s State) ClaimedTier(index int) bool {
	if index < 0 || index >= MaxClaimedTiers {
		return false
	}
	return s.Claimed&(uint32(1)<<uint(index)) != 0
}

// ClaimedCount 是今天已领的档数（1 起的档号就靠它推给客户端）。
func (s State) ClaimedCount() int {
	return bits.OnesCount32(s.Claimed)
}

// Claim 在内存里记一次领取（**不落库**，落库由 workflow 做）。index 是 `Tier.Index`。
// 同一档重复记会被拒绝 —— 幂等的第一道闸（第二道是事件键）。
func (s State) Claim(index int) (State, error) {
	if index < 0 || index >= MaxClaimedTiers {
		return s, fmt.Errorf("honeytime tier index %d out of range", index)
	}
	if s.ClaimedTier(index) {
		return s, fmt.Errorf("honeytime tier %d is already claimed today", index)
	}
	out := s
	out.Claimed |= uint32(1) << uint(index)
	return out, nil
}
