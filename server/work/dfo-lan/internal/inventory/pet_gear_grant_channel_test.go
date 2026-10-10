package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"testing"
)

// mods/newchar_kit.lua 用 `grant_item` 发宠物装备（刻意不走邮件，见该脚本文件头）：
// 服务端这条通道必须把宠物装备落进宠物装备栏 320..375，并且**一个头像块都不带**。
//
// 为什么专门钉它：2026-10-07 实机事故里，邮件领取路径给宠物装备写上了 30 字节的
// "时装默认孔"块（缺陷已修），穿上槽 27/28/29 后登录追加包被
// protocol.DetailedEquipment 整包拒绝、角色卡在选人界面。规则脚本改用
// `grant_item` 的前提就是这条通道干净 —— 它一旦被加上同样的补孔逻辑，事故会重演。
func TestGrantPetGearLandsCleanInPetContainer(t *testing.T) {
	const src = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	full, err := OpenFullEquipmentCatalog("testdata/equipment-flow", src)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	gear := &EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: src}, Full: full}
	rules, err := LoadBagRules("../../configs/inventory.current37.json")
	if err != nil {
		t.Fatal(err)
	}
	a := &Awarder{Catalog: catalog.LootCatalog{}, Rules: rules, Equipment: gear}

	const petGear = uint32(100950255) // [artifact red]，宠物装备
	kind, err := gear.EquipmentKind(petGear)
	if err != nil || !IsPetGear(kind) {
		t.Fatalf("样本变了：%d 的类型是 %q (%v)，不再是宠物装备", petGear, kind, err)
	}

	out, receipt, err := a.Grant(json.RawMessage(`{}`), petGear, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipt.Slots) != 1 || receipt.Slots[0] < PetGearFirst || receipt.Slots[0] > PetGearLast {
		t.Fatalf("宠物装备应落 %d..%d，实际槽位 %v", PetGearFirst, PetGearLast, receipt.Slots)
	}
	b, err := ReadBag(out)
	if err != nil {
		t.Fatal(err)
	}
	rows := b.Special[7]
	if len(rows) != 1 || rows[0].Template != petGear || rows[0].Slot != receipt.Slots[0] {
		t.Fatalf("宠物装备没有进宠物容器 list 7：%+v", rows)
	}
	if len(rows[0].AvatarOptions) != 0 || len(rows[0].AvatarSockets) != 0 {
		t.Fatalf("宠物装备被写上了头像块（原缺陷形态）：opts=%v sockets=%v", rows[0].AvatarOptions, rows[0].AvatarSockets)
	}
	if len(b.Equipment) != 0 {
		t.Fatalf("宠物装备不该落普通装备栏：%+v", b.Equipment)
	}
}
