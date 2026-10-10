// Package attendance 是活动 331「每日签到」的源驱动实现。
//
// 内容全部来自 PVF（`live/event/kor/2026/0326_attendancedailyevent/attendancedailyevent.evt`），
// 服务端只做三件事：解析这份脚本、按客户端口径算「今天能不能领」、发奖。
//
// 为什么这个包是**功能独立**的而不是塞进 boostup：两者脚本、活动号、协议帧都不同
// （662=礼包/训练，331=签到/op1379+op680），共用一条线只会让两边互相耦合。
package attendance

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// EventStart / EventEnd 是 331 的**运营窗口**，逐字取自官服 NOTI108 抓包
// （internal/legion/event_info_official.plain 里 id=331 那条：start=1785801600 /
// end=1791277198，即 2026-08-04 → 2026-10-06）。
//
// EventStart 同时是**存档的期号**（cycleStart）：换期改这两行 + op108 那一行，玩家的
// 签到进度会自动从头算（见 State.Normalize），不需要运维清档。
//
// ⚠️ **不要拿窗口当「活动开不开」的判据**：客户端 `[event on]` 只看 op108 里那条记录的
// `present`，不与 start/end 比较（2026-10-10 实测：窗口已过而 331 仍为 on）。
const (
	EventStart = 1785801600
	EventEnd   = 1791277198
	// ResetOffsetSeconds 是客户端算「下一次重置」时加的那个偏移
	// （`floor(t/86400)*86400 + 21600`）⇒ **每天 06:00 UTC 换日**。
	// 窗口文案写的是「Reset every day at 09:00 UTC」，与客户端自己的计算不一致；
	// 服务端跟**客户端计算**走，否则「第几天可领」会错位。
	ResetOffsetSeconds = 21600
)

// DayNumber 把某一时刻折算成「第几个签到日」（以 06:00 UTC 为界的日号）。
// 只有**差值**有意义：`today > lastClaimDay` 就表示跨过了一次换日。
func DayNumber(unixSeconds int64) int64 {
	return (unixSeconds - ResetOffsetSeconds) / 86400
}

// ScriptPath 是签到脚本在 PVF 里的路径（`list/event.lst` 的 331 指向它）。
const ScriptPath = "live/event/kor/2026/0326_attendancedailyevent/attendancedailyevent.evt"

// EventID 是活动号。它同时是 op680 请求里那个 u32（实机样本 `4b0100 00` = 331）。
const EventID uint32 = 331

// MaxDays 是脚本 `[reward infos]` 里 `[reward info]` 的条数上限（day 0..6）。
// 客户端固定读 28 个 day_state 槽，但脚本只给前 7 天内容。
const MaxDays = 7

// Reward 是 `[reward items]` 里的一对（模板, 数量）。
type Reward struct {
	Template uint32
	Count    uint32
}

// Day 是一天的奖励。
type Day struct {
	Index   int
	Rewards []Reward
}

// Catalog 是这份脚本解析出来的玩法内容。
type Catalog struct {
	// MailTitleKey / MailMessageKey 是 `[reward mail title]` / `[reward mail message]`
	// 里的**本地化键**。源只给键名，键→文案的解析表在客户端字符串表里（服务端要读它
	// 得再加一个 PVF 域），所以这里把键原样留着：开单时记进事件，文案先用可读字面串
	// （见 cmd/wireprobe 的签到领取）。**这是运营文案，不是玩法规则**。
	MailTitleKey   string
	MailMessageKey string
	// Accumulate 是 `[allow accumulate attendance]`（1 = 漏签不清零）。
	Accumulate bool
	// FirstLoginPopup 是 `[first login open popup]`。
	FirstLoginPopup bool
	// UIPath 是 `[ui path]`，仅留档（客户端自己按活动号找 UI）。
	UIPath string
	// DBTableName 是 `[db table name]`，仅留档。
	DBTableName string
	// Days 按 day 升序，Index 是脚本里写的 `[day] N`。
	Days []Day
}

// TextSource 只要「按路径读一份脚本文本」这一件事 —— 与 boostup.TokenSource 同一个
// 归档，不第二次加载 PVF。
type TextSource interface {
	Text(string) (string, error)
}

