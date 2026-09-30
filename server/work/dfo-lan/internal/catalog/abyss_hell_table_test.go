package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"dfolan/internal/catalog/pvf"
)

// [MERGE-20260928-ABYSS-HELL-TABLE] 深渊专用的 etc/itemdropinfo_monster_hell.etc
// 必须真的被 ImportLoot 读进 Rules —— 不是被容错分支静默跳过。
//
// 背景：这张表一直在 client-build 的 PVF 里（index 5055835，645 cells），但导入
// 清单漏了它，深渊副本只能回落普通怪的 monseter 表，实机表现是「深渊只掉紫装」。
//
// 同时锁住它与 monseter 的**结构差异**：深渊表的 [basis of rarity dicision] 是
// 2 行 × 9 列且**没有** [drop prob] 段，用来防止后续把两张表当成同一套概率结构。
func TestAbyssHellDropTableIsImported(t *testing.T) {
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
	c, e := ImportLoot(a, 130)
	if e != nil {
		t.Fatal(e)
	}

	const hell = "etc/itemdropinfo_monster_hell.etc"
	if _, ok := c.Rules[hell]; !ok {
		t.Fatalf("深渊表未导入；Rules 现有 %d 张：%v", len(c.Rules), keys(c.Rules))
	}
	if len(c.Rules) < 5 {
		t.Fatalf("Rules 应 >= 5 张（含深渊表），实得 %d", len(c.Rules))
	}

	// 深渊表与普通怪表必须都能解析出 [basis of rarity dicision]，
	// 且深渊表**没有** [drop prob] 段 —— 这是两张表结构不同的可证伪证据。
	hellCells := c.Rules[hell].Cells
	monCells := c.Rules["etc/itemdropinfo_monseter.etc"].Cells
	hr := sectionValues(hellCells, "[basis of rarity dicision]")
	mr := sectionValues(monCells, "[basis of rarity dicision]")
	t.Logf("hell  rarity cells=%d  monseter rarity cells=%d", len(hr), len(mr))
	if len(hr) == 0 {
		t.Fatal("深渊表缺 [basis of rarity dicision]")
	}
	if len(hr) == len(mr) {
		t.Fatalf("深渊表与普通表的 rarity 段长度相同(%d)，与实测不符", len(hr))
	}
	if got := sectionValues(hellCells, "[drop prob]"); len(got) != 0 {
		t.Fatalf("深渊表不该有 [drop prob]，实得 %d 个值", len(got))
	}
	if got := sectionValues(monCells, "[drop prob]"); len(got) == 0 {
		t.Fatal("普通表应含 [drop prob]")
	}
}

func sectionValues(cells []pvf.Token, header string) []int32 {
	for i, c := range cells {
		if c.Type != 3 || c.Text != header {
			continue
		}
		var out []int32
		for j := i + 1; j < len(cells); j++ {
			if cells[j].Type == 3 && cells[j].Text != "" {
				break
			}
			out = append(out, cells[j].Value)
		}
		return out
	}
	return nil
}

func keys(m map[string]ScriptRecord) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
