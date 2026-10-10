package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"sort"
	"strings"
	"testing"
)

// The cascade used to be paid for round by round against the database: complete a
// quest, re-read every quest row the character owns, walk the index again. These
// pin the replacement - a closure over the prerequisite graph computed in memory -
// to the behaviour the rounds produced, without a database in the loop.
func closureIndex(entries ...*Entry) *Index {
	// The service's lazy Index() reuses the injected index only when its Source
	// matches the process save identity, so the fixture carries that identity.
	x := &Index{Source: pvf.ArchiveSnapshot{}.SaveIdentity(), Entries: map[uint32]*Entry{}}
	for _, e := range entries {
		x.Entries[e.ID] = e
		x.Ordered = append(x.Ordered, e.ID)
	}
	sort.Slice(x.Ordered, func(i, j int) bool { return x.Ordered[i] < x.Ordered[j] })
	return x
}

func closureService(x *Index, blocked map[uint32]bool) *Service {
	return &Service{
		Catalog: catalog.QuestCatalog{Source: pvf.ArchiveSnapshot{}},
		Skip:    &catalog.QuestSkip{Blocked: blocked},
		index:   x,
	}
}

// epicEntry is a quest that passes every gate except the ones a case sets.
func epicEntry(id uint32, prerequisite ...uint32) *Entry {
	e := &Entry{
		ID: id, Grade: "[epic]", Implemented: true, RewardUsable: true, GrowUsable: true,
		TargetUsable: true, MinimumLevel: 1, MaximumLevel: 10000, Prerequisites: prerequisite,
	}
	if len(prerequisite) > 0 {
		// One block: the ids inside it are a conjunction, which is what a plain
		// prerequisite list used to mean.
		e.PrerequisiteGroups = [][]uint32{append([]uint32{}, prerequisite...)}
	}
	return e
}

var closureState = character.State{Level: 111, Advancement: 3, Awakening: 2}

// A chain whose ids run backwards against the story has to settle in one press.
// Id order is the only order the index offers, so a single pass would take 3 and
// stop; the closure repeats until a pass adds nothing.
func TestSkipClosureFollowsTheChainWhateverOrderTheIdsAreIn(t *testing.T) {
	s := closureService(closureIndex(epicEntry(1, 3), epicEntry(2, 1), epicEntry(3)), nil)
	order, excluded, stopped := s.skipClosure("[mage]", closureState, map[uint32]string{}, MainlineGrades(), 0)
	if stopped != "exhausted" {
		t.Fatalf("stopped=%s, want exhausted", stopped)
	}
	if len(excluded) != 0 {
		t.Fatalf("excluded %v, want none", excluded)
	}
	want := []uint32{3, 1, 2}
	if len(order) != len(want) {
		t.Fatalf("settled %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("settled %v, want %v", order, want)
		}
	}
}

// What the character already finished is not settled again, and a quest behind an
// unfinished prerequisite is not reached: the closure has to agree with the offer
// list on both, or one press would either double-pay or skip ahead of the story.
func TestSkipClosureRespectsCompletedRowsAndPrerequisites(t *testing.T) {
	s := closureService(closureIndex(epicEntry(1), epicEntry(2, 1), epicEntry(3, 2)), nil)
	order, _, _ := s.skipClosure("[mage]", closureState, map[uint32]string{1: "completed"}, MainlineGrades(), 0)
	if len(order) != 2 || order[0] != 2 || order[1] != 3 {
		t.Fatalf("settled %v, want [2 3]", order)
	}
	// 1 is only accepted, not completed, so 2 stays locked: an accepted quest is
	// settled by this press but does not unlock anything until it is.
	order, _, _ = s.skipClosure("[mage]", closureState, map[uint32]string{}, MainlineGrades(), 0)
	if len(order) != 3 {
		t.Fatalf("settled %v, want the whole chain", order)
	}
}

