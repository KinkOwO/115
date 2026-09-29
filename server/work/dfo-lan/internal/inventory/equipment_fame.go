package inventory

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strings"
)

type equipmentFameLevel struct {
	level  int32
	fields map[string][]pvf.Token
}

func fameFieldKey(key string) bool {
	switch key {
	case "[fame value]", "[fame table]", "[add fame value]", "[minimum level]", "[maximum level]", "[grade]", "[rarity]", "[part set index]":
		return true
	}
	return false
}

// 名望使用有层级的字段投影，不能把等级分段里的所有同名字段拼成基础名望。
// 顶层重复标量以后一个声明为准；等级分段单独保存，读取时按角色等级选择。
func equipmentFameSections(cells []pvf.Token) (map[string][]pvf.Token, []equipmentFameLevel) {
	fields := map[string][]pvf.Token{}
	var levels []equipmentFameLevel
	closable := map[string]bool{}
	for _, t := range cells {
		if t.Type == 3 && strings.HasPrefix(t.Text, "[/") {
			closable["["+t.Text[2:]] = true
		}
	}
	var stack []string
	key := ""
	for i, t := range cells {
		if t.Type != 3 {
			if fameFieldKey(key) {
				if len(stack) == 0 {
					fields[key] = append(fields[key], t)
				}
				if len(stack) == 1 && stack[0] == "[level section ability]" && len(levels) > 0 {
					row := &levels[len(levels)-1]
					row.fields[key] = append(row.fields[key], t)
				}
			}
			continue
		}
		key = t.Text
		if strings.HasPrefix(key, "[/") {
			if len(stack) > 0 && stack[len(stack)-1] == "["+key[2:] {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if fameFieldKey(key) {
			if len(stack) == 0 {
				fields[key] = nil
			}
			if len(stack) == 1 && stack[0] == "[level section ability]" && len(levels) > 0 {
				levels[len(levels)-1].fields[key] = nil
			}
		}
		if closable[key] {
			if len(stack) == 0 && key == "[level section ability]" && i+1 < len(cells) && cells[i+1].Type == 0 {
				levels = append(levels, equipmentFameLevel{cells[i+1].Value, map[string][]pvf.Token{}})
			}
			stack = append(stack, key)
		}
	}
	return fields, levels
}

// FameDefinition独立解析名望所需的继承字段，保持发放、穿戴的既有解析行为。
func (c *EquipmentCatalog) FameDefinition(id uint32, level byte) (EquipmentDefinition, error) {
	return c.fameDefinition(id, level, 0)
}

func (c *EquipmentCatalog) fameDefinition(id uint32, level byte, depth int) (EquipmentDefinition, error) {
	d, err := c.Definition(id)
	if err != nil {
		return d, err
	}
	if depth >= 8 {
		return d, fmt.Errorf("名望装备继承超过八层：%d", id)
	}
	fields := map[string][]pvf.Token{}
	target := importTarget(d.Fields["[import script]"])
	if target != 0 && target != id {
		base, e := c.fameDefinition(target, level, depth+1)
		if e != nil {
			return d, e
		}
		for k, v := range base.Fields {
			fields[k] = v
		}
	}
	source := d.fameFields
	if source == nil {
		source = d.Fields
	}
	for k, v := range source {
		if fameFieldKey(k) {
			fields[k] = v
		}
	}
	for _, section := range d.fameLevels {
		if section.level <= int32(level) {
			for k, v := range section.fields {
				fields[k] = v
			}
		}
	}
	d.Fields = fields
	return d, nil
}
