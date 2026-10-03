// equipmentwearimport exports the policy's basic equipment selection with
// current PVF eligibility fields. It never writes to the source archive.
package equipmentwearimport

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"os"
)

func validateArguments(base, output string) error {
	if base != "" {
		return errors.New("-base is retired; basic equipment IDs come from the native PVF drop policy")
	}
	if output == "" {
		return errors.New("-output is required")
	}
	return nil
}

func Run() {
	sourcePath := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only source archive")
	policyPath := flag.String("policy", "configs/pvf-drop-policy.json", "existing basic-equipment policy")
	base := flag.String("base", "", "retired; supplying a JSON seed is refused")
	output := flag.String("output", "", "required output path for the basic equipment selection")
	flag.Parse()
	if err := validateArguments(*base, *output); err != nil {
		log.Fatal(err)
	}

	source, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: *sourcePath, MaxBytes: gamedata.DefaultMaxBytes})
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()
	policy, err := inventory.ReadDropPolicy(*policyPath)
	if err != nil {
		log.Fatal(err)
	}
	index, err := source.ItemIndex("")
	if err != nil {
		log.Fatal(err)
	}
	// An empty quest catalog with the same source preserves the historical
	// 1536-entry basic whitelist boundary; quest reward expansion belongs to
	// questequipmentimport.
	emptyQuests := catalog.QuestCatalog{Source: source.Snapshot()}
	selection, err := source.EquipmentSelection(index, emptyQuests, policy)
	if err != nil {
		log.Fatal(err)
	}
	selection.OrdinaryPool = nil
	data, err := json.MarshalIndent(selection, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*output, data, 0600); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s from native PVF basic whitelist: %d rows", *output, len(selection.Rows))
}
