package dungeon

import (
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
)

// [MERGE-20260928-LAYER-SEQUENCE-EXIT] 实机回归（2026-09-28 客户端 0xC0000005 闪退）：
// 100004981 的 (6,0) 挂着**四张**层图 [100001053, 100001054, 100001055, 100001056]，
// 其中 1053/1054/1055 房里的怪**全是** team=0 noncombat 演员。只看 LayerRoomIsCinematic
// 会把第 2 张 1054 也当成「演出图」，兜底把玩家弹回上一格 (5,0)，客户端又从头重播
// 474→1053→1054，最后退化成「同图再进同图」—— StartMap 发 ReuseRoom 而客户端没有该
// 图缓存，直接闪退。
// 判据必须加上「当前是序列最后一张」。
func TestLayerExitOnlyAtLastMap(t *testing.T) {
	c := catalog.LoadNativeFullDungeons(t)
	d := c.Dungeons[100004981]

	var layer catalog.DungeonLayer
	found := false
	for _, mz := range d.Mazes {
		for _, l := range mz.Layers {
			if len(l.Maps) == 4 {
				layer, found = l, true
			}
		}
	}
	if !found {
		t.Fatal("100004981 里找不到 4 张的层图序列")
	}
	if layer.Maps[1] != 100001054 || layer.Maps[3] != 100001056 {
		t.Fatalf("层图序列与预期不符: %v", layer.Maps)
	}

	s, e := Select(c, protocol.DungeonSelection{ID: 100004981, Difficulty: 2, Party: 65535}, 110, nil)
	if e != nil {
		t.Fatal(e)
	}
	s.Loaded = true

	// 中间几张（1053/1054/1055）：不是序列末尾，不该兜底。
	for _, mid := range layer.Maps[:3] {
		s.Room = catalog.DungeonRoom{X: layer.Position[0], Y: layer.Position[1], Map: mid}
		script, ok := c.Maps[mid]
		if !ok {
			t.Fatalf("map %d 未导入", mid)
		}
		ms, err := fixedMonsters(script, d.BasisLevel)
		if err != nil {
			t.Fatal(err)
		}
		s.Monsters = ms
		s.Dead = map[uint16]bool{}
		if s.LayerRoomIsCinematic() && s.AtLayerLastMap() {
			t.Fatalf("map %d 是序列中途，不该满足出口兜底", mid)
		}
	}

	// 最后一张（1056）：有可战斗怪，同样不满足。
	s.Room = catalog.DungeonRoom{X: layer.Position[0], Y: layer.Position[1], Map: layer.Maps[3]}
	script, ok := c.Maps[layer.Maps[3]]
	if !ok {
		t.Fatalf("map %d 未导入", layer.Maps[3])
	}
	ms, err := fixedMonsters(script, d.BasisLevel)
	if err != nil {
		t.Fatal(err)
	}
	s.Monsters = ms
	s.Dead = map[uint16]bool{}
	if s.LayerRoomIsCinematic() {
		t.Fatalf("map %d 有可战斗怪，不该判为演出图", layer.Maps[3])
	}
	if !s.AtLayerLastMap() {
		t.Fatalf("map %d 应是序列最后一张", layer.Maps[3])
	}
}
