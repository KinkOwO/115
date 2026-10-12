// Package oracleresearch 是活动 685「奥拉蔻的装备模块研究」的**源驱动目录**。
//
// 内容全部来自 PVF：脚本路径**从 `list/event.lst` 的索引解析**（活动 685 的绑定），
// 与 313（`internal/honeytime`）和 1093（`internal/extradrop`）同一条纪律 ——
// 不设回退：拿一份来源不明的脚本发奖，比不装配这个活动更危险。
//
// ## 脚本形态（`.ctp`）
//
// `Live/Event/Kor/2026/0326_OracleResearch/OracleSimulationResearch.ctp`（15 222 B）。
// 2026-10-12 全量解码（见 `analysis/tasks/next201-…md §8`）：
//
//	[event period]        起止（KST）
//	[season] → [period]
//	[function enabled]    production | combine | disjoint stackable   ← 面板三页签
//	[dummy period item]   590015778
//	[item table] → 两个 [group]：set（13 个套装模块）/ parts（11 个部位模块）
//	[production]          [material] → [index]=590015808(神秘能量) + [cost]=10
//	                      [result item] = 590015809(Oracle's Keepsake) × 5
//	                      [slot] 48 格（24 个模块各自的槽位，UI 用）
//	[combine]             [slot]×9 + [recipe] 715 格 = **143 条五元组**
//	                      (套装模块, 数量, 部位模块, 数量, 成品装备)
//	[disjoint stackable]  [result item] = 590015045 × 30（Doom Oracle Box (1)）
//	[ui main] … 面板 xui 与动画
package oracleresearch

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"dfolan/internal/attendance"
	"dfolan/internal/catalog/pvf"
)

// Model 是这份目录的形状名。与 `loot.AttunementModel` / `extradrop.Model` 同一条纪律：
// 形状对不上就拒绝 —— 列错位时"发错物品"比"什么都不发"更糟。
const Model = "simulation-research-ctp-v1"

// EventID 是活动号（op108 活动行、面板帧所属活动、C→S 动作命令都指它）。
const EventID uint32 = 685

// EventListPath 是「活动号 → 脚本路径」的索引（与 313/1093 同一份）。
const EventListPath = "list/event.lst"

// FunctionProduce / Combine / Disjoint 是 `[function enabled]` 的三个取值（本机实测）。
const (
	FunctionProduce  = "production"
	FunctionCombine  = "combine"
	FunctionDisjoint = "disjoint stackable"
)

// Recipe 是 `[combine] [recipe]` 的一条五元组：套装模块 + 部位模块 → 成品装备。
type Recipe struct {
	SetModule  uint32
	SetCount   uint32
	PartModule uint32
	PartCount  uint32
	ResultItem uint32
}

// Group 是 `[item table]` 的一个 [group]（本机 `set` 13 个 + `parts` 11 个）。
type Group struct {
	Type  string
	Items []uint32
}

