package inventory

// 附魔（Enchantment，附魔宝珠 / CMD272 ENCHANT_BY_BEAD）
//
// 数据链：宝珠物品（脚本 [stackable type] [enchant waste] + [monster card id]）
// → 怪物卡（stackable/monstercard/mcard_*.stk）→ 卡的 [enchant table]/[enchant index] 才是附魔属性。
// 服务端只负责：扣 1 颗宝珠、把「这张卡」写进装备行 offset 14，客户端据此显示附魔属性。
// 新附魔**覆盖**旧的（直接覆写 offset 14）。
//
// 装备行 offset 14 = 附魔卡(u32)：
//   取证 row_writer sub_14576D8B0 —— `*(u32*)(a2+14)` 经 sub_146E922E0 写进物品对象 obj+8；
//   文档 docs/进度-20260917-副本可进性与GM工具.md 记为 enchantIndex@14(u32) 附魔卡。
//
// 规则数据来自 configs/enchant-beads.json（scripts/export_enchant_beads.py 只读 PVF 导出）。

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
)

// enchantCardOffset 是装备 181 字节行里附魔卡（u32）的偏移。
const enchantCardOffset = 14

const enchantModel = "enchant-bead-v1"

type enchantBeadConfig struct {
	Version int    `json:"version"`
	Source  string `json:"source"`
	Rule    string `json:"rule"`
	Beads   []struct {
		Template uint32 `json:"template"`
		Path     string `json:"path"`
		Card     uint32 `json:"card"`
		Expires  bool   `json:"expires"`
	} `json:"beads"`
}

// enchantBeadCards 是「附魔宝珠模板 → 附魔卡模板」查表。
var enchantBeadCards map[uint32]uint32

// LoadEnchantBeads 读取附魔宝珠清单；文件缺失时附魔整体拒绝（不影响其它系统）。
func LoadEnchantBeads(path string) error {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var c enchantBeadConfig
	if err = json.Unmarshal(b, &c); err != nil {
		return err
	}
	if c.Version != 1 || len(c.Beads) == 0 {
		return fmt.Errorf("附魔宝珠规则源定义不完整")
	}
	m := make(map[uint32]uint32, len(c.Beads))
	for _, bd := range c.Beads {
		if bd.Card != 0 {
			m[bd.Template] = bd.Card
		}
	}
	enchantBeadCards = m
	return nil
}

// EnchantBeadsLoaded 报告附魔宝珠清单是否已装载。
func EnchantBeadsLoaded() bool { return enchantBeadCards != nil }

// EnchantCardForBead 返回该宝珠对应的附魔卡（怪物卡）模板。
func EnchantCardForBead(template uint32) (uint32, bool) {
	c, ok := enchantBeadCards[template]
	return c, ok
}

// IsEnchantBead 报告模板是否为附魔宝珠。
func IsEnchantBead(template uint32) bool {
	_, ok := EnchantCardForBead(template)
	return ok
}

// enchantCard 读装备行里的附魔卡（offset 14，u32 小端）。
func enchantCard(row []byte) uint32 {
	if len(row) < enchantCardOffset+4 {
		return 0
	}
	return binary.LittleEndian.Uint32(row[enchantCardOffset:])
}

// setEnchantCard 把附魔卡写进装备行 offset 14（覆盖旧附魔）。
func setEnchantCard(row []byte, card uint32) {
	if len(row) < enchantCardOffset+4 {
		return
	}
	binary.LittleEndian.PutUint32(row[enchantCardOffset:], card)
}

// EnchantReceipt 是一次附魔的结果。
type EnchantReceipt struct {
	Request        protocol.EnchantByBeadRequest `json:"request"`
	BeadTemplate   uint32                        `json:"bead_template"`
	BeadSpace      byte                          `json:"bead_space"`
	BeadSlot       uint16                        `json:"bead_slot"`
	BeadRemaining  uint32                        `json:"bead_remaining"`
	Equipment      BagEquipment                  `json:"equipment"`
	EquipmentSpace byte                          `json:"equipment_space"`
	EquipmentSlot  uint16                        `json:"equipment_slot"`
	Card           uint32                        `json:"card"`
	PrevCard       uint32                        `json:"prev_enchant"`
	RowBefore      string                        `json:"row_before_hex"`
	RowAfter       string                        `json:"row_after_hex"`
}

