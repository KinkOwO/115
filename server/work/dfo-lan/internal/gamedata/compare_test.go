package gamedata

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComparePreservesSourceTokensAndSkillSlotDifferences(t *testing.T) {
	a := catalog.Characters{Source: pvf.ArchiveSnapshot{Path: "old", Checksum: "hash"}, Professions: map[byte]catalog.Profession{
		9: {ID: 9, InitialSkillSlots: map[uint16]uint16{118: 0}, InitialSkills: []int32{118, 1, 1}},
	}}
	b := catalog.Characters{Source: pvf.ArchiveSnapshot{Path: "new", Checksum: "hash"}, Professions: map[byte]catalog.Profession{
		9: {ID: 9, InitialSkillSlots: map[uint16]uint16{118: 1}, InitialSkills: []int32{118, 2, 1}},
	}}
	r := Compare(a, b, 10)
	if r.Count != 2 || r.Truncated || r.Differences[0].Path != "/professions/9/initial_skill_cells/1" || r.Differences[1].Path != "/professions/9/initial_skill_slots/118" {
		t.Fatalf("unexpected skill audit: %+v", r)
	}
}

func TestCompareIncludesRuntimeIndexesExcludedFromJSON(t *testing.T) {
	legacy := catalog.WorldCatalog{NPCMoves: []catalog.NPCMove{{NPCID: 1, TargetNPC: 2}}, NPCPlaces: map[uint32][]catalog.NPCPlace{1: {{Town: 3, Area: 4}}}}
	direct := catalog.WorldCatalog{}
	r := Compare(legacy, direct, 10)
	if r.Count != 2 || r.Differences[0].Path != "/NPCMoves/length" || r.Differences[1].Path != "/NPCPlaces/1" {
		t.Fatalf("runtime indexes were hidden: %+v", r)
	}
}

func TestSourceChecksHashBeforeParsingUntrustedArchive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.pvf")
	if err := os.WriteFile(path, []byte("not an archive"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Open(Options{Mode: PVF, ArchivePath: path, ExpectedChecksum: strings.Repeat("00", 32)})
	if err == nil || !strings.Contains(err.Error(), "source mismatch") {
		t.Fatalf("wrong source was parsed: %v", err)
	}
}

func TestCompareReportsMissingKeysAndBoundsDetails(t *testing.T) {
	a := map[string][]int{"a/b": {1, 2}, "missing": {7}}
	b := map[string][]int{"a/b": {2}, "new": {8}}
	r := Compare(a, b, 2)
	if r.Count != 4 || len(r.Differences) != 2 || !r.Truncated || r.Differences[0].Path != "/a~1b/length" {
		t.Fatalf("unexpected bounded audit: %+v", r)
	}
	if got := Compare([]int(nil), []int{}, 1); got.Count != 0 {
		t.Fatal(got)
	}
	if got := Compare([]int{1, 2}, []int{2, 1}, 0); got.Count != 2 || !got.Truncated {
		t.Fatal(got)
	}
}

func TestSourceRejectsInvalidModeAndUnpinnedPVF(t *testing.T) {
	for _, options := range []Options{{Mode: "auto"}, {Mode: PVF}, {Mode: PVF, ArchivePath: "missing", ExpectedChecksum: "wrong"}} {
		if _, err := Open(options); err == nil {
			t.Fatalf("accepted unsafe options: %+v", options)
		}
	}
	source, err := Open(Options{ArchivePath: "missing"})
	if err != nil || source.archive != nil {
		t.Fatalf("JSON unexpectedly opens PVF: %v", err)
	}
	if _, err := source.Characters("missing"); err == nil {
		t.Fatal("missing JSON was accepted")
	}
}

func TestAuditRefusesUnknownDomainAndMissingBaseline(t *testing.T) {
	source, _ := Open(Options{})
	for _, domain := range []string{"unknown", "characters", "world", "quests", "progression"} {
		r := AuditCatalog(domain, "missing", source, 2)
		if r.Equivalent || r.Error == "" {
			t.Fatalf("audit hid error: %+v", r)
		}
	}
}
