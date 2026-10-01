package inventory

import (
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
)

// 这一组用例盯住「装备继承」的**业务落地**：applyInherit 只依赖装备目录与角色
// 存档里的背包 JSON，不需要数据库，所以能在普通 `go test` 里跑完整条链路。
//
// 起因：CMD1722 曾经整条没有分派 —— 服务层与流程层都在，就是没人调，客户端按下
// 确认后服务端既不改状态也不回包，玩家看到的就是「按下继承毫无效果」。
// 分派由 cmd/wireprobe/inherit_wiring_test.go 钉住，这里钉住落库结果。
//
// ★ 方向：请求里两个 UI 槽没有「源 / 目标」语义，方向按等级定 ——
// **高等级件 = 材料件（清零保留），低等级件 = 基础件（接收等级）**。
// 官方文案 dstr 69059 写明材料件「will lose all its Enchantment/Reinforcement/
// Amplification/Refining levels」，且未写「disappear」⇒ 清零保留、不删除。
//
// 要搬运的东西全部落在 181 字节装备行里，且**服务端本来就有权威定义**：
//
//	offset 10 低五位  强化 / 增幅共用的等级字节（amplifyReinforceOffset，
//	                  reinforceLevelMask = 0x1f；高三位是再封装次数，必须保留）
//	offset 19         次元属性类型（amplifyTypeOffset，0 = 没有红字）
//	offset 20         次元属性数值（amplifyValueOffset）
//	offset 14         附魔卡（u32，enchantCardOffset）
//	BagEquipment.Refine  锻造（精炼）等级，独立字段
const (
	inheritMatSlot   = 9
	inheritBaseSlot  = 11
	inheritMatTmpl   = 900000011 // 本用例专用，注入到装备目录
	inheritBaseTmpl  = 900000012
	inheritSealCount = 0x40   // offset 10 的 bit5-7 = 再封装次数（这里是 2）
	inheritTestCard  = 510000 // 测试用附魔卡模板
)

// inheritFixture 装两件装备：matLevel 是材料件的强化/增幅等级（高），matType 是
// 次元类型；基础件等级恒 0，baseSeal 是它自己的再封装次数，用来验证不被覆盖。
// worn = true 时材料件穿在身上（容器 3），否则两件都在背包（容器 0）。
func inheritFixture(t *testing.T, matLevel, matType, matValue, baseSeal byte, worn bool) (*WearService, storage.Character) {
	t.Helper()
	c, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	eq, e := LoadEquipmentCatalog("../../configs/equipment.current35.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range []uint32{inheritMatTmpl, inheritBaseTmpl} {
		eq.index[id] = EquipmentDefinition{
			ID:     id,
			Path:   "equipment/character/common/jacket/cloth/inherit_test.equ",
			SHA256: strings.Repeat("b", 64),
			Fields: map[string][]pvf.Token{
				"[equipment type]": {{Type: 6, Text: "coat"}},
			},
		}
	}
	mat := make([]byte, protocol.CurrentItemRecordSize)
	binary.LittleEndian.PutUint32(mat[2:], inheritMatTmpl)
	mat[amplifyReinforceOffset] = matLevel
	mat[amplifyTypeOffset] = matType
	mat[amplifyValueOffset] = matValue

	base := make([]byte, protocol.CurrentItemRecordSize)
	binary.LittleEndian.PutUint32(base[2:], inheritBaseTmpl)
	base[amplifyReinforceOffset] = baseSeal // 等级 0，只带再封装次数

	bag := Bag{
		Version:   "ordinary-bag-v1",
		Equipment: []BagEquipment{{Slot: inheritBaseSlot, Template: inheritBaseTmpl, Record: base}},
	}
	if worn {
		bag.Worn = []BagEquipment{{Slot: inheritMatSlot, Template: inheritMatTmpl, Record: mat}}
	} else {
		bag.Equipment = append(bag.Equipment, BagEquipment{Slot: inheritMatSlot, Template: inheritMatTmpl, Record: mat})
	}
	state, e := SaveBag(json.RawMessage(`{"level":100,"advancement":0}`), bag)
	if e != nil {
		t.Fatal(e)
	}
	return &WearService{Catalog: eq}, storage.Character{ID: 1, ConfigVersion: c.Source.SaveIdentity(), State: state}
}

