package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"reflect"
)

func (s *ItemService) DisjointAvatar(ctx context.Context, role database.Character, req protocol.DisjointAvatarRequest) (database.Character, inventory.AvatarDisjointReceipt, bool, error) {
	var receipt inventory.AvatarDisjointReceipt
	fail := func(err error) (database.Character, inventory.AvatarDisjointReceipt, bool, error) {
		return role, inventory.AvatarDisjointReceipt{}, false, err
	}
	if s == nil || s.Store == nil || s.Items == nil {
		return fail(fmt.Errorf("avatar disjoint service unavailable"))
	}
	sequence, err := s.Items.AvatarDisjointSequence(InventoryRole(role))
	if err != nil {
		return fail(err)
	}
	key := fmt.Sprintf("avatar-disjoint:%d:%d:%d", sequence, req.Slot, req.Template)
	saved, applied, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Items.Catalog.Source.SaveIdentity(), key, inventory.AvatarDisjointModel,
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			return s.Items.PrepareAvatarDisjoint(InventoryRole(current), req, sequence)
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
	if receipt.Slot != req.Slot || receipt.Template != req.Template || receipt.Sequence != sequence || receipt.Source != s.Items.Catalog.Source.SaveIdentity() {
		return fail(fmt.Errorf("avatar disjoint receipt conflict"))
	}
	if err := s.Items.PrepareAvatarDisjointReceipt(&receipt, saved.State); err != nil {
		return fail(err)
	}
	saved.WireID = role.WireID
	return saved, receipt, applied, nil
}
func (s *ItemService) AddAvatarSocket(ctx context.Context, role database.Character, req protocol.AddAvatarSocketRequest) (database.Character, inventory.AvatarSocketReceipt, bool, error) {
	fail := func(err error) (database.Character, inventory.AvatarSocketReceipt, bool, error) {
		return role, inventory.AvatarSocketReceipt{}, false, err
	}
	if s == nil || s.Store == nil || s.Items == nil {
		return fail(fmt.Errorf("avatar socket service unavailable"))
	}
	sequence, err := s.Items.AvatarSocketSequence(InventoryRole(role))
	if err != nil {
		return fail(err)
	}
	key := fmt.Sprintf("avatar-socket:%d:%d:%d:%d", sequence, req.AvatarSlot, req.Template, req.DeviceSlot)
	saved, applied, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Items.Catalog.Source.SaveIdentity(), key, inventory.AvatarSocketModel,
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			return s.Items.PrepareAvatarSocket(InventoryRole(current), req, sequence)
		})
	if err != nil {
		return fail(err)
	}
	raw, err := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if err != nil {
		return fail(err)
	}
	var receipt inventory.AvatarSocketReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return fail(err)
	}
	if receipt.Request != req || receipt.Sequence != sequence || receipt.Source != s.Items.Catalog.Source.SaveIdentity() {
		return fail(fmt.Errorf("avatar socket receipt conflict"))
	}
	if err := s.Items.PrepareAvatarSocketReceipt(&receipt, saved.State); err != nil {
		return fail(err)
	}
	saved.WireID = role.WireID
	return saved, receipt, applied, nil
}
func (s *ItemService) UseEmblems(ctx context.Context, role database.Character, req protocol.UseEmblemRequest) (database.Character, inventory.EmblemInlayReceipt, bool, error) {
	fail := func(err error) (database.Character, inventory.EmblemInlayReceipt, bool, error) {
		return role, inventory.EmblemInlayReceipt{}, false, err
	}
	if s == nil || s.Store == nil || s.Items == nil {
		return fail(fmt.Errorf("avatar emblem service unavailable"))
	}
	sequence, err := s.Items.EmblemInlaySequence(InventoryRole(role))
	if err != nil {
		return fail(err)
	}
	key, err := inventory.EmblemInlayEventKey(sequence, req)
	if err != nil {
		return fail(err)
	}
	saved, applied, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Items.Catalog.Source.SaveIdentity(), key, inventory.EmblemInlayModel,
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			return s.Items.PrepareEmblemInlay(InventoryRole(current), req, sequence)
		})
	if err != nil {
		return fail(err)
	}
	raw, err := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if err != nil {
		return fail(err)
	}
	var receipt inventory.EmblemInlayReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return fail(err)
	}
	if receipt.Sequence != sequence || receipt.Source != s.Items.Catalog.Source.SaveIdentity() || !reflect.DeepEqual(receipt.Request, req) {
		return fail(fmt.Errorf("avatar emblem receipt conflict"))
	}
	if err := s.Items.PrepareEmblemInlayReceipt(&receipt, saved.State); err != nil {
		return fail(err)
	}
	saved.WireID = role.WireID
	return saved, receipt, applied, nil
}
func (s *ItemService) CompoundEmblems(ctx context.Context, role database.Character, req protocol.CompoundEmblemRequest) (database.Character, inventory.EmblemCompoundReceipt, bool, error) {
	fail := func(err error) (database.Character, inventory.EmblemCompoundReceipt, bool, error) {
		return role, inventory.EmblemCompoundReceipt{}, false, err
	}
	if s == nil || s.Store == nil || s.Items == nil {
		return fail(fmt.Errorf("emblem compound service unavailable"))
	}
	sequence, err := s.Items.EmblemCompoundSequence(InventoryRole(role))
	if err != nil {
		return fail(err)
	}
	key, err := inventory.EmblemCompoundEventKey(sequence, req)
	if err != nil {
		return fail(err)
	}
	saved, applied, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Items.Catalog.Source.SaveIdentity(), key, inventory.EmblemCompoundModel,
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			return s.Items.PrepareEmblemCompound(InventoryRole(current), req, sequence)
		})
	if err != nil {
		return fail(err)
	}
	raw, err := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if err != nil {
		return fail(err)
	}
	var receipt inventory.EmblemCompoundReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return fail(err)
	}
	if receipt.Sequence != sequence || receipt.Mode != req.Mode || receipt.Source != s.Items.Catalog.Source.SaveIdentity() || !reflect.DeepEqual(receipt.Inputs, req.Inputs) {
		return fail(fmt.Errorf("emblem compound receipt conflict"))
	}
	if err := s.Items.PrepareEmblemCompoundReceipt(&receipt, saved.State); err != nil {
		return fail(err)
	}
	saved.WireID = role.WireID
	return saved, receipt, applied, nil
}
