package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// 装备库「装备生成 / 制作」的成本表 —— 同一份源的 `[create cost]` 段。
//
//	contents/2025/equipmentsetjournal/etc/equipmentsetjournal.cos
//	    → configs/equipment-create-cost.generated.json
//
// 语义（源 + 实机「生成单个部位」窗口）：
//
//	一组 [group] = 一个「套装档」，[item index] 列出该档可以生成的全部装备；
//	[costs] 里是**可选成本**（[cost] 1 / [cost] 2 是两种付法，任选其一即可）；
//	每行 `<模板> <数量>`，其中**模板 0 表示金币**。
//
// 实机对照（2026-09-29）：组 1 的 cost1 = `10361513`×1 + 金币 30000、
// cost2 = `10361513`×1 + `10401346`×6；玩家看到的「变换确认」窗底部成本 35,000
// 与组 2（r6 档）的 35000 同量级 —— 说明这张表就是那两个窗口背后的真源。

// CreateCostGoldTemplate 是成本行里代表**金币**的模板号（源里写 0）。
const CreateCostGoldTemplate uint32 = 0

// EquipmentCreateCost 是 `[create cost]` 段的结构化投影。
type EquipmentCreateCost struct {
	Source pvf.ArchiveSnapshot `json:"source"`
	Path   string              `json:"path"`
	SHA256 string              `json:"sha256"`
	Bytes  int                 `json:"bytes"`

	Groups []CreateCostGroup `json:"groups"`
}

// CreateCostGroup 是一个套装档：能生成哪些装备、以及可选成本。
type CreateCostGroup struct {
	Index int                `json:"index"`
	Items []uint32           `json:"items"`
	Costs []CreateCostOption `json:"costs"`
}

// CreateCostOption 是一种付法（源里 `[cost] N`）。
type CreateCostOption struct {
	Number int              `json:"number"`
	Pairs  []CreateCostItem `json:"pairs"`
}

// CreateCostItem 是一行成本。Template == CreateCostGoldTemplate(0) 时 Amount 是金币数。
type CreateCostItem struct {
	Template uint32 `json:"template"`
	Amount   uint32 `json:"amount"`
}

// Gold 报告这一行是不是金币。
func (i CreateCostItem) Gold() bool { return i.Template == CreateCostGoldTemplate }

// LoadEquipmentCreateCost 读导入产物，并校验它确实来自当前归档。
func LoadEquipmentCreateCost(path, source string) (EquipmentCreateCost, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return EquipmentCreateCost{}, e
	}
	var out EquipmentCreateCost
	if e = json.Unmarshal(b, &out); e != nil {
		return EquipmentCreateCost{}, fmt.Errorf("equipment create cost %s: %w", path, e)
	}
	if source != "" && out.Source.Checksum != "" && out.Source.Checksum != source {
		return EquipmentCreateCost{}, fmt.Errorf("equipment create cost source mismatch: file=%s loaded=%s",
			out.Source.Checksum, source)
	}
	return out, nil
}

// ImportEquipmentCreateCost 从内层归档读取并解析 `[create cost]`。
func ImportEquipmentCreateCost(a *pvf.Archive) (EquipmentCreateCost, error) {
	out := EquipmentCreateCost{Path: EquipmentJournalPath}
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
	groups, e := ParseEquipmentCreateCost(text)
	if e != nil {
		return out, e
	}
	out.Groups = groups
	out.Source = a.Snapshot()
	return out, nil
}

// ParseEquipmentCreateCost 解析 `[create cost]` 段。
//
// 复用装备库那套通用树解析（`parseJournalTree`）：
// 源里 `[group] N … [/group]`、`[costs]/[cost]`、`[item index]` 都是**带尾标记的容器**，
// 不能按"有没有头"去猜。
func ParseEquipmentCreateCost(text string) ([]CreateCostGroup, error) {
	root, e := parseJournalTree(text)
	if e != nil {
		return nil, e
	}
	sec := root.child("create cost")
	if sec == nil {
		return nil, fmt.Errorf("equipment create cost: [create cost] section is missing")
	}
	var out []CreateCostGroup
	for _, g := range sec.children("group") {
		grp := CreateCostGroup{}
		// ⚠️ 与 `[new peculiar group]` 不同：这里的 `[group]` **没有头**，
		// 序号写在同级的 `[index] N` 子节点里。按"头"读会全部得到 0。
		if idx := g.child("index"); idx != nil {
			if v, ok := journalUint(idx.Head); ok {
				grp.Index = int(v)
			}
		}
		if items := g.child("item index"); items != nil {
			grp.Items = journalUints(items.Values)
		}
		if costs := g.child("costs"); costs != nil {
			for _, c := range costs.children("cost") {
				opt := CreateCostOption{}
				if n, ok := journalUint(c.Head); ok {
					opt.Number = int(n)
				}
				for _, line := range c.Values {
					f := strings.Fields(line)
					if len(f) < 2 {
						continue
					}
					tpl, ok1 := journalUint(f[0])
					amt, ok2 := journalUint(f[1])
					if !ok1 || !ok2 {
						continue
					}
					opt.Pairs = append(opt.Pairs, CreateCostItem{Template: tpl, Amount: amt})
				}
				if len(opt.Pairs) > 0 {
					grp.Costs = append(grp.Costs, opt)
				}
			}
		}
		if len(grp.Items) == 0 {
			return nil, fmt.Errorf("equipment create cost: group %d has no [item index]", grp.Index)
		}
		out = append(out, grp)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("equipment create cost: [create cost] has no [group]")
	}
	for _, g := range out {
		if len(g.Costs) == 0 {
			return nil, fmt.Errorf("equipment create cost: group %d has no usable [cost] rows", g.Index)
		}
	}
	return out, nil
}

// GroupFor 找出某个装备模板属于哪个套装档（即"要生成它得付哪一档的成本"）。
//
// 源里同一模板可能同时出现在多个档（例如 Scourge 后缀件在 r4 档里另有条目），
// 但**不同档的成本结构不同**；这里返回**第一个命中**并让调用方按需再筛。
func (c EquipmentCreateCost) GroupFor(template uint32) (CreateCostGroup, bool) {
	if template == 0 {
		return CreateCostGroup{}, false
	}
	for _, g := range c.Groups {
		for _, t := range g.Items {
			if t == template {
				return g, true
			}
		}
	}
	return CreateCostGroup{}, false
}

// Option 按 [create cost] 里 [cost] 行的**序号（1 起）**取一支付法。
//
// 这个序号就是 CMD2259 请求头 `[13]`（`EquipmentCraftRequest.PayOption`）——
// 实机 2026-09-29：`[13]=1` 那次扣的是金币、`[13]=2` 那次玩家点的是巡礼之印那支。
func (g CreateCostGroup) Option(number int) (CreateCostOption, bool) {
	for _, c := range g.Costs {
		if c.Number == number {
			return c, true
		}
	}
	return CreateCostOption{}, false
}

// Numbers 列出本档所有付法序号（报错/诊断用，升序）。
func (g CreateCostGroup) Numbers() []int {
	out := make([]int, 0, len(g.Costs))
	for _, c := range g.Costs {
		out = append(out, c.Number)
	}
	sort.Ints(out)
	return out
}

// Templates 汇总全表出现过的装备模板，供诊断/校验用。
func (c EquipmentCreateCost) Templates() []uint32 {
	seen := map[uint32]bool{}
	var out []uint32
	for _, g := range c.Groups {
		for _, t := range g.Items {
			if t == 0 || seen[t] {
				continue
			}
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}
