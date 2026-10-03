// skinstorageimport exports every `[add skin storage]` stackable template with
// the skin it registers, so the server can resolve a consumed item to a cargo ID.
package skinstorageimport

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"log"
	"os"
)

func Run() {
	source := flag.String("source", "../client-build/Script.inner.pvf", "read-only source inner PVF")
	output := flag.String("output", "", "explicit output path for an audit export")
	flag.Parse()
	if *output == "" {
		log.Fatal("-output is required for an audit export")
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		log.Fatal(err)
	}
	data, err := catalog.ImportSkinStorage(a)
	if err != nil {
		log.Fatal(err)
	}
	b, err := json.Marshal(data)
	if err != nil {
		log.Fatal(err)
	}
	if err = os.WriteFile(*output, b, 0644); err != nil {
		log.Fatal(err)
	}
	damageFonts := 0
	for _, entry := range data.Entries {
		if entry.IsDamageFont() {
			damageFonts++
		}
	}
	log.Printf("skin storage templates=%d damage_fonts=%d missing_skins=%d source=%s output=%s",
		len(data.Entries), damageFonts, len(data.MissingSkins), data.Source.Checksum, *output)
}
