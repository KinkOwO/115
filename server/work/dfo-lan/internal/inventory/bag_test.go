package inventory

import (
	"dfolan/internal/catalog"
	"encoding/json"
	"math"
	"testing"
)

func TestBagAtomicCapacityAndPreservesState(t *testing.T) {
	c, e := catalog.LoadLoot("../../configs/loot.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	rules, e := LoadBagRules("../../configs/inventory.compat90.json")
	if e != nil {
		t.Fatal(e)
	}
	var id uint32
	for k, v := range c.Items {
		if v.StackableType == "[throw]" {
			id = k
			break
		}
	}
	if id == 0 {
		t.Fatal("missing real source throwable")
	}
	raw := json.RawMessage(`{"future_field":{"keep":true},"level":3,"experience":2490}`)
	b, e := ReadBag(raw)
	if e != nil {
		t.Fatal(e)
	}
	b, slot, e := b.Add(c, rules, id, 1)
	if e != nil || slot != 65 {
		t.Fatal(slot, e)
	}
	b, slot, e = b.Add(c, rules, id, 2)
	if e != nil || slot != 65 || b.Items[0].Amount != 3 {
		t.Fatal(b, e)
	}
	b, _, e = b.Add(c, rules, 0, 34)
	if e != nil {
		t.Fatal(e)
	}
	saved, e := SaveBag(raw, b)
	if e != nil {
		t.Fatal(e)
	}
	var fields map[string]json.RawMessage
	json.Unmarshal(saved, &fields)
	if string(fields["future_field"]) != `{"keep":true}` || string(fields["experience"]) != "2490" {
		t.Fatal("unrelated state lost")
	}
	restored, e := ReadBag(saved)
	if e != nil || restored.Gold != 34 || len(restored.Items) != 1 {
		t.Fatal(restored, e)
	}
	full := Bag{Version: "ordinary-bag-v1", Gold: math.MaxUint32}
	for n := 65; n <= 120; n++ {
		full.Items = append(full.Items, BagItem{Slot: uint16(n), Template: id, Amount: rules.MissingStackLimit})
	}
	before, _ := json.Marshal(full)
	if _, _, e = full.Add(c, rules, id, 1); e == nil {
		t.Fatal("full bag awarded")
	}
	if _, _, e = full.Add(c, rules, 0, 1); e == nil {
		t.Fatal("gold overflow accepted")
	}
	after, _ := json.Marshal(full)
	if string(before) != string(after) {
		t.Fatal("failed grant mutated bag")
	}
}

func TestBagCoinWalletAndConsolidation(t *testing.T) {
	c, e := catalog.LoadLoot("../../configs/loot.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	rules, e := LoadBagRules("../../configs/inventory.compat90.json")
	if e != nil {
		t.Fatal(e)
	}

	// 1. Add Life Token (template 1)
	b := Bag{Version: "ordinary-bag-v1", Gold: 1000}
	b, slot, e := b.Add(c, rules, 1, 10)
	if e != nil || slot != 1 || b.Coin != 10 {
		t.Fatalf("expected slot 1, coin 10, got slot=%d coin=%d err=%v", slot, b.Coin, e)
	}
	if len(b.Items) != 0 {
		t.Fatalf("coin must not be in b.Items, got %d items", len(b.Items))
	}

	// 2. Rows includes slot 0 (gold) and slot 1 (coin)
	rows := b.Rows()
	if len(rows) < 2 {
		t.Fatalf("expected at least 2 rows (gold + coin), got %d", len(rows))
	}
	slot0 := uint16(rows[0][0]) | uint16(rows[0][1])<<8
	slot1 := uint16(rows[1][0]) | uint16(rows[1][1])<<8
	if slot0 != 0 || slot1 != 1 {
		t.Fatalf("row slots mismatch: %d, %d", slot0, slot1)
	}

	// 3. ReadBag legacy consolidation
	legacyRaw := json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","gold":500,"coin":5,"items":[{"slot":65,"Template":1,"Amount":20},{"slot":66,"Template":15,"Amount":1}]}}`)
	restored, e := ReadBag(legacyRaw)
	if e != nil {
		t.Fatal(e)
	}
	if restored.Coin != 25 { // 5 + 20
		t.Fatalf("expected coin 25 after consolidation, got %d", restored.Coin)
	}
	if len(restored.Items) != 1 || restored.Items[0].Template != 15 {
		t.Fatalf("expected 1 item (template 15), got %+v", restored.Items)
	}

	// 4. Consume coin
	consumed, rem, e := restored.Consume(c, 1, 1)
	if e != nil || rem != 24 || consumed.Coin != 24 {
		t.Fatalf("consume coin failed: rem=%d coin=%d err=%v", rem, consumed.Coin, e)
	}
}

func TestQuestStackRoutesAndSavedSlotSweep(t *testing.T) {
	c, err := catalog.LoadLoot("../../configs/loot.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SupplementStackables("../../configs/items.index.json"); err != nil {
		t.Fatal(err)
	}
	rules, err := LoadBagRules("../../configs/inventory.current37.json")
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Items[4790].StackableType; got != "[quest]" {
		t.Fatalf("quest source changed: %q", got)
	}
	bag := Bag{Version: "ordinary-bag-v1"}
	bag, slot, err := bag.Add(c, rules, 4790, 3)
	if err != nil || slot != 177 {
		t.Fatalf("ordinary grant: slot=%d err=%v", slot, err)
	}
	mail := MailItem{Stack: &BagItem{Template: 4790, Amount: 2}}
	bag, err = bag.AddMailItem(c, rules, nil, mail)
	if err != nil || len(bag.Items) != 1 || bag.Items[0].Amount != 5 {
		t.Fatalf("mail grant: bag=%+v err=%v", bag.Items, err)
	}
	old := Bag{Version: "ordinary-bag-v1", Items: []BagItem{
		{Slot: 65, Template: 4790, Amount: 3},
		{Slot: 177, Template: 4790, Amount: 2},
		{Slot: 121, Template: 10418028, Amount: 5},
	}}
	fixed, moved, err := SweepStackSlots(old, c, rules)
	if err != nil || !moved {
		t.Fatalf("sweep: moved=%v err=%v", moved, err)
	}
	if len(fixed.Items) != 2 || fixed.Items[0].Slot != 177 || fixed.Items[0].Amount != 5 || fixed.Items[1] != old.Items[2] {
		t.Fatalf("unexpected sweep result: %+v", fixed.Items)
	}
	again, moved, err := SweepStackSlots(fixed, c, rules)
	if err != nil || moved || len(again.Items) != len(fixed.Items) {
		t.Fatalf("sweep must be idempotent: moved=%v err=%v", moved, err)
	}
}

func TestStackSlotSweepPreservesExpiryAndRollsBackWhenFull(t *testing.T) {
	c, err := catalog.LoadLoot("../../configs/loot.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SupplementStackables("../../configs/items.index.json"); err != nil {
		t.Fatal(err)
	}
	rules, err := LoadBagRules("../../configs/inventory.current37.json")
	if err != nil {
		t.Fatal(err)
	}
	old := Bag{Version: "ordinary-bag-v1", Items: []BagItem{
		{Slot: 65, Template: 4790, Amount: 3, ExpireTime: 100},
		{Slot: 177, Template: 4790, Amount: 2, ExpireTime: 200},
	}}
	fixed, moved, err := SweepStackSlots(old, c, rules)
	if err != nil || !moved || len(fixed.Items) != 2 || fixed.Items[1].Slot != 178 || fixed.Items[1].ExpireTime != 100 {
		t.Fatalf("expiry changed during sweep: %+v, %v", fixed.Items, err)
	}
	for slot := uint16(178); slot <= 232; slot++ {
		old.Items = append(old.Items, BagItem{Slot: slot, Template: 4790, Amount: 1000, ExpireTime: 200})
	}
	result, moved, err := SweepStackSlots(old, c, rules)
	if err == nil || moved || len(result.Items) != len(old.Items) || result.Items[0] != old.Items[0] {
		t.Fatalf("full category must preserve saved bag: moved=%v err=%v", moved, err)
	}
}
