package inventory

import (
	crand "crypto/rand"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

// PremiumActivation is the account contract effect produced by item use.
type PremiumActivation struct {
	Type           uint8
	DurationSecond int64
}

// Premium preserves the existing receipt JSON consumed by the client flow.
type Premium struct {
	Type            uint8 `json:"type"`
	EndTime         int64 `json:"end_time"`
	RemainingSecond int64 `json:"remaining_seconds"`
}

// ContractResolver resolves the premium activation carried by an item.
type ContractResolver func(uint32) (PremiumActivation, bool)

func (s *ItemService) Bootstrap(role Role) ([]byte, error) {
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return nil, fmt.Errorf("inventory source mismatch")
	}
	b, err := ReadBag(role.State)
	if err != nil {
		return nil, err
	}
	return protocol.InventoryRestore(b.Rows(), b.Expansion)
}

type ConsumeReceipt struct {
	SeasonExperience uint32    `json:"season_experience,omitempty"`
	EventKey         string    `json:"-"`
	Slot             uint16    `json:"slot"`
	Template         uint32    `json:"template"`
	Remaining        uint32    `json:"remaining"`
	Source           string    `json:"source"`
	Premiums         []Premium `json:"premiums,omitempty"`
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

func (s *ItemService) ConsumeKey(role Role, r protocol.UseStackableRequest) (string, error) {
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return "", fmt.Errorf("consume source mismatch")
	}
	if r.List != 0 && r.List != 7 {
		// list0 is the ordinary bag. Other containers (the shared temporary
		// inventories) have their own unverified semantics.
		return "", fmt.Errorf("unsupported source container %d", r.List)
	}
	if IsReinforcementTicket(r.Template) {
		return "", fmt.Errorf("强化券必须选择装备后使用，不能作为普通消耗品扣除")
	}
	key := fmt.Sprintf("consume:%d:%d:%d", r.Slot, r.Template, r.Instance)
	if r.List == 7 {
		key = fmt.Sprintf("consume-pet:%d:%d:%d", r.Slot, r.Template, r.Instance)
	}
	return key, nil
}

// SeasonCapsuleApplier is the consumer capability for the adventure state
// transition, invoked at the original point inside the inventory transaction.
type SeasonCapsuleApplier func(json.RawMessage, uint32, time.Time) (json.RawMessage, uint32, error)

func (s *ItemService) PrepareConsume(current Role, r protocol.UseStackableRequest, seasonCapsule bool, applySeason SeasonCapsuleApplier, resolveContract ContractResolver) (json.RawMessage, json.RawMessage, []PremiumActivation, error) {
	var out ConsumeReceipt
	b, e := ReadBag(current.State)
	if e != nil {
		return nil, nil, nil, e
	}
	if _, contract := resolveContract(r.Template); contract || seasonCapsule {
		for _, row := range b.Items {
			if row.Slot == r.Slot && row.Template == r.Template && protocol.StoredItemExpired(row.ExpireTime, time.Now().Unix()) {
				return nil, nil, nil, fmt.Errorf("物品已过期")
			}
		}
	}
	var remaining uint32
	if r.List == 7 {
		definition, ok := s.Catalog.Items[r.Template]
		if !ok || !IsPetFeed(definition.StackableType) {
			return nil, nil, nil, fmt.Errorf("unsupported pet consumable effect")
		}
		if !HasEquippedCreature(current.State) {
			return nil, nil, nil, fmt.Errorf("no equipped creature to feed")
		}
		b, remaining, e = b.ConsumePet(s.Catalog, r.Slot, r.Template)
		if e == nil {
			var fed bool
			b, fed = FeedEquippedCreatureBag(b)
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
	var usedPremiums []PremiumActivation
	if contract, ok := resolveContract(r.Template); ok {
		usedPremiums = append(usedPremiums, PremiumActivation{Type: contract.Type, DurationSecond: contract.DurationSecond})
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
			var premium *PremiumActivation
			b, premium, e = s.grantBoxPrize(b, prize, resolveContract)
			if e != nil {
				return nil, nil, nil, e
			}
			if premium != nil {
				usedPremiums = append(usedPremiums, *premium)
			}
		}
	}
	b, premiums, e := s.settleBoxRewardBag(b, resolveContract)
	if e != nil {
		return nil, nil, nil, e
	}
	premiums = append(premiums, usedPremiums...)
	updated, e := SaveBag(current.State, b)
	if e != nil {
		return nil, nil, nil, e
	}
	var seasonGain uint32
	updated, seasonGain, e = applySeason(updated, r.Template, time.Now())
	if e != nil {
		return nil, nil, nil, e
	}
	if points != nil {
		if updated, e = saveBoxPoints(updated, r.Template, points); e != nil {
			return nil, nil, nil, e
		}
	}
	out = ConsumeReceipt{Slot: r.Slot, Template: r.Template, Remaining: remaining,
		Source: s.Catalog.Source.SaveIdentity(), Granted: granted, Points: points, SeasonExperience: seasonGain}
	receipt, e := json.Marshal(out)
	return updated, receipt, premiums, e
}
