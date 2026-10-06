package character

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

func ImportFameRules(a *pvf.Archive, index catalog.ItemIndex) (*FameRules, error) {
	return importFameRules(a, index, true)
}

func importFameRules(a *pvf.Archive, index catalog.ItemIndex, withItems bool) (*FameRules, error) {
	if a == nil || index.Source.Checksum == "" || a.Snapshot().Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("fame PVF/index source mismatch")
	}
	r := FameRules{Version: 1, Source: a.Snapshot().Checksum, Sources: map[string]string{}, Tables: map[string]map[int]int64{}, Items: map[uint32]fameSourceValue{}, Sets: map[int][]fameThreshold{}, ItemPoints: map[uint32][]fameSetPoint{}, Groups: map[uint32][]int32{}}
	read := func(path string) ([]fameSourceSection, error) {
		script, err := catalog.ReadScript(a, path)
		if err != nil {
			return nil, err
		}
		r.Sources[path] = script.SHA256
		return fameSourceSections(script.Cells), nil
	}
	rows, err := read("etc/famevalueinfo.etc")
	if err != nil {
		return nil, err
	}
	for _, block := range fameSourceBlocks(rows, "[info]") {
		kind := fameSourceField(block, "[type]")
		if len(kind) != 1 || kind[0].Type != 6 {
			return nil, fmt.Errorf("invalid fame table type")
		}
		r.Tables[kind[0].Text], err = fameSourcePairs[int, int64](fameSourceField(block, "[table]"))
		if err != nil {
			return nil, err
		}
	}
	for _, block := range fameSourceBlocks(rows, "[correspond list]") {
		kind := fameSourceField(block, "[type]")
		if len(kind) != 1 || kind[0].Type != 6 {
			return nil, fmt.Errorf("invalid fame correspondence type")
		}
		if kind[0].Text != "separate upgrade" {
			continue
		}
		target := fameSourceField(block, "[correspond type]")
		if len(target) != 1 || target[0].Text != "normal upgrade" {
			return nil, fmt.Errorf("unverified fame upgrade correspondence")
		}
		r.Refine, err = fameSourcePairs[int, int](fameSourceField(block, "[formula]"))
		if err != nil {
			return nil, err
		}
		conditional := fameSourceField(block, "[level condition formula]")
		if len(conditional) < 2 || conditional[0].Type != 0 || conditional[0].Value != 115 || conditional[1].Type != 0 || conditional[1].Value != 1000 {
			return nil, fmt.Errorf("fame refine condition changed")
		}
		r.Refine115, err = fameSourcePairs[int, int](conditional[2:])
		if err != nil {
			return nil, err
		}
	}
	expanded, err := fameSourceNumbers(fameSourceField(rows, "[add value by expand set point]"))
	if err != nil {
		return nil, err
	}
	if len(expanded)%2 != 0 {
		return nil, fmt.Errorf("incomplete expanded fame thresholds")
	}
	positions := map[int]int{}
	for i := 0; i < len(expanded); i += 2 {
		point := int(expanded[i])
		if position, ok := positions[point]; ok {
			r.Expanded[position].Fame = expanded[i+1]
		} else {
			positions[point] = len(r.Expanded)
			r.Expanded = append(r.Expanded, fameThreshold{Point: point, Fame: expanded[i+1]})
		}
	}
	r.MemoryRestore, err = fameSourcePairs[byte, int64](fameSourceField(rows, "[memory restore fame value]"))
	if err != nil {
		return nil, err
	}
	r.MemoryActivate, err = fameSourcePairs[byte, int64](fameSourceField(rows, "[memory activate fame value]"))
	if err != nil {
		return nil, err
	}
	r.MemoryRuminations, err = fameSourcePairs[byte, int64](fameSourceField(rows, "[memory ruminations fame value]"))
	if err != nil {
		return nil, err
	}
	awakening, err := fameSourceTriples[uint32, byte](fameSourceField(rows, "[equipment awakening fame value]"))
	if err != nil {
		return nil, err
	}
	r.SoleQuality, err = fameSourceTriples[uint32, byte](fameSourceField(rows, "[sole equipment quality fame value]"))
	if err != nil {
		return nil, err
	}
	r.SolePenalty, err = fameSourceTriples[uint32, int](fameSourceField(rows, "[sole equipment penalty fame value]"))
	if err != nil {
		return nil, err
	}
	const pointPath = "etc/115lvability/setpointinfo.cos"
	raw, err := a.ReadRaw(pointPath)
	if err != nil {
		return nil, err
	}
	r.Sources[pointPath] = fmt.Sprintf("%x", sha256.Sum256(raw))
	text, err := a.ReadText(pointPath)
	if err != nil {
		return nil, err
	}
	groups, err := fameSourcePointGroups(text)
	if err != nil {
		return nil, err
	}
	grouping, err := read("etc/equipmentgrouping.etc")
	if err != nil {
		return nil, err
	}
	r.Awakening = map[uint32]map[byte]int64{}
	for _, block := range fameSourceBlocks(grouping, "[ability group]") {
		ids, err := fameSourceNumbers(fameSourceField(block, "[index]"))
		if err != nil || len(ids) != 1 || ids[0] < 0 || ids[0] > math.MaxUint32 {
			return nil, fmt.Errorf("invalid fame equipment group")
		}
		group := uint32(ids[0])
		templates, err := fameSourceNumbers(fameSourceField(block, "[list]"))
		if err != nil {
			return nil, err
		}
		for _, v := range templates {
			if v <= 0 || v > math.MaxUint32 {
				return nil, fmt.Errorf("invalid fame group template")
			}
			template := uint32(v)
			if !slices.Contains(r.Groups[template], int32(group)) {
				r.Groups[template] = append(r.Groups[template], int32(group))
			}
			for rank, fame := range awakening[group] {
				if r.Awakening[template] == nil {
					r.Awakening[template] = map[byte]int64{}
				}
				r.Awakening[template][rank] = max(r.Awakening[template][rank], fame)
			}
			for _, point := range groups[group] {
				entries := r.ItemPoints[template]
				if slices.Contains(entries, point) {
					continue
				}
				for _, old := range entries {
					if old.Set == point.Set && old.Awakening == point.Awakening {
						return nil, fmt.Errorf("ambiguous source set point for template %d", template)
					}
				}
				r.ItemPoints[template] = append(entries, point)
			}
		}
	}
	listingPath := "etc/115lvability/equipmentsetpointtable.lst"
	listing, err := catalog.ReadScript(a, listingPath)
	if err != nil {
		return nil, err
	}
	r.Sources[listingPath] = listing.SHA256
	entries, err := catalog.ParseIndex(listing.Cells)
	if err != nil {
		return nil, err
	}
	pointValues := map[uint32][]int64{}
	for _, entry := range entries {
		path := "etc/115lvability/" + entry.Path
		pointRows, err := read(path)
		if err != nil {
			return nil, err
		}
		pointValues[entry.ID], err = fameSourceNumbers(fameSourceField(pointRows, "[fame value]"))
		if err != nil {
			return nil, err
		}
	}
	sets, err := read("etc/equipmentpartset.etc")
	if err != nil {
		return nil, err
	}
	for _, set := range sets {
		if set.tag != "[equipment part set]" || len(set.cells) < 2 {
			continue
		}
		if set.cells[0].Type != 0 || set.cells[1].Type != 6 {
			return nil, fmt.Errorf("invalid fame part-set reference")
		}
		setID := int(set.cells[0].Value)
		path := "equipment/" + strings.ToLower(set.cells[1].Text)
		if _, exists := a.FindFile(path); !exists {
			continue
		} // The existing source projection omits stale part sets.
		script, err := catalog.ReadScript(a, path)
		if err != nil {
			return nil, err
		}
		setRows := fameSourceSections(script.Cells)
		var thresholds []fameThreshold
		for i, row := range setRows {
			if row.tag != "[piece set point]" {
				continue
			}
			v, err := fameSourceNumbers(row.cells)
			if err != nil || len(v) == 0 {
				return nil, fmt.Errorf("missing fame set-point threshold")
			}
			end := i + 1
			for end < len(setRows) && setRows[end].tag != "[/piece set point]" && setRows[end].tag != "[piece set point]" {
				end++
			}
			table, err := fameSourceNumbers(fameSourceField(setRows[i+1:end], "[set point table index]"))
			if err != nil {
				return nil, err
			}
			if len(table) > 0 && table[0] >= 0 && table[0] <= math.MaxUint32 && len(pointValues[uint32(table[0])]) > 0 {
				thresholds = append(thresholds, fameThreshold{Point: int(v[0]), Fame: pointValues[uint32(table[0])][0]})
			}
		}
		if len(thresholds) > 0 {
			slices.SortStableFunc(thresholds, func(a, b fameThreshold) int { return a.Point - b.Point })
			r.Sets[setID] = thresholds
			r.Sources[path] = script.SHA256
		}
	}
	if withItems {
		if err := readFameSourceItems(a, index, &r); err != nil {
			return nil, err
		}
	}
	return NewFameRules(r)
}

