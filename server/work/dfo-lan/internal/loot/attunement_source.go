package loot

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"sort"
	"strings"
)

// ImportAttunementRewards binds tables by their native dungeon declaration:
// every etc/rewardboostinfo/**.ctp names its own dungeon ([dungeon index]),
// so the payable scope comes straight from the source. No external selection
// list is accepted — 单一内容真源铁律（server/AGENTS.md §0）。
func ImportAttunementRewards(a *pvf.Archive) (*AttunementRewards, error) {
	if a == nil {
		return nil, fmt.Errorf("attunement import requires PVF")
	}
	var paths []string
	seenPaths := map[string]bool{}
	if err := a.IterateFilesUnder("etc/rewardboostinfo", func(f pvf.File) error {
		path := strings.ToLower(strings.ReplaceAll(f.ArchivePath, "\\", "/"))
		if strings.HasPrefix(path, "etc/rewardboostinfo/") && strings.HasSuffix(path, ".ctp") && !seenPaths[path] {
			paths = append(paths, path)
			seenPaths[path] = true
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(paths)
	bindings := map[uint32]string{}
	for _, path := range paths {
		table, err := a.CTP(path)
		if err != nil {
			return nil, fmt.Errorf("reward table identity %s: %w", path, err)
		}
		var id uint32
		for _, r := range table.Records {
			if r.Parent >= 0 || r.Name != dungeonIndexColumn {
				continue
			}
			v, err := attunementSingleNumber(r)
			if err != nil {
				return nil, fmt.Errorf("reward dungeon identity %s: %w", path, err)
			}
			if id != 0 {
				return nil, fmt.Errorf("repeated reward dungeon declaration in %s", path)
			}
			id = v
		}
		if id == 0 {
			// A reward table that names no dungeon cannot be paid anywhere, so it
			// is refused rather than silently skipped.
			return nil, fmt.Errorf("reward table %s declares no dungeon index", path)
		}
		if old := bindings[id]; old != "" {
			return nil, fmt.Errorf("ambiguous reward table for dungeon %d: %s and %s", id, old, path)
		}
		bindings[id] = path
	}
	ids := make([]uint32, 0, len(bindings))
	for id := range bindings {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	doc := AttunementRewards{Model: AttunementModel, Archive: a.Snapshot()}
	for _, id := range ids {
		table, err := ReadAttunementTable(a, bindings[id])
		if err != nil {
			return nil, err
		}
		doc.Tables = append(doc.Tables, table)
	}
	return NewAttunementRewards(doc)
}

// Clone keeps native source tables immutable when a runtime consumer applies
// the independently configured rebalance policy.
func (a *AttunementRewards) Clone() (*AttunementRewards, error) {
	if a == nil {
		return nil, fmt.Errorf("nil attunement rewards")
	}
	doc := *a
	doc.Tables = append([]attunementDungeon(nil), a.Tables...)
	for i := range doc.Tables {
		t := &doc.Tables[i]
		t.Fixed = append([]attunementFixed(nil), t.Fixed...)
		for j := range t.Fixed {
			t.Fixed[j].Entries = append([]attunementEntry(nil), t.Fixed[j].Entries...)
		}
		t.Additional = append([]attunementAdditional(nil), t.Additional...)
		for j := range t.Additional {
			t.Additional[j].Entries = append([]attunementEntry(nil), t.Additional[j].Entries...)
		}
		t.Hidden = append([]attunementHidden(nil), t.Hidden...)
		for j := range t.Hidden {
			t.Hidden[j].Entries = append([]attunementHiddenEntry(nil), t.Hidden[j].Entries...)
		}
		t.Coupons = append([]attunementCoupon(nil), t.Coupons...)
		for j := range t.Coupons {
			t.Coupons[j].Entries = append([]attunementEntry(nil), t.Coupons[j].Entries...)
		}
	}
	return NewAttunementRewards(doc)
}