// The grade filter is the boundary a bulk sweep once got wrong: it cleared 65 side
// quests a "complete the main quest" button has no business touching. An excluded
// quest is reported and, because it is not completed, holds back its dependants.
func TestSkipClosureLeavesOtherGradesAlone(t *testing.T) {
	side := epicEntry(2)
	side.Grade = "[side]"
	behind := epicEntry(3, 2)
	s := closureService(closureIndex(epicEntry(1), side, behind), nil)
	order, excluded, _ := s.skipClosure("[mage]", closureState, map[uint32]string{}, MainlineGrades(), 0)
	if len(order) != 1 || order[0] != 1 {
		t.Fatalf("settled %v, want [1]", order)
	}
	if len(excluded) != 1 || excluded[0].Quest != 2 || !excluded[0].Excluded || excluded[0].Grade != "[side]" {
		t.Fatalf("excluded %+v, want the side quest named once", excluded)
	}
}

// list/notclearquestbyticket.lst is the source's own "this one has to be played"
// list. A quest on it is not settled and nothing behind it is either, which is
// what makes the frontier report the place the story actually stopped.
func TestSkipClosureHonoursTheSourceBlacklist(t *testing.T) {
	s := closureService(closureIndex(epicEntry(1), epicEntry(2, 1)), map[uint32]bool{1: true})
	order, _, _ := s.skipClosure("[mage]", closureState, map[uint32]string{}, MainlineGrades(), 0)
	if len(order) != 0 {
		t.Fatalf("settled %v, want nothing", order)
	}
}

// The audience gates are the reason the main line dead-ended at 3231: a quest that
// names another class has to stay closed, and one that names this class through
// [target character] alone has to open.
func TestSkipClosureAppliesTheAudienceGates(t *testing.T) {
	thief := epicEntry(1)
	thief.Jobs = []string{"[thief]"}
	mageByTarget := epicEntry(2)
	mageByTarget.TargetCharacters = []targetCharacter{{job: "[mage]", advancement: -1, awakening: -1}}
	otherBranch := epicEntry(4)
	otherBranch.TargetCharacters = []targetCharacter{{job: "[mage]", advancement: 1, awakening: -1}}
	s := closureService(closureIndex(thief, mageByTarget, otherBranch), nil)
	order, _, _ := s.skipClosure("[mage]", closureState, map[uint32]string{}, MainlineGrades(), 0)
	if len(order) != 1 || order[0] != 2 {
		t.Fatalf("settled %v, want [2] - only the quest [target character] opens to a mage on this branch", order)
	}
	// The same quest is closed to a class it does not list, which is what keeps one
	// class from being handed another class's story.
	if order, _, _ = s.skipClosure("[thief]", closureState, map[uint32]string{}, MainlineGrades(), 0); len(order) != 1 || order[0] != 1 {
		t.Fatalf("a thief settled %v, want [1] - the mage-targeted quests stay shut", order)
	}
}

// The cap is what makes a first live run of an unlimited button measurable instead
// of a bet, so it has to stop mid-chain and say so.
func TestSkipClosureStopsAtTheLimit(t *testing.T) {
	s := closureService(closureIndex(epicEntry(1), epicEntry(2, 1), epicEntry(3, 2)), nil)
	order, _, stopped := s.skipClosure("[mage]", closureState, map[uint32]string{}, MainlineGrades(), 2)
	if stopped != "limit" || len(order) != 2 {
		t.Fatalf("stopped=%s settled=%v, want limit and two quests", stopped, order)
	}
}

// A row the character already holds is settled whatever its gates say now: the
// offer list makes the same exception, and refusing it here would leave a quest in
// the log that nothing can finish.
func TestSkipClosureSettlesAnAcceptedRowPastItsGates(t *testing.T) {
	tooHigh := epicEntry(1)
	tooHigh.MinimumLevel = 200
	s := closureService(closureIndex(tooHigh), nil)
	if order, _, _ := s.skipClosure("[mage]", closureState, map[uint32]string{1: "accepted"}, MainlineGrades(), 0); len(order) != 1 {
		t.Fatalf("settled %v, want the accepted row", order)
	}
	if order, _, _ := s.skipClosure("[mage]", closureState, map[uint32]string{}, MainlineGrades(), 0); len(order) != 0 {
		t.Fatalf("settled %v, want nothing while the level gate holds", order)
	}
}

