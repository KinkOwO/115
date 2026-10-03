// questequipmentimport exports the native PVF equipment selection, including
// basic server-policy entries and equipment referenced by native quest rewards.
package questequipmentimport

import (
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
		return errors.New("-base is retired; equipment selection is derived from native PVF and policy")
	}
	if output == "" {
		return errors.New("-output is required")
	}
	return nil
}

func writeCatalog(c *inventory.EquipmentCatalog, output string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(output, data, 0600)
}

func Run() {
	sourcePath := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only source archive")
	policyPath := flag.String("policy", "configs/pvf-drop-policy.json", "existing basic-equipment and drop policy")
	base := flag.String("base", "", "retired; supplying a JSON seed is refused")
	output := flag.String("output", "", "required output path for the native equipment selection")
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
	quests, err := source.Quests("")
	if err != nil {
		log.Fatal(err)
	}
	selection, err := source.EquipmentSelection(index, quests, policy)
	if err != nil {
		log.Fatal(err)
	}
	if err := writeCatalog(selection, *output); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s from native PVF selection: %d rows (basic whitelist %d; quest rewards included)",
		*output, len(selection.Rows), len(policy.BasicEquipmentIDs))
}
