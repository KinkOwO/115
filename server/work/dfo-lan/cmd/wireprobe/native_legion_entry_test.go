package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/legion"
	"encoding/binary"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestNativeLegionAllSourceBossEntries(t *testing.T) {
	path := os.Getenv("DFO_REPAIR_PVF")
	if path == "" {
		t.Skip("native PVF required")
	}
	a, err := catalog.OpenTestArchiveCached(path, "")
	if err != nil {
		t.Fatal(err)
	}
	contents, err := catalog.ImportLegionContents(a)
	if err != nil {
		t.Fatal(err)
	}
	var ids []uint32
	for _, name := range []string{"Dimension Cloister", "DuskyIsland"} {
		for _, d := range contents.Contents[name].Dungeons {
			ids = append(ids, d.Dungeon)
		}
	}
	dungeons, err := catalog.ImportRuntimeDungeons(a, ids)
	if err != nil {
		t.Fatal(err)
	}
	defer dungeons.CloseMapSource()
	dungeons.LegionContents = &contents
	for _, name := range []string{"Dimension Cloister", "DuskyIsland"} {
		t.Run(name+"/runtime-dispatch", func(t *testing.T) {
			c := contents.Contents[name]
			w := &worldSession{dungeons: &dungeons, level: 140, role: database.Character{ID: 8, WireID: 8}, state: database.WorldState{Position: database.WorldPosition{Town: c.Waiting.Town, Area: c.Waiting.Area}}}
			s := &legionSession{}
			content, _, selectID, _ := nativeLegionIdentity(name)
			send := func(id uint16, p []byte) legionResult {
				t.Helper()
				result, handled, err := s.handleNativeLegion(w, p, id)
				if err != nil || !handled || len(result.Packets) == 0 {
					t.Fatalf("dispatch %d: handled=%v error=%v", id, handled, err)
				}
				return result
			}
			start := make([]byte, 17)
			binary.LittleEndian.PutUint32(start[13:], content)
			send(legion.CmdStart, start)
			if plan, e := w.nativeLegionTick(s.nativeRun.ReadyAt); e != nil || len(plan) != 1 {
				t.Fatal("countdown", e)
			}
			selection := make([]byte, 19)
			binary.LittleEndian.PutUint32(selection[13:], 1)
			send(selectID, selection)
			binary.LittleEndian.PutUint32(selection[13:], 2)
			binary.LittleEndian.PutUint16(selection[17:], s.nativeRun.Offers[0].Index)
			send(selectID, selection)
			entry := make([]byte, 21)
			binary.LittleEndian.PutUint32(entry[13:], content)
			send(legion.CmdEnterDungeon, entry)
			if w.activeDungeon == nil || s.nativeRun.State != legion.NativeFighting || w.activeDungeon.RunID != s.nativeRun.Attempt {
				t.Fatal("entry not installed")
			}
		})
	}
	for _, name := range []string{"Dimension Cloister", "DuskyIsland"} {
		c := contents.Contents[name]
		for index, stage := range c.Dungeons {
			t.Run(fmt.Sprintf("%s/%d", name, stage.Dungeon), func(t *testing.T) {
				order := [3]int{index, (index + 1) % len(c.Dungeons), (index + 2) % len(c.Dungeons)}
				r, err := legion.NewNativeRun(c, "normal", order, time.Time{}, func(n int64) (int64, error) { return 0, nil })
				if err != nil {
					t.Fatal(err)
				}
				now := time.Now()
				r.Ready(now)
				if err = r.Open(false, now); err != nil {
					t.Fatal(err)
				}
				if err = r.Select(r.Offers[0].Index, now); err != nil {
					t.Fatal(err)
				}
				w := &worldSession{dungeons: &dungeons, level: 140, role: database.Character{ID: 7, WireID: 7, Name: "source"}, state: database.WorldState{Position: database.WorldPosition{Town: c.Waiting.Town, Area: c.Waiting.Area, X: uint16(c.Waiting.X), Y: uint16(c.Waiting.Y)}}}
				s, plan, err := w.prepareNativeLegionStage(r)
				if err != nil {
					t.Fatal(err)
				}
				if w.activeDungeon != nil || r.State != legion.NativeReady {
					t.Fatal("preparation prematurely installed dungeon")
				}
				foundInfo, foundMap := false, false
				for _, p := range plan {
					if p.ID == 28 {
						foundInfo = true
						if binary.LittleEndian.Uint32(p.Payload) != stage.Dungeon {
							t.Fatal("foreign captured dungeon")
						}
					}
					if p.ID == 29 {
						foundMap = true
					}
				}
				if !foundInfo || !foundMap {
					t.Fatal("source entry incomplete")
				}
				if err = r.Enter(0, stage.Dungeon, s.RunID); err != nil {
					t.Fatal(err)
				}
				s.Loaded = true
				if !s.ArenaBoss {
					if s.Completed() {
						t.Fatal("empty approach room completed stage")
					}
					s, err = s.Move(dungeons, s.Maze.Boss)
					if err != nil {
						t.Fatal(err)
					}
					s.Loaded = true
				}
				foundBoss := false
				for _, m := range s.Monsters {
					foundBoss = foundBoss || m.Template == stage.Boss
				}
				if !foundBoss {
					t.Fatal("content boss absent from battle room")
				}
				for _, m := range s.Monsters {
					if _, err = s.ConfirmDeath(uint32(m.Entity), w.role.WireID, w.role.WireID); err != nil {
						t.Fatal(err)
					}
				}
				if !s.Completed() {
					t.Fatal("source arena cannot complete")
				}
				if err = r.Clear(s.Definition.ID, s.RunID, s.Completed()); err != nil {
					t.Fatal(err)
				}
				t.Logf("dungeon=%d sourceBoss=%d monsters=%+v", stage.Dungeon, stage.Boss, s.Monsters)
			})
		}
	}
}
