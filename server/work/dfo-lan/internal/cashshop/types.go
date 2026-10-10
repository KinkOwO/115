package cashshop

import (
	"dfolan/internal/inventory"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
)

// CashOrder is constructed from a server-side catalog, never client prices.
type CashOrder struct {
	Key          string          `json:"key"`
	Account      int64           `json:"account"`
	Character    int64           `json:"character"`
	Source       string          `json:"source"`
	Lines        []CashOrderLine `json:"lines"`
	DeliveryMode string          `json:"delivery_mode,omitempty"`
	// 零值沿用已发布的金库 1 订单摘要；45 为第二金库，12 为账号金库。
	VaultSpace byte `json:"vault_space,omitempty"`
}
type CashOrderLine struct {
	AvatarOption  byte   `json:"avatar_option,omitempty"`
	Product       uint32 `json:"product"`
	Template      uint32 `json:"template"`
	Quantity      uint32 `json:"quantity"`
	Units         uint32 `json:"units"`
	GoldUnitPrice uint32 `json:"gold_unit_price,omitempty"`
	UnitPrice     uint32 `json:"unit_price"`
}
type CashDelivery struct {
	ID       int64  `json:"id"`
	Product  uint32 `json:"product"`
	Template uint32 `json:"template"`
	Amount   uint32 `json:"amount"`
	Quantity uint32 `json:"quantity,omitempty"`
}
type CashPremium struct {
	Type            uint8 `json:"type"`
	EndTime         int64 `json:"end_time"`
	RemainingSecond int64 `json:"remaining_seconds"`
}
type CashReceipt struct {
	Order          string                `json:"order"`
	Before         uint64                `json:"before"`
	After          uint64                `json:"after"`
	Charged        uint64                `json:"charged"`
	GoldCharged    uint64                `json:"gold_charged,omitempty"`
	Deliveries     []CashDelivery        `json:"deliveries"`
	Premiums       []CashPremium         `json:"premiums,omitempty"`
	CharacterState json.RawMessage       `json:"character_state,omitempty"`
	Vault          *inventory.VaultState `json:"vault,omitempty"`
	VaultSpace     byte                  `json:"vault_space,omitempty"`
	VaultGold      uint32                `json:"vault_gold,omitempty"`
	// SkillTreeUnlocked marks an order that flipped the character's second skill
	// page from locked to unlocked (Skill Type Extension Ticket). The caller
	// re-publishes the actor's USERINFO1 so the client re-reads the selector
	// byte without a relog.
	SkillTreeUnlocked bool `json:"skill_tree_unlocked,omitempty"`
}

func (o CashOrder) Total() (uint64, error) {
	total, _, err := o.Totals()
	return total, err
}

// GoldTotal validates the catalog order and returns its authoritative Gold cost.
func (o CashOrder) GoldTotal() (uint64, error) {
	_, total, err := o.Totals()
	return total, err
}

func (o CashOrder) Totals() (uint64, uint64, error) {
	source, e := hex.DecodeString(o.Source)
	if e != nil || len(source) != 32 || o.Account <= 0 || o.Character <= 0 || len(o.Key) < 16 || len(o.Key) > 128 || len(o.Lines) == 0 || len(o.Lines) > 32 {
		return 0, 0, fmt.Errorf("invalid cash order")
	}
	var total, gold uint64
	for _, l := range o.Lines {
		if l.Product == 0 || l.Template == 0 || l.Quantity == 0 || l.Quantity > 1000 || l.Units == 0 || (l.UnitPrice == 0) == (l.GoldUnitPrice == 0) {
			return 0, 0, fmt.Errorf("invalid cash order line")
		}
		if uint64(l.Quantity)*uint64(l.Units) > math.MaxUint32 {
			return 0, 0, fmt.Errorf("cash delivery amount overflow")
		}
		total += uint64(l.Quantity) * uint64(l.UnitPrice)
		gold += uint64(l.Quantity) * uint64(l.GoldUnitPrice)
		if total > math.MaxInt32 || gold > math.MaxUint32 {
			return 0, 0, fmt.Errorf("cash order price overflow")
		}
	}
	return total, gold, nil
}

type CashPremiumActivation struct {
	Type           uint8
	DurationSecond int64
}
