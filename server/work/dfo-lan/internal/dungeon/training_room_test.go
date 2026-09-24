package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"path/filepath"
	"testing"
)

func TestSelectTrainingRoomFromSource(t *testing.T) {
	c, err := catalog.LoadDungeons(filepath.Join("..", "..", "configs", "dungeons.training-room.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []byte{0, 1} {
		for _, flag := range []byte{0, 1} {
			r := protocol.DungeonSelection{ID: 5000, Mode: mode, Flag: flag, Party: 65535}
			s, err := SelectTrainingRoom(c, r, 1)
			if err != nil || s.Room.Map != 36250 || !s.Definition.NoFatigue {
				t.Fatalf("mode=%d flag=%d session=%+v err=%v", mode, flag, s, err)
			}
		}
	}
	for _, change := range []func(*protocol.DungeonSelection){
		func(r *protocol.DungeonSelection) { r.Mode = 2 },
		func(r *protocol.DungeonSelection) { r.Flag = 2 },
		func(r *protocol.DungeonSelection) { r.Quest = 10100 },
		func(r *protocol.DungeonSelection) { r.Party = 2 },
	} {
		r := protocol.DungeonSelection{ID: 5000, Party: 65535}
		change(&r)
		if _, err := SelectTrainingRoom(c, r, 1); err == nil {
			t.Fatalf("unsupported request accepted: %+v", r)
		}
	}
}
