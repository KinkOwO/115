package workflow

import (
	"context"
	"dfolan/internal/storage"
)

func (s *QuestService) ClearActQuests(ctx context.Context, role storage.Character) (int, error) {
	states, err := s.Store.Quests(ctx, role.AccountID, role.ID)
	if err != nil {
		return 0, err
	}
	ids, err := s.Quest.ActClearPlan(role, states)
	if err != nil {
		return 0, err
	}
	return s.Store.ClearActQuests(ctx, role.AccountID, role.ID, s.Quest.Catalog.Source.SaveIdentity(), ids)
}
