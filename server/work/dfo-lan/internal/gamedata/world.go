package gamedata

import (
	"dfolan/internal/catalog"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"strings"
)

// WorldProjectionAddition names source-backed phase graphs absent from older
// JSON exports. The complete field audit still reports these additions.
type WorldProjectionAddition struct {
	Area  string `json:"area"`
	Slots int    `json:"slots"`
}

// CompareWorldMigration checks every existing runtime field. It permits only
// a newly retained phase graph whose root order, script identities and flattened
// NPC rows agree with the source and the existing catalog. It never suppresses
// a changed placement, permission, portal, script or already-present phase graph.
func CompareWorldMigration(legacy, direct catalog.WorldCatalog, limit int) (Comparison, []WorldProjectionAddition, error) {
	projected := direct
	projected.Areas = make(map[string]catalog.WorldArea, len(direct.Areas))
	var additions []WorldProjectionAddition
	for key, area := range direct.Areas {
		before, exists := legacy.Areas[key]
		if exists && len(before.PhaseMaps) == 0 && len(area.PhaseMaps) != 0 {
			if err := validatePhaseAddition(area); err != nil {
				return Comparison{}, nil, fmt.Errorf("area %s phase projection: %w", key, err)
			}
			additions = append(additions, WorldProjectionAddition{Area: key, Slots: len(area.PhaseMaps)})
			area.PhaseMaps = nil
		}
		projected.Areas[key] = area
	}
	sort.Slice(additions, func(i, j int) bool { return additions[i].Area < additions[j].Area })
	return Compare(legacy, projected, limit), additions, nil
}

func validatePhaseAddition(area catalog.WorldArea) error {
	var refs []string
	active := false
	for _, cell := range area.Definition {
		if cell.Type == 3 {
			active = cell.Text == "[phase]"
			continue
		}
		if active {
			if cell.Type != 6 {
				return fmt.Errorf("unresolved phase reference")
			}
			refs = append(refs, cell.Text)
		}
	}
	if len(refs) != len(area.PhaseMaps) {
		return fmt.Errorf("phase count differs from area definition")
	}
	var npcs []catalog.PhaseNPC
	for i, slot := range area.PhaseMaps {
		if slot.Index != int32(i) || slot.SourcePath != refs[i] || len(slot.Pending) != 0 {
			return fmt.Errorf("phase %d order, reference or pending data differs", i)
		}
		name := strings.ToLower(strings.ReplaceAll(refs[i], "\\", "/"))
		if !phasePathMatches(slot.Map.Path, name) {
			return fmt.Errorf("phase %d map does not match source reference", i)
		}
		for _, script := range append([]catalog.ScriptRecord{slot.Map}, slot.ImportedScripts...) {
			digest, err := hex.DecodeString(script.SHA256)
			if script.Path == "" || err != nil || len(digest) != 32 {
				return fmt.Errorf("phase %d missing script identity", i)
			}
			npcs = append(npcs, catalog.PhaseNPCsFromScript(script)...)
		}
	}
	if Compare(area.PhaseNPCs, npcs, 0).Count != 0 {
		return fmt.Errorf("phase graph changes flattened NPC projection")
	}
	return nil
}

func phasePathMatches(actual, reference string) bool {
	for _, candidate := range []string{reference, "map/" + strings.TrimPrefix(reference, "map/")} {
		if actual == candidate || actual == path.Join(path.Dir(candidate), "(r)"+path.Base(candidate)) {
			return true
		}
	}
	return false
}
