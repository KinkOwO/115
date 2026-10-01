package character

import (
	"context"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"sort"
)

// Skill Reset window checkbox masks (CMD483 Confirm frame, byte 2).
const (
	ResetOrdinarySkills = 1 // reset purchased ranks to the source floor, refunding SP
	ResetEnhance        = 2 // clear the 3 Enhance (intension) slots
	ResetEvolve         = 4 // clear the 5 Evolve (option) slots and restore the fixed 5-point pool
)

// ResetSkills is the Skill Reset window path (CMD483 with a (style, mask)
// Confirm body). style selects the skill tree (0/1), mask carries the
// checkboxes; bits outside the defined mask are dropped and an empty mask is
// refused. Persists through the same character-event transaction as Learn.
func (s *Service) ResetSkills(ctx context.Context, role Character, key string, style, mask byte) (Character, bool, error) {
	tree := style
	if tree > 1 {
		tree = 0
	}
	mask &= ResetOrdinarySkills | ResetEnhance | ResetEvolve
	if mask == 0 {
		return role, false, fmt.Errorf("empty skill reset mask")
	}
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "source-skill-reset-window-v1", func(cur Character) (json.RawMessage, json.RawMessage, error) {
		var st State
		if e := json.Unmarshal(cur.State, &st); e != nil {
			return nil, nil, e
		}
		if e := s.applySkillReset(ctx, cur, &st, int(tree), mask); e != nil {
			return nil, nil, e
		}
		p, e := mergeSkillState(cur.State, st)
		if e != nil {
			return nil, nil, e
		}
		receipt, _ := json.Marshal(map[string]any{"tree": tree, "mask": mask, "sp": st.SkillPoints[tree]})
		return p, receipt, nil
	})
	if e != nil {
		return role, false, e
	}
	saved.WireID = role.WireID
	return saved, applied, nil
}

// applySkillReset mutates a decoded State in place; split out so the window
// arithmetic can be exercised without a database.
func (s *Service) applySkillReset(ctx context.Context, cur Character, st *State, tree int, mask byte) error {
	if tree != 0 && tree != 1 {
		return fmt.Errorf("invalid skill tree")
	}
	if mask&ResetOrdinarySkills != 0 {
		if e := s.resetOrdinarySkills(ctx, cur, st, tree); e != nil {
			return e
		}
	}
	if variationUnlocked(st) && (mask&ResetEnhance != 0 || mask&ResetEvolve != 0) {
		v := st.SkillVariations[tree]
		if mask&ResetEnhance != 0 {
			v.Intensions = nil
		}
		if mask&ResetEvolve != 0 {
			v.Options = nil
		}
		fillVariationSlots(&v)
		st.SkillVariations[tree] = v
		if mask&ResetEvolve != 0 {
			// The Evolve checkbox restores the fixed pool; Enhance never
			// touches the VP balance.
			st.TechniquePoints[tree] = 5
		}
	}
	return nil
}

// resetOrdinarySkills refunds every rank above the source floor (initial +
// automatic advancement + satisfied awakening grants) into SP, leaves
// LearnedSkills exactly at the floor, and rebuilds SkillSlots from skillRows.
func (s *Service) resetOrdinarySkills(ctx context.Context, cur Character, st *State, tree int) error {
	floor, e := s.skillFloor(cur, *st, tree)
	if e != nil {
		return e
	}
	refund := 0
	if s.Learning != nil {
		var ids []int
		for id := range st.LearnedSkills[tree] {
			ids = append(ids, int(id))
		}
		// Ascending ID order keeps the refund deterministic.
		sort.Ints(ids)
		for _, raw := range ids {
			id := uint16(raw)
			rank := st.LearnedSkills[tree][id]
			if floor[id] >= rank {
				continue
			}
			d, ok, sourceErr := s.Learning.Definition(cur.Profession, id)
			if sourceErr != nil {
				return sourceErr
			}
			if !ok {
				continue
			}
			for lv := int(floor[id]) + 1; lv <= int(rank); lv++ {
				cost, e := d.refundCostForState(*st, lv)
				if e != nil {
					return fmt.Errorf("skill %d rank %d: %w", id, lv, e)
				}
				refund += cost
			}
		}
	}
	total := int(st.SkillPoints[tree]) + refund
	if total > 65535 {
		return fmt.Errorf("SP refund overflow")
	}
	// Keep exactly the source floor for the skills this character actually
	// holds. The other source grants (initial cells, advancement automation)
	// are not stored in LearnedSkills at all, so dropping this tree cannot
	// lose them.
	next := map[uint16]byte{}
	for id := range st.LearnedSkills[tree] {
		if floor[id] == 0 {
			continue
		}
		next[id] = floor[id]
	}
	st.SkillPoints[tree] = uint16(total)
	st.LearnedSkills[tree] = next
	rows, e := s.skillRows(cur, *st, tree)
	if e != nil {
		return e
	}
	st.SkillSlots[tree] = map[uint16]uint16{}
	for _, v := range rows {
		st.SkillSlots[tree][v.ID] = v.Slot
	}
	return nil
}

// refundCostForState returns the [purchase cost] band for a refunded rank
// WITHOUT the eligibility/pre-requisite checks that Cost performs. A full-tree
// reset refunds skills in ascending ID order, so a prerequisite may already
// have been refunded before its dependent ranks are priced; re-running "can
// this skill be learned" would abort the whole reset halfway.
func (d LearningDefinition) refundCostForState(state State, target int) (int, error) {
	cost := d.Ints("[purchase cost]")
	if len(cost) == 0 {
		return 0, fmt.Errorf("skill learning fields unavailable")
	}
	idx := target - 1
	if idx >= len(cost) {
		idx = len(cost) - 1
	}
	if idx < 0 {
		idx = 0
	}
	if cost[idx] < 0 {
		return 0, fmt.Errorf("skill learning fields unavailable")
	}
	return cost[idx], nil
}

// ResetResponse is the id29 tail frame for the Skill Reset window: the
// purchase-success header (SP/TP) plus the variation blocks — filled 3+5
// empty slots when the panel is unlocked, none otherwise.
func (s *Service) ResetResponse(role Character, tree byte) ([]byte, error) {
	var state State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return nil, e
	}
	if tree > 1 {
		tree = 0
	}
	p, e := protocol.SkillPurchaseSuccess(tree, state.SkillPoints[tree], state.TechniquePoints[tree], nil)
	if e != nil {
		return nil, e
	}
	v := state.SkillVariations[tree]
	if variationUnlocked(&state) {
		fillVariationSlots(&v)
	}
	return protocol.SkillPurchaseVariations(p, 0, v.Intensions, v.Options)
}
