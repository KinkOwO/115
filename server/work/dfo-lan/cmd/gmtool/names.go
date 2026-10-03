// 本文件解决两个"数据来源"问题，都只在离线构建时跑一次（-build-data）：
//
//  1. 物品显示名：客户端内层 PVF 里的文本表才是游戏真正显示的文本。
//     runtime/l10n/names.zh.json 是另一条流水线（work-cn/tools/rewrite_str.py）
//     的中间产物：它的物品名来自 90 版中文客户端词典 + 术语逐词对照 + 机器翻译，
//     而客户端真正加载的是内层 PVF 里的 string/*.uv.str 文本表。两者会不一致，例如
//     name_101001153：names.zh.json =「原始的星辰 - 短剑」，客户端 =「太初之星 - 短剑」。
//     所以这里从客户端内层 PVF 的文本表导出 names.client.json，作为 GM 工具的第一名字来源，
//     并且不覆盖 runtime/l10n/names.zh.json（那条流水线还在用）。
//
//  2. 装备部位/等级：items.index.json 只带 id/name/kind/grade/rarity，没有
//     [equipment type] 与 [minimum level]。这里把装备目录里这两列抽成一张紧凑的
//     id -> [部位cell, 最低等级] 表（equipment.slots.json），供筛选使用。
//
// 证据（2026-09 本机实测）：
//   - 客户端内层 PVF：D:\115us\work-cn\verify\clientpatch\Script.inner.pvf
//     （format=dfo_20260901_inner，5,650,173 文件 / 82,571 组）
//   - string/equipment.uv.str 里含 "name_101001153>太初之星 - 短剑"（游戏里显示的名字）
//   - string/ui.uv.str 里含 common_rarity_0..8 = 普通/高级/稀有/神器/史诗/勇者/传说/神话/太初
//   - 两份文本表合计覆盖 items.index.json 全部 386,230 件物品的 name_<id>，无遗漏
package main

import (
	"bufio"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/gamedata"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 客户端文本表按优先级排列：先出现的表优先，避免同名 key 被别的域覆盖。
//
// 依据：work-cn/tools/rewrite_str.py 的 NAME_DOMAINS（生成 names.zh.json 时用的同一批
// 物品文本域），但把真正存物品名的 equipment/stackable 放在最前面 —— 实测这两张表
// 就覆盖了 items.index.json 全部 386,230 件物品的 name_<id>。
//
// uv = 该客户端（DFO 115 内层 Script.inner.pvf）实际加载的那一套文本表；
// 汉化补丁（work-cn/tools/rewrite_str.py + build_patch.py）写回的也正是这些表。
var clientNameTables = []string{
	"string/equipment.uv.str",    // 装备名 name_<id>
	"string/stackable.uv.str",    // 堆叠物名 name_<id>
	"string/setequipment.uv.str", // 套装名
	"string/creature.uv.str",     // 宠物名
	"string/pet.uv.str",          // 宠物名（旧表）
	"string/skin.uv.str",         // 装扮名
	"string/cos.uv.str",          // 装扮名
	"string/itemshop.uv.str",     // 商城物品名
	"string/etc.uv.str",          // 杂项名
	"string/ui.uv.str",           // 品级名 common_rarity_<n>
	"string/character.uv.str",    // 职业名 growtype_name_<n>
}

// clientNamePrefixes 是 names.client.json 里保留的 key 前缀。
//
// 与 index.go 的 loadNames 一致：GM 工具只需要物品名、职业名、品级名三类，
// 其余（技能/任务/界面文本）在导出时就丢掉，成品文件从 66MB 降到 ~18MB。
var clientNamePrefixes = []string{namePrefix, growNamePrefix, rarityNamePrefix}

// buildClientNames 从客户端内层 PVF 的文本表导出名字表。
//
// 文本表格式（string/*.uv.str，dataType=3）：UTF-16LE，每行 "key>value"，
// 以 // 开头的是注释行 —— 与 work-cn/tools/rewrite_str.py 的解析方式一致。
func buildClientNames(pvfPath, outPath string) (int, error) {
	a, err := pvf.LoadArchive(pvf.Options{Path: pvfPath, MaxBytes: 2 * 1024 * 1024 * 1024})
	if err != nil {
		return 0, fmt.Errorf("打开客户端内层 PVF %s: %w", pvfPath, err)
	}
	names := make(map[string]string, 512*1024)
	read := 0
	for _, table := range clientNameTables {
		text, err := a.ReadText(table)
		if err != nil {
			// 单张表缺失不影响其它表：PVF 版本差异下允许少表。
			fmt.Fprintf(os.Stderr, "提示：客户端文本表 %s 不可读（%v），跳过\n", table, err)
			continue
		}
		read++
		n := 0
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimRight(line, "\r")
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}
			i := strings.Index(line, ">")
			if i <= 0 {
				continue
			}
			key := strings.TrimSpace(line[:i])
			val := line[i+1:]
			if key == "" || val == "" || !hasWantedPrefix(key) {
				continue
			}
			if _, dup := names[key]; dup {
				continue // 先出现的表优先
			}
			names[key] = val
			n++
		}
		fmt.Fprintf(os.Stderr, "  客户端文本表 %-28s 新增 %d 条\n", table, n)
	}
	if read == 0 {
		return 0, fmt.Errorf("客户端内层 PVF %s 里没有任何可读的 string/*.uv.str 文本表", pvfPath)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return 0, err
	}
	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	w := bufio.NewWriterSize(f, 1<<20)
	// 直接按 key 排序写出，diff 时可读；格式与 names.zh.json 完全一致
	// （扁平 map，key -> 文本），所以 index.go 的 loadNames 不用改就能读。
	keys := make([]string, 0, len(names))
	for k := range names {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if _, err := w.WriteString("{"); err != nil {
		return 0, err
	}
	for i, k := range keys {
		if i > 0 {
			if err := w.WriteByte(','); err != nil {
				return 0, err
			}
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return 0, err
		}
		vb, err := json.Marshal(names[k])
		if err != nil {
			return 0, err
		}
		if _, err := w.Write(kb); err != nil {
			return 0, err
		}
		if err := w.WriteByte(':'); err != nil {
			return 0, err
		}
		if _, err := w.Write(vb); err != nil {
			return 0, err
		}
	}
	if _, err := w.WriteString("}"); err != nil {
		return 0, err
	}
	if err := w.Flush(); err != nil {
		return 0, err
	}
	if err := f.Close(); err != nil {
		return 0, err
	}
	return len(names), nil
}

