// flippool 从**内层 PVF** 现场枚举装备，生成末世录终局翻牌专用的装备池。
//
// 业主口径（2026-10-08）：「所有发放装备的格子，改为随机发放装备，
// 魔法\神器\史诗，概率为 15%\35%\50%」，并选定**新建一个装备池子**——
// 因为维纳斯的 `venus-flip-gear.generated.json` 是 `rarity=[2,3]`
// （魔法/神器）**没有 SS(史诗)**，凑不出三档。
//
// 技术路线与维纳斯池当年的脚本 `tmp_enum_gear` 同源：
//
//	list/equipment.lst（PVF 全量装备清单，ID → .equ 路径）
//	  → 逐条解析 [rarity] / [minimum level] / [equipment type] / [attach type]
//	  → 按 rarity 2/3/4 分档，只保留指定等级、可发放的常规部位
//	  → 写成 ApocalypseFlipGearPool 形状的 JSON
//
// 用法（在 server/work/dfo-lan 下）：
//
//	go run ./cmd/dfo-tool flippool \
//	  -source runtime/pvf_source/Script.inner.pvf \
//	  -out configs/apocalypse-flip-gear.generated.json \
//	  -level 115
//
// 只读 PVF，不写任何游戏数据。
package flippool

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/gamedata"
)

// 三档品级与权重（业主口径 15/35/50）。
//
// 序号与 inventory.equipmentRarityNames 同序：
//
//	2 = rare（魔法）  3 = unique（神器）  4 = epic（史诗）
var tiers = []struct {
	rarity int
	label  string
	weight int
}{
	{2, "魔法", 15},
	{3, "神器", 35},
	{4, "史诗", 50},
}

// 可发放的常规部位：与 `inventory.equipment.go` 的掉落池口径一致
// （有耐久，或首饰三类）。
var jewelry = map[string]bool{"[ring]": true, "[bracelet]": true, "[necklace]": true}

type poolFile struct {
	Source struct {
		Format   string `json:"format"`
		Checksum string `json:"checksum"`
		Origin   string `json:"origin"`
	} `json:"source"`
	Level int `json:"level"`
	Tiers []struct {
		Rarity    int      `json:"rarity"`
		Label     string   `json:"label"`
		Weight    int      `json:"weight"`
		Templates []uint32 `json:"templates"`
	} `json:"tiers"`
}

// fieldValue 取某个字段的第一个数值。
func fieldValue(cells []pvf.Token, name string) (int64, bool) {
	for i, c := range cells {
		if c.Type == 3 && c.Text == name {
			if i+1 < len(cells) && cells[i+1].Type == 0 {
				return int64(cells[i+1].Value), true
			}
			return 0, false
		}
	}
	return 0, false
}

// fieldText 取某个字段的第一个文本（如 `[equipment type]` 的 `[coat]`）。
func fieldText(cells []pvf.Token, name string) (string, bool) {
	for i, c := range cells {
		if c.Type == 3 && c.Text == name {
			if i+1 < len(cells) && cells[i+1].Type == 6 {
				return cells[i+1].Text, true
			}
			return "", false
		}
	}
	return "", false
}

func equipmentPaths(source *gamedata.Source) (map[uint32]string, error) {
	list, err := source.Script("list/equipment.lst")
	if err != nil {
		return nil, err
	}
	rows, err := catalog.ParseIndex(list.Cells)
	if err != nil {
		return nil, err
	}
	paths := make(map[uint32]string, len(rows))
	for _, row := range rows {
		paths[row.ID] = row.Path
	}
	return paths, nil
}

func Run() {
	sourcePath := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only PVF source archive")
	outPath := flag.String("out", "configs/apocalypse-flip-gear.generated.json", "pool output path")
	level := flag.Int("level", 115, "only keep equipment whose [minimum level] equals this")
	maxPerTier := flag.Int("max-per-tier", 0, "cap templates per tier (0 = no cap)")
	flag.Parse()

	source, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: *sourcePath, MaxBytes: gamedata.DefaultMaxBytes})
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()

	paths, err := equipmentPaths(source)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("flippool: %d equipment entries in list/equipment.lst", len(paths))

	ids := make([]uint32, 0, len(paths))
	for id := range paths {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	buckets := map[int][]uint32{}
	var scanned, skipped, resolved int
	for _, id := range ids {
		script, err := source.ResolveScript(paths[id])
		if err != nil {
			skipped++
			continue
		}
		resolved++
		rarity, ok := fieldValue(script.Cells, "[rarity]")
		if !ok {
			continue
		}
		wanted := false
		for _, t := range tiers {
			if int(rarity) == t.rarity {
				wanted = true
			}
		}
		if !wanted {
			continue
		}
		minLevel, ok := fieldValue(script.Cells, "[minimum level]")
		if !ok || int(minLevel) != *level {
			continue
		}
		// 可发放性：`[free]` 或 `[trade]`（交易属性由 DFO_ALLOW_TRADE_EQUIPMENT 放行，
		// 与掉落池同口径），且要有耐久或是首饰。
		if attach, ok := fieldText(script.Cells, "[attach type]"); !ok ||
			(attach != "[free]" && attach != "[trade]" && attach != "[trade delete]") {
			continue
		}
		kind, _ := fieldText(script.Cells, "[equipment type]")
		if _, hasDur := fieldValue(script.Cells, "[durability]"); !hasDur && !jewelry[kind] {
			continue
		}
		scanned++
		buckets[int(rarity)] = append(buckets[int(rarity)], id)
	}
	log.Printf("flippool: resolved=%d skipped=%d selected=%d", resolved, skipped, scanned)

	var out poolFile
	out.Source.Format = string(source.Snapshot().Format)
	out.Source.Checksum = source.Snapshot().Checksum
	out.Source.Origin = fmt.Sprintf("list/equipment.lst scan (minimum level %d, rarity 2/3/4, wearable kinds)", *level)
	out.Level = *level
	total := 0
	for _, t := range tiers {
		list := buckets[t.rarity]
		if *maxPerTier > 0 && len(list) > *maxPerTier {
			list = list[:*maxPerTier]
		}
		out.Tiers = append(out.Tiers, struct {
			Rarity    int      `json:"rarity"`
			Label     string   `json:"label"`
			Weight    int      `json:"weight"`
			Templates []uint32 `json:"templates"`
		}{Rarity: t.rarity, Label: t.label, Weight: t.weight, Templates: list})
		total += len(list)
		log.Printf("  %s(rarity %d) weight=%d%% templates=%d", t.label, t.rarity, t.weight, len(list))
	}
	if total == 0 {
		log.Fatalf("flippool: no templates matched (level %d) — 检查 -level 与源", *level)
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(*outPath, raw, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("flippool: wrote %d templates to %s", total, *outPath)
}
