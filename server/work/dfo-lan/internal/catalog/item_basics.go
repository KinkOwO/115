package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"log"
	"path"
	"sort"
	"strings"
)

type ItemBasicOptions struct{ Periods, Prices, Materials bool }
type ItemBasics struct {
	Index     ItemIndex
	Periods   *ItemPeriodCatalog
	Prices    *ShopPrices
	Materials *ItemMaterials
}

// ImportItemBasics visits native LIST bindings once. It keeps typed projections,
// never a cache of every script's tokens. Standalone importers remain available
// as independent parity oracles and for scopes that don't need a joint scan.
func ImportItemBasics(a *pvf.Archive, options ItemBasicOptions) (ItemBasics, error) {
	if a == nil {
		return ItemBasics{}, fmt.Errorf("missing item archive")
	}
	out := ItemBasics{Index: ItemIndex{Source: a.Snapshot(), Items: map[uint32]ItemIndexEntry{}, IndexHashes: map[string]string{}}}
	if options.Periods {
		out.Periods = &ItemPeriodCatalog{Schema: ItemPeriodCatalogSchema, Source: a.Snapshot()}
	}
	if options.Prices {
		out.Prices = &ShopPrices{Source: a.Snapshot().Checksum, Items: map[uint32]ShopPrice{}}
	}
	materials := itemMaterialsDoc{Version: 1, Source: a.Snapshot().Checksum}
	excluded := map[string]int{}
	for _, kind := range []string{"equipment", "stackable"} {
		list, err := ResolveScript(a, "list/"+kind+".lst")
		if err != nil {
			return ItemBasics{}, err
		}
		out.Index.IndexHashes[list.Path] = list.SHA256
		rows, err := ParseIndex(list.Cells)
		if err != nil {
			return ItemBasics{}, err
		}
		for i, row := range rows {
			if _, dup := out.Index.Items[row.ID]; dup {
				return ItemBasics{}, fmt.Errorf("item ID %d occurs in both source lists", row.ID)
			}
			p := row.Path
			if !strings.HasPrefix(p, kind+"/") {
				p = path.Join(kind, p)
			}
			entry := ItemIndexEntry{ID: row.ID, Path: p, Kind: kind}
			if kind == "equipment" && (strings.Contains(p, "/avatar/") || strings.Contains(p, "/at_avatar/")) {
				entry.Kind = "avatar"
			}
			if kind == "stackable" || options.Periods || options.Prices {
				cells, err := a.Tokens(ResolveScriptPath(a, p))
				if err != nil {
					return ItemBasics{}, fmt.Errorf("item %d: %w", row.ID, err)
				}
				if kind == "stackable" {
					types := sectionCells(cells, "[stackable type]")
					if len(types) > 0 {
						entry.StackableType = types[0].Text
					}
					if n, ok := lootInt(cells, "[stack limit]"); ok && n > 0 {
						entry.StackLimit = uint32(n)
					}
				}
				if options.Periods && hasItemPeriod(cells) {
					out.Periods.Templates = append(out.Periods.Templates, row.ID)
				}
				if options.Prices && row.ID != 0 {
					price, err := ShopPriceFromScript(cells)
					if err != nil {
						excluded[err.Error()]++
					} else {
						out.Prices.Items[row.ID] = price
					}
				}
				// Material import intentionally has no (r) fallback. Preserve that
				// exact-path boundary even though other projections resolve aliases.
				if options.Materials && kind == "stackable" && strings.HasSuffix(p, ".stk") {
					if _, exact := a.FindFile(p); exact {
						costs, err := ItemMaterialCosts(cells)
						if err != nil {
							return ItemBasics{}, fmt.Errorf("material script %d: %w", row.ID, err)
						}
						if len(costs) > 0 {
							materials.Items = append(materials.Items, ItemMaterialEntry{Template: row.ID, Path: p, Materials: costs})
						}
					}
				}
			}
			out.Index.Items[row.ID] = entry
			if i%4096 == 4095 {
				a.ReleaseReadCaches()
			}
		}
	}
	if err := out.Index.Validate(); err != nil {
		return ItemBasics{}, err
	}
	if out.Periods != nil {
		sort.Slice(out.Periods.Templates, func(i, j int) bool { return out.Periods.Templates[i] < out.Periods.Templates[j] })
	}
	if out.Prices != nil {
		log.Printf("PVF price exclusions (sales refused): %v", excluded)
	}
	if options.Materials {
		sort.Slice(materials.Items, func(i, j int) bool { return materials.Items[i].Template < materials.Items[j].Template })
		var err error
		out.Materials, err = newItemMaterials(materials)
		if err != nil {
			return ItemBasics{}, err
		}
	}
	return out, nil
}
