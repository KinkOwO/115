package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func odysseyScenes(t *testing.T) catalog.DungeonCatalog {
	t.Helper()
	c, e := catalog.LoadDungeons("../../configs/dungeons.odyssey-scenes-release.json")
	if e != nil {
		t.Fatal(e)
	}
	return c
}

func TestOdysseySceneAPCs(t *testing.T) {
	c := odysseyScenes(t)
	total, hostile := 0, 0
	for id, script := range c.Maps {
		ms, e := fixedMonsters(script, 115)
		if e != nil {
			continue
		}
		next := uint32(0)
		seen := map[uint16]bool{}
		for _, m := range ms {
			if seen[m.Entity] {
				t.Fatal("duplicate entity", id)
			}
			seen[m.Entity] = true
			if !m.APC {
				continue
			}
			total++
			if m.SourceIndex != next || m.Rank != 5 {
				t.Fatal("APC native source index/rank", m)
			}
			next++
			if m.Team == 100 {
				hostile++
				if m.NonCombat {
					t.Fatal("hostile APC skipped")
				}
			} else if !m.NonCombat {
				t.Fatal("friendly APC blocks")
			}
		}
	}
	if total != 48 || hostile != 6 {
		t.Fatalf("APCs=%d hostile=%d", total, hostile)
	}
	s, e := Select(c, protocol.DungeonSelection{ID: 100004938, Difficulty: 2, Party: 65535}, 37, nil)
	if e != nil {
		t.Fatal(e)
	}
	s, e = s.enterRoom(c, catalog.DungeonRoom{X: 2, Y: 1, Map: 100016012})
	if e != nil {
		t.Fatal(e)
	}
	s.Loaded = true
	var apc protocol.DungeonMonster
	for _, m := range s.Monsters {
		if m.APC {
			apc = m
		} else {
			s.Dead[m.Entity] = true
		}
	}
	if apc.Template != 55424 {
		t.Fatal("missing Lenny fight")
	}
	if _, e = s.ConfirmDeath(uint32(apc.Entity), 99, 14); e == nil {
		t.Fatal("foreign killer accepted")
	}
	fresh, e := s.ConfirmDeath(uint32(apc.Entity), 14, 14)
	if e != nil || !fresh || !s.RoomCleared() {
		t.Fatal(fresh, e)
	}
	fresh, e = s.ConfirmDeath(uint32(apc.Entity), 14, 14)
	if e != nil || fresh {
		t.Fatal("duplicate APC death", e)
	}
	t.Logf("APC PASS: %d source actors; %d hostile; Lenny gates scene until owned death; duplicate idempotent", total, hostile)
}

func TestOdysseySceneInvalidCatalog(t *testing.T) {
	for _, kind := range []string{"source", "maphash", "duplicate", "skip", "missingmap", "missingroute"} {
		t.Run(kind, func(t *testing.T) {
			c := odysseyScenes(t)
			switch kind {
			case "source":
				c.SceneRoutes[0].Source = "wrong"
			case "maphash":
				c.SceneRoutes[0].MapSHA256 = "wrong"
			case "duplicate":
				c.SceneRoutes = append(c.SceneRoutes, c.SceneRoutes[0])
			case "skip":
				c.SceneRoutes[0].To = c.SceneRoutes[0].From
			case "missingmap":
				delete(c.Maps, c.SceneRoutes[0].To)
			case "missingroute":
				c.SceneRoutes = c.SceneRoutes[1:]
			}
			p := filepath.Join(t.TempDir(), "catalog.json")
			data, e := json.Marshal(c)
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(p, data, 0600); e != nil {
				t.Fatal(e)
			}
			if _, e = catalog.LoadDungeons(p); e == nil {
				t.Fatal("invalid route catalog accepted")
			}
		})
	}
}

