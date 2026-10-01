package inventory

import (
	"context"
	"crypto/rand"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strings"
)

// 装备调适（CMD2258 `ENUM_CMDPACKET_EQUIPMENT_AWAKENING`）的业务实现。
//
// 规则**全部来自内层 PVF 的直读投影**（`catalog.EquipmentAwakeningRules`，
// 源 = etc/115lvability/equipmentawakeningoptionsystem.cos），本文件不含任何数值常量。
//
// 状态落点（与客户端同一口径，见 internal/character/fame.go 的同一处注释）：
//
//	装备实例行（181B）的 +170 就是调适阶段（客户端 14576D8B0 把 170 映射到实例 285，
//	名望侧 internal/character/fame.go 早就按 rules.Awakening[template][Record[170]] 取值）。
//
// 阶段语义（源里 [condition] + [upgrade result]）：
//
//	阶段 < [max awakening]：目标模板不变，只把 +170 加一；
//	阶段 == [max awakening]：必须命中 [upgrade result] 的候选目标 ⇒ 换模板、+170 归零。
//
// 材料：源里 [need materials] 的每个 [group] 就是一套付法（本版本 1 = 只用基础材料、
// 2 = 额外消耗催化剂），玩家在客户端选择的组号由请求的 MaterialGroup 带进来。
// `10361512..10361516` / `10415191` 同时是账号共享材料仓库的固定格
// （account_materials.go 的账户材料表），所以整笔事务走 CommitAccountMaterialEvent。

// AwakeningReceipt 是一次调适的回执（落进角色存档，供幂等重放取回）。
type AwakeningReceipt struct {
	Request protocol.EquipmentAwakeningRequest `json:"request"`

	Level  int `json:"level"`
	Rarity int `json:"rarity"`

	GroupIndex int `json:"group_index"`
	Stage      int `json:"stage"`
	Rate       int `json:"rate"`
	Result     byte

	StageBefore    int    `json:"stage_before"`
	StageAfter     int    `json:"stage_after"`
	TemplateBefore uint32 `json:"template_before"`
	TemplateAfter  uint32 `json:"template_after"`
	Upgraded       bool   `json:"upgraded"`

	Spent []AwakeningSpend `json:"spent,omitempty"`
	Gold  uint32           `json:"gold"`

	Equipment      BagEquipment `json:"equipment"`
	EquipmentSpace byte         `json:"equipment_space"`
	EquipmentSlot  uint16       `json:"equipment_slot"`
}

// AwakeningSpend 是本次消耗的一项（Template 0 = 金币）。
type AwakeningSpend struct {
	Template    uint32 `json:"template"`
	Amount      uint32 `json:"amount"`
	FromStorage bool   `json:"from_storage,omitempty"`
}

const awakeningStateField = "last_equipment_awakening"

type storedAwakening struct {
	Key string `json:"key"`
	AwakeningReceipt
}

// 调适规则的运行期注入点：启动期由 cmd/wireprobe 用直读投影安装。
var awakeningRules *catalog.EquipmentAwakeningRules

// SetEquipmentAwakeningRules 安装装备调适规则表（直读投影）。
func SetEquipmentAwakeningRules(rules *catalog.EquipmentAwakeningRules) { awakeningRules = rules }

// EquipmentAwakeningRulesLoaded 报告规则表是否已装载。
func EquipmentAwakeningRulesLoaded() bool { return awakeningRules != nil }

// EquipmentAwakeningRulesSource 返回已装载规则表的源指纹（诊断/门禁用）。
func EquipmentAwakeningRulesSource() string {
	if awakeningRules == nil {
		return ""
	}
	return awakeningRules.Source.Checksum
}

