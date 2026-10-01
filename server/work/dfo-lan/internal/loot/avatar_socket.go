package loot

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

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
	var e error
	r.Ack, e = protocol.AddAvatarSocketSuccess(r.Request)
	if e != nil {
		return e
	}
	b, e := inventory.ReadBag(state)
	if e != nil {
		return e
	}
	for _, row := range b.Special[1] {
		if row.Slot == r.Request.AvatarSlot && row.Template == r.Request.Template {
			r.Avatar, e = inventory.EquipmentPayload(1, []inventory.BagEquipment{row}, false)
			if e != nil {
				return e
			}
			r.Inventory, e = protocol.InventoryRestore(b.Rows(), b.Expansion)
			return e
		}
	}
	return fmt.Errorf("avatar socket receipt target missing")
}

// Opening and consuming the device commit together under the character lock.
// A transaction retry returns the saved receipt without consuming another unit.
func (s *Service) AddAvatarSocket(ctx context.Context, role storage.Character, r protocol.AddAvatarSocketRequest) (storage.Character, AvatarSocketReceipt, bool, error) {
	var result AvatarSocketReceipt
	fail := func(e error) (storage.Character, AvatarSocketReceipt, bool, error) {
		return role, AvatarSocketReceipt{}, false, e
	}
	if s == nil || s.Store == nil || s.AvatarSockets == nil || s.Equipment == nil {
		return fail(fmt.Errorf("avatar socket service unavailable"))
	}
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return fail(fmt.Errorf("avatar socket inventory source mismatch"))
	}
	before, e := inventory.ReadBag(role.State)
	if e != nil {
		return fail(e)
	}
	if before.AvatarSocketSeq == math.MaxUint64 {
		return fail(fmt.Errorf("avatar socket sequence exhausted"))
	}
	sequence := before.AvatarSocketSeq
	key := fmt.Sprintf("avatar-socket:%d:%d:%d:%d", sequence, r.AvatarSlot, r.Template, r.DeviceSlot)
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Catalog.Source.SaveIdentity(), key, "avatar-socket-v1",
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			b, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			if b.AvatarSocketSeq != sequence {
				return nil, nil, fmt.Errorf("stale avatar socket sequence")
			}
			b, device, e := b.AddAvatarSocket(s.Catalog, s.Equipment, s.AvatarSockets, r, uint32(time.Now().Unix()))
			if e != nil {
				return nil, nil, e
			}
			b.AvatarSocketSeq++
			updated, e := inventory.SaveBag(current.State, b)
			if e != nil {
				return nil, nil, e
			}
			result = AvatarSocketReceipt{Request: r, DeviceTemplate: device, Sequence: sequence, Source: s.Catalog.Source.SaveIdentity()}
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
	if result.Request != r || result.Sequence != sequence || result.Source != s.Catalog.Source.SaveIdentity() {
		return fail(fmt.Errorf("avatar socket receipt conflict"))
	}
	if e := result.prepare(saved.State); e != nil {
		return fail(e)
	}
	saved.WireID = role.WireID
	return saved, result, applied, nil
}
