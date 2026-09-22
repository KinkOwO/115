package main

import (
	"path/filepath"
	"testing"
)

// 网关（cmd/wireprobe/main.go）用 SupplementStackables 把 items.index.json 里的
// 非掉落模板补进 loot 目录；admin 之前漏了这一步，于是"网关里能掉落的东西，
// admin 却发不出来"（奥德赛银币 10418036 / 金币 10418035 按设计不在怪物掉落
// 目录里）。这条用例盯住补挂后的可达性，以及 -item-index 省略时的自动发现。
func TestBuildAwarderSupplementsNonDropTemplates(t *testing.T) {
	loot := filepath.Join("..", "..", "configs", "loot.next25.json")
	rules := filepath.Join("..", "..", "configs", "inventory.next29.json")
	gear := filepath.Join("..", "..", "configs", "equipment.current35.json")
	explicit := filepath.Join("..", "..", "configs", "items.index.json")

	for name, index := range map[string]string{
		"discovered next to the loot catalog": "",
		"explicit -item-index":                explicit,
	} {
		t.Run(name, func(t *testing.T) {
			a, err := buildAwarder(loot, index, rules, gear)
			if err != nil {
				t.Fatal(err)
			}
			if a == nil {
				t.Fatal("no awarder")
			}
			for _, id := range []uint32{10418036, 10418035} {
				if a.Catalog.Items[id].Kind != "stackable" {
					t.Fatalf("template %d unreachable: %+v", id, a.Catalog.Items[id])
				}
			}
		})
	}
}

// 显式给的目录/索引不存在时必须拒绝，而不是带着半装载的目录发货。
func TestBuildAwarderRefusesBadCatalogs(t *testing.T) {
	loot := filepath.Join("..", "..", "configs", "loot.next25.json")
	rules := filepath.Join("..", "..", "configs", "inventory.next29.json")
	gear := filepath.Join("..", "..", "configs", "equipment.current35.json")
	missing := filepath.Join("..", "..", "configs", "does-not-exist.json")

	if _, err := buildAwarder(missing, "", rules, gear); err == nil {
		t.Fatal("missing loot catalog accepted")
	}
	if _, err := buildAwarder(loot, missing, rules, gear); err == nil {
		t.Fatal("explicit missing items index accepted")
	}
	if _, err := buildAwarder(loot, "", missing, gear); err == nil {
		t.Fatal("missing bag rules accepted")
	}
}
