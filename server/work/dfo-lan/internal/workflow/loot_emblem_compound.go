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

func (s *LootService) CompoundEmblems(ctx context.Context, role storage.Character, req protocol.CompoundEmblemRequest) (storage.Character, loot.EmblemCompoundReceipt, bool, error) {
	fail := func(err error) (storage.Character, loot.EmblemCompoundReceipt, bool, error) {
		return role, loot.EmblemCompoundReceipt{}, false, err
	}
	if s == nil || s.Store == nil || s.Loot == nil {
		return fail(fmt.Errorf("emblem compound service unavailable"))
	}
	sequence, err := s.Loot.EmblemCompoundSequence(LootRole(role))
	if err != nil {
		return fail(err)
	}
	key, err := loot.EmblemCompoundEventKey(sequence, req)
	if err != nil {
		return fail(err)
	}
	saved, applied, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), key, loot.EmblemCompoundModel,
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			return s.Loot.PrepareEmblemCompound(LootRole(current), req, sequence)
		})
	if err != nil {
		return fail(err)
	}
	raw, err := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if err != nil {
		return fail(err)
	}
	var receipt loot.EmblemCompoundReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return fail(err)
	}
	if receipt.Sequence != sequence || receipt.Mode != req.Mode || receipt.Source != s.Loot.Catalog.Source.SaveIdentity() || !reflect.DeepEqual(receipt.Inputs, req.Inputs) {
		return fail(fmt.Errorf("emblem compound receipt conflict"))
	}
	if err := s.Loot.PrepareEmblemCompoundReceipt(&receipt, saved.State); err != nil {
		return fail(err)
	}
	saved.WireID = role.WireID
	return saved, receipt, applied, nil
}
