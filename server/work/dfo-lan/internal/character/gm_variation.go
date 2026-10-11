package character

import (
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"sort"
)

// GMVariationEdit edits the current client's ordinary VP panel, not legacy TP.
// No point balance is accepted: its five credits are derived from allocations.
type GMVariationEdit struct {
	Preset     byte                      `json:"preset"`
	Intensions []protocol.SkillVariation `json:"intensions"`
	Options    []protocol.SkillVariation `json:"options"`
}

type GMVariationCandidate struct {
	ID      uint16   `json:"id"`
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Enhance []uint32 `json:"enhance"`
	Evolve  []uint32 `json:"evolve"`
}

type GMVariationView struct {
	Unlocked   bool                   `json:"unlocked"`
	Remaining  int                    `json:"remaining"`
	Allocation SkillVariationState    `json:"allocation"`
	Candidates []GMVariationCandidate `json:"candidates"`
}

func (s *Service) GMVariationView(role Character) (GMVariationView, error) {
	var out GMVariationView
	var st State
	if err := json.Unmarshal(role.State, &st); err != nil {
		return out, err
	}
	if s.Learning == nil {
		return out, fmt.Errorf("115 VP source unavailable")
	}
	known, err := s.knownSkills(role, st, 0)
	if err != nil {
		return out, err
	}
	out.Unlocked = variationUnlocked(&st)
	out.Allocation = st.SkillVariations[0]
	fillVariationSlots(&out.Allocation)
	out.Remaining = 5 - activeEvolutions(out.Allocation.Options)
	out.Candidates = []GMVariationCandidate{}
	ids := []int{}
	for id, rank := range known {
		if rank > 0 {
			ids = append(ids, int(id))
		}
	}
	sort.Ints(ids)
	for _, id := range ids {
		d, ok, err := s.Learning.Definition(role.Profession, uint16(id))
		if err != nil {
			return out, err
		}
		if !ok || !(d.ForAdvancement(int(st.Advancement)) || d.ForAwakening(int(st.Advancement), int(st.Awakening))) {
			continue
		}
		row := GMVariationCandidate{ID: uint16(id), Name: d.Path, Path: d.Path, Enhance: []uint32{}, Evolve: []uint32{}}
		for _, t := range d.Fields["[name]"] {
			if t.Text != "" {
				row.Name = t.Text
				break
			}
		}
		for choice := uint32(1); choice <= 2; choice++ {
			for _, t := range d.Fields["[variation point]"] {
				if t.Type != 3 {
					continue
				}
				if t.Text == fmt.Sprintf("[intension%d]", choice) {
					row.Enhance = append(row.Enhance, choice)
				}
				if t.Text == fmt.Sprintf("[option%d]", choice) {
					row.Evolve = append(row.Evolve, choice)
				}
			}
		}
		// The GM catalog lists explicitly declared variants only; it does not
		// manufacture alternatives for source rows with no VP declaration.
		if len(row.Enhance) > 0 || len(row.Evolve) > 0 {
			out.Candidates = append(out.Candidates, row)
		}
	}
	return out, nil
}

func (s *Service) GMEditVariations(role Character, edit GMVariationEdit) (json.RawMessage, error) {
	var st State
	if err := json.Unmarshal(role.State, &st); err != nil {
		return nil, err
	}
	if s.Learning == nil || !variationUnlocked(&st) {
		return nil, fmt.Errorf("需要当前115技能源及第三次觉醒")
	}
	if len(edit.Intensions) != 3 || len(edit.Options) != 5 {
		return nil, fmt.Errorf("VP需要3个强化槽和5个进化槽")
	}
	known, err := s.knownSkills(role, st, 0)
	if err != nil {
		return nil, err
	}
	req := protocol.SkillPurchase{Tree: 0, Preset: edit.Preset, Intensions: append([]protocol.SkillVariation(nil), edit.Intensions...), Options: append([]protocol.SkillVariation(nil), edit.Options...)}
	for group, rows := range [][]protocol.SkillVariation{req.Intensions, req.Options} {
		for i := range rows {
			rows[i].Status = 1
			rows[i].Empty = 0
			if rows[i].ID == 0 {
				rows[i].Choice = 3
				rows[i].Status = 2
				if group == 1 {
					rows[i].Status = 0
					rows[i].Empty = 1
				}
			}
		}
	}
	if err = s.applyVariations(role.Profession, &st, known, req); err != nil {
		return nil, err
	}
	// Targeted raw patch preserves unrelated and future save fields, including
	// the other talent tree and fields private to later versions.
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(role.State, &fields); err != nil {
		return nil, err
	}
	var trees [2]map[string]json.RawMessage
	if raw := fields["skill_variations"]; len(raw) > 0 {
		if err = json.Unmarshal(raw, &trees); err != nil {
			return nil, err
		}
	}
	if trees[0] == nil {
		trees[0] = map[string]json.RawMessage{}
	}
	v := st.SkillVariations[0]
	for k, value := range map[string]any{"preset": v.Preset, "intensions": v.Intensions, "options": v.Options} {
		trees[0][k], err = json.Marshal(value)
		if err != nil {
			return nil, err
		}
	}
	fields["skill_variations"], err = json.Marshal(trees)
	if err != nil {
		return nil, err
	}
	// Existing 115 save/wire contract retains the technique_points field for
	// the VP balance; follow login reconciliation rather than renaming it.
	st.TechniquePoints[0] = uint16(5 - activeEvolutions(v.Options))
	fields["technique_points"], err = json.Marshal(st.TechniquePoints)
	if err != nil {
		return nil, err
	}
	return json.Marshal(fields)
}
