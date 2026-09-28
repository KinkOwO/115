package dungeon

import (
	"os"
	"path/filepath"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
)

// [MERGE-20260928-START-LAYER-EXIT] 实机回归（2026-09-28 晦月湖 100004777，第二段剧情闪退）。
//
// 起点 (0,0) 的层图 100015633 放完剧情点门要前进到 (1,0)，而前进那一包的 StartMap
// 必须带上**进这一格时客户端带来的换图记录** —— 客户端靠 Transition 安置角色，留默认
// 全 f 会落点错位，紧接的第二段剧情一开就崩（events: 199 带记录 0405bc02.. 正常，
// 208 只剩 ffffffff.. 之后 exit_shutdown_signal）。
//
// 锁住：进层图时存下的记录，能被 SceneEntryRecord 原样取回。
func TestSceneEntryRecordIsRetained(t *testing.T) {
	raw, e := os.ReadFile(filepath.Join("..", "..", "configs", "dungeons.full.json"))
	if e != nil {
		t.Skip("full catalog missing:", e)
	}
	tmp := filepath.Join(t.TempDir(), "d.json")
	if e = os.WriteFile(tmp, raw, 0o600); e != nil {
		t.Fatal(e)
	}
	c, e := catalog.LoadDungeons(tmp)
	if e != nil {
		t.Fatal(e)
	}
	d, ok := c.Dungeons[100004777]
	if !ok {
		t.Fatal("100004777 不在 full 导出里")
	}
	mz := d.Mazes[0]

	var startLayer []uint32
	for _, l := range mz.Layers {
		if l.Position == mz.Start {
			startLayer = l.Maps
		}
	}
	if len(startLayer) == 0 {
		t.Fatalf("起点 %v 没有层图，前提变了", mz.Start)
	}

	// 客户端进层图时带来的落点记录（同实机 199 行的形态）。
	entry := [18]byte{0, 0, 0, 0, 4, 5, 0xbc, 0x02, 0x2c, 0x01, 0, 0, 0, 0, 0, 0, 0, 0}
	s := &Session{
		Definition: d, Loaded: true, Dead: map[uint16]bool{},
		Room: catalog.DungeonRoom{X: mz.Start[0], Y: mz.Start[1], Map: startLayer[len(startLayer)-1]},
		Maze: mz,
	}
	script, ok := c.Maps[s.Room.Map]
	if !ok {
		t.Fatalf("map %d 未导入", s.Room.Map)
	}
	if s.Monsters, e = fixedMonsters(script, d.BasisLevel); e != nil {
		t.Fatal(e)
	}

	// 用「客户端主动进层图」那一包（带记录）走一次 MoveScene，记录应被留存。
	if _, e = s.MoveScene(c, protocol.DungeonRoomTransition{
		Dungeon: 100004777, Position: mz.Start, LayerChange: true, Record: entry,
	}); e != nil {
		t.Fatalf("进层图应成功: %v", e)
	}
	got, ok := s.SceneEntryRecord()
	if !ok {
		t.Fatal("进层图带记录时，SceneEntryRecord 应报告已留存")
	}
	if got != entry {
		t.Fatalf("留存记录不一致:\n  want %x\n  got  %x", entry, got)
	}
	t.Logf("留存换图记录 %x，可被前进那一包复用", got)
}