// Catalog 是 `OracleSimulationResearch.ctp` 解析出来的内容。
type Catalog struct {
	Path    string
	SHA256  string
	Bytes   int
	Version uint32
	// PeriodStart / PeriodEnd 是 `[event period]` 的两个时刻（运营数据，只留档）。
	PeriodStart string
	PeriodEnd   string
	// Functions 是 `[function enabled]` 的三个功能名，按源顺序。
	Functions []string
	// Groups 是 `[item table]`，按源顺序（set 在前、parts 在后）。
	Groups []Group
	// Modules 是两个组拍平后的顺序 —— **生产按这个顺序轮换**（美服说明："模块的两种类型
	// 按生产次数顺序轮换"）。顺序取自源，不自造。
	Modules []uint32
	// ProductionMaterial / ProductionCost 是 `[production] [material] [index] / [cost]`。
	ProductionMaterial uint32
	ProductionCost     uint32
	// ProductionResultItem / ProductionResultCount 是 `[production] [result item]`。
	ProductionResultItem  uint32
	ProductionResultCount uint32
	// DisjointItem / DisjointCount 是 `[disjoint stackable] [result item]`（分解**每件**的产出）。
	DisjointItem  uint32
	DisjointCount uint32
	// Recipes 是 `[combine] [recipe]` 的 143 条五元组。
	Recipes []Recipe
	// CombineUnlock 是 `[combine]` 三个 result 槽的 `[args]`：**该槽解锁所需的「已产次数」**。
	// 官方 UI：“Assemble Slots are unlocked sequentially based on the Module Produce count.”
	// 本机 = 0 / 10 / 15（第 1/2/3 行），与 `sub_1404264C0` 的 `v11 > *(u32*)(obj+1253)` 对应。
	CombineUnlock []uint32
	// Slots 是 `[production] [slot]` 的 24 对 (模块号, 槽位号)。
	//
	// ★ C→S 的 2413/2414 请求里带的是**槽位号**（IDA `sub_140426970`：
	// `v19[2] = sub_140425090(obj, 套装模块号)`），所以服务端必须用它反查模块号。
	// 本机两者恰好差 590015754，但**不写死这个差**：从源里读，换季/换源自动跟着变。
	Slots []ModuleSlot
}

// Load 从归档里读并解析本活动的脚本（路径**只从索引解析**）。
func Load(a *pvf.Archive) (*Catalog, error) {
	text, err := a.ReadText(EventListPath)
	if err != nil {
		return nil, fmt.Errorf("oracleresearch 索引 %s: %w", EventListPath, err)
	}
	path, err := ResolveEventPath(text, EventID)
	if err != nil {
		return nil, err
	}
	table, err := a.CTP(path)
	if err != nil {
		return nil, fmt.Errorf("oracleresearch 脚本 %s: %w", path, err)
	}
	return Parse(table)
}

