package inventory

import (
	"dfolan/internal/catalog"
	"math/rand"
	"testing"
)

func testBagCatalog(t *testing.T) (catalog.LootCatalog, BagRules) {
	t.Helper()
	c, err := catalog.LoadLoot("../../configs/loot.next25.json")
	if err != nil {
		t.Fatalf("LoadLoot failed: %v", err)
	}
	rules, err := LoadBagRules("../../configs/inventory.current37.json")
	if err != nil {
		t.Fatalf("LoadBagRules failed: %v", err)
	}
	return c, rules
}

func testEquipmentCatalog(t *testing.T, source string) *EquipmentCatalog {
	t.Helper()
	eq, err := LoadEquipmentCatalog("../../configs/equipment.current37.json", source)
	if err != nil {
		t.Fatalf("LoadEquipmentCatalog failed: %v", err)
	}
	return eq
}

func TestBagDisjointFallbackSuccess(t *testing.T) {
	c, rules := testBagCatalog(t)
	b := Bag{
		Equipment: []BagEquipment{
			{Slot: 11, Template: 27054, Durability: 35},
		},
		Items: []BagItem{
			{Slot: 121, Template: ClearCubeFragmentID, Amount: 10},
		},
	}

	updated, res, err := b.Disjoint(c, rules, nil, []uint16{11}, 0xFFFF)
	if err != nil {
		t.Fatalf("Disjoint failed: %v", err)
	}
	if len(updated.Equipment) != 0 {
		t.Fatalf("expected equipment slot 11 to be removed, got len=%d", len(updated.Equipment))
	}
	if len(res.DeletedSlots) != 1 || res.DeletedSlots[0] != 11 {
		t.Fatalf("expected deleted slot 11, got %v", res.DeletedSlots)
	}
	// Fallback gives clear cubes and common soul
	var hasCube, hasSoul bool
	for _, rw := range res.Rewards {
		if rw.Template == ClearCubeFragmentID && rw.Count > 0 {
			hasCube = true
		}
		if rw.Template == CommonSoulID && rw.Count > 0 {
			hasSoul = true
		}
	}
	if !hasCube || !hasSoul {
		t.Fatalf("expected clear cube and soul in rewards, got %+v", res.Rewards)
	}
}

// TestBagDisjointSlot12WornCoexistence 测试截图核心问题：
// 身上穿戴槽 12 存在装备时，背包槽 12 的装备（如 Cotton Armguard）仍能正常分解，不会报 "cannot disjoint worn equipment at slot 12"。
func TestBagDisjointSlot12WornCoexistence(t *testing.T) {
	c, rules := testBagCatalog(t)
	eq := testEquipmentCatalog(t, rules.Source)

	b := Bag{
		Worn: []BagEquipment{
			{Slot: 12, Template: 10000, Durability: 60}, // 穿戴槽 12 穿了武器/防具
		},
		Equipment: []BagEquipment{
			{Slot: 12, Template: 22002, Durability: 0}, // 背包槽 12 的布护臂 (Cotton Armguard)
		},
	}

	updated, res, err := b.Disjoint(c, rules, eq, []uint16{12}, 0xFFFF)
	if err != nil {
		t.Fatalf("Disjoint slot 12 should succeed even when worn slot 12 is occupied: %v", err)
	}
	if len(updated.Equipment) != 0 {
		t.Fatalf("expected bag equipment slot 12 to be removed, got %d items", len(updated.Equipment))
	}
	if len(updated.Worn) != 1 || updated.Worn[0].Slot != 12 {
		t.Fatalf("expected worn equipment to remain untouched, got %+v", updated.Worn)
	}
	if len(res.DeletedSlots) != 1 || res.DeletedSlots[0] != 12 {
		t.Fatalf("expected deleted slot 12, got %v", res.DeletedSlots)
	}

	// 验证布护臂（Uncommon 蓝装）产物：
	// 1. 无色小晶块 (3037)
	// 2. 有色小晶块 (3033..3036)
	// 3. 高级灵魂 (10100116)
	var cubeCount, colorCubeCount, soulCount uint32
	for _, rw := range res.Rewards {
		switch rw.Template {
		case ClearCubeFragmentID:
			cubeCount += rw.Count
		case DarkCubeFragmentID, LightCubeFragmentID, FireCubeFragmentID, WaterCubeFragmentID:
			colorCubeCount += rw.Count
		case UncommonSoulID:
			soulCount += rw.Count
		}
	}
	if cubeCount == 0 {
		t.Fatalf("expected clear cube fragments, got 0")
	}
	if colorCubeCount == 0 {
		t.Fatalf("expected colored cube fragments for uncommon equipment, got 0")
	}
	if soulCount == 0 {
		t.Fatalf("expected uncommon souls for uncommon equipment, got 0")
	}
}