var (
	famePointBlock   = regexp.MustCompile(`(?s)\[info\](.*?)\[/info\]`)
	famePointInteger = regexp.MustCompile(`\[([^\]]+)\]\s*(-?\d+)`)
)

func fameSourcePointGroups(text string) (map[uint32][]fameSetPoint, error) {
	text = strings.SplitN(text, "[/set point]", 2)[0]
	groups := map[uint32][]fameSetPoint{}
	for _, block := range famePointBlock.FindAllStringSubmatch(text, -1) {
		fields := map[string]int64{}
		for _, pair := range famePointInteger.FindAllStringSubmatch(block[1], -1) {
			v, err := strconv.ParseInt(pair[2], 10, 64)
			if err != nil {
				return nil, err
			}
			fields[pair[1]] = v
		}
		for _, name := range []string{"group", "awakening", "part set index", "value"} {
			if _, ok := fields[name]; !ok {
				return nil, fmt.Errorf("missing fame point field %s", name)
			}
		}
		if fields["group"] < 0 || fields["group"] > math.MaxUint32 || fields["awakening"] < 0 || fields["awakening"] > math.MaxUint8 {
			return nil, fmt.Errorf("invalid fame point group or rank")
		}
		group := uint32(fields["group"])
		groups[group] = append(groups[group], fameSetPoint{Set: int(fields["part set index"]), Point: int(fields["value"]), Awakening: byte(fields["awakening"])})
	}
	return groups, nil
}

