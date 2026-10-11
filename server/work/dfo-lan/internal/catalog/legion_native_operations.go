package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
)

// Different contents reuse numeric operation types with different semantics.
// Keep their tables separate, including indices shared by multiple modes.
type NativeLegionOperations struct {
	Path, SHA256   string
	Modes          map[string][]uint16
	Definitions    map[uint16]NativeLegionOperation
	WeeklyBase     string
	WeeklyDungeons [][]uint32
	ClearRewards   []NativeLegionClearReward
}

type NativeLegionOperation struct {
	Type, SelectPool, Probability uint32
	FixedValue                    *float64
	Rewards                       []NativeLegionReward
	MemberLimit                   string
	Ranges                        []NativeLegionRange
	AllowCoins                    []int32
	Fields                        map[string][]pvf.Token
}

// Native operation reward rows include a tier and account-binding flag.
// Retain those operands; they are not extra quantities or drop probabilities.
type NativeLegionReward struct {
	Difficulty                              string
	Template, Amount, Weight, Tier, Account uint32
}
type NativeLegionClearReward struct {
	DungeonIndex, Template, Amount, Weight, Account uint32
}

type NativeLegionRange struct {
	Party, Difficulty      string
	Minimum, Maximum, Step float64
}

func nativeOperationNumber(t pvf.Token) (float64, error) {
	var value float64
	switch t.Type {
	case 0:
		value = float64(t.Value)
	case 2:
		value = float64(t.Number)
	default:
		return 0, fmt.Errorf("non-numeric operation operand")
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("non-finite operation operand")
	}
	return value, nil
}