// ResolveEventPath 按 `list/event.lst` 解析某活动号的脚本路径。
//
// 口径与 `honeytime.ResolveScriptPath` / `extradrop.ResolveEventPath` **逐条一致**
// （成对、重复绑定报错、转小写与正斜杠、禁绝对路径与 `..`、后缀白名单）。
// ⚠️ 这已经**第三份**了 —— 合并成一个共享助手是明确待办（三份口径漂移的风险随份数增长）。
func ResolveEventPath(indexText string, eventID uint32) (string, error) {
	tokens := attendance.Tokenize(indexText)
	if len(tokens) == 0 || len(tokens)%2 != 0 {
		return "", fmt.Errorf("oracleresearch 索引 %s: 条目不是成对的 (id, path)（%d 个 token）", EventListPath, len(tokens))
	}
	found := ""
	for i := 0; i+1 < len(tokens); i += 2 {
		id, convErr := strconv.ParseUint(tokens[i], 10, 32)
		if convErr != nil {
			return "", fmt.Errorf("oracleresearch 索引 %s: 第 %d 项的 id %q 不是数字", EventListPath, i/2, tokens[i])
		}
		if uint32(id) != eventID {
			continue
		}
		path := strings.ToLower(strings.ReplaceAll(tokens[i+1], `\`, "/"))
		switch {
		case path == "":
			return "", fmt.Errorf("oracleresearch 索引 %s: 活动 %d 的路径是空串", EventListPath, eventID)
		case strings.HasPrefix(path, "/"):
			return "", fmt.Errorf("oracleresearch 索引 %s: 活动 %d 的路径是绝对路径（%s）", EventListPath, eventID, path)
		case strings.Contains(path, ".."):
			return "", fmt.Errorf("oracleresearch 索引 %s: 活动 %d 的路径含 `..`（%s）", EventListPath, eventID, path)
		case !strings.HasSuffix(path, ".ctp") && !strings.HasSuffix(path, ".evt") && !strings.HasSuffix(path, ".cos"):
			return "", fmt.Errorf("oracleresearch 索引 %s: 活动 %d 的路径后缀不认（%s）", EventListPath, eventID, path)
		}
		if found != "" {
			return "", fmt.Errorf("oracleresearch 索引 %s: 活动 %d 有重复绑定", EventListPath, eventID)
		}
		found = path
	}
	if found == "" {
		return "", fmt.Errorf("oracleresearch 索引 %s: 没有活动 %d 的绑定", EventListPath, eventID)
	}
	return found, nil
}

// Parse 把已解码的 CTP 表翻成目录。**纯函数**（不碰归档），单测可直接喂手搭的表。
func Parse(t *pvf.CTPTable) (*Catalog, error) {
	if t == nil {
		return nil, fmt.Errorf("oracleresearch: 空表")
	}
	c := &Catalog{Path: t.Path, SHA256: t.SHA256, Bytes: t.Bytes, Version: t.Version}
	byIndex := make(map[int]*pvf.CTPRecord, len(t.Records))
	for i := range t.Records {
		byIndex[t.Records[i].Index] = &t.Records[i]
	}
	top := func(name string) (*pvf.CTPRecord, error) {
		col := t.Column(name)
		if col == nil || len(col.Records) == 0 {
			return nil, fmt.Errorf("oracleresearch: 缺少 %s 段", name)
		}
		r := byIndex[int(col.Records[0])]
		if r == nil {
			return nil, fmt.Errorf("oracleresearch: %s 指向不存在的记录 %d", name, col.Records[0])
		}
		return r, nil
	}
	children := func(r *pvf.CTPRecord, name string) []*pvf.CTPRecord {
		var out []*pvf.CTPRecord
		for _, ref := range r.Refs {
			if ref.Name != name {
				continue
			}
			for _, v := range ref.Values {
				if child := byIndex[int(v)]; child != nil {
					out = append(out, child)
				}
			}
		}
		return out
	}
	one := func(recs []*pvf.CTPRecord, what string) (*pvf.CTPRecord, error) {
		if len(recs) != 1 {
			return nil, fmt.Errorf("oracleresearch: %s 有 %d 条，期望 1", what, len(recs))
		}
		return recs[0], nil
	}

	// [event period]：两个文本。
	period, err := top("[event period]")
	if err != nil {
		return nil, err
	}
	if dates := texts(period); len(dates) == 2 {
		c.PeriodStart, c.PeriodEnd = dates[0], dates[1]
	} else {
		return nil, fmt.Errorf("oracleresearch: [event period] 有 %d 个文本，期望 2", len(dates))
	}

	// [function enabled]：三个功能名（本机 production / combine / disjoint stackable）。
	fn, err := top("[function enabled]")
	if err != nil {
		return nil, err
	}
	c.Functions = texts(fn)
	if len(c.Functions) == 0 {
		return nil, fmt.Errorf("oracleresearch: [function enabled] 没有任何文本")
	}

	// [item table] → 若干 [group]，每组 [type] + [list]。
	itemTable, err := top("[item table]")
	if err != nil {
		return nil, err
	}
	groupRecs := children(itemTable, "[group]")
	if len(groupRecs) == 0 {
		return nil, fmt.Errorf("oracleresearch: [item table] 里没有 [group]")
	}
	for gi, gr := range groupRecs {
		var g Group
		ts, err := one(children(gr, "[type]"), fmt.Sprintf("第 %d 组的 [type]", gi))
		if err != nil {
			return nil, err
		}
		if v := texts(ts); len(v) == 1 {
			g.Type = v[0]
		} else {
			return nil, fmt.Errorf("oracleresearch: 第 %d 组的 [type] 有 %d 个文本，期望 1", gi, len(v))
		}
		ls, err := one(children(gr, "[list]"), fmt.Sprintf("第 %d 组的 [list]", gi))
		if err != nil {
			return nil, err
		}
		if g.Items, err = numbers(ls); err != nil {
			return nil, fmt.Errorf("oracleresearch: 第 %d 组的 [list]: %w", gi, err)
		}
		if len(g.Items) == 0 {
			return nil, fmt.Errorf("oracleresearch: 第 %d 组的 [list] 是空的", gi)
		}
		c.Groups = append(c.Groups, g)
		c.Modules = append(c.Modules, g.Items...)
	}

	// [production]：[material]([index],[cost]) 与 [result item](物品, 数量)。
	prod, err := top("[production]")
	if err != nil {
		return nil, err
	}
	mat, err := one(children(prod, "[material]"), "[production] 的 [material]")
	if err != nil {
		return nil, err
	}
	idx, err := one(children(mat, "[index]"), "[material] 的 [index]")
	if err != nil {
		return nil, err
	}
	if c.ProductionMaterial, err = number(idx); err != nil {
		return nil, fmt.Errorf("oracleresearch: [production] 材料号: %w", err)
	}
	cost, err := one(children(mat, "[cost]"), "[material] 的 [cost]")
	if err != nil {
		return nil, err
	}
	// [cost] 的单元格是混排的（本机 = 一个 opaque + 一个数值）⇒ 取**第一个可读的数**，
	// 不去猜 opaque 那格是什么（本仓对未证字段的一贯做法：不猜，记录缺口）。
	if c.ProductionCost, err = firstNumber(cost); err != nil {
		return nil, fmt.Errorf("oracleresearch: [production] 费用: %w", err)
	}
	res, err := one(children(prod, "[result item]"), "[production] 的 [result item]")
	if err != nil {
		return nil, err
	}
	if nums, err := numbers(res); err == nil && len(nums) >= 2 {
		c.ProductionResultItem, c.ProductionResultCount = nums[0], nums[1]
	} else {
		return nil, fmt.Errorf("oracleresearch: [production] 的 [result item] 不是 (物品, 数量)")
	}

	// [disjoint stackable]：[result item] = (物品, 每件数量)。
	dis, err := top("[disjoint stackable]")
	if err != nil {
		return nil, err
	}
	dres, err := one(children(dis, "[result item]"), "[disjoint stackable] 的 [result item]")
	if err != nil {
		return nil, err
	}
	if nums, err := numbers(dres); err == nil && len(nums) >= 2 {
		c.DisjointItem, c.DisjointCount = nums[0], nums[1]
	} else {
		return nil, fmt.Errorf("oracleresearch: [disjoint stackable] 的 [result item] 不是 (物品, 每件数量)")
	}

	// [combine]：[recipe] 的五元组。
	comb, err := top("[combine]")
	if err != nil {
		return nil, err
	}
	recipe, err := one(children(comb, "[recipe]"), "[combine] 的 [recipe]")
	if err != nil {
		return nil, err
	}
	if c.Recipes, err = recipes(recipe); err != nil {
		return nil, fmt.Errorf("oracleresearch: [combine] 的 [recipe]: %w", err)
	}
	// [combine] 的 9 个 [slot] 里，attribute=result 的那 3 个，其 [args] 就是该槽的解锁门槛。
	// 只认 attribute 为 result 的：material 槽的 [args] 是 {set, parts} 两个文本，不是数。
	for _, slotRec := range children(comb, "[slot]") {
		attrRecs := children(slotRec, "[attribute]")
		if len(attrRecs) != 1 {
			continue
		}
		attr := texts(attrRecs[0])
		if len(attr) != 1 || attr[0] != "result" {
			continue
		}
		argRecs := children(slotRec, "[args]")
		if len(argRecs) != 1 {
			return nil, fmt.Errorf("oracleresearch: [combine] 的 result [slot] 没有唯一的 [args]")
		}
		vals := texts(argRecs[0])
		if len(vals) != 1 {
			return nil, fmt.Errorf("oracleresearch: [combine] 的 result [slot] [args] 不是单个文本")
		}
		n, cerr := strconv.ParseUint(strings.TrimSpace(vals[0]), 10, 32)
		if cerr != nil {
			return nil, fmt.Errorf("oracleresearch: [combine] 的 result [slot] [args] %q 不是数: %w", vals[0], cerr)
		}
		c.CombineUnlock = append(c.CombineUnlock, uint32(n))
	}
	if len(c.CombineUnlock) != 3 {
		return nil, fmt.Errorf("oracleresearch: [combine] 的 result 槽有 %d 个，期望 3", len(c.CombineUnlock))
	}
	// [production] [slot]：24 对 (模块号, 槽位号)，供 2413/2414 用槽位号反查模块。
	if slotRecs := children(prod, "[slot]"); len(slotRecs) == 1 {
		nums, serr := numbers(slotRecs[0])
		if serr != nil {
			return nil, fmt.Errorf("oracleresearch: [production] 的 [slot]: %w", serr)
		}
		if len(nums) == 0 || len(nums)%2 != 0 {
			return nil, fmt.Errorf("oracleresearch: [production] 的 [slot] 有 %d 个数，不是 2 的整数倍", len(nums))
		}
		for i := 0; i < len(nums); i += 2 {
			c.Slots = append(c.Slots, ModuleSlot{Module: nums[i], Slot: nums[i+1]})
		}
	}
	return c, nil
}

// texts 取一个记录里所有文本型单元格。
func texts(r *pvf.CTPRecord) []string {
	var out []string
	for _, cell := range r.Cells {
		switch cell.Kind {
		case "name", "name_indexed":
			out = append(out, cell.Name)
		}
	}
	return out
}

// numbers 取一个记录里所有数值型单元格（float，或能解析成十进制的文本）。
func numbers(r *pvf.CTPRecord) ([]uint32, error) {
	var out []uint32
	for _, cell := range r.Cells {
		switch cell.Kind {
		case "float":
			if cell.Float < 0 {
				return nil, fmt.Errorf("负数 %g", cell.Float)
			}
			out = append(out, uint32(cell.Float+0.5))
		case "name", "name_indexed":
			v, err := strconv.ParseUint(strings.TrimSpace(cell.Name), 10, 32)
			if err != nil {
				return nil, fmt.Errorf("文本 %q 不是十进制数", cell.Name)
			}
			out = append(out, uint32(v))
		}
	}
	return out, nil
}

// number 读"只有一个数"的记录。
func number(r *pvf.CTPRecord) (uint32, error) {
	nums, err := numbers(r)
	if err != nil {
		return 0, err
	}
	if len(nums) != 1 {
		return 0, fmt.Errorf("有 %d 个数，期望 1", len(nums))
	}
	return nums[0], nil
}

// firstNumber 取第一个可读的数（用于混排单元格，例如 `[cost]`）。
func firstNumber(r *pvf.CTPRecord) (uint32, error) {
	nums, err := numbers(r)
	if err != nil {
		return 0, err
	}
	if len(nums) == 0 {
		return 0, fmt.Errorf("没有可读的数")
	}
	return nums[0], nil
}

// recipes 解析 `[combine] [recipe]`：**五元组**（套装模块, 数量, 部位模块, 数量, 成品）。
//
// 个数不是 5 的整数倍就拒绝（宁可拒绝也不要按错位发装备）。
func recipes(r *pvf.CTPRecord) ([]Recipe, error) {
	nums, err := numbers(r)
	if err != nil {
		return nil, err
	}
	if len(nums) == 0 || len(nums)%5 != 0 {
		return nil, fmt.Errorf("有 %d 个数，不是 5 的整数倍", len(nums))
	}
	out := make([]Recipe, 0, len(nums)/5)
	seen := map[uint32]bool{}
	for i := 0; i < len(nums); i += 5 {
		rc := Recipe{SetModule: nums[i], SetCount: nums[i+1], PartModule: nums[i+2], PartCount: nums[i+3], ResultItem: nums[i+4]}
		if rc.SetModule == 0 || rc.PartModule == 0 || rc.ResultItem == 0 {
			return nil, fmt.Errorf("第 %d 条配方里有 0（%+v）", i/5, rc)
		}
		if rc.SetCount == 0 || rc.PartCount == 0 {
			return nil, fmt.Errorf("第 %d 条配方的数量是 0（%+v）", i/5, rc)
		}
		if seen[rc.ResultItem] {
			return nil, fmt.Errorf("成品 %d 在配方里出现两次", rc.ResultItem)
		}
		seen[rc.ResultItem] = true
		out = append(out, rc)
	}
	return out, nil
}

// FindRecipe 按（套装模块, 部位模块）找配方；找不到 ok=false。
func (c *Catalog) FindRecipe(setModule, partModule uint32) (Recipe, bool) {
	if c == nil {
		return Recipe{}, false
	}
	for _, r := range c.Recipes {
		if r.SetModule == setModule && r.PartModule == partModule {
			return r, true
		}
	}
	return Recipe{}, false
}

// ModuleAt 返回"本自然周第 n 次生产（n 从 0 起）"应产出的模块号。
//
// 依据：美服说明「模块的两种类型按生产次数顺序轮换」+ 源里 `[item table]` 的固定顺序
// ⇒ **按 `Modules` 的顺序循环取**。`Modules` 空时 ok=false（源没给清单就不生产）。
func (c *Catalog) ModuleAt(n uint32) (uint32, bool) {
	if c == nil || len(c.Modules) == 0 {
		return 0, false
	}
	return c.Modules[int(n)%len(c.Modules)], true
}

// ProductionCostFor 返回本自然周第 n 次生产（n 从 0 起）的费用。
//
// 依据：源 `[production] [material] [cost]` = **基准**（本机 10）+ 美服说明
// 「每次累计生产所需能量递增 10」⇒ 第 n 次 = `基准 × (n+1)`。
func (c *Catalog) ProductionCostFor(n uint32) uint32 {
	if c == nil {
		return 0
	}
	return c.ProductionCost * (n + 1)
}

// Describe 返回一行摘要（日志用）。
func (c *Catalog) Describe() string {
	if c == nil {
		return "<nil>"
	}
	return fmt.Sprintf("oracleresearch685 path=%s functions=%v modules=%d cost=%d(+%d/次) result=%d×%d disjoint=%d×%d recipes=%d",
		c.Path, c.Functions, len(c.Modules), c.ProductionCost, c.ProductionCost,
		c.ProductionResultItem, c.ProductionResultCount, c.DisjointItem, c.DisjointCount, len(c.Recipes))
}

// FunctionNames 返回排序后的功能名（日志/对比用，避免调用方依赖源顺序）。
func (c *Catalog) FunctionNames() []string {
	if c == nil {
		return nil
	}
	out := append([]string(nil), c.Functions...)
	sort.Strings(out)
	return out
}

// ModuleSlot 是 `[production] [slot]` 的一对：**模块物品号 ↔ 槽位号**。
//
// 槽位号就是客户端在 C→S 请求里用的下标（IDA `sub_140426970` 用它拼 2413 的正文字段 3/4），
// 所以服务端必须由它反查模块号 —— 本机两者恰好差 590015754，但那是**巧合**，不写死。
type ModuleSlot struct {
	Module uint32
	Slot   uint32
}

// ModuleAtSlot 按槽位号反查模块物品号。
func (c *Catalog) ModuleAtSlot(slot uint32) (uint32, bool) {
	if c == nil {
		return 0, false
	}
	for _, s := range c.Slots {
		if s.Slot == slot {
			return s.Module, true
		}
	}
	return 0, false
}

// RecipeFor 按（套装模块, 部位模块）查一条配方。
//
// 本机 143 条 = 13 套装 × 11 部位（全组合）⇒ 任意一对都有解；
// 但仍按"查不到就拒绝"写，源变了也不会发错东西。
func (c *Catalog) RecipeFor(setModule, partModule uint32) (Recipe, bool) {
	if c == nil {
		return Recipe{}, false
	}
	for _, r := range c.Recipes {
		if r.SetModule == setModule && r.PartModule == partModule {
			return r, true
		}
	}
	return Recipe{}, false
}

// UnlockForRow 返回第 row 行组装槽解锁所需的「已产次数」（越界返回 0）。
func (c *Catalog) UnlockForRow(row int) uint32 {
	if c == nil || row < 0 || row >= len(c.CombineUnlock) {
		return 0
	}
	return c.CombineUnlock[row]
}
