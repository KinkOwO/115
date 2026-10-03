// shieldaudit reads the source window and equipment scripts; it never writes
// character catalogs or client resources. Existing config_version is preserved.
package shieldaudit

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
)

func Run() {
	source := flag.String("pvf", "../client-build/Script.inner.pvf", "read-only inner PVF")
	characters := flag.String("characters", "", "deprecated path; native professions are always read from PVF")
	full := flag.String("equipment-full", "", "deprecated prefix; source equipment is always read from PVF")
	wear := flag.String("wear-rules", "configs/equipment-wear.current35.json", "source slot map")
	profession := flag.Uint("profession", 12, "knight profession")
	out := flag.String("export", "", "explicit diagnostic output path (required)")
	flag.Parse()
	if err := run(*source, *characters, *full, *wear, *out, *profession); err != nil {
		log.Fatal(err)
	}
}

func run(source, characters, full, wear, out string, profession uint) error {
	if out == "" {
		return fmt.Errorf("explicit -export output is required")
	}
	if characters != "" || full != "" {
		return fmt.Errorf("legacy -characters and -equipment-full inputs are retired; use -pvf")
	}
	native, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: source})
	if err != nil {
		return err
	}
	defer native.Close()
	jobs, err := native.Characters("")
	if err != nil {
		return err
	}
	if profession > 255 || jobs.Professions[byte(profession)].Job != "[knight]" {
		return fmt.Errorf("profession is not knight")
	}
	rules, err := inventory.LoadWearRules(wear, native.Snapshot().Checksum)
	if err != nil {
		return err
	}
	index, err := native.ItemIndex("")
	if err != nil {
		return err
	}
	c, err := native.KnightShields(index, jobs, rules)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(out, append(data, '\n'), 0600); err != nil {
		return err
	}
	log.Printf("knight shield source=%s window=%s rows=%d output=%s", c.Source.Checksum, c.WindowSHA256, len(c.Rows), out)
	return nil
}

func parseWindow(cells []pvf.Token) ([]inventory.KnightShieldRow, error) {
	return inventory.ParseKnightShieldWindow(cells)
}
