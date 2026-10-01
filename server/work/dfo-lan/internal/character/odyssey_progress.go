package character

import (
	"dfolan/internal/savecontract"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"sort"
)

func (s *ProgressionService) odysseyCompleted(role storage.Character) ([]uint32, error) {
	var state struct {
		Completed []uint32          `json:"odyssey_completed_dungeons"`
		Times     map[string]uint32 `json:"dungeon_best_times"`
	}
	if err := json.Unmarshal(role.State, &state); err != nil {
		return nil, err
	}
	seen := map[uint32]bool{}
	for _, id := range state.Completed {
		if s.Odyssey.ClearLevels[id] == 0 {
			return nil, fmt.Errorf("unknown Odyssey journal dungeon %d", id)
		}
		seen[id] = true
	}
	// Earlier builds recorded clears only when the result screen was settled.
	for id := range s.Odyssey.ClearLevels {
		if state.Times[fmt.Sprintf("%d:normal:solo", id)] > 0 {
			seen[id] = true
		}
	}
	ids := make([]uint32, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func (s *ProgressionService) OdysseyProgressPayload(role storage.Character) ([]byte, error) {
	if s.Odyssey == nil || !OdysseyRole(role) {
		return nil, nil
	}
	if role.ConfigVersion != savecontract.Identity() {
		return nil, fmt.Errorf("Odyssey journal source mismatch")
	}
	ids, err := s.odysseyCompleted(role)
	if err != nil {
		return nil, err
	}
	return protocol.OdysseyCharacterProgress(ids)
}

func (s *ProgressionService) saveOdysseyCompletion(role storage.Character, id uint32) (json.RawMessage, error) {
	if s.Odyssey.ClearLevels[id] == 0 {
		return nil, fmt.Errorf("unknown Odyssey completion")
	}
	ids, err := s.odysseyCompleted(role)
	if err != nil {
		return nil, err
	}
	i := sort.Search(len(ids), func(i int) bool { return ids[i] >= id })
	if i == len(ids) || ids[i] != id {
		ids = append(ids, id)
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(role.State, &fields); err != nil {
		return nil, err
	}
	fields["odyssey_completed_dungeons"], err = json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	return json.Marshal(fields)
}
