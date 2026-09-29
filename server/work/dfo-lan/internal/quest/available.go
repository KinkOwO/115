package quest

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

func (s *Service) Available(ctx context.Context, role storage.Character) ([]uint32, error) {
	var state character.State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return nil, e
	}
	if role.ConfigVersion != s.Catalog.Source.Checksum {
		return nil, fmt.Errorf("quest availability source mismatch")
	}
	states, e := s.Store.Quests(ctx, role.AccountID, role.ID)
	if e != nil {
		return nil, e
	}
	status := map[uint32]string{}
	for _, q := range states {
		status[uint32(q.ID)] = q.Status
	}
	job := s.Professions.Professions[role.Profession].Job
	x := s.Index()
	var ids []uint32
	for _, id := range x.Ordered {
		if id == 0 || id >= 40000 || status[id] == "completed" {
			continue
		}
		if status[id] == "accepted" {
			ids = append(ids, id)
			continue
		}
		en := x.Entries[id]
		// A quest whose objective or reward this build cannot settle must not
		// be offered: accepting one strands the character on a quest that can
		// be handed in forever without ever completing.
		if !en.Implemented || !en.RewardUsable || !en.GrowUsable || !en.TargetUsable {
			continue
		}
		if uint32(state.Level) < en.MinimumLevel || uint32(state.Level) > en.MaximumLevel {
			continue
		}
		allowed := jobAllowed(en.Jobs, job) && targetCharacterAllowed(en.TargetCharacters, en.NonTargetCharacters, job, state.Advancement, state.Awakening)
		if !prerequisitesMet(en.PrerequisiteGroups, status) {
			allowed = false
		}
		for _, g := range en.GrowTypes {
			if g >= 0 && g != int32(state.Advancement) {
				allowed = false
			}
		}
		if allowed && !collisionsBlocked(en.Collisions, status) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// collisionsBlocked reports whether a mutually exclusive branch peer is
// already taken: [collision quest] quests (Silent City faction, job change,
// ...) must disappear from the available list once the character accepted or
// completed one of them, so the remaining branches cannot be re-chosen.
func collisionsBlocked(collisions []uint32, status map[uint32]string) bool {
	for _, c := range collisions {
		if s, ok := status[c]; ok && (s == "accepted" || s == "completed") {
			return true
		}
	}
	return false
}
