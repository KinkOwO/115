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

// 秘宝精度提升（CMD2288 `ENUM_CMDPACKET_SOLE_EQUIPMENT_QUALITY`）的业务实现。
//
// 规则**全部来自内层 PVF 的直读投影**（`catalog.SoleEquipmentRules`，
// 源 = etc/115lvability/soleequipmentsystem.cos），本文件只保留两个源里没有的口径：
// 单次增量范围（业主实机口径，见 catalog.SoleQualityGainMin/Max）。
//
// 状态落点：装备实例行（181B）的 **+172** 就是「精度」——
// internal/character/fame.go 早就在消费它：
//
//	entry.Quality = rules.SoleQuality[item.Template][item.Record[172]]
//
// 所以服务端**不另设镜像字段**（BagEquipment 不加 SoleQuality），避免双源不一致；
// 读写都只经过 `Record[172]`。
//
// 材料：源 `[quality need materials]` 的 `[group] 0` 是实物、`[group] 1` 把最后一项换成
// **金币**（模板 0）。分界精度见 catalog.SoleMaterialGroupSwitch（< 50 → 组 0）。
// 其中 `10361515` 之类的模板同时是账号共享材料仓库的固定格，所以整笔事务走
// CommitAccountMaterialEvent（与装备调适同一套幂等/事务设施）。

// SoleQualityReceipt 是一次精度提升的回执（落进角色存档，供幂等重放取回）。
type SoleQualityReceipt struct {
	Request protocol.SoleQualityRequest `json:"request"`

	Template uint32 `json:"template"`
	// Container / Slot 原样回显给客户端（回包要用）。
	Container byte   `json:"container"`
	Slot      uint16 `json:"slot"`

	QualityBefore int `json:"quality_before"`
	QualityAfter  int `json:"quality_after"`
	QualityGain   int `json:"quality_gain"`
	MaxQuality    int `json:"max_quality"`
	GroupIndex    int `json:"group_index"`

	Spent []SoleQualitySpend `json:"spent,omitempty"`
	Gold  uint32             `json:"gold"`

	// RecordHealed：读装备时发现实例行 `+2` 模板字段与 Template 不一致，已自愈（同调适）。
	RecordHealed bool `json:"record_healed,omitempty"`
}

// SoleQualitySpend 是本次消耗的一项（Template 0 = 金币）。
type SoleQualitySpend struct {
	Template    uint32 `json:"template"`
	Amount      uint32 `json:"amount"`
	FromStorage bool   `json:"from_storage,omitempty"`
	// StorageAmount 是**实际**从账号共享材料仓库扣掉的数量（0 = 全从背包扣）。
	// 有了它，回包刷新判断与日志才能区分"真的动了仓库"与"只是这个模板属于仓库品类"。
	StorageAmount uint32 `json:"storage_amount,omitempty"`
}

const soleStateField = "last_sole_quality"

type storedSoleQuality struct {
	Key string `json:"key"`
	SoleQualityReceipt
}

// 秘宝精度规则的运行期注入点：启动期由 cmd/wireprobe 用直读投影安装。
var soleEquipmentRules *catalog.SoleEquipmentRules

// SetSoleEquipmentRules 安装秘宝精度规则表（直读投影）。
func SetSoleEquipmentRules(rules *catalog.SoleEquipmentRules) { soleEquipmentRules = rules }

// SoleEquipmentRulesLoaded 报告规则表是否已装载。
func SoleEquipmentRulesLoaded() bool { return soleEquipmentRules != nil }

// SoleEquipmentRulesSource 返回已装载规则表的源指纹（诊断/门禁用）。
func SoleEquipmentRulesSource() string {
	if soleEquipmentRules == nil {
		return ""
	}
	return soleEquipmentRules.Source.Checksum
}

