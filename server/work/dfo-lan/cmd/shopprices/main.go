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
	out := flag.String("out", "", "required diagnostic output; not a runtime configuration")
	flag.Parse()
	if *out == "" {
		log.Fatal("explicit -out is required for diagnostic export")
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1 << 30})
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()
	index, err := catalog.ImportItemIndex(a)
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
