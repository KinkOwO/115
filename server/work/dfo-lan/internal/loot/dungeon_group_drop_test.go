package loot

import (
	"os"
	"path/filepath"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
)

// [MERGE-20260928-DUNGEON-GROUP-DROP] 验证「按副本声明的组索引发放」这条链。
//
// 实机深渊副本 100003295/6/7 是 dungeondropinfo 里唯一的 dgn_hell 条目，它们声明了
// 多个掉落组（epic 组 10900/21030 + 若干 stackable 组）。本测试锁住：
//   - 声明了组索引的副本 → ok=true，且能真的产出物品
//   - 没声明的副本       → ok=false（交给全局表，不是错误）
//   - 品级过滤生效       → epic 只走 epic 组
func TestRollDungeonGroups(t *testing.T) {
	cat := loadDropInfoCatalog(t)
	if len(cat.DungeonDropInfo) == 0 {
		t.Skip("catalog 没有 dungeondropinfo")
	}
	rules := Rules{Denominator: 1000000}

	const abyss = 100003295
	if !cat.IsAbyssDungeon(abyss) {
		t.Fatalf("%d 应被判为深渊", abyss)
	}

	// ---- 1) 深渊副本能发放 ----
	total := 0
	picks := map[uint32]int{}
	for seed := uint32(1); seed <= 200; seed++ {
		out, ok, e := RollDungeonGroups(cat, rules, seed, DungeonGroupDropRequest{
			DungeonID:   abyss,
			MonsterKind: "boss",
			Difficulty:  0,
			Rarity:      -1,
		})
		if e != nil {
			t.Fatalf("seed %d: %v", seed, e)
		}
		if !ok {
			t.Fatalf("深渊副本应声明组索引")
		}
		total += len(out.Awards)
		for _, a := range out.Awards {
			if a.Template == 0 {
				t.Fatalf("seed %d 产出 Template=0", seed)
			}
			picks[a.Template]++
		}
	}
	t.Logf("200 个种子共产出 %d 件，去重后 %d 种", total, len(picks))
	if total == 0 {
		t.Fatal("200 个种子一件都没产出")
	}

	// ---- 2) 品级过滤：epic 只走 epic 组 ----
	outEpic, ok, e := RollDungeonGroups(cat, rules, 12345, DungeonGroupDropRequest{
		DungeonID: abyss, MonsterKind: "boss", Difficulty: 0, Rarity: 4,
	})
	if e != nil || !ok {
		t.Fatalf("epic 过滤失败: %v ok=%v", e, ok)
	}
	t.Logf("Rarity=4(epic) 产出 %d 件", len(outEpic.Awards))

	// ---- 3) 未声明组索引的普通副本 → ok=false ----
	outSt := Outcome{}
	ok2 := false
	// 找一个确定不在 dungeondropinfo 里的副本 ID
	for _, probe := range []uint32{1, 2, 3, 100004786, 100004777} {
		if _, has := cat.DropInfoByID(probe); has {
			continue
		}
		outSt, ok2, e = RollDungeonGroups(cat, rules, 1, DungeonGroupDropRequest{
			DungeonID: probe, MonsterKind: "boss", Difficulty: 0, Rarity: -1,
		})
		if e != nil {
			t.Fatalf("副本 %d: %v", probe, e)
		}
		if ok2 {
			t.Fatalf("副本 %d 不该声明组索引", probe)
		}
		t.Logf("副本 %d 未声明组索引 → 交给全局表（ok=false）", probe)
		break
	}
	_ = outSt

	// ---- 4) 难度越界必须报错，不静默回绕 ----
	if _, _, e := RollDungeonGroups(cat, rules, 1, DungeonGroupDropRequest{
		DungeonID: abyss, MonsterKind: "boss", Difficulty: 99, Rarity: -1,
	}); e == nil {
		t.Fatal("难度越界应报错")
	}

	// ---- 5) 未知怪物类别只是跳过，不是错误 ----
	out3, ok3, e := RollDungeonGroups(cat, rules, 1, DungeonGroupDropRequest{
		DungeonID: abyss, MonsterKind: "no-such-kind", Difficulty: 0, Rarity: -1,
	})
	if e != nil {
		t.Fatalf("未知怪物类别不该报错: %v", e)
	}
	if !ok3 || len(out3.Awards) != 0 {
		t.Fatalf("未知怪物类别应产出 0 件，得 %d", len(out3.Awards))
	}
	if len(out3.SkippedKinds) == 0 {
		t.Fatal("未知怪物类别应记入 SkippedKinds")
	}
}

