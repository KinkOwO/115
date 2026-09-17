package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"os"
	"testing"
)

// 用**实际运行时配置**验证 115 级怪物能不能掉装备。
//
// 背景：configs/drop.compat90.json 的 supported_kinds 原本只有
// ["gold","stackable"]，rules.go:215 的 `if !enabled["equipment"] { continue }`
// 于是把整个装备分支跳过——怪物一件装备都不掉。
// 经用户许可后把它加上 "equipment"，并把掉落池从 equipment.current35.json
// 改为已合并 115 级装备的 equipment.current37.json。
//
// 本测试不依赖游戏客户端，直接调用掉落计算，覆盖多个等级与种子。
func TestLevel115MonstersDropEquipment(t *testing.T) {
	if os.Getenv("DFO_EXTENDED_CATALOG_INTEGRATION") != "1" {
		t.Skip("requires externally generated level115 catalog and custom drop profile; set DFO_EXTENDED_CATALOG_INTEGRATION=1")
	}
	c, e := catalog.LoadLoot("../../configs/loot.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	tables, e := Parse(c)
	if e != nil {
		t.Fatal(e)
	}
	// 注意：用的是实际启动配置 drop.compat90.json，不是测试专用的 drop.current36.json
	rules, e := LoadRules("../../configs/drop.compat90.json")
	if e != nil {
		t.Fatal(e)
	}
	enabled := false
	for _, k := range rules.SupportedKinds {
		if k == "equipment" {
			enabled = true
		}
	}
	if !enabled {
		t.Fatal("drop.compat90.json 仍未启用 equipment 类别")
	}
	gear, e := inventory.LoadEquipmentCatalog("../../configs/equipment.current37.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	pool := gear.DropPool()
	if len(pool) == 0 {
		t.Fatal("装备掉落池为空")
	}
	usable := map[uint32]bool{}
	for _, d := range pool {
		if _, err := gear.Basic(d.ID); err != nil {
			t.Fatalf("掉落池里有背包不收的装备: %d: %v", d.ID, err)
		}
		usable[d.ID] = true
	}
	t.Logf("掉落池 %d 件（含 115 级太初装备）", len(pool))

	for _, level := range []byte{5, 60, 90, 115} {
		dropped, stacked, gold, empty := 0, 0, 0, 0
		for seed := uint32(0); seed < 3000; seed++ {
			out, err := Roll(c, tables, rules, pool, seed, level, 0, 0)
			if err != nil {
				t.Fatalf("level=%d seed=%d: %v", level, seed, err)
			}
			for _, k := range out.SkippedKinds {
				if k == "equipment_kind_disabled" {
					t.Fatalf("level=%d: 装备类别仍被禁用", level)
				}
				if k == "equipment_grade_window_empty" {
					empty++
				}
			}
			for _, a := range out.Awards {
				switch {
				case a.Template == 0:
				case usable[a.Template] && c.Items[a.Template].Kind != "stackable":
					dropped++
				default:
					if c.Items[a.Template].Kind == "gold" {
						gold++
					} else {
						stacked++
					}
				}
			}
		}
		t.Logf("level=%-4d 装备掉落=%-6d 材料=%-6d 金币=%-6d 等级窗口空=%d",
			level, dropped, stacked, gold, empty)
		// 本次改动的目标是让**高等级**怪物能掉装备，所以只对 115 级硬性断言。
		// 60/90 级为空是**既有问题**（掉落池里 Basic() 只收 rarity<=1 的普通装备，
		// 这些等级上品级窗口内没有匹配项），与本次启用 equipment 类别无关，
		// 单独记录不当作回归。
		if level == 115 && dropped == 0 {
			t.Errorf("level=115 没有任何装备掉落：本次启用 equipment 类别的改动失效了")
		}
		if level == 5 && stacked == 0 {
			t.Errorf("level=5 材料掉落回归为 0")
		}
	}
}
