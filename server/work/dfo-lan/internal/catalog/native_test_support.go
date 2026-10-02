package catalog

import (
	"os"
	"testing"

	"dfolan/internal/catalog/pvf"
)

// OpenNativeArchive opens the inner PVF named by DFO_PVF_CORE_TEST_ARCHIVE. They
// are skipped when the environment does not provide one, so a bare checkout
// stays runnable. Callers can override DFO_PVF_CORE_TEST_SHA256 for the
// source-pin tests.
func OpenNativeArchive(t *testing.T) *pvf.Archive {
	t.Helper()
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native PVF source parity")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 1024 * 1024 * 1024}, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatalf("open native PVF: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
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

// LoadNativeFullDungeons imports the archive's complete dungeon list bound to
// its world gate references, matching the scope of the old full export.
// Errors abort via t.Fatal so call sites stay single-line.
func LoadNativeFullDungeons(t *testing.T) DungeonCatalog {
	t.Helper()
	a := OpenNativeArchive(t)
	world, err := ImportWorld(a)
	if err != nil {
		t.Fatal(err)
	}
	c, err := ImportFullDungeons(a, world, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}