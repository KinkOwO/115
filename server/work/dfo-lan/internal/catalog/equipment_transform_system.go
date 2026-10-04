package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// 装备/融合/晶体变换的费用与返还表（唯一内容真源 = 内层 PVF）。
//
// 源：`etc/115lvability/equipmenttransformsystem.cos`（10,334 B，客户端字符串块
// `Etc/115LvAbility/EquipmentTransformSystem.cos` 自证的加载路径）。
//
// 这个文件同时承载三条链，各有独立的 `[need …]` / `[refund …]` 段：
//
//	装备变换（CMD2259 action=1）  [need materials]        + [refund materials]
//	融合变换                      [need amalgamation …]   + [refund amalgamation …]
//	晶体/誓约变换（CMD2381）      [need primer materials] + [refund primer materials]
//
// ★ 为什么必须直读：CMD2259 的装备变换成本**曾经硬编码在 Go 里**
// （`walletSoulByRarity` + 常量 `transformGoldCost = 50000`）。源里五档金币是
// **25000/30000/35000/40000/50000**，所以旧口径对 rare..epic 一直**多扣**金币
// （只有 primeval 恰好等于 50000）。同一份表还是 CMD2381 的唯一计费来源
// （`[need primer materials]` 没有灵魂项，只有金币 / 巡礼之印两支）。
//
// 列语义由源自身坐实（不是推断）：同文件的 `[material list for ui]` 里
// `[group] 1` 的末项是 `0`（金币占位）、`[group] 2` 的末项是 `10401346`（巡礼之印）；
// `equipmentsetjournal.cos` 的 `[create cost]` 同构（`[cost] 1` = 登记证 + `0 30000`，
// `[cost] 2` = 登记证 + `10401346 6`）。所以：
//
//	[group] 1 → 金币那一支（请求头付款方式序号 1）
//	[group] 2 → 替材料那一支（10401346 = 巡礼之印，序号 2）
const EquipmentTransformSystemPath = "etc/115lvability/equipmenttransformsystem.cos"

// TransformChain 是三条链的名字，用来取各自的表。
type TransformChain string

const (
	TransformChainEquipment    TransformChain = "equipment"
	TransformChainAmalgamation TransformChain = "amalgamation"
	TransformChainPrimer       TransformChain = "primer"
)

// transformRarityNames 把源里的稀有度名映射到装备 `[rarity]` 码值。
//
// 依据是源自身：本文件的 `[refund materials]` 按 rare/unique/legendary/epic/primeval
// 顺序给出的灵魂 10361512/10361513/10361514/10361515/10361516，各自 `.stk` 的
// `[rarity]` 恰为 2/3/6/4/8 —— 名字与码值的绑定由源表互证，不是抄 Go 常量。
var transformRarityNames = map[int32]string{
	2: "rare",
	3: "unique",
	6: "legendary",
	4: "epic",
	8: "primeval",
}

// TransformRarityName 把装备 `[rarity]` 码值翻成源里的稀有度名。
func TransformRarityName(rarity int32) (string, bool) {
	name, ok := transformRarityNames[rarity]
	return name, ok
}

// TransformMaterial 是一行材料：模板 + 数量。
type TransformMaterial struct {
	Template uint32 `json:"template"`
	Count    uint32 `json:"count"`
}

// TransformPaymentOption 是一支付法（源里的 `[group] N`）。
type TransformPaymentOption struct {
	// Number 是 `[group]` 后面的序号（1 起）。请求头的付款方式序号按它取支。
	Number int `json:"number"`
	// Gold 是这一支的金币数（源里的 `0 <amount>` 行；0 = 这一支不要金币）。
	Gold uint32 `json:"gold,omitempty"`
	// Items 是这一支的材料行（不含金币）。
	Items []TransformMaterial `json:"items,omitempty"`
}

// TransformNeedRow 是 `[need …]` 里的一条 `[info]`：一个稀有度 + 它的可选付法。
type TransformNeedRow struct {
	Rarity  string                  `json:"rarity"`
	Options []TransformPaymentOption `json:"options,omitempty"`
}

