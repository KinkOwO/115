package gamedata

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/npcpresence"
	"dfolan/internal/quest"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestRuntimeDetailsLocalArchiveParity(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete runtime details parity")
	}
	a, err := catalog.OpenTestArchiveCached(p, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	chars, err := catalog.ImportCharacters(a)
	if err != nil {
		t.Fatal(err)
	}
	skills, err := character.ImportLearningCatalog(a, chars)
	if err != nil {
		t.Fatal(err)
	}
	defer skills.Close()
	beforeSkills := append([]character.LearningDefinition(nil), skills.Rows...)
	quests, err := catalog.ImportQuests(a)
	if err != nil {
		t.Fatal(err)
	}
	beforeQuests := quests
	beforeQuests.Quests = make(map[uint32]catalog.QuestDefinition, len(quests.Quests))
	for id, q := range quests.Quests {
		beforeQuests.Quests[id] = q
	}
	beforeIndex := quest.BuildIndex(beforeQuests)
	world, err := catalog.ImportWorld(a)
	if err != nil {
		t.Fatal(err)
	}
	town, issues := catalog.TownArrivalSceneWhitelist(beforeQuests, world)
	index, err := catalog.ImportItemIndex(a)
	if err != nil {
		t.Fatal(err)
	}
	items := catalog.LootCatalog{Source: a.Snapshot(), Items: map[uint32]catalog.LootItem{}}
	if err = items.SupplementItemIndex(index); err != nil {
		t.Fatal(err)
	}
	if err = skills.EnableRuntimeDetails(a); err != nil {
		t.Fatal(err)
	}
	if err = quests.EnableRuntimeDetails(a); err != nil {
		t.Fatal(err)
	}
	defer quests.Close()
	if err = items.EnableRuntimeDetails(a, index); err != nil {
		t.Fatal(err)
	}
	defer items.CloseDetails()
	if !reflect.DeepEqual(beforeIndex, quest.BuildIndex(quests)) {
		t.Fatal("quest availability/objective index changed")
	}
	gotTown, gotIssues := catalog.TownArrivalSceneWhitelist(quests, world)
	npcBefore, beforeErr := npcpresence.NewIndex(world, beforeQuests)
	npcAfter, afterErr := npcpresence.NewIndex(world, quests)
	if (beforeErr == nil) != (afterErr == nil) || beforeErr != nil && beforeErr.Error() != afterErr.Error() || !reflect.DeepEqual(npcBefore, npcAfter) {
		t.Fatal("quest NPC visibility graph changed", beforeErr, afterErr, Compare(npcBefore, npcAfter, 5))
	}
	t.Log("quest index, town arrivals and complete NPC visibility graph match")
	if !reflect.DeepEqual(town, gotTown) || !reflect.DeepEqual(issues, gotIssues) {
		t.Fatal("town arrival scene admission changed")
	}
	if err = a.CompactRuntimeStrings(); err != nil {
		t.Fatal(err)
	}
	var itemCount int
	begin := time.Now()
	for id, r := range index.Items {
		if r.Kind != "stackable" || id == 0 {
			continue
		}
		eager, err := catalog.ResolveScript(a, r.Path)
		if err != nil {
			t.Fatal(err)
		}
		lazy, err := items.ItemScript(id)
		if err != nil || !reflect.DeepEqual(eager, lazy) {
			t.Fatalf("item %d differs: %v", id, err)
		}
		itemCount++
	}
	t.Logf("all stackable details=%d compare=%s", itemCount, time.Since(begin))
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	for _, eager := range beforeSkills {
		lazy, ok, err := skills.Definition(eager.Job, eager.ID)
		if err != nil || !ok || !reflect.DeepEqual(eager, lazy) {
			t.Fatalf("skill job=%d id=%d differs: %v", eager.Job, eager.ID, err)
		}
	}
	for id, eager := range beforeQuests.Quests {
		lazy, err := quests.Definition(id)
		if err != nil || !reflect.DeepEqual(eager, lazy) {
			t.Fatalf("quest %d differs: %v", id, err)
		}
	}
	if err = skills.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err = skills.Definition(beforeSkills[0].Job, beforeSkills[0].ID); err == nil {
		t.Fatal("closed skill source allowed")
	}
	if err = quests.Close(); err != nil {
		t.Fatal(err)
	}
	for id, q := range beforeQuests.Quests {
		if q.Script.Path != "" && len(q.Script.SHA256) == 64 {
			if _, err = quests.Definition(id); err == nil {
				t.Fatal("closed quest source allowed")
			}
			break
		}
	}
	if err = items.CloseDetails(); err != nil {
		t.Fatal(err)
	}
	for id := range items.Items {
		if _, err = items.ItemScript(id); err == nil {
			t.Fatal("closed item source allowed")
		}
		break
	}
	t.Logf("source=%s skills=%d quests=%d town scenes=%d", index.Source.Checksum, len(beforeSkills), len(beforeQuests.Quests), len(town))
}
