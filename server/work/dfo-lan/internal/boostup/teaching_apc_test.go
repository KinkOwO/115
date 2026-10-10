package boostup

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func teachingAPCCells(template, event int32) []pvf.Token {
	return []pvf.Token{
		{Type: 3, Text: "[character]"},
		{Type: 3, Text: "[index]"}, {Value: 7},
		{Type: 3, Text: "[apc index]"}, {Value: template},
		{Type: 3, Text: "[usable contents]"}, {Type: 5, Text: "boost up event"},
		{Type: 3, Text: "[/usable contents]"},
		{Type: 3, Text: "[type]"}, {Type: 5, Text: "event"}, {Value: event},
		{Type: 3, Text: "[/character]"},
	}
}

func TestTeachingAPCsFollowSourceBindings(t *testing.T) {
	for _, template := range []int32{2504, 9997} {
		rows, err := ParseTeachingAPCs(teachingAPCCells(template, 886))
		if err != nil || len(rows) != 1 || rows[0] != (TeachingAPC{7, uint32(template), 886}) {
			t.Fatalf("changed source did not control binding: %+v %v", rows, err)
		}
	}
	reserved := []pvf.Token{{Type: 3, Text: "[character]"}, {Type: 3, Text: "[index]"}, {Value: 5}, {Type: 3, Text: "[/character]"}}
	rows, err := ParseTeachingAPCs(append(reserved, teachingAPCCells(2504, 662)...))
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	none := teachingAPCCells(55806, -1)
	none[9].Text = "none"
	rows, err = ParseTeachingAPCs(none)
	if err != nil || len(rows) != 0 {
		t.Fatal("reserved non-event row became a teaching companion", rows, err)
	}
	bad := teachingAPCCells(0, 662)
	if _, err = ParseTeachingAPCs(bad); err == nil {
		t.Fatal("missing AIC template accepted")
	}
	if _, err = ParseTeachingAPCs(append(teachingAPCCells(2504, 662), teachingAPCCells(3001, 662)...)); err == nil {
		t.Fatal("duplicate special index accepted")
	}
}
