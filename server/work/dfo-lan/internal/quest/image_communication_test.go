package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestImageCommunicationSelectsOnlyMatchingPendingQuest(t *testing.T) {
	c := catalog.QuestCatalog{Quests: map[uint32]catalog.QuestDefinition{
		3741: {Kind: "[meet npc]", ObjectiveCells: []pvf.Token{{Type: 0, Value: 2001}}},
	}}
	c.Source.Checksum = imageCommunicationSourceChecksum
	state := QuestState{ID: 3741, Status: "accepted", Progress: 1,
		ConfigVersion: c.Source.SaveIdentity(), ProgressModel: SingleMeetNPC}
	check := func(states []QuestState, wantNPC uint32) {
		t.Helper()
		qid, npc, err := imageCommunicationTarget(c, states)
		if err != nil || npc != wantNPC || (npc != 0 && qid != 3741) {
			t.Fatalf("target (%d,%d,%v), want NPC %d", qid, npc, err, wantNPC)
		}
	}
	check([]QuestState{state}, 2001)
	state.Progress = 0
	check([]QuestState{state}, 0)
	state.Progress = 1
	state.Status = "completed"
	check([]QuestState{state}, 0)
	state.Status = "accepted"
	c.Quests[3741] = catalog.QuestDefinition{Kind: "[meet npc]", ObjectiveCells: []pvf.Token{{Type: 0, Value: 76}}}
	check([]QuestState{state}, 0)
}

func TestImageCommunicationRejectsDifferentCatalogSource(t *testing.T) {
	_, _, err := imageCommunicationTarget(catalog.QuestCatalog{}, nil)
	if err == nil {
		t.Fatal("unverified PVF source accepted")
	}
}
