package character

import (
	"dfolan/internal/inventory"
	"testing"
)

// 奥德赛角色不走任务链路，扩展装备槽靠通关指定副本解锁；
// 规则来源见 analysis/tasks/next50-odyssey-expanded-equip-slot.md §1。
func TestOdysseyExpandEquipMask(t *testing.T) {
	cases := []struct {
		id   uint32
		want byte
		ok   bool
		desc string
	}{
		{100004950, inventory.ExpandSupport, true, "安徒恩讨伐战 -> support (槽 22)"},
		{100004953, inventory.ExpandMagicStone, true, "使徒卢克 -> magic stone (槽 23)"},
		{100004969, inventory.ExpandEarring, true, "盖波加 -> earring (槽 25)"},
		{100004967, 0, false, "普通奥德赛副本不解锁"},
		{100004968, 0, false, "普通奥德赛副本不解锁"},
		{100004963, 0, false, "普通奥德赛副本不解锁"},
		{0, 0, false, "空 id"},
	}
	for _, c := range cases {
		got, ok := OdysseyExpandEquipMask(c.id)
		if ok != c.ok || got != c.want {
			t.Fatalf("%s: dungeon %d got (%d, %v), want (%d, %v)", c.desc, c.id, got, ok, c.want, c.ok)
		}
	}
}

// 解锁位与槽位号不是同一个值（耳环的奖励标量是 2，位是 4）；这里把两者钉死，
// 以免日后有人"顺手"把标量当成掩码 OR 进去。
func TestOdysseyExpandEquipMaskBitsMatchSlots(t *testing.T) {
	if inventory.ExpandSupport != 1<<0 {
		t.Fatalf("support bit = %d, want 1 (slot 22)", inventory.ExpandSupport)
	}
	if inventory.ExpandMagicStone != 1<<1 {
		t.Fatalf("magic stone bit = %d, want 2 (slot 23)", inventory.ExpandMagicStone)
	}
	if inventory.ExpandEarring != 1<<4 {
		t.Fatalf("earring bit = %d, want 16 (slot 25)", inventory.ExpandEarring)
	}
	if all := inventory.ExpandSupport | inventory.ExpandMagicStone | inventory.ExpandEarring; all != 19 {
		t.Fatalf("三个槽全解锁应为 19 (1|2|16)，实际 %d", all)
	}
}
