package loot

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"
)

type EmblemCompoundReceipt struct {
	Inputs    []protocol.CompoundEmblemInput  `json:"inputs"`
	Mode      byte                            `json:"mode"`
	Sequence  uint64                          `json:"sequence"`
	Rewards   []protocol.CompoundEmblemReward `json:"rewards"`
	Source    string                          `json:"source"`
	Ack       []byte                          `json:"-"`
	Inventory []byte                          `json:"-"`
}

func emblemCompoundDraw(limit uint32) (uint32, error) {
	if limit == 0 {
		return 0, fmt.Errorf("empty emblem compound random range")
	}
	n, e := rand.Int(rand.Reader, new(big.Int).SetUint64(uint64(limit)))
	if e != nil {
		return 0, e
	}
	return uint32(n.Uint64()), nil
}

func (r *EmblemCompoundReceipt) prepare(state json.RawMessage) error {
	var e error
	r.Ack, e = protocol.CompoundEmblemSuccess(r.Rewards)
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

func (s *Service) CompoundEmblems(ctx context.Context, role storage.Character, r protocol.CompoundEmblemRequest) (storage.Character, EmblemCompoundReceipt, bool, error) {
	var result EmblemCompoundReceipt
	fail := func(e error) (storage.Character, EmblemCompoundReceipt, bool, error) {
		return role, EmblemCompoundReceipt{}, false, e
	}
	if s == nil || s.Store == nil || s.EmblemCompound == nil {
		return fail(fmt.Errorf("emblem compound service unavailable"))
	}
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return fail(fmt.Errorf("emblem compound inventory source mismatch"))
	}
	before, e := inventory.ReadBag(role.State)
	if e != nil {
		return fail(e)
	}
	if before.EmblemCompoundSeq == math.MaxUint64 {
		return fail(fmt.Errorf("emblem compound sequence exhausted"))
	}
	sequence := before.EmblemCompoundSeq
	identity, e := json.Marshal(struct {
		Inputs []protocol.CompoundEmblemInput
		Mode   byte
	}{r.Inputs, r.Mode})
	if e != nil {
		return fail(e)
	}
	key := fmt.Sprintf("emblem-compound:%d:%x", sequence, sha256.Sum256(identity))
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Catalog.Source.SaveIdentity(), key, "emblem-compound-v1",
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			b, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			if b.EmblemCompoundSeq != sequence {
				return nil, nil, fmt.Errorf("stale emblem compound sequence")
			}
			b, rewards, e := b.CompoundEmblems(s.Catalog, s.BagRules, s.EmblemCompound, r, emblemCompoundDraw)
			if e != nil {
				return nil, nil, e
			}
			b.EmblemCompoundSeq++
			updated, e := inventory.SaveBag(current.State, b)
			if e != nil {
				return nil, nil, e
			}
			result = EmblemCompoundReceipt{Inputs: append([]protocol.CompoundEmblemInput(nil), r.Inputs...), Mode: r.Mode, Sequence: sequence, Rewards: rewards, Source: s.Catalog.Source.SaveIdentity()}
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
	if result.Sequence != sequence || result.Mode != r.Mode || result.Source != s.Catalog.Source.SaveIdentity() || !reflect.DeepEqual(result.Inputs, r.Inputs) {
		return fail(fmt.Errorf("emblem compound receipt conflict"))
	}
	if e := result.prepare(saved.State); e != nil {
		return fail(e)
	}
	saved.WireID = role.WireID
	return saved, result, applied, nil
}
