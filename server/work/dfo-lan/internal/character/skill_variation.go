package character

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

type SkillVariationState struct {
	Preset     byte                      `json:"preset,omitempty"`
	Intensions []protocol.SkillVariation `json:"intensions,omitempty"`
	Options    []protocol.SkillVariation `json:"options,omitempty"`
}

func (s *Service) applyVariations(job byte, state *State, known map[uint16]byte, req protocol.SkillPurchase) error {
	v := state.SkillVariations[req.Tree]
	if req.Intensions != nil || req.Options != nil {
		// This pilot only enables the fully unlocked, level115 character path.
		if state.Level < 115 || state.Awakening != 3 {
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
			found := false
			for _, t := range d.Fields["[variation point]"] {
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
	v := st.SkillVariations[0]
	if v.Intensions == nil && v.Options == nil {
		return nil, nil
	}
	p, e := protocol.SkillPurchaseSuccess(0, st.SkillPoints[0], st.TechniquePoints[0], nil)
	if e != nil {
		return nil, e
	}
	return protocol.SkillPurchaseVariations(p, 0, v.Intensions, v.Options)
}
