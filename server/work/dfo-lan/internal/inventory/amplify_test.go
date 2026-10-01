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
	// 纯净增幅书（随机表恒为 [(0,100)]）：应识别为 Pure，且不能走加权摇值（值 0 会被当成无红字）。
	if !IsPureGrimoire(10406171) {
		t.Error("10406171 应被识别为纯净增幅书")
	}
	if IsPureGrimoire(10356325) {
		t.Error("白银书 10356325 不应是纯净增幅书")
	}
	if _, ok := AmplifyGrimoireRollValue(10406171); ok {
		t.Error("纯净增幅书的加权摇值应被绕过（值 0），不应返回可用数值")
	}
}

// ClassifyAmplifyBook：客户端只会对「增幅书」发 CMD205，所以「不在 433 加权表里」
// （脚本没有 [amplification random value] 段）的模板一律按**纯净增幅书**兜底，而不是拒绝。
// 回归：除 1286 外，玩家实测用过的其它纯书（590704000 等）也要能识别。
func TestClassifyAmplifyBookFallsBackToPure(t *testing.T) {
	loadGrimoiresForTest(t)

	// 普通 / 白银书：不是 golden / pure，value 落在 3..6。
	g, p, v := ClassifyAmplifyBook(10356325)
	if g || p {
		t.Errorf("白银书 10356325 不该是 golden/pure（g=%v p=%v）", g, p)
	}
	if v < 3 || v > 6 {
		t.Errorf("白银书摇出 %d，不在 3..6", v)
	}

	// 已登记的纯书（1286）与实测用过的其它纯书（590704000、10356261…）→ pure。
	for _, tpl := range []uint32{1286, 590704000, 10356261, 10354248, 10360807, 10000605} {
		if _, pure, _ := ClassifyAmplifyBook(tpl); !pure {
			t.Errorf("纯书 %d 应判为 pure", tpl)
		}
	}

	// 黄金书 → golden；★ 它**不是** pure（本服按摇值即等级处理，不并入纯净）。
	if golden, pure, _ := ClassifyAmplifyBook(50002538); !golden || pure {
		t.Error("50002538（golden）应判为 golden")
	}

	// ★ 兜底：完全不在清单、也没有增幅段的模板 → pure（不拒绝）。
	g, p, v = ClassifyAmplifyBook(999999)
	if !p || g {
		t.Errorf("未知模板 999999 应兜底判为 pure（g=%v p=%v v=%d）", g, p, v)
	}
	if v != 0 {
		t.Errorf("兜底 pure 时 value 应为 0，实际 %d", v)
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
		`{"template":70003,"path":"stackable/dfo/cash/2022/1108/black_friday/grimorie/super_amplification_grimoire.stk","random":[{"value":8,"weight":50}],"expires":false},` +
		`{"template":70004,"path":"stackable/dfo/cash/pure_amplification_scroll.stk","random":[{"value":0,"weight":100}],"expires":false}` +
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
	if !IsPureGrimoire(70004) {
		t.Error("随机表值恒 0 的 70004 应识别为纯净增幅书")
	}
	if IsPureGrimoire(70001) {
		t.Error("golden 书 70001 不应是纯净增幅书")
	}
	if IsGoldenGrimoire(3037) {
		t.Error("无色小晶块不应被当成增幅书")
	}
}

// 纯净增幅书为「纯净」行为：只落「增幅 +0」，**不写红字数值**（offset 20 = 0）。
// 服主要求「只需要增幅 +0，不需要力量 +6」——0 在客户端 = 没有次元属性数值，该行不显示。
func TestPureGrimoireWritesNoRedValue(t *testing.T) {
	row := make([]byte, 181)
	row[amplifyReinforceOffset] = 9 // 旧强化 +9
	red, lvl := amplifyGrimoireOutcome(row, 6, true)
	if red != 0 {
		t.Errorf("纯书红字数值应为 0（不显示红字），实际 %d", red)
	}
	if lvl != 0 {
		t.Errorf("纯书增幅等级应为 +0，实际 +%d", lvl)
	}
	if amplifyLevel(row) != 0 {
		t.Errorf("纯书等级字节应为 0，实际 %d", amplifyLevel(row))
	}
}

// 没有 [amplification random value] 段、却被客户端当增幅书发 CMD205 的纯书
// （如本服 1286），必须靠 pure_templates 显式登记才能被识别。
func TestPureTemplatesFromConfig(t *testing.T) {
	saved := amplifyGrimoires
	defer func() { amplifyGrimoires = saved }()

	const src = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	doc := `{"version":1,"source":"` + src + `","rule":"x",` +
		`"types":{"1":"体力","2":"精神","3":"力量","4":"智力"},` +
		`"pure_templates":[1286],` +
		`"grimoires":[` +
		`{"template":10356325,"path":"stackable/10356001/10356325.stk","random":[{"value":3,"weight":40}],"expires":true}` +
		`]}`
	path := filepath.Join(t.TempDir(), "amplify-grimoire.json")
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := LoadAmplifyGrimoires(path); err != nil {
		t.Fatal(err)
	}
	if !IsPureGrimoire(1286) {
		t.Error("pure_templates 里的 1286 应识别为纯净增幅书")
	}
	if IsPureGrimoire(10356325) {
		t.Error("普通增幅书 10356325 不应是纯书（它走随机摇值）")
	}
	// 不在清单、也不在 grimoires 里的模板一律不是增幅书。
	if IsPureGrimoire(999999) {
		t.Error("无关模板不应被当成纯书")
	}
}

// 增幅等级落在 offset 10 低五位（普通/白银/强烈书/黄金书摇值即等级）；
// 纯净增幅书改为「纯净」行为（等级锁 +0、红字数值清 0）。
// 否则客户端只显示「增幅 +0」却看不到等级变化，玩家以为「红字打上了、等级没动」。
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
	// 黄金增幅书：与纯净增幅书是**两个不同产物**，本服按服主要求走「摇值即等级」
	// （与白银/强烈书同一套规则）—— 摇到 11 就该落 +11，红字数值也是 11。
	row3 := make([]byte, 181)
	row3[amplifyReinforceOffset] = 0xA0 | 12 // 再封装 5 + 旧等级 12
	red, lvl := amplifyGrimoireOutcome(row3, 11, false)
	if red != 11 {
		t.Errorf("黄金书红字数值应等于摇出值 11，实际 %d", red)
	}
	if lvl != 11 {
		t.Errorf("黄金书增幅等级应为摇出值 +11，实际 +%d", lvl)
	}
	if amplifyLevel(row3) != 11 {
		t.Errorf("黄金书等级字节应为 11，实际 %d", amplifyLevel(row3))
	}
	if row3[amplifyReinforceOffset]>>5 != 5 {
		t.Errorf("黄金书破坏了再封装次数：%#02x", row3[amplifyReinforceOffset])
	}
	// 纯净增幅书：同一入参下必须锁 +0 且不写红字数值（与上面那条形成对照）。
	row4 := make([]byte, 181)
	row4[amplifyReinforceOffset] = 0xA0 | 12
	pred, plvl := amplifyGrimoireOutcome(row4, 11, true)
	if pred != 0 || plvl != 0 || amplifyLevel(row4) != 0 || row4[amplifyReinforceOffset]>>5 != 5 {
		t.Errorf("纯净书应为 +0/红字 0 且保留再封装：red=%d lvl=%d lvlbyte=%d repack=%#02x",
			pred, plvl, amplifyLevel(row4), row4[amplifyReinforceOffset])
	}
}
