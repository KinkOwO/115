package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
)

// OdysseySource 是「奥德赛系目录来自哪一份 PVF」的身份令牌。
//
// 2026-10-01（next146）：由 const 改为 var，并提供 SetOdysseySource。
// 原因：PVF 直读模式下所有目录都从**当次内层归档**（checksum 每次重建都变）构建，
// 而角色的 ConfigVersion 也等于该内层 checksum（`character/service.go` 的
// `ConfigVersion: s.Catalog.Source.Checksum`）。若此处仍钉死历史常量
// `7ef2db59…`，奥德赛全家族（成长/章节/路线/兑换/黑鸦/赤红铁矿…）的
// 「源身份」门禁会在直读启动时全部失败 —— 实机首个撞墙点是
// `Odyssey journal routes source mismatch`。
//
// 启动时由 PVF 准备阶段调用 SetOdysseySource(实际内层 checksum) 覆盖；
// 未调用（JSON/历史模式）时保持历史常量，行为不变。
//
// ⚠️ 它同时是**写入存档的运行时身份**（odyssey_chapter 的 CommitCharacterEvent 用它作
// 乐观锁令牌、比较 role.ConfigVersion），所以必须与当次真实内层一致，不能只做「忽略比较」。
var OdysseySource = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"

// SetOdysseySource 把奥德赛源身份切到当次实际内层 checksum。
// 只接受 64 位 hex；非法值忽略（保持历史常量），避免把空串/短串写进存档身份。
func SetOdysseySource(checksum string) {
	if len(checksum) != 64 {
		return
	}
	for i := 0; i < len(checksum); i++ {
		c := checksum[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return
		}
	}
	OdysseySource = checksum
}

// OdysseyGraduateRewardTemplate pins the [complete reward info] [reward]
// template: the booster box a graduated character receives once.
const OdysseyGraduateRewardTemplate uint32 = 10420561

type OdysseyGrowth struct {
	Creation     *OdysseyCreateRewards   `json:"-"`
	LevelActions map[byte][]string       `json:"-"`
	Source       string                  `json:"source"`
	Definition   ScriptRecord            `json:"definition"`
	Items        map[uint32]ScriptRecord `json:"items"`
	ClearLevels  map[uint32]byte         `json:"-"`
	EntryLevels  map[uint32]byte         `json:"-"`
	Gifts        map[byte]uint32         `json:"-"`
	// Quests mirrors the [quest clear]/[remove clear quest]/[show quest]/
	// [branch quest] tables of the same .etc file (graduation mainline plan).
	Quests *OdysseyQuests `json:"-"`
	// GraduateReward is the [complete reward info] [reward] template: the
	// booster box granted once a character graduates (P3 subitem 8).
	GraduateReward uint32 `json:"-"`
}

func LoadOdysseyGrowth(path string) (*OdysseyGrowth, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var r OdysseyGrowth
	if e = json.Unmarshal(b, &r); e != nil {
		return nil, e
	}
	return NewOdysseyGrowth(r)
}