// ApplyEquipmentAwakening 执行一次装备调适（CMD2258 mode=0）。
//
// 幂等：同一 (角色, key) 的重放不会二次扣料，回执从存档字段取回。
func (s *WearService) ApplyEquipmentAwakening(ctx context.Context, role storage.Character, key string, r protocol.EquipmentAwakeningRequest) (storage.Character, AwakeningReceipt, error) {
	var out AwakeningReceipt
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, out, fmt.Errorf("装备调适需要有效装备目录及角色存档")
	}
	if awakeningRules == nil {
		return role, out, fmt.Errorf("装备调适规则未装载")
	}
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() && role.ConfigVersion != s.Catalog.Source.Checksum {
		return role, out, fmt.Errorf("装备调适需要有效装备目录及角色存档")
	}
	applied := false
	saved, _, _, err := s.Store.CommitAccountMaterialEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "equipment-awakening-v1",
		func(current storage.Character, counts json.RawMessage) (json.RawMessage, json.RawMessage, error) {
			next, nextCounts, receipt, e := s.applyAwakening(current, counts, key, r)
			if e != nil {
				return nil, nil, e
			}
			out = receipt
			applied = true
			return next, nextCounts, nil
		})
	if err != nil {
		return role, out, err
	}
	if !applied {
		stored, e := readAwakeningReceipt(saved.State, key)
		if e != nil {
			return role, out, e
		}
		out = stored
	}
	saved.WireID = role.WireID
	return saved, out, nil
}