func hasWantedPrefix(key string) bool {
	for _, p := range clientNamePrefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

// slotMapEntry 是 equipment.slots.json 里一件装备的 [部位cell, 最低等级]。
// 用数组而不是对象，是为了让这张 12 万行的表尽量小（紧凑 JSON 约 3MB）。
type slotMapEntry struct {
	Cell  string `json:"cell"`  // [equipment type] 的真实 cell 文本，如 "[weapon]"
	Level int32  `json:"level"` // [minimum level]
}

// slotMapFile 是 equipment.slots.json 的顶层结构。
// rows 缺失/为 0 等级表示目录里没有该 cell（例如某件装备只有数值 cell）。
type slotMapFile struct {
	Source  string                  `json:"source"`
	BuiltAt string                  `json:"built_at"`
	Slots   map[string]slotMapEntry `json:"slots"`
}

// catalogRow 是装备目录里一行的最小子集。
// 结构对应 configs/equipment.current37.json / equipment-full.json：
//
//	{"ID":10018,"Path":"...","Fields":{"[equipment type]":[{"type":6,"text":"[coat]"}],
//	 "[minimum level]":[{"type":0,"value":20}]}}
type catalogRow struct {
	ID     uint32                 `json:"ID"`
	Fields map[string][]pvf.Token `json:"Fields"`
}

// buildEquipmentSlots exports current native PVF display metadata.
func buildEquipmentSlots(outPath, archivePath string) (int, []string, error) {
	source, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: archivePath})
	if err != nil {
		return 0, nil, err
	}
	defer source.Close()
	index, err := source.ItemIndex("")
	if err != nil {
		return 0, nil, err
	}
	slots := map[string]slotMapEntry{}
	err = source.VisitItemDisplay(index, func(row catalog.ItemDisplay) error {
		if row.Kind == "equipment" && row.ID != 0 && row.EquipmentType != "" {
			slots[strconv.FormatUint(uint64(row.ID), 10)] = slotMapEntry{Cell: row.EquipmentType, Level: row.MinimumLevel}
		}
		return nil
	})
	if err != nil {
		return 0, nil, err
	}
	if len(slots) == 0 {
		return 0, nil, fmt.Errorf("native equipment display metadata is empty")
	}
	used := []string{source.Snapshot().Checksum}
	data, err := json.Marshal(slotMapFile{Source: used[0], BuiltAt: time.Now().Format(time.RFC3339), Slots: slots})
	if err != nil {
		return 0, used, err
	}
	if err = os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return 0, used, err
	}
	if err = os.WriteFile(outPath, data, 0644); err != nil {
		return 0, used, err
	}
	return len(slots), used, nil
}

// loadSlotMap 读取 equipment.slots.json。文件不存在时返回空表而不是报错 ——
// 此时 GM 工具回落为"只用服务端现役装备目录"给装备分部位（见 refineEquipment）。
func loadSlotMap(path string) map[string]slotMapEntry {
	out := map[string]slotMapEntry{}
	if strings.TrimSpace(path) == "" {
		return out
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var f slotMapFile
	if err := json.Unmarshal(b, &f); err != nil {
		fmt.Fprintf(os.Stderr, "警告：装备部位表 %s 不是合法 JSON（%v），本次只用装备目录\n", path, err)
		return map[string]slotMapEntry{}
	}
	return f.Slots
}
