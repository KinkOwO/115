package dungeon

import "dfolan/internal/catalog"

// Tianqing three-fix port: only source-bound post-boss transitions and
// confirmed-death settlement. No automatic movement or timed completion.

func (s *Session) layerPendingUnvisited(pos [2]byte) bool {
	if s == nil {
		return false
	}
	for _, layer := range s.Maze.Layers {
		if layer.Position != pos {
			continue
		}
		for _, m := range layer.Maps {
			if _, visited := s.Visited[m]; !visited {
				return true
			}
		}
	}
	return false
}

func (s *Session) adjacentUnvisitedRoom(target [2]byte) bool {
	if s == nil {
		return false
	}
	// 只对**没有**层图的副本生效：副本若用 `layered` 表达「boss 后过场」（maze.Layers
	// 非空，如苏醒之森 100004977），普通相邻格一律不放行 —— 保持 2026-09-28 的既有
	// 语义（那里的 boss 后该走层图格，别的普通相邻格必须继续被拒）。
	if len(s.Maze.Layers) > 0 {
		return false
	}
	dx := int(target[0]) - int(s.Room.X)
	dy := int(target[1]) - int(s.Room.Y)
	if dx*dx+dy*dy != 1 {
		return false
	}
	for _, room := range s.Maze.Rooms {
		if [2]byte{room.X, room.Y} != target {
			continue
		}
		if _, visited := s.Visited[room.Map]; visited {
			return false
		}
		return true
	}
	return false
}

func bossRoomWithSpecialPassive(c catalog.DungeonCatalog, maze catalog.DungeonMaze, room catalog.DungeonRoom) bool {
	if [2]byte{room.X, room.Y} != maze.Boss || room.Map == 0 {
		return false
	}
	script, err := c.MapScript(room.Map)
	if err != nil {
		return false
	}
	return hasSpecialPassiveObject(script)
}

func hasSpecialPassiveObject(script catalog.ScriptRecord) bool {
	active := false
	for _, cell := range script.Cells {
		if cell.Type == 3 {
			active = cell.Text == "[special passive object]"
			continue
		}
		if active && cell.Type == 0 {
			return true
		}
	}
	return false
}
