package inventory

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

const testWeapon uint32 = 101010912

// 武器类型判定要真的走一遍目录（[equipment type] 文本在基础件上），所以这份测试
// 从配置里读同一份目录，而不是手搓一个空壳。
func replicateTestCatalog(t *testing.T) *EquipmentCatalog {
	t.Helper()
	const path = "../../configs/equipment.current37.json"
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var shell struct {
		Source struct {
			Checksum string `json:"checksum"`
		} `json:"source"`
	}
	if e = json.Unmarshal(raw, &shell); e != nil {
		t.Fatal(e)
	}
	cat, e := LoadEquipmentCatalog(path, shell.Source.Checksum)
	if e != nil {
		t.Fatalf("LoadEquipmentCatalog: %v", e)
	}
	return cat
}

// 幻化一次要同时吃掉武器本体和一枚模具，并把武器模板登记进仓库。
func TestReplicateWeaponSkinConsumesWeaponAndMold(t *testing.T) {
	b := Bag{
		Equipment: []BagEquipment{{Slot: 9, Template: testWeapon}},
		Items:     []BagItem{{Slot: 130, Template: LinusMold, Amount: 3}},
	}
	got, cost, e := b.ReplicateWeaponSkin(9, nil, LinusMold, ReplicationActor{})
	if e != nil {
		t.Fatalf("replicate: %v", e)
	}
	if len(got.Equipment) != 0 {
		t.Fatalf("equipment=%v, want the weapon gone", got.Equipment)
	}
	if len(got.Items) != 1 || got.Items[0].Amount != 2 {
		t.Fatalf("items=%v, want one mold row left with 2", got.Items)
	}
	if len(got.WeaponSkins) != 1 || got.WeaponSkins[0] != testWeapon {
		t.Fatalf("skins=%v, want [%d]", got.WeaponSkins, testWeapon)
	}
	if cost.Duplicate || cost.Mold != LinusMold || cost.MoldSlot != 130 || cost.MoldAmount != 2 {
		t.Fatalf("cost=%+v", cost)
	}
	// 原背包不能被改到：调用方要在校验失败时能用原值。
	if len(b.Equipment) != 1 || b.Items[0].Amount != 3 {
		t.Fatalf("input mutated: %+v", b)
	}
}

// 只有一枚模具时整格消失——这是"缺料"与"已扣掉"最容易混淆的一步。
func TestReplicateWeaponSkinDropsLastMoldRow(t *testing.T) {
	b := Bag{
		Equipment: []BagEquipment{{Slot: 9, Template: testWeapon}},
		Items:     []BagItem{{Slot: 129, Template: LinusSteelMold, Amount: 1}},
	}
	got, cost, e := b.ReplicateWeaponSkin(9, nil, LinusMold, ReplicationActor{})
	if e != nil {
		t.Fatalf("replicate: %v", e)
	}
	if len(got.Items) != 0 {
		t.Fatalf("items=%v, want the mold row gone", got.Items)
	}
	if cost.Mold != LinusSteelMold || cost.MoldAmount != 0 {
		t.Fatalf("cost=%+v", cost)
	}
}

// 两种模具都在时，扣的是玩家在窗口里选中的那种（CMD1592 的 mode 只表达这个）。
// 2026-09-27 实机：不看 mode 一律先扣普通模具，选精钢那枚也照样扣掉普通的。
func TestReplicateWeaponSkinMoldFollowsSelection(t *testing.T) {
	both := func() Bag {
		return Bag{
			Equipment: []BagEquipment{{Slot: 9, Template: testWeapon}},
			Items: []BagItem{
				{Slot: 120, Template: LinusMold, Amount: 1},
				{Slot: 129, Template: LinusSteelMold, Amount: 1},
			},
		}
	}
	got, cost, e := both().ReplicateWeaponSkin(9, nil, MoldForMode(2), ReplicationActor{})
	if e != nil {
		t.Fatalf("replicate: %v", e)
	}
	if cost.Mold != LinusSteelMold || cost.MoldSlot != 129 {
		t.Fatalf("cost=%+v, want the steel mold at 129", cost)
	}
	if len(got.Items) != 1 || got.Items[0].Template != LinusMold {
		t.Fatalf("items=%v, want only the plain mold left", got.Items)
	}
	got, cost, e = both().ReplicateWeaponSkin(9, nil, MoldForMode(1), ReplicationActor{})
	if e != nil {
		t.Fatalf("replicate: %v", e)
	}
	if cost.Mold != LinusMold || cost.MoldSlot != 120 {
		t.Fatalf("cost=%+v, want the plain mold at 120", cost)
	}
	if len(got.Items) != 1 || got.Items[0].Template != LinusSteelMold {
		t.Fatalf("items=%v, want only the steel mold left", got.Items)
	}
}

