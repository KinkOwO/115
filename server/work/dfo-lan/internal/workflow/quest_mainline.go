package workflow

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/savecontract"
	"dfolan/internal/storage"
)

func (s *QuestService) OdysseyMainline(ctx context.Context, role storage.Character) (int, []uint16, error) {
	if s == nil || s.Quest == nil || s.Store == nil {
		return 0, nil, nil
	}
	questRole := character.Character{ID: role.ID, AccountID: role.AccountID, WireID: role.WireID,
		Name: role.Name, Profession: role.Profession, Request: role.Request,
		ConfigVersion: role.ConfigVersion, State: role.State, CreatedAt: role.CreatedAt}
	clear, branches, eligible, e := s.Quest.RoleMainlinePlan(questRole)
	if e != nil {
		return 0, nil, e
	}
	if !eligible {
		return 0, nil, nil
	}
	cleared, e := s.Store.ClearQuests(ctx, role.AccountID, role.ID, savecontract.Identity(), clear)
	if e != nil {
		return 0, nil, e
	}
	return cleared, branches, nil
}
