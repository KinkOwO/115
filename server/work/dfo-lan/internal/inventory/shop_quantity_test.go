package inventory

import (
	"math"
	"reflect"
	"testing"
)

func TestSellQuantityAndNoMutation(t *testing.T) {
	for _, count := range []uint32{1, 200, 1000} {
		b := Bag{Version: "ordinary-bag-v1", Gold: 1998, Items: []BagItem{{Slot: 124, Template: 1150, Amount: 1000}, {Slot: 125, Template: 1151, Amount: 5}}}
		next, template, gold, e := b.Sell(testShopBagRules(), 0, 124, count, 40)
		if e != nil || template != 1150 || gold != 40*count || next.Gold != 1998+40*count {
			t.Fatalf("sale count %d: %+v %d %d %v", count, next, template, gold, e)
		}
		if b.Items[0].Amount != 1000 || b.Gold != 1998 {
			t.Fatal("mutated input")
		}
		if count == 1000 {
			if len(next.Items) != 1 || next.Items[0].Slot != 125 {
				t.Fatal(next)
			}
		} else if next.Items[0].Amount != 1000-count {
			t.Fatal(next)
		}
	}
}

func TestSellRejectionsAreAtomic(t *testing.T) {
	base := Bag{Version: "ordinary-bag-v1", Gold: 1998, Items: []BagItem{{Slot: 124, Template: 1150, Amount: 1000}}, Equipment: []BagEquipment{{Slot: 15, Template: 5001}}}
	for _, tc := range []struct {
		name               string
		slot               uint16
		count, price, gold uint32
	}{
		{"zero", 124, 0, 40, 1998}, {"oversell", 124, 1001, 40, 1998},
		{"signed quantity", 124, math.MaxUint32, 40, 1998}, {"missing slot", 126, 1, 40, 1998},
		{"product overflow", 124, 1000, math.MaxUint32, 0}, {"balance overflow", 124, 1, 40, math.MaxUint32 - 39},
		{"equipment quantity", 15, 2, 40, 1998},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := base
			b.Gold = tc.gold
			next, _, _, e := b.Sell(testShopBagRules(), 0, tc.slot, tc.count, tc.price)
			if e == nil || !reflect.DeepEqual(next, b) || base.Items[0].Amount != 1000 {
				t.Fatalf("non-atomic refusal: %+v %v", next, e)
			}
		})
	}
	next, _, gold, e := base.Sell(testShopBagRules(), 0, 124, 1000, 0)
	if e != nil || gold != 0 || next.Gold != base.Gold || len(next.Items) != 0 {
		t.Fatalf("zero source value %+v %v", next, e)
	}
}
