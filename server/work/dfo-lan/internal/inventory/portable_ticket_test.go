package inventory

// 便携强化器 / 增幅器 / 锻造炉的识别与落地回归。
//
// 判据：PVF 物品脚本 [action type]（[portable upgrade] / [portable amplify] /
// [portable genuine damage upgrade upgrede]），模板 id 全集见 portable_ticket.go。
// 三个模板是 2026-10-07 实机事件日志确认的用户实际持有值。

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

const (
	portableTestGearSlot = 24
	portableTestMatSlot  = 107
	portableTestWeapon   = 900000002 // 本用例专用，注入到装备目录
)

// 识别：只有三个模板被认可，强化券/矛盾结晶/Powerful Energy 等相邻道具不误伤。
func TestPortableTemplateRecognition(t *testing.T) {
	if !IsPortableUpgradeTemplate(590723097) {
		t.Error("590723097 应是便携强化器（[portable upgrade]）")
	}
	if !IsPortableAmplifyTemplate(590723096) {
		t.Error("590723096 应是便携增幅器（[portable amplify]）")
	}
	if !IsPortableRefineTemplate(10307734) {
		t.Error("10307734 应是便携锻造炉（[portable genuine damage upgrade upgrede]）")
	}
	// 相邻/易混道具绝不能误判为便携道具。
	if IsPortableUpgradeTemplate(7814) {
		t.Error("强化券 7814 不应被当作便携强化器")
	}
	if IsPortableAmplifyTemplate(3242) {
		t.Error("矛盾结晶体 3242 不应被当作便携增幅器")
	}
	if IsPortableRefineTemplate(3326) {
		t.Error("Powerful Energy 3326 不应被当作便携锻造炉")
	}
}

// 便携锻造场景：武器 + 材料槽放锻造炉 → 消耗 1 个锻造炉、锻造成功升到 +1。
func portableWeaponFixture(t *testing.T) (*WearService, Role) {
	t.Helper()
	loadRefineRulesForTest(t)
	c, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	eq, e := LoadEquipmentCatalog("../../configs/equipment.current35.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	eq.index[portableTestWeapon] = EquipmentDefinition{
		ID:     portableTestWeapon,
		Path:   "equipment/character/common/sword/sword/portable_test.equ",
		SHA256: strings.Repeat("a", 64),
		Fields: map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: "[weapon]"}},
		},
	}
	rec := make([]byte, protocol.CurrentItemRecordSize)
	binary.LittleEndian.PutUint32(rec[2:], portableTestWeapon)
	bag := Bag{
		Version:   "ordinary-bag-v1",
		Items:     []BagItem{{Slot: portableTestMatSlot, Template: 10307734, Amount: 1}},
		Equipment: []BagEquipment{{Slot: portableTestGearSlot, Template: portableTestWeapon, Record: rec}},
	}
	state, e := SaveBag(json.RawMessage(`{"level":100,"advancement":0}`), bag)
	if e != nil {
		t.Fatal(e)
	}
	return &WearService{Catalog: eq}, Role{ConfigVersion: c.Source.SaveIdentity(), State: state}
}

func TestPortableRefineConsumesOneRefiner(t *testing.T) {
	svc, role := portableWeaponFixture(t)
	req := protocol.RefineRequest{
		EquipmentSpace:    0,
		EquipmentSlot:     portableTestGearSlot,
		EquipmentTemplate: portableTestWeapon,
		MaterialSlot:      portableTestMatSlot,
	}
	next, out, err := svc.ApplyRefine(role, req)
	if err != nil {
		t.Fatalf("便携锻造应通过: %v", err)
	}
	if out.MaterialSpent != 1 {
		t.Fatalf("应恰好消耗 1 个锻造炉，实际 %d", out.MaterialSpent)
	}
	if out.MaterialRemaining != 0 {
		t.Fatalf("锻造炉用完剩余应为 0，实际 %d", out.MaterialRemaining)
	}
	// +0 锻造成功率 100%（官方表 +0~+2 必成），应升到 +1。
	if !out.Success || out.LevelAfter != 1 {
		t.Fatalf("+0 锻造必成，期望 success=true level=1，实际 success=%v level=%d",
			out.Success, out.LevelAfter)
	}
	// 锻造炉被整行扣掉。
	bag, e := ReadBag(next)
	if e != nil {
		t.Fatal(e)
	}
	for _, item := range bag.Items {
		if item.Slot == portableTestMatSlot {
			t.Fatalf("锻造炉用完应整行移除，仍存在: %+v", item)
		}
	}
	// 装备仍在（扣道具不能连武器一起丢），且锻造等级落库为 1。
	for _, gear := range bag.Equipment {
		if gear.Slot == portableTestGearSlot {
			if gear.Refine != 1 {
				t.Fatalf("锻造后装备等级 = %d，期望 1", gear.Refine)
			}
			return
		}
	}
	t.Fatal("锻造后找不到那件武器")
}

