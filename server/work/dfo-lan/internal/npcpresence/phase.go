package npcpresence

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

type PhaseRule struct {
	Ordinal, Kind int
	Value, Index  int32
	Resolved      bool
}

// ProjectPhaseRules preserves all source rules. Only single completed-quest
// conditions are closed for evaluation below; event predicates have different
// cache inputs and some have side effects. Their source tags are retained.
func ProjectPhaseRules(cells []pvf.Token) ([]PhaseRule, []string) {
	kinds := map[string]int{"[recent clear dungeon]": 0, "[regional movement]": 1,
		"[completed quest]": 2, "[by event]": 3, "[depend townarea event]": 4, "[while event active]": 5}
	var rows []PhaseRule
	var gaps []string
	for i := 0; i < len(cells); i++ {
		if cells[i].Type != 3 || cells[i].Text != "[phase shift]" {
			continue
		}
		r := PhaseRule{Ordinal: len(rows), Kind: -1, Resolved: true}
		hasCondition, hasIndex := false, false
		j := i + 1
		for j < len(cells) && !(cells[j].Type == 3 && cells[j].Text == "[/phase shift]") {
			switch {
			case cells[j].Type == 3 && cells[j].Text == "[condition]" && j+2 < len(cells) &&
				cells[j+1].Type == 6 && cells[j+2].Type == 0:
				kind, ok := kinds[cells[j+1].Text]
				r.Resolved = r.Resolved && ok && !hasCondition
				r.Kind, r.Value, hasCondition = kind, cells[j+2].Value, true
				j += 3
			case cells[j].Type == 3 && cells[j].Text == "[phase Index]" && j+1 < len(cells) && cells[j+1].Type == 0:
				r.Resolved = r.Resolved && !hasIndex && cells[j+1].Value >= -1
				r.Index, hasIndex = cells[j+1].Value, true
				j += 2
			default:
				r.Resolved = false
				j++
			}
		}
		r.Resolved = r.Resolved && hasCondition && hasIndex && j < len(cells)
		if !r.Resolved {
			gaps = append(gaps, fmt.Sprintf("phase rule %d has unsupported source syntax", r.Ordinal))
		}
		rows = append(rows, r)
		i = j
	}
	return rows, gaps
}

// PhaseCache models the closed kind-2 cache path, not all native conditions.
// A successful CMD4's verified560/576 resets establish phase-1. Restoring a
// completed snapshot later does not reset phases advanced in the meantime.
type PhaseCache struct {
	resetKnown bool
	phases     map[uint32]int32
	unknown    map[uint32]bool
}

func (c *PhaseCache) SuccessfulSelectionReset() {
	c.resetKnown = true
	c.phases, c.unknown = make(map[uint32]int32), make(map[uint32]bool)
}

func (c *PhaseCache) Forget() { *c = PhaseCache{} }

// EvaluateCompleted returns nil if source scope or either required cache/set
// input is unknown. Such a resolve poisons this town's cached phase until an
// explicit reset or independently observed phase snapshot is supplied.
func (c *PhaseCache) EvaluateCompleted(town uint32, rules []PhaseRule, completed QuestSet) (*int32, []string) {
	if c.phases == nil {
		c.phases, c.unknown = make(map[uint32]int32), make(map[uint32]bool)
	}
	phase, known := c.phases[town]
	if !known && c.resetKnown {
		phase, known = -1, true
	}
	var gaps []string
	if !known || c.unknown[town] {
		gaps = append(gaps, "initial phase cache is unknown")
	}
	if !completed.Known {
		gaps = append(gaps, "completed quest membership is unknown")
	}
	keys := make(map[int32]bool)
	validInitial := phase == -1
	for _, r := range rules {
		if !r.Resolved || r.Kind != 2 || r.Value < 0 || r.Index < -1 || keys[r.Value] {
			gaps = append(gaps, "town source is outside unique single completed-quest condition scope")
		}
		keys[r.Value] = true
		validInitial = validInitial || phase == r.Index
	}
	if !validInitial {
		gaps = append(gaps, "initial phase does not belong to this source town")
	}
	if len(gaps) != 0 {
		c.unknown[town] = true
		return nil, gaps
	}
	for _, r := range rules {
		if completed.Has(uint32(r.Value)) == True && r.Index > phase {
			phase = r.Index
		}
	}
	c.phases[town] = phase
	return &phase, nil
}

// ObservePhase must describe this town's native cache at this point, not just
// a phase resource path (several slots can have the same path).
func (c *PhaseCache) ObservePhase(town uint32, phase int32) {
	if c.phases == nil {
		c.phases, c.unknown = make(map[uint32]int32), make(map[uint32]bool)
	}
	c.phases[town], c.unknown[town] = phase, false
}