// 选中的那种没有库存时退回另一种，而不是直接拒绝。
func TestReplicateWeaponSkinFallsBackToOtherMold(t *testing.T) {
	b := Bag{
		Equipment: []BagEquipment{{Slot: 9, Template: testWeapon}},
		Items:     []BagItem{{Slot: 129, Template: LinusSteelMold, Amount: 1}},
	}
	_, cost, e := b.ReplicateWeaponSkin(9, nil, MoldForMode(1), ReplicationActor{})
	if e != nil {
		t.Fatalf("replicate: %v", e)
	}
	if cost.Mold != LinusSteelMold {
		t.Fatalf("cost=%+v, want the fallback steel mold", cost)
	}
}

// 非本职业能用的武器整单拒绝：客户端自己的确认框不拦职业，复制出来的皮肤进了仓库
// 本职业也用不了（实机 2026-09-27 反馈）。判定与"能不能穿上"同源。
func TestReplicateWeaponSkinRefusesForeignJob(t *testing.T) {
	cat := replicateTestCatalog(t)
	b := Bag{
		Equipment: []BagEquipment{{Slot: 9, Template: testWeapon}},
		Items:     []BagItem{{Slot: 130, Template: LinusMold, Amount: 1}},
	}
	got, _, e := b.ReplicateWeaponSkin(9, cat, LinusMold, ReplicationActor{Job: "[gunner]", Advancement: 1})
	if e == nil {
		t.Fatal("replicate accepted a weapon this job cannot use")
	}
	if len(got.Equipment) != 1 || len(got.Items) != 1 {
		t.Fatalf("refusal changed the bag: %+v", got)
	}
	// 换成武器自己声明的职业家族（101010912 是 [swordman]）必须放行，否则正常流程
	// 会被一起挡掉。
	if _, _, e := b.ReplicateWeaponSkin(9, cat, LinusMold, ReplicationActor{Job: "[swordman]"}); e != nil {
		t.Fatalf("replicate refused the weapon's own job: %v", e)
	}
}

// 没有模具就整个拒绝，背包原样返回 —— 宁可什么都不发生，也不能吃装备不给皮肤。
func TestReplicateWeaponSkinRefusesWithoutMold(t *testing.T) {
	b := Bag{Equipment: []BagEquipment{{Slot: 9, Template: testWeapon}}}
	got, cost, e := b.ReplicateWeaponSkin(9, nil, LinusMold, ReplicationActor{})
	if e == nil {
		t.Fatalf("replicate succeeded without a mold: %+v", cost)
	}
	if len(got.Equipment) != 1 || len(got.WeaponSkins) != 0 {
		t.Fatalf("refusal changed the bag: %+v", got)
	}
}

// 槽位上是防具（不是武器）时必须拒绝：请求里只有槽位号，拖了东西就可能指到别的装备。
func TestReplicateWeaponSkinRefusesNonWeapon(t *testing.T) {
	cat := replicateTestCatalog(t)
	b := Bag{
		Equipment: []BagEquipment{{Slot: 9, Template: 10400}},
		Items:     []BagItem{{Slot: 130, Template: LinusMold, Amount: 1}},
	}
	got, _, e := b.ReplicateWeaponSkin(9, cat, LinusMold, ReplicationActor{})
	if e == nil {
		t.Fatal("replicate accepted a coat")
	}
	if len(got.Equipment) != 1 || len(got.Items) != 1 {
		t.Fatalf("refusal changed the bag: %+v", got)
	}
}

