package loot

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// MoonRewardChoice 是翻牌里的一项**固定产出**（门票、硬币这类按张数给的堆叠物）。
//
// Count 就是要发放的数量（不再有权重）：月湖的翻牌不是"从一张大表里抽一件"，而是
// 固定给门票与银币、外带随机若干件装备（玩家 2026-10-02 的实测截图：银币 ×14、深渊
// 门票 ×60、另加 4 件装备）。
type MoonRewardChoice struct {
	Template uint32
	Count    uint32
	// Group 记录它来自源里哪个掉落组，只用于启动日志与排查。
	Group uint32 `json:"group,omitempty"`
}

// MoonEquipmentPolicy 描述翻牌里"随机装备"那一段：件数分布 + 掷骰用的等级与怪物档位。
//
// **品质不在这里指定**：每件装备独立走源的掉落/品质判定表（Tables.Rarity 与
// Tables.Probability，与深渊同一套），所以"稀有…太初都出得来、概率贴合深渊"是表本身
// 给的，不是本仓编的。
//
// Weights 是每个件数（Min..Max，长度必须相等）的权重。用加权而不是等概率，是因为
// 牌面上 6 个格子、实际产出集中在 2~3 件：等概率 1..4 会让"只出一件"太常见。
type MoonEquipmentPolicy struct {
	Min, Max uint32
	Weights  []uint32
	Level    byte
	Rank     byte
}

type MoonRewardPolicy struct {
	Source    string
	Draws     uint32
	Choices   []MoonRewardChoice
	Equipment MoonEquipmentPolicy
}
type MoonRewardGrant struct {
	Template, Count uint32
	Record          []byte
}
type MoonRewardPlan struct {
	Source, Run, Rules string
	Account, Character int64
	Grants             []MoonRewardGrant
}

const MoonRewardModel = "moon-solo-clear-v1"

var ErrMoonBagFull = errors.New("Moon reward pending: bag full")

func (s *Service) ValidateMoonRewards(p MoonRewardPolicy) error {
	hash, e := hex.DecodeString(p.Source)
	if e != nil || len(hash) != 32 || s == nil || p.Source != s.Catalog.Source.SaveIdentity() || p.Draws == 0 || p.Draws > 16 || len(p.Choices) == 0 || len(p.Choices) > 4096 {
		return fmt.Errorf("invalid Moon reward policy/source")
	}
	var total uint64
	for _, v := range p.Choices {
		if v.Template == 0 || v.Count == 0 {
			return fmt.Errorf("invalid Moon reward choice")
		}
		total += uint64(v.Count)
		if _, e := s.moonGrant(v); e != nil {
			return e
		}
	}
	if total == 0 {
		return fmt.Errorf("empty Moon reward")
	}
	if p.Equipment.Max > 0 {
		if p.Equipment.Min == 0 || p.Equipment.Max < p.Equipment.Min || p.Equipment.Max > 16 || p.Equipment.Level == 0 || p.Equipment.Rank > 3 {
			return fmt.Errorf("invalid Moon equipment policy")
		}
		if len(p.Equipment.Weights) != int(p.Equipment.Max-p.Equipment.Min+1) {
			return fmt.Errorf("Moon equipment weights must cover every count in range")
		}
		weightTotal := uint64(0)
		for _, w := range p.Equipment.Weights {
			weightTotal += uint64(w)
		}
		if weightTotal == 0 {
			return fmt.Errorf("empty Moon equipment weights")
		}
		if s.Equipment == nil || len(s.Equipment.DropPool()) == 0 {
			return fmt.Errorf("Moon equipment pool is unavailable")
		}
		// 用**实际会走的参数**掷一次，确认这套等级/档位在源的掉落表里有解：
		// 进本前就失败，比打完 Boss 才发现发不出装备好。
		if _, e := Roll(s.Catalog, s.Tables, s.Rules, s.Equipment.DropPool(), 1, p.Equipment.Level, p.Equipment.Rank, 0); e != nil {
			return fmt.Errorf("Moon equipment roll: %w", e)
		}
	}
	return nil
}

