package inventory

import (
	"path/filepath"
	"testing"
)

func loadEnchantBeadsForTest(t *testing.T) {
	t.Helper()
	if EnchantBeadsLoaded() {
		return
	}
	path := filepath.Join("..", "..", "configs", "enchant-beads.json")
	if err := LoadEnchantBeads(path); err != nil {
		t.Fatalf("装载附魔宝珠清单失败: %v", err)
	}
	if !EnchantBeadsLoaded() {
		t.Skip("configs/enchant-beads.json 不存在，跳过附魔用例")
	}
}

// 宝珠 → 附魔卡 的对照（数据来自 scripts/export_enchant_beads.py 的 PVF 导出）。
func TestEnchantBeadMapping(t *testing.T) {
	loadEnchantBeadsForTest(t)
	// bead_sieghart.stk → mcard_sieghart.stk
	if card, ok := EnchantCardForBead(2600294); !ok || card != 3600 {
		t.Errorf("宝珠 2600294 应映射到卡 3600，实际 ok=%v card=%d", ok, card)
	}
	if !IsEnchantBead(2600294) {
		t.Error("2600294 应识别为附魔宝珠")
	}
	// 非宝珠。
	if IsEnchantBead(10356325) {
		t.Error("白银增幅书 10356325 不应是附魔宝珠")
	}
	if _, ok := EnchantCardForBead(999999); ok {
		t.Error("未知模板不应映射到附魔卡")
	}
}

// 装备行 offset 14（u32 小端）读写附魔卡，且不碰相邻字节。
func TestEnchantCardRowAccessors(t *testing.T) {
	row := make([]byte, 181)
	if enchantCard(row) != 0 {
		t.Error("空行的附魔卡应为 0")
	}
	setEnchantCard(row, 3600)
	if got := enchantCard(row); got != 3600 {
		t.Errorf("读回附魔卡 = %d，期望 3600", got)
	}
	if row[13] != 0 || row[18] != 0 {
		t.Errorf("写 offset 14 污染了相邻字节: % x", row[13:19])
	}
	// 覆盖旧附魔。
	setEnchantCard(row, 10015138)
	if got := enchantCard(row); got != 10015138 {
		t.Errorf("覆盖后附魔卡 = %d，期望 10015138", got)
	}
}