// 12159 theoculus_14.qst writes three prerequisite blocks - [12158 3870],
// [12158 3871], [12158 3872] - and 3870/3871/3872 are the same story beat for
// three different classes, two of which are mutually exclusive by audience. No
// character can satisfy more than one block, so the blocks are alternatives and
// flattening them into a list makes the quest permanently unreachable.
func TestSkipClosureTakesAnyOnePrerequisiteBlock(t *testing.T) {
	otherClass := epicEntry(4)
	otherClass.Jobs = []string{"[thief]"}
	alternatives := epicEntry(3)
	alternatives.PrerequisiteGroups = [][]uint32{{1, 2}, {1, 4}}
	alternatives.Prerequisites = []uint32{1, 2, 4}
	s := closureService(closureIndex(epicEntry(1), epicEntry(2), otherClass, alternatives), nil)
	order, _, stopped := s.skipClosure("[mage]", closureState, map[uint32]string{}, MainlineGrades(), 0)
	if stopped != "exhausted" {
		t.Fatalf("stopped=%s, want exhausted", stopped)
	}
	want := map[uint32]bool{1: true, 2: true, 3: true}
	if len(order) != len(want) {
		t.Fatalf("settled %v, want %v", order, want)
	}
	for _, id := range order {
		if !want[id] {
			t.Fatalf("settled %v, want %v", order, want)
		}
	}
}

// Alternatives are not a licence to ignore prerequisites: a block whose ids are
// not all complete still holds the quest back.
func TestSkipClosureStillRefusesAnUnsatisfiedBlock(t *testing.T) {
	behind := epicEntry(3)
	behind.PrerequisiteGroups = [][]uint32{{1, 2}}
	s := closureService(closureIndex(epicEntry(1), behind), nil)
	if order, _, _ := s.skipClosure("[mage]", closureState, map[uint32]string{}, MainlineGrades(), 0); len(order) != 1 || order[0] != 1 {
		t.Fatalf("settled %v, want [1] - 2 is missing so the block is not satisfied", order)
	}
}

// The source files also use an empty [pre required quest] section for "no
// prerequisite"; the catalog's parser drops those sections entirely
// (parsePrerequisiteGroups flushes nothing for an empty group), so the quest
// arrives with no groups and settles. A hand-built empty group is deliberately
// not exercised: the loader never emits one.
func TestSkipClosureReadsAnEmptyRequirementAsNoPrerequisite(t *testing.T) {
	empty := epicEntry(1)
	empty.PrerequisiteGroups = nil
	s := closureService(closureIndex(empty), nil)
	if order, _, _ := s.skipClosure("[mage]", closureState, map[uint32]string{}, MainlineGrades(), 0); len(order) != 1 {
		t.Fatalf("settled %v, want the quest with no prerequisite groups", order)
	}
}

// The frontier report is the only answer to "why did the story stop", so a real
// wall must survive the limit even when another class's quests have lower ids.
// Truncating during the walk - which runs in ascending id - filled the budget with
// "written for another character" and hid the blacklisted quest entirely, which
// read in the log as "no walls at all" while one was still standing.
func TestBlockedFrontierReportsWallsBeforeOtherClasses(t *testing.T) {
	otherClass := epicEntry(1)
	otherClass.Jobs = []string{"[thief]"}
	s := closureService(closureIndex(otherClass, epicEntry(9)), map[uint32]bool{9: true})
	got := s.blockedFrontier("[mage]", closureState, map[uint32]string{}, 1)
	if len(got) != 1 || got[0].Quest != 9 {
		t.Fatalf("frontier = %+v, want only the wall at 9", got)
	}
	if want := "list/notclearquestbyticket.lst forbids skipping it"; got[0].Reason != want {
		t.Fatalf("reason = %q, want %q", got[0].Reason, want)
	}
	// With room for both, the wall still comes first and the other class's quest is
	// labelled as what it is rather than as a gap in this build.
	got = s.blockedFrontier("[mage]", closureState, map[uint32]string{}, 5)
	if len(got) != 2 || got[0].Quest != 9 || got[1].Quest != 1 {
		t.Fatalf("frontier = %+v, want the wall at 9 then the other class's quest at 1", got)
	}
	if !strings.Contains(got[1].Reason, "written for another character") {
		t.Fatalf("reason = %q, want it to name the other class", got[1].Reason)
	}
}
