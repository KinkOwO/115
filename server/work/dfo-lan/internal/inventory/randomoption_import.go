package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strings"
)

type pvfOptionValueRatio struct {
	Low, High float32
	Weight    int32
}

type pvfOptionQuantity struct {
	Rarity, OptionType, Level, Min, Max int32
}

type pvfOptionGroupChoice struct {
	Rarity   int32
	Group    string
	Quantity int32
	Groups   [3]int32
}

type pvfOptionTypeWeight struct {
	Rarity, Level int32
	Weights       [3]int32
}

type pvfOptionDifferentWeight struct {
	Rarity, OptionType, Min, Max int32
}

type pvfOptionBreakSealCost struct {
	Rarity, Level, Part, Cost int32
}

// sectionCells splits a script into its bracketed optionSections. Section tags are
// type-3 tokens; everything between a tag and its matching [/tag] belongs to
// it. Sub-tags (e.g. [common] inside [quantity ratio]) start their own
// section; the parent resumes after the sub-tag closes.
func optionSections(cells []pvf.Token) map[string][][]pvf.Token {
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

func optionIntCell(t pvf.Token, context string) (int32, error) {
	if t.Type != 0 {
		return 0, fmt.Errorf("%s: expected int cell, got type %d (%q)", context, t.Type, t.Text)
	}
	return t.Value, nil
}

func optionFloatCell(t pvf.Token, context string) (float32, error) {
	// Integral ratios (the closing 1.0) are stored as plain int cells.
	if t.Type == 0 {
		return float32(t.Value), nil
	}
	if t.Type != 2 {
		return 0, fmt.Errorf("%s: expected float cell, got type %d", context, t.Type)
	}
	return t.Number, nil
}

func optionStringCell(t pvf.Token, context string) (string, error) {
	if t.Type != 6 {
		return "", fmt.Errorf("%s: expected string cell, got type %d", context, t.Type)
	}
	return strings.Trim(t.Text, "`"), nil
}

// optionInts flattens a section's cells, which must all be plain integers.
func optionInts(rows map[string][][]pvf.Token, name string) ([]int32, error) {
	blocks, ok := rows[name]
	if !ok || len(blocks) != 1 {
		return nil, fmt.Errorf("section %s missing or repeated", name)
	}
	out := make([]int32, 0, len(blocks[0]))
	for i, t := range blocks[0] {
		v, e := optionIntCell(t, fmt.Sprintf("%s cell %d", name, i))
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}

func ImportRandomOptionData(a *pvf.Archive) (RandomOptionData, error) {
	var out RandomOptionData
	if a == nil {
		return out, fmt.Errorf("missing random option archive")
	}
	out.Model = "current115-randomoption-v1"
	out.Source.Checksum = a.Snapshot().Checksum
	out.TableSHA256 = map[string]string{}
	out.QuantityWeights = map[int32][][2]int32{}
	out.OptionGroups = map[int32][][2]int32{}
	numbering, err := catalog.ResolveScript(a, "etc/randomoption/optionnumbering.etc")
	if err != nil {
		return out, err
	}
	out.TableSHA256["etc/randomoption/optionnumbering.etc"] = numbering.SHA256
	numberingSecs := optionSections(numbering.Cells)
	quantity, err := catalog.ResolveScript(a, "etc/randomoption/optionquantity.etc")
	if err != nil {
		return out, err
	}
	out.TableSHA256["etc/randomoption/optionquantity.etc"] = quantity.SHA256
	quantitySecs := optionSections(quantity.Cells)
	grouping, err := catalog.ResolveScript(a, "etc/randomoption/optiongrouping.etc")
	if err != nil {
		return out, err
	}
	out.TableSHA256["etc/randomoption/optiongrouping.etc"] = grouping.SHA256
	groupingSecs := optionSections(grouping.Cells)
	selection, err := catalog.ResolveScript(a, "etc/randomoption/optiongroupselection.etc")
	if err != nil {
		return out, err
	}
	out.TableSHA256["etc/randomoption/optiongroupselection.etc"] = selection.SHA256
	selectionSecs := optionSections(selection.Cells)
	overall1, err := catalog.ResolveScript(a, "etc/randomoption/randomizedoptionoverall1.etc")
	if err != nil {
		return out, err
	}
	out.TableSHA256["etc/randomoption/randomizedoptionoverall1.etc"] = overall1.SHA256
	overall1Secs := optionSections(overall1.Cells)
	overall2, err := catalog.ResolveScript(a, "etc/randomoption/randomizedoptionoverall2.etc")
	if err != nil {
		return out, err
	}
	out.TableSHA256["etc/randomoption/randomizedoptionoverall2.etc"] = overall2.SHA256
	overall2Secs := optionSections(overall2.Cells)
	// [option value ratio]: float low, float high, int weight, repeated.
	{
		blocks := numberingSecs["option value ratio"]
		if len(blocks) != 1 {
			return out, fmt.Errorf("option value ratio blocks: %d", len(blocks))
		}
		cells := blocks[0]
		if len(cells)%3 != 0 || len(cells) == 0 {
			return out, fmt.Errorf("option value ratio cells %d not a multiple of 3", len(cells))
		}
		for i := 0; i < len(cells); i += 3 {
			low, err := optionFloatCell(cells[i], "value ratio low")
			if err != nil {
				return out, err
			}
			high, err := optionFloatCell(cells[i+1], "value ratio high")
			if err != nil {
				return out, err
			}
			weight, err := optionIntCell(cells[i+2], "value ratio weight")
			if err != nil {
				return out, err
			}
			out.ValueRatios = append(out.ValueRatios, pvfOptionValueRatio{low, high, weight})
		}
	}

	// [quantity ratio]: [common] = rarity 2, [unique] = rarity 3, each a flat
	// (quantity, weight) pair list.
	for name, rarity := range map[string]int32{"common": 2, "unique": 3} {
		blocks := quantitySecs[name]
		if len(blocks) != 1 {
			return out, fmt.Errorf("quantity ratio %s blocks: %d", name, len(blocks))
		}
		cells := blocks[0]
		if len(cells)%2 != 0 || len(cells) == 0 {
			return out, fmt.Errorf("quantity ratio %s cells %d not even", name, len(cells))
		}
		for i := 0; i < len(cells); i += 2 {
			q, err := optionIntCell(cells[i], "quantity")
			if err != nil {
				return out, err
			}
			w, err := optionIntCell(cells[i+1], "quantity weight")
			if err != nil {
				return out, err
			}
			out.QuantityWeights[rarity] = append(out.QuantityWeights[rarity], [2]int32{q, w})
		}
	}

	// [option quantity]: rarity, -1, option type, level, min, max.
	{
		values, err := optionInts(quantitySecs, "option quantity")
		if err != nil {
			return out, err
		}
		if len(values)%6 != 0 {
			return out, fmt.Errorf("option quantity cells %d not a multiple of 6", len(values))
		}
		for i := 0; i < len(values); i += 6 {
			if values[i+1] != -1 {
				return out, fmt.Errorf("option quantity row %d fallback %d", i/6, values[i+1])
			}
			out.OptionQuantities = append(out.OptionQuantities, pvfOptionQuantity{values[i], values[i+2], values[i+3], values[i+4], values[i+5]})
		}
	}

	// [option group]: group id followed by (option id, weight) pairs; the
	// section repeats once per group.
	for _, block := range groupingSecs["option group"] {
		if len(block) < 3 || (len(block)-1)%2 != 0 {
			return out, fmt.Errorf("option group block of %d cells", len(block))
		}
		id, err := optionIntCell(block[0], "option group id")
		if err != nil {
			return out, err
		}
		if _, dup := out.OptionGroups[id]; dup {
			return out, fmt.Errorf("option group %d repeated", id)
		}
		for i := 1; i < len(block); i += 2 {
			option, err := optionIntCell(block[i], "option id")
			if err != nil {
				return out, err
			}
			weight, err := optionIntCell(block[i+1], "option weight")
			if err != nil {
				return out, err
			}
			out.OptionGroups[id] = append(out.OptionGroups[id], [2]int32{option, weight})
		}
	}

	// [choose option group]: rarity, -1, group name, quantity, 3, three group
	// ids, 0, 0, 1000 per row.
	{
		blocks := selectionSecs["choose option group"]
		if len(blocks) != 1 {
			return out, fmt.Errorf("choose option group blocks: %d", len(blocks))
		}
		cells := blocks[0]
		const width = 11
		if len(cells)%width != 0 {
			return out, fmt.Errorf("choose option group cells %d not a multiple of %d", len(cells), width)
		}
		for i := 0; i < len(cells); i += width {
			rarity, err := optionIntCell(cells[i], "choice rarity")
			if err != nil {
				return out, err
			}
			fallback, err := optionIntCell(cells[i+1], "choice fallback")
			if err != nil {
				return out, err
			}
			name, err := optionStringCell(cells[i+2], "choice group name")
			if err != nil {
				return out, err
			}
			row := pvfOptionGroupChoice{Rarity: rarity, Group: name}
			rest := make([]int32, 0, 8)
			for j := 3; j < width; j++ {
				v, err := optionIntCell(cells[i+j], "choice tail")
				if err != nil {
					return out, err
				}
				rest = append(rest, v)
			}
			if fallback != -1 || rest[1] != 3 || rest[5] != 0 || rest[6] != 0 || rest[7] != 1000 {
				return out, fmt.Errorf("choose option group row %d shape drifted: fallback=%d rest=%v", i/width, fallback, rest)
			}
			row.Quantity = rest[0]
			row.Groups = [3]int32{rest[2], rest[3], rest[4]}
			out.GroupChoices = append(out.GroupChoices, row)
		}
	}

	// [option type]: rarity, -1, level, three type weights.
	{
		values, err := optionInts(overall1Secs, "option type")
		if err != nil {
			return out, err
		}
		if len(values)%6 != 0 {
			return out, fmt.Errorf("option type cells %d not a multiple of 6", len(values))
		}
		for i := 0; i < len(values); i += 6 {
			if values[i+1] != -1 {
				return out, fmt.Errorf("option type row %d fallback %d", i/6, values[i+1])
			}
			out.OptionTypeWeights = append(out.OptionTypeWeights, pvfOptionTypeWeight{values[i], values[i+2], [3]int32{values[i+3], values[i+4], values[i+5]}})
		}
	}

	// [different weight]: rarity, option type, min, max.
	{
		values, err := optionInts(overall2Secs, "different weight")
		if err != nil {
			return out, err
		}
		if len(values)%4 != 0 {
			return out, fmt.Errorf("different weight cells %d not a multiple of 4", len(values))
		}
		for i := 0; i < len(values); i += 4 {
			out.DifferentWeights = append(out.DifferentWeights, pvfOptionDifferentWeight{values[i], values[i+1], values[i+2], values[i+3]})
		}
	}

	// [break seal cost]: rarity, level, part category, gold.
	{
		values, err := optionInts(overall2Secs, "break seal cost")
		if err != nil {
			return out, err
		}
		if len(values)%4 != 0 {
			return out, fmt.Errorf("break seal cost cells %d not a multiple of 4", len(values))
		}
		for i := 0; i < len(values); i += 4 {
			out.BreakSealCosts = append(out.BreakSealCosts, pvfOptionBreakSealCost{values[i], values[i+1], values[i+2], values[i+3]})
		}
	}

	if len(out.ValueRatios) == 0 || len(out.QuantityWeights) != 2 || len(out.OptionQuantities) == 0 ||
		len(out.OptionGroups) == 0 || len(out.GroupChoices) == 0 || len(out.OptionTypeWeights) == 0 ||
		len(out.DifferentWeights) == 0 || len(out.BreakSealCosts) == 0 {
		return out, fmt.Errorf("incomplete random option rules")
	}

	_, err = NewRandomOptionCatalog(out, out.Source.Checksum)
	return out, err
}
