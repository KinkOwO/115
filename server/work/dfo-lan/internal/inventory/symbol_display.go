package inventory

import (
	"dfolan/internal/catalog"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// SymbolAttributes projects the very same [enchant] cells that the client's
// native option collector consumes. It is presentation only, not a second
// gameplay rule engine. Keep skill identities separate instead of adding
// unrelated skill levels together.
type SymbolAttributes map[string]float64

func (out SymbolAttributes) Add(s catalog.ScriptRecord, skillsOnly bool) {
	inEnchant, field := false, ""
	for i := 0; i < len(s.Cells); i++ {
		t := s.Cells[i]
		if t.Type == 3 {
			if t.Text == "[enchant]" {
				inEnchant = true
			}
			if t.Text == "[/enchant]" {
				inEnchant = false
			}
			field = t.Text
			continue
		}
		if !inEnchant {
			continue
		}
		if field == "[skill levelup]" {
			if t.Type == 6 && i+2 < len(s.Cells) && s.Cells[i+1].Type == 0 && s.Cells[i+2].Type == 0 {
				out[fmt.Sprintf("skill:%s:%d", t.Text, s.Cells[i+1].Value)] += float64(s.Cells[i+2].Value)
				i += 2
			}
			continue
		}
		if skillsOnly {
			continue
		}
		if t.Type == 0 {
			out[field] += float64(t.Value)
		} else if t.Type == 2 {
			v := float64(math.Float32frombits(uint32(t.Value)))
			if !math.IsNaN(v) && !math.IsInf(v, 0) {
				out[field] += v
			}
		}
	}
}
func (out SymbolAttributes) Merge(other SymbolAttributes) {
	for k, v := range other {
		out[k] += v
	}
}
func (out SymbolAttributes) Text() string {
	type field struct {
		key, label string
		scale      float64
		suffix     string
	}
	fields := []field{
		{"[physical attack]", "力量", 1, ""}, {"[magical attack]", "智力", 1, ""},
		{"[physical defense]", "体力", 1, ""}, {"[magical defense]", "精神", 1, ""},
		{"[attack speed]", "攻击速度", .1, "%"}, {"[cast speed]", "施放速度", .1, "%"}, {"[move speed]", "移动速度", .1, "%"},
		{"[physical critical hit]", "物理暴击率", 1, "%"}, {"[magical critical hit]", "魔法暴击率", 1, "%"},
		{"[hit recovery]", "硬直", 1, ""}, {"[inventory limit]", "负重上限", .001, "kg"},
		{"[hp max]", "HP最大值", 1, ""}, {"[mp max]", "MP最大值", 1, ""},
		{"[jump]", "跳跃力", 1, ""}, {"[fame value]", "冒险家名望", 1, ""},
	}
	var lines []string
	seen := map[string]bool{}
	for _, f := range fields {
		seen[f.key] = true
		if n := out[f.key] * f.scale; n != 0 {
			lines = append(lines, f.label+" "+fmt.Sprintf("%+g", math.Round(n*1000)/1000)+f.suffix)
		}
	}
	var keys []string
	for k, v := range out {
		if !seen[k] && v != 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.HasPrefix(k, "skill:") {
			lines = append(lines, "技能等级 +"+strconv.FormatFloat(out[k], 'g', -1, 64)+"（悬停徽章查看）")
		} else {
			// Preserve a source field we cannot translate instead of silently dropping
			// its contribution or assigning it to a different attribute.
			lines = append(lines, strings.Trim(k, "[]")+" "+fmt.Sprintf("%+g", out[k]))
		}
	}
	if len(lines) == 0 {
		return "无属性加成"
	}
	return strings.Join(lines, "  ·  ")
}
