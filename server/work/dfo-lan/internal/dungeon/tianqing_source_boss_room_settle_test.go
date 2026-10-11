package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

// [MERGE-20261005-BOSS-ROOM-DEAD-ECHO] 实机回归（2026-10-05，dungeon 88
// `North Ghent Gate` = dungeon/act7/gentnorthgate_r.dgn，maze 0 的 boss 格
// (6,1)=map 22443）：
//
// 玩家把 boss 房里的 11 只怪**全部**打死（含 `61236 … [fixed] [boss]` 的 rank3 boss），
// 而客户端**一个 CMD117 都不发** —— 因为同一张图的 `[special passive object]` 里
// 还挂着 `10283 … [trap] 2620`，客户端把它算进「房间是否清空」。服务端旧有的兜底
// 要么要求 `!hasFightableBoss()`（display-boss 版），要么要求 `SourceBoss != 0`
// （带 hunt boss 的副本），两条在这个副本上都不成立 ⇒ 副本打完却不结算。
//
// 这条钉住新加的兜底：源 maze 声明的 boss 格 + 房里有可战斗 boss + 房里再无活怪。
func TestSourceBossRoomSettlesWhenClientSkipsBossCheck(t *testing.T) {
	c, err := catalog.LoadDungeons(tianqingDungeonFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	def, ok := c.Dungeons[88]
	if !ok || len(def.Mazes) == 0 {
		t.Fatal("dungeon 88 missing from catalog")
	}
	maze := def.Mazes[0]
	// 注意：maze 0 在源里对 (6,1) 有两条记录（`map 6 1 22443` 与 `boss 6 1 22443`），
	// 只有后者带 Boss 标记 —— atSourceBossMap() 认的是它。
	var boss catalog.DungeonRoom
	for _, r := range maze.Rooms {
		if [2]byte{r.X, r.Y} == maze.Boss && r.Boss {
			boss = r
			break
		}
	}
	if boss.Map != 22443 {
		t.Fatalf("dungeon 88 maze 0 的 boss 格应为 (6,1)=22443，实际 %+v（源已变，先核对）", boss)
	}

	s, err := Select(c, protocol.DungeonSelection{ID: 88, Difficulty: 1, Party: 65535}, 57, nil)
	if err != nil {
		t.Fatalf("Select(88) 失败，前提已变：%v", err)
	}
	next, err := s.enterRoom(c, boss)
	if err != nil {
		t.Fatalf("enterRoom(22443) 失败：%v", err)
	}
	next.Loaded = true
	if !next.atSourceBossMap() {
		t.Fatal("应被认作源 maze 声明的 boss 格")
	}
	if !next.hasFightableBoss() {
		t.Fatal("22443 里应当有可战斗 boss（rank3/team100）")
	}
	if next.roomEnemiesDead() {
		t.Fatal("怪还没死时不该算清空")
	}

	// 客户端没发 CMD117 —— completionTarget 保持 0。
	if next.completionTarget != 0 {
		t.Fatalf("前提已变：completionTarget = %d", next.completionTarget)
	}
	next.TryComplete()
	if next.Completed() {
		t.Fatal("房里还有活怪时不该结算")
	}

	// 玩家把 11 只全打死。
	if next.Dead == nil {
		next.Dead = map[uint16]bool{}
	}
	for _, m := range next.Monsters {
		next.Dead[m.Entity] = true
	}
	next.TryComplete()
	if !next.Completed() {
		t.Fatal("boss 房怪全死、客户端不发 CMD117 时，应当由服务端结算")
	}
}

// 反面守卫：普通（非 boss）房间的怪全死不该结算 —— 这条兜底必须严格锁在
// 「源 maze 声明的 boss 格」上，否则半路清一个房间就会被判通关。
func TestNonBossRoomDoesNotSettleOnClear(t *testing.T) {
	c, err := catalog.LoadDungeons(tianqingDungeonFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	def := c.Dungeons[88]
	maze := def.Mazes[0]
	var mid catalog.DungeonRoom
	for _, r := range maze.Rooms {
		if [2]byte{r.X, r.Y} == [2]byte{1, 1} {
			mid = r
			break
		}
	}
	if mid.Boss || mid.Map == 0 {
		t.Fatalf("源已变：maze 0 的 (1,1) 不再是普通格（%+v）", mid)
	}
	s, err := Select(c, protocol.DungeonSelection{ID: 88, Difficulty: 1, Party: 65535}, 57, nil)
	if err != nil {
		t.Fatal(err)
	}
	next, err := s.enterRoom(c, mid)
	if err != nil {
		t.Fatal(err)
	}
	next.Loaded = true
	if next.Dead == nil {
		next.Dead = map[uint16]bool{}
	}
	for _, m := range next.Monsters {
		next.Dead[m.Entity] = true
	}
	next.TryComplete()
	if next.Completed() {
		t.Fatalf("普通格 (%d,%d) 清空后不该结算", mid.X, mid.Y)
	}
}
