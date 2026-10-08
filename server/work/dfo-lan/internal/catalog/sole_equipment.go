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

// 秘宝精度单次提升的口径。**两套并存，由运行期开关选择**（见 internal/inventory/sole.go）：
//
//   - 默认（单机化）：每次在 [SoleQualityGainMin, SoleQualityGainMax]（5..20）均匀取值，
//     到 `[max quality]` 截断。这是本仓库原有的业主口径。
//   - 原版（国服，开关 `-sole-quality-native` / `DFO_SOLE_QUALITY_NATIVE=1`）：
//     保底 +1、按 SoleQualityGreatPercent 触发大成功，且单次不得越过源 `[quality group]`
//     的分段上限（0/25/50/75/100）。
//
// ⚠️ 源里**没有**增量数值表：`[quality ability]` 是"精度 → 属性"的查表，
// `[quality group]` 只给分段边界。所以两套增量口径都来自业主，与**有源**的分段边界分开存放。
const (
	// SoleQualityGainMin/Max 是**单机口径**的单次增量范围（默认启用）。
	SoleQualityGainMin = 5
	SoleQualityGainMax = 20

	// SoleQualityBaseGain 是**原版口径**的保底提升量。
	SoleQualityBaseGain = 1
	// SoleQualityGreatPercent 是**原版口径**的大成功触发概率（百分点），
	// 命中后增量在 [2, 当前分段剩余] 内均匀取值。
	SoleQualityGreatPercent = 25
)

// BandCaps 返回分段上限（升序，末项 = MaxQuality）。
//
// 源 `[quality group]` 是去重升序的扁平边界（`0 25 / 25 50 / 50 75 / 75 100`
// → `0,25,50,75,100`），去掉 0 之后就是每段的上限。
func (i SoleEquipmentInfo) BandCaps() []int {
	caps := make([]int, 0, len(i.Boundaries))
	for _, b := range i.Boundaries {
		if b > 0 && b <= i.MaxQuality {
			caps = append(caps, b)
		}
	}
	if len(caps) == 0 || caps[len(caps)-1] != i.MaxQuality {
		caps = append(caps, i.MaxQuality)
	}
	return caps
}

// BandCap 返回精度 quality 所在分段的封顶值（单次提升不得越过它）。
func (i SoleEquipmentInfo) BandCap(quality int) int {
	for _, c := range i.BandCaps() {
		if quality < c {
			return c
		}
	}
	return i.MaxQuality
}

// BandNode 报告精度是否正好停在**分段节点**上（25/50/75）。
//
// 节点后的下一次精炼必定大成功。末段上限（MaxQuality）不算节点：
// 那已经是终点，不该再精炼。
func (i SoleEquipmentInfo) BandNode(quality int) bool {
	if quality >= i.MaxQuality {
		return false
	}
	for _, c := range i.BandCaps() {
		if quality == c {
			return true
		}
	}
	return false
}

// SoleEquipmentMaterial 是精度提升成本里的一项（Template 0 = 金币）。
type SoleEquipmentMaterial struct {
	Template uint32 `json:"template"`
	Amount   int64  `json:"amount"`
}

// Gold 报告该项是否为金币。
func (m SoleEquipmentMaterial) Gold() bool { return m.Template == 0 }

