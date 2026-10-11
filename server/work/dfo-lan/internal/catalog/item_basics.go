package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"log"
	"path"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// ItemScript is valid only during its consumer call. Consumers retain their
// typed projections, not Raw or the entire Cells slice. Exact distinguishes a
// listed path from native (r) fallback; each domain keeps its own admission rule.
type ItemScript struct {
	Item  ItemIndexEntry
	Cells []pvf.Token
	Raw   []byte
	Exact bool
}

func (s ItemScript) SHA256() string { return fmt.Sprintf("%x", sha256.Sum256(s.Raw)) }

type ItemScriptConsumer func(ItemScript) error
type ItemBasicOptions struct {
	Periods, Prices, Materials, Skins, Boosters bool
	Consumers                                   []ItemScriptConsumer
}
type ItemBasics struct {
	Index       ItemIndex
	Periods     *ItemPeriodCatalog
	Prices      *ShopPrices
	Materials   *ItemMaterials
	Skins       *SkinStorageCatalog
	Boosters    map[uint32]BoosterDefinition
	ScriptsRead uint64
	// MissingScripts/缺失样本：LIST 行引用的源脚本在基线 PVF 里不存在（devpack
	// 基线差异，2026-10-05 实测 2,190 条 = 对方历史整合的 native_clone/creature/
	// title 等物品）。这些行保留索引、跳过脚本级投影，与「未知物品保留」的存档
	// 口径一致；样本随启动日志上报。
	MissingScripts       uint64
	MissingScriptSamples []string
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
	var skinImport *jointSkinImport
	var boosterImport *jointBoosterImport
	var err error
	if options.Skins {
		skinImport, err = newJointSkinImport(a)
		if err != nil {
			return ItemBasics{}, err
		}
	}
	if options.Boosters {
		boosterImport, err = newJointBoosterImport(a)
		if err != nil {
			return ItemBasics{}, err
		}
	}
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
		needRead := kind == "stackable" || options.Periods || options.Prices
		var reads []itemScriptRead
		if needRead {
			reads = prefetchItemScripts(a, rows, kind)
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
			if needRead {
				r := reads[i]
				if r.missingScript {
					out.MissingScripts++
					if len(out.MissingScriptSamples) < 8 {
						out.MissingScriptSamples = append(out.MissingScriptSamples, r.err.Error())
					}
					out.Index.Items[row.ID] = entry
					continue
				}
				if r.err != nil {
					return ItemBasics{}, r.err
				}
				cells, raw := r.cells, r.raw
				out.ScriptsRead++
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
				if kind == "stackable" {
					script := ItemScript{Item: entry, Cells: cells, Raw: raw, Exact: r.exact}
					if skinImport != nil {
						if err := skinImport.consume(script); err != nil {
							return ItemBasics{}, err
						}
					}
					if boosterImport != nil {
						if err := boosterImport.consume(script); err != nil {
							return ItemBasics{}, err
						}
					}
					for _, consume := range options.Consumers {
						if err := consume(script); err != nil {
							return ItemBasics{}, err
						}
					}
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
					if r.exact {
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
	if skinImport != nil {
		out.Skins = skinImport.finish()
	}
	if boosterImport != nil {
		out.Boosters = boosterImport.finish()
	}
	if out.MissingScripts > 0 {
		log.Printf("PVF item scripts missing (devpack baseline gap): %d rows skipped; samples: %v",
			out.MissingScripts, out.MissingScriptSamples)
	}
	return out, nil
}

// itemScriptRead is the prefetched native script for one LIST row.
type itemScriptRead struct {
	cells         []pvf.Token
	raw           []byte
	exact         bool
	missingScript bool
	err           error
}

// prefetchItemScripts reads and tokenizes LIST rows in bounded parallel batches.
// Results are stored per row so the caller can apply them in source order; the
// archive read path (FindFile/ReadRaw/TokensFromRaw) is safe for concurrent use.
// This is the bulk-import hot path, so the parallelism benefits both startup
// preparation and tests.
func prefetchItemScripts(a *pvf.Archive, rows []IndexEntry, kind string) []itemScriptRead {
	out := make([]itemScriptRead, len(rows))
	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	const batch = 1024
	for start := 0; start < len(rows); start += batch {
		end := start + batch
		if end > len(rows) {
			end = len(rows)
		}
		idxCh := make(chan int)
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range idxCh {
					out[i] = readItemScript(a, rows[i], kind)
				}
			}()
		}
		for i := start; i < end; i++ {
			idxCh <- i
		}
		close(idxCh)
		wg.Wait()
	}
	return out
}

func readItemScript(a *pvf.Archive, row IndexEntry, kind string) itemScriptRead {
	p := row.Path
	if !strings.HasPrefix(p, kind+"/") {
		p = path.Join(kind, p)
	}
	resolved := ResolveScriptPath(a, p)
	file, found := a.FindFile(resolved)
	if !found || file.DataType != 1 {
		return itemScriptRead{missingScript: true, err: fmt.Errorf("item %d: missing source script %s", row.ID, resolved)}
	}
	raw, err := a.ReadRaw(resolved)
	if err != nil {
		return itemScriptRead{err: fmt.Errorf("item %d: %w", row.ID, err)}
	}
	cells, err := a.TokensFromRaw(raw)
	if err != nil {
		return itemScriptRead{err: fmt.Errorf("item %d: %w", row.ID, err)}
	}
	_, exact := a.FindFile(p)
	return itemScriptRead{cells: cells, raw: raw, exact: exact}
}
