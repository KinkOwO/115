// mazechanceimport 生成「按权重掷骰选图」的白名单 overlay：
// 从副本源里读出每张 maze 声明的 [maze chance rate]，写进
// configs/dungeons.maze-chance-rates.json。
//
// 为什么不去读 PVF：dungeons.full.json 是 cmd/dungeonfull 对源的忠实转储，
// **连原始 cells 都在**，所以这里直接扫源真值即可 —— 省掉一次 400–700 MB 的
// PVF 装载（客户端在跑时必然 OOM），而且保证 overlay 与运行时读的是同一份数据。
//
// 用法：
//
//	mazechanceimport                                # 白名单只列小深渊，权重用官方原值
//	mazechanceimport -dungeons 100005014 -maze 1=2% # 把小深渊 maze 1 的概率改为 2%
package main

import (
	"dfolan/internal/catalog"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
)

// weightBasis 是服务端内部使用的权重基数。源表本身的量纲并不统一（实测同时
// 存在合计 100 / 1e3 / 1e4 / 2e5 / 1e6 的写法），但覆盖值统一按百万分制表达。
const weightBasis = 1000000

type fullCatalog struct {
	Source struct {
		Checksum string `json:"checksum"`
	} `json:"source"`
	Dungeons map[string]struct {
		Script catalog.ScriptRecord `json:"script"`
		Mazes  []struct {
			Index byte `json:"index"`
		} `json:"mazes"`
	} `json:"dungeons"`
}

func main() {
	full := flag.String("full", "configs/dungeons.full.json", "source dungeon catalog to read [maze chance rate] from")
	out := flag.String("out", "configs/dungeons.maze-chance-rates.json", "whitelist overlay to write")
	ids := flag.String("dungeons", "100005014", "comma separated dungeon ids to whitelist")
	overrides := flag.String("maze", "", "per-maze probability override, e.g. 1=2%; indices not listed keep their source ratio")
	flag.Parse()

	raw, err := os.ReadFile(*full)
	if err != nil {
		log.Fatal(err)
	}
	var source fullCatalog
	if err := json.Unmarshal(raw, &source); err != nil {
		log.Fatal(err)
	}
	if source.Source.Checksum == "" {
		log.Fatal("source catalog has no checksum")
	}
	spec, err := parseOverrides(*overrides)
	if err != nil {
		log.Fatal(err)
	}

	overlay := catalog.MazeChanceOverlay{SourceChecksum: source.Source.Checksum}
	for _, idText := range strings.Split(*ids, ",") {
		idText = strings.TrimSpace(idText)
		if idText == "" {
			continue
		}
		id, err := strconv.ParseUint(idText, 10, 32)
		if err != nil {
			log.Fatalf("bad dungeon id %q: %v", idText, err)
		}
		entry, ok := source.Dungeons[idText]
		if !ok {
			log.Fatalf("dungeon %d is absent from %s", id, *full)
		}
		rates, ok := catalog.ReadMazeChanceRates(entry.Script)
		if !ok {
			log.Fatalf("dungeon %d does not declare [maze chance rate] on every maze", id)
		}
		if len(rates) != len(entry.Mazes) {
			log.Fatalf("dungeon %d: read %d rates for %d mazes", id, len(rates), len(entry.Mazes))
		}
		final, err := applyOverrides(rates, spec)
		if err != nil {
			log.Fatalf("dungeon %d: %v", id, err)
		}
		var total uint64
		for _, w := range final {
			total += uint64(w)
		}
		log.Printf("dungeon %d: source %v -> server %v (percent %s)", id, rates, final, percentList(final, total))
		overlay.Dungeons = append(overlay.Dungeons, catalog.MazeChanceDungeon{
			ID:            uint32(id),
			DungeonSHA256: entry.Script.SHA256,
			SourceRates:   rates,
			Rates:         final,
		})
	}
	if len(overlay.Dungeons) == 0 {
		log.Fatal("no dungeon selected")
	}
	// 与既有 overlay 一致：紧凑 JSON，无缩进无尾换行。
	b, err := json.Marshal(overlay)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, b, 0644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s (%d dungeon(s))", *out, len(overlay.Dungeons))
}

func percentList(weights []uint32, total uint64) string {
	if total == 0 {
		return "(all zero)"
	}
	parts := make([]string, 0, len(weights))
	for _, w := range weights {
		parts = append(parts, fmt.Sprintf("%.4f%%", 100*float64(w)/float64(total)))
	}
	return strings.Join(parts, " / ")
}

// parseOverrides 解析 -maze 的 `index=percent` 列表（可逗号分隔成多项）。
func parseOverrides(spec string) (map[int]float64, error) {
	out := map[int]float64{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		indexText, percentText, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("bad -maze %q: want <index>=<percent>", part)
		}
		index, err := strconv.Atoi(strings.TrimSpace(indexText))
		if err != nil || index < 0 {
			return nil, fmt.Errorf("bad -maze index %q", indexText)
		}
		percent, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(percentText), "%"), 64)
		if err != nil || percent <= 0 {
			return nil, fmt.Errorf("bad -maze percent %q", percentText)
		}
		out[index] = percent
	}
	return out, nil
}

// applyOverrides 把指定的 maze 拉到给定概率，其余按**源比例**分剩下的份额。
//
// 「按源比例」是刻意的：它让被改写的那张图单独变化，而其余各张之间的相对关系
// 一点不动 —— 和掉落调参层里「按现比例补给高档」是同一个原则。
//
// 取整用整数截断，误差补给最后一个非覆盖项，保证合计恰好是 weightBasis。
func applyOverrides(source []uint32, spec map[int]float64) ([]uint32, error) {
	if len(spec) == 0 {
		return append([]uint32(nil), source...), nil
	}
	out := make([]uint32, len(source))
	var fixed uint32
	for index, percent := range spec {
		if index >= len(source) {
			return nil, fmt.Errorf("maze %d does not exist (source has %d)", index, len(source))
		}
		w := uint32(math.Round(percent / 100 * weightBasis))
		if w == 0 {
			return nil, fmt.Errorf("maze %d override rounds to 0", index)
		}
		out[index] = w
		fixed += w
	}
	if fixed >= weightBasis {
		return nil, fmt.Errorf("overrides take %d of %d", fixed, weightBasis)
	}
	rest := weightBasis - fixed
	var sourceSum uint32
	for i := range source {
		if _, overridden := spec[i]; !overridden {
			sourceSum += source[i]
		}
	}
	if sourceSum == 0 {
		return nil, fmt.Errorf("no source weight left to redistribute")
	}
	var assigned uint32
	last := -1
	for i := range source {
		if _, overridden := spec[i]; overridden {
			continue
		}
		last = i
		w := uint32(uint64(rest) * uint64(source[i]) / uint64(sourceSum))
		out[i] = w
		assigned += w
	}
	if last < 0 {
		return nil, fmt.Errorf("nothing left to carry the rounding remainder")
	}
	out[last] += rest - assigned
	var total uint64
	for _, w := range out {
		total += uint64(w)
	}
	if total != weightBasis {
		return nil, fmt.Errorf("weights add up to %d, want %d", total, weightBasis)
	}
	return out, nil
}
