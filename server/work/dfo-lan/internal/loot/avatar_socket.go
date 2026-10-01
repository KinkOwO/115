package loot

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

const AvatarSocketModel = "avatar-socket-v1"

type AvatarSocketReceipt struct {
	Request        protocol.AddAvatarSocketRequest `json:"request"`
	DeviceTemplate uint32                          `json:"device_template"`
	Sequence       uint64                          `json:"sequence"`
	Source         string                          `json:"source"`
	Ack            []byte                          `json:"-"`
	Avatar         []byte                          `json:"-"`
	Inventory      []byte                          `json:"-"`
}

func (r *AvatarSocketReceipt) prepare(state json.RawMessage) error {
	var err error
	r.Ack, err = protocol.AddAvatarSocketSuccess(r.Request)
	if err != nil {
		return err
	}
	b, err := inventory.ReadBag(state)
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
	return fmt.Errorf("avatar socket receipt target missing")
}

// AvatarSocketSequence validates the save identity and returns the sequence
// used to fence the workflow transaction.
func (s *Service) AvatarSocketSequence(role Role) (uint64, error) {
	if s == nil || s.AvatarSockets == nil || s.Equipment == nil {
		return 0, fmt.Errorf("avatar socket service unavailable")
	}
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return 0, fmt.Errorf("avatar socket inventory source mismatch")
	}
	b, err := inventory.ReadBag(role.State)
	if err != nil {
		return 0, err
	}
	if b.AvatarSocketSeq == math.MaxUint64 {
		return 0, fmt.Errorf("avatar socket sequence exhausted")
	}
	return b.AvatarSocketSeq, nil
}

// PrepareAvatarSocket performs the loot-domain state change. The workflow owns
// persistence and receipt replay.
func (s *Service) PrepareAvatarSocket(current Role, req protocol.AddAvatarSocketRequest, sequence uint64) (json.RawMessage, json.RawMessage, error) {
	b, err := inventory.ReadBag(current.State)
	if err != nil {
		return nil, nil, err
	}
	if b.AvatarSocketSeq != sequence {
		return nil, nil, fmt.Errorf("stale avatar socket sequence")
	}
	b, device, err := b.AddAvatarSocket(s.Catalog, s.Equipment, s.AvatarSockets, req, uint32(time.Now().Unix()))
	if err != nil {
		return nil, nil, err
	}
	b.AvatarSocketSeq++
	updated, err := inventory.SaveBag(current.State, b)
	if err != nil {
		return nil, nil, err
	}
	receipt := AvatarSocketReceipt{Request: req, DeviceTemplate: device, Sequence: sequence, Source: s.Catalog.Source.SaveIdentity()}
	if err := receipt.prepare(updated); err != nil {
		return nil, nil, err
	}
	data, err := json.Marshal(receipt)
	return updated, data, err
}

func (s *Service) PrepareAvatarSocketReceipt(receipt *AvatarSocketReceipt, state json.RawMessage) error {
	return receipt.prepare(state)
}
