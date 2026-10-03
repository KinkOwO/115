package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestSourceBossRequiresSeparateStoryDeathAndCheck(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	for _, early := range []bool{false, true} {
		s, e := Select(c, protocol.DungeonSelection{ID: 3, Party: 65535, Quest: 3145}, 1, map[uint16]bool{3145: true})
		if e != nil {
			t.Fatal(e)
		}
		check := protocol.BossCheckRequest{Actor: 3, Target: s.Monsters[0].Entity}
		s.Loaded = true
		if e = s.BossCheck(check, 3); e == nil {
			t.Fatal("nonboss room completion")
		}
		for _, xy := range [][2]byte{{1, 1}, {1, 0}, {2, 0}, {3, 0}} {
			for _, m := range s.Monsters {
				if !m.NonCombat {
					_, e = s.ConfirmDeath(uint32(m.Entity), 3, 3)
					if e != nil {
						t.Fatal(e)
					}
				}
			}
			s, e = s.Move(c, xy)
			if e != nil {
				t.Fatal(e)
			}
			s.Loaded = true
		}
		check.Target = s.Monsters[0].Entity
		if e = s.BossCheck(check, 4); e == nil {
			t.Fatal("wrong actor")
		}
		unknown := check
		unknown.Target = 60000
		if e = s.BossCheck(unknown, 3); e == nil {
			t.Fatal("unknown boss")
		}
		if early {
			if e = s.BossCheck(check, 3); e != nil {
				t.Fatal(e)
			}
		}
		for _, m := range s.Monsters {
			if !m.NonCombat {
				_, e = s.ConfirmDeath(uint32(m.Entity), 3, 3)
				if e != nil {
					t.Fatal(e)
				}
			}
		}
		if !s.RoomCleared() || s.Completed() {
			t.Fatal("ordinary clear bypassed story boss")
		}
		if !early {
			if e = s.BossCheck(check, 3); e != nil {
				t.Fatal(e)
			}
		}
		if s.Completed() {
			t.Fatal("boss check fabricated dummy death")
		}
		_, e = s.ConfirmDeath(uint32(s.Monsters[2].Entity), 3, 3)
		if e != nil {
			t.Fatal(e)
		}
		if !s.Completed() || s.CompletionTarget() != check.Target {
			t.Fatal("final confirmation failed")
		}
		if e = s.BossCheck(check, 3); e != nil {
			t.Fatal("identical replay", e)
		}
		other := check
		other.Target = s.Monsters[2].Entity
		if e = s.BossCheck(other, 3); e == nil {
			t.Fatal("conflicting replay")
		}
		if _, e = s.Move(c, [2]byte{2, 0}); e == nil {
			t.Fatal("left completed boss room by normal gate")
		}
	}
}

func TestOdysseyBossCheckImmediateCompletion(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.odyssey-scenes-release.json")
	if e != nil {
		t.Fatal(e)
	}
	s, e := Select(c, protocol.DungeonSelection{ID: 100004969, Difficulty: 2, Party: 65535}, 89, nil)
	if e != nil {
		t.Fatal(e)
	}
	s.Loaded = true
	// BossCheck in Odyssey should be immediately admitted and complete the dungeon
	check := protocol.BossCheckRequest{Actor: 10, Target: 9999}
	if err := s.BossCheck(check, 10); err != nil {
		t.Fatalf("expected Odyssey BossCheck to pass, got: %v", err)
	}
	if !s.Completed() || s.CompletionTarget() != 9999 {
		t.Fatalf("expected dungeon to be completed with target 9999, got completed=%v target=%d", s.Completed(), s.CompletionTarget())
	}
}