// 目录可用时真实武器必须通过类型校验（[equipment type] = [weapon]）。
func TestReplicateWeaponSkinAcceptsWeaponFromCatalog(t *testing.T) {
	cat := replicateTestCatalog(t)
	b := Bag{
		Equipment: []BagEquipment{{Slot: 9, Template: testWeapon}},
		Items:     []BagItem{{Slot: 130, Template: LinusMold, Amount: 1}},
	}
	if _, _, e := b.ReplicateWeaponSkin(9, cat, LinusMold, ReplicationActor{}); e != nil {
		t.Fatalf("replicate refused a weapon: %v", e)
	}
}

// 空槽位同样拒绝，且不碰模具。
func TestReplicateWeaponSkinRefusesEmptySlot(t *testing.T) {
	b := Bag{Items: []BagItem{{Slot: 130, Template: LinusMold, Amount: 1}}}
	if _, _, e := b.ReplicateWeaponSkin(9, nil, LinusMold, ReplicationActor{}); e == nil {
		t.Fatal("replicate accepted an empty slot")
	}
}

// [required job skill] 是成对的（职业文本 → 技能 id），只对本职业那一条生效；没有这个
// 字段的武器（太刀一类）不设门槛。
func TestRequiredJobSkillReadsJobPairs(t *testing.T) {
	fields := map[string][]pvf.Token{
		"[required job skill]": {
			{Type: 6, Text: "[swordman]"}, {Type: 0, Value: 33},
			{Type: 6, Text: "[demonic swordman]"}, {Type: 0, Value: 33},
			{Type: 6, Text: "[at swordman]"}, {Type: 0, Value: 95},
		},
	}
	for job, want := range map[string]uint16{
		"[swordman]": 33, "[demonic swordman]": 33, "[at swordman]": 95, "[gunner]": 0,
	} {
		if got := RequiredJobSkill(fields, job); got != want {
			t.Fatalf("RequiredJobSkill(%s)=%d, want %d", job, got, want)
		}
	}
	for name, f := range map[string]map[string][]pvf.Token{
		"absent": nil,
		"empty":  {"[required job skill]": nil},
	} {
		if got := RequiredJobSkill(f, "[swordman]"); got != 0 {
			t.Fatalf("%s: RequiredJobSkill=%d, want 0", name, got)
		}
	}
}

// 光剑要求光剑精通，而 [usable job] 只认 [swordman] —— 鬼剑士五系转职的职业文本都是
// 它，狂战士照样过；只有 [required job skill] 才区分得开。实机 2026-09-27：狂战士能把
// 光剑复制进幻化仓库，复制出来的皮肤本职业一辈子用不上。
func TestReplicateWeaponSkinGatesOnRequiredJobSkill(t *testing.T) {
	const beamsword uint32 = 401040091
	cat := &EquipmentCatalog{index: map[uint32]EquipmentDefinition{
		beamsword: {
			ID: beamsword, Path: "beamsword.equ", SHA256: "beamsword",
			Fields: map[string][]pvf.Token{
				"[equipment type]":     {{Type: 6, Text: "[weapon]"}, {Type: 0, Value: 19}},
				"[usable job]":         {{Type: 6, Text: "[swordman]"}},
				"[required job skill]": {{Type: 6, Text: "[swordman]"}, {Type: 0, Value: 33}},
			},
		},
	}}
	bag := func() Bag {
		return Bag{
			Equipment: []BagEquipment{{Slot: 9, Template: beamsword}},
			Items:     []BagItem{{Slot: 130, Template: LinusMold, Amount: 1}},
		}
	}
	var asked uint16
	refused := ReplicationActor{Job: "[swordman]", Advancement: 3,
		CanUseSkill: func(skill uint16) bool { asked = skill; return false }}
	got, _, e := bag().ReplicateWeaponSkin(9, cat, LinusMold, refused)
	if e == nil {
		t.Fatal("replicate accepted a weapon whose mastery is outside this advancement")
	}
	if asked != 33 {
		t.Fatalf("gate asked about skill %d, want 33", asked)
	}
	if len(got.Equipment) != 1 || len(got.Items) != 1 || len(got.WeaponSkins) != 0 {
		t.Fatalf("refusal changed the bag: %+v", got)
	}
	allowed := ReplicationActor{Job: "[swordman]", Advancement: 1,
		CanUseSkill: func(uint16) bool { return true }}
	if _, _, e := bag().ReplicateWeaponSkin(9, cat, LinusMold, allowed); e != nil {
		t.Fatalf("the subclass that owns the mastery was refused: %v", e)
	}
	// 目录不可用（CanUseSkill 为 nil）时不设这道门槛，保持原有行为。
	if _, _, e := bag().ReplicateWeaponSkin(9, cat, LinusMold, ReplicationActor{Job: "[swordman]"}); e != nil {
		t.Fatalf("gate fired without a skill catalog: %v", e)
	}
}

