package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
)

type LootItem struct {
	ID            uint32 `json:"id"`
	Kind          string `json:"kind"`
	Grade, Rarity int32
	Weight        uint32
	StackableType string `json:"stackable_type,omitempty"`
	StackLimit    uint32 `json:"stack_limit,omitempty"`
	Script        ScriptRecord
}
type LootCatalog struct {
	Source       pvf.ArchiveSnapshot     `json:"source"`
	MaximumGrade uint32                  `json:"maximum_grade"`
	Rules        map[string]ScriptRecord `json:"rules"`
	Items        map[uint32]LootItem     `json:"items"`
	IndexHashes  map[string]string       `json:"index_hashes"`
	Skipped      []string                `json:"skipped,omitempty"`
	Pending      []string                `json:"pending,omitempty"`
}

func lootInt(c []pvf.Token, name string) (int32, bool) {
	a := sectionCells(c, name)
	if len(a) != 1 || a[0].Type != 0 {
		return 0, false
	}
	return a[0].Value, true
}

// ImportLoot preserves current typed item definitions. Only item grades inside
// the explicitly requested import range are projected into this runtime pool.
func ImportLoot(a *pvf.Archive, maxGrade uint32) (LootCatalog, error) {
	c := LootCatalog{Source: a.Snapshot(), MaximumGrade: maxGrade, Rules: map[string]ScriptRecord{}, Items: map[uint32]LootItem{}, IndexHashes: map[string]string{}}
	if maxGrade == 0 || maxGrade > 200 {
		return c, fmt.Errorf("invalid loot import grade range")
	}
	for _, name := range []string{"etc/itemdropinfo_monseter.etc", "etc/itemdropinfo_common.etc", "etc/itemdropinfo_control.etc", "etc/dungeonbossdrop.etc"} {
		s, e := ResolveScript(a, name)
		if e != nil {
			return c, e
		}
		c.Rules[name] = s
	}
	refs := map[string]map[uint32]string{}
	for _, kind := range []string{"stackable", "equipment"} {
		index, e := ResolveScript(a, "list/"+kind+".lst")
		if e != nil {
			return c, e
		}
		c.IndexHashes[index.Path] = index.SHA256
		rows, e := ParseIndex(index.Cells)
		if e != nil {
			return c, e
		}
		refs[kind] = map[uint32]string{}
		for _, r := range rows {
			refs[kind][r.ID] = r.Path
		}
	}
	read := func(kind string, id uint32) (ScriptRecord, error) {
		p, ok := refs[kind][id]
		if !ok {
			return ScriptRecord{}, fmt.Errorf("item missing source list")
		}
		if !strings.HasPrefix(p, kind+"/") {
			p = path.Join(kind, p)
		}
		return ResolveScript(a, p)
	}
	var ids []uint32
	for id := range refs["stackable"] {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		p := strings.TrimPrefix(refs["stackable"][id], "stackable/")
		excluded := false
		for _, prefix := range []string{"cash/", "quest/", "recipe/", "temp/", "event/", "emblem/", "monstercard/"} {
			excluded = excluded || strings.HasPrefix(p, prefix)
		}
		if excluded {
			continue
		}
		s, e := read("stackable", id)
		if e != nil {
			c.Skipped = append(c.Skipped, fmt.Sprintf("stackable %d unreadable", id))
			continue
		}
		rate, ok := lootInt(s.Cells, "[creation rate]")
		if !ok || rate <= 0 {
			continue
		}
		grade, ok := lootInt(s.Cells, "[grade]")
		if !ok || grade <= 0 || uint32(grade) > maxGrade {
			continue
		}
		rarity, _ := lootInt(s.Cells, "[rarity]")
		if rarity < 0 || rarity > 6 {
			continue
		}
		typeCells := sectionCells(s.Cells, "[stackable type]")
		if len(typeCells) == 0 || typeCells[0].Type != 6 {
			continue
		}
		limit, _ := lootInt(s.Cells, "[stack limit]")
		if limit < 0 {
			continue
		}
		c.Items[id] = LootItem{ID: id, Kind: "stackable", Grade: grade, Rarity: rarity, Weight: 1, Script: s, StackableType: typeCells[0].Text, StackLimit: uint32(limit)}
	}
	dictionary, e := ResolveScript(a, "etc/itemdictionary/itemdictionary.etc")
	if e != nil {
		return c, e
	}
	c.IndexHashes[dictionary.Path] = dictionary.SHA256
	// Current dictionary rows contain 16..37 integers: category is column3,
	// whereas 90CN uses column4. Its generation column is not established.
	// Preserve provenance and refuse equipment projection instead of silently
	// treating flags as weights and awarding unrelated items.
	c.Pending = []string{"equipment dictionary generation semantics", "independent/world/explicit monster item pools"}
	return c, nil
}

func LoadLoot(path string) (LootCatalog, error) {
	var c LootCatalog
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(b, &c); e != nil {
		return c, e
	}
	if len(c.Source.Checksum) != 64 || c.MaximumGrade == 0 || len(c.Rules) != 4 {
		return c, fmt.Errorf("invalid loot catalog")
	}
	for id, item := range c.Items {
		if id == 0 || id != item.ID || item.Weight == 0 || item.Grade <= 0 || uint32(item.Grade) > c.MaximumGrade || len(item.Script.SHA256) != 64 {
			return c, fmt.Errorf("invalid loot item projection")
		}
	}
	return c, nil
}

// SupplementStackables supplements the catalog with stackable definitions
// from an items.index.json file for inventory/quickslot/consume operations,
// without overriding any existing monster drop items.
func (c *LootCatalog) SupplementStackables(path string) error {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var doc struct {
		Source struct {
			Checksum string `json:"checksum"`
		} `json:"source"`
		Items map[string]struct {
			ID            uint32 `json:"id"`
			Kind          string `json:"kind"`
			Path          string `json:"path"`
			StackableType string `json:"stackable_type"`
			StackLimit    uint32 `json:"stack_limit"`
		} `json:"items"`
	}
	dec := json.NewDecoder(f)
	if err := dec.Decode(&doc); err != nil {
		return err
	}
	if doc.Source.Checksum != "" && c.Source.Checksum != "" && doc.Source.Checksum != c.Source.Checksum {
		return fmt.Errorf("items index source mismatch: got %s want %s", doc.Source.Checksum, c.Source.Checksum)
	}

	if c.Items == nil {
		c.Items = make(map[uint32]LootItem, len(doc.Items))
	}
	for _, it := range doc.Items {
		if it.Kind != "stackable" || it.ID == 0 {
			continue
		}
		if _, exists := c.Items[it.ID]; !exists {
			c.Items[it.ID] = LootItem{
				ID:            it.ID,
				Kind:          "stackable",
				StackableType: it.StackableType,
				StackLimit:    it.StackLimit,
				Script:        ScriptRecord{Path: it.Path},
			}
		}
	}
	return nil
}
