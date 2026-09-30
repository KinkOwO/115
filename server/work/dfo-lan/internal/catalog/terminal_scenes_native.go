package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"path"
	"sort"
	"strings"
)

type TerminalSceneOverlay struct {
	Source struct {
		Checksum string `json:"checksum"`
	} `json:"source"`
	Scenes []DungeonTerminalScene `json:"terminal_scenes"`
}

// ImportTerminalScenes uses the same quest/maze/ACT/CMT chain as the exporter.
// It does not infer a closing scene from a packet or a filename.
func ImportTerminalScenes(a *pvf.Archive, d DungeonCatalog, quests QuestCatalog) (TerminalSceneOverlay, error) {
	var out TerminalSceneOverlay
	if a == nil || d.Source.Checksum == "" || a.Snapshot().Checksum != d.Source.Checksum || quests.Source.Checksum != d.Source.Checksum {
		return out, fmt.Errorf("terminal scene PVF/quest/dungeon source mismatch")
	}
	index, err := ReadScript(a, "list/cinematic.lst")
	if err != nil {
		return out, err
	}
	rows, err := ParseIndex(index.Cells)
	if err != nil {
		return out, err
	}
	paths := map[uint32]string{}
	for _, row := range rows {
		paths[row.ID] = row.Path
	}
	var scenes []DungeonTerminalScene
	seen := map[[3]uint32]bool{}
	for _, def := range d.Dungeons {
		for _, maze := range def.Mazes {
			quest, ok := quests.Quests[uint32(maze.Quest)]
			if !ok || quest.Kind != "[clear map]" || len(quest.ObjectiveCells) != 1 || quest.ObjectiveCells[0].Type != 0 || quest.ObjectiveCells[0].Value <= 0 {
				continue
			}
			for _, layer := range maze.Layers {
				if len(layer.Maps) == 0 {
					continue
				}
				objective := uint32(quest.ObjectiveCells[0].Value)
				boss := false
				for _, room := range maze.Rooms {
					if room.Boss && room.Map == objective && [2]byte{room.X, room.Y} == layer.Position {
						boss = true
					}
				}
				if !boss {
					continue
				}
				final := layer.Maps[len(layer.Maps)-1]
				m, ok := d.Maps[final]
				if !ok {
					continue
				}
				var actionPath string
				for i, t := range m.Cells {
					if t.Type == 3 && t.Text == "[basic action]" && i+1 < len(m.Cells) && m.Cells[i+1].Type == 6 {
						actionPath = path.Join(path.Dir(m.Path), strings.ReplaceAll(m.Cells[i+1].Text, "\\", "/"))
						break
					}
				}
				if actionPath == "" {
					continue
				}
				action, err := ResolveScript(a, actionPath)
				if err != nil {
					continue
				}
				for i, t := range action.Cells {
					if t.Type != 3 || t.Text != "[CINEMATIC]" || i+1 >= len(action.Cells) || action.Cells[i+1].Type != 0 {
						continue
					}
					cmtPath := paths[uint32(action.Cells[i+1].Value)]
					if cmtPath == "" {
						continue
					}
					cmt, err := ResolveScript(a, cmtPath)
					if err != nil {
						continue
					}
					xmin, xmax, ymin, ymax, ok := changeMapArea(cmt.Cells)
					// Zero and 255 coordinates are source sentinels, not witnessed
					// positions in a final layer.
					if !ok || xmin == 0 || ymin == 0 || xmax == 255 || ymax == 255 {
						continue
					}
					key := [3]uint32{def.ID, uint32(maze.Index), final}
					if seen[key] {
						return out, fmt.Errorf("multiple closing scenes for %v", key)
					}
					seen[key] = true
					scenes = append(scenes, DungeonTerminalScene{
						Source: d.Source.Checksum, Dungeon: def.ID, Maze: maze.Index, Quest: maze.Quest, Position: layer.Position,
						ObjectiveMap: objective, FinalMap: final, XMin: xmin, XMax: xmax, YMin: ymin, YMax: ymax,
						ObjectiveCinematicDestroyTemplate: cinematicDestroyedObjective(a, paths, d.Maps[objective], objective),
						DungeonSHA256:                     def.Script.SHA256, MapSHA256: m.SHA256, ActionSHA256: action.SHA256,
						CinematicSHA256: cmt.SHA256, CinematicPath: cmt.Path,
					})
				}
			}
		}
	}
	sort.Slice(scenes, func(i, j int) bool {
		if scenes[i].Dungeon != scenes[j].Dungeon {
			return scenes[i].Dungeon < scenes[j].Dungeon
		}
		return scenes[i].Maze < scenes[j].Maze
	})

	out.Source.Checksum = d.Source.Checksum
	out.Scenes = scenes
	return out, nil
}