// inheritEntry 造一条继承记录：A 侧 = 材料件，B 侧 = 基础件（方向由服务层按等级定，
// 所以顺序其实无所谓）。matSpace/baseSpace 是客户端声明的容器。
func inheritEntry(matSpace, baseSpace byte) protocol.InheritEntry {
	return protocol.InheritEntry{
		SlotA:     inheritMatSlot,
		TemplateA: inheritMatTmpl,
		SpaceA:    matSpace,
		SlotB:     inheritBaseSlot,
		TemplateB: inheritBaseTmpl,
		SpaceB:    baseSpace,
		Const:     257,
	}
}

// applyOne 是对 applyInherit 的单记录便捷封装：返回落库后的 state 与唯一回执。
func applyOne(t *testing.T, svc *WearService, role storage.Character, e protocol.InheritEntry) (json.RawMessage, InheritReceipt) {
	t.Helper()
	next, receipts, err := svc.applyInherit(role, []protocol.InheritEntry{e})
	if err != nil {
		t.Fatalf("继承落库失败: %v", err)
	}
	if len(receipts) != 1 {
		t.Fatalf("回执数 = %d，期望 1", len(receipts))
	}
	return next, receipts[0]
}

func inheritGearRow(t *testing.T, state json.RawMessage, slot uint16) []byte {
	t.Helper()
	bag, e := ReadBag(state)
	if e != nil {
		t.Fatal(e)
	}
	list := append(append([]BagEquipment(nil), bag.Equipment...), bag.Worn...)
	for _, gear := range list {
		if gear.Slot == slot {
			row := EquipmentRow(gear)
			return row[:]
		}
	}
	t.Fatalf("继承后找不到槽 %d 的装备", slot)
	return nil
}

func inheritGearAt(t *testing.T, state json.RawMessage, slot uint16) BagEquipment {
	t.Helper()
	bag, e := ReadBag(state)
	if e != nil {
		t.Fatal(e)
	}
	list := append(append([]BagEquipment(nil), bag.Equipment...), bag.Worn...)
	for _, gear := range list {
		if gear.Slot == slot {
			return gear
		}
	}
	t.Fatalf("找不到槽 %d 的装备", slot)
	return BagEquipment{}
}

// setMatEnchant 给材料件写附魔卡（供附魔继承用例），返回新 state。
func setMatEnchant(t *testing.T, role storage.Character, card uint32) storage.Character {
	t.Helper()
	bag, e := ReadBag(role.State)
	if e != nil {
		t.Fatal(e)
	}
	for i := range bag.Equipment {
		if bag.Equipment[i].Slot == inheritMatSlot {
			row := EquipmentRow(bag.Equipment[i])
			setEnchantCard(row[:], card)
			bag.Equipment[i].Record = append([]byte(nil), row[:]...)
			break
		}
	}
	state, e := SaveBag(role.State, bag)
	if e != nil {
		t.Fatal(e)
	}
	role.State = state
	return role
}

// 背包 → 背包：等级、次元类型、次元数值三个字节必须一起落到基础件，
// 且基础件自己的再封装次数（offset 10 高三位）不能被抹掉。
func TestApplyInheritTransfersLevelTypeAndValue(t *testing.T) {
	svc, role := inheritFixture(t, 20, 3, 12, inheritSealCount, false)
	next, out := applyOne(t, svc, role, inheritEntry(0, 0))
	if out.BeforeLevel != 0 || out.AfterLevel != 20 {
		t.Errorf("基础件等级 = %d → %d，期望 0 → 20", out.BeforeLevel, out.AfterLevel)
	}
	if out.MaterialLevel != 20 {
		t.Errorf("回执材料件等级 = %d，期望 20（清零前的值）", out.MaterialLevel)
	}
	if out.AmplifyType != 3 || out.AmplifyValue != 12 {
		t.Errorf("次元属性 = 类型%d 数值%d，期望 3/12", out.AmplifyType, out.AmplifyValue)
	}
	row := inheritGearRow(t, next, inheritBaseSlot)
	if got := row[amplifyReinforceOffset] & reinforceLevelMask; got != 20 {
		t.Errorf("基础件 offset10 等级 = %d，期望 20", got)
	}
	if got := row[amplifyReinforceOffset] >> 5; got != 2 {
		t.Errorf("基础件的再封装次数被破坏：%#02x", row[amplifyReinforceOffset])
	}
	if row[amplifyTypeOffset] != 3 || row[amplifyValueOffset] != 12 {
		t.Errorf("基础件次元属性 = %d/%d，期望 3/12", row[amplifyTypeOffset], row[amplifyValueOffset])
	}
}

