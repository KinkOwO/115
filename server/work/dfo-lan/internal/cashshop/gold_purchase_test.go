package cashshop

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestGoldShopDebitAndDelivery(t *testing.T) {
	c := syntheticCatalog(t, "[waste]")
	c.Entries[0].Row[3].Value = 100
	c.Entries[0].Row[5].Value = 0
	p := &Pilot{Config: c}
	cart := []protocol.CeraCartItem{{Product: 3999917, Quantity: 2}}
	t.Run("price and saved balance", func(t *testing.T) {
		l := &packLedger{state: json.RawMessage(`{"keep":true,"inventory":{"version":"ordinary-bag-v1","gold":1000}}`)}
		_, _, err := p.Purchase(context.Background(), l, 1, 1, "gold-fixture-0001", cart)
		b, be := inventory.ReadBag(l.state)
		gold, ge := l.order.GoldTotal()
		if err != nil || be != nil || ge != nil || gold != 200 || b.Gold != 800 || len(b.Items) != 2 || !strings.Contains(string(l.state), "keep") || l.order.Lines[0].UnitPrice != 0 {
			t.Fatalf("bag=%+v order=%+v errors=%v/%v/%v", b, l.order, err, be, ge)
		}
	})
	t.Run("insufficient balance", func(t *testing.T) {
		original := json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","gold":199}}`)
		l := &packLedger{state: original}
		_, _, err := p.Purchase(context.Background(), l, 1, 1, "gold-fixture-0002", cart)
		if err == nil || !strings.Contains(err.Error(), "insufficient Gold") || string(l.state) != string(original) {
			t.Fatalf("state=%s err=%v", l.state, err)
		}
	})
	t.Run("full bag rolls back debit", func(t *testing.T) {
		b := inventory.Bag{Gold: 1000}
		for slot := uint16(65); slot <= 120; slot++ {
			b.Items = append(b.Items, inventory.BagItem{Slot: slot, Template: 14, Amount: 1})
		}
		raw, err := inventory.SaveBag(json.RawMessage(`{}`), b)
		if err != nil {
			t.Fatal(err)
		}
		l := &packLedger{state: raw}
		_, _, err = p.Purchase(context.Background(), l, 1, 1, "gold-fixture-0003", cart)
		if err == nil || string(l.state) != string(raw) {
			t.Fatalf("full bag changed: %v", err)
		}
	})
	t.Setenv("DFO_SHOP_OPEN_ALL", "1")
	for _, mutate := range []func(*PilotConfig){
		func(c *PilotConfig) { c.Entries[0].Row[5].Value = 1 },
		func(c *PilotConfig) { c.Entries[0].Row[3].Value = -1 },
		func(c *PilotConfig) { c.Entries[0].Row[4].Value = 1 },
		func(c *PilotConfig) { c.Entries[0].Row[6].Value = 1 },
		func(c *PilotConfig) { c.Entries[0].Row[7].Value = 1 },
	} {
		copy := syntheticCatalog(t, "[waste]")
		copy.Entries[0].Row[3].Value, copy.Entries[0].Row[5].Value = 100, 0
		mutate(&copy)
		if _, _, err := copy.classify(copy.Entries[0]); err == nil {
			t.Fatal(fmt.Sprintf("ambiguous or unsupported currency accepted: %+v", copy.Entries[0].Row))
		}
	}
}
