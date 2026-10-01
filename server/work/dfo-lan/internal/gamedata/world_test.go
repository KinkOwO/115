package gamedata

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"strings"
	"testing"
)

func phaseWorldPair() (catalog.WorldCatalog, catalog.WorldCatalog) {
	script := catalog.ScriptRecord{Path: "map/phase.map", SHA256: strings.Repeat("a", 64), Cells: []pvf.Token{
		{Type: 3, Text: "[NPC]"}, {Type: 0, Value: 7}, {Type: 6, Text: "npc"}, {Type: 0, Value: 100}, {Type: 0, Value: 200}, {Type: 0, Value: 0},
	}}
	area := catalog.WorldArea{Town: 1, Area: 0, Definition: []pvf.Token{{Type: 3, Text: "[phase]"}, {Type: 6, Text: "phase.map"}}, PhaseNPCs: catalog.PhaseNPCsFromScript(script)}
	legacy := catalog.WorldCatalog{Areas: map[string]catalog.WorldArea{"1/0": area}}
	area.PhaseMaps = []catalog.PhaseMap{{Index: 0, SourcePath: "phase.map", Map: script}}
	direct := catalog.WorldCatalog{Areas: map[string]catalog.WorldArea{"1/0": area}}
	return legacy, direct
}

func TestWorldMigrationRetainsFullAuditAndOnlyClassifiesSourcePhaseAdditions(t *testing.T) {
	legacy, direct := phaseWorldPair()
	if Compare(legacy, direct, 10).Count != 1 {
		t.Fatal("full field audit lost the added graph")
	}
	c, additions, err := CompareWorldMigration(legacy, direct, 10)
	if err != nil || c.Count != 0 || len(additions) != 1 || additions[0].Slots != 1 {
		t.Fatal(c, additions, err)
	}
	if len(direct.Areas["1/0"].PhaseMaps) != 1 {
		t.Fatal("comparison mutated direct catalog")
	}
	area := direct.Areas["1/0"]
	area.MinimumLevel = 90
	direct.Areas["1/0"] = area
	c, _, err = CompareWorldMigration(legacy, direct, 10)
	if err != nil || c.Count != 1 {
		t.Fatal("permission change suppressed", c, err)
	}
}

func TestWorldMigrationRejectsUnprovenOrChangedPhaseGraphs(t *testing.T) {
	for _, change := range []struct {
		name  string
		apply func(*catalog.WorldArea)
	}{
		{"ordinal", func(a *catalog.WorldArea) { a.PhaseMaps[0].Index = 1 }},
		{"reference", func(a *catalog.WorldArea) { a.PhaseMaps[0].SourcePath = "other.map" }},
		{"map", func(a *catalog.WorldArea) { a.PhaseMaps[0].Map.Path = "map/other.map" }},
		{"missing_hash", func(a *catalog.WorldArea) { a.PhaseMaps[0].Map.SHA256 = "" }},
		{"pending", func(a *catalog.WorldArea) { a.PhaseMaps[0].Pending = []string{"missing import"} }},
		{"definition", func(a *catalog.WorldArea) { a.Definition = nil }},
		{"placement", func(a *catalog.WorldArea) { a.PhaseNPCs = nil }},
	} {
		t.Run(change.name, func(t *testing.T) {
			legacy, direct := phaseWorldPair()
			area := direct.Areas["1/0"]
			change.apply(&area)
			direct.Areas["1/0"] = area
			if _, _, err := CompareWorldMigration(legacy, direct, 10); err == nil {
				t.Fatal("unproven phase graph accepted")
			}
		})
	}
	legacy, direct := phaseWorldPair()
	legacy.Areas["1/0"] = direct.Areas["1/0"]
	area := direct.Areas["1/0"]
	area.PhaseMaps = append([]catalog.PhaseMap(nil), area.PhaseMaps...)
	area.PhaseMaps[0].Map.SHA256 = strings.Repeat("b", 64)
	direct.Areas["1/0"] = area
	c, additions, err := CompareWorldMigration(legacy, direct, 10)
	if err != nil || c.Count != 1 || len(additions) != 0 {
		t.Fatal("existing graph change suppressed", c, additions, err)
	}
}
