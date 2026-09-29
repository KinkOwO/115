package inventory

import "testing"

// 掉落池准入的实验开关：`[trade]` / `[trade delete]` 与耐久免检部位。
//
// 为什么值得单测：`DFO_ALLOW_TRADE_EQUIPMENT` 是**实验开关**，默认必须与改动前逐字节一致
// （只认 `[free]` + 三个首饰部位）；打开后才放宽。两条都有断言，避免"默认就被改宽"。
func TestAcceptableAttachSwitch(t *testing.T) {
	cases := []struct {
		attach string
		off    bool
		on     bool
	}{
		{"[free]", true, true},
		{"[trade]", false, true},
		{"[trade delete]", false, true},
		{"[account]", false, false},
		{"", false, false},
		{"[unknown]", false, false},
	}
	for _, c := range cases {
		t.Setenv("DFO_ALLOW_TRADE_EQUIPMENT", "0")
		if got := acceptableAttach(c.attach); got != c.off {
			t.Errorf("开关关：acceptableAttach(%q)=%v，期望 %v", c.attach, got, c.off)
		}
		t.Setenv("DFO_ALLOW_TRADE_EQUIPMENT", "1")
		if got := acceptableAttach(c.attach); got != c.on {
			t.Errorf("开关开：acceptableAttach(%q)=%v，期望 %v", c.attach, got, c.on)
		}
	}
}

// 耐久免检部位：开关关时只认原来的三个；开时与发放路径（`durabilityOptional`，17 个部位）对齐 ——
// 深渊装备的部位（`[support]` / `[earring]` / `[magic stone]` / `[primer]`）正是靠这条进池。
func TestPoolJewelryForSwitch(t *testing.T) {
	t.Setenv("DFO_ALLOW_TRADE_EQUIPMENT", "0")
	if !poolJewelryFor("[amulet]") {
		t.Error("开关关：[amulet] 本来就在池子里")
	}
	for _, kind := range []string{"[earring]", "[support]", "[magic stone]", "[primer]"} {
		if poolJewelryFor(kind) {
			t.Errorf("开关关：%s 不应进池（保持改动前行为）", kind)
		}
	}

	t.Setenv("DFO_ALLOW_TRADE_EQUIPMENT", "1")
	for _, kind := range []string{"[amulet]", "[earring]", "[support]", "[magic stone]", "[primer]"} {
		if !poolJewelryFor(kind) {
			t.Errorf("开关开：%s 应与 durabilityOptional 对齐后进池", kind)
		}
	}
	if poolJewelryFor("[coat]") {
		t.Error("开关开：[coat] 有耐久段，不该走免检表")
	}
}
