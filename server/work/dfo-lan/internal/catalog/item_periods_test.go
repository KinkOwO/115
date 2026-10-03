package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// 历史 item-period 导出（configs/item-period-tags.json，源 7ef2）与当前源的对照测试
// 已删除：该 JSON 是历史基线，服务端运行期改由原生 ImportItemPeriods/ItemBasics 提供，
// 历史基线仅审计。原生到期模板分类由 item_basics_test.go 的联合导入对照覆盖。

func TestItemPeriodCatalogRejectsRepeatedTemplates(t *testing.T) {
	const source = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	data := ItemPeriodCatalog{Schema: ItemPeriodCatalogSchema, Templates: []uint32{12, 12}}
	data.Source.Checksum = source
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "periods.json")
	if err := os.WriteFile(file, b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadItemPeriods(file, source); err == nil {
		t.Fatal("duplicate template accepted")
	}
}
