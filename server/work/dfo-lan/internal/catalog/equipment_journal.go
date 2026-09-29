package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// EquipmentJournalPath 是装备库（装备图鉴）的规则表源文件。
//
// 它是装备库**全部规则判据**的真源，而不是 id 白名单：
//   - [max equipment count] 99                      → 普通收录上限
//   - [max equipment count by equipment type]       → `[oath]`, rarity, 上限（誓约类同 rarity 只有 1）
//   - [new peculiar group] 的 5 个 [group]          → 收藏类别 0..4（[set mark]）+ 各自的组 ID / 按钮
//   - [part set index] / [oath group] / [primer group] → 部位套装与两类专用组
//
// 为什么**不用**分享版那种 1,527 条显式收录目录：那是在别的客户端版本上派生的产物。
// 本版本的判据是「minimum level == 115 且 rarity ∈ {2,3,4,6,8}」（见规格 0026-DISJOINTITEM），
// 上限来自本文件；两者都不需要枚举全部可收录 id。
//
// 字段名一律保留源里的写法，不翻译语义；含义未证的值按位置保留。
const EquipmentJournalPath = "contents/2025/equipmentsetjournal/etc/equipmentsetjournal.cos"

// journalRarities 是"可登记装备库"的 rarity 集合。
// 真源两处互相印证：规格 0026 的「minimum level=115 且 rarity 为 2/3/4/6/8 时登记」，
// 以及本文件的 [max equipment count by equipment type] 只列了 2/3/4/6/8 五个誓约条目。
var journalRarities = map[uint32]bool{2: true, 3: true, 4: true, 6: true, 8: true}

// EquipmentJournalRules 是 equipmentsetjournal.cos 的结构化投影。
type EquipmentJournalRules struct {
	Source pvf.ArchiveSnapshot `json:"source"`
	Path   string              `json:"path"`
	SHA256 string              `json:"sha256"`
	Bytes  int                 `json:"bytes"`

	// Maximum 是 [max equipment count]：普通装备的绝对收录份数上限。
	Maximum uint32 `json:"maximum"`
	// MaximumByType 是 [max equipment count by equipment type]：按类型/稀有度收紧的上限。
	MaximumByType []JournalTypeLimit `json:"maximum_by_type,omitempty"`
	// MaxAwakening 是 [max awakening count]。
	MaxAwakening uint32 `json:"max_awakening_count"`

	PartSetIndex               []uint32 `json:"part_set_index,omitempty"`
	IgnoreAmalgamComboboxIndex []uint32 `json:"ignore_amalgam_combobox_index,omitempty"`
	CommonPartSetPointItem     []uint32 `json:"common_part_set_point_item_index,omitempty"`
	CommonPrimerItem           []uint32 `json:"common_primer_item_index,omitempty"`
	SoleEquipmentGroup         []uint32 `json:"sole_equipment_group,omitempty"`
	DefaultGroup               []uint32 `json:"default_group,omitempty"`
	AmalgamationGroup          []uint32 `json:"amalgamation_group,omitempty"`
	OathGroup                  []uint32 `json:"oath_group,omitempty"`
	PrimerGroup                []uint32 `json:"primer_group,omitempty"`
	FirstRegisterItem          []uint32 `json:"first_register_item_index,omitempty"`

	// Categories 是 [new peculiar group] 的 5 个 [group]：收藏类别（[set mark]）0..4。
	// UI 只摆了 4 个标签页（实测覆盖 0/1/3/4），类别 2 有配置但无页面入口 —— 玩家点不到，
	// 所以客户端不会为它发 CMD2264。行为上表现为"这一类永远是空"。
	Categories []JournalCategory `json:"categories,omitempty"`

	// WeaponGroups / PeculiarGroups 是两张大分组表（[info] 块），仅供诊断与后续功能使用，
	// 本批不参与收录判据。
	WeaponGroups   []JournalInfoGroup `json:"weapon_groups,omitempty"`
	PeculiarGroups []JournalInfoGroup `json:"peculiar_groups,omitempty"`

	// DisjointGuide 是 [disjoint guide] 的标量项（分解引导 UI 参数）。
	DisjointGuide map[string]int64 `json:"disjoint_guide,omitempty"`
}

