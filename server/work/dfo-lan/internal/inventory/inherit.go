package inventory

import (
	"context"
	"encoding/json"
	"fmt"

	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
)

// 装备继承（CMD 1722）的服务层。
//
// ★ 一次 1722 请求可以带**多条继承记录**（客户端 sub_14138A210 遍历窗口里所有
// `*(entry+244) == 2` 的条目，每条记录 = 一对装备 = 一次独立继承）。玩家做
// 「多个装备同时轮换继承」时一次会发多条 —— 服务端必须逐条处理，而不是要求
// 恰好两件（否则「解出 4 件」直接拒绝，表现就是「轮换继承无效化」）。
//
// ★ 方向判定（2026-09-28 定稿）：请求体里两个 UI 槽**没有「源 / 目标」语义**
// （玩家两件装备怎么拖都行），所以方向按游戏设计铁律定：
//
//	继承永远是「高等级件 = 材料件（material，被消耗）→ 低等级件 = 基础件（base，
//	保留并接收等级）」。两件等级相同 ⇒ 无法判定方向且继承无意义 ⇒ 拒绝。
//
// ★★ 材料件的归宿（官方文案取证，不是猜的）：
// 客户端 PVF 字符串表 dstr 69059（继承确认弹窗，紧邻 69060「This item can't Inherit.」）：
//
//	"Do you really want to transfer?\nThe target item can't be Transcended once the
//	 transfer is completed. The material equipment will lose all its
//	 Enchantment/Reinforcement/Amplification/Refining levels."
//
// ⇒ 材料件**失去所有附魔 / 强化 / 增幅 / 精炼等级**，但**不是消失**（对比 101033204
// 那条明写「The material equipment will disappear」—— 官方要表达「消失」时会明写）。
// 所以这里把材料件的强化/增幅等级、次元属性、附魔卡、锻造全部清零，
// 装备本体留在原格 —— **不删除**。
//
// 这条推翻了本项目早期「没有证据就不动源装备」的决定：当时只有实机 7 个样本、
// 材料模板恒 0；现在有了 dstr 原文 + 另一份独立实机验证，证据足够。
//
// 要搬运的东西全部落在 181 字节装备行里，且**服务端本来就有权威定义**：
//
//	offset 10 低五位  强化 / 增幅共用的等级字节（amplifyReinforceOffset，
//	                  reinforceLevelMask = 0x1f；高三位是再封装次数，必须保留）
//	offset 19         次元属性类型（amplifyTypeOffset，0 = 没有红字）
//	offset 20         次元属性数值（amplifyValueOffset）
//	offset 14         附魔卡（u32，enchantCardOffset，见 enchant.go）
//	BagEquipment.Refine  锻造（精炼）等级（权威值，见 refine.go）
//
// 前三个与增幅系统同一套常量，见 amplify.go 顶部的取证注释。
const inheritModel = "inherit-v1"

// InheritReceipt 是一次继承（一条记录）的落库结果。
type InheritReceipt struct {
	BaseSlot         uint16 `json:"base_slot"`
	BaseSpace        byte   `json:"base_space"`
	BaseTemplate     uint32 `json:"base_template"`
	MaterialSlot     uint16 `json:"material_slot"`
	MaterialSpace    byte   `json:"material_space"`
	MaterialTemplate uint32 `json:"material_template"`
	// BeforeLevel / AfterLevel 是基础件 offset 10 低五位在改动前后的值。
	BeforeLevel byte `json:"before_level"`
	AfterLevel  byte `json:"after_level"`
	// MaterialLevel 是材料件被清零前的等级。
	MaterialLevel byte `json:"material_level"`
	// AmplifyType / AmplifyValue 是转移的次元属性（offset 19 / 20）。
	AmplifyType  byte `json:"amplify_type"`
	AmplifyValue byte `json:"amplify_value"`
	// Refine 是转移的锻造等级（0 = 材料件没有锻造，未转移）。
	Refine byte `json:"refine"`
	// EnchantCard 是转移的附魔卡模板（0 = 材料件没有附魔，未转移）。
	EnchantCard uint32 `json:"enchant_card"`
}

