package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
)

// logMazeChance 把「哪些副本按权重掷骰选图、每张图各占多少」念出来。
//
// 这条日志是刻意的：官方权重是按长期反复刷取的生态设计的，小深渊那组我们对它做了
// 改写（见 configs/dungeons.maze-chance-rates.json 里的 source_rates / rates）。
// 不念出来，以后有人拿这份表的数字去对官方数据，就会误判成「客户端表读错了」。
func logMazeChance(c *catalog.DungeonCatalog) {
	if c == nil {
		return
	}
	ids := make([]uint32, 0, 4)
	for id, d := range c.Dungeons {
		if len(d.MazeChanceRates) > 0 {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		rates := c.Dungeons[id].MazeChanceRates
		var total uint64
		for _, w := range rates {
			total += uint64(w)
		}
		parts := make([]string, 0, len(rates))
		for i, w := range rates {
			parts = append(parts, fmt.Sprintf("maze %d %.4f%%", i, 100*float64(w)/float64(total)))
		}
		log.Printf("dungeon %d rolls its maze by [maze chance rate]: %s", id, strings.Join(parts, " · "))
	}
	if len(ids) > 0 {
		log.Printf("%d dungeon(s) pick the maze by weight; every other dungeon keeps the lowest-index rule", len(ids))
	}
}

// forceMaze 是实机验证用的诊断开关（DFO_MAZE_FORCE="<dungeon>:<index>[, ...]"）：
// 把指定副本的权重改成「只有第 index 张可选」。
//
// 为什么需要它：2%（更别说官方那 0.7143%）靠手刷是撞不到的，而「进了异空间到底
// 会不会加载 special 地图、天平演出对不对、掉的是不是光辉灵魂结晶」必须能确定性
// 地复现一次。它只作用于已经启用权重的副本，其它副本一个字节都不动。
func forceMaze(c *catalog.DungeonCatalog, spec string) error {
	if c == nil || strings.TrimSpace(spec) == "" {
		return nil
	}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		dungeonText, indexText, ok := strings.Cut(part, ":")
		if !ok {
			return fmt.Errorf("bad DFO_MAZE_FORCE %q: want <dungeon>:<index>", part)
		}
		id, err := strconv.ParseUint(strings.TrimSpace(dungeonText), 10, 32)
		if err != nil {
			return fmt.Errorf("bad DFO_MAZE_FORCE dungeon %q: %w", dungeonText, err)
		}
		index, err := strconv.Atoi(strings.TrimSpace(indexText))
		if err != nil {
			return fmt.Errorf("bad DFO_MAZE_FORCE index %q: %w", indexText, err)
		}
		d, ok := c.Dungeons[uint32(id)]
		if !ok {
			return fmt.Errorf("DFO_MAZE_FORCE names absent dungeon %d", id)
		}
		if len(d.MazeChanceRates) == 0 {
			return fmt.Errorf("DFO_MAZE_FORCE names dungeon %d, which does not roll its maze by weight", id)
		}
		if index < 0 || index >= len(d.MazeChanceRates) {
			return fmt.Errorf("DFO_MAZE_FORCE index %d is outside dungeon %d's %d maze(s)",
				index, id, len(d.MazeChanceRates))
		}
		rates := make([]uint32, len(d.MazeChanceRates))
		rates[index] = 1
		d.MazeChanceRates = rates
		c.Dungeons[uint32(id)] = d
		log.Printf("MAZE FORCE: dungeon %d will always enter maze %d (diagnostic override, unset DFO_MAZE_FORCE to restore)", id, index)
	}
	return nil
}

// noteMazeEntry 把「这次进的是哪张 maze、加载的是哪张地图」写进 gateway.err。
//
// 选图改成按权重掷骰之后，「到底有没有进到 special 那张图」不该只能靠掉落去反推
// —— 2% 的概率下，日志是唯一能当场回答它的东西。
func noteMazeEntry(s *dungeon.Session) {
	if s == nil {
		return
	}
	log.Printf("dungeon %d entry: maze %d -> map %d", s.Definition.ID, s.Maze.Index, s.Room.Map)
}
