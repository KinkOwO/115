// questchain answers "why does the main story stop here?" by replaying the
// availability rule against the real catalog for a given character, and by
// following the prerequisite graph forward from the completed quests.
//
// Read-only. Loads the same configs the gateway loads.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/quest"
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"
)

func main() {
	questFile := flag.String("quests", "configs/quests.generated.json", "quest catalog")
	level := flag.Uint("level", 9, "character level")
	adv := flag.Int("advancement", 0, "advancement stage")
	job := flag.String("job", "[archer]", "profession job tag")
	done := flag.String("completed", "3145,3146,3147,3148,4873,2109,21650", "completed quest ids")
	flag.Parse()

	c, e := catalog.LoadQuests(*questFile)
	if e != nil {
		log.Fatal(e)
	}
	x := quest.BuildIndex(c)
	completed := map[uint32]bool{}
	for _, s := range strings.Split(*done, ",") {
		var id uint32
		fmt.Sscan(strings.TrimSpace(s), &id)
		if id != 0 {
			completed[id] = true
		}
	}
	run(x, c, completed, byte(*level), byte(*adv), *job)
}

func run(x *quest.Index, c catalog.QuestCatalog, completed map[uint32]bool, level, adv byte, job string) {
	fmt.Printf("== availability for a level %d %s (advancement %d)\n", level, job, adv)
	offered, blocked := classify(x, completed, level, adv, job)
	sort.Slice(offered, func(i, j int) bool { return offered[i] < offered[j] })
	fmt.Printf("offered now (%d): %v\n", len(offered), offered)

	fmt.Println("\n== quests gated directly behind the completed set")
	next := successors(x, completed)
	sort.Slice(next, func(i, j int) bool { return next[i] < next[j] })
	for _, id := range next {
		if completed[id] {
			continue
		}
		fmt.Printf("  %d: %s\n", id, strings.Join(blocked[id], "; "))
	}

	fmt.Println("\n== reasons a quest that passes job+prereq is still withheld")
	hist := map[string]int{}
	for id, e := range x.Entries {
		if completed[id] || !jobAllowed(e, job) || !prereqMet(e, completed) {
			continue
		}
		for _, r := range whyBlocked(e, level, adv) {
			hist[r]++
		}
	}
	type kv struct {
		r string
		n int
	}
	var rows []kv
	for r, n := range hist {
		rows = append(rows, kv{r, n})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].n > rows[j].n })
	for _, row := range rows {
		fmt.Printf("  %5d  %s\n", row.n, row.r)
	}
}
