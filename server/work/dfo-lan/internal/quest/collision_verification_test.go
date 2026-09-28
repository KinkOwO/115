package quest

import (
	"dfolan/internal/catalog"
	"slices"
	"testing"
)

// simulateOffered mirrors Service.Available's per-quest decision (available.go)
// with the real index entries and the real gate helpers, so the faction
// scenarios below verify the server's actual offer logic without a database.
func simulateOffered(x *Index, status map[uint32]string, id uint32, level uint32, job string, advancement, awakening byte) bool {
	if id == 0 || id >= 40000 || status[id] == "completed" {
		return false
	}
	if status[id] == "accepted" {
		return true
	}
	en := x.Entries[id]
	if !en.Implemented || !en.RewardUsable || !en.GrowUsable || !en.TargetUsable {
		return false
	}
	if level < en.MinimumLevel || level > en.MaximumLevel {
		return false
	}
	allowed := jobAllowed(en.Jobs, job) && targetCharacterAllowed(en.TargetCharacters, job, advancement, awakening)
	if !prerequisitesMet(en.PrerequisiteGroups, status) {
		allowed = false
	}
	for _, g := range en.GrowTypes {
		if g >= 0 && g != int32(advancement) {
			allowed = false
		}
	}
	return allowed && !collisionsBlocked(en.Collisions, status)
}

func loadQuestIndex(t *testing.T) *Index {
	t.Helper()
	c, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	return BuildIndex(c)
}

