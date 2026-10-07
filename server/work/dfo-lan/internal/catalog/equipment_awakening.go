package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// 装备调适（CMD2258 `ENUM_CMDPACKET_EQUIPMENT_AWAKENING`）的**直读**规则源。
//
// 源（唯一内容真源 = 内层 PVF，不经任何导出 JSON）：
//
//	etc/115lvability/equipmentawakeningoptionsystem.cos   ← 本文档解析的规则表
//	etc/115lvability/equipmentawakeningoption.lst         ← 选项 ID → 加成表（见 Options）
//
// `equipmentawakeningoptionsystem.cos` 的结构（原生文本，逐段）：
//
//	[max awakening] 3                      调适阶段上限
//	[infos]
//	 [info]
//	  [condition] 115 `rare` 0             最低等级 / 稀有度 / 阶段
//	  [need materials]
//	   [group] 1                           材料组 1：只用基础材料
//	    0 2 0 150000 10361512 75           阶段 项目数 (模板 数量)…
//	   [/group]
//	   [group] 2                           材料组 2：额外消耗催化剂 10401346
//	    0 2 10401346 30 10361512 75
//	   [/group]
//	  [/need materials]
//	  [refund materials]                   返还（CMD2258 mode=1 / 转换降品）
//	   0 0
//	   1 1 10361512 75
//	  [/refund materials]
//	  [rates]                              各阶段成功率（本版本全 100）
//	   0 100
//	  [/rates]
//	  [upgrade result]                     阶段 3 的升品映射：源模板 候选数 目标…
//	   101001149 1 101001150
//	  [/upgrade result]
//	 [/info]
//	[/infos]
//
// 行格式约定（实测全部三个变体都是 `<阶段> <项目数> [<模板> <数量>]…`，**模板 0 = 金币**）：
//   - 数量为 0 的项目在源里被省略（例如 `0 2 0 150000 10361512 75` 只有两项），
//     所以**不能**按固定列数读，必须按"项目数"驱动。
//   - `[refund materials]` 用同一行格式，阶段 0 恒为 `0 0`（无返还）。
const (
	EquipmentAwakeningSystemPath        = "etc/115lvability/equipmentawakeningoptionsystem.cos"
	EquipmentAwakeningOptionListingPath = "etc/115lvability/equipmentawakeningoption.lst"
	// EquipmentAwakeningGoldTemplate 是成本行里代表金币的模板号（源里写 0）。
	EquipmentAwakeningGoldTemplate uint32 = 0
)

// EquipmentAwakeningRarityNames 与装备源 `[rarity]` 的数值一一对应（0..8）。
//
// 与 internal/inventory 的强化品质表同序（common…primeval）；调适规则表用**名字**
// 声明条件（`[condition] 115 `rare` 0`），装备源用的是**数值**，两者靠这张表对齐。
var EquipmentAwakeningRarityNames = []string{
	"common", "uncommon", "rare", "unique", "epic", "chronicle", "legendary", "mythology", "primeval",
}

// EquipmentAwakeningRarityName 把装备源的 `[rarity]` 数值换成调适规则表里的名字。
func EquipmentAwakeningRarityName(value int) (string, bool) {
	if value < 0 || value >= len(EquipmentAwakeningRarityNames) {
		return "", false
	}
	return EquipmentAwakeningRarityNames[value], true
}

// EquipmentAwakeningItem 是成本/返还里的一行：模板 0 表示金币。
type EquipmentAwakeningItem struct {
	Template uint32 `json:"template"`
	Amount   uint32 `json:"amount"`
}

// Gold 报告这一行是不是金币。
func (i EquipmentAwakeningItem) Gold() bool { return i.Template == EquipmentAwakeningGoldTemplate }

// EquipmentAwakeningRow 是某一阶段的一行成本/返还。
type EquipmentAwakeningRow struct {
	Stage int                      `json:"stage"`
	Items []EquipmentAwakeningItem `json:"items"`
}

