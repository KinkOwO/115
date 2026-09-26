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
	Currency              *OdysseyCurrency
	ChapterDrop           *OdysseyChapterDrop
	Attunement            *AttunementRewards
	QuestDropBonusPercent int
	// attunementRolled 保证一轮只抽一次专属奖励：同一只源领主再被确认死亡
	// （或同模板的第二只 rank3）都不会重复发奖。
	attunementRolled   bool
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
	result, e := RollWithBonus(s.Catalog, s.Tables, s.Rules, s.Equipment.DropPool(), seed, monster.Level, monster.Rank, difficultyIndex(s.Rules, d.Difficulty), s.QuestDropBonusPercent)
	if e != nil {
		return nil, e
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
	// [hunt boss] 领主（DungeonDefinition.AttunementBoss）—— 这与通关判定锚在
	// 同一个事实上，而不是再叠一道「副本是否已通关」的闸门：该闸门多余，且一旦
	// 时序不同就会静默扣下奖励，正是本次要消灭的失败形态。
	// 没有奖励表的副本在这里连种子都不消耗。
	if s.Attunement.Enabled() && d.Definition.AttunementBoss != 0 &&
		monster.Rank == 3 && monster.Template == d.Definition.AttunementBoss && !s.attunementRolled {
		awards, next, err := s.Attunement.Roll(result.NextSeed, d.Definition.ID, uint32(d.Maze.Index))
		if err != nil {
			return nil, err
		}
		s.attunementRolled = true
		result.Awards = append(result.Awards, awards...)
		result.NextSeed = next
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
