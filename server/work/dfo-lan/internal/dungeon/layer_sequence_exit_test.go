package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/testfixture"
	"testing"
)

// [MERGE-20260928-LAYER-SEQUENCE-EXIT] 实机回归（2026-09-28「贵族机要」）：
// 100004968 的 (0,2) 挂着**两张**层图 [100015974, 100016356]（_e_scene_1、_cc），
// 客户端依次播出后点门（id=38）。上一版兜底一律「回 base」，而回 base 会重置客户端
// 的层索引，它又从第一张重播 —— 973→974→356→973 死循环，症状是「放一次技能就重看
// 一次剧情」。多张序列走完后必须**前进**到相邻房间。
func TestLayerSequenceExitAdvancesInsteadOfLooping(t *testing.T) {
	c, e := catalog.LoadDungeons(testfixture.DungeonPath(t, "dungeons.odyssey-scenes-release.json"))
	if e != nil {
		t.Fatal(e)
	}
	d, ok := c.Dungeons[100004968]
	if !ok {
		t.Skip("100004968 不在 scenes 导出里")
	}

	// 找到那两张层图所在的位置。
	var pos [2]byte
	var last uint32
	found := false
	for _, mz := range d.Mazes {
		for _, l := range mz.Layers {
			if len(l.Maps) > 1 {
				pos, last, found = l.Position, l.Maps[len(l.Maps)-1], true
			}
		}
	}
	if !found {
		t.Fatal("100004968 里找不到多张 layer")
	}
	t.Logf("多张 layer 位置 %v，最后一张 %d", pos, last)

	// 把玩家放到最后一张层图的房间里，模拟客户端播完序列后点门。
	s, e := Select(c, protocol.DungeonSelection{ID: 100004968, Difficulty: 2, Party: 65535}, 90, nil)
	if e != nil {
		t.Fatal(e)
	}
	s, e = s.enterRoom(c, catalog.DungeonRoom{X: pos[0], Y: pos[1], Map: last})
	if e != nil {
		t.Fatal(e)
	}
	clearScene(s)

	// 无记录的 LayerChange 请求（客户端在层图里点门发的正是这种形态）。
	next, e := s.MoveScene(c, protocol.DungeonRoomTransition{
		Dungeon: 100004968, Position: pos, LayerChange: true,
	})
	if e != nil {
		t.Fatalf("多张序列走完后点门应能前进，得到: %v", e)
	}
	if next.Room.Map == last {
		t.Fatal("没有离开层图")
	}
	// 必须落在相邻房间，而不是绕回同位置的 base 层图（那会让客户端重播）。
	if [2]byte{next.Room.X, next.Room.Y} == pos {
		t.Fatalf("应前进到相邻房间，却仍在 %v（map=%d）", pos, next.Room.Map)
	}
	dx := int(next.Room.X) - int(pos[0])
	dy := int(next.Room.Y) - int(pos[1])
	if dx*dx+dy*dy != 1 {
		t.Fatalf("落点不是相邻房间: %+v", next.Room)
	}
	t.Logf("前进到相邻房间 (%d,%d) map=%d", next.Room.X, next.Room.Y, next.Room.Map)
}
