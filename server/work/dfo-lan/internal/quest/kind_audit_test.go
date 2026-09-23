package quest

import (
	"dfolan/internal/catalog"
	"fmt"
	"sort"
	"testing"
)

// TestAuditObjectiveKinds is a survey, not an assertion: it groups every quest in
// the catalog by its `[type]` cell and reports how many of each the server can
// settle. It exists because a quest whose objective model is missing is silently
// dropped from the acceptable-quest list (available.go), which is how an entire
// content block can disappear with no server-side error at all — the apocalypse
// chain was found that way (analysis/tasks/next74-legion-entry-prereq-chain.md).
//
// Run it with -v after touching initialProgress or ReachRange/SeekMeet: the
// "not settleable" column is the list of content that is currently unreachable.
func TestAuditObjectiveKinds(t *testing.T) {
	c, e := catalog.LoadQuests("../../configs/quests.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	x := BuildIndex(c)

	type row struct {
		kind      string
		total     int
		settle    int
		examples  []string
		unsupport []uint32
	}
	rows := map[string]*row{}
	for id, d := range c.Quests {
		r := rows[d.Kind]
		if r == nil {
			r = &row{kind: d.Kind}
			rows[d.Kind] = r
		}
		r.total++
		en := x.Entries[id]
		if en != nil && en.Implemented {
			r.settle++
			continue
		}
		r.unsupport = append(r.unsupport, id)
		if len(r.examples) < 4 {
			r.examples = append(r.examples, fmt.Sprintf("%d %s", id, d.Script.Path))
		}
	}

	list := make([]*row, 0, len(rows))
	for _, r := range rows {
		list = append(list, r)
	}
	sort.Slice(list, func(i, j int) bool {
		// Unsettleable kinds first, then by how much content they hold.
		if (list[i].total-list[i].settle == 0) != (list[j].total-list[j].settle == 0) {
			return list[i].total-list[i].settle > 0
		}
		return list[i].total > list[j].total
	})

	var unreachable int
	for _, r := range list {
		if r.total-r.settle == 0 {
			continue
		}
		unreachable += r.total - r.settle
		sort.Slice(r.unsupport, func(i, j int) bool { return r.unsupport[i] < r.unsupport[j] })
		t.Logf("UNSETTLEABLE %-40s settle=%d/%d ids=%v", r.kind, r.settle, r.total, r.unsupport)
		for _, ex := range r.examples {
			t.Logf("        e.g. %s", ex)
		}
	}
	t.Logf("kinds=%d quests=%d unreachable=%d", len(list), len(c.Quests), unreachable)

	// The other direction: kinds that are settleable are equally worth naming, so
	// a later reader can tell "missing model" from "kind nobody uses".
	for _, r := range list {
		if r.total-r.settle != 0 {
			continue
		}
		t.Logf("settleable   %-40s settle=%d/%d", r.kind, r.settle, r.total)
	}
}
