package adventure

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"
)

//go:embed season_rules.json
var seasonRulesJSON []byte

type SeasonLevel struct {
	Level uint32 `json:"level"`
	Upper uint32 `json:"upper"`
	Fame  uint32 `json:"fame"`
}
type SeasonContent struct {
	ID           uint32            `json:"id"`
	Category     uint32            `json:"category"`
	DefaultExp   int64             `json:"default_exp"`
	Difficulties map[uint32]uint32 `json:"difficulties"`
	Rule         uint32            `json:"rule"`
	CSOnly       bool              `json:"cs_only"`
}
type SeasonPenalty struct {
	MinimumLevel uint32      `json:"minimum_level"`
	MaximumLevel uint32      `json:"maximum_level"`
	Rule         uint32      `json:"rule"`
	DefaultRatio uint32      `json:"default_ratio"`
	Ranges       [][3]uint32 `json:"ranges"`
}
type SeasonReward struct {
	Level    uint32 `json:"level"`
	Mask     uint32 `json:"mask"`
	Template uint32 `json:"template"`
	Count    uint32 `json:"count"`
}
type SeasonCapsule struct {
	ID           uint32 `json:"id"`
	Category     uint32 `json:"category"`
	Difficulty   uint32 `json:"difficulty"`
	MinimumLevel int    `json:"minimum_level"`
	Path         string `json:"path"`
}
type SeasonRules struct {
	specialStart    time.Time
	specialEnd      time.Time
	specialTemplate uint32
	OathCost        struct {
		Gold      uint32 `json:"gold"`
		Materials []struct {
			Template uint32 `json:"template"`
			Count    uint32 `json:"count"`
		} `json:"materials"`
	} `json:"oath_cost"`
	Capsules        map[uint32]SeasonCapsule `json:"capsules"`
	Season          uint32                   `json:"season"`
	MinimumLevel    byte                     `json:"minimum_level"`
	DisplayMaxLevel uint32                   `json:"display_max_level"`
	MaxAcquisitions uint32                   `json:"max_acquisitions"`
	Levels          []SeasonLevel            `json:"levels"`
	Penalties       []SeasonPenalty          `json:"penalties"`
	Contents        []SeasonContent          `json:"contents"`
	Rewards         []SeasonReward           `json:"rewards"`
	Items           map[uint32]Item          `json:"items"`
	OathEquipment   []uint32                 `json:"oath_equipment"`
	SpecialReward   []string                 `json:"special_reward"`
	SHA256          string                   `json:"sha256"`
}

var loadSeason = sync.OnceValues(func() (*SeasonRules, error) {
	var r SeasonRules
	if err := json.Unmarshal(seasonRulesJSON, &r); err != nil {
		return nil, err
	}
	if r.Season == 0 || r.MinimumLevel == 0 || len(r.Levels) == 0 || r.DisplayMaxLevel == 0 || int(r.DisplayMaxLevel+r.MaxAcquisitions) != len(r.Levels) {
		return nil, fmt.Errorf("迷雾誓约等级源不完整")
	}
	for i, level := range r.Levels {
		if level.Level != uint32(i+1) || i > 0 && level.Upper <= r.Levels[i-1].Upper {
			return nil, fmt.Errorf("迷雾誓约经验阈值无效")
		}
	}
	var mask uint32
	for _, reward := range r.Rewards {
		if reward.Level == 0 || reward.Level > r.DisplayMaxLevel || reward.Count == 0 || reward.Mask == 0 || mask&reward.Mask != 0 || r.Items[reward.Template].Path == "" {
			return nil, fmt.Errorf("迷雾誓约奖励源不完整")
		}
		mask |= reward.Mask
	}
	if len(r.OathEquipment) == 0 || len(r.OathCost.Materials) == 0 {
		return nil, fmt.Errorf("誓约装备及费用源不完整")
	}
	if len(r.SpecialReward) != 3 {
		return nil, fmt.Errorf("迷雾30阶活动规则不完整")
	}
	var err error
	r.specialStart, err = time.Parse("2006-01-02 15:04:05", r.SpecialReward[0])
	if err != nil {
		return nil, err
	}
	r.specialEnd, err = time.Parse("2006-01-02 15:04:05", r.SpecialReward[1])
	if err != nil {
		return nil, err
	}
	template, err := strconv.ParseUint(r.SpecialReward[2], 10, 32)
	if err != nil {
		return nil, err
	}
	r.specialTemplate = uint32(template)
	if r.Items[r.specialTemplate].Path == "" || !r.specialEnd.After(r.specialStart) {
		return nil, fmt.Errorf("迷雾30阶活动奖励无效")
	}
	return &r, nil
})