// NewOdysseyGrowth shares validation and runtime index construction across native and JSON sources.
func NewOdysseyGrowth(r OdysseyGrowth) (*OdysseyGrowth, error) {
	if r.Source != OdysseySource || r.Definition.SHA256 != "638e71ab8fdc84b4be28db8ca3302fd1dfe689a9b771907514297edee4b8c8e8" {
		return nil, fmt.Errorf("Odyssey source mismatch")
	}
	r.ClearLevels = map[uint32]byte{}
	r.EntryLevels = map[uint32]byte{}
	r.Gifts = map[byte]uint32{}
	for _, name := range []string{"[grow up level on dungeon clear]", "[dungeon list by level]", "[reward info]"} {
		c := sectionCells(r.Definition.Cells, name)
		if len(c) == 0 || len(c)%2 != 0 {
			return nil, fmt.Errorf("invalid Odyssey table %s", name)
		}
		for i := 0; i < len(c); i += 2 {
			a, b := c[i], c[i+1]
			if a.Type != 0 || b.Type != 0 || a.Value <= 0 || b.Value <= 0 {
				return nil, fmt.Errorf("invalid Odyssey pair")
			}
			if name == "[grow up level on dungeon clear]" {
				if b.Value > 115 || r.ClearLevels[uint32(a.Value)] != 0 {
					return nil, fmt.Errorf("invalid clear level")
				}
				r.ClearLevels[uint32(a.Value)] = byte(b.Value)
			} else {
				if a.Value > 115 {
					return nil, fmt.Errorf("invalid reward/entry level")
				}
				if name == "[reward info]" {
					if r.Gifts[byte(a.Value)] != 0 {
						return nil, fmt.Errorf("duplicate gift level")
					}
					r.Gifts[byte(a.Value)] = uint32(b.Value)
				} else {
					if r.EntryLevels[uint32(b.Value)] != 0 {
						return nil, fmt.Errorf("duplicate dungeon")
					}
					r.EntryLevels[uint32(b.Value)] = byte(a.Value)
				}
			}
		}
	}
	if len(r.ClearLevels) != 50 || len(r.EntryLevels) != 50 || len(r.Gifts) != 3 {
		return nil, fmt.Errorf("incomplete Odyssey tables")
	}
	for id, target := range r.ClearLevels {
		if r.EntryLevels[id] == 0 || target <= r.EntryLevels[id] {
			return nil, fmt.Errorf("invalid Odyssey level progression")
		}
	}
	for _, id := range r.Gifts {
		if r.Items[id].SHA256 == "" {
			return nil, fmt.Errorf("missing Odyssey gift script")
		}
	}
	// Graduation reward: the [complete reward info] [reward] template must be
	// a real [booster info] box script, distinct from every level gift.
	graduate, e := sectionRewardTemplate(r.Definition.Cells, "[complete reward info]")
	if e != nil {
		return nil, e
	}
	r.GraduateReward = graduate
	script, ok := r.Items[graduate]
	if !ok || script.SHA256 == "" {
		return nil, fmt.Errorf("missing Odyssey graduation reward script %d", graduate)
	}
	if !hasToken(script.Cells, "[booster info]") {
		return nil, fmt.Errorf("Odyssey graduation reward %d is not a [booster info] box", graduate)
	}
	for _, gift := range r.Gifts {
		if gift == graduate {
			return nil, fmt.Errorf("Odyssey graduation reward duplicates level gift %d", gift)
		}
	}
	// Quest tables: parsed and pinned against the source shape; see
	// odyssey_quests.go for the exact assertions.
	r.Quests, e = loadOdysseyQuests(r.Definition.Cells)
	if e != nil {
		return nil, e
	}
	r.LevelActions, e = ParseOdysseyLevelActions(r.Definition.Cells)
	if e != nil {
		return nil, e
	}
	return &r, nil
}

// sectionRewardTemplate extracts the template id that follows the [reward]
// tag inside the named section.
func sectionRewardTemplate(cells []pvf.Token, name string) (uint32, error) {
	start, end, ok := findSectionBounds(cells, name)
	if !ok {
		return 0, fmt.Errorf("missing Odyssey section %s", name)
	}
	for i := start + 1; i < end; i++ {
		if cells[i].Type == 3 && cells[i].Text == "[reward]" {
			if i+1 >= end || cells[i+1].Type != 0 || cells[i+1].Value <= 0 {
				return 0, fmt.Errorf("invalid [reward] inside %s", name)
			}
			return uint32(cells[i+1].Value), nil
		}
	}
	return 0, fmt.Errorf("missing [reward] inside %s", name)
}

func hasToken(cells []pvf.Token, text string) bool {
	for _, c := range cells {
		if c.Type == 3 && c.Text == text {
			return true
		}
	}
	return false
}