// EquipmentAwakeningCostGroup 是一个材料组（源里 `[group] N`）。
type EquipmentAwakeningCostGroup struct {
	Index int                     `json:"index"`
	Rows  []EquipmentAwakeningRow `json:"rows"`
}

// Row 取某一阶段的成本行。
func (g EquipmentAwakeningCostGroup) Row(stage int) (EquipmentAwakeningRow, bool) {
	for _, r := range g.Rows {
		if r.Stage == stage {
			return r, true
		}
	}
	return EquipmentAwakeningRow{}, false
}

// Stages 列出本组声明过的阶段（升序，诊断用）。
func (g EquipmentAwakeningCostGroup) Stages() []int {
	out := make([]int, 0, len(g.Rows))
	for _, r := range g.Rows {
		out = append(out, r.Stage)
	}
	sort.Ints(out)
	return out
}

// EquipmentAwakeningUpgrade 是 `[upgrade result]` 的一行：阶段 3 成功后的候选目标。
type EquipmentAwakeningUpgrade struct {
	Count   int      `json:"count"`
	Targets []uint32 `json:"targets"`
}

// EquipmentAwakeningInfo 是一条 `[condition] <等级> <稀有度> <阶段>` 规则。
type EquipmentAwakeningInfo struct {
	Level  int    `json:"level"`
	Rarity string `json:"rarity"`
	Stage  int    `json:"stage"`

	Groups   []EquipmentAwakeningCostGroup        `json:"groups"`
	Refunds  []EquipmentAwakeningRow              `json:"refunds"`
	Rates    map[int]int                          `json:"rates"`
	Upgrades map[uint32]EquipmentAwakeningUpgrade `json:"upgrades"`
}

// Group 取一个材料组（源里组号从 1 起）。
func (i EquipmentAwakeningInfo) Group(index int) (EquipmentAwakeningCostGroup, bool) {
	for _, g := range i.Groups {
		if g.Index == index {
			return g, true
		}
	}
	return EquipmentAwakeningCostGroup{}, false
}

// GroupIndexes 列出本规则声明的材料组号（升序）。
func (i EquipmentAwakeningInfo) GroupIndexes() []int {
	out := make([]int, 0, len(i.Groups))
	for _, g := range i.Groups {
		out = append(out, g.Index)
	}
	sort.Ints(out)
	return out
}

// Refund 取某一阶段的返还行。
func (i EquipmentAwakeningInfo) Refund(stage int) (EquipmentAwakeningRow, bool) {
	for _, r := range i.Refunds {
		if r.Stage == stage {
			return r, true
		}
	}
	return EquipmentAwakeningRow{}, false
}

// Rate 取某一阶段的成功率（源里是百分比整数）。
func (i EquipmentAwakeningInfo) Rate(stage int) (int, bool) {
	v, ok := i.Rates[stage]
	return v, ok
}

// Upgrade 取某个源模板的升品目标。
func (i EquipmentAwakeningInfo) Upgrade(template uint32) (EquipmentAwakeningUpgrade, bool) {
	u, ok := i.Upgrades[template]
	return u, ok
}

// EquipmentAwakeningRules 是整张调适规则表。
type EquipmentAwakeningRules struct {
	Source pvf.ArchiveSnapshot `json:"source"`
	Path   string              `json:"path"`
	SHA256 string              `json:"sha256"`
	Bytes  int                 `json:"bytes"`

	// MaxLevel = 源里 `[max awakening]`，本版本 3。
	MaxLevel int                      `json:"max_level"`
	Infos    []EquipmentAwakeningInfo `json:"infos"`

	// Upgrades 是**跨块合并**的升品表：源模板 → 候选目标。
	//
	// 为什么必须合并：`[upgrade result]` 的表**不按当前档分块**。实测 2026-10-02：
	// `100051304`（稀有防具）的升品条目写在 `[condition] 115 rare 1` 块里，而升品动作
	// 发生在**阶 3**（客户端第 4 次请求时面板显示的成本 = `rare 3` 的行 3：
	// `100000 金币 + 10361513×40 + 10400396×1`，实机截图与源逐字段一致）。
	// 只按当前档的块查会永远查不到 ⇒ 每次升品都被拒（现象："1–3 次成功，第 4 次无法升品"）。
	Upgrades map[uint32]EquipmentAwakeningUpgrade `json:"upgrades"`
}

