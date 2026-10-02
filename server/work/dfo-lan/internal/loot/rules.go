package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

// ErrOutOfDropRange reports that the imported drop model does not cover the
// monster being rolled: its level sits above the catalog's import ceiling, or the
// source tables carry no row for it.
//
// It is a data-coverage gap, not a defect, so callers are expected to treat it as
// "this monster pays nothing" rather than failing the request that reported the
// death. Withholding a confirmed death for this reason leaves the client's
// monsters alive and the gate unopened - the failure the FFFF killer arm in
// dungeon.ConfirmDeath documents. Defects of the model itself (a malformed table, a
// source inconsistency, an exhausted drop identity) deliberately do NOT carry this
// sentinel: those must still fail loudly.
var ErrOutOfDropRange = errors.New("drop source range is not imported")

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

type Tables struct {
	Probability, Gold, Grade, Rank, Rarity []float64
	// Five categories, each containing five difficulty columns in the PVF.
	Difficulty []float64
}

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
	if cells := Cells(c.Rules["etc/itemdropinfo_monseter.etc"].Cells, "[dungeon difficulty drop bonusrate]"); len(cells) > 0 {
		var err error
		t.Difficulty, err = numbers(cells)
		if err != nil || len(t.Difficulty) != 25 {
			return t, fmt.Errorf("invalid source difficulty table: %v", err)
		}
		for _, rate := range t.Difficulty {
			if rate < 0 {
				return t, fmt.Errorf("negative source difficulty multiplier")
			}
		}
	} else if c.ClearReward != nil {
		return t, fmt.Errorf("PVF ordinary difficulty table missing")
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
	// A declared map pool must receive successful item rolls even when the
	// generic pool has no candidate at the rolled grade/rarity.
	ItemBudget int
}

// Roll ports the explicitly named reference formula. The equipment category
// selects from the caller's bag-usable gear pool, matched on the same source
// rarity roll and grade window as stackables; the item dictionary's own
// generation weights are still unrecovered, so selection inside that window is
// uniform rather than dictionary-weighted. This is not official-server parity.
func Roll(c catalog.LootCatalog, t Tables, r Rules, pool []inventory.EquipmentDrop, seed uint32, level, rank, difficulty byte) (Outcome, error) {
	return RollWithBonus(c, t, r, pool, seed, level, rank, difficulty, 0)
}

func RollWithBonus(c catalog.LootCatalog, t Tables, r Rules, pool []inventory.EquipmentDrop, seed uint32, level, rank, difficulty byte, questDropBonusPercent int) (Outcome, error) {
	return rollWithBonus(c, t, r, pool, seed, level, rank, difficulty, questDropBonusPercent, false)
}

// RollOrdinary uses current source difficulty columns. The explicitly named
// compatibility formula and the separately confirmed special-mode sequence
// remain distinct; this does not claim an official 115 server formula.
func RollOrdinary(c catalog.LootCatalog, t Tables, r Rules, pool []inventory.EquipmentDrop, seed uint32, level, rank, difficulty byte, questDropBonusPercent int) (Outcome, error) {
	if len(t.Difficulty) != 25 || difficulty >= 5 {
		return Outcome{}, fmt.Errorf("ordinary source difficulty unavailable")
	}
	for _, rate := range t.Difficulty {
		if rate < 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
			return Outcome{}, fmt.Errorf("invalid ordinary source multiplier")
		}
	}
	return rollWithBonus(c, t, r, pool, seed, level, rank, difficulty, questDropBonusPercent, true)
}

