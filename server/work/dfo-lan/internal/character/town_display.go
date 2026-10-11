package character

import (
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

// TownDisplayCharacter is a transient, non-playable actor. It never calls a
// Store or a reward notifier. Initial equipment and growth reuse the ordinary
// source-backed character services; no account, item or quest is persisted.
func (s *Service) TownDisplayCharacter(actor uint16, name string, job byte, level byte, growth *ProgressionService) (Character, error) {
	if s == nil || actor == 0 || actor == 65535 || level == 0 {
		return Character{}, fmt.Errorf("invalid transient town character")
	}
	prof, ok := s.Catalog.Professions[job]
	if !ok || prof.RawSHA256 == "" || len(prof.InitialAttributes) == 0 {
		return Character{}, fmt.Errorf("town character profession has no source attributes")
	}
	state := State{Level: 1, Attributes: prof.InitialAttributes, InitialSkills: prof.InitialSkills, SourcePath: prof.Path, SourceSHA256: prof.RawSHA256}
	raw, err := json.Marshal(state)
	if err != nil {
		return Character{}, err
	}
	role := Character{ID: -int64(actor), WireID: actor, Name: name, Profession: job, ConfigVersion: s.Catalog.Source.SaveIdentity(), State: raw}
	if level > 1 {
		if growth == nil {
			return Character{}, fmt.Errorf("town character growth source is missing")
		}
		role, err = growth.AdminLevel(role, int(level))
		if err != nil {
			return Character{}, err
		}
	}
	if worn := s.creationWorn(prof, 0, level); len(worn) > 0 {
		role.State, err = inventory.SaveBag(role.State, inventory.Bag{Version: "ordinary-bag-v1", Worn: worn})
	}
	return role, err
}