// ★ 材料件「清零保留」：等级 / 次元属性全归零，但装备本体必须还在原格。
// dstr 69059 写的是 lose all its ... levels，不是 disappear —— 删错了不可逆。
func TestApplyInheritZeroesButKeepsMaterialItem(t *testing.T) {
	svc, role := inheritFixture(t, 15, 1, 7, 0, false)
	next, _, err := svc.applyInherit(role, []protocol.InheritEntry{inheritEntry(0, 0)})
	if err != nil {
		t.Fatalf("继承落库失败: %v", err)
	}
	row := inheritGearRow(t, next, inheritMatSlot)
	if got := row[amplifyReinforceOffset] & reinforceLevelMask; got != 0 {
		t.Errorf("材料件等级未清零：%d，期望 0", got)
	}
	if row[amplifyTypeOffset] != 0 || row[amplifyValueOffset] != 0 {
		t.Errorf("材料件次元属性未清零：%d/%d", row[amplifyTypeOffset], row[amplifyValueOffset])
	}
	// 模板号还在 ⇒ 装备本体没被删。
	if got := binary.LittleEndian.Uint32(row[2:]); got != inheritMatTmpl {
		t.Errorf("材料件不见了：槽 %d 的模板 = %d，期望 %d", inheritMatSlot, got, inheritMatTmpl)
	}
}

// 材料件穿在身上（容器 3）、基础件在背包（容器 0）是最常见的一种用法，
// 旧会话的实机样本正是这一形态（记录内 +30 = 3）。回执必须报**实际定位到的**
// 容器，否则流程层会发错 NOTI14。
func TestApplyInheritResolvesWornMaterial(t *testing.T) {
	svc, role := inheritFixture(t, 15, 1, 7, 0, true)
	next, out := applyOne(t, svc, role, inheritEntry(3, 0))
	if out.MaterialSpace != 3 {
		t.Errorf("回执材料件容器 = %d，期望 3", out.MaterialSpace)
	}
	if out.BaseSpace != 0 {
		t.Errorf("回执基础件容器 = %d，期望 0", out.BaseSpace)
	}
	if got := inheritGearRow(t, next, inheritBaseSlot)[amplifyReinforceOffset] & reinforceLevelMask; got != 15 {
		t.Errorf("基础件等级 = %d，期望 15", got)
	}
}

// 请求里两个 UI 槽没有方向语义：把顺序反过来（低等级件先填）结果必须完全一样。
// 这条防止有人把「请求里的第 1 件」直接当成基础件。
func TestApplyInheritDirectionIsByLevelNotByOrder(t *testing.T) {
	svc, role := inheritFixture(t, 13, 2, 4, 0, false)
	flipped := protocol.InheritEntry{
		SlotA:     inheritBaseSlot,
		TemplateA: inheritBaseTmpl,
		SpaceA:    0,
		SlotB:     inheritMatSlot,
		TemplateB: inheritMatTmpl,
		SpaceB:    0,
	}
	next, out := applyOne(t, svc, role, flipped)
	if out.BaseSlot != inheritBaseSlot || out.MaterialSlot != inheritMatSlot {
		t.Errorf("方向判错：基础件槽 %d / 材料件槽 %d，期望 %d / %d",
			out.BaseSlot, out.MaterialSlot, inheritBaseSlot, inheritMatSlot)
	}
	if got := inheritGearRow(t, next, inheritBaseSlot)[amplifyReinforceOffset] & reinforceLevelMask; got != 13 {
		t.Errorf("基础件等级 = %d，期望 13", got)
	}
}

// 请求里报的容器可能不靠谱（客户端查槽位失败时会写成 3 再回退一次），
// 所以两边都按「先声明容器、再另一个」回退查找，**且用模板号对账** ——
// 背包格号与穿戴槽号是两套重叠编号，只按槽位查会拿错装备。
func TestApplyInheritFallsBackToOtherSpace(t *testing.T) {
	// 材料件其实在背包（worn=false），但请求里报的是容器 3。
	svc, role := inheritFixture(t, 8, 2, 3, 0, false)
	_, out := applyOne(t, svc, role, inheritEntry(3, 0))
	if out.MaterialSpace != 0 {
		t.Errorf("回执材料件容器 = %d，期望回退到 0", out.MaterialSpace)
	}
}