// UpgradeSource 在**全表**里查某个源模板的升品候选（升品动作必须用它，理由见 Upgrades）。
//
// 全局表为空时（例如单元测试手工构造的规则只填了分块表）回退到逐块查找。
func (r EquipmentAwakeningRules) UpgradeSource(template uint32) (EquipmentAwakeningUpgrade, bool) {
	if u, ok := r.Upgrades[template]; ok {
		return u, true
	}
	for _, info := range r.Infos {
		if u, ok := info.Upgrades[template]; ok {
			return u, true
		}
	}
	return EquipmentAwakeningUpgrade{}, false
}

// Info 按 (等级, 稀有度, 阶段) 取规则。
func (r EquipmentAwakeningRules) Info(level int, rarity string, stage int) (EquipmentAwakeningInfo, bool) {
	for _, i := range r.Infos {
		if i.Level == level && i.Rarity == rarity && i.Stage == stage {
			return i, true
		}
	}
	return EquipmentAwakeningInfo{}, false
}

// StagesFor 列出某个 (等级, 稀有度) 声明过的全部阶段（升序，诊断/校验用）。
func (r EquipmentAwakeningRules) StagesFor(level int, rarity string) []int {
	var out []int
	for _, i := range r.Infos {
		if i.Level == level && i.Rarity == rarity {
			out = append(out, i.Stage)
		}
	}
	sort.Ints(out)
	return out
}

// Templates 汇总 `[upgrade result]` 里出现过的全部源模板（诊断用）。
func (r EquipmentAwakeningRules) Templates() []uint32 {
	seen := map[uint32]bool{}
	var out []uint32
	for _, i := range r.Infos {
		for src := range i.Upgrades {
			if !seen[src] {
				seen[src] = true
				out = append(out, src)
			}
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a] < out[b] })
	return out
}

// ImportEquipmentAwakeningRules 从内层归档读取并解析调适规则表。
func ImportEquipmentAwakeningRules(a *pvf.Archive) (EquipmentAwakeningRules, error) {
	out := EquipmentAwakeningRules{Path: EquipmentAwakeningSystemPath}
	if a == nil {
		return out, fmt.Errorf("equipment awakening: archive is nil")
	}
	if _, ok := a.FindFile(EquipmentAwakeningSystemPath); !ok {
		return out, fmt.Errorf("%s is missing from the archive", EquipmentAwakeningSystemPath)
	}
	raw, e := a.ReadRaw(EquipmentAwakeningSystemPath)
	if e != nil {
		return out, e
	}
	sum := sha256.Sum256(raw)
	out.SHA256, out.Bytes = hex.EncodeToString(sum[:]), len(raw)
	text, e := a.ReadText(EquipmentAwakeningSystemPath)
	if e != nil {
		return out, e
	}
	parsed, e := ParseEquipmentAwakeningRules(text)
	if e != nil {
		return out, e
	}
	parsed.Source = a.Snapshot()
	parsed.Path = out.Path
	parsed.SHA256, parsed.Bytes = out.SHA256, out.Bytes
	return parsed, nil
}

