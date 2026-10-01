// randomoptionimport exports the same parser and validation used by PVF runtime.
package main

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"encoding/json"
	"flag"
	"log"
	"os"
)

func main() {
	source := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only inner PVF")
	output := flag.String("output", "configs/randomoption.current37.json", "output catalog")
	baseline := flag.String("baseline", "", "explicit deployment metadata checksum; actual archive hash remains exported_from_checksum")
	flag.Parse()
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		log.Fatal(err)
	}
	data, err := inventory.ImportRandomOptionData(a)
	if err != nil {
		log.Fatal(err)
	}
	snapshot := a.Snapshot()
	if *baseline != "" {
		if len(*baseline) != 64 {
			log.Fatal("baseline must be a sha256 hex string")
		}
		snapshot.Checksum = *baseline
	}
	out := struct {
		inventory.RandomOptionData
		Source               pvf.ArchiveSnapshot `json:"source"`
		ExportedFromChecksum string              `json:"exported_from_checksum,omitempty"`
	}{data, snapshot, a.Snapshot().Checksum}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	b = append(b, '\n')
	tmp := *output + ".tmp"
	if err = os.WriteFile(tmp, b, 0600); err != nil {
		log.Fatal(err)
	}
	if err = os.Rename(tmp, *output); err != nil {
		log.Fatal(err)
	}
	log.Printf("DONE randomoption ratios=%d groups=%d choices=%d costs=%d sha256=%x", len(data.ValueRatios), len(data.OptionGroups), len(data.GroupChoices), len(data.BreakSealCosts), sha256.Sum256(b))
}
