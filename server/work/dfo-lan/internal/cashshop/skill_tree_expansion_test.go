package cashshop

import (
	"context"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"testing"
)

type skillPurchaseRegressionLedger struct {
	packLedger
	skillCalls int
}

func (l *skillPurchaseRegressionLedger) PurchaseCashSkillTreeExpansion(_ context.Context, o CashOrder) (CashReceipt, bool, error) {
	l.skillCalls++
	l.order = o
	return CashReceipt{}, true, nil
}
func skillPurchaseRegressionPilot(t *testing.T) *Pilot {
	t.Helper()
	c := syntheticCatalog(t, "[etc]")
	skill := c.Entries[0]
	skill.Row = append([]pvf.Token(nil), skill.Row...)
	skill.Row[0].Value = int32(skillTreeSKU)
	skill.Row[1].Value = int32(skillTreeTemplate)
	skill.Row[2].Value = 1
	skill.Row[5].Value = 390
	skill.Section = "[item mod or ext]"
	c.Entries = append([]OrdinaryProduct{skill}, c.Entries...)
	return &Pilot{Config: c}
}
func TestSkillTreeExpansionDoesNotClaimOrdinaryOrUnknownSKU(t *testing.T) {
	for _, id := range []uint32{3999917, 3400489, 21000000} {
		p := skillPurchaseRegressionPilot(t)
		ledger := &skillPurchaseRegressionLedger{}
		_, _, handled, err := p.TryPurchaseSkillTreeExpansion(context.Background(), ledger, 1, 1, "skill-regression-0001", []protocol.CeraCartItem{{Product: id, Quantity: 1}})
		if handled || err != nil || ledger.skillCalls != 0 {
			t.Fatalf("ordinary SKU %d claimed by skill handler: handled=%v err=%v calls=%d", id, handled, err, ledger.skillCalls)
		}
	}
}
func TestSkillTreeExpansionOrdinaryPurchaseContinues(t *testing.T) {
	p := skillPurchaseRegressionPilot(t)
	ledger := &skillPurchaseRegressionLedger{packLedger: packLedger{state: json.RawMessage(`{}`)}}
	_, applied, err := p.Purchase(context.Background(), ledger, 1, 1, "ordinary-regression-0001", []protocol.CeraCartItem{{Product: 3999917, Quantity: 1}})
	if err != nil || !applied || ledger.skillCalls != 0 || len(ledger.order.Lines) != 1 || ledger.order.Lines[0].Product != 3999917 {
		t.Fatalf("ordinary purchase misrouted: applied=%v err=%v calls=%d order=%+v", applied, err, ledger.skillCalls, ledger.order)
	}
}
func TestSkillTreeExpansionStillUnlocksOwnSKU(t *testing.T) {
	p := skillPurchaseRegressionPilot(t)
	ledger := &skillPurchaseRegressionLedger{}
	_, applied, handled, err := p.TryPurchaseSkillTreeExpansion(context.Background(), ledger, 1, 1, "skill-regression-0001", []protocol.CeraCartItem{{Product: skillTreeSKU, Quantity: 1}})
	if err != nil || !applied || !handled || ledger.skillCalls != 1 || ledger.order.Lines[0].Template != skillTreeTemplate {
		t.Fatalf("skill ticket broke: applied=%v handled=%v err=%v calls=%d", applied, handled, err, ledger.skillCalls)
	}
}
func TestSkillTreeExpansionDoesNotFallbackToAnotherSKU(t *testing.T) {
	p := skillPurchaseRegressionPilot(t)
	p.Config.Entries[0].Row[0].Value = 3999900
	ledger := &skillPurchaseRegressionLedger{}
	_, _, handled, err := p.TryPurchaseSkillTreeExpansion(context.Background(), ledger, 1, 1, "skill-regression-0001", []protocol.CeraCartItem{{Product: skillTreeSKU, Quantity: 1}})
	if handled || err != nil || ledger.skillCalls != 0 {
		t.Fatalf("missing SKU used another product: handled=%v err=%v", handled, err)
	}
}
