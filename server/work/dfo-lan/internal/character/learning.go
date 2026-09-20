package character

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

func knownSkills(s State, tree int) (map[uint16]byte, error) {
	base := initialSkills(s)
	out := map[uint16]byte{}
	for _, v := range base {
		out[v.ID] = v.Level
	}
	for id, level := range s.LearnedSkills[tree] {
		if id == 0 || level == 0 {
			return nil, fmt.Errorf("invalid learned skill state")
		}
		out[id] = level
	}
	return out, nil
}
func skillOrder(state State, known map[uint16]byte) []int {
	ids := make([]int, 0, len(known))
	seen := map[uint16]bool{}
	// Use the same normalized stream as knownSkills.  Persisted characters
	// created before the initial-skill repair may retain conditional or broken
	// triples; iterating the raw cells would reintroduce those IDs with a zero
	// level (or twice), which makes the entry-addition packet invalid.
	for _, skill := range initialSkills(state) {
		id := skill.ID
		if seen[id] {
			continue
		}
		ids = append(ids, int(id))
		seen[id] = true
	}
	var added []int
	for id := range known {
		if !seen[id] {
			added = append(added, int(id))
		}
	}
	sort.Ints(added)
	return append(ids, added...)
}
func (s *Service) skillRows(role storage.Character, state State, tree int) ([]protocol.LearnedSkill, error) {
	known, e := s.knownSkills(role, state, tree)
	if e != nil {
		return nil, e
	}
	prof, ok := s.Catalog.Professions[role.Profession]
	if !ok || prof.RawSHA256 != state.SourceSHA256 {
		return nil, fmt.Errorf("skill profession source mismatch")
	}
	ids := skillOrder(state, known)
	slots := map[uint16]uint16{}
	used := map[uint16]bool{}
	// Persisted shortcut moves are authoritative, even when an older state
	// records only the moved rows. Reserve every explicit row first so a
	// source default cannot claim a slot that a later row already owns.
	for _, raw := range ids {
		id := uint16(raw)
		if v, ok := state.SkillSlots[tree][id]; ok {
			if v >= 255 || used[v] {
				return nil, fmt.Errorf("invalid/duplicate saved skill slot")
			}
			slots[id] = v
			used[v] = true
		}
	}
	for _, raw := range ids {
		id := uint16(raw)
		if _, ok := slots[id]; ok {
			continue
		}
		slot := uint16(65535)
		if byAdv := prof.AdvancementSkillSlots[state.Advancement]; byAdv != nil {
			if v, ok := byAdv[id]; ok {
				slot = v
			}
		}
		if slot == 65535 {
			if v, ok := prof.InitialSkillSlots[id]; ok {
				slot = v
			}
		}
		if slot != 65535 && slot >= 255 {
			return nil, fmt.Errorf("invalid source skill slot")
		}
		if slot != 65535 && used[slot] {
			// A saved custom row owns the slot. Put the source-default row into
			// the ordinary palette below instead of rejecting or overwriting it.
			slot = 65535
		}
		if slot != 65535 {
			used[slot] = true
		}
		slots[id] = slot
	}
	// Current145c7a730 searches the first free slot at14 after the quickbar.
	// The manager allocates512 entries, while CMD28/29 use an8-bit slot index;
	//255 is the request/reply sentinel, so ordinary projected slots stop at254.
	if s.Learning != nil {
		for _, raw := range ids {
			id := uint16(raw)
			_, ok := s.Learning.index[role.Profession][id]
			if !ok {
				return nil, fmt.Errorf("learned skill missing from profession")
			}
			if slots[id] != 65535 {
				continue
			}
			for slot := uint16(14); slot < 255; slot++ {
				if !used[slot] {
					slots[id] = slot
					used[slot] = true
					break
				}
			}
			if slots[id] == 65535 {
				return nil, fmt.Errorf("ordinary skill palette is full")
			}
		}
	}
	out := make([]protocol.LearnedSkill, 0, len(ids))
	for _, raw := range ids {
		id := uint16(raw)
		row := protocol.LearnedSkill{ID: id, Level: known[id], Slot: slots[id]}
		if source := prof.SkillCommands[id]; len(source) > 0 {
			row.Commands = append([]uint32(nil), source...)
		}
		out = append(out, row)
	}
	return out, nil
}
func mergeSkillState(raw json.RawMessage, state State) (json.RawMessage, error) {
	p, e := json.Marshal(state)
	if e != nil {
		return nil, e
	}
	var old, fields map[string]json.RawMessage
	if e = json.Unmarshal(raw, &old); e != nil {
		return nil, e
	}
	if e = json.Unmarshal(p, &fields); e != nil {
		return nil, e
	}
	for k, v := range fields {
		old[k] = v
	}
	return json.Marshal(old)
}
func (s *Service) Learn(ctx context.Context, role storage.Character, key string, req protocol.SkillPurchase) (storage.Character, bool, error) {
	if s.Learning == nil || req.Tree != 0 {
		return role, false, fmt.Errorf("learning service/source unavailable")
	}
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "source-skill-learning-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		var state State
		if e := json.Unmarshal(current.State, &state); e != nil {
			return nil, nil, e
		}
		known, e := s.knownSkills(current, state, int(req.Tree))
		if e != nil {
			return nil, nil, e
		}
		seen := map[uint16]bool{}
		base := initialSkills(state)
		floor := map[uint16]byte{}
		for _, v := range base {
			floor[v.ID] = v.Level
		}
		free, e := s.automaticSkills(current, state)
		if e != nil {
			return nil, nil, e
		}
		for id, rank := range free {
			if floor[id] < rank {
				floor[id] = rank
			}
		}
		for stage := byte(1); stage <= state.Awakening; stage++ {
			grants := s.Catalog.Professions[current.Profession].AwakeningSkills[state.Advancement][stage]
			for i := 0; i+1 < len(grants); i += 2 {
				id, rank := uint16(grants[i]), byte(grants[i+1])
				if known[id] >= rank && floor[id] < rank {
					floor[id] = rank
				}
			}
		}
		points := int(state.SkillPoints[req.Tree])
		changes := map[uint16]byte{}
		var newlyLearned []uint16
		effectiveLevel := int(state.Level)
		if s.Store != nil {
			if hasTactician, _ := s.Store.HasActivePremium(ctx, role.AccountID, storage.PremiumTactician, time.Now()); hasTactician {
				effectiveLevel += 5
			}
		}
		for _, v := range req.Entries {
			d, ok := s.Learning.index[current.Profession][v.ID]
			if !ok || seen[v.ID] || v.Delta == 0 || v.Refund > 1 {
				return nil, nil, fmt.Errorf("invalid job skill learning request")
			}
			seen[v.ID] = true
			target := int(known[v.ID]) + int(v.Delta)
			if v.Refund == 1 {
				target = int(known[v.ID]) - int(v.Delta)
				if target < int(floor[v.ID]) || target < 0 {
					return nil, nil, fmt.Errorf("cannot refund source initial ranks or absent skill")
				}
				for lv := int(known[v.ID]); lv > target; lv-- {
					cost, err := d.costForLevel(state, effectiveLevel, lv, known)
					if err != nil {
						return nil, nil, err
					}
					points += cost
					if points > 65535 {
						return nil, nil, fmt.Errorf("SP refund overflow")
					}
				}
			}
			for lv := int(known[v.ID]) + 1; lv <= target; lv++ {
				cost, e := d.costForLevel(state, effectiveLevel, lv, known)
				if e != nil {
					return nil, nil, fmt.Errorf("skill %d rank %d: %w", v.ID, lv, e)
				}
				points -= cost
				if points < 0 {
					return nil, nil, fmt.Errorf("not enough SP")
				}
			}
			if known[v.ID] == 0 && target > 0 {
				newlyLearned = append(newlyLearned, v.ID)
			}
			known[v.ID] = byte(target)
			changes[v.ID] = byte(target)
		}
		for id, level := range known {
			if level == 0 {
				continue
			}
			pre := s.Learning.index[current.Profession][id].Ints("[pre required skill]")
			if len(pre)%2 != 0 {
				return nil, nil, fmt.Errorf("invalid skill prerequisite")
			}
			for i := 0; i < len(pre); i += 2 {
				if int(known[uint16(pre[i])]) < pre[i+1] {
					return nil, nil, fmt.Errorf("refund would invalidate learned skill prerequisite")
				}
			}
		}
		if len(changes) == 0 && req.Intensions == nil && req.Options == nil {
			return nil, nil, fmt.Errorf("empty learning request")
		}
		if e := s.applyVariations(current.Profession, &state, known, req); e != nil {
			return nil, nil, e
		}
		if state.LearnedSkills[req.Tree] == nil {
			state.LearnedSkills[req.Tree] = map[uint16]byte{}
		}
		for id, lv := range changes {
			if lv == 0 {
				delete(state.LearnedSkills[req.Tree], id)
				delete(state.SkillSlots[req.Tree], id)
			} else {
				state.LearnedSkills[req.Tree][id] = lv
			}
		}
		state.SkillPoints[req.Tree] = uint16(points)
		rows, e := s.skillRows(current, state, int(req.Tree))
		if e != nil {
			return nil, nil, e
		}
		s.placeNewShortcuts(current.Profession, rows, newlyLearned)
		state.SkillSlots[req.Tree] = map[uint16]uint16{}
		for _, v := range rows {
			state.SkillSlots[req.Tree][v.ID] = v.Slot
		}
		p, e := mergeSkillState(current.State, state)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(map[string]any{"skills": changes, "sp": points, "source": s.Learning.Source.Checksum})
		return p, receipt, e
	})
	saved.WireID = role.WireID
	return saved, applied, e
}