// SoleQualityDiagnostics 汇总当前规则表的规模（启动日志用）。
func SoleQualityDiagnostics() string {
	if soleEquipmentRules == nil {
		return "rules=absent"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "items=%d", len(soleEquipmentRules.Items))
	for _, template := range soleEquipmentRules.Templates() {
		info, _ := soleEquipmentRules.Info(template)
		fmt.Fprintf(&b, " %d(max=%d,groups=%d)", template, info.MaxQuality, len(info.Groups))
	}
	return b.String()
}

// SoleQualityGainMaxOf 返回该秘宝的精度上限（0 表示规则里没有这件秘宝）。
func SoleQualityGainMaxOf(template uint32) int {
	if soleEquipmentRules == nil {
		return 0
	}
	info, ok := soleEquipmentRules.Info(template)
	if !ok {
		return 0
	}
	return info.MaxQuality
}

// soleQualityGain 掷一次单次增量（源里没有这张表，口径见 catalog.SoleQualityGainMin/Max）。
func soleQualityGain() int {
	lo, hi := catalog.SoleQualityGainMin, catalog.SoleQualityGainMax
	if hi <= lo {
		return lo
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(hi-lo+1)))
	if err != nil {
		return lo
	}
	return lo + int(n.Int64())
}

// SoleQualityPlan 是**规则侧**对一次精度提升的判定结果：选中的规则块、付法、成本行与
// 精度推进结果。不含任何存档改动，便于单独验证（见 sole_test.go）。
type SoleQualityPlan struct {
	Info catalog.SoleEquipmentInfo

	GroupIndex int
	Cost       []catalog.SoleEquipmentMaterial

	QualityBefore int
	QualityAfter  int
	Gain          int
}

// PlanSoleQuality 用直读规则表判定一次精度提升（不触碰存档）。
//
// 判据全部来自源 + 请求：
//   - `[item index]` 决定这件装备是不是秘宝（不在表里 ⇒ 拒绝，不猜）；
//   - 当前精度 = 实例行 +172，`[max quality]` 是上限（到达上限即拒绝）；
//   - `groupIndex` 由**请求的 selector** 决定（protocol.SoleMaterialGroupForSelector）——
//     不是按精度推的，见 `Rules.Materials` 的注释；
//   - `gain` 由调用者掷（源里没有增量表），到上限按 `[max quality]` 截断并回填实际增量。
func PlanSoleQuality(rules *catalog.SoleEquipmentRules, template uint32, quality, gain, groupIndex int) (SoleQualityPlan, error) {
	var plan SoleQualityPlan
	if rules == nil {
		return plan, Refuse(RefusalUnsupported, "秘宝精度规则未装载")
	}
	info, ok := rules.Info(template)
	if !ok {
		return plan, Refuse(RefusalUnsupported, "模板 %d 不是源 [infos] 里登记的秘宝", template)
	}
	if quality < 0 || quality > info.MaxQuality {
		return plan, Refuse(RefusalLimit, "秘宝精度 %d 超出源声明的 0..%d", quality, info.MaxQuality)
	}
	// 上限检查必须在扣料之前：满精度再提升没有任何源依据（客户端也会禁按钮）。
	if quality >= info.MaxQuality {
		return plan, Refuse(RefusalLimit, "秘宝精度已达上限 %d", info.MaxQuality)
	}
	cost, ok := rules.Materials(template, groupIndex)
	if !ok {
		return plan, Refuse(RefusalMaterials, "秘宝 %d 没有材料组 %d 的成本表", template, groupIndex)
	}
	if gain <= 0 {
		gain = soleQualityGain()
	}
	after := quality + gain
	if after > info.MaxQuality {
		after = info.MaxQuality
	}
	if after <= quality {
		return plan, Refuse(RefusalLimit, "秘宝精度已达上限 %d", info.MaxQuality)
	}
	return SoleQualityPlan{Info: info, GroupIndex: groupIndex, Cost: cost,
		QualityBefore: quality, QualityAfter: after, Gain: after - quality}, nil
}

