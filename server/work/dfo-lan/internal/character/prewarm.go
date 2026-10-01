package character

import (
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

// PrepareRoleDetails warms the actual skill floors and learned dependencies,
// never every profession. It does not change a save or spend skill points.
func (s *Service) PrepareRoleDetails(role storage.Character) error {
	if s.Learning == nil {
		return nil
	}
	if role.ConfigVersion != s.Learning.Source.Checksum {
		return fmt.Errorf("role/skill source mismatch")
	}
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return err
	}
	for tree := range state.LearnedSkills {
		known, err := s.knownSkills(role, state, tree)
		if err != nil {
			return err
		}
		for id := range known {
			if _, ok, err := s.Learning.Definition(role.Profession, id); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("learned skill %d missing", id)
			}
		}
	}
	return nil
}