func ParseNativeLegionOperations(s ScriptRecord) (*NativeLegionOperations, error) {
	out := &NativeLegionOperations{Path: s.Path, SHA256: s.SHA256, Modes: map[string][]uint16{}, Definitions: map[uint16]NativeLegionOperation{}}
	for _, table := range scriptWarpBlocks(s.Cells, "[operation table]") {
		for _, block := range scriptWarpBlocks(table, "[operation]") {
			kind := sectionCells(block, "[type]")
			if len(kind) != 1 || kind[0].Type != 6 || kind[0].Text == "" {
				return nil, fmt.Errorf("%s: invalid operation mode", s.Path)
			}
			if _, dup := out.Modes[kind[0].Text]; dup {
				return nil, fmt.Errorf("duplicate operation mode")
			}
			seen := map[uint16]bool{}
			for _, v := range sectionCells(block, "[index]") {
				if v.Type != 0 || v.Value <= 0 || v.Value > math.MaxInt16 || seen[uint16(v.Value)] {
					return nil, fmt.Errorf("invalid operation mode index")
				}
				index := uint16(v.Value)
				seen[index] = true
				out.Modes[kind[0].Text] = append(out.Modes[kind[0].Text], index)
			}
		}
	}
	for _, block := range scriptWarpBlocks(s.Cells, "[operation data set]") {
		identity := sectionCells(block, "[index]")
		if len(identity) != 1 || identity[0].Type != 0 || identity[0].Value <= 0 || identity[0].Value > math.MaxInt16 {
			return nil, fmt.Errorf("invalid native operation identity")
		}
		index := uint16(identity[0].Value)
		if _, dup := out.Definitions[index]; dup {
			return nil, fmt.Errorf("duplicate native operation%d", index)
		}
		def := NativeLegionOperation{Fields: map[string][]pvf.Token{}}
		for i, t := range block {
			if t.Type != 3 || len(t.Text) < 2 || t.Text[1] == '/' {
				continue
			}
			if _, dup := def.Fields[t.Text]; dup {
				return nil, fmt.Errorf("operation%d duplicate field%s", index, t.Text)
			}
			end := i + 1
			for end < len(block) && block[end].Type != 3 {
				end++
			}
			def.Fields[t.Text] = append([]pvf.Token(nil), block[i+1:end]...)
		}
		for tag, target := range map[string]*uint32{"[type]": &def.Type, "[select pool]": &def.SelectPool, "[probability]": &def.Probability} {
			v := def.Fields[tag]
			if len(v) != 1 || v[0].Type != 0 || v[0].Value < 0 {
				return nil, fmt.Errorf("operation%d invalid%s", index, tag)
			}
			*target = uint32(v[0].Value)
		}
		if fixed := def.Fields["[type fixed value]"]; len(fixed) > 0 {
			if len(fixed) != 1 {
				return nil, fmt.Errorf("operation%d invalid fixed value", index)
			}
			n, e := nativeOperationNumber(fixed[0])
			if e != nil {
				return nil, e
			}
			def.FixedValue = &n
		}
		rewards := def.Fields["[reward data]"]
		if len(rewards)%6 != 0 {
			return nil, fmt.Errorf("operation%d incomplete reward row", index)
		}
		for i := 0; i < len(rewards); i += 6 {
			if rewards[i].Type != 6 || rewards[i].Text == "" {
				return nil, fmt.Errorf("operation%d invalid reward mode", index)
			}
			v := [5]uint32{}
			for j := range v {
				t := rewards[i+j+1]
				if t.Type != 0 || t.Value < 0 || (j < 3 && t.Value == 0) {
					return nil, fmt.Errorf("operation%d invalid reward operand", index)
				}
				v[j] = uint32(t.Value)
			}
			def.Rewards = append(def.Rewards, NativeLegionReward{rewards[i].Text, v[0], v[1], v[2], v[3], v[4]})
		}
		member := def.Fields["[member limit]"]
		if len(member) != 1 || member[0].Type != 6 || member[0].Text == "" {
			return nil, fmt.Errorf("invalid operation member dimension")
		}
		def.MemberLimit = member[0].Text
		ranges := def.Fields["[random range value]"]
		if len(ranges)%5 != 0 {
			return nil, fmt.Errorf("incomplete native range")
		}
		for i := 0; i < len(ranges); i += 5 {
			if ranges[i].Type != 6 || ranges[i+1].Type != 6 {
				return nil, fmt.Errorf("invalid native range dimensions")
			}
			v := [3]float64{}
			for j := range v {
				n, e := nativeOperationNumber(ranges[i+2+j])
				if e != nil {
					return nil, e
				}
				v[j] = n
			}
			// Dusky's nonconstant step0 grammar is retained, never coerced
			// into Ispins' discrete range sampler.
			if v[0] > v[1] || v[2] < 0 {
				return nil, fmt.Errorf("invalid native range bounds")
			}
			def.Ranges = append(def.Ranges, NativeLegionRange{ranges[i].Text, ranges[i+1].Text, v[0], v[1], v[2]})
		}
		for _, v := range def.Fields["[allow coin]"] {
			if v.Type != 0 || v.Value < -1 {
				return nil, fmt.Errorf("invalid native coin list")
			}
			def.AllowCoins = append(def.AllowCoins, v.Value)
		}
		out.Definitions[index] = def
	}
	if len(out.Modes) == 0 || len(out.Definitions) == 0 {
		return nil, fmt.Errorf("empty native operation source")
	}
	for _, indices := range out.Modes {
		for _, index := range indices {
			if _, ok := out.Definitions[index]; !ok {
				return nil, fmt.Errorf("mode references missing operation%d", index)
			}
		}
	}
	clears := sectionCells(s.Cells, "[dungeon operation clear reward]")
	if len(clears)%5 != 0 {
		return nil, fmt.Errorf("incomplete operation clear reward")
	}
	for i := 0; i < len(clears); i += 5 {
		v := [5]uint32{}
		for j := range v {
			t := clears[i+j]
			if t.Type != 0 || t.Value < 0 || (j > 0 && j < 4 && t.Value == 0) {
				return nil, fmt.Errorf("invalid operation clear reward")
			}
			v[j] = uint32(t.Value)
		}
		out.ClearRewards = append(out.ClearRewards, NativeLegionClearReward{v[0], v[1], v[2], v[3], v[4]})
	}
	if base := sectionCells(s.Cells, "[week base time]"); len(base) > 0 {
		if len(base) != 1 || base[0].Type != 6 {
			return nil, fmt.Errorf("invalid weekly base")
		}
		out.WeeklyBase = base[0].Text
		weeks, count := sectionCells(s.Cells, "[week count]"), sectionCells(s.Cells, "[weekly dungeon count]")
		if len(weeks) != 1 || len(count) != 1 || weeks[0].Type != 0 || count[0].Type != 0 || weeks[0].Value <= 0 || count[0].Value <= 0 {
			return nil, fmt.Errorf("invalid weekly rotation dimensions")
		}
		rows := sectionCells(s.Cells, "[weekly dungeon info]")
		if int64(len(rows)) != int64(weeks[0].Value)*int64(count[0].Value) {
			return nil, fmt.Errorf("incomplete weekly rotation")
		}
		for i := 0; i < len(rows); i += int(count[0].Value) {
			var row []uint32
			for _, v := range rows[i : i+int(count[0].Value)] {
				if v.Type != 0 || v.Value < 0 {
					return nil, fmt.Errorf("invalid weekly dungeon index")
				}
				row = append(row, uint32(v.Value))
			}
			out.WeeklyDungeons = append(out.WeeklyDungeons, row)
		}
	}
	return out, nil
}

// Paths are native loader contracts (14209cfe0 / 141c21880). Content values
// remain in the source and are not copied into an executor policy table.
func importNativeLegionOperations(a *pvf.Archive, contents map[string]LegionContent) error {
	for name, path := range map[string]string{"Dimension Cloister": "contents/2022/dimensioncloister/etc/myresdimensioncloister.etc", "DuskyIsland": "contents/2023/duskyisland/etc/duskyisland.etc"} {
		content, ok := contents[name]
		if !ok {
			continue
		}
		if _, exists := a.FindFile(path); !exists {
			continue
		}
		s, err := ResolveScript(a, path)
		if err != nil {
			return err
		}
		for i, t := range s.Cells {
			if t.Type == 10 {
				s.Cells[i].Text = a.ResolveString(int(t.Value))
			}
		}
		content.NativeOperations, err = ParseNativeLegionOperations(s)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for _, week := range content.NativeOperations.WeeklyDungeons {
			for _, index := range week {
				if int(index) >= len(content.Dungeons) {
					return fmt.Errorf("weekly dungeon absent from COS")
				}
			}
		}
		contents[name] = content
	}
	return nil
}