func cloneStatus(src map[uint32]string) map[uint32]string {
	dst := make(map[uint32]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// TestCollisionSymmetryAcrossCatalog audits the imported catalog's
// [collision quest] edges. Peers outside this catalog (quests the importer
// skipped, e.g. 2943/12830) are inert and skipped. Asymmetric edges that do
// exist in the source (e.g. 3874 -> 3873 without a back edge) are honored
// one-way by the server and reported as information, not failures.
func TestCollisionSymmetryAcrossCatalog(t *testing.T) {
	x := loadQuestIndex(t)
	asym := 0
	for id, en := range x.Entries {
		for _, peer := range en.Collisions {
			p := x.Entries[peer]
			if p == nil {
				continue
			}
			if !slices.Contains(p.Collisions, id) {
				t.Logf("source quirk (honored one-way): quest %d -> %d lacks a back edge", id, peer)
				asym++
			}
		}
	}
	t.Logf("%d one-way collision edges audited", asym)
}

// Silent City (寂静城) faction choice structure, as imported from source:
//
//	lead-ins (zelba_18_xx)          Luke_01 branches        follow-ups
//	3870 (branch A)                 \__ 3923 ⇄ 3924 ⇄ 3925  \__ 3926..3928
//	3871 ⇄ 3872 (branch B)          → the cross-branch      → per-branch epics
//	3873 / 3874 (branch C)             exclusivity lives       (Luke_02_xx)
//
// The lead-ins are only alternatives within a branch. The three branches stay
// open until the player picks one of 3923/3924/3925; the reported bug let a
// character pick all three. The collision filter must hide the unchosen
// siblings once a branch is accepted or completed.
var factionBranch = []uint32{3923, 3924, 3925}

// factionLeadInChoice is one completed lead-in per branch plus the shared 3883.
var factionLeadInChoice = map[uint32]string{
	3870: "completed", 3871: "completed", 3873: "completed", 3883: "completed",
}

func TestSilentCityFactionChoiceMutuallyExclusive(t *testing.T) {
	x := loadQuestIndex(t)
	const level uint32 = 100
	const advancement, awakening byte = 1, 0
	job := "" // Luke faction quests carry no [job] restriction → every profession

	// Stage 1 — the choice moment: all three branches are offered.
	for _, id := range factionBranch {
		if !simulateOffered(x, factionLeadInChoice, id, level, job, advancement, awakening) {
			t.Fatalf("stage1: faction branch %d must be offered at the choice moment", id)
		}
	}

	// Stage 2 — branch 3923 chosen: its siblings leave the offer list.
	chosen := cloneStatus(factionLeadInChoice)
	chosen[3923] = "accepted"
	for _, id := range []uint32{3924, 3925} {
		if simulateOffered(x, chosen, id, level, job, advancement, awakening) {
			t.Fatalf("stage2: branch %d must vanish once 3923 is accepted", id)
		}
	}
	if !simulateOffered(x, chosen, 3923, level, job, advancement, awakening) {
		t.Fatal("stage2: the chosen branch stays active")
	}

	// Stage 3 — 3923 completed: siblings can never be re-selected.
	done := cloneStatus(factionLeadInChoice)
	done[3923] = "completed"
	for _, id := range []uint32{3924, 3925} {
		if simulateOffered(x, done, id, level, job, advancement, awakening) {
			t.Fatalf("stage3: branch %d must stay hidden after 3923 completes", id)
		}
	}

	// Stage 4 — pre-fix bugged state (3923 completed, 3924 still accepted):
	// the remaining sibling must not be offered either.
	bugged := cloneStatus(factionLeadInChoice)
	bugged[3923] = "completed"
	bugged[3924] = "accepted"
	if simulateOffered(x, bugged, 3925, level, job, advancement, awakening) {
		t.Fatal("stage4: 3925 must not be offered in the pre-fix double-branch state")
	}
}

// TestIntraBranchLeadInAlternatives: the branch B lead-ins 3871/3872 and the
// branch C lead-ins 3873/3874 are alternatives that the fix also keeps
// mutually exclusive (they carry [collision quest] in source).
func TestIntraBranchLeadInAlternatives(t *testing.T) {
	x := loadQuestIndex(t)
	const level uint32 = 100
	const advancement, awakening byte = 1, 0
	job := ""

	// Branch B: taking 3871 hides 3872.
	b := map[uint32]string{3870: "completed", 3871: "completed", 3883: "completed"}
	if simulateOffered(x, b, 3872, level, job, advancement, awakening) {
		t.Fatal("branch B: 3872 must vanish once alternative 3871 is taken")
	}
	b2 := map[uint32]string{3870: "completed", 3872: "completed", 3883: "completed"}
	if simulateOffered(x, b2, 3871, level, job, advancement, awakening) {
		t.Fatal("branch B: 3871 must vanish once alternative 3872 is taken")
	}

	// Branch C: source carries only the forward edge 3874 -> 3873, so taking
	// 3873 hides 3874, while 3873 stays offerable after 3874.
	c := map[uint32]string{3870: "completed", 3873: "completed", 3883: "completed"}
	if simulateOffered(x, c, 3874, level, job, advancement, awakening) {
		t.Fatal("branch C: 3874 must vanish once 3873 is taken (forward edge)")
	}
	c2 := map[uint32]string{3870: "completed", 3874: "completed", 3883: "completed"}
	if !simulateOffered(x, c2, 3873, level, job, advancement, awakening) {
		t.Fatal("branch C: 3873 stays offerable after 3874 (source has no back edge)")
	}
}

// TestAcceptGateRejectsSiblingBranches mirrors the Accept gate in service.go:
// a branch may not be accepted while a collision peer is accepted or completed.
func TestAcceptGateRejectsSiblingBranches(t *testing.T) {
	x := loadQuestIndex(t)
	status := map[uint32]string{3923: "accepted"}
	for _, id := range []uint32{3924, 3925} {
		if !collisionsBlocked(x.Entries[id].Collisions, status) {
			t.Fatalf("accept gate must reject branch %d while 3923 is accepted", id)
		}
	}
	status = map[uint32]string{3923: "completed"}
	for _, id := range []uint32{3924, 3925} {
		if !collisionsBlocked(x.Entries[id].Collisions, status) {
			t.Fatalf("accept gate must reject branch %d after 3923 completed", id)
		}
	}
}