// 两件等级相同 ⇒ 无法判定方向且继承无意义 ⇒ 必须拒绝，不能挑一件当材料。
func TestApplyInheritRejectsEqualLevels(t *testing.T) {
	// 两件都是 0 级（fixture 里基础件恒 0 级，材料件也传 0）。
	svc, role := inheritFixture(t, 0, 0, 0, 0, false)
	if _, _, err := svc.applyInherit(role, []protocol.InheritEntry{inheritEntry(0, 0)}); err == nil ||
		!strings.Contains(err.Error(), "等级相同") {
		t.Fatalf("两件等级相同应拒绝，实际 %v", err)
	}
}

// 同一件装备既当材料又当基础：必须拒绝，否则会把自己的低五位写回自己、看起来像成功。
func TestApplyInheritRejectsSameItem(t *testing.T) {
	svc, role := inheritFixture(t, 12, 1, 5, 0, false)
	e := inheritEntry(0, 0)
	e.SlotB, e.TemplateB = e.SlotA, e.TemplateA
	if _, _, err := svc.applyInherit(role, []protocol.InheritEntry{e}); err == nil ||
		!strings.Contains(err.Error(), "同一格") {
		t.Fatalf("同一件应拒绝，实际 %v", err)
	}
}

// 模板号对不上（玩家换过装备）必须拒绝：两个容器都找不到就报错，绝不按槽位蒙一个。
func TestApplyInheritRejectsTemplateMismatch(t *testing.T) {
	svc, role := inheritFixture(t, 12, 1, 5, 0, false)
	e := inheritEntry(0, 0)
	e.TemplateB = inheritBaseTmpl + 1
	if _, _, err := svc.applyInherit(role, []protocol.InheritEntry{e}); err == nil ||
		!strings.Contains(err.Error(), "找不到模板") {
		t.Fatalf("模板不符应拒绝，实际 %v", err)
	}
}

// 次元属性「只增不删」：材料件没有红字（offset 19 == 0）时，不能把基础件
// 原有的红字清成 0。
func TestApplyInheritKeepsBaseAmplifyWhenMaterialHasNone(t *testing.T) {
	svc, role := inheritFixture(t, 9, 0, 0, 0, false)
	// 给基础件预置一套红字（fixture 只造了 0 级，这里直接改存档）。
	bag, e := ReadBag(role.State)
	if e != nil {
		t.Fatal(e)
	}
	for i := range bag.Equipment {
		if bag.Equipment[i].Slot == inheritBaseSlot {
			row := EquipmentRow(bag.Equipment[i])
			row[amplifyTypeOffset] = 4
			row[amplifyValueOffset] = 21
			bag.Equipment[i].Record = append([]byte(nil), row[:]...)
		}
	}
	state, e := SaveBag(role.State, bag)
	if e != nil {
		t.Fatal(e)
	}
	role.State = state
	next, _, err := svc.applyInherit(role, []protocol.InheritEntry{inheritEntry(0, 0)})
	if err != nil {
		t.Fatalf("继承落库失败: %v", err)
	}
	row := inheritGearRow(t, next, inheritBaseSlot)
	if row[amplifyTypeOffset] != 4 || row[amplifyValueOffset] != 21 {
		t.Errorf("基础件原有红字被清空：%d/%d，期望 4/21", row[amplifyTypeOffset], row[amplifyValueOffset])
	}
}

// 锻造（精炼）「只增不删」：材料件有锻造才覆盖，材料件没锻造时基础件的锻造保留。
func TestApplyInheritTransfersRefineOnlyWhenPositive(t *testing.T) {
	svc, role := inheritFixture(t, 7, 0, 0, 0, false)
	bag, e := ReadBag(role.State)
	if e != nil {
		t.Fatal(e)
	}
	for i := range bag.Equipment {
		switch bag.Equipment[i].Slot {
		case inheritBaseSlot:
			bag.Equipment[i].Refine = 5
		case inheritMatSlot:
			bag.Equipment[i].Refine = 3
		}
	}
	state, e := SaveBag(role.State, bag)
	if e != nil {
		t.Fatal(e)
	}
	role.State = state
	next, out := applyOne(t, svc, role, inheritEntry(0, 0))
	if out.Refine != 3 {
		t.Errorf("回执锻造 = %d，期望 3", out.Refine)
	}
	if got := inheritGearAt(t, next, inheritBaseSlot).Refine; got != 3 {
		t.Errorf("基础件锻造 = %d，期望 3（材料件的锻造）", got)
	}
	if got := inheritGearAt(t, next, inheritMatSlot).Refine; got != 0 {
		t.Errorf("材料件锻造未清零：%d，期望 0", got)
	}
}