// ApplySoleQuality 执行一次秘宝精度提升（CMD2288）。
//
// 幂等：同一 (角色, key) 的重放不会二次扣料，回执从存档字段取回。
func (s *WearService) ApplySoleQuality(ctx context.Context, role storage.Character, key string, r protocol.SoleQualityRequest) (storage.Character, SoleQualityReceipt, error) {
	var out SoleQualityReceipt
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, out, fmt.Errorf("秘宝精度需要有效装备目录及角色存档")
	}
	if soleEquipmentRules == nil {
		return role, out, fmt.Errorf("秘宝精度规则未装载")
	}
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() && role.ConfigVersion != s.Catalog.Source.Checksum {
		return role, out, fmt.Errorf("秘宝精度需要有效装备目录及角色存档")
	}
	applied := false
	saved, _, _, err := s.Store.CommitAccountMaterialEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "sole-quality-v1",
		func(current storage.Character, counts json.RawMessage) (json.RawMessage, json.RawMessage, error) {
			next, nextCounts, receipt, e := s.applySoleQuality(current, counts, key, r)
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
		stored, e := readSoleQualityReceipt(saved.State, key)
		if e != nil {
			return role, out, e
		}
		out = stored
	}
	saved.WireID = role.WireID
	return saved, out, nil
}

