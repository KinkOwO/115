package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"sort"
	"testing"
)

// TestSetPointAwakeningThreeFamiliesNative 钉住「写调适 3 能不能加分」的完整判据。
//
// 背景（2026-10-07 取证）：`record[170]` 的调适档位同时就是
// `etc/115lvability/setpointinfo.cos` 的 `[awakening]`。套装积分要查两层表：
//
//	模板 --(etc/equipmentgrouping.etc 的 [ability group])--> 能力组号
//	能力组号 + 调适档位 --(setpointinfo.cos 的 [info])--> 每件积分
//
// `[part set index]` 为 -1 的行用该件 `.equ` 自己的 `[part set index]` 补上
// （internal/character/fame.go：`if id == -1 { id = set }`）。
//
// 结论：**能靠调适 3 加分的模板不止「稀有度 4」**。源里恰好有 5 个部位家族
// （能力组 51/52/53/58/190 与各部位专用组），每个部位的档位是
// 稀3 → +30（家族 51）、稀6 → +30（家族 52）、稀4 → +30（家族 53 或 58）、
// 稀4 的 190 家族 → +30（265 分那一族）。稀有度 8（秘宝/太初星蕴石）与
// 武器/誓约/星蕴石在表里没有可命中行，写几都是 0。
//
// 本测试把「哪些稀有度确实能加分」压到当前源上，防止再按「只有稀 4」下结论。
//
//	DFO_PVF_CORE_TEST_ARCHIVE=<integ>/Script.inner.pvf
//	go test ./internal/catalog/ -run SetPointAwakeningThreeFamiliesNative -count=1 -v
func TestSetPointAwakeningThreeFamiliesNative(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for set point family parity")
	}
	a, err := OpenTestArchiveCached(p, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	rules, err := ImportPointRules(a)
	if err != nil {
		t.Fatal(err)
	}
	groups, err := importTestAbilityGroups(t, a)
	if err != nil {
		t.Fatal(err)
	}

	// 每个能力组在各档位下的分值（只统计 part set index >= 0 的行 + -1 行的原始值）。
	type rankValues map[int32][]uint32
	byGroup := map[uint32]rankValues{}
	for _, r := range rules.Set.Rules {
		if byGroup[r.Group] == nil {
			byGroup[r.Group] = rankValues{}
		}
		byGroup[r.Group][r.Awakening] = append(byGroup[r.Group][r.Awakening], r.Value)
	}

	// 家族 51/52/53/58/190 必须都同时有「低档」和「高档」，且高档更大 —— 这就是
	// 「调适 3 加分」在源里的形态。
	cases := []struct {
		group           uint32
		low, high       uint32
		wantAwakeningHi int32
	}{
		{51, 115, 145, 3},
		{52, 165, 195, 3},
		{53, 215, 245, 3},
		{58, 215, 245, 3},
		{190, 235, 265, 3},
	}
	for _, c := range cases {
		rows := byGroup[c.group]
		if rows == nil {
			t.Errorf("set point group %d missing from %s", c.group, SetPointInfoPath)
			continue
		}
		if !hasValue(rows[0], c.low) {
			t.Errorf("group %d awakening 0 = %v, want to contain %d", c.group, rows[0], c.low)
		}
		if !hasValue(rows[c.wantAwakeningHi], c.high) {
			t.Errorf("group %d awakening %d = %v, want to contain %d", c.group, c.wantAwakeningHi, rows[c.wantAwakeningHi], c.high)
		}
	}

	// 反例：这些能力组在表里**没有**任何行 —— 写几都是 0，不要硬塞调适。
	// 注意 306 是耳环的组之一，它在表里**有**一行（[awakening] 0 / [part set index] -1 /
	// value 145），但耳环 `.equ` 本身没有 `[part set index]` 字段 ⇒ `-1` 无处回退 ⇒ 该行
	// 永远命不中，实际仍是 0 分。所以判据不能只看「组在表里有没有行」。
	for _, g := range []uint32{244, 256, 257, 159, 310, 313} {
		if rows, ok := byGroup[g]; ok {
			t.Errorf("ability group %d unexpectedly has set point rows: %v", g, rows)
		}
	}
	if rows, ok := byGroup[306]; !ok {
		t.Errorf("ability group 306 should carry the unattributable -1 row")
	} else {
		t.Logf("group 306 有行 %v，但耳环无 [part set index] ⇒ 命不中（0 分）", rows)
	}

	// 成员表本身要非空且分组号连续可用（防止解析器把 [index] 读错位）。
	if len(groups) < 1000 {
		t.Fatalf("ability group membership = %d, want > 1000", len(groups))
	}
	t.Logf("setpointinfo rows=%d ability groups=%d", len(rules.Set.Rules), len(groups))
}

