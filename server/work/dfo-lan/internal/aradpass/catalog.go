// Package aradpass 实现「战令」——DFO 的**阿拉德通行证 / Arad Pass**。
//
// 真源是客户端脚本 `live/event/kor/<年>/<月日>_newaradpass/newaradpassevent.evt`：
//
//	[event period]       赛季起止
//	[pass level info]    1..50 级各自需要的**累计**经验（0, 600, 1200, …, 50000）
//	[reward table]       普通 26 档 + 皇家 25 档，每档真实物品号
//	[normal mission]     16 条日常任务（各自给多少通行证经验）
//	[season mission]     12 条赛季任务
//	[weekly gainable exp] / [purchase pass exp] / [pass item index] / …
//
// 协议侧（0x08A1 / 0x08A2）的字段划分见 internal/game/protocol/aradpass.go 的注释。
package aradpass

import (
	"fmt"
	"strconv"
	"strings"
)

// 战令在客户端活动表里的「开放时段 id」（`EVENT_INFO` 门禁用）；领奖请求带的是
// ClaimScheduleID —— 抓包那份用的是 516，10005 是同一活动的另一个开放时段。
const (
	ClaimScheduleID uint32 = 516
	AltScheduleID   uint32 = 10005
)

// TextSource 只要「按路径读一份脚本文本」这一件事。
//
// 口子留在本包、实现在 gamedata：本包不关心脚本从哪来（PVF 归档 / 磁盘目录 /
// 内存 map 都行），由调用方注入 —— 与 attendance / honeytime 两个域同一取舍，
// 也让本包在无 PVF 的纯单测环境里照样可测。
type TextSource interface {
	Text(path string) (string, error)
}

// 奖励轨道（源 `[reward table]` 第 1 列）：0 = 普通、2 = 皇家。
const (
	TrackNormal = 0
	TrackRoyal  = 2
)

// 任务轨道：源里 `[normal mission]` 16 条对应 0x08A1 的第一张任务列表，
// `[season mission]` 12 条对应第二张。**与奖励轨道同名但语义不同**，别混用。
const (
	MissionTrackNormal = 0
	MissionTrackSeason = 2
)

// IsNormalDungeon 报告某副本是否在源 `[normal dungeon index list]` 里
// （`[normal mission] normal_dungeon_2` 只数这些图）。
func (c *Catalog) IsNormalDungeon(id uint32) bool {
	if c == nil {
		return false
	}
	for _, d := range c.NormalDungeons {
		if d == id {
			return true
		}
	}
	return false
}

// IsSchedule 报告某个活动 id 是不是战令。
func IsSchedule(id uint32) bool { return id == ClaimScheduleID || id == AltScheduleID }

// RewardAt 按「轨道 + 槽位」取一档奖励（源里 slot 从 1 起）。
func (c *Catalog) RewardAt(track, slot int) (Reward, bool) {
	if c == nil {
		return Reward{}, false
	}
	for _, r := range c.Rewards {
		if r.Track == track && r.Slot == slot {
			return r, true
		}
	}
	return Reward{}, false
}

// RewardAtLevel 按「轨道 + **等级**」取一档奖励。
//
// ★★ 2026-10-11 实机定位：**客户端在 CMD680 里给的"槽位"其实是等级**。
// 证据（都是偶数/奇数各自成立）：皇家轨奖励都在偶数级，玩家连点三格发来的是
// `2 / 6 / 8`；普通轨奖励都在奇数级，官服抓包那次发来的是 `1`。
// 若按源表里的"每轨序号"解，`6/8` 会被当成 Lv12/Lv16 而以"等级不足"拒绝 —— 
// 而玩家 9 级根本点不到那两格。⇒ 3 号字段 = **等级**，不是序号。
func (c *Catalog) RewardAtLevel(track, level int) (Reward, bool) {
	if c == nil {
		return Reward{}, false
	}
	for _, r := range c.Rewards {
		if r.Track == track && r.Level == level {
			return r, true
		}
	}
	return Reward{}, false
}

// MaxRewardLevel 是奖励表里出现过的最大等级（已领档位图的位数用它兜底）。
func (c *Catalog) MaxRewardLevel() int {
	if c == nil {
		return 0
	}
	max := 0
	for _, r := range c.Rewards {
		if r.Level > max {
			max = r.Level
		}
	}
	return max
}

