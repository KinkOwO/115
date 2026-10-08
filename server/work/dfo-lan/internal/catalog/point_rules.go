package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// 装备 / 誓约「积分」源表（PVF 直读，2026-10-04 取证）。
//
// 背景：用户实机在「装备库 → 誓约」看到 `0/750次` 与「已添加的 誓约积分: 0」。
// 取证结论（见 docs/protocol/oath-set-points-20261004.md §7）：
//   - 客户端**不自己算总分**：它把服务端推来的「每角色一对 Set/Oath Point」原样存进角色实体
//     （NOTI 2634：`u16 实体键 + u32 A + u32 B` → `实体+1872/+1876`）；
//   - **每件**装备/晶体的分数由这两张源表给出（客户端只用它做逐件明细）；
//   - 我们此前既没算过分、也没发过 2634 ⇒ 显示恒 0。
//
// 两份源文件都是**文本**（UTF-16；`pvf.Archive.ReadText` 按 dataType 自动解码），
// 且**分段结构**（只解析第一段，后面的段是别的东西）：
//
//	etc/115lvability/setpointinfo.cos
//	  [set point] [table] [info] [group] g [awakening] a [part set index] p [value] v [/info] … [/table] [/set point]
//	  [noti part set points] 1200/1650/2100/2550  ← 达到档位时的通知特效（2550 = primeval）
//	  [grade list]  1 750 …rare / 2 1200 …unique / … / 5 2550 …primeval   ← 积分档位阶梯（截图里的「稀有 I 达成750」）
//	  [set point info]  按套装号的图标/名称元数据
//	  [add parameters]  2620/2690/… → AddParameter/i.etc（额外参数档）
//
//	etc/115lvability2/oathpointinfo.cos
//	  [oath point] [table] <模板> <awakening> <part set index> <积分> … [/table] [/oath point]
//	  [noti oath points] / [grade list] / [min oath point] 1200 ← 誓约侧门槛
//
// `[awakening]` 是装备实例的**调适档位**（0 = 未调适，源里同样给了基础分 ⇒ "0 分"不是设计使然）；
// `[part set index]` 是 `.equ` 的套装号（-1 = 通用行）。积分数值是**服务端**要算并推送的。
const (
	// SetPointInfoPath 是逐件「套装积分」表。
	SetPointInfoPath = "etc/115lvability/setpointinfo.cos"
	// OathPointInfoPath 是逐件「誓约积分」表（誓约/晶体那一支）。
	OathPointInfoPath = "etc/115lvability2/oathpointinfo.cos"
)

// SetPointRule 是 `[set point] [table]` 的一行：某 group 在某个调适档位、某个套装号下的积分。
type SetPointRule struct {
	Group        uint32
	Awakening    int32
	PartSetIndex int32
	Value        uint32
}

// OathPointRule 是 `[oath point] [table]` 的一行：某模板在某个调适档位、某个套装号下的积分。
type OathPointRule struct {
	Template     uint32
	Awakening    int32
	PartSetIndex int32
	Value        uint32
}

// SetPointGrade 是 `[grade list]` 的一行：积分达到 `Threshold` 即进入 `Grade` 档。
type SetPointGrade struct {
	Grade      int32
	Threshold  uint32
	NameDSTR   string
	NumberDSTR string
	Rarity     string
	Color      string
}

// SetPointTable 是 `setpointinfo.cos` 第一段 + 档位阶梯的投影。
type SetPointTable struct {
	Rules  []SetPointRule
	Grades []SetPointGrade
}

// OathPointTable 是 `oathpointinfo.cos` 第一段 + 誓约门槛的投影。
type OathPointTable struct {
	Rules        []OathPointRule
	MinOathPoint uint32
}

// PointRules 是两张积分表的直读投影。
type PointRules struct {
	Source         pvf.ArchiveSnapshot
	Set            SetPointTable
	Oath           OathPointTable
	SetPointBytes  int
	OathPointBytes int
	SetPointSHA256 string
	OathSHA256     string
	// AbilityGroups 是套装积分的第一层映射（模板 -> 能力组号），来自
	// etc/equipmentgrouping.etc 的 `[ability group]`。没有它 SetPoints 算不出来。
	AbilityGroups map[uint32][]uint32
}

