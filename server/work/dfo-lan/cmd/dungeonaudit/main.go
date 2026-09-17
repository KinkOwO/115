// dungeonaudit 统计一份副本目录里“普通进城选图”（quest==0）能进哪些副本，
// 以及进不去的原因分布。用来在改解析器/选取规则前后做对照。
//
// 用法：
//
//	dungeonaudit -catalog configs/dungeons.full.json -level 115
//	dungeonaudit -catalog ... -list            # 额外打印可进/不可进的 ID
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"flag"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
)

func main() {
	catalogPath := flag.String("catalog", "configs/dungeons.full.json", "dungeon catalog to audit")
	level := flag.Int("level", 115, "character level used for the minimum-level gate")
	quest := flag.Int("quest", 0, "requested quest id (0 = ordinary town entry)")
	list := flag.Bool("list", false, "print every dungeon id grouped by result")
	idsFlag := flag.String("ids", "", "comma separated dungeon ids to describe in detail")
	flag.Parse()

	c, e := catalog.LoadDungeons(*catalogPath)
	if e != nil {
		log.Fatal(e)
	}
	reasons := map[string]int{}
	ok := []uint32{}
	bad := map[string][]uint32{}
	ids := make([]uint32, 0, len(c.Dungeons))
	for id := range c.Dungeons {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		r := protocol.DungeonSelection{ID: id, Party: 65535, Quest: uint32(*quest)}
		s, e := dungeon.Select(c, r, byte(*level), map[uint16]bool{})
		if e != nil {
			reasons[e.Error()]++
			bad[e.Error()] = append(bad[e.Error()], id)
			continue
		}
		ok = append(ok, id)
		_ = s
	}
	log.Printf("catalog=%s dungeons=%d maps=%d", *catalogPath, len(c.Dungeons), len(c.Maps))
	log.Printf("quest=%d level=%d enterable=%d refused=%d", *quest, *level, len(ok), len(ids)-len(ok))
	for _, reason := range sortedKeys(reasons) {
		fmt.Printf("  %6d  %s\n", reasons[reason], reason)
	}
	if *list {
		fmt.Println("enterable ids:", ok)
		for _, reason := range sortedKeys(reasons) {
			fmt.Printf("%s (%d): %v\n", reason, len(bad[reason]), bad[reason])
		}
	}
	for _, field := range strings.Split(*idsFlag, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		id, err := strconv.ParseUint(field, 10, 32)
		if err != nil {
			log.Fatalf("bad -ids entry %q: %v", field, err)
		}
		r := protocol.DungeonSelection{ID: uint32(id), Party: 65535, Quest: uint32(*quest)}
		s, e := dungeon.Select(c, r, byte(*level), map[uint16]bool{})
		if e != nil {
			fmt.Printf("dungeon %d: REFUSED  %v\n", id, e)
			continue
		}
		fmt.Printf("dungeon %d: OK  maze=%d rooms=%d start=%v boss=%v level=%d+  startmap=%d monsters=%d\n",
			id, s.Maze.Index, len(s.Maze.Rooms), s.Maze.Start, s.Maze.Boss,
			s.Definition.MinimumLevel, s.Room.Map, len(s.Monsters))
	}
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return m[out[i]] > m[out[j]] })
	return out
}
