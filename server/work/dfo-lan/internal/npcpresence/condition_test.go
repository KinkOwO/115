package npcpresence

import "testing"

// 144F3B900 first checks1176 protection, may enqueue a temporary show, then
// enqueues the source action.146D00BD0 preserves the first equal-rank effect.
// These expectations exercise their composition, not a collapsed last-write
// model. They intentionally do not claim to cover full CMD handler side effects.
func TestProtectedAndUnprotectedHideHaveDifferentFirstBatchEffects(t *testing.T) {
	block := VisibilityBlock{Condition: 1, Show: false, Revert: true, NPCs: []uint32{469}}
	for _, protected := range []bool{false, true} {
		r := NewConstructedVisibilityReplay()
		if protected {
			r.protected[469] = True
		}
		r.ApplyCondition(100, []VisibilityBlock{block}, 1, true)
		if got := r.Snapshot(469); got.LogicalShow != False || got.EntityVisible != False || got.Protected != False {
			t.Fatalf("immediate final source hide: %+v", got)
		}
		winner := r.ResolveBatch(map[uint32]Rank{100: {RankKnown, 1, 1}})[469]
		if winner.Show != protected {
			t.Fatalf("protected=%v first effect=%+v", protected, winner)
		}
	}
}

func TestDeleteProtectsSubsequentBlockWithoutCollapsingItsEffects(t *testing.T) {
	r := NewConstructedVisibilityReplay()
	blocks := []VisibilityBlock{
		{Condition: 0, Protect: true, Revert: true, NPCs: []uint32{469}},
		{Condition: 0, Revert: true, NPCs: []uint32{469}},
	}
	r.ApplyCondition(100, blocks, 0, true)
	if len(r.batch) != 4 || r.batch[0].Show || r.batch[1].Show || !r.batch[2].Show || r.batch[3].Show {
		t.Fatalf("native ordered delete/hide records: %+v", r.batch)
	}
	winner := r.ResolveBatch(map[uint32]Rank{100: {RankKnown, 1, 1}})[469]
	if winner.Show || r.Snapshot(469).EntityVisible != False {
		t.Fatal("first source hide must win")
	}
}

func TestRevertHonorsBlockFlagAndLeavesProtectionUnchanged(t *testing.T) {
	r := NewConstructedVisibilityReplay()
	r.protected[469] = True
	r.RequestHide(469)
	r.ApplyCondition(100, []VisibilityBlock{{Condition: 0, Show: true, Revert: false, NPCs: []uint32{469}}}, 0, false)
	if len(r.batch) != 0 || r.Snapshot(469).EntityVisible != False {
		t.Fatal("disabled revert changed state")
	}
	r.ApplyCondition(100, []VisibilityBlock{{Condition: 0, Show: false, Revert: true, NPCs: []uint32{469}}}, 0, false)
	if len(r.batch) != 1 || !r.batch[0].Show || r.Snapshot(469).Protected != True {
		t.Fatalf("inverse hide is show; revert does not clear protection: %+v", r.batch)
	}
}

func TestUnknownProtectionPreservesBatchAmbiguityAfterImmediateAction(t *testing.T) {
	r := NewVisibilityReplay()
	r.ApplyCondition(100, []VisibilityBlock{{Condition: 1, Show: false, Revert: true, NPCs: []uint32{469}}}, 1, true)
	if r.Snapshot(469).Protected != False {
		t.Fatal("native unprotect branch is explicit")
	}
	r.ResolveBatch(map[uint32]Rank{100: {RankKnown, 1, 1}})
	if r.Snapshot(469).EntityVisible != Unknown {
		t.Fatal("unobserved initial first effect could change tie winner")
	}
}

func TestConditionFilterAndProtectionResetAreIndependentOfShowOverride(t *testing.T) {
	r := NewConstructedVisibilityReplay()
	r.AddShowOverride(469, 100)
	r.ShowEntity(469)
	r.protected[469] = True
	r.ApplyCondition(100, []VisibilityBlock{{Condition: 3, Protect: true, NPCs: []uint32{469}}}, 2, true)
	if len(r.batch) != 0 {
		t.Fatal("clearing must not execute on clearable")
	}
	r.ClearProtection()
	if got := r.Snapshot(469); got.Protected != False || got.ShowOverride != True || got.EntityVisible != True {
		t.Fatalf("NOTI342 protection reset cleared unrelated visibility: %+v", got)
	}
}
