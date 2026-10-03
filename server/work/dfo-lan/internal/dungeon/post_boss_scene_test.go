package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/testfixture"
	"testing"
)

// [MERGE-20260928-POSTBOSS-SCENE] 实机回归（2026-09-28「苏醒之森」）：
// 100004977 在 boss 房 (5,0) 就判完成，但后面还有 (6,0) 的过场（层图 100017263）。
// 玩家打完 boss 想走过去时，Move 的完成守卫一律拒，服务端回
// "boss completion is pending or already accepted"，传送阵过不去。
// 修好后：完成之后仍可走向「有剧情层图」的相邻格，普通邻格照旧被拒。
func TestPostBossSceneRoomStillReachable(t *testing.T) {
	c, e := catalog.LoadDungeons(testfixture.DungeonPath(t, "dungeons.odyssey-scenes-release.json"))
	if e != nil {
		t.Fatal(e)
	}
	d, ok := c.Dungeons[100004977]
	if !ok {
		t.Skip("100004977 不在 scenes 导出里")
	}
	var boss catalog.DungeonRoom
	var scenePos [2]byte
	hasScene := false
	for _, mz := range d.Mazes {
		for _, r := range mz.Rooms {
			if r.Boss {
				boss = r
			}
		}
		for _, l := range mz.Layers {
			if len(l.Maps) > 0 {
				scenePos, hasScene = l.Position, true
			}
		}
	}
	if !hasScene {
		t.Fatal("100004977 没有层图格")
	}
	t.Logf("boss 房 (%d,%d) map=%d；层图格 %v", boss.X, boss.Y, boss.Map, scenePos)

	// 1. 完成之后走向剧情层图格：必须放行。
	s, e := Select(c, protocol.DungeonSelection{ID: 100004977, Difficulty: 2, Party: 65535}, 105, nil)
	if e != nil {
		t.Fatal(e)
	}
	s, e = s.enterRoom(c, boss)
	if e != nil {
		t.Fatal(e)
	}
	clearScene(s)
	s.completed = true
	next, e := s.Move(c, scenePos)
	if e != nil {
		t.Fatalf("完成之后走向剧情层图格 %v 被拒: %v", scenePos, e)
	}
	if [2]byte{next.Room.X, next.Room.Y} != scenePos {
		t.Fatalf("应落在 %v，得到 %+v", scenePos, next.Room)
	}
	t.Logf("走到 %+v map=%d", next.Room, next.Room.Map)

	// 2. 完成之后走向不带层图的普通相邻格：仍须被拒。
	s2, e := Select(c, protocol.DungeonSelection{ID: 100004977, Difficulty: 2, Party: 65535}, 105, nil)
	if e != nil {
		t.Fatal(e)
	}
	s2, e = s2.enterRoom(c, boss)
	if e != nil {
		t.Fatal(e)
	}
	clearScene(s2)
	s2.completed = true
	checked := 0
	for _, r := range d.Mazes[0].Rooms {
		target := [2]byte{r.X, r.Y}
		if target == scenePos {
			continue
		}
		dx := int(target[0]) - int(boss.X)
		dy := int(target[1]) - int(boss.Y)
		if dx*dx+dy*dy != 1 {
			continue
		}
		checked++
		if _, e := s2.Move(c, target); e == nil {
			t.Fatalf("完成之后不该能走向普通相邻格 %v", target)
		}
	}
	if checked == 0 {
		t.Log("boss 房没有普通邻格可对照")
	}
}