// TestBagDisjointCalculatorsAcrossRarities 验证各品质装备产物符合 PVF 规则
func TestBagDisjointCalculatorsAcrossRarities(t *testing.T) {
	rng := rand.New(rand.NewSource(12345))

	// 1. 普通白装 (Rarity 0, Lv 1, Value 432) -> 无色小晶块 + 普通灵魂
	commonInfo := DisjointEquipmentInfo{
		Template:     10000,
		Rarity:       0,
		MinimumLevel: 1,
		Value:        432,
	}
	commonRewards := CalculateDisjointRewards(commonInfo, rng)
	var commonCube, commonSoul bool
	for _, rw := range commonRewards {
		if rw.Template == ClearCubeFragmentID && rw.Count >= 1 {
			commonCube = true
		}
		if rw.Template == CommonSoulID && rw.Count >= 1 {
			commonSoul = true
		}
	}
	if !commonCube || !commonSoul {
		t.Fatalf("common equipment rewards mismatch: %+v", commonRewards)
	}

	// 2. 优秀蓝装 (Rarity 1, Lv 5, Value 2880) -> 无色小晶块 + 彩色小晶块 + 高级灵魂
	uncommonInfo := DisjointEquipmentInfo{
		Template:     22002,
		Rarity:       1,
		MinimumLevel: 5,
		Value:        2880,
	}
	uncommonRewards := CalculateDisjointRewards(uncommonInfo, rng)
	var uncCube, uncColor, uncSoul bool
	for _, rw := range uncommonRewards {
		if rw.Template == ClearCubeFragmentID && rw.Count == 5 {
			uncCube = true
		}
		if (rw.Template >= 3033 && rw.Template <= 3036) && rw.Count >= 1 {
			uncColor = true
		}
		if rw.Template == UncommonSoulID && rw.Count >= 1 {
			uncSoul = true
		}
	}
	if !uncCube || !uncColor || !uncSoul {
		t.Fatalf("uncommon equipment rewards mismatch: %+v", uncommonRewards)
	}

	// 3. 稀有紫装 (Rarity 2, Lv 50, Value 30000) -> 无色 + 下级元素结晶 + 稀有灵魂
	rareInfo := DisjointEquipmentInfo{
		Template:     20000,
		Rarity:       2,
		MinimumLevel: 50,
		Value:        30000,
	}
	rareRewards := CalculateDisjointRewards(rareInfo, rng)
	var rareCube, rareCrystal, rareSoul bool
	for _, rw := range rareRewards {
		if rw.Template == ClearCubeFragmentID && rw.Count >= 1 {
			rareCube = true
		}
		if rw.Template == LowGradeElementalCrystalID && rw.Count >= 1 {
			rareCrystal = true
		}
		if rw.Template == RareSoulID && rw.Count >= 1 {
			rareSoul = true
		}
	}
	if !rareCube || !rareCrystal || !rareSoul {
		t.Fatalf("rare equipment rewards mismatch: %+v", rareRewards)
	}

	// 4. 神器粉装 (Rarity 3, Lv 85, Value 80000) -> 无色 + 上级元素结晶 + 神器灵魂
	uniqueInfo := DisjointEquipmentInfo{
		Template:     30000,
		Rarity:       3,
		MinimumLevel: 85,
		Value:        80000,
	}
	uniqueRewards := CalculateDisjointRewards(uniqueInfo, rng)
	var uCube, uCrystal, uSoul bool
	for _, rw := range uniqueRewards {
		if rw.Template == ClearCubeFragmentID && rw.Count >= 1 {
			uCube = true
		}
		if rw.Template == HighGradeElementalCrystalID && rw.Count >= 1 {
			uCrystal = true
		}
		if rw.Template == UniqueSoulID && rw.Count >= 1 {
			uSoul = true
		}
	}
	if !uCube || !uCrystal || !uSoul {
		t.Fatalf("unique equipment rewards mismatch: %+v", uniqueRewards)
	}

	// 5. 史诗橙装 (Rarity 4, Lv 100, Value 120000) -> 无色 + 史诗灵魂
	epicInfo := DisjointEquipmentInfo{
		Template:     40000,
		Rarity:       4,
		MinimumLevel: 100,
		Value:        120000,
	}
	epicRewards := CalculateDisjointRewards(epicInfo, rng)
	var eCube, eSoul bool
	for _, rw := range epicRewards {
		if rw.Template == ClearCubeFragmentID && rw.Count >= 1 {
			eCube = true
		}
		if rw.Template == EpicSoulID && rw.Count >= 1 {
			eSoul = true
		}
	}
	if !eCube || !eSoul {
		t.Fatalf("epic equipment rewards mismatch: %+v", epicRewards)
	}

	// 6. 115 级史诗装备 (Rarity 4, Lv 115, Value 200000) -> 无色 + 115 史诗灵魂 (10361515)
	epic115Info := DisjointEquipmentInfo{
		Template:     50000,
		Rarity:       4,
		MinimumLevel: 115,
		Value:        200000,
	}
	epic115Rewards := CalculateDisjointRewards(epic115Info, rng)
	var e115Cube, e115Soul bool
	for _, rw := range epic115Rewards {
		if rw.Template == ClearCubeFragmentID && rw.Count >= 1 {
			e115Cube = true
		}
		if (rw.Template == Soul115EpicID || rw.Template == 10362399) && rw.Count >= 1 {
			e115Soul = true
		}
	}
	if !e115Cube || !e115Soul {
		t.Fatalf("115 epic equipment rewards mismatch: %+v", epic115Rewards)
	}
}