// Load 读并解析签到脚本。
func Load(src TextSource) (*Catalog, error) {
	text, err := src.Text(ScriptPath)
	if err != nil {
		return nil, err
	}
	return Parse(text)
}

// Parse 解析 attendancedailyevent.evt 的文本。
//
// 脚本形态（逐字取自本机 PVF 的导出）是「空行分隔的标签 + 值」，值有时在同一行、
// 有时在下一行（`[reward info]` / `[day]` / `0` 各自一行），所以先拍平成 token 流
// 再走，别按行做状态机：
//
//	[event period] `2026-07-06 09:00:00` `2026-10-06 09:00:00` [/event period]
//	[db table name] `event_2603_daily_attendance`
//	[allow accumulate attendance] 1
//	[reward mail title] `event_331_mail_title`
//	[reward mail message] `event_331_mail_msg`
//	[reward infos]
//	[reward info] [day] 0 [reward items] 590015045 1000 590015133 1 [/reward items] [/reward info]
//	… 共 7 条 …
//	[/reward infos]
//
// `[reward items]` 的值是**成对**的（模板, 数量），奇数个直接报错（宁可拒绝，不要猜）。
func Parse(text string) (*Catalog, error) {
	tokens := tokenize(text)
	c := &Catalog{}
	var current *Day
	for i := 0; i < len(tokens); i++ {
		tag := tokens[i]
		if !strings.HasPrefix(tag, "[") {
			return nil, fmt.Errorf("attendance script: 意外的裸值 %q", tag)
		}
		switch tag {
		case "[reward infos]", "[/reward infos]":
			// 容器标签：里面由 [reward info] 块自己走，别当成未知块整块跳过
			// （那样会把 7 天内容全吃掉）。
		case "[/reward info]":
			if current == nil {
				return nil, fmt.Errorf("attendance script: [/reward info] 没有配对的 [reward info]")
			}
			c.Days = append(c.Days, *current)
			current = nil
		case "[reward info]":
			if current != nil {
				return nil, fmt.Errorf("attendance script: [reward info] 嵌套")
			}
			current = &Day{}
		case "[day]":
			value, next, err := scalar(tokens, i)
			if err != nil {
				return nil, fmt.Errorf("attendance script: [day] %w", err)
			}
			if current == nil {
				return nil, fmt.Errorf("attendance script: [day] 出现在 [reward info] 之外")
			}
			day, convErr := strconv.Atoi(value)
			if convErr != nil || day < 0 || day >= MaxDays {
				return nil, fmt.Errorf("attendance script: [day] 取值 %q 越界", value)
			}
			current.Index = day
			i = next
		case "[reward items]":
			values, next, err := until(tokens, i+1, "[/reward items]")
			if err != nil {
				return nil, fmt.Errorf("attendance script: [reward items] %w", err)
			}
			if current == nil {
				return nil, fmt.Errorf("attendance script: [reward items] 出现在 [reward info] 之外")
			}
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("attendance script: [reward items] 有 %d 个值，模板与数量必须成对", len(values))
			}
			for k := 0; k < len(values); k += 2 {
				template, e1 := strconv.ParseUint(values[k], 10, 32)
				count, e2 := strconv.ParseUint(values[k+1], 10, 32)
				if e1 != nil || e2 != nil {
					return nil, fmt.Errorf("attendance script: [reward items] 的 %q/%q 不是数字", values[k], values[k+1])
				}
				current.Rewards = append(current.Rewards, Reward{Template: uint32(template), Count: uint32(count)})
			}
			i = next
		case "[reward mail title]", "[reward mail message]", "[ui path]", "[db table name]":
			value, next, err := scalar(tokens, i)
			if err != nil {
				return nil, fmt.Errorf("attendance script: %s %w", tag, err)
			}
			switch tag {
			case "[reward mail title]":
				c.MailTitleKey = value
			case "[reward mail message]":
				c.MailMessageKey = value
			case "[ui path]":
				c.UIPath = value
			case "[db table name]":
				c.DBTableName = value
			}
			i = next
		case "[allow accumulate attendance]", "[first login open popup]":
			value, next, err := scalar(tokens, i)
			if err != nil {
				return nil, fmt.Errorf("attendance script: %s %w", tag, err)
			}
			on := value != "0"
			if tag == "[allow accumulate attendance]" {
				c.Accumulate = on
			} else {
				c.FirstLoginPopup = on
			}
			i = next
		default:
			// 不认识的标签块整块跳过（脚本自己会加标签，服务端不该因此拒绝整个活动）。
			if next := skipBlock(tokens, i); next > i {
				i = next
			}
		}
	}
	if current != nil {
		return nil, fmt.Errorf("attendance script: [reward info] 没有闭合")
	}
	if len(c.Days) == 0 {
		return nil, fmt.Errorf("attendance script: 没有任何 [reward info]")
	}
	// 按 day 排序，并检查不重复 —— `[reward info]` 的顺序不保证就是 day 的顺序。
	seen := map[int]bool{}
	ordered := make([]Day, 0, len(c.Days))
	for d := 0; d < MaxDays; d++ {
		for _, day := range c.Days {
			if day.Index == d && !seen[d] {
				seen[d] = true
				ordered = append(ordered, day)
			}
		}
	}
	if len(ordered) != len(c.Days) {
		return nil, fmt.Errorf("attendance script: [day] 有重复取值")
	}
	c.Days = ordered
	return c, nil
}

