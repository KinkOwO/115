package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// ShopPrice preserves the distinction between an absent purchase price and an
// explicit zero. Ordinary NPC sales use one fifth of [value], falling back
// to [price]. Native stackable parser 147110790 initializes value to -1;
// 14500e480 / 14717b8f0 apply the ordinary sale rate (200 per thousand).
type ShopPrice struct {
	Buy  *uint32 `json:"buy,omitempty"`
	Sell uint32  `json:"sell"`
}

type ShopPrices struct {
	Source string               `json:"source_pvf_sha256"`
	Items  map[uint32]ShopPrice `json:"items"`
}

func ShopPriceFromScript(cells []pvf.Token) (ShopPrice, error) {
	price, value := int64(0), int64(-1)
	hasPrice := false
	for i, cell := range cells {
		if cell.Type != 3 {
			continue
		}
		switch cell.Text {
		case "[price]", "[value]", "[add price]", "[add value]":
			if i+1 >= len(cells) || cells[i+1].Type != 0 {
				return ShopPrice{}, fmt.Errorf("invalid item %s", cell.Text)
			}
			n := int64(cells[i+1].Value)
			switch cell.Text {
			case "[price]":
				price, hasPrice = n, true
			case "[value]":
				value = n
			case "[add price]":
				price, hasPrice = price+n, true
			case "[add value]":
				value += n
			}
		}
	}
	if price < 0 || price > math.MaxInt32 || value < -1 || value > math.MaxInt32 {
		return ShopPrice{}, fmt.Errorf("invalid item price/value range")
	}
	result := ShopPrice{}
	if hasPrice {
		n := uint32(price)
		result.Buy = &n
	}
	if value == -1 {
		value = price
	}
	result.Sell = uint32(value / 5)
	return result, nil
}

func LoadShopPrices(path, source string) (*ShopPrices, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var c ShopPrices
	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return nil, err
	}
	if len(c.Source) != 64 || c.Source != source || len(c.Items) == 0 {
		return nil, fmt.Errorf("shop prices source mismatch or empty catalog")
	}
	if _, ok := c.Items[0]; ok {
		return nil, fmt.Errorf("invalid shop price template zero")
	}
	return &c, nil
}
