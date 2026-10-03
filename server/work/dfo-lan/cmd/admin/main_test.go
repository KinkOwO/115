package main

import (
	"dfolan/internal/managementdata"
	"flag"
	"os"
	"testing"
)

func TestAdminRejectsRetiredJSONSource(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := managementdata.Register(fs)
	if err := fs.Parse([]string{"-catalog-source", "json"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Open(); err == nil {
		t.Fatal("retired content source accepted")
	}
}
func TestAdminNativeNonDropAndFullEquipment(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native admin catalog preparation")
	}
	f := managementdata.Flags{Mode: "pvf", ArchivePath: path, Checksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256")}
	s, e := f.Open()
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	a, e := managementdata.Awarder(s, "../../configs/pvf-drop-policy.json", "../../configs/inventory.next29.json")
	if e != nil {
		t.Fatal(e)
	}
	defer a.Equipment.Full.Close()
	for _, id := range []uint32{10418036, 10418035} {
		if a.Catalog.Items[id].Kind != "stackable" {
			t.Fatal("non-drop currency unreachable", id)
		}
	}
	if _, e := a.Equipment.Reward(100050791); e != nil {
		t.Fatal("import-script equipment unreachable", e)
	}
}