// TransformRefundRow 是 `[refund …]` 里的一行。
//
//	<等级> `<稀有度>` <分支> [<标签>] <件数> [<模板> <数量>]…
//
// `[refund materials]` / `[refund primer materials]` 是 4 列头（无标签），
// `[refund amalgamation materials]` 多一列标签（源里 3 / 4 / -1，语义未闭环）。
type TransformRefundRow struct {
	Rarity string              `json:"rarity"`
	Branch int                 `json:"branch"`
	Tag    int                 `json:"tag,omitempty"`
	HasTag bool                `json:"has_tag,omitempty"`
	Items  []TransformMaterial `json:"items,omitempty"`
}

// EquipmentTransformSystem 是 `equipmenttransformsystem.cos` 的结构化投影。
type EquipmentTransformSystem struct {
	Source pvf.ArchiveSnapshot `json:"source"`
	Path   string              `json:"path"`
	SHA256 string              `json:"sha256"`
	Bytes  int                 `json:"bytes"`

	EquipmentNeed      []TransformNeedRow   `json:"equipment_need,omitempty"`
	EquipmentRefund    []TransformRefundRow `json:"equipment_refund,omitempty"`
	AmalgamationNeed   []TransformNeedRow   `json:"amalgamation_need,omitempty"`
	AmalgamationRefund []TransformRefundRow `json:"amalgamation_refund,omitempty"`
	PrimerNeed         []TransformNeedRow   `json:"primer_need,omitempty"`
	PrimerRefund       []TransformRefundRow `json:"primer_refund,omitempty"`
}

// Need 取某条链的费用表。
func (s EquipmentTransformSystem) Need(chain TransformChain) []TransformNeedRow {
	switch chain {
	case TransformChainEquipment:
		return s.EquipmentNeed
	case TransformChainAmalgamation:
		return s.AmalgamationNeed
	case TransformChainPrimer:
		return s.PrimerNeed
	}
	return nil
}

// Refund 取某条链的返还表。
func (s EquipmentTransformSystem) Refund(chain TransformChain) []TransformRefundRow {
	switch chain {
	case TransformChainEquipment:
		return s.EquipmentRefund
	case TransformChainAmalgamation:
		return s.AmalgamationRefund
	case TransformChainPrimer:
		return s.PrimerRefund
	}
	return nil
}

// Payment 回答「该稀有度、该付款方式序号」要付什么。
//
// 序号直接来自请求头（2259 的 `[13]` u8 / 2381 的 `[13]` u32，都是 1 起），
// **刻意不做"付不起就换另一支"的回退**：玩家点的是哪支就扣哪支（既有先例：
// 2026-09-29 13:52 那次错扣 35,000 金币）。
func (s EquipmentTransformSystem) Payment(chain TransformChain, rarity string, option int) (TransformPaymentOption, bool) {
	if option < 1 {
		return TransformPaymentOption{}, false
	}
	for _, row := range s.Need(chain) {
		if row.Rarity != rarity {
			continue
		}
		for _, opt := range row.Options {
			if opt.Number == option {
				return opt, true
			}
		}
		return TransformPaymentOption{}, false
	}
	return TransformPaymentOption{}, false
}

// RefundFor 取某稀有度、某分支的返还材料。
func (s EquipmentTransformSystem) RefundFor(chain TransformChain, rarity string, branch int) ([]TransformMaterial, bool) {
	for _, row := range s.Refund(chain) {
		if row.Rarity == rarity && row.Branch == branch {
			return append([]TransformMaterial(nil), row.Items...), true
		}
	}
	return nil, false
}

// ImportEquipmentTransformSystem 从内层归档直读变换费用/返还表。
func ImportEquipmentTransformSystem(a *pvf.Archive) (EquipmentTransformSystem, error) {
	out := EquipmentTransformSystem{Path: EquipmentTransformSystemPath}
	if _, ok := a.FindFile(EquipmentTransformSystemPath); !ok {
		return out, fmt.Errorf("%s is missing from the archive", EquipmentTransformSystemPath)
	}
	raw, e := a.ReadRaw(EquipmentTransformSystemPath)
	if e != nil {
		return out, e
	}
	sum := sha256.Sum256(raw)
	out.SHA256, out.Bytes = hex.EncodeToString(sum[:]), len(raw)
	text, e := a.ReadText(EquipmentTransformSystemPath)
	if e != nil {
		return out, e
	}
	parsed, e := ParseEquipmentTransformSystem(text)
	if e != nil {
		return out, e
	}
	parsed.Source = a.Snapshot()
	parsed.Path = EquipmentTransformSystemPath
	parsed.SHA256, parsed.Bytes = out.SHA256, out.Bytes
	// 三条链的 `[need …]` 是**全部**消费点：缺任何一条都说明读错了文件。
	if len(parsed.EquipmentNeed) == 0 || len(parsed.PrimerNeed) == 0 {
		return parsed, fmt.Errorf("equipment transform system: 缺关键段（equipment=%d primer=%d）",
			len(parsed.EquipmentNeed), len(parsed.PrimerNeed))
	}
	return parsed, nil
}

