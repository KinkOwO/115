package character

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"
)

// Explicit native archives make this a read-only compatibility audit. It never
// opens storage or rewrites a player's state to match a new resource hash.
func TestEntrySkillsAcrossNativeClientVersions(t *testing.T) {
	paths := []string{os.Getenv("DFO_COMPAT_OLD_ARCHIVE"), os.Getenv("DFO_COMPAT_NEW_ARCHIVE")}
	if paths[0] == "" || paths[1] == "" {
		t.Skip("set DFO_COMPAT_OLD_ARCHIVE and DFO_COMPAT_NEW_ARCHIVE")
	}
	var services [2]*Service
	for i, path := range paths {
		a, err := pvf.LoadArchive(pvf.Options{Path: path, MaxBytes: 1024 * 1024 * 1024})
		if err != nil {
			t.Fatal(err)
		}
		chars, err := catalog.ImportCharacters(a)
		if err != nil {
			a.Close()
			t.Fatal(err)
		}
		learning, err := ImportLearningCatalog(a, chars)
		if err != nil {
			a.Close()
			t.Fatal(err)
		}
		services[i] = &Service{Catalog: chars, Learning: learning}
		t.Logf("archive[%d]=%s professions=%d", i, a.Snapshot().Checksum, len(chars.Professions))
		if err := a.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for _, pair := range [][2]int{{0, 0}, {0, 1}, {1, 1}, {1, 0}} {
		source, target := services[pair[0]], services[pair[1]]
		t.Run(fmt.Sprintf("%d-to-%d", pair[0], pair[1]), func(t *testing.T) {
			count, changedHash := 0, 0
			for job, p := range source.Catalog.Professions {
				current, ok := target.Catalog.Professions[job]
				if !ok || current.Path != p.Path {
					t.Fatalf("profession %d reference changed", job)
				}
				branches := map[byte]bool{0: true}
				for branch := range p.AdvancementSkills {
					branches[branch] = true
				}
				var ordered []int
				for branch := range branches {
					ordered = append(ordered, int(branch))
				}
				sort.Ints(ordered)
				for _, branch := range ordered {
					state := State{Level: 115, Advancement: byte(branch), AllJobsPilot: true,
						SourcePath: p.Path, SourceSHA256: p.RawSHA256, InitialSkills: p.InitialSkills}
					raw, err := json.Marshal(state)
					if err != nil {
						t.Fatal(err)
					}
					role := Character{Profession: job, ConfigVersion: source.Catalog.Source.SaveIdentity(), State: raw}
					before := append([]byte(nil), raw...)
					if _, err := target.EntrySkills(role); err != nil {
						t.Fatalf("profession %d branch %d: %v", job, branch, err)
					}
					if !bytes.Equal(before, role.State) {
						t.Fatalf("profession %d state was rewritten", job)
					}
					count++
					if p.RawSHA256 != current.RawSHA256 {
						changedHash++
					}
				}
			}
			t.Logf("entry projections=%d changed profession hashes=%d storage_accessed=false", count, changedHash)
		})
	}
}