func TestBagDisjointMissingItemRejected(t *testing.T) {
	c, rules := testBagCatalog(t)
	b := Bag{
		Equipment: []BagEquipment{
			{Slot: 11, Template: 27054, Durability: 35},
		},
	}

	_, _, err := b.Disjoint(c, rules, nil, []uint16{12}, 0xFFFF)
	if err == nil {
		t.Fatal("expected error for nonexistent equipment at slot 12, got nil")
	}
}

func TestBagDisjointSlotOutOfRange(t *testing.T) {
	c, rules := testBagCatalog(t)
	b := Bag{
		Equipment: []BagEquipment{
			{Slot: 11, Template: 27054, Durability: 35},
		},
	}

	_, _, err := b.Disjoint(c, rules, nil, []uint16{99}, 0xFFFF)
	if err == nil {
		t.Fatal("expected error for slot 99 outside range, got nil")
	}
}

func TestBagDisjointCannotDisjointWorn(t *testing.T) {
	c, rules := testBagCatalog(t)
	b := Bag{
		Worn: []BagEquipment{
			{Slot: 15, Template: 10001, Durability: 30},
		},
		Equipment: []BagEquipment{
			{Slot: 11, Template: 27054, Durability: 35},
		},
	}

	// 请求仅存在于 Worn 的槽位 15，因为背包中没有此物品，应当被正确拒绝
	_, _, err := b.Disjoint(c, rules, nil, []uint16{15}, 0xFFFF)
	if err == nil {
		t.Fatal("expected error when requesting disjoint on slot not in equipment bag, got nil")
	}
}
