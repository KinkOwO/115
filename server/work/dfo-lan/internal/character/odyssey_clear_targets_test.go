package character

import (
	"testing"

	"dfolan/internal/catalog"
)

// [MERGE-20260928-ODYSSEY-CLEAR-NO-TARGET] 奥德赛副本不一定都在成长阶梯上。
// 100004984..100004989（正是 scenes 导出里没有 scene_routes 的那一批，实机里
// 「前往阿拉德」等就是它们）也是 Odyssey=true，但 ClearLevels 里没有条目。
// 结算时不能再因为 target==0 就直接报 "missing source clear target" —— 那会让
// 它们即便被判为完成也发不出完成数据，客户端看不到任何变化、卡在 boss 房里。
func TestOdysseyClearLevelsCoverage(t *testing.T) {
	g, e := catalog.LoadOdysseyGrowth("../../configs/odyssey-growth-release.json")
	if e != nil {
		t.Fatal(e)
	}
	levels := g.ClearLevels
	if len(levels) == 0 {
		t.Fatal("成长阶梯为空")
	}
	// 这批不是进度节点：结算时必须走「只记通关、不抬等级」的分支。
	for _, did := range []uint32{100004984, 100004985, 100004986, 100004987, 100004988, 100004989} {
		if levels[did] != 0 {
			t.Fatalf("%d 出现在成长阶梯上（level=%d），本用例的前提变了", did, levels[did])
		}
	}
	// 真正的进度节点必须有目标等级，否则 OdysseyClear 会退化成不抬等级。
	for _, did := range []uint32{100004944, 100004950, 100004953, 100004957, 100004959} {
		if levels[did] == 0 {
			t.Fatalf("进度节点 %d 缺少目标等级", did)
		}
	}
}
