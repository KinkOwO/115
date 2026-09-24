package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"fmt"
	"strings"
)

var trainingRoomIDs = map[uint32]bool{
	5000: true, 100003314: true, 100003651: true, 100003652: true,
}

func IsTrainingRoom(c catalog.DungeonCatalog, id uint32) bool {
	d, ok := c.Dungeons[id]
	if !ok || !trainingRoomIDs[id] {
		return false
	}
	path := strings.ToLower(strings.ReplaceAll(d.Script.Path, "\\", "/"))
	return strings.Contains(path, "/poongjintrainingroom/")
}

func SelectTrainingRoom(c catalog.DungeonCatalog, r protocol.DungeonSelection, level byte) (*Session, error) {
	if !IsTrainingRoom(c, r.ID) {
		return nil, fmt.Errorf("dungeon is not a source training room")
	}
	d := c.Dungeons[r.ID]
	if uint32(level) < d.MinimumLevel {
		return nil, fmt.Errorf("training room minimum level not met")
	}
	if r.Difficulty > 5 || r.Extra != 0 || r.Mode > 1 || r.Flag > 1 ||
		(r.Party != 1 && r.Party != 65535) || r.Reserved != 0 || r.Tail != 0 ||
		r.Quest != 0 || r.Options != [2]byte{} || r.Event != 0 {
		return nil, fmt.Errorf("unsupported training room option")
	}
	if len(d.Mazes) != 1 || d.Mazes[0].Quest != 0 || len(d.Mazes[0].Pending) != 0 {
		return nil, fmt.Errorf("training room has no resolved single source maze")
	}
	d.NoFatigue = true
	s, err := newSession(c, d, d.Mazes[0])
	if err != nil {
		return nil, err
	}
	s.Difficulty = r.Difficulty
	return s, nil
}
