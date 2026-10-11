package catalog

import (
	"os"
	"testing"
)

func TestRaidNativeETCRecruitingRoom(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("current PVF required")
	}
	a, err := OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := ImportChannelDirectory(a)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := ImportRaidEntrances(a, &d)
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range []uint32{98, 107} {
		r := entries[ch]
		s, err := ResolveScript(a, r.Path)
		if err != nil {
			t.Fatal(err)
		}
		want := sectionCells(s.Cells, "[RECRUITING ROOM]")
		if len(want) != 2 {
			t.Fatal("missing native preparation room")
		}
		if r.Recruiting == nil || r.Recruiting.TownID != uint32(want[0].Value) || r.Recruiting.AreaID != uint32(want[1].Value) || len(r.Recruiting.Walkable) == 0 || r.Recruiting.AreaID == r.Waiting.AreaID {
			t.Fatalf("channel %d discarded ETC preparation room: %+v", ch, r.Recruiting)
		}
	}
	// Old sources with no explicit recruitment declaration keep that absence.
	for _, r := range entries {
		if r.Bakal != nil && r.Recruiting != nil {
			t.Fatal("inferred an undeclared legacy preparation room")
		}
	}
}
