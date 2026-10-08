package loot

import "testing"

func TestOmenLedgerRestoresMissesAndPublishesPityOnce(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	seed := omenSeedWithOutcome(t, a, 0, false, false)
	o := NewOmenLedger(a)
	o.SetState(1, 0, OmenPityMisses-2)
	out, _, err := o.Advance(1, omenDungeon, seed)
	if err != nil || out.Pity || o.Misses(1) != OmenPityMisses-1 {
		t.Fatalf("29th miss: outcome=%+v misses=%d err=%v", out, o.Misses(1), err)
	}
	// A new ledger models restarting the process and restoring the saved counters.
	restored := NewOmenLedger(a)
	restored.SetState(1, o.Held(1), o.Misses(1))
	out, _, err = restored.preview(1, omenDungeon, seed)
	if err != nil || !out.Pity || out.After != 1 || out.MissesAfter != 0 {
		t.Fatalf("30th miss preview: outcome=%+v err=%v", out, err)
	}
	if restored.Held(1) != 0 || restored.Misses(1) != OmenPityMisses-1 {
		t.Fatal("preview consumed the saved counters")
	}
	if err := restored.commit(1, out); err != nil {
		t.Fatal(err)
	}
	if restored.Held(1) != 1 || restored.Misses(1) != 0 {
		t.Fatal("published pity did not update both counters")
	}
	if err := restored.commit(1, out); err == nil {
		t.Fatal("same pity outcome was committed twice")
	}
}

func TestOmenLedgerRejectsConcurrentMissCounterChange(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	o := NewOmenLedger(a)
	o.SetState(1, 0, 10)
	out, _, err := o.preview(1, omenDungeon, omenSeedWithOutcome(t, a, 0, false, false))
	if err != nil {
		t.Fatal(err)
	}
	o.SetState(1, 0, 11)
	if err := o.commit(1, out); err == nil {
		t.Fatal("concurrent miss counter change was overwritten")
	}
	if o.Misses(1) != 11 {
		t.Fatal("rejected commit changed the current counter")
	}
}
