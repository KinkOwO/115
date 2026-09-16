package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// reportSkillPanel contrasts what a base-job character actually owns with the
// cells its client skill page can draw. A skill the character holds but the
// layout has no cell for cannot be shown at all, which is what an "incorrect
// skill panel" looks like from the player's side.
func reportSkillPanel(manifestPath, charactersPath string, job int) {
	mb, e := os.ReadFile(manifestPath)
	if e != nil {
		fmt.Printf("\n== skill panel scan unavailable: %v\n", e)
		return
	}
	var m struct {
		OriginalCommon []uint32 `json:"original_common_ids"`
		Added          []struct {
			ID uint32    `json:"id"`
			XY [2]int32  `json:"xy"`
			G  string    `json:"source_grow"`
		} `json:"added"`
		Missing []uint32 `json:"eligible_not_in_any_layout"`
	}
	if e = json.Unmarshal(mb, &m); e != nil {
		fmt.Printf("\n== skill panel manifest unreadable: %v\n", e)
		return
	}

	drawable := map[uint32]string{}
	for _, id := range m.OriginalCommon {
		drawable[id] = "original base cell"
	}
	for _, a := range m.Added {
		drawable[a.ID] = fmt.Sprintf("cell borrowed from grow %q at (%d,%d)", a.G, a.XY[0], a.XY[1])
	}
	cb, e := os.ReadFile(charactersPath)
	if e != nil {
		fmt.Printf("\n== character catalog unavailable: %v\n", e)
		return
	}
	var cc struct {
		Professions map[string]struct {
			Job           int     `json:"job_id"`
			InitialSkills []int32 `json:"initial_skill_cells"`
		} `json:"professions"`
	}
	_ = json.Unmarshal(cb, &cc)

	fmt.Printf("\n== base-job skill panel (job %d)\n", job)
	fmt.Printf("  drawable cells (%d):\n", len(drawable))
	var ids []uint32
	for id := range drawable {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		fmt.Printf("    skill %-5d %s\n", id, drawable[id])
	}
	fmt.Printf("  eligible but NO cell anywhere in the layout file (%d):\n    %v\n",
		len(m.Missing), m.Missing)
	fmt.Printf("  note: a skill in that second list cannot be drawn on the page at all,\n")
	fmt.Printf("        no matter what the server sends in NOTI19.\n")
}
