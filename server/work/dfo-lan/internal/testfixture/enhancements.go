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

var enhancementHashes = map[string]string{
	"reinforcement-tickets.json": "8bbeae8b169b7e497d1fe44379c0c6033880e495c859d23649167c6404dfa793",
	"reinforcement-gold.json":    "027dddc534e28bce80bb5905b6ec84f0f38703682f2e38cf143f66cbcaf8bd4f",
	"amplify-grimoire.json":      "7c46704b5bb0a741a232154d23ce8f8ad586d281a99c9b845793a03016bda715",
	"amplify-upgrade.json":       "007838a55923a641078be3ef2cc644147ca3b55ad57a7f93defad5d3e1f70f01",
	"amplify-tickets.json":       "80e12f24eecedeced73c2e4c557339cd0fe5ee01661dc91bc7554dbfa68dc0a4",
	"enchant-beads.json":         "3419fba4fef95135fcc889778f6dbace2e773eb175eba9403d075b51b33c6f31",
}

// EnhancementsDir materializes all six immutable historical enhancement exports
// in an isolated test directory. Runtime preparation reads the native PVF source.
func EnhancementsDir(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()
	for name := range enhancementHashes {
		materializeEnhancement(t, dir, name)
	}
	return dir
}

// EnhancementPath provides one complete historical family for consumer tests.
func EnhancementPath(t testing.TB, name string) string {
	t.Helper()
	dir := t.TempDir()
	materializeEnhancement(t, dir, name)
	return filepath.Join(dir, name)
}

func materializeEnhancement(t testing.TB, dir, name string) {
	t.Helper()
	want, ok := enhancementHashes[name]
	if !ok {
		t.Fatalf("unknown historical enhancement fixture %q", name)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate historical enhancement fixtures")
	}
	f, err := os.Open(filepath.Join(filepath.Dir(file), "testdata", "enhancements", name+".gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	out, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(out, h), r)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		t.Fatalf("materialize %s: copy=%v close=%v", name, copyErr, closeErr)
	}
	if got := fmt.Sprintf("%x", h.Sum(nil)); got != want {
		t.Fatalf("historical %s bytes changed: %s", name, got)
	}
}