// ImportPointRules 从内层归档直读两张积分表。缺任何一个都报错（不做 JSON 回落）。
func ImportPointRules(a *pvf.Archive) (PointRules, error) {
	var out PointRules
	raw, e := a.ReadRaw(SetPointInfoPath)
	if e != nil {
		return out, e
	}
	out.SetPointBytes = len(raw)
	sum := sha256.Sum256(raw)
	out.SetPointSHA256 = hex.EncodeToString(sum[:])
	text, e := a.ReadText(SetPointInfoPath)
	if e != nil {
		return out, e
	}
	if out.Set, e = ParseSetPointInfo(text); e != nil {
		return out, fmt.Errorf("%s: %w", SetPointInfoPath, e)
	}
	raw, e = a.ReadRaw(OathPointInfoPath)
	if e != nil {
		return out, e
	}
	out.OathPointBytes = len(raw)
	sum = sha256.Sum256(raw)
	out.OathSHA256 = hex.EncodeToString(sum[:])
	text, e = a.ReadText(OathPointInfoPath)
	if e != nil {
		return out, e
	}
	if out.Oath, e = ParseOathPointInfo(text); e != nil {
		return out, fmt.Errorf("%s: %w", OathPointInfoPath, e)
	}
	// 套装积分的第一层映射（模板 -> 能力组）。缺了它 SetPoints 只能返回"算不出"，
	// 而 NOTI2634 会把 Set Point 记 0 —— 所以这里和两张表一样，读不到就显式报错。
	if out.AbilityGroups, e = abilityGroupMembership(a); e != nil {
		return out, e
	}
	out.Source = a.Snapshot()
	return out, nil
}

// sectionLines 取 `[open] … [close]` 之间的行（不含两个边界）。
func sectionLines(text, open, close string) []string {
	var out []string
	inside := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case open:
			inside = true
			continue
		case close:
			if inside {
				return out
			}
			continue
		}
		if inside {
			out = append(out, trimmed)
		}
	}
	return out
}

// ParseSetPointInfo 解析 `setpointinfo.cos`：第一段 `[set point]` 的 `[info]` 行 + `[grade list]` 档位阶梯。
func ParseSetPointInfo(text string) (SetPointTable, error) {
	var out SetPointTable
	var cur SetPointRule
	var haveGroup, haveValue bool
	flush := func() {
		if haveGroup && haveValue {
			out.Rules = append(out.Rules, cur)
		}
		cur, haveGroup, haveValue = SetPointRule{}, false, false
	}
	for _, line := range sectionLines(text, "[set point]", "[/set point]") {
		switch {
		case line == "[info]":
			flush()
		case line == "[/info]":
			flush()
		case strings.HasPrefix(line, "[group]"):
			v, e := parseIntField(line, "[group]")
			if e != nil {
				return out, e
			}
			cur.Group, haveGroup = uint32(v), true
		case strings.HasPrefix(line, "[awakening]"):
			v, e := parseIntField(line, "[awakening]")
			if e != nil {
				return out, e
			}
			cur.Awakening = int32(v)
		case strings.HasPrefix(line, "[part set index]"):
			v, e := parseIntField(line, "[part set index]")
			if e != nil {
				return out, e
			}
			cur.PartSetIndex = int32(v)
		case strings.HasPrefix(line, "[value]"):
			v, e := parseIntField(line, "[value]")
			if e != nil {
				return out, e
			}
			cur.Value, haveValue = uint32(v), true
		}
	}
	flush()
	if len(out.Rules) == 0 {
		return out, fmt.Errorf("set point table has no [info] row")
	}
	for _, line := range sectionLines(text, "[grade list]", "[/grade list]") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		grade, e1 := strconv.ParseInt(fields[0], 10, 32)
		threshold, e2 := strconv.ParseUint(fields[1], 10, 32)
		if e1 != nil || e2 != nil || !strings.HasPrefix(fields[2], "<37::") {
			continue
		}
		g := SetPointGrade{
			Grade:      int32(grade),
			Threshold:  uint32(threshold),
			NameDSTR:   fields[2],
			NumberDSTR: fields[3],
		}
		if len(fields) >= 5 {
			g.Rarity = strings.Trim(fields[4], "`")
		}
		if len(fields) >= 6 {
			g.Color = strings.Trim(fields[5], "`")
		}
		out.Grades = append(out.Grades, g)
	}
	return out, nil
}

// ParseOathPointInfo 解析 `oathpointinfo.cos`：`[oath point]` 的 `<模板> <awakening> <套装号> <积分>` 行
// 与 `[min oath point] N` 门槛。
func ParseOathPointInfo(text string) (OathPointTable, error) {
	var out OathPointTable
	for _, line := range sectionLines(text, "[oath point]", "[/oath point]") {
		if line == "" || strings.HasPrefix(line, "[") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		nums := make([]int64, 0, 4)
		ok := true
		for _, f := range fields[:4] {
			v, e := strconv.ParseInt(f, 10, 64)
			if e != nil {
				ok = false
				break
			}
			nums = append(nums, v)
		}
		if !ok || nums[0] <= 0 {
			continue
		}
		out.Rules = append(out.Rules, OathPointRule{
			Template:     uint32(nums[0]),
			Awakening:    int32(nums[1]),
			PartSetIndex: int32(nums[2]),
			Value:        uint32(nums[3]),
		})
	}
	if len(out.Rules) == 0 {
		return out, fmt.Errorf("oath point table has no row")
	}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "[min oath point]") {
			continue
		}
		v, e := parseIntField(trimmed, "[min oath point]")
		if e != nil {
			return out, e
		}
		out.MinOathPoint = uint32(v)
	}
	return out, nil
}

