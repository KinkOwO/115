package cashshop

import (
	"encoding/json"
	"fmt"
)

// DeliverCreatureForManagement exposes the existing creature placement only.
// It is a pure bag transformation, not a purchase or a player-store operation.
func (p *Pilot) DeliverCreatureForManagement(raw json.RawMessage, template, amount uint32) (json.RawMessage, error) {
	if p == nil || amount == 0 || amount > 50 {
		return nil, fmt.Errorf("invalid creature management delivery")
	}
	h, err := p.resolveDeliveryType(template)
	if err != nil {
		return nil, err
	}
	if h.Kind != "[creature]" {
		return nil, fmt.Errorf("source template is not a creature")
	}
	return p.deliverAmount(raw, template, amount)
}
