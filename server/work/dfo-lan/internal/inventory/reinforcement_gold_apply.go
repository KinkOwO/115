package inventory

import (
	"context"
	"crypto/rand"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"math/big"
)

// 账号材料仓库固定格数量：363..379（17 格；0/1 是两条光辉灵魂）。
const accountMaterialCells = 17

// 金币强化（普通强化）：材料 + 金币 → 掷骰 → 成功升级 / 失败按实测表降级或破坏。
//
// 与固定等级强化券是两条并列路径：
//   - 券路径（applyReinforcement）在背包里找券，事务只动角色；
//   - 金币路径在这里，材料可能在账号共享材料仓库（无色小晶块固定格 367）或背包材料格，
//     所以必须走 CommitAccountMaterialEvent —— 它把「扣账号材料」和「改角色存档」放进同一事务，
//     并按 (character, event_key) 幂等，重放不会二次扣料。
type GoldReinforcementReceipt struct {
	Request             protocol.ReinforcementRequest `json:"request"`
	Mode                string                        `json:"mode"` // normal = 无色小晶块；safe = 安全强化材料
	MaterialTemplate    uint32                        `json:"material_template"`
	MaterialSlot        uint16                        `json:"material_slot"`
	MaterialFromStorage bool                          `json:"material_from_storage"`
	MaterialRemaining   uint32                        `json:"material_remaining"`
	MaterialSpent       uint32                        `json:"material_spent"`
	GoldSpent           uint32                        `json:"gold_spent"`
	Gold                uint32                        `json:"gold"`
	Old, Level, Result  byte
	// PostLevel 是落库后的真实等级；Level 是回包里要写的值。
	// 实机证据：把降级（8→7）写进回包会让客户端发 ADD_HACKTYPE_CNT 并锁死窗口，
	// 所以失败回包的 level 必须等于 old（与固定券分支的校验一致），掉级只靠装备行下发。
	PostLevel byte `json:"post_level"`
	Rate      int  `json:"rate"`   // 本次掷骰用的成功率（安全强化含失败补正）
	Streak    int  `json:"streak"` // 安全强化本级的连续失败次数（成功后清零）
	Destroyed bool `json:"destroyed,omitempty"`
	// Protected：本次失败落在「会碎」区间，但背包里有强化保护券，装备被保住（等级归零）。
	Protected bool `json:"protected,omitempty"`
	// ProtectionSlot：被消耗的那张保护券在背包里的槽位。客户端强化窗口没有保护券槽
	// （请求里恒为 0xffff），这是服务端全背包查找后找到的位置。
	ProtectionSlot uint16       `json:"protection_slot,omitempty"`
	Equipment      BagEquipment `json:"equipment"`
	EquipmentSpace byte         `json:"equipment_space"`
	EquipmentSlot  uint16       `json:"equipment_slot"`
}

// 角色存档里保存最近一次金币强化回执的字段（幂等重放时按 key 取回）。
const goldReinforcementStateField = "last_gold_reinforcement"

type storedGoldReinforcement struct {
	Key string `json:"key"`
	GoldReinforcementReceipt
}

// ReinforceWithMaterial 执行一次金币强化。材料引用为请求里的 (list, slot)：
// 363..379 = 账号材料仓库格，其它 = 角色背包槽位。
func (s *WearService) ReinforceWithMaterial(ctx context.Context, role storage.Character, key string, r protocol.ReinforcementRequest) (storage.Character, GoldReinforcementReceipt, error) {
	var out GoldReinforcementReceipt
	if s == nil || s.Store == nil || s.Catalog == nil || s.Catalog.Source.Checksum != role.ConfigVersion || s.BagRules.Source != role.ConfigVersion {
		return role, out, fmt.Errorf("金币强化需要有效装备目录及角色存档")
	}
	if !GoldRulesLoaded() {
		return role, out, fmt.Errorf("金币强化规则未装载")
	}
	applied := false
	saved, _, _, err := s.Store.CommitAccountMaterialEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "gold-reinforcement-v1", func(current storage.Character, counts json.RawMessage) (json.RawMessage, json.RawMessage, error) {
		next, nextCounts, receipt, err := s.applyGoldReinforcement(current, counts, key, r)
		if err != nil {
			return nil, nil, err
		}
		out = receipt
		applied = true
		return next, nextCounts, nil
	})
	if err != nil {
		return role, out, err
	}
	if !applied {
		// 重放：事务里没有再次执行，回执从存档字段取回。
		stored, e := readGoldReinforcementReceipt(saved.State, key)
		if e != nil {
			return role, out, e
		}
		out = stored
	}
	saved.WireID = role.WireID
	if _, err = protocol.ReinforcementGoldReply(r, out.MaterialRemaining, out.Old, out.Level, out.Result); err != nil {
		return role, out, err
	}
	return saved, out, nil
}

