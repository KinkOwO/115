package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"dfolan/internal/catalog/pvf"
)

// [MERGE-20260928-DUNGEON-DROPINFO] 验证 etc/dungeondropinfo.cos 经 ImportLoot 进
// 服务端，并且 [dungeon type] 只有 dgn_normal / dgn_hell 两种取值。
//
// 这张表是「深渊按深渊掉落、常规按常规掉落」的**权威依据**：深渊由 `dgn_hell`
// 明确标记，不靠副本名或段标记猜。实机 100003295/6/7 是仅有的三个深渊副本。
func TestParseDungeonDropInfo(t *testing.T) {
	a := openSnapshotForDropInfo(t)

	cat, e := ImportLoot(a, 130)
	if e != nil {
		t.Fatal(e)
	}
	if len(cat.DungeonDropInfo) == 0 {
		t.Fatal("ImportLoot 没有带上 dungeondropinfo")
	}
	if cat.DungeonDropInfoSource.Path == "" || len(cat.DungeonDropInfoSource.SHA256) != 64 {
		t.Fatalf("dungeondropinfo 来源未记录: %+v", cat.DungeonDropInfoSource)
	}
	t.Logf("ImportLoot 带上 dungeondropinfo：副本数=%d source=%s",
		len(cat.DungeonDropInfo), cat.DungeonDropInfoSource.Path)

	// ---- 1) [dungeon type] 只有两种取值（核心不变量）----
	byType := map[string]int{}
	byGrade := map[string]int{}
	groups := map[uint32]bool{}
	for _, entries := range cat.DungeonDropInfo {
		for _, en := range entries {
			byType[en.DungeonType]++
			byGrade[en.Grade]++
			if en.DropGroup != 0 {
				groups[en.DropGroup] = true
			}
		}
	}
	t.Logf("按类型=%v 按品级=%v 引用组数=%d", byType, byGrade, len(groups))
	for k := range byType {
		if k != "dgn_normal" && k != "dgn_hell" {
			t.Fatalf("意外的 [dungeon type]: %q", k)
		}
	}
	if byType["dgn_hell"] == 0 {
		t.Fatal("没有解析出任何 dgn_hell（深渊）条目")
	}

	// ---- 2) 深渊副本必须是少数几个，且 IsAbyssDungeon 判得出来 ----
	abyss := []uint32{}
	for did := range cat.DungeonDropInfo {
		if cat.IsAbyssDungeon(did) {
			abyss = append(abyss, did)
		}
	}
	t.Logf("IsAbyssDungeon 判为深渊的副本(%d 个): %v", len(abyss), abyss)
	if len(abyss) == 0 {
		t.Fatal("没有任何副本被判为深渊")
	}
	if len(abyss) > 50 {
		t.Fatalf("深渊副本数异常偏多: %d", len(abyss))
	}

	// ---- 3) 深渊副本的每条记录都必须是 dgn_hell，且组可解析出物品 ----
	for _, did := range abyss {
		ents, _ := cat.DropInfoByID(did)
		for _, en := range ents {
			if en.DungeonType != "dgn_hell" {
				t.Fatalf("深渊副本 %d 出现非 dgn_hell 条目: %+v", did, en)
			}
			if len(en.RateList) == 0 {
				t.Fatalf("深渊副本 %d 品级 %s 缺 rate list", did, en.Grade)
			}
			if en.DropGroup == 0 {
				continue
			}
			g, ok := cat.DropGroupByID(en.DropGroup)
			if !ok {
				t.Logf("   副本 %d 品级 %s 的组 %d 不在 DropGroups 里", did, en.Grade, en.DropGroup)
				continue
			}
			items := len(g.Explicit) + len(g.Smart)
			t.Logf("   副本 %d 品级 %-10s 组 %-6d 物品数=%d",
				did, en.Grade, en.DropGroup, items)
		}
	}
}

// ---- 4) 普通副本不该被判为深渊 ----
func TestNormalDungeonIsNotAbyss(t *testing.T) {
	a := openSnapshotForDropInfo(t)
	cat, e := ImportLoot(a, 130)
	if e != nil {
		t.Fatal(e)
	}
	n := 0
	for did, ents := range cat.DungeonDropInfo {
		if cat.IsAbyssDungeon(did) {
			continue
		}
		for _, en := range ents {
			if en.DungeonType != "dgn_normal" {
				t.Fatalf("非深渊副本 %d 出现 %q 条目", did, en.DungeonType)
			}
			n++
			if n > 200 {
				break
			}
		}
		if n > 200 {
			break
		}
	}
	t.Logf("抽查非深渊条目 %d 条，全部为 dgn_normal", n)
}

func openSnapshotForDropInfo(t *testing.T) *pvf.Archive {
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
	return a
}