// 反向：材料件没有锻造时，基础件自己的锻造不能被抹掉。
func TestApplyInheritKeepsBaseRefineWhenMaterialHasNone(t *testing.T) {
	svc, role := inheritFixture(t, 7, 0, 0, 0, false)
	bag, e := ReadBag(role.State)
	if e != nil {
		t.Fatal(e)
	}
	for i := range bag.Equipment {
		if bag.Equipment[i].Slot == inheritBaseSlot {
			bag.Equipment[i].Refine = 5
		}
	}
	state, e := SaveBag(role.State, bag)
	if e != nil {
		t.Fatal(e)
	}
	role.State = state
	next, out := applyOne(t, svc, role, inheritEntry(0, 0))
	if out.Refine != 0 {
		t.Errorf("回执锻造 = %d，期望 0（材料件没有锻造）", out.Refine)
	}
	if got := inheritGearAt(t, next, inheritBaseSlot).Refine; got != 5 {
		t.Errorf("基础件原有锻造被清空：%d，期望 5", got)
	}
}

// 附魔卡（offset 14，u32）要从材料件搬到基础件，材料件附魔卡清零。
func TestApplyInheritTransfersEnchantCard(t *testing.T) {
	svc, role := inheritFixture(t, 9, 0, 0, 0, false)
	role = setMatEnchant(t, role, inheritTestCard)
	next, out := applyOne(t, svc, role, inheritEntry(0, 0))
	if out.EnchantCard != inheritTestCard {
		t.Errorf("回执附魔卡 = %d，期望 %d", out.EnchantCard, inheritTestCard)
	}
	if got := enchantCard(inheritGearRow(t, next, inheritBaseSlot)); got != inheritTestCard {
		t.Errorf("基础件附魔卡 = %d，期望 %d", got, inheritTestCard)
	}
	if got := enchantCard(inheritGearRow(t, next, inheritMatSlot)); got != 0 {
		t.Errorf("材料件附魔卡未清零：%d，期望 0", got)
	}
}

// 附魔「只增不删」：材料件没有附魔时，基础件原有附魔不能被清空。
func TestApplyInheritKeepsBaseEnchantWhenMaterialHasNone(t *testing.T) {
	svc, role := inheritFixture(t, 9, 0, 0, 0, false)
	bag, e := ReadBag(role.State)
	if e != nil {
		t.Fatal(e)
	}
	for i := range bag.Equipment {
		if bag.Equipment[i].Slot == inheritBaseSlot {
			row := EquipmentRow(bag.Equipment[i])
			setEnchantCard(row[:], inheritTestCard)
			bag.Equipment[i].Record = append([]byte(nil), row[:]...)
		}
	}
	state, e := SaveBag(role.State, bag)
	if e != nil {
		t.Fatal(e)
	}
	role.State = state
	next, out := applyOne(t, svc, role, inheritEntry(0, 0))
	if out.EnchantCard != 0 {
		t.Errorf("回执附魔卡 = %d，期望 0（材料件没有附魔）", out.EnchantCard)
	}
	if got := enchantCard(inheritGearRow(t, next, inheritBaseSlot)); got != inheritTestCard {
		t.Errorf("基础件原有附魔被清空：%d，期望 %d", got, inheritTestCard)
	}
}

// 穿戴槽位可以并存两条记录：Group 0 = 真装备、Group 1 = 幻化外观
// （见 Bag.WornBaseItems 与 avatar_clone_coexistence_test.go）。
// 继承**只能**打到 Group 0 那一件上 —— 按槽位盲选会把等级写到外观件上，
// 玩家看到的是「继承没生效」。
func TestApplyInheritTargetsRealGearNotAppearanceAvatar(t *testing.T) {
	svc, role := inheritFixture(t, 15, 1, 7, 0, true)
	bag, e := ReadBag(role.State)
	if e != nil {
		t.Fatal(e)
	}
	look := make([]byte, protocol.CurrentItemRecordSize)
	binary.LittleEndian.PutUint32(look[2:], 900000099) // 外观件模板
	bag.Worn = append(bag.Worn, BagEquipment{Slot: inheritMatSlot, Template: 900000099, Group: 1, Record: look})
	state, e := SaveBag(role.State, bag)
	if e != nil {
		t.Fatal(e)
	}
	role.State = state
	next, _, err := svc.applyInherit(role, []protocol.InheritEntry{inheritEntry(3, 0)})
	if err != nil {
		t.Fatalf("穿戴侧继承失败: %v", err)
	}
	newBag, e := ReadBag(next)
	if e != nil {
		t.Fatal(e)
	}
	for _, w := range newBag.Worn {
		if w.Slot != inheritMatSlot {
			continue
		}
		level := EquipmentRow(w)[amplifyReinforceOffset] & reinforceLevelMask
		if w.Group == 1 && level != 0 {
			t.Errorf("幻化外观件被改了：Group %d 模板 %d 等级 %d，应为 0", w.Group, w.Template, level)
		}
		if w.Group == 0 && w.Template != inheritMatTmpl {
			t.Errorf("真装备被外观件顶掉了：Group 0 的模板 = %d，期望 %d", w.Template, inheritMatTmpl)
		}
	}
	// 基础件仍应正常收到等级。
	if got := inheritGearRow(t, next, inheritBaseSlot)[amplifyReinforceOffset] & reinforceLevelMask; got != 15 {
		t.Errorf("基础件等级 = %d，期望 15", got)
	}
}