func CurrentSeason() (*SeasonRules, error) { return loadSeason() }

func (r *SeasonRules) SpecialRewardAt(s SeasonState, now time.Time) (uint32, bool) {
	return r.specialTemplate, !s.SpecialClaimed && r.DisplayLevel(s) >= 30 && !now.Before(r.specialStart) && now.Before(r.specialEnd)
}

type SeasonHistory struct {
	ID         uint32 `json:"id"`
	Category   uint32 `json:"category"`
	Count      uint32 `json:"count"`
	Experience uint32 `json:"experience"`
	CSOnly     bool   `json:"cs_only,omitempty"`
}
type OathAcquisition struct {
	Template uint32 `json:"template"`
	Time     int64  `json:"time"`
	Name     string `json:"name"`
}
type SeasonState struct {
	Season         uint32            `json:"season"`
	Experience     uint32            `json:"experience"`
	RewardMask     uint32            `json:"reward_mask,omitempty"`
	Week           string            `json:"week,omitempty"`
	History        []SeasonHistory   `json:"history,omitempty"`
	Acquisitions   []OathAcquisition `json:"acquisitions,omitempty"`
	SpecialClaimed bool              `json:"special_claimed,omitempty"`
}

// 采用当前客户端周活动使用的周二09:00 UTC边界，与操作系统时区无关。
func SeasonWeek(now time.Time) string {
	t := now.UTC().Add(-9 * time.Hour)
	days := (int(t.Weekday()) - int(time.Tuesday) + 7) % 7
	return t.AddDate(0, 0, -days).Format("2006-01-02")
}

func (r *SeasonRules) Normalize(s *SeasonState, now time.Time) error {
	if s.Season == 0 {
		s.Season = r.Season
	}
	if s.Season != r.Season {
		return fmt.Errorf("迷雾誓约赛季不匹配，保留旧存档等待迁移")
	}
	if s.Experience > r.Levels[len(r.Levels)-1].Upper || len(s.Acquisitions) > int(r.MaxAcquisitions) {
		return fmt.Errorf("迷雾誓约存档超出源上限")
	}
	week := SeasonWeek(now)
	if s.Week < week {
		s.Week, s.History = week, nil
	}
	if len(s.History) > len(r.Contents) {
		return fmt.Errorf("迷雾誓约历史条目数无效")
	}
	seen := map[[3]uint32]bool{}
	for _, h := range s.History {
		cs := uint32(0)
		if h.CSOnly {
			cs = 1
		}
		key := [3]uint32{h.ID, h.Category, cs}
		if _, ok := r.Content(h.ID, h.Category, h.CSOnly); !ok || seen[key] || h.Count == 0 || h.Experience == 0 {
			return fmt.Errorf("迷雾誓约历史来源无效")
		}
		seen[key] = true
	}
	return nil
}

// 原生0x140569800逐档使用累计经验 <= 上界；不是角色等级，也不是普通角色经验。
func (r *SeasonRules) Level(s SeasonState) uint32 {
	i := sort.Search(len(r.Levels), func(i int) bool { return s.Experience <= r.Levels[i].Upper })
	if i == len(r.Levels) {
		return r.Levels[len(r.Levels)-1].Level
	}
	return r.Levels[i].Level
}
func (r *SeasonRules) DisplayLevel(s SeasonState) uint32 { return min(r.Level(s), r.DisplayMaxLevel) }

func (r *SeasonRules) Content(id, category uint32, cs bool) (SeasonContent, bool) {
	for _, content := range r.Contents {
		if content.ID == id && (category == 0 || content.Category == category) && content.CSOnly == cs {
			return content, true
		}
	}
	return SeasonContent{}, false
}

