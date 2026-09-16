package inventory

import (
	"dfolan/internal/catalog"
	"encoding/json"
	"fmt"
)

type AwardReceipt struct {
	Template, Amount uint32
	Slots            []uint16
}
type Awarder struct {
	Catalog   catalog.LootCatalog
	Rules     BagRules
	Equipment *EquipmentCatalog
}

func (a *Awarder) Grant(raw json.RawMessage, id, amount uint32) (json.RawMessage, AwardReceipt, error) {
	r := AwardReceipt{Template: id, Amount: amount}
	if a == nil || a.Catalog.Source.Checksum != a.Rules.Source {
		return nil, r, fmt.Errorf("inventory award source missing")
	}
	b, e := ReadBag(raw)
	if e != nil {
		return nil, r, e
	}
	if id == 0 || a.Catalog.Items[id].Kind == "stackable" {
		var slot uint16
		b, slot, e = b.Add(a.Catalog, a.Rules, id, amount)
		r.Slots = []uint16{slot}
	} else {
		if a.Equipment == nil || a.Equipment.Source.Checksum != a.Rules.Source {
			return nil, r, fmt.Errorf("equipment award source missing")
		}
		b, r.Slots, e = b.AddEquipment(a.Equipment, a.Rules.EquipmentSlots, id, amount)
	}
	if e != nil {
		return nil, r, e
	}
	out, e := SaveBag(raw, b)
	return out, r, e
}