// ParseEquipmentAwakeningRules 解析 `equipmentawakeningoptionsystem.cos` 的文本。
//
// 复用装备库那套通用树解析（同包的 `parseJournalTree`）：源里 `[infos]` / `[info]` /
// `[need materials]` / `[group]` / `[rates]` 都是**带尾标记的容器**，`[max awakening]`
// 是叶子 —— 不能按"有没有头"猜。
func ParseEquipmentAwakeningRules(text string) (EquipmentAwakeningRules, error) {
	root, e := parseJournalTree(text)
	if e != nil {
		return EquipmentAwakeningRules{}, e
	}
	out := EquipmentAwakeningRules{Path: EquipmentAwakeningSystemPath}
	maxNode := root.child("max awakening")
	if maxNode == nil {
		return out, fmt.Errorf("equipment awakening: [max awakening] is missing")
	}
	maxLevel, ok := journalUint(maxNode.Head)
	if !ok || maxLevel == 0 || maxLevel > 255 {
		return out, fmt.Errorf("equipment awakening: [max awakening] %q is not a usable level cap", maxNode.Head)
	}
	out.MaxLevel = int(maxLevel)

	infos := root.child("infos")
	if infos == nil {
		return out, fmt.Errorf("equipment awakening: [infos] is missing")
	}
	for _, node := range infos.children("info") {
		info, e := parseAwakeningInfo(node)
		if e != nil {
			return out, e
		}
		out.Infos = append(out.Infos, info)
	}
	if len(out.Infos) == 0 {
		return out, fmt.Errorf("equipment awakening: [infos] has no [info]")
	}
	// 同一个 (等级, 稀有度, 阶段) 只能有一条（重复会让"取哪条"变得随机）。
	seen := map[string]bool{}
	for _, i := range out.Infos {
		key := fmt.Sprintf("%d/%s/%d", i.Level, i.Rarity, i.Stage)
		if seen[key] {
			return out, fmt.Errorf("equipment awakening: duplicate [condition] %d `%s` %d", i.Level, i.Rarity, i.Stage)
		}
		seen[key] = true
	}
	// 合并出**跨块**的升品表：升品动作发生在阶 3，而 `[upgrade result]` 的源模板可能
	// 被写在任意一个 `[condition]` 块里（见 EquipmentAwakeningRules.Upgrades 的说明）。
	out.Upgrades = map[uint32]EquipmentAwakeningUpgrade{}
	for _, info := range out.Infos {
		for src, up := range info.Upgrades {
			prev, ok := out.Upgrades[src]
			if !ok {
				out.Upgrades[src] = up
				continue
			}
			merged := prev
			switch {
			case len(merged.Targets) == 0:
				// 空候选（源里 `模板 0`）不能盖掉有候选的那条。
				merged = up
			case len(up.Targets) > 0:
				// 两块都给了候选：取并集（源的语义未定，宁可放宽；去重保序）。
				for _, t := range up.Targets {
					dup := false
					for _, have := range merged.Targets {
						if have == t {
							dup = true
							break
						}
					}
					if !dup {
						merged.Targets = append(merged.Targets, t)
					}
				}
				if up.Count > merged.Count {
					merged.Count = up.Count
				}
			}
			out.Upgrades[src] = merged
		}
	}
	if len(out.Upgrades) == 0 {
		return out, fmt.Errorf("equipment awakening: [upgrade result] is empty across every [condition]")
	}
	return out, nil
}

