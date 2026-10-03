package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"path/filepath"
	"testing"
)

func TestCapturedTrombeHellSelectionUsesSourceSealRoom(t *testing.T) {
	config := filepath.Join("..", "..", "configs")
	c, err := catalog.LoadDungeons(filepath.Join(config, "dungeons.full.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.AttachHellPartyMaps(&c, filepath.Join(config, "dungeons.hell-party-maps.json")); err != nil {
		t.Fatal(err)
	}
	body, err := hex.DecodeString("670000000300000100ffff0000000000ff180000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	r, err := protocol.DecodeDungeonSelection(body)
	if err != nil {
		t.Fatal(err)
	}
	s, err := Select(c, r, 115, map[uint16]bool{6399: true})
	if err != nil {
		t.Fatal(err)
	}
	if s.HellPosition == nil || *s.HellPosition != [2]byte{3, 0} || s.Maze.Index != 7 {
		t.Fatalf("captured request selected wrong maze or Hell coordinate: maze=%d hell=%v", s.Maze.Index, s.HellPosition)
	}
	found := false
	for _, room := range s.Maze.Rooms {
		if [2]byte{room.X, room.Y} == [2]byte{3, 0} {
			found = room.Map == 60069
		}
	}
	if !found {
		t.Fatal("Trombe's current source seal room is unavailable")
	}
	info := protocol.DungeonInfo(protocol.DungeonInfoState{ID: r.ID, Difficulty: r.Difficulty, Maze: s.Maze.Index, Boss: s.Maze.Boss, Hell: s.HellPosition})
	if info[10] != 3 || info[11] != 0 {
		t.Fatalf("NOTI28 Hell coordinate = %x", info[10:12])
	}
}
