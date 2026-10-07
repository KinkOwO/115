package inventory

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/binary"
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
	b, err := ReadBag(state)
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
			r.Avatar, err = EquipmentPayload(1, []BagEquipment{row}, false)
			if err != nil {
				return err
			}
			r.Inventory, err = protocol.InventoryRestore(b.Rows(), b.Expansion)
			return err
		}
	}
	return fmt.Errorf("avatar emblem receipt target missing")
}

func (s *ItemService) EmblemInlaySequence(role Role) (uint64, error) {
	if s == nil || s.EmblemInlay == nil || s.Equipment == nil {
		return 0, fmt.Errorf("avatar emblem service unavailable")
	}
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return 0, fmt.Errorf("avatar emblem inventory source mismatch")
	}
	b, err := ReadBag(role.State)
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

func (s *ItemService) PrepareEmblemInlay(current Role, req protocol.UseEmblemRequest, sequence uint64) (json.RawMessage, json.RawMessage, error) {
	b, err := ReadBag(current.State)
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
	updated, err := SaveEmblemInlay(current.State, b)
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

func (s *ItemService) PrepareEmblemInlayReceipt(receipt *EmblemInlayReceipt, state json.RawMessage) error {
	return receipt.prepare(state)
}

func (b Bag) UseEmblems(c catalog.LootCatalog, eq *EquipmentCatalog, rules *EmblemInlayRules, req protocol.UseEmblemRequest, now int64) (Bag, error) {
	fail := func(err error) (Bag, error) { return b, err }
	if err := req.Validate(); err != nil {
		return fail(err)
	}
	if eq == nil || rules == nil || rules.Source != c.Source.Checksum || eq.Source.Checksum != c.Source.Checksum {
		return fail(fmt.Errorf("avatar emblem source unavailable or mismatched"))
	}
	target := -1
	for i, row := range b.Special[1] {
		if row.Slot == req.AvatarSlot {
			if target >= 0 {
				return fail(fmt.Errorf("duplicate avatar emblem target slot"))
			}
			target = i
		}
	}
	if target < 0 || b.Special[1][target].Template != req.Template {
		return fail(fmt.Errorf("missing or stale avatar emblem target"))
	}
	item := b.Special[1][target]
	if err := item.ValidateRecord(); err != nil {
		return fail(err)
	}
	if _, err := eq.definitionResolved(item.Template, 0); err != nil {
		return fail(err)
	}
	// 2026-10-07（合并源码候选）：时装徽章入口不再限定 [skin avatar] 的
	// "2 × [M socket]" 布局。客户端显示的孔**完全来自存档 avatar_options**，
	// 而 115 级时装/光环按 PVF 定义可带 [S socket]（白金）与最多 5 个孔；
	// 原门禁会让这些时装镶嵌被 "unsupported source skin avatar socket layout"
	// 拒绝。现在改为按请求的孔位与定义生成的孔掩码逐孔判定。
	if len(item.AvatarOptions) > avatarSocketOptionsSize || len(item.AvatarOptions)%6 != 0 || (len(item.AvatarSockets) != 0 && len(item.AvatarSockets) != 4) {
		return fail(fmt.Errorf("unsupported or unopened avatar emblem extension"))
	}
	options := make([]byte, avatarSocketOptionsSize)
	if len(item.AvatarOptions) == 0 {
		// 老存档时装在默认孔规则上线前发放，扩展为空。按 PVF 时装定义的
		// 默认孔就地补上，否则嵌徽章会被 "socket is not open" 拒绝，
		// 客户端也看不到孔（补 0 孔 = 定义本身无默认孔，仍按原规则拒绝）。
		copy(options, eq.DefaultAvatarSockets(item.Template))
	} else {
		copy(options, item.AvatarOptions)
	}
	used := map[uint16]uint32{}
	for _, in := range req.Inputs {
		if in.Socket >= 5 {
			return fail(fmt.Errorf("avatar emblem socket index out of range"))
		}
		offset := int(in.Socket) * 6
		socket := binary.LittleEndian.Uint16(options[offset:])
		if socket == 0 {
			return fail(fmt.Errorf("avatar emblem socket is not open"))
		}
		mask := rules.Masks[in.Template]
		definition, ok := c.Items[in.Template]
		if !ok || definition.Kind != "stackable" || definition.StackableType != "[avatar emblem]" || mask&socket == 0 {
			return fail(fmt.Errorf("avatar emblem does not match socket color"))
		}
		if binary.LittleEndian.Uint32(options[offset+2:]) == in.Template {
			return fail(fmt.Errorf("avatar socket already contains this emblem"))
		}
		found := -1
		for i, row := range b.Items {
			if row.Slot == in.Slot {
				if found >= 0 {
					return fail(fmt.Errorf("duplicate avatar emblem inventory slot"))
				}
				found = i
			}
		}
		used[in.Slot]++
		if found < 0 || b.Items[found].Template != in.Template || b.Items[found].Amount < used[in.Slot] || protocol.StoredItemExpired(b.Items[found].ExpireTime, now) {
			return fail(fmt.Errorf("missing, stale, expired or insufficient avatar emblem stack"))
		}
		binary.LittleEndian.PutUint32(options[offset+2:], in.Template)
	}
	// All validation precedes any mutation. Clone both changed slices and keep
	// every other instance field, extension, currency and inventory space.
	next := b
	next.Items = make([]BagItem, 0, len(b.Items))
	for _, row := range b.Items {
		count := used[row.Slot]
		if count == 0 {
			next.Items = append(next.Items, row)
			continue
		}
		row.Amount -= count
		if row.Amount > 0 {
			next.Items = append(next.Items, row)
		}
	}
	next.Special = make(map[byte][]BagEquipment, len(b.Special))
	for space, rows := range b.Special {
		next.Special[space] = append([]BagEquipment(nil), rows...)
	}
	item.AvatarOptions = options
	next.Special[1][target] = item
	return next, nil
}

// SaveEmblemInlay writes only the three inventory fields owned by this
// operation. Other inventory fields (including newer avatar capacity fields)
// and other special spaces must survive a mixed-version source integration.
func SaveEmblemInlay(state json.RawMessage, b Bag) (json.RawMessage, error) {
	updated, err := SaveBag(state, b)
	if err != nil {
		return nil, err
	}
	var original, next map[string]json.RawMessage
	if err := json.Unmarshal(state, &original); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(updated, &next); err != nil {
		return nil, err
	}
	var oldBag, newBag map[string]json.RawMessage
	if err := json.Unmarshal(original["inventory"], &oldBag); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(next["inventory"], &newBag); err != nil {
		return nil, err
	}
	var oldSpaces, newSpaces map[string]json.RawMessage
	if err := json.Unmarshal(oldBag["special_equipment"], &oldSpaces); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(newBag["special_equipment"], &newSpaces); err != nil {
		return nil, err
	}
	if oldBag == nil || oldSpaces == nil || len(newSpaces["1"]) == 0 {
		return nil, fmt.Errorf("missing saved avatar emblem inventory")
	}
	oldBag["items"] = newBag["items"]
	oldBag["emblem_inlay_seq"] = newBag["emblem_inlay_seq"]
	oldSpaces["1"] = newSpaces["1"]
	oldBag["special_equipment"], err = json.Marshal(oldSpaces)
	if err != nil {
		return nil, err
	}
	original["inventory"], err = json.Marshal(oldBag)
	if err != nil {
		return nil, err
	}
	return json.Marshal(original)
}
