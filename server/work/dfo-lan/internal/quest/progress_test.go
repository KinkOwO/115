package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestSingleClearMapStartsPending(t *testing.T) {
	d := catalog.QuestDefinition{Kind: "[clear map]", ObjectiveCells: []pvf.Token{{Type: 0, Value: 76126}}}
	initial, model, e := InitialProgress(d)
	if e != nil || initial != 1 || model != SingleClearMap {
		t.Fatalf("uncompleted map became submittable: %d %s %v", initial, model, e)
	}
	d.ObjectiveCells = append(d.ObjectiveCells, pvf.Token{Type: 0, Value: 76127})
	if _, _, e = InitialProgress(d); e == nil {
		t.Fatal("unverified multiple-objective encoding accepted")
	}
}