// SetPointFor 精确查一行（group + awakening + part set index 全等）。
func (p PointRules) SetPointFor(group uint32, awakening int32, partSetIndex int32) (uint32, bool) {
	for _, r := range p.Set.Rules {
		if r.Group == group && r.Awakening == awakening && r.PartSetIndex == partSetIndex {
			return r.Value, true
		}
	}
	return 0, false
}

// OathPointFor 精确查一行（模板 + awakening + part set index 全等）。
func (p PointRules) OathPointFor(template uint32, awakening int32, partSetIndex int32) (uint32, bool) {
	for _, r := range p.Oath.Rules {
		if r.Template == template && r.Awakening == awakening && r.PartSetIndex == partSetIndex {
			return r.Value, true
		}
	}
	return 0, false
}

// GradeFor 返回积分总量所处的档位（取不超过该分值的最高档；低于最低档返回 ok=false）。
func (p PointRules) GradeFor(points uint32) (SetPointGrade, bool) {
	var best SetPointGrade
	found := false
	for _, g := range p.Set.Grades {
		if g.Threshold <= points && (!found || g.Threshold >= best.Threshold) {
			best, found = g, true
		}
	}
	return best, found
}

// pointGroupingPath 是「能力组」成员表：`[ability group] [index] <组号> [list] <模板…>`。
//
// 这是套装积分的第一层映射：`setpointinfo.cos` 的 `[info] [group] <g>` **不是装备分组**，
// 而是这里的组号。第二层是 `[group] g + [awakening] a`（`[part set index]` 为 -1 的行用
// 该件 `.equ` 自己的 `[part set index]` 补上）。
//
// 为什么在这里读而不是复用名望侧的解析器：积分规则住在本包，跨包注入一个只为拼
// 字符串的 map 不值得；`character.FameRules.Groups` 由 fame 侧从同一文件解析给
// `[equip grouping]` 用，两边读同一个源、同一口径，都不维护手工清单（§0.2）。
const pointGroupingPath = "etc/equipmentgrouping.etc"

