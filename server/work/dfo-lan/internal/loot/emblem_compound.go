package loot

import (
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
)

const EmblemCompoundModel = "emblem-compound-v1"

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

func (s *Service) EmblemCompoundSequence(role Role) (uint64, error) {
	if s == nil || s.EmblemCompound == nil {
		return 0, fmt.Errorf("emblem compound service unavailable")
	}
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return 0, fmt.Errorf("emblem compound inventory source mismatch")
	}
	b, err := inventory.ReadBag(role.State)
	if err != nil {
		return 0, err
	}
	if b.EmblemCompoundSeq == math.MaxUint64 {
		return 0, fmt.Errorf("emblem compound sequence exhausted")
	}
	return b.EmblemCompoundSeq, nil
}

func EmblemCompoundEventKey(sequence uint64, req protocol.CompoundEmblemRequest) (string, error) {
	identity, err := json.Marshal(struct {
		Inputs []protocol.CompoundEmblemInput
		Mode   byte
	}{req.Inputs, req.Mode})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("emblem-compound:%d:%x", sequence, sha256.Sum256(identity)), nil
}

// PrepareEmblemCompound applies the loot-domain state transition. The workflow
// owns persistence and receipt replay.
func (s *Service) PrepareEmblemCompound(current Role, req protocol.CompoundEmblemRequest, sequence uint64) (json.RawMessage, json.RawMessage, error) {
	b, err := inventory.ReadBag(current.State)
	if err != nil {
		return nil, nil, err
	}
	if b.EmblemCompoundSeq != sequence {
		return nil, nil, fmt.Errorf("stale emblem compound sequence")
	}
	b, rewards, err := b.CompoundEmblems(s.Catalog, s.BagRules, s.EmblemCompound, req, emblemCompoundDraw)
	if err != nil {
		return nil, nil, err
	}
	b.EmblemCompoundSeq++
	updated, err := inventory.SaveBag(current.State, b)
	if err != nil {
		return nil, nil, err
	}
	receipt := EmblemCompoundReceipt{Inputs: append([]protocol.CompoundEmblemInput(nil), req.Inputs...), Mode: req.Mode, Sequence: sequence, Rewards: rewards, Source: s.Catalog.Source.SaveIdentity()}
	if err := receipt.prepare(updated); err != nil {
		return nil, nil, err
	}
	data, err := json.Marshal(receipt)
	return updated, data, err
}

func (s *Service) PrepareEmblemCompoundReceipt(receipt *EmblemCompoundReceipt, state json.RawMessage) error {
	return receipt.prepare(state)
}
