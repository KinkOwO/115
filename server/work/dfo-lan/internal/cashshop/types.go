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
	Product   uint32 `json:"product"`
	Template  uint32 `json:"template"`
	Quantity  uint32 `json:"quantity"`
	Units     uint32 `json:"units"`
	UnitPrice uint32 `json:"unit_price"`
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
	Deliveries     []CashDelivery        `json:"deliveries"`
	Premiums       []CashPremium         `json:"premiums,omitempty"`
	CharacterState json.RawMessage       `json:"character_state,omitempty"`
	Vault          *inventory.VaultState `json:"vault,omitempty"`
	VaultSpace     byte                  `json:"vault_space,omitempty"`
	VaultGold      uint32                `json:"vault_gold,omitempty"`
}

func (o CashOrder) Total() (uint64, error) {
	source, e := hex.DecodeString(o.Source)
	if e != nil || len(source) != 32 || o.Account <= 0 || o.Character <= 0 || len(o.Key) < 16 || len(o.Key) > 128 || len(o.Lines) == 0 || len(o.Lines) > 32 {
		return 0, fmt.Errorf("invalid cash order")
	}
	var total uint64
	for _, l := range o.Lines {
		if l.Product == 0 || l.Template == 0 || l.Quantity == 0 || l.Quantity > 1000 || l.Units == 0 || l.UnitPrice == 0 {
			return 0, fmt.Errorf("invalid cash order line")
		}
		if uint64(l.Quantity)*uint64(l.Units) > math.MaxUint32 {
			return 0, fmt.Errorf("cash delivery amount overflow")
		}
		total += uint64(l.Quantity) * uint64(l.UnitPrice)
		if total > math.MaxInt32 {
			return 0, fmt.Errorf("cash order price overflow")
		}
	}
	return total, nil
}

type CashPremiumActivation struct {
	Type           uint8
	DurationSecond int64
}