// JournalTypeLimit 是 [max equipment count by equipment type] 的一行：
//
//	`[oath]`, 2, 1
type JournalTypeLimit struct {
	Kind    string `json:"kind"`
	Rarity  uint32 `json:"rarity"`
	Maximum uint32 `json:"maximum"`
}

// JournalCategory 是 [new peculiar group] 里的一个 [group]：一个收藏类别。
type JournalCategory struct {
	Mark    uint32   `json:"mark"`
	Index   uint32   `json:"index"`
	Name    string   `json:"name"`
	Button  string   `json:"button_type"`
	Members []uint32 `json:"members,omitempty"`
}

// JournalInfoGroup 是 [weapon group] / [peculiar group] 里的一个 [info] 块。
type JournalInfoGroup struct {
	Index   uint32   `json:"index"`
	Members []uint32 `json:"members"`
}

// LimitFor 回答「该物品能否登记装备库、上限多少」。三层判据
// （真源：规格 CMD/0026-DISJOINTITEM + 本目录的 [max equipment count] /
// [max equipment count by equipment type]）：
//
//  1. rarity 不在 {2,3,4,6,8} ⇒ **不可登记**（只走普通分解）。
//  2. 源里**显式列出**了 (kind, rarity) ⇒ 用那个上限（本版本只有 `[oath]` 的五个条目 = 1）。
//  3. 其它 ⇒ **回落普通上限**（[max equipment count] = 99）。
//
// ⚠️ 第 3 条一度被我写成"拒绝"，那是**过度收紧**：源里 `[max equipment count by equipment type]`
// **只声明了 `[oath]`**，对 `[weapon]` / `[earring]` / `[shoes]` … 一个字都没写，不构成
// "这些类型不许登记"。拿真实目录（424,216 条）跑一遍才暴露：115 级的 `[earring]` 等被误拒，
// 而那恰恰是"装备库添加"最常见的对象。
func (r EquipmentJournalRules) LimitFor(kind string, rarity uint32) (uint32, bool) {
	if !journalRarities[rarity] {
		return 0, false
	}
	for _, l := range r.MaximumByType {
		if l.Kind == kind && l.Rarity == rarity {
			return l.Maximum, true
		}
	}
	if r.Maximum == 0 {
		return 0, false
	}
	return r.Maximum, true
}

// Category 按 [set mark] 取收藏类别。
func (r EquipmentJournalRules) Category(mark uint32) (JournalCategory, bool) {
	for _, c := range r.Categories {
		if c.Mark == mark {
			return c, true
		}
	}
	return JournalCategory{}, false
}

// LoadEquipmentJournalRules 读入导入物（cmd/equipmentjournalimport 的产物）。
func LoadEquipmentJournalRules(path, source string) (EquipmentJournalRules, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return EquipmentJournalRules{}, e
	}
	var out EquipmentJournalRules
	if e = json.Unmarshal(b, &out); e != nil {
		return EquipmentJournalRules{}, fmt.Errorf("equipment journal rules %s: %w", path, e)
	}
	if source != "" && out.Source.Checksum != "" && out.Source.Checksum != source {
		return EquipmentJournalRules{}, fmt.Errorf("equipment journal rules source mismatch: file=%s loaded=%s",
			out.Source.Checksum, source)
	}
	return out, nil
}

// ImportEquipmentJournalRules 从内层归档读取并解析装备库规则表。
//
// 与其它导入物一致：Source 记归档指纹、SHA256/Bytes 记**原始字节**（ReadRaw），
// 解析用 ReadText 解出的文本。
func ImportEquipmentJournalRules(a *pvf.Archive) (EquipmentJournalRules, error) {
	out := EquipmentJournalRules{Path: EquipmentJournalPath}
	if _, ok := a.FindFile(EquipmentJournalPath); !ok {
		return out, fmt.Errorf("%s is missing from the archive", EquipmentJournalPath)
	}
	raw, e := a.ReadRaw(EquipmentJournalPath)
	if e != nil {
		return out, e
	}
	sum := sha256.Sum256(raw)
	out.SHA256, out.Bytes = hex.EncodeToString(sum[:]), len(raw)
	text, e := a.ReadText(EquipmentJournalPath)
	if e != nil {
		return out, e
	}
	parsed, e := ParseEquipmentJournalRules(text)
	if e != nil {
		return out, e
	}
	parsed.Source = a.Snapshot()
	parsed.Path = EquipmentJournalPath
	parsed.SHA256, parsed.Bytes = out.SHA256, out.Bytes
	if parsed.Maximum == 0 || len(parsed.Categories) == 0 {
		return parsed, fmt.Errorf("equipment journal: 规则表缺关键段（maximum=%d categories=%d）",
			parsed.Maximum, len(parsed.Categories))
	}
	return parsed, nil
}