func TestOdysseySceneBaseline(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.skycastle-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := hex.DecodeString("4af4f5050200000000ffff000000000000000000000000000000000000000000")
	request, e := protocol.DecodeDungeonSelection(raw)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Select(c, request, 37, nil); e == nil {
		t.Fatal("baseline did not reproduce")
	}
	t.Log("BASELINE captured Behemoth CMD16:", e)
	c = odysseyScenes(t)
	s, e := Select(c, request, 37, nil)
	if e != nil || s.Room.Map != 100016004 {
		t.Fatal(s, e)
	}
	if _, e = Select(c, request, 36, nil); e == nil {
		t.Fatal("minimum level bypassed")
	}
	t.Log("MODIFIED captured Behemoth CMD16: map100016004; level37 admitted, level36 rejected")
}

func clearScene(s *Session) {
	s.Loaded = true
	for _, m := range s.Monsters {
		s.Dead[m.Entity] = true
	}
}

func TestOdysseySceneAllRooms(t *testing.T) {
	c := odysseyScenes(t)
	count, rooms := 0, 0
	for _, d := range c.Dungeons {
		if !d.Odyssey {
			continue
		}
		count++
		for _, maze := range d.Mazes {
			t.Run(fmt.Sprintf("dungeon%d", d.ID), func(t *testing.T) {
				if len(maze.Pending) > 0 {
					t.Fatal(maze.Pending)
				}
				s, e := Select(c, protocol.DungeonSelection{ID: d.ID, Difficulty: d.DesignatedDifficulty, Party: 65535, Quest: uint32(maze.Quest)}, byte(d.MinimumLevel), map[uint16]bool{maze.Quest: true})
				if e != nil {
					t.Fatal(e)
				}
				var maps []uint32
				for _, r := range maze.Rooms {
					maps = append(maps, r.Map)
				}
				for _, l := range maze.Layers {
					maps = append(maps, l.Maps...)
				}
				for _, id := range maps {
					rooms++
					m, e := fixedMonsters(c.Maps[id], d.BasisLevel)
					if e != nil {
						t.Errorf("map%d: %v", id, e)
						continue
					}
					if _, e = protocol.StartMap(protocol.StartMapState{Map: id, Monsters: m}); e != nil {
						t.Errorf("map%d packet: %v", id, e)
					}
				}
				for _, layer := range maze.Layers {
					var room catalog.DungeonRoom
					for _, r := range maze.Rooms {
						if [2]byte{r.X, r.Y} == layer.Position {
							room = r
						}
					}
					s, e = s.enterRoom(c, room)
					if e != nil {
						t.Fatal(e)
					}
					for _, target := range layer.Maps {
						clearScene(s)
						// This catalog traversal supplies source-stage prerequisites;
						// the actual key-death/warp chain is exercised separately.
						var warps []scriptWarpRoute
						if err := json.Unmarshal(scriptWarpData, &warps); err != nil {
							t.Fatal(err)
						}
						s.ScriptWarps = map[uint32]bool{}
						for _, w := range warps {
							if w.Dungeon == d.ID && w.To == s.Room.Map {
								s.ScriptWarps[w.From] = true
							}
						}
						var routes []catalog.DungeonSceneRoute
						for _, r := range c.SceneRoutes {
							if r.Dungeon == d.ID && r.Maze == maze.Index && r.From == s.Room.Map {
								routes = append(routes, r)
							}
						}
						if len(routes) == 0 {
							t.Fatalf("no route from%d", s.Room.Map)
						}
						var next *Session
						for _, r := range routes {
							next, e = s.MoveScene(c, protocol.DungeonRoomTransition{Dungeon: d.ID, Position: layer.Position, LayerChange: true, Record: r.Record})
							if e != nil {
								t.Fatalf("from%d: %v", s.Room.Map, e)
							}
							if next.Room.Map != target || next.RunID != s.RunID {
								t.Fatal("wrong target/run")
							}
							bad := r.Record
							bad[6] ^= 1
							if _, e = s.MoveScene(c, protocol.DungeonRoomTransition{Dungeon: d.ID, Position: layer.Position, LayerChange: true, Record: bad}); e == nil {
								t.Fatal("tampered landing accepted")
							}
						}
						s = next
					}
				}
			})
		}
	}
	if count != 50 {
		t.Fatal(count)
	}
	t.Logf("SOURCE TRAVERSAL: dungeons=%d room visits=%d routes=%d; simulated deaths, not live client acceptance", count, rooms, len(c.SceneRoutes))
}

