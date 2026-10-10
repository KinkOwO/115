package dungeon

import "fmt"

// 次元回廊（军团家族）用到的会话适配器。
//
// 为什么需要它们：次元回廊的怪物**不是**本会话刷出来的，而是由回放的官服进图帧列
// （N29 START_MAP 等）在客户端生成的；客户端上报 CMD39（怪物死亡）时带的是**它自己
// 那份怪物编号**。会话这份 `.dgn` 怪物表原先自己从 4096 起编号，于是
// `ConfirmDeath` / `BossCheck` 一律查不到客户端报的号 —— 实机表现就是
// 「BOSS 死了却不通关、没有横幅、没有翻牌」。
//
// 见 `internal/legion/dimension_cloister_entity.go` 顶部的完整证据链。

// LegionMonsterEntities 返回当前房间怪物表的编号列（顺序与 `Monsters` 一致）。
func (s *Session) LegionMonsterEntities() []uint16 {
	if s == nil {
		return nil
	}
	out := make([]uint16, len(s.Monsters))
	for i := range s.Monsters {
		out[i] = s.Monsters[i].Entity
	}
	return out
}

// SetLegionMonsterEntities 用给定的编号列整体改写当前房间怪物表的编号。
//
// 逐项校验后再整体落地：任何一个编号不合法（0 / 65535）或与别的怪物重复，
// 本次调用**一个字节都不改**，调用方据此退化到旧行为。
func (s *Session) SetLegionMonsterEntities(entities []uint16) error {
	if s == nil {
		return fmt.Errorf("次元回廊编号对齐缺少会话")
	}
	if len(entities) != len(s.Monsters) {
		return fmt.Errorf("次元回廊编号列 %d 项、会话怪物 %d 只，不匹配",
			len(entities), len(s.Monsters))
	}
	seen := make(map[uint16]bool, len(entities))
	for i, entity := range entities {
		if entity == 0 || entity == 65535 {
			return fmt.Errorf("次元回廊第 %d 只怪物的编号 %d 无效", i, entity)
		}
		if seen[entity] {
			return fmt.Errorf("次元回廊第 %d 只怪物的编号 %d 重复", i, entity)
		}
		seen[entity] = true
	}
	for i, entity := range entities {
		s.Monsters[i].Entity = entity
	}
	return nil
}

// LegionArenaRosterHasBoss 报告当前房间怪物表里是否有一只「可上报的领主」
// （rank3 或 rank5..8 的 APC，且不是剧情演员）。
//
// 次元回廊进图时用来做一次安全检查：编号对齐**必须**落在真领主身上，
// 否则宁可当场报错，也不要拿一张错位的表去认「BOSS 已死」。
func (s *Session) LegionArenaRosterHasBoss() bool {
	if s == nil {
		return false
	}
	for _, m := range s.Monsters {
		if m.NonCombat || m.Team == 0 {
			continue
		}
		if m.Rank == 3 || m.APC && m.Rank >= 5 && m.Rank <= 8 {
			return true
		}
	}
	return false
}

// LegionArenaBossDown 报告「这只上报的领主已确认死亡之后，本房间还有没有活着的领主」。
//
// 次元回廊的每一关就是一场 boss 战（进图房间即竞技场），领主全灭即本界清关 ——
// 这比 `roomEnemiesDead()` 宽松：客户端不会为领主带走的杂兵逐个上报死亡，
// 而本族的会话里根本没有那些杂兵。
func (s *Session) LegionArenaBossDown() bool {
	if s == nil {
		return false
	}
	sawBoss := false
	for _, m := range s.Monsters {
		if m.NonCombat || m.Team == 0 {
			continue
		}
		if m.Rank != 3 && !(m.APC && m.Rank >= 5 && m.Rank <= 8) {
			continue
		}
		sawBoss = true
		if !s.Dead[m.Entity] {
			return false
		}
	}
	return sawBoss
}
