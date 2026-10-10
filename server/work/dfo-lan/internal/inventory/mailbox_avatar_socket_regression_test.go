package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"testing"
)

// 实机缺陷回归（2026-10-07）：GM 按 mods/newchar_kit.lua 发「宠物装备」邮件
// （模板 500950039~500950043 / 500960034~500960037 / 500970014，`[artifact *]`），
// 领取时被无条件补上了 30 字节全零的 avatar_options；穿上穿戴槽 27/28/29 后，
// 登录追加包在 protocol.DetailedEquipment 报
// "avatar blob on non-avatar detailed row"，该角色卡在选人界面进不去。
//
// 规则：默认徽章孔只补**时装**（`[equipment type]` 以 " avatar]" 结尾，容器 space 1）。
// 宠物装备 / 普通装备一个字节都不能带。
func TestAddMailItemFillsAvatarSocketsOnlyForAvatars(t *testing.T) {
	const src = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	full, err := OpenFullEquipmentCatalog("testdata/equipment-flow", src)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	c := &EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: src}, Full: full}
	rules, err := LoadBagRules("../../configs/inventory.current37.json")
	if err != nil {
		t.Fatal(err)
	}

	const petGear, coatAvatar, ordinary = uint32(100950255), uint32(517500000), uint32(100051322)
	kind, err := c.EquipmentKind(petGear)
	if err != nil || !IsPetGear(kind) {
		t.Fatalf("样本变了：%d 的类型是 %q (%v)，不再是宠物装备", petGear, kind, err)
	}
	if !c.IsAvatarBagItem(coatAvatar) {
		t.Fatalf("样本变了：%d 不再是时装物品", coatAvatar)
	}
	if c.IsAvatarBagItem(ordinary) {
		t.Fatalf("样本变了：%d 被当成了时装物品", ordinary)
	}

	// 1) 宠物装备：落宠物栏，且不带任何头像块。
	bag, err := Bag{Version: "ordinary-bag-v1"}.AddMailItem(catalog.LootCatalog{}, rules, c, MailItem{
		Equipment: &BagEquipment{Template: petGear}})
	if err != nil {
		t.Fatal(err)
	}
	rows := bag.Special[7]
	if len(rows) != 1 || rows[0].Template != petGear {
		t.Fatalf("宠物装备没有进宠物栏：%+v", bag.Special)
	}
	if len(rows[0].AvatarOptions) != 0 || len(rows[0].AvatarSockets) != 0 {
		t.Fatalf("宠物装备被补了头像块（原缺陷）：opts=%v sockets=%v", rows[0].AvatarOptions, rows[0].AvatarSockets)
	}

	// 2) 普通装备：同样不带。
	bag, err = Bag{Version: "ordinary-bag-v1"}.AddMailItem(catalog.LootCatalog{}, rules, c, MailItem{
		Equipment: &BagEquipment{Template: ordinary}})
	if err != nil {
		t.Fatal(err)
	}
	if len(bag.Equipment) != 1 || bag.Equipment[0].Template != ordinary {
		t.Fatalf("普通装备没有进装备背包：%+v", bag.Equipment)
	}
	if len(bag.Equipment[0].AvatarOptions) != 0 {
		t.Fatalf("普通装备被补了头像块：%v", bag.Equipment[0].AvatarOptions)
	}

	// 3) 时装（space 1 邮件）：仍然按 PVF 默认孔补 30 字节块，2026-10-07 的
	//    「入包即带孔」修复不回退。
	bag, err = Bag{Version: "ordinary-bag-v1"}.AddMailItem(catalog.LootCatalog{}, rules, c, MailItem{
		Space: 1, Equipment: &BagEquipment{Template: coatAvatar}})
	if err != nil {
		t.Fatal(err)
	}
	avatars := bag.Special[1]
	if len(avatars) != 1 || avatars[0].Template != coatAvatar {
		t.Fatalf("时装没有进时装栏：%+v", bag.Special)
	}
	if len(avatars[0].AvatarOptions) != avatarSocketOptionsSize {
		t.Fatalf("时装默认孔 = %d 字节，期望 %d", len(avatars[0].AvatarOptions), avatarSocketOptionsSize)
	}
}
