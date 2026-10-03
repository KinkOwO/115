package dungeon

import (
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
)

// [MERGE-20260928-LAYER-SEQUENCE-FINAL] 实机回归（2026-09-28 晦月湖 100004777）。
//
//	(1,0) map=100015634 挂着层图 [100008950]，只有一张（序列最后一张）。
//	100008950 是**剧情层图**：[monster] 段 59 行全是 team=1 的布景怪，玩家进去只播
//	剧情、不需要打。客户端播完剧情后自己发 CMD45 要下一张，服务端曾因「房里怪没清」
//	拒成 "no next layer map"，角色卡在图上不再显示。
//
// 关键断言：序列走到最后一张时，**怪没清也必须放行** —— 判据不能要求 roomEnemiesDead，
// 也不能要求 Record 匹配（剧情层图的请求带客户端自己的落点记录）。
func TestLayerSequenceFinalExitsWithoutClearingRoom(t *testing.T) {
	c := catalog.LoadNativeFullDungeons(t)
	d, ok := c.Dungeons[100004777]
	if !ok {
		t.Fatal("100004777 不在 full 导出里")
	}
	mz := d.Mazes[0]

	// 前提：锁定 plot 层图 100008950 所在的格子（(1,0)，base 100015634）。
	const plotLayer = 100008950
	var pos [2]byte
	var base uint32
	found := false
	for _, l := range mz.Layers {
		for _, m := range l.Maps {
			if m == plotLayer {
				pos, base, found = l.Position, 0, true
			}
		}
	}
	if !found {
		t.Fatalf("100004777 没有挂 layer %d 的格子，前提变了: %+v", plotLayer, mz.Layers)
	}
	for _, r := range mz.Rooms {
		if r.X == pos[0] && r.Y == pos[1] {
			base = r.Map
		}
	}
	if base == 0 {
		t.Fatalf("位置 %v 没有 base 房间", pos)
	}
	t.Logf("层图格 %v base=%d layer=%d", pos, base, plotLayer)

	s := &Session{
		Definition: d, Loaded: true, Dead: map[uint16]bool{},
		Room: catalog.DungeonRoom{X: pos[0], Y: pos[1], Map: plotLayer},
		Maze: mz,
	}
	script, ok := c.Maps[plotLayer]
	if !ok {
		t.Fatalf("map %d 未导入", plotLayer)
	}
	monsters, err := fixedMonsters(script, d.BasisLevel)
	if err != nil {
		t.Fatal(err)
	}
	s.Monsters = monsters
	t.Logf("层图 %d monsters=%d roomEnemiesDead=%v atLayerLast=%v",
		plotLayer, len(s.Monsters), s.roomEnemiesDead(), s.AtLayerLastMap())

	if !s.AtLayerLastMap() {
		t.Fatal("该层图只有一张，应判为序列最后一张")
	}
	if s.roomEnemiesDead() {
		t.Fatal("前提错误：该剧情层图的布景怪不该被判为已清场")
	}

	// 关键断言：怪没清，但序列已到最后一张且客户端请求换图 —— 必须放行。
	next, e := s.MoveScene(c, protocol.DungeonRoomTransition{
		Dungeon: 100004777, Position: pos, LayerChange: true,
		Record: [18]byte{0, 1, 0, 0, 4, 5, 100, 0, 100, 0},
	})
	if e != nil {
		t.Fatalf("剧情层图序列走完应放行，得到: %v", e)
	}
	if next.Room.Map == plotLayer {
		t.Fatalf("应离开层图 %d，仍在原图", plotLayer)
	}
	// 更关键：不能回 base —— 客户端进 base 这一格又会自动播这张层图，形成
	// 100015634↔100008950 每秒来回一次、一直重看剧情（实机 2026-09-28）。
	if next.Room.Map == base {
		t.Fatalf("不能回 base %d（会导致 base↔层图 死循环重看剧情），得到 (%d,%d) map=%d",
			base, next.Room.X, next.Room.Y, next.Room.Map)
	}
	t.Logf("放行到 (%d,%d) map=%d", next.Room.X, next.Room.Y, next.Room.Map)
}
