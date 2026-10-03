package cashshop

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"testing"
)

type packLedger struct {
	state json.RawMessage
	order CashOrder
}

func (l *packLedger) PurchaseCashToBag(_ context.Context, o CashOrder, fn func(json.RawMessage) (json.RawMessage, error)) (CashReceipt, bool, error) {
	state, e := fn(l.state)
	if e != nil {
		return CashReceipt{}, false, e
	}
	l.state = state
	l.order = o
	return CashReceipt{}, true, nil
}
func TestShopPilotPacksAndAtomicCapacity(t *testing.T) {
	p := nativePilot(t, false)
	products, e := p.products()
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range []struct{ id, units, price uint32 }{{3000674, 5, 490}, {3000675, 10, 900}, {3000676, 30, 2550}, {3003001, 3, 210}, {3003002, 5, 350}, {3003003, 10, 700}, {3000394, 10, 850}, {3000395, 5, 450}} {
		product := products[v.id]
		if product.Units != v.units || product.Cera != v.price {
			t.Fatalf("bundle source mismatch %+v", product)
		}
		l := &packLedger{state: json.RawMessage(`{}`)}
		_, _, err := p.Purchase(context.Background(), l, 1, 1, "bundle-test-0001", []protocol.CeraCartItem{{Product: v.id, Quantity: 1}})
		if err != nil {
			t.Fatal(err)
		}
		bag, err := inventory.ReadBag(l.state)
		if err != nil || len(bag.Items) != 1 || bag.Items[0].Amount != v.units {
			t.Fatal("bundle delivery", err)
		}
	}
	// Reproduce the user's existing41 boxes spread over41 slots. Buying50
	// must add to an existing stack, without deleting/reordering old rows.
	legacy := inventory.Bag{Version: "ordinary-bag-v1"}
	for slot := uint16(65); slot < 106; slot++ {
		legacy.Items = append(legacy.Items, inventory.BagItem{Slot: slot, Template: 15, Amount: 1})
	}
	legacyRaw, err := inventory.SaveBag(json.RawMessage(`{}`), legacy)
	if err != nil {
		t.Fatal(err)
	}
	legacyLedger := &packLedger{state: legacyRaw}
	_, _, err = p.Purchase(context.Background(), legacyLedger, 1, 1, "legacy-fifty-0001", []protocol.CeraCartItem{{Product: 3000121, Quantity: 1}})
	if err != nil {
		t.Fatal(err)
	}
	legacyAfter, err := inventory.ReadBag(legacyLedger.state)
	if err != nil || len(legacyAfter.Items) != 41 || legacyAfter.Items[0].Amount != 51 {
		t.Fatal("legacy fifty-pack failed", err)
	}
	for _, test := range []struct{ id, template, price uint32 }{{3000673, 14, 100}, {3000396, 21, 100}, {3003000, 590722509, 70}} {
		v := products[test.id]
		if v.Template != test.template || v.Cera != test.price {
			t.Fatalf("ordinary source mismatch %+v", v)
		}
		l := &packLedger{state: json.RawMessage(`{}`)}
		_, _, e = p.Purchase(context.Background(), l, 1, 1, "ordinary-test-0001", []protocol.CeraCartItem{{Product: test.id, Quantity: 2}})
		if e != nil {
			t.Fatal(e)
		}
		bag, e := inventory.ReadBag(l.state)
		if e != nil || len(bag.Items) != 1 || bag.Items[0].Template != test.template || bag.Items[0].Amount != 2 {
			t.Fatal("wrong ordinary delivery", e)
		}
		ack, e := protocol.CeraPurchaseOrdinarySuccess(test.id, 2)
		if e != nil || len(ack) != 49 {
			t.Fatal("ordinary response", e)
		}
	}
	for _, test := range []struct{ id, amount, price uint32 }{{3000118, 1, 45}, {3000119, 10, 400}, {3000120, 30, 1100}, {3000121, 50, 1700}} {
		v := products[test.id]
		if v.Units != test.amount || v.Cera != test.price {
			t.Fatalf("PVF product%v", v)
		}
		l := &packLedger{state: json.RawMessage(`{"level":1}`)}
		_, _, e = p.Purchase(context.Background(), l, 1, 1, "pack-order-test-0001", []protocol.CeraCartItem{{Product: test.id, Quantity: 1}})
		if e != nil {
			t.Fatal(e)
		}
		bag, e := inventory.ReadBag(l.state)
		if e != nil || len(bag.Items) != 1 || bag.Items[0].Amount != test.amount {
			t.Fatal("wrong pack amount", e)
		}
	}
	l := &packLedger{state: json.RawMessage(`{}`)}
	_, _, e = p.Purchase(context.Background(), l, 1, 1, "pack-order-test-0002", []protocol.CeraCartItem{{Product: 3000121, Quantity: 1}})
	if e != nil {
		t.Fatal(e)
	}
	full := inventory.Bag{Version: "ordinary-bag-v1"}
	for slot := uint16(65); slot <= 120; slot++ {
		full.Items = append(full.Items, inventory.BagItem{Slot: slot, Template: 15, Amount: 2147483647})
	}
	l.state, e = inventory.SaveBag(l.state, full)
	if e != nil {
		t.Fatal(e)
	}
	before := string(l.state)
	_, _, e = p.Purchase(context.Background(), l, 1, 1, "pack-order-test-0003", []protocol.CeraCartItem{{Product: 3000119, Quantity: 1}})
	if e == nil || string(l.state) != before {
		t.Fatal("partial ten-pack persisted")
	}
}

