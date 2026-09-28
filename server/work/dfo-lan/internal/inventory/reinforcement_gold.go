package inventory

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// 金币强化（普通强化）的服务端规则。数据来自 configs/reinforcement-gold.json，
// 该文件由 scripts/export_reinforcement_gold.py 从客户端 PVF 的 etc/upgrade.etc 导出，
// 成功率与失败惩罚一段是玩家实测值（客户端不含成功率表）。
//
// 费用公式（四点实机验证：+0 防具/首饰 147750、+0 武器 177300、+15 武器 8155800）：
//
//	材料数量 = levels[当前强化等级].material_count          （模板 3037 / 3171 共用）
//	金币     = base[装备等级] × rarityWeight[品质] × levelWeight[当前等级][0] × (武器 ? 1.2 : 1)
//
// 其中装备等级 ≥100 用 [cost 100lv] 覆盖表；等级权与品质权都取自 upgrade.etc 的命名段。
type goldRulesConfig struct {
	Version                  int                      `json:"version"`
	Source                   string                   `json:"source"`
	Table                    string                   `json:"table"`
	Materials                []goldMaterialDefinition `json:"materials"`
	MaterialsPendingSafePath []goldMaterialDefinition `json:"materials_pending_safe_path"`
	SafePathMaterials        []uint32                 `json:"safe_path_materials"`
	SafePathRules            struct {
		WeaponOnly              bool `json:"weapon_only"`
		MaxLevel                int  `json:"max_level"`
		RateSameAsNormalThrough int  `json:"rate_same_as_normal_through"`
		StageRate               map[string]struct {
			Base int `json:"base"`
			Step int `json:"step"`
			Cap  int `json:"cap"`
		} `json:"stage_rate"`
		ResetOnSuccess bool `json:"reset_on_success"`
	} `json:"safe_path_rules"`
	MaxUpgradeLevel        int             `json:"max_upgrade_level"`
	MaterialCountSemantics string          `json:"material_count_semantics"`
	Levels                 []goldLevelRule `json:"levels"`
	Gold                   struct {
		BaseByEquipLevel        []uint32  `json:"base_by_equip_level"`
		BaseByEquipLevel100Lv   []uint32  `json:"base_by_equip_level_100lv"`
		Base100LvFromEquipLevel int       `json:"base_100lv_from_equip_level"`
		RarityWeight            []float64 `json:"rarity_weight"`
		RarityWeight100Lv       []float64 `json:"rarity_weight_100lv"`
		LevelWeight             []struct {
			Level   int       `json:"level"`
			Weights []float64 `json:"weights"`
		} `json:"level_weight"`
		WeaponFactor float64 `json:"weapon_factor"`
	} `json:"gold"`
	Failure struct {
		DestroyEnabled          bool                 `json:"destroy_enabled"`
		SafeUpgrade             []goldSafeUpgradeRow `json:"safe_upgrade"`
		SafeUpgradeMinLevel     []int                `json:"safe_upgrade_min_level"`
		SafeUpgradeUsableRarity []string             `json:"safe_upgrade_usable_rarity"`
		SafeUpgradeReplaceItem  []uint32             `json:"safe_upgrade_replace_item"`
		PlayerMeasured          struct {
			Source                    string         `json:"source"`
			SuccessRatePercentByLevel map[string]int `json:"success_rate_percent_by_level"`
		} `json:"player_measured"`
	} `json:"failure"`
}

// 品质序号 → 名字，与客户端 [correction grade by rarity] 的列举顺序一致（0 起）。
var equipmentRarityNames = []string{
	"common", "uncommon", "rare", "unique", "epic", "chronicle", "legendary", "mythology", "primeval",
}

