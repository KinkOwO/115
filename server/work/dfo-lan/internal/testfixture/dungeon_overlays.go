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

// DungeonOverlayPath materializes a byte-identical historical source-map
// overlay for native parity and regression tests. Production never reads it.
func DungeonOverlayPath(t testing.TB, name string) string {
	t.Helper()
	want, ok := map[string]string{
		"dungeons.hell-party-maps.json":          "701996d14e9d578acd11cc96c40aff533e702ed27376a07c238e9f0425edfeea",
		"dungeons.tower-of-grief-maps.json":      "8c62dd91a80e91e1a6e57e7b584da34a236a76b30dfc0761e43a7eef2b4226ef",
		"dungeons.tower-of-dazzlement-maps.json": "9d966d0c7ca6117ab7f60f1d72eaf4ceaef7d80e8b1b2a0370abe7017b311023",
		"dungeons.tournament-quest-maps.json":    "162f6d2782272a8c4796384123764a550bf28cc0206b070e1579d57a04ac5c2f",
	}[name]
	if !ok {
		t.Fatalf("unknown historical dungeon overlay %q", name)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate historical dungeon overlay fixtures")
	}
	in, err := os.Open(filepath.Join(filepath.Dir(file), "testdata", "dungeons", "overlays", name+".gz"))
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
