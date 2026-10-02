package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

// [MERGE-20260928-BOSS-ROOM-ACTOR] 实机回归（2026-09-28「前往阿拉德」）：
// 奥德赛的 100004984..100004989 这 5 个副本（正是 scenes 导出里没有 scene_routes
// 的那一批）声明的 boss 房里只摆 rank=0 的源怪（tpl=109019487）。客户端不会为
// rank0 发 CMD117，原来三条兜底路径都要求 rank3 / SourceBoss / 层图，全不成立 ——
// 副本永远结算不了，CompletionTarget 也是 0（编码器拒绝，整批完成数据被丢弃），
// 玩家在 boss 房里点门只收到 door_ack。
func TestOdysseyRankZeroBossRoomCompletes(t *testing.T) {
	c := catalog.LoadNativeFullDungeons(t)
	for _, did := range []uint32{100004984, 100004985, 100004986, 100004987, 100004988, 100004989} {
		d, ok := c.Dungeons[did]
		if !ok {
			t.Fatalf("%d 不在目录", did)
		}
		if !d.Odyssey {
			t.Fatalf("%d 应为奥德赛副本", did)
		}
		var boss catalog.DungeonRoom
		for _, mz := range d.Mazes {
			for _, r := range mz.Rooms {
				if r.Boss {
					boss = r
				}
			}
		}
		s, e := Select(c, protocol.DungeonSelection{ID: did, Difficulty: 2, Party: 65535}, 72, nil)
		if e != nil {
			t.Fatal(e)
		}
		s, e = s.enterRoom(c, boss)
		if e != nil {
			t.Fatal(e)
		}
		clearScene(s)
		s.tryComplete()
		if !s.Completed() {
			t.Fatalf("%d 的 boss 房 (boss 地图 %d) 清空后应结算", did, boss.Map)
		}
		if got := s.CompletionTarget(); got == 0 || got == 65535 {
			t.Fatalf("%d 的完成身份不可编码: %d", did, got)
		}
	}
}
