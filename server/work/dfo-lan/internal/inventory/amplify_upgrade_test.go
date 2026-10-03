package inventory

// 增幅费用规则回归使用完整历史压缩夹具，运行真源为 PVF。
// （历史 PVF 导出 从 PVF 的 etc/amplifyupgrade.etc 只读导出）。

import (
	"dfolan/internal/testfixture"
	"testing"
)

func loadAmplifyUpgradeRulesForTest(t *testing.T) {
	t.Helper()
	if amplifyUpgradeRules != nil {
		return
	}
	path := testfixture.EnhancementPath(t, "amplify-upgrade.json")
	if err := LoadAmplifyUpgradeRules(path); err != nil {
		t.Fatalf("装载增幅规则失败: %v", err)
	}
	if !AmplifyUpgradeRulesLoaded() {
		t.Fatal("historical amplify-upgrade.json fixture failed to activate")
	}
}

// 材料与消耗：矩阵里材料恒为 3242（矛盾结晶体），数量 = 当前等级 + 1。
func TestAmplifyUpgradeMaterialAndCost(t *testing.T) {
	loadAmplifyUpgradeRulesForTest(t)
	if !IsAmplifyMaterial(3242) {
		t.Error("3242（矛盾结晶体）应被识别为增幅材料")
	}
	if IsAmplifyMaterial(3037) {
		t.Error("3037（无色小晶块）是强化材料，不应当增幅材料")
	}
	if got, ok := AmplifyMaterialCount(0); !ok || got != 1 {
		t.Errorf("+0 增幅消耗 = %d,%v，期望 1", got, ok)
	}
	if got, ok := AmplifyMaterialCount(9); !ok || got != 10 {
		t.Errorf("+9 增幅消耗 = %d,%v，期望 10", got, ok)
	}
	if got, ok := AmplifyGold(0); !ok || got != 0 {
		t.Errorf("+0 增幅金币 = %d,%v，期望 0", got, ok)
	}
	if got, ok := AmplifyGold(4); !ok || got != 20000 {
		t.Errorf("+4 增幅金币 = %d,%v，期望 20000", got, ok)
	}
}

// 成功率来自官方页（PVF 里没有成功率表，强化已证实）。
func TestAmplifyUpgradeSuccessRate(t *testing.T) {
	loadAmplifyUpgradeRulesForTest(t)
	want := map[int]int{0: 100, 1: 100, 2: 100, 3: 100, 4: 70, 5: 60, 6: 50, 7: 50, 8: 40, 9: 30}
	for level, rate := range want {
		if got := AmplifySuccessPercent(level); got != rate {
			t.Errorf("+%d 成功率 = %d，期望 %d", level, got, rate)
		}
	}
}

// 安全增幅：材料 10327282（和谐结晶）、上限 +9、门槛 100 级 + rare..primeval，
// 且武器与非武器消耗不同（[safe upgrade] 每级两行）。
func TestAmplifySafeUpgradeRules(t *testing.T) {
	loadAmplifyUpgradeRulesForTest(t)
	if !IsAmplifySafeMaterial(10327282) {
		t.Error("10327282（和谐结晶）应被识别为安全增幅材料")
	}
	if IsAmplifySafeMaterial(3242) {
		t.Error("3242 是普通增幅材料，不应当安全增幅材料")
	}
	if got := AmplifySafeMaxLevel(); got != 9 {
		t.Errorf("安全增幅上限 = %d，期望 9", got)
	}
	// 门槛：100 级 + unique（序号 3）通过；99 级或 common（序号 0）不通过。
	if !AmplifySafeEligible(100, 3) {
		t.Error("100 级 unique 装备应可安全增幅")
	}
	if AmplifySafeEligible(99, 3) {
		t.Error("99 级装备不满足 100 级门槛")
	}
	if AmplifySafeEligible(100, 0) {
		t.Error("common 品质不在白名单内")
	}
	// 武器与非武器消耗不同（官方：武器与非武器消耗不同）。
	wCount, wGold, ok1 := AmplifySafeCost(0, true)
	nCount, nGold, ok2 := AmplifySafeCost(0, false)
	if !ok1 || !ok2 {
		t.Fatal("安全增幅 +0 的费用应能取到（武器 / 非武器）")
	}
	if wCount != 36 || nCount != 16 {
		t.Errorf("安全增幅 +0 材料：武器=%d 非武器=%d，期望 36 / 16", wCount, nCount)
	}
	// 官方页「Safe Amplification」表：+0→1 武器 x36 / 430,100 Gold，非武器 x16 / 189,860 Gold。
	// （金币取 [safe upgrade] 第 8 列；早期错取第 3 列，导致安全增幅一分钱不收。）
	if wGold != 430100 || nGold != 189860 {
		t.Errorf("安全增幅 +0 金币：武器=%d 非武器=%d，期望 430100 / 189860", wGold, nGold)
	}
	// 安全增幅明显比普通增幅贵（普通 +0 只要 1 个矛盾结晶体）。
	if plain, ok := AmplifyMaterialCount(0); !ok || plain != 1 {
		t.Errorf("普通增幅 +0 消耗 = %d,%v，期望 1", plain, ok)
	}
}

