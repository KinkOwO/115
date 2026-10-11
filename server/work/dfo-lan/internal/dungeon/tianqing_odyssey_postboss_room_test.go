package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

// 实机 2026-10-04：奥德赛 100004971（36_Bakal）打完 boss (6,0)=100016540 后，小地图上
// 还有 (7,0)=100016547 与 (8,0)=100016548 两格，但走不过去 —— 服务端回
// `dungeon_request_refused: boss completion is pending or already accepted`。
//
// 根因：源里这两格 `[map specification]` 标的是 `map` 而不是 `layered`（对比
// dimensioncloister.dgn 的 boss 前后都是 `layered`），所以 maze.Layers 为空，
// 旧的 adjacentLayerPending 永远 false，完成守卫把这段挡死。
//
// 这里钉死新放行条件：boss 完成后，**相邻且还没去过**的 maze 房间可走；
// 已经去过的相邻房间仍被拒（防 boss 后乱走）。
func TestOdysseyPostBossAdjacentRoomIsReachable(t *testing.T) {
	c, e := catalog.LoadDungeons(tianqingDungeonFixture(t))
	if e != nil {
		t.Fatal(e)
	}
	def, ok := c.Dungeons[100004971]
	if !ok || len(def.Mazes) == 0 {
		t.Fatal("dungeon 100004971 missing from catalog")
	}
	maze := def.Mazes[0]
	var boss catalog.DungeonRoom
	for _, r := range maze.Rooms {
		if r.Map == 100016540 {
			boss = r
		}
	}
	if boss.Map == 0 || boss.X != 6 || boss.Y != 0 {
		t.Fatalf("boss room (6,0)=100016540 not imported as expected: %+v", boss)
	}
	if len(maze.Layers) != 0 {
		t.Fatalf("100004971 的 boss 后房间本应是普通 map（layers 为空），源已变: %+v", maze.Layers)
	}
	s := &Session{
		Definition: def, Maze: maze, Room: boss, Loaded: true,
		Dead:       map[uint16]bool{},
		NextEntity: 0x1000,
		// 从 (5,0) 走过来：boss 房与来路都算去过，boss 后的两格没去过。
		Visited:          map[uint32][]protocol.DungeonMonster{100016385: nil, 100016546: nil, 100016540: nil},
		completionTarget: 1, // boss 已完成
	}
	next, e := s.Move(c, [2]byte{7, 0})
	if e != nil || next == nil || next.Room.Map != 100016547 {
		t.Fatalf("boss 后的相邻未访问房间 (7,0) 走不过去: next=%+v err=%v", next, e)
	}
	// 已去过的相邻房间（来路 (5,0)）仍要被拒，避免 boss 完成后到处乱走。
	if _, e = s.Move(c, [2]byte{5, 0}); e == nil {
		t.Fatal("boss 完成后仍能走回已访问的相邻房间，守卫被放宽过头")
	}
}