func (s *WearService) applyAwakening(role storage.Character, counts json.RawMessage, key string, r protocol.EquipmentAwakeningRequest) (json.RawMessage, json.RawMessage, AwakeningReceipt, error) {
	var out AwakeningReceipt
	fail := func(kind RefusalKind, format string, args ...any) (json.RawMessage, json.RawMessage, AwakeningReceipt, error) {
		return nil, nil, out, Refuse(kind, format, args...)
	}
	rules := awakeningRules
	if r.Mode != 0 {
		return fail(RefusalUnsupported, "调适只支持 mode=0（初始化/返还未实现：源 [refund materials] 需另开一单）")
	}
	bag, err := ReadBag(role.State)
	if err != nil {
		return nil, nil, out, err
	}
	materials, err := ReadAccountMaterials(counts)
	if err != nil {
		return nil, nil, out, err
	}

	items := bag.Equipment
	if r.Space == 3 {
		items = bag.Worn
	}
	gearIndex := -1
	for i, gear := range items {
		if gear.Slot == r.Slot {
			gearIndex = i
			break
		}
	}
	if gearIndex < 0 {
		return fail(RefusalItems, "调适目标不在指定的所属角色槽位（空间 %d 槽 %d）", r.Space, r.Slot)
	}
	gear := items[gearIndex]
	if err := gear.ValidateRecord(); err != nil {
		return nil, nil, out, err
	}
	d, err := s.Catalog.Definition(gear.Template)
	if err != nil {
		return nil, nil, out, err
	}
	level, ok := singleInt(d, "[minimum level]")
	if !ok {
		return fail(RefusalUnsupported, "无法核对装备最低等级")
	}
	rarity, ok := singleInt(d, "[rarity]")
	if !ok {
		return fail(RefusalUnsupported, "无法核对装备品质")
	}
	// 当前阶段 = 实例行 +170（客户端 14576D8B0 的同一映射）。
	stage := 0
	if len(gear.Record) > awakeningStageOffset {
		stage = int(gear.Record[awakeningStageOffset])
	}
	plan, err := PlanAwakening(rules, gear.Template, int(level), int(rarity), stage, int(r.MaterialGroup), r.Target)
	if err != nil {
		return nil, nil, out, err
	}
	cost := plan.Cost
	newStage, newTemplate, upgraded := plan.StageAfter, plan.TemplateAfter, plan.Upgraded
	beforeTemplate, rate := plan.TemplateBefore, plan.Rate

	// 校验并扣除成本（金币 + 材料）。
	//
	// 调适请求里**没有槽位**（只有材料组），所以材料来源由服务端定：
	// 账号共享材料仓库优先，不足部分兜底扣背包 —— 因为 `SweepAccountMaterials`
	// 只会在清扫时机（选角/拾取/信件/分解）把背包里的账号材料搬进仓库，
	// 刚发到背包、尚未清扫的堆叠也必须能用来付。
	spent := make([]AwakeningSpend, 0, len(cost.Items))
	gold := uint32(0)
	for _, item := range cost.Items {
		if item.Gold() {
			if item.Amount > math.MaxUint32 {
				return fail(RefusalGold, "金币成本超出范围")
			}
			gold += item.Amount
			continue
		}
		storageCount := uint32(0)
		fromStorage := false
		if _, _, ok := AccountMaterialTarget(item.Template); ok {
			storageCount = materials.Count(item.Template)
			fromStorage = true
		}
		bagCount := uint32(0)
		for _, row := range bag.Items {
			if row.Template == item.Template {
				bagCount += row.Amount
			}
		}
		if storageCount+bagCount < item.Amount {
			return fail(RefusalMaterials, "调适材料 %d 不足（需要 %d，账号仓库 %d + 背包 %d）",
				item.Template, item.Amount, storageCount, bagCount)
		}
		remaining := item.Amount
		if storageCount > 0 {
			take := storageCount
			if take > remaining {
				take = remaining
			}
			next, _, e := materials.Spend(item.Template, take)
			if e != nil {
				return nil, nil, out, e
			}
			materials = next
			remaining -= take
		}
		if remaining > 0 {
			next, _, e := consumeBagTemplate(bag, item.Template, remaining)
			if e != nil {
				return nil, nil, out, e
			}
			bag = next
		}
		spent = append(spent, AwakeningSpend{Template: item.Template, Amount: item.Amount, FromStorage: fromStorage})
	}
	if gold > 0 {
		if bag.Gold < gold {
			return fail(RefusalGold, "金币不足（需要 %d，持有 %d）", gold, bag.Gold)
		}
		bag.Gold -= gold
		spent = append(spent, AwakeningSpend{Template: catalog.EquipmentAwakeningGoldTemplate, Amount: gold})
	}

	// 落库：只动必要字段（阶段字节；升品时连模板一起换），其余实例字节原样保留。
	row := EquipmentRow(gear)
	row[awakeningStageOffset] = byte(newStage)
	gear.Template = newTemplate
	gear.Record = append([]byte(nil), row[:]...)
	items = append([]BagEquipment(nil), items...)
	items[gearIndex] = gear
	if r.Space == 3 {
		bag.Worn = items
	} else {
		bag.Equipment = items
	}

	out = AwakeningReceipt{
		Request: r, Level: int(level), Rarity: int(rarity), GroupIndex: plan.Group.Index,
		Stage: stage, Rate: rate, Result: 0,
		StageBefore: stage, StageAfter: newStage,
		TemplateBefore: beforeTemplate, TemplateAfter: newTemplate, Upgraded: upgraded,
		Spent: spent, Gold: bag.Gold,
		Equipment: gear, EquipmentSpace: r.Space, EquipmentSlot: r.Slot,
	}

	next, err := SaveBag(role.State, bag)
	if err != nil {
		return nil, nil, out, err
	}
	next, err = writeAwakeningReceipt(next, key, out)
	if err != nil {
		return nil, nil, out, err
	}
	nextCounts, err := materials.Save()
	if err != nil {
		return nil, nil, out, err
	}
	return next, nextCounts, out, nil
}

// awakeningStageOffset 是装备实例行里的调适阶段字节（0xAA = 170）。
const awakeningStageOffset = 170

