package gamedata

import (
	"dfolan/internal/quest"
	"os"
	"reflect"
	"testing"
)

func TestQuestSkipNativeSourceAndRuntimeProjection(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native source validation")
	}
	s, err := Open(Options{Mode: PVF, ArchivePath: path, ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256"), DerivedCacheDir: "-"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c, err := s.Quests("")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	rules, err := s.QuestSkipRules()
	if err != nil {
		t.Fatal(err)
	}
	if rules.Source.SaveIdentity() != c.Source.SaveIdentity() {
		t.Fatal("skip rules did not use the quest source")
	}
	before := map[uint32]bool{}
	for id, d := range c.Quests {
		if quest.RewardableOnForceClear(d) {
			before[id] = true
		}
	}
	if len(before) == 0 || len(rules.Blocked) == 0 || len(rules.Items) == 0 {
		t.Fatal("empty native skip rules")
	}
	if err = s.EnableRuntimeDetails(&c, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	index := quest.BuildIndex(c)
	after := map[uint32]bool{}
	mainline := 0
	for id, en := range index.Entries {
		if en.PaysOnForceClear {
			after[id] = true
		}
		if quest.MainlineGrades().Allows(en.Grade) {
			mainline++
		}
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("runtime clipping lost source reward flags: %v vs %v", before, after)
	}
	t.Logf("source=%s quests=%d mainline=%d blocked=%d key_items=%v marked_rewards=%v", rules.Source.Checksum, len(c.Quests), mainline, len(rules.Blocked), rules.Items, after)
}
