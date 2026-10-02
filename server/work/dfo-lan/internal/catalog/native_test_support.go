package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"dfolan/internal/catalog/pvf"
)

// The native test archive is process-wide: every package that exercises the
// real inner PVF opens it once and reuses the checksum-verified metadata for
// the rest of the test binary. TestMain closes it after the run. The metadata
// cache (pvf.OpenReadOnlyCached) then makes a fresh package run reuse the
// parsed directory instead of re-parsing 726 MiB per test.
var (
	nativeArchiveOnce sync.Once
	nativeArchive     *pvf.Archive
	nativeArchiveErr  error
)

var errNativeArchiveEnvMissing = fmt.Errorf("DFO_PVF_CORE_TEST_ARCHIVE is not set")

// nativeModuleRoot locates the module root by walking up from the test working
// directory (go test runs each package in its own directory) until go.mod is
// found. Relative cache directories resolve against it so every package shares
// one runtime/pvf-cache instead of writing a copy beside each package.
var nativeModuleRoot = sync.OnceValues(func() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
})

// nativeParserIdentity binds the metadata cache to the pvf parser source under
// internal/catalog/pvf. Every test package hashes the same files, so all of
// them share one cache entry; any parser source change invalidates it. The old
// per-executable identity gave every test binary its own ~170MB cache copy.
var nativeParserIdentity = sync.OnceValues(func() (string, error) {
	root, err := nativeModuleRoot()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, "internal", "catalog", "pvf")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
})

// nativeCacheDir resolves DFO_PVF_CACHE_DIR (default runtime/pvf-cache) against
// the module root. "-" disables caching, matching the production flag.
func nativeCacheDir() string {
	dir := strings.TrimSpace(os.Getenv("DFO_PVF_CACHE_DIR"))
	if dir == "" {
		dir = "runtime/pvf-cache"
	}
	if dir == "-" || filepath.IsAbs(dir) {
		return dir
	}
	root, err := nativeModuleRoot()
	if err != nil {
		return dir
	}
	return filepath.Join(root, dir)
}

func requireNativeArchive(t *testing.T) {
	t.Helper()
	if os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE") == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native PVF source parity")
	}
}

// openNativeArchive opens the process-wide archive once. It never calls
// t.Skip/Fatal so a failed Once still reports deterministically to callers.
func openNativeArchive() (*pvf.Archive, error) {
	nativeArchiveOnce.Do(func() {
		path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
		if path == "" {
			nativeArchiveErr = errNativeArchiveEnvMissing
			return
		}
		parser, err := nativeParserIdentity()
		if err != nil {
			log.Printf("native PVF metadata cache unavailable; native parse: %v", err)
		}
		a, err := pvf.OpenReadOnlyCached(
			pvf.Options{Path: path, MaxBytes: 1024 * 1024 * 1024},
			os.Getenv("DFO_PVF_CORE_TEST_SHA256"),
			nativeCacheDir(), parser,
		)
		if err != nil {
			nativeArchiveErr = err
			return
		}
		nativeArchive = a
	})
	return nativeArchive, nativeArchiveErr
}

// OpenTestArchiveCached opens an inner PVF at an explicit path through the same
// process-wide metadata cache the native helpers use, so test packages that
// resolve their own override path (for example DFO_LOOT_PVF) reuse the parsed
// directory instead of re-parsing the whole archive. The caller owns the
// returned archive (unlike OpenNativeArchive's shared one).
func OpenTestArchiveCached(path, sha256 string) (*pvf.Archive, error) {
	parser, err := nativeParserIdentity()
	if err != nil {
		parser = ""
	}
	return pvf.OpenReadOnlyCached(
		pvf.Options{Path: path, MaxBytes: 1024 * 1024 * 1024},
		sha256,
		nativeCacheDir(),
		parser,
	)
}

// CloseNativeArchive releases the process-wide native test archive. TestMain
// calls it once the package's tests finish; it is safe when nothing opened and
// safe to call more than once.
func CloseNativeArchive() error {
	if nativeArchive == nil {
		return nil
	}
	return nativeArchive.Close()
}

// OpenNativeArchive returns the process-wide inner PVF named by
// DFO_PVF_CORE_TEST_ARCHIVE. It skips when the environment does not provide
// one, so a bare checkout stays runnable. Callers can override
// DFO_PVF_CORE_TEST_SHA256 for the source-pin tests. The returned archive is
// shared for the whole test binary; do not close it from individual tests.
func OpenNativeArchive(t *testing.T) *pvf.Archive {
	t.Helper()
	requireNativeArchive(t)
	a, err := openNativeArchive()
	if err != nil {
		t.Fatalf("open native PVF: %v", err)
	}
	return a
}

// LoadNativeDungeons imports exactly the requested source dungeons and the maps
// they reference. ValidateDungeons applies the same runtime checks as the
// historical catalog.LoadDungeons snapshot. Errors abort via t.Fatal so call
// sites stay single-line.
func LoadNativeDungeons(t *testing.T, ids ...uint32) DungeonCatalog {
	t.Helper()
	c, err := ImportDungeons(OpenNativeArchive(t), ids)
	if err != nil {
		t.Fatal(err)
	}
	c, err = ValidateDungeons(c)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

var (
	nativeFullOnce sync.Once
	nativeFull     DungeonCatalog
	nativeFullErr  error
)

// LoadNativeFullDungeons imports the archive's complete dungeon list bound to
// its world gate references, matching the scope of the old full export. The
// deterministic result is computed once per test binary and reused by every
// call in the package. Errors abort via t.Fatal so call sites stay single-line.
func LoadNativeFullDungeons(t *testing.T) DungeonCatalog {
	t.Helper()
	requireNativeArchive(t)
	nativeFullOnce.Do(func() {
		a, err := openNativeArchive()
		if err != nil {
			nativeFullErr = err
			return
		}
		world, err := ImportWorld(a)
		if err != nil {
			nativeFullErr = err
			return
		}
		nativeFull, nativeFullErr = ImportFullDungeons(a, world, nil)
	})
	if nativeFullErr != nil {
		t.Fatal(nativeFullErr)
	}
	return nativeFull
}

// LoadNativeLootTables imports only the drop-rule tables (Rules, DropGroups,
// DungeonDropInfo) once per test binary. It is the narrow counterpart of
// ImportLoot for routing/boundary tests that never read LootCatalog.Items, so
// they skip the per-stackable scan over the whole archive.
func LoadNativeLootTables(t *testing.T) LootCatalog {
	t.Helper()
	requireNativeArchive(t)
	c, err := importLootDropTables(OpenNativeArchive(t), 130)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
