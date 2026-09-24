package cashshop

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// contractCartLedger captures the mixed-order dispatch: contract activations
// plus the ordinary delivery of the same cart.
type contractCartLedger struct {
	state         json.RawMessage
	mixedCalls    int
	mixedOrder    storage.CashOrder
	mixedPremiums map[int]storage.CashPremiumActivation
}

func (l *contractCartLedger) PurchaseCashToBag(_ context.Context, o storage.CashOrder, fn func(json.RawMessage) (json.RawMessage, error)) (storage.CashReceipt, bool, error) {
	state, e := fn(l.state)
	if e != nil {
		return storage.CashReceipt{}, false, e
	}
	l.state = state
	return storage.CashReceipt{}, true, nil
}

func (l *contractCartLedger) PurchaseCashMixed(_ context.Context, o storage.CashOrder, fn func(json.RawMessage) (json.RawMessage, error), premiums map[int]storage.CashPremiumActivation) (storage.CashReceipt, bool, error) {
	state, e := fn(l.state)
	if e != nil {
		return storage.CashReceipt{}, false, e
	}
	l.state = state
	l.mixedCalls++
	l.mixedOrder = o
	l.mixedPremiums = premiums
	end := time.Now().Unix() + 3600
	var out []storage.CashPremium
	for _, act := range premiums {
		out = append(out, storage.CashPremium{Type: act.Type, EndTime: end})
	}
	return storage.CashReceipt{Order: o.Key, Premiums: out}, true, nil
}

// contractCartCatalog mirrors the live [item period or contract] rows
// (Tactician 45, Conqueror 34, Cube 10000389) plus one ordinary product.
func contractCartCatalog(t *testing.T) PilotConfig {
	t.Helper()
	hash := strings.Repeat("a", 64)
	contractRow := func(product, template, cera int32) []pvf.Token {
		row := make([]pvf.Token, 14)
		row[0].Value = product
		row[1].Value = template
		row[2].Value = 1
		row[5].Value = cera
		row[8] = pvf.Token{Type: 6, Text: "contract"}
		row[9].Value = 90
		row[11].Value = -1
		row[12] = pvf.Token{Type: 6}
		row[13].Value = -1
		return row
	}
	ordinaryRow := func(product, template, cera int32) []pvf.Token {
		row := make([]pvf.Token, 14)
		row[0].Value = product
		row[1].Value = template
		row[2].Value = 1
		row[5].Value = cera
		row[8] = pvf.Token{Type: 6, Text: "potion"}
		row[9].Value = 0
		row[11].Value = -1
		row[12] = pvf.Token{Type: 6}
		row[13].Value = -1
		return row
	}
	var cells []pvf.Token
	appendSection := func(name string, rows [][]pvf.Token) {
		cells = append(cells, pvf.Token{Type: 3, Text: name})
		for _, row := range rows {
			cells = append(cells, row...)
		}
	}
	appendSection("[item period or contract]", [][]pvf.Token{
		contractRow(3500001, 45, 280),
		contractRow(3500009, 34, 420),
		contractRow(3500016, 10000389, 85),
	})
	appendSection("[item]", [][]pvf.Token{ordinaryRow(3000999, 15, 100)})
	shop := catalog.ScriptRecord{SHA256: hash, Cells: cells}
	var index catalog.ScriptRecord
	for id, name := range map[uint32]string{45: "stackable/cash/c1.stk", 34: "stackable/cash/c2.stk", 10000389: "stackable/cash/c3.stk", 15: "stackable/cash/p.stk"} {
		index.Cells = append(index.Cells, pvf.Token{Value: int32(id)}, pvf.Token{Type: 6, Text: name})
	}
	index.SHA256 = hash
	c, e := importShopScripts(pvf.ArchiveSnapshot{Checksum: hash}, shop, index, func(name string) (catalog.ScriptRecord, error) {
		return catalog.ScriptRecord{SHA256: hash, Path: name, Cells: []pvf.Token{{Type: 3, Text: "[stackable type]"}, {Type: 6, Text: "[etc]"}}}, nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return c
}

// 实机 2026-09-23:合并购买契约曾被 "premium contracts require a separate
// order" 整单拒绝;契约行现在逐行激活、可与普通商品同单。
func TestContractCartMerged(t *testing.T) {
	c := contractCartCatalog(t)
	p := &Pilot{Config: c}
	ledger := &contractCartLedger{state: json.RawMessage(`{}`)}
	cart := []protocol.CeraCartItem{
		{Product: 3500001, Quantity: 1},
		{Product: 3500009, Quantity: 2},
		{Product: 3500016, Quantity: 1},
	}
	_, applied, e := p.Purchase(context.Background(), ledger, 1, 1, "contract-cart-0001", cart)
	if e != nil || !applied {
		t.Fatal("merged contract cart", e)
	}
	if ledger.mixedCalls != 1 {
		t.Fatal("expected one mixed order", ledger.mixedCalls)
	}
	if len(ledger.mixedPremiums) != 3 {
		t.Fatal("expected three activations", ledger.mixedPremiums)
	}
	if act := ledger.mixedPremiums[0]; act.Type != 27 || act.DurationSecond != 7*86400 {
		t.Fatal("tactician activation", act)
	}
	if act := ledger.mixedPremiums[1]; act.Type != 22 || act.DurationSecond != 2*15*86400 {
		t.Fatal("conqueror activation scaled by quantity", act)
	}
	if act := ledger.mixedPremiums[2]; act.Type != 92 || act.DurationSecond != 7*86400 {
		t.Fatal("cube activation", act)
	}
	if len(ledger.mixedOrder.Lines) != 3 || ledger.mixedOrder.Lines[0].UnitPrice != 280 || ledger.mixedOrder.Lines[1].UnitPrice != 420 {
		t.Fatal("merged order pricing", ledger.mixedOrder.Lines)
	}
}

// A cart may mix contract lines with ordinary merchandise; only the ordinary
// lines reach the bag.
func TestMixedContractAndItemCart(t *testing.T) {
	c := contractCartCatalog(t)
	p := &Pilot{Config: c}
	ledger := &contractCartLedger{state: json.RawMessage(`{}`)}
	cart := []protocol.CeraCartItem{
		{Product: 3000999, Quantity: 2},
		{Product: 3500001, Quantity: 1},
	}
	_, applied, e := p.Purchase(context.Background(), ledger, 1, 1, "mixed-contract-0001", cart)
	if e != nil || !applied {
		t.Fatal("mixed cart", e)
	}
	if ledger.mixedCalls != 1 || len(ledger.mixedPremiums) != 1 {
		t.Fatal("mixed dispatch", ledger.mixedCalls, ledger.mixedPremiums)
	}
	bag, e := inventory.ReadBag(ledger.state)
	if e != nil || len(bag.Items) != 1 || bag.Items[0].Template != 15 || bag.Items[0].Amount != 2 {
		t.Fatal("ordinary line delivery", e, bag)
	}
}
