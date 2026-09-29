package catalog

import (
	"encoding/json"
	"fmt"
	"os"
)

// ItemMaterials captures an item's own `[need material]` cost.
//
// 商店表 itemshop/**.shp **没有价格字段**：商店里「用材料交换」的商品，材料成本写在
// **物品脚本**的 `[need material] <模板> <数量>`（可多组）。服务端原来只认 `.shp` 里的
// [need material]，于是这些商品落进「金币价」分支，而物品又没写 [price] → 直接拒绝，
// 客户端就弹「仓库已满」（真实原因 missing or overflowing source purchase price）。
//
// 规则数据来自 configs/item-materials.json（scripts/export_item_materials.py 只读 PVF 导出）。
type ItemMaterialCost struct {
	Template uint32 `json:"template"`
	Count    uint32 `json:"count"`
}

type itemMaterialEntry struct {
	Template  uint32             `json:"template"`
	Path      string             `json:"path"`
	Materials []ItemMaterialCost `json:"materials"`
}

type itemMaterialsDoc struct {
	Version int                 `json:"version"`
	Source  string              `json:"source"`
	Items   []itemMaterialEntry `json:"items"`
}

// ItemMaterials is the exported catalog, queryable by template.
type ItemMaterials struct {
	byTemplate map[uint32][]ItemMaterialCost
}

// LoadItemMaterials reads the item [need material] catalog. A missing file yields
// a nil catalog (the shop falls back to gold pricing).
func LoadItemMaterials(path string) (*ItemMaterials, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc itemMaterialsDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if doc.Version != 1 || len(doc.Items) == 0 {
		return nil, fmt.Errorf("物品材料规则源定义不完整")
	}
	m := &ItemMaterials{byTemplate: make(map[uint32][]ItemMaterialCost, len(doc.Items))}
	for _, it := range doc.Items {
		if it.Template == 0 || len(it.Materials) == 0 {
			continue
		}
		m.byTemplate[it.Template] = it.Materials
	}
	return m, nil
}

// Materials reports the item's own [need material] cost, if any.
func (m *ItemMaterials) Materials(template uint32) ([]ItemMaterialCost, bool) {
	if m == nil {
		return nil, false
	}
	costs, ok := m.byTemplate[template]
	return costs, ok
}