// 安全强化（窗口另一侧 safeMaterialPanel）逐级费用：等级 / 启用 / 金币 / b / c / 材料模板 / 数量 / 价值。
// 它和普通强化的矩阵数量、[cost] 金币是两套独立曲线，所以 3171 这类材料不能按普通强化扣料。
type goldSafeUpgradeRow struct {
	Level    int    `json:"level"`
	Enabled  int    `json:"enabled"`
	Gold     uint32 `json:"gold"`
	B        int    `json:"b"`
	C        int    `json:"c"`
	Material uint32 `json:"material"`
	Count    uint32 `json:"count"`
	Value    uint32 `json:"value"`
}

type goldMaterialDefinition struct {
	Template uint32   `json:"template"`
	Path     string   `json:"path"`
	Name     string   `json:"name"`
	Where    []string `json:"where"`
}

type goldLevelRule struct {
	Level            int    `json:"level"`
	MaterialTemplate uint32 `json:"material_template"`
	MaterialCount    uint32 `json:"material_count"`
}

// 强化等级写在装备实例行偏移 10 的低五位（券路径同一约定）。
const goldLevelFieldMask = 31

var goldRules *goldRulesConfig

// LoadGoldRules 读取金币强化规则；文件不存在时保持未加载状态，金币路径整体拒绝。
func LoadGoldRules(path string) error {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var c goldRulesConfig
	if err = json.Unmarshal(b, &c); err != nil {
		return err
	}
	if c.Version != 1 || len(c.Source) != 64 || len(c.Levels) == 0 || len(c.Gold.BaseByEquipLevel) == 0 ||
		len(c.Gold.RarityWeight) == 0 || len(c.Gold.RarityWeight100Lv) == 0 || len(c.Gold.LevelWeight) == 0 ||
		len(c.Materials) == 0 || c.MaxUpgradeLevel <= 0 {
		return fmt.Errorf("金币强化规则源定义不完整")
	}
	for _, lv := range c.Levels {
		if lv.Level < 0 || lv.MaterialCount == 0 {
			return fmt.Errorf("金币强化规则第 %d 级无效", lv.Level)
		}
	}
	goldRules = &c
	return nil
}

// GoldRulesLoaded 报告金币强化规则是否已装载。
func GoldRulesLoaded() bool { return goldRules != nil }

// IsGoldMaterial 判断模板是否是被认可的金币强化材料（无色小晶块 / 炉岩核）。
func IsGoldMaterial(template uint32) bool {
	if goldRules == nil {
		return false
	}
	for _, m := range goldRules.Materials {
		if m.Template == template {
			return true
		}
	}
	return false
}

// GoldMaterialDefinition 返回材料定义。
func GoldMaterialDefinition(template uint32) (goldMaterialDefinition, bool) {
	if goldRules == nil {
		return goldMaterialDefinition{}, false
	}
	for _, m := range goldRules.Materials {
		if m.Template == template {
			return m, true
		}
	}
	return goldMaterialDefinition{}, false
}

// GoldMaterialCount 返回某强化等级需要的材料数量（两种材料同数量）。
func GoldMaterialCount(level int) (uint32, bool) {
	if goldRules == nil || level < 0 || level >= len(goldRules.Levels) {
		return 0, false
	}
	return goldRules.Levels[level].MaterialCount, true
}

// GoldSuccessPercent 返回该强化等级的基础成功率（百分数）。
func GoldSuccessPercent(level int) (int, bool) {
	if goldRules == nil {
		return 0, false
	}
	table := goldRules.Failure.PlayerMeasured.SuccessRatePercentByLevel
	if table == nil {
		return 0, false
	}
	if v, ok := table[fmt.Sprint(level)]; ok {
		return v, true
	}
	if v, ok := table["default"]; ok {
		return v, true
	}
	return 0, false
}

// GoldMaxUpgradeLevel 返回本表允许的最高强化结果等级（客户端行内五位上限 31 更紧）。
func GoldMaxUpgradeLevel() int {
	if goldRules == nil {
		return 0
	}
	max := goldRules.MaxUpgradeLevel
	if max > goldLevelFieldMask {
		max = goldLevelFieldMask
	}
	return max
}

