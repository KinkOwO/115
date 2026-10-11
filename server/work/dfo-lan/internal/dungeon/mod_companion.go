package dungeon

import (
	"fmt"
	"log"

	"dfolan/internal/game/protocol"
	"dfolan/internal/modcompanion"
)

// 本文件把服务端层 mod 注册的**副本同伴**接到会话的怪物表上。
//
// # 为什么同伴在副本里是"一行 AIC"而不是一个 actor
//
// 城镇假人（`internal/modactor`）靠 `(channel, town, area)` 这个场景键被别人看见；
// 副本没有这个键 —— 玩家进本时 peer 会被 `retire`，两边都不在同一个"场景"里。
// 副本里"谁在场"完全由 NOTI29（START_MAP）的怪物表决定，所以同伴要进本，就得是
// 表里的一行。
//
// 这一行用的是客户端原生能力：native145b20dc0 对 `SourceIndex == 10000` 有独立分支
// ——"creates the AIC by Template, applies the packet Team and enables following"。
// 官服正是靠它把剧情同伴带进下一间房。所以**打斗是客户端原生 AIC 完成的**，服务端
// 只负责让这一行出现在每一间房里；服务端没有 HP / 伤害 / AI，也读不到它们。
//
// # 零值语义
//
// 没有任何 mod 注册（`modcompanion.Current()` 为零值）时 `ApplyModCompanions`
// 直接返回，怪物表一个字节都不变 —— 与加这条能力之前逐位相同。

// AddCompanions 把一批友好 AIC 加进本次副本：当前房间立刻带上它们，之后每一间房
// 由既有的携带逻辑（`rememberCompanions` / `appendMissingCompanions`）带过去。
//
// 行的形状由本函数钉死，调用方只给模板与档位：
//   - `APC=true` / `Team=0` / `NonCombat=true` —— 与 map 原生 `[ai character]` 同伴
//     同形，且不参与清房判定（`RoomCleared` 只看 `!NonCombat` 的行）；
//   - `SourceIndex` 统一交给 `appendMissingCompanions` 写成 `dynamicAPCSourceIndex`
//     （10000），也就是上面那条原生分支。
//
// 名字里带 Mod 是因为它只服务服务端层 mod（`internal/modcompanion`）；原生地图同伴
// 走 `appendFixedAPCs`，两条路共用同一份携带与编码逻辑。
func (s *Session) AddCompanions(rows []protocol.DungeonMonster) error {
	if s == nil || len(rows) == 0 {
		return nil
	}
	normalized := make([]protocol.DungeonMonster, 0, len(rows))
	for _, row := range rows {
		if row.Template == 0 {
			return fmt.Errorf("副本同伴缺 AIC 模板")
		}
		if row.Rank == 0 {
			row.Rank = 5
		}
		if row.Rank < 5 || row.Rank > 8 {
			return fmt.Errorf("副本同伴 AIC %d 的档位 %d 超出 5..8", row.Template, row.Rank)
		}
		// 编号/来源索引/尾巴一律重新分配：调用方给的这些值没有意义，
		// 留着只会让同一行在不同房间里带着不同的编号。
		row.Entity = 0
		row.APC = true
		row.Team = 0
		row.NonCombat = true
		row.SourceIndex = 0
		row.SourceTail = [2]int32{}
		row.SpawnOrder = 0
		row.Hidden = false
		normalized = append(normalized, row)
	}
	merged := rememberCompanions(s.companions, normalized)
	monsters, err := appendMissingCompanions(s.Monsters, merged, &s.NextEntity, s.monsterEntityStride())
	if err != nil {
		return err
	}
	s.companions = merged
	s.Monsters = monsters
	if s.Visited == nil {
		s.Visited = map[uint32][]protocol.DungeonMonster{}
	}
	s.Visited[s.Room.Map] = monsters
	return nil
}

// ApplyModCompanions 把当前注册的副本同伴加进这个会话；没有注册就什么都不做。
//
// 失败**不返回错误**：同伴是增益，不该因为多一行 AIC 就让玩家进不了副本。
// 失败原因写日志 —— 实际会碰到的只有两种：房间怪物已到 255 上限，实体编号用尽。
func (s *Session) ApplyModCompanions() {
	if s == nil {
		return
	}
	rows := modCompanionRows(s.Definition.ID)
	if len(rows) == 0 {
		return
	}
	if err := s.AddCompanions(rows); err != nil {
		log.Printf("副本同伴未加入 dungeon=%d map=%d：%v", s.Definition.ID, s.Room.Map, err)
		return
	}
	log.Printf("副本同伴已加入 dungeon=%d map=%d：%d 个（AIC 模板见 mods/town.companion/config.json）",
		s.Definition.ID, s.Room.Map, len(rows))
}

// modCompanionRows 把已注册的同伴翻成怪物行；没有注册或该副本被排除时返回 nil。
func modCompanionRows(dungeonID uint32) []protocol.DungeonMonster {
	rules := modcompanion.Current()
	if !rules.AppliesTo(dungeonID) {
		return nil
	}
	rows := make([]protocol.DungeonMonster, 0, len(rules.Companions))
	for _, c := range rules.Companions {
		rows = append(rows, protocol.DungeonMonster{Template: c.Template, Rank: c.Rank})
	}
	return rows
}
