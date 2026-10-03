package shopimport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicCatalogReplacement(t *testing.T) {
	name := filepath.Join(t.TempDir(), "catalog.json")
	if e := writeAtomic(name, []byte("old")); e != nil {
		t.Fatal(e)
	}
	if e := writeAtomic(name, []byte("new")); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(name)
	if e != nil || string(b) != "new" {
		t.Fatal("replacement", e)
	}
	if e = writeAtomic(filepath.Join(name, "invalid"), []byte("broken")); e == nil {
		t.Fatal("invalid destination accepted")
	}
	b, e = os.ReadFile(name)
	if e != nil || string(b) != "new" {
		t.Fatal("failed write damaged catalog", e)
	}
}
