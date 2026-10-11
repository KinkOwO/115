package legion

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestStagedRunOwnershipRetryAndSequentialSettlement(t *testing.T) {
	s := &catalog.LegionContents{Source: pvf.ArchiveSnapshot{Checksum: "test"}, Contents: map[string]catalog.LegionContent{"DuskyIsland": {
		LastPhase: 2, Dungeons: []catalog.LegionDungeon{{Dungeon: 101, Boss: 201}, {Dungeon: 102, Boss: 202}, {Dungeon: 103, Boss: 203}}, NativeOperations: &catalog.NativeLegionOperations{Modes: map[string][]uint16{"normal": {1}}, Definitions: map[uint16]catalog.NativeLegionOperation{1: {MemberLimit: "party", SelectPool: 0, Probability: 1, Ranges: []catalog.NativeLegionRange{{Party: "party", Difficulty: "normal", Minimum: 0, Maximum: 0}}}}},
	}}}
	r, err := NewStagedRun(s, "DuskyIsland", -1, StagedOperationMode{"normal", "party", "normal"})
	if err != nil {
		t.Fatal(err)
	}
	offer := func() {
		t.Helper()
		if err := r.Offer(s, []catalog.NativeLegionCard{{Index: 1, Pool: 0}}); err != nil {
			t.Fatal(err)
		}
		if err := r.Select(1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = r.Entry(0); err == nil {
		t.Fatal("entry before operation selection")
	}
	offer()
	if err = r.Enter(0, 101, "retry"); err != nil {
		t.Fatal(err)
	}
	if err = r.RewardEnd("retry"); err == nil {
		t.Fatal("client reward request bypassed combat")
	}
	if err = r.Retreat("retry"); err != nil {
		t.Fatal(err)
	}
	if err = r.Complete("retry", 101); err == nil {
		t.Fatal("retreated dungeon retained clear authority")
	}
	for stage, run := range []string{"one", "two", "three"} {
		offer()
		if _, err = r.Entry(stage + 1); err == nil {
			t.Fatal("skipped source phase")
		}
		if err = r.Enter(stage, uint32(101+stage), run); err != nil {
			t.Fatal(err)
		}
		if err = r.Complete("another", uint32(101+stage)); err == nil {
			t.Fatal("foreign run completed source phase")
		}
		if err = r.Complete(run, uint32(101+stage)); err != nil {
			t.Fatal(err)
		}
		if err = r.Retreat(run); err == nil {
			t.Fatal("durable clear converted into retreat")
		}
		if err = r.RewardEnd(run); err != nil {
			t.Fatal(err)
		}
		if err = r.RewardEnd(run); err == nil {
			t.Fatal("stale settlement retained active owner")
		}
		if r.Stage() != stage+1 {
			t.Fatal("duplicate advanced the phase", r.Stage())
		}
	}
	if !r.Finished() {
		t.Fatal("three source fights did not end run")
	}
	if _, err = r.Entry(2); err == nil {
		t.Fatal("completed run reused for entry")
	}
}

func TestNativeProjectionPreservesOpaqueFieldsAndDistinctParameters(t *testing.T) {
	for id, size := range map[uint16]int{NotiCorridorInfo: 107, NotiDuskyInfo: 133} {
		body := make([]byte, size)
		for i := range body {
			body[i] = byte(i)
		}
		p, err := ReadStagedProjection(id, body)
		if err != nil {
			t.Fatal(err)
		}
		r := StagedOperationRecord{Index: 9, ParameterA: 30, ParameterB: 400, Tail: [4]byte{1, 2, 3, 4}}
		if err = p.SetOperation(0, r); err != nil {
			t.Fatal(err)
		}
		decoded, err := p.Operation(0)
		if err != nil || decoded != r {
			t.Fatal(decoded, err)
		}
		offset := 63
		if id == NotiDuskyInfo {
			offset = 59
		}
		out := p.Bytes()
		for i, v := range body {
			if (i < offset || i >= offset+10) && out[i] != v {
				t.Fatalf("unrelated native field overwritten at%d", i)
			}
		}
		out[0] = 0
		if p.Bytes()[0] != body[0] {
			t.Fatal("projection aliases caller")
		}
		if _, err = ReadStagedProjection(id, body[:size-1]); err == nil {
			t.Fatal("truncated projection accepted")
		}
	}
}

func TestDuskyProjectionSourceIndexPreservesUnknownDungeonBytes(t *testing.T) {
	body := make([]byte, 133)
	for i := range body {
		body[i] = byte(i*17 + 9)
	}
	p, err := ReadStagedProjection(NotiDuskyInfo, body)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.SetDungeonIndex(1, 2); err != nil {
		t.Fatal(err)
	}
	got := p.Bytes()
	for i := range body {
		if i != 35 && got[i] != body[i] {
			t.Fatalf("overwrote unknown dungeon byte at %d", i)
		}
	}
	index, err := p.DungeonIndex(1)
	if err != nil || index != 2 {
		t.Fatal(index, err)
	}
	if err = p.SetDungeonIndex(1, 256); err == nil {
		t.Fatal("silently truncated source index")
	}
}

func TestStagedRunRejectsCrossModeOperationAndRange(t *testing.T) {
	s := &catalog.LegionContents{Source: pvf.ArchiveSnapshot{Checksum: "mode"}, Contents: map[string]catalog.LegionContent{"DuskyIsland": {
		LastPhase: 2, Dungeons: []catalog.LegionDungeon{{Dungeon: 101, Boss: 201}, {Dungeon: 102, Boss: 202}, {Dungeon: 103, Boss: 203}},
		NativeOperations: &catalog.NativeLegionOperations{Modes: map[string][]uint16{"normal": {1}, "expert": {2}}, Definitions: map[uint16]catalog.NativeLegionOperation{
			1: {MemberLimit: "party", Probability: 1, Ranges: []catalog.NativeLegionRange{{Party: "party", Difficulty: "normal", Minimum: 1, Maximum: 1}, {Party: "party", Difficulty: "expert", Minimum: 1.5, Maximum: 1.5}}},
			2: {MemberLimit: "party", Probability: 1, Ranges: []catalog.NativeLegionRange{{Party: "party", Difficulty: "normal", Minimum: 1, Maximum: 1}}},
		}},
	}}}
	r, err := NewStagedRun(s, "DuskyIsland", -1, StagedOperationMode{"normal", "party", "normal"})
	if err != nil {
		t.Fatal(err)
	}
	for _, card := range []catalog.NativeLegionCard{{Index: 2, Parameter: 1}, {Index: 1, Parameter: 1.5}} {
		if err = r.Offer(s, []catalog.NativeLegionCard{card}); err == nil {
			t.Fatal("foreign mode/range obtained offer authority", card)
		}
	}
	if err = r.Offer(s, []catalog.NativeLegionCard{{Index: 1, Parameter: 1}}); err != nil {
		t.Fatal(err)
	}
}