// ApplyEnchantByBead 处理 CMD272：把宝珠对应的附魔卡写进装备行，扣掉一颗宝珠。
func (s *WearService) ApplyEnchantByBead(ctx context.Context, role storage.Character, key string, r protocol.EnchantByBeadRequest) (storage.Character, EnchantReceipt, error) {
	var out EnchantReceipt
	if s == nil || s.Store == nil || s.Catalog == nil || s.BagRules.Source != role.ConfigVersion {
		return role, out, fmt.Errorf("附魔需要有效装备目录及角色存档")
	}
	if !EnchantBeadsLoaded() {
		return role, out, fmt.Errorf("附魔宝珠规则未装载")
	}
	if r.BeadSpace != 0 {
		return role, out, fmt.Errorf("附魔宝珠容器 %d 不支持（仅背包）", r.BeadSpace)
	}
	return commitEquipmentEvent(ctx, s.Store, role, key, enchantModel, func(current storage.Character) (json.RawMessage, EnchantReceipt, error) {
		return s.applyEnchantByBead(current, r)
	})
}

func (s *WearService) applyEnchantByBead(role storage.Character, r protocol.EnchantByBeadRequest) (json.RawMessage, EnchantReceipt, error) {
	var out EnchantReceipt
	bag, err := ReadBag(role.State)
	if err != nil {
		return nil, out, err
	}
	// 目标装备：按请求里的装备空间找（0 背包 / 3 已穿戴）。
	space := r.EquipSpace
	items := bag.Equipment
	if space == 3 {
		items = bag.Worn
	}
	index := -1
	for i, gear := range items {
		if gear.Slot == r.EquipSlot {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, out, fmt.Errorf("目标装备不在 空间%d 槽%d", r.EquipSpace, r.EquipSlot)
	}
	gear := items[index]
	if err = gear.ValidateRecord(); err != nil {
		return nil, out, err
	}
	d, err := s.Catalog.Definition(gear.Template)
	if err != nil {
		return nil, out, err
	}
	if _, ok := d.Fields["[equipment type]"]; !ok {
		return nil, out, fmt.Errorf("目标不是装备")
	}

	// 扣宝珠：背包里那个槽必须就是这颗宝珠。
	rows := append([]BagItem(nil), bag.Items...)
	remaining := uint32(0)
	card := uint32(0)
	beadTemplate := uint32(0)
	found := false
	for i, item := range rows {
		if item.Slot != r.BeadSlot {
			continue
		}
		c, ok := EnchantCardForBead(item.Template)
		if !ok || item.Amount == 0 {
			return nil, out, fmt.Errorf("宝珠槽位里不是附魔宝珠（槽 %d 里是模板 %d）", r.BeadSlot, item.Template)
		}
		card = c
		beadTemplate = item.Template
		remaining = item.Amount - 1
		if remaining == 0 {
			rows = append(rows[:i:i], rows[i+1:]...)
		} else {
			rows[i].Amount = remaining
		}
		found = true
		break
	}
	if !found {
		return nil, out, fmt.Errorf("附魔宝珠不在背包里")
	}

	row := EquipmentRow(gear)
	rowBefore := fmt.Sprintf("%x", row[:])
	prevCard := enchantCard(row[:])
	setEnchantCard(row[:], card)
	gear.Record = append([]byte(nil), row[:]...)

	out = EnchantReceipt{
		Request: r, BeadTemplate: beadTemplate, BeadSpace: r.BeadSpace, BeadSlot: r.BeadSlot,
		BeadRemaining: remaining, Equipment: gear, EquipmentSpace: space, EquipmentSlot: r.EquipSlot,
		Card: card, PrevCard: prevCard,
		RowBefore: rowBefore, RowAfter: fmt.Sprintf("%x", row[:]),
	}
	items = append([]BagEquipment(nil), items...)
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