func readFameSourceItems(a *pvf.Archive, index catalog.ItemIndex, r *FameRules) error {
	type entry struct {
		item catalog.ItemIndexEntry
		file int
	}
	entries := []entry{}
	for _, item := range index.Items {
		if item.Kind != "stackable" {
			continue
		}
		f, ok := a.FindFile(item.Path)
		if !ok {
			return fmt.Errorf("missing fame source item %d", item.ID)
		}
		entries = append(entries, entry{item, f.Index})
	}
	slices.SortFunc(entries, func(a, b entry) int { return a.file - b.file })
	for i, row := range entries {
		if i%4096 == 4095 {
			a.ReleaseReadCaches()
		}
		cells, err := a.Tokens(row.item.Path)
		if err != nil {
			return err
		}
		fields := fameSourceSections(cells)
		direct := fameSourceField(fields, "[fame value]")
		table := fameSourceField(fields, "[fame table]")
		additional := fameSourceField(fields, "[add fame value]")
		if len(direct) == 0 && len(table) == 0 && len(additional) == 0 {
			continue
		}
		var item fameSourceValue
		if len(direct) > 0 {
			values, err := fameSourceNumbers(direct)
			if err != nil {
				return err
			}
			item.Value = values[0]
		}
		if len(additional) > 0 {
			values, err := fameSourceNumbers(additional)
			if err != nil {
				return err
			}
			item.Additional = values[0]
		}
		if len(table) > 0 {
			if len(table) != 2 || table[0].Type != 6 || table[1].Type != 0 {
				return fmt.Errorf("invalid item fame table %d", row.item.ID)
			}
			item.Table = table[0].Text
			item.Index = int(table[1].Value)
		}
		r.Items[row.item.ID] = item
		script, err := catalog.ReadScript(a, row.item.Path)
		if err != nil {
			return err
		}
		r.Sources[row.item.Path] = script.SHA256
	}
	return nil
}
