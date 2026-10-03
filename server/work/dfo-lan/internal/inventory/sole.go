package inventory

import (
	"crypto/rand"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strings"
)

// 秘宝精度提升（CMD2288 `ENUM_CMDPACKET_SOLE_EQUIPMENT_QUALITY`）的业务实现。
//
// 规则**全部来自内层 PVF 的直读投影**（`catalog.SoleEquipmentRules`，
// 源 = etc/115lvability/soleequipmentsystem.cos）。分段边界（`[quality group]`）是**有源**的；
// 源里缺的只有"单次涨多少"这一项口径，见 catalog.SoleQualityBaseGain / SoleQualityGreatPercent。
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
// **金币**（模板 0）。**组号由请求的 selector 决定**（玩家面板第三项的切换按钮），
// 与当前精度无关 —— 见 protocol.SoleMaterialGroupForSelector。
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
		fmt.Fprintf(&b, " %d(max=%d,groups=%d,create=%d)", template, info.MaxQuality, len(info.Groups), len(info.CreateGroups))
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

// 秘宝精度单次提升有**两套口径**，由运行期开关选择（业主 2026-10-02 要求，两套并存）：
//
//   - 默认（单机化，开关关闭）：每次在 catalog.SoleQualityGainMin..Max（5..20）均匀取值，
//     到 `[max quality]` 截断。这是本仓库原有的口径。
//   - 原版（国服，开关打开）：全程只涨不掉；每次保底 +1，25% 概率大成功（[2, 当前分段剩余]），
//     单次不得越过源 `[quality group]` 的分段上限；于是 24/49/74/99 必然只走到节点，
//     而 25/50/75 之后必定大成功。
//
// 入口：`-sole-quality-native` / `DFO_SOLE_QUALITY_NATIVE=1`（默认关，见 cmd/wireprobe/main.go）。
var soleQualityNative bool

// SetSoleQualityNative 切换秘宝精度的结算口径（true = 原版/国服那套）。
func SetSoleQualityNative(on bool) { soleQualityNative = on }

// SoleQualityNative 报告当前是否使用原版（国服）口径。
func SoleQualityNative() bool { return soleQualityNative }

// soleQualityGain 掷一次单次增量（两套口径见上）。
func soleQualityGain(info catalog.SoleEquipmentInfo, quality int) int {
	if !soleQualityNative {
		// 单机口径：5..20 均匀；到 [max quality] 的截断由 PlanSoleQuality 负责。
		lo, hi := catalog.SoleQualityGainMin, catalog.SoleQualityGainMax
		if hi <= lo {
			return lo
		}
		return lo + randBelow(hi-lo+1)
	}
	// 原版（国服）口径：保底 +1，节点后必暴击，大成功在 [2, 当前分段剩余]。
	room := info.BandCap(quality) - quality
	if room <= 0 {
		return 0
	}
	great := info.BandNode(quality) || randBelow(100) < catalog.SoleQualityGreatPercent
	if !great {
		return catalog.SoleQualityBaseGain
	}
	if room == 1 {
		// 只差 1 点：大成功也只能吃 1（封顶优先，绝不跨段）。
		return 1
	}
	return 2 + randBelow(room-1) // [2, room]
}

// randBelow 返回 [0, n) 的均匀随机数；n <= 0 时返回 0。
func randBelow(n int) int {
	if n <= 0 {
		return 0
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0
	}
	return int(v.Int64())
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
		gain = soleQualityGain(info, quality)
	}
	// **封顶分两套口径**：原版按**当前分段上限**截断（不许跨段，这样 24→25、49→50
	// 这类临界值不可能被跳过）；单机口径按 `[max quality]` 截断（保持本仓库原有行为）。
	limit := info.MaxQuality
	if soleQualityNative {
		limit = info.BandCap(quality)
	}
	if quality+gain > limit {
		gain = limit - quality
	}
	after := quality + gain
	if after <= quality {
		return plan, Refuse(RefusalLimit, "秘宝精度已达上限 %d", info.MaxQuality)
	}
	return SoleQualityPlan{Info: info, GroupIndex: groupIndex, Cost: cost,
		QualityBefore: quality, QualityAfter: after, Gain: after - quality}, nil
}

// ValidateSoleQuality checks the runtime prerequisites before workflow
// starts the account-material transaction.
func (s *WearService) ValidateSoleQuality(role Role) error {
	if s == nil || s.Catalog == nil {
		return fmt.Errorf("秘宝精度需要有效装备目录及角色存档")
	}
	if soleEquipmentRules == nil {
		return fmt.Errorf("秘宝精度规则未装载")
	}
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() && role.ConfigVersion != s.Catalog.Source.Checksum {
		return fmt.Errorf("秘宝精度需要有效装备目录及角色存档")
	}
	return nil
}

// PrepareSoleQuality computes the updated character and account-material
// states. The workflow owns persistence and replay handling.
func (s *WearService) PrepareSoleQuality(role Role, counts json.RawMessage, key string, r protocol.SoleQualityRequest) (json.RawMessage, json.RawMessage, SoleQualityReceipt, error) {
	return s.applySoleQuality(role, counts, key, r)
}