func TestShopPilotSourceAndDelivery(t *testing.T) {
	p := nativePilot(t, false)
	products, e := p.products()
	if e != nil || products[3000118].Cera != 45 || products[3000118].Template != 15 {
		t.Fatalf("unexpected current PVF product %+v %v", products[3000118], e)
	}
	raw := json.RawMessage(`{"level":55,"unrelated":true}`)
	for i := 0; i < 56; i++ {
		raw, e = p.deliverAmount(raw, 15, 1)
		if e != nil {
			t.Fatal(i, e)
		}
	}
	b, e := inventory.ReadBag(raw)
	if e != nil || len(b.Items) != 1 || b.Items[0].Amount != 56 {
		t.Fatal(e)
	}
	if _, e = p.deliverAmount(raw, 15, 1); e != nil {
		t.Fatal("existing stack should accept more", e)
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || string(fields["level"]) != "55" || string(fields["unrelated"]) != "true" {
		t.Fatal("noninventory fields changed")
	}
	for _, entry := range p.Config.Entries {
		if entry.Row[0].Value != 3000118 {
			continue
		}
		entry.Row = append(entry.Row[:0:0], entry.Row...)
		entry.Row[3].Value = 1
		if _, _, e = p.Config.classify(entry); e == nil {
			t.Fatal("gold price accepted")
		}
	}
}

func TestShopPilotCartAndSplitStacks(t *testing.T) {
	p := nativePilot(t, false)
	l := &packLedger{state: json.RawMessage(`{"level":55}`)}
	cart := []protocol.CeraCartItem{{Product: 3000121, Quantity: 21}, {Product: 3000674, Quantity: 2}, {Product: 3003001, Quantity: 1}}
	_, applied, err := p.Purchase(context.Background(), l, 1, 1, "mixed-cart-test-0001", cart)
	if err != nil || !applied || len(l.order.Lines) != 3 {
		t.Fatal("mixed cart", err)
	}
	b, err := inventory.ReadBag(l.state)
	if err != nil {
		t.Fatal(err)
	}
	amounts := map[uint32]uint32{}
	for _, row := range b.Items {
		amounts[row.Template] += row.Amount
		if row.Amount > 2147483647 {
			t.Fatal("stack overflow")
		}
	}
	if len(b.Items) != 3 || amounts[15] != 1050 || amounts[14] != 10 || amounts[590722509] != 3 {
		t.Fatal("wrong mixed delivery", b)
	}
	// First cart line fits an existing stack; second needs a new, unavailable slot.
	full := inventory.Bag{Version: "ordinary-bag-v1"}
	for slot := uint16(65); slot <= 120; slot++ {
		full.Items = append(full.Items, inventory.BagItem{Slot: slot, Template: 15, Amount: 2147483646})
	}
	l.state, err = inventory.SaveBag(json.RawMessage(`{}`), full)
	if err != nil {
		t.Fatal(err)
	}
	before := string(l.state)
	_, _, err = p.Purchase(context.Background(), l, 1, 1, "mixed-cart-test-0002", []protocol.CeraCartItem{{Product: 3000118, Quantity: 1}, {Product: 3000673, Quantity: 1}})
	if err == nil || string(l.state) != before {
		t.Fatal("partial cart persisted", err)
	}
	for _, invalid := range [][]protocol.CeraCartItem{nil, make([]protocol.CeraCartItem, 33), {{Product: 3000118, Quantity: 57}}, {{Product: 3000118, Quantity: 1}, {Product: 999999999, Kind: 4, Quantity: 1}}} {
		if _, _, err = p.Purchase(context.Background(), l, 1, 1, "mixed-cart-bad-0001", invalid); err == nil || string(l.state) != before {
			t.Fatal("invalid cart changed state")
		}
	}
	// Fill multiple partial stacks without requiring a free slot.
	raw, err := p.deliverAmount(l.state, 15, 56)
	if err != nil {
		t.Fatal(err)
	}
	b, err = inventory.ReadBag(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range b.Items {
		if row.Amount != 2147483647 {
			t.Fatal("partial stack not filled", row)
		}
	}
}

func TestShopPilotMaterialProductsAndCategories(t *testing.T) {
	p := nativePilot(t, false)
	products, err := p.products()
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []struct {
		id, template, price uint32
		material            bool
	}{
		{3001195, 50022396, 990, true}, {3001196, 50022395, 1290, true},
		{3002398, 10308836, 190, true}, {3002399, 10308357, 690, true},
		{3000393, 2660452, 50, false}, {3000669, 10000704, 190, false},
	} {
		product := products[v.id]
		if product.Template != v.template || product.Cera != v.price || product.Units != 1 {
			t.Fatal("source mismatch", v, product)
		}
		l := &packLedger{state: json.RawMessage(`{}`)}
		_, _, err = p.Purchase(context.Background(), l, 1, 1, "category-fixture-0001", []protocol.CeraCartItem{{Product: v.id, Quantity: 2}})
		if err != nil {
			t.Fatal(v, err)
		}
		b, err := inventory.ReadBag(l.state)
		if err != nil || len(b.Items) != 1 || b.Items[0].Template != v.template || b.Items[0].Amount != 2 {
			t.Fatal("delivery", v, b, err)
		}
		want := uint16(65)
		if v.material {
			want = 121
		}
		if b.Items[0].Slot != want {
			t.Fatal("wrong category", v, b)
		}
		if ack, err := protocol.CeraPurchaseOrdinarySuccess(v.id, 2); err != nil || len(ack) != 49 {
			t.Fatal("ACK", v, err)
		}
	}
	full := inventory.Bag{Version: "ordinary-bag-v1"}
	for slot := uint16(65); slot <= 120; slot++ {
		full.Items = append(full.Items, inventory.BagItem{Slot: slot, Template: 15, Amount: 1000})
	}
	raw, err := inventory.SaveBag(json.RawMessage(`{}`), full)
	if err != nil {
		t.Fatal(err)
	}
	l := &packLedger{state: raw}
	if _, _, err = p.Purchase(context.Background(), l, 1, 1, "material-independent-001", []protocol.CeraCartItem{{Product: 3001195, Quantity: 1}}); err != nil {
		t.Fatal("full consumables blocked material", err)
	}
	full.Items = nil
	for slot := uint16(121); slot <= 176; slot++ {
		full.Items = append(full.Items, inventory.BagItem{Slot: slot, Template: 50022396, Amount: 1000})
	}
	l.state, err = inventory.SaveBag(json.RawMessage(`{}`), full)
	if err != nil {
		t.Fatal(err)
	}
	before := string(l.state)
	_, _, err = p.Purchase(context.Background(), l, 1, 1, "material-full-cart-001", []protocol.CeraCartItem{{Product: 3000118, Quantity: 1}, {Product: 3001196, Quantity: 1}})
	if err == nil || string(l.state) != before {
		t.Fatal("full material left partial consumable delivery")
	}
	if _, _, err = p.Purchase(context.Background(), l, 1, 1, "consumable-independent-001", []protocol.CeraCartItem{{Product: 3000118, Quantity: 1}}); err != nil {
		t.Fatal("full material blocked consumable", err)
	}
}

func TestShopPilotPackageDelivery(t *testing.T) {
	t.Setenv("DFO_SHOP_OPEN_ALL", "1")
	p := nativePilot(t, true)
	// Verify package 3400489
	l := &packLedger{state: json.RawMessage(`{}`)}
	_, applied, err := p.Purchase(context.Background(), l, 1, 1, "pkg-test-order-0001", []protocol.CeraCartItem{{Product: 3400489, Quantity: 1}})
	if err != nil {
		t.Fatalf("purchase package failed: %v", err)
	}
	if !applied {
		t.Fatal("purchase not applied")
	}
	b, err := inventory.ReadBag(l.state)
	if err != nil {
		t.Fatalf("read bag failed: %v", err)
	}
	// The placeholder 590722921 must NOT be in the bag
	for _, it := range b.Items {
		if it.Template == 590722921 {
			t.Fatalf("placeholder template 590722921 found in bag, expected unpackaged boxes")
		}
	}
	// Expected 6 boxes
	expected := map[uint32]uint32{
		590722922: 1,
		590722923: 1,
		590722926: 1,
		590722927: 1,
		590722928: 1,
		590722929: 1,
	}
	if len(b.Items) != len(expected) {
		t.Fatalf("expected %d items in bag, got %d: %+v", len(expected), len(b.Items), b.Items)
	}
	for _, it := range b.Items {
		cnt, ok := expected[it.Template]
		if !ok {
			t.Fatalf("unexpected item in bag: %d", it.Template)
		}
		if it.Amount != cnt {
			t.Fatalf("item %d amount = %d, want %d", it.Template, it.Amount, cnt)
		}
		if it.Slot < 65 || it.Slot > 120 {
			t.Fatalf("item %d slot = %d out of consumable range [65, 120]", it.Template, it.Slot)
		}
		if it.ExpireTime != MaxExpireTime {
			t.Fatalf("item %d ExpireTime = %d, want MaxExpireTime %d", it.Template, it.ExpireTime, MaxExpireTime)
		}
	}
}

func TestShopPilotLifeTokenPurchase(t *testing.T) {
	p := nativePilot(t, false)

	// Create a full bag where all consumables and materials slots are occupied
	full := inventory.Bag{Version: "ordinary-bag-v1", Gold: 1000, Coin: 5}
	for slot := uint16(65); slot <= 120; slot++ {
		full.Items = append(full.Items, inventory.BagItem{Slot: slot, Template: 15, Amount: 1000})
	}
	for slot := uint16(121); slot <= 176; slot++ {
		full.Items = append(full.Items, inventory.BagItem{Slot: slot, Template: 50022396, Amount: 1000})
	}
	raw, err := inventory.SaveBag(json.RawMessage(`{}`), full)
	if err != nil {
		t.Fatal(err)
	}

	l := &packLedger{state: raw}

	// Purchase Life Token 10 EA (product 3000110)
	_, applied, err := p.Purchase(context.Background(), l, 1, 1, "life-token-order-0001", []protocol.CeraCartItem{
		{Product: 3000110, Quantity: 2}, // 2 * 10 = 20 coins
	})
	if err != nil || !applied {
		t.Fatalf("life token purchase failed: %v", err)
	}

	b, err := inventory.ReadBag(l.state)
	if err != nil {
		t.Fatal(err)
	}
	if b.Coin != 25 { // 5 initial + 20
		t.Fatalf("expected coin=25, got %d", b.Coin)
	}
	// The number of regular items must still be 112 (56 consumable + 56 material), no new bag item added!
	if len(b.Items) != 112 {
		t.Fatalf("expected 112 items in bag, got %d", len(b.Items))
	}

	// Purchase Life Token 1 EA (product 3000109)
	_, applied, err = p.Purchase(context.Background(), l, 1, 1, "life-token-order-0002", []protocol.CeraCartItem{
		{Product: 3000109, Quantity: 3}, // 3 * 1 = 3 coins
	})
	if err != nil || !applied {
		t.Fatalf("life token purchase 1 EA failed: %v", err)
	}

	b, err = inventory.ReadBag(l.state)
	if err != nil {
		t.Fatal(err)
	}
	if b.Coin != 28 { // 25 + 3
		t.Fatalf("expected coin=28, got %d", b.Coin)
	}

	// Verify rows contains slot 1
	rows := b.Rows()
	hasSlot1 := false
	for _, r := range rows {
		slot := uint16(r[0]) | uint16(r[1])<<8
		tpl := uint32(r[2]) | uint32(r[3])<<8 | uint32(r[4])<<16 | uint32(r[5])<<24
		cnt := uint32(r[6]) | uint32(r[7])<<8 | uint32(r[8])<<16 | uint32(r[9])<<24
		if slot == 1 {
			if tpl != 1 || cnt != 28 {
				t.Fatalf("slot 1 row mismatch: tpl=%d cnt=%d", tpl, cnt)
			}
			hasSlot1 = true
			break
		}
	}
	if !hasSlot1 {
		t.Fatal("rows missing slot 1 coin entry")
	}
}
