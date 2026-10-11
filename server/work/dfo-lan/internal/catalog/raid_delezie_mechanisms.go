package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// These are source references, not inferred effect implementations. A symbol
// reference does not establish when to increment it or authorize a client action.
type DelezieSymbolReference struct {
	Row, Parent  int
	Kind, Symbol string
	ID           uint32
}

type DelezieMechanismBindings struct {
	Symbols map[int]DelezieSymbolReference
	Timers  map[int]DelezieTimerReference
}

func parseDelezieMechanismBindings(tree *pvf.CTPTree, symbols map[string]uint32, timers map[string]DelezieTimerRule) (*DelezieMechanismBindings, error) {
	if tree == nil || tree.Table == nil {
		return nil, fmt.Errorf("Delezie mechanism source missing")
	}
	out := &DelezieMechanismBindings{Symbols: map[int]DelezieSymbolReference{}, Timers: map[int]DelezieTimerReference{}}
	bindSymbol := func(row pvf.CTPRecord) error {
		if len(row.Cells) != 1 || len(tree.Children(row.Index, "")) != 0 {
			return fmt.Errorf("%s row %d %s: expected one mechanism symbol", tree.Table.Path, row.Index, row.Name)
		}
		value, err := delezieScalarSymbol(row.Cells[0], symbols)
		if err != nil {
			return fmt.Errorf("%s row %d %s: %w", tree.Table.Path, row.Index, row.Name, err)
		}
		out.Symbols[row.Index] = DelezieSymbolReference{Row: row.Index, Parent: row.Parent, Kind: row.Name, Symbol: value.Symbol, ID: value.SymbolID}
		return nil
	}
	root, err := delezieOptionalRoot(tree, "[DELEZIE ETC SYMBOL REFERENCE]")
	if err != nil {
		return nil, err
	}
	if root >= 0 {
		seen := map[string]bool{}
		for _, row := range tree.Children(root, "") {
			if seen[row.Name] {
				return nil, fmt.Errorf("%s row %d: duplicate mechanism symbol field %s", tree.Table.Path, row.Index, row.Name)
			}
			seen[row.Name] = true
			if err := bindSymbol(row); err != nil {
				return nil, err
			}
		}
	}
	// Field names below are native reference contracts from 1476F2C00;
	// their values and IDs come exclusively from this mode's tree/declarations.
	for _, section := range []struct {
		root   string
		fields []string
	}{
		{"[DELEZIE SALGEIFER EVENT]", []string{"[CREATE COUNT SYMBOL]"}},
		{"[DELEZIE SYNTHESIZE]", []string{"[SYNTHESIZE COUNT SYMBOL]", "[FORCED SYNTHESIZE COUNT SYMBOL]"}},
		{"[DELEZIE PLAGUE LOCATION]", []string{"[NORMAL SUMMON PLAGUE SYMBOL]"}},
		{"[DELEZIE CONSUMING]", []string{"[IS CONSUMING SYMBOL]", "[IS CONSUMING LOCK SYMBOL]", "[IS LAST DEAL CONSUMING SYMBOL]", "[DELEZIE SHORTCUT HP]", "[DEAL CONSUMING TIME POINT SYMBOL]"}},
		{"[DELEZIE INFECTION]", []string{"[SUCCESS INFECTION COUNT SYMBOL]"}},
	} {
		root, err := delezieOptionalRoot(tree, section.root)
		if err != nil {
			return nil, err
		}
		if root < 0 {
			continue
		}
		for _, field := range section.fields {
			rows := tree.Children(root, field)
			if len(rows) > 1 {
				return nil, fmt.Errorf("%s row %d: duplicate mechanism field %s", tree.Table.Path, root, field)
			}
			if len(rows) == 1 {
				if err := bindSymbol(rows[0]); err != nil {
					return nil, err
				}
			}
		}
	}
	root, err = delezieOptionalRoot(tree, "[DELEZIE NOBLE SACRIFICE]")
	if err != nil {
		return nil, err
	}
	if root >= 0 {
		for _, rule := range tree.Children(root, "[NOBLE SACRIFICE]") {
			row, err := tree.One(rule.Index, "[SYMBOL]")
			if err != nil {
				return nil, err
			}
			if err := bindSymbol(row); err != nil {
				return nil, err
			}
		}
	}
	root, err = delezieOptionalRoot(tree, "[DELEZIE ETC TIMER REFERENCE]")
	if err != nil {
		return nil, err
	}
	if root >= 0 {
		seen := map[string]bool{}
		for _, row := range tree.Children(root, "") {
			if seen[row.Name] || len(tree.Children(row.Index, "")) != 0 {
				return nil, fmt.Errorf("%s row %d: duplicate or nested mechanism timer field %s", tree.Table.Path, row.Index, row.Name)
			}
			seen[row.Name] = true
			ref, err := delezieTimerKey(row.Cells, timers)
			if err != nil {
				return nil, fmt.Errorf("%s row %d %s: %w", tree.Table.Path, row.Index, row.Name, err)
			}
			ref.Row, ref.Kind = row.Index, row.Name
			out.Timers[row.Index] = ref
		}
	}
	return out, nil
}

// MechanismProgram rebinds the exact owned mode tree. Shared header references
// are available separately; mode-local objects/recipes are never inherited.
func (r *DelezieRaidRules) MechanismProgram(mode *DelezieModeRules) (*DelezieMechanismBindings, error) {
	if r == nil || mode == nil || mode.Difficulty.Name() == "" || r.Modes[mode.Difficulty] != mode || r.Symbols == nil || r.Timers == nil {
		return nil, fmt.Errorf("Delezie mechanism mode/declarations do not belong to source rules")
	}
	symbols, err := parseDelezieSymbols(r.Symbols)
	if err != nil {
		return nil, err
	}
	timers, err := parseDelezieTimers(r.Timers)
	if err != nil {
		return nil, err
	}
	return parseDelezieMechanismBindings(mode.Phase, symbols, timers)
}

// The normal CTP is also the shared header. In particular it declares HARD
// 1/2 PHASE END timer references even though those fields are absent from the
// hard phase table. Retain that header layer without borrowing normal objects.
func (r *DelezieRaidRules) HeaderMechanismProgram() (*DelezieMechanismBindings, error) {
	if r == nil || r.Header == nil || r.Symbols == nil || r.Timers == nil {
		return nil, fmt.Errorf("Delezie shared header/declarations missing")
	}
	symbols, err := parseDelezieSymbols(r.Symbols)
	if err != nil {
		return nil, err
	}
	timers, err := parseDelezieTimers(r.Timers)
	if err != nil {
		return nil, err
	}
	return parseDelezieMechanismBindings(r.Header, symbols, timers)
}

type DelezieTimerProgram struct {
	PhasePath, ObjectPath string
	Phase, Objects        map[int]DelezieTimerReference
}

func (r *DelezieRaidRules) TimerProgram(mode *DelezieModeRules) (*DelezieTimerProgram, error) {
	bindings, err := r.MechanismProgram(mode)
	if err != nil {
		return nil, err
	}
	timers, err := parseDelezieTimers(r.Timers)
	if err != nil {
		return nil, err
	}
	phase, err := parseDelezieTimerReferences(mode.Phase, timers)
	if err != nil {
		return nil, err
	}
	objects, err := parseDelezieTimerReferences(mode.Objects, timers)
	if err != nil {
		return nil, err
	}
	for row, ref := range bindings.Timers {
		phase[row] = ref
	}
	return &DelezieTimerProgram{PhasePath: mode.Phase.Table.Path, ObjectPath: mode.Objects.Table.Path, Phase: phase, Objects: objects}, nil
}
