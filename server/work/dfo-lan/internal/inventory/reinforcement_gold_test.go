package inventory

import (
	"path/filepath"
	"testing"
)

// 实机锚点（2026-09-27，角色 6 在强化窗口读到的数值）：
//
//	+0 布甲 100051381（115 级、品质 4）：无色 10、金币 147750
//	+0 首饰 100301826（115 级、品质 8）：无色 10、金币 147750
//	+0 太刀 101011322（115 级、品质 8、武器）：无色 10、金币 177300
//	+15 太刀：无色 220、金币 8155800
//
// 这些用例是费用公式的回归网：改权重或换配置若不再命中，说明与客户端不一致。
func loadGoldRulesForTest(t *testing.T) {
	t.Helper()
	if GoldRulesLoaded() {
		return
	}
	path := filepath.Join("..", "..", "configs", "reinforcement-gold.json")
	if err := LoadGoldRules(path); err != nil {
		t.Fatalf("装载金币强化规则失败: %v", err)
	}
	if !GoldRulesLoaded() {
		t.Skip("configs/reinforcement-gold.json 不存在，跳过金币强化用例")
	}
}

func TestGoldMaterialCountMatchesLiveAnchors(t *testing.T) {
	loadGoldRulesForTest(t)
	for _, c := range []struct {
		level int
		want  uint32
	}{{0, 10}, {15, 220}, {16, 240}} {
		got, ok := GoldMaterialCount(c.level)
		if !ok {
			t.Fatalf("等级 %d 没有材料数量", c.level)
		}
		if got != c.want {
			t.Errorf("等级 %d 材料数量 = %d，实机为 %d", c.level, got, c.want)
		}
	}
}

func TestGoldCostMatchesLiveAnchors(t *testing.T) {
	loadGoldRulesForTest(t)
	cases := []struct {
		name       string
		equipLevel int
		rarity     int
		level      int
		weapon     bool
		want       uint32
	}{
		{"+0 布甲(115,品质4)", 115, 4, 0, false, 147750},
		{"+0 首饰(115,品质8)", 115, 8, 0, false, 147750},
		{"+0 太刀(115,品质8,武器)", 115, 8, 0, true, 177300},
		{"+15 太刀(115,品质8,武器)", 115, 8, 15, true, 8155800},
	}
	for _, c := range cases {
		got, err := GoldCost(c.equipLevel, c.rarity, c.level, c.weapon)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s 金币 = %d，实机为 %d", c.name, got, c.want)
		}
	}
}