// moonStackDestination 返回该堆叠物品的堆叠上限，并要求它的类别有明确的背包槽位带。
//
// 这是"堆叠奖励能不能结算"的**唯一判据**：发奖（moonGrant）与翻牌池的直读推导
// （MoonSettleableStack）共用它。两处一旦各写一份，就会出现"启动/进本前校验通过、
// 领奖时才失败"（或反过来"池被莫名其妙清空"）这类只在实机才暴露的漂移。
//
// 为什么不用 inventory 的 [throw] 兜底（shop.Buy 对未知类别就是这么做的）：
// 翻牌发的是装备/材料这类**长期资产**，落到消耗品页是错的页面；兜底值本身是猜测，
// 而门禁要求"有明确背包类别的堆叠物品"。所以这里宁可拒绝。
func (s *Service) moonStackDestination(item catalog.LootItem) (uint32, error) {
	limit := item.StackLimit
	if limit == 0 {
		limit = s.BagRules.MissingStackLimit
	}
	if _, ok := s.BagRules.Slots[item.StackableType]; !ok {
		return 0, fmt.Errorf("Moon stack destination missing")
	}
	return limit, nil
}

// MoonSettleableStack 报告模板能否作为月湖翻牌的堆叠结算项。
//
// 返回的 Count 固定为 1（翻牌一行一件），Weight 留给调用方按**来源表**填 ——
// 池的成员与权重来自可直读的掉落组表，本判定只回答"这一件能不能结算"。
// 不可结算的成员（[etc] / [upgradable legacy] / 装备模板等）在这里被过滤掉，
// 调用方因此不需要看 LootItem 的细节。
func (s *Service) MoonSettleableStack(template uint32) (MoonRewardChoice, bool) {
	if s == nil || template == 0 {
		return MoonRewardChoice{}, false
	}
	item, ok := s.Catalog.Items[template]
	if !ok || item.Kind != "stackable" {
		return MoonRewardChoice{}, false
	}
	if _, e := s.moonStackDestination(item); e != nil {
		return MoonRewardChoice{}, false
	}
	return MoonRewardChoice{Template: template, Count: 1}, true
}

// moonEquipmentDestination 校验一件模板能否作为月湖翻牌的**普通装备**结算，并返回
// 它的耐久上限与最低装备等级。判据与发奖（moonGrant）完全同源，不另写一份。
//
//   - 必须有装备源（s.Equipment）；
//   - `[equipment type]` 要解析得出来，且不是另开容器的部位（inventory.EquipmentBagSpace
//     != 0 表示时装/宠物/护石那类）；
//   - 装备源必须给得出耐久（Equipment.Reward）。
func (s *Service) moonEquipmentDestination(template uint32) (uint16, uint32, error) {
	if s.Equipment == nil {
		return 0, 0, fmt.Errorf("Moon equipment needs source")
	}
	def, e := s.Equipment.Definition(template)
	if e != nil {
		return 0, 0, e
	}
	kind := def.Fields["[equipment type]"]
	if len(kind) == 0 || inventory.EquipmentBagSpace(kind[0].Text) != 0 {
		return 0, 0, fmt.Errorf("Moon reward requires ordinary equipment")
	}
	durability, e := s.Equipment.Reward(template)
	if e != nil {
		return 0, 0, e
	}
	var level uint32
	if lv := def.Fields["[minimum level]"]; len(lv) == 1 && lv[0].Type == 0 && lv[0].Value > 0 {
		level = uint32(lv[0].Value)
	}
	return durability, level, nil
}

// MoonSettleableEquipment 报告模板能否作为月湖翻牌的普通装备结算项，且最低装备等级
// 不低于 minLevel（调用方传副本自己声明的 `[minimum required level]`）。
//
// minLevel 是**排除低级池**用的：月湖第二层同时声明了 115 级装备组（21251，11 部位）
// 与 100 级大池（组 3，792 件），不过滤的话 100 级装备会因为件数多而淹没 115 级产出
// （玩家 2026-10-02 核实的口径：月湖出 115 级装备 + 深渊门票 + 硬币，不该混 100 级）。
func (s *Service) MoonSettleableEquipment(template, minLevel uint32) (MoonRewardChoice, bool) {
	if s == nil || template == 0 {
		return MoonRewardChoice{}, false
	}
	_, level, e := s.moonEquipmentDestination(template)
	if e != nil || level < minLevel {
		return MoonRewardChoice{}, false
	}
	return MoonRewardChoice{Template: template, Count: 1}, true
}

