package npcpresence

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func tag(s string) pvf.Token  { return pvf.Token{Type: 3, Text: s} }
func word(s string) pvf.Token { return pvf.Token{Type: 6, Text: s} }
func num(n int32) pvf.Token   { return pvf.Token{Type: 0, Value: n} }
func row(id int32) []pvf.Token {
	return []pvf.Token{num(id), word("[left]"), num(100), num(200), num(0)}
}

func TestPlacementOptionalFieldsCarryAndLaterVectorReplaces(t *testing.T) {
	cells := []pvf.Token{tag("[NPC]"), num(100), word("[visible on dungeon if quest clear]"), num(71), word("[visible on dungeon clear]"), word("[right]"), num(1), num(2), num(3)}
	cells = append(cells, row(101)...)
	cells = append(cells, tag("[another section]"), tag("[NPC]"))
	cells = append(cells, row(102)...)
	p := ProjectPlacements(cells)
	if len(p.Rows) != 3 || len(p.Unresolved) != 0 {
		t.Fatalf("projection: %+v", p)
	}
	for i, r := range p.Rows {
		if r.Quest != 71 || !r.Completed || !r.DungeonClear || r.SurvivesLastSection != (i == 2) {
			t.Fatalf("carried row: %+v", r)
		}
	}
	if p.Rows[0].Gate(QuestSet{}, QuestSet{}) != False {
		t.Fatal("replaced source row remained eligible")
	}
}

func TestPlacementAcceptedAndCompletedMembershipAreIndependent(t *testing.T) {
	p := Placement{Quest: 71, Resolved: true, SurvivesLastSection: true}
	accepted := QuestSet{Known: true, IDs: map[uint32]bool{71: true}}
	completed := QuestSet{Known: true}
	if p.Gate(accepted, completed) != True {
		t.Fatal("accepted placement refused")
	}
	p.Completed = true
	if p.Gate(accepted, completed) != False || p.Gate(accepted, QuestSet{}) != Unknown {
		t.Fatal("completed membership conflated with accepted or missing set")
	}
}

func TestMarkerWithoutQuestStillBypassesGate(t *testing.T) {
	p := ProjectPlacements([]pvf.Token{tag("[NPC]"), num(100), word("[visible on dungeon if quest clear]"), word("[left]"), num(1), num(2), num(0)})
	if len(p.Rows) != 1 || p.Rows[0].Quest != -1 || p.Rows[0].Gate(QuestSet{}, QuestSet{}) != True {
		t.Fatalf("projection: %+v", p)
	}
}

func TestMalformedEarlierSectionTaintsCarriedFields(t *testing.T) {
	cells := []pvf.Token{tag("[NPC]"), num(100), word("[unknown marker]"), tag("[NPC]")}
	cells = append(cells, row(101)...)
	p := ProjectPlacements(cells)
	if len(p.Rows) != 1 || len(p.Unresolved) != 1 || p.Rows[0].Gate(QuestSet{Known: true}, QuestSet{Known: true}) != Unknown {
		t.Fatalf("guessed parser defaults: %+v", p)
	}
}

func TestLaterEmptySectionRemovesPreviousRows(t *testing.T) {
	cells := append([]pvf.Token{tag("[NPC]")}, row(100)...)
	cells = append(cells, tag("[NPC]"))
	p := ProjectPlacements(cells)
	if len(p.Rows) != 1 || p.Rows[0].SurvivesLastSection {
		t.Fatalf("empty last section: %+v", p)
	}
}
