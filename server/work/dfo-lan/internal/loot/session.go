package loot

import (
	"crypto/rand"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"errors"
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
	// 1至3对应已冻结的黑鸦装备分支；0沿用普通地面拾取回执。
	BlackPurgatoryIndex byte
}
type Session struct {
	BlackPurgatory       *BlackPurgatoryRewards
	BlackPurgatoryPlan   *CardPlan
	blackPurgatoryRolled bool
	Currency             *OdysseyCurrency
	ChapterDrop          *OdysseyChapterDrop
	Attunement           *AttunementRewards
	RewardBoxes          RewardBoxSource
	// Omen 是征兆系统的按角色累积账（见 omen.go）。为 nil 时这条线完全不推进。
	Omen                  *OmenLedger
	QuestDropBonusPercent int
	// OathTier 是本场天平下发的 oath 档位（40..45；0 = 未知/非深渊）。
	//
	// 天平档位与征兆是**两条平行的线**（业主 2026-10-01 定调）：这里按档位发对应的
	// 「星蕴石自选套装罐子」，而 omen.go 的结算各发各的 —— 同一场里两条都触发就各自
	// 发自己那份，互不覆盖、也不互相抑制。
	OathTier uint16
	// attunementRolled 保证一轮只抽一次专属奖励：同一只源领主再被确认死亡
	// （或同模板的第二只 rank3）都不会重复发奖。
	attunementRolled bool
	// omenRolled 与 attunementRolled 同理：同一场只推进一次征兆。
	omenRolled bool
	// oathTierRolled 与上面两个同理：同一场只按天平档位发一次罐子。
	oathTierRolled     bool
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
	blackBoss := s.BlackPurgatory != nil && s.BlackPurgatoryPlan != nil && BlackPurgatoryBossDeath(d, entity)
	if monster.NonCombat || monster.APC || monster.Level == 0 || d.Unowned[entity] && !blackBoss {
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
	if (!excludeGold || !excludeRandom) && !d.Unowned[entity] {
		var e error
		result, e = RollWithBonus(s.Catalog, s.Tables, s.Rules, s.Equipment.DropPool(), seed, monster.Level, monster.Rank, difficultyIndex(s.Rules, d.Difficulty), s.QuestDropBonusPercent)
		if errors.Is(e, ErrOutOfDropRange) && blackBoss {
			// 通用旧掉落表的等级上限不能阻断已冻结的黑鸦专属奖励。
			result = Outcome{NextSeed: seed, SkippedKinds: []string{"黑鸦通用掉落超出导入等级"}}
		} else if e != nil {
			return nil, e
		}
		// [MERGE-20260928-DUNGEON-GROUP-DROP] 副本自带掉落组时的分支。
		//
		// 两条组来源，**优先用带率的那条**：
		//
		//  1. etc/dungeondropinfo.cos —— 全局索引，带品级与率，但只有 281 个副本有。
		//     RollDungeonGroups 自己 roll 率，命中才发组里的物品。
		//  2. 副本脚本 [normal group index] —— 副本自选，3200 个副本都有，但不带率。
		//     率和品级已经由上面 RollWithBonus 的全局表定完了，这条只负责把「出什么
		//     物品」换成副本自选的池子。
		//
		// 实机刷的「深渊：最终调律者」100005014 落在第 2 条：它不在 dungeondropinfo 里，
		// 但带 `[normal group index] 1 21251 1 21476`（21251 是深渊 epic 组）。这正是此前
		// 服务端拿不到它的原因 —— 全局表给普通怪的装备率只有 0.37%（135×0.2×1.4/10000），
		// 11 只杂兵期望出 0.04 件，看起来就是「装备不爆」。
		//
		// 两条都命中时只走第 1 条：dungeondropinfo 带率，语义更完整。
		//
		// ⚠️ 与外部包的一处**有意收窄**：这两条只在「该副本不排除掉落、且这只怪属于本次
		// 战斗」时启用（即仍在本 `if` 内）。排除标记是副本作者写下的意图，宁可不发。
		groupTaken := false
		if entries, has := s.Catalog.DropInfoByID(d.Definition.ID); has && len(entries) > 0 {
			grpOut, _, gerr := RollDungeonGroups(s.Catalog, s.Rules, result.NextSeed, DungeonGroupDropRequest{
				DungeonID:   d.Definition.ID,
				MonsterKind: monsterKindName(monster.Rank),
				Difficulty:  int(difficultyIndex(s.Rules, d.Difficulty)),
				Rarity:      -1,
			})
			if gerr != nil {
				return nil, gerr
			}
			groupTaken = true
			result.Awards = replaceItemAwards(result.Awards, grpOut.Awards)
			result.SkippedKinds = append(result.SkippedKinds, grpOut.SkippedKinds...)
			result.NextSeed = grpOut.NextSeed
		} else if ids, ok, gerr := DungeonGroupIndices(d.Definition, int(difficultyIndex(s.Rules, d.Difficulty))); gerr != nil {
			return nil, gerr
		} else if ok && len(ids) > 0 {
			// 件数由全局表先定：「掉不掉、掉几件」是怪物侧的判断，组只决定「掉什么」。
			// 没有这层约束时每只小怪都会把每个声明组各抽一件 —— 100005014 声明 2 组，
			// 实机就变成每只小怪必掉 2 件、13 只小怪 26 件/把（「普通小怪爆了一地」）。
			budget := 0
			for _, a := range result.Awards {
				if a.Template != 0 {
					budget += int(a.Amount)
				}
			}
			grpOut, _, gerr := RollDeclaredGroups(s.Catalog, ids, budget, result.NextSeed)
			if gerr != nil {
				return nil, gerr
			}
			groupTaken = true
			result.Awards = replaceItemAwards(result.Awards, grpOut.Awards)
			result.SkippedKinds = append(result.SkippedKinds, grpOut.SkippedKinds...)
			result.NextSeed = grpOut.NextSeed
		}
		// 没有声明组的副本保持全局表结果不动，groupTaken 只用于排查。
		_ = groupTaken
	}
	result.Awards = filterDungeonAwards(d.Definition, result.Awards)
	if d.Definition.Odyssey && s.Currency != nil {
		// ⚠️ 这里必须比**内层真哈希**（`Checksum`），不能比契约身份（`SaveIdentity()`）：
		// `s.Currency.Source` 是 `ImportOdysseyCurrency` 里按 `a.Snapshot().Checksum` 赋的值，
		// 两个运行期目录对象之间比的是"这一份源"，与存档身份无关。
		// 2026-10-01 上游 4171f55「身份口径统一」把这一行批量改成了 SaveIdentity() ⇒
		// 恒不相等 ⇒ **奥德赛副本每只小怪的死亡结算（Death）直接返回错误**：
		// 掉落/经验/货币全丢、客户端拿不到结算 ⇒ 清完怪不开门（实机 23:26 每秒刷
		// `dungeon_request_refused: Odyssey currency source mismatch`）。
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
		// 天平档位（业主 2026-10-01 定调）：与征兆**平行**的一条线 —— 变色就发对应的
		// 「星蕴石自选套装罐子」，与上面那条各发各的，同一场都触发就拿两份。
		// 罐子同样交给下面统一的 OpenRewardBoxes 展开（源写着「以开封状态发放」），
		// 所以玩家拿到的是里面的装备而不是盒子。
		if coffer := oathTierCoffer(s.OathTier); coffer != 0 && !s.oathTierRolled {
			awards = append(awards, Award{Template: coffer, Amount: 1})
			s.oathTierRolled = true
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
	bossStart := len(result.Awards)
	var bossIndices []byte
	if blackBoss && !s.blackPurgatoryRolled {
		p := s.BlackPurgatoryPlan
		if p.Run != d.RunID || p.Source != s.Catalog.Source.SaveIdentity() || p.Model != blackPurgatoryCardModel || p.BossModel != blackPurgatoryBossModel {
			return nil, fmt.Errorf("黑鸦地面奖励缺少匹配的冻结奖单")
		}
		for i, award := range p.BossItems {
			if award == (Award{}) {
				continue
			}
			if _, ok := s.BlackPurgatory.bossDurability[award.Template]; !ok || award.Amount != 1 {
				return nil, fmt.Errorf("黑鸦地面奖励不在已校验的装备组内")
			}
			result.Awards = append(result.Awards, award)
			bossIndices = append(bossIndices, byte(i+1))
		}
	}
	if d.NextEntity == 0 || uint64(d.NextEntity)+uint64(len(result.Awards)) >= 65535 || uint64(s.next)+uint64(len(result.Awards)) >= 65535 {
		return nil, fmt.Errorf("drop identity exhausted")
	}
	var rows []protocol.SceneDrop
	for awardIndex, a := range result.Awards {
		// Scene drops and monsters share the native object namespace. Consume
		// the run's allocator so a later room cannot reuse a drop identity.
		object := uint32(d.NextEntity)
		d.NextEntity++
		slot := uint16(s.next)
		s.next++
		drop := Drop{Run: s.Run, Map: d.Room.Map, Owner: s.Actor, Object: object, Slot: slot, Award: a}
		if awardIndex >= bossStart {
			drop.BlackPurgatoryIndex = bossIndices[awardIndex-bossStart]
		}
		s.Objects[object] = drop
		// Gear carries its source durability in the scene row, exactly as it
		// does in a bag row; a stackable carries its amount instead.
		item := protocol.OrdinaryItem(slot, a.Template, a.Amount)
		if durability, ok := s.Equipment.Durability(a.Template); ok && !s.stackable(a.Template) {
			item = inventory.EquipmentRow(inventory.BagEquipment{Slot: slot, Template: a.Template, Durability: durability})
		}
		if drop.BlackPurgatoryIndex != 0 {
			item = inventory.EquipmentRow(inventory.BagEquipment{Slot: slot, Template: a.Template, Durability: s.BlackPurgatory.bossDurability[a.Template]})
		}
		rows = append(rows, protocol.SceneDrop{Object: object, Item: item, Sentinel: 65535, Owner: s.Actor})
	}
	s.seeds[d.Room.Map] = result.NextSeed
	s.deaths[entity] = rows
	s.blackPurgatoryRolled = s.blackPurgatoryRolled || blackBoss
	s.Skipped[entity] = result.SkippedKinds
	return append([]protocol.SceneDrop(nil), rows...), nil
}

// monsterKindName 把怪物的 rank 映射到 dungeondropinfo [rate list] 的类别键。
//
// 映射依据（只读观察，不做发明）：
//   - [rate list] 里实际出现的键只有 `boss` / `named` / `normal`
//   - 服务端 DungeonMonster.Rank 的取值 0..3 对应 normal0 / champion1 / super2 / boss3
//     （见 protocol.DungeonMonster 的注释）
//   - 所以 rank3 唯一对应 boss，rank0 对应 normal，rank1/2（champion/super）落在
//     named 上 —— 这是三档对四档的唯一无损映射。
//
// 未在 [rate list] 里出现的键一律由 RollDungeonGroups 记进 SkippedKinds，不发物品。
func monsterKindName(rank byte) string {
	switch rank {
	case 3:
		return "boss"
	case 0:
		return "normal"
	default:
		return "named"
	}
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

// oathTierCoffer 把天平档位（40..45）映射成「星蕴石自选套装罐子」。
//
// 四档对四个罐子，内容是**实测展开**的（见 docs/protocol/endkeeper-of-order-primer-20260926.md
// §38.2，与业主提供的官方奖励表逐位吻合）：
//
//	unique(42)    → 10416150 → 12 × rarity 3（神器）
//	legendary(43) → 10417545 → 12 × rarity 6（传说）
//	epic(44)      → 10417552 → 12 × rarity 4（史诗）
//	primeval(45)  → 10417571 → 12 × rarity 8（太初）
//
// normal(40) / rare(41) **不发**：官方奖励表里没有 rarity 2 的罐子（行 0 的条目数就是 0），
// 而国服 1710 场实测里 32.05% 正是「不变色、不出东西」。
//
// 这条线与征兆（omen.go 的 [coupon drop table] 结算）**平行**：各发各的，同一场都触发
// 就各自兑现一份（业主 2026-10-01 定调）。
func oathTierCoffer(tier uint16) uint32 {
	switch {
	case tier >= 45:
		return 10417571
	case tier >= 44:
		return 10417552
	case tier >= 43:
		return 10417545
	case tier >= 42:
		return 10416150
	default:
		return 0
	}
}