// Auto-bind only newly purchased active skills. Rank upgrades never undo a
// player's existing layout; a full bar keeps the skill in its book slot.
func (s *Service) placeNewShortcuts(job byte, rows []protocol.LearnedSkill, fresh []uint16) {
	used := map[uint16]bool{}
	for _, row := range rows {
		used[row.Slot] = true
	}
	for _, id := range fresh {
		if !s.Learning.index[job][id].Active() {
			continue
		}
		for i := range rows {
			if rows[i].ID != id || rows[i].Slot < 14 {
				continue
			}
			for slot := uint16(0); slot < 14; slot++ {
				if !used[slot] {
					delete(used, rows[i].Slot)
					rows[i].Slot = slot
					used[slot] = true
					break
				}
			}
		}
	}
}
func (s *Service) MoveSkill(ctx context.Context, role storage.Character, key string, req protocol.SkillMove) (storage.Character, bool, error) {
	if s.Learning == nil || req.Tree != 0 || req.From == 255 || req.To == 255 || req.From == req.To {
		return role, false, fmt.Errorf("invalid ordinary skill move")
	}
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "source-skill-slots-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		var state State
		if e := json.Unmarshal(current.State, &state); e != nil {
			return nil, nil, e
		}
		rows, e := s.skillRows(current, state, 0)
		if e != nil {
			return nil, nil, e
		}
		var from, to uint16
		slots := map[uint16]uint16{}
		for _, v := range rows {
			slots[v.ID] = v.Slot
			if v.Slot == uint16(req.From) {
				from = v.ID
			}
			if v.Slot == uint16(req.To) {
				to = v.ID
			}
		}
		if from == 0 {
			return nil, nil, fmt.Errorf("source slot is empty")
		}
		for _, id := range []uint16{from, to} {
			if id != 0 && !s.Learning.index[current.Profession][id].Active() {
				return nil, nil, fmt.Errorf("passive skill cannot occupy a shortcut")
			}
		}
		slots[from] = uint16(req.To)
		if to != 0 {
			slots[to] = uint16(req.From)
		}
		state.SkillSlots[0] = slots
		p, e := mergeSkillState(current.State, state)
		if e != nil {
			return nil, nil, e
		}
		r, e := json.Marshal(req)
		return p, r, e
	})
	saved.WireID = role.WireID
	return saved, applied, e
}
func (s *Service) LearningResponse(role storage.Character, req protocol.SkillPurchase) ([]byte, error) {
	var state State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return nil, e
	}
	rows, e := s.skillRows(role, state, int(req.Tree))
	if e != nil {
		return nil, e
	}
	byID := map[uint16]protocol.LearnedSkill{}
	for _, v := range rows {
		byID[v.ID] = v
	}
	var changed []protocol.LearnedSkill
	for _, v := range req.Entries {
		row, ok := byID[v.ID]
		if !ok {
			if v.Refund != 1 {
				return nil, fmt.Errorf("learned response missing skill")
			}
			row = protocol.LearnedSkill{ID: v.ID, Slot: 65535}
		}
		changed = append(changed, row)
	}
	p, e := protocol.SkillPurchaseSuccess(req.Tree, state.SkillPoints[req.Tree], state.TechniquePoints[req.Tree], changed)
	if e != nil {
		return nil, e
	}
	v := state.SkillVariations[req.Tree]
	return protocol.SkillPurchaseVariations(p, req.Mode, v.Intensions, v.Options)
}