// TestSetPointsRealArchiveNative 端到端核对：`ImportPointRules` 装载的能力组 →
// `SetPoints` 的实际得分。这条覆盖"两层映射接起来之后到底算几分"，是修复的主证据。
//
//	DFO_PVF_CORE_TEST_ARCHIVE=<integ>/Script.inner.pvf
//	go test ./internal/catalog/ -run SetPointsRealArchiveNative -count=1 -v
func TestSetPointsRealArchiveNative(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for set point end-to-end parity")
	}
	a, err := OpenTestArchiveCached(p, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	rules, err := ImportPointRules(a)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules.AbilityGroups) == 0 {
		t.Fatal("ImportPointRules must fill AbilityGroups; SetPoints cannot work without it")
	}
	t.Logf("ability groups=%d set rows=%d oath rows=%d",
		len(rules.AbilityGroups), len(rules.Set.Rules), len(rules.Oath.Rules))

	// 五件 190 家族防具（套装号 16203）+ 一件稀有度 8 秘宝（能力组不在表里）。
	items := []PointItem{
		{Template: 100051381, Awakening: 0, PartSetIndex: 16203},
		{Template: 100101260, Awakening: 0, PartSetIndex: 16203},
		{Template: 100151201, Awakening: 0, PartSetIndex: 16203},
		{Template: 100201173, Awakening: 0, PartSetIndex: 16203},
		{Template: 100251213, Awakening: 0, PartSetIndex: 16203},
		{Template: 100391142, Awakening: 0, PartSetIndex: -1},
	}
	for _, it := range items {
		t.Logf("  %d groups=%v", it.Template, rules.AbilityGroups[it.Template])
	}
	// 调适 0：5 × 235 = 1175，命中 5 件（秘宝的能力组在表里没有行 ⇒ 0）。
	if total, hits := rules.SetPoints(items); total != 5*235 || hits != 5 {
		t.Errorf("awakening 0: got %d hits=%d, want %d/5", total, hits, 5*235)
	}
	// 调适 3：5 × 265 = 1325 —— 这就是"调适写对了"在服务端侧应得的分数。
	for i := range items {
		items[i].Awakening = 3
	}
	if total, hits := rules.SetPoints(items); total != 5*265 || hits != 5 {
		t.Errorf("awakening 3: got %d hits=%d, want %d/5", total, hits, 5*265)
	}
	// 反例一：稀 6 肩 100151213 属能力组 52，而表里 52 只有档位 0（165）⇒ 写调适 3 反而 0 分。
	// 这正是"档位只精确匹配、不能回退 0 档"的依据。
	if total, _ := rules.SetPoints([]PointItem{{Template: 100151213, Awakening: 0, PartSetIndex: 16299}}); total != 165 {
		t.Errorf("100151213 awakening 0 = %d, want 165", total)
	}
	if total, hits := rules.SetPoints([]PointItem{{Template: 100151213, Awakening: 3, PartSetIndex: 16299}}); total != 0 || hits != 0 {
		t.Errorf("100151213 awakening 3 = %d/%d, want 0/0 (group 52 has no rank 3 row)", total, hits)
	}
	// 反例二：星蕴石 100401606 的能力组（244/256）在表里没有行 ⇒ 恒 0，与调适无关。
	if total, hits := rules.SetPoints([]PointItem{{Template: 100401606, Awakening: 3, PartSetIndex: -1}}); total != 0 || hits != 0 {
		t.Errorf("100401606 = %d/%d, want 0/0", total, hits)
	}
	// 反例三：耳环 100391142 走 group 306，该组有 -1 行，但耳环本身没有 `[part set index]`
	// ⇒ 归属不成立、不累加（`-1` 无处回退）。这是"归属不确定就不瞎算"的依据。
	if total, hits := rules.SetPoints([]PointItem{{Template: 100391142, Awakening: 0, PartSetIndex: -1}}); total != 0 || hits != 0 {
		t.Errorf("100391142 = %d/%d, want 0/0 (no part set index to attribute the -1 row)", total, hits)
	}
}

func hasValue(xs []uint32, want uint32) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// importTestAbilityGroups 是 internal/toolcmd/setpointdiag 同一口径的测试侧副本：
// `[index] <组号>` 后面跟 `[list] <模板…> [/list]`。
func importTestAbilityGroups(t *testing.T, a *pvf.Archive) (map[uint32][]uint32, error) {
	t.Helper()
	script, err := ResolveScript(a, "etc/equipmentgrouping.etc")
	if err != nil {
		return nil, err
	}
	out := map[uint32][]uint32{}
	var group int64 = -1
	pendingIndex, inList := false, false
	for _, tok := range script.Cells {
		if tok.Type == 3 {
			switch tok.Text {
			case "[ability group]", "[/ability group]":
				group, pendingIndex, inList = -1, false, false
			case "[index]":
				pendingIndex, inList = true, false
			case "[list]":
				inList = true
			case "[/list]":
				inList = false
			}
			continue
		}
		if tok.Type != 0 {
			continue
		}
		if pendingIndex {
			group, pendingIndex = int64(tok.Value), false
			continue
		}
		if inList && group >= 0 && tok.Value > 0 {
			out[uint32(tok.Value)] = append(out[uint32(tok.Value)], uint32(group))
		}
	}
	keys := make([]uint32, 0, len(out))
	for k := range out {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return out, nil
}
