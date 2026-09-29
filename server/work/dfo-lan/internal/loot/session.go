package loot

import (
	"crypto/rand"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"fmt"
	"sync"
)

type Drop struct {
	Run    string
	Map    uint32
	Owner  uint16
	Object uint32
	Slot   uint16
	Award  Award
}
type Session struct {
	Currency    *OdysseyCurrency
	ChapterDrop *OdysseyChapterDrop
	Attunement  *AttunementRewards
	RewardBoxes RewardBoxSource
	// Omen 是征兆系统的按角色累积账（见 omen.go）。为 nil 时这条线完全不推进。
	Omen                  *OmenLedger
	QuestDropBonusPercent int
	// attunementRolled 保证一轮只抽一次专属奖励：同一只源领主再被确认死亡
	// （或同模板的第二只 rank3）都不会重复发奖。
	attunementRolled bool
	// omenRolled 与 attunementRolled 同理：同一场只推进一次征兆。
	omenRolled         bool
	mu                 sync.Mutex
	Catalog            catalog.LootCatalog
	Tables             Tables
	Rules              Rules
	Equipment          *inventory.EquipmentCatalog
	Run                string
	Account, Character int64
	Actor              uint16
	next               uint32
	seeds              map[uint32]uint32
	deaths             map[uint16][]protocol.SceneDrop
	Objects            map[uint32]Drop
	Skipped            map[uint16][]string
}

func NewSession(c catalog.LootCatalog, t Tables, r Rules, equipment *inventory.EquipmentCatalog, run string, account, character int64, actor uint16) *Session {
	return &Session{Catalog: c, Tables: t, Rules: r, Equipment: equipment, Run: run, Account: account, Character: character, Actor: actor, next: 1, seeds: map[uint32]uint32{}, deaths: map[uint16][]protocol.SceneDrop{}, Objects: map[uint32]Drop{}, Skipped: map[uint16][]string{}}
}
func (s *Session) Death(d *dungeon.Session, entity uint16) ([]protocol.SceneDrop, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d == nil || d.RunID != s.Run || !d.Loaded || !d.Dead[entity] {
		return nil, fmt.Errorf("drop without owned confirmed death")
	}
	var monster *protocol.DungeonMonster
	for _, m := range d.Monsters {
		if m.Entity == entity {
			v := m
			monster = &v
			break
		}
	}
	if monster == nil {
		return nil, fmt.Errorf("drop target outside current room")
	}
	if p, ok := s.deaths[entity]; ok {
		return append([]protocol.SceneDrop(nil), p...), nil
	}
	if monster.NonCombat || monster.APC || monster.Level == 0 || d.Unowned[entity] {
		s.deaths[entity] = nil
		return nil, nil
	}
	seed, ok := s.seeds[d.Room.Map]
	if !ok {
		if e := binary.Read(rand.Reader, binary.LittleEndian, &seed); e != nil {
			return nil, e
		}
	}
	result := Outcome{NextSeed: seed}
	excludeGold, excludeRandom := dungeonDropExclusions(d.Definition)
	if !excludeGold || !excludeRandom {
		var e error
		result, e = RollWithBonus(s.Catalog, s.Tables, s.Rules, s.Equipment.DropPool(), seed, monster.Level, monster.Rank, difficultyIndex(s.Rules, d.Difficulty), s.QuestDropBonusPercent)
		if e != nil {
			return nil, e
		}
	}
	result.Awards = filterDungeonAwards(d.Definition, result.Awards)
	if d.Definition.Odyssey && s.Currency != nil {
		if s.Currency.Source != s.Catalog.Source.Checksum {
			return nil, fmt.Errorf("Odyssey currency source mismatch")
		}
		coins, next, err := s.Currency.Roll(result.NextSeed, monster.Rank)
		if err != nil {
			return nil, err
		}
		result.Awards = append(result.Awards, coins...)
		result.NextSeed = next
	}
	// 章节最终领主的章节盒（手册 P3 子项 3）。条件是"确认死亡的怪 Rank==3 且该副本
	// 是某章 final"；整表默认关闭，禁用时 Roll 原样返回种子 ⇒ 不扰动其它掉落。
	if d.Definition.Odyssey && s.ChapterDrop != nil && monster.Rank == 3 {
		box, next, err := s.ChapterDrop.Roll(result.NextSeed, d.Definition.ID)
		if err != nil {
			return nil, err
		}
		result.Awards = append(result.Awards, box...)
		result.NextSeed = next
	}
	// 调律之边界（深渊）：源领主死亡时给专属奖励。触发器取自副本脚本自己写的
	// [clear condition] [hunt boss] 领主（DungeonDefinition.SourceBoss）—— 这与通关判定锚在
	// 同一个事实上，而不是再叠一道「副本是否已通关」的闸门：该闸门多余，且一旦
	// 时序不同就会静默扣下奖励，正是本次要消灭的失败形态。
	// 没有奖励表的副本在这里连种子都不消耗。
	// 没有奖励表的副本在这里连种子都不消耗：Roll 按副本查表，查不到就原样返回种子。
	// 所以把触发器放宽到「任何声明了源领主的副本」不会给别的副本发奖。
	if s.Attunement.Enabled() && d.Definition.SourceBoss != 0 &&
		monster.Rank == 3 && monster.Template == d.Definition.SourceBoss && !s.attunementRolled {
		awards, next, err := s.Attunement.Roll(result.NextSeed, d.Definition.ID, uint32(d.Maze.Index))
		if err != nil {
			return nil, err
		}
		// 征兆（omen）：本副本的通关判定与源领主的死亡是同一个事实（见上），所以
		// 征兆也在这里推进。它和固定奖励走**同一条**开箱路径（见下面的统一展开），
		// 分两条路就等于同一件东西有两个分布。
		if s.Omen != nil && !s.omenRolled {
			outcome, omenAwards, err := s.Omen.Advance(s.Character, d.Definition.ID, next)
			if err != nil {
				return nil, err
			}
			awards = append(awards, omenAwards...)
			next = outcome.Seed
			s.omenRolled = true
		}
		s.attunementRolled = true
		result.Awards = append(result.Awards, awards...)
		result.NextSeed = next
	}
	// 包装展开：**所有来源统一在这里做一次** —— 通用掉落池、章节盒、调律专属奖励、征兆。
	//
	// 源自己写着「不实际发放礼盒，以开封状态发放」（这批盒子的客户端文案就是这个），
	// 所以礼盒落到玩家脚下，在任何一条线上都是错的 —— 它打不开。此前展开只挂在调律
	// 那一块，于是通用掉落给出的包装原样落地，实机反馈里那批盒子就是这么来的。
	//
	// 不含包装的掉落在这里**连随机数都不消耗**（OpenRewardBoxes 只在真的开箱时才掷骰），
	// 所以对没有包装的副本，这条改动逐字节等于旧行为。金币的 template 0 在物品目录里
	// 是 stackable，会被原样放行，不会被当成奖励表那个「本次没有」的空槽吃掉。
	if len(result.Awards) > 0 {
		if s.RewardBoxes == nil {
			// 启动期 ValidateBoxes 会拦下「有奖励表却没有礼包目录」这个组合，真到这里
			// 说明配置被动过。记一笔而不是静默按包装发：包装落地是错的，但拒绝整条怪死
			// 请求更糟。
			result.SkippedKinds = append(result.SkippedKinds, "reward_boxes_unavailable")
		} else {
			opened, next, unresolved := OpenRewardBoxes(result.NextSeed, s.RewardBoxes, result.Awards)
			result.Awards = opened
			result.NextSeed = next
			result.SkippedKinds = append(result.SkippedKinds, unresolved...)
		}
	}
	if d.NextEntity == 0 || uint64(d.NextEntity)+uint64(len(result.Awards)) >= 65535 || uint64(s.next)+uint64(len(result.Awards)) >= 65535 {
		return nil, fmt.Errorf("drop identity exhausted")
	}
	var rows []protocol.SceneDrop
	for _, a := range result.Awards {
		// Scene drops and monsters share the native object namespace. Consume
		// the run's allocator so a later room cannot reuse a drop identity.
		object := uint32(d.NextEntity)
		d.NextEntity++
		slot := uint16(s.next)
		s.next++
		drop := Drop{s.Run, d.Room.Map, s.Actor, object, slot, a}
		s.Objects[object] = drop
		// Gear carries its source durability in the scene row, exactly as it
		// does in a bag row; a stackable carries its amount instead.
		item := protocol.OrdinaryItem(slot, a.Template, a.Amount)
		if durability, ok := s.Equipment.Durability(a.Template); ok && !s.stackable(a.Template) {
			item = inventory.EquipmentRow(inventory.BagEquipment{Slot: slot, Template: a.Template, Durability: durability})
		}
		rows = append(rows, protocol.SceneDrop{Object: object, Item: item, Sentinel: 65535, Owner: s.Actor})
	}
	s.seeds[d.Room.Map] = result.NextSeed
	s.deaths[entity] = rows
	s.Skipped[entity] = result.SkippedKinds
	return append([]protocol.SceneDrop(nil), rows...), nil
}

