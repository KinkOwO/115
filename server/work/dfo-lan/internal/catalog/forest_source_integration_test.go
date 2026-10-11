package catalog_test

import (
	"dfolan/internal/catalog"
	"dfolan/internal/legion"
	"os"
	"testing"
)

func TestForestSourceIntegrationAudit(t *testing.T) {
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
	for _, ids := range [][3]uint32{legion.ForestStageDungeons, legion.ForestHardStageDungeons} {
		for _, id := range ids {
			limit, ok := limits.Dungeons[id]
			if !ok {
				t.Fatalf("forest dungeon %d missing from %s [dungeon list]", id, script.Path)
			}
			t.Logf("forest dungeon=%d potion_limit=%d source=%s sha256=%s", id, limit, script.Path, script.SHA256)
		}
	}
	contents, err := catalog.ImportLegionContents(a)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ForestOfAwakeningNormal", "ForestOfAwakeningHard"} {
		content, ok := contents.Contents[name]
		if !ok {
			t.Fatalf("forest content absent: %s", name)
		}
		t.Logf("content=%s phase_rewards=%d native_reward_rows=%d native_stage_rows=%d missing=%v", name, content.PhaseReward, len(content.Rewards), len(content.Dungeons), content.Missing)
	}
}
