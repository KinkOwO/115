package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"strings"
	"testing"
)

// 秘宝装备不作为继承材料（2026-10-04 业主规则）。
//
// 背景：继承把材料件清成白板（官方 dstr 69059 语义：失去强化/增幅/附魔/锻造，本体保留）。
// 玩家把白板的秘宝耳环分解后，本体消失且无法经图鉴替换找回（22/23/25 槽不在替换覆盖内）⇒
// 材料件命中秘宝 ID 段（100391000..100391999）或秘宝登记表时必须拒绝继承，**两件都不动**。
func TestInheritRefusesSoleEarringAsMaterial(t *testing.T) {
	const (
		soleEarring = 100391003 // 秘宝段内合成定义（Eternal Fragment - Earrings 位）
		normalItem  = 100051282 // 普通防具（对照材料/基础件）
		stoneTier   = 100313740 // g121 融合石（非秘宝段，验证保护不误伤）
		soleSupport = 100346156 // 秘宝登记表：辅助
		soleStone   = 100354181 // 秘宝登记表：魔法石
		soleCube    = 100391142 // 秘宝登记表：耳环（天气立方体）
	)

	gear, err := NewEquipmentCatalog(EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: "inherit-protect-test"},
		Rows: []EquipmentDefinition{
			synthRow(soleEarring, "[earring]"),
			synthRow(normalItem, "[coat]"),
			synthRow(stoneTier, "[amalgamation stone]"),
			synthRow(soleSupport, "[support]"),
			synthRow(soleStone, "[magic stone]"),
			synthRow(soleCube, "[earring]"),
		},
	}, "inherit-protect-test")
	if err != nil {
		t.Fatal(err)
	}
	svc := &WearService{Catalog: gear}

	// 合成装备行：模板号必须写入 offset 2..6（ValidateRecord 校验），等级写 offset 10 低五位。
	record := func(template uint32, level byte) []byte {
		rec := make([]byte, protocol.CurrentItemRecordSize)
		binary.LittleEndian.PutUint32(rec[2:6], template)
		rec[amplifyReinforceOffset] = level & reinforceLevelMask
		return rec
	}

	newBag := func() Bag {
		return Bag{Equipment: []BagEquipment{
			{Slot: 100, Template: soleEarring, Durability: 50, Record: record(soleEarring, 10)},
			{Slot: 101, Template: normalItem, Durability: 50, Record: record(normalItem, 0)},
		}}
	}

	// 1) 秘宝耳环等级高 ⇒ 方向判定后它成为材料件 ⇒ 必须拒绝（本体不被清除）。
	bag := newBag()
	_, err = svc.applyInheritEntry(&bag, protocol.InheritEntry{
		SlotA: 100, TemplateA: soleEarring, SpaceA: 0,
		SlotB: 101, TemplateB: normalItem, SpaceB: 0,
	})
	if err == nil || !strings.Contains(err.Error(), "秘宝装备,不能作为继承材料") {
		t.Fatalf("秘宝耳环作材料应被拒绝：%v", err)
	}
	// 拒绝后两件装备的等级都不动（材料件的 +10 没有被清掉）。
	if got := protectLevelAt(bag, 100); got != 10 {
		t.Fatalf("拒绝后秘宝耳环的等级不应被清：%d", got)
	}
	if got := protectLevelAt(bag, 101); got != 0 {
		t.Fatalf("拒绝后基础件也不该被改：%d", got)
	}

	// 2) 反向：秘宝耳环等级低 ⇒ 它是基础件（保留并接收等级），不受保护规则限制。
	bag = newBag()
	bag.Equipment[1].Record = record(normalItem, 20) // 普通装备 +20，秘宝耳环 +10 ⇒ 普通件成为材料
	receipt, err := svc.applyInheritEntry(&bag, protocol.InheritEntry{
		SlotA: 100, TemplateA: soleEarring, SpaceA: 0,
		SlotB: 101, TemplateB: normalItem, SpaceB: 0,
	})
	if err != nil {
		t.Fatalf("秘宝耳环作基础件不应被拒绝：%v", err)
	}
	if receipt.MaterialTemplate != normalItem {
		t.Fatalf("材料件应为普通装备 %d，实际 %d", normalItem, receipt.MaterialTemplate)
	}
	if got := protectLevelAt(bag, 100); got != 20 {
		t.Fatalf("秘宝耳环（基础件）应接收等级 20，实际 %d", got)
	}
	// 材料件（普通装备）按规则清零保留在原格。
	if got := protectLevelAt(bag, 101); got != 0 {
		t.Fatalf("材料件应清零保留：%d", got)
	}

	// 3) 登记表覆盖的秘宝源（辅助 / 魔法石 / 耳环）同样受保护 ——
	//    即使 ID 不在 100391xxx 段内（辅助/魔法石走登记表分支）。
	oldRules := soleEquipmentRules
	SetSoleEquipmentRules(&catalog.SoleEquipmentRules{Items: map[uint32]catalog.SoleEquipmentInfo{
		soleSupport: {}, soleStone: {}, soleCube: {},
	}})
	t.Cleanup(func() { SetSoleEquipmentRules(oldRules) })

	for _, tpl := range []uint32{soleSupport, soleStone, soleCube} {
		if !isSoleEquipmentTemplate(tpl) {
			t.Fatalf("登记表秘宝 %d 应命中保护判定", tpl)
		}
		bag := Bag{Equipment: []BagEquipment{
			{Slot: 200, Template: tpl, Durability: 50, Record: record(tpl, 15)},
			{Slot: 201, Template: normalItem, Durability: 50, Record: record(normalItem, 0)},
		}}
		entry := protocol.InheritEntry{
			SlotA: 200, TemplateA: tpl, SpaceA: 0,
			SlotB: 201, TemplateB: normalItem, SpaceB: 0,
		}
		_, err = svc.applyInheritEntry(&bag, entry)
		if err == nil || !strings.Contains(err.Error(), "秘宝装备,不能作为继承材料") {
			t.Fatalf("登记表秘宝 %d 作材料应被拒绝：%v", tpl, err)
		}
		if got := protectLevelAt(bag, 200); got != 15 {
			t.Fatalf("拒绝后登记表秘宝 %d 的等级不应被清：%d", tpl, got)
		}
	}
	// 登记表外的普通装备与融合石不受影响（isSoleEquipmentTemplate 为假）。
	if isSoleEquipmentTemplate(normalItem) || isSoleEquipmentTemplate(stoneTier) {
		t.Fatalf("普通装备/融合石不应命中秘宝保护")
	}
}

// synthRow 构造一条最小的装备目录行（继承校验只读 [equipment type]）。
func synthRow(id uint32, kind string) EquipmentDefinition {
	return EquipmentDefinition{
		ID:     id,
		Path:   "test/inherit/" + protectItoa(id) + ".equ",
		SHA256: strings.Repeat("a", 64),
		Fields: map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: kind}},
		},
	}
}

func protectItoa(v uint32) string {
	if v == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

func protectLevelAt(bag Bag, slot uint16) byte {
	for _, it := range bag.Equipment {
		if it.Slot == slot {
			rec := EquipmentRow(it)
			return rec[amplifyReinforceOffset] & reinforceLevelMask
		}
	}
	return 0
}
