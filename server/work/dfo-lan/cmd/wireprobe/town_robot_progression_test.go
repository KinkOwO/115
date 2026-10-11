package main

import (
	"bytes"
	"dfolan/internal/character"
	"encoding/json"
	"os"
	"testing"
)

func TestTownRobotProgressionRequiresLeaveAndPreservesArchive(t *testing.T) {
	f := robotManagementFixture(t)
	m := f.manager
	p := m.characters.Catalog.Professions[0]
	p.AdvancementGrowth = map[byte]map[string]float32{1: {"hp max": 1}}
	m.characters.Catalog.Professions[0] = p
	before, _ := os.ReadFile(m.path)
	f.parties = map[*lanPeer]*townRobotParty{nil: {bots: []*townRobot{{recordID: 1}}}}
	for _, action := range []string{"advancement", "awakening", "skills"} {
		if err := f.changeRobot(townRobotChange{ID: 1, Revision: 1, Action: action, Advancement: 1, Awakening: 1}); err == nil {
			t.Fatalf("in-party %s accepted", action)
		}
	}
	after, _ := os.ReadFile(m.path)
	if !bytes.Equal(before, after) || m.records[1].Revision != 1 {
		t.Fatal("rejected progression changed archive")
	}
	f.parties = nil
	if err := f.changeRobot(townRobotChange{ID: 1, Revision: 1, Action: "advancement", Advancement: 1}); err != nil {
		t.Fatal(err)
	}
	var state character.State
	json.Unmarshal(m.records[1].Role.State, &state)
	if state.Advancement != 1 || state.Awakening != 0 || m.records[1].Revision != 2 {
		t.Fatal("advancement not persisted")
	}
	if err := f.changeRobot(townRobotChange{ID: 1, Revision: 2, Action: "advancement", Advancement: 2}); err == nil {
		t.Fatal("unsafe branch switch accepted")
	}
}
