package main

import (
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

// 诊断：自选盒（[booster select category]）里**走装备发放路径**的物品，在实机发放时
// 走的是 openBoosterItem → Destination 3 → Bag.AddEquipment → EquipmentCatalog.Reward(id)。
// Reward 失败或装备栏满都会让上层回 Refusal(4)，客户端把它显示成「库存已满」。
//
// 只查 equipment/ 下的模板：装扮（avatar/）与宠物走 Destination 1/2，用各自的目录与
// 行结构，拿装备目录去查它们只会得到假阳性（第一版就是这么误报 1438 件的）。
func TestSelectionBoxItemsAreGrantable(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native historical selection equipment audit")
	}
	c, err := gamedata.PrepareCatalogs(gamedata.CatalogInputs{Selection: "items,equipment,selection-boxes", ArchivePath: path, ArchiveChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256"), DerivedCacheDir: "-"}, gamedata.CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Equipment.Close()
	boxes := c.SelectionBoxes
	gear := &inventory.EquipmentCatalog{Full: c.Equipment}
	index := c.Items
	data, err := os.ReadFile("../../internal/catalog/testdata/selection-historical-templates.json")
	if err != nil {
		t.Fatal(err)
	}
	var historical []uint32
	if err := json.Unmarshal(data, &historical); err != nil {
		t.Fatal(err)
	}

	reasons := map[string]int{}
	badTemplates := map[uint32]string{}
	boxesWithBad := 0
	checked := 0
	for _, id := range historical {
		box, ok := boxes.ByTemplate(id)
		if !ok {
			t.Fatalf("historical box %d disappeared", id)
		}
		boxBad := 0
		for _, cat := range box.Categories {
			for _, it := range cat.Items {
				path := index.Items[it.Template].Path
				// 只有 equipment/ 下、且不是装扮的物品才走 Destination 3 的装备发放；装扮
				// （avatar/ 与 at_avatar/ 两种命名）与宠物各有自己的目的地，拿装备目录去查
				// 它们只会得到假阳性。注意 booster_flow 里判装扮用的是 "/avatar/"，漏了
				// at_avatar —— 这里用宽松的 "avatar" 以免再误报。
				if !strings.Contains(path, "equipment/") || strings.Contains(path, "avatar") {
					continue
				}
				checked++
				if _, err := gear.Reward(it.Template); err != nil {
					reason := err.Error()
					reasons[reason]++
					badTemplates[it.Template] = reason
					boxBad++
				}
			}
		}
		if boxBad > 0 {
			boxesWithBad++
		}
	}
	keys := make([]string, 0, len(reasons))
	for k := range reasons {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return reasons[keys[i]] > reasons[keys[j]] })
	for _, k := range keys {
		t.Logf("  原因 ×%d: %s", reasons[k], k)
	}
	sample := make([]uint32, 0, 12)
	for id := range badTemplates {
		sample = append(sample, id)
	}
	sort.Slice(sample, func(i, j int) bool { return sample[i] < sample[j] })
	for i, id := range sample {
		if i >= 8 {
			break
		}
		def, derr := gear.Definition(id)
		if derr != nil {
			t.Logf("  样例 %d: definition 查不到（%v）", id, derr)
			continue
		}
		kind := ""
		if f := def.Fields["[equipment type]"]; len(f) > 0 {
			kind = f[0].Text
		}
		t.Logf("  样例 %d %s | 部位=%q | rarity %d 值 | durability %d 值 | 原因: %s",
			id, def.Path, kind, len(def.Fields["[rarity]"]), len(def.Fields["[durability]"]), badTemplates[id])
	}
	// 装备路径上的每一件都必须发得出去：Reward 跟随 [import script] 后这里应为 0。
	// 曾经是 78 个盒子 / 543 件（其中 500 余件是薄壳装备：自身只有 [import script]，
	// 不带 [rarity]/[equipment type]/[durability]）。
	if boxesWithBad > 0 || len(badTemplates) > 0 {
		t.Fatalf("自选盒装备路径仍有发不出去的物品：盒子 %d 个、模板 %d 个（共检查 %d 件）",
			boxesWithBad, len(badTemplates), checked)
	}
	t.Logf("装备路径 %d 件全部可发放", checked)
}
