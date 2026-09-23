package inventory

import "testing"

// Spend is the counterpart of Add, and a skill cost is paid from the
// account-shared store, so the two have to agree on where a stack lives.
func TestAccountMaterialsSpend(t *testing.T) {
	materials, _, e := NewAccountMaterials().Add(3037, 50)
	if e != nil {
		t.Fatal(e)
	}
	swept, slot, e := materials.Add(3033, 4)
	if e != nil {
		t.Fatal(e)
	}
	if slot != 363 {
		t.Fatalf("slot = %d, want 363", slot)
	}
	materials = swept

	spent, slot, e := materials.Spend(3037, 15)
	if e != nil {
		t.Fatalf("Spend: %v", e)
	}
	if slot != 367 {
		t.Errorf("slot = %d, want 367", slot)
	}
	if got := spent.Count(3037); got != 35 {
		t.Errorf("count after spend = %d, want 35", got)
	}
	if got := spent.Count(3033); got != 4 {
		t.Errorf("unrelated stack changed: %d, want 4", got)
	}
	// The original is untouched: Spend returns a new value.
	if got := materials.Count(3037); got != 50 {
		t.Errorf("Spend mutated its receiver: %d, want 50", got)
	}

	// Spending the last of a stack drops the cell, because Rows omits zeros.
	emptied, _, e := spent.Spend(3037, 35)
	if e != nil {
		t.Fatal(e)
	}
	if _, present := emptied.Counts[367]; present {
		t.Errorf("zero-count cell retained: %v", emptied.Counts)
	}
}

func TestAccountMaterialsSpendRefusals(t *testing.T) {
	materials, _, e := NewAccountMaterials().Add(3037, 10)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e := materials.Spend(3037, 11); e == nil {
		t.Error("spending more than the stack holds was accepted")
	}
	if _, _, e := materials.Spend(3037, 0); e == nil {
		t.Error("spending zero was accepted")
	}
	// 3033 is 无色小晶块's sibling; 42 is not a shared material at all.
	if _, _, e := materials.Spend(42, 1); e == nil {
		t.Error("spending a non-shared template was accepted")
	}
	if got := materials.Count(3037); got != 10 {
		t.Errorf("a refused spend changed the count: %d, want 10", got)
	}
}