func (s *WearService) applyGoldReinforcement(role storage.Character, counts json.RawMessage, key string, r protocol.ReinforcementRequest) (json.RawMessage, json.RawMessage, GoldReinforcementReceipt, error) {
	var out GoldReinforcementReceipt
	fail := func(kind RefusalKind, reason string) (json.RawMessage, json.RawMessage, GoldReinforcementReceipt, error) {
		return nil, nil, out, Refuse(kind, "%s", reason)
	}
	// 形状检查与券路径一致：单次；@9 是材料槽位。
	// tail[4]（Multiple）左侧普通强化为 0、右侧安全强化为 1，所以这里不判断，改到材料解析之后。
	// ProtectionSlot 不参与形状校验：客户端强化窗口**没有**独立保护券槽位（实测恒 0xffff），
	// 原版语义是「背包持有保护券、失败到会碎区间自动消耗」，服务端自己全背包查找。
	if r.Mode != 0 || (r.EquipmentSpace != 0 && r.EquipmentSpace != 3) || r.TicketSpace != 0 ||
		r.TicketSlot == 0xffff || r.MaterialSlot != 0xffff {
		return fail(RefusalUnsupported, "强化只支持单次普通/安全强化，不支持增幅")
	}
	if r.Multiple > 1 {
		return fail(RefusalUnsupported, "不支持批量强化")
	}
	bag, err := ReadBag(role.State)
	if err != nil {
		return nil, nil, out, err
	}
	materials, err := ReadAccountMaterials(counts)
	if err != nil {
		return nil, nil, out, err
	}

	// 材料引用
	slot := r.TicketSlot
	var (
		materialTemplate uint32
		materialAmount   uint32
		fromStorage      bool
		bagIndex         = -1
	)
	if slot >= AccountMaterialSlotBase && slot < AccountMaterialSlotBase+accountMaterialCells {
		template, ok := StorageRowTemplate(slot)
		if !ok {
			return fail(RefusalGeneric, "材料槽不是账号材料仓库的格子")
		}
		materialTemplate, materialAmount, fromStorage = template, materials.Count(template), true
	} else {
		for i, item := range bag.Items {
			if item.Slot == slot {
				bagIndex, materialTemplate, materialAmount = i, item.Template, item.Amount
				break
			}
		}
		if bagIndex < 0 {
			return fail(RefusalItems, "材料不在所属角色背包")
		}
	}
	if !IsGoldMaterial(materialTemplate) && !IsSafeMaterial(materialTemplate) {
		return fail(RefusalGeneric, "窗口里的物品不是强化的消耗材料")
	}
	// 实机判据：左侧普通强化 tail[4]=0 + @9=367（无色小晶块）；右侧安全强化 tail[4]=1 + @9=136（安全材料）。
	safe := r.Multiple == 1
	if safe && !IsSafeMaterial(materialTemplate) {
		return fail(RefusalGeneric, "安全强化需要安全强化材料（10327281 / 10327284）")
	}
	if !safe && !IsGoldMaterial(materialTemplate) {
		return fail(RefusalGeneric, "普通强化只收无色小晶块；安全强化材料请放到另一侧")
	}

	// 目标装备
	items := bag.Equipment
	if r.EquipmentSpace == 3 {
		items = bag.Worn
	}
	gearIndex := -1
	for i, gear := range items {
		if gear.Slot == r.EquipmentSlot && gear.Template == r.EquipmentTemplate {
			gearIndex = i
			break
		}
	}
	if gearIndex < 0 {
		return fail(RefusalGeneric, "目标装备不在指定的所属角色槽位")
	}
	gear := items[gearIndex]
	if err = gear.ValidateRecord(); err != nil {
		return nil, nil, out, err
	}
	d, err := s.Catalog.Definition(gear.Template)
	if err != nil {
		return nil, nil, out, err
	}
	kind := d.Fields["[equipment type]"]
	if len(kind) == 0 || kind[0].Type != 6 {
		return fail(RefusalGeneric, "目标装备类型无效")
	}
	switch kind[0].Text {
	case "[weapon]", "[coat]", "[pants]", "[shoulder]", "[waist]", "[shoes]", "[amulet]", "[wrist]", "[ring]", "[support]", "[magic stone]", "[earring]":
	default:
		return fail(RefusalGeneric, "此类物品不能强化")
	}
	row := EquipmentRow(gear)
	old := row[10] & goldLevelFieldMask

	equipLevel, levelOK := singleInt(d, "[minimum level]")
	rarity, rarityOK := singleInt(d, "[rarity]")
	if !levelOK || !rarityOK {
		return fail(RefusalUnsupported, "无法核对装备等级或品质")
	}
	weapon := kind[0].Text == "[weapon]"
	if safe && SafePathWeaponOnly() && !weapon {
		return fail(RefusalGeneric, "安全强化只对武器开放")
	}

	// 需求：普通强化的材料数量只看强化等级、金币吃装备等级/品质/部位；
	// 安全强化两条都来自 [safe upgrade] 段（与普通强化是独立曲线），且只覆盖 0..11 级。
	var needed, gold uint32
	if safe {
		if int(old) >= SafePathMaxLevel() {
			return fail(RefusalLimit, "安全强化最高到 +"+fmt.Sprint(SafePathMaxLevel()-1)+"：请改用无色小晶块")
		}
		if !SafeUpgradeEligible(int(equipLevel), int(rarity)) {
			return fail(RefusalGeneric, "该装备不符合安全强化条件（100 级以上 + rare..primeval）")
		}
		_, count, cost, ok := SafeUpgradeCost(int(old))
		if !ok {
			return fail(RefusalGeneric, "当前强化等级不在安全强化表内")
		}
		needed, gold = count, cost
	} else {
		if int(old) >= GoldMaxUpgradeLevel() {
			return fail(RefusalLimit, "强化已达到客户端上限 +"+fmt.Sprint(GoldMaxUpgradeLevel())+"：更高的结果等级会让客户端判定异常并锁死强化面板")
		}
		count, ok := GoldMaterialCount(int(old))
		if !ok {
			return fail(RefusalGeneric, "当前强化等级不在金币强化表内")
		}
		cost, e := GoldCost(int(equipLevel), int(rarity), int(old), weapon)
		if e != nil {
			return nil, nil, out, e
		}
		needed, gold = count, cost
	}
	if materialAmount < needed {
		return fail(RefusalGeneric, "强化材料数量不足（需要 "+fmt.Sprint(needed)+" 个）")
	}
	if bag.Gold < gold {
		return fail(RefusalGold, "金币不足（需要 "+fmt.Sprint(gold)+"）")
	}

	// 掷骰：普通强化用实测表；安全强化 0-9 同表、10→11 与 11→12 另有失败补正。
	streaks, err := readSafeStreaks(role.State)
	if err != nil {
		return nil, nil, out, err
	}
	streakKey := safeStreakKey(r.EquipmentSpace, r.EquipmentSlot, int(old))
	var rate int
	var ok bool
	if safe {
		rate, ok = SafeSuccessPercent(int(old), streaks[streakKey])
	} else {
		rate, ok = GoldSuccessPercent(int(old))
	}
	if !ok {
		return fail(RefusalGeneric, "当前强化等级没有成功率")
	}
	roll, err := rand.Int(rand.Reader, big.NewInt(100))
	if err != nil {
		return nil, nil, out, err
	}
	level, result := old, byte(1)
	destroyed := false
	protected := false
	protectionSlot := uint16(0) // 背包里强化保护券所在槽位（客户端窗口无保护券槽，自动查找）
	if roll.Int64() < int64(rate) {
		level, result = old+1, 0
	} else if !safe {
		switch GoldPenalty(int(old), weapon) {
		case goldPenaltyDownOne:
			level = old - 1
		case goldPenaltyDownThree:
			if old >= 3 {
				level = old - 3
			} else {
				level = 0
			}
		case goldPenaltyDestroy:
			if goldDestroyEnabled() {
				destroyed = true
				// 客户端强化窗口没有独立保护券槽位；原版语义是「背包持有保护券、
				// 失败到会碎区间自动消耗」。在背包里找强化保护券，找到则保护：
				// 装备不破坏、强化等级归零、消耗一张券；找不到才摧毁。
				if slot, found := findProtectionTicket(bag, false); found {
					destroyed = false
					protected = true
					level = 0
					protectionSlot = slot
				}
			}
		}
	}
	// 安全强化失败不减级也不破坏（这正是它要多花材料与金币的原因）。

	// 安全强化的失败补正：只有 10→11 / 11→12 两级有补正，成功后清零。
	if safe {
		if result == 0 {
			if SafePathResetsStreakOnSuccess() {
				delete(streaks, streakKey)
			}
		} else if _, stage := goldRules.SafePathRules.StageRate[fmt.Sprint(old)]; stage && streaks[streakKey] < 255 {
			streaks[streakKey]++
		}
	}

	// 落库：破坏则移除该装备行，否则写回强化等级。
	if destroyed {
		remaining := append([]BagEquipment(nil), items[:gearIndex]...)
		remaining = append(remaining, items[gearIndex+1:]...)
		if r.EquipmentSpace == 3 {
			bag.Worn = remaining
		} else {
			bag.Equipment = remaining
		}
	} else {
		row[10] = row[10]&0xe0 | level
		gear.Record = append([]byte(nil), row[:]...)
		items = append([]BagEquipment(nil), items...)
		items[gearIndex] = gear
		if r.EquipmentSpace == 3 {
			bag.Worn = items
		} else {
			bag.Equipment = items
		}
	}

	// 扣材料与金币：无论成败都消耗。
	materialRemaining := uint32(0)
	if fromStorage {
		materials, _, err = materials.Spend(materialTemplate, needed)
		if err != nil {
			return nil, nil, out, err
		}
		materialRemaining = materials.Count(materialTemplate)
	} else {
		nextBag, remaining, e := consumeBagAmount(bag, slot, materialTemplate, needed)
		if e != nil {
			return nil, nil, out, e
		}
		bag, materialRemaining = nextBag, remaining
	}
	bag.Gold -= gold

	// 保护券触发时扣一张保护券（失败保护装备，代价是消耗券）。
	if protected {
		bag, err = consumeProtectionTicket(bag, protectionSlot)
		if err != nil {
			return nil, nil, out, err
		}
	}

	mode := "normal"
	if safe {
		mode = "safe"
	}
	// 回包等级：成功写新等级（客户端上限 15）；失败必须写 old，否则客户端判定异常。
	replyLevel := level
	if result == 1 {
		replyLevel = old
	}
	out = GoldReinforcementReceipt{
		Request: r, Mode: mode, MaterialTemplate: materialTemplate, MaterialSlot: slot,
		MaterialFromStorage: fromStorage, MaterialRemaining: materialRemaining,
		MaterialSpent: needed, GoldSpent: gold, Gold: bag.Gold,
		Old: old, Level: replyLevel, PostLevel: level, Rate: rate, Streak: streaks[streakKey],
		Result: result, Destroyed: destroyed, Protected: protected, ProtectionSlot: protectionSlot,
		Equipment: gear, EquipmentSpace: r.EquipmentSpace, EquipmentSlot: r.EquipmentSlot,
	}
	if out.Equipment.Record != nil {
		out.Equipment.Record = append([]byte(nil), out.Equipment.Record...)
	}
	next, err := SaveBag(role.State, bag)
	if err != nil {
		return nil, nil, out, err
	}
	next, err = writeGoldReinforcementReceipt(next, key, out)
	if err != nil {
		return nil, nil, out, err
	}
	next, err = writeSafeStreaks(next, streaks)
	if err != nil {
		return nil, nil, out, err
	}
	nextCounts, err := materials.Save()
	if err != nil {
		return nil, nil, out, err
	}
	return next, nextCounts, out, nil
}

