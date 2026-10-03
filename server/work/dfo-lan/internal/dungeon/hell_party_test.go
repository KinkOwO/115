package dungeon

import (
	"dfolan/internal/catalog"
	"reflect"
	"strings"
	"testing"
)

func TestHellPartyMazeSourceBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name    string
		maze    catalog.DungeonMaze
		hell    catalog.DungeonHellParty
		refusal string
	}{
		{"single native Hell room", catalog.DungeonMaze{Size: [2]byte{1, 1}, Rooms: []catalog.DungeonRoom{{Map: 7, Boss: true}}}, catalog.DungeonHellParty{SealMap: 7}, ""},
		{"native Hell boss room", catalog.DungeonMaze{Size: [2]byte{2, 1}, Boss: [2]byte{1, 0}, Rooms: []catalog.DungeonRoom{{Map: 6}, {X: 1, Map: 7, Boss: true}}}, catalog.DungeonHellParty{SealMap: 7, SealPosition: [2]byte{1, 0}}, ""},
		{"story boss cannot be replaced", catalog.DungeonMaze{Size: [2]byte{2, 1}, Boss: [2]byte{1, 0}, Rooms: []catalog.DungeonRoom{{Map: 6}, {X: 1, Map: 8, Boss: true}}}, catalog.DungeonHellParty{SealMap: 7, SealPosition: [2]byte{1, 0}}, "conflicts"},
		{"disconnected story route", catalog.DungeonMaze{Size: [2]byte{2, 1}, Boss: [2]byte{1, 0}, Rooms: []catalog.DungeonRoom{{Map: 6}, {X: 1, Map: 8, Boss: true}}}, catalog.DungeonHellParty{SealMap: 7, SealPosition: [2]byte{3, 0}}, "unreachable"},
		{"absent sentinel", catalog.DungeonMaze{}, catalog.DungeonHellParty{SealMap: 7, SealPosition: [2]byte{255, 0}}, "absent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := append([]catalog.DungeonRoom(nil), tc.maze.Rooms...)
			got, err := hellPartyMaze(tc.maze, tc.hell)
			if tc.refusal != "" {
				if err == nil || !strings.Contains(err.Error(), tc.refusal) {
					t.Fatalf("want %s, got %v", tc.refusal, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if got.Boss != tc.maze.Boss || !got.Rooms[len(got.Rooms)-1].Boss {
					t.Fatal("native Hell boss lost its completion identity")
				}
			}
			if !reflect.DeepEqual(before, tc.maze.Rooms) {
				t.Fatal("changed shared source maze")
			}
		})
	}
}

func TestHellPartyMazeKeepsSealOnRevisitWithoutChangingNormalRoute(t *testing.T) {
	maze := catalog.DungeonMaze{Size: [2]byte{3, 1}, Boss: [2]byte{2, 0}, Rooms: []catalog.DungeonRoom{{Map: 6}, {X: 1, Map: 8}, {X: 2, Map: 9, Boss: true}}, Layers: []catalog.DungeonLayer{{Position: [2]byte{1, 0}, Maps: []uint32{10}}, {Position: [2]byte{2, 0}, Maps: []uint32{11}}}}
	hell := catalog.DungeonHellParty{SealMap: 7, SealPosition: [2]byte{1, 0}}
	got, err := hellPartyMaze(maze, hell)
	if err != nil {
		t.Fatal(err)
	}
	if got.Rooms[1].Map != 7 || len(got.Layers) != 1 || got.Layers[0].Position != maze.Boss {
		t.Fatal("Hell seal retains an unrelated story layer or loses boss layer")
	}
	if maze.Rooms[1].Map != 8 || len(maze.Layers) != 2 {
		t.Fatal("normal source route changed")
	}
}
