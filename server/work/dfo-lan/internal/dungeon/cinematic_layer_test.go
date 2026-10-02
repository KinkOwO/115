package dungeon

import (
	"testing"

	"dfolan/internal/catalog"
)

// [MERGE-20260928-CINEMATIC-LAYER] 实机回归（2026-09-28「德洛斯矿山的决战」）：
// 100004959 的 (0,0) 格 base 是 100016541（_start 剧情图），层图是 100016270
// （_bb），房里 3 只怪都是 team=0 noncombat=true 的**可击败演员** —— 客户端照样打
// 并发 CMD39。只按 noncombat 判定会把这张图误当演出图，兜底把玩家弹回 100016541，
// 客户端再进层图、再被弹回，来回循环（症状：「放一次技能就重看一次剧情」）。
// 判据必须同时看死亡记录。
func TestLayerRoomIsCinematicDistinguishesPlayableLayers(t *testing.T) {
	c := catalog.LoadNativeFullDungeons(t)

	// 1. 德洛斯矿山的可打层图 100016270：没打之前「无战斗迹象」，但一旦有怪被确认
	//    打死就不是演出图（客户端会自己推进）。
	d := c.Dungeons[100004959]
	for _, mapID := range []uint32{100016541, 100016270} {
		script, ok := c.Maps[mapID]
		if !ok {
			t.Fatalf("map %d 未导入", mapID)
		}
		ms, e := fixedMonsters(script, d.BasisLevel)
		if e != nil {
			t.Fatal(e)
		}
		s := &Session{
			Definition: d, Loaded: true, Dead: map[uint16]bool{},
			Room: catalog.DungeonRoom{X: 0, Y: 0, Map: mapID},
			Maze: d.Mazes[0], Monsters: ms,
		}
		if !s.LayerRoomIsCinematic() {
			t.Fatalf("map %d 未开打时应无战斗迹象", mapID)
		}
		for _, m := range ms {
			s.Dead[m.Entity] = true
			if s.LayerRoomIsCinematic() {
				t.Fatalf("map %d 有怪被确认打死后不该再判为演出图", mapID)
			}
		}
	}

	// 2. 安图恩的 100016165：4 只 team=100 可战斗怪，任何时刻都不是演出图。
	anton := c.Dungeons[100004950]
	script, ok := c.Maps[100016165]
	if !ok {
		t.Fatal("100016165 未导入")
	}
	ms, e := fixedMonsters(script, anton.BasisLevel)
	if e != nil {
		t.Fatal(e)
	}
	s := &Session{
		Definition: anton, Loaded: true, Dead: map[uint16]bool{},
		Room: catalog.DungeonRoom{X: 0, Y: 5, Map: 100016165},
		Maze: anton.Mazes[0], Monsters: ms,
	}
	if s.LayerRoomIsCinematic() {
		t.Fatal("100016165 有可战斗怪，不该判为演出图")
	}

	// 3. 奥德赛演出层图 100016083：1 只 rank3 noncombat，客户端从不打它。
	od := c.Dungeons[100004944]
	var layerMap uint32
	for _, mz := range od.Mazes {
		for _, l := range mz.Layers {
			if len(l.Maps) == 1 && l.Maps[0] == 100016083 {
				layerMap = 100016083
			}
		}
	}
	if layerMap == 0 {
		t.Fatal("100004944 里找不到层图 100016083")
	}
	script, ok = c.Maps[layerMap]
	if !ok {
		t.Fatal("100016083 未导入")
	}
	ms, e = fixedMonsters(script, od.BasisLevel)
	if e != nil {
		t.Fatal(e)
	}
	s = &Session{
		Definition: od, Loaded: true, Dead: map[uint16]bool{},
		Room: catalog.DungeonRoom{X: 1, Y: 0, Map: layerMap},
		Maze: od.Mazes[0], Monsters: ms,
	}
	if !s.LayerRoomIsCinematic() {
		t.Fatal("100016083 是演出层图，应判为演出图以便兜底出口")
	}
}