func TestOdysseySceneRevisitAndEmptyVisited(t *testing.T) {
	c := odysseyScenes(t)
	s, e := Select(c, protocol.DungeonSelection{ID: 100004938, Difficulty: 2, Party: 65535}, 37, nil)
	if e != nil {
		t.Fatal(e)
	}
	s, e = s.enterRoom(c, catalog.DungeonRoom{X: 2, Y: 1, Map: 100016012})
	if e != nil {
		t.Fatal(e)
	}
	clearScene(s)
	var r catalog.DungeonSceneRoute
	for _, v := range c.SceneRoutes {
		if v.From == s.Room.Map {
			r = v
			break
		}
	}
	request := protocol.DungeonRoomTransition{Dungeon: r.Dungeon, Position: r.Position, LayerChange: true, Record: r.Record}
	// [MERGE-20260928-SCENE-REVISIT] 层图可以往返。它没有怪，Visited[to] 恒为空，
	// 原来这里把「已访问且为空」当成重放拒掉，导致客户端第二次进层图就卡在门上
	// （实机 2026-09-28 安图恩讨伐战 100004950 的 164 ↔ 165）。两种情况都该放行。
	s.Visited[r.To] = nil
	if _, e = s.MoveScene(c, request); e != nil {
		t.Fatalf("层图重访（空怪列表）应放行: %v", e)
	}
	s, e = s.enterRoom(c, catalog.DungeonRoom{X: 2, Y: 1, Map: 100016012})
	if e != nil {
		t.Fatal(e)
	}
	clearScene(s)
	delete(s.Visited, r.To)
	s, e = s.MoveScene(c, request)
	if e != nil {
		t.Fatal(e)
	}
	clearScene(s)
	s, e = s.Move(c, [2]byte{3, 1})
	if e != nil {
		t.Fatal(e)
	}
	clearScene(s)
	s, e = s.Move(c, [2]byte{2, 1})
	if e != nil || s.Room.Map != r.To {
		t.Fatal(s, e)
	}
}

// [MERGE-20260928-SCENE-REVISIT] 实机回归（2026-09-28 安图恩讨伐战）：
// 100004950 的 Start (0,5) 挂了一张层图 100016165（anton_00.cmt 的 [CHANGE MAP]）。
// 客户端在 base 100016164 与层图 100016165 之间往返，第二次进入层图曾被
// "scene layer missing or already visited" 拒掉，客户端卡在门的蓝圈上过不去。
//
// [MERGE-20260928-CINEMATIC-LAYER] 同时锁住「战斗层图不是出口」：100016165 有 4 只
// 可战斗怪，客户端打完会自己走下一步；服务端若替它合成出口，会把玩家弹回 base
// （100016164，站了 3 个 NPC 的房间），客户端再进层图、再被弹回，来回循环 ——
// 实机症状正是「NPC 反复重新说话」。
func TestOdysseySceneAntonLayerRoundTrip(t *testing.T) {
	c := odysseyScenes(t)
	s, e := Select(c, protocol.DungeonSelection{ID: 100004950, Difficulty: 2, Party: 65535}, 60, nil)
	if e != nil {
		t.Fatal(e)
	}
	var r catalog.DungeonSceneRoute
	found := false
	for _, v := range c.SceneRoutes {
		if v.Dungeon == 100004950 {
			r, found = v, true
			break
		}
	}
	if !found {
		t.Fatal("100004950 没有场景路由")
	}
	if s.Room.Map != r.From {
		t.Fatalf("起始房间应为 %d，得到 %d", r.From, s.Room.Map)
	}
	request := protocol.DungeonRoomTransition{Dungeon: r.Dungeon, Position: r.Position, LayerChange: true, Record: r.Record}

	// 进入层图可以重复进行（层图重访不再被拒）。
	for round := 0; round < 3; round++ {
		clearScene(s)
		next, err := s.MoveScene(c, request)
		if err != nil {
			t.Fatalf("第 %d 次进入层图 %d 被拒: %v", round+1, r.To, err)
		}
		if next.Room.Map != r.To {
			t.Fatalf("应进入层图 %d，得到 %d", r.To, next.Room.Map)
		}
		if next.LayerRoomIsCinematic() {
			t.Fatalf("100016165 有 4 只可战斗怪，不该被判为演出层图")
		}
		// [MERGE-20260928-LAYER-ROOM-CLEARED] 该层图是序列最后一张（只有它一张），
		// 判定出口用 roomEnemiesDead 而不是 LayerRoomIsCinematic：怪清完即视为可退出。
		// 这正是实机「晦月湖」缺的那一步：客户端清完场发 CMD45 要下一张，
		// 服务端回 "no next layer map"，角色卡在图上。
		clearScene(next)
		if !next.roomEnemiesDead() {
			t.Fatal("清场后应满足出口条件")
		}
		back, err := next.MoveScene(c, protocol.DungeonRoomTransition{
			Dungeon: r.Dungeon, Position: r.Position, LayerChange: true,
		})
		if err != nil {
			t.Fatalf("清场后应能从层图退出: %v", err)
		}
		if back.Room.Map == r.To {
			t.Fatalf("清场后应离开层图 %d，仍在原图", r.To)
		}
		// 重新从 base 出发。
		s, e = s.enterRoom(c, catalog.DungeonRoom{X: r.Position[0], Y: r.Position[1], Map: r.From})
		if e != nil {
			t.Fatal(e)
		}
	}
}

