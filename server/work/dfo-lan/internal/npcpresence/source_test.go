package npcpresence

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"strings"
	"testing"
)

func TestQuestSourceKeepsOrderedEffectsAndIndependentCancelFlag(t *testing.T) {
	cells := []pvf.Token{tag("[npc visibility]"), tag("[condition]"), word("[accept]"), tag("[visibility]"), word("[delete]"), tag("[npc]"), num(100), {Type: 7, Value: 123}, num(100), tag("[/npc]"), tag("[revert]"), word("[false]"), tag("[/npc visibility]"), tag("[visible npc]"), num(0), tag("[npc visivility not revert]")}
	p := ProjectQuest(catalog.QuestDefinition{Script: catalog.ScriptRecord{Cells: cells}})
	if len(p.Gaps) != 0 || len(p.Blocks) != 1 || !p.Blocks[0].Protect || p.Blocks[0].Show || p.Blocks[0].Revert || len(p.Blocks[0].NPCs) != 2 || p.CancelRevert || p.ShowOverride == nil || *p.ShowOverride != 0 {
		t.Fatalf("source: %+v", p)
	}
}

func TestMalformedOrDuplicateQuestFieldsCannotApplyPartialSource(t *testing.T) {
	p := ProjectQuest(catalog.QuestDefinition{Script: catalog.ScriptRecord{Cells: []pvf.Token{tag("[visible npc]"), num(71), tag("[visible npc]"), num(72)}}})
	if p.ShowOverride != nil || len(p.Gaps) != 1 {
		t.Fatalf("duplicate override guessed: %+v", p)
	}
	i := Index{Quests: map[uint32]QuestProjection{90: p}}
	r := NewConstructedVisibilityReplay()
	if i.ApplyCondition(r, 90, 90, 0, true) == nil || r.Snapshot(100).LogicalShow != Unknown {
		t.Fatal("partial unresolved source applied")
	}
}

func TestClearingProjectionDoesNotClaimAClosedConsumer(t *testing.T) {
	i := Index{Quests: map[uint32]QuestProjection{90: {Blocks: []VisibilityBlock{{Condition: 3, Show: true, NPCs: []uint32{100}}}}}}
	if i.ApplyCondition(NewConstructedVisibilityReplay(), 90, 90, 3, true) == nil {
		t.Fatal("unclosed clearing consumer applied")
	}
}

func TestUnifiedIndexRejectsSourceMismatchAndRequiresFinalMapProof(t *testing.T) {
	hash := strings.Repeat("a", 64)
	w := catalog.WorldCatalog{Source: pvf.ArchiveSnapshot{Checksum: hash}, Areas: map[string]catalog.WorldArea{"80/0": {Town: 80, Map: catalog.ScriptRecord{Path: "map/base.map", SHA256: hash}}}}
	q := catalog.QuestCatalog{Source: pvf.ArchiveSnapshot{Checksum: strings.Repeat("b", 64)}}
	if _, err := NewIndex(w, q); err == nil {
		t.Fatal("mixed PVF catalogs accepted")
	}
	q.Source = w.Source
	i, err := NewIndex(w, q)
	if err != nil {
		t.Fatal(err)
	}
	phase := int32(-1)
	r := NewConstructedVisibilityReplay()
	r.RequestShow(100)
	query := Query{Town: 80, NPC: 100, Phase: &phase, Instances: map[uint32]Truth{100: True}, Visibility: r}
	if got := i.ResolveNPC(query); got.State != StateUnknown {
		t.Fatalf("unproved final root: %+v", got)
	}
	query.FinalRoot = "map/other.map"
	if got := i.ResolveNPC(query); got.State != StateUnknown {
		t.Fatalf("wrong final root: %+v", got)
	}
	query.FinalRoot = "map/base.map"
	if got := i.ResolveNPC(query); got.State != Present {
		t.Fatalf("known selected root: %+v", got)
	}
}
