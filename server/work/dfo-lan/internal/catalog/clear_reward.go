package catalog

import (
	"fmt"
	"math"

	"dfolan/internal/catalog/pvf"
)

// ClearRewardTable projects the ordinary card rule sections of the current PVF.
// The complete ScriptRecord remains in LootCatalog.Rules, including special
// dungeon rewards. A projection is not permission to award from an unknown
// branch: profile auxiliary values and category indexes have no assumed meaning.
type ClearRewardTable struct {
	Path, SHA256        string
	Profiles            []ClearRewardProfile
	KindProbability     [2]int32
	ItemTypeProbability [4]int32
	DifficultyBonus     [5]int32
	Rarity              [9]int32
	PartyBonus          []ClearRewardTypeBonus
	GoldDifficultyBonus []ClearRewardTypeBonus
	MapCountRates       [][2]int32
	GoldCardCosts       [][2]int32
	GradeReference      [][3]int32
	RarityControl       [5]int32
	GoldCardCreateRate  float64
}

type ClearRewardProfile struct {
	Name string
	Rows []ClearRewardLevelRow
}

type ClearRewardLevelRow struct {
	MinimumLevel, MaximumLevel, Probability, Auxiliary int32
}

type ClearRewardTypeBonus struct {
	DungeonType int32
	Values      []float64
}

// ClearRewardProfile.Row has no nearest-level or alternate-profile fallback.
func (p ClearRewardProfile) Row(level int32) (ClearRewardLevelRow, bool) {
	for _, r := range p.Rows {
		if level >= r.MinimumLevel && level <= r.MaximumLevel {
			return r, true
		}
	}
	return ClearRewardLevelRow{}, false
}

func (t ClearRewardTable) Profile(name string) (ClearRewardProfile, bool) {
	for _, p := range t.Profiles {
		if p.Name == name {
			return p, true
		}
	}
	return ClearRewardProfile{}, false
}

// clearRewardSection refuses repeated/truncated core sections. Sections with
// explicit closing tags may contain nested blocks; flat sections end at the
// next tag. This avoids sectionCells discarding the nested dungeon-type rows.
func clearRewardSection(s ScriptRecord, name string, closed bool) ([]pvf.Token, error) {
	start := -1
	for i, c := range s.Cells {
		if c.Type == 3 && c.Text == name {
			if start >= 0 {
				return nil, fmt.Errorf("%s: repeated %s", s.Path, name)
			}
			start = i + 1
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("%s: missing %s", s.Path, name)
	}
	close := "[/" + name[1:]
	for i := start; i < len(s.Cells); i++ {
		c := s.Cells[i]
		if c.Type == 3 && ((!closed) || c.Text == close) {
			return s.Cells[start:i], nil
		}
	}
	if closed {
		return nil, fmt.Errorf("%s: unterminated %s", s.Path, name)
	}
	return s.Cells[start:], nil
}

func clearRewardIntegers(cells []pvf.Token) ([]int32, error) {
	out := make([]int32, len(cells))
	for i, c := range cells {
		if c.Type != 0 {
			return nil, fmt.Errorf("expected integer at cell %d", i)
		}
		out[i] = c.Value
	}
	return out, nil
}

func clearRewardBonuses(cells []pvf.Token, width int) ([]ClearRewardTypeBonus, error) {
	var out []ClearRewardTypeBonus
	seen := map[int32]bool{}
	for len(cells) > 0 {
		if len(cells) < width+3 || cells[0].Type != 3 || cells[0].Text != "[dungeon type]" ||
			cells[1].Type != 0 || cells[width+2].Type != 3 || cells[width+2].Text != "[/dungeon type]" {
			return nil, fmt.Errorf("invalid nested dungeon-type bonus")
		}
		row := ClearRewardTypeBonus{DungeonType: cells[1].Value}
		if seen[row.DungeonType] {
			return nil, fmt.Errorf("duplicate dungeon-type bonus %d", row.DungeonType)
		}
		seen[row.DungeonType] = true
		for _, c := range cells[2 : width+2] {
			var v float64
			switch c.Type {
			case 0:
				v = float64(c.Value)
			case 2:
				v = float64(c.Number)
			default:
				return nil, fmt.Errorf("nonnumeric dungeon-type bonus")
			}
			if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fmt.Errorf("invalid dungeon-type bonus")
			}
			row.Values = append(row.Values, v)
		}
		out = append(out, row)
		cells = cells[width+3:]
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty dungeon-type bonuses")
	}
	return out, nil
}