// ApplyInherit 处理 CMD 1722：逐条记录把材料件的强化/增幅/锻造/附魔转移到基础件，
// 材料件清零保留。一次请求可以带多条记录，全部成功才落库。
func (s *WearService) ApplyInherit(ctx context.Context, role storage.Character, key string, r protocol.InheritRequest) (storage.Character, []InheritReceipt, error) {
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, nil, fmt.Errorf("装备继承需要有效装备目录及角色存档")
	}
	if len(r.Entries) == 0 {
		return role, nil, fmt.Errorf("装备继承请求里没有有效的继承记录")
	}
	return commitEquipmentEvent(ctx, s.Store, role, key, inheritModel, func(current storage.Character) (json.RawMessage, []InheritReceipt, error) {
		return s.applyInherit(current, r.Entries)
	})
}

// applyInherit 逐条记录继承：共用一次 ReadBag / SaveBag，后面的记录能看到前面
// 记录的改动（轮换继承时一件装备可能先当基础件、再被另一对当材料件）。
func (s *WearService) applyInherit(role storage.Character, entries []protocol.InheritEntry) (json.RawMessage, []InheritReceipt, error) {
	bag, err := ReadBag(role.State)
	if err != nil {
		return nil, nil, err
	}
	receipts := make([]InheritReceipt, 0, len(entries))
	for i, e := range entries {
		receipt, err := s.applyInheritEntry(&bag, e)
		if err != nil {
			return nil, receipts, fmt.Errorf("第 %d 条继承记录: %w", i+1, err)
		}
		receipts = append(receipts, receipt)
	}
	next, err := SaveBag(role.State, bag)
	if err != nil {
		return nil, receipts, err
	}
	return next, receipts, nil
}

// applyInheritEntry 处理一条继承记录（A/B 两件装备），原地修改 bag。
//
// 两件装备都按「wire 声明的容器优先、再另一个容器」反查，**并且用模板号对账**：
// 模板号是可靠的身份（见 protocol/inherit.go 的取证），所以回退查找不会拿错装备；
// 而纯按槽位查会踩「背包格号与穿戴槽号是两套重叠编号」的坑
// （穿戴手镯 [wrist]=20 会被背包 20 号格里的东西遮蔽）。
func (s *WearService) applyInheritEntry(bag *Bag, e protocol.InheritEntry) (InheritReceipt, error) {
	var out InheritReceipt
	baseGear, baseSpace, err := locateInheritGear(*bag, e.SpaceA, e.SlotA, e.TemplateA)
	if err != nil {
		return out, fmt.Errorf("继承装备 A: %w", err)
	}
	matGear, matSpace, err := locateInheritGear(*bag, e.SpaceB, e.SlotB, e.TemplateB)
	if err != nil {
		return out, fmt.Errorf("继承装备 B: %w", err)
	}
	if baseSpace == matSpace && baseGear.Slot == matGear.Slot {
		return out, fmt.Errorf("两件继承装备不能是同一格（容器 %d 槽 %d）", baseSpace, baseGear.Slot)
	}
	for _, gear := range []BagEquipment{baseGear, matGear} {
		if err = gear.ValidateRecord(); err != nil {
			return out, err
		}
		d, e := s.Catalog.Definition(gear.Template)
		if e != nil {
			return out, e
		}
		if _, ok := d.Fields["[equipment type]"]; !ok {
			return out, fmt.Errorf("模板 %d 不是装备", gear.Template)
		}
	}

	matRow := EquipmentRow(matGear)
	baseRow := EquipmentRow(baseGear)
	levelOf := func(row [protocol.CurrentItemRecordSize]byte) byte {
		return row[amplifyReinforceOffset] & reinforceLevelMask
	}
	lb, lm := levelOf(baseRow), levelOf(matRow)
	// 方向：高等级件是材料件（被消耗），低等级件是基础件（保留并接收等级）。
	if lb > lm {
		baseGear, matGear = matGear, baseGear
		baseSpace, matSpace = matSpace, baseSpace
		baseRow, matRow = matRow, baseRow
		lb, lm = lm, lb
	}
	if lb == lm {
		return out, fmt.Errorf("两件装备等级相同（均为 %d），无法判定继承方向，已拒绝", lb)
	}
	if lm == 0 {
		// 上面 lb < lm 的前提下 lm == 0 意味着两件都是 0，已被上一条拦住。
		return out, fmt.Errorf("材料件没有可继承的等级（offset 10 低五位为 0）")
	}

	newLevel := matRow[amplifyReinforceOffset] & reinforceLevelMask
	// 基础件接收等级：保留它自己的高三位（再封装次数），只覆盖低五位。
	baseRow[amplifyReinforceOffset] = baseRow[amplifyReinforceOffset]&^byte(reinforceLevelMask) | newLevel
	// 次元属性（红字）只在材料件确实带增幅时才覆盖：否则会把基础件原有的红字清空。
	if matRow[amplifyTypeOffset] != 0 {
		baseRow[amplifyTypeOffset] = matRow[amplifyTypeOffset]
		baseRow[amplifyValueOffset] = matRow[amplifyValueOffset]
	}
	// 附魔卡（offset 14，u32）同样「只增不删」：材料件有附魔才覆盖基础件。
	matCard := enchantCard(matRow[:])
	if matCard != 0 {
		setEnchantCard(baseRow[:], matCard)
		out.EnchantCard = matCard
	}
	// 锻造（精炼）同样「只增不删」：材料件有锻造才覆盖。用 refineLevel 读权威值
	// （优先 BagEquipment.Refine，回退行内字节，兼容旧档）。
	matRefine := refineLevel(matGear, matRow[:])
	if matRefine > 0 {
		setRefineLevel(&baseGear, baseRow[:], matRefine)
		out.Refine = matRefine
	}
	baseGear.Record = append([]byte(nil), baseRow[:]...)

	// 材料件清零保留（dstr 69059）：等级 / 次元属性 / 附魔 / 锻造归零，装备本体留原格。
	matRow[amplifyReinforceOffset] &= ^byte(reinforceLevelMask)
	matRow[amplifyTypeOffset] = 0
	matRow[amplifyValueOffset] = 0
	setEnchantCard(matRow[:], 0)
	setRefineLevel(&matGear, matRow[:], 0)             // 先清锻造（写 Refine + 行内镜像）
	matGear.Record = append([]byte(nil), matRow[:]...) // 再快照行，确保行内锻造字节已归零

	writeBack(bag, baseSpace, baseGear)
	writeBack(bag, matSpace, matGear)

	out.BaseSlot = baseGear.Slot
	out.BaseSpace = baseSpace
	out.BaseTemplate = baseGear.Template
	out.MaterialSlot = matGear.Slot
	out.MaterialSpace = matSpace
	out.MaterialTemplate = matGear.Template
	out.BeforeLevel = lb
	out.AfterLevel = newLevel
	out.MaterialLevel = lm
	out.AmplifyType = baseRow[amplifyTypeOffset]
	out.AmplifyValue = baseRow[amplifyValueOffset]
	return out, nil
}

