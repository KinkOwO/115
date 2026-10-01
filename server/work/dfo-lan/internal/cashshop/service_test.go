package cashshop

import (
	"context"
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

type fakeLedger struct {
	calls int
	order CashOrder
}

func (f *fakeLedger) PurchaseCash(_ context.Context, o CashOrder) (CashReceipt, bool, error) {
	f.calls++
	f.order = o
	return CashReceipt{}, true, nil
}
func TestPurchasePricesServerCatalog(t *testing.T) {
	ledger := &fakeLedger{}
	p := Product{ID: 3400489, Template: 590722921, Units: 1, Cera: 3180, Enabled: true}
	s := Service{Catalog: Catalog{Source: strings.Repeat("a", 64), Products: map[uint32]Product{p.ID: p}}, Ledger: ledger}
	raw, _ := hex.DecodeString("000001000029e3330001000000000000")
	cart, e := protocol.DecodeCeraCart(raw)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.Purchase(context.Background(), 1, 9, "server-order-000001", cart, time.Now()); e != nil {
		t.Fatal(e)
	}
	if ledger.calls != 1 || ledger.order.Lines[0].UnitPrice != 3180 || ledger.order.Lines[0].Template != 590722921 {
		t.Fatalf("%+v", ledger)
	}
	for _, change := range []func(*Product){func(p *Product) { p.Enabled = false }, func(p *Product) { p.Cera = 0 }, func(p *Product) { p.Ends = time.Now().Add(-time.Hour) }, func(p *Product) { p.Starts = time.Now().Add(time.Hour) }, func(p *Product) { p.Kind = 1 }} {
		bad := p
		change(&bad)
		s.Catalog.Products[p.ID] = bad
		if _, _, e = s.Purchase(context.Background(), 1, 9, "server-order-000002", cart, time.Now()); e == nil {
			t.Fatal("bad product reached ledger")
		}
	}
	if ledger.calls != 1 {
		t.Fatal("rejected requests touched money")
	}
}

func TestTropicalPackageSourceEvidence(t *testing.T) {
	// Keep the actual PVF row as evidence, not an automatically enabled catalog.
	b, e := os.ReadFile("testdata/product3400489.json")
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Section string `json:"section"`
		Cells   []struct {
			Type  int    `json:"type"`
			Value int64  `json:"value"`
			Text  string `json:"text"`
		} `json:"cells"`
	}
	if e = json.Unmarshal(b, &rows); e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		if r.Section == "[package related]" {
			if len(r.Cells) != 14 || r.Cells[0].Value != 3400489 || r.Cells[1].Value != 590722921 || r.Cells[2].Value != 1 || r.Cells[5].Value != 3180 {
				t.Fatal("PVF row changed")
			}
			return
		}
	}
	t.Fatal("missing source product")
}

func TestQuoteInvalidContext(t *testing.T) {
	var absent *Service
	if _, err := absent.Quote(1, 1, "server-order-0001", nil, time.Now()); err == nil {
		t.Fatal("nil service accepted")
	}
	ledger := &fakeLedger{}
	p := Product{ID: 3400489, Template: 590722921, Units: 1, Cera: 3180, Enabled: true}
	s := Service{Catalog: Catalog{Source: strings.Repeat("z", 64), Products: map[uint32]Product{p.ID: p}}, Ledger: ledger}
	cart := []protocol.CeraCartItem{{Product: p.ID, Quantity: 1}}
	if _, _, err := s.Purchase(context.Background(), 1, 1, "server-order-0001", cart, time.Now()); err == nil || ledger.calls != 0 {
		t.Fatal("invalid source reached ledger")
	}
}
