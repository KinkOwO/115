package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/storage"
	"encoding/json"
	"os"
	"testing"
)

func TestSeekMeetBossClearLazyLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for lazy NPC quest completion")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 1024 * 1024 * 1024}, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	quests, err := catalog.ImportQuests(a)
	if err != nil {
		t.Fatal(err)
	}
	dungeons, err := catalog.ImportRuntimeDungeons(a, []uint32{71})
	if err != nil {
		t.Fatal(err)
	}
	defer dungeons.CloseMapSource()
	if err = a.CompactRuntimeStrings(); err != nil {
		t.Fatal(err)
	}
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	s := &Service{Catalog: quests, Dungeons: &dungeons}
	def := dungeons.Dungeons[71]
	run := &dungeon.Session{Definition: def}
	for _, maze := range def.Mazes {
		if maze.Quest == 3634 {
			run.Maze = maze
			for _, room := range maze.Rooms {
				if room.Boss {
					run.Room = room
				}
			}
		}
	}
	role := storage.Character{State: json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","items":[{"slot":95,"Template":10164777,"Amount":1}]}}`)}
	if !s.seekMeetBossClearMatch(s.Index().Entries[3634], role, run) {
		t.Fatal("cold map read lost source NPC quest completion")
	}
	if dungeons.Maps[run.Room.Map].Cells != nil {
		t.Fatal("quest completion populated map metadata tokens")
	}
}
