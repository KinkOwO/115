// randomoptionimport exports the current client's magic-seal random option
// rules. It reads the seven authoritative tables under etc/randomoption/ from
// the inner PVF and writes one source-pinned JSON catalog the unseal handler
// consumes:
//
//	etc/randomoption/optionnumbering.etc          [option value ratio]
//	etc/randomoption/optionquantity.etc           [quantity ratio] + [option quantity]
//	etc/randomoption/optiongrouping.etc           [option group] pool weights
//	etc/randomoption/optiongroupselection.etc     [choose option group] per (rarity, group, count)
//	etc/randomoption/randomizedoptionoverall1.etc [option type] weights per (rarity, level)
//	etc/randomoption/randomizedoptionoverall2.etc [different weight] + [break seal cost]
//
// Equipment membership is NOT exported: the wireprobe runtime already opens
// the full equipment catalog and reads [random option], [rarity],
// [minimum level] and [item group name] from each definition directly.
package main

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

type valueRatio struct {
	Low, High float32
	Weight    int32
}

type optionQuantity struct {
	Rarity, OptionType, Level, Min, Max int32
}

type optionGroupChoice struct {
	Rarity   int32
	Group    string
	Quantity int32
	Groups   [3]int32
}

type optionTypeWeight struct {
	Rarity, Level int32
	Weights       [3]int32
}

type differentWeight struct {
	Rarity, OptionType, Min, Max int32
}

type breakSealCost struct {
	Rarity, Level, Part, Cost int32
}

type export struct {
	Model string `json:"model"`
	// Source.Checksum is the deployment baseline every runtime catalog
	// cross-checks against. When the inner archive was re-materialized with
	// identical content but a different container hash (see -baseline),
	// Source still carries the baseline while ExportedFromChecksum keeps the
	// archive this export actually read.
	Source               pvf.ArchiveSnapshot  `json:"source"`
	ExportedFromChecksum string               `json:"exported_from_checksum,omitempty"`
	TableSHA256          map[string]string    `json:"table_sha256"`
	ValueRatios          []valueRatio         `json:"value_ratios"`
	QuantityWeights      map[int32][][2]int32 `json:"quantity_weights"`
	OptionQuantities     []optionQuantity     `json:"option_quantities"`
	OptionGroups         map[int32][][2]int32 `json:"option_groups"`
	GroupChoices         []optionGroupChoice  `json:"group_choices"`
	OptionTypeWeights    []optionTypeWeight   `json:"option_type_weights"`
	DifferentWeights     []differentWeight    `json:"different_weights"`
	BreakSealCosts       []breakSealCost      `json:"break_seal_costs"`
}

// sectionCells splits a script into its bracketed sections. Section tags are
// type-3 tokens; everything between a tag and its matching [/tag] belongs to
// it. Sub-tags (e.g. [common] inside [quantity ratio]) start their own
// section; the parent resumes after the sub-tag closes.
func sections(cells []pvf.Token) map[string][][]pvf.Token {
	out := map[string][][]pvf.Token{}
	var stack []string
	var current []pvf.Token
	flush := func() {
		if len(stack) > 0 && len(current) > 0 {
			out[stack[len(stack)-1]] = append(out[stack[len(stack)-1]], current)
		}
		current = nil
	}
	for _, t := range cells {
		if t.Type == 3 && strings.HasPrefix(t.Text, "[") {
			name := strings.TrimSuffix(strings.TrimPrefix(t.Text, "["), "]")
			if strings.HasPrefix(name, "/") {
				flush()
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
				continue
			}
			flush()
			stack = append(stack, name)
			continue
		}
		current = append(current, t)
	}
	flush()
	return out
}

func intCell(t pvf.Token, context string) (int32, error) {
	if t.Type != 0 {
		return 0, fmt.Errorf("%s: expected int cell, got type %d (%q)", context, t.Type, t.Text)
	}
	return t.Value, nil
}

func floatCell(t pvf.Token, context string) (float32, error) {
	// Integral ratios (the closing 1.0) are stored as plain int cells.
	if t.Type == 0 {
		return float32(t.Value), nil
	}
	if t.Type != 2 {
		return 0, fmt.Errorf("%s: expected float cell, got type %d", context, t.Type)
	}
	return t.Number, nil
}

func stringCell(t pvf.Token, context string) (string, error) {
	if t.Type != 6 {
		return "", fmt.Errorf("%s: expected string cell, got type %d", context, t.Type)
	}
	return strings.Trim(t.Text, "`"), nil
}

