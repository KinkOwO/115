package character

import (
	"encoding/json"
	"fmt"
	"sort"
)

type CompanionSkillChoice struct {
	ID       uint16 `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Rank     byte   `json:"rank"`
	NextCost *int   `json:"next_cost,omitempty"`
	Active   bool   `json:"active"`
}

// CompanionSkillChoices is a view of the same source/learning rules used by
// the player skill window. It does not grant ranks or invent a parallel table.
func (s *Service) CompanionSkillChoices(role Character) ([]CompanionSkillChoice, error) {
	if s == nil || s.Learning == nil || role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return nil, fmt.Errorf("skill source unavailable")
	}
	var st State
	if err := json.Unmarshal(role.State, &st); err != nil {
		return nil, err
	}
	known, err := s.knownSkills(role, st, 0)
	if err != nil {
		return nil, err
	}
	rows := []CompanionSkillChoice{}
	for _, ref := range s.Learning.Rows {
		if ref.Job != role.Profession {
			continue
		}
		d, ok, err := s.Learning.Definition(ref.Job, ref.ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("missing skill %d", ref.ID)
		}
		if known[d.ID] == 0 && !d.ForAdvancement(int(st.Advancement)) && !d.ForAwakening(int(st.Advancement), int(st.Awakening)) {
			continue
		}
		r := CompanionSkillChoice{ID: d.ID, Path: d.Path, Name: d.Path, Rank: known[d.ID], Active: d.ShortcutCapable()}
		if names := d.Fields["[name]"]; len(names) == 1 && names[0].Text != "" {
			r.Name = names[0].Text
		}
		if cost, e := d.costForState(st, int(r.Rank)+1, known); e == nil && cost <= int(st.SkillPoints[0]) {
			r.NextCost = &cost
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows, nil
}
