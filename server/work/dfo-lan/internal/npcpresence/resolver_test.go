package npcpresence

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"strings"
	"testing"
)

func TestResolverSeparatesEligiblePlacementFromNativeInstance(t *testing.T) {
	r := NewConstructedVisibilityReplay()
	r.RequestShow(100)
	m := MapEvidence{RootPath: "map/phase.map", Selected: True, Placements: []SourcePlacement{{Placement: Placement{NPC: 100, Quest: -1, Resolved: true, SurvivesLastSection: true}}}}
	result := Resolve(100, m, QuestSet{}, QuestSet{}, r)
	if result.State != StateUnknown || len(result.Placements) != 1 || result.Placements[0].Gate != True {
		t.Fatalf("placement authorized instance: %+v", result)
	}
	m.Instances = map[uint32]Truth{100: True}
	if got := Resolve(100, m, QuestSet{}, QuestSet{}, r); got.State != Present {
		t.Fatalf("present: %+v", got)
	}
	r.RequestHide(100)
	if got := Resolve(100, m, QuestSet{}, QuestSet{}, r); got.State != Hidden {
		t.Fatalf("hidden: %+v", got)
	}
	m.Instances[100] = False
	if got := Resolve(100, m, QuestSet{}, QuestSet{}, r); got.State != Absent {
		t.Fatalf("absent: %+v", got)
	}
}

func TestUnknownMapDoesNotUseOtherwiseKnownNPC(t *testing.T) {
	r := NewConstructedVisibilityReplay()
	r.RequestShow(100)
	m := MapEvidence{RootPath: "map/candidate.map", Instances: map[uint32]Truth{100: True}}
	if got := Resolve(100, m, QuestSet{}, QuestSet{}, r); got.State != StateUnknown || got.InstancePresent != Unknown {
		t.Fatalf("candidate treated as selected root: %+v", got)
	}
}

func TestShowOverrideKeepsExistingNPCVisible(t *testing.T) {
	r := NewConstructedVisibilityReplay()
	r.RequestHide(100)
	r.AddShowOverride(100, 71)
	r.ShowEntity(100)
	m := MapEvidence{RootPath: "map/a.map", Selected: True, Instances: map[uint32]Truth{100: True}}
	if got := Resolve(100, m, QuestSet{}, QuestSet{}, r); got.State != Present || got.Visibility.LogicalShow != False {
		t.Fatalf("override: %+v", got)
	}
	r.RemoveShowOverride(100, 71)
	if got := Resolve(100, m, QuestSet{}, QuestSet{}, r); got.State != Hidden {
		t.Fatalf("released override: %+v", got)
	}
}

func TestMapProjectionPreservesDuplicateRootSlotsAndParserBoundaries(t *testing.T) {
	script := catalog.ScriptRecord{Path: "map/shared.map", SHA256: strings.Repeat("a", 64), Cells: append([]pvf.Token{tag("[NPC]")}, row(100)...)}
	imported := catalog.ScriptRecord{Path: "map/import.map", SHA256: strings.Repeat("b", 64), Cells: append([]pvf.Token{tag("[NPC]")}, row(101)...)}
	a := catalog.WorldArea{PhaseMaps: []catalog.PhaseMap{{Index: 0, Map: script}, {Index: 1, Map: script, ImportedScripts: []catalog.ScriptRecord{imported}}}}
	phase := int32(1)
	m := ProjectMap(a, &phase)
	if m.RootPath != script.Path || len(m.Placements) != 2 || m.Selected != Unknown || *m.Phase != 1 {
		t.Fatalf("root projection: %+v", m)
	}
	if !m.Placements[0].SurvivesLastSection || !m.Placements[1].SurvivesLastSection {
		t.Fatal("separate map vectors replaced each other")
	}
}

func TestOldFlattenedCatalogDoesNotInventPhaseRoot(t *testing.T) {
	a := catalog.WorldArea{PhaseNPCs: []catalog.PhaseNPC{{ID: 100, MapPath: "map/shared.map"}}}
	phase := int32(1)
	if m := ProjectMap(a, &phase); m.RootPath != "" || m.Selected != Unknown || len(m.Gaps) == 0 {
		t.Fatalf("old catalog inferred root: %+v", m)
	}
}
