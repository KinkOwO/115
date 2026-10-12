package oracleresearch

import (
	"encoding/json"
	"fmt"
	"time"
)

// StateKey 是这份进度在角色 state JSON 里的键。
//
// 与 `honeytime` 的 `event_honeytime115`、`attendance` 的 `event_attendance115` 并列：
// 活动进度一律挂在角色存档里（**不是**账号级 —— 美服说明里"每周二删除神秘能量"作用在
// 角色身上；具体口径见 next201 §12）。
const StateKey = "event_oracle685"

// State 是活动 685 的角色级进度。
//
// **必须保持可比较**（`State` 里不放 slice/map）：调用方用 `!=` 判"归一化后真的变了才落库"，
// 这是 `honeytime` 那条线验证过的写法。
type State struct {
	Version int `json:"version,omitempty"`
	// WeekStart 是这份进度所属的自然周（**周二 00:00 UTC** 的 Unix 秒）。
	WeekStart int64 `json:"week_start,omitempty"`
	// Produced 是本自然周**已完成的**生产次数（n 从 0 起：0 = 还没生产过）。
	//
	// 它同时决定两件事（都在目录层）：
	//   * 下一次的费用 `ProductionCostFor(n)` = 基准 × (n+1)
	//   * 下一次产出的模块 `ModuleAt(n)`（按源顺序轮换）
	Produced uint32 `json:"produced,omitempty"`
	// RowsUsed 记录本自然周**已经组装过的组装槽行**（0..2）。
	//
	// 官方 UI：“A slot will be deactivated after Assembling.”
	//        “Assemble Slot Activation Status is reset every Tuesday at 09:00 UTC.”
	// ⇒ 每行每周只能用一次；这是**服务端强制**的规则，按钮能不能点都不影响正确性。
	RowsUsed [3]bool `json:"rows_used,omitempty"`
	// Dismantles 是本角色**累计**分解次数，只用来给邮件事件键做序号
	//（分解可以反复做、没有自然周期 ⇒ 键 = 周起点 + 这个计数）。
	// 刻意**不随周归零**：单调递增才不会和历史键撞车。
	Dismantles uint32 `json:"dismantles,omitempty"`
}

// WeekStartOf 返回 t 所属自然周的起点（**周二 09:00 UTC**）。
//
// 依据：官方活动 UI 的帮助原文**三次**都写
//   "… is reset every Tuesday at **09:00 UTC**"
//（神秘能量 / 生产费用与次数 / 组装槽激活状态）。
// ⇒ 周界 = 周二 09:00 UTC；脚本 `[event period]` 的 `2026-07-07 09:00:00` 是 **UTC**（不是 KST）。
// ⚠️ 2026-10-12 修正：此前写成"周二 00:00 UTC"，比官方早 9 小时。
func WeekStartOf(t time.Time) int64 {
	u := t.UTC()
	// Go 的 Weekday：Sunday=0 … Tuesday=2。先取本周二 09:00 UTC，再按需回退一周。
	days := (int(u.Weekday()) - int(time.Tuesday) + 7) % 7
	start := time.Date(u.Year(), u.Month(), u.Day(), 9, 0, 0, 0, time.UTC).AddDate(0, 0, -days)
	if u.Before(start) {
		start = start.AddDate(0, 0, -7)
	}
	return start.Unix()
}

// Normalize 把状态对齐到 weekStart 所属的那一周：跨周就把已产次数归零。
//
// 与 `honeytime.State.Normalize` 同一条纪律：**归一化只改"该改的"**，
// 调用方用 `==` 判断是否真的变了，只有变了才落库。
func (s State) Normalize(weekStart int64) State {
	next := s
	if next.Version == 0 {
		next.Version = 1
	}
	if next.WeekStart != weekStart {
		next.WeekStart = weekStart
		next.Produced = 0
		// 组装槽激活状态与已产次数同一条周界（官方：都按周二 09:00 UTC 重置）。
		next.RowsUsed = [3]bool{}
	}
	return next
}

// ReadState 从角色 state JSON 里读出进度。没有这一段就是「本周全新」。
func ReadState(raw json.RawMessage) (State, error) {
	fresh := State{Version: 1, WeekStart: 0, Produced: 0}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return State{}, err
	}
	blob, ok := fields[StateKey]
	if !ok || len(blob) == 0 {
		return fresh, nil
	}
	var st State
	if err := json.Unmarshal(blob, &st); err != nil {
		return State{}, fmt.Errorf("%s: %w", StateKey, err)
	}
	if st.Version == 0 {
		st.Version = 1
	}
	// ★ 一次性迁移：2026-10-12 之前周界写成「周二 00:00 UTC」，官方是 09:00 UTC。
	// 老档的 week_start 落在午夜（% 86400 == 0）⇒ 整体 +9 小时。
	// 不迁移的话，改周界当天**当前这一周会被判成跨周**，已产次数与组装槽会被白白归零一次。
	if st.WeekStart != 0 && st.WeekStart%86400 == 0 {
		st.WeekStart += 9 * 3600
	}
	return st, nil
}

// WriteState 把进度写回整份 state JSON（其余键原样保留）。
func WriteState(raw json.RawMessage, st State) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	blob, err := json.Marshal(st)
	if err != nil {
		return nil, err
	}
	if fields == nil {
		fields = map[string]json.RawMessage{}
	}
	fields[StateKey] = blob
	return json.Marshal(fields)
}

// NextCost 是本自然周"下一次生产"的费用（n = 已产次数）。
func (c *Catalog) NextCost(st State) uint32 {
	if c == nil {
		return 0
	}
	return c.ProductionCostFor(st.Produced)
}

// NextModule 是本自然周"下一次生产"应产出的模块。
func (c *Catalog) NextModule(st State) (uint32, bool) {
	if c == nil {
		return 0, false
	}
	return c.ModuleAt(st.Produced)
}
