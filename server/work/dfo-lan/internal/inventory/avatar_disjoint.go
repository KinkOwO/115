package inventory

import (
	"crypto/rand"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
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
	r.Avatars, e = SpecialEquipmentRestorePayload(state, 1)
	if e != nil {
		return e
	}
	b, e := ReadBag(state)
	if e != nil {
		return e
	}
	r.Inventory, e = protocol.InventoryRestore(b.Rows(), b.Expansion)
	return e
}

// AvatarDisjointSequence validates the caller's save identity and provides the
// sequence used to fence the following character-event transaction.
func (s *ItemService) AvatarDisjointSequence(role Role) (uint64, error) {
	if s == nil || s.AvatarDisjoint == nil || s.Equipment == nil {
		return 0, fmt.Errorf("avatar disjoint service unavailable")
	}
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return 0, fmt.Errorf("avatar disjoint inventory source mismatch")
	}
	b, err := ReadBag(role.State)
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
func (s *ItemService) PrepareAvatarDisjoint(current Role, req protocol.DisjointAvatarRequest, sequence uint64) (json.RawMessage, json.RawMessage, error) {
	b, err := ReadBag(current.State)
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
	updated, err := SaveBag(current.State, b)
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

func (s *ItemService) PrepareAvatarDisjointReceipt(receipt *AvatarDisjointReceipt, state json.RawMessage) error {
	return receipt.prepare(state)
}

func (b Bag) DisjointAvatar(c catalog.LootCatalog, bagRules BagRules, eq *EquipmentCatalog, rules *AvatarDisjointRules, r protocol.DisjointAvatarRequest, draw func(uint32) (uint32, error)) (Bag, []protocol.DisjointRewardEntry, error) {
	fail := func(e error) (Bag, []protocol.DisjointRewardEntry, error) { return b, nil, e }
	if rules == nil || eq == nil || draw == nil || rules.Source != c.Source.Checksum || eq.Source.Checksum != c.Source.Checksum || bagRules.Source != c.Source.Checksum {
		return fail(fmt.Errorf("avatar disjoint source unavailable or mismatched"))
	}
	target := -1
	for i, row := range b.Special[1] {
		if row.Slot == r.Slot {
			target = i
			break
		}
	}
	if target < 0 {
		return fail(fmt.Errorf("avatar disjoint target missing"))
	}
	item := b.Special[1][target]
	if item.Template != r.Template {
		return fail(fmt.Errorf("stale avatar disjoint target"))
	}
	if e := item.ValidateRecord(); e != nil {
		return fail(e)
	}
	d, e := eq.definitionResolved(item.Template, 0)
	if e != nil {
		return fail(e)
	}
	if !d.IsAvatar() {
		return fail(fmt.Errorf("avatar disjoint target is not an avatar"))
	}
	if _, blocked := d.Fields["[impossible disjoint]"]; blocked {
		return fail(fmt.Errorf("avatar disjoint prohibited by source"))
	}
	if item.Period != 0 && item.Period != protocol.MaxItemPeriod {
		return fail(fmt.Errorf("limited-period avatar cannot be disjointed"))
	}
	// Embedded emblem byte layouts are not implemented by the current server.
	// Reject such instances rather than consuming unaccounted player value.
	if len(item.AvatarSockets) > 0 {
		return fail(fmt.Errorf("avatar disjoint socket state unsupported"))
	}
	grade := d.Fields["[grade]"]
	if len(grade) != 1 || grade[0].Type != 0 || len(rules.Rolls[grade[0].Value]) == 0 {
		return fail(fmt.Errorf("unsupported source avatar grade"))
	}
	next := b
	next.Special = make(map[byte][]BagEquipment, len(b.Special))
	for space, rows := range b.Special {
		next.Special[space] = append([]BagEquipment(nil), rows...)
	}
	next.Special[1] = append(next.Special[1][:target], next.Special[1][target+1:]...)
	var rewards []protocol.DisjointRewardEntry
	for _, prob := range rules.Rolls[grade[0].Value] {
		n, e := draw(100)
		if e != nil {
			return fail(e)
		}
		if n >= 100 {
			return fail(fmt.Errorf("invalid avatar random draw"))
		}
		col := 0
		for col < 3 && n >= prob[col] {
			n -= prob[col]
			col++
		}
		if col == 3 {
			return fail(fmt.Errorf("invalid avatar probability row"))
		}
		pool := rules.Pools[int32(col+2)]
		if len(pool) == 0 {
			return fail(fmt.Errorf("empty source emblem pool"))
		}
		n, e = draw(uint32(len(pool)))
		if e != nil {
			return fail(e)
		}
		if n >= uint32(len(pool)) {
			return fail(fmt.Errorf("invalid emblem random draw"))
		}
		id := pool[n]
		var slot uint16
		next, slot, e = next.Add(c, bagRules, id, 1)
		if e != nil {
			return fail(e)
		}
		found := false
		for i := range rewards {
			if rewards[i].Slot == slot && rewards[i].Template == id {
				rewards[i].Count++
				found = true
				break
			}
		}
		if !found {
			rewards = append(rewards, protocol.DisjointRewardEntry{Slot: slot, Template: id, Count: 1})
		}
	}
	return next, rewards, nil
}