// ints flattens a section's cells, which must all be plain integers.
func ints(rows map[string][][]pvf.Token, name string) ([]int32, error) {
	blocks, ok := rows[name]
	if !ok || len(blocks) != 1 {
		return nil, fmt.Errorf("section %s missing or repeated", name)
	}
	out := make([]int32, 0, len(blocks[0]))
	for i, t := range blocks[0] {
		v, e := intCell(t, fmt.Sprintf("%s cell %d", name, i))
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}

func main() {
	source := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only inner PVF")
	output := flag.String("output", "configs/randomoption.current37.json", "output catalog")
	baseline := flag.String("baseline", "", "deployment baseline checksum pinned into source.checksum; the true archive hash is kept in exported_from_checksum")
	flag.Parse()
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	read := func(p string) (catalog.ScriptRecord, map[string][][]pvf.Token) {
		s, err := catalog.ResolveScript(a, p)
		if err != nil {
			log.Fatalf("%s: %v", p, err)
		}
		return s, sections(s.Cells)
	}
	numbering, numberingSecs := read("etc/randomoption/optionnumbering.etc")
	quantity, quantitySecs := read("etc/randomoption/optionquantity.etc")
	grouping, groupingSecs := read("etc/randomoption/optiongrouping.etc")
	selection, selectionSecs := read("etc/randomoption/optiongroupselection.etc")
	overall1, overall1Secs := read("etc/randomoption/randomizedoptionoverall1.etc")
	overall2, overall2Secs := read("etc/randomoption/randomizedoptionoverall2.etc")

	snapshot := a.Snapshot()
	trueChecksum := snapshot.Checksum
	if *baseline != "" {
		if len(*baseline) != 64 {
			log.Fatal("baseline must be a sha256 hex string")
		}
		snapshot.Checksum = *baseline
	}
	out := export{
		Model:                "current115-randomoption-v1",
		Source:               snapshot,
		ExportedFromChecksum: trueChecksum,
		TableSHA256: map[string]string{
			"etc/randomoption/optionnumbering.etc":          numbering.SHA256,
			"etc/randomoption/optionquantity.etc":           quantity.SHA256,
			"etc/randomoption/optiongrouping.etc":           grouping.SHA256,
			"etc/randomoption/optiongroupselection.etc":     selection.SHA256,
			"etc/randomoption/randomizedoptionoverall1.etc": overall1.SHA256,
			"etc/randomoption/randomizedoptionoverall2.etc": overall2.SHA256,
		},
		QuantityWeights: map[int32][][2]int32{},
		OptionGroups:    map[int32][][2]int32{},
	}

	// [option value ratio]: float low, float high, int weight, repeated.
	{
		blocks := numberingSecs["option value ratio"]
		if len(blocks) != 1 {
			log.Fatalf("option value ratio blocks: %d", len(blocks))
		}
		cells := blocks[0]
		if len(cells)%3 != 0 || len(cells) == 0 {
			log.Fatalf("option value ratio cells %d not a multiple of 3", len(cells))
		}
		for i := 0; i < len(cells); i += 3 {
			low, err := floatCell(cells[i], "value ratio low")
			if err != nil {
				log.Fatal(err)
			}
			high, err := floatCell(cells[i+1], "value ratio high")
			if err != nil {
				log.Fatal(err)
			}
			weight, err := intCell(cells[i+2], "value ratio weight")
			if err != nil {
				log.Fatal(err)
			}
			out.ValueRatios = append(out.ValueRatios, valueRatio{low, high, weight})
		}
	}

	// [quantity ratio]: [common] = rarity 2, [unique] = rarity 3, each a flat
	// (quantity, weight) pair list.
	for name, rarity := range map[string]int32{"common": 2, "unique": 3} {
		blocks := quantitySecs[name]
		if len(blocks) != 1 {
			log.Fatalf("quantity ratio %s blocks: %d", name, len(blocks))
		}
		cells := blocks[0]
		if len(cells)%2 != 0 || len(cells) == 0 {
			log.Fatalf("quantity ratio %s cells %d not even", name, len(cells))
		}
		for i := 0; i < len(cells); i += 2 {
			q, err := intCell(cells[i], "quantity")
			if err != nil {
				log.Fatal(err)
			}
			w, err := intCell(cells[i+1], "quantity weight")
			if err != nil {
				log.Fatal(err)
			}
			out.QuantityWeights[rarity] = append(out.QuantityWeights[rarity], [2]int32{q, w})
		}
	}

	// [option quantity]: rarity, -1, option type, level, min, max.
	{
		values, err := ints(quantitySecs, "option quantity")
		if err != nil {
			log.Fatal(err)
		}
		if len(values)%6 != 0 {
			log.Fatalf("option quantity cells %d not a multiple of 6", len(values))
		}
		for i := 0; i < len(values); i += 6 {
			if values[i+1] != -1 {
				log.Fatalf("option quantity row %d fallback %d", i/6, values[i+1])
			}
			out.OptionQuantities = append(out.OptionQuantities, optionQuantity{values[i], values[i+2], values[i+3], values[i+4], values[i+5]})
		}
	}

	// [option group]: group id followed by (option id, weight) pairs; the
	// section repeats once per group.
	for _, block := range groupingSecs["option group"] {
		if len(block) < 3 || (len(block)-1)%2 != 0 {
			log.Fatalf("option group block of %d cells", len(block))
		}
		id, err := intCell(block[0], "option group id")
		if err != nil {
			log.Fatal(err)
		}
		if _, dup := out.OptionGroups[id]; dup {
			log.Fatalf("option group %d repeated", id)
		}
		for i := 1; i < len(block); i += 2 {
			option, err := intCell(block[i], "option id")
			if err != nil {
				log.Fatal(err)
			}
			weight, err := intCell(block[i+1], "option weight")
			if err != nil {
				log.Fatal(err)
			}
			out.OptionGroups[id] = append(out.OptionGroups[id], [2]int32{option, weight})
		}
	}

	// [choose option group]: rarity, -1, group name, quantity, 3, three group
	// ids, 0, 0, 1000 per row.
	{
		blocks := selectionSecs["choose option group"]
		if len(blocks) != 1 {
			log.Fatalf("choose option group blocks: %d", len(blocks))
		}
		cells := blocks[0]
		const width = 11
		if len(cells)%width != 0 {
			log.Fatalf("choose option group cells %d not a multiple of %d", len(cells), width)
		}
		for i := 0; i < len(cells); i += width {
			rarity, err := intCell(cells[i], "choice rarity")
			if err != nil {
				log.Fatal(err)
			}
			fallback, err := intCell(cells[i+1], "choice fallback")
			if err != nil {
				log.Fatal(err)
			}
			name, err := stringCell(cells[i+2], "choice group name")
			if err != nil {
				log.Fatal(err)
			}
			row := optionGroupChoice{Rarity: rarity, Group: name}
			rest := make([]int32, 0, 8)
			for j := 3; j < width; j++ {
				v, err := intCell(cells[i+j], "choice tail")
				if err != nil {
					log.Fatal(err)
				}
				rest = append(rest, v)
			}
			if fallback != -1 || rest[1] != 3 || rest[5] != 0 || rest[6] != 0 || rest[7] != 1000 {
				log.Fatalf("choose option group row %d shape drifted: fallback=%d rest=%v", i/width, fallback, rest)
			}
			row.Quantity = rest[0]
			row.Groups = [3]int32{rest[2], rest[3], rest[4]}
			out.GroupChoices = append(out.GroupChoices, row)
		}
	}

	// [option type]: rarity, -1, level, three type weights.
	{
		values, err := ints(overall1Secs, "option type")
		if err != nil {
			log.Fatal(err)
		}
		if len(values)%6 != 0 {
			log.Fatalf("option type cells %d not a multiple of 6", len(values))
		}
		for i := 0; i < len(values); i += 6 {
			if values[i+1] != -1 {
				log.Fatalf("option type row %d fallback %d", i/6, values[i+1])
			}
			out.OptionTypeWeights = append(out.OptionTypeWeights, optionTypeWeight{values[i], values[i+2], [3]int32{values[i+3], values[i+4], values[i+5]}})
		}
	}

	// [different weight]: rarity, option type, min, max.
	{
		values, err := ints(overall2Secs, "different weight")
		if err != nil {
			log.Fatal(err)
		}
		if len(values)%4 != 0 {
			log.Fatalf("different weight cells %d not a multiple of 4", len(values))
		}
		for i := 0; i < len(values); i += 4 {
			out.DifferentWeights = append(out.DifferentWeights, differentWeight{values[i], values[i+1], values[i+2], values[i+3]})
		}
	}

	// [break seal cost]: rarity, level, part category, gold.
	{
		values, err := ints(overall2Secs, "break seal cost")
		if err != nil {
			log.Fatal(err)
		}
		if len(values)%4 != 0 {
			log.Fatalf("break seal cost cells %d not a multiple of 4", len(values))
		}
		for i := 0; i < len(values); i += 4 {
			out.BreakSealCosts = append(out.BreakSealCosts, breakSealCost{values[i], values[i+1], values[i+2], values[i+3]})
		}
	}

	if len(out.ValueRatios) == 0 || len(out.QuantityWeights) != 2 || len(out.OptionQuantities) == 0 ||
		len(out.OptionGroups) == 0 || len(out.GroupChoices) == 0 || len(out.OptionTypeWeights) == 0 ||
		len(out.DifferentWeights) == 0 || len(out.BreakSealCosts) == 0 {
		log.Fatal("incomplete random option rules")
	}
	b, e := json.MarshalIndent(out, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	b = append(b, '\n')
	tmp := *output + ".tmp"
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		log.Fatal(e)
	}
	if e = os.Rename(tmp, *output); e != nil {
		log.Fatal(e)
	}
	sum := sha256.Sum256(b)
	log.Printf("DONE randomoption ratios=%d quantityWeights=%d quantities=%d groups=%d choices=%d typeWeights=%d different=%d costs=%d sha256=%x",
		len(out.ValueRatios), len(out.QuantityWeights), len(out.OptionQuantities), len(out.OptionGroups), len(out.GroupChoices), len(out.OptionTypeWeights), len(out.DifferentWeights), len(out.BreakSealCosts), sum)
}