func TestGoldCostScalesWithEquipmentLevel(t *testing.T) {
	loadGoldRulesForTest(t)
	// 低等级装备走 [cost] 基础表（非 100lv 覆盖），必须明显更便宜。
	low, err := GoldCost(10, 4, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	high, err := GoldCost(115, 4, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if low >= high || low == 0 {
		t.Errorf("10 级装备金币 %d 不小于 115 级 %d", low, high)
	}
}

func TestGoldSuccessPercentByLevel(t *testing.T) {
	loadGoldRulesForTest(t)
	want := map[int]int{0: 100, 3: 100, 4: 80, 9: 30, 10: 25, 11: 20, 15: 11, 16: 10, 40: 10}
	for level, rate := range want {
		got, ok := GoldSuccessPercent(level)
		if !ok {
			t.Fatalf("等级 %d 没有成功率", level)
		}
		if got != rate {
			t.Errorf("等级 %d 成功率 = %d%%，实测表为 %d%%", level, got, rate)
		}
	}
}

func TestGoldPenaltyTable(t *testing.T) {
	loadGoldRulesForTest(t)
	cases := []struct {
		level  int
		weapon bool
		want   goldPenalty
	}{
		{0, true, goldPenaltyKeep},
		{3, false, goldPenaltyKeep},
		{4, true, goldPenaltyKeep},
		{9, true, goldPenaltyKeep},
		{4, false, goldPenaltyDownOne},
		{9, false, goldPenaltyDownOne},
		{10, true, goldPenaltyDownThree},
		{11, true, goldPenaltyDownThree},
		{10, false, goldPenaltyDestroy},
		{12, true, goldPenaltyDestroy},
		{15, false, goldPenaltyDestroy},
	}
	for _, c := range cases {
		if got := GoldPenalty(c.level, c.weapon); got != c.want {
			t.Errorf("等级 %d 武器=%v 惩罚 = %v，期望 %v", c.level, c.weapon, got, c.want)
		}
	}
}

func TestGoldMaterialRecognition(t *testing.T) {
	loadGoldRulesForTest(t)
	if !IsGoldMaterial(3037) {
		t.Error("无色小晶块 3037 应被认可为强化材料")
	}
	// 3171（炉岩核 Ryan Core）走的是窗口另一侧的安全强化，费用/数量来自 [safe upgrade]，
	// 不能按普通强化的矩阵数量扣料 —— 未确认前必须拒绝。
	if IsGoldMaterial(3171) {
		t.Error("炉岩核 3171 属安全强化路径，不应被当作普通强化材料")
	}
	if IsGoldMaterial(3038) {
		t.Error("未登记模板不应被当作强化材料")
	}
	if def, ok := GoldMaterialDefinition(3037); !ok || def.Path != "stackable/material/cubepiece_clear.stk" {
		t.Errorf("无色小晶块定义 = %+v，期望 cubepiece_clear", def)
	}
}

// 安全强化：仅武器、上限 +12、0-9 与普通强化同表，10→11 与 11→12 有失败补正（成功清零）。
func TestSafeSuccessRateBonus(t *testing.T) {
	loadGoldRulesForTest(t)
	if !SafePathWeaponOnly() {
		t.Error("安全强化应仅对武器开放")
	}
	if got := SafePathMaxLevel(); got != 12 {
		t.Errorf("安全强化最高等级 = %d，期望 12", got)
	}
	if !SafePathResetsStreakOnSuccess() {
		t.Error("成功应清零失败补正")
	}
	for _, c := range []struct {
		level, streak, want int
	}{
		{0, 0, 100}, {3, 9, 100}, {4, 0, 80}, {9, 0, 30}, // 0-3 与普通强化同表；4/9 级基础值
		{4, 1, 85}, {4, 4, 100}, {8, 2, 50}, {9, 3, 45}, // 官方：4→5..9→10 失败每次 +5%p
		{10, 0, 8}, {10, 1, 10}, {10, 5, 18}, {10, 40, 40}, // 8% +2%p/次，封顶 40%
		{11, 0, 3}, {11, 1, 4}, {11, 10, 13}, {11, 30, 25}, // 3% +1%p/次，封顶 25%
	} {
		got, ok := SafeSuccessPercent(c.level, c.streak)
		if !ok {
			t.Fatalf("等级 %d 连续失败 %d 次没有安全强化概率", c.level, c.streak)
		}
		if got != c.want {
			t.Errorf("安全强化 %d 级(连败 %d) = %d%%，期望 %d%%", c.level, c.streak, got, c.want)
		}
	}
	if _, ok := SafeSuccessPercent(12, 0); ok {
		t.Error("12 级以上不应有安全强化概率")
	}
}

// 安全强化（窗口右侧）的费用表来自 [safe upgrade]，材料 10327281 / 替代 10327284，
// 条件 = 100 级以上 + rare..primeval。
func TestSafeUpgradeTableShape(t *testing.T) {
	loadGoldRulesForTest(t)
	rows := goldRules.Failure.SafeUpgrade
	if len(rows) == 0 {
		t.Fatal("配置缺少 [safe upgrade] 数据")
	}
	first, last := rows[0], rows[len(rows)-1]
	if first.Level != 0 || first.Gold != 0 || first.Material != 10327281 || first.Count != 40 {
		t.Errorf("安全强化 0 级 = %+v，期望 0 金币 / 10327281 × 40", first)
	}
	if last.Level != 11 || last.Gold != 94000 || last.Count != 1529 {
		t.Errorf("安全强化 11 级 = %+v，期望 94000 金币 / 1529 个", last)
	}
	if got := SafeUpgradeMaxLevel(); got != 11 {
		t.Errorf("安全强化最高等级 = %d，期望 11", got)
	}
	if _, count, gold, ok := SafeUpgradeCost(4); !ok || count != 81 || gold != 20000 {
		t.Errorf("安全强化 4 级 = %d 个 / %d 金币，期望 81 / 20000", count, gold)
	}
	if !IsSafeMaterial(10327281) || !IsSafeMaterial(10327284) {
		t.Error("10327281 / 10327284 应被认可为安全强化材料")
	}
	if IsGoldMaterial(10327281) {
		t.Error("安全强化材料不应被当作普通强化材料")
	}
	// 条件：100 级以上 + rare/unique/epic/legendary/mythology/primeval
	for _, c := range []struct {
		level  int
		rarity int
		want   bool
	}{{115, 4, true}, {115, 8, true}, {100, 2, true}, {115, 0, false}, {115, 1, false}, {115, 5, false}, {90, 4, false}} {
		if got := SafeUpgradeEligible(c.level, c.rarity); got != c.want {
			t.Errorf("安全强化条件(等级 %d 品质 %d) = %v，期望 %v", c.level, c.rarity, got, c.want)
		}
	}
}

func TestGoldMaxUpgradeLevelClampedToLevelField(t *testing.T) {
	loadGoldRulesForTest(t)
	// 实机 2026-09-27：CMD80 的结果等级超过 15 时客户端上报 ADD_HACKTYPE_CNT 并锁死强化面板，
	// 所以上限取 15（与固定券分支同一范围），而不是 PVF 里 [max upgrade level by rarity] 的 50。
	if got := GoldMaxUpgradeLevel(); got != 15 {
		t.Errorf("最高结果等级 = %d，实机上限应为 15", got)
	}
	if got := GoldMaxUpgradeLevel(); got > goldLevelFieldMask {
		t.Errorf("最高结果等级 %d 超出实例行可表达范围 %d", got, goldLevelFieldMask)
	}
}
