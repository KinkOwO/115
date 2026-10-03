// Package testfixture materializes immutable historical catalogs for tests.
// Runtime commands must use native PVF providers, never these snapshots.
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

// CatalogPath preserves complete historical graphs and their original bytes.
// Each test owns its temporary files, including the world's NPC side catalog.
func CatalogPath(t testing.TB, domain string) string {
	t.Helper()
	var names []string
	switch domain {
	case "world":
		names = []string{"world.generated.json", "npc-teleport.generated.json"}
	case "quests":
		names = []string{"quests.generated.json"}
	default:
		t.Fatalf("unknown historical catalog %q", domain)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate historical fixtures")
	}
	dir := t.TempDir()
	for _, name := range names {
		materialize(t, filepath.Join(filepath.Dir(file), "testdata", name+".gz"), filepath.Join(dir, name), name)
	}
	return filepath.Join(dir, names[0])
}

func materialize(t testing.TB, source, target, name string) {
	t.Helper()
	want := map[string]string{
		"world.generated.json":        "be4676943376b1286fcc6016bbc1d70d60d992906bdc77a9e7d4e63026a8e0ae",
		"quests.generated.json":       "a19382d7b0f12803a1b703268e68852f224e82e3816169051f09303852b0442f",
		"npc-teleport.generated.json": "aac89fba07cee4f4be92b4ece0873ae80aee4c8d4bbda76394a88b27d48178c8",
	}[name]
	f, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
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
}
