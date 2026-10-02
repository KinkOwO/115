package dungeon

import (
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
)

// [MERGE-20260928-START-LAYER-EXIT] 实机回归（2026-09-28 黑屏闪退）：
// 100004782 的 (0,0) **既是 Start 又挂着层图** [100015938]（只有一张，所以它同时
// 也是「序列最后一张」）。旧逻辑判它满足出口兜底 → 回 base，而 base 就是起点自己，
// 客户端进这一格时本来就会自动播那张层图 → 675↔938 来回走 → 退化成「同图再进同图」
// （ReuseRoom 无缓存）直接 0xC0000005。
// 修好后：起点层图格必须**前进**到相邻格。
func TestStartLayerRoomAdvancesInsteadOfLooping(t *testing.T) {
	c := catalog.LoadNativeFullDungeons(t)
	d, ok := c.Dungeons[100004782]
	if !ok {
		t.Fatal("100004782 不在 full 导出里")
	}
	mz := d.Mazes[0]
	// 前提：起点格挂着一张层图。
	var layerMaps []uint32
	for _, l := range mz.Layers {
		if l.Position == mz.Start {
			layerMaps = l.Maps
		}
	}
	if len(layerMaps) == 0 {
		t.Fatalf("100004782 的起点 %v 没有层图，前提变了", mz.Start)
	}
	t.Logf("起点 %v 的层图: %v", mz.Start, layerMaps)

	s := &Session{
		Definition: d, Loaded: true, Dead: map[uint16]bool{},
		Room: catalog.DungeonRoom{X: mz.Start[0], Y: mz.Start[1], Map: layerMaps[len(layerMaps)-1]},
		Maze: mz,
	}
	script, ok := c.Maps[layerMaps[len(layerMaps)-1]]
	if !ok {
		t.Fatalf("map %d 未导入", layerMaps[len(layerMaps)-1])
	}
	monsters, err := fixedMonsters(script, d.BasisLevel)
	if err != nil {
		t.Fatal(err)
	}
	s.Monsters = monsters
	t.Logf("当前房间 %+v monsters=%d cinematic=%v atLayerLast=%v layerAtStart=%v",
		s.Room, len(s.Monsters), s.LayerRoomIsCinematic(), s.AtLayerLastMap(), s.LayerAtStart(mz.Start))

	if !s.AtLayerLastMap() {
		t.Fatal("该层图只有一张，应判为序列最后一张")
	}
	if !s.LayerRoomIsCinematic() {
		t.Fatal("该层图房里没有可战斗怪，应判为演出图")
	}
	if !s.LayerAtStart(mz.Start) {
		t.Fatal("起点层图格应被 LayerAtStart 认出")
	}

	// 走门：必须前进到相邻格，而不是回到起点自己。
	next, e := s.MoveScene(c, protocol.DungeonRoomTransition{
		Dungeon: 100004782, Position: mz.Start, LayerChange: true,
	})
	if e != nil {
		t.Fatalf("起点层图格走门应能前进，得到: %v", e)
	}
	if [2]byte{next.Room.X, next.Room.Y} == mz.Start {
		t.Fatalf("应前进到相邻格，却仍在起点 %v（map=%d）", mz.Start, next.Room.Map)
	}
	dx := int(next.Room.X) - int(mz.Start[0])
	dy := int(next.Room.Y) - int(mz.Start[1])
	if dx*dx+dy*dy != 1 {
		t.Fatalf("落点不是相邻格: %+v", next.Room)
	}
	t.Logf("前进到相邻格 (%d,%d) map=%d", next.Room.X, next.Room.Y, next.Room.Map)
}
