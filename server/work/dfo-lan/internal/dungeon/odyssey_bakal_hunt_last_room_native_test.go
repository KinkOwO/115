package dungeon_test

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"os"
	"testing"
)

// 奥德赛巴卡尔（dungeon 100004971）的通关格是 maze 的**最后一格**，不是脚本标的 boss 坐标。
//
//	DFO_PVF_CORE_TEST_ARCHIVE=D:/DNF115/115US/115-main/server/work/client-build/Script.inner.pvf
//	go test ./internal/dungeon/ -run 'BakalHuntBossLastRoom' -count=1 -v
//
// 当前 PVF 直读出来的结构（9x4，start (0,1)，[boss] (6,0)）：
//
//	(6,0) map=100016540 109019144 rank=3 team=100 **NonCombat**（三阶段演出假身）
//	                       + 109019534 rank=0 team=0 NonCombat
//	(7,0) map=100016547 109015349 rank=0 team=100
//	(8,0) map=100016548 脚本声明的 [hunt boss] 109019257 rank=3 team=100 可击杀
//
// 实机 2026-10-05（runtime/...20261005_004920_165334_next37/events.jsonl 16:52:59、16:56:45）：
// 玩家进入 (6,0)、客户端发完 CMD37，服务端在 300ms 内自行发出 N37/N291/N115/N31 通关链，
// 屏幕上的巴卡尔还有 99% 血条；随后玩家在 boss 房死亡，金币/Pilot/硬币三条复活路径因
// resultSent 已置位一律拒绝，10 秒失败结算直接弹回城镇 —— 即「boss 房死亡不复活 + 提前标记通关」。
func TestBakalHuntBossLastRoomNative(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for the Odyssey Bakal hunt-boss room layout")
	}
	source, err := gamedata.Open(gamedata.Options{
		Mode:             gamedata.PVF,
		ArchivePath:      path,
		ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256"),
		DerivedCacheDir:  t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	world, err := source.World("")
	if err != nil {
		t.Fatal(err)
	}
	c, err := source.RuntimeFullDungeons(world, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseMapSource()

	d, ok := c.Dungeons[100004971]
	if !ok {
		t.Fatal("dungeon 100004971 (Odyssey Bakal) is missing from the source")
	}
	if !d.Odyssey || d.HuntBoss != 109019257 {
		t.Fatalf("100004971 odyssey=%v huntBoss=%d, want Odyssey with 109019257", d.Odyssey, d.HuntBoss)
	}

	start, err := dungeon.Select(c, protocol.DungeonSelection{ID: 100004971, Difficulty: byte(d.DesignatedDifficulty), Party: 65535}, 115, nil)
	if err != nil {
		t.Fatalf("Select(100004971) failed: %v", err)
	}
	reached := walkMaze(t, c, start)

	// (6,0)：脚本声明的 boss 坐标。进房加载完成**不许**结算 —— hunt 目标不在场。
	bossRoom := reached[100016540]
	if bossRoom == nil {
		t.Fatal("cannot reach the declared boss room map 100016540")
	}
	if !bossRoom.Room.Boss || [2]byte{bossRoom.Room.X, bossRoom.Room.Y} != bossRoom.Maze.Boss {
		t.Fatalf("100016540 must be the source boss room, got boss=%v pos=(%d,%d) mazeBoss=%v",
			bossRoom.Room.Boss, bossRoom.Room.X, bossRoom.Room.Y, bossRoom.Maze.Boss)
	}
	for _, m := range bossRoom.Monsters {
		if m.Template == d.HuntBoss {
			t.Fatalf("hunt target %d unexpectedly stands in the display-boss room", d.HuntBoss)
		}
	}
	bossRoom.TryComplete()
	if bossRoom.Completed() {
		t.Error("display-boss room settled the run on entry while the declared hunt target is elsewhere")
	}

	// (8,0)：最后一格摆着可击杀的 hunt 真身；客户端为它发 CMD117 时按奥德赛契约结算。
	final := reached[100016548]
	if final == nil {
		t.Fatal("cannot reach the final room map 100016548")
	}
	var target uint16
	for _, m := range final.Monsters {
		if m.Template == d.HuntBoss && m.Rank == 3 && !m.NonCombat && !m.APC && m.Team != 0 {
			target = m.Entity
		}
	}
	if target == 0 {
		t.Fatalf("final room 100016548 carries no live rank-3 hunt boss %d", d.HuntBoss)
	}
	final.TryComplete()
	if final.Completed() {
		t.Fatalf("final room settles on entry before the hunt boss dies (monsters=%d)", len(final.Monsters))
	}
	if err := final.BossCheck(protocol.BossCheckRequest{Actor: 7, Target: target}, 7); err != nil {
		t.Fatalf("BossCheck(%d) refused: %v", target, err)
	}
	if !final.Completed() {
		t.Error("Odyssey run must complete once the client reports its hunt boss")
	}
}

// walkMaze 从起始格按相邻格一路走完整张源 maze，返回 map id 到已加载会话的映射。
func walkMaze(t *testing.T, c catalog.DungeonCatalog, start *dungeon.Session) map[uint32]*dungeon.Session {
	t.Helper()
	reached := map[uint32]*dungeon.Session{}
	seen := map[[2]byte]bool{{start.Room.X, start.Room.Y}: true}
	start.Loaded = true
	queue := []*dungeon.Session{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		reached[cur.Room.Map] = cur
		for _, step := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			pos := [2]byte{uint8(int(cur.Room.X) + step[0]), uint8(int(cur.Room.Y) + step[1])}
			if seen[pos] {
				continue
			}
			seen[pos] = true
			next, err := cur.Move(c, pos)
			if err != nil {
				continue
			}
			next.Loaded = true
			queue = append(queue, next)
		}
	}
	return reached
}
