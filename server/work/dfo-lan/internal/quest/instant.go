package quest

import (
	"dfolan/internal/catalog"
	"strings"
)

// Grade returns the quest's own [grade] classification token exactly as the
// source spells it ("[guide]", "[epic]", "[side]", ...), or "" when the row
// carries none. This is the token the client colours the quest log with; the
// shortcut sweep must honour it because a single button is not a licence to
// clear side quests too.
func Grade(d catalog.QuestDefinition) string {
	g := cells(d.Script.Cells, "[grade]")
	if len(g) == 1 && g[0].Type == 6 && strings.TrimSpace(g[0].Text) != "" {
		return g[0].Text
	}
	return ""
}

// GradeSet is the sweep's filter: which [grade] tokens may be completed.
type GradeSet struct {
	all    bool
	grades map[string]bool
}

// Allows reports whether one source [grade] token is inside the set.
func (g GradeSet) Allows(token string) bool {
	if g.all {
		return true
	}
	return g.grades[gradeName(token)]
}

// gradeName normalises one supplied grade name onto the source's spelling:
// "guide", "GUIDE" and "[guide]" all become "[guide]".
func gradeName(text string) string {
	return "[" + strings.TrimSpace(strings.Trim(strings.ToLower(strings.TrimSpace(text)), "[]")) + "]"
}

// MainlineGrades is the sweep's filter: the story chain ([epic], 994 of this
// build's 2844 rows) and the 110-level guidance chain ([guide], 37). Everything
// else - [side], [daily], [dungeon and life], [title] - is not what a "complete
// the main quest" button may touch, which is the boundary a bulk sweep crossed
// once and cleared 65 records it should have left standing.
func MainlineGrades() GradeSet {
	return GradeSet{grades: map[string]bool{"[epic]": true, "[guide]": true}}
}

// Mainline reports whether the source classes a quest as main line.
func Mainline(d catalog.QuestDefinition) bool {
	return MainlineGrades().Allows(Grade(d))
}
