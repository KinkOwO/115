package aradpass

import (
	"encoding/json"
	"fmt"
)

// StateKey 是战令状态在角色 State JSON 里的键（与 boostup 的 `boost_up115` 同级）。
const StateKey = "arad_pass115"

const stateVersion = "arad-pass-v1"

// State 是角色的战令进度。
//
// 经验与等级是**角色级**的（0x08A1 是账号级下发，但经验由角色游玩获得，原服
// `[db table name]` 也把通行证表与"角色表"分成两张）。
type State struct {
	Version string `json:"version"`
	// Exp 是**累计**通行证经验（等级由 Catalog.LevelFor 换算，不单独存）。
	Exp uint32 `json:"exp,omitempty"`
	// Royal 表示已购买皇家通行证（源里的 royal 轨）。
	Royal bool `json:"royal,omitempty"`
	// Claimed 记录已领取的奖励档，键见 ClaimKey。
	Claimed map[string]bool `json:"claimed,omitempty"`
	// DailyExp 是**当天**已通过任务获得的经验（封顶见 RecordMission）。
	//
	// ★ 业主 2026-10-11 明确要求：本机是**单机修复**，不必照原版按周设计 ——
	// 改成**每天**有上限、每天 06:00（UTC，与签到同一条换日规则）重置，
	// 这样玩家每天都能重复刷任务经验。
	DailyExp uint32 `json:"daily_exp,omitempty"`
	// DailyKey 是任务进度与经验池归属的**游戏日**（`RollDaily` 的入参，形如 `2026-10-11`）。
	//
	// 换日时（登录、或下一次记任务时发现 `DailyKey != 今天`）会清空 `Missions`、
	// `Rewarded` 与经验池；**已领奖励（`Claimed`）不清** —— 那是赛季一次性的。
	DailyKey string `json:"daily_key,omitempty"`
	// RoyalDailyKey 是「Royal Pass 每日登录礼包」已领到的**游戏日**（09:00 UTC 换日）。
	//
	// 与 `DailyKey`（任务行，06:00 换日）**分开两个字段**：面板对这两处写的是不同的
	// 重置时刻（任务行跟随官方周/日口径，每日礼包面板明写 9:00 UTC），混用会错位。
	RoyalDailyKey string `json:"royal_daily_key,omitempty"`
	// WeeklyExp / WeekKey 是**旧版按周口径**留下的字段：只用于读老存档，不再写。
	WeeklyExp uint32 `json:"weekly_exp,omitempty"`
	WeekKey   string `json:"week_key,omitempty"`
	// Missions 是**本周**各任务已完成的次数（键见 missionKey）；跨周清空。
	Missions map[string]uint32 `json:"missions,omitempty"`
	// Rewarded 记录本周哪些任务的经验已经发过（键同上）——任务做满后只给一次经验。
	Rewarded map[string]bool `json:"rewarded,omitempty"`
	// FirstRewards 表示"购票首发奖励"（源 `[royalpass first reward]`）已经发过。
	FirstRewards bool `json:"first_rewards,omitempty"`
}

// Activate 返回"皇家通行证已激活"的副本：置 Royal 并加购票赠送经验（源 `[purchase pass exp]`）。
//
// 源里通行证是**点券商城商品**（`[pass item index] 10419755`，`[item category] event` +
// `[cerashop detail info template] 2`），买到的效果就是"皇家轨开启 + 送 5000 经验"。
// 本函数只改状态；首发奖励的发放由调用方在同一笔事务里做。
func (s State) Activate(c *Catalog) State {
	next := s
	next.Royal = true
	if c != nil {
		next.Exp += c.PurchaseExp
	}
	return next
}

// missionKey 是任务计数的稳定键：`<轨道>:<序号>`。
func missionKey(track, index int) string { return fmt.Sprintf("%d:%d", track, index) }

// ClaimKey 是奖励档的稳定键：`<track>:<slot>`。
func ClaimKey(track, slot int) string { return fmt.Sprintf("%d:%d", track, slot) }

