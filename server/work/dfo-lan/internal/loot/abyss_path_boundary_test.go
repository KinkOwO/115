package loot

import (
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
)

// [MERGE-20260928-DUNGEON-GROUP-DROP] 分界回归：实机深渊 100005014 打不出装备时，
// 先确认它到底走哪条路径。
//
// 事实（只读观察）：
//   - 100005014 不在 etc/dungeondropinfo.cos 的 198 个副本里 → DropInfoByID 应返回 false
//   - 它有 [difficulty dropitem group list]（内含 [normal group index] = 1 21251 1 21476）
//   - 它有 [exclude monster random drop] 与 [exclude monster card area drop]
//
// 结论：它走**原有全局表**路径，本轮新加的组索引分支不该碰它。
// 若这里断言失败，说明新路径的分界写错了，那才是回归。
func TestAbyss100005014TakesLegacyPath(t *testing.T) {
	cat := loadDropInfoCatalog(t)

	const did = 100005014
	if _, has := cat.DropInfoByID(did); has {
		t.Errorf("100005014 意外出现在 dungeondropinfo 里 —— 新路径会接管它")
	} else {
		t.Logf("100005014 不在 dungeondropinfo → 走原有全局表（ok=false）")
	}

	// 直接调用一次，确认返回 ok=false（调用方随后保持旧行为）
	out, ok, e := RollDungeonGroups(cat, Rules{Denominator: 1000000}, 1, DungeonGroupDropRequest{
		DungeonID: did, MonsterKind: "boss", Difficulty: 0, Rarity: -1,
	})
	if e != nil {
		t.Fatalf("不该报错: %v", e)
	}
	if ok {
		t.Errorf("100005014 不该走新路径（ok 应为 false），实得 ok=%v awards=%v", ok, out.Awards)
	}
	t.Logf("RollDungeonGroups(100005014) → ok=%v awards=%d", ok, len(out.Awards))
}

// 顺带锁住：100005014 的两个 exclude 标记确实存在，且服务端目前只认 [exclude gold drop]。
// 这条记录的是**既有缺口**，不是本轮的回归 —— 用日志形式呈现，便于后续补齐。
func TestAbyss100005014ExcludeMarkers(t *testing.T) {
	c := catalog.LoadNativeDungeons(t, 100005014)
	d, ok := c.Dungeons[100005014]
	if !ok {
		t.Skip("100005014 not in catalog")
	}
	mark := func(want string) bool {
		for _, cell := range d.Script.Cells {
			if cell.Type == 3 && cell.Text == want {
				return true
			}
		}
		return false
	}
	for _, m := range []string{
		"[exclude gold drop]",
		"[exclude monster random drop]",
		"[exclude monster card area drop]",
		"[difficulty dropitem group list]",
	} {
		t.Logf("   %-38s %v", m, mark(m))
	}
	// [MERGE-20260929-EXCLUDE-RANDOM] 状态更新：
	//   [exclude gold drop]               已实现
	//   [exclude monster random drop]     已实现（本次）
	//   [exclude monster card area drop]  刻意不实现（无对应路径，不猜）
	if mark("[exclude monster card area drop]") {
		t.Log("注意：[exclude monster card area drop] 未实装 —— 服务端没有「卡片区域掉落」" +
			"这条独立路径（怪物卡经宝珠/附魔的 [monster card id] 关联），做了就是照猜。")
	}
}

func loadPVFOrSkip(t *testing.T, p string) *pvf.Archive {
	t.Helper()
	a, e := catalog.OpenTestArchiveCached(p, "")
	if e != nil {
		t.Skip("cannot open PVF:", e)
	}
	return a
}
