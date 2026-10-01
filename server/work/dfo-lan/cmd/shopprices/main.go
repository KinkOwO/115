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
	index, err := catalog.LoadItemIndex(*indexPath)
	if err != nil {
		log.Fatal(err)
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1 << 30})
	if err != nil {
		log.Fatal(err)
	}
	result, err := catalog.ImportShopPrices(a, index)
	if err != nil {
		log.Fatal(err)
	}
	b, err := json.Marshal(result)
	if err != nil {
		log.Fatal(err)
	}
	if err = os.WriteFile(*out, b, 0600); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Exported %d source prices.\n", len(result.Items))
}