// transformRarityFromHead 从 `115 \`rare\`` 这类头里取稀有度名。
func transformRarityFromHead(head string) (string, bool) {
	i := strings.IndexByte(head, '`')
	if i < 0 {
		return "", false
	}
	j := strings.IndexByte(head[i+1:], '`')
	if j < 0 {
		return "", false
	}
	name := strings.TrimSpace(head[i+1 : i+1+j])
	if name == "" {
		return "", false
	}
	return name, true
}

// parseTransformOption 解析一个 `[group] N`：值行是 `模板 数量`（模板 0 = 金币）。
func parseTransformOption(g *journalNode) (TransformPaymentOption, error) {
	out := TransformPaymentOption{}
	n, ok := journalUint(g.Head)
	if !ok {
		return out, fmt.Errorf("equipment transform system: bad [group] head %q", g.Head)
	}
	out.Number = int(n)
	for _, line := range g.Values {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return out, fmt.Errorf("equipment transform system: [group] %d row %q is not `template count`",
				out.Number, line)
		}
		tpl, ok1 := journalUint(fields[0])
		amount, ok2 := journalUint(fields[1])
		if !ok1 || !ok2 {
			return out, fmt.Errorf("equipment transform system: [group] %d row %q is not numeric",
				out.Number, line)
		}
		if tpl == 0 {
			out.Gold += amount
			continue
		}
		out.Items = append(out.Items, TransformMaterial{Template: tpl, Count: amount})
	}
	return out, nil
}

// parseTransformNeed 解析一个 `[need …]` 段：`[info]` → `[condition]` + `[cost]`。
func parseTransformNeed(sec *journalNode) ([]TransformNeedRow, error) {
	var out []TransformNeedRow
	for _, info := range sec.children("info") {
		cond := info.child("condition")
		if cond == nil {
			return nil, fmt.Errorf("equipment transform system: [%s] [info] 缺 [condition]", sec.Name)
		}
		rarity, ok := transformRarityFromHead(cond.Head)
		if !ok {
			return nil, fmt.Errorf("equipment transform system: bad [condition] %q", cond.Head)
		}
		cost := info.child("cost")
		if cost == nil {
			return nil, fmt.Errorf("equipment transform system: [%s] %s 缺 [cost]", sec.Name, rarity)
		}
		row := TransformNeedRow{Rarity: rarity}
		for _, g := range cost.children("group") {
			opt, e := parseTransformOption(g)
			if e != nil {
				return nil, e
			}
			row.Options = append(row.Options, opt)
		}
		if len(row.Options) == 0 {
			return nil, fmt.Errorf("equipment transform system: [%s] %s 没有付法", sec.Name, rarity)
		}
		out = append(out, row)
	}
	return out, nil
}

