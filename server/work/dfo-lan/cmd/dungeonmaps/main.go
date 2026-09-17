// dungeonmaps 补齐 dungeons.full.json 里被旧解析器漏掉的地图脚本。
//
// 背景：旧解析器把含「多地图节点」的 [map specification] 整张 maze 判成
// unsupported room specification，rooms 归零，于是这些房间引用的地图从未
// 进入地图目录。服务端启动时用新解析器重新解析 maze，就会引用到目录里
// 不存在的地图（实测 dungeon 86 的 boss 房 20314..20317）。
//
// 本工具只做补缺：
//   - 不删除、不修改任何已有地图条目；
//   - 不修改 source 段。角色与任务的 config_version 必须继续等于
//     source.checksum（旧值 7ef2db59…），换掉会让所有任务副本被拒。
//
// 用法：
//
//	dungeonmaps -catalog configs/dungeons.full.json -source runtime/pvf_source/Script.inner.pvf
//	dungeonmaps -catalog ... -source ... -dry-run=false -output configs/dungeons.full.json.new
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
)

func main() {
	catalogPath := flag.String("catalog", "configs/dungeons.full.json", "existing catalog to inspect")
	pvfPath := flag.String("source", "", "original PVF, opened read-only")
	output := flag.String("output", "", "output catalog path (required unless -dry-run)")
	dryRun := flag.Bool("dry-run", true, "report the missing maps without writing anything")
	flag.Parse()
	if *pvfPath == "" {
		log.Fatal("-source is required")
	}
	if !*dryRun && *output == "" {
		log.Fatal("-output is required when -dry-run=false")
	}

	c, e := catalog.LoadDungeons(*catalogPath)
	if e != nil {
		log.Fatal(e)
	}
	// LoadDungeons 用当前解析器重算每个副本的 maze，所以这里看到的房间与
	// 备选地图就是服务端启动时会引用的集合。
	need := map[uint32]bool{}
	for _, d := range c.Dungeons {
		for _, m := range d.Mazes {
			for _, r := range m.Rooms {
				need[r.Map] = true
				for _, alt := range r.Alternates {
					need[alt] = true
				}
			}
		}
	}
	var missing []uint32
	for id := range need {
		if _, ok := c.Maps[id]; !ok {
			missing = append(missing, id)
		}
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
	log.Printf("dungeons=%d referenced_maps=%d present_maps=%d missing=%d", len(c.Dungeons), len(need), len(c.Maps), len(missing))
	if len(missing) == 0 {
		fmt.Println("nothing missing")
		return
	}
	fmt.Println("missing map ids:", missing)
	if *dryRun {
		log.Printf("dry run: %s unchanged", *catalogPath)
		return
	}

	a, e := pvf.LoadArchive(pvf.Options{Path: *pvfPath, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	script, e := catalog.ReadScript(a, "list/map.lst")
	if e != nil {
		log.Fatal(e)
	}
	rows, e := catalog.ParseIndex(script.Cells)
	if e != nil {
		log.Fatal(e)
	}
	paths := map[uint32]string{}
	for _, r := range rows {
		paths[r.ID] = r.Path
	}
	added := 0
	for _, id := range missing {
		name, ok := paths[id]
		if !ok {
			log.Printf("map %d: not in list/map.lst, skipped", id)
			continue
		}
		s, e := catalog.ResolveScript(a, name)
		if e != nil {
			log.Printf("map %d: %v", id, e)
			continue
		}
		c.Maps[id] = s
		added++
	}
	log.Printf("added maps=%d total=%d", added, len(c.Maps))

	// 与既有的 dungeons.full.json 保持一致：紧凑 JSON（该文件 274MB，
	// 缩进格式会膨胀到 700MB，拖慢服务端启动解析）。
	b, e := json.Marshal(c)
	if e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(*output, b, 0600); e != nil {
		log.Fatal(e)
	}
	log.Printf("wrote %s bytes=%d source_checksum=%s", *output, len(b), c.Source.Checksum)
}
