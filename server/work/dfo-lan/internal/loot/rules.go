package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

type Rules struct {
	Model                          string    `json:"model"`
	ReferenceSHA256                string    `json:"reference_sha256"`
	Denominator                    uint32    `json:"denominator"`
	DifficultyBonus                []float64 `json:"difficulty_bonus"`
	SupportedKinds                 []string  `json:"supported_kinds"`
	ExcludedItemPaths              []string  `json:"excluded_item_path_prefixes"`
	MaximumPickupX, MaximumPickupY uint16
}

func LoadRules(p string) (Rules, error) {
	var r Rules
	b, e := os.ReadFile(p)
	if e != nil {
		return r, e
	}
	if e = json.Unmarshal(b, &r); e != nil {
		return r, e
	}
	if r.Model != "reference90-gold-stack-v1" || len(r.ReferenceSHA256) != 64 || r.Denominator != 10000 || len(r.DifficultyBonus) != 5 || r.MaximumPickupX == 0 || r.MaximumPickupY == 0 {
		return r, fmt.Errorf("invalid drop compatibility policy")
	}
	for _, x := range r.DifficultyBonus {
		if x <= 0 || math.IsNaN(x) || math.IsInf(x, 0) {
			return r, fmt.Errorf("invalid difficulty rate")
		}
	}
	seen := map[string]bool{}
	for _, kind := range r.SupportedKinds {
		if seen[kind] || (kind != "gold" && kind != "stackable" && kind != "equipment") {
			return r, fmt.Errorf("unsupported drop kind")
		}
		seen[kind] = true
	}
	if len(seen) == 0 {
		return r, fmt.Errorf("no enabled drop kinds")
	}
	return r, nil
}
func Cells(c []pvf.Token, name string) []pvf.Token {
	var out []pvf.Token
	active := false
	for _, t := range c {
		if t.Type == 3 {
			active = t.Text == name
			continue
		}
		if active {
			out = append(out, t)
		}
	}
	return out
}
func numbers(c []pvf.Token) ([]float64, error) {
	var v []float64
	for _, t := range c {
		switch t.Type {
		case 0:
			v = append(v, float64(t.Value))
		case 2:
			n, e := strconv.ParseFloat(strconv.FormatFloat(float64(t.Number), 'g', -1, 32), 64)
			if e != nil || math.IsNaN(n) || math.IsInf(n, 0) {
				return nil, fmt.Errorf("invalid source float")
			}
			v = append(v, n)
		default:
			return nil, fmt.Errorf("nonnumeric drop table")
		}
	}
	return v, nil
}

type Tables struct{ Probability, Gold, Grade, Rank, Rarity []float64 }

func Parse(c catalog.LootCatalog) (Tables, error) {
	var t Tables
	for _, f := range []struct {
		path, name string
		columns    int
		dst        *[]float64
	}{
		{"etc/itemdropinfo_monseter.etc", "[drop prob]", 7, &t.Probability},
		{"etc/itemdropinfo_common.etc", "[gold drop ref table]", 3, &t.Gold},
		{"etc/itemdropinfo_monseter.etc", "[item drop ref table]", 3, &t.Grade},
		{"etc/itemdropinfo_monseter.etc", "[monster type drop bonusrate]", 4, &t.Rank},
		{"etc/itemdropinfo_monseter.etc", "[basis of rarity dicision]", 9, &t.Rarity},
	} {
		a, e := numbers(Cells(c.Rules[f.path].Cells, f.name))
		if e != nil || len(a) == 0 || len(a)%f.columns != 0 {
			return t, fmt.Errorf("invalid source %s: %v", f.name, e)
		}
		*f.dst = a
	}
	if len(t.Rank) != 20 || len(t.Rarity) != 36 {
		return t, fmt.Errorf("unsupported source drop table shape")
	}
	return t, nil
}

type RNG struct{ Seed uint32 }

func (r *RNG) Next(n uint32) uint32 {
	a := r.Seed*1103515245 + 12345
	b := a*1103515245 + 12345
	c := b*1103515245 + 12345
	r.Seed = c
	v := ((((a>>16)&0x7ff)<<10)^((b>>16)&0x3ff))<<10 | ((c >> 16) & 0x3ff)
	if n > 0 {
		return v % n
	}
	return v
}

type Award struct{ Template, Amount uint32 }
type Outcome struct {
	Awards       []Award
	SkippedKinds []string
	NextSeed     uint32
}

