package storage

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestGoldCashOrderValidationAndLegacyJSON(t *testing.T) {
	o := cashFixture()
	raw, err := json.Marshal(o)
	expected := `{"key":"fixture-order-0001","account":1,"character":1,"source":"` + strings.Repeat("a", 64) + `","lines":[{"product":3400489,"template":590722921,"quantity":1,"units":1,"unit_price":3180}]}`
	if err != nil || string(raw) != expected {
		t.Fatalf("historical Cera order digest input changed: %s %v", raw, err)
	}
	o.Lines[0].UnitPrice, o.Lines[0].GoldUnitPrice = 0, 100
	if cera, gold, err := o.totals(); err != nil || cera != 0 || gold != 100 {
		t.Fatalf("gold quote: %d/%d %v", cera, gold, err)
	}
	o.Lines = append(o.Lines, cashFixture().Lines[0])
	if cera, gold, err := o.totals(); err != nil || cera != 3180 || gold != 100 {
		t.Fatalf("mixed quote: %d/%d %v", cera, gold, err)
	}
	o.Lines[0].UnitPrice = 1
	if _, _, err := o.totals(); err == nil {
		t.Fatal("dual currency line accepted")
	}
	o.Lines[0].UnitPrice, o.Lines[0].GoldUnitPrice = 0, 0
	if _, _, err := o.totals(); err == nil {
		t.Fatal("free line accepted")
	}
	o.Lines[0].GoldUnitPrice, o.Lines[0].Quantity = math.MaxUint32, 2
	if _, _, err := o.totals(); err == nil {
		t.Fatal("overflow accepted")
	}
}
