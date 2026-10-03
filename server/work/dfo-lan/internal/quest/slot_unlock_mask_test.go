package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/testfixture"
	"testing"
)

// [slot expansion] 的 [reward int data] 是槽索引，存档里要写的是解锁位。
// 两者只在 support 上巧合相等：耳环的索引 2 对应位 4，而魔法石的索引 1 对应位 2。
// 一旦有人把索引直接当位用，耳环任务会"完成但不解锁"，且没有任何报错。
func TestSlotUnlockMaskIsNotTheRewardScalar(t *testing.T) {
	c, e := catalog.LoadQuests(testfixture.CatalogPath(t, "quests"))
	if e != nil {
		t.Fatal(e)
	}
	for id, want := range map[uint32]byte{649: ExpandSupport, 650: ExpandMagicStone, 2636: ExpandEarring} {
		d, ok := c.Quests[id]
		if !ok {
			t.Fatalf("source quest %d missing", id)
		}
		slot, ok := slotExpansion(d)
		if !ok {
			t.Fatalf("quest %d is not recognised as a slot expansion", id)
		}
		got, ok := slotUnlockMask(slot)
		if !ok {
			t.Fatalf("quest %d slot index %d has no unlock bit", id, slot)
		}
		if got != want {
			t.Fatalf("quest %d: index %d -> mask %d, want %d", id, slot, got, want)
		}
	}
	// 索引 2 与位 4 的差别必须被钉住：这是最容易被写成 |= slot 的地方。
	if slot, _ := slotExpansion(c.Quests[2636]); slot == ExpandEarring {
		t.Fatal("the earring index and its unlock bit are no longer distinguishable")
	}
	// 值域外的索引没有解锁位。
	for _, slot := range []byte{3, 4, 255} {
		if _, ok := slotUnlockMask(slot); ok {
			t.Fatalf("slot index %d produced an unlock bit", slot)
		}
	}
	// 三个任务依次完成：0 -> 1 -> 3 -> 19。
	var acc byte
	for _, id := range []uint32{649, 650, 2636} {
		slot, _ := slotExpansion(c.Quests[id])
		mask, _ := slotUnlockMask(slot)
		acc |= mask
	}
	if acc != 1|2|16 {
		t.Fatalf("all three quests leave %d, want 19", acc)
	}
}
