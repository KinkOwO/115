package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

// 实机 2026-10-04：苏醒之森 100004977 的 `[map specification]` 里 (6,0) 同时是
// `map 100017262`（base）与 `layered 100017263`（层图）。boss 房 (5,0)=100016438
// 打完就判完成，玩家按 Move 的放行走到了 (6,0) 的 base、清完房，客户端随即主动发
// layer change 要进层图看收尾 —— 服务端却因 s.Completed() 回
// "scene transition requires owned cleared loaded room"，剧情卡住、通关无法展开。
//
// 本测试用**源路由表里真实的那份落点记录**（客户端就是这么发的），只验证「通关后
// 这道守卫不再拦」。防篡改语义另由下面和既有的 RejectsInvalidTransitions 钉住。
func TestOdysseyPostBossLayerSceneEntry(t *testing.T) {
	c := odysseyScenes(t)
	d, ok := c.Dungeons[100004977]
	if !ok {
		t.Skip("100004977 不在 scenes 导出里")
	}
	var base catalog.DungeonRoom
	layerMaps := 0
	for _, mz := range d.Mazes {
		for _, r := range mz.Rooms {
			if r.Map == 100017262 {
				base = r
			}
		}
		for _, l := range mz.Layers {
			if l.Position == [2]byte{6, 0} {
				layerMaps = len(l.Maps)
			}
		}
	}
	if base.Map == 0 || base.X != 6 || base.Y != 0 {
		t.Fatalf("(6,0) base 100017262 未按预期导入: %+v", base)
	}
	if layerMaps == 0 {
		t.Fatal("(6,0) 没有挂 layer —— 源已变，本测试前提失效")
	}

	// (6,0) 这个位置、from base 100017262 的源路由记录
	var rec [18]byte
	hasRoute := false
	for _, rt := range c.SceneRoutes {
		if rt.Dungeon == 100004977 && rt.Position == [2]byte{6, 0} && rt.From == 100017262 {
			rec, hasRoute = rt.Record, true
		}
	}
	t.Logf("(6,0) from 100017262 的源路由: has=%v record=%x", hasRoute, rec)

	s, e := Select(c, protocol.DungeonSelection{ID: 100004977, Difficulty: 2, Party: 65535}, 105, nil)
	if e != nil {
		t.Fatal(e)
	}
	s, e = s.enterRoom(c, base)
	if e != nil {
		t.Fatal(e)
	}
	clearScene(s)
	s.completed = true // boss 房已经判定通关

	if !s.layerPendingUnvisited([2]byte{6, 0}) {
		t.Fatal("layerPendingUnvisited((6,0)) 应为 true")
	}

	// 1) 通关后带源记录进当前格的层图：必须放行。
	next, e := s.MoveScene(c, protocol.DungeonRoomTransition{
		Dungeon: 100004977, Position: [2]byte{6, 0}, LayerChange: true, Record: rec,
	})
	if e != nil {
		t.Fatalf("通关后进入当前格的层图被拒: %v", e)
	}
	if next.Room.Map != 100017263 {
		t.Fatalf("应落到层图 100017263，实得 %+v", next.Room)
	}

	// 2) 同样的请求但落点被篡改：仍必须被拒（放宽不得破坏防篡改）。
	s2, e := Select(c, protocol.DungeonSelection{ID: 100004977, Difficulty: 2, Party: 65535}, 105, nil)
	if e != nil {
		t.Fatal(e)
	}
	s2, e = s2.enterRoom(c, base)
	if e != nil {
		t.Fatal(e)
	}
	clearScene(s2)
	s2.completed = true
	bad := rec
	bad[6] ^= 1
	if _, e = s2.MoveScene(c, protocol.DungeonRoomTransition{
		Dungeon: 100004977, Position: [2]byte{6, 0}, LayerChange: true, Record: bad,
	}); e == nil {
		t.Fatal("通关后带篡改落点的层图请求竟然被接受")
	}
}

// 放宽只针对「当前格挂着 layer」：通关后在**没有 layer 的格**上换图仍必须被拒。
func TestPostBossLayerSceneStillRejectsPlainRoom(t *testing.T) {
	c := odysseyScenes(t)
	d, ok := c.Dungeons[100004977]
	if !ok {
		t.Skip("100004977 不在 scenes 导出里")
	}
	var plain catalog.DungeonRoom
	for _, mz := range d.Mazes {
		for _, r := range mz.Rooms {
			if r.Map == 100016430 { // (0,1) 起点，该格没有 layer
				plain = r
			}
		}
	}
	if plain.Map == 0 {
		t.Fatal("(0,1)=100016430 未导入")
	}
	s, e := Select(c, protocol.DungeonSelection{ID: 100004977, Difficulty: 2, Party: 65535}, 105, nil)
	if e != nil {
		t.Fatal(e)
	}
	s, e = s.enterRoom(c, plain)
	if e != nil {
		t.Fatal(e)
	}
	clearScene(s)
	s.completed = true
	if _, e = s.MoveScene(c, protocol.DungeonRoomTransition{
		Dungeon: 100004977, Position: [2]byte{0, 1}, LayerChange: true,
	}); e == nil {
		t.Fatal("通关后在无 layer 的格上换图竟然被放行")
	}
}