// difficultyIndex maps a run's client difficulty onto the bonus table. The
// client numbers difficulties from 1 (1=Normal .. 5=Hero) while the reference
// bonus table is 0-based, so a run is one column lower than its label.
//
// 2026-09-23 实测：掉落一直按难度 0 计算（加成恒 1.0），副本自己声明的难度
// 从未生效 —— 奥德赛 56 个副本的 [designated difficulty] 全部是 2，客户端
// 选图上报的也是 2，对应的加成是 1.2。难度列比加成表长时夹到最后一列，
// 避免英雄难度把掉落推到 "drop source range is not imported"。
func difficultyIndex(r Rules, difficulty byte) byte {
	if difficulty == 0 {
		return 0
	}
	i := int(difficulty) - 1
	if i >= len(r.DifficultyBonus) {
		i = len(r.DifficultyBonus) - 1
	}
	if i < 0 {
		return 0
	}
	return byte(i)
}

// stackable reports whether this identity belongs to the ordinary stackable
// pool. The two source lists have independent identity spaces, so a stackable
// always wins and gear is only assumed for identities it does not claim.
func (s *Session) stackable(id uint32) bool {
	if s.Currency != nil && s.Currency.Items[id].Kind == "stackable" {
		return true
	}
	return id == 0 || s.Catalog.Items[id].Kind == "stackable"
}

func (s *Session) Owned(d *dungeon.Session, account, character int64, actor uint16, object uint32) (Drop, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.Objects[object]
	if !ok || d == nil || !d.Loaded || d.RunID != s.Run || v.Map != d.Room.Map || account != s.Account || character != s.Character || actor != s.Actor || actor != v.Owner {
		return Drop{}, fmt.Errorf("pickup object is not owned in current room")
	}
	return v, nil
}
