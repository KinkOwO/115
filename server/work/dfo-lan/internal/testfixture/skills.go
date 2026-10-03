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

// SkillCatalogPath materializes a byte-identical historical learning catalog.
// The two exports encode distinct historical projections used by regressions;
// neither is a runtime content source.
func SkillCatalogPath(t testing.TB, version string) string {
	t.Helper()
	want, ok := map[string]string{
		"next27":  "fd2116b15287acc3236cee0f7875f12a50f597a9dd3b251d8ba8ba8a4c099cd5",
		"release": "21b0e874beb5872c41b2e206fb6c8d0b5eddad1b96bedc7e102a85c7bbd8d112",
	}[version]
	if !ok {
		t.Fatalf("unknown historical skill projection %q", version)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate historical skill fixtures")
	}
	name := "skills." + version + ".json"
	in, err := os.Open(filepath.Join(filepath.Dir(file), "testdata", "skills", name+".gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	r, err := gzip.NewReader(in)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	target := filepath.Join(t.TempDir(), name)
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
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
	return target
}