// goldBaseByEquipLevel 解析 [cost] / [cost 100lv] 覆盖表。
func goldBaseByEquipLevel(equipLevel int) (uint32, error) {
	g := goldRules.Gold
	if len(g.BaseByEquipLevel100Lv) > 0 && equipLevel >= g.Base100LvFromEquipLevel {
		index := equipLevel - g.Base100LvFromEquipLevel
		if index >= len(g.BaseByEquipLevel100Lv) {
			index = len(g.BaseByEquipLevel100Lv) - 1
		}
		return g.BaseByEquipLevel100Lv[index], nil
	}
	if equipLevel < 0 {
		return 0, fmt.Errorf("装备等级无效")
	}
	if equipLevel >= len(g.BaseByEquipLevel) {
		equipLevel = len(g.BaseByEquipLevel) - 1
	}
	return g.BaseByEquipLevel[equipLevel], nil
}

func goldRarityWeight(equipLevel, rarity int) (float64, error) {
	g := goldRules.Gold
	table := g.RarityWeight
	if len(g.RarityWeight100Lv) > 0 && equipLevel >= g.Base100LvFromEquipLevel {
		table = g.RarityWeight100Lv
	}
	if rarity < 0 || rarity >= len(table) {
		return 0, fmt.Errorf("装备品质 %d 不在金币强化权重表内", rarity)
	}
	return table[rarity], nil
}

func goldLevelWeight(level int) (float64, error) {
	rows := goldRules.Gold.LevelWeight
	if len(rows) == 0 {
		return 0, fmt.Errorf("金币强化等级权重表为空")
	}
	if level < 0 {
		return 0, fmt.Errorf("强化等级无效")
	}
	if level >= len(rows) {
		level = len(rows) - 1
	}
	w := rows[level].Weights
	if len(w) == 0 {
		return 0, fmt.Errorf("金币强化等级权重第 %d 级为空", level)
	}
	return w[0], nil
}

// GoldCost 计算一次强化的金币消耗。
func GoldCost(equipLevel, rarity, level int, weapon bool) (uint32, error) {
	if goldRules == nil {
		return 0, fmt.Errorf("金币强化规则未装载")
	}
	base, err := goldBaseByEquipLevel(equipLevel)
	if err != nil {
		return 0, err
	}
	rarityWeight, err := goldRarityWeight(equipLevel, rarity)
	if err != nil {
		return 0, err
	}
	levelWeight, err := goldLevelWeight(level)
	if err != nil {
		return 0, err
	}
	factor := 1.0
	if weapon {
		factor = goldRules.Gold.WeaponFactor
	}
	gold := float64(base) * rarityWeight * levelWeight * factor
	if gold <= 0 {
		return 0, nil
	}
	if gold > math.MaxUint32 {
		return 0, fmt.Errorf("金币消耗溢出")
	}
	return uint32(gold + 0.5), nil
}

// goldPenalty 是失败后的处理方式。
type goldPenalty int

const (
	goldPenaltyKeep      goldPenalty = iota // 等级不变
	goldPenaltyDownOne                      // 掉 1 级
	goldPenaltyDownThree                    // 掉 3 级
	goldPenaltyDestroy                      // 装备被破坏
)

// GoldPenalty 按实测表给出失败惩罚：0-3 无、4-9 武器不掉级/其他 -1、
// 10-11 武器 -3/其他破坏、12 以上全部破坏。destroy_enabled 关闭时把破坏降级为“等级不变”。
func GoldPenalty(level int, weapon bool) goldPenalty {
	switch {
	case level <= 3:
		return goldPenaltyKeep
	case level <= 9:
		if weapon {
			return goldPenaltyKeep
		}
		return goldPenaltyDownOne
	case level <= 11:
		if weapon {
			return goldPenaltyDownThree
		}
		return goldPenaltyDestroy
	default:
		return goldPenaltyDestroy
	}
}

func goldDestroyEnabled() bool { return goldRules != nil && goldRules.Failure.DestroyEnabled }

// GoldRulesSource 返回导出用的 PVF 摘要，便于把配置与实机核对。
func GoldRulesSource() string {
	if goldRules == nil {
		return ""
	}
	return goldRules.Source
}

