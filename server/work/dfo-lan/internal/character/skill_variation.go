package character

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"time"
)

type SkillVariationState struct {
	Preset     byte                      `json:"preset,omitempty"`
	Intensions []protocol.SkillVariation `json:"intensions,omitempty"`
	Options    []protocol.SkillVariation `json:"options,omitempty"`
}

// variationUnlocked is the single VP-panel gate: the third awakening alone
// unlocks Skill Evolve/Enhance. Level never participates — a character that
// awakened at 100 and has not reached the cap yet must still own the panel.
func variationUnlocked(state *State) bool {
	return state.Awakening == 3
}

// activeEvolutions counts the Evolve rows that actually consume a VP point,
// mirroring the remaining-credit arithmetic in
// protocol.SkillPurchaseVariations.
func activeEvolutions(options []protocol.SkillVariation) int {
	n := 0
	for _, v := range options {
		if v.ID != 0 && v.Choice >= 1 && v.Choice <= 2 {
			n++
		}
	}
	return n
}

// fillVariationSlots pads a VP block to the fixed wire width: three Enhance
// (intension) rows and five Evolve (option) rows. The client renders whatever
// it decodes, so a third-awakened character must always receive the whole
// block; an empty slice leaves the VP panel blank instead of showing the
// unallocated slots.
func fillVariationSlots(v *SkillVariationState) {
	for len(v.Intensions) < 3 {
		v.Intensions = append(v.Intensions, protocol.SkillVariation{ID: 0, Choice: 3, Status: 2})
	}
	for len(v.Options) < 5 {
		v.Options = append(v.Options, protocol.SkillVariation{ID: 0, Choice: 3, Status: 0, Empty: 1})
	}
}