// journalNode 是 [name] … [/name] 的通用树节点；值行按出现顺序留在 Values 里。
type journalNode struct {
	Name     string
	Head     string
	Values   []string
	Children []*journalNode
}

func sectionTag(s string) (name, head string, open bool, ok bool) {
	if !strings.HasPrefix(s, "[") {
		return "", "", false, false
	}
	i := strings.IndexByte(s, ']')
	if i < 0 {
		return "", "", false, false
	}
	inner := s[1:i]
	if strings.HasPrefix(inner, "/") {
		return strings.TrimSpace(inner[1:]), "", false, true
	}
	return strings.TrimSpace(inner), strings.TrimSpace(s[i+1:]), true, true
}

// parseJournalTree 把这段脚本切成树。
//
// 关键点：源里 `[name] 值` 既可能是**叶子**（`[title] 101036479`、`[set mark] 0`），
// 也可能是**带头的容器**（`[group] 1 … [/group]`）—— 不能按"有没有头"判断。
// 所以先扫一遍收尾标记，**凡是出现过 `[/name]` 的 name 才是容器**，其余一律当叶子。
func parseJournalTree(text string) (*journalNode, error) {
	lines := strings.Split(text, "\n")
	containers := map[string]bool{}
	for _, line := range lines {
		s := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if name, _, open, ok := sectionTag(s); ok && !open {
			containers[name] = true
		}
	}
	root := &journalNode{Name: ""}
	stack := []*journalNode{root}
	for n, line := range lines {
		s := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if s == "" {
			continue
		}
		name, head, open, ok := sectionTag(s)
		if !ok {
			cur := stack[len(stack)-1]
			cur.Values = append(cur.Values, s)
			continue
		}
		if open {
			node := &journalNode{Name: name, Head: head}
			cur := stack[len(stack)-1]
			cur.Children = append(cur.Children, node)
			if containers[name] {
				stack = append(stack, node)
			}
			continue
		}
		if len(stack) == 1 || stack[len(stack)-1].Name != name {
			top := ""
			if len(stack) > 1 {
				top = stack[len(stack)-1].Name
			}
			return nil, fmt.Errorf("equipment journal: line %d closes [%s] inside [%s]", n+1, name, top)
		}
		stack = stack[:len(stack)-1]
	}
	if len(stack) != 1 {
		return nil, fmt.Errorf("equipment journal: %d section(s) left open", len(stack)-1)
	}
	return root, nil
}

