package dungeon

import (
	"dfolan/internal/catalog"
	"fmt"
)

// hellPartyMaze applies the DGN seal room to this run only. CMD16 Mode=1
// and NOTI28 Hell XY use the same native path for every dungeon; the PVF
// defines membership and coordinates, rather than a server dungeon ID list.
func hellPartyMaze(maze catalog.DungeonMaze, hell catalog.DungeonHellParty) (catalog.DungeonMaze, error) {
	position := hell.SealPosition
	if position[0] == 255 || position[1] == 255 {
		return maze, fmt.Errorf("Hell Party seal coordinate is absent")
	}
	seal := catalog.DungeonRoom{X: position[0], Y: position[1], Map: hell.SealMap}
	found := false
	for _, room := range maze.Rooms {
		if [2]byte{room.X, room.Y} != position {
			continue
		}
		// Dedicated Hell dungeons already put their seal map at the start
		// or boss (sometimes both). Keep the source boss marker. A quest
		// boss on a different map must never be replaced by a Hell room.
		if (position == maze.Start || position == maze.Boss) && room.Map != hell.SealMap {
			return maze, fmt.Errorf("Hell Party seal room conflicts with source start or boss")
		}
		seal.Boss = room.Boss
		found = true
		break
	}
	if !found && (position == maze.Start || position == maze.Boss) {
		return maze, fmt.Errorf("Hell Party seal room conflicts with source start or boss")
	}
	maze.Rooms = append([]catalog.DungeonRoom(nil), maze.Rooms...)
	if found {
		for i, room := range maze.Rooms {
			if [2]byte{room.X, room.Y} == position {
				maze.Rooms[i] = seal
			}
		}
	} else {
		maze.Rooms = append(maze.Rooms, seal)
	}
	// Require a path through the source rooms; do not fabricate connecting
	// rooms for an incompatible story maze. Trombe's verified quest route
	// reaches the seal through its boss coordinate.
	rooms := make(map[[2]byte]bool, len(maze.Rooms))
	for _, room := range maze.Rooms {
		rooms[[2]byte{room.X, room.Y}] = true
	}
	seen := map[[2]byte]bool{maze.Start: true}
	queue := [][2]byte{maze.Start}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == position {
			break
		}
		for _, delta := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			x, y := int(current[0])+delta[0], int(current[1])+delta[1]
			if x < 0 || y < 0 || x >= 255 || y >= 255 {
				continue
			}
			next := [2]byte{byte(x), byte(y)}
			if rooms[next] && !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	if !seen[position] {
		return maze, fmt.Errorf("Hell Party seal room is unreachable in requested source maze")
	}
	if maze.Size[0] <= position[0] {
		maze.Size[0] = position[0] + 1
	}
	if maze.Size[1] <= position[1] {
		maze.Size[1] = position[1] + 1
	}
	// Story layers at the replaced coordinate belong to the original
	// map. Room revisits must retain the selected native seal map.
	layers := make([]catalog.DungeonLayer, 0, len(maze.Layers))
	for _, layer := range maze.Layers {
		if layer.Position != position {
			layers = append(layers, layer)
		}
	}
	maze.Layers = layers
	return maze, nil
}