// 仓库里的老条目：修好复制校验之前登记进去的跨职业外观不会被删，而是靠佩戴这一步拦住
// （实机 2026-09-27：狂战士仓库里躺着光剑）。判定与复制路径同源，但解除必须永远放行。
func TestWeaponSkinUsableGatesOnJob(t *testing.T) {
	const beamsword uint32 = 401040091
	cat := &EquipmentCatalog{index: map[uint32]EquipmentDefinition{
		beamsword: {
			ID: beamsword, Path: "beamsword.equ", SHA256: "beamsword",
			Fields: map[string][]pvf.Token{
				"[equipment type]":     {{Type: 6, Text: "[weapon]"}, {Type: 0, Value: 19}},
				"[usable job]":         {{Type: 6, Text: "[swordman]"}},
				"[required job skill]": {{Type: 6, Text: "[swordman]"}, {Type: 0, Value: 33}},
			},
		},
	}}
	berserker := ReplicationActor{Job: "[swordman]", Advancement: 3,
		CanUseSkill: func(uint16) bool { return false }}
	if err := cat.WeaponSkinUsable(beamsword, berserker); !errors.Is(err, ErrWeaponSkinNotUsable) {
		t.Fatalf("berserker wearing a beamsword: %v, want ErrWeaponSkinNotUsable", err)
	}
	swordmaster := ReplicationActor{Job: "[swordman]", Advancement: 1,
		CanUseSkill: func(uint16) bool { return true }}
	if err := cat.WeaponSkinUsable(beamsword, swordmaster); err != nil {
		t.Fatalf("the subclass that owns the mastery was refused: %v", err)
	}
	// 别的职业连 [usable job] 都过不去，同样是这个哨兵错误：调用方只认一种"戴不上"。
	foreign := ReplicationActor{Job: "[gunner]", Advancement: 1,
		CanUseSkill: func(uint16) bool { return true }}
	if err := cat.WeaponSkinUsable(beamsword, foreign); !errors.Is(err, ErrWeaponSkinNotUsable) {
		t.Fatalf("gunner: %v, want ErrWeaponSkinNotUsable", err)
	}
	for name, tc := range map[string]struct {
		cat   *EquipmentCatalog
		skin  uint32
		actor ReplicationActor
	}{
		"unapply":             {cat, 0, berserker},
		"nil catalog":         {nil, beamsword, berserker},
		"unknown skin":        {cat, 999999, berserker},
		"no skill catalog":    {cat, beamsword, ReplicationActor{Job: "[swordman]"}},
		"no job on the actor": {cat, beamsword, ReplicationActor{}},
	} {
		if err := tc.cat.WeaponSkinUsable(tc.skin, tc.actor); err != nil {
			t.Fatalf("%s was refused: %v", name, err)
		}
	}
}

// 同一件外观已经登记过：不扣任何东西（客户端自己的重复提示走这条）。
func TestReplicateWeaponSkinSkipsDuplicate(t *testing.T) {
	b := Bag{
		Equipment:   []BagEquipment{{Slot: 9, Template: testWeapon}},
		Items:       []BagItem{{Slot: 130, Template: LinusMold, Amount: 1}},
		WeaponSkins: []uint32{testWeapon},
	}
	got, cost, e := b.ReplicateWeaponSkin(9, nil, LinusMold, ReplicationActor{})
	if e != nil {
		t.Fatalf("replicate: %v", e)
	}
	if !cost.Duplicate {
		t.Fatalf("cost=%+v, want Duplicate", cost)
	}
	if len(got.Equipment) != 1 || len(got.Items) != 1 || len(got.WeaponSkins) != 1 {
		t.Fatalf("duplicate changed the bag: %+v", got)
	}
}