// writeBack 把一件装备按容器写回背包（0 = 背包、3 = 已穿戴）。
//
// 三个字段一起对账（槽位 + 模板 + Group）：穿戴列表里同一槽位可以并存两条记录
// —— Group 0 是真装备、Group 1 是幻化外观（见 Bag.WornBaseItems 的注释与
// avatar_clone_coexistence_test.go）。只按槽位写回会把改动打到外观那一件上。
func writeBack(bag *Bag, space byte, gear BagEquipment) {
	items := bag.Equipment
	if space == 3 {
		items = bag.Worn
	}
	for i := range items {
		if items[i].Slot == gear.Slot && items[i].Template == gear.Template && items[i].Group == gear.Group {
			items[i] = gear
			return
		}
	}
}

// locateInheritGear 按请求里的容器与槽位取装备，并用模板号对账。
//
// 客户端只在两个容器里放装备（0 = 背包、3 = 已穿戴），sub_14138A210 里
// sub_145AD5C20 查不到时就把容器写成 3 再查一次（sub_145AD5F80），
// 所以请求里报的容器不一定准 ⇒ 先查声明的容器、再查另一个，全程用模板号确认身份。
// 两处都找不到就明确报错，绝不「只按槽位」蒙一个。
func locateInheritGear(bag Bag, space byte, slot uint16, template uint32) (BagEquipment, byte, error) {
	order := []byte{space}
	if space != 0 {
		order = append(order, 0)
	} else {
		order = append(order, 3)
	}
	for _, sp := range order {
		items := bag.Equipment
		if sp == 3 {
			items = bag.Worn
		}
		for _, gear := range items {
			// ★ 穿戴侧必须只认 Group 0（真装备）：同一穿戴槽可以并存一条
			// Group 1 的幻化外观记录，按槽位盲选会拿到外观那一件
			// （Bag.WornBaseItems 的取舍规则、avatar_clone_coexistence_test.go）。
			if sp == 3 && gear.Group != 0 {
				continue
			}
			if gear.Slot == slot && gear.Template == template {
				return gear, sp, nil
			}
		}
	}
	return BagEquipment{}, 0, fmt.Errorf("容器 %d 槽位 %d 里找不到模板 %d 的装备（装备可能已被移动，请重开继承窗口重试）", space, slot, template)
}