// consumeBagAmount 从背包某槽位扣掉 amount 个（扣空即移除该行）。
func consumeBagAmount(b Bag, slot uint16, template, amount uint32) (Bag, uint32, error) {
	if amount == 0 {
		return b, 0, fmt.Errorf("invalid bag material amount")
	}
	rows := append([]BagItem(nil), b.Items...)
	for i, row := range rows {
		if row.Slot != slot {
			continue
		}
		if row.Template != template {
			return b, 0, fmt.Errorf("槽位里的材料与请求不符")
		}
		if row.Amount < amount {
			return b, 0, fmt.Errorf("材料数量不足")
		}
		remaining := row.Amount - amount
		if remaining == 0 {
			rows = append(rows[:i:i], rows[i+1:]...)
		} else {
			rows[i].Amount = remaining
		}
		b.Items = rows
		return b, remaining, nil
	}
	return b, 0, Refuse(RefusalItems, "材料不在所属角色背包")
}

// 安全强化的失败补正计数：键 = 空间:槽位:尝试前等级，值 = 连续失败次数（成功清零）。
// 存在角色存档里，跨会话保留（原版补正是累计的）。
const goldSafeStreakStateField = "reinforce_safe_streaks"

func safeStreakKey(space byte, slot uint16, level int) string {
	return fmt.Sprintf("%d:%d:%d", space, slot, level)
}