// consumeBagTemplate 从背包里按模板扣 amount 个（跨多个堆叠槽，扣空即移除该行）。
func consumeBagTemplate(b Bag, template, amount uint32) (Bag, uint32, error) {
	if amount == 0 {
		return b, 0, fmt.Errorf("invalid bag material amount")
	}
	rows := append([]BagItem(nil), b.Items...)
	remaining := amount
	for i := 0; i < len(rows) && remaining > 0; {
		row := rows[i]
		if row.Template != template {
			i++
			continue
		}
		take := row.Amount
		if take > remaining {
			take = remaining
		}
		remaining -= take
		if row.Amount == take {
			rows = append(rows[:i:i], rows[i+1:]...)
			continue
		}
		rows[i].Amount = row.Amount - take
		i++
	}
	if remaining != 0 {
		return b, 0, Refuse(RefusalMaterials, "调适材料 %d 在背包里不足", template)
	}
	b.Items = rows
	return b, amount - remaining, nil
}

func readAwakeningReceipt(state json.RawMessage, key string) (AwakeningReceipt, error) {
	var out AwakeningReceipt
	if len(state) == 0 {
		return out, fmt.Errorf("装备调适回执丢失（存档为空）")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(state, &fields); err != nil {
		return out, err
	}
	raw, ok := fields[awakeningStateField]
	if !ok {
		return out, fmt.Errorf("装备调适回执不存在")
	}
	var stored storedAwakening
	if err := json.Unmarshal(raw, &stored); err != nil {
		return out, err
	}
	if stored.Key != key {
		return out, fmt.Errorf("装备调适回执与请求不匹配")
	}
	return stored.AwakeningReceipt, nil
}

func writeAwakeningReceipt(state json.RawMessage, key string, receipt AwakeningReceipt) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(state, &fields); err != nil {
		return nil, err
	}
	blob, err := json.Marshal(storedAwakening{Key: key, AwakeningReceipt: receipt})
	if err != nil {
		return nil, err
	}
	fields[awakeningStateField] = blob
	return json.Marshal(fields)
}

// AwakeningRefusalCode 把拒绝原因映射成客户端 CMD2258 应答里的 u16 结果码。
//
// 客户端 sub_140B899B0 只在**状态字节为 0（失败）**时才读这个码，并用它选提示文案
// （1/3/119/217 → 各自的物品提示；其余走 sub_146ADFC80 的消息表，未登记 = 无文案）。
// 目前源的拒绝原因还没有对应的原生文案证据，统一回 0（面板会复位，玩家看到的是"操作未生效"），
// 真实原因写在服务端 `equipment_awakening_refused` 事件里。
func AwakeningRefusalCode(err error) uint16 {
	switch RefusalOf(err) {
	case RefusalMaterials, RefusalGold, RefusalItems, RefusalLimit, RefusalEquipment, RefusalUnsupported:
		return 0
	default:
		return 0
	}
}

// AwakeningDiagnostics 汇总当前规则表的规模（启动日志用）。
func AwakeningDiagnostics() string {
	if awakeningRules == nil {
		return "rules=absent"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "max=%d infos=%d upgrade_templates=%d", awakeningRules.MaxLevel, len(awakeningRules.Infos), len(awakeningRules.Templates()))
	return b.String()
}

// rollAwakening 用源的百分比成功率掷骰（本版本恒 100，保留给取证后的失败分支）。
func rollAwakening(rate int) (bool, error) {
	if rate >= 100 {
		return true, nil
	}
	if rate <= 0 {
		return false, nil
	}
	roll, err := rand.Int(rand.Reader, big.NewInt(100))
	if err != nil {
		return false, err
	}
	return roll.Int64() < int64(rate), nil
}

// AwakeningPlan 是**规则侧**对一次调适的判定结果：选中的规则块、付法、成本行与
// 阶段/模板的推进结果。不含任何存档改动，便于单独验证（见 equipment_awakening_test.go）。
type AwakeningPlan struct {
	Info  catalog.EquipmentAwakeningInfo
	Group catalog.EquipmentAwakeningCostGroup
	Cost  catalog.EquipmentAwakeningRow
	Rate  int

	StageBefore, StageAfter       int
	TemplateBefore, TemplateAfter uint32
	Upgraded                      bool
}

