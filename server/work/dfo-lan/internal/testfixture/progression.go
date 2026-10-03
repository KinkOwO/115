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

// ProgressionPath exposes the complete historical progression export to tests.
func ProgressionPath(t testing.TB) string {
	t.Helper()
	const expected = "587f1bdc26618607a91931ec987d6bda7f72ed6b5e1924edd7658d4b1c75eb5a"
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate progression fixture")
	}
	f, err := os.Open(filepath.Join(filepath.Dir(file), "testdata", "progression", "progression.next25.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	path := filepath.Join(t.TempDir(), "progression.next25.json")
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(out, hash), r)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		t.Fatalf("materialize progression fixture: copy=%v close=%v", copyErr, closeErr)
	}
	if got := fmt.Sprintf("%x", hash.Sum(nil)); got != expected {
		t.Fatalf("historical progression bytes changed: %s", got)
	}
	return path
}
