// shopprices exports compact NPC prices from the same PVF as the item index.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	source := flag.String("source", "", "inner PVF, read only")
	indexPath := flag.String("index", "configs/items.index.json", "item index")
	out := flag.String("out", "configs/shop-prices.json", "price catalog")
	flag.Parse()
	var index struct {
		Source pvf.ArchiveSnapshot `json:"source"`
		Items  map[uint32]struct {
			ID   uint32 `json:"id"`
			Path string `json:"path"`
		} `json:"items"`
	}
	b, err := os.ReadFile(*indexPath)
	if err != nil {
		log.Fatal(err)
	}
	if err = json.Unmarshal(b, &index); err != nil {
		log.Fatal(err)
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1 << 30})
	if err != nil {
		log.Fatal(err)
	}
	if a.Snapshot().Checksum != index.Source.Checksum {
		log.Fatal("PVF/index source mismatch")
	}
	result := catalog.ShopPrices{Source: index.Source.Checksum, Items: make(map[uint32]catalog.ShopPrice)}
	failed := 0
	failures := map[string]int{}
	for id, entry := range index.Items {
		if id == 0 {
			continue
		} // Gold is a balance, not a saleable bag item.
		if entry.ID != id || entry.Path == "" {
			log.Fatal("invalid indexed item")
		}
		script, e := catalog.ResolveScript(a, entry.Path)
		if e != nil {
			failed++
			failures["unreadable script"]++
			continue
		}
		price, e := catalog.ShopPriceFromScript(script.Cells)
		if e != nil {
			failed++
			failures[e.Error()]++
			continue
		}
		result.Items[id] = price
	}
	b, err = json.Marshal(result)
	if err != nil {
		log.Fatal(err)
	}
	if err = os.WriteFile(*out, b, 0600); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Exported %d source prices; %d unreadable/invalid entries excluded (sales refused).\n", len(result.Items), failed)
	fmt.Printf("Excluded reasons: %v\n", failures)
}
