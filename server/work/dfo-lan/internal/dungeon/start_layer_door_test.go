package dungeon

import (
	"testing"

	"dfolan/internal/catalog"
)

// [MERGE-20260928-START-LAYER-EXIT] 实机回归（2026-09-28 晦月湖 100004777，闪退）。
//
//	(0,0) **既是迷宫起点、又挂着层图 [100015633]**（base 100008953）。
//	玩家点门走的是 CMD38（服务端合成出口）。若按普通场景房「回 base」处理，就是送回
//	起点自己 —— 客户端进这一格本来就会自动播那张层图，于是 层图↔base 往返、重复放
//	剧情，实测 3 秒内黑屏闪退（events: 266 map=100015633 → 275 map=100008953 →
//	278 exit_shutdown_signal）。
//
// 锁住：起点层图格点门必须**前进**，不能回 base。
func TestStartLayerSceneDoorAdvancesNotBase(t *testing.T) {
	c := catalog.LoadNativeFullDungeons(t)
	d, ok := c.Dungeons[100004777]
	if !ok {
		t.Fatal("100004777 不在 full 导出里")
	}
	mz := d.Mazes[0]

	// 前提：起点格挂着层图。
	var startLayer []uint32
	for _, l := range mz.Layers {
		if l.Position == mz.Start {
			startLayer = l.Maps
		}
	}
	if len(startLayer) == 0 {
		t.Fatalf("100004777 的起点 %v 没有层图，前提变了: %+v", mz.Start, mz.Layers)
	}
	var startBase uint32
	for _, r := range mz.Rooms {
		if [2]byte{r.X, r.Y} == mz.Start {
			startBase = r.Map
		}
	}
	t.Logf("起点 %v base=%d layer=%v", mz.Start, startBase, startLayer)

	s := &Session{
		Definition: d, Loaded: true, Dead: map[uint16]bool{},
		Room: catalog.DungeonRoom{X: mz.Start[0], Y: mz.Start[1], Map: startLayer[len(startLayer)-1]},
		Maze: mz,
	}
	script, ok := c.Maps[s.Room.Map]
	if !ok {
		t.Fatalf("map %d 未导入", s.Room.Map)
	}
	monsters, err := fixedMonsters(script, d.BasisLevel)
	if err != nil {
		t.Fatal(err)
	}
	s.Monsters = monsters
	if !s.LayerAtStart(mz.Start) {
		t.Fatal("起点层图格应被 LayerAtStart 认出")
	}

	// 走门（MD38 合成的出口）。
	next, e := s.ExitSceneRoom(c, mz.Start)
	if e != nil {
		t.Fatalf("起点层图格点门应能前进，得到: %v", e)
	}
	if next.Room.Map == startBase {
		t.Fatalf("不能回 base %d（= 起点自己，会重复放剧情直到闪退）", startBase)
	}
	if [2]byte{next.Room.X, next.Room.Y} == mz.Start {
		t.Fatalf("应前进到相邻格，却仍在起点 %v（map=%d）", mz.Start, next.Room.Map)
	}
	dx := int(next.Room.X) - int(mz.Start[0])
	dy := int(next.Room.Y) - int(mz.Start[1])
	if dx*dx+dy*dy != 1 {
		t.Fatalf("落点不是相邻格: %+v", next.Room)
	}
	t.Logf("前进到 (%d,%d) map=%d  diag=%s", next.Room.X, next.Room.Y, next.Room.Map, next.SceneDiagnostic())
}