// IsSafeMaterial 判断模板是否是窗口另一侧（安全强化）的材料：
// 实机左侧 @9=367（无色小晶块）、右侧 @9=136（10327281）且 tail[4]=1，两条路请求形状只差这个标志位。
func IsSafeMaterial(template uint32) bool {
	if goldRules == nil {
		return false
	}
	for _, m := range goldRules.SafePathMaterials {
		if m == template {
			return true
		}
	}
	return false
}

// SafeUpgradeMaxLevel 返回 [safe upgrade] 覆盖的最高强化等级（本版 0..11）。
func SafeUpgradeMaxLevel() int {
	if goldRules == nil || len(goldRules.Failure.SafeUpgrade) == 0 {
		return -1
	}
	max := 0
	for _, row := range goldRules.Failure.SafeUpgrade {
		if row.Level > max {
			max = row.Level
		}
	}
	return max
}

// SafeUpgradeCost 返回安全强化在该等级的材料数量与金币（材料模板由 [safe upgrade] 固定给出）。
func SafeUpgradeCost(level int) (uint32, uint32, uint32, bool) {
	if goldRules == nil {
		return 0, 0, 0, false
	}
	for _, row := range goldRules.Failure.SafeUpgrade {
		if row.Level == level {
			return row.Material, row.Count, row.Gold, true
		}
	}
	return 0, 0, 0, false
}

// SafeUpgradeEligible 核对 [safe upgrade item condition]：装备等级下限 + 允许的品质集合。
func SafeUpgradeEligible(equipLevel, rarity int) bool {
	if goldRules == nil {
		return false
	}
	if min := goldRules.Failure.SafeUpgradeMinLevel; len(min) > 0 {
		lowest := min[0]
		for _, v := range min {
			if v > lowest {
				lowest = v
			}
		}
		if equipLevel < lowest {
			return false
		}
	}
	if rarity < 0 || rarity >= len(equipmentRarityNames) {
		return false
	}
	name := equipmentRarityNames[rarity]
	for _, usable := range goldRules.Failure.SafeUpgradeUsableRarity {
		if usable == name {
			return true
		}
	}
	return false
}

// SafePathWeaponOnly 报告安全强化是否仅限武器（原版 115US：是）。
func SafePathWeaponOnly() bool {
	return goldRules != nil && goldRules.SafePathRules.WeaponOnly
}

// SafePathMaxLevel 返回安全强化能到达的最高等级（原版 +12，所以 11→12 仍可点）。
func SafePathMaxLevel() int {
	if goldRules == nil || goldRules.SafePathRules.MaxLevel <= 0 {
		return SafeUpgradeMaxLevel() + 1
	}
	return goldRules.SafePathRules.MaxLevel
}

// SafeSuccessPercent 返回安全强化在某个等级的成功率。
// 0→1 .. 9→10 与普通强化同表；10→11 基础 8%、11→12 基础 3%，
// 每次失败按 stage_rate 的 step 递增，封顶 cap（成功清零，计数由调用方持久化）。
func SafeSuccessPercent(level, streak int) (int, bool) {
	if goldRules == nil {
		return 0, false
	}
	limit := goldRules.SafePathRules.RateSameAsNormalThrough
	if level <= limit {
		return GoldSuccessPercent(level)
	}
	stage, ok := goldRules.SafePathRules.StageRate[fmt.Sprint(level)]
	if !ok || stage.Base <= 0 {
		return 0, false
	}
	rate := stage.Base
	if streak > 0 && stage.Step > 0 {
		rate += stage.Step * streak
	}
	if stage.Cap > 0 && rate > stage.Cap {
		rate = stage.Cap
	}
	return rate, true
}

// SafePathResetsStreakOnSuccess 报告成功是否清零失败补正。
func SafePathResetsStreakOnSuccess() bool {
	return goldRules == nil || goldRules.SafePathRules.ResetOnSuccess
}
