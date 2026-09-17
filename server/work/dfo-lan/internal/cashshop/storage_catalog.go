package cashshop

import (
	"dfolan/internal/catalog"
	"fmt"
)

// StorageCatalog supplements storage validation without expanding monster
// drops. It returns an independent item map and admits only enabled ordinary
// definitions whose delivery layout is already validated by the shop.
func (p *Pilot) StorageCatalog(base catalog.LootCatalog) (catalog.LootCatalog, error) {
	if p == nil || base.Source.Checksum != p.Config.Source.Checksum {
		return base, fmt.Errorf("storage catalog source mismatch")
	}
	if e := p.Config.validate(); e != nil {
		return base, e
	}
	out := base
	out.Items = make(map[uint32]catalog.LootItem, len(base.Items))
	for id, item := range base.Items {
		out.Items[id] = item
	}
	for _, entry := range p.Config.Entries {
		product, h, e := p.Config.classify(entry)
		if e != nil {
			continue
		}
		if old, ok := out.Items[product.Template]; ok && old.Script.SHA256 != entry.Item.SHA256 {
			return base, fmt.Errorf("conflicting storage item %d", product.Template)
		}
		out.Items[product.Template] = catalog.LootItem{ID: product.Template, Kind: "stackable", StackableType: h.Kind, StackLimit: h.Limit, Script: entry.Item}
	}
	return out, nil
}
