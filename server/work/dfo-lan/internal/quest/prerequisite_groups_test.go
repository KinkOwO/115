package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/testfixture"
	"reflect"
	"testing"
)

func TestAlternativePrerequisiteSections(t *testing.T) {
	c, err := catalog.LoadQuests(testfixture.CatalogPath(t, "quests"))
	if err != nil {
		t.Fatal(err)
	}
	x := BuildIndex(c)
	act2 := x.Entries[3240]
	if act2 == nil || !reflect.DeepEqual(act2.PrerequisiteGroups, [][]uint32{{3232}, {3237}}) {
		t.Fatalf("quest 3240 prerequisite sections changed: %+v", act2)
	}
	for _, done := range []uint32{3232, 3237} {
		if !prerequisitesMet(act2.PrerequisiteGroups, map[uint32]string{done: "completed"}) {
			t.Fatalf("quest 3240 blocked after alternative %d completed", done)
		}
	}
	if prerequisitesMet(act2.PrerequisiteGroups, nil) {
		t.Fatal("quest 3240 offered before either branch completed")
	}

	// Luke's branch requires 3883 AND one of 3871/3872. This checks that
	// IDs within one section still form a conjunction.
	luke := x.Entries[3924]
	if luke == nil || !reflect.DeepEqual(luke.PrerequisiteGroups, [][]uint32{{3883, 3871}, {3883, 3872}}) {
		t.Fatalf("quest 3924 prerequisite sections changed: %+v", luke)
	}
	if prerequisitesMet(luke.PrerequisiteGroups, map[uint32]string{3871: "completed"}) ||
		!prerequisitesMet(luke.PrerequisiteGroups, map[uint32]string{3883: "completed", 3872: "completed"}) {
		t.Fatal("compound prerequisite section semantics changed")
	}
}
