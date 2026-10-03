package character

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"
)

// All learning fields, source identities and runtime details are checked;
// a same-count projection is insufficient evidence for deleting exports.
func TestNativeLearningCurrentArchiveCompleteDetails(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete native skill details")
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: path, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if a.Snapshot().Checksum != "8b2a9f83247e000a28acd5134b616da725f46def39980b5373030e8cbc5d0934" {
		t.Fatalf("unexpected native source: %s", a.Snapshot().Checksum)
	}
	chars, err := catalog.ImportCharacters(a)
	if err != nil {
		t.Fatal(err)
	}
	eager, err := ImportLearningCatalog(a, chars)
	if err != nil {
		t.Fatal(err)
	}
	lazy, err := ImportLearningCatalog(a, chars)
	if err != nil {
		t.Fatal(err)
	}
	if err = lazy.EnableRuntimeDetails(a); err != nil {
		t.Fatal(err)
	}
	defer lazy.Close()
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	rows := append([]LearningDefinition(nil), eager.Rows...)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Job != rows[j].Job {
			return rows[i].Job < rows[j].Job
		}
		return rows[i].ID < rows[j].ID
	})
	if len(rows) != 3224 {
		t.Fatalf("native skills: %d", len(rows))
	}
	for _, want := range rows {
		got, ok, err := lazy.Definition(want.Job, want.ID)
		if err != nil || !ok || !reflect.DeepEqual(got, want) {
			t.Fatalf("native detail %d/%d differs: found=%t error=%v", want.Job, want.ID, ok, err)
		}
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != "352504f912d7621d446affc4c098f18cb026ea88898265c90eb04cbf3fb2ef26" {
		t.Fatalf("complete native learning content changed: %s", got)
	}
	t.Logf("3224 complete native skill definitions and details unchanged after parent source close")
}
