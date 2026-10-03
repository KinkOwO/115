// townprobe: 临时探针——直读指定 (城镇, 区域) 的落点与可行走信息。
// 用法: townprobe -archive <inner.pvf> -town 213 [-areas 0,1,2]
package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"

	"dfolan/internal/gamedata"
)

func main() {
	archive := flag.String("archive", "", "inner PVF path")
	town := flag.Uint64("town", 0, "town id")
	areas := flag.String("areas", "0,1,2", "area ids to probe")
	flag.Parse()
	if *archive == "" || *town == 0 {
		flag.Usage()
		return
	}
	src, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: *archive})
	if err != nil {
		panic(err)
	}
	defer src.Close()
	for _, a := range strings.Split(*areas, ",") {
		id, _ := strconv.ParseUint(a, 10, 32)
		area, err := src.TownArea(uint32(*town), uint32(id))
		if err != nil {
			fmt.Printf("town %d/%d: ERROR %v\n", *town, id, err)
			continue
		}
		x, y := area.Spawn()
		fmt.Printf("town %d/%d: map=%s spawn=(%d,%d) walkable=%d rects=%v\n",
			*town, id, area.MapPath, x, y, len(area.Walkable), area.Walkable)
	}
}
