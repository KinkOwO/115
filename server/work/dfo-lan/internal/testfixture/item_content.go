package testfixture

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ItemContentPath returns a complete historical item-domain catalog with only
// its provenance checksum changed to the supplied archive checksum. The source
// file is hash-pinned before that temporary rewrite; the fixture is for audit
// comparisons only and must never be used by runtime loaders.
func ItemContentPath(t testing.TB, domain, checksum string) string {
	t.Helper()
	if len(checksum) != 64 {
		t.Fatalf("invalid item-content provenance checksum %q", checksum)
	}
	fixture := map[string]struct{ file, hash string }{
		"materials": {"item-materials.json", "2b8c998ea701f4734b86db3aefc4794abf9235fa2dce96a83a3588dfa9280e3a"},
		"periods":   {"item-period-tags.json", "6f271d29ac7013f971a147c0ef0130d682c4522a5eedf3fff917612119b0265e"},
		"skins":     {"skin-storage-items.json", "7dea69de5ba61a5b4c321192926b291780e59bd67428552d90514add629f0d85"},
	}[domain]
	if fixture.file == "" {
		t.Fatalf("unknown item-content fixture %q", domain)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate item-content fixtures")
	}
	source := filepath.Join(filepath.Dir(file), "testdata", "item-content", fixture.file+".gz")
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
	original, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(original)); got != fixture.hash {
		t.Fatalf("historical %s fixture changed: %s", domain, got)
	}
	var document map[string]any
	if err := json.Unmarshal(original, &document); err != nil {
		t.Fatalf("decode historical %s fixture: %v", domain, err)
	}
	if domain == "materials" {
		if _, ok := document["source"].(string); !ok {
			t.Fatal("materials provenance is not a checksum string")
		}
		document["source"] = checksum
	} else {
		source, ok := document["source"].(map[string]any)
		if !ok {
			t.Fatalf("%s provenance is not an archive snapshot", domain)
		}
		if _, ok := source["checksum"].(string); !ok {
			t.Fatalf("%s provenance checksum is missing", domain)
		}
		source["checksum"] = checksum
	}
	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	updated = append(updated, '\n')
	path := filepath.Join(t.TempDir(), fixture.file)
	if err := os.WriteFile(path, bytes.Clone(updated), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
