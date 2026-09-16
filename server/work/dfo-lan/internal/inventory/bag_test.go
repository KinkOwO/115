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
		full.Items = append(full.Items, BagItem{uint16(n), id, rules.MissingStackLimit})
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
