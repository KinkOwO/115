package catalog

import "testing"

// 真源形状钉死：四张表 + 毕业奖励（next63 / 手册 P3 子项 8/9）。
func TestOdysseyQuestTablesShape(t *testing.T) {
	g, e := LoadOdysseyGrowth("../../configs/odyssey-growth-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	q := g.Quests
	if q == nil {
		t.Fatal("quest tables not loaded")
	}
	total := 0
	levels := []byte{}
	for _, row := range q.ClearLevels {
		levels = append(levels, row.Level)
		total += len(row.Quest)
	}
	if len(levels) != OdysseyQuestClearLevels || total != OdysseyQuestClearTotal {
		t.Fatalf("[quest clear] %d 级 / %d 条，期望 11 / 354", len(levels), total)
	}
	if len(q.RemoveClear) != OdysseyQuestRemoveTotal {
		t.Fatalf("[remove clear quest] %d 条，期望 10", len(q.RemoveClear))
	}
	if len(q.Show) != OdysseyQuestShowTotal {
		t.Fatalf("[show quest] %d 条，期望 2", len(q.Show))
	}
	branchLevels := map[byte]bool{}
	for _, b := range q.Branches {
		branchLevels[b.Level] = true
	}
	if len(branchLevels) != OdysseyBranchLevelCount || !branchLevels[80] || !branchLevels[102] || !branchLevels[115] {
		t.Fatalf("[branch quest] 关卡集合错误: %v", branchLevels)
	}
	if g.GraduateReward != 10420561 {
		t.Fatalf("毕业奖励模板 %d，期望 10420561", g.GraduateReward)
	}
	t.Logf("[quest clear] 级别 %v；毕业盒 %d", levels, g.GraduateReward)
}

// 真源语义：12911 被 [remove clear quest] 从 102 主线救回；22987 只在分支表。
func TestOdysseyClearedAtSemantics(t *testing.T) {
	g, e := LoadOdysseyGrowth("../../configs/odyssey-growth-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	q := g.Quests
	full := q.ClearedAt(OdysseyGraduationLevelV)
	// 354 条跨级有 20 个重复 id（并集 334），移除表 10 条里只有 12911 真正
	// 命中并集（其余是防御性行），所以满级清除集是 333 条。
	if len(full) != 333 {
		t.Fatalf("ClearedAt(115) %d 条，期望 333（并集 334 减去 12911）", len(full))
	}
	contains := func(ids []uint32, id uint32) bool {
		for _, v := range ids {
			if v == id {
				return true
			}
		}
		return false
	}
	if contains(full, 12911) {
		t.Fatal("12911 应被移除表救回")
	}
	if contains(full, 22987) {
		t.Fatal("深渊引导 22987 不得进入清除集")
	}
	if !contains(full, 13111) {
		t.Fatal("100 级表任务丢失")
	}
	low := q.ClearedAt(17)
	if contains(low, 13111) {
		t.Fatal("低等级清除集混入了高等级表")
	}
	// 降序包含：ClearedAt(17) ⊂ ClearedAt(115)。
	for _, id := range low {
		if !contains(full, id) {
			t.Fatalf("低等级任务 %d 未包含在满级集合里", id)
		}
	}
	// 结果必须升序，保证落库确定性。
	for i := 1; i < len(full); i++ {
		if full[i] <= full[i-1] {
			t.Fatalf("清除集不是严格升序: %v", full[i-1:i+1])
		}
	}
}

// 分支表按 (level, profession) 取行；-1 为全职业行。
func TestOdysseyBranchQuestsUpTo(t *testing.T) {
	g, e := LoadOdysseyGrowth("../../configs/odyssey-growth-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	q := g.Quests
	contains := func(ids []uint32, id uint32) bool {
		for _, v := range ids {
			if v == id {
				return true
			}
		}
		return false
	}
	b80 := q.BranchQuestsUpTo(80, 0)
	if len(b80) != 1 || !contains(b80, 3868) {
		t.Fatalf("80 级全职业分支应为 3868: %v", b80)
	}
	b80job11 := q.BranchQuestsUpTo(80, 11)
	if len(b80job11) != 2 || !contains(b80job11, 3869) {
		t.Fatalf("职业 11 在 80 级应有 3868+3869: %v", b80job11)
	}
	b115 := q.BranchQuestsUpTo(OdysseyGraduationLevelV, 0)
	if !contains(b115, 12884) || !contains(b115, 22987) {
		t.Fatalf("115 级分支缺 12884/22987: %v", b115)
	}
	// 源顺序钉死：80 表是 -1→3868、11→3869。
	if q.Branches[0].Level != 80 || q.Branches[0].Job != -1 || q.Branches[0].Quest != 3868 {
		t.Fatalf("分支首行漂移: %+v", q.Branches[0])
	}
}
