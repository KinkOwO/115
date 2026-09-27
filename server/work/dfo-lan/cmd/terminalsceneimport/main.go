// terminalsceneimport extracts final-layer [CHANGE MAP] scene bounds from the
// current PVF for single-map clear quests. Run after rebuilding the catalogs.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
)

type questEntry struct {
	Kind      string      `json:"kind"`
	Objective []pvf.Token `json:"objective_cells"`
}

func main() {
	archivePath := flag.String("pvf", "../client-build/Script.inner.pvf", "current 115 inner PVF")
	dungeonsPath := flag.String("dungeons", "configs/dungeons.full.json", "full dungeon catalog")
	questsPath := flag.String("quests", "configs/quests.generated.json", "quest catalog")
	output := flag.String("out", "configs/dungeons.terminal-scenes.json", "source scene metadata")
	flag.Parse()
	if err := run(*archivePath, *dungeonsPath, *questsPath, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(archivePath, dungeonsPath, questsPath, output string) error {
	d, err := catalog.LoadDungeons(dungeonsPath)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(questsPath)
	if err != nil {
		return err
	}
	var quests struct {
		Source struct {
			Checksum string `json:"checksum"`
		} `json:"source"`
		Quests map[uint32]questEntry `json:"quests"`
	}
	if err := json.Unmarshal(b, &quests); err != nil {
		return err
	}
	if quests.Source.Checksum != d.Source.Checksum {
		return fmt.Errorf("quest and dungeon source mismatch")
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: archivePath, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		return err
	}
	if a.Snapshot().Checksum != d.Source.Checksum {
		return fmt.Errorf("PVF and dungeon source mismatch")
	}
	index, err := catalog.ReadScript(a, "list/cinematic.lst")
	if err != nil {
		return err
	}
	rows, err := catalog.ParseIndex(index.Cells)
	if err != nil {
		return err
	}
	paths := map[uint32]string{}
	for _, row := range rows {
		paths[row.ID] = row.Path
	}
	var scenes []catalog.DungeonTerminalScene
	seen := map[[3]uint32]bool{}
	for _, def := range d.Dungeons {
		for _, maze := range def.Mazes {
			quest, ok := quests.Quests[uint32(maze.Quest)]
			if !ok || quest.Kind != "[clear map]" || len(quest.Objective) != 1 || quest.Objective[0].Type != 0 || quest.Objective[0].Value <= 0 {
				continue
			}
			for _, layer := range maze.Layers {
				if len(layer.Maps) == 0 {
					continue
				}
				objective := uint32(quest.Objective[0].Value)
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
				action, err := catalog.ResolveScript(a, actionPath)
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
					cmt, err := catalog.ResolveScript(a, cmtPath)
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
						return fmt.Errorf("multiple closing scenes for %v", key)
					}
					seen[key] = true
					scenes = append(scenes, catalog.DungeonTerminalScene{
						Source: d.Source.Checksum, Dungeon: def.ID, Maze: maze.Index, Quest: maze.Quest, Position: layer.Position,
						ObjectiveMap: objective, FinalMap: final, XMin: xmin, XMax: xmax, YMin: ymin, YMax: ymax,
						DungeonSHA256: def.Script.SHA256, MapSHA256: m.SHA256, ActionSHA256: action.SHA256,
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
	var overlay struct {
		Source struct {
			Checksum string `json:"checksum"`
		} `json:"source"`
		Scenes []catalog.DungeonTerminalScene `json:"terminal_scenes"`
	}
	overlay.Source.Checksum = d.Source.Checksum
	overlay.Scenes = scenes
	b, err = json.MarshalIndent(overlay, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.WriteFile(output, b, 0644); err != nil {
		return err
	}
	fmt.Printf("exported %d terminal scenes to %s\n", len(scenes), output)
	return nil
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
