// Package extradrop 是活动 1093「额外掉落奖励」的**源驱动目录**。
//
// 内容全部来自 PVF：脚本路径**从 `list/event.lst` 的索引解析**（活动 1093 的绑定），
// 不是从目录名/日期猜的 —— 这条纪律与 313（`internal/honeytime`）一致，原因也一样：
// 仓库里可能躺着**没有被任何索引引用**的同名脚本，写死它会让面板与发奖两套数据。
//
// ## 脚本形态（`.ctp`，不是文本 evt）
//
// `AdditionalDropRewardEvent.ctp` 是列式二进制表，用 `internal/catalog/pvf` 的
// `Archive.CTP` 解码。**列结构与 `etc/rewardboostinfo/**.ctp`（调律那条线）不同** ——
// 那边是 `[additional drop table]/[fixed drop table]/…`，这边是
// `[event period]/[target dungeon type]/[max amount]/[group table]`。
// 能复用的是**解码器与"读进来就自检"的做法**，不是那张表的模型。
//
// 形状（2026-10-11 实测，`analysis/tasks/next200-…md §2`）：
//
//	[event period]         两个文本：起止时刻（KST）
//	[target dungeon type]  一个文本：ENDKEEPER OF ORDER
//	[max amount]           → [amount] 一个数 / [type] 若干文本（"drop item"/"party drop item"）
//	[group table]          → N 个 [group]：
//	    [type]             "get item" / "drop item" / "party drop item"
//	    [item level]       一个数（本机 115）
//	    [item type]        若干文本（装备位名）
//	    [rule]             可选文本（本机 "check only higher rarity"）
//	    [reward table]     **三元组**（稀有度名, 物品号, 数量）
package extradrop

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"dfolan/internal/attendance"
	"dfolan/internal/catalog/pvf"
)

// Model 是这份目录的形状名。与 `loot.AttunementModel` 同一条纪律：
// 形状对不上就拒绝 —— 列错位时"发错物品"比"什么都不发"更糟。
const Model = "additional-drop-reward-ctp-v1"

// EventListPath 是「活动号 → 脚本路径」的索引（与 313 同一份）。
const EventListPath = "list/event.lst"

// EventID 是活动号（同时是 CMD680 载荷里的 u32，也是 op108 活动行的 id）。
const EventID uint32 = 1093

// RewardTier 是 `[reward table]` 的一行：**（稀有度名, 物品号, 数量）**。
type RewardTier struct {
	Rarity string
	Item   uint32
	Amount uint32
}

// Group 是一个 `[group]`。
type Group struct {
	// Type 是 `[type]`：本机三种 `get item` / `drop item` / `party drop item`。
	Type string
	// ItemLevel 是 `[item level]`。
	ItemLevel uint32
	// ItemTypes 是 `[item type]` 的装备位名（本机 14 类，如 primer/oath/weapon…）。
	ItemTypes []string
	// Rule 是可选 `[rule]`（本机只有 party drop item 组有："check only higher rarity"）。
	Rule string
	// Tiers 是 `[reward table]`，按源顺序。
	Tiers []RewardTier
}

// Catalog 是 `AdditionalDropRewardEvent.ctp` 解析出来的内容。
type Catalog struct {
	Path    string
	SHA256  string
	Bytes   int
	Version uint32
	// PeriodStart / PeriodEnd 是 `[event period]` 的两个时刻（**运营数据**，只留档；
	// 客户端不比较窗口，同 313 的结论）。
	PeriodStart string
	PeriodEnd   string
	// TargetDungeonType 是 `[target dungeon type]`（本机 `ENDKEEPER OF ORDER`）。
	TargetDungeonType string
	// TargetDungeonIDs 是上面那个类型**对应的副本 id**，由 `etc/rewardboostinfo/**` 解析而来
	// （见 resolveTargetDungeons）。**用 id 判副本，不要用类型字符串** ——
	// 2026-10-12 实测：副本脚本未必声明 `[dungeon type]`，而 rewardboostinfo 那侧一定声明
	// `[dungeon index]`（`etc/rewardboostinfo/endkeeperoforder/normal.ctp` → 100005014）。
	TargetDungeonIDs []uint32
	// MaxAmount / MaxTypes 是 `[max amount]`（本机 1000，限定 drop item / party drop item）。
	MaxAmount uint32
	MaxTypes  []string
	// Groups 是 `[group table]`，按源顺序。
	Groups []Group
}

// Load 从归档里读并解析本活动的脚本。
//
// 路径**只从索引解析**，找不到就报错 —— 不设回退（同 313：拿一份来源不明的脚本发奖，
// 比不装配这个活动更危险）。
func Load(a *pvf.Archive) (*Catalog, error) {
	text, err := a.ReadText(EventListPath)
	if err != nil {
		return nil, fmt.Errorf("extradrop 索引 %s: %w", EventListPath, err)
	}
	path, err := ResolveEventPath(text, EventID)
	if err != nil {
		return nil, err
	}
	table, err := a.CTP(path)
	if err != nil {
		return nil, fmt.Errorf("extradrop 脚本 %s: %w", path, err)
	}
	c, err := Parse(table)
	if err != nil {
		return nil, err
	}
	if err := resolveTargetDungeons(a, c); err != nil {
		return nil, err
	}
	return c, nil
}

