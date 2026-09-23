package quest

import (
	"dfolan/internal/catalog"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// apocalypseEntryQuest is the quest the apocalypse (末世录) legion entry is bound
// to: dungeon 100005220 declares `quests = [23099]` in configs/dungeons.full.json,
// and the quest's own script names the same dungeon in its `[dungeon info]` cell.
//
// The client will not offer the legion entry until the player is inside the quest
// chain that leads here, and the server only publishes a quest in the
// acceptable-quest list (NOTI21) when it can settle that quest's objective
// (internal/quest/available.go: `Implemented && RewardUsable && GrowUsable`).
// Those two facts compose into a nasty failure mode: **one unsupported quest
// anywhere upstream silently removes the whole content**, and because the
// available list just omits it, nothing in the server log says why. That is the
// failure this test makes loud.
//
// The chain is walked from the source rather than hardcoded: it is a tree, not a
// line (22997 alone requires 22990, 22993 and 22996), and the ids are re-numbered
// between client builds.
const apocalypseEntryQuest uint32 = 23099

// prerequisiteClosure returns every quest reachable from the roots by following
// `[pre required quest]`, breadth first, so the first break is named first.
func prerequisiteClosure(t *testing.T, c catalog.QuestCatalog, roots []uint32) []uint32 {
	t.Helper()
	seen := map[uint32]bool{}
	order := []uint32{}
	queue := append([]uint32(nil), roots...)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		order = append(order, id)
		d, ok := c.Quests[id]
		if !ok {
			t.Errorf("quest %d is absent from the catalog; the chain breaks here", id)
			continue
		}
		for _, p := range d.Prerequisites {
			if !seen[p] {
				queue = append(queue, p)
			}
		}
	}
	return order
}

// knownBlockedQuests used to hold 23053/23054/23055/23099, which were dropped
// from the acceptable list because `[monster kill checkpoint]` had no objective
// model. That model is implemented now (internal/quest/progress.go,
// MonsterKillCheckpointShape), so the map is empty and the chain must be fully
// offered.
//
// Keep the mechanism: an entry added back here is a claim that a link is
// understood and deliberately blocked, and the test still FAILS on a break that
// is not listed. Anything listed without that justification would just be a
// loosened assertion.
var knownBlockedQuests = map[uint32]string{}

// TestApocalypsePrerequisiteChainIsOffered reports whether the server can offer
// every quest upstream of the apocalypse entry. Unknown breaks fail; the known
// unimplemented objective model skips with the fix spelled out, so the suite
// stays green without pretending the chain works.
func TestApocalypsePrerequisiteChainIsOffered(t *testing.T) {
	c, e := catalog.LoadQuests("../../configs/quests.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	entry, ok := c.Quests[apocalypseEntryQuest]
	if !ok {
		t.Fatalf("entry quest %d is absent from the catalog", apocalypseEntryQuest)
	}
	if !strings.Contains(entry.Script.Path, "apocalypse") {
		t.Fatalf("quest %d is %s, want an apocalypse scenario quest (the source was renumbered)",
			apocalypseEntryQuest, entry.Script.Path)
	}

	x := BuildIndex(c)
	chain := prerequisiteClosure(t, c, []uint32{apocalypseEntryQuest})

	var offered, known, unexpected []string
	for _, id := range chain {
		d := c.Quests[id]
		en := x.Entries[id]
		if en == nil {
			unexpected = append(unexpected, fmt.Sprintf("%d %s -> no index entry", id, d.Kind))
			continue
		}
		why := ""
		switch {
		case !en.Implemented:
			why = "objective not settleable"
		case !en.RewardUsable:
			why = "reward not settleable"
		case !en.GrowUsable:
			why = "grow condition not settleable"
		}
		if why == "" {
			offered = append(offered, fmt.Sprintf("%d %s", id, d.Kind))
			continue
		}
		shapes := make([]string, 0, len(d.ObjectiveCells))
		for _, cell := range d.ObjectiveCells {
			shapes = append(shapes, fmt.Sprintf("t%d=%v", cell.Type, cell.Value))
		}
		breakLine := fmt.Sprintf("%d %s -> %s | objective cells: %s | pending: %s | %s",
			id, d.Kind, why, strings.Join(shapes, " "),
			strings.Join(d.Pending, ","), d.Script.Path)
		if _, ok := knownBlockedQuests[id]; ok {
			known = append(known, breakLine)
		} else {
			unexpected = append(unexpected, breakLine)
		}
	}

	t.Logf("apocalypse chain: %d quests reachable from %d (%d offered, %d known-blocked, %d unexpected)",
		len(chain), apocalypseEntryQuest, len(offered), len(known), len(unexpected))

	if len(unexpected) > 0 {
		sort.Strings(unexpected)
		t.Fatalf("%d quest(s) upstream of the apocalypse entry are not offered and are NOT in the known-blocked "+
			"list; the client needs the whole chain completed, so the entry cannot unlock:\n  %s",
			len(unexpected), strings.Join(unexpected, "\n  "))
	}
	if len(known) > 0 {
		sort.Strings(known)
		t.Skipf("known blocker: %d quest(s) upstream of the apocalypse entry use the `[monster kill checkpoint]` "+
			"objective model, which internal/quest does not implement, so the entry cannot unlock yet. "+
			"Those four quests are the only users of that model in the catalog, so implementing it (the key "+
			"semantics of the [int data] triples 109019626..109019630 are still unresolved - see "+
			"analysis/tasks/next74-legion-entry-prereq-chain.md) unblocks the whole chain:\n  %s",
			len(known), strings.Join(known, "\n  "))
	}
}
