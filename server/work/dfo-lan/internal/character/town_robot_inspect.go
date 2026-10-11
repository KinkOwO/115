package character

import (
	"dfolan/internal/game/protocol"
	"encoding/json"
)

// TownRobotInfo115 projects only isolated robot state. It neither queries nor
// mutates a player account, and never falls back to the requesting player's bag.
func (s *Service) TownRobotInfo115(role Character, wireContext [2]byte) ([]byte, error) {
	snapshot, err := s.TownRobotCompanionSnapshot(role)
	if err != nil {
		return nil, err
	}
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return nil, err
	}
	fame, err := s.EquipmentFame(role.State)
	if err != nil {
		return nil, err
	}
	view := protocol.OtherUserInfo115{
		ActorServerID: role.WireID, Context: wireContext,
		Experience: state.Experience, Stats: snapshot.Stats,
		Worn: snapshot.Worn, ExpandEquipFlags: snapshot.ExpandEquipFlags, Fame: fame,
	}
	for tree := range view.SkillTrees {
		known, err := s.knownSkills(role, state, tree)
		if err != nil {
			return nil, err
		}
		for _, id := range skillOrder(state, known) {
			view.SkillTrees[tree] = append(view.SkillTrees[tree], protocol.EntrySkill{ID: uint16(id), Level: known[uint16(id)]})
		}
	}
	return protocol.OtherUserInfoReply115(view)
}
