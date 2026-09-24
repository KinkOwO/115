package quest

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"sort"
)

// GraduationQuestPlan combines source clear tables and earlier-level epics.
// Episode quests and level-115 content remain playable; rewards are not run.
func (s *Service) GraduationQuestPlan(role storage.Character) ([]uint16, error) {
	if s.Odyssey == nil || s.Odyssey.Quests == nil || len(s.Catalog.Quests) == 0 ||
		role.ConfigVersion != s.Odyssey.Source || role.ConfigVersion != s.Catalog.Source.Checksum || role.ConfigVersion != s.Professions.Source.Checksum {
		return nil, fmt.Errorf("graduation quest catalogs are missing or mismatched")
	}
	var state character.State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return nil, err
	}
	if state.Level < character.OdysseyGraduationLevel {
		return nil, fmt.Errorf("graduation requires max level")
	}
	profession, ok := s.Professions.Professions[role.Profession]
	if !ok {
		return nil, fmt.Errorf("graduation profession absent from source")
	}
	clear := map[uint32]bool{}
	for _, id := range s.Odyssey.Quests.ClearedAt(character.OdysseyGraduationLevel) {
		clear[id] = true
	}
	for id, q := range s.Catalog.Quests {
		grade := cells(q.Script.Cells, "[grade]")
		if len(grade) == 1 && grade[0].Type == 6 && grade[0].Text == "[epic]" && q.MinimumLevel > 0 && q.MinimumLevel < uint32(character.OdysseyGraduationLevel) {
			clear[id] = true
		}
	}
	for _, branch := range s.Odyssey.Quests.Branches {
		if branch.Level >= character.OdysseyGraduationLevel {
			delete(clear, branch.Quest)
		}
	}
	ids := make([]uint16, 0, len(clear))
	for id := range clear {
		q, ok := s.Catalog.Quests[id]
		if id == 0 || id >= 65535 || (ok && q.ID != id) {
			return nil, fmt.Errorf("graduation quest %d absent or invalid", id)
		}
		// The source [quest clear] table also includes historical quests no
		// longer present in the playable catalog. Its explicit clear instruction
		// is authoritative for these IDs; only catalog epics require job gates.
		if !ok {
			ids = append(ids, uint16(id))
			continue
		}
		if q.MinimumLevel >= uint32(character.OdysseyGraduationLevel) {
			continue
		}
		allowed := jobAllowed(q.Jobs, profession.Job)
		for _, grow := range cells(q.Script.Cells, "[grow type]") {
			if grow.Type != 0 {
				return nil, fmt.Errorf("graduation quest %d grow type unresolved", id)
			}
			if grow.Value >= 0 && grow.Value != int32(state.Advancement) {
				allowed = false
			}
		}
		if allowed {
			ids = append(ids, uint16(id))
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func (s *Service) applyOdysseyGraduation(role storage.Character, rewardPaid bool) (json.RawMessage, json.RawMessage, []uint16, error) {
	if !character.CreatedAsOdyssey(role) || s.Progression == nil {
		return nil, nil, nil, fmt.Errorf("invalid Odyssey graduation role/service")
	}
	ids, err := s.GraduationQuestPlan(role)
	if err != nil {
		return nil, nil, nil, err
	}
	raw := role.State
	if !character.OdysseyGraduated(role) {
		raw, _, err = s.Progression.ApplyOdysseyGraduation(role)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	var doc map[string]json.RawMessage
	if err = json.Unmarshal(raw, &doc); err != nil {
		return nil, nil, nil, err
	}
	doc["odyssey_graduation_version"] = json.RawMessage(`2`)
	// The mail path is not implemented. Keep a debt without touching the bag.
	// Honour old paid receipts so previously granted boxes cannot repeat.
	owed := uint32(0)
	if !rewardPaid {
		owed = s.Odyssey.GraduateReward
		if owed == 0 {
			return nil, nil, nil, fmt.Errorf("graduation reward template missing")
		}
	}
	doc["odyssey_graduation_reward_owed"], _ = json.Marshal(owed)
	raw, err = json.Marshal(doc)
	if err != nil {
		return nil, nil, nil, err
	}
	proof, err := json.Marshal(map[string]any{"quests": ids, "reward_owed": owed, "reward_already_paid": rewardPaid, "source": role.ConfigVersion})
	return raw, proof, ids, err
}

// GraduateOdyssey runs only at login or an actual return-to-town boundary.
// Eligibility is rechecked by the callback using the locked current row.
func (s *Service) GraduateOdyssey(ctx context.Context, role storage.Character) (storage.Character, bool, error) {
	if !character.CreatedAsOdyssey(role) {
		return role, false, nil
	}
	var state character.State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return role, false, err
	}
	if state.Level < character.OdysseyGraduationLevel || state.OdysseyGraduationVersion >= 2 {
		return role, false, nil
	}
	if s.Store == nil || s.Odyssey == nil || s.Progression == nil {
		return role, false, fmt.Errorf("Odyssey graduation services missing")
	}
	return s.Store.CommitOdysseyGraduation(ctx, role.AccountID, role.ID, s.Odyssey.Source, s.applyOdysseyGraduation)
}
