package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// Asrahan combat identities are in the phase's symbol-group records, not
// DGN.Boss. Source slots remain separate: HP, maximum HP, three party damage
// accumulators, and gauge values must never be substituted for one another.
type AsrahanCombatGroup struct {
	Index, Dungeon, Monster uint32
	NativeKind              int32
	HP, MaximumHP           string
	PartyDamage             [3]string
	Symbols                 map[int]string
	Fields                  []pvf.Token
}

// Compile only binds records explicitly owned by this source phase. Optional
// symbols use the source's -1 sentinel. The other operands are retained; this
// function does not infer HP scales, spawn positions or client timer fields.
func CompileAsrahanCombatGroups(r *AsrahanRaidRules, p AsrahanRaidPhase) ([]AsrahanCombatGroup, error) {
	if r == nil || len(r.Groups) == 0 || len(p.Dungeons) == 0 {
		return nil, fmt.Errorf("Asrahan combat source missing")
	}
	seenIndex, seenDungeon := map[uint32]bool{}, map[uint32]bool{}
	groups := make([]AsrahanCombatGroup, 0, len(r.Groups))
	for _, raw := range r.Groups {
		if len(raw.Header) != 3 || len(raw.Fields) != 17 {
			return nil, fmt.Errorf("unbound Asrahan combat group shape")
		}
		for _, cell := range raw.Header {
			if cell.Type != 0 {
				return nil, fmt.Errorf("invalid Asrahan combat header")
			}
		}
		if raw.Header[0].Value <= 0 || raw.Header[1].Value <= 0 || raw.Header[2].Value < -1 {
			return nil, fmt.Errorf("invalid Asrahan combat identity")
		}
		g := AsrahanCombatGroup{Index: uint32(raw.Header[0].Value), Dungeon: uint32(raw.Header[1].Value), NativeKind: raw.Header[2].Value, Symbols: map[int]string{}, Fields: append([]pvf.Token(nil), raw.Fields...)}
		if _, exists := p.Dungeons[g.Dungeon]; !exists || seenIndex[g.Index] || seenDungeon[g.Dungeon] {
			return nil, fmt.Errorf("foreign/duplicate Asrahan combat group")
		}
		seenIndex[g.Index], seenDungeon[g.Dungeon] = true, true
		for _, slot := range []int{0, 1, 2, 3, 4, 5, 6, 10, 12, 13, 16} {
			cell := raw.Fields[slot]
			if cell.Type == 0 && cell.Value == -1 {
				continue
			}
			if cell.Type != 3 || r.Symbols[cell.Text] == 0 {
				return nil, fmt.Errorf("undeclared Asrahan group symbol slot %d", slot)
			}
			g.Symbols[slot] = cell.Text
		}
		for _, slot := range []int{7, 8, 9, 11, 14, 15} {
			if raw.Fields[slot].Type != 0 || raw.Fields[slot].Value < -1 {
				return nil, fmt.Errorf("invalid Asrahan group operand slot %d", slot)
			}
		}
		if raw.Fields[7].Value <= 0 || g.Symbols[0] == "" || g.Symbols[13] == "" {
			return nil, fmt.Errorf("Asrahan combat group has no monster/HP source")
		}
		g.Monster, g.HP, g.MaximumHP = uint32(raw.Fields[7].Value), g.Symbols[0], g.Symbols[13]
		for i := range g.PartyDamage {
			g.PartyDamage[i] = g.Symbols[i+2]
			if g.PartyDamage[i] == "" {
				return nil, fmt.Errorf("Asrahan combat group lacks party damage symbol")
			}
		}
		groups = append(groups, g)
	}
	return groups, nil
}
