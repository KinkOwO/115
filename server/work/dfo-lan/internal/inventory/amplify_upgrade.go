package inventory

// 增幅（Amplification，NPC Klonter）：把装备已有的次元属性「+N」往上升。
//
// 与「增幅书打红字（CMD205）」的关系：
//   - 增幅书（CMD205）负责**给装备打上**次元属性（record[19]=类型、record[20]=数值）；
//   - 增幅（本文件，CMD80 mode=1）负责把那个数值**继续往上加**。
//   所以增幅的前置条件是装备必须已有次元属性，即 record[19] != 0。
//
// 协议（2026-09-28 实机抓包确认）：增幅不是独立 opcode，而是 **CMD80 的 mode=1**，
// 与强化共用同一个 ReinforcementRequest 结构（44 字节明文，唯一的差别就是 [0]=1）。
// 材料放在「券槽」位（[9..10]），MaterialSlot（[11..12]）为 0xffff 未用。
//
// 运行费用规则由 gamedata 从 PVF 的 etc/amplifyupgrade.etc 准备：
// 材料 3242 矛盾结晶体、每级消耗 = 等级+1、
// 金币列。成功率与失败惩罚 **PVF 里没有**，以官方页数据为准（同强化：PVF 无成功率表）。

import (
	"crypto/rand"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

const amplifyUpgradeModel = "amplify-upgrade-v1"

// 增幅结果的三种失败处理（对应配置里的 failure_penalty）。
const (
	amplifyPenaltyNone    = "none"
	amplifyPenaltyDown    = "level_down_1"
	amplifyPenaltyDestroy = "destroy"
)

type amplifyUpgradeConfig struct {
	Version          int    `json:"version"`
	Source           string `json:"source"`
	MaterialTemplate uint32 `json:"material_template"`
	Levels           []struct {
		Level            int      `json:"level"`
		MaterialTemplate uint32   `json:"material_template"`
		MaterialCount    uint32   `json:"material_count"`
		GoldColumns      []uint32 `json:"gold_columns"`
	} `json:"levels"`
	SafeMaterialTemplates []uint32 `json:"safe_material_templates"`
	SafeUpgrade           []struct {
		Level    int    `json:"level"`
		Enabled  int    `json:"enabled"`
		Gold     uint32 `json:"gold"`
		Material uint32 `json:"material"`
		Count    uint32 `json:"count"`
		Value    uint32 `json:"value"`
	} `json:"safe_upgrade"`
	SafeUpgradeUsableRarity []string `json:"safe_upgrade_usable_rarity"`
	SafeUpgradeMinLevel     []int    `json:"safe_upgrade_min_level"`
	Official                struct {
		SuccessRatePercentByLevel map[string]int `json:"success_rate_percent_by_level"`
		FailurePenalty            struct {
			ZeroToSix   string `json:"0-6"`
			SevenToNine string `json:"7-9"`
			TenPlus     string `json:"10+"`
		} `json:"failure_penalty"`
		SafeAmplify struct {
			MaxLevel int `json:"max_level"`
		} `json:"safe_amplify"`
	} `json:"official"`
}

var amplifyUpgradeRules *amplifyUpgradeConfig

// LoadAmplifyUpgradeRules 读取增幅规则；文件缺失时该功能整体拒绝（不影响强化/打红字）。
func LoadAmplifyUpgradeRules(path string) error {
	var c amplifyUpgradeConfig
	if loaded, err := loadOptionalJSON(path, &c); !loaded || err != nil {
		return err
	}

	if c.Version != 1 || len(c.Source) != 64 || len(c.Levels) == 0 {
		return fmt.Errorf("增幅规则源定义不完整")
	}
	amplifyUpgradeRules = &c
	return nil
}

func AmplifyUpgradeRulesLoaded() bool { return amplifyUpgradeRules != nil }

// IsAmplifyMaterial 报告模板是否为增幅材料（矛盾结晶体 3242）。
func IsAmplifyMaterial(template uint32) bool {
	return amplifyUpgradeRules != nil && template == amplifyUpgradeRules.MaterialTemplate
}

// AmplifyMaterialCount 返回从 level 增幅到 level+1 需要消耗的材料数量（= 等级 + 1）。
func AmplifyMaterialCount(level int) (uint32, bool) {
	if amplifyUpgradeRules == nil || level < 0 || level >= len(amplifyUpgradeRules.Levels) {
		return 0, false
	}
	return amplifyUpgradeRules.Levels[level].MaterialCount, true
}

// AmplifyGold 返回该等级的金币费用（+0..+3 为 0）。
func AmplifyGold(level int) (uint32, bool) {
	if amplifyUpgradeRules == nil || level < 0 || level >= len(amplifyUpgradeRules.Levels) {
		return 0, false
	}
	cols := amplifyUpgradeRules.Levels[level].GoldColumns
	if len(cols) == 0 {
		return 0, true
	}
	return cols[0], true
}

// AmplifySuccessPercent 返回该等级的成功率（官方页数据，PVF 里没有成功率表）。
func AmplifySuccessPercent(level int) int {
	if amplifyUpgradeRules == nil {
		return 0
	}
	m := amplifyUpgradeRules.Official.SuccessRatePercentByLevel
	if v, ok := m[fmt.Sprint(level)]; ok {
		return v
	}
	return m["default"]
}

// AmplifyPenalty 返回该等级失败时的处理。
func AmplifyPenalty(level int) string {
	if amplifyUpgradeRules == nil {
		return amplifyPenaltyNone
	}
	p := amplifyUpgradeRules.Official.FailurePenalty
	switch {
	case level >= 10:
		return p.TenPlus
	case level >= 7:
		return p.SevenToNine
	default:
		return p.ZeroToSix
	}
}

// IsAmplifySafeMaterial 报告模板是否为安全增幅材料（Harmonious Crystals 10327282）。
func IsAmplifySafeMaterial(template uint32) bool {
	if amplifyUpgradeRules == nil {
		return false
	}
	for _, t := range amplifyUpgradeRules.SafeMaterialTemplates {
		if t == template {
			return true
		}
	}
	return false
}

// AmplifySafeMaxLevel 返回安全增幅允许的**当前最高等级**（官方："+9 or lower Amplified"）。
// 语义是「当前 +9 时仍可安全增幅到 +10」，所以可行区间为 level <= 9，拒绝条件为 level > 9。
// 当前等级是 +10 及以上就必须改用矛盾结晶体。
func AmplifySafeMaxLevel() int {
	if amplifyUpgradeRules != nil && amplifyUpgradeRules.Official.SafeAmplify.MaxLevel > 0 {
		return amplifyUpgradeRules.Official.SafeAmplify.MaxLevel
	}
	return 9
}

// AmplifySafeCost 返回该等级安全增幅的材料数量与金币。
// [safe upgrade] 每级两行：enabled=1 是武器行、enabled=0 是非武器行（官方：武器与非武器消耗不同）。
func AmplifySafeCost(level int, weapon bool) (uint32, uint32, bool) {
	if amplifyUpgradeRules == nil {
		return 0, 0, false
	}
	want := 0
	if weapon {
		want = 1
	}
	for _, r := range amplifyUpgradeRules.SafeUpgrade {
		if r.Level == level && r.Enabled == want {
			return r.Count, r.Gold, true
		}
	}
	return 0, 0, false
}

// AmplifySafeEligible 报告该装备是否符合安全增幅条件（装备等级 >= 100 且品质在白名单内）。
func AmplifySafeEligible(equipLevel, rarity int) bool {
	if amplifyUpgradeRules == nil {
		return false
	}
	for _, v := range amplifyUpgradeRules.SafeUpgradeMinLevel {
		if equipLevel < v {
			return false
		}
	}
	if rarity < 0 || rarity >= len(equipmentRarityNames) {
		return false
	}
	name := equipmentRarityNames[rarity]
	for _, usable := range amplifyUpgradeRules.SafeUpgradeUsableRarity {
		if usable == name {
			return true
		}
	}
	return false
}

var amplifyUpgradeRandomInt = func(n int) (int, error) {
	if n <= 1 {
		return 0, nil
	}
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return int(binary.BigEndian.Uint32(b[:]) % uint32(n)), nil
}

// AmplifyUpgradeReceipt 是一次增幅的结果。
type AmplifyUpgradeReceipt struct {
	Request        protocol.ReinforcementRequest `json:"request"`
	Equipment      BagEquipment                  `json:"equipment"`
	EquipmentSpace byte                          `json:"equipment_space"`
	EquipmentSlot  uint16                        `json:"equipment_slot"`
	AmplifyType    byte                          `json:"amplify_type"`
	LevelBefore    byte                          `json:"level_before"`
	LevelAfter     byte                          `json:"level_after"`
	Result         byte                          `json:"result"` // 0 成功 / 1 失败
	Penalty        string                        `json:"penalty"`
	Destroyed      bool                          `json:"destroyed"`
	// Protected：本次增幅失败落在「会碎」区间，但背包里有增幅保护券，装备被保住（等级归零）。
	Protected bool `json:"protected,omitempty"`
	// ProtectionSlot：被消耗的那张增幅保护券在背包里的槽位（客户端增幅窗口没有保护券槽，
	// 请求里恒为 0xffff，这是服务端全背包查找后找到的位置）。
	ProtectionSlot    uint16 `json:"protection_slot,omitempty"`
	Safe              bool   `json:"safe"` // 是否走安全增幅（材料不同、失败无惩罚）
	MaterialSlot      uint16 `json:"material_slot"`
	MaterialRemaining uint32 `json:"material_remaining"`
	MaterialSpent     uint32 `json:"material_spent"`
	GoldSpent         uint32 `json:"gold_spent"`
	Gold              uint32 `json:"gold"`
	SuccessPercent    int    `json:"success_percent"`
}

// ApplyAmplifyUpgrade 处理 CMD80 mode=1：校验次元属性与材料、扣费、按官方成功率判定。

func (s *WearService) ApplyAmplifyUpgrade(role Role, r protocol.ReinforcementRequest) (json.RawMessage, AmplifyUpgradeReceipt, error) {
	var out AmplifyUpgradeReceipt
	bag, err := ReadBag(role.State)
	if err != nil {
		return nil, out, err
	}
	// 目标装备：按请求里的空间找（0 背包 / 3 已穿戴），找不到再退回另一侧。
	space, items, index := bag.findEquipment(r.EquipmentSpace, r.EquipmentSlot, r.EquipmentTemplate)
	if index < 0 {
		return nil, out, Refuse(RefusalItems, "目标装备不在背包或已穿戴槽位里")
	}
	gear := items[index]
	if err = gear.ValidateRecord(); err != nil {
		return nil, out, err
	}
	d, err := s.Catalog.Definition(gear.Template)
	if err != nil {
		return nil, out, err
	}
	if _, ok := d.Fields["[equipment type]"]; !ok {
		return nil, out, Refuse(RefusalEquipment, "目标不是装备")
	}

	row := EquipmentRow(gear)
	// 安全增幅的门槛需要装备等级、品质与部位。
	equipLevel, levelOK := singleInt(d, "[minimum level]")
	rarity, rarityOK := singleInt(d, "[rarity]")
	if !levelOK || !rarityOK {
		return nil, out, Refuse(RefusalUnsupported, "无法核对装备等级或品质")
	}
	weapon := d.Fields["[equipment type]"][0].Text == "[weapon]"

	// 前置：必须先有次元属性（增幅书打过红字）。
	ampType := row[amplifyTypeOffset]
	if ampType == 0 {
		return nil, out, Refuse(RefusalUnsupported, "该装备没有次元属性，不能增幅（先用增幅书打红字）")
	}
	level := amplifyLevel(row[:])

	// 安全增幅按「券槽」位放的材料模板区分（与强化用材料区分 ticket/gold/safe 同套路）。
	// 便携增幅器（[portable amplify]）恒走普通材料增幅路径（safe=false），只耗道具本身。
	safe := false
	portable := false
	for _, item := range bag.Items {
		if item.Slot == r.TicketSlot {
			safe = IsAmplifySafeMaterial(item.Template)
			portable = IsPortableAmplifyTemplate(item.Template)
			break
		}
	}
	if portable {
		safe = false
	}

	var count, gold uint32
	if portable {
		// 便携增幅器固定消耗 1 个、不耗金币。
		count, gold = 1, 0
	} else if safe {
		// ★ 官方条件原文（dfoneople「Safe Amplification」）："+9 or lower Amplified"。
		// 也就是**当前 +9 时仍然可以用安全增幅打到 +10**，可行区间是「当前等级 <= +9」。
		// 早期写成 level >= 9 就拒绝，正好把官方表最后一档 +9→+10（x320 / 5,084,870 Gold）挡掉了 ——
		// 玩家每次在 +9 点增幅都被弹「材料不足」，根因就是这个 off-by-one。
		if level > AmplifySafeMaxLevel() {
			return nil, out, fmt.Errorf("安全增幅最高只能把 +%d 打到 +%d（当前已是 +%d）：更高等级请改用矛盾结晶体",
				AmplifySafeMaxLevel(), AmplifySafeMaxLevel()+1, level)
		}
		if !AmplifySafeEligible(int(equipLevel), int(rarity)) {
			return nil, out, Refuse(RefusalUnsupported, "该装备不符合安全增幅条件（100 级以上 + rare..primeval）")
		}
		c, g, ok := AmplifySafeCost(level, weapon)
		if !ok {
			return nil, out, Refuse(RefusalLimit, "当前增幅等级不在安全增幅表内")
		}
		count, gold = c, g
	} else {
		c, ok := AmplifyMaterialCount(level)
		if !ok {
			return nil, out, Refuse(RefusalLimit, "增幅等级 %d 超出规则表范围", level)
		}
		g, ok := AmplifyGold(level)
		if !ok {
			return nil, out, fmt.Errorf("增幅等级 %d 取不到金币费用", level)
		}
		count, gold = c, g
	}
	if bag.Gold < gold {
		return nil, out, Refuse(RefusalGold, "金币不足：需要 %d，持有 %d", gold, bag.Gold)
	}

	// 扣材料：请求里「券槽」位放的就是增幅材料。
	rows := append([]BagItem(nil), bag.Items...)
	remaining := uint32(0)
	found := false
	for i, item := range rows {
		if item.Slot != r.TicketSlot {
			continue
		}
		if safe {
			if !IsAmplifySafeMaterial(item.Template) {
				return nil, out, Refuse(RefusalMaterials, "增幅材料槽位放的不是安全增幅材料（槽 %d 里是模板 %d，需要 10327282）",
					r.TicketSlot, item.Template)
			}
		} else if portable {
			if !IsPortableAmplifyTemplate(item.Template) {
				return nil, out, Refuse(RefusalMaterials, "增幅材料槽位放的不是便携增幅器（槽 %d 里是模板 %d）",
					r.TicketSlot, item.Template)
			}
		} else if !IsAmplifyMaterial(item.Template) {
			return nil, out, Refuse(RefusalMaterials, "增幅材料槽位放的不是矛盾结晶体（槽 %d 里是模板 %d，需要 3242）",
				r.TicketSlot, item.Template)
		}
		if item.Amount < count {
			return nil, out, Refuse(RefusalMaterials, "增幅材料不足：需要 %d，持有 %d", count, item.Amount)
		}
		remaining = item.Amount - count
		if remaining == 0 {
			rows = append(rows[:i:i], rows[i+1:]...)
		} else {
			rows[i].Amount = remaining
		}
		found = true
		break
	}
	if !found {
		return nil, out, Refuse(RefusalItems, "增幅材料不在背包里")
	}
	bag.Gold -= gold

	// 判定成功率（官方页数据；便携增幅器沿用普通增幅成功率表）。
	percent := AmplifySuccessPercent(level)
	roll, err := amplifyUpgradeRandomInt(100)
	if err != nil {
		return nil, out, err
	}
	success := roll < percent
	// 结果码必须与「等级变化」自洽，否则客户端会判定异常并锁死增幅窗口
	// （强化那边已有同样约束：ReinforcementGoldReply 拒绝 result==1 && old != level）：
	//   0 = 成功            → 新等级 = 旧等级 + 1
	//   1 = 失败且等级不变  → 新等级 = 旧等级（客户端 handler 要求 old == new）
	//   2 = 失败且降级      → 新等级 < 旧等级（客户端 handler 要求 old > new）
	//   3 = 失败且摧毁      → 装备消失
	protected := false
	penalty := amplifyPenaltyNone
	protectionSlot := uint16(0) // 背包里增幅保护券所在槽位（客户端窗口无保护券槽，自动查找）
	result := byte(1)
	newLevel := byte(level)
	destroyed := false
	if success {
		result = 0
		newLevel = byte(level + 1)
		setAmplifyLevel(row[:], newLevel)
	} else if safe {
		// 安全增幅失败**无惩罚**（不掉级、不摧毁）—— 代价是材料贵得多
		// （对比：普通 +0 只要 1 个矛盾结晶体，安全 +0 要 36 个和谐结晶）。
		penalty = amplifyPenaltyNone
	} else {
		penalty = AmplifyPenalty(level)
		switch penalty {
		case amplifyPenaltyDown:
			// 降级只能发生在 +7 及以上，level 必 > 0；真到 0 时退化为「等级不变」。
			if level > 0 {
				newLevel = byte(level - 1)
				result = 2
			}
			setAmplifyLevel(row[:], newLevel)
		case amplifyPenaltyDestroy:
			destroyed = true
			result = 3
			// 客户端增幅窗口没有独立保护券槽位；原版语义是「背包持有保护券、
			// 失败到会碎区间自动消耗」。在背包里找增幅保护券，找到则保护：
			// 装备不破坏、增幅等级归零、消耗一张券；找不到才摧毁。
			if slot, found := findProtectionTicket(bag, true); found {
				destroyed = false
				newLevel = 0
				result = 2
				penalty = amplifyPenaltyDown
				setAmplifyLevel(row[:], 0)
				protected = true
				protectionSlot = slot
			}
		}
	}

	out = AmplifyUpgradeReceipt{
		Request: r, Equipment: gear, EquipmentSpace: space, EquipmentSlot: r.EquipmentSlot,
		AmplifyType: ampType, LevelBefore: byte(level), LevelAfter: newLevel,
		Result: result, Penalty: penalty, Destroyed: destroyed, Protected: protected, ProtectionSlot: protectionSlot, Safe: safe,
		MaterialSlot: r.TicketSlot, MaterialRemaining: remaining, MaterialSpent: count,
		GoldSpent: gold, Gold: bag.Gold, SuccessPercent: percent,
	}

	items = append([]BagEquipment(nil), items...)
	if destroyed {
		items = append(items[:index:index], items[index+1:]...)
	} else {
		gear.Record = append([]byte(nil), row[:]...)
		items[index] = gear
		out.Equipment.Record = append([]byte(nil), gear.Record...)
	}
	if space == 3 {
		bag.Worn = items
	} else {
		bag.Equipment = items
	}
	bag.Items = rows

	// 保护券触发时扣一张保护券（失败保护装备，代价是消耗券）。
	if protected {
		bag, err = consumeProtectionTicket(bag, protectionSlot)
		if err != nil {
			return nil, out, err
		}
	}

	next, err := SaveBag(role.State, bag)
	if err != nil {
		return nil, out, err
	}
	return next, out, nil
}
