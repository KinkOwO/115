package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestSetPointsUseNativeGroupsWithoutMixingSets(t *testing.T) {
	cells := []pvf.Token{{Type: 3, Text: "[ability group]"}, {Type: 3, Text: "[index]"}, {Value: 54}, {Type: 3, Text: "[list]"}, {Value: 100}, {Value: 100}, {Value: 101}, {Type: 3, Text: "[/ability group]"}}
	groups, e := EquipmentAbilityGroups(cells)
	if e != nil {
		t.Fatal(e)
	}
	if len(groups[100]) != 1 {
		t.Fatal("duplicate membership")
	}
	p := PointRules{Groups: groups, Set: SetPointTable{Rules: []SetPointRule{{54, 0, -1, 265}, {54, 1, -1, 285}}}}
	items := []PointItem{{100, 0, 16201}, {101, 1, 16201}, {100, 0, 16202}, {999, 0, 16201}}
	if total, hits := p.SetPoints(items); total != 550 || hits != 3 {
		t.Fatalf("points=%d hits=%d", total, hits)
	}
	if total, hits := p.SetPoints([]PointItem{{100, 9, 16201}, {100, 0, -1}}); total != 0 || hits != 0 {
		t.Fatal("invented rank or set")
	}
	p.Groups[100] = []uint32{54, 55}
	p.Set.Rules = append(p.Set.Rules, SetPointRule{55, 0, -1, 265})
	if total, hits := p.SetPoints([]PointItem{{100, 0, 16201}}); total != 265 || hits != 1 {
		t.Fatal("counted duplicate group twice")
	}
	p.Set.Rules[len(p.Set.Rules)-1].Value = 266
	if total, hits := p.SetPoints([]PointItem{{100, 0, 16201}}); total != 0 || hits != 0 {
		t.Fatal("ambiguous source guessed")
	}
}
