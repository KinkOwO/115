package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"sort"
	"strings"
)

// ItemDisplay is source metadata for management searches, never a grant rule.
// NameKey is the actual script reference, which may name another ID or chn_*.
type ItemDisplay struct {
	ID                                                              uint32
	Kind, Path, SHA256, Name, NameKey, EquipmentType, StackableType string
	Grade, Rarity, MinimumLevel                                     int32
	StackLimit                                                      uint32
}

func parseDisplayTable(text string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "//") {
			continue
		}
		key, val, ok := strings.Cut(line, ">")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			continue
		}
		if _, exists := out[key]; !exists {
			out[key] = val
		}
	}
	return out
}

// Management metadata historically selects the first field occurrence. Later
// occurrences can belong to recipe/condition blocks (e.g. coat_10194 [grade]
// 1 followed by a nested 1,70 range). They must not invalidate or replace the
// displayed scalar. This projection does not interpret gameplay conditions.
func itemDisplayScalar(cells []pvf.Token, name string) (int32, bool) {
	for i, cell := range cells {
		if cell.Type == 3 && cell.Text == name {
			if i+1 < len(cells) && cells[i+1].Type == 0 {
				return cells[i+1].Value, true
			}
			return 0, false
		}
	}
	return 0, false
}

// VisitItemDisplay uses only native LIST identities and exact name references.
// Temporary script caches are bounded; consumers retain only displayed fields.
func VisitItemDisplay(a *pvf.Archive, index ItemIndex, visit func(ItemDisplay) error) error {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum || visit == nil {
		return fmt.Errorf("item display source mismatch or missing visitor")
	}
	if err := index.Validate(); err != nil {
		return err
	}
	tables, err := a.StringTables()
	if err != nil {
		return err
	}
	textTables := map[int]map[string]string{}
	ids := make([]uint32, 0, len(index.Items))
	for id := range index.Items {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i, id := range ids {
		item := index.Items[id]
		script, err := ResolveScript(a, item.Path)
		if err != nil {
			return fmt.Errorf("item display %d: %w", id, err)
		}
		d := ItemDisplay{ID: id, Kind: item.Kind, Path: script.Path, SHA256: script.SHA256, StackableType: item.StackableType, StackLimit: item.StackLimit}
		if n, ok := itemDisplayScalar(script.Cells, "[grade]"); ok {
			d.Grade = n
		}
		if n, ok := itemDisplayScalar(script.Cells, "[rarity]"); ok {
			d.Rarity = n
		}
		if n, ok := itemDisplayScalar(script.Cells, "[minimum level]"); ok {
			d.MinimumLevel = n
		}
		for _, cell := range sectionCells(script.Cells, "[equipment type]") {
			if cell.Text != "" {
				d.EquipmentType = cell.Text
				break
			}
		}
		ref, found, err := a.ItemNameRef(script.Path)
		if err != nil {
			return err
		}
		if found {
			d.Name = ref.Raw
			if ref.IsRef {
				d.NameKey = ref.Ref.Key
				if _, ok := textTables[ref.Ref.Table]; !ok {
					path, exists := tables[ref.Ref.Table]
					if !exists {
						return fmt.Errorf("item %d references unknown string table %d", id, ref.Ref.Table)
					}
					text, err := a.ReadText(path)
					if err != nil {
						return err
					}
					textTables[ref.Ref.Table] = parseDisplayTable(text)
				}
				if name, ok := textTables[ref.Ref.Table][ref.Ref.Key]; ok {
					d.Name = name
				}
			}
		}
		if err := visit(d); err != nil {
			return err
		}
		if i%512 == 511 {
			a.ReleaseReadCaches()
		}
	}
	a.ReleaseReadCaches()
	return nil
}
