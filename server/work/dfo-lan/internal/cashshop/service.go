// Package cashshop prices requests on the server before the atomic cash ledger.
package cashshop

import (
	"context"
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"fmt"
	"math"
	"time"
)

type Product struct {
	ID       uint32
	Template uint32
	Units    uint32
	Cera     uint32
	Kind     byte
	Option   byte
	Enabled  bool
	Starts   time.Time
	Ends     time.Time
}
type Catalog struct {
	Source   string
	Products map[uint32]Product
}
type Ledger interface {
	PurchaseCash(context.Context, CashOrder) (CashReceipt, bool, error)
}
type Service struct {
	Catalog Catalog
	Ledger  Ledger
}

// Quote takes no price or recipient from the wire. Unsupported merchandise is
// disabled until its options, restrictions and delivery protocol are recovered.
func (s *Service) Quote(account, character int64, key string, cart []protocol.CeraCartItem, now time.Time) (CashOrder, error) {
	if s == nil {
		return CashOrder{}, fmt.Errorf("cash catalog missing")
	}
	order := CashOrder{Key: key, Account: account, Character: character, Source: s.Catalog.Source}
	source, err := hex.DecodeString(s.Catalog.Source)
	if err != nil || len(source) != 32 || account <= 0 || character <= 0 || len(key) < 16 || len(key) > 128 || len(cart) == 0 || len(cart) > 32 {
		return order, fmt.Errorf("invalid purchase context")
	}
	var total uint64
	for _, item := range cart {
		p, ok := s.Catalog.Products[item.Product]
		if !ok || !p.Enabled || p.ID != item.Product || p.Template == 0 || p.Cera == 0 || p.Units == 0 {
			return order, fmt.Errorf("product %d not enabled for cash delivery", item.Product)
		}
		if item.Kind != p.Kind || item.Option != p.Option {
			return order, fmt.Errorf("unsupported product option")
		}
		if item.Quantity == 0 || item.Quantity > 1000 || uint64(item.Quantity)*uint64(p.Units) > math.MaxUint32 {
			return order, fmt.Errorf("invalid purchase quantity")
		}
		if (!p.Starts.IsZero() && now.Before(p.Starts)) || (!p.Ends.IsZero() && !now.Before(p.Ends)) {
			return order, fmt.Errorf("product not on sale")
		}
		total += uint64(item.Quantity) * uint64(p.Cera)
		if total > math.MaxInt32 {
			return order, fmt.Errorf("purchase exceeds native currency range")
		}
		order.Lines = append(order.Lines, CashOrderLine{Product: p.ID, Template: p.Template, Quantity: item.Quantity, Units: p.Units, UnitPrice: p.Cera})
	}
	return order, nil
}

func (s *Service) Purchase(ctx context.Context, account, character int64, key string, cart []protocol.CeraCartItem, now time.Time) (CashReceipt, bool, error) {
	var r CashReceipt
	if s == nil || s.Ledger == nil {
		return r, false, fmt.Errorf("cash ledger missing")
	}
	o, e := s.Quote(account, character, key, cart, now)
	if e != nil {
		return r, false, e
	}
	return s.Ledger.PurchaseCash(ctx, o)
}
