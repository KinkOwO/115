package workflow

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"reflect"
)

func (s *LootService) UseEmblems(ctx context.Context, role storage.Character, req protocol.UseEmblemRequest) (storage.Character, loot.EmblemInlayReceipt, bool, error) {
	fail := func(err error) (storage.Character, loot.EmblemInlayReceipt, bool, error) {
		return role, loot.EmblemInlayReceipt{}, false, err
	}
	if s == nil || s.Store == nil || s.Loot == nil {
		return fail(fmt.Errorf("avatar emblem service unavailable"))
	}
	sequence, err := s.Loot.EmblemInlaySequence(LootRole(role))
	if err != nil {
		return fail(err)
	}
	key, err := loot.EmblemInlayEventKey(sequence, req)
	if err != nil {
		return fail(err)
	}
	saved, applied, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), key, loot.EmblemInlayModel,
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			return s.Loot.PrepareEmblemInlay(LootRole(current), req, sequence)
		})
	if err != nil {
		return fail(err)
	}
	raw, err := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if err != nil {
		return fail(err)
	}
	var receipt loot.EmblemInlayReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return fail(err)
	}
	if receipt.Sequence != sequence || receipt.Source != s.Loot.Catalog.Source.SaveIdentity() || !reflect.DeepEqual(receipt.Request, req) {
		return fail(fmt.Errorf("avatar emblem receipt conflict"))
	}
	if err := s.Loot.PrepareEmblemInlayReceipt(&receipt, saved.State); err != nil {
		return fail(err)
	}
	saved.WireID = role.WireID
	return saved, receipt, applied, nil
}
