package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
)

// ItemIndex is shared by inventory supplementation and reward classification.
// IDs come exclusively from the two source lists, never from script filenames.
type ItemIndexEntry struct {
	ID            uint32 `json:"id"`
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	StackableType string `json:"stackable_type,omitempty"`
	StackLimit    uint32 `json:"stack_limit,omitempty"`
}

type ItemIndex struct {
	Source      pvf.ArchiveSnapshot       `json:"source"`
	Items       map[uint32]ItemIndexEntry `json:"items"`
	IndexHashes map[string]string         `json:"index_hashes"`
}

func LoadItemIndex(filename string) (ItemIndex, error) {
	var c ItemIndex
	f, err := os.Open(filename)
	if err != nil {
		return c, err
	}
	defer f.Close()
	if err = json.NewDecoder(f).Decode(&c); err != nil {
		return c, err
	}
	return c, c.Validate()
}

func (c ItemIndex) Validate() error {
	if len(c.Source.Checksum) != 64 || len(c.Items) == 0 {
		return fmt.Errorf("invalid item index source or empty index")
	}
	for id, r := range c.Items {
		if id != r.ID || r.Path == "" || (r.Kind != "equipment" && r.Kind != "avatar" && r.Kind != "stackable") {
			return fmt.Errorf("invalid item index entry %d", id)
		}
	}
	return nil
}

func ImportItemIndex(a *pvf.Archive) (ItemIndex, error) {
	c := ItemIndex{Source: a.Snapshot(), Items: map[uint32]ItemIndexEntry{}, IndexHashes: map[string]string{}}
	for _, kind := range []string{"equipment", "stackable"} {
		s, err := ResolveScript(a, "list/"+kind+".lst")
		if err != nil {
			return c, err
		}
		c.IndexHashes[s.Path] = s.SHA256
		rows, err := ParseIndex(s.Cells)
		if err != nil {
			return c, err
		}
		for i, row := range rows {
			if _, exists := c.Items[row.ID]; exists {
				return c, fmt.Errorf("item ID %d occurs in both source lists", row.ID)
			}
			p := row.Path
			if !strings.HasPrefix(p, kind+"/") {
				p = path.Join(kind, p)
			}
			r := ItemIndexEntry{ID: row.ID, Path: p, Kind: kind}
			if kind == "equipment" && (strings.Contains(p, "/avatar/") || strings.Contains(p, "/at_avatar/")) {
				r.Kind = "avatar"
			}
			if kind == "stackable" {
				script, err := ResolveScript(a, p)
				if err != nil {
					return c, fmt.Errorf("item %d: %w", row.ID, err)
				}
				types := sectionCells(script.Cells, "[stackable type]")
				if len(types) > 0 {
					r.StackableType = types[0].Text
				}
				if n, ok := lootInt(script.Cells, "[stack limit]"); ok && n > 0 {
					r.StackLimit = uint32(n)
				}
			}
			c.Items[row.ID] = r
			// Bulk import must not retain every expanded body group indefinitely.
			if i%4096 == 4095 {
				a.ReleaseReadCaches()
			}
		}
	}
	return c, c.Validate()
}

// SupplementItemIndex preserves drop-pool membership and existing item values.
// Extra stackables are usable inventory templates, not new monster drops.
func (c *LootCatalog) SupplementItemIndex(index ItemIndex) error {
	if index.Source.Checksum != c.Source.Checksum {
		return fmt.Errorf("items index source mismatch")
	}
	if c.Items == nil {
		c.Items = map[uint32]LootItem{}
	}
	for id, it := range index.Items {
		if it.Kind != "stackable" || id == 0 {
			continue
		}
		if _, exists := c.Items[id]; !exists {
			c.Items[id] = LootItem{ID: id, Kind: "stackable", StackableType: it.StackableType, StackLimit: it.StackLimit, Script: ScriptRecord{Path: it.Path}}
		}
	}
	return nil
}

// ItemIndexPaths supplies explicit ID bindings for PVF script consumers.
func (c ItemIndex) ItemIndexPaths(kind string) map[uint32]string {
	out := map[uint32]string{}
	for id, r := range c.Items {
		if kind == "" || r.Kind == kind || kind == "equipment" && r.Kind == "avatar" {
			out[id] = r.Path
		}
	}
	return out
}
