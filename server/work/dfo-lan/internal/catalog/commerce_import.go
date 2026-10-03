package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"log"
	"sort"
	"strings"
)

func sortedItemIDs(index ItemIndex) []uint32 {
	ids := make([]uint32, 0, len(index.Items))
	for id := range index.Items {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// ImportShopPrices uses the same native price projection as the exporter.
// Invalid prices stay absent so purchase and sale continue to be refused.
func ImportShopPrices(a *pvf.Archive, index ItemIndex) (*ShopPrices, error) {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("prices/PVF source mismatch")
	}
	c := &ShopPrices{Source: index.Source.Checksum, Items: map[uint32]ShopPrice{}}
	excluded := map[string]int{}
	for i, id := range sortedItemIDs(index) {
		if id == 0 {
			continue
		}
		script, err := ResolveScript(a, index.Items[id].Path)
		if err != nil {
			return nil, fmt.Errorf("price script %d: %w", id, err)
		}
		price, err := ShopPriceFromScript(script.Cells)
		if err != nil {
			excluded[err.Error()]++
		} else {
			c.Items[id] = price
		}
		if i%4096 == 4095 {
			a.ReleaseReadCaches()
		}
	}
	log.Printf("PVF price exclusions (sales refused): %v", excluded)
	return c, nil
}

// ItemMaterialCosts reads complete numeric pairs. Truncated or non-numeric
// costs must not silently become free purchases.
func ItemMaterialCosts(cells []pvf.Token) ([]ItemMaterialCost, error) {
	var values []pvf.Token
	inMaterial := false
	for _, t := range cells {
		if t.Type == 3 {
			inMaterial = t.Text == "[need material]"
			continue
		}
		if inMaterial {
			values = append(values, t)
		}
	}
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("incomplete material pair")
	}
	var costs []ItemMaterialCost
	for i := 0; i < len(values); i += 2 {
		// Template zero is the source's gold balance, not a missing item.
		if values[i].Type != 0 || values[i+1].Type != 0 || values[i].Value < 0 || values[i+1].Value <= 0 {
			return nil, fmt.Errorf("invalid material pair")
		}
		costs = append(costs, ItemMaterialCost{Template: uint32(values[i].Value), Count: uint32(values[i+1].Value)})
	}
	return costs, nil
}

func ImportItemMaterials(a *pvf.Archive, index ItemIndex) (*ItemMaterials, error) {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("materials/PVF source mismatch")
	}
	doc := itemMaterialsDoc{Version: 1, Source: index.Source.Checksum}
	for i, id := range sortedItemIDs(index) {
		item := index.Items[id]
		if item.Kind != "stackable" || !strings.HasSuffix(item.Path, ".stk") {
			continue
		}
		// Match the source export's exact binding; an absent path has no rule.
		if _, ok := a.FindFile(item.Path); !ok {
			continue
		}
		cells, err := a.Tokens(item.Path)
		if err != nil {
			return nil, fmt.Errorf("material script %d: %w", id, err)
		}
		costs, err := ItemMaterialCosts(cells)
		if err != nil {
			return nil, fmt.Errorf("material script %d: %w", id, err)
		}
		if len(costs) > 0 {
			doc.Items = append(doc.Items, ItemMaterialEntry{Template: id, Path: item.Path, Materials: costs})
		}
		if i%4096 == 4095 {
			a.ReleaseReadCaches()
		}
	}
	return newItemMaterials(doc)
}
