package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/binary"
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
	b, err := ReadBag(state)
	if err != nil {
		return err
	}
	for _, row := range b.Special[1] {
		if row.Slot == r.Request.AvatarSlot && row.Template == r.Request.Template {
			r.Avatar, err = EquipmentPayload(1, []BagEquipment{row}, false)
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
func (s *ItemService) AvatarSocketSequence(role Role) (uint64, error) {
	if s == nil || s.AvatarSockets == nil || s.Equipment == nil {
		return 0, fmt.Errorf("avatar socket service unavailable")
	}
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return 0, fmt.Errorf("avatar socket inventory source mismatch")
	}
	b, err := ReadBag(role.State)
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
func (s *ItemService) PrepareAvatarSocket(current Role, req protocol.AddAvatarSocketRequest, sequence uint64) (json.RawMessage, json.RawMessage, error) {
	b, err := ReadBag(current.State)
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
	updated, err := SaveBag(current.State, b)
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

func (s *ItemService) PrepareAvatarSocketReceipt(receipt *AvatarSocketReceipt, state json.RawMessage) error {
	return receipt.prepare(state)
}

// Native 145770370 reads five packed (u16 socket type, u32 emblem) entries.
// 65519 is the M socket accepted by its second pass; zero has no socket entry.
const avatarMultiSocket uint16 = 65519

func (b Bag) AddAvatarSocket(c catalog.LootCatalog, eq *EquipmentCatalog, rules *AvatarSocketRules, r protocol.AddAvatarSocketRequest, now uint32) (Bag, uint32, error) {
	fail := func(e error) (Bag, uint32, error) { return b, 0, e }
	if rules == nil || eq == nil || rules.Source != c.Source.Checksum || eq.Source.Checksum != c.Source.Checksum {
		return fail(fmt.Errorf("avatar socket source unavailable or mismatched"))
	}
	if _, e := protocol.AddAvatarSocketSuccess(r); e != nil {
		return fail(e)
	}
	device := -1
	for i, row := range b.Items {
		if row.Slot == r.DeviceSlot {
			if device >= 0 {
				return fail(fmt.Errorf("duplicate avatar socket device slot"))
			}
			device = i
		}
	}
	if device < 0 {
		return fail(fmt.Errorf("avatar socket device missing"))
	}
	tool := b.Items[device]
	// Only the source mode 3 skin device is implemented by this native flow.
	if rules.Devices[tool.Template] != 3 || tool.Amount == 0 {
		return fail(fmt.Errorf("unsupported or empty avatar socket device"))
	}
	if protocol.StoredItemExpired(tool.ExpireTime, int64(now)) {
		return fail(fmt.Errorf("avatar socket device expired"))
	}
	target := -1
	for i, row := range b.Special[1] {
		if row.Slot == r.AvatarSlot {
			if target >= 0 {
				return fail(fmt.Errorf("duplicate avatar socket target slot"))
			}
			target = i
		}
	}
	if target < 0 {
		return fail(fmt.Errorf("avatar socket target missing"))
	}
	item := b.Special[1][target]
	if item.Template != r.Template {
		return fail(fmt.Errorf("stale avatar socket target"))
	}
	if e := item.ValidateRecord(); e != nil {
		return fail(e)
	}
	d, e := eq.definitionResolved(item.Template, 0)
	if e != nil {
		return fail(e)
	}
	typeCells := d.Fields["[equipment type]"]
	if !d.IsAvatar() || len(typeCells) != 2 || typeCells[0].Text != "[skin avatar]" {
		return fail(fmt.Errorf("skin avatar socket device requires a skin avatar"))
	}
	// Keep this operation bounded to the current source's two M socket skins.
	// Unknown script layouts are refused before consuming any player item.
	selectCells := d.Fields["[avatar type select]"]
	n := len(selectCells)
	if n < 3 || selectCells[n-3].Type != 0 || selectCells[n-3].Value != 2 || selectCells[n-2].Text != "[M socket]" || selectCells[n-1].Text != "[M socket]" {
		return fail(fmt.Errorf("unsupported source skin avatar socket layout"))
	}
	if len(item.AvatarOptions) > 30 || len(item.AvatarOptions)%6 != 0 || (len(item.AvatarSockets) != 0 && len(item.AvatarSockets) != 4) {
		return fail(fmt.Errorf("unsupported avatar extension layout"))
	}
	options := make([]byte, 30)
	copy(options, item.AvatarOptions)
	for i := 0; i < 5; i++ {
		if binary.LittleEndian.Uint16(options[i*6:]) != 0 || binary.LittleEndian.Uint32(options[i*6+2:]) != 0 {
			return fail(fmt.Errorf("avatar already has socket or emblem data"))
		}
	}
	binary.LittleEndian.PutUint16(options, avatarMultiSocket)
	binary.LittleEndian.PutUint16(options[6:], avatarMultiSocket)
	next, _, e := b.Consume(c, r.DeviceSlot, tool.Template)
	if e != nil {
		return fail(e)
	}
	next.Special = make(map[byte][]BagEquipment, len(b.Special))
	for space, rows := range b.Special {
		next.Special[space] = append([]BagEquipment(nil), rows...)
	}
	item.AvatarOptions = options
	next.Special[1][target] = item
	return next, tool.Template, nil
}