func parseAwakeningInfo(node *journalNode) (EquipmentAwakeningInfo, error) {
	info := EquipmentAwakeningInfo{Rates: map[int]int{}, Upgrades: map[uint32]EquipmentAwakeningUpgrade{}}
	cond := node.child("condition")
	if cond == nil {
		return info, fmt.Errorf("equipment awakening: [info] without [condition]")
	}
	fields := strings.Fields(strings.ReplaceAll(cond.Head, "`", " "))
	if len(fields) != 3 {
		return info, fmt.Errorf("equipment awakening: [condition] %q is not <level> <rarity> <stage>", cond.Head)
	}
	level, e1 := strconv.Atoi(fields[0])
	stage, e2 := strconv.Atoi(fields[2])
	if e1 != nil || e2 != nil || level < 0 || level > 255 || stage < 0 || stage > 255 {
		return info, fmt.Errorf("equipment awakening: [condition] %q has an unusable level or stage", cond.Head)
	}
	if fields[1] == "" {
		return info, fmt.Errorf("equipment awakening: [condition] %q has an empty rarity", cond.Head)
	}
	info.Level, info.Rarity, info.Stage = level, fields[1], stage

	if need := node.child("need materials"); need != nil {
		for _, group := range need.children("group") {
			index, ok := journalUint(group.Head)
			if !ok || index == 0 || index > 255 {
				return info, fmt.Errorf("equipment awakening: %d/%s/%d has an unusable [group] %q", level, info.Rarity, stage, group.Head)
			}
			rows, e := parseAwakeningRows(group.Values)
			if e != nil {
				return info, fmt.Errorf("equipment awakening: %d/%s/%d group %d: %w", level, info.Rarity, stage, index, e)
			}
			info.Groups = append(info.Groups, EquipmentAwakeningCostGroup{Index: int(index), Rows: rows})
		}
		if len(info.Groups) == 0 {
			return info, fmt.Errorf("equipment awakening: %d/%s/%d has no material [group]", level, info.Rarity, stage)
		}
	} else {
		return info, fmt.Errorf("equipment awakening: %d/%s/%d has no [need materials]", level, info.Rarity, stage)
	}

	if refund := node.child("refund materials"); refund != nil {
		rows, e := parseAwakeningRows(refund.Values)
		if e != nil {
			return info, fmt.Errorf("equipment awakening: %d/%s/%d refund: %w", level, info.Rarity, stage, e)
		}
		info.Refunds = rows
	}

	if rates := node.child("rates"); rates != nil {
		for _, line := range rates.Values {
			f := strings.Fields(line)
			if len(f) != 2 {
				return info, fmt.Errorf("equipment awakening: %d/%s/%d [rates] row %q is not <stage> <rate>", level, info.Rarity, stage, line)
			}
			at, e1 := strconv.Atoi(f[0])
			rate, e2 := strconv.Atoi(f[1])
			if e1 != nil || e2 != nil || at < 0 {
				return info, fmt.Errorf("equipment awakening: %d/%s/%d [rates] row %q is not numeric", level, info.Rarity, stage, line)
			}
			info.Rates[at] = rate
		}
	} else {
		return info, fmt.Errorf("equipment awakening: %d/%s/%d has no [rates]", level, info.Rarity, stage)
	}

	if upgrade := node.child("upgrade result"); upgrade != nil {
		for _, line := range upgrade.Values {
			src, entry, e := parseAwakeningUpgrade(line)
			if e != nil {
				return info, fmt.Errorf("equipment awakening: %d/%s/%d [upgrade result] row %q: %w", level, info.Rarity, stage, line, e)
			}
			info.Upgrades[src] = entry
		}
	}
	return info, nil
}

// parseAwakeningRows 解析 `<阶段> <项目数> [<模板> <数量>]…` 的行集合。
//
// ⚠️ 第二个数字是**项目数上限**：源里会把数量为 0 的项目整对省略
// （实测 `[condition] 115 epic 5` 的 `3 3 0 800000 10415191 100` 声明 3 项却只带 2 对），
// 所以判据是"实际项目数不超过声明值"，不是相等。
func parseAwakeningRows(lines []string) ([]EquipmentAwakeningRow, error) {
	var out []EquipmentAwakeningRow
	seen := map[int]bool{}
	for _, line := range lines {
		f := strings.Fields(line)
		if len(f) < 2 {
			return nil, fmt.Errorf("row %q is not <stage> <count>", line)
		}
		stage, e1 := strconv.Atoi(f[0])
		count, e2 := strconv.Atoi(f[1])
		if e1 != nil || e2 != nil || stage < 0 || stage > 255 || count < 0 {
			return nil, fmt.Errorf("row %q has an unusable stage or item count", line)
		}
		if (len(f)-2)%2 != 0 || len(f)-2 > 2*count {
			return nil, fmt.Errorf("row %q declares at most %d item(s) but carries %d column(s)", line, count, len(f)-2)
		}
		if seen[stage] {
			return nil, fmt.Errorf("stage %d is declared twice", stage)
		}
		seen[stage] = true
		row := EquipmentAwakeningRow{Stage: stage}
		for i := 2; i < len(f); i += 2 {
			template, e3 := strconv.ParseUint(f[i], 10, 32)
			amount, e4 := strconv.ParseUint(f[i+1], 10, 32)
			if e3 != nil || e4 != nil {
				return nil, fmt.Errorf("row %q has a non-numeric item", line)
			}
			row.Items = append(row.Items, EquipmentAwakeningItem{Template: uint32(template), Amount: uint32(amount)})
		}
		out = append(out, row)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Stage < out[b].Stage })
	return out, nil
}