func ParseClearRewardTable(s ScriptRecord) (ClearRewardTable, error) {
	out := ClearRewardTable{Path: s.Path, SHA256: s.SHA256}
	ints := func(name string, closed bool) ([]int32, error) {
		cells, err := clearRewardSection(s, name, closed)
		if err != nil {
			return nil, err
		}
		v, err := clearRewardIntegers(cells)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", s.Path, name, err)
		}
		return v, nil
	}
	count, err := ints("[drop prob count]", false)
	if err != nil {
		return out, err
	}
	if len(count) != 1 || count[0] <= 0 || count[0] > 200 {
		return out, fmt.Errorf("%s: invalid drop-probability row count", s.Path)
	}
	cells, err := clearRewardSection(s, "[drop prob]", true)
	if err != nil {
		return out, err
	}
	seen := map[string]bool{}
	width := 1 + int(count[0])*4
	for len(cells) > 0 {
		if len(cells) < width || cells[0].Type != 6 || cells[0].Text == "" || seen[cells[0].Text] {
			return out, fmt.Errorf("%s: invalid four-column card profile", s.Path)
		}
		p := ClearRewardProfile{Name: cells[0].Text}
		seen[p.Name] = true
		v, err := clearRewardIntegers(cells[1:width])
		if err != nil {
			return out, fmt.Errorf("%s profile %q: %w", s.Path, p.Name, err)
		}
		for i := 0; i < len(v); i += 4 {
			r := ClearRewardLevelRow{v[i], v[i+1], v[i+2], v[i+3]}
			if r.MinimumLevel <= 0 || r.MaximumLevel < r.MinimumLevel || r.Probability < 0 ||
				(len(p.Rows) > 0 && r.MinimumLevel <= p.Rows[len(p.Rows)-1].MaximumLevel) {
				return out, fmt.Errorf("%s: invalid card profile level range", s.Path)
			}
			p.Rows = append(p.Rows, r)
		}
		out.Profiles = append(out.Profiles, p)
		cells = cells[width:]
	}
	if len(out.Profiles) == 0 {
		return out, fmt.Errorf("%s: empty card profiles", s.Path)
	}
	for _, part := range []struct {
		name   string
		dst    []int32
		closed bool
	}{
		{"[drop kind prob]", out.KindProbability[:], false},
		{"[drop item type prob]", out.ItemTypeProbability[:], false},
		{"[dungeon difficulty drop bonusrate]", out.DifficultyBonus[:], false},
		{"[basis of rarity dicision]", out.Rarity[:], false},
		{"[item drop rarity control]", out.RarityControl[:], true},
	} {
		v, err := ints(part.name, part.closed)
		if err != nil {
			return out, err
		}
		if len(v) != len(part.dst) {
			return out, fmt.Errorf("%s: invalid width for %s", s.Path, part.name)
		}
		copy(part.dst, v)
	}
	for _, part := range []struct {
		name  string
		width int
		dst   *[]ClearRewardTypeBonus
	}{
		{"[party member drop bonusrate]", 4, &out.PartyBonus},
		{"[dungeon difficulty gold drop bonusrate]", 5, &out.GoldDifficultyBonus},
	} {
		v, err := clearRewardSection(s, part.name, true)
		if err != nil {
			return out, err
		}
		*part.dst, err = clearRewardBonuses(v, part.width)
		if err != nil {
			return out, fmt.Errorf("%s %s: %w", s.Path, part.name, err)
		}
	}
	for _, part := range []struct {
		name string
		dst  *[][2]int32
	}{
		{"[reward item rate per map max count]", &out.MapCountRates},
		{"[gold card cost table]", &out.GoldCardCosts},
	} {
		v, err := ints(part.name, true)
		if err != nil {
			return out, err
		}
		if len(v) == 0 || len(v)%2 != 0 {
			return out, fmt.Errorf("%s: invalid pairs in %s", s.Path, part.name)
		}
		for i := 0; i < len(v); i += 2 {
			if v[i] <= 0 || v[i+1] < 0 || (i > 0 && v[i] <= v[i-2]) {
				return out, fmt.Errorf("%s: invalid ordered key/value in %s", s.Path, part.name)
			}
			*part.dst = append(*part.dst, [2]int32{v[i], v[i+1]})
		}
	}
	v, err := ints("[item drop ref table]", false)
	if err != nil {
		return out, err
	}
	if len(v) == 0 || len(v)%3 != 0 {
		return out, fmt.Errorf("%s: invalid card grade rows", s.Path)
	}
	for i := 0; i < len(v); i += 3 {
		out.GradeReference = append(out.GradeReference, [3]int32{v[i], v[i+1], v[i+2]})
	}
	cells, err = clearRewardSection(s, "[gold card create rate]", false)
	if err != nil {
		return out, err
	}
	if len(cells) != 1 || (cells[0].Type != 2 && cells[0].Type != 0) {
		return out, fmt.Errorf("%s: invalid gold-card create rate", s.Path)
	}
	if cells[0].Type == 2 {
		out.GoldCardCreateRate = float64(cells[0].Number)
	} else {
		out.GoldCardCreateRate = float64(cells[0].Value)
	}
	if out.GoldCardCreateRate < 0 || math.IsNaN(out.GoldCardCreateRate) || math.IsInf(out.GoldCardCreateRate, 0) {
		return out, fmt.Errorf("%s: invalid gold-card create rate", s.Path)
	}
	return out, nil
}
