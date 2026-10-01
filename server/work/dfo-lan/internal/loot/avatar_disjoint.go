package loot

import (
	"context"
	"crypto/rand"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
)

type AvatarDisjointReceipt struct {
	Slot      uint16                         `json:"slot"`
	Template  uint32                         `json:"template"`
	Sequence  uint64                         `json:"sequence"`
	Rewards   []protocol.DisjointRewardEntry `json:"rewards"`
	Source    string                         `json:"source"`
	Ack       []byte                         `json:"-"`
	Avatars   []byte                         `json:"-"`
	Inventory []byte                         `json:"-"`
}

func avatarDisjointDraw(limit uint32) (uint32, error) {
	if limit == 0 {
		return 0, fmt.Errorf("empty avatar random range")
	}
	n, e := rand.Int(rand.Reader, new(big.Int).SetUint64(uint64(limit)))
	if e != nil {
		return 0, e
	}
	return uint32(n.Uint64()), nil
}

func (r *AvatarDisjointReceipt) prepare(state json.RawMessage) error {
	var e error
	r.Ack, e = protocol.DisjointAvatarSuccess(r.Slot, r.Rewards)
	if e != nil {
		return e
	}
	r.Avatars, e = inventory.SpecialEquipmentRestorePayload(state, 1)
	if e != nil {
		return e
	}
	b, e := inventory.ReadBag(state)
	if e != nil {
		return e
	}
	r.Inventory, e = protocol.InventoryRestore(b.Rows(), b.Expansion)
	return e
}

// One character-row transaction removes the avatar, places all rewards and
// saves the random result. Retrying that transaction returns its receipt.
func (s *Service) DisjointAvatar(ctx context.Context, role storage.Character, r protocol.DisjointAvatarRequest) (storage.Character, AvatarDisjointReceipt, bool, error) {
	var result AvatarDisjointReceipt
	fail := func(e error) (storage.Character, AvatarDisjointReceipt, bool, error) {
		return role, AvatarDisjointReceipt{}, false, e
	}
	if s == nil || s.Store == nil || s.AvatarDisjoint == nil || s.Equipment == nil {
		return fail(fmt.Errorf("avatar disjoint service unavailable"))
	}
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return fail(fmt.Errorf("avatar disjoint inventory source mismatch"))
	}
	before, e := inventory.ReadBag(role.State)
	if e != nil {
		return fail(e)
	}
	if before.AvatarDisjointSeq == math.MaxUint64 {
		return fail(fmt.Errorf("avatar disjoint sequence exhausted"))
	}
	sequence := before.AvatarDisjointSeq
	key := fmt.Sprintf("avatar-disjoint:%d:%d:%d", sequence, r.Slot, r.Template)
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Catalog.Source.SaveIdentity(), key, "avatar-disjoint-v1",
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			b, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			if b.AvatarDisjointSeq != sequence {
				return nil, nil, fmt.Errorf("stale avatar disjoint sequence")
			}
			b, rewards, e := b.DisjointAvatar(s.Catalog, s.BagRules, s.Equipment, s.AvatarDisjoint, r, avatarDisjointDraw)
			if e != nil {
				return nil, nil, e
			}
			b.AvatarDisjointSeq++
			updated, e := inventory.SaveBag(current.State, b)
			if e != nil {
				return nil, nil, e
			}
			result = AvatarDisjointReceipt{Slot: r.Slot, Template: r.Template, Sequence: sequence, Rewards: rewards, Source: s.Catalog.Source.SaveIdentity()}
			// Reject unserializable state before committing any player inventory.
			if e := result.prepare(updated); e != nil {
				return nil, nil, e
			}
			receipt, e := json.Marshal(result)
			return updated, receipt, e
		})
	if e != nil {
		return fail(e)
	}
	raw, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return fail(e)
	}
	if e := json.Unmarshal(raw, &result); e != nil {
		return fail(e)
	}
	if result.Slot != r.Slot || result.Template != r.Template || result.Sequence != sequence || result.Source != s.Catalog.Source.SaveIdentity() {
		return fail(fmt.Errorf("avatar disjoint receipt conflict"))
	}
	if e := result.prepare(saved.State); e != nil {
		return fail(e)
	}
	saved.WireID = role.WireID
	return saved, result, applied, nil
}
