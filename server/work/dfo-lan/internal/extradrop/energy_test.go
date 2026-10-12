package extradrop

import "testing"

// TestEnergyForMapsRarityToCount 钉住"装备稀有度 → 能量数量"的查表口径。
//
// 稀有度数值取自 `catalog.EquipmentAwakeningRarityNames` 的顺序：
//
//	0 common / 1 uncommon / 2 rare / 3 unique / 4 epic / 5 chronicle
//	6 legendary / 7 mythology / 8 primeval
//
// 与 ctp `[reward table]` 第一列的名字**逐名对齐**，所以这里同时钉住"名字对得上"这件事。
func TestEnergyForMapsRarityToCount(t *testing.T) {
	c, err := Parse(syntheticTable())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cases := []struct {
		group  string
		rarity int32
		want   uint32
		ok     bool
	}{
		{GroupGetItem, 2, 1, true},   // rare
		{GroupGetItem, 3, 2, true},   // unique
		{GroupGetItem, 8, 5, true},   // primeval
		{GroupGetItem, 4, 0, false},  // epic：合成表里 get item 组没有这一档
		{GroupDropItem, 3, 5, true},  // unique
		{GroupDropItem, 6, 0, false}, // 合成表 drop item 组只有 unique
		{GroupPartyDropItem, 6, 10, true},
		{GroupGetItem, 99, 0, false}, // 越界稀有度
		{GroupGetItem, -1, 0, false},
		{"不存在的组", 3, 0, false},
	}
	for _, tc := range cases {
		got, ok := c.EnergyFor(tc.group, tc.rarity)
		if ok != tc.ok {
			t.Fatalf("EnergyFor(%q,%d) ok=%v, want %v", tc.group, tc.rarity, ok, tc.ok)
		}
		if ok && got.Amount != tc.want {
			t.Fatalf("EnergyFor(%q,%d) amount=%d, want %d", tc.group, tc.rarity, got.Amount, tc.want)
		}
	}
	if _, ok := c.EnergyForParty(6); !ok {
		t.Fatal("EnergyForParty(legendary) 应当命中")
	}
	if c.Describe() == "" {
		t.Fatal("Describe 不该为空")
	}
}
