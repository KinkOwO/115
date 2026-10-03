package workflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"math"
)

func (s *WearService) RecastAvatar(ctx context.Context, role storage.Character, r protocol.RecastAvatarRequest) (storage.Character, inventory.AvatarRecastReceipt, bool, error) {
	var receipt inventory.AvatarRecastReceipt
	fail := func(e error) (storage.Character, inventory.AvatarRecastReceipt, bool, error) {
		return role, inventory.AvatarRecastReceipt{}, false, e
	}
	if s == nil || s.Store == nil || s.Catalog == nil || s.AvatarRecast == nil || s.AvatarRecastLoot == nil || s.Catalog.Source.Checksum != s.AvatarRecast.Source || s.AvatarRecastLoot.Source.Checksum != s.AvatarRecast.Source || s.BagRules.Source != s.AvatarRecast.Source || role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return fail(fmt.Errorf("avatar recast service/source unavailable"))
	}
	before, e := inventory.ReadBag(role.State)
	if e != nil {
		return fail(e)
	}
	if before.AvatarRecastSeq == math.MaxUint64 {
		return fail(fmt.Errorf("avatar recast sequence exhausted"))
	}
	sequence := before.AvatarRecastSeq
	request, e := json.Marshal(r)
	if e != nil {
		return fail(e)
	}
	key := fmt.Sprintf("avatar-recast-emblem:%d:%x", sequence, sha256.Sum256(request))
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Catalog.Source.SaveIdentity(), key, "avatar-recast-emblem-v2", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		prior, e := inventory.ReadBag(current.State)
		if e != nil {
			return nil, nil, e
		}
		b, rewards, matches, e := s.rules().RecastAvatarBag(InventoryRole(current), r, inventory.DrawRecastAvatar)
		if e != nil {
			return nil, nil, e
		}
		if b.AvatarRecastSeq != sequence+1 {
			return nil, nil, fmt.Errorf("stale avatar recast sequence")
		}
		state, e := inventory.SaveBag(current.State, b)
		if e != nil {
			return nil, nil, e
		}
		receipt = inventory.AvatarRecastReceipt{Request: r, Rewards: rewards, Matches: matches, Sequence: sequence, Source: s.Catalog.Source.SaveIdentity(), Updates: inventory.ChangedItemRows(prior, b)}
		// Validate every response payload before the transaction consumes assets.
		if e := receipt.Prepare(state); e != nil {
			return nil, nil, e
		}
		raw, e := json.Marshal(receipt)
		return state, raw, e
	})
	if e != nil {
		return fail(e)
	}
	raw, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return fail(e)
	}
	if e := json.Unmarshal(raw, &receipt); e != nil {
		return fail(e)
	}
	storedRequest, e := json.Marshal(receipt.Request)
	if e != nil || !bytes.Equal(storedRequest, request) || receipt.Source != s.Catalog.Source.SaveIdentity() || receipt.Sequence != sequence {
		return fail(fmt.Errorf("avatar recast receipt conflict"))
	}
	if e := receipt.Prepare(saved.State); e != nil {
		return fail(e)
	}
	saved.WireID = role.WireID
	return saved, receipt, applied, nil
}
