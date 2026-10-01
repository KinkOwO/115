package adventure

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"slices"
)

// ImportRecommendedRules follows the existing event/worldmap projection.
// Worldmap entries are dungeon/quest pairs; the quest is never another dungeon.
func ImportRecommendedRules(a *pvf.Archive) (*RecommendedRules, error) {
	if a == nil {
		return nil, fmt.Errorf("recommended dungeons require PVF")
	}
	const sourcePath = "event/conditioneventchkdungeon.evt"
	script, err := catalog.ReadScript(a, sourcePath)
	if err != nil {
		return nil, err
	}
	listing, err := catalog.ReadScript(a, "list/worldmap.lst")
	if err != nil {
		return nil, err
	}
	entries, err := catalog.ParseIndex(listing.Cells)
	if err != nil {
		return nil, err
	}
	paths := map[uint32]string{}
	for _, row := range entries {
		paths[row.ID] = row.Path
	}
	specific, err := recommendedRanges(script.Cells, "[specific dungeon event level]")
	if err != nil {
		return nil, err
	}
	worldRanges, err := recommendedRanges(script.Cells, "[worldmap event level]")
	if err != nil {
		return nil, err
	}
	r := RecommendedRules{SourceChecksum: a.Snapshot().Checksum, Sources: map[string]string{sourcePath: script.SHA256}, Ranges: map[uint32][2]uint32{}, Excluded: []uint32{}, AmbiguousDungeons: []uint32{}, UnavailableWorldmaps: []uint32{}}
	r.MinimumLevel, err = ruleUint(script.Cells, "[apply level]")
	if err != nil {
		return nil, err
	}
	ids := []uint32{}
	for world := range worldRanges {
		ids = append(ids, world)
	}
	slices.Sort(ids)
	ambiguous := map[uint32]bool{}
	for _, world := range ids {
		path, listed := paths[world]
		if _, found := a.FindFile(path); !listed || !found {
			r.UnavailableWorldmaps = append(r.UnavailableWorldmaps, world)
			continue
		}
		wdm, err := catalog.ReadScript(a, path)
		if err != nil {
			return nil, err
		}
		r.Sources[path] = wdm.SHA256
		dungeons, err := recommendedWorldmapDungeons(wdm.Cells)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for _, dungeon := range dungeons {
			if old, ok := r.Ranges[dungeon]; ok && old != worldRanges[world] {
				ambiguous[dungeon] = true
			}
			r.Ranges[dungeon] = worldRanges[world]
		}
	}
	for dungeon := range ambiguous {
		delete(r.Ranges, dungeon)
	}
	for dungeon, bounds := range specific {
		r.Ranges[dungeon] = bounds
	}
	excluded := map[uint32]bool{}
	for _, t := range ruleSection(script.Cells, "[unable dungeon]") {
		if t.Type != 0 || t.Value <= 0 {
			return nil, fmt.Errorf("invalid recommended dungeon exclusion")
		}
		excluded[uint32(t.Value)] = true
	}
	for id := range excluded {
		r.Excluded = append(r.Excluded, id)
	}
	slices.Sort(r.Excluded)
	for dungeon := range ambiguous {
		if _, specificRange := specific[dungeon]; !specificRange && !excluded[dungeon] {
			r.AmbiguousDungeons = append(r.AmbiguousDungeons, dungeon)
		}
	}
	slices.Sort(r.AmbiguousDungeons)
	return NewRecommendedRules(r)
}

func recommendedRanges(cells []pvf.Token, tag string) (map[uint32][2]uint32, error) {
	values := ruleSection(cells, tag)
	if len(values)%3 != 0 {
		return nil, fmt.Errorf("unpaired recommended range: %s", tag)
	}
	r := map[uint32][2]uint32{}
	for i := 0; i < len(values); i += 3 {
		for _, t := range values[i : i+3] {
			if t.Type != 0 || t.Value < 0 {
				return nil, fmt.Errorf("invalid recommended range: %s", tag)
			}
		}
		id, low, high := uint32(values[i].Value), uint32(values[i+1].Value), uint32(values[i+2].Value)
		if _, ok := r[id]; ok || low == 0 || high < low {
			return nil, fmt.Errorf("duplicate or inverted recommended range: %s", tag)
		}
		r[id] = [2]uint32{low, high}
	}
	return r, nil
}

func recommendedWorldmapDungeons(cells []pvf.Token) ([]uint32, error) {
	var out []uint32
	for _, rows := range ruleBlocks(cells, "[dungeon]") {
		for at := 0; at < len(rows); {
			if rows[at].Type != 0 || rows[at].Value <= 0 {
				return nil, fmt.Errorf("invalid worldmap dungeon id")
			}
			dungeon := uint32(rows[at].Value)
			at++
			if at < len(rows) && rows[at].Type == 3 {
				if rows[at].Text != "[in progress]" && rows[at].Text != "[is clear quest]" {
					return nil, fmt.Errorf("unverified worldmap condition %s", rows[at].Text)
				}
				at++
			}
			if at >= len(rows) || rows[at].Type != 0 {
				return nil, fmt.Errorf("missing worldmap quest condition")
			}
			at++
			out = append(out, dungeon)
		}
	}
	return out, nil
}
