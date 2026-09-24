package quest

import (
	"context"
	"dfolan/internal/storage"
	"fmt"
	"slices"
)

// ActClearPlan selects only epic quests that the character has already
// accepted. CMD1422 does not carry a quest ID, so the persisted accepted set is
// the only source-backed boundary for the current Act quest(s). In particular,
// completing one quest must not recursively add its newly unlocked successor.
func (s *Service) ActClearPlan(role storage.Character, states []storage.QuestState) ([]uint16, error) {
	if role.ConfigVersion != s.Catalog.Source.Checksum {
		return nil, fmt.Errorf("quest clear source mismatch")
	}
	ids := make([]uint16, 0, 1)
	for _, q := range states {
		if q.Status != "accepted" {
			continue
		}
		if q.ConfigVersion != s.Catalog.Source.Checksum {
			return nil, fmt.Errorf("accepted quest %d requires source migration", q.ID)
		}
		d, ok := s.Catalog.Quests[uint32(q.ID)]
		if !ok {
			return nil, fmt.Errorf("accepted quest %d absent from source index", q.ID)
		}
		grade := cells(d.Script.Cells, "[grade]")
		if len(grade) == 1 && grade[0].Type == 6 && grade[0].Text == "[epic]" {
			ids = append(ids, q.ID)
		}
	}
	slices.Sort(ids)
	return ids, nil
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