// SoleEquipmentInfo 是源里一件秘宝的 `[info]`。
type SoleEquipmentInfo struct {
	Template   uint32 `json:"template"`
	MaxQuality int    `json:"max_quality"`
	// Groups 是 `[quality need materials]`：给**成品**加精度的成本表（CMD2288）。
	Groups map[int][]SoleEquipmentMaterial `json:"groups"`
	// CreateGroups 是 `[create need materials]`：把**半成品做成成品**的成本表（CMD2289）。
	// 与 Groups 是两套独立表（源里就分两段写），决定组号的仍是请求里的 selector。
	// 缺这一段 = 这件秘宝没有制作配方（精度仍然可用），所以它是可选的。
	CreateGroups map[int][]SoleEquipmentMaterial `json:"create_groups,omitempty"`
	// CreateMovieTime / CreateWaitTime 是源里 `[create movie time]` / `[create wait time]`
	// 的毫秒数（各件不同：Venus 18000/1200、Nabel 13500/1500、Diregie 18800/2100）。
	//
	// 客户端自己播这段动画，服务端**不消费**它们。解析出来只有一个用途：回包时机的
	// 对照实验 —— 实机 2026-10-02 发现"三件里只有时长最短的那件播了动画"，怀疑是
	// ack 回得太早让客户端判定已完成、直接跳过演出（见 cmd/wireprobe/sole_flow.go）。
	CreateMovieTime int   `json:"create_movie_time,omitempty"`
	CreateWaitTime  int   `json:"create_wait_time,omitempty"`
	Boundaries      []int `json:"boundaries,omitempty"`
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

// CreateMaterials 给出某件秘宝在**指定制作组**下的制作成本（源 `[create need materials]`，CMD2289）。
//
// 与 Materials 是**两套独立表**：Materials 读 `[quality need materials]`（给成品加精度），
// 本函数读 `[create need materials]`（把半成品做成成品）。组号同样由**请求的 selector** 决定
// （protocol.SoleMaterialGroupForSelector），不按精度/进度推 —— 口径仍是"面板显示什么就扣什么"。
//
// 模板不在源 `[infos]` 里、这件秘宝没有这一段、或组号不存在 ⇒ `(nil, false)`：**不猜、不回落**
// （与 Materials 同口径；文档 §4 明确要求）。
func (r SoleEquipmentRules) CreateMaterials(template uint32, groupIndex int) ([]SoleEquipmentMaterial, bool) {
	info, ok := r.Items[template]
	if !ok {
		return nil, false
	}
	items, ok := info.CreateGroups[groupIndex]
	if !ok || len(items) == 0 {
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
	// `[create need materials]` 是**秘宝制作**（CMD2289）的成本表，与精度提升是两套独立表。
	// 源里有些 [info] 没有这一段 —— 那只是这件秘宝没有制作配方（精度仍然可用），
	// 所以**缺段合法**；但一旦声明了就必须严格解析（组号非法、行不成对都报错，不半读）。
	if create := node.child("create need materials"); create != nil {
		info.CreateGroups = map[int][]SoleEquipmentMaterial{}
		for _, group := range create.children("group") {
			groupIndex, ok := journalUint(group.Head)
			if !ok || groupIndex > 255 {
				return info, fmt.Errorf("sole equipment: %d has an unusable create [group] %q", template, group.Head)
			}
			items, e := parseSoleMaterials(group.Values)
			if e != nil {
				return info, fmt.Errorf("sole equipment: %d create group %d: %w", template, groupIndex, e)
			}
			info.CreateGroups[int(groupIndex)] = items
		}
	}
	// 制作演出三件套的时长（客户端自己播，服务端只用于回包时机的对照实验）。
	//
	// ⚠️ 值写在**节标题后面的同一行**（`[create movie time] 18000 18000`），在 PVF 节模型里
	// 就是 `Head`；节**内部**的行（`Values`）是空的 —— 2026-10-02 第一次就栽在这里，
	// 解析出 0 导致整个延迟实验静默失效（探针抓到 movie_time: 0 才发现）。
	if s := node.child("create movie time"); s != nil {
		info.CreateMovieTime = parseSoleFirstMillis([]string{s.Head})
	}
	if s := node.child("create wait time"); s != nil {
		info.CreateWaitTime = parseSoleFirstMillis([]string{s.Head})
	}
	return info, nil
}

// parseSoleFirstMillis 读 `<毫秒>` 或 `<毫秒> <毫秒>` 形式的第一项
// （源里 `[create movie time] 18000 18000` 与 `[create wait time] 1200` 都是这个形状）。
func parseSoleFirstMillis(lines []string) int {
	for _, line := range lines {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if v, err := strconv.ParseInt(f[0], 10, 32); err == nil && v > 0 {
			return int(v)
		}
	}
	return 0
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
