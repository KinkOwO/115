package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"log"
)

// 装备调适（CMD2258）的直读准备与安装。
//
// 源（唯一内容真源 = 内层 PVF）：
//
//	etc/115lvability/equipmentawakeningoptionsystem.cos   规则表（阶段上限/成本/返还/成功率/升品映射）
//	etc/115lvability/equipmentawakeningoption.lst          选项索引（`[equipment awakening option]` 的 ID → 加成表）
//
// 与其它直读目录一致：**不读任何 configs/*.json**，解析失败直接报错（不静默回落）。
func preparePVFEquipmentAwakening(c *pvfCoreCatalogs, s *gamedata.Source) error {
	rules := c.awakeningRules
	if rules == nil {
		imported, err := s.EquipmentAwakening()
		if err != nil {
			return err
		}
		rules = imported
	}
	options := c.awakeningOptions
	if options == nil {
		imported, err := s.EquipmentAwakeningOptions()
		if err != nil {
			return err
		}
		options = imported
	}
	c.awakeningRules, c.awakeningOptions = rules, options
	s.ReleaseReadCaches()
	log.Printf("PVF equipment awakening prepared: max=%d conditions=%d stage-tables=%d upgrade-sources=%d options=%d",
		rules.MaxLevel, len(rules.Infos), awakeningStageTables(rules), len(rules.Templates()), len(options.Entries))
	return nil
}

// awakeningStageTables 统计规则里声明的成本行总数（启动日志的规模核对用）。
func awakeningStageTables(rules *catalog.EquipmentAwakeningRules) int {
	total := 0
	for _, info := range rules.Infos {
		for _, group := range info.Groups {
			total += len(group.Rows)
		}
	}
	return total
}

// installEquipmentAwakening 把直读投影装进 inventory 的运行期注入点。
func (c pvfCoreCatalogs) installEquipmentAwakening() (func(), error) {
	if c.awakeningRules == nil {
		return func() {}, nil
	}
	inventory.SetEquipmentAwakeningRules(c.awakeningRules)
	return func() { inventory.SetEquipmentAwakeningRules(nil) }, nil
}