// DayRewards 返回某一天的奖励；越界返回 ok=false。
func (c *Catalog) DayRewards(day int) ([]Reward, bool) {
	if c == nil || day < 0 {
		return nil, false
	}
	for _, d := range c.Days {
		if d.Index == day {
			return d.Rewards, true
		}
	}
	return nil, false
}

// tokenize 把脚本文本拍平成 token：`[tag]` 原样保留，值去掉反引号与首尾空白。
//
// 两种值要区别对待：**反引号串**（“ `a b` “）整体是一个 token，内部允许空格；
// **裸值段**（`590015045 1000 590015133 1`）按空白切成多个 token。
func tokenize(text string) []string {
	var out []string
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		for len(line) > 0 {
			switch {
			case line[0] == '[':
				end := strings.IndexByte(line, ']')
				if end < 0 {
					out = append(out, line)
					line = ""
					continue
				}
				out = append(out, line[:end+1])
				line = strings.TrimSpace(line[end+1:])
			case line[0] == '`':
				end := strings.IndexByte(line[1:], '`')
				if end < 0 {
					out = append(out, strings.Trim(line, "`"))
					line = ""
					continue
				}
				out = append(out, line[1:1+end])
				line = strings.TrimSpace(line[end+2:])
			default:
				cut := len(line)
				if idx := strings.IndexAny(line, "[`"); idx >= 0 {
					cut = idx
				}
				out = append(out, strings.Fields(line[:cut])...)
				line = strings.TrimSpace(line[cut:])
			}
		}
	}
	return out
}

// scalar 读标签后面的第一个值。
func scalar(tokens []string, i int) (string, int, error) {
	if i+1 >= len(tokens) || strings.HasPrefix(tokens[i+1], "[") {
		return "", i, fmt.Errorf("缺少取值")
	}
	return tokens[i+1], i + 1, nil
}

// until 收集 token 直到终止标签（不含），返回最后一个已消费的下标。
func until(tokens []string, from int, terminator string) ([]string, int, error) {
	var values []string
	for i := from; i < len(tokens); i++ {
		if tokens[i] == terminator {
			return values, i, nil
		}
		if strings.HasPrefix(tokens[i], "[") {
			return nil, i, fmt.Errorf("遇到未闭合的 %s（在 %s 之前）", tokens[i], terminator)
		}
		values = append(values, tokens[i])
	}
	return nil, len(tokens), fmt.Errorf("找不到 %s", terminator)
}

// skipBlock 跳过 i 处的一个块：有配对 `[/x]` 就跳到它之后；没有配对时，如果下一个
// token 是裸值（`[level limit]` / `0` 各占一行这种）就把它一起跳掉 —— 否则循环推进
// 一格之后会撞上那个裸值并报错。
func skipBlock(tokens []string, i int) int {
	open := tokens[i]
	close := "[/" + strings.TrimPrefix(open, "[")
	for j := i + 1; j < len(tokens); j++ {
		if tokens[j] == close {
			return j
		}
	}
	if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "[") {
		return i + 1
	}
	return i
}