func (s *WearService) applySoleQuality(role Role, counts json.RawMessage, key string, r protocol.SoleQualityRequest) (json.RawMessage, json.RawMessage, SoleQualityReceipt, error) {
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

// ReadSoleQualityReceipt 取回存在存档里的回执（workflow 重放路径用）。
func ReadSoleQualityReceipt(state json.RawMessage, key string) (SoleQualityReceipt, error) {
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

// 秘宝制作（CMD2289 `ENUM_CMDPACKET_SOLE_EQUIPMENT_CREATE`）的业务实现。
//
// 与精度提升（CMD2288）同族、共用同一套账号材料事务设施，但**内容真源是另一段**：
// 成本读源 `[create need materials]`（catalog.SoleEquipmentRules.CreateGroups），语义是
// "把一件半成品做成成品秘宝"。成品走**背包装备槽** —— 请求里不带容器/槽位，这与精度提升
// 不同：后者是对已有的一件升精度，前者是**新出一件**（用户诉求 ③：成品要落到背包里）。
//
// 扣料口径与精度提升完全一致：账号共享材料仓库优先，不足部分兜底扣背包（刚发到背包、
// 还没被清扫搬进仓库的堆叠也必须能付）。成本里的 `模板 0` 是金币（与源里 `0 100000000`
// 那一行同义，见 catalog.SoleEquipmentMaterial.Gold）。

// SoleCreateReceipt 是一次秘宝制作的回执（落进角色存档，供幂等重放取回）。
type SoleCreateReceipt struct {
	Request protocol.SoleCreateRequest `json:"request"`

	Template   uint32 `json:"template"`
	GroupIndex int    `json:"group_index"`
	// Slot 是成品落进背包装备槽的槽位（回包要刷新这一行）。
	Slot uint16 `json:"slot"`

	Spent []SoleCreateSpend `json:"spent,omitempty"`
	Gold  uint32            `json:"gold"`

	// MovieTime / WaitTime 是源里该件秘宝的制作演出时长（毫秒；客户端自己播，服务端不消费）。
	// 挂在这里只是为了回包时机的对照实验：ack 若远早于动画，客户端会判定"已完成"跳过演出
	// （实机 2026-10-02：三件里只有时长最短的那件播了）。见 cmd/wireprobe/sole_flow.go。
	MovieTime int `json:"movie_time,omitempty"`
	WaitTime  int `json:"wait_time,omitempty"`
}

// SoleCreateSpend 是本次消耗的一项（Template 0 = 金币）。
type SoleCreateSpend struct {
	Template uint32 `json:"template"`
	Amount   uint32 `json:"amount"`
	// FromStorage / StorageAmount：**实际**从账号共享材料仓库扣掉的数量（0 = 全从背包扣）。
	// 与精度提升同一口径 —— 只标"这个模板属于账号材料品类"会让回包刷新判断与日志都失真。
	FromStorage   bool   `json:"from_storage,omitempty"`
	StorageAmount uint32 `json:"storage_amount,omitempty"`
}

const soleCreateStateField = "last_sole_create"

type storedSoleCreate struct {
	Key string `json:"key"`
	SoleCreateReceipt
}

// SoleCreatePlan 是**规则侧**对一次制作的判定结果：选中的规则块与成本行。
// 不含任何存档改动，便于单独验证（见 sole_test.go）。
type SoleCreatePlan struct {
	Info       catalog.SoleEquipmentInfo
	GroupIndex int
	Cost       []catalog.SoleEquipmentMaterial
}

// PlanSoleCreate 用直读规则表判定一次秘宝制作（不触碰存档）。
//
// 判据全部来自源 + 请求：
//   - `[item index]` 决定这个模板是不是源里登记的秘宝（不在表里 ⇒ 拒绝，不猜）；
//   - `[create need materials]` 决定成本；**没有这一段** ⇒ 这件秘宝没有制作配方（拒绝）；
//   - 组号由**请求的 selector** 决定（protocol.SoleMaterialGroupForSelector），与精度/进度无关。
func PlanSoleCreate(rules *catalog.SoleEquipmentRules, template uint32, groupIndex int) (SoleCreatePlan, error) {
	var plan SoleCreatePlan
	if rules == nil {
		return plan, Refuse(RefusalUnsupported, "秘宝制作规则未装载")
	}
	info, ok := rules.Info(template)
	if !ok {
		return plan, Refuse(RefusalUnsupported, "模板 %d 不是源 [infos] 里登记的秘宝", template)
	}
	cost, ok := rules.CreateMaterials(template, groupIndex)
	if !ok {
		return plan, Refuse(RefusalMaterials, "秘宝 %d 没有制作组 %d 的成本表", template, groupIndex)
	}
	return SoleCreatePlan{Info: info, GroupIndex: groupIndex, Cost: cost}, nil
}

// ValidateSoleCreate checks the runtime prerequisites before the workflow starts
// the account-material transaction.
func (s *WearService) ValidateSoleCreate(role Role) error {
	if s == nil || s.Catalog == nil {
		return fmt.Errorf("秘宝制作需要有效装备目录及角色存档")
	}
	if soleEquipmentRules == nil {
		return fmt.Errorf("秘宝制作规则未装载")
	}
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() && role.ConfigVersion != s.Catalog.Source.Checksum {
		return fmt.Errorf("秘宝制作需要有效装备目录及角色存档")
	}
	return nil
}

// PrepareSoleCreate computes the updated character and account-material states.
// The workflow owns persistence and replay handling.
func (s *WearService) PrepareSoleCreate(role Role, counts json.RawMessage, key string, r protocol.SoleCreateRequest) (json.RawMessage, json.RawMessage, SoleCreateReceipt, error) {
	return s.applySoleCreate(role, counts, key, r)
}

func (s *WearService) applySoleCreate(role Role, counts json.RawMessage, key string, r protocol.SoleCreateRequest) (json.RawMessage, json.RawMessage, SoleCreateReceipt, error) {
	var out SoleCreateReceipt
	fail := func(kind RefusalKind, format string, args ...any) (json.RawMessage, json.RawMessage, SoleCreateReceipt, error) {
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
	// 制作组来自**请求的 selector**（与精度提升同一个映射函数、同一条口径）。
	groupIndex, ok := protocol.SoleMaterialGroupForSelector(r.Selector)
	if !ok {
		return fail(RefusalUnsupported, "秘宝制作请求的材料选择 %d 不在已知的组表里（实机只见过 0/1）", r.Selector)
	}
	plan, err := PlanSoleCreate(soleEquipmentRules, r.Template, groupIndex)
	if err != nil {
		return nil, nil, out, err
	}

	// 校验并扣除成本（材料 + 金币）：账号共享材料仓库优先，不足兜底扣背包。
	spent := make([]SoleCreateSpend, 0, len(plan.Cost))
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
			return fail(RefusalMaterials, "秘宝制作材料 %d 不足（需要 %d，账号材料仓库 %d + 背包 %d）",
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
		spent = append(spent, SoleCreateSpend{Template: item.Template, Amount: amount,
			FromStorage: storageTaken > 0, StorageAmount: storageTaken})
	}
	if gold > 0 {
		if bag.Gold < gold {
			return fail(RefusalGold, "金币不足（需要 %d，持有 %d）", gold, bag.Gold)
		}
		bag.Gold -= gold
		spent = append(spent, SoleCreateSpend{Template: 0, Amount: gold})
	}

	// 成品落袋：请求里不带容器/槽位，成品按装备规则进**背包装备槽**
	//（AddEquipment 自己找空槽、按装备源取耐久）。
	bag, placed, err := bag.AddEquipment(s.Catalog, s.BagRules.EquipmentSlots, r.Template, 1)
	if err != nil {
		return nil, nil, out, err
	}
	if len(placed) == 0 {
		return fail(RefusalItems, "背包没有空装备槽，成品秘宝无处安放")
	}

	out = SoleCreateReceipt{Request: r, Template: r.Template, GroupIndex: plan.GroupIndex,
		Slot: placed[0], Spent: spent, Gold: bag.Gold,
		MovieTime: plan.Info.CreateMovieTime, WaitTime: plan.Info.CreateWaitTime}

	next, err := SaveBag(role.State, bag)
	if err != nil {
		return nil, nil, out, err
	}
	next, err = writeSoleCreateReceipt(next, key, out)
	if err != nil {
		return nil, nil, out, err
	}
	nextCounts, err := materials.Save()
	if err != nil {
		return nil, nil, out, err
	}
	return next, nextCounts, out, nil
}

// ReadSoleCreateReceipt 取回存在存档里的回执（workflow 重放路径用）。
func ReadSoleCreateReceipt(state json.RawMessage, key string) (SoleCreateReceipt, error) {
	var out SoleCreateReceipt
	if len(state) == 0 {
		return out, fmt.Errorf("秘宝制作回执丢失（存档为空）")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(state, &fields); err != nil {
		return out, err
	}
	raw, ok := fields[soleCreateStateField]
	if !ok {
		return out, fmt.Errorf("秘宝制作回执不存在")
	}
	var stored storedSoleCreate
	if err := json.Unmarshal(raw, &stored); err != nil {
		return out, err
	}
	if stored.Key != key {
		return out, fmt.Errorf("秘宝制作回执与请求不匹配")
	}
	return stored.SoleCreateReceipt, nil
}

func writeSoleCreateReceipt(state json.RawMessage, key string, receipt SoleCreateReceipt) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(state, &fields); err != nil {
		return nil, err
	}
	blob, err := json.Marshal(storedSoleCreate{Key: key, SoleCreateReceipt: receipt})
	if err != nil {
		return nil, err
	}
	fields[soleCreateStateField] = blob
	return json.Marshal(fields)
}
