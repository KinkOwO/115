package npcpresence

import "dfolan/internal/catalog"

// ProjectMap selects a source root from an explicitly supplied phase. It does
// not claim that this root won native map overrides or was instantiated.
// Import traversal order is retained as source evidence; imported rows are
// separate parser invocations, never concatenated into one placement vector.
func ProjectMap(area catalog.WorldArea, phase *int32) MapEvidence {
	m := MapEvidence{Phase: phase}
	if phase == nil {
		m.Gaps = append(m.Gaps, "initial/current phase cache is not supplied")
		return m
	}
	var root catalog.ScriptRecord
	var imports []catalog.ScriptRecord
	if *phase == -1 {
		root, imports = area.Map, area.ImportedScripts
	} else if *phase >= 0 {
		matches := 0
		for _, slot := range area.PhaseMaps {
			if slot.Index == *phase {
				matches++
				root, imports = slot.Map, slot.ImportedScripts
				m.Gaps = append(m.Gaps, slot.Pending...)
			}
		}
		if matches != 1 {
			m.Gaps = append(m.Gaps, "phase source slot is absent or ambiguous; flattened NPC rows cannot replace it")
			return m
		}
	} else {
		m.Gaps = append(m.Gaps, "phase index is outside the confirmed source scope")
		return m
	}
	if root.Path == "" || len(root.SHA256) != 64 {
		m.Gaps = append(m.Gaps, "root map source identity is missing")
		return m
	}
	m.RootPath = root.Path
	for _, script := range append([]catalog.ScriptRecord{root}, imports...) {
		if script.Path == "" || len(script.SHA256) != 64 {
			m.Gaps = append(m.Gaps, "map import source identity is missing")
			continue
		}
		projection := ProjectPlacements(script.Cells)
		m.Gaps = append(m.Gaps, projection.Unresolved...)
		for _, row := range projection.Rows {
			m.Placements = append(m.Placements, SourcePlacement{script.Path, script.SHA256, row})
		}
	}
	// Native area/activity/dungeon map overrides are separate from source slot
	// selection. Until those are supplied, Selected remains Unknown.
	m.Gaps = append(m.Gaps, "final map override order is not supplied")
	return m
}