// abilityGroupMembership 把 `etc/equipmentgrouping.etc` 的 `[ability group]` 块解析成
// 模板 -> 组号。字段布局与名望侧解析器一致：`[index] <组号>` 后跟 `[list] <模板…> [/list]`。
func abilityGroupMembership(a *pvf.Archive) (map[uint32][]uint32, error) {
	script, err := ResolveScript(a, pointGroupingPath)
	if err != nil {
		return nil, err
	}
	out := map[uint32][]uint32{}
	var group int64 = -1
	pendingIndex, inList := false, false
	for _, tok := range script.Cells {
		if tok.Type == 3 {
			switch tok.Text {
			case "[ability group]", "[/ability group]":
				group, pendingIndex, inList = -1, false, false
			case "[index]":
				pendingIndex, inList = true, false
			case "[list]":
				inList = true
			case "[/list]":
				inList = false
			}
			continue
		}
		if tok.Type != 0 {
			continue
		}
		if pendingIndex {
			group, pendingIndex = int64(tok.Value), false
			continue
		}
		if inList && group >= 0 && tok.Value > 0 {
			id := uint32(tok.Value)
			out[id] = append(out[id], uint32(group))
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s has no [ability group] membership", pointGroupingPath)
	}
	return out, nil
}

// PointItem 是参与算分的一件装备：模板 + 调适档位 + 套装号（`.equ` 的 `[part set index]`，没有则 -1）。
type PointItem struct {
	Template     uint32
	Awakening    int32
	PartSetIndex int32
}

// OathPoints 求誓约积分合计：每件按 (模板, 档位, 套装号) 查表，逐级回退。返回 (合计, 命中件数)。
//
// 回退顺序（都来自实读源表）：
//  1. 精确行 (模板, 档位, 该件套装号)；
//  2. 该件套装号非 -1 时，回退到通用行 (模板, 档位, -1)；
//  3. 否则在 (模板, 档位) 的行里取：优先 -1 行，其次**唯一**那一行。
//     例：誓约核心 `100610065` 只有 `0 16210 455` 这一行 ⇒ 取 455（它不是 -1 行，
//     但该模板在源里就这一条，属确定命中）；同一 (模板, 档位) 有多行且都不是 -1 时
//     **跳过**（归属不确定，宁可少算也不瞎算）。
//
// 未调适（档位 0）同样有基础分（晶体 `100401592` = 45）—— 这是"誓约积分本该非 0"的直接依据。
func (p PointRules) OathPoints(items []PointItem) (uint32, int) {
	var total uint32
	hits := 0
	for _, it := range items {
		if v, ok := p.oathPointForItem(it); ok {
			total += v
			hits++
		}
	}
	return total, hits
}

// oathPointForItem 查单件的誓约积分（含档位回退与唯一行回退）。
func (p PointRules) oathPointForItem(it PointItem) (uint32, bool) {
	if v, ok := p.OathPointFor(it.Template, it.Awakening, it.PartSetIndex); ok {
		return v, true
	}
	if it.PartSetIndex != -1 {
		if v, ok := p.OathPointFor(it.Template, it.Awakening, -1); ok {
			return v, true
		}
	}
	// 该件的调适档位在源里没有对应行（老件/未知档位）⇒ 回退 0 档基础分，
	// 而不是把整件丢掉：0 档是"未调适"的权威基线，源里每支都给了行。
	if it.Awakening != 0 {
		if v, ok := p.OathPointFor(it.Template, 0, it.PartSetIndex); ok {
			return v, true
		}
		if it.PartSetIndex != -1 {
			if v, ok := p.OathPointFor(it.Template, 0, -1); ok {
				return v, true
			}
		}
	}
	// (模板, 档位) 唯一行回退：优先 -1 行，其次唯一那一行。
	value, rows := uint32(0), 0
	var general uint32
	haveGeneral := false
	for _, r := range p.Oath.Rules {
		if r.Template != it.Template || r.Awakening != it.Awakening {
			continue
		}
		rows++
		value = r.Value
		if r.PartSetIndex == -1 {
			general, haveGeneral = r.Value, true
		}
	}
	switch {
	case haveGeneral:
		return general, true
	case rows == 1:
		return value, true
	}
	return 0, false
}

// SetPoints 求套装积分合计：每件按 (能力组, 档位) 查 `setpointinfo.cos`，逐行加总。
//
// 两层映射（2026-10-07 取证）：
//
//	模板 --(AbilityGroups：equipmentgrouping.etc 的 [ability group])--> 能力组号
//	能力组号 + 档位 --(setpointinfo.cos 的 [info])--> 每件积分
//
// 行归属：`[part set index]` 为 -1 的行用该件自己的 `[part set index]` 补上
// （与名望侧 `character.fame` 同一口径：`if id == -1 { id = set }`）。补完仍 <= 0
// （该件没有套装号可归属，例如没有 `[part set index]` 的耳环命中 group 306 的 -1 行）
// **不累加**——归属不确定就不瞎算。
//
// 档位只做精确匹配：不能像誓约那样回退 0 档。实机与源都证明「能力组在表里没有该档位
// 的行 ⇒ 该件 0 分」（例：`group 52` 只有 `[awakening] 0`，写调适 3 反而归 0），
// 回退 0 档会把 0 分算成有分。
//
// 返回 (合计, 命中件数)。未装载 AbilityGroups 时返回 (0, 0)：调用方必须把它当
// "算不出"，不得拿 0 冒充"该角色积分为 0"。
func (p PointRules) SetPoints(items []PointItem) (uint32, int) {
	if len(p.AbilityGroups) == 0 {
		return 0, 0
	}
	var total uint32
	hits := 0
	for _, it := range items {
		groups := p.AbilityGroups[it.Template]
		if len(groups) == 0 {
			continue
		}
		got := false
		for _, g := range groups {
			for _, r := range p.Set.Rules {
				if r.Group != g || r.Awakening != it.Awakening {
					continue
				}
				owner := r.PartSetIndex
				if owner == -1 {
					owner = it.PartSetIndex
				}
				if owner <= 0 {
					continue
				}
				total += r.Value
				got = true
			}
		}
		if got {
			hits++
		}
	}
	return total, hits
}

// parseIntField 取 "[tag] 123" 形式的数值（裸标签视为错误，由调用方决定是否容忍）。
func parseIntField(line, tag string) (int64, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, tag))
	if rest == "" {
		return 0, fmt.Errorf("%s has no value: %q", tag, line)
	}
	v, e := strconv.ParseInt(strings.Fields(rest)[0], 10, 64)
	if e != nil {
		return 0, fmt.Errorf("%s: %w", tag, e)
	}
	return v, nil
}
