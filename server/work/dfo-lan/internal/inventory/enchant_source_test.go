package inventory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// 真实数据用例：侧车 full 目录是否覆盖玩家实际穿戴的高阶装备。
//
// applyEnchantByBead 对目标装备有一道硬门 ——
//
//	d, err := s.Catalog.Definition(gear.Template)
//	if _, ok := d.Fields["[equipment type]"]; !ok { 拒绝 }
//
// 而 EquipmentCatalog 的常规索引（configs/equipment.current37.json，4.5 MB）
// 只收录了几千行、没有 100 级装备；这些模板只在侧车目录 equipment-full
// （536 MB 索引 + 326 MB 数据）里。Catalog.Definition 只有在 Full 非 nil 时
// 才会去查侧车目录，所以「侧车是否装载」直接决定附魔能不能成 ——
// 而装载路径是 channel_probe.py 通过 DFO_EQUIPMENT_FULL_CATALOG 传进来的，
// 一旦那条线断了，附魔会以「目标不是装备」被拒，其它功能却看不出异常。
//
// 这条用例把那只依赖钉住：模板清单取自 test-xl（角色 11）2026-09-28 的实际存档。
func TestFullEquipmentCatalogCoversWornTemplates(t *testing.T) {
	prefix := "../inventory/testdata/equipment-flow"
	if _, err := os.Stat(prefix + ".index.json"); err != nil {
		t.Fatal("equipment test fixture missing", err)
	}
	raw, err := os.ReadFile(prefix + ".index.json")
	if err != nil {
		t.Fatal(err)
	}
	var head struct {
		Source struct {
			Checksum string `json:"Checksum"`
		} `json:"Source"`
	}
	if err = json.Unmarshal(raw, &head); err != nil {
		t.Fatal(err)
	}
	if head.Source.Checksum == "" {
		t.Fatal("侧车索引没有 Source.Checksum，无法打开")
	}
	full, err := OpenFullEquipmentCatalog(prefix, head.Source.Checksum)
	if err != nil {
		t.Fatalf("打开侧车 full 失败: %v", err)
	}
	defer full.Close()

	// test-xl 穿戴的模板（去重）。
	worn := []uint32{
		101001153, 100301875, 100313578, 100323468, 100401640, 100401592,
		100401608, 100401641, 100331747, 100401618, 100101193, 100401598,
		100261099, 100051288,
	}
	for _, tmpl := range worn {
		d, e := full.Definition(tmpl)
		if e != nil {
			t.Errorf("Definition(%d) 失败: %v —— 该模板不在侧车目录里，附魔会被判「目标不是装备」", tmpl, e)
			continue
		}
		if _, ok := d.Fields["[equipment type]"]; !ok {
			t.Errorf("Definition(%d) 没有 [equipment type] 段，附魔会被判「目标不是装备」", tmpl)
		}
	}
}

// 附魔宝珠的识别链：宝珠模板 → 附魔卡（怪物卡）。抽两张 test-xl 手上真有
// 的宝珠（GM 发放用）钉住，它们必须落在配置表里，否则服务端会以
// 「宝珠槽位里不是附魔宝珠」拒绝 —— 而那是玩家看不出区别的失败。
func TestEnchantBeadsCoverGrantedTemplates(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "enchant-beads.json")
	if _, err := os.Stat(path); err != nil {
		t.Skip("宝珠表不在默认位置，跳过")
	}
	if err := LoadEnchantBeads(path); err != nil {
		t.Fatalf("装载宝珠表失败: %v", err)
	}
	// 2026-09-28 用 admin 发到 test-xl 背包 67/68/69 的三颗。
	granted := map[uint32]uint32{2600294: 3600, 2600295: 3601, 2600296: 3602}
	for bead, want := range granted {
		got, ok := EnchantCardForBead(bead)
		if !ok {
			t.Errorf("宝珠 %d 不在配置表里，附魔会被拒", bead)
			continue
		}
		if got != want {
			t.Errorf("宝珠 %d 的附魔卡 = %d，期望 %d", bead, got, want)
		}
		if !IsEnchantBead(bead) {
			t.Errorf("IsEnchantBead(%d) = false", bead)
		}
	}
}
