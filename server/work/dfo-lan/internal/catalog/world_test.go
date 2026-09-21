package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorldAreaLevelGatesFromPermissionBlock(t *testing.T) {
	// 实源 stormpass.map（43/0）的 [permission] 块同时给出 [need level] 50 与
	// [odyssey enter level] 45。客户端在奥德赛模式下用后者判定（拒绝提示
	// DSTR 535 填的是 45），解析必须同时保留两个值，而不是只留 [need level]。
	tokens := []pvf.Token{
		{Type: 3, Text: "[area]"},
		{Type: 0, Value: 0},
		{Type: 6, Text: "map/cataclysm/town/stormpass/stormpass.map"},
		{Type: 3, Text: "[permission]"},
		{Type: 3, Text: "[need level]"},
		{Type: 0, Value: 50},
		{Type: 3, Text: "[odyssey enter level]"},
		{Type: 0, Value: 45},
		{Type: 3, Text: "[/permission]"},
		{Type: 6, Text: "[normal]"},
		{Type: 3, Text: "[/area]"},
	}
	areas, err := parseWorldAreas(43, tokens)
	if err != nil {
		t.Fatal(err)
	}
	if len(areas) != 1 {
		t.Fatalf("expected 1 area, got %d", len(areas))
	}
	if areas[0].MinimumLevel != 50 || areas[0].OdysseyMinimumLevel != 45 {
		t.Fatalf("gates: need %d odyssey %d", areas[0].MinimumLevel, areas[0].OdysseyMinimumLevel)
	}
	if len(areas[0].Pending) != 0 {
		t.Fatalf("unexpected pending: %v", areas[0].Pending)
	}
}

func TestWorldAreaOdysseyGateMustBeNumeric(t *testing.T) {
	// 条件式或非数值的 [odyssey enter level] 不得被当成一个猜出来的数字门槛：
	// 紧跟进条件子块（type 3 标签）时按"源里没有该门槛"处理，段落里出现
	// 非数值单元（type 6）时记为未解析。
	conditional := []pvf.Token{
		{Type: 3, Text: "[area]"},
		{Type: 0, Value: 0},
		{Type: 6, Text: "map/x.map"},
		{Type: 3, Text: "[permission]"},
		{Type: 3, Text: "[need level]"},
		{Type: 0, Value: 50},
		{Type: 3, Text: "[odyssey enter level]"},
		{Type: 3, Text: "[check condition]"},
		{Type: 0, Value: 12},
		{Type: 3, Text: "[/check condition]"},
		{Type: 0, Value: 45},
		{Type: 3, Text: "[/permission]"},
		{Type: 3, Text: "[/area]"},
	}
	areas, err := parseWorldAreas(43, conditional)
	if err != nil {
		t.Fatal(err)
	}
	if len(areas) != 1 || areas[0].MinimumLevel != 50 || areas[0].OdysseyMinimumLevel != 0 {
		t.Fatalf("conditional odyssey gate accepted: %+v", areas)
	}

	nonNumeric := []pvf.Token{
		{Type: 3, Text: "[area]"},
		{Type: 0, Value: 0},
		{Type: 6, Text: "map/x.map"},
		{Type: 3, Text: "[permission]"},
		{Type: 3, Text: "[need level]"},
		{Type: 0, Value: 50},
		{Type: 3, Text: "[odyssey enter level]"},
		{Type: 6, Text: "[normal]"},
		{Type: 3, Text: "[/permission]"},
		{Type: 3, Text: "[/area]"},
	}
	areas, err = parseWorldAreas(43, nonNumeric)
	if err != nil {
		t.Fatal(err)
	}
	if len(areas) != 1 || areas[0].OdysseyMinimumLevel != 0 {
		t.Fatalf("non-numeric odyssey gate accepted: %+v", areas)
	}
	found := false
	for _, pending := range areas[0].Pending {
		if pending == "conditional odyssey level rule requires interpretation" {
			found = true
		}
	}
	if !found {
		t.Fatalf("pending: %v", areas[0].Pending)
	}
}

func TestLoadWorldBackfillsOdysseyGate(t *testing.T) {
	// 升级前导出的 world 目录没有 odyssey_minimum_level 字段，但保留了完整
	// area definition；加载时按需回填，避免为了一个新增数据块重导 31 MB 配置。
	catalog := map[string]any{
		"source": map[string]any{"checksum": strings.Repeat("a", 64)},
		"areas": map[string]any{
			"43/1": map[string]any{
				"town": 43, "area": 1, "map_path": "map/x.map", "minimum_level": 50,
				"definition": []pvf.Token{
					{Type: 3, Text: "[permission]"},
					{Type: 3, Text: "[need level]"}, {Type: 0, Value: 50},
					{Type: 3, Text: "[odyssey enter level]"}, {Type: 0, Value: 45},
					{Type: 3, Text: "[/permission]"},
				},
			},
			"43/2": map[string]any{
				"town": 43, "area": 2, "map_path": "map/y.map", "minimum_level": 50,
				"definition": []pvf.Token{
					{Type: 3, Text: "[permission]"},
					{Type: 3, Text: "[need level]"}, {Type: 0, Value: 50},
					{Type: 3, Text: "[/permission]"},
				},
			},
		},
	}
	raw, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "world.json")
	if err = os.WriteFile(file, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	w, err := LoadWorld(file)
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Areas["43/1"].OdysseyMinimumLevel; got != 45 {
		t.Fatalf("backfilled gate %d", got)
	}
	if got := w.Areas["43/2"].OdysseyMinimumLevel; got != 0 {
		t.Fatalf("area without source gate gained %d", got)
	}
}