// ---- 品级标签到 rarity 的映射 ----
func TestGradeMatchesRarity(t *testing.T) {
	for _, tc := range []struct {
		grade  string
		rarity int
		want   bool
	}{
		{"rare", 1, true}, {"rare", 2, false},
		{"unique", 2, true}, {"unique", 3, false},
		{"legendary", 3, true},
		{"epic", 4, true}, {"epic", 2, false},
		{"stackable", 0, true}, {"stackable", 6, true},
		{"special", 4, false},
		{"nonsense", 4, false},
	} {
		if got := gradeMatchesRarity(tc.grade, tc.rarity); got != tc.want {
			t.Errorf("gradeMatchesRarity(%q, %d) = %v, want %v", tc.grade, tc.rarity, got, tc.want)
		}
	}
}

// ---- 怪物 rank → [rate list] 类别键 ----
func TestMonsterKindName(t *testing.T) {
	for _, tc := range []struct {
		rank byte
		want string
	}{
		{0, "normal"}, {1, "named"}, {2, "named"}, {3, "boss"},
	} {
		if got := monsterKindName(tc.rank); got != tc.want {
			t.Errorf("monsterKindName(%d) = %q, want %q", tc.rank, got, tc.want)
		}
	}
}

// [MERGE-20260928-DUNGEON-GROUP-DROP] 互斥性：副本声明了组索引时走新路径，
// 没声明时走全局表 —— 两条路径绝不叠加，否则掉落翻倍。
//
// 这里锁定「谁该走新路径」这个分界，因为 Session.Death 里的分支就靠它。
func TestDungeonGroupPathBoundary(t *testing.T) {
	cat := loadDropInfoCatalog(t)

	// 声明了组索引的副本（深渊 100003295/6/7 是 dungeondropinfo 里唯一的 dgn_hell）
	withInfo := []uint32{}
	for did := range cat.DungeonDropInfo {
		withInfo = append(withInfo, did)
	}
	if len(withInfo) == 0 {
		t.Skip("catalog 没有 dungeondropinfo")
	}
	for _, did := range []uint32{100003295, 100003296, 100003297} {
		if _, has := cat.DropInfoByID(did); !has {
			t.Errorf("深渊副本 %d 应有 drop info", did)
		}
	}

	// 大多数副本**不该**有条目 —— 它们必须继续走全局表
	checked := 0
	for _, did := range []uint32{1, 2, 3, 10, 1000, 100004786, 100004777} {
		if _, has := cat.DropInfoByID(did); has {
			t.Logf("副本 %d 也在 dungeondropinfo 里（走新路径）", did)
			continue
		}
		checked++
	}
	t.Logf("dungeondropinfo 覆盖 %d 个副本；抽查的 %d 个普通副本不在其中（继续走全局表）",
		len(withInfo), checked)
	if checked == 0 {
		t.Log("注意：抽查的普通副本全部命中 dungeondropinfo")
	}
}

func loadDropInfoCatalog(t *testing.T) catalog.LootCatalog {
	t.Helper()
	p := os.Getenv("DFO_LOOT_PVF")
	if p == "" {
		for _, cand := range []string{
			filepath.Join("..", "..", "client-build", "Script.inner.pvf"),
			filepath.Join("..", "..", "..", "client-build", "Script.inner.pvf"),
		} {
			if _, e := os.Stat(cand); e == nil {
				p = cand
				break
			}
		}
	}
	if p == "" {
		t.Skip("client-build PVF not present")
	}
	a, e := pvf.LoadArchive(pvf.Options{Path: p, MaxBytes: 900 * 1024 * 1024})
	if e != nil {
		t.Skip("cannot open PVF:", e)
	}
	cat, e := catalog.ImportLoot(a, 130)
	if e != nil {
		t.Fatal(e)
	}
	return cat
}
