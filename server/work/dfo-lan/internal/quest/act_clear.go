package quest

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

// ActClearPlan follows source epic quest prerequisites up to the character's
// current level. Other grades and quests behind an unmet non-epic prerequisite
// are never included. Existing completions seed the walk.
func (s *Service) ActClearPlan(role storage.Character, states []storage.QuestState) ([]uint16, error) {
	if role.ConfigVersion != s.Catalog.Source.Checksum {
		return nil, fmt.Errorf("quest clear source mismatch")
	}
	var state character.State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return nil, err
	}
	status := make(map[uint32]string, len(states))
	for _, q := range states {
		status[uint32(q.ID)] = q.Status
	}
	job := s.Professions.Professions[role.Profession].Job
	index := s.Index()
	var clear []uint16
	for changed := true; changed; {
		changed = false
		for _, id := range index.Ordered {
			if id == 0 || id >= 40000 || status[id] != "" && status[id] != "accepted" {
				continue
			}
			d := s.Catalog.Quests[id]
			grade := cells(d.Script.Cells, "[grade]")
			if len(grade) != 1 || grade[0].Type != 6 || grade[0].Text != "[epic]" ||
				len(d.Pending) != 0 || d.MinimumLevel == 0 || d.MinimumLevel > uint32(state.Level) ||
				!jobAllowed(d.Jobs, job) {
				continue
			}
			e := index.Entries[id]
			allowed := e != nil && e.GrowUsable
			if allowed {
				for _, grow := range e.GrowTypes {
					if grow >= 0 && grow != int32(state.Advancement) {
						allowed = false
					}
				}
			}
			if !allowed || status[id] != "accepted" && !prerequisitesMet(e.PrerequisiteGroups, status) {
				continue
			}
			status[id] = "completed"
			clear = append(clear, uint16(id))
			changed = true
		}
	}
	return clear, nil
}

func (s *Service) ClearActQuests(ctx context.Context, role storage.Character) (int, error) {
	states, err := s.Store.Quests(ctx, role.AccountID, role.ID)
	if err != nil {
		return 0, err
	}
	ids, err := s.ActClearPlan(role, states)
	if err != nil {
		return 0, err
	}
	return s.Store.ClearActQuests(ctx, role.AccountID, role.ID, s.Catalog.Source.Checksum, ids)
}
