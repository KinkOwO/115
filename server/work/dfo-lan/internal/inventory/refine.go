package inventory

// 锻造（Refine，NPC Kiri）：只作用于武器，提升 Independent Attack，上限 +8。
//
// 与强化（CMD80 mode=0）/ 增幅（CMD80 mode=1）的关系：
//   - 锻造是**独立 opcode CMD 430**，请求/回包结构与 CMD80 完全不同（没有 mode 字节）；
//   - 强化等级与增幅等级共用装备行偏移 10 的低五位，锻造等级是**另一格**；
//   - 官方规则：武器专属、上限 +8、失败**等级不变且装备不碎**（这是与强化最大的差别）。
//
// 协议布局取自实机抓包 + 客户端 handler sub_145886FF0 的反编译：
//   请求 45 字节：容器 / 装备槽 / 模板 / 材料槽 / 名称长度 / 名称 / 2 字节尾部
//   回包 13 字节：见 protocol.RefineReply
//
// 官方规则（服主核定，115 神界原版）：
//   - 仅限武器，上限 Refine 8；
//   - 失败不掉级、武器不碎，只扣本次提交的气息；
//   - ★ 不消耗金币（和强化/增幅不一样，客户端锻造面板没有金币一栏）；
//   - 单次消耗 = 7 *（等级 + 1），成功失败都扣：+0→1 收 7 个，+7→8 收 56 个。
//
// 规则数据来自 configs/refine.json。成功率表与消耗表由服主提供（115 版本）；
// 材料消耗 PVF 里没有表（etc/ 下 15022 个文件全列举无 refine*.etc）。

