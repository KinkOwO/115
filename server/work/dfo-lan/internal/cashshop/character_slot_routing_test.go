package cashshop

import (
	"context"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"testing"
)

type characterSlotRoutingLedger struct {
	packLedger
	slotCalls int
	slots     int
	slotOrder CashOrder
}

func (l *characterSlotRoutingLedger) PurchaseCashCharacterSlots(_ context.Context, order CashOrder, slots int) (CashReceipt, bool, error) {
	l.slotCalls++
	l.slots = slots
	l.slotOrder = order
	return CashReceipt{}, true, nil
}

func characterSlotRoutingPilot(t *testing.T, ordinaryID uint32, slotFirst bool) *Pilot {
	t.Helper()
	c := syntheticCatalog(t, "[etc]")
	ordinary := c.Entries[0]
	ordinary.Row[0].Value = int32(ordinaryID)
	slot := ordinary
	slot.Row = append([]pvf.Token(nil), ordinary.Row...)
	slot.Section = "[item mod or ext]"
	slot.Row[0].Value = characterSlotSKU
	slot.Row[1].Value = characterSlotTemplate
	slot.Row[2].Value = 1
	slot.Row[5].Value = 290
	if slotFirst {
		c.Entries = []OrdinaryProduct{slot, ordinary}
	} else {
		c.Entries = []OrdinaryProduct{ordinary, slot}
	}
	p, err := NewPilot(c, c.Source.Checksum, true)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCharacterSlotRoutingLeavesOrdinaryPurchaseUnchanged(t *testing.T) {
	for _, id := range []uint32{500001, 3999917} {
		for _, first := range []bool{true, false} {
			p := characterSlotRoutingPilot(t, id, first)
			ledger := &characterSlotRoutingLedger{packLedger: packLedger{state: json.RawMessage(`{}`)}}
			cart := []protocol.CeraCartItem{{Product: id, Quantity: 1}}
			_, applied, err := p.Purchase(context.Background(), ledger, 1, 1, "ordinary-routing-order", cart)
			if err != nil || !applied || ledger.slotCalls != 0 {
				t.Fatalf("ordinary product %d, slot first=%t: applied=%t slot calls=%d error=%v", id, first, applied, ledger.slotCalls, err)
			}
			bag, err := inventory.ReadBag(ledger.state)
			total, priceErr := ledger.order.Total()
			if err != nil || priceErr != nil || len(bag.Items) != 1 || bag.Items[0].Template != 999817 || bag.Items[0].Amount != 7 || len(ledger.order.Lines) != 1 || ledger.order.Lines[0].Product != id || total != 123 {
				t.Fatalf("ordinary delivery or source price changed: bag=%+v order=%+v error=%v", bag, ledger.order, err)
			}
		}
	}
}

func TestCharacterSlotRoutingOnlyHandlesItsOwnSKU(t *testing.T) {
	p := characterSlotRoutingPilot(t, 3999917, true)
	for _, id := range []uint32{3999917, 9999999} {
		_, _, handled, err := p.TryPurchaseCharacterSlotExpansion(context.Background(), nil, 1, 1, "routing-skip-order", []protocol.CeraCartItem{{Product: id, Quantity: 1}})
		if handled || err != nil {
			t.Fatalf("unrelated product %d intercepted: handled=%t error=%v", id, handled, err)
		}
	}
	ledger := &characterSlotRoutingLedger{}
	_, applied, err := p.Purchase(context.Background(), ledger, 1, 1, "slot-routing-order", []protocol.CeraCartItem{{Product: characterSlotSKU, Quantity: 1}})
	total, priceErr := ledger.slotOrder.Total()
	if err != nil || priceErr != nil || !applied || ledger.slotCalls != 1 || ledger.slots != 1 || total != 290 || len(ledger.slotOrder.Lines) != 1 || ledger.slotOrder.Lines[0].Template != characterSlotTemplate {
		t.Fatalf("slot expansion changed: applied=%t calls=%d slots=%d order=%+v error=%v", applied, ledger.slotCalls, ledger.slots, ledger.slotOrder, err)
	}
}

func TestCharacterSlotRoutingRejectsWrongTemplate(t *testing.T) {
	p := characterSlotRoutingPilot(t, 3999917, true)
	p.Config.Entries[0].Row[1].Value = 999817
	ledger := &characterSlotRoutingLedger{}
	_, _, handled, err := p.TryPurchaseCharacterSlotExpansion(context.Background(), ledger, 1, 1, "slot-wrong-template", []protocol.CeraCartItem{{Product: characterSlotSKU, Quantity: 1}})
	if handled || err != nil || ledger.slotCalls != 0 {
		t.Fatalf("wrong template reached expansion handler: handled=%t calls=%d error=%v", handled, ledger.slotCalls, err)
	}
}