// parseTransformRefund 解析一个 `[refund …]` 段。
//
// 行格式：`<等级> \`<稀有度>\` <分支> [<标签>] <件数> [<模板> <数量>]…`
// 尾部的 `[<模板> <数量>]…` 对数必须与「件数」一致 —— 不一致说明表变了，宁可报错。
func parseTransformRefund(sec *journalNode) ([]TransformRefundRow, error) {
	var out []TransformRefundRow
	for _, line := range sec.Values {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			return nil, fmt.Errorf("equipment transform system: [%s] row %q is too short", sec.Name, line)
		}
		if _, ok := journalUint(fields[0]); !ok {
			return nil, fmt.Errorf("equipment transform system: [%s] row %q: bad level", sec.Name, line)
		}
		rarity, ok := transformRarityFromHead(fields[1])
		if !ok {
			return nil, fmt.Errorf("equipment transform system: [%s] row %q: bad rarity", sec.Name, line)
		}
		branch, ok := journalInt(fields[2])
		if !ok {
			return nil, fmt.Errorf("equipment transform system: [%s] row %q: bad branch", sec.Name, line)
		}
		row := TransformRefundRow{Rarity: rarity, Branch: branch}
		rest := fields[3:]
		// 两种形状：4 列头（branch count pairs）与 5 列头（branch tag count pairs）。
		if count, ok := journalUint(rest[0]); ok && len(rest) == 1+2*int(count) {
			row.Items, _ = transformPairs(rest[1:])
		} else if len(rest) >= 2 {
			tag, ok1 := journalInt(rest[0])
			count, ok2 := journalUint(rest[1])
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("equipment transform system: [%s] row %q: bad tag/count", sec.Name, line)
			}
			row.HasTag, row.Tag = true, tag
			if len(rest) != 2+2*int(count) {
				return nil, fmt.Errorf("equipment transform system: [%s] row %q: %d 件与 %d 个模板不符",
					sec.Name, line, count, (len(rest)-2)/2)
			}
			row.Items, _ = transformPairs(rest[2:])
		} else {
			return nil, fmt.Errorf("equipment transform system: [%s] row %q: bad shape", sec.Name, line)
		}
		out = append(out, row)
	}
	return out, nil
}

// transformPairs 把 `模板 数量 模板 数量 …` 切成材料行。
func transformPairs(fields []string) ([]TransformMaterial, error) {
	var out []TransformMaterial
	for i := 0; i+1 < len(fields); i += 2 {
		tpl, ok1 := journalUint(fields[i])
		amount, ok2 := journalUint(fields[i+1])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("equipment transform system: bad material pair %q %q", fields[i], fields[i+1])
		}
		out = append(out, TransformMaterial{Template: tpl, Count: amount})
	}
	return out, nil
}

// journalInt 解析有符号十进制（源里 `[refund amalgamation materials]` 的标签列有 -1）。
func journalInt(s string) (int, bool) {
	v, e := strconv.Atoi(strings.TrimSpace(s))
	if e != nil {
		return 0, false
	}
	return v, true
}

// ParseEquipmentTransformSystem 解析 `equipmenttransformsystem.cos` 的文本。
func ParseEquipmentTransformSystem(text string) (EquipmentTransformSystem, error) {
	root, e := parseJournalTree(text)
	if e != nil {
		return EquipmentTransformSystem{}, e
	}
	var out EquipmentTransformSystem
	sum := sha256.Sum256([]byte(text))
	out.SHA256, out.Bytes = hex.EncodeToString(sum[:]), len(text)

	need := func(name string) ([]TransformNeedRow, error) {
		sec := root.child(name)
		if sec == nil {
			return nil, fmt.Errorf("equipment transform system: missing [%s]", name)
		}
		return parseTransformNeed(sec)
	}
	refund := func(name string) ([]TransformRefundRow, error) {
		sec := root.child(name)
		if sec == nil {
			return nil, fmt.Errorf("equipment transform system: missing [%s]", name)
		}
		return parseTransformRefund(sec)
	}
	if out.EquipmentNeed, e = need("need materials"); e != nil {
		return out, e
	}
	if out.EquipmentRefund, e = refund("refund materials"); e != nil {
		return out, e
	}
	if out.AmalgamationNeed, e = need("need amalgamation materials"); e != nil {
		return out, e
	}
	if out.AmalgamationRefund, e = refund("refund amalgamation materials"); e != nil {
		return out, e
	}
	if out.PrimerNeed, e = need("need primer materials"); e != nil {
		return out, e
	}
	if out.PrimerRefund, e = refund("refund primer materials"); e != nil {
		return out, e
	}
	if len(out.EquipmentNeed) == 0 || len(out.PrimerNeed) == 0 {
		return out, fmt.Errorf("equipment transform system: 关键段为空（equipment=%d primer=%d）",
			len(out.EquipmentNeed), len(out.PrimerNeed))
	}
	return out, nil
}
