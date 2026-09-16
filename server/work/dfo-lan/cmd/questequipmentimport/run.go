package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"log"
	"sort"
)

// section returns the cells that follow a named marker, the same way
// progression/experience.go reads quest scripts.
func section(cells []pvf.Token, name string) []pvf.Token {
	var r []pvf.Token
	on := false
	for _, v := range cells {
		if v.Type == 3 {
			on = v.Text == name
			continue
		}
		if on {
			r = append(r, v)
		}
	}
	return r
}

// rewardTemplates collects every integer a reward section mentions. Counts and
// job tuples are mixed in with the template ids, so membership in the source
// equipment list is what decides: an integer the list does not carry is a
// count or a job number, not a template.
func rewardTemplates(q catalog.QuestCatalog, known map[uint32]string) map[uint32]bool {
	want := map[uint32]bool{}
	for _, d := range q.Quests {
		for _, tag := range []string{"[reward int data]", "[reward selection int data]"} {
			for _, c := range section(d.Script.Cells, tag) {
				if c.Type == 0 && c.Value > 0 && known[uint32(c.Value)] != "" {
					want[uint32(c.Value)] = true
				}
			}
		}
	}
	return want
}


func run(a *pvf.Archive, c *inventory.EquipmentCatalog, q catalog.QuestCatalog,
	paths map[uint32]string, out string) {
	have := map[uint32]bool{}
	for _, r := range c.Rows {
		have[r.ID] = true
	}
	want := rewardTemplates(q, paths)
	var missing []uint32
	for id := range want {
		if !have[id] {
			missing = append(missing, id)
		}
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
	log.Printf("quest rewards reference %d listed templates; %d are new",
		len(want), len(missing))

	added, failed := 0, 0
	for _, id := range missing {
		s, e := resolve(a, paths[id])
		if e != nil {
			if failed < 10 {
				log.Printf("skipped %d (%s): %v", id, paths[id], e)
			}
			failed++
			continue
		}
		c.Rows = append(c.Rows, inventory.EquipmentDefinition{
			ID: id, Path: s.Path, SHA256: s.SHA256})
		added++
	}
	sort.Slice(c.Rows, func(i, j int) bool { return c.Rows[i].ID < c.Rows[j].ID })
	writeCatalog(c, out)
	log.Printf("wrote %s: %d rows (%d added, %d unreadable)",
		out, len(c.Rows), added, failed)
}
