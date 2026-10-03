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

// LotteryPath materializes one complete historical lottery export for tests.
// Runtime catalog loading must use the prepared PVF projection instead.
func LotteryPath(t testing.TB, name string) string {
	t.Helper()
	want, ok := map[string]string{
		"lottery-item-pools.json":      "ee743bc7626a82d3cde880bf17164c37499637f079a294aa963673b42421c447",
		"lottery-equipment-pools.json": "a54a36ada189308dc0f003ee7ba8fd312cdfd7c533ace9c7904ab14f289cdf87",
	}[name]
	if !ok {
		t.Fatalf("unknown historical lottery fixture %q", name)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate historical lottery fixtures")
	}
	f, err := os.Open(filepath.Join(filepath.Dir(file), "testdata", "lottery", name+".gz"))
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
