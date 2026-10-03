package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"testing"
)

func odysseyQuestFixture(t *testing.T) *catalog.OdysseyGrowth {
	t.Helper()
	g, e := catalog.LoadOdysseyGrowth("../../configs/odyssey-growth-release.json")
	if e != nil {
		t.Fatal(e)
	}
	return g
}

// 纯策略：满级计划清 345 条（354 减去移除表命中的 9 条），分支全保留。
func TestOdysseyMainlinePlanAtGraduation(t *testing.T) {
	g := odysseyQuestFixture(t)
	clear, branches, e := OdysseyMainlinePlan(g, 0, catalog.OdysseyGraduationLevelV)
	if e != nil {
		t.Fatal(e)
	}
	if len(clear) != 333 {
		t.Fatalf("清除计划 %d 条，期望 333（并集 334 减去 12911）", len(clear))
	}
	if len(branches) != 3 {
		t.Fatalf("分支 %d 条（80/102/115 各一条全职业行，职业 0 不含 3869），期望 3: %v", len(branches), branches)
	}
	for _, b := range branches {
		for _, c := range clear {
			if b == c {
				t.Fatalf("分支任务 %d 同时出现在清除计划里", b)
			}
		}
	}
	// 双保险：即使源漂移，22987/12884 也绝不能被清除。
	for _, c := range clear {
		if c == 22987 || c == 12884 {
			t.Fatalf("清除计划吞掉了必须存活的分支任务 %d", c)
		}
	}
}

func TestOdysseyMainlinePlanRejects(t *testing.T) {
	if _, _, e := OdysseyMainlinePlan(nil, 0, 115); e == nil {
		t.Fatal("nil 目录被接受")
	}
	g := odysseyQuestFixture(t)
	if _, _, e := OdysseyMainlinePlan(g, 0, 116); e == nil {
		t.Fatal("超毕业等级被接受")
	}
}

// 执行层门禁：未挂目录 / 非奥德赛角色都是 no-op，不碰数据库。
func TestOdysseyMainlineGating(t *testing.T) {
	g := odysseyQuestFixture(t)
	s := &Service{Odyssey: g} // Store 故意为 nil：门禁必须先拦
	if _, _, eligible, e := s.RoleMainlinePlan(character.Character{}); e != nil || eligible {
		t.Fatalf("nil Odyssey 目录应 no-op: %t %v", eligible, e)
	}
	s2 := &Service{}
	if _, _, eligible, e := s2.RoleMainlinePlan(character.Character{}); e != nil || eligible {
		t.Fatalf("nil Store 应 no-op: %t %v", eligible, e)
	}
}