// 失败惩罚：+0~+6 无变化、+7~+9 降一级、+10 以上摧毁。
func TestAmplifyUpgradePenalty(t *testing.T) {
	loadAmplifyUpgradeRulesForTest(t)
	cases := []struct {
		level int
		want  string
	}{
		{0, amplifyPenaltyNone},
		{6, amplifyPenaltyNone},
		{7, amplifyPenaltyDown},
		{9, amplifyPenaltyDown},
		{10, amplifyPenaltyDestroy},
		{15, amplifyPenaltyDestroy},
	}
	for _, c := range cases {
		if got := AmplifyPenalty(c.level); got != c.want {
			t.Errorf("+%d 失败惩罚 = %q，期望 %q", c.level, got, c.want)
		}
	}
}

// 增幅等级 = 偏移 10 的低五位（与强化共用同一个字节）。
// offset 19 是次元属性类型、offset 20 是次元属性数值（红字）——增幅绝不能碰这两个字节：
// 早期把等级写进 offset 20，结果是红字数值被等级覆盖、等级字节仍是 0，装备上看不到任何「+N」。
func TestAmplifyLevelUsesReinforceByte(t *testing.T) {
	row := make([]byte, 181)
	row[amplifyReinforceOffset] = 0x60 // 高三位 = 再封装次数 3
	row[amplifyTypeOffset] = 3
	row[amplifyValueOffset] = 7
	if got := amplifyLevel(row); got != 0 {
		t.Fatalf("初始增幅等级应为 0，实际 %d", got)
	}
	setAmplifyLevel(row, 9)
	if got := amplifyLevel(row); got != 9 {
		t.Fatalf("写入 +9 后应读到 9，实际 %d", got)
	}
	if row[amplifyReinforceOffset]>>5 != 3 {
		t.Errorf("写等级破坏了高三位的再封装次数：%#02x", row[amplifyReinforceOffset])
	}
	if row[amplifyValueOffset] != 7 {
		t.Errorf("增幅改写了次元属性数值（offset 20）：%d", row[amplifyValueOffset])
	}
	if row[amplifyTypeOffset] != 3 {
		t.Errorf("增幅改写了次元属性类型（offset 19）：%d", row[amplifyTypeOffset])
	}
}

// 官方「Safe Amplification」表（dfoneople）：条件原文是 "+9 or lower Amplified"，
// 也就是 **+9 → +10 是安全路径允许的最后一档**（武器 x320 / 5,084,870 Gold，非武器 x277 / 2,937,440 Gold）。
// 早期把边界写成 level >= 9 拒绝，正好把这一档挡掉 —— 玩家每次在 +9 点增幅都失败。
func TestAmplifySafeUpgradeAllowsNineToTen(t *testing.T) {
	loadAmplifyUpgradeRulesForTest(t)
	if got := AmplifySafeMaxLevel(); got != 9 {
		t.Fatalf("安全增幅的当前等级上限应为 9（即允许打到 +10），实际 %d", got)
	}
	if count, gold, ok := AmplifySafeCost(9, true); !ok {
		t.Fatal("安全增幅 +9→+10（武器）应能取到费用")
	} else if count != 320 || gold != 5084870 {
		t.Errorf("安全增幅 +9→+10 武器应为 x320 / 5,084,870 Gold，实际 x%d / %d", count, gold)
	}
	if count, gold, ok := AmplifySafeCost(9, false); !ok {
		t.Fatal("安全增幅 +9→+10（非武器）应能取到费用")
	} else if count != 277 || gold != 2937440 {
		t.Errorf("安全增幅 +9→+10 非武器应为 x277 / 2,937,440 Gold，实际 x%d / %d", count, gold)
	}
	// +10 起必须改用矛盾结晶体，安全表里不该再有这一行。
	if _, _, ok := AmplifySafeCost(10, true); ok {
		t.Error("安全增幅表不应包含 +10 → +11")
	}
}
