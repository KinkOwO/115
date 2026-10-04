package dungeon_test

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/gamedata"
	"dfolan/internal/game/protocol"
	"os"
	"testing"
)

// 蔚蓝号（Azure Main，dungeon 100004131）的迷宫/房间结构：把**当前 PVF 直读**出来的结果
// 与官服 2026-10-03 抓包里那 7 帧 NOTI29 的房间序列对照。
//
//	DFO_PVF_CORE_TEST_ARCHIVE=D:\115us\server\work\client-build\Script.inner.pvf
//	go test ./internal/dungeon/ -run 'AzureMainMaze' -count=1 -v
//
// 官服实测房间（E:/迅雷下载/20261003-214424/decoded/F16-s2c.txt #417/#428/#495/#532/#539/#586/#595）：
//
//	pos(0,0) map=100012699 0 只
//	pos(1,0) map=100012700 15 只
//	pos(1,1) map=100012705 2 只
//	pos(1,2) map=100012701 0 只
//	pos(1,3) map=100012710 2 只
//	pos(1,2) flag=2（ExitLayer 换层形态）
//	pos(2,2) map=100012704 2 只（含 BOSS 109017562 rank=3）
func TestAzureMainMazeNative(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for Azure Main maze parity")
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

	d, ok := c.Dungeons[100004131]
	if !ok {
		t.Fatal("dungeon 100004131 (Azure Main) is missing from the source")
	}
	t.Logf("dungeon 100004131: mazes=%d minLevel=%d basisLevel=%d Tutorial=%v Odyssey=%v SourceBoss=%d HuntBoss=%d",
		len(d.Mazes), d.MinimumLevel, d.BasisLevel, d.Tutorial, d.Odyssey, d.SourceBoss, d.HuntBoss)
	official := map[uint32]bool{
		100012699: true, 100012700: true, 100012705: true,
		100012701: true, 100012710: true, 100012704: true,
	}
	found := map[uint32]bool{}
	for _, m := range d.Mazes {
		t.Logf("  maze %d: size=%v start=%v boss=%v quest=%d questFlag=%d rooms=%d layers=%d pending=%v",
			m.Index, m.Size, m.Start, m.Boss, m.Quest, m.QuestFlag, len(m.Rooms), len(m.Layers), m.Pending)
		for _, lay := range m.Layers {
			t.Logf("     layer pos=(%d,%d) maps=%v", lay.Position[0], lay.Position[1], lay.Maps)
		}
		for _, r := range m.Rooms {
			t.Logf("     room (%d,%d) map=%d boss=%v alternates=%v", r.X, r.Y, r.Map, r.Boss, r.Alternates)
			if official[r.Map] {
				found[r.Map] = true
			}
		}
	}
	// 官服的 6 个不同房间地图必须都能在当前源里找到（第 7 帧是 (1,2) 的换层形态，
	// 复用同一个 map，所以只断言 6 个不同的 id）。
	for id := range official {
		if !found[id] {
			t.Errorf("official room map %d is missing from the source maze of 100004131", id)
		}
	}
	_ = catalog.DungeonRoom{}

	// 迷宫选择必须与官服抓包一致：N28 `[7]` = 1（F16-s2c.txt #414）。
	// 默认规则取 index 最小（= 0），而主路径上两者只有 (1,1) 不同 ——
	// maze 1 = 100012705（官服实测 2 只怪），maze 0 = 100012707（本服实测 27 只）。
	s, err := dungeon.Select(c, protocol.DungeonSelection{ID: 100004131, Difficulty: 0, Party: 65535}, 115, nil)
	if err != nil {
		t.Fatalf("Select(100004131) failed: %v", err)
	}
	if s.Maze.Index != 1 {
		t.Fatalf("Azure Main must use maze 1 (official capture), got %d", s.Maze.Index)
	}
	if s.Room.Map != 100012699 {
		t.Fatalf("Azure Main start room = %d, want 100012699", s.Room.Map)
	}
}
