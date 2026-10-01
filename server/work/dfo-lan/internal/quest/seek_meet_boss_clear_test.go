package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"encoding/json"
	"testing"
)

func TestSeekMeetBossClearRequiresOwnedSourceAndItems(t *testing.T) {
	quests, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	dungeons, err := catalog.LoadDungeons("../../configs/dungeons.full.json")
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{Catalog: quests, Dungeons: &dungeons}
	en := s.Index().Entries[3634]
	def := dungeons.Dungeons[71]
	var maze catalog.DungeonMaze
	for _, row := range def.Mazes {
		if row.Quest == 3634 {
			maze = row
		}
	}
	if maze.Quest == 0 {
		t.Fatal("quest 3634 source maze absent")
	}
	run := &dungeon.Session{Definition: def, Maze: maze}
	for _, room := range maze.Rooms {
		if room.Boss {
			run.Room = room
		}
	}
	role := character.Character{State: json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","items":[{"slot":95,"Template":10164777,"Amount":1}]}}`)}
	if !s.seekMeetBossClearMatch(en, role, run) {
		t.Fatal("owned quest boss with cure and source NPC did not match")
	}
	withoutCure := role
	withoutCure.State = json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1"}}`)
	if s.seekMeetBossClearMatch(en, withoutCure, run) {
		t.Fatal("boss clear without cure matched")
	}
	foreign := *run
	foreign.Maze.Quest = 3635
	if s.seekMeetBossClearMatch(en, role, &foreign) {
		t.Fatal("foreign quest maze matched")
	}
	foreign = *run
	foreign.Room.Map++
	if s.seekMeetBossClearMatch(en, role, &foreign) {
		t.Fatal("foreign boss map matched")
	}
	changed := dungeons
	changed.Maps = make(map[uint32]catalog.ScriptRecord, len(dungeons.Maps))
	for id, script := range dungeons.Maps {
		changed.Maps[id] = script
	}
	delete(changed.Maps, 91798)
	s.Dungeons = &changed
	if s.seekMeetBossClearMatch(en, role, run) {
		t.Fatal("boss map without source NPC evidence matched")
	}
	s.Dungeons = &dungeons
	// A future source quest with the same objective and its own maze must use
	// the same rule without depending on the historical quest ID.
	const successor = uint32(60001)
	newQuests := quests
	newQuests.Quests = make(map[uint32]catalog.QuestDefinition, len(quests.Quests)+1)
	for id, q := range quests.Quests {
		newQuests.Quests[id] = q
	}
	newQuest := quests.Quests[3634]
	newQuest.ID = successor
	newQuests.Quests[successor] = newQuest
	newDungeons := dungeons
	newDungeons.Dungeons = make(map[uint32]catalog.DungeonDefinition, len(dungeons.Dungeons))
	for id, definition := range dungeons.Dungeons {
		newDungeons.Dungeons[id] = definition
	}
	newDef := def
	newDef.Mazes = append([]catalog.DungeonMaze(nil), def.Mazes...)
	for i := range newDef.Mazes {
		if newDef.Mazes[i].Index == maze.Index {
			newDef.Mazes[i].Quest = uint16(successor)
		}
	}
	newDungeons.Dungeons[71] = newDef
	newRun := *run
	newRun.Definition = newDef
	newRun.Maze.Quest = uint16(successor)
	s.Catalog, s.Dungeons = newQuests, &newDungeons
	s.index = nil
	if !s.seekMeetBossClearMatch(s.Index().Entries[successor], role, &newRun) {
		t.Fatal("equivalent future quest did not match")
	}
}