func (s *WearService) applySoleQuality(role storage.Character, counts json.RawMessage, key string, r protocol.SoleQualityRequest) (json.RawMessage, json.RawMessage, SoleQualityReceipt, error) {
	var out SoleQualityReceipt
	fail := func(kind RefusalKind, format string, args ...any) (json.RawMessage, json.RawMessage, SoleQualityReceipt, error) {
		return nil, nil, out, Refuse(kind, format, args...)
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
	if r.Container == 3 {
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
		return fail(RefusalItems, "秘宝不在指定的所属角色容器（容器 %d 槽 %d）", r.Container, r.Slot)
	}
	gear := items[gearIndex]
	healedRecord := false
	// 自愈复用装备调适那套：实例行 `+2` 的模板字段与 Template 不一致会让
	// ValidateRecord 挡住这件装备的**所有**后续操作（详见 healAwakeningRecord 的注释）。
	if healed, changed := healAwakeningRecord(gear); changed {
		gear, healedRecord = healed, true
	}
	if err := gear.ValidateRecord(); err != nil {
		return nil, nil, out, err
	}

	quality := 0
	if len(gear.Record) > catalog.SoleQualityRecordOffset {
		quality = int(gear.Record[catalog.SoleQualityRecordOffset])
	}
	// 材料组来自**请求的 selector**（客户端面板第三项材料的切换按钮），不是按精度推的。
	groupIndex, ok := protocol.SoleMaterialGroupForSelector(r.Selector)
	if !ok {
		return fail(RefusalUnsupported, "秘宝精度请求的材料选择 %d 不在已知的组表里（实机只见过 0/1）", r.Selector)
	}
	plan, err := PlanSoleQuality(soleEquipmentRules, gear.Template, quality, 0, groupIndex)
	if err != nil {
		return nil, nil, out, err
	}

	// 校验并扣除成本（金币 + 材料）。与装备调适同一口径：
	// 秘宝请求里没有槽位信息，材料来源由服务端定 —— 账号共享材料仓库优先，
	// 不足部分兜底扣背包（刚发到背包、尚未被清扫搬进仓库的堆叠也必须能付）。
	spent := make([]SoleQualitySpend, 0, len(plan.Cost))
	gold := uint32(0)
	for _, item := range plan.Cost {
		if item.Gold() {
			if item.Amount > math.MaxUint32 {
				return fail(RefusalGold, "金币成本超出范围")
			}
			gold += uint32(item.Amount)
			continue
		}
		amount := uint32(item.Amount)
		storageCount := uint32(0)
		if _, _, ok := AccountMaterialTarget(item.Template); ok {
			storageCount = materials.Count(item.Template)
		}
		bagCount := uint32(0)
		for _, row := range bag.Items {
			if row.Template == item.Template {
				bagCount += row.Amount
			}
		}
		if storageCount+bagCount < amount {
			return fail(RefusalMaterials, "秘宝精度材料 %d 不足（需要 %d，账号材料仓库 %d + 背包 %d）",
				item.Template, amount, storageCount, bagCount)
		}
		remaining := amount
		storageTaken := uint32(0)
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
			storageTaken = take
		}
		if remaining > 0 {
			next, _, e := consumeBagTemplate(bag, item.Template, remaining)
			if e != nil {
				return nil, nil, out, e
			}
			bag = next
		}
		// FromStorage/StorageAmount 记录**实际**从账号材料仓库扣的数量（原来只标"是不是
		// 账号材料"，即使全从背包扣也算 true，会让回包刷新判断与日志都失真）。
		spent = append(spent, SoleQualitySpend{Template: item.Template, Amount: amount,
			FromStorage: storageTaken > 0, StorageAmount: storageTaken})
	}
	if gold > 0 {
		if bag.Gold < gold {
			return fail(RefusalGold, "金币不足（需要 %d，持有 %d）", gold, bag.Gold)
		}
		bag.Gold -= gold
		spent = append(spent, SoleQualitySpend{Template: 0, Amount: gold})
	}

	// 落库：只动精度那一格，其余实例字节（强化/增幅/附魔/阶段…）原样保留。
	row := EquipmentRow(gear)
	row[catalog.SoleQualityRecordOffset] = byte(plan.QualityAfter)
	gear.Record = append([]byte(nil), row[:]...)
	if err := gear.ValidateRecord(); err != nil {
		return nil, nil, out, err
	}
	items = append([]BagEquipment(nil), items...)
	items[gearIndex] = gear
	if r.Container == 3 {
		bag.Worn = items
	} else {
		bag.Equipment = items
	}

	out = SoleQualityReceipt{
		Request: r, Template: gear.Template, Container: r.Container, Slot: r.Slot,
		QualityBefore: plan.QualityBefore, QualityAfter: plan.QualityAfter, QualityGain: plan.Gain,
		MaxQuality: plan.Info.MaxQuality, GroupIndex: plan.GroupIndex,
		Spent: spent, Gold: bag.Gold, RecordHealed: healedRecord,
	}

	next, err := SaveBag(role.State, bag)
	if err != nil {
		return nil, nil, out, err
	}
	next, err = writeSoleQualityReceipt(next, key, out)
	if err != nil {
		return nil, nil, out, err
	}
	nextCounts, err := materials.Save()
	if err != nil {
		return nil, nil, out, err
	}
	return next, nextCounts, out, nil
}

func readSoleQualityReceipt(state json.RawMessage, key string) (SoleQualityReceipt, error) {
	var out SoleQualityReceipt
	if len(state) == 0 {
		return out, fmt.Errorf("秘宝精度回执丢失（存档为空）")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(state, &fields); err != nil {
		return out, err
	}
	raw, ok := fields[soleStateField]
	if !ok {
		return out, fmt.Errorf("秘宝精度回执不存在")
	}
	var stored storedSoleQuality
	if err := json.Unmarshal(raw, &stored); err != nil {
		return out, err
	}
	if stored.Key != key {
		return out, fmt.Errorf("秘宝精度回执与请求不匹配")
	}
	return stored.SoleQualityReceipt, nil
}

func writeSoleQualityReceipt(state json.RawMessage, key string, receipt SoleQualityReceipt) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(state, &fields); err != nil {
		return nil, err
	}
	blob, err := json.Marshal(storedSoleQuality{Key: key, SoleQualityReceipt: receipt})
	if err != nil {
		return nil, err
	}
	fields[soleStateField] = blob
	return json.Marshal(fields)
}
