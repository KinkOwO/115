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

// 秘宝精度提升（CMD2288 `ENUM_CMDPACKET_SOLE_EQUIPMENT_QUALITY`）的**直读**规则源。
//
// 源（唯一内容真源 = 内层 PVF，不经任何导出 JSON）：
//
//	etc/115lvability/soleequipmentsystem.cos
//
// 结构（实测文本，三件秘宝同一形状）：
//
//	[infos]
//	 [info]
//	  [item index] 100354181
//	  [max quality] 100
//	  [quality need materials]
//	   [group] 0
//	    10404807 8                      ← <模板> <数量>，模板 0 = 金币
//	    10404719 20
//	    10401346 800
//	   [/group]
//	   [group] 1
//	    10404807 8
//	    10404719 20
//	    0 4000000
//	   [/group]
//	  [/quality need materials]
//	  [quality group]                     ← 精度分档边界（0/25/50/75/100）
//	   0 25
//	   25 50
//	   50 75
//	   75 100
//	  [/quality group]
//	  [quality ability] …                ← 精度 → 属性，服务端不消费（客户端算面板）
//	  [create need materials] …          ← 秘宝“制作”，与精度提升是另一套（单独立项）
//	 [/info]
//	[/infos]
//
// 与装备调适（equipment_awakening.go）同一口径：**不读任何 configs/*.json**，
// 解析失败直接报错，绝不静默回落到内置表。
const SoleEquipmentSystemPath = "etc/115lvability/soleequipmentsystem.cos"

// SoleQualityRecordOffset 是「精度」在 181B 装备实例行里的偏移。
//
// 与消费侧 internal/character/fame.go 是**同一映射**（那里按
// `rules.SoleQuality[item.Template][item.Record[172]]` 取名望值），
// 所以服务端唯一的精度真源就是这一格，不另设镜像字段（避免双源不一致）。
const SoleQualityRecordOffset = 172

// 单次精度提升量的范围。
//
// ⚠️ **源里没有这张表**（`[quality ability]` 是精度→属性值，不含单次增量），
// 这里采用业主提供的**实机口径 5..20**；到 `[max quality]` 处按上限截断（截断时回填实际增量）。
const (
	SoleQualityGainMin = 5
	SoleQualityGainMax = 20
)

// SoleEquipmentMaterial 是精度提升成本里的一项（Template 0 = 金币）。
type SoleEquipmentMaterial struct {
	Template uint32 `json:"template"`
	Amount   int64  `json:"amount"`
}

// Gold 报告该项是否为金币。
func (m SoleEquipmentMaterial) Gold() bool { return m.Template == 0 }

// SoleEquipmentInfo 是源里一件秘宝的 `[info]`。
type SoleEquipmentInfo struct {
	Template   uint32                   `json:"template"`
	MaxQuality int                      `json:"max_quality"`
	Groups     map[int][]SoleEquipmentMaterial `json:"groups"`
	Boundaries []int                    `json:"boundaries,omitempty"`
}

// SoleEquipmentRules 是秘宝精度提升的完整直读规则表。
type SoleEquipmentRules struct {
	Source pvf.ArchiveSnapshot `json:"source"`
	Path   string              `json:"path"`
	SHA256 string              `json:"sha256"`
	Bytes  int                 `json:"bytes"`

	Items map[uint32]SoleEquipmentInfo `json:"items"`
}

