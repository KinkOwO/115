package audit36

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"sort"
)

// npcRows repeats world.Service.HasNPC's source scan so the audit reports
// exactly the identities that handler accepts, plus each row's trailing
// fields so their meaning can be established. Read-only.
func npcRows(a catalog.WorldArea) map[uint32][]pvf.Token {
	out := map[uint32][]pvf.Token{}
	for _, script := range append([]catalog.ScriptRecord{a.Map}, a.ImportedScripts...) {
		c := script.Cells
		for i := 0; i < len(c); i++ {
			if c[i].Type != 3 || c[i].Text != "[NPC]" {
				continue
			}
			for i++; i < len(c) && c[i].Type != 3; i += 5 {
				if i+4 >= len(c) || c[i].Type != 0 || c[i+1].Type != 6 ||
					c[i+2].Type != 0 || c[i+3].Type != 0 || c[i+4].Type != 0 {
					break
				}
				if c[i].Value > 0 {
					out[uint32(c[i].Value)] = append([]pvf.Token(nil), c[i+1:i+5]...)
				}
			}
		}
	}
	return out
}

func reportNPC(w catalog.WorldCatalog, towns []int, wanted []uint32) {
	fmt.Printf("\n== npc presence by area (towns=%v)\n", towns)
	placed := map[uint32][]string{}
	detail := map[uint32]string{}
	for _, town := range towns {
		for area := 0; area < 32; area++ {
			a, ok := w.Areas[catalog.AreaKey(uint32(town), uint32(area))]
			if !ok {
				continue
			}
			rows := npcRows(a)
			var ids []uint32
			for id := range rows {
				ids = append(ids, id)
			}
			sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
			fmt.Printf("  town %d area %-3d level>=%-3d walkable=%d npcs=%v\n",
				town, area, a.MinimumLevel, len(a.Walkable), ids)
			for _, id := range ids {
				placed[id] = append(placed[id], fmt.Sprintf("%d/%d", town, area))
				if detail[id] == "" {
					detail[id] = brief(rows[id])
				}
			}
		}
	}
	fmt.Printf("  wanted quest npc lookup (row = name, then three ints):\n")
	for _, id := range wanted {
		where := placed[id]
		if len(where) == 0 {
			fmt.Printf("    npc %-10d NOT FOUND in scanned towns\n", id)
			continue
		}
		fmt.Printf("    npc %-10d at %v row=[%s]\n", id, where, detail[id])
	}
}
