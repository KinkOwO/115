package inventory

import "fmt"

// SymbolStarFame is the single projection used by both the UI and role fame.
// FameCell is an increment; StartFame is added once, with no guessed scale factor.
func SymbolStarFame(layout *SymbolSocketLayout, name string, progress SymbolProgress) (uint32, error) {
	if layout == nil || layout.Rules == nil {
		return 0, fmt.Errorf("symbol rules unavailable")
	}
	typ, ok := SymbolTypes[name]
	if !ok {
		return 0, fmt.Errorf("unknown symbol color")
	}
	tier, ok := layout.Rules.Tiers[SymbolTierKey{Type: typ, Grade: progress.Grade}]
	if !ok {
		return 0, fmt.Errorf("unknown symbol tier")
	}
	if _, err := layout.Rules.FixedAt(typ, progress.Grade, progress.Level); err != nil {
		return 0, err
	}
	total := uint64(tier.StartFame)
	for level, values := range tier.Levels {
		if level <= progress.Level {
			total += uint64(values.FameCell)
		}
	}
	if total > uint64(^uint32(0)) {
		return 0, fmt.Errorf("symbol fame overflow")
	}
	return uint32(total), nil
}