func (s *Service) moonGrant(v MoonRewardChoice) (MoonRewardGrant, error) {
	g := MoonRewardGrant{Template: v.Template, Count: v.Count}
	if item, ok := s.Catalog.Items[v.Template]; ok && item.Kind == "stackable" {
		limit, e := s.moonStackDestination(item)
		if e != nil {
			return g, e
		}
		if v.Count > limit {
			return g, fmt.Errorf("Moon reward exceeds stack limit")
		}
		return g, nil
	}
	if v.Count != 1 {
		return g, fmt.Errorf("Moon equipment needs source and count1")
	}
	durability, _, e := s.moonEquipmentDestination(v.Template)
	if e != nil {
		return g, e
	}
	row := protocol.OrdinaryItem(0, v.Template, 0)
	binary.LittleEndian.PutUint16(row[11:], durability)
	g.Record = append([]byte(nil), row[:]...)
	return g, nil
}
func (s *Service) PlanMoonReward(role Role, run *dungeon.Session, p MoonRewardPolicy) (MoonRewardPlan, error) {
	var out MoonRewardPlan
	if s == nil || run == nil || !MoonRunID(run.RunID) || run.Definition.ID != 100004137 || !run.Completed() || role.ConfigVersion != p.Source {
		return out, fmt.Errorf("Moon reward before owned final")
	}
	if e := s.ValidateMoonRewards(p); e != nil {
		return out, e
	}
	raw, _ := json.Marshal(p)
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	out = MoonRewardPlan{Source: p.Source, Run: run.RunID, Rules: digest, Account: role.AccountID, Character: role.ID}
	for _, v := range p.Choices {
		g, e := s.moonGrant(v)
		if e != nil {
			return out, e
		}
		out.Grants = append(out.Grants, g)
	}
	equipment, e := s.rollMoonEquipment(digest, run, role, p.Equipment)
	if e != nil {
		return out, e
	}
	out.Grants = append(out.Grants, equipment...)
	return out, nil
}

// moonEquipmentCount 按 Weights 抽这次的装备件数。
//
// 抽成纯函数（只吃 policy 与一段种子）是为了能把"期望落在 2~3 件"这条体验口径钉进测试：
// 等概率 1..4 的期望是 2.5 但"1 件"占 25%，实测体感就是"经常只出一件"；加权后
// {1,3,3,2} 的期望是 2.67 且"1 件"只占 1/9。
func moonEquipmentCount(p MoonEquipmentPolicy, seed []byte) (uint32, error) {
	if p.Min == 0 || p.Max < p.Min {
		return 0, fmt.Errorf("invalid Moon equipment count range")
	}
	if len(p.Weights) != int(p.Max-p.Min+1) {
		return 0, fmt.Errorf("Moon equipment weights must cover every count in range")
	}
	total := uint64(0)
	for _, weight := range p.Weights {
		total += uint64(weight)
	}
	if total == 0 {
		return 0, fmt.Errorf("empty Moon equipment weights")
	}
	// 用种子派生一个稳定的 64 位值；不足 8 字节时按小端右补零（测试里会传短种子）。
	var raw [8]byte
	copy(raw[:], seed)
	roll := binary.LittleEndian.Uint64(raw[:]) % total
	for i, weight := range p.Weights {
		if roll < uint64(weight) {
			return p.Min + uint32(i), nil
		}
		roll -= uint64(weight)
	}
	return p.Min, nil
}

