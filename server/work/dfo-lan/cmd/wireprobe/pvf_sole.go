package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"log"
	"strconv"
	"strings"
)

// 秘宝精度提升（CMD2288）的直读准备与安装。
//
// 源（唯一内容真源 = 内层 PVF）：
//
//	etc/115lvability/soleequipmentsystem.cos   规则表（每件秘宝的 [item index] /
//	                                           [max quality] / [quality need materials] / [quality group]）
//
// 与其它直读目录一致：**不读任何 configs/*.json**，解析失败直接报错（不静默回落）。
func preparePVFSoleEquipment(c *pvfCoreCatalogs, s *gamedata.Source) error {
	rules := c.soleRules
	if rules == nil {
		imported, err := s.SoleEquipment()
		if err != nil {
			return err
		}
		rules = imported
	}
	c.soleRules = rules
	s.ReleaseReadCaches()
	log.Printf("PVF sole equipment prepared: items=%d %s", len(rules.Items), soleItemsSummary(rules))
	return nil
}

// soleItemsSummary 把每件秘宝的模板与上限拼成一行（启动日志的规模核对用）。
func soleItemsSummary(rules *catalog.SoleEquipmentRules) string {
	var b strings.Builder
	for _, template := range rules.Templates() {
		info, _ := rules.Info(template)
		b.WriteString(" ")
		b.WriteString(strconv.FormatUint(uint64(template), 10))
		b.WriteString("(max=")
		b.WriteString(strconv.Itoa(info.MaxQuality))
		b.WriteString(",groups=")
		b.WriteString(strconv.Itoa(len(info.Groups)))
		b.WriteString(")")
	}
	return b.String()
}

// installSoleEquipment 把直读投影装进 inventory 的运行期注入点。
func (c pvfCoreCatalogs) installSoleEquipment() (func(), error) {
	if c.soleRules == nil {
		return func() {}, nil
	}
	inventory.SetSoleEquipmentRules(c.soleRules)
	return func() { inventory.SetSoleEquipmentRules(nil) }, nil
}
