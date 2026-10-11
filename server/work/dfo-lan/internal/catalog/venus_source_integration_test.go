package catalog_test

import (
	"dfolan/internal/catalog"
	"dfolan/internal/legion"
	"os"
	"testing"
)

func TestVenusSourcePotionIntegrationAudit(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("current native PVF required")
	}
	a, err := catalog.OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	script, err := catalog.ResolveScript(a, catalog.StackableDungeonLimitPath)
	if err != nil {
		t.Fatal(err)
	}
	limits, err := catalog.ParseStackableDungeonLimits(script)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range legion.VenusStageDungeons {
		limit, ok := limits.Dungeons[id]
		if !ok {
			t.Fatalf("venus dungeon %d missing from %s", id, script.Path)
		}
		t.Logf("venus dungeon=%d potion_limit=%d source=%s sha256=%s", id, limit, script.Path, script.SHA256)
	}
}