// cinematicDestroyedObjective recognizes a sole source boss removed by its
// own map action. That removal has no corresponding native monster-death CMD.
func cinematicDestroyedObjective(a *pvf.Archive, paths map[uint32]string, m ScriptRecord, mapID uint32) uint32 {
	var template uint32
	count := 0
	var actionPath string
	for i, t := range m.Cells {
		if t.Type == 3 && t.Text == "[monster]" {
			count++
			boss := false
			for j := i + 1; j < len(m.Cells) && m.Cells[j].Text != "[/monster]"; j++ {
				boss = boss || m.Cells[j].Text == "[boss]"
			}
			if boss && i+1 < len(m.Cells) && m.Cells[i+1].Type == 0 && m.Cells[i+1].Value > 0 {
				template = uint32(m.Cells[i+1].Value)
			}
		}
		if t.Type == 3 && t.Text == "[basic action]" && i+1 < len(m.Cells) && m.Cells[i+1].Type == 6 {
			actionPath = path.Join(path.Dir(m.Path), strings.ReplaceAll(m.Cells[i+1].Text, "\\", "/"))
		}
	}
	if count != 1 || template == 0 || actionPath == "" {
		return 0
	}
	action, err := ResolveScript(a, actionPath)
	if err != nil {
		return 0
	}
	for i, t := range action.Cells {
		if t.Type != 3 || t.Text != "[CINEMATIC]" || i+1 >= len(action.Cells) || action.Cells[i+1].Type != 0 {
			continue
		}
		cmtPath := paths[uint32(action.Cells[i+1].Value)]
		if cmtPath == "" {
			continue
		}
		cmt, err := ResolveScript(a, cmtPath)
		if err != nil || !cinematicDestroysSoleMonster(cmt.Cells, mapID) {
			continue
		}
		return template
	}
	return 0
}

func cinematicDestroysSoleMonster(cells []pvf.Token, mapID uint32) bool {
	mapMatched := false
	actorZero := false
	for i, t := range cells {
		if t.Type != 3 {
			continue
		}
		if t.Text == "[MAP]" && i+1 < len(cells) && cells[i+1].Type == 0 {
			mapMatched = uint32(cells[i+1].Value) == mapID
		}
		if t.Text == "[ACTOR]" {
			actorZero = false
			monster, indexZero := false, false
			for j := i + 1; j < len(cells) && cells[j].Text != "[/ACTOR]"; j++ {
				if cells[j].Text == "[TYPE]" && j+1 < len(cells) && cells[j+1].Text == "[MONSTER]" {
					monster = true
				}
				if cells[j].Text == "[INDEX]" && j+1 < len(cells) && cells[j+1].Type == 0 && cells[j+1].Value == 0 {
					indexZero = true
				}
			}
			actorZero = monster && indexZero
		}
		if t.Text == "[/SCENE]" {
			actorZero = false
		}
		if actorZero && t.Text == "[DESTROY]" && i+1 < len(cells) {
			for j := i + 1; j < len(cells) && cells[j].Text != "[/DESTROY]"; j++ {
				if cells[j].Text == "[IS REWARD]" && j+1 < len(cells) && cells[j+1].Type == 0 && cells[j+1].Value == 1 {
					return mapMatched
				}
			}
		}
	}
	return false
}

func changeMapArea(c []pvf.Token) (uint16, uint16, uint16, uint16, bool) {
	var values [4]uint16
	var seen [4]bool
	active := false
	for i := 0; i < len(c); i++ {
		t := c[i]
		if t.Type != 3 {
			continue
		}
		if t.Text == "[CHANGE MAP]" {
			active = true
			values = [4]uint16{}
			seen = [4]bool{}
			continue
		}
		if !active {
			continue
		}
		if t.Text == "[/CHANGE MAP]" {
			active = false
			continue
		}
		which := -1
		switch t.Text {
		case "[X MIN]":
			which = 0
		case "[X MAX]":
			which = 1
		case "[Y MIN]":
			which = 2
		case "[Y MAX]":
			which = 3
		}
		if which < 0 {
			continue
		}
		if i+1 >= len(c) || c[i+1].Type != 0 || c[i+1].Value < 0 || c[i+1].Value > 65535 {
			return 0, 0, 0, 0, false
		}
		values[which] = uint16(c[i+1].Value)
		seen[which] = true
	}
	for _, v := range seen {
		if !v {
			return 0, 0, 0, 0, false
		}
	}
	if values[0] > values[1] || values[2] > values[3] {
		return 0, 0, 0, 0, false
	}
	return values[0], values[1], values[2], values[3], true
}
