package npcpresence

import "testing"

// Expected transitions below come from native functions, not a reconstructed
// screenshot: hidden-list loading146D0FA55 -> RequestHide; source show1
// consumer144F3A877 -> RequestShow; entity hidden set insertion145978490 and
// deletion145982020 confirm polarity. These are static vectors, not live ACKs.
func TestHiddenListAndSourceShowHaveOppositePolarity(t *testing.T) {
	r := NewConstructedVisibilityReplay()
	r.RequestHide(670)
	if got := r.Snapshot(670); got.LogicalShow != False || got.EntityVisible != False {
		t.Fatalf("hidden-list operation: %+v", got)
	}
	r.RequestShow(670)
	if got := r.Snapshot(670); got.LogicalShow != True || got.EntityVisible != True {
		t.Fatalf("source show operation: %+v", got)
	}
}

func TestTwoQuestOverridesKeepHiddenRequestVisibleUntilLastRemoval(t *testing.T) {
	r := NewConstructedVisibilityReplay()
	r.RequestHide(469)
	for _, quest := range []uint32{7957, 7959, 7959} {
		r.AddShowOverride(469, quest)
		r.ShowEntity(469)
	}
	r.RequestHide(469)
	if got := r.Snapshot(469); got.EntityVisible != True || got.LogicalShow != False {
		t.Fatalf("override must preserve entity display despite logical hide: %+v", got)
	}
	r.RemoveShowOverride(469, 7957)
	r.RemoveShowOverride(469, 9999)
	if got := r.Snapshot(469); got.EntityVisible != True || got.ShowOverride != True {
		t.Fatalf("other quest must retain override: %+v", got)
	}
	r.RemoveShowOverride(469, 7959)
	if got := r.Snapshot(469); got.EntityVisible != False || got.ShowOverride != False {
		t.Fatalf("last unique quest restores logical hide: %+v", got)
	}
}

func TestOverrideRegistrationAndForceShowAreSeparateOperations(t *testing.T) {
	r := NewConstructedVisibilityReplay()
	r.RequestHide(469)
	r.AddShowOverride(469, 7957)
	if got := r.Snapshot(469).EntityVisible; got != False {
		t.Fatalf("registration alone changed entity: %v", got)
	}
	r.ShowEntity(469)
	if got := r.Snapshot(469); got.EntityVisible != True || got.LogicalShow != False {
		t.Fatalf("force show must preserve logical state: %+v", got)
	}
}

func TestLastOverrideRemovalWithoutLogicalRecordDoesNotInventShow(t *testing.T) {
	r := NewConstructedVisibilityReplay()
	r.AddShowOverride(469, 7957)
	r.RemoveShowOverride(469, 7957)
	if got := r.Snapshot(469); got.LogicalShow != Unknown || got.EntityVisible != Unknown {
		t.Fatalf("missing logical record: %+v", got)
	}
	r.AddShowOverride(469, 7957)
	r.ShowEntity(469)
	r.RemoveShowOverride(469, 7957)
	if got := r.Snapshot(469); got.EntityVisible != True || got.LogicalShow != Unknown {
		t.Fatalf("native absence leaves existing entity display unchanged: %+v", got)
	}
}

func TestUnknownSnapshotDiffersFromVerifiedEmptyOverrideTable(t *testing.T) {
	r := NewVisibilityReplay()
	r.RequestHide(469)
	if got := r.Snapshot(469); got.LogicalShow != False || got.EntityVisible != Unknown {
		t.Fatalf("missing override snapshot: %+v", got)
	}
	r.AddShowOverride(469, 7957)
	r.RequestHide(469)
	if got := r.Snapshot(469).EntityVisible; got != True {
		t.Fatalf("known positive override suffices: %v", got)
	}
	r.RemoveShowOverride(469, 7957)
	if got := r.Snapshot(469).EntityVisible; got != Unknown {
		t.Fatalf("unobserved other quests may still override: %v", got)
	}
	r.Forget()
	if got := r.Snapshot(469); got.LogicalShow != Unknown || got.EntityVisible != Unknown || got.ShowOverride != Unknown {
		t.Fatalf("gap retained stale knowledge: %+v", got)
	}
}

func TestBatchUsesSignedNativeRankAndFirstInsertedTie(t *testing.T) {
	r := NewConstructedVisibilityReplay()
	for _, e := range []Effect{{670, 999, false}, {670, 10, true}, {670, 10, false}} {
		r.AppendEffect(e)
	}
	winners := r.ResolveBatch(map[uint32]Rank{
		999: {RankKnown, 37, 20}, 10: {RankKnown, 46, -1},
	})
	if winners[670].Quest != 10 || !winners[670].Show || r.Snapshot(670).EntityVisible != True {
		t.Fatalf("rank must use parent before sequence, not quest ID; first tie wins: %+v", winners)
	}
	r.RequestHide(670)
	if winners = r.ResolveBatch(nil); len(winners) != 0 || r.Snapshot(670).EntityVisible != False {
		t.Fatal("consumed batch must not replay old effects")
	}
}

func TestKnownIneligibleAndZeroBaselinePreserveImmediateState(t *testing.T) {
	r := NewConstructedVisibilityReplay()
	r.RequestShow(670)
	for _, e := range []Effect{{670, 1, false}, {670, 2, false}, {670, 3, false}} {
		r.AppendEffect(e)
	}
	winners := r.ResolveBatch(map[uint32]Rank{
		1: {Status: RankIneligible}, 2: {RankKnown, 0, 0}, 3: {RankKnown, -1, 100},
	})
	if len(winners) != 0 || r.Snapshot(670).EntityVisible != True {
		t.Fatal("no rank greater than native(0,0) must preserve immediate state")
	}
}

func TestMissingRankIsUnknownAndDoesNotPoisonOtherNPCs(t *testing.T) {
	r := NewConstructedVisibilityReplay()
	r.AppendEffect(Effect{670, 1, true})
	r.AppendEffect(Effect{670, 2, false})
	r.AppendEffect(Effect{308, 3, true})
	winners := r.ResolveBatch(map[uint32]Rank{1: {RankKnown, 100, 1}, 3: {RankKnown, 1, 1}})
	if _, ok := winners[670]; ok || r.Snapshot(670).EntityVisible != Unknown {
		t.Fatal("missing metadata evidence must not be silently skipped")
	}
	if winners[308].Quest != 3 || r.Snapshot(308).EntityVisible != True {
		t.Fatal("different NPC has independently complete evidence")
	}
	r.RequestShow(670)
	if r.Snapshot(670).EntityVisible != True {
		t.Fatal("explicit show restores known visibility after a rank gap")
	}
}

func TestBatchHideRespectsExistingQuestShowOverride(t *testing.T) {
	r := NewConstructedVisibilityReplay()
	r.AddShowOverride(469, 7957)
	r.ShowEntity(469)
	r.AppendEffect(Effect{469, 100, false})
	r.ResolveBatch(map[uint32]Rank{100: {RankKnown, 9, 9}})
	if got := r.Snapshot(469); got.LogicalShow != False || got.EntityVisible != True {
		t.Fatalf("batch winner hide retains active override: %+v", got)
	}
}