// Mission 按「轨道 + 序号」取一条任务（轨道 0 = `[normal mission]`，2 = `[season mission]`）。
func (c *Catalog) Mission(track, index int) (Mission, bool) {
	if c == nil {
		return Mission{}, false
	}
	rows := c.NormalMissions
	if track == MissionTrackSeason {
		rows = c.SeasonMissions
	}
	for _, m := range rows {
		if m.Index == index {
			return m, true
		}
	}
	return Mission{}, false
}

// Level 是一级的经验门槛（累计）。
type Level struct {
	Level int
	Exp   uint32
}

// Reward 是奖励表的一行。
//
// 源格式（`newaradpassevent.evt` 的 `[reward table]`）是每行 7 个 token：
//
//	<track> <slot> <item> <count> <level> <title> <msg>
//
// 轨道 0 = 普通（26 档，等级 1,3,…,49,50），轨道 2 = 皇家（25 档，等级 2,4,…,50）。
type Reward struct {
	Track int    `json:"track"`
	Slot  int    `json:"slot"`
	Item  uint32 `json:"item"`
	Count uint32 `json:"count"`
	Level int    `json:"level"`
	Title string `json:"title,omitempty"`
}

// Mission 是一条通行证任务。
type Mission struct {
	Index int    `json:"index"`
	Key   string `json:"key"`
	Count uint32 `json:"count"`
	Exp   uint32 `json:"exp"`
}

// Catalog 是 `newaradpassevent.evt` 的结构化投影。
type Catalog struct {
	Path     string `json:"path"`
	SHA256   string `json:"-"`
	DBTable  string `json:"db_table"`
	CharTable string `json:"char_table"`

	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
	UIFile      string `json:"ui_file"`

	MaxLevelDisplay int      `json:"max_level_display"`
	PassItem        []uint32 `json:"pass_item,omitempty"`
	ExpandRoyal     []uint32 `json:"expand_royal,omitempty"`
	ExpandRental    []uint32 `json:"expand_rental,omitempty"`
	RemainDays      []int    `json:"remain_days,omitempty"`

	WeeklyGainableExp uint32 `json:"weekly_gainable_exp"`
	PurchaseExp       uint32 `json:"purchase_pass_exp"`

	Levels []Level `json:"levels"`

	DailyReward  []uint32 `json:"daily_reward,omitempty"`
	SeasonReward []uint32 `json:"season_reward,omitempty"`
	Rewards      []Reward `json:"rewards,omitempty"`

	NormalMissions []Mission `json:"normal_missions,omitempty"`
	SeasonMissions []Mission `json:"season_missions,omitempty"`

	RoyalFirstReward []uint32 `json:"royal_first_reward,omitempty"`
	NormalDungeons   []uint32 `json:"normal_dungeon_index,omitempty"`

	PassCharacterDiscount uint32 `json:"pass_character_discount,omitempty"`
}

// MaxExp 是满级累计经验（`[pass level info]` 的最后一条）。
func (c *Catalog) MaxExp() uint32 {
	if c == nil || len(c.Levels) == 0 {
		return 0
	}
	return c.Levels[len(c.Levels)-1].Exp
}

// LevelFor 把累计经验换算成等级。
//
// 口径与抓包实证一致：等级 = 1 + 已达成的门槛数。等级 1 的门槛是 0，
// 所以 exp=0 是 1 级、exp=600（第二档门槛）升到 2 级 —— 与 18 份 0x08A1 样本的
// (level, exp) 组合逐条吻合。
func (c *Catalog) LevelFor(exp uint32) uint32 {
	if c == nil || len(c.Levels) == 0 {
		return 1
	}
	level := uint32(1)
	for _, l := range c.Levels {
		if l.Level <= 1 {
			continue
		}
		if exp >= l.Exp {
			level = uint32(l.Level)
			continue
		}
		break
	}
	return level
}

// RewardsFor 返回某一轨在给定等级上可领取的奖励（轨道 0 = 普通，2 = 皇家）。
func (c *Catalog) RewardsFor(track int) []Reward {
	if c == nil {
		return nil
	}
	var out []Reward
	for _, r := range c.Rewards {
		if r.Track == track {
			out = append(out, r)
		}
	}
	return out
}