// rollMoonEquipment 产出翻牌里那 1..4 件装备。
//
// 每一件都**独立**走源的掉落/品质判定表（Roll → Tables.Probability 定出不出、
// Tables.Rarity 定什么品质），所以品质分布就是游戏自己的那套率（与深渊同源），
// 本仓不另编概率。件数由 RunID/账号/角色派生出的稳定种子决定：同一局重放结果一致，
// 不同局之间随机。
//
// 表里那一掷可能什么都不给（这是率本身的一部分），所以这里按"拿到够 count 件"重试，
// 最多 8×count 次；次数用完仍不足就按实际件数发 —— 宁可少发也不编一件出来。
func (s *Service) rollMoonEquipment(digest string, run *dungeon.Session, role Role, p MoonEquipmentPolicy) ([]MoonRewardGrant, error) {
	if s.Equipment == nil {
		return nil, fmt.Errorf("Moon equipment roll needs the source equipment pool")
	}
	if p.Min == 0 || p.Max < p.Min || p.Max > 16 || p.Level == 0 || p.Rank > 3 {
		return nil, fmt.Errorf("invalid Moon equipment policy")
	}
	pool := s.Equipment.DropPool()
	if len(pool) == 0 {
		return nil, fmt.Errorf("Moon equipment pool is empty")
	}
	base := fmt.Sprintf("%s/%s/%d/%d/equipment", digest, run.RunID, role.AccountID, role.ID)
	// 件数按 Weights 加权抽（不是等概率）：实机体感"只出一件"太常见，而牌面有 6 个格子、
	// 官方产出集中在 2~3 件。
	drawn := sha256.Sum256([]byte(base))
	count, e := moonEquipmentCount(p, drawn[:])
	if e != nil {
		return nil, e
	}
	out := make([]MoonRewardGrant, 0, count)
	// 表里那一掷可能什么都不给（这是率本身的一部分），所以按"拿到够 count 件"重试。
	// 上限给得宽一些：这张表出装备的率本来就不高，太紧会因为掷空而少发。
	for i := uint32(0); len(out) < int(count) && i < count*32; i++ {
		seed := sha256.Sum256([]byte(fmt.Sprintf("%s/%d", base, i)))
		outcome, e := Roll(s.Catalog, s.Tables, s.Rules, pool, binary.LittleEndian.Uint32(seed[:]), p.Level, p.Rank, 0)
		if e != nil {
			return nil, e
		}
		for _, a := range outcome.Awards {
			if a.Template == 0 || a.Amount == 0 {
				continue
			}
			// 只要装备：掉表里还可能有金币/材料，它们不是这一段要发的东西。
			if _, _, e := s.moonEquipmentDestination(a.Template); e != nil {
				continue
			}
			g, e := s.moonGrant(MoonRewardChoice{Template: a.Template, Count: 1})
			if e != nil {
				continue
			}
			out = append(out, g)
			break
		}
	}
	return out, nil
}
func (s *Service) DecodeMoonReward(role Role, run string, raw json.RawMessage) (MoonRewardPlan, error) {
	var p MoonRewardPlan

	if e := json.Unmarshal(raw, &p); e != nil {
		return p, e
	}
	if p.Run != run || p.Source != role.ConfigVersion || p.Source != s.Catalog.Source.SaveIdentity() || p.Account != role.AccountID || p.Character != role.ID || len(p.Grants) == 0 || len(p.Grants) > 16 {
		return p, fmt.Errorf("foreign/corrupt Moon reward proof")
	}
	return p, nil
}
func MoonRunID(run string) bool { v, e := hex.DecodeString(run); return e == nil && len(v) == 16 }
func (s *Service) applyMoonRewards(state json.RawMessage, p MoonRewardPlan) (json.RawMessage, error) {
	bag, e := inventory.ReadBag(state)
	if e != nil {
		return nil, e
	}
	for _, g := range p.Grants {
		if len(g.Record) == 0 {
			bag, _, e = bag.Add(s.Catalog, s.BagRules, g.Template, g.Count)
		} else {
			if len(g.Record) != protocol.CurrentItemRecordSize || g.Count != 1 || binary.LittleEndian.Uint32(g.Record[2:]) != g.Template {
				return nil, fmt.Errorf("invalid frozen Moon equipment")
			}
			var slots []uint16
			bag, slots, e = bag.AddEquipment(s.Equipment, s.BagRules.EquipmentSlots, g.Template, 1)
			if e == nil {
				for i := range bag.Equipment {
					if bag.Equipment[i].Slot == slots[0] {
						bag.Equipment[i].Record = append([]byte(nil), g.Record...)
						bag.Equipment[i].Durability = binary.LittleEndian.Uint16(g.Record[11:])
					}
				}
			}
		}
		if e != nil {
			// Upstream inventory exposes these two capacity errors as strings.
			// Do not convert source/DB/corruption failures into retryable full-bag.
			if e.Error() == "equipment bag is full" || e.Error() == "bag category is full" {
				return nil, ErrMoonBagFull
			}
			return nil, e
		}
	}
	return inventory.SaveBag(state, bag)
}
func (p MoonRewardPlan) WireRows() []protocol.ConquestRewardValue115 {
	out := make([]protocol.ConquestRewardValue115, 0, len(p.Grants))
	for _, g := range p.Grants {
		r := protocol.ConquestRewardValue115{Template: g.Template, Value: g.Count}
		if len(g.Record) == protocol.CurrentItemRecordSize {
			r.Equipment = true
			r.Value = binary.LittleEndian.Uint32(g.Record[6:])
		}
		out = append(out, r)
	}
	return out
}

// ApplyMoonRewards prepares a reward state without persistence.
func (s *Service) ApplyMoonRewards(state json.RawMessage, p MoonRewardPlan) (json.RawMessage, error) {
	return s.applyMoonRewards(state, p)
}