func (n *journalNode) child(name string) *journalNode {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func (n *journalNode) children(name string) []*journalNode {
	var out []*journalNode
	for _, c := range n.Children {
		if c.Name == name {
			out = append(out, c)
		}
	}
	return out
}

func journalUint(s string) (uint32, bool) {
	v, e := strconv.ParseUint(strings.TrimSpace(s), 10, 32)
	if e != nil {
		return 0, false
	}
	return uint32(v), true
}

// journalUints 收集一个段里的全部十进制值（值行可能是空格分隔的多列）。
func journalUints(values []string) []uint32 {
	var out []uint32
	for _, v := range values {
		for _, f := range strings.Fields(v) {
			if n, ok := journalUint(f); ok {
				out = append(out, n)
			}
		}
	}
	return out
}

// journalInfoGroups 解析 [weapon group] / [peculiar group] 这种 [info] 列表。
func journalInfoGroups(sec *journalNode) []JournalInfoGroup {
	if sec == nil {
		return nil
	}
	var out []JournalInfoGroup
	for _, info := range sec.children("info") {
		g := JournalInfoGroup{}
		if ix := info.child("index"); ix != nil {
			if n, ok := journalUint(ix.Head); ok {
				g.Index = n
			}
		}
		if l := info.child("list"); l != nil {
			g.Members = journalUints(l.Values)
		}
		out = append(out, g)
	}
	return out
}

// ParseEquipmentJournalRules 解析 equipmentsetjournal.cos 的文本。
func ParseEquipmentJournalRules(text string) (EquipmentJournalRules, error) {
	root, e := parseJournalTree(text)
	if e != nil {
		return EquipmentJournalRules{}, e
	}
	var out EquipmentJournalRules
	out.SHA256 = func() string {
		sum := sha256.Sum256([]byte(text))
		return hex.EncodeToString(sum[:])
	}()
	out.Bytes = len(text)

	if s := root.child("max equipment count"); s != nil {
		n, ok := journalUint(s.Head)
		if !ok {
			return EquipmentJournalRules{}, fmt.Errorf("equipment journal: bad [max equipment count] %q", s.Head)
		}
		out.Maximum = n
	} else {
		return EquipmentJournalRules{}, fmt.Errorf("equipment journal: missing [max equipment count]")
	}
	if s := root.child("max equipment count by equipment type"); s != nil {
		for _, v := range s.Values {
			parts := strings.Split(v, ",")
			if len(parts) < 3 {
				continue
			}
			kind := strings.Trim(strings.TrimSpace(parts[0]), "`")
			rarity, ok1 := journalUint(parts[1])
			maximum, ok2 := journalUint(parts[2])
			if !ok1 || !ok2 {
				continue
			}
			out.MaximumByType = append(out.MaximumByType, JournalTypeLimit{
				Kind: kind, Rarity: rarity, Maximum: maximum,
			})
		}
	}
	if s := root.child("max awakening count"); s != nil {
		if n, ok := journalUint(s.Head); ok {
			out.MaxAwakening = n
		}
	}

	scalar := func(name string, dst *[]uint32) {
		if s := root.child(name); s != nil {
			*dst = journalUints(s.Values)
		}
	}
	scalar("part set index", &out.PartSetIndex)
	scalar("ignore amalgam combobox index", &out.IgnoreAmalgamComboboxIndex)
	scalar("common part set point item index", &out.CommonPartSetPointItem)
	scalar("common primer item index", &out.CommonPrimerItem)
	scalar("sole equipment group", &out.SoleEquipmentGroup)
	scalar("default group", &out.DefaultGroup)
	scalar("amalgamation group", &out.AmalgamationGroup)
	scalar("oath group", &out.OathGroup)
	scalar("primer group", &out.PrimerGroup)
	scalar("first register item index", &out.FirstRegisterItem)

	if sec := root.child("new peculiar group"); sec != nil {
		for _, g := range sec.children("group") {
			var c JournalCategory
			if ix := g.child("index"); ix != nil {
				if n, ok := journalUint(ix.Head); ok {
					c.Index = n
				}
			}
			if mk := g.child("set mark"); mk != nil {
				if n, ok := journalUint(mk.Head); ok {
					c.Mark = n
				}
			}
			if nm := g.child("name"); nm != nil {
				c.Name = nm.Head
			}
			if bt := g.child("button type"); bt != nil {
				c.Button = strings.Trim(bt.Head, "`")
			}
			if l := g.child("list"); l != nil {
				c.Members = journalUints(l.Values)
			}
			out.Categories = append(out.Categories, c)
		}
	}
	out.WeaponGroups = journalInfoGroups(root.child("weapon group"))
	out.PeculiarGroups = journalInfoGroups(root.child("peculiar group"))

	if sec := root.child("disjoint guide"); sec != nil {
		out.DisjointGuide = map[string]int64{}
		for _, c := range sec.Children {
			v, e := strconv.ParseInt(c.Head, 10, 64)
			if e != nil {
				continue
			}
			out.DisjointGuide[c.Name] = v
		}
	}
	return out, nil
}
