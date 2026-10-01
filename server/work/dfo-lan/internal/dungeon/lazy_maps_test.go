package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestLazyMapEntryAndRoomLocalArchiveParity(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native entry and room parity")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 1024 * 1024 * 1024}, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	ids := []uint32{3, 22, 25, 86, 100004136, 100004137}
	eager, err := catalog.ImportDungeons(a, ids)
	if err != nil {
		t.Fatal(err)
	}
	lazy, err := catalog.ImportRuntimeDungeons(a, ids)
	if err != nil {
		t.Fatal(err)
	}
	defer lazy.CloseMapSource()
	if err = a.CompactRuntimeStrings(); err != nil {
		t.Fatal(err)
	}
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	var entered, rooms int
	var coldTotal, coldMax time.Duration
	for _, id := range ids {
		def, ok := eager.Dungeons[id]
		if !ok {
			t.Fatalf("dungeon %d absent", id)
		}
		for _, maze := range def.Mazes {
			before, e1 := newSession(eager, def, maze)
			after, e2 := newSession(lazy, def, maze)
			if (e1 == nil) != (e2 == nil) || (e1 != nil && e1.Error() != e2.Error()) {
				t.Fatalf("entry %d/%d: %v versus %v", id, maze.Index, e1, e2)
			}
			if e1 != nil {
				continue
			}
			entered++
			if !reflect.DeepEqual(before.Room, after.Room) || !reflect.DeepEqual(before.Monsters, after.Monsters) {
				t.Fatalf("entry %d/%d changed room or spawn facts", id, maze.Index)
			}
			for _, room := range maze.Rooms {
				resolved, _, ok, resolveErr := resolveRoomMap(eager, room)
				if !ok || resolveErr != nil {
					continue
				}
				r1, e1 := before.enterRoom(eager, resolved)
				coldBegin := time.Now()
				r2, e2 := after.enterRoom(lazy, resolved)
				coldDuration := time.Since(coldBegin)
				coldTotal += coldDuration
				coldMax = max(coldMax, coldDuration)
				if (e1 == nil) != (e2 == nil) || (e1 != nil && e1.Error() != e2.Error()) {
					t.Fatalf("room %d: %v versus %v", room.Map, e1, e2)
				}
				if e1 == nil {
					rooms++
					if !reflect.DeepEqual(r1.Room, r2.Room) || !reflect.DeepEqual(r1.Monsters, r2.Monsters) || !reflect.DeepEqual(r1.Visited, r2.Visited) {
						t.Fatalf("room %d changed spawn or revisit facts", room.Map)
					}
				}
				lazy.ReleaseMapReadCache()
			}
		}
	}
	if entered == 0 || rooms == 0 {
		t.Fatal("did not exercise native rooms")
	}
	t.Logf("native entry mazes=%d room transitions=%d read total=%s max=%s", entered, rooms, coldTotal, coldMax)
	moonBefore, err := MoonInitialProgress(eager)
	if err != nil {
		t.Fatal(err)
	}
	lazy.ReleaseMapReadCache()
	moonAfter, err := MoonInitialProgress(lazy)
	if err != nil || !reflect.DeepEqual(moonBefore, moonAfter) {
		t.Fatal("Moon floor populations changed on cold reads", err)
	}
	if err = lazy.CloseMapSource(); err != nil {
		t.Fatal(err)
	}
	def := lazy.Dungeons[3]
	if _, err = newSession(lazy, def, def.Mazes[0]); err == nil {
		t.Fatal("entry accepted unavailable map source")
	}
	if _, err = MoonInitialProgress(lazy); err == nil {
		t.Fatal("Moon progress accepted unavailable map source")
	}
}