// ★ 多记录：客户端一次 1722 请求可以带多条继承记录（多对装备同时轮换继承）。
// 服务端必须逐条独立处理，全部成功才落库。早期实现要求恰好 2 件、会误拒多对轮换，
// 本用例钉住该回归。
func TestApplyInheritMultipleEntries(t *testing.T) {
	// 造两对完全独立的装备：用两套槽位/模板。
	mk := func(slot uint16, tmpl uint32, level byte, worn bool) BagEquipment {
		row := make([]byte, protocol.CurrentItemRecordSize)
		binary.LittleEndian.PutUint32(row[2:], tmpl)
		row[amplifyReinforceOffset] = level
		return BagEquipment{Slot: slot, Template: tmpl, Record: row}
	}
	c, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	eq, e := LoadEquipmentCatalog("../../configs/equipment.current35.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range []uint32{101, 102, 103, 104} {
		eq.index[id] = EquipmentDefinition{
			ID:     id,
			Path:   "equipment/character/common/jacket/cloth/inherit_test.equ",
			SHA256: strings.Repeat("b", 64),
			Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "coat"}}},
		}
	}
	// 对 1：材料槽 30（模板 101，等级 10）→ 基础槽 31（模板 102，等级 0）
	// 对 2：材料槽 32（模板 103，等级 7）→ 基础槽 33（模板 104，等级 0）
	bag := Bag{Version: "ordinary-bag-v1", Equipment: []BagEquipment{
		mk(31, 102, 0, false), mk(30, 101, 10, false),
		mk(33, 104, 0, false), mk(32, 103, 7, false),
	}}
	state, e := SaveBag(json.RawMessage(`{"level":100,"advancement":0}`), bag)
	if e != nil {
		t.Fatal(e)
	}
	role := storage.Character{ID: 1, ConfigVersion: c.Source.SaveIdentity(), State: state}
	svc := &WearService{Catalog: eq}

	entries := []protocol.InheritEntry{
		{SlotA: 30, TemplateA: 101, SpaceA: 0, SlotB: 31, TemplateB: 102, SpaceB: 0, Const: 257},
		{SlotA: 32, TemplateA: 103, SpaceA: 0, SlotB: 33, TemplateB: 104, SpaceB: 0, Const: 257},
	}
	next, receipts, err := svc.applyInherit(role, entries)
	if err != nil {
		t.Fatalf("多记录继承失败: %v", err)
	}
	if len(receipts) != 2 {
		t.Fatalf("回执数 = %d，期望 2", len(receipts))
	}
	if got := inheritGearRow(t, next, 31)[amplifyReinforceOffset] & reinforceLevelMask; got != 10 {
		t.Errorf("对1基础件(槽31)等级 = %d，期望 10", got)
	}
	if got := inheritGearRow(t, next, 33)[amplifyReinforceOffset] & reinforceLevelMask; got != 7 {
		t.Errorf("对2基础件(槽33)等级 = %d，期望 7", got)
	}
	if got := inheritGearRow(t, next, 30)[amplifyReinforceOffset] & reinforceLevelMask; got != 0 {
		t.Errorf("对1材料件(槽30)等级未清零 = %d", got)
	}
	if got := inheritGearRow(t, next, 32)[amplifyReinforceOffset] & reinforceLevelMask; got != 0 {
		t.Errorf("对2材料件(槽32)等级未清零 = %d", got)
	}
}