// ---- 状态（存在角色存档的 state JSON 里，键 event_attendance115）----

// State 是一个角色在某期签到活动上的进度。
//
// **为什么是角色级而不是账号级**：源脚本里**没有** `[attach type]`（逐字核对过），
// 参考文档给的「账号级」是它对自己客户端的推测；本仓既有的活动存档（boostup）也都在
// 角色 state 里，并且领取的幂等靠 `CommitCharacterEvent` 的事件键 —— 那套机制是
// **角色维度**的。用账号级要新开表 + 迁移小节 + 5 处 sqlc 镜像，换来的只是"小号不能
// 再领一次"，而源里没有依据。⇒ 先按角色级落地，档位口径记进 next193。
type State struct {
	Version int `json:"version"`
	// EventID 冗余存一份，防止将来同一 state 键下混进别的活动。
	EventID uint32 `json:"event_id"`
	// CycleStart 是这一期活动的起点（脚本 `[event period]` 的起始 unix 秒）。
	// 换期时它变了 ⇒ 进度自动重置，不需要额外的运维动作。
	CycleStart int64 `json:"cycle_start"`
	// ClaimedDays 是已领天数（0..MaxDays），`[allow accumulate attendance] 1` ⇒ 单调递增。
	ClaimedDays int `json:"claimed_days"`
	// LastClaimDay 是上一次领奖那天折算出的"签到日号"（06:00 UTC 为界），
	// -1 = 从没领过。判「今天还能不能领」只看它与今天的差。
	LastClaimDay int64 `json:"last_claim_day"`
}

const stateKey = "event_attendance115"

// ReadState 从角色 state JSON 里读出签到进度。没有这一段就是「本期全新」。
func ReadState(raw json.RawMessage) (State, error) {
	fresh := State{Version: 1, EventID: EventID, CycleStart: 0, ClaimedDays: 0, LastClaimDay: -1}
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
	if s.Version != 1 || s.EventID != EventID || s.ClaimedDays < 0 || s.ClaimedDays > MaxDays {
		return State{}, fmt.Errorf("invalid attendance state")
	}
	return s, nil
}

// WriteState 把签到进度写回角色 state JSON（保留其它键）。
func WriteState(raw json.RawMessage, s State) (json.RawMessage, error) {
	if s.Version != 1 || s.EventID != EventID || s.ClaimedDays < 0 || s.ClaimedDays > MaxDays {
		return nil, fmt.Errorf("invalid attendance state")
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

// Normalize 把 state 对齐到当前这一期：换期（cycleStart 变了）或首次使用都重置进度，
// 并补上默认值。调用方拿到它之后才谈"今天能不能领"。
func (s State) Normalize(cycleStart int64) State {
	if s.Version != 1 || s.EventID != EventID || s.CycleStart != cycleStart {
		return State{Version: 1, EventID: EventID, CycleStart: cycleStart, ClaimedDays: 0, LastClaimDay: -1}
	}
	if s.LastClaimDay == 0 && s.ClaimedDays == 0 {
		s.LastClaimDay = -1
	}
	return s
}

// AvailableToday 是"今天还能不能领一次"的唯一判据。
//
//	today  今天的"签到日号"（06:00 UTC 为界的整数，见 cmd/wireprobe/attendance_flow.go）
//
// claimed == 0 ⇒ 本期一次都没领过 ⇒ 今天可领（官服新号样本就是 attended=1、第 1 格可领）。
// 领满 MaxDays 后不再可领（脚本里没有第 8 天）。
func (s State) AvailableToday(today int64) bool {
	if s.ClaimedDays >= MaxDays {
		return false
	}
	if s.ClaimedDays <= 0 {
		return true
	}
	return today > s.LastClaimDay
}

// Claim 在内存里记一次领取：天数 +1、上次领取日 = today。**不落库**（落库由 workflow 做）。
func (s State) Claim(today int64) (State, error) {
	if !s.AvailableToday(today) {
		return s, fmt.Errorf("attendance day is not claimable today")
	}
	s.ClaimedDays++
	s.LastClaimDay = today
	return s, nil
}
