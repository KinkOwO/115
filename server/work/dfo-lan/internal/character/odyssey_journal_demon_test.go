package character

import (
	"dfolan/internal/game/protocol"
	"encoding/json"
	"testing"
)

// [MERGE-20260928-JOURNAL-LANDING] 实机回归：2026-09-28 一次会话的最后一条
// area_refused 目标是 town=31 area=2（路由表第 12 站 (31,2,357,310)，
// dungeons=[100004954,100004955]）。客户端从传送门/地图选择器出发时报的落点与
// 路由表坐标不同，修好后必须仍能放行。
func TestOdysseyJournalDemonRealmLanding(t *testing.T) {
	s, role := odysseyGrowthFixture(t)
	var nodes []odysseyJournalNode
	if e := json.Unmarshal(odysseyJournalRoutes, &nodes); e != nil {
		t.Fatal(e)
	}
	idx := -1
	var node odysseyJournalNode
	for i, n := range nodes {
		if n.Destination[0] == 31 && n.Destination[1] == 2 {
			idx, node = i, n
			break
		}
	}
	if idx < 0 {
		t.Fatal("路由表里没有 (31,2)")
	}
	t.Logf("(31,2) 是第 %d 站，路由坐标=(%d,%d)，dungeons=%v", idx+1, node.Destination[2], node.Destination[3], node.Dungeons)

	// 通关前面所有站。
	for i := 0; i < idx; i++ {
		for _, id := range nodes[i].Dungeons {
			var err error
			role.State, err = s.saveOdysseyCompletion(role, id)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	// [MERGE-20260928-JOURNAL-TAILFLAGS] 用实机那条被拒请求的真实形态：从魔界 (22,4)
	// 去 (31,2)，尾部标志是 [0,2]（不是全零，也不是地图选择器的 5）。落点也照实机
	// 用 (641,321)，与路由表坐标 (357,310) 不同。
	for _, tail := range [][2]byte{{0, 2}, {0, 0}, {1, 2}} {
		r := protocol.AreaChangeRequest{
			Town: 31, Area: 2, X: 641, Y: 321, Flag: 5,
			PreviousTown: 22, PreviousArea: 4, TailFlags: tail,
		}
		if !s.OdysseyJournalTeleport(role, r) {
			t.Fatalf("(31,2) 尾部标志 %v 被拒", tail)
		}
	}
	// 地图选择器（标志位 5）仍走另一条路，不由日志阶梯放行。
	for _, tail := range [][2]byte{{5, 0}, {0, 5}} {
		r := protocol.AreaChangeRequest{
			Town: 31, Area: 2, X: 641, Y: 321, Flag: 5,
			PreviousTown: 22, PreviousArea: 4, TailFlags: tail,
		}
		if s.OdysseyJournalTeleport(role, r) {
			t.Fatalf("地图选择器 %v 不该由日志阶梯放行", tail)
		}
	}
	// 客户端落点取路由坐标之外的几个点，都应放行。
	for _, landing := range [][2]uint16{
		{uint16(node.Destination[2]), uint16(node.Destination[3])},
		{706, 281},
		{500, 400},
	} {
		r := protocol.AreaChangeRequest{Town: 31, Area: 2, X: landing[0], Y: landing[1], Flag: 5}
		if !s.OdysseyJournalTeleport(role, r) {
			t.Fatalf("(31,2) 落点 %v 被拒", landing)
		}
	}
}
