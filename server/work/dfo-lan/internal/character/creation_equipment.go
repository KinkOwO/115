package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
)

// creationWorn 按源 [create equipment list] 投影新角色的初始穿戴。
//
// 依据：
//   - catalog 已把该段落解析成 CreateEquipmentBySlot（0 基槽，与 Advancement 同域）
//     和 CreateEquipmentOrder（部位在源文件里的出现顺序）；
//   - 部位到穿戴槽的映射取自 WearRules（native_evidence "1470cb2a0 current equipment
//     type map initializer"），这里不另立一套编号；
//   - 单件能否穿由 inventory.WearableBy 判定（最低等级、可用职业、可用转职三条），
//     与手动穿戴共用同一处判断。
//
// 任何缺失都只跳过这一件：没有该槽、槽值是 0、部位没有槽位映射、装备定义或耐久
// 取不到、装备自身部位与槽标签不一致、不满足穿戴条件、依赖快照不一致。绝不报错，
// 也绝不因此拒绝创建。
//
// 这里完全按源表投影，不做任何职业特例：源里"转职槽 → 该部位装备"的对应关系
// 就是官方定义。曾试过给剑魂（鬼剑士槽 1）把光剑换成通用短剑，已回滚——那是
// 技能侧的问题（光剑掌握的自动学习等级是 1，把学习等级错配成 15 才导致 1 级
// 没自动学会），不该由装备投影来兜。
func (s *Service) creationWorn(prof catalog.Profession, advancement, level byte) []inventory.BagEquipment {
	if s.Equipment == nil || s.Equipment.Source.Checksum != s.Catalog.Source.Checksum || s.WearRules.Source != s.Catalog.Source.Checksum || len(prof.CreateEquipmentBySlot) == 0 {
		return nil
	}
	var worn []inventory.BagEquipment
	used := map[uint16]bool{}
	for _, label := range prof.CreateEquipmentOrder {
		template := prof.CreateEquipmentBySlot[label][advancement]
		if template == 0 {
			continue
		}
		slot, ok := s.WearRules.Slots[label]
		if !ok || used[slot] {
			continue
		}
		definition, err := s.Equipment.Definition(template)
		if err != nil {
			continue
		}
		kind := definition.Fields["[equipment type]"]
		if len(kind) == 0 || kind[0].Text != label {
			continue
		}
		durability, err := s.Equipment.Reward(template)
		if err != nil {
			continue
		}
		if err := inventory.WearableBy(definition.Fields, kind[0].Text, prof.Job, advancement, level); err != nil {
			continue
		}
		worn = append(worn, inventory.BagEquipment{Slot: slot, Template: template, Durability: durability})
		used[slot] = true
	}
	return worn
}
