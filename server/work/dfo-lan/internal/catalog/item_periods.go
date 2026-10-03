package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
)

const ItemPeriodCatalogSchema = "item-period-tags-v1"

// ItemPeriodCatalog records only source templates that declare a time limit.
// The exported source checksum prevents using IDs from another PVF build.
type ItemPeriodCatalog struct {
	Schema    string              `json:"schema"`
	Source    pvf.ArchiveSnapshot `json:"source"`
	Templates []uint32            `json:"templates"`
}

func hasItemPeriod(cells []pvf.Token) bool {
	for _, cell := range cells {
		if cell.Type != 3 {
			continue
		}
		switch cell.Text {
		case "[expiration date]", "[usable period]", "[period]", "[usable datetime]":
			return true
		}
	}
	return false
}

// ImportItemPeriods scans every stackable and equipment template in the
// source lists. It never infers a period from item names or saved instances.
func ImportItemPeriods(a *pvf.Archive) (ItemPeriodCatalog, error) {
	result := ItemPeriodCatalog{Schema: ItemPeriodCatalogSchema, Source: a.Snapshot()}
	seen := make(map[uint32]bool)
	for _, kind := range []string{"stackable", "equipment"} {
		index, err := ResolveScript(a, "list/"+kind+".lst")
		if err != nil {
			return ItemPeriodCatalog{}, err
		}
		rows, err := ParseIndex(index.Cells)
		if err != nil {
			return ItemPeriodCatalog{}, err
		}
		for i, row := range rows {
			if i%4096 == 4095 {
				a.ReleaseReadCaches()
			}
			name := row.Path
			if !strings.HasPrefix(name, kind+"/") {
				name = path.Join(kind, name)
			}
			cells, err := a.Tokens(name)
			if errors.Is(err, pvf.ErrFileNotFound) {
				name = path.Join(path.Dir(name), "(r)"+path.Base(name))
				cells, err = a.Tokens(name)
			}
			if err != nil {
				return ItemPeriodCatalog{}, fmt.Errorf("template %d (%s): %w", row.ID, name, err)
			}
			if hasItemPeriod(cells) && !seen[row.ID] {
				result.Templates = append(result.Templates, row.ID)
				seen[row.ID] = true
			}
		}
	}
	sort.Slice(result.Templates, func(i, j int) bool { return result.Templates[i] < result.Templates[j] })
	return result, nil
}

func LoadItemPeriods(file, expectedSource string) ([]uint32, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var data ItemPeriodCatalog
	if err = json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	if data.Schema != ItemPeriodCatalogSchema || len(expectedSource) != 64 || data.Source.Checksum != expectedSource || len(data.Templates) == 0 {
		return nil, fmt.Errorf("item period catalog source/schema mismatch or empty")
	}
	var previous uint32
	for _, template := range data.Templates {
		if template == 0 || template <= previous {
			return nil, fmt.Errorf("item period catalog IDs must be positive and increasing")
		}
		previous = template
	}
	return data.Templates, nil
}
