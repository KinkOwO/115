package workflow

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

func (s *LootService) DisjointAvatar(ctx context.Context, role storage.Character, req protocol.DisjointAvatarRequest) (storage.Character, loot.AvatarDisjointReceipt, bool, error) {
	var receipt loot.AvatarDisjointReceipt
	fail := func(err error) (storage.Character, loot.AvatarDisjointReceipt, bool, error) {
		return role, loot.AvatarDisjointReceipt{}, false, err
	}
	if s == nil || s.Store == nil || s.Loot == nil {
		return fail(fmt.Errorf("avatar disjoint service unavailable"))
	}
	sequence, err := s.Loot.AvatarDisjointSequence(LootRole(role))
	if err != nil {
		return fail(err)
	}
	key := fmt.Sprintf("avatar-disjoint:%d:%d:%d", sequence, req.Slot, req.Template)
	saved, applied, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), key, loot.AvatarDisjointModel,
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			return s.Loot.PrepareAvatarDisjoint(LootRole(current), req, sequence)
		})
	if err != nil {
		return fail(err)
	}
	raw, err := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if err != nil {
		return fail(err)
	}
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return fail(err)
	}
	if receipt.Slot != req.Slot || receipt.Template != req.Template || receipt.Sequence != sequence || receipt.Source != s.Loot.Catalog.Source.SaveIdentity() {
		return fail(fmt.Errorf("avatar disjoint receipt conflict"))
	}
	if err := s.Loot.PrepareAvatarDisjointReceipt(&receipt, saved.State); err != nil {
		return fail(err)
	}
	saved.WireID = role.WireID
	return saved, receipt, applied, nil
}
