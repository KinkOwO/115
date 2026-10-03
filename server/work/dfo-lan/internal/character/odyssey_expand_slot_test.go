package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"testing"
)

// 奥德赛角色不走任务链路，扩展装备槽靠通关指定副本解锁；
// 规则来源见 analysis/tasks/next50-odyssey-expanded-equip-slot.md §1。
func TestOdysseyExpandEquipMask(t *testing.T) {
	r, e := catalog.LoadOdysseyGrowth("../../configs/odyssey-growth-release.json")
	if e != nil {
		t.Fatal(e)
	}
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
		{100004980, inventory.ExpandSupport | inventory.ExpandMagicStone | inventory.ExpandEarring, true, "源 110 级全槽解锁"},
	}
	for _, c := range cases {
		got := odysseySlotActionMask(r.LevelActions[r.ClearLevels[c.id]])
		ok := got != 0
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

func TestOdysseyGrowthExecutesCrossedSlotActions(t *testing.T) {
	s, r := odysseyGrowthFixture(t)
	s.Odyssey.LevelActions = map[byte][]string{12: {"unlock support"}, 14: {"unlock magic stone"}}
	next, e := s.ApplyOdysseyTarget(r, 15)
	if e != nil {
		t.Fatal(e)
	}
	b, e := inventory.ReadBag(next.State)
	if e != nil {
		t.Fatal(e)
	}
	if b.ExpandEquipFlags != inventory.ExpandSupport|inventory.ExpandMagicStone {
		t.Fatalf("crossed source actions not executed: %+v", b)
	}
	r.Request = nil
	if _, e = s.ApplyOdysseyTarget(r, 15); e == nil {
		t.Fatal("ordinary mode consumed Odyssey level actions")
	}
}

func TestOdysseySlotActionsReconcileReachedLevelsWithoutRegrant(t *testing.T) {
	s, r := odysseyGrowthFixture(t)
	s.Odyssey.LevelActions = map[byte][]string{12: {"unlock support"}}
	next, e := s.ApplyOdysseyTarget(r, 15)
	if e != nil {
		t.Fatal(e)
	}
	before := append([]byte(nil), next.State...)
	again, e := s.ApplyOdysseyTarget(next, 15)
	if e != nil {
		t.Fatal(e)
	}
	if string(again.State) != string(before) {
		t.Fatal("repeat action changed state")
	}
	s.Odyssey.LevelActions = map[byte][]string{12: {"unlock earring"}}
	changed, e := s.ApplyOdysseyTarget(next, 15)
	if e != nil {
		t.Fatal(e)
	}
	b, e := inventory.ReadBag(changed.State)
	if e != nil {
		t.Fatal(e)
	}
	if b.ExpandEquipFlags != inventory.ExpandSupport|inventory.ExpandEarring {
		t.Fatal("updated source action did not reconcile", b.ExpandEquipFlags)
	}
}
