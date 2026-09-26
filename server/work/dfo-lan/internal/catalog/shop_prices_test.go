package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShopPricesSourceSemantics(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fields  []any
		buy     int64
		sell    uint32
		invalid bool
	}{
		{"antidote 1150", []any{"[price]", 200}, 200, 40, false},
		{"explicit value", []any{"[price]", 200, "[value]", 100}, 200, 20, false},
		{"explicit zero", []any{"[price]", 200, "[value]", 0}, 200, 0, false},
		{"native unset value", []any{"[price]", 200, "[value]", -1}, 200, 40, false},
		{"teleport 2600014", nil, -1, 0, false},
		{"free", []any{"[price]", 0}, 0, 0, false},
		{"truncate", []any{"[price]", 204}, 204, 40, false},
		{"add fields", []any{"[price]", 200, "[add price]", 10, "[value]", 100, "[add value]", 5}, 210, 21, false},
		{"missing number", []any{"[price]"}, 0, 0, true},
		{"negative price", []any{"[price]", -2}, 0, 0, true},
		{"negative value", []any{"[value]", -2}, 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cells []pvf.Token
			for _, v := range tc.fields {
				switch x := v.(type) {
				case string:
					cells = append(cells, pvf.Token{Type: 3, Text: x})
				case int:
					cells = append(cells, pvf.Token{Type: 0, Value: int32(x)})
				}
			}
			p, e := ShopPriceFromScript(cells)
			if tc.invalid {
				if e == nil {
					t.Fatal("accepted invalid price")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if p.Sell != tc.sell || (tc.buy < 0) != (p.Buy == nil) || p.Buy != nil && int64(*p.Buy) != tc.buy {
				t.Fatalf("price %+v", p)
			}
		})
	}
}

func TestShopPricesRequireMatchingSource(t *testing.T) {
	source := strings.Repeat("a", 64)
	p := filepath.Join(t.TempDir(), "prices.json")
	b, _ := json.Marshal(ShopPrices{Source: source, Items: map[uint32]ShopPrice{1150: {Sell: 40}}})
	if e := os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := LoadShopPrices(p, source); e != nil {
		t.Fatal(e)
	}
	if _, e := LoadShopPrices(p, strings.Repeat("b", 64)); e == nil {
		t.Fatal("accepted stale PVF prices")
	}
}
