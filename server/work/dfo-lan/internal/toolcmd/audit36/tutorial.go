package audit36

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// reportTutorial checks whether every ordinary job's tutorial dungeon can
// actually be opened by the existing session builder. Importing the maps is
// not the same as being able to spawn them: a source row the monster parser
// refuses would black-screen the client instead of failing here.
func reportTutorial(routesPath, dungeonsPath string) {
	rb, e := os.ReadFile(routesPath)
	if e != nil {
		fmt.Printf("\n== tutorial scan unavailable: %v\n", e)
		return
	}
	var routes catalog.TutorialCatalog
	if e = json.Unmarshal(rb, &routes); e != nil {
		fmt.Printf("\n== tutorial routes unreadable: %v\n", e)
		return
	}
	db, e := os.ReadFile(dungeonsPath)
	if e != nil {
		fmt.Printf("\n== tutorial dungeons unavailable: %v\n", e)
		return
	}
	var dc catalog.DungeonCatalog
	if e = json.Unmarshal(db, &dc); e != nil {
		fmt.Printf("\n== tutorial dungeons unreadable: %v\n", e)
		return
	}

	fmt.Printf("\n== tutorial routes (%d) and their dungeons (%d imported)\n",
		len(routes.Flows), len(dc.Dungeons))
	ok, bad := 0, 0
	for _, f := range routes.Flows {
		tag := f.Job
		if f.EventOnly {
			tag += " (event only)"
		}
		d, exists := dc.Dungeons[f.Dungeon]
		if !exists {
			fmt.Printf("  %-22s dungeon %-10d NOT IMPORTED\n", tag, f.Dungeon)
			bad++
			continue
		}
		var mazes, rooms int
		var pending []string
		for _, m := range d.Mazes {
			mazes++
			rooms += len(m.Rooms)
			pending = append(pending, m.Pending...)
		}
		// The room builder is what the live entry path runs; exercise it.
		_, err := dungeon.SelectTutorial(dc, f.Dungeon)
		status := "ok"
		if err != nil {
			status = "REFUSED: " + err.Error()
			bad++
		} else {
			ok++
		}
		fmt.Printf("  %-22s dungeon %-10d tutorial=%v mazes=%d rooms=%d min=%d basis=%d %s\n",
			tag, f.Dungeon, d.Tutorial, mazes, rooms, d.MinimumLevel, d.BasisLevel, status)
		if len(pending) > 0 {
			sort.Strings(pending)
			fmt.Printf("      maze pending: %v\n", pending)
		}
	}
	fmt.Printf("  openable=%d refused=%d\n", ok, bad)
}

var _ = protocol.DungeonSelection{}