// ReadState 读角色 State 里的战令进度；缺键 = 全新进度。
func ReadState(state json.RawMessage) (State, error) {
	out := State{Version: stateVersion}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(state, &fields); e != nil {
		return out, e
	}
	raw, ok := fields[StateKey]
	if !ok {
		return out, nil
	}
	if e := json.Unmarshal(raw, &out); e != nil {
		return out, fmt.Errorf("arad pass: %w", e)
	}
	if out.Version == "" {
		out.Version = stateVersion
	}
	if out.Version != stateVersion {
		return out, fmt.Errorf("unsupported saved arad pass %q", out.Version)
	}
	return out, nil
}

// WriteState 把战令进度写回角色 State，**保留其它未知键**（与 SaveBag 同一约定）。
func WriteState(state json.RawMessage, s State) (json.RawMessage, error) {
	if s.Version == "" {
		s.Version = stateVersion
	}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(state, &fields); e != nil {
		return nil, e
	}
	if fields == nil {
		return nil, fmt.Errorf("character state is not an object")
	}
	raw, e := json.Marshal(s)
	if e != nil {
		return nil, e
	}
	fields[StateKey] = raw
	return json.Marshal(fields)
}

// Level 用目录的等级门槛把累计经验换算成等级。
func (s State) Level(c *Catalog) uint32 { return c.LevelFor(s.Exp) }

// IsClaimed 报告某一档是否已领。
func (s State) IsClaimed(track, slot int) bool { return s.Claimed[ClaimKey(track, slot)] }

// Claim 返回"已记录该档领取"的副本（不改原值；未提供的 Claimed map 会被建好）。
func (s State) Claim(track, slot int) State {
	next := s
	next.Claimed = make(map[string]bool, len(s.Claimed)+1)
	for k, v := range s.Claimed {
		next.Claimed[k] = v
	}
	next.Claimed[ClaimKey(track, slot)] = true
	return next
}

// AddExp 返回"加上经验"的副本。
func (s State) AddExp(exp uint32) State {
	next := s
	next.Exp += exp
	return next
}

// RollDaily 是**换日**：把任务进度与经验池归零并记上新的游戏日。
//
// 只在 `dayKey` 与状态里的不同时生效；返回新状态（调用方自行决定是否落库）。
//
// ★ 首次迁移：老存档没有 `DailyKey`（空串）时**只补键、不清进度** ——
// 否则升级那一刻会把玩家当天的进度误清。
func (s State) RollDaily(dayKey string) State {
	next := s
	if dayKey == "" || next.DailyKey == dayKey {
		return next
	}
	first := next.DailyKey == ""
	next.DailyKey = dayKey
	if first {
		return next
	}
	next.Missions = nil
	next.Rewarded = nil
	next.DailyExp = 0
	next.WeeklyExp = 0
	return next
}

// RecordMission 把某条任务推进 n 次，并返回（新状态, 本次实际获得的经验）。
//
// 口径来自源脚本：
//   - 每条任务写的是 `<序号> <键> <次数> <经验>` —— 次数是**需要重复多少次**，
//     经验是**做满那一次**给多少（源 `[weekly gainable exp] 7150` 原本是每周经验上限，
//     全部任务经验之和远超它 ⇒ 经验按"做满一次"结算、再由上限封顶）；
//   - 任务计数与"经验已发"标记**每天清零**（见 RollDaily）：业主 2026-10-11 明确要求
//     改成每日口径，`dayKey` 变了就整组清零 ⇒ 玩家每天都能再刷一轮。
//
// 未达次数不发经验；做满后（当天）不再重复发；超出**当天**上限的部分丢弃。
func (s State) RecordMission(c *Catalog, dayKey string, track, index int, n uint32) (State, uint32) {
	next := s.RollDaily(dayKey)
	mission, ok := c.Mission(track, index)
	if !ok || n == 0 || mission.Count == 0 {
		return next, 0
	}
	key := missionKey(track, index)
	cur := next.Missions[key]
	if cur >= mission.Count {
		return next, 0 // 本周已做满
	}
	cur += n
	if cur > mission.Count {
		cur = mission.Count
	}
	next.Missions = withMission(next.Missions, key, cur)
	if cur < mission.Count || next.Rewarded[key] {
		return next, 0
	}
	gained, dayTotal := mission.Exp, next.DailyExp+mission.Exp
	// 源 `[weekly gainable exp] 7150`：本机按业主口径当作**当天**上限使用。
	if cap := c.WeeklyGainableExp; cap > 0 {
		if next.DailyExp >= cap {
			gained = 0
		} else if dayTotal > cap {
			gained = cap - next.DailyExp
		}
	}
	next.Rewarded = withRewarded(next.Rewarded, key)
	if gained == 0 {
		return next, 0
	}
	next.Exp += gained
	next.DailyExp += gained
	return next, gained
}

