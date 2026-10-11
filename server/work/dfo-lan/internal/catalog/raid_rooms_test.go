package catalog

import (
	"dfolan/internal/catalog/pvf"
	"math"
	"testing"
)

func TestRaidCTPRecruitingRoomComesFromRootSource(t *testing.T) {
	number := func(v float64) pvf.CTPCell { return pvf.CTPCell{Kind: "float", Float: v} }
	rows := []pvf.CTPRecord{
		{Index: 0, Parent: -1, Name: "[RECRUITING ROOM]", Cells: []pvf.CTPCell{number(701), number(7)}},
		{Index: 1, Parent: -1, Name: "[WAITING ROOM]", Cells: []pvf.CTPCell{number(701), number(9)}},
		{Index: 2, Parent: -1, Name: "[RAID LIST WINDOW DIFFICULTY INFO]"},
		{Index: 3, Parent: 2, Name: "[INFO]"},
		{Index: 4, Parent: 3, Name: "[CHANNEL TYPE]", Cells: []pvf.CTPCell{number(112)}},
		{Index: 5, Parent: 3, Name: "[RECRUITING ROOM]", Cells: []pvf.CTPCell{number(999), number(1)}},
	}
	values, channels, err := raidCTPEntryValues(&pvf.CTPTable{Path: "room-fixture.ctp", Records: rows})
	if err != nil || len(channels) != 1 || channels[0] != 112 || len(values["[RECRUITING ROOM]"]) != 2 || values["[RECRUITING ROOM]"][0] != 701 || values["[RECRUITING ROOM]"][1] != 7 || values["[WAITING ROOM]"][1] != 9 {
		t.Fatalf("source recruiting header lost: %v %v %v", values, channels, err)
	}
	for _, invalid := range [][]pvf.CTPCell{
		{number(701)}, {number(-1), number(7)}, {number(701), number(7.5)}, {number(701), number(math.NaN())},
	} {
		bad := append([]pvf.CTPRecord(nil), rows...)
		bad[0].Cells = invalid
		if _, _, err := raidCTPEntryValues(&pvf.CTPTable{Path: "room-fixture.ctp", Records: bad}); err == nil {
			t.Fatal("malformed source recruiting room accepted")
		}
	}
}
