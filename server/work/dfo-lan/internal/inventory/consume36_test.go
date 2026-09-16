package inventory

import (
	"dfolan/internal/catalog"
	"testing"
)

func TestConsumeChecksOwnershipAndStack(t *testing.T) {
	c, e := catalog.LoadLoot("../../configs/loot.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	var stackable uint32
	for id, v := range c.Items {
		if v.Kind == "stackable" && (stackable == 0 || id < stackable) {
			stackable = id
		}
	}
	if stackable == 0 {
		t.Fatal("no source stackable")
	}
	b := Bag{Version: "ordinary-bag-v1", Items: []BagItem{{Slot: 7, Template: stackable, Amount: 2}}}

	after, remaining, e := b.Consume(c, 7, stackable)
	if e != nil || remaining != 1 {
		t.Fatalf("first use: remaining=%d err=%v", remaining, e)
	}
	after, remaining, e = after.Consume(c, 7, stackable)
	if e != nil || remaining != 0 {
		t.Fatalf("second use: remaining=%d err=%v", remaining, e)
	}
	// An emptied slot is removed, the way every other bag path marks absence.
	for _, row := range after.Items {
		if row.Slot == 7 {
			t.Fatal("emptied slot was left in the bag")
		}
	}
	if _, _, e = after.Consume(c, 7, stackable); e == nil {
		t.Fatal("consumed from an empty slot")
	}
	if _, _, e = b.Consume(c, 8, stackable); e == nil {
		t.Fatal("consumed from a slot the bag does not own")
	}
	if _, _, e = b.Consume(c, 7, stackable+999999); e == nil {
		t.Fatal("consumed an item the slot does not hold")
	}
	// Gear is not a stackable and must not be spendable through this path.
	gear := Bag{Version: "ordinary-bag-v1", Equipment: []BagEquipment{{Slot: 40, Template: stackable}}}
	if _, _, e = gear.Consume(c, 40, stackable); e == nil {
		t.Fatal("consumed an equipment slot")
	}
}