// resolveTargetDungeons 把 `[target dungeon type]` 解析成副本 id 集合。
//
// 依据（两边都来自源，不做人工表）：`etc/rewardboostinfo/<目录>/*.ctp` 每张表都声明自己的
// `[dungeon index]`，而**目录名就是副本的类型名**（本机 `endkeeperoforder` ↔
// `[target dungeon type] ENDKEEPER OF ORDER`）。比较时把两边都归一化（小写 + 去掉非字母数字）。
//
// 找不到匹配目录**不是错误**（活动可能只声明类型不声明 index），此时退回类型字符串比较。
func resolveTargetDungeons(a *pvf.Archive, c *Catalog) error {
	want := normalizeType(c.TargetDungeonType)
	if want == "" {
		return nil
	}
	var paths []string
	if err := a.IterateFilesUnder("etc/rewardboostinfo", func(f pvf.File) error {
		p := strings.ToLower(strings.ReplaceAll(f.ArchivePath, `\`, "/"))
		if strings.HasPrefix(p, "etc/rewardboostinfo/") && strings.HasSuffix(p, ".ctp") {
			paths = append(paths, p)
		}
		return nil
	}); err != nil {
		return err
	}
	sort.Strings(paths)
	seen := map[uint32]bool{}
	for _, p := range paths {
		rest := strings.TrimPrefix(p, "etc/rewardboostinfo/")
		dir := rest
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			dir = rest[:i]
		}
		if normalizeType(dir) != want {
			continue
		}
		table, err := a.CTP(p)
		if err != nil {
			return fmt.Errorf("extradrop 副本绑定 %s: %w", p, err)
		}
		for _, r := range table.Records {
			if r.Parent >= 0 || r.Name != "[dungeon index]" {
				continue
			}
			v, err := number(&r)
			if err != nil {
				return fmt.Errorf("extradrop 副本绑定 %s: %w", p, err)
			}
			if v != 0 && !seen[v] {
				seen[v] = true
				c.TargetDungeonIDs = append(c.TargetDungeonIDs, v)
			}
		}
	}
	sort.Slice(c.TargetDungeonIDs, func(i, j int) bool { return c.TargetDungeonIDs[i] < c.TargetDungeonIDs[j] })
	return nil
}

// normalizeType 归一化类型名：小写、去掉所有非字母数字（`ENDKEEPER OF ORDER` → `endkeeperoforder`）。
func normalizeType(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ResolveEventPath 按 `list/event.lst` 的索引解析某活动号的脚本路径。
//
// 口径与 `honeytime.ResolveScriptPath` **逐条一致**（两两成对、重复绑定报错、
// 路径转小写 + 反斜杠转正斜杠、必须非绝对/无 `..`/后缀 `.ctp|.evt`）。
// ⚠️ 两处各写一份是当前的取舍（313 那份已合并、本轮不动它）；**合并到同一个助手是待办**
// —— 行为口径一旦漂移，症状是"某个活动悄悄读到另一份脚本"，和 313 那次一样难查。
func ResolveEventPath(indexText string, eventID uint32) (string, error) {
	tokens := attendance.Tokenize(indexText)
	if len(tokens) == 0 || len(tokens)%2 != 0 {
		return "", fmt.Errorf("extradrop 索引 %s: 条目不是成对的 (id, path)（%d 个 token）", EventListPath, len(tokens))
	}
	found := ""
	for i := 0; i+1 < len(tokens); i += 2 {
		id, convErr := strconv.ParseUint(tokens[i], 10, 32)
		if convErr != nil {
			return "", fmt.Errorf("extradrop 索引 %s: 第 %d 项的 id %q 不是数字", EventListPath, i/2, tokens[i])
		}
		if uint32(id) != eventID {
			continue
		}
		path := strings.ToLower(strings.ReplaceAll(tokens[i+1], `\`, "/"))
		switch {
		case path == "":
			return "", fmt.Errorf("extradrop 索引 %s: 活动 %d 的路径是空串", EventListPath, eventID)
		case strings.HasPrefix(path, "/"):
			return "", fmt.Errorf("extradrop 索引 %s: 活动 %d 的路径是绝对路径（%s）", EventListPath, eventID, path)
		case strings.Contains(path, ".."):
			return "", fmt.Errorf("extradrop 索引 %s: 活动 %d 的路径含 `..`（%s）", EventListPath, eventID, path)
		case !strings.HasSuffix(path, ".ctp") && !strings.HasSuffix(path, ".evt") && !strings.HasSuffix(path, ".cos"):
			return "", fmt.Errorf("extradrop 索引 %s: 活动 %d 的路径后缀不认（%s）", EventListPath, eventID, path)
		}
		if found != "" {
			return "", fmt.Errorf("extradrop 索引 %s: 活动 %d 有重复绑定", EventListPath, eventID)
		}
		found = path
	}
	if found == "" {
		return "", fmt.Errorf("extradrop 索引 %s: 没有活动 %d 的绑定", EventListPath, eventID)
	}
	return found, nil
}
