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
	if e := p.Config.Validate(); e != nil {
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
		// 允许商城道具补充或覆盖仓库目录，不因 items.index.json 缺少 sha256 导致启动阻断
		out.Items[product.Template] = catalog.LootItem{ID: product.Template, Kind: "stackable", StackableType: h.Kind, StackLimit: h.Limit, Script: entry.Item}
	}
	return out, nil
}