func (s *Service) applyVariations(job byte, state *State, known map[uint16]byte, req protocol.SkillPurchase) error {
	v := state.SkillVariations[req.Tree]
	if req.Intensions != nil || req.Options != nil {
		// The VP panel is unlocked by the third awakening alone. It used to
		// require level 115 as well, which refused every character that
		// awakened at 100 and had not reached the cap yet.
		if !variationUnlocked(state) {
			return fmt.Errorf("variation unlock progression not satisfied")
		}
		v.Preset = req.Preset
	}
	if req.Intensions != nil {
		v.Intensions = append([]protocol.SkillVariation(nil), req.Intensions...)
	}
	if req.Options != nil {
		v.Options = append([]protocol.SkillVariation(nil), req.Options...)
	}
	for group, rows := range [][]protocol.SkillVariation{v.Intensions, v.Options} {
		if rows != nil && (group == 0 && len(rows) != 3 || group == 1 && len(rows) != 5) {
			return fmt.Errorf("invalid variation slot count")
		}
		seen := map[uint16]bool{}
		for _, r := range rows {
			if r.ID == 0 {
				if r.Choice != 3 {
					return fmt.Errorf("invalid empty variation choice")
				}
				continue
			}
			if seen[r.ID] || known[r.ID] == 0 {
				return fmt.Errorf("variation skill %d absent/duplicate", r.ID)
			}
			seen[r.ID] = true
			if r.Choice < 1 || r.Choice > 2 {
				return fmt.Errorf("invalid active variation choice")
			}
			d, ok := s.Learning.index[job][r.ID]
			if !ok || !(d.ForAdvancement(int(state.Advancement)) || d.ForAwakening(int(state.Advancement), int(state.Awakening))) {
				return fmt.Errorf("variation skill belongs to another profession")
			}
			tag := fmt.Sprintf("[intension%d]", r.Choice)
			if group == 1 {
				tag = fmt.Sprintf("[option%d]", r.Choice)
			}
			// A skill with no [variation point] block at all carries no source
			// declaration to compare against. Several third-awakening skills
			// are shaped that way and the client still offers them; refusing
			// them here dropped the whole apply. A skill that does declare the
			// block must still declare this tag.
			vp := d.Fields["[variation point]"]
			if len(vp) == 0 {
				continue
			}
			found := false
			for _, t := range vp {
				if t.Type == 3 && t.Text == tag {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("skill %d lacks source variation %s", r.ID, tag)
			}
		}
	}
	state.SkillVariations[req.Tree] = v
	return nil
}

func (s *Service) VariationRestore(role storage.Character) ([]byte, error) {
	var st State
	if e := json.Unmarshal(role.State, &st); e != nil {
		return nil, e
	}
	// Below the third awakening the client has no VP panel, so it must not
	// receive a variation block at all.
	if !variationUnlocked(&st) {
		return nil, nil
	}
	v := st.SkillVariations[0]
	fillVariationSlots(&v)
	p, e := protocol.SkillPurchaseSuccess(0, st.SkillPoints[0], st.TechniquePoints[0], nil)
	if e != nil {
		return nil, e
	}
	return protocol.SkillPurchaseVariations(p, 0, v.Intensions, v.Options)
}

// ResetAutoSet is the Reset / Auto Set button (CMD483).
//
// The client sends one request whose body does not decode as a plain
// (tree, mask) pair, then re-lays the recommended shortcuts itself. The server
// therefore only clears what it owns: the purchased ranks (refunded into SP),
// the persisted slot layout, and — for a third-awakened character — the Enhance
// and Evolve allocations. Clearing the persisted SkillSlots matters as much as
// clearing the ranks: skillRows prefers them, so stale rows would keep the
// quickbar occupied and push the recommended skills into the 14+ palette.
func (s *Service) ResetAutoSet(ctx context.Context, role storage.Character, key string, tree, mask byte) (storage.Character, error) {
	saved, _, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "source-skill-reset-v1", func(cur storage.Character) (json.RawMessage, json.RawMessage, error) {
		var st State
		if e := json.Unmarshal(cur.State, &st); e != nil {
			return nil, nil, e
		}
		if e := s.resetAutoState(ctx, cur, &st, tree, mask); e != nil {
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
		return role, e
	}
	saved.WireID = role.WireID
	return saved, nil
}

// resetAutoState mutates a decoded State in place. It is split out so the
// refund arithmetic can be exercised without a database.
func (s *Service) resetAutoState(ctx context.Context, cur storage.Character, st *State, tree, mask byte) error {
	if tree > 1 {
		return fmt.Errorf("invalid skill tree")
	}
	if mask&1 != 0 {
		known, e := s.knownSkills(cur, *st, int(tree))
		if e != nil {
			return e
		}
		floor, e := s.skillFloor(cur, *st, int(tree))
		if e != nil {
			return e
		}
		effectiveLevel := int(st.Level)
		if s.Store != nil {
			if has, e := s.Store.HasActivePremium(ctx, cur.AccountID, storage.PremiumTactician, time.Now()); e == nil && has {
				effectiveLevel += 5
			}
		}
		refund := 0
		if s.Learning != nil {
			for id, rank := range st.LearnedSkills[tree] {
				if floor[id] >= rank {
					continue
				}
				d, ok := s.Learning.index[cur.Profession][id]
				if !ok {
					continue
				}
				for lv := int(floor[id]) + 1; lv <= int(rank); lv++ {
					cost, e := d.costForLevel(*st, effectiveLevel, lv, known)
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
		// holds; every rank above it was bought with SP and is refunded. The
		// other source grants (initial cells, advancement automation) are not
		// stored in LearnedSkills at all, so dropping this tree cannot lose
		// them.
		next := map[uint16]byte{}
		for id := range st.LearnedSkills[tree] {
			if floor[id] == 0 {
				continue
			}
			next[id] = floor[id]
		}
		st.SkillPoints[tree] = uint16(total)
		st.LearnedSkills[tree] = next
		st.SkillSlots[tree] = map[uint16]uint16{}
	}
	if variationUnlocked(st) {
		v := st.SkillVariations[tree]
		if mask&2 != 0 || mask&4 != 0 {
			if mask&2 != 0 {
				v.Intensions = nil
			}
			if mask&4 != 0 {
				v.Options = nil
			}
			fillVariationSlots(&v)
			st.SkillVariations[tree] = v
		}
	}
	return nil
}

// skillFloor is the rank a skill holds without SP: the source initial grants,
// the advancement's automatic grants, and the level-satisfied awakening grants.
// It mirrors the floor Learn enforces, so a reset refunds only what the player
// actually paid. Reading the awakening grants alone would refund the rank-1
// (and automatic) portion of every starter skill.
func (s *Service) skillFloor(role storage.Character, st State, tree int) (map[uint16]byte, error) {
	floor := map[uint16]byte{}
	for _, v := range initialSkills(st) {
		floor[v.ID] = v.Level
	}
	free, e := s.automaticSkills(role, st)
	if e != nil {
		return nil, e
	}
	awakened, e := s.awakeningSkills(role, st)
	if e != nil {
		return nil, e
	}
	for _, group := range []map[uint16]byte{free, awakened} {
		for id, rank := range group {
			if floor[id] < rank {
				floor[id] = rank
			}
		}
	}
	return floor, nil
}

// ReconcileTechniquePoints repairs the VP ledger of characters that reached
// the third awakening before the grant/backfill existed. The grant only fires
// at the awakening instant, so an already-awakened save that shows 5 points in
// the panel still carries a zero on disk — one ordinary Learn response then
// clears the panel. This is the login-time repair: balance = 5 − Evolve
// selections, and it is a no-op (never touches any other field) when the
// ledger already matches.
func (s *Service) ReconcileTechniquePoints(ctx context.Context, role storage.Character) (storage.Character, bool, error) {
	var state State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return role, false, e
	}
	if !variationUnlocked(&state) {
		return role, false, nil
	}
	want := uint16(5)
	if n := activeEvolutions(state.SkillVariations[0].Options); n <= 5 {
		want = uint16(5 - n)
	}
	if state.TechniquePoints[0] == want {
		return role, false, nil
	}
	// The event key carries the target balance so a pre-awakening login can
	// never poison the key that a later awakening still needs.
	key := fmt.Sprintf("technique-points-reconcile-v1:%d:want-%d", role.ID, want)
	saved, backfilled, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "technique-points-reconcile-v1", func(cur storage.Character) (json.RawMessage, json.RawMessage, error) {
		var st State
		if e := json.Unmarshal(cur.State, &st); e != nil {
			return nil, nil, e
		}
		if !variationUnlocked(&st) {
			return cur.State, nil, nil
		}
		want := uint16(5)
		if n := activeEvolutions(st.SkillVariations[0].Options); n <= 5 {
			want = uint16(5 - n)
		}
		if st.TechniquePoints[0] == want {
			return cur.State, nil, nil
		}
		st.TechniquePoints[0] = want
		p, e := mergeSkillState(cur.State, st)
		if e != nil {
			return nil, nil, e
		}
		receipt, _ := json.Marshal(map[string]any{"technique_points": want, "source": "reconcile"})
		return p, receipt, nil
	})
	if e != nil {
		return role, false, e
	}
	saved.WireID = role.WireID
	return saved, backfilled, nil
}
