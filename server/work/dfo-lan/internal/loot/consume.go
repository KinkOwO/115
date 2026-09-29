package loot

import (
	"context"
	crand "crypto/rand"
	"dfolan/internal/adventure"
	"dfolan/internal/cashshop"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

type ConsumeReceipt struct {
	SeasonExperience uint32                `json:"season_experience,omitempty"`
	EventKey         string                `json:"-"`
	Slot             uint16                `json:"slot"`
	Template         uint32                `json:"template"`
	Remaining        uint32                `json:"remaining"`
	Source           string                `json:"source"`
	Premiums         []storage.CashPremium `json:"premiums,omitempty"`
	// Granted records every prize a box handed out, including pity draws, so a
	// replayed request returns the same prizes instead of drawing new ones.
	Granted []ConsumeGrant `json:"granted,omitempty"`
	// Points is the box's pity counter state after this open.
	Points map[string]uint32 `json:"points,omitempty"`
}

// ConsumeGrant is one prize placed in the bag by opening a box.
type ConsumeGrant struct {
	Template uint32 `json:"template"`
	Count    uint32 `json:"count"`
	Group    int32  `json:"group,omitempty"`
}

func boxSeed() (int64, error) {
	var b [8]byte
	if _, e := crand.Read(b[:]); e != nil {
		return 0, e
	}
	return int64(binary.LittleEndian.Uint64(b[:])), nil
}

// Consume settles one stackable use durably. The decrement and its receipt
// commit in the same character transaction the pickup path uses, so a replayed
// or retried request returns the original receipt instead of spending a second
// unit — the native client sends this on a hotkey press, which is exactly the
// kind of input that repeats.
//
// The recovery effect is the client's own: the native success acknowledgement
// carries no restored amount, so nothing is invented here.
func (s *Service) Consume(ctx context.Context, role storage.Character, r protocol.UseStackableRequest) (storage.Character, ConsumeReceipt, bool, error) {
	var out ConsumeReceipt
	fail := func(e error) (storage.Character, ConsumeReceipt, bool, error) {
		return role, out, false, e
	}
	if role.ConfigVersion != s.Catalog.Source.Checksum {
		return fail(fmt.Errorf("consume source mismatch"))
	}
	if r.List != 0 && r.List != 7 {
		// list0 is the ordinary bag. Other containers (the shared temporary
		// inventories) have their own unverified semantics.
		return fail(fmt.Errorf("unsupported source container %d", r.List))
	}
	if inventory.IsReinforcementTicket(r.Template) {
		return fail(fmt.Errorf("强化券必须选择装备后使用，不能作为普通消耗品扣除"))
	}
	seasonRules, e := adventure.CurrentSeason()
	if e != nil {
		return fail(e)
	}
	_, seasonCapsule := seasonRules.Capsules[r.Template]
	key := fmt.Sprintf("consume:%d:%d:%d", r.Slot, r.Template, r.Instance)
	if r.List == 7 {
		key = fmt.Sprintf("consume-pet:%d:%d:%d", r.Slot, r.Template, r.Instance)
	}
	saved, applied, e := s.Store.CommitCharacterPremiumEvent(ctx, role.AccountID, role.ID,
		s.Catalog.Source.Checksum, key, s.Rules.Model,
		func(current storage.Character) (json.RawMessage, json.RawMessage, []storage.CashPremiumActivation, error) {
			b, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, nil, e
			}
			if _, contract := cashshop.ResolveContractItem(r.Template); contract || seasonCapsule {
				for _, row := range b.Items {
					if row.Slot == r.Slot && row.Template == r.Template && protocol.StoredItemExpired(row.ExpireTime, time.Now().Unix()) {
						return nil, nil, nil, fmt.Errorf("物品已过期")
					}
				}
			}
			var remaining uint32
			if r.List == 7 {
				definition, ok := s.Catalog.Items[r.Template]
				if !ok || !inventory.IsPetFeed(definition.StackableType) {
					return nil, nil, nil, fmt.Errorf("unsupported pet consumable effect")
				}
				if !inventory.HasEquippedCreature(current.State) {
					return nil, nil, nil, fmt.Errorf("no equipped creature to feed")
				}
				b, remaining, e = b.ConsumePet(s.Catalog, r.Slot, r.Template)
				if e == nil {
					var fed bool
					b, fed = inventory.FeedEquippedCreatureBag(b)
					if !fed {
						return nil, nil, nil, fmt.Errorf("equipped creature is already fully fed")
					}
				}
			} else {
				b, remaining, e = b.Consume(s.Catalog, r.Slot, r.Template)
			}
			if e != nil {
				return nil, nil, nil, e
			}
			var usedPremiums []storage.CashPremiumActivation
			if contract, ok := cashshop.ResolveContractItem(r.Template); ok {
				usedPremiums = append(usedPremiums, storage.CashPremiumActivation{Type: contract.Type, DurationSecond: contract.DurationSecond})
			}
			// A box spends itself, hands out its source lot row and advances its
			// pity counters in the same character transaction, so a retried
			// hotkey press can neither open it twice nor advance pity twice. An
			// unknown prize family refuses the open instead of dropping the
			// prize into a guessed slot.
			var granted []ConsumeGrant
			var points map[string]uint32
			if table, ok := s.Boxes.Table(r.Template); ok && r.List == 0 {
				seed, e := boxSeed()
				if e != nil {
					return nil, nil, nil, e
				}
				counters, e := readBoxPoints(current.State, r.Template)
				if e != nil {
					return nil, nil, nil, e
				}
				var roll BoxRoll
				roll, points, e = table.Open(rand.New(rand.NewSource(seed)), counters)
				if e != nil {
					return nil, nil, nil, e
				}
				granted = append(granted, roll.Main)
				granted = append(granted, roll.Bonus...)
				granted = append(granted, roll.Section...)
				for _, prize := range granted {
					var premium *storage.CashPremiumActivation
					b, premium, e = s.grantBoxPrize(b, prize)
					if e != nil {
						return nil, nil, nil, e
					}
					if premium != nil {
						usedPremiums = append(usedPremiums, *premium)
					}
				}
			}
			b, premiums, e := s.settleBoxRewardBag(b)
			if e != nil {
				return nil, nil, nil, e
			}
			premiums = append(premiums, usedPremiums...)
			updated, e := inventory.SaveBag(current.State, b)
			if e != nil {
				return nil, nil, nil, e
			}
			var seasonGain uint32
			updated, seasonGain, e = adventure.ApplySeasonCapsule(updated, r.Template, time.Now())
			if e != nil {
				return nil, nil, nil, e
			}
			if points != nil {
				if updated, e = saveBoxPoints(updated, r.Template, points); e != nil {
					return nil, nil, nil, e
				}
			}
			out = ConsumeReceipt{Slot: r.Slot, Template: r.Template, Remaining: remaining,
				Source: s.Catalog.Source.Checksum, Granted: granted, Points: points, SeasonExperience: seasonGain}
			receipt, e := json.Marshal(out)
			return updated, receipt, premiums, e
		})
	if e != nil {
		return fail(e)
	}
	receipt, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return fail(e)
	}
	if e = json.Unmarshal(receipt, &out); e != nil {
		return fail(e)
	}
	if out.Source != s.Catalog.Source.Checksum || out.Template != r.Template || out.Slot != r.Slot {
		return fail(fmt.Errorf("consume receipt conflict"))
	}
	out.EventKey = key
	saved.WireID = role.WireID
	return saved, out, applied, nil
}
