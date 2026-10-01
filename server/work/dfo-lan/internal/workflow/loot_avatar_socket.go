package workflow

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

func (s *LootService) AddAvatarSocket(ctx context.Context, role storage.Character, req protocol.AddAvatarSocketRequest) (storage.Character, loot.AvatarSocketReceipt, bool, error) {
	fail := func(err error) (storage.Character, loot.AvatarSocketReceipt, bool, error) {
		return role, loot.AvatarSocketReceipt{}, false, err
	}
	if s == nil || s.Store == nil || s.Loot == nil {
		return fail(fmt.Errorf("avatar socket service unavailable"))
	}
	sequence, err := s.Loot.AvatarSocketSequence(LootRole(role))
	if err != nil {
		return fail(err)
	}
	key := fmt.Sprintf("avatar-socket:%d:%d:%d:%d", sequence, req.AvatarSlot, req.Template, req.DeviceSlot)
	saved, applied, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), key, loot.AvatarSocketModel,
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			return s.Loot.PrepareAvatarSocket(LootRole(current), req, sequence)
		})
	if err != nil {
		return fail(err)
	}
	raw, err := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if err != nil {
		return fail(err)
	}
	var receipt loot.AvatarSocketReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return fail(err)
	}
	if receipt.Request != req || receipt.Sequence != sequence || receipt.Source != s.Loot.Catalog.Source.SaveIdentity() {
		return fail(fmt.Errorf("avatar socket receipt conflict"))
	}
	if err := s.Loot.PrepareAvatarSocketReceipt(&receipt, saved.State); err != nil {
		return fail(err)
	}
	saved.WireID = role.WireID
	return saved, receipt, applied, nil
}
