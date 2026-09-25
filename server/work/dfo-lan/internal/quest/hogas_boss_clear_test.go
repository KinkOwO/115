package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/storage"
	"encoding/json"
	"testing"
)

func TestHogasBossClearRequiresOwnedSourceAndCure(t *testing.T) {
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
	role := storage.Character{State: json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","items":[{"slot":95,"Template":10164777,"Amount":1}]}}`)}
	if !s.hogasBossClearMatch(en, role, run) {
		t.Fatal("owned quest boss with cure and source NPC did not match")
	}
	withoutCure := role
	withoutCure.State = json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1"}}`)
	if s.hogasBossClearMatch(en, withoutCure, run) {
		t.Fatal("boss clear without cure matched")
	}
	foreign := *run
	foreign.Maze.Quest = 3635
	if s.hogasBossClearMatch(en, role, &foreign) {
		t.Fatal("foreign quest maze matched")
	}
	foreign = *run
	foreign.Room.Map++
	if s.hogasBossClearMatch(en, role, &foreign) {
		t.Fatal("foreign boss map matched")
	}
	changed := dungeons
	changed.Maps = make(map[uint32]catalog.ScriptRecord, len(dungeons.Maps))
	for id, script := range dungeons.Maps {
		changed.Maps[id] = script
	}
	delete(changed.Maps, 91798)
	s.Dungeons = &changed
	if s.hogasBossClearMatch(en, role, run) {
		t.Fatal("boss map without source NPC evidence matched")
	}
}
