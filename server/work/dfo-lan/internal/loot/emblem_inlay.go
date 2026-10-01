package loot

import (
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

const EmblemInlayModel = "avatar-emblem-inlay-v1"

type EmblemInlayReceipt struct {
	Request   protocol.UseEmblemRequest `json:"request"`
	Sequence  uint64                    `json:"sequence"`
	Source    string                    `json:"source"`
	Ack       []byte                    `json:"-"`
	Avatar    []byte                    `json:"-"`
	Inventory []byte                    `json:"-"`
}

func (r *EmblemInlayReceipt) prepare(state json.RawMessage) error {
	b, err := inventory.ReadBag(state)
	if err != nil {
		return err
	}
	var amounts []protocol.EmblemStackAmount
	seen := map[uint16]bool{}
	for _, in := range r.Request.Inputs {
		if seen[in.Slot] {
			continue
		}
		seen[in.Slot] = true
		row := protocol.EmblemStackAmount{Slot: in.Slot}
		for _, item := range b.Items {
			if item.Slot == in.Slot {
				row.Amount = item.Amount
				break
			}
		}
		amounts = append(amounts, row)
	}
	r.Ack, err = protocol.UseEmblemSuccess(amounts)
	if err != nil {
		return err
	}
	for _, row := range b.Special[1] {
		if row.Slot == r.Request.AvatarSlot && row.Template == r.Request.Template {
			r.Avatar, err = inventory.EquipmentPayload(1, []inventory.BagEquipment{row}, false)
			if err != nil {
				return err
			}
			r.Inventory, err = protocol.InventoryRestore(b.Rows(), b.Expansion)
			return err
		}
	}
	return fmt.Errorf("avatar emblem receipt target missing")
}

func (s *Service) EmblemInlaySequence(role Role) (uint64, error) {
	if s == nil || s.EmblemInlay == nil || s.Equipment == nil {
		return 0, fmt.Errorf("avatar emblem service unavailable")
	}
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return 0, fmt.Errorf("avatar emblem inventory source mismatch")
	}
	b, err := inventory.ReadBag(role.State)
	if err != nil {
		return 0, err
	}
	if b.EmblemInlaySeq == math.MaxUint64 {
		return 0, fmt.Errorf("avatar emblem sequence exhausted")
	}
	return b.EmblemInlaySeq, nil
}

func EmblemInlayEventKey(sequence uint64, req protocol.UseEmblemRequest) (string, error) {
	if err := req.Validate(); err != nil {
		return "", err
	}
	identity, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("avatar-emblem:%d:%x", sequence, sha256.Sum256(identity)), nil
}

func (s *Service) PrepareEmblemInlay(current Role, req protocol.UseEmblemRequest, sequence uint64) (json.RawMessage, json.RawMessage, error) {
	b, err := inventory.ReadBag(current.State)
	if err != nil {
		return nil, nil, err
	}
	if b.EmblemInlaySeq != sequence || sequence == math.MaxUint64 {
		return nil, nil, fmt.Errorf("stale or exhausted avatar emblem sequence")
	}
	b, err = b.UseEmblems(s.Catalog, s.Equipment, s.EmblemInlay, req, time.Now().Unix())
	if err != nil {
		return nil, nil, err
	}
	b.EmblemInlaySeq++
	updated, err := inventory.SaveEmblemInlay(current.State, b)
	if err != nil {
		return nil, nil, err
	}
	receipt := EmblemInlayReceipt{Request: req, Sequence: sequence, Source: s.Catalog.Source.SaveIdentity()}
	if err := receipt.prepare(updated); err != nil {
		return nil, nil, err
	}
	data, err := json.Marshal(receipt)
	return updated, data, err
}

func (s *Service) PrepareEmblemInlayReceipt(receipt *EmblemInlayReceipt, state json.RawMessage) error {
	return receipt.prepare(state)
}
