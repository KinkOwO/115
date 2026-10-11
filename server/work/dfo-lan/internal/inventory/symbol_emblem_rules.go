package inventory

import (
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

const SymbolEmblemSourceDirectory = "live/else/chn/common/emblemremodel/symbolemblemsystem/"

// These are source projections, not a native client controller. Fame cells and
// probabilities retain their original units until the consuming code is bound.
// No inventory mutation or combat-stat installation is authorized by parsing.
type SymbolEmblemRules struct {
	SourceChecksum   string
	Tables           map[string]string
	Tiers            map[SymbolTierKey]SymbolTier
	Fixed, Resonance map[uint32]SymbolOptions
	ResonanceIndices map[SymbolResonanceKey]uint32
	Steps            map[SymbolStepKey]SymbolResonanceStep
	CostItem         uint32
	Tuning           SymbolTuningRules
}
type SymbolTierKey struct{ Type, Grade uint32 }
type SymbolTier struct {
	StartFame uint32
	Levels    map[uint32]SymbolLevel
}
type SymbolLevel struct{ OptionIndex, FameCell uint32 }
type SymbolOptions struct{ Dealer, Buffer map[string]float64 }
type SymbolResonanceKey struct{ SymbolGrade, EmblemGrade uint32 }
type SymbolStepKey struct{ Grade, Level uint32 }
type SymbolResonanceStep struct{ Cost, SuccessRate, ButtonWeight uint32 }

// Rates retain native raw units. Parsing these fields does not enable an RNG,
// charge materials, or grant compensation; those require a bound executor.
type SymbolTuningRules struct {
	DuplicateButtonBonusRate                 uint32
	ComboRates                               map[uint32]uint32
	MultiColorCost, MultiColorAppearanceRate uint32
	FailRewardItem, FailRewardCount          uint32
	FailMailTitleKey, FailMailTextKey        string
}

func ImportSymbolEmblemRules(a *pvf.Archive) (*SymbolEmblemRules, error) {
	if a == nil {
		return nil, fmt.Errorf("symbol emblem source missing")
	}
	tables := map[string]*pvf.CTPTable{}
	for _, name := range []string{"symbolemblemsystem.ctp", "symboloptioninfoscript.ctp", "symbolemblemresonancesystem.ctp"} {
		path := SymbolEmblemSourceDirectory + name
		raw, e := a.ReadRaw(path)
		if e != nil {
			return nil, e
		}
		table, e := pvf.ReadCTP(path, raw)
		if e != nil {
			return nil, e
		}
		tables[name] = table
	}
	return ParseSymbolEmblemRules(a.Snapshot().Checksum, tables)
}

func ParseSymbolEmblemRules(source string, tables map[string]*pvf.CTPTable) (*SymbolEmblemRules, error) {
	b, e := hex.DecodeString(source)
	if e != nil || len(b) != 32 {
		return nil, fmt.Errorf("invalid symbol emblem source identity")
	}
	r := &SymbolEmblemRules{SourceChecksum: source, Tables: map[string]string{}, Tiers: map[SymbolTierKey]SymbolTier{}, Fixed: map[uint32]SymbolOptions{}, Resonance: map[uint32]SymbolOptions{}, ResonanceIndices: map[SymbolResonanceKey]uint32{}, Steps: map[SymbolStepKey]SymbolResonanceStep{}}
	trees := map[string]*pvf.CTPTree{}
	for _, name := range []string{"symbolemblemsystem.ctp", "symboloptioninfoscript.ctp", "symbolemblemresonancesystem.ctp"} {
		table := tables[name]
		if table == nil || table.Path != SymbolEmblemSourceDirectory+name {
			return nil, fmt.Errorf("symbol emblem table identity mismatch: %s", name)
		}
		hash, e := hex.DecodeString(table.SHA256)
		if e != nil || len(hash) != 32 {
			return nil, fmt.Errorf("invalid symbol emblem table digest")
		}
		tree, e := pvf.NewCTPTree(table)
		if e != nil {
			return nil, e
		}
		trees[name] = tree
		r.Tables[name] = table.SHA256
	}
	options := trees["symboloptioninfoscript.ctp"]
	for _, entry := range []struct {
		tag    string
		target map[uint32]SymbolOptions
	}{{"[symbol fixed option list]", r.Fixed}, {"[symbol resonance option list]", r.Resonance}} {
		root, e := options.One(-1, entry.tag)
		if e != nil {
			return nil, e
		}
		for _, row := range options.Children(root.Index, "[option]") {
			index, e := symbolInteger(options, row.Index, "[index]")
			if e != nil {
				return nil, e
			}
			if _, exists := entry.target[index]; exists {
				return nil, fmt.Errorf("duplicate symbol option %d", index)
			}
			dealer, e := symbolStats(options, row.Index, "[dealer]")
			if e != nil {
				return nil, e
			}
			buffer, e := symbolStats(options, row.Index, "[buffer]")
			if e != nil {
				return nil, e
			}
			entry.target[index] = SymbolOptions{Dealer: dealer, Buffer: buffer}
		}
		if len(entry.target) == 0 {
			return nil, fmt.Errorf("empty symbol option list")
		}
	}
	system := trees["symbolemblemsystem.ctp"]
	for _, row := range system.Children(-1, "[symbol option]") {
		typ, e := symbolInteger(system, row.Index, "[type]")
		if e != nil {
			return nil, e
		}
		grade, e := symbolInteger(system, row.Index, "[grade]")
		if e != nil {
			return nil, e
		}
		base, e := symbolInteger(system, row.Index, "[start fame]")
		if e != nil {
			return nil, e
		}
		key := SymbolTierKey{typ, grade}
		if _, ok := r.Tiers[key]; ok {
			return nil, fmt.Errorf("duplicate symbol tier")
		}
		stat, e := system.One(row.Index, "[level stat]")
		if e != nil {
			return nil, e
		}
		values, e := symbolIntegers(stat)
		if e != nil || len(values) == 0 || len(values)%3 != 0 {
			return nil, fmt.Errorf("symbol level stat must contain triples")
		}
		tier := SymbolTier{StartFame: base, Levels: map[uint32]SymbolLevel{}}
		for i := 0; i < len(values); i += 3 {
			level, index, fame := values[i], values[i+1], values[i+2]
			if level != uint32(i/3+1) {
				return nil, fmt.Errorf("noncontiguous symbol level")
			}
			if _, ok := r.Fixed[index]; !ok {
				return nil, fmt.Errorf("symbol fixed option %d absent", index)
			}
			tier.Levels[level] = SymbolLevel{OptionIndex: index, FameCell: fame}
		}
		r.Tiers[key] = tier
	}
	for _, row := range system.Children(-1, "[resonance option]") {
		grade, e := symbolInteger(system, row.Index, "[symbol grade]")
		if e != nil {
			return nil, e
		}
		for _, child := range system.Children(row.Index, "[emblem grade]") {
			eg, e := symbolInteger(system, child.Index, "[grade]")
			if e != nil {
				return nil, e
			}
			index, e := symbolInteger(system, child.Index, "[resonance option index]")
			if e != nil {
				return nil, e
			}
			if _, ok := r.Resonance[index]; !ok {
				return nil, fmt.Errorf("symbol resonance option absent")
			}
			key := SymbolResonanceKey{grade, eg}
			if _, ok := r.ResonanceIndices[key]; ok {
				return nil, fmt.Errorf("duplicate symbol resonance key")
			}
			r.ResonanceIndices[key] = index
		}
	}
	resonance := trees["symbolemblemresonancesystem.ctp"]
	columns := map[SymbolStepKey]uint8{}
	for i, column := range []struct{ root, field string }{{"[resonance cost]", "[cost]"}, {"[resonance success rate]", "[rate]"}, {"[resonance button weight]", "[weight]"}} {
		root, e := resonance.One(-1, column.root)
		if e != nil {
			return nil, e
		}
		for _, row := range resonance.Children(root.Index, "[option]") {
			grade, e := symbolInteger(resonance, row.Index, "[grade]")
			if e != nil {
				return nil, e
			}
			level, e := symbolInteger(resonance, row.Index, "[level]")
			if e != nil || level == 0 {
				return nil, fmt.Errorf("invalid resonance level")
			}
			value, e := symbolInteger(resonance, row.Index, column.field)
			if e != nil {
				return nil, e
			}
			key := SymbolStepKey{grade, level}
			bit := uint8(1 << i)
			if columns[key]&bit != 0 {
				return nil, fmt.Errorf("duplicate resonance step column")
			}
			columns[key] |= bit
			step := r.Steps[key]
			switch i {
			case 0:
				step.Cost = value
			case 1:
				step.SuccessRate = value
			case 2:
				step.ButtonWeight = value
			}
			r.Steps[key] = step
		}
	}
	for key, mask := range columns {
		if mask != 7 {
			return nil, fmt.Errorf("incomplete resonance step: %+v", key)
		}
	}
	r.CostItem, e = symbolInteger(resonance, -1, "[cost item index]")
	if e != nil || r.CostItem == 0 {
		return nil, fmt.Errorf("resonance cost item missing")
	}

	if e = r.readTuning(resonance); e != nil {
		return nil, e
	}
	if len(r.Tiers) == 0 || len(r.ResonanceIndices) == 0 || len(r.Steps) == 0 {
		return nil, fmt.Errorf("empty symbol emblem source")
	}
	return r, nil
}

func (r *SymbolEmblemRules) readTuning(t *pvf.CTPTree) error {
	var e error
	out := SymbolTuningRules{ComboRates: map[uint32]uint32{}}
	out.DuplicateButtonBonusRate, e = symbolInteger(t, -1, "[duplicate button bonus rate]")
	if e != nil {
		return e
	}
	out.MultiColorCost, e = symbolInteger(t, -1, "[multi color cost]")
	if e != nil || out.MultiColorCost == 0 {
		return fmt.Errorf("invalid multi color cost: %v", e)
	}
	out.MultiColorAppearanceRate, e = symbolInteger(t, -1, "[multi color appearance rate]")
	if e != nil {
		return e
	}
	root, e := t.One(-1, "[resonance combo rate]")
	if e != nil {
		return e
	}
	for _, row := range t.Children(root.Index, "[option]") {
		step, e := symbolInteger(t, row.Index, "[step]")
		if e != nil || step == 0 {
			return fmt.Errorf("invalid resonance combo step: %v", e)
		}
		rate, e := symbolInteger(t, row.Index, "[rate]")
		if e != nil {
			return e
		}
		if _, exists := out.ComboRates[step]; exists {
			return fmt.Errorf("duplicate resonance combo step %d", step)
		}
		out.ComboRates[step] = rate
	}
	if len(out.ComboRates) == 0 {
		return fmt.Errorf("empty resonance combo rates")
	}
	// The source stores contiguous steps. Do not accept a missing interior row.
	for step := uint32(1); step <= uint32(len(out.ComboRates)); step++ {
		if _, exists := out.ComboRates[step]; !exists {
			return fmt.Errorf("noncontiguous resonance combo steps")
		}
	}
	reward, e := t.One(-1, "[fail reward]")
	if e != nil {
		return e
	}
	values, e := symbolIntegers(reward)
	if e != nil || len(values) != 2 || values[0] < 2 || values[1] == 0 || len(t.Children(reward.Index, "")) != 0 {
		return fmt.Errorf("invalid resonance fail reward")
	}
	out.FailRewardItem, out.FailRewardCount = values[0], values[1]
	mail, e := t.One(-1, "[fail reward mail]")
	if e != nil {
		return e
	}
	if len(mail.Cells) != 2 || len(t.Children(mail.Index, "")) != 0 {
		return fmt.Errorf("invalid resonance fail mail")
	}
	for _, cell := range mail.Cells {
		if (cell.Kind != "name" && cell.Kind != "name_indexed") || cell.Name == "" || !utf8.ValidString(cell.Name) || strings.ContainsRune(cell.Name, 0) {
			return fmt.Errorf("invalid resonance fail mail key")
		}
	}
	out.FailMailTitleKey, out.FailMailTextKey = mail.Cells[0].Name, mail.Cells[1].Name
	r.Tuning = out
	return nil
}

func symbolStats(t *pvf.CTPTree, parent int, name string) (map[string]float64, error) {
	root, e := t.One(parent, name)
	if e != nil {
		return nil, e
	}
	out := map[string]float64{}
	for _, row := range t.Children(root.Index, "") {
		if len(row.Cells) != 1 || row.Cells[0].Kind != "float" || math.IsNaN(row.Cells[0].Float) || math.IsInf(row.Cells[0].Float, 0) || row.Cells[0].Float < 0 || len(t.Children(row.Index, "")) != 0 {
			return nil, fmt.Errorf("invalid symbol stat %s", row.Name)
		}
		if _, ok := out[row.Name]; ok {
			return nil, fmt.Errorf("duplicate symbol stat %s", row.Name)
		}
		out[row.Name] = row.Cells[0].Float
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty symbol stat block")
	}
	return out, nil
}
func symbolIntegers(row pvf.CTPRecord) ([]uint32, error) {
	out := make([]uint32, 0, len(row.Cells))
	for _, c := range row.Cells {
		if c.Kind != "float" || math.IsNaN(c.Float) || c.Float < 0 || c.Float > math.MaxUint32 || math.Trunc(c.Float) != c.Float {
			return nil, fmt.Errorf("%s requires unsigned integer cells", row.Name)
		}
		out = append(out, uint32(c.Float))
	}
	return out, nil
}
func symbolInteger(t *pvf.CTPTree, parent int, name string) (uint32, error) {
	row, e := t.One(parent, name)
	if e != nil {
		return 0, e
	}
	v, e := symbolIntegers(row)
	if e != nil || len(v) != 1 || len(t.Children(row.Index, "")) != 0 {
		return 0, fmt.Errorf("%s requires one integer", name)
	}
	return v[0], nil
}
func (r *SymbolEmblemRules) FixedAt(typ, grade, level uint32) (SymbolOptions, error) {
	if r == nil {
		return SymbolOptions{}, fmt.Errorf("symbol rules unavailable")
	}
	tier, ok := r.Tiers[SymbolTierKey{typ, grade}]
	if !ok {
		return SymbolOptions{}, fmt.Errorf("unknown symbol tier")
	}
	value, ok := tier.Levels[level]
	if !ok {
		return SymbolOptions{}, fmt.Errorf("unknown symbol level")
	}
	return cloneSymbolOptions(r.Fixed[value.OptionIndex]), nil
}
func (r *SymbolEmblemRules) ResonanceAt(symbolGrade, emblemGrade uint32) (SymbolOptions, error) {
	if r == nil {
		return SymbolOptions{}, fmt.Errorf("symbol rules unavailable")
	}
	index, ok := r.ResonanceIndices[SymbolResonanceKey{symbolGrade, emblemGrade}]
	if !ok {
		return SymbolOptions{}, fmt.Errorf("unsupported emblem grade")
	}
	return cloneSymbolOptions(r.Resonance[index]), nil
}
func cloneSymbolOptions(v SymbolOptions) SymbolOptions {
	clone := func(src map[string]float64) map[string]float64 {
		out := make(map[string]float64, len(src))
		for k, v := range src {
			out[k] = v
		}
		return out
	}
	return SymbolOptions{Dealer: clone(v.Dealer), Buffer: clone(v.Buffer)}
}