// 便携强化器 / 增幅器共用夹具：一件可强化装备 + 材料槽（强化=106/增幅=105）放便携道具。
// gearTmpl 注入装备目录；priorType 非 0 时给装备写上红字（增幅前置）。
func portableGearFixture(t *testing.T, gearTmpl uint32, itemTmpl uint32, itemAmount uint32, ticketSlot uint16, priorType byte) (*WearService, Role) {
	t.Helper()
	c, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	eq, e := LoadEquipmentCatalog("../../configs/equipment.current35.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	eq.index[gearTmpl] = EquipmentDefinition{
		ID:     gearTmpl,
		Path:   "equipment/character/common/jacket/cloth/portable_test.equ",
		SHA256: strings.Repeat("a", 64),
		Fields: map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: "[coat]"}},
			"[minimum level]":  {{Type: 0, Value: 100}},
			"[rarity]":         {{Type: 0, Value: 4}},
		},
	}
	rec := make([]byte, protocol.CurrentItemRecordSize)
	binary.LittleEndian.PutUint32(rec[2:], gearTmpl)
	if priorType != 0 {
		rec[amplifyTypeOffset] = priorType
	}
	bag := Bag{
		Version:   "ordinary-bag-v1",
		Items:     []BagItem{{Slot: ticketSlot, Template: itemTmpl, Amount: itemAmount}},
		Equipment: []BagEquipment{{Slot: portableTestGearSlot, Template: gearTmpl, Record: rec}},
	}
	state, e := SaveBag(json.RawMessage(`{"level":100,"advancement":0}`), bag)
	if e != nil {
		t.Fatal(e)
	}
	return &WearService{Catalog: eq}, Role{ConfigVersion: c.Source.SaveIdentity(), State: state}
}

// 便携强化器：材料槽放强化器 → 走便携分支，消耗 1 个、不耗金币、成功率 30%、装备还在。
func TestPortableUpgradeReinforcesWithoutGold(t *testing.T) {
	loadGoldRulesForTest(t)
	const gearTmpl = uint32(900000003)
	svc, role := portableGearFixture(t, gearTmpl, 590723097, 1, 106, 0)
	req := protocol.ReinforcementRequest{
		Mode: 0, EquipmentSpace: 0, EquipmentSlot: portableTestGearSlot, EquipmentTemplate: gearTmpl,
		TicketSpace: 0, TicketSlot: 106, MaterialSlot: 0xffff,
	}
	next, _, out, err := svc.ApplyGoldReinforcement(role, json.RawMessage(`{}`), "portable-test", req)
	if err != nil {
		t.Fatalf("便携强化应通过校验: %v", err)
	}
	if out.Mode != "portable_upgrade" {
		t.Fatalf("mode = %q，期望 portable_upgrade", out.Mode)
	}
	if out.MaterialTemplate != 590723097 || out.MaterialSpent != 1 {
		t.Fatalf("应消耗 1 个强化器 590723097，实际模板 %d 数量 %d", out.MaterialTemplate, out.MaterialSpent)
	}
	if out.GoldSpent != 0 {
		t.Fatalf("便携强化不耗金币，实际 %d", out.GoldSpent)
	}
	// 成功率沿用普通强化表：+0 必成（100%）。
	if out.Rate != 100 {
		t.Fatalf("便携强化 +0 应沿用普通强化成功率 100%%，实际 %d", out.Rate)
	}
	// 强化器被整行扣掉、装备仍在。
	bag, e := ReadBag(next)
	if e != nil {
		t.Fatal(e)
	}
	for _, item := range bag.Items {
		if item.Slot == 106 {
			t.Fatalf("强化器用完应整行移除: %+v", item)
		}
	}
	for _, gear := range bag.Equipment {
		if gear.Slot == portableTestGearSlot {
			return
		}
	}
	t.Fatal("强化后找不到装备")
}

// 便携增幅器：材料槽放增幅器（装备已有红字）→ 消耗 1 个、不耗金币、成功率 30%。
func TestPortableAmplifyUpgradesWithoutGold(t *testing.T) {
	loadAmplifyUpgradeRulesForTest(t)
	const gearTmpl = uint32(900000004)
	svc, role := portableGearFixture(t, gearTmpl, 590723096, 1, 105, 3) // 已有红字类型 3 = 力量
	req := protocol.ReinforcementRequest{
		Mode: 1, EquipmentSpace: 0, EquipmentSlot: portableTestGearSlot, EquipmentTemplate: gearTmpl,
		TicketSpace: 0, TicketSlot: 105, MaterialSlot: 0xffff,
	}
	next, out, err := svc.ApplyAmplifyUpgrade(role, req)
	if err != nil {
		t.Fatalf("便携增幅应通过校验: %v", err)
	}
	if out.MaterialSpent != 1 {
		t.Fatalf("应消耗 1 个增幅器，实际 %d", out.MaterialSpent)
	}
	if out.GoldSpent != 0 {
		t.Fatalf("便携增幅不耗金币，实际 %d", out.GoldSpent)
	}
	// 成功率沿用普通增幅表：+0 必成（100%）。
	if out.SuccessPercent != 100 {
		t.Fatalf("便携增幅 +0 应沿用普通增幅成功率 100%%，实际 %d", out.SuccessPercent)
	}
	// 增幅器被整行扣掉。
	bag, e := ReadBag(next)
	if e != nil {
		t.Fatal(e)
	}
	for _, item := range bag.Items {
		if item.Slot == 105 {
			t.Fatalf("增幅器用完应整行移除: %+v", item)
		}
	}
}
