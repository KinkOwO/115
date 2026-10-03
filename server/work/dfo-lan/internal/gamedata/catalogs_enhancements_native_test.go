package gamedata

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestRetiredEnhancementJSONCannotBecomeRuntimeSource(t *testing.T) {
	for _, c := range []*Catalogs{{}, {selected: map[string]bool{"enhancements": true}}} {
		if err := c.LoadEnhancements("../../configs"); err == nil {
			t.Fatal("retired enhancement JSON runtime fallback accepted")
		}
	}
}

func TestNativeEnhancementsCurrentArchiveFingerprint(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete native enhancement fingerprint")
	}
	s, err := Open(Options{Mode: PVF, ArchivePath: path, ExpectedChecksum: "8b2a9f83247e000a28acd5134b616da725f46def39980b5373030e8cbc5d0934"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	index, err := s.ItemIndex("")
	if err != nil {
		t.Fatal(err)
	}
	native, err := s.Enhancements(index, "../../configs/pvf-enhancement-policy.json")
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(native)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(b)); got != "22083b62b1071c074bb6592612309c7dfd94d3070838623477a828fd2c551507" {
		t.Fatalf("complete native enhancement content changed: %s", got)
	}
	if len(native.ReinforcementTickets) != 1196 || len(native.AmplifyTickets) != 1629 || len(native.Grimoires.Grimoires) != 433 || len(native.Enchant.Beads) != 4846 || len(native.Gold.Levels) != 255 || len(native.Amplify.Levels) != 255 {
		t.Fatal("native enhancement family coverage changed")
	}
}
