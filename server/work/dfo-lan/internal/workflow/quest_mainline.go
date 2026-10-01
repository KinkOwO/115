package workflow

import (
	"context"
	"dfolan/internal/storage"
)

func (s *QuestService) OdysseyMainline(ctx context.Context, role storage.Character) (int, []uint16, error) {
	clear, branches, eligible, e := s.Quest.RoleMainlinePlan(role)
	if e != nil {
		return 0, nil, e
	}
	if !eligible {
		return 0, nil, nil
	}
	cleared, e := s.Store.ClearQuests(ctx, role.AccountID, role.ID, s.Quest.Odyssey.Source, clear)
	if e != nil {
		return 0, nil, e
	}
	return cleared, branches, nil
}