// UI将高级地下城和军团归入同一经验槽；衰减不能按单张地图分别累计。
func seasonSlot(category uint32) uint32 {
	switch category {
	case 1:
		return 1
	case 2, 3:
		return 2
	case 4:
		return 3
	case 6:
		return 4
	default:
		return 5
	}
}

func (r *SeasonRules) Gain(s *SeasonState, content SeasonContent, difficulty uint32, now time.Time) (uint32, error) {
	if err := r.Normalize(s, now); err != nil {
		return 0, err
	}
	base, ok := content.Difficulties[difficulty]
	if content.DefaultExp >= 0 {
		base, ok = uint32(content.DefaultExp), true
	}
	if !ok {
		return 0, fmt.Errorf("迷雾誓约没有该玩法难度的经验规则")
	}
	var weekly uint64
	for _, h := range s.History {
		if seasonSlot(h.Category) == seasonSlot(content.Category) {
			weekly += uint64(h.Experience)
		}
	}
	ratio := uint32(100)
	level := r.Level(*s)
	for _, p := range r.Penalties {
		if p.Rule != content.Rule || level < p.MinimumLevel || level > p.MaximumLevel {
			continue
		}
		ratio = p.DefaultRatio
		for _, span := range p.Ranges {
			if weekly >= uint64(span[0]) && weekly <= uint64(span[1]) {
				ratio = span[2]
				break
			}
		}
		break
	}
	amount := uint32(uint64(base) * uint64(ratio) / 100)
	// 原生0x14593730C以整张经验表末项判断道具是否已满；100阶只是展示上限。
	// 超出当前兑换门槛的经验继续保留，不能因还未兑换装备而吞掉经验。
	capExp := r.Levels[len(r.Levels)-1].Upper
	if s.Experience >= capExp {
		amount = 0
	} else {
		amount = min(amount, capExp-s.Experience)
	}
	if amount == 0 {
		return 0, nil
	}
	s.Experience += amount
	for i := range s.History {
		h := &s.History[i]
		if h.ID == content.ID && h.Category == content.Category && h.CSOnly == content.CSOnly {
			h.Count++
			h.Experience += amount
			return amount, nil
		}
	}
	s.History = append(s.History, SeasonHistory{ID: content.ID, Category: content.Category, Count: 1, Experience: amount, CSOnly: content.CSOnly})
	return amount, nil
}

func ReadSeason(raw json.RawMessage) (SeasonState, error) {
	var fields struct {
		Season SeasonState `json:"season_level"`
	}
	err := json.Unmarshal(raw, &fields)
	return fields.Season, err
}
func SaveSeason(raw json.RawMessage, s SeasonState) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("角色存档不是JSON对象")
	}
	value, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	fields["season_level"] = value
	return json.Marshal(fields)
}

// 道具携带的是玩法类别、来源编号和难度档；经验始终查源表，不相信客户端数量。
func ApplySeasonCapsule(raw json.RawMessage, template uint32, now time.Time) (json.RawMessage, uint32, error) {
	rules, err := CurrentSeason()
	if err != nil {
		return nil, 0, err
	}
	capsule, ok := rules.Capsules[template]
	if !ok {
		return raw, 0, nil
	}
	var role struct {
		Level  byte        `json:"level"`
		Season SeasonState `json:"season_level"`
	}
	if err = json.Unmarshal(raw, &role); err != nil {
		return nil, 0, err
	}
	if role.Level < rules.MinimumLevel || int(role.Level) < capsule.MinimumLevel {
		return nil, 0, fmt.Errorf("角色未达到迷雾誓约经验道具使用等级")
	}
	content, ok := rules.Content(capsule.ID, capsule.Category, true)
	if !ok {
		return nil, 0, fmt.Errorf("经验道具没有对应的迷雾誓约源规则")
	}
	gain, err := rules.Gain(&role.Season, content, capsule.Difficulty, now)
	if err != nil {
		return nil, 0, err
	}
	if gain == 0 {
		return nil, 0, fmt.Errorf("迷雾誓约经验已满或当前不能获得经验，不扣除道具")
	}
	next, err := SaveSeason(raw, role.Season)
	return next, gain, err
}