// Parse 解析 `newaradpassevent.evt`。
func Parse(text string) (*Catalog, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("arad pass: empty event script")
	}
	c := &Catalog{}
	c.DBTable, c.CharTable = firstStrings(section(text, "db table name"), 2)
	if period := section(text, "event period"); len(period) > 0 {
		c.PeriodStart = period[0]
		if len(period) > 1 {
			c.PeriodEnd = period[1]
		}
	}
	if ui := section(text, "ui file"); len(ui) > 0 {
		c.UIFile = ui[0]
	}
	if v, ok := firstUint(section(text, "max level display")); ok {
		c.MaxLevelDisplay = int(v)
	}
	c.PassItem = uints(section(text, "pass item index"))
	c.ExpandRoyal = uints(section(text, "expand royal item index"))
	c.ExpandRental = uints(section(text, "expand rental item index"))
	for _, v := range uints(section(text, "remain days warning")) {
		c.RemainDays = append(c.RemainDays, int(v))
	}
	if v, ok := firstUint(section(text, "weekly gainable exp")); ok {
		c.WeeklyGainableExp = v
	}
	if v, ok := firstUint(section(text, "purchase pass exp")); ok {
		c.PurchaseExp = v
	}
	tokens := tokenize(sectionRaw(text, "pass level info"))
	for i := 0; i+1 < len(tokens); i += 2 {
		lv, ok1 := tokens[i].Num()
		ex, ok2 := tokens[i+1].Num()
		if !ok1 || !ok2 || lv == 0 {
			break
		}
		c.Levels = append(c.Levels, Level{Level: int(lv), Exp: ex})
	}
	if len(c.Levels) < 2 {
		return nil, fmt.Errorf("arad pass: [pass level info] is missing or too short")
	}
	c.DailyReward = itemIds(section(text, "daily reward list"))
	c.SeasonReward = itemIds(section(text, "season reward list"))
	c.Rewards = parseRewardTable(tokenize(sectionRaw(text, "reward table")))
	if len(c.Rewards) == 0 {
		return nil, fmt.Errorf("arad pass: [reward table] is missing")
	}
	c.NormalMissions = parseMissions(tokenize(sectionRaw(text, "normal mission")))
	c.SeasonMissions = parseMissions(tokenize(sectionRaw(text, "season mission")))
	c.RoyalFirstReward = itemIds(section(text, "royalpass first reward"))
	c.NormalDungeons = uints(section(text, "normal dungeon index list"))
	if v, ok := firstUint(section(text, "pass character discount")); ok {
		c.PassCharacterDiscount = v
	}
	return c, nil
}

// parseRewardTable 解析 `[reward table]`。
//
// 每行**固定 7 个 token**，行首就是轨道号：
//
//	<track> <slot> <item> <count> <level> <title> <msg>
//
// 2026 第 3 赛季实测：357 个 token = 51 行 × 7（普通轨 track=0 共 26 行 +
// 皇家轨 track=2 共 25 行），轨道号在**每一行**都重复写，不是只在换轨处写一次。
// `msg` 是与 title 配对的文案键，行为上不需要，解析时跳过。
func parseRewardTable(tokens []token) []Reward {
	var out []Reward
	for i := 0; i+6 < len(tokens); i += 7 {
		track, ok := tokens[i].Num()
		if !ok {
			break
		}
		slot, ok := tokens[i+1].Num()
		if !ok {
			break
		}
		item, ok := tokens[i+2].Num()
		if !ok {
			break
		}
		count, _ := tokens[i+3].Num()
		level, _ := tokens[i+4].Num()
		out = append(out, Reward{
			Track: int(track), Slot: int(slot), Item: item, Count: count, Level: int(level),
			Title: tokens[i+5].Str,
		})
	}
	return out
}

// parseMissions 解析 `[normal mission]` / `[season mission]`：`<index> <key> <count> <exp>`。
func parseMissions(tokens []token) []Mission {
	var out []Mission
	for i := 0; i+3 < len(tokens); {
		index, ok1 := tokens[i].Num()
		key := tokens[i+1]
		count, ok2 := tokens[i+2].Num()
		exp, ok3 := tokens[i+3].Num()
		if !ok1 || key.Str == "" || !ok2 || !ok3 {
			break
		}
		out = append(out, Mission{Index: int(index), Key: key.Str, Count: count, Exp: exp})
		i += 4
	}
	return out
}

// ---- 脚本分词 ----

type token struct {
	Str string
}