// Roll ports the explicitly named reference formula. The equipment category
// selects from the caller's bag-usable gear pool, matched on the same source
// rarity roll and grade window as stackables; the item dictionary's own
// generation weights are still unrecovered, so selection inside that window is
// uniform rather than dictionary-weighted. This is not official-server parity.
func Roll(c catalog.LootCatalog, t Tables, r Rules, pool []inventory.EquipmentDrop, seed uint32, level, rank, difficulty byte) (Outcome, error) {
	var out Outcome
	if len(t.Rank) != 20 || len(t.Rarity) != 36 || len(t.Probability)%7 != 0 || len(t.Gold)%3 != 0 || len(t.Grade)%3 != 0 || r.Denominator == 0 {
		return out, fmt.Errorf("invalid drop model tables")
	}
	enabled := map[string]bool{}
	for _, kind := range r.SupportedKinds {
		enabled[kind] = true
	}
	if level == 0 || rank > 3 || int(difficulty) >= len(r.DifficultyBonus) || uint32(level)+3 > c.MaximumGrade {
		return out, fmt.Errorf("drop source range is not imported")
	}
	var prob, gold, grade []float64
	for i := 0; i < len(t.Probability); i += 7 {
		v := t.Probability[i : i+7]
		if float64(level) >= v[0] && float64(level) <= v[1] {
			if prob != nil {
				return out, fmt.Errorf("overlapping drop ranges")
			}
			prob = v[2:]
		}
	}
	for i := 0; i < len(t.Gold); i += 3 {
		if t.Gold[i] == float64(level) {
			gold = t.Gold[i : i+3]
		}
	}
	for i := 0; i < len(t.Grade); i += 3 {
		if t.Grade[i] == float64(level) {
			grade = t.Grade[i : i+3]
		}
	}
	if len(prob) != 5 || gold == nil || grade == nil {
		return out, fmt.Errorf("missing source drop level")
	}
	if gold[1] <= 0 || gold[2] < 0 || gold[2] > 100 {
		return out, fmt.Errorf("invalid source gold")
	}
	rng := RNG{seed}
	diff := r.DifficultyBonus[difficulty]
	rate := func(category int) uint32 {
		n := math.Floor(prob[category] * t.Rank[category*4+int(rank)] * diff)
		if n < 0 {
			return 0
		}
		if n > float64(r.Denominator) {
			return r.Denominator
		}
		return uint32(n)
	}
	amount := int64(gold[1])
	if gold[2] > 0 {
		amount += (int64(rng.Next(uint32(gold[2])*2+1)) - int64(gold[2])) * int64(gold[1]) / 100
	}
	a := math.Floor(float64(amount) * diff)
	if a < 1 || a > math.MaxUint32 {
		return out, fmt.Errorf("gold amount overflow")
	}
	if rng.Next(r.Denominator) < rate(0) && enabled["gold"] {
		out.Awards = append(out.Awards, Award{0, uint32(a)})
	}
	for category := 1; category <= 3; category++ {
		if rng.Next(r.Denominator) >= rate(category) {
			continue
		}
		roll := float64(rng.Next(1000000) + 1)
		rarity := int32(0)
		for i, v := range t.Rarity[:9] {
			if roll <= v {
				rarity = int32(i)
				break
			}
		}
		if category == 2 {
			if !enabled["equipment"] {
				out.SkippedKinds = append(out.SkippedKinds, "equipment_kind_disabled")
				continue
			}
			gear := equipmentCandidates(pool, rarity, level, grade)
			if len(gear) == 0 && rarity > 0 {
				gear = equipmentCandidates(pool, 0, level, grade)
			}
			if len(gear) == 0 {
				out.SkippedKinds = append(out.SkippedKinds, "equipment_grade_window_empty")
				continue
			}
			out.Awards = append(out.Awards, Award{gear[rng.Next(uint32(len(gear)))].ID, 1})
			continue
		}
		candidates := func(rare int32) []catalog.LootItem {
			var ids []catalog.LootItem
			for _, item := range c.Items {
				excluded := false
				for _, prefix := range r.ExcludedItemPaths {
					excluded = excluded || strings.HasPrefix(item.Script.Path, prefix)
				}
				if excluded {
					continue
				}
				if item.Kind == "stackable" && item.Rarity == rare && float64(item.Grade) >= float64(level)-grade[1] && float64(item.Grade) < float64(level)+grade[2] {
					ids = append(ids, item)
				}
			}
			sort.Slice(ids, func(i, j int) bool {
				if ids[i].Grade != ids[j].Grade {
					return ids[i].Grade < ids[j].Grade
				}
				return ids[i].ID < ids[j].ID
			})
			return ids
		}
		ids := candidates(rarity)
		if len(ids) == 0 && rarity > 0 {
			ids = candidates(0)
		}
		if len(ids) > 0 {
			id := ids[rng.Next(uint32(len(ids)))].ID
			if enabled["stackable"] {
				out.Awards = append(out.Awards, Award{id, 1})
			}
		}
	}
	out.NextSeed = rng.Seed
	return out, nil
}

// equipmentCandidates applies the same grade window the stackable branch uses,
// so gear tracks the monster's level exactly as ordinary drops do.
func equipmentCandidates(pool []inventory.EquipmentDrop, rarity int32, level byte, grade []float64) []inventory.EquipmentDrop {
	var out []inventory.EquipmentDrop
	for _, d := range pool {
		if d.Rarity != rarity {
			continue
		}
		if float64(d.Grade) >= float64(level)-grade[1] && float64(d.Grade) < float64(level)+grade[2] {
			out = append(out, d)
		}
	}
	return out
}