import (
	"crypto/rand"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

const refineModel = "refine-v1"

// 锻造失败处理（官方只有一种：等级不变）。
const refinePenaltyNone = "none"

type refineConfig struct {
	Version          int    `json:"version"`
	Source           string `json:"source"`
	MaterialTemplate uint32 `json:"material_template"`
	MaterialName     string `json:"material_name"`
	MaxLevel         int    `json:"max_level"`
	// GoldCost 恒为 0：★ 原版锻造**不收金币**，和强化/增幅不一样 ——
	// 客户端锻造面板只显示 Powerful Energy 数量，根本没有金币一栏。
	// 这里保留字段只为把这条规则写死在配置里，业务侧不得出现任何扣金逻辑。
	GoldCost int `json:"gold_cost"`
	// RecordOffset 是装备 181 字节行里锻造等级所在的那一格。
	// 客户端反序列化器 sub_14576D8B0 里，记录偏移 76/77/78/81 是四个「受保护」
	// 字节（走 sub_146E922E0 + 196 的混淆写），默认取 76，实机校准后改这里即可。
	RecordOffset              int            `json:"record_offset"`
	SuccessRatePercentByLevel map[string]int `json:"success_rate_percent_by_level"`
	MaterialCountByLevel      map[string]int `json:"material_count_by_level"`
	FailurePenalty            string         `json:"failure_penalty"`
}

var refineRules *refineConfig

// LoadRefineRules 读取锻造规则；文件缺失时锻造整体拒绝（不影响强化/增幅/打红字）。
func LoadRefineRules(path string) error {
	var c refineConfig
	if loaded, err := loadOptionalJSON(path, &c); !loaded || err != nil {
		return err
	}

	if c.Version != 1 || c.MaterialTemplate == 0 || len(c.SuccessRatePercentByLevel) == 0 {
		return fmt.Errorf("锻造规则源定义不完整")
	}
	refineRules = &c
	return nil
}

func RefineRulesLoaded() bool { return refineRules != nil }

// RefineMaterialTemplate 返回锻造材料模板（Powerful Energy 3326）。
func RefineMaterialTemplate() uint32 {
	if refineRules == nil {
		return 0
	}
	return refineRules.MaterialTemplate
}

// IsRefineMaterial 报告模板是否为锻造材料。
func IsRefineMaterial(template uint32) bool {
	return refineRules != nil && template == refineRules.MaterialTemplate
}

// RefineMaxLevel 返回锻造等级上限（官方 +8）。
func RefineMaxLevel() int {
	if refineRules == nil || refineRules.MaxLevel <= 0 {
		return 8
	}
	return refineRules.MaxLevel
}

// RefineRecordOffset 返回装备行里锻造等级的字节偏移；<0 表示不写入行（只存服务端状态）。
func RefineRecordOffset() int {
	if refineRules == nil {
		return -1
	}
	if refineRules.RecordOffset < 0 || refineRules.RecordOffset >= protocol.CurrentItemRecordSize {
		return -1
	}
	return refineRules.RecordOffset
}

// RefineGoldCost 返回锻造的金币费用。
//
// ★ 恒为 0：原版锻造不收金币（与强化/增幅不同），客户端锻造面板没有金币一栏。
// 保留这个访问器是为了让「不收金币」成为一条被测试钉住的显式规则，
// 而不是「代码里恰好没写扣钱」这种随时会被改坏的状态。
func RefineGoldCost() uint32 {
	if refineRules == nil || refineRules.GoldCost <= 0 {
		return 0
	}
	return uint32(refineRules.GoldCost)
}

// RefineSuccessPercent 返回从 level 锻到 level+1 的成功率。
func RefineSuccessPercent(level int) int {
	if refineRules == nil {
		return 0
	}
	m := refineRules.SuccessRatePercentByLevel
	if v, ok := m[fmt.Sprint(level)]; ok {
		return v
	}
	return m["default"]
}

// RefineMaterialCount 返回该等级需要消耗的材料数量。
func RefineMaterialCount(level int) (uint32, bool) {
	if refineRules == nil || level < 0 {
		return 0, false
	}
	m := refineRules.MaterialCountByLevel
	if v, ok := m[fmt.Sprint(level)]; ok && v > 0 {
		return uint32(v), true
	}
	if v, ok := m["default"]; ok && v > 0 {
		return uint32(v), true
	}
	return 0, false
}

var refineRandomInt = func(n int) (int, error) {
	if n <= 1 {
		return 0, nil
	}
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return int(binary.BigEndian.Uint32(b[:]) % uint32(n)), nil
}

// refineLevel 读装备的当前锻造等级。
//
// 权威值是 BagEquipment.Refine（服务端自己记的状态）；只有从没锻过的装备才回退到
// 行内字节（这样把旧存档里手工改过的行也能读出来）。
func refineLevel(gear BagEquipment, row []byte) byte {
	if gear.Refine > 0 {
		return gear.Refine
	}
	if off := RefineRecordOffset(); off >= 0 && len(row) > off {
		if v := row[off]; v > 0 && int(v) <= RefineMaxLevel() {
			return v
		}
	}
	return 0
}

// setRefineLevel 写锻造等级：服务端状态 + 行内镜像（供客户端渲染）。
func setRefineLevel(gear *BagEquipment, row []byte, level byte) {
	if int(level) > RefineMaxLevel() {
		level = byte(RefineMaxLevel())
	}
	gear.Refine = level
	if off := RefineRecordOffset(); off >= 0 && len(row) > off {
		row[off] = level
	}
}

// RefineReceipt 是一次锻造的结果。
type RefineReceipt struct {
	Request           protocol.RefineRequest `json:"request"`
	Equipment         BagEquipment           `json:"equipment"`
	EquipmentSpace    byte                   `json:"equipment_space"`
	EquipmentSlot     uint16                 `json:"equipment_slot"`
	LevelBefore       byte                   `json:"level_before"`
	LevelAfter        byte                   `json:"level_after"`
	Result            byte                   `json:"result"` // 0 成功 / 1 失败不变
	Success           bool                   `json:"success"`
	MaterialSlot      uint16                 `json:"material_slot"`
	MaterialRemaining uint32                 `json:"material_remaining"`
	MaterialSpent     uint32                 `json:"material_spent"`
	SuccessPercent    int                    `json:"success_percent"`
	RowBefore         string                 `json:"row_before_hex"`
	RowAfter          string                 `json:"row_after_hex"`
	RecordOffset      int                    `json:"record_offset"`
}

// ApplyRefine 处理 CMD430：校验武器与材料、扣料、按成功率判定，失败等级不变。

func (s *WearService) ApplyRefine(role Role, r protocol.RefineRequest) (json.RawMessage, RefineReceipt, error) {
	var out RefineReceipt
	bag, err := ReadBag(role.State)
	if err != nil {
		return nil, out, err
	}
	// 目标装备：按请求里的空间找（0 背包 / 3 已穿戴），找不到再退回另一侧。
	space, items, index := bag.findEquipment(r.EquipmentSpace, r.EquipmentSlot, r.EquipmentTemplate)
	if index < 0 {
		return nil, out, Refuse(RefusalItems, "目标装备不在背包或已穿戴槽位里")
	}
	gear := items[index]
	if err = gear.ValidateRecord(); err != nil {
		return nil, out, err
	}
	d, err := s.Catalog.Definition(gear.Template)
	if err != nil {
		return nil, out, err
	}
	kind, ok := d.Fields["[equipment type]"]
	if !ok || len(kind) == 0 {
		return nil, out, Refuse(RefusalEquipment, "目标不是装备")
	}
	// ★ 官方：只有武器能锻造（dstr 35128 "Items that are not weapons cannot be refined."）。
	if kind[0].Text != "[weapon]" {
		return nil, out, Refuse(RefusalLimit, "只有武器可以锻造（当前是 %s）", kind[0].Text)
	}

	row := EquipmentRow(gear)
	rowBefore := fmt.Sprintf("%x", row[:])
	level := int(refineLevel(gear, row[:]))
	if level >= RefineMaxLevel() {
		return nil, out, Refuse(RefusalLimit, "锻造等级已达上限 +%d", RefineMaxLevel())
	}
	count, ok := RefineMaterialCount(level)
	if !ok {
		return nil, out, Refuse(RefusalMaterials, "锻造等级 %d 取不到材料消耗", level)
	}

	// 扣材料：请求里只带一个材料槽。
	rows := append([]BagItem(nil), bag.Items...)
	remaining := uint32(0)
	found := false
	for i, item := range rows {
		if item.Slot != r.MaterialSlot {
			continue
		}
		if !IsRefineMaterial(item.Template) {
			return nil, out, Refuse(RefusalMaterials, "锻造材料槽位放的不是 %s（槽 %d 里是模板 %d，需要 %d）",
				refineRules.MaterialName, r.MaterialSlot, item.Template, refineRules.MaterialTemplate)
		}
		if item.Amount < count {
			return nil, out, Refuse(RefusalMaterials, "锻造材料不足：需要 %d，持有 %d", count, item.Amount)
		}
		remaining = item.Amount - count
		if remaining == 0 {
			rows = append(rows[:i:i], rows[i+1:]...)
		} else {
			rows[i].Amount = remaining
		}
		found = true
		break
	}
	if !found {
		return nil, out, Refuse(RefusalItems, "锻造材料不在背包里")
	}

	// 判定成功率。
	percent := RefineSuccessPercent(level)
	roll, err := refineRandomInt(100)
	if err != nil {
		return nil, out, err
	}
	success := roll < percent
	result := byte(1)
	newLevel := byte(level)
	if success {
		result = 0
		newLevel = byte(level + 1)
	}
	// 官方规则：失败**等级不变、装备不碎**，所以结果码只有 0 / 1，
	// 且 result==1 时客户端 handler 要求 old == new（protocol.RefineReply 已校验）。
	setRefineLevel(&gear, row[:], newLevel)

	out = RefineReceipt{
		Request: r, Equipment: gear, EquipmentSpace: space, EquipmentSlot: r.EquipmentSlot,
		LevelBefore: byte(level), LevelAfter: newLevel, Result: result, Success: success,
		MaterialSlot: r.MaterialSlot, MaterialRemaining: remaining, MaterialSpent: count,
		SuccessPercent: percent,
		RowBefore:      rowBefore, RowAfter: fmt.Sprintf("%x", row[:]),
		RecordOffset: RefineRecordOffset(),
	}

	items = append([]BagEquipment(nil), items...)
	gear.Record = append([]byte(nil), row[:]...)
	items[index] = gear
	out.Equipment.Record = append([]byte(nil), gear.Record...)
	if space == 3 {
		bag.Worn = items
	} else {
		bag.Equipment = items
	}
	bag.Items = rows

	next, err := SaveBag(role.State, bag)
	if err != nil {
		return nil, out, err
	}
	return next, out, nil
}
