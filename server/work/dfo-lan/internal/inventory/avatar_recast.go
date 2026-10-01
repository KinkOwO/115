package inventory

import (
	"crypto/rand"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
)

func recastAvatarDraw(limit uint32) (uint32, error) {
	if limit == 0 {
		return 0, fmt.Errorf("empty recast reward range")
	}
	n, e := rand.Int(rand.Reader, new(big.Int).SetUint64(uint64(limit)))
	if e != nil {
		return 0, e
	}
	return uint32(n.Uint64()), nil
}

func (s *WearService) RecastAvatarBag(role Role, r protocol.RecastAvatarRequest, draw func(uint32) (uint32, error)) (Bag, []protocol.DisjointRewardEntry, []string, error) {
	b, e := ReadBag(role.State)
	if e != nil {
		return b, nil, nil, e
	}
	fail := func(e error) (Bag, []protocol.DisjointRewardEntry, []string, error) { return b, nil, nil, e }
	// The user's single-avatar recast consumes this item and awards source
	// emblems. Six-input mode has not been requested or established.
	if r.Mode != 1 || len(r.Items) != 1 {
		return fail(fmt.Errorf("unsupported avatar recast mode"))
	}
	job, ok := s.Professions.Professions[role.Profession]
	if !ok {
		return fail(fmt.Errorf("avatar recast profession unavailable"))
	}
	request := r.Items[0]
	target := -1
	for i, item := range b.Special[1] {
		if item.Slot == request.Slot {
			target = i
			break
		}
	}
	if target < 0 {
		return fail(fmt.Errorf("avatar recast target missing"))
	}
	item := b.Special[1][target]
	if item.Template != request.Template {
		return fail(fmt.Errorf("stale avatar recast target"))
	}
	if e := item.ValidateRecord(); e != nil {
		return fail(e)
	}
	if item.Period != 0 && item.Period != protocol.MaxItemPeriod {
		return fail(fmt.Errorf("limited-period avatar recast unsupported"))
	}
	if len(item.AvatarOptions) != 0 || len(item.AvatarSockets) != 0 {
		return fail(fmt.Errorf("embedded avatar recast state unsupported"))
	}
	d, e := s.Catalog.Definition(item.Template)
	if e != nil {
		return fail(e)
	}
	kind := d.Fields["[equipment type]"]
	if !d.IsAvatar() || len(kind) == 0 {
		return fail(fmt.Errorf("avatar recast target is not an avatar"))
	}
	if _, blocked := d.Fields["[impossible disjoint]"]; blocked {
		return fail(fmt.Errorf("avatar disjoint prohibited by source"))
	}
	part, ok := s.AvatarRecast.Jobs[job.Job][kind[0].Text]
	grade := d.Fields["[grade]"]
	if !ok || len(grade) != 1 || grade[0].Type != 0 || grade[0].Value != part.Grade {
		return fail(fmt.Errorf("avatar recast grade/part unavailable"))
	}
	attach := d.Fields["[attach type]"]
	if len(attach) != 1 || (attach[0].Text != "[free]" && attach[0].Text != "[trade]" && attach[0].Text != "[avatar trade]") {
		return fail(fmt.Errorf("avatar recast requires tradable avatar"))
	}
	var actor struct {
		Level       byte `json:"level"`
		Advancement byte `json:"advancement"`
	}
	if e := json.Unmarshal(role.State, &actor); e != nil {
		return fail(e)
	}
	if e := WearableBy(d.Fields, kind[0].Text, job.Job, actor.Advancement, actor.Level); e != nil {
		return fail(e)
	}
	optionJob, ok := part.Options[r.Option]
	if !ok || (optionJob != "" && optionJob != "[all]" && optionJob != job.Job) {
		return fail(fmt.Errorf("avatar recast option unavailable for profession"))
	}
	if b.AvatarRecastSeq == math.MaxUint64 {
		return fail(fmt.Errorf("avatar recast sequence exhausted"))
	}
	pools := part.Emblems[r.Option]
	if len(pools) == 0 || draw == nil {
		return fail(fmt.Errorf("avatar recast has no matching source emblem"))
	}
	next := b
	next.Special = make(map[byte][]BagEquipment, len(b.Special))
	for space, rows := range b.Special {
		next.Special[space] = append([]BagEquipment(nil), rows...)
	}
	next.Special[1] = append(next.Special[1][:target], next.Special[1][target+1:]...)
	var rewards []protocol.DisjointRewardEntry
	var matches []string
	for _, weights := range s.AvatarRecast.Rolls[part.Grade] {
		// Keep the source reward count and relative grade weights. A grade
		// without any matching attribute/part is excluded before drawing.
		var total uint32
		for i, weight := range weights {
			if len(pools[int32(i+2)].Templates) > 0 {
				total += weight
			}
		}
		if total == 0 {
			return fail(fmt.Errorf("avatar recast source grades have no matching emblem"))
		}
		n, e := draw(total)
		if e != nil {
			return fail(e)
		}
		if n >= total {
			return fail(fmt.Errorf("invalid recast grade draw"))
		}
		var pool avatarRecastEmblemPool
		for i, weight := range weights {
			candidate := pools[int32(i+2)]
			if len(candidate.Templates) == 0 {
				continue
			}
			if n < weight {
				pool = candidate
				break
			}
			n -= weight
		}
		n, e = draw(uint32(len(pool.Templates)))
		if e != nil {
			return fail(e)
		}
		if n >= uint32(len(pool.Templates)) {
			return fail(fmt.Errorf("invalid recast emblem draw"))
		}
		id := pool.Templates[n]
		var slot uint16
		next, slot, e = next.Add(*s.AvatarRecastLoot, s.BagRules, id, 1)
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
		matches = append(matches, pool.Match)
	}
	if len(rewards) == 0 {
		return fail(fmt.Errorf("empty recast reward"))
	}
	next.AvatarRecastSeq++
	return next, rewards, matches, nil
}

type AvatarRecastReceipt struct {
	Request   protocol.RecastAvatarRequest           `json:"request"`
	Rewards   []protocol.DisjointRewardEntry         `json:"rewards"`
	Matches   []string                               `json:"matches"`
	Sequence  uint64                                 `json:"sequence"`
	Source    string                                 `json:"source"`
	Ack       []byte                                 `json:"-"`
	Avatars   []byte                                 `json:"-"`
	Inventory []byte                                 `json:"-"`
	Popup     []byte                                 `json:"-"`
	Updates   [][protocol.CurrentItemRecordSize]byte `json:"updates"`
}

func (r *AvatarRecastReceipt) Prepare(state json.RawMessage) error {
	var e error
	r.Ack, e = protocol.RecastAvatarConsumed(r.Request)
	if e != nil {
		return e
	}
	r.Avatars, e = SpecialEquipmentRestorePayload(state, 1)
	if e != nil {
		return e
	}
	r.Inventory, e = protocol.InventoryUpdate(r.Updates)
	if e != nil {
		return e
	}
	r.Popup, e = protocol.AvatarRewardPopup(r.Rewards)
	return e
}

// DrawRecastAvatar supplies the random draw used by the workflow transaction.
func DrawRecastAvatar(limit uint32) (uint32, error) { return recastAvatarDraw(limit) }