func (t token) Num() (uint32, bool) {
	if t.Str == "" {
		return 0, false
	}
	v, e := strconv.ParseUint(t.Str, 10, 32)
	if e != nil {
		return 0, false
	}
	return uint32(v), true
}

// tokenize 把一段脚本切成 token：反引号字符串算一个 token，其余按空白切。
func tokenize(s string) []token {
	var out []token
	for i := 0; i < len(s); {
		ch := s[i]
		switch {
		case ch == '`':
			j := strings.IndexByte(s[i+1:], '`')
			if j < 0 {
				return out
			}
			out = append(out, token{Str: strings.TrimSpace(s[i+1 : i+1+j])})
			i += j + 2
		case ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n':
			i++
		default:
			j := i
			for j < len(s) && !strings.ContainsRune(" \t\r\n`", rune(s[j])) {
				j++
			}
			out = append(out, token{Str: s[i:j]})
			i = j
		}
	}
	return out
}

// sectionRaw 取出 `[name] … [/name]` 之间的原文（不含标记）。
//
// ★ 与移植来源的**有意分歧（修 bug）**：这段脚本里有一批段**没有收尾标签**
// （实测 `[pass item index]` / `[db table name]` / `[max level display]` /
// `[weekly gainable exp]` / `[purchase pass exp]` / `[ui file]`：下一个标签就是下一段），
// 而原实现找不到 `[/name]` 时把 **EOF 之前的所有内容**都当成这一段 ⇒ `PassItem` 会收进
// 后面所有段的数字（实测 20 项，`assert len(PassItem)==1` 当场报错）。
// 所以补一条：**没有收尾标签时，以下一个段标签为界**。
//
// ⚠️ 只认"行首（或前面是空白）的 `[`"，免得把值里本身带方括号的文本当成段边界。
func sectionRaw(text, name string) string {
	open := "[" + name + "]"
	closeTag := "[/" + name + "]"
	i := strings.Index(text, open)
	if i < 0 {
		return ""
	}
	rest := text[i+len(open):]
	if j := strings.Index(rest, closeTag); j >= 0 {
		return rest[:j]
	}
	for k := 0; k < len(rest); k++ {
		if rest[k] != '[' {
			continue
		}
		if k > 0 {
			switch rest[k-1] {
			case '\n', '\r', ' ', '\t':
			default:
				continue // 值里的 `[`，不是段边界
			}
		}
		return rest[:k]
	}
	return rest
}

// section 取出一个段的 token，且**只保留非数字 token 之外的字符串**留给调用方
// 用 firstStrings；数字段请用 uints 或 sectionRaw。
func section(text, name string) []string {
	var out []string
	for _, t := range tokenize(sectionRaw(text, name)) {
		out = append(out, t.Str)
	}
	return out
}

// itemIds 解析 `<物品> <数量> <标题> <文案>` 这种**四列一行**的列表，只取物品号。
//
// 这几个段（`[daily reward list]` / `[season reward list]` / `[royalpass first reward]`）
// 都是四列一行；直接用"取所有数字"的写法会把**数量列**（通常就是 1）也当成物品号 ——
// 实机 2026-10-04 就因此多发了几枚金币（模板 1 = 金币）。
func itemIds(tokens []string) []uint32 {
	var out []uint32
	for i := 0; i+1 < len(tokens); i += 4 {
		token := tokens[i]
		if _, e := strconv.ParseUint(tokens[i+1], 10, 32); e != nil {
			break // 形状不符就停，别把后面的标题当物品
		}
		v, e := strconv.ParseUint(token, 10, 32)
		if e != nil {
			break
		}
		if v != 0 {
			out = append(out, uint32(v))
		}
	}
	return out
}

func uints(tokens []string) []uint32 {
	var out []uint32
	for _, s := range tokens {
		if v, e := strconv.ParseUint(s, 10, 32); e == nil && v != 0 {
			out = append(out, uint32(v))
		}
	}
	return out
}

func firstUint(tokens []string) (uint32, bool) {
	for _, s := range tokens {
		if v, e := strconv.ParseUint(s, 10, 32); e == nil {
			return uint32(v), true
		}
	}
	return 0, false
}

func firstStrings(tokens []string, n int) (string, string) {
	var a, b string
	for _, s := range tokens {
		if _, e := strconv.ParseUint(s, 10, 32); e == nil {
			continue
		}
		if a == "" {
			a = s
			continue
		}
		if b == "" {
			b = s
			break
		}
	}
	return a, b
}