// Templates 返回规则表里登记的全部秘宝模板（升序，便于日志与测试）。
func (r SoleEquipmentRules) Templates() []uint32 {
	out := make([]uint32, 0, len(r.Items))
	for template := range r.Items {
		out = append(out, template)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Info 查某件秘宝的规则。
func (r SoleEquipmentRules) Info(template uint32) (SoleEquipmentInfo, bool) {
	info, ok := r.Items[template]
	return info, ok
}

// Materials 给出某件秘宝在**指定材料组**下的精度提升成本。
//
// ⚠️ 组号**不是**由精度推导的（这一点 2026-10-02 实机纠正过）：客户端面板上第三项材料
// 可以在「实物（`10401346` 之类）」与「金币」之间切换，请求明文 `+16..19` 的 selector
// 就是那次选择 —— 实测 selector 1 用组 0（实物）、selector 0 用组 1（金币）。
// 映射放在 protocol.SoleQualityRequest.MaterialGroup，这里只按组号取成本。
func (r SoleEquipmentRules) Materials(template uint32, groupIndex int) ([]SoleEquipmentMaterial, bool) {
	info, ok := r.Items[template]
	if !ok {
		return nil, false
	}
	items, ok := info.Groups[groupIndex]
	if !ok {
		return nil, false
	}
	return items, true
}

// ImportSoleEquipmentRules 从内层归档读取并解析秘宝精度规则表。
func ImportSoleEquipmentRules(a *pvf.Archive) (SoleEquipmentRules, error) {
	out := SoleEquipmentRules{Path: SoleEquipmentSystemPath}
	if a == nil {
		return out, fmt.Errorf("sole equipment: archive is nil")
	}
	if _, ok := a.FindFile(SoleEquipmentSystemPath); !ok {
		return out, fmt.Errorf("%s is missing from the archive", SoleEquipmentSystemPath)
	}
	raw, e := a.ReadRaw(SoleEquipmentSystemPath)
	if e != nil {
		return out, e
	}
	sum := sha256.Sum256(raw)
	out.SHA256, out.Bytes = hex.EncodeToString(sum[:]), len(raw)
	text, e := a.ReadText(SoleEquipmentSystemPath)
	if e != nil {
		return out, e
	}
	parsed, e := ParseSoleEquipmentRules(text)
	if e != nil {
		return out, e
	}
	parsed.Source = a.Snapshot()
	parsed.Path = out.Path
	parsed.SHA256, parsed.Bytes = out.SHA256, out.Bytes
	return parsed, nil
}

// ParseSoleEquipmentRules 解析 `soleequipmentsystem.cos` 的文本。
//
// 复用装备库那套通用树解析（`parseJournalTree`）：`[infos]`/`[info]`/`[quality need materials]`/
// `[group]` 都是**带尾标记的容器**，`[item index]`/`[max quality]` 是叶子。
func ParseSoleEquipmentRules(text string) (SoleEquipmentRules, error) {
	root, e := parseJournalTree(text)
	if e != nil {
		return SoleEquipmentRules{}, e
	}
	out := SoleEquipmentRules{Path: SoleEquipmentSystemPath, Items: map[uint32]SoleEquipmentInfo{}}
	infos := root.child("infos")
	if infos == nil {
		return out, fmt.Errorf("sole equipment: [infos] is missing")
	}
	for _, node := range infos.children("info") {
		info, e := parseSoleInfo(node)
		if e != nil {
			return out, e
		}
		if _, dup := out.Items[info.Template]; dup {
			return out, fmt.Errorf("sole equipment: duplicate [item index] %d", info.Template)
		}
		out.Items[info.Template] = info
	}
	if len(out.Items) == 0 {
		return out, fmt.Errorf("sole equipment: [infos] has no [info]")
	}
	return out, nil
}

func parseSoleInfo(node *journalNode) (SoleEquipmentInfo, error) {
	info := SoleEquipmentInfo{Groups: map[int][]SoleEquipmentMaterial{}}
	index := node.child("item index")
	if index == nil {
		return info, fmt.Errorf("sole equipment: [info] without [item index]")
	}
	template, ok := journalUint(index.Head)
	if !ok || template == 0 {
		return info, fmt.Errorf("sole equipment: [item index] %q is not a usable template", index.Head)
	}
	info.Template = template

	maxQuality := node.child("max quality")
	if maxQuality == nil {
		return info, fmt.Errorf("sole equipment: %d has no [max quality]", template)
	}
	cap, ok := journalUint(maxQuality.Head)
	if !ok || cap == 0 || cap > 255 {
		return info, fmt.Errorf("sole equipment: %d [max quality] %q is not a usable cap", template, maxQuality.Head)
	}
	info.MaxQuality = int(cap)

	need := node.child("quality need materials")
	if need == nil {
		return info, fmt.Errorf("sole equipment: %d has no [quality need materials]", template)
	}
	for _, group := range need.children("group") {
		groupIndex, ok := journalUint(group.Head)
		if !ok || groupIndex > 255 {
			return info, fmt.Errorf("sole equipment: %d has an unusable material [group] %q", template, group.Head)
		}
		items, e := parseSoleMaterials(group.Values)
		if e != nil {
			return info, fmt.Errorf("sole equipment: %d group %d: %w", template, groupIndex, e)
		}
		info.Groups[int(groupIndex)] = items
	}
	if len(info.Groups) == 0 {
		return info, fmt.Errorf("sole equipment: %d has no material [group]", template)
	}
	// 两组都必须存在：精度跨过分界点时要能换付法，缺一组会让"精度 50 以后"无成本可算。
	for _, required := range []int{0, 1} {
		if _, ok := info.Groups[required]; !ok {
			return info, fmt.Errorf("sole equipment: %d has no material group %d", template, required)
		}
	}
	// `[quality group]` 是精度分档边界（0/25/50/75/100）。**服务端不用它算成本**：
	// 材料组由客户端请求的 selector 决定（见 Materials 的注释），这里只留档便于诊断。
	if qg := node.child("quality group"); qg != nil {
		info.Boundaries, _ = parseSoleBoundaries(qg.Values)
	}
	return info, nil
}

// parseSoleMaterials 解析 `<模板> <数量>` 的行集合（模板 0 = 金币）。
func parseSoleMaterials(lines []string) ([]SoleEquipmentMaterial, error) {
	var out []SoleEquipmentMaterial
	for _, line := range lines {
		f := strings.Fields(line)
		if len(f) != 2 {
			return nil, fmt.Errorf("row %q is not <template> <amount>", line)
		}
		template, e1 := strconv.ParseUint(f[0], 10, 32)
		amount, e2 := strconv.ParseInt(f[1], 10, 64)
		if e1 != nil || e2 != nil || amount <= 0 {
			return nil, fmt.Errorf("row %q is not a positive template/amount pair", line)
		}
		out = append(out, SoleEquipmentMaterial{Template: uint32(template), Amount: amount})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty material group")
	}
	return out, nil
}

// parseSoleBoundaries 解析 `[quality group]` 的分档边界（每行一对，展开去重后升序）。
func parseSoleBoundaries(lines []string) ([]int, error) {
	seen := map[int]bool{}
	for _, line := range lines {
		for _, field := range strings.Fields(line) {
			value, e := strconv.Atoi(field)
			if e != nil || value < 0 || value > 255 {
				return nil, fmt.Errorf("boundary row %q is not numeric", line)
			}
			seen[value] = true
		}
	}
	out := make([]int, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Ints(out)
	return out, nil
}
