package loot

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

type MoonRewardChoice struct{ Template, Count, Weight uint32 }
type MoonRewardPolicy struct {
	Source  string
	Draws   uint32
	Choices []MoonRewardChoice
}
type MoonRewardGrant struct {
	Template, Count uint32
	Record          []byte
}
type MoonRewardPlan struct {
	Source, Run, Rules string
	Account, Character int64
	Grants             []MoonRewardGrant
}

const moonRewardModel = "moon-solo-clear-v1"

var ErrMoonBagFull = errors.New("Moon reward pending: bag full")

func (s *Service) ValidateMoonRewards(p MoonRewardPolicy) error {
	hash, e := hex.DecodeString(p.Source)
	if e != nil || len(hash) != 32 || s == nil || p.Source != s.Catalog.Source.SaveIdentity() || p.Draws == 0 || p.Draws > 16 || len(p.Choices) == 0 || len(p.Choices) > 4096 {
		return fmt.Errorf("invalid Moon reward policy/source")
	}
	var total uint64
	for _, v := range p.Choices {
		if v.Template == 0 || v.Count == 0 || v.Weight == 0 {
			return fmt.Errorf("invalid Moon reward choice")
		}
		total += uint64(v.Weight)
		if _, e := s.moonGrant(v); e != nil {
			return e
		}
	}
	if total == 0 {
		return fmt.Errorf("empty Moon weight")
	}
	return nil
}
func (s *Service) moonGrant(v MoonRewardChoice) (MoonRewardGrant, error) {
	g := MoonRewardGrant{Template: v.Template, Count: v.Count}
	if item, ok := s.Catalog.Items[v.Template]; ok && item.Kind == "stackable" {
		limit := item.StackLimit
		if limit == 0 {
			limit = s.BagRules.MissingStackLimit
		}
		if v.Count > limit {
			return g, fmt.Errorf("Moon reward exceeds stack limit")
		}
		if _, ok := s.BagRules.Slots[item.StackableType]; !ok {
			return g, fmt.Errorf("Moon stack destination missing")
		}
		return g, nil
	}
	if s.Equipment == nil || v.Count != 1 {
		return g, fmt.Errorf("Moon equipment needs source and count1")
	}
	def, e := s.Equipment.Definition(v.Template)
	if e != nil {
		return g, e
	}
	kind := def.Fields["[equipment type]"]
	if len(kind) == 0 || inventory.EquipmentBagSpace(kind[0].Text) != 0 {
		return g, fmt.Errorf("Moon reward requires ordinary equipment")
	}
	durability, e := s.Equipment.Reward(v.Template)
	if e != nil {
		return g, e
	}
	row := protocol.OrdinaryItem(0, v.Template, 0)
	binary.LittleEndian.PutUint16(row[11:], durability)
	g.Record = append([]byte(nil), row[:]...)
	return g, nil
}
func (s *Service) FreezeMoonReward(ctx context.Context, role storage.Character, run *dungeon.Session, p MoonRewardPolicy) (MoonRewardPlan, error) {
	var out MoonRewardPlan
	if s == nil || s.Store == nil || run == nil || !moonRunID(run.RunID) || run.Definition.ID != 100004137 || !run.Completed() || role.ConfigVersion != p.Source {
		return out, fmt.Errorf("Moon reward before owned final")
	}
	if e := s.ValidateMoonRewards(p); e != nil {
		return out, e
	}
	raw, _ := json.Marshal(p)
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	out = MoonRewardPlan{Source: p.Source, Run: run.RunID, Rules: digest, Account: role.AccountID, Character: role.ID}
	total := uint64(0)
	for _, v := range p.Choices {
		total += uint64(v.Weight)
	}
	for i := uint32(0); i < p.Draws; i++ {
		seed := sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%d/%d/%d", digest, run.RunID, role.AccountID, role.ID, i)))
		roll := binary.LittleEndian.Uint64(seed[:]) % total
		for _, v := range p.Choices {
			if roll < uint64(v.Weight) {
				g, e := s.moonGrant(v)
				if e != nil {
					return out, e
				}
				out.Grants = append(out.Grants, g)
				break
			}
			roll -= uint64(v.Weight)
		}
	}
	body, e := json.Marshal(out)
	if e != nil {
		return out, e
	}
	_, _, e = s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, p.Source, "moon-clear:"+run.RunID, moonRewardModel, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		return current.State, body, nil
	})
	if e != nil {
		return out, e
	}
	return s.ReadMoonReward(ctx, role, run.RunID)
}
func (s *Service) ReadMoonReward(ctx context.Context, role storage.Character, run string) (MoonRewardPlan, error) {
	var p MoonRewardPlan
	if s == nil || s.Store == nil || !moonRunID(run) {
		return p, fmt.Errorf("invalid Moon reward owner/run")
	}
	raw, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, "moon-clear:"+run)
	if e != nil {
		return p, e
	}
	if e = json.Unmarshal(raw, &p); e != nil {
		return p, e
	}
	if p.Run != run || p.Source != role.ConfigVersion || p.Source != s.Catalog.Source.SaveIdentity() || p.Account != role.AccountID || p.Character != role.ID || len(p.Grants) == 0 || len(p.Grants) > 16 {
		return p, fmt.Errorf("foreign/corrupt Moon reward proof")
	}
	return p, nil
}
func (s *Service) ClaimMoonReward(ctx context.Context, role storage.Character, run string) (storage.Character, bool, error) {
	p, e := s.ReadMoonReward(ctx, role, run)
	if e != nil {
		return role, false, e
	}
	saved, fresh, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, p.Source, "moon-grant:"+run, moonRewardModel, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		state, e := s.applyMoonRewards(current.State, p)
		if e != nil {
			return nil, nil, e
		}
		receipt, _ := json.Marshal(p)
		return state, receipt, nil
	})
	saved.WireID = role.WireID
	return saved, fresh, e
}
func moonRunID(run string) bool { v, e := hex.DecodeString(run); return e == nil && len(v) == 16 }
func (s *Service) applyMoonRewards(state json.RawMessage, p MoonRewardPlan) (json.RawMessage, error) {
	bag, e := inventory.ReadBag(state)
	if e != nil {
		return nil, e
	}
	for _, g := range p.Grants {
		if len(g.Record) == 0 {
			bag, _, e = bag.Add(s.Catalog, s.BagRules, g.Template, g.Count)
		} else {
			if len(g.Record) != protocol.CurrentItemRecordSize || g.Count != 1 || binary.LittleEndian.Uint32(g.Record[2:]) != g.Template {
				return nil, fmt.Errorf("invalid frozen Moon equipment")
			}
			var slots []uint16
			bag, slots, e = bag.AddEquipment(s.Equipment, s.BagRules.EquipmentSlots, g.Template, 1)
			if e == nil {
				for i := range bag.Equipment {
					if bag.Equipment[i].Slot == slots[0] {
						bag.Equipment[i].Record = append([]byte(nil), g.Record...)
						bag.Equipment[i].Durability = binary.LittleEndian.Uint16(g.Record[11:])
					}
				}
			}
		}
		if e != nil {
			// Upstream inventory exposes these two capacity errors as strings.
			// Do not convert source/DB/corruption failures into retryable full-bag.
			if e.Error() == "equipment bag is full" || e.Error() == "bag category is full" {
				return nil, ErrMoonBagFull
			}
			return nil, e
		}
	}
	return inventory.SaveBag(state, bag)
}
func (p MoonRewardPlan) WireRows() []protocol.ConquestRewardValue115 {
	out := make([]protocol.ConquestRewardValue115, 0, len(p.Grants))
	for _, g := range p.Grants {
		r := protocol.ConquestRewardValue115{Template: g.Template, Value: g.Count}
		if len(g.Record) == protocol.CurrentItemRecordSize {
			r.Equipment = true
			r.Value = binary.LittleEndian.Uint32(g.Record[6:])
		}
		out = append(out, r)
	}
	return out
}

// Recovery is per owning character and only discovers durable, unclaimed
// clear plans. Full bags retain their plan. It never re-rolls with new policy.
func (s *Service) RecoverMoonRewards(ctx context.Context, role storage.Character) (storage.Character, error) {
	rows, e := s.Store.DB.Query(ctx, `SELECT substr(event_key,12) FROM character_events e WHERE character_id=$1 AND model=$2 AND event_key LIKE 'moon-clear:%' AND NOT EXISTS(SELECT 1 FROM character_events g WHERE g.character_id=e.character_id AND g.event_key='moon-grant:'||substr(e.event_key,12)) ORDER BY created_at LIMIT 16`, role.ID, moonRewardModel)
	if e != nil {
		return role, e
	}
	var runs []string
	for rows.Next() {
		var r string
		if e = rows.Scan(&r); e != nil {
			rows.Close()
			return role, e
		}
		runs = append(runs, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return role, e
	}
	for _, run := range runs {
		saved, _, err := s.ClaimMoonReward(ctx, role, run)
		if err != nil {
			return role, err
		}
		role = saved
	}
	return role, nil
}
