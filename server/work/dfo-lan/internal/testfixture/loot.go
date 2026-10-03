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

// LootLevel150Path exposes the retired full loot export only to offline tests.
func LootLevel150Path(t testing.TB) string {
	t.Helper()
	const expected = "939c837b9c1b966cf1655dace420361d03613354869c17a607cbe703b7e0b6cf"
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate loot fixture")
	}
	f, err := os.Open(filepath.Join(filepath.Dir(file), "testdata", "loot", "loot.level150.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	path := filepath.Join(t.TempDir(), "loot.level150.json")
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(out, hash), r)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		t.Fatalf("materialize loot fixture: copy=%v close=%v", copyErr, closeErr)
	}
	if got := fmt.Sprintf("%x", hash.Sum(nil)); got != expected {
		t.Fatalf("historical loot bytes changed: %s", got)
	}
	return path
}