// [ISPINS-ARENA-BOSS] 军团阶段本（伊斯大陆 nemaug 100002987）的结算回归：
// 源迷宫把 boss 坐标标在 (0,0)/100006472，官服 s4 却在 start 房 (1,1)/100006476
// 开打并结算（帧 451/337/495）。未置 ArenaBoss 时 BossCheck 必须维持拒绝
//（2026-10-03 实测回归原因：`boss check target is not a source boss in this
// room`，整场无结算）；置位后 CMD117 受理 + 死亡驱动结算双路可用。
func TestIspinsArenaBossCompletion(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.full.json")
	if e != nil {
		t.Fatal(e)
	}
	newSession := func() *Session {
		s, e := Select(c, protocol.DungeonSelection{ID: 100002987, Party: 65535}, 115, nil)
		if e != nil {
			t.Fatal(e)
		}
		// 进本落在 start 房：官服实证（帧 451 Boss 位置 (1,1) 不变，N28 帧 449
		// 仍回源迷宫 boss 坐标 (0,0)）。
		if s.Room.X != 1 || s.Room.Y != 1 || s.Room.Map != 100006476 || s.Room.Boss {
			t.Fatalf("unexpected entry room %+v", s.Room)
		}
		if len(s.Monsters) != 1 || s.Monsters[0].Rank != 3 {
			t.Fatalf("expected the single rank-3 nemaug boss, got %+v", s.Monsters)
		}
		return s
	}
	target := func(s *Session) uint16 { return s.Monsters[0].Entity }

	s := newSession()
	s.Loaded = true
	// 回归护栏：不置 ArenaBoss 的会话必须在 start 房拒绝 CMD117。
	if e := s.BossCheck(protocol.BossCheckRequest{Actor: 3, Target: target(s)}, 3); e == nil {
		t.Fatal("non-arena session must keep refusing the entry-room boss check")
	}

	// CMD117 驱动：置位后受理，boss 未死不结算，死亡后结算并回显目标。
	s = newSession()
	s.ArenaBoss = true
	s.Loaded = true
	if e := s.BossCheck(protocol.BossCheckRequest{Actor: 3, Target: target(s)}, 3); e != nil {
		t.Fatal(e)
	}
	if s.Completed() {
		t.Fatal("boss check must not complete before the boss dies")
	}
	if e := s.BossCheck(protocol.BossCheckRequest{Actor: 3, Target: target(s) + 1}, 3); e == nil {
		t.Fatal("arena waiver must still require a real source boss target")
	}
	if _, e = s.ConfirmDeath(uint32(target(s)), 3, 3); e != nil {
		t.Fatal(e)
	}
	if !s.Completed() || s.CompletionTarget() != target(s) {
		t.Fatal("arena boss death must complete the run with the reported target")
	}

	// 死亡驱动：官服 s4 后两个阶段没有 CMD117 也照常结算（4 条 N31），
	// completionTarget 恒为 0 时由进房清空兜底。
	s = newSession()
	s.ArenaBoss = true
	s.Loaded = true
	if _, e = s.ConfirmDeath(uint32(target(s)), 3, 3); e != nil {
		t.Fatal(e)
	}
	if !s.Completed() {
		t.Fatal("arena boss death must complete the run without CMD117")
	}
	if s.CompletionTarget() == 0 {
		t.Fatal("death-driven completion must still expose a reportable target for NOTI115")
	}
}

func TestDungeon22CinematicActorDoesNotBlockBossCompletion(t *testing.T) {
	c, err := catalog.LoadDungeons("../../configs/dungeons.full.json")
	if err != nil {
		t.Fatal(err)
	}
	d := c.Dungeons[22]
	var maze catalog.DungeonMaze
	for _, m := range d.Mazes {
		if m.Index == 1 {
			maze = m
			break
		}
	}
	var room catalog.DungeonRoom
	for _, r := range maze.Rooms {
		if r.Map == 53371 {
			room = r
			break
		}
	}
	monsters, err := fixedMonsters(c.Maps[53371], d.BasisLevel)
	if err != nil {
		t.Fatal(err)
	}
	s := &Session{Definition: d, Maze: maze, Room: room, Monsters: monsters, Loaded: true, Dead: map[uint16]bool{}}
	var boss, display, cinematic uint16
	for _, m := range monsters {
		switch {
		case m.Template == 61103 && m.Rank == 3 && m.Team == 100:
			boss = m.Entity
		case m.Template == 63821 && m.Rank == 3 && m.Team == 100 && m.NonCombat:
			display = m.Entity
		case m.Template == 109015630 && m.Rank == 3 && m.Team == 0:
			cinematic = m.Entity
		}
	}
	if boss == 0 || display == 0 || cinematic == 0 {
		t.Fatalf("unexpected dungeon 22 boss actors: boss=%d display=%d cinematic=%d", boss, display, cinematic)
	}
	s.completionTarget = boss
	s.Dead[boss] = true
	s.tryComplete()
	if s.Completed() {
		t.Fatal("completed before the team-100 display boss died")
	}
	s.Dead[display] = true
	s.tryComplete()
	if !s.Completed() || s.CompletionTarget() != boss {
		t.Fatalf("team-0 actor blocked completion: completed=%v target=%d", s.Completed(), s.CompletionTarget())
	}
}
