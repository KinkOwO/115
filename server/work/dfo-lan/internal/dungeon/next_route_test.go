package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestGrandFloresMainQuestMapsAreReachable(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.next28.json")
	if e != nil {
		t.Fatal(e)
	}
	for _, qid := range []uint16{3145, 3146, 3147, 3148, 3149, 3150, 3151} {
		var found bool
		for id, d := range c.Dungeons {
			for _, maze := range d.Mazes {
				if maze.Quest != qid {
					continue
				}
				found = true
				s, e := Select(c, protocol.DungeonSelection{ID: id, Quest: uint32(qid), Party: 65535}, 20, map[uint16]bool{qid: true})
				if e != nil {
					t.Fatalf("quest%d: %v", qid, e)
				}
				for _, room := range maze.Rooms {
					m, e := fixedMonsters(c.Maps[room.Map], d.BasisLevel)
					if e != nil {
						t.Fatalf("quest%d map%d: %v", qid, room.Map, e)
					}
					if len(m) == 0 {
						t.Logf("source room%d has no monster rows", room.Map)
					}
				}
				reached := map[[2]byte]bool{s.Maze.Start: true}
				queue := [][2]byte{s.Maze.Start}
				for len(queue) > 0 {
					x := queue[0]
					queue = queue[1:]
					for _, r := range maze.Rooms {
						xy := [2]byte{r.X, r.Y}
						dx, dy := int(x[0])-int(r.X), int(x[1])-int(r.Y)
						if dx < 0 {
							dx = -dx
						}
						if dy < 0 {
							dy = -dy
						}
						if dx+dy == 1 && !reached[xy] {
							reached[xy] = true
							queue = append(queue, xy)
						}
					}
				}
				if !reached[maze.Boss] {
					t.Fatalf("quest%d boss disconnected", qid)
				}
			}
		}
		if !found {
			t.Fatalf("quest%d dungeon not imported", qid)
		}
	}
}