// parseAwakeningUpgrade 解析 `[upgrade result]` 的一行：`<源模板> <候选数> [<目标>]…`。
//
// 与成本行同一口径：候选数是上限，实际候选不超过它（源里 `117010253 0` 就是"没有候选"）。
func parseAwakeningUpgrade(line string) (uint32, EquipmentAwakeningUpgrade, error) {
	var entry EquipmentAwakeningUpgrade
	f := strings.Fields(line)
	if len(f) < 2 {
		return 0, entry, fmt.Errorf("not <source> <count>")
	}
	src, e1 := strconv.ParseUint(f[0], 10, 32)
	count, e2 := strconv.Atoi(f[1])
	if e1 != nil || e2 != nil || count < 0 {
		return 0, entry, fmt.Errorf("unusable source or candidate count")
	}
	if len(f)-2 > count {
		return 0, entry, fmt.Errorf("declares at most %d candidate(s) but carries %d", count, len(f)-2)
	}
	entry.Count = count
	for _, v := range f[2:] {
		n, e := strconv.ParseUint(v, 10, 32)
		if e != nil {
			return 0, entry, fmt.Errorf("non-numeric candidate %q", v)
		}
		entry.Targets = append(entry.Targets, uint32(n))
	}
	return uint32(src), entry, nil
}

// EquipmentAwakeningOptions 是 `equipmentawakeningoption.lst` 的投影：
// 选项 ID → 该选项的加成表路径。
//
// 装备源 `.equ` 里的 `[equipment awakening option] N` 就是这个 ID；对应的 etc 用
// `[parameter] [level] n` 声明各阶段的加成（`[skill bonus rate]` / `[equipment buff]`）。
// 名望侧的 `[equipment awakening fame value]` 走的是另一条既有链路
// （internal/character/fame_native.go，按 equipmentgrouping.etc 分组）。
type EquipmentAwakeningOptions struct {
	Source      pvf.ArchiveSnapshot `json:"source"`
	ListingPath string              `json:"listing_path"`
	SHA256      string              `json:"sha256"`
	Bytes       int                 `json:"bytes"`

	Entries []IndexEntry `json:"entries"`
}

// Path 按选项 ID 取加成表路径（相对归档根）。
func (o EquipmentAwakeningOptions) Path(id uint32) (string, bool) {
	for _, e := range o.Entries {
		if e.ID == id {
			return e.Path, true
		}
	}
	return "", false
}

// ImportEquipmentAwakeningOptions 读取选项索引表。
func ImportEquipmentAwakeningOptions(a *pvf.Archive) (EquipmentAwakeningOptions, error) {
	out := EquipmentAwakeningOptions{ListingPath: EquipmentAwakeningOptionListingPath}
	if a == nil {
		return out, fmt.Errorf("equipment awakening options: archive is nil")
	}
	if _, ok := a.FindFile(EquipmentAwakeningOptionListingPath); !ok {
		return out, fmt.Errorf("%s is missing from the archive", EquipmentAwakeningOptionListingPath)
	}
	raw, e := a.ReadRaw(EquipmentAwakeningOptionListingPath)
	if e != nil {
		return out, e
	}
	sum := sha256.Sum256(raw)
	out.SHA256, out.Bytes = hex.EncodeToString(sum[:]), len(raw)
	script, e := ReadScript(a, EquipmentAwakeningOptionListingPath)
	if e != nil {
		return out, e
	}
	entries, e := ParseIndex(script.Cells)
	if e != nil {
		return out, e
	}
	if len(entries) == 0 {
		return out, fmt.Errorf("equipment awakening options: listing has no entries")
	}
	out.Entries = entries
	out.Source = a.Snapshot()
	return out, nil
}