func readSafeStreaks(state json.RawMessage) (map[string]int, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(state, &fields); err != nil {
		return nil, err
	}
	out := map[string]int{}
	raw, ok := fields[goldSafeStreakStateField]
	if !ok {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func writeSafeStreaks(state json.RawMessage, streaks map[string]int) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(state, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("character state is not an object")
	}
	if len(streaks) == 0 {
		delete(fields, goldSafeStreakStateField)
		return json.Marshal(fields)
	}
	value, err := json.Marshal(streaks)
	if err != nil {
		return nil, err
	}
	fields[goldSafeStreakStateField] = value
	return json.Marshal(fields)
}

func writeGoldReinforcementReceipt(state json.RawMessage, key string, receipt GoldReinforcementReceipt) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(state, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("character state is not an object")
	}
	value, err := json.Marshal(storedGoldReinforcement{Key: key, GoldReinforcementReceipt: receipt})
	if err != nil {
		return nil, err
	}
	fields[goldReinforcementStateField] = value
	return json.Marshal(fields)
}

func readGoldReinforcementReceipt(state json.RawMessage, key string) (GoldReinforcementReceipt, error) {
	var fields map[string]json.RawMessage
	var out GoldReinforcementReceipt
	if err := json.Unmarshal(state, &fields); err != nil {
		return out, err
	}
	raw, ok := fields[goldReinforcementStateField]
	if !ok {
		return out, Refuse(RefusalUnsupported, "找不到金币强化回执")
	}
	var stored storedGoldReinforcement
	if err := json.Unmarshal(raw, &stored); err != nil {
		return out, err
	}
	if stored.Key != key {
		return out, fmt.Errorf("金币强化回执与请求不符")
	}
	return stored.GoldReinforcementReceipt, nil
}

// GoldMaterialSlotForTemplate 供 GM/测试查询模板对应的账号材料仓库格（找不到返回 false）。
func GoldMaterialSlotForTemplate(template uint32) (uint16, bool) {
	return AccountMaterialSlot(template)
}
