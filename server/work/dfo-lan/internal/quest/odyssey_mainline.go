package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"encoding/json"
	"fmt"
)

// OdysseyMainlinePlan is the pure graduation mainline strategy (IO-free): the
// story quests an Arad Odyssey character counts as cleared at its current
// level, plus the branch quests that must stay reachable so the player is
// never blocked by a story quest they were never offered.
func OdysseyMainlinePlan(g *catalog.OdysseyGrowth, profession byte, level byte) (clear []uint16, branches []uint16, err error) {
	if g == nil || g.Quests == nil {
		return nil, nil, fmt.Errorf("Odyssey quest tables not loaded")
	}
	if level > catalog.OdysseyGraduationLevelV {
		return nil, nil, fmt.Errorf("level %d beyond Odyssey graduation level", level)
	}
	for _, id := range g.Quests.ClearedAt(level) {
		clear = append(clear, uint16(id))
	}
	// Double insurance: a branch quest must never end up inside the cleared
	// set, even if a future source edit reintroduces the overlap.
	skip := map[uint16]bool{}
	for _, id := range clear {
		skip[id] = true
	}
	for _, id := range g.Quests.BranchQuestsUpTo(level, profession) {
		if !skip[uint16(id)] {
			branches = append(branches, uint16(id))
		}
	}
	return clear, branches, nil
}

// RoleMainlinePlan returns the quests to clear and the branch quests that must
// remain reachable. Persistence belongs to workflow.
func (s *Service) RoleMainlinePlan(role character.Character) ([]uint16, []uint16, bool, error) {
	if s.Odyssey == nil || !character.CreatedAsOdyssey(role) {
		return nil, nil, false, nil
	}
	var state character.State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return nil, nil, false, e
	}
	clear, branches, e := OdysseyMainlinePlan(s.Odyssey, role.Profession, state.Level)
	if e != nil {
		return nil, nil, false, e
	}
	return clear, branches, true, nil
}