// PlanAwakening 用直读规则表判定一次调适（阶段推进 / 升品），不触碰存档。
//
// 判据全部来自源：
//   - `[condition]` 用 (等级, 品质名, 阶段) 选块，阶段 = 实例行 +170；
//   - `[need materials]` 用请求里的材料组号选付法，用阶段选成本行；
//   - 阶段 < `[max awakening]` ⇒ 只推进阶段、模板必须不变；
//   - 阶段 == 上限 ⇒ 目标必须是该模板 `[upgrade result]` 的候选之一，成功后模板换、阶段归零。
func PlanAwakening(rules *catalog.EquipmentAwakeningRules, template uint32, level, rarity, stage, groupIndex int, target uint32) (AwakeningPlan, error) {
	var plan AwakeningPlan
	if rules == nil {
		return plan, Refuse(RefusalUnsupported, "装备调适规则未装载")
	}
	rarityName, ok := catalog.EquipmentAwakeningRarityName(rarity)
	if !ok {
		return plan, Refuse(RefusalUnsupported, "装备品质 %d 不在调适规则的品质表内", rarity)
	}
	info, ok := rules.Info(level, rarityName, stage)
	if !ok {
		return plan, Refuse(RefusalLimit, "该装备在阶段 %d 没有调适规则（源：等级 %d、品质 %s；可用阶段 %v）",
			stage, level, rarityName, rules.StagesFor(level, rarityName))
	}
	group, ok := info.Group(groupIndex)
	if !ok {
		return plan, Refuse(RefusalMaterials, "材料组 %d 不在本阶段的付法里（可用 %v）", groupIndex, info.GroupIndexes())
	}
	cost, ok := group.Row(stage)
	if !ok {
		return plan, Refuse(RefusalLimit, "材料组 %d 没有阶段 %d 的成本行（源里有 %v）", group.Index, stage, group.Stages())
	}
	rate, ok := info.Rate(stage)
	if !ok {
		return plan, Refuse(RefusalUnsupported, "阶段 %d 没有成功率声明", stage)
	}
	// 本版本 21 组策略全是 100%；非满成功率需要先拿到客户端的结果展示证据，
	// 否则失败分支（扣料但阶段不动）无法与客户端表现对齐。
	if rate != 100 {
		return plan, Refuse(RefusalUnsupported, "阶段 %d 的成功率是 %d%%，非满成功率尚未取证", stage, rate)
	}
	plan = AwakeningPlan{Info: info, Group: group, Cost: cost, Rate: rate,
		StageBefore: stage, TemplateBefore: template}

	if stage < rules.MaxLevel {
		if target != protocol.EquipmentAwakeningNoTarget && target != template {
			return plan, Refuse(RefusalUnsupported, "阶段 %d 不换模板：请求带了目标 %d（当前 %d）", stage, target, template)
		}
		plan.StageAfter, plan.TemplateAfter = stage+1, template
		return plan, nil
	}
	upgrade, ok := info.Upgrade(template)
	if !ok {
		return plan, Refuse(RefusalLimit, "该装备在阶段 %d 没有升品目标（源 [upgrade result] 无此模板）", stage)
	}
	if len(upgrade.Targets) == 0 {
		return plan, Refuse(RefusalLimit, "该装备在阶段 %d 的升品候选为空", stage)
	}
	if target == protocol.EquipmentAwakeningNoTarget || target == 0 {
		return plan, Refuse(RefusalUnsupported, "阶段 %d 必须指定升品目标（候选 %v）", stage, upgrade.Targets)
	}
	for _, candidate := range upgrade.Targets {
		if candidate == target {
			plan.StageAfter, plan.TemplateAfter, plan.Upgraded = 0, target, true
			return plan, nil
		}
	}
	return plan, Refuse(RefusalUnsupported, "目标 %d 不在阶段 %d 的升品候选 %v 里", target, stage, upgrade.Targets)
}
