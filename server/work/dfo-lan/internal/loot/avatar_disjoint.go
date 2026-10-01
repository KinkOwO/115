package loot

import (
	"crypto/rand"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
)

const AvatarDisjointModel = "avatar-disjoint-v1"

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

// AvatarDisjointSequence validates the caller's save identity and provides the
// sequence used to fence the following character-event transaction.
func (s *Service) AvatarDisjointSequence(role Role) (uint64, error) {
	if s == nil || s.AvatarDisjoint == nil || s.Equipment == nil {
		return 0, fmt.Errorf("avatar disjoint service unavailable")
	}
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return 0, fmt.Errorf("avatar disjoint inventory source mismatch")
	}
	b, err := inventory.ReadBag(role.State)
	if err != nil {
		return 0, err
	}
	if b.AvatarDisjointSeq == math.MaxUint64 {
		return 0, fmt.Errorf("avatar disjoint sequence exhausted")
	}
	return b.AvatarDisjointSeq, nil
}

// PrepareAvatarDisjoint applies only the loot-domain state transition. The
// workflow owns persistence and replays the saved receipt on retries.
func (s *Service) PrepareAvatarDisjoint(current Role, req protocol.DisjointAvatarRequest, sequence uint64) (json.RawMessage, json.RawMessage, error) {
	b, err := inventory.ReadBag(current.State)
	if err != nil {
		return nil, nil, err
	}
	if b.AvatarDisjointSeq != sequence {
		return nil, nil, fmt.Errorf("stale avatar disjoint sequence")
	}
	b, rewards, err := b.DisjointAvatar(s.Catalog, s.BagRules, s.Equipment, s.AvatarDisjoint, req, avatarDisjointDraw)
	if err != nil {
		return nil, nil, err
	}
	b.AvatarDisjointSeq++
	updated, err := inventory.SaveBag(current.State, b)
	if err != nil {
		return nil, nil, err
	}
	receipt := AvatarDisjointReceipt{Slot: req.Slot, Template: req.Template, Sequence: sequence, Rewards: rewards, Source: s.Catalog.Source.SaveIdentity()}
	if err := receipt.prepare(updated); err != nil {
		return nil, nil, err
	}
	data, err := json.Marshal(receipt)
	return updated, data, err
}

func (s *Service) PrepareAvatarDisjointReceipt(receipt *AvatarDisjointReceipt, state json.RawMessage) error {
	return receipt.prepare(state)
}
