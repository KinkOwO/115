// itemshopimport exports the source item-shop tables (itemshop/**.shp).
//
// Each table prices its goods with [need material] <template> <count> — the
// Odyssey shop pays in silver/gold coins (10418036/10418035). The gateway never
// read this table: it charged a hard-coded 1 gold per purchase, so buying a
// 100-silver box was effectively free (live report 2026-09-23: "silver coins
// were not deducted"). This artifact is the missing data.
//
//	go run ./cmd/itemshopimport -source ../client-build/Script.inner.pvf \
//	    -output configs/itemshop-candidate.json
package main

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Material is one [need material] entry: pay Count of Template.
type Material struct {
	Template uint32 `json:"template"`
	Count    uint32 `json:"count"`
}

// Offer is one [item] block of a shop tab.
type Offer struct {
	Tab            uint32     `json:"tab"`
	Index          uint32     `json:"index"`
	Template       uint32     `json:"template"`
	PurchaseAmount uint32     `json:"purchase_amount,omitempty"`
	Materials      []Material `json:"materials,omitempty"`
}

// Shop is one itemshop/*.shp file, keyed by the shop id the client sends as
// NpcID in its CMD21 purchase request (the file name before the underscore).
type Shop struct {
	Npc    uint32  `json:"npc,omitempty"`
	Type   string  `json:"type,omitempty"`
	Path   string  `json:"path"`
	SHA256 string  `json:"sha256"`
	Offers []Offer `json:"offers"`
}

type document struct {
	Model  string            `json:"model"`
	Source pvf.ArchiveSnapshot `json:"source"`
	Shops  map[string]Shop   `json:"shops"`
}

const itemShopModel = "source-item-shops-v1"

// shopIDFromPath extracts the id from "itemshop/100001019_aradodyssey.shp".
func shopIDFromPath(path string) (uint32, bool) {
	base := path
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	base = strings.TrimSuffix(base, ".shp")
	if i := strings.Index(base, "_"); i >= 0 {
		base = base[:i]
	}
	n, err := strconv.ParseUint(base, 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
}

// parseShop walks one table's typed cells.
func parseShop(cells []pvf.Token) (uint32, string, []Offer) {
	var npc uint32
	var kind string
	var offers []Offer
	var cur *Offer
	tab := uint32(0)
	inList := false
	for i := 0; i < len(cells); i++ {
		c := cells[i]
		if c.Type == 3 {
			switch c.Text {
			case "[NPC]", "[npc]":
				if i+1 < len(cells) && cells[i+1].Type == 0 {
					npc = uint32(cells[i+1].Value)
					i++
				}
			case "[type]":
				if i+1 < len(cells) && cells[i+1].Type == 6 {
					kind = cells[i+1].Text
					i++
				}
			case "[tab]":
				tab++
				inList = false
			case "[sell item list]":
				inList = true
			case "[/sell item list]":
				inList = false
			case "[item]":
				if inList {
					cur = &Offer{Tab: tab}
					if i+1 < len(cells) && cells[i+1].Type == 0 {
						cur.Index = uint32(cells[i+1].Value)
						i++
					}
				}
			case "[index]":
				if cur != nil && i+1 < len(cells) && cells[i+1].Type == 0 {
					cur.Template = uint32(cells[i+1].Value)
					i++
				}
			case "[purchase amount]":
				if cur != nil && i+1 < len(cells) && cells[i+1].Type == 0 {
					cur.PurchaseAmount = uint32(cells[i+1].Value)
					i++
				}
			case "[need material]":
				for j := i + 1; j+1 < len(cells) && !(cells[j].Type == 3 && cells[j].Text == "[/need material]"); j += 2 {
					if cells[j].Type == 0 && cells[j+1].Type == 0 {
						cur.Materials = append(cur.Materials, Material{
							Template: uint32(cells[j].Value),
							Count:    uint32(cells[j+1].Value),
						})
					}
				}
			case "[/item]":
				if cur != nil {
					if cur.PurchaseAmount == 0 {
						cur.PurchaseAmount = 1
					}
					if cur.Template != 0 {
						offers = append(offers, *cur)
					}
					cur = nil
				}
			}
			continue
		}
	}
	return npc, kind, offers
}

func main() {
	source := flag.String("source", "../client-build/Script.inner.pvf", "source PVF, opened read-only")
	output := flag.String("output", "configs/itemshop-candidate.json", "exported catalog")
	debug := flag.String("debug", "", "comma separated shop ids: print their parsed offers and exit")
	flag.Parse()

	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1 << 30})
	if err != nil {
		log.Fatalf("load pvf: %v", err)
	}

	type entry struct {
		id   uint32
		path string
	}
	var entries []entry
	for _, f := range a.Files() {
		if !strings.HasSuffix(f.ArchivePath, ".shp") || !strings.Contains(f.ArchivePath, "itemshop/") {
			continue
		}
		id, ok := shopIDFromPath(f.ArchivePath)
		if !ok {
			continue
		}
		entries = append(entries, entry{id: id, path: f.ArchivePath})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })

	if *debug != "" {
		wanted := map[uint32]bool{}
		for _, part := range strings.Split(*debug, ",") {
			n, err := strconv.ParseUint(strings.TrimSpace(part), 10, 32)
			if err != nil {
				log.Fatalf("bad -debug id %q", part)
			}
			wanted[uint32(n)] = true
		}
		for _, e := range entries {
			if !wanted[e.id] {
				continue
			}
			cells, err := a.Tokens(e.path)
			if err != nil {
				log.Printf("%d %s: %v", e.id, e.path, err)
				continue
			}
			npc, kind, offers := parseShop(cells)
			fmt.Printf("%d %s npc=%d type=%s offers=%d\n", e.id, e.path, npc, kind, len(offers))
			for _, o := range offers {
				fmt.Printf("  tab=%d index=%d template=%d amount=%d pay=%v\n", o.Tab, o.Index, o.Template, o.PurchaseAmount, o.Materials)
			}
		}
		return
	}

	doc := document{Model: itemShopModel, Source: a.Snapshot(), Shops: map[string]Shop{}}
	start := time.Now()
	offers := 0
	for _, e := range entries {
		cells, err := a.Tokens(e.path)
		if err != nil {
			log.Printf("skip %s: %v", e.path, err)
			continue
		}
		npc, kind, list := parseShop(cells)
		if len(list) == 0 {
			continue
		}
		raw, err := a.ReadRaw(e.path)
		if err != nil {
			log.Printf("skip %s: raw read: %v", e.path, err)
			continue
		}
		sum := sha256.Sum256(raw)
		doc.Shops[strconv.FormatUint(uint64(e.id), 10)] = Shop{
			Npc:    npc,
			Type:   kind,
			Path:   e.path,
			SHA256: hex.EncodeToString(sum[:]),
			Offers: list,
		}
		offers += len(list)
	}

	out, err := json.MarshalIndent(doc, "", " ")
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(*output, out, 0o644); err != nil {
		log.Fatalf("write %s: %v", *output, err)
	}
	fmt.Printf("item shops: %d (%d offers; files %d) -> %s (%d bytes) in %v\n",
		len(doc.Shops), offers, len(entries), *output, len(out), time.Since(start))
}
