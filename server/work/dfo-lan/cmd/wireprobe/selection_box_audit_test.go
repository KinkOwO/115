package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"sort"
	"testing"
)

// 诊断：自选盒（[booster select category]）在实机发放时走的是
// openBoosterItem → Destination 3 → Bag.AddEquipment → EquipmentCatalog.Reward(id)。
// Reward 失败或装备栏满都会让上层回 Refusal(4)，客户端把它显示成「库存已满」。
// 这条用例用**运行时同一套目录**（equipment.current37 + equipment-full）把每个自选盒
// 的每一件都过一遍，把"服务端根本发不出来"的盒子挑出来——这正是"右键自选盒报库存已满"
// 的可疑根因之一。
func TestSelectionBoxItemsAreGrantable(t *testing.T) {
	boxes, err := catalog.LoadSelectionBoxes("../../configs/selection-boxes-candidate.json")
	if err != nil {
		t.Fatal(err)
	}
	full, err := inventory.OpenFullEquipmentCatalog("../../configs/equipment-full", boxes.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	gear, err := inventory.LoadEquipmentCatalog("../../configs/equipment.current37.json", boxes.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	gear.Full = full

	reasons := map[string]int{}
	badTemplates := map[uint32]string{}
	boxesWithBad := 0
	checked := 0
	for _, box := range boxes.Boxes {
		boxBad := 0
		for _, cat := range box.Categories {
			for _, it := range cat.Items {
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
	t.Logf("检查 %d 件；无法发放的盒子 %d 个；无法发放的模板 %d 个", checked, boxesWithBad, len(badTemplates))
	for _, k := range keys {
		t.Logf("  原因 ×%d: %s", reasons[k], k)
	}
	sample := make([]uint32, 0, 12)
	for id := range badTemplates {
		sample = append(sample, id)
	}
	sort.Slice(sample, func(i, j int) bool { return sample[i] < sample[j] })
	for i, id := range sample {
		if i >= 10 {
			break
		}
		t.Logf("  样例 %d: %s", id, badTemplates[id])
	}
}