// withMission 写一份新的任务计数表（值接收者的方法不就地改原 map）。
func withMission(m map[string]uint32, key string, value uint32) map[string]uint32 {
	out := make(map[string]uint32, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	out[key] = value
	return out
}

func withRewarded(m map[string]bool, key string) map[string]bool {
	out := make(map[string]bool, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	out[key] = true
	return out
}

// MissionCount 取本周某任务的完成次数（诊断/刷新用）。
func (s State) MissionCount(track, index int) uint32 { return s.Missions[missionKey(track, index)] }

// HasAnyClaim 报告这个角色是否领过**任意一档**奖励。
//
// 用来驱动 0x08A1 的 `0x1C2` 字节：抓包那 18 份样本里，**登录首帧是 0、领奖之后
// 17 份全是 1**（见 internal/game/protocol/aradpass115.go 的头部注释）。也就是说
// 这一格在"没领过"和"领过"两种状态下取值不同 —— 它更像是"已领过东西"的标记，
// 而不是"面板已就绪"。
//
// ★ 这一条是 2026-10-11 实机"Free 第 1 档显示绿勾 Complete、点不动"的首要嫌疑：
// 我们此前**硬写 1**，等于对客户端宣称"已经领过了"。
func (s State) HasAnyClaim() bool { return len(s.Claimed) > 0 }

// ClaimCount 已领档数（诊断用）。
func (s State) ClaimCount() int { return len(s.Claimed) }

// MissionProgress 是任务列表一行的两个计数（面板 `n / m` 与 `k time(s)`）。
//
// ⚠ 语义边界（**当前实现的口径**）：`Missions[key]` = 已推进的次数（上限 = 源里的 `<次数>`），
// `Rewarded[key]` = 这一轮已经做满。源脚本明写 `[normal mission]` 是**可重复**的
// （"Normal missions are repeatable, but their EXP rewards cannot exceed the weekly
// maximum EXP limit"），而 `RecordMission` 做满后**不重置进度、不累加轮数**，
// 所以 `Times` 目前只能是 0/1。做成真正的可重复计数需要在 `RecordMission` 里
// "做满即归零 + 轮数 +1" —— 那是行为变更，先在这里记一笔，不偷偷改。
type MissionProgress struct {
	// Progress 写 0x08A1 行内 **@+1**（面板 `n / m` 的 n）。
	//
	// ⚠ 第一版写在 @+3，实机证明客户端读的是 @+1 ⇒ 任务页一直显示 `0 / m`。
	Progress uint16
	// Exp 是该任务单次完成给的经验（源 `<次数> <经验>` 的后者），用来算 `Total`。
	Exp uint16
	// Times 写 0x08A1 行内 @+5（面板 `k time(s)` 的 k）。
	Times uint16
}

// MissionProgressList 按源里 `<序号>` 1..n 生成某条轨的逐行计数（`[normal mission]` = 16、
// `[season mission]` = 12 —— 用 `len(c.NormalMissions)` / `len(c.SeasonMissions)` 取）。
//
// 序号从 **1** 起：客户端遇到 `row[0] == 0` 就停止收行，写 0 会把后面的行全丢掉。
func (s State) MissionProgressList(c *Catalog, track int, count int) []MissionProgress {
	if c == nil || count <= 0 {
		return nil
	}
	out := make([]MissionProgress, 0, count)
	for index := 1; index <= count; index++ {
		m, ok := c.Mission(track, index)
		if !ok {
			out = append(out, MissionProgress{})
			continue
		}
		progress := s.Missions[missionKey(track, index)]
		if m.Count > 0 && progress > m.Count {
			progress = m.Count
		}
		var times uint16
		if s.Rewarded[missionKey(track, index)] {
			times = 1
		}
		out = append(out, MissionProgress{Progress: uint16(progress), Exp: uint16(m.Exp), Times: times})
	}
	return out
}
