// shieldaudit reads the source window and equipment scripts; it never writes
// character catalogs or client resources. Existing config_version is preserved.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	source := flag.String("pvf", "../client-build/Script.inner.pvf", "read-only inner PVF")
	characters := flag.String("characters", "configs/characters.generated.json", "existing profession catalog, never rewritten")
	full := flag.String("equipment-full", "configs/equipment-full", "full equipment catalog prefix")
	wear := flag.String("wear-rules", "configs/equipment-wear.current35.json", "source slot map")
	profession := flag.Uint("profession", 12, "knight profession")
	out := flag.String("export", "configs/equipment-knight-shield.full-candidate.json", "side-car output")
	flag.Parse()
	if err := run(*source, *characters, *full, *wear, *out, *profession); err != nil {
		log.Fatal(err)
	}
}

func run(source, characters, full, wear, out string, profession uint) error {
	jobs, err := catalog.LoadCharacters(characters)
	if err != nil {
		return err
	}
	if profession > 255 || jobs.Professions[byte(profession)].Job != "[knight]" {
		return fmt.Errorf("profession is not knight")
	}
	rules, err := inventory.LoadWearRules(wear, jobs.Source.Checksum)
	if err != nil {
		return err
	}
	if rules.Slots["[support weapon]"] != 24 {
		return fmt.Errorf("source support weapon slot is not 24")
	}
	equipment, err := inventory.OpenFullEquipmentCatalog(full, jobs.Source.Checksum)
	if err != nil {
		return err
	}
	defer equipment.Close()
	archive, err := pvf.LoadArchive(pvf.Options{Path: source, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		return err
	}
	if archive.Snapshot().Checksum != jobs.Source.Checksum {
		return fmt.Errorf("PVF checksum differs from existing config_version; do not rewrite character catalog")
	}
	window, err := catalog.ResolveScript(archive, "etc/character/knight/shieldwindownewdata.etc")
	if err != nil {
		return err
	}
	c := inventory.KnightShields{Source: archive.Snapshot(), Chain: window.Path, WindowSHA256: window.SHA256, WearSource: rules.Source, Profession: byte(profession)}
	rows, err := parseWindow(window.Cells)
	if err != nil {
		return err
	}
	for _, r := range rows {
		def, e := equipment.Definition(r.Item)
		if e != nil {
			return e
		}
		native, e := catalog.ResolveScript(archive, def.Path)
		if e != nil {
			return e
		}
		if native.SHA256 != def.SHA256 {
			return fmt.Errorf("shield %d source equipment differs", r.Item)
		}
		kind, sub := def.Fields["[equipment type]"], def.Fields["[sub type]"]
		if len(kind) == 0 || kind[0].Text != "[support weapon]" {
			return fmt.Errorf("shield %d is not support weapon", r.Item)
		}
		r.EquSHA256 = def.SHA256
		r.WearSlot = 24
		if len(sub) > 0 {
			r.SubType = sub[0].Value
		} else if len(kind) > 1 {
			r.SubType = kind[1].Value
		}
		c.Rows = append(c.Rows, r)
	}
	if err = c.Validate(jobs.Source.Checksum); err != nil {
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
	var rows []inventory.KnightShieldRow
	var current *inventory.KnightShieldRow
	tag := ""
	seen := map[string]bool{}
	for _, t := range cells {
		if t.Type == 3 {
			if t.Text == "[shield]" {
				if current != nil {
					return nil, fmt.Errorf("nested shield entry")
				}
				current = &inventory.KnightShieldRow{Position: uint16(len(rows) + 1)}
				seen = map[string]bool{}
			} else if t.Text == "[/shield]" && current != nil {
				rows = append(rows, *current)
				current = nil
			}
			tag = t.Text
			continue
		}
		if current == nil {
			continue
		}
		switch tag {
		case "[item index]", "[grow type]", "[get condition]", "[get shield level]", "[open quest index]", "[clear quest index]":
			if seen[tag] {
				return nil, fmt.Errorf("duplicate shield field %s", tag)
			}
			seen[tag] = true
			if tag == "[get condition]" {
				if t.Type != 6 || (t.Text != "level" && t.Text != "quest") {
					return nil, fmt.Errorf("unknown shield condition %q", t.Text)
				}
				current.Condition = t.Text
				continue
			}
			if t.Type != 0 || t.Value < 0 {
				return nil, fmt.Errorf("invalid shield field %s", tag)
			}
			switch tag {
			case "[item index]":
				current.Item = uint32(t.Value)
			case "[grow type]":
				current.GrowType = uint32(t.Value)
			case "[get shield level]":
				if t.Value > 255 {
					return nil, fmt.Errorf("shield level exceeds byte")
				}
				current.RequiredLevel = byte(t.Value)
			case "[open quest index]":
				current.OpenQuest = uint32(t.Value)
			case "[clear quest index]":
				current.ClearQuest = uint32(t.Value)
			}
		}
	}
	if current != nil {
		return nil, fmt.Errorf("unterminated shield entry")
	}
	return rows, nil
}