func TestOdysseySceneAllFinalBosses(t *testing.T) {
	c := odysseyScenes(t)
	for _, d := range c.Dungeons {
		if !d.Odyssey {
			continue
		}
		for _, maze := range d.Mazes {
			t.Run(fmt.Sprintf("boss%d", d.ID), func(t *testing.T) {
				s, e := Select(c, protocol.DungeonSelection{ID: d.ID, Difficulty: d.DesignatedDifficulty, Party: 65535, Quest: uint32(maze.Quest)}, byte(d.MinimumLevel), map[uint16]bool{maze.Quest: true})
				if e != nil {
					t.Fatal(e)
				}
				var room catalog.DungeonRoom
				for _, r := range maze.Rooms {
					if r.Boss {
						room = r
					}
				}
				for _, l := range maze.Layers {
					if l.Position == maze.Boss {
						room.Map = l.Maps[len(l.Maps)-1]
					}
				}
				if d.HuntBoss != 0 {
					found := 0
					for _, candidate := range maze.Rooms {
						for _, l := range maze.Layers {
							if l.Position == [2]byte{candidate.X, candidate.Y} {
								candidate.Map = l.Maps[len(l.Maps)-1]
							}
						}
						ms, err := fixedMonsters(c.Maps[candidate.Map], d.BasisLevel)
						if err != nil {
							t.Fatal(err)
						}
						for _, m := range ms {
							if m.Template == d.HuntBoss && m.Rank == 3 {
								room = candidate
								found++
							}
						}
					}
					if found != 1 {
						t.Fatalf("source hunt target maps=%d", found)
					}
				}
				s, e = s.enterRoom(c, room)
				if e != nil {
					t.Fatal(e)
				}
				s.Loaded = true
				var target uint16
				for _, m := range s.Monsters {
					if m.Rank == 3 && (d.HuntBoss == 0 || m.Template == d.HuntBoss) {
						target = m.Entity
					}
				}
				if target == 0 {
					t.Fatalf("final map%d lacks fixed boss", room.Map)
				}
				if e = s.BossCheck(protocol.BossCheckRequest{Actor: 14, Target: target}, 14); e != nil {
					t.Fatal(e)
				}
				for _, m := range s.Monsters {
					if _, e = s.ConfirmDeath(uint32(m.Entity), 14, 14); e != nil {
						t.Fatal(e)
					}
				}
				if !s.Completed() {
					t.Fatal("did not complete after all source death reports")
				}
			})
		}
	}
}
