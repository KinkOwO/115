package testfixture

import (
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// DungeonPath exposes complete immutable historical exports only to tests.
// The original bytes preserve old admission/layout/scene regression coverage.
func DungeonPath(t testing.TB, name string) string {
	t.Helper()
	want, ok := map[string]string{
		"dungeons.generated.json":              "595e27d519ffc549fd206ef6906d3d5cba9e394c5633f1302ef75c80a392b1da",
		"dungeons.next28.json":                 "c75f77fdd259b256c8ad0491a88bae9173ee01033ea996aa937decb4c945aa9a",
		"dungeons.odyssey-candidate.json":      "7e4bae92d2321d53e2caf81a1fe36af93bbdf28f72d7f6fac1daaf9d8c29cc38",
		"dungeons.odyssey-release.json":        "f9a56ed8ee23a7a8623dba5e8b227e83be3fb57d5600cd7d2fccef668da87b75",
		"dungeons.odyssey-scenes-release.json": "26e1f74bfe7a03bfb8277b0334396fc1b38339afe0b771a1dd50f8df26cb9f6f",
		"dungeons.skycastle-candidate.json":    "43c1e0808e5c88cc9e30bb8a397612be9d984d8585dfc192f7d04a5809222971",
	}[name]
	if !ok {
		t.Fatalf("unknown historical dungeon catalog %q", name)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate historical dungeon fixture")
	}
	f, err := os.Open(filepath.Join(filepath.Dir(file), "testdata", name+".gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	path := filepath.Join(t.TempDir(), name)
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(out, hash), r)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		t.Fatalf("materialize %s: copy=%v close=%v", name, copyErr, closeErr)
	}
	if got := fmt.Sprintf("%x", hash.Sum(nil)); got != want {
		t.Fatalf("historical %s bytes changed: %s", name, got)
	}
	return path
}
