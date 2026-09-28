package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

// 增幅书（红字书）清单来自 scripts/export_amplify_grimoire.py：
// 识别方式是物品脚本含 [amplification random value]，段内是 (次元属性数值, 权重) 加权表。
func loadGrimoiresForTest(t *testing.T) {
	t.Helper()
	if AmplifyGrimoiresLoaded() {
		return
	}
	path := filepath.Join("..", "..", "configs", "amplify-grimoire.json")
	if err := LoadAmplifyGrimoires(path); err != nil {
		t.Fatalf("装载增幅书清单失败: %v", err)
	}
	if !AmplifyGrimoiresLoaded() {
		t.Skip("configs/amplify-grimoire.json 不存在，跳过增幅书用例")
	}
}

func TestAmplifyGrimoireRecognition(t *testing.T) {
	loadGrimoiresForTest(t)
	// 玩家背包里那本（10356325，脚本无到期时间）：数值 3..6，权重 40/30/20/10。
	table, ok := AmplifyGrimoireValueTable(10356325)
	if !ok {
		t.Fatal("10356325 应被识别为增幅书")
	}
	want := []amplifyValueWeight{{3, 40}, {4, 30}, {5, 20}, {6, 10}}
	if len(table) != len(want) {
		t.Fatalf("数值表 = %v，期望 %v", table, want)
	}
	for i := range want {
		if table[i] != want[i] {
			t.Fatalf("数值表 = %v，期望 %v", table, want)
		}
	}
	if _, ok := AmplifyGrimoireValueTable(3037); ok {
		t.Error("无色小晶块不应被当成增幅书")
	}
}

// 摇出来的数值必须落在书自己的表里，且不得为 0 —— 数值 0 在客户端等于「没有次元属性」。
func TestAmplifyGrimoireRollValue(t *testing.T) {
	loadGrimoiresForTest(t)
	saved := amplifyRandomInt
	defer func() { amplifyRandomInt = saved }()
	seen := map[byte]int{}
	for roll := 0; roll < 100; roll++ {
		amplifyRandomInt = func(n int) (int, error) { return roll % n, nil }
		v, ok := AmplifyGrimoireRollValue(10356325)
		if !ok {
			t.Fatalf("第 %d 次摇号失败", roll)
		}
		if v < 3 || v > 6 {
			t.Fatalf("摇出数值 %d，不在 3..6", v)
		}
		seen[v]++
	}
	for _, v := range []byte{3, 4, 5, 6} {
		if seen[v] == 0 {
			t.Errorf("数值 %d 从未被摇出：%v", v, seen)
		}
	}
	if _, ok := AmplifyGrimoireRollValue(3037); ok {
		t.Error("无色小晶块不应能摇出数值")
	}
}

func TestAmplifyTypeNames(t *testing.T) {
	for want, name := range map[int]string{3: "力量", 4: "智力", 1: "体力", 2: "精神"} {
		if got := AmplifyTypeName(want); got != name {
			t.Errorf("类型 %d 名称 = %q，期望 %q", want, got, name)
		}
	}
	if AmplifyTypeName(99) == "" {
		t.Error("未知类型也应有可读名称")
	}
}

// 黄金增幅书按脚本路径里的 golden 识别（纯净的黄金增幅书会删除强化等级）；普通/银书/超级书不识别。
func TestIsGoldenGrimoireFromPath(t *testing.T) {
	saved := amplifyGrimoires
	defer func() { amplifyGrimoires = saved }()

	const src = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	doc := `{"version":1,"source":"` + src + `","rule":"x",` +
		`"types":{"1":"体力","2":"精神","3":"力量","4":"智力"},` +
		`"grimoires":[` +
		`{"template":70001,"path":"stackable/dfo/cash/lost_treasure/2015/0818/amplification_book_golden.stk","random":[{"value":7,"weight":50}],"expires":true},` +
		`{"template":70002,"path":"stackable/dfo/cash/lost_treasure/2015/0818/amplification_book_silver.stk","random":[{"value":3,"weight":50}],"expires":true},` +
		`{"template":70003,"path":"stackable/dfo/cash/2022/1108/black_friday/grimorie/super_amplification_grimoire.stk","random":[{"value":8,"weight":50}],"expires":false}` +
		`]}`
	path := filepath.Join(t.TempDir(), "amplify-grimoire.json")
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := LoadAmplifyGrimoires(path); err != nil {
		t.Fatal(err)
	}
	if !IsGoldenGrimoire(70001) {
		t.Error("路径含 golden 的 70001 应识别为黄金增幅书")
	}
	if IsGoldenGrimoire(70002) {
		t.Error("silver 书 70002 不应是黄金增幅书")
	}
	if IsGoldenGrimoire(70003) {
		t.Error("super_amplification_grimoire 70003 不应是黄金增幅书（它只加红字、不删强化等级）")
	}
	if IsGoldenGrimoire(3037) {
		t.Error("无色小晶块不应被当成增幅书")
	}
}

// 所有增幅书统一规则：**摇出的数值同时就是增幅等级**（offset 10 低五位）。
// 白银/普通 3..6、强烈 8..13、黄金 8..12 —— 打完红字就该直接看到 +N，
// 否则客户端只显示「增幅 +0」，玩家看到的就是「红字打上了、等级没动」。
func TestAmplifyGrimoireWritesLevelByte(t *testing.T) {
	// 白银书摇到 5：等级应变成 +5，且不能动再封装次数（bit5-7）。
	row := make([]byte, 181)
	row[amplifyReinforceOffset] = 0x60 // 再封装次数 = 3
	if got := applyGrimoireLevel(row, 5); got != 5 {
		t.Fatalf("白银书打完应为 +5，实际 +%d", got)
	}
	if row[amplifyReinforceOffset]>>5 != 3 {
		t.Errorf("写等级破坏了再封装次数：%#02x", row[amplifyReinforceOffset])
	}
	// 原强化 +10 的装备用增幅书打红字：等级被摇值覆盖（不再是 10）。
	row2 := make([]byte, 181)
	row2[amplifyReinforceOffset] = 10
	applyGrimoireLevel(row2, 3)
	if amplifyLevel(row2) != 3 {
		t.Errorf("增幅书应把等级写成摇出的 3，实际 %d", amplifyLevel(row2))
	}
	// 黄金书摇到 11：按服主要求与白银书同规则，落成 +11（不按官方清 0），
	// 再封装次数同样保留。
	row3 := make([]byte, 181)
	row3[amplifyReinforceOffset] = 0xA0 | 12 // 再封装 5 + 旧等级 12
	if got := applyGrimoireLevel(row3, 11); got != 11 {
		t.Errorf("黄金书摇到 11 应落成 +11，实际 +%d", got)
	}
	if row3[amplifyReinforceOffset]>>5 != 5 {
		t.Errorf("黄金书破坏了再封装次数：%#02x", row3[amplifyReinforceOffset])
	}
}