func rollWithBonus(c catalog.LootCatalog, t Tables, r Rules, pool []inventory.EquipmentDrop, seed uint32, level, rank, difficulty byte, questDropBonusPercent int, sourceDifficulty bool) (Outcome, error) {
	var out Outcome
	if len(t.Rank) != 20 || len(t.Rarity) != 36 || len(t.Probability)%7 != 0 || len(t.Gold)%3 != 0 || len(t.Grade)%3 != 0 || r.Denominator == 0 {
		return out, fmt.Errorf("invalid drop model tables")
	}
	enabled := map[string]bool{}
	for _, kind := range r.SupportedKinds {
		enabled[kind] = true
	}
	if level == 0 || rank > 3 || int(difficulty) >= len(r.DifficultyBonus) || !sourceDifficulty && uint32(level)+3 > c.MaximumGrade {
		return out, ErrOutOfDropRange
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
		return out, fmt.Errorf("%w: missing source drop level", ErrOutOfDropRange)
	}
	if gold[1] <= 0 || gold[2] < 0 || gold[2] > 100 {
		return out, fmt.Errorf("invalid source gold")
	}
	rng := RNG{seed}
	diff := r.DifficultyBonus[difficulty]
	categoryDifficulty := func(category int) float64 {
		if sourceDifficulty {
			return t.Difficulty[category*5+int(difficulty)]
		}
		return diff
	}
	rate := func(category int) uint32 {
		bonus := 1.0
		if category == 3 && questDropBonusPercent > 0 {
			bonus += float64(questDropBonusPercent) / 100.0
		}
		n := math.Floor(prob[category] * t.Rank[category*4+int(rank)] * categoryDifficulty(category) * bonus)
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
	a := math.Floor(float64(amount) * categoryDifficulty(0))
	if sourceDifficulty && categoryDifficulty(0) == 0 {
		a = 1
	} // zero source multiplier also makes gold probability zero
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
		if sourceDifficulty && (category == 2 && enabled["equipment"] || category != 2 && enabled["stackable"]) {
			out.ItemBudget++
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
			if len(gear) == 0 && !sourceDifficulty {
				gear = equipmentCandidatesNearest(pool, rarity, level, grade)
			}
			if len(gear) == 0 {
				out.SkippedKinds = append(out.SkippedKinds, "equipment_grade_window_empty")
				continue
			}
			if sourceDifficulty {
				item, err := weightedEquipment(&rng, gear)
				if err != nil {
					return out, err
				}
				out.Awards = append(out.Awards, Award{item.ID, 1})
			} else {
				out.Awards = append(out.Awards, Award{gear[rng.Next(uint32(len(gear)))].ID, 1})
			}
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
		if len(ids) == 0 && rarity > 0 && !sourceDifficulty {
			ids = candidates(0)
		}
		if len(ids) == 0 && sourceDifficulty && enabled["stackable"] {
			out.SkippedKinds = append(out.SkippedKinds, fmt.Sprintf("stackable_grade_window_empty: category=%d level=%d rarity=%d", category, level, rarity))
		}
		if len(ids) > 0 {
			var id uint32
			if sourceDifficulty {
				// Creation weights are source values, not uniform candidate counts.
				var total uint64
				for _, item := range ids {
					total += uint64(item.Weight)
				}
				if total == 0 || total > math.MaxUint32 {
					return out, fmt.Errorf("invalid ordinary stackable weights")
				}
				pick := rng.Next(uint32(total))
				for _, item := range ids {
					if pick < item.Weight {
						id = item.ID
						break
					}
					pick -= item.Weight
				}
			} else {
				id = ids[rng.Next(uint32(len(ids)))].ID
			}
			if enabled["stackable"] {
				out.Awards = append(out.Awards, Award{id, 1})
			}
		}
	}
	out.NextSeed = rng.Seed
	return out, nil
}

// equipmentCandidatesNearest retries the grade window on the neighbouring
// rarities when the rolled one has no gear at that level.
//
// 2026-09-23 实机：奥德赛 100004940（basis level 55）击杀记录里出现
// `drop_rules_pending / equipment_grade_window_empty` —— 装备概率已经命中，
// 却一件都没掉。原因是稀有度 roll 与掉落池的等级覆盖错开：装备目录里
// rarity=0 的 744 件全在 grade<=20，grade>=22 的 2430 件全是 rarity>=1；
// 而 [basis of rarity dicision] 首行给 rarity=0 的概率是 700000/1000000。
// 于是 50 级以上的怪有 70% 的装备命中落在一个必然为空的候选集上。
// 这里按"先向上、再向下"的顺序找最近的可用稀有度，把已经命中的那次
// 掉落真正发出来；概率结构本身不变。
func equipmentCandidatesNearest(pool []inventory.EquipmentDrop, rarity int32, level byte, grade []float64) []inventory.EquipmentDrop {
	const maxRarity = 8 // [basis of rarity dicision] 的 9 列 ⇒ 稀有度 0..8
	for r := rarity + 1; r <= maxRarity; r++ {
		if gear := equipmentCandidates(pool, r, level, grade); len(gear) > 0 {
			return gear
		}
	}
	for r := rarity - 1; r >= 0; r-- {
		if gear := equipmentCandidates(pool, r, level, grade); len(gear) > 0 {
			return gear
		}
	}
	return nil
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
