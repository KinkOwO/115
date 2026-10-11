package loot

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
)

// 本文件把沉月湖的翻牌产出**整条换成源声明驱动**（2026-10-09）。
//
// 与 moon_rewards.go 里那套（`moonFixedRewards` 代码常量表 + `MoonEquipmentPolicy` 的 1..4 件
// 与权重）的分工：那套是**本地口径**；这里全部读 `.dgn` 自己声明的东西，一个数都不自己编：
//
//	catalog.DungeonRewardBlock
//	  ├─ Contents / Item   「产物列表」页与其内容（客户端那两个图标）
//	  ├─ Groups            [normal group index]：本难度声明的掉落组（个数+组号）
//	  ├─ Multiple          [reward multiple info]：类别 + 各难度倍数（装备 7/6、誓约·星蕴石 3/2）
//	  └─ Special           [special setinfo reward] / [special custom reward info]：
//	                       名 + 率 + 归属列表 + 数量 + 组号
//
// 产出模型（三段，逐段可回指源标签）：
//
//	1. 固定产物   ← 固定组（[normal group index] 里非装备那些组）的成员
//	2. 基础产物   ← 逐条 Special：按 Rate/1000 掷一次，命中从它的组里等权抽 1 件
//	3. 效率产出   ← 逐条 Multiple：类别 2（装备）掷 Normal 次装备池（源组 ∩ 可入包装备，
//	                走源的品质/概率表 ⇒ 品质分布就是游戏自己那套率）；类别 3（誓约·星蕴石）
//	                掷 Normal 次**誓约/结晶装备池**（源掉落表里整组都是 `[oath]`/`[primer]`
//	                装备的那些组，按源权重）—— 源里 [special setinfo reward] 指向的那两组装的是
//	                打不开的显示件，见 MoonSourcePolicy.OathPool 的取证
//
// ⚠️ 两条**明确的假设**（只写在这里，不散落到调用方）：
//
//   - **率的量纲按「千分比」**（`Rate/1000`，>1000 视为必中）。依据是跨副本交叉：
//     单块合计 640 / 960 / 1280 / 1418 / 1852 都不落在 1000 或 10000 这类整数基上 ⇒
//     不是"互斥概率各分一份"，而是独立率；三个副本里同一个 `MonsterCard_ExpectationReward_0`
//     恒为 320 ⇒ 320 是固定设计值。见 analysis/tasks/next190 §D3。
//   - **族别判定 = 名字 + 组内容的图标**（见 MoonOathDeclaration）：名字含 Oath / Crystal
//     归誓约·星蕴石族；含 Equipment / Weapon 归装备族；其余（Card / Mine_* / …）不参与倍数。
//     依据同样是跨副本：100004306 只有装备族条目、`[reward multiple info]` 就只有类别 2 一条；
//     100004520 有誓约族、才有类别 3。
//
// ⚠️ **发不出去的条目要记下来，而不是丢掉**：背包槽位带（`configs/inventory.current37.json`）
// 只登记了 `[throw] / [material] / [quest] / [material expert job] / [etc]`，源里还有
// `[upgradable legacy]`（通宝袖珍罐这类）没有槽位带 ⇒ 那类一律拒发，并把原因写进
// Skipped，由上层打日志 —— 宁缺勿编，也不静默。
type MoonSourcePolicy struct {
	// Source 是存档身份（与其它策略一致的门禁）。
	Source string
	// Block 是选定的难度块（D6：单人走 [contents] = normal 的那块）。
	Block catalog.DungeonRewardBlock
	// Fixed 是固定产物的成员（模板 + 数量），由调用方从固定组的成员推导。
	Fixed []MoonRewardChoice
	// EquipmentPool 是装备掷骰的候选池：源装备组成员 ∩ 目录里"可入包"的装备。
	EquipmentPool []inventory.EquipmentDrop
	// OathPool 是誓约·星蕴石掷骰的候选 —— **誓约/结晶装备**，不是源里那两个「显示盒」。
	//
	// 取证（2026-10-09 业主实机 + PVF 直读）：
	//
	//   - `[special setinfo reward]` 指向的组 21468 / 21470 装的是 10419326..10419337
	//     「X Oath/Crystal Set」与 10419544「Dim Oath Crystal」—— 都是 `[etc]`、
	//     **没有产出段**（玩家打不开）的显示件；
	//   - 真正的誓约/结晶是**装备**：`equipment/character/common/{oath,primer}/*.equ`，
	//     `[equipment type]` = `[oath]` / `[primer]`，`[rarity]` 取 2/3/4/6/8
	//     （rare/unique/legendary/epic/primeval，见 inventory/oath_grade.go），
	//     实测 189 件、全 115 级；
	//   - 掉落表里另有一族「整组成员都是誓约/结晶装备」的组（21456..21460、21464..21467、
	//     21481..21483），实测 100 件、全 115 级、权重全 20 ⇒ 等权。
	//
	// 装配见 MoonOathEquipmentPool；判族见 MoonOathDeclaration。
	OathPool []MoonOathPoolMember
	// RarityPool 是**品级 → 模板**的掷骰池（源里每个品级一个组，见装配层的说明）。
	// 非空时装备掷骰走"按源 rarity 表掷品级 → 从该品级抽一件"，这是本副本的正确口径：
	// `Roll()` 那种"按 [grade] 窗口挑候选"在源组的 grade（117~121）与副本等级（115）跨度较大时
	// 会挑不到候选而退到最近品级（实测退成稀有）。
	RarityPool map[int32][]uint32
	// SettlementDungeon 是**结算层**的副本号：通关结算落在哪一层就传哪一层
	// （沉月湖第二层 100004137 / 蔚蓝号 100004134）。它只是门禁，不含本副本专属逻辑。
	SettlementDungeon uint32
	// KeepAsIs 是**真·客户端可开的罐子**的模板集合：booster 目录里声明了
	// `[oath item booster]` / `[lottery ani info]` 的那些。与深渊**同一条判据**
	// （cmd/wireprobe/booster_flow.go 的 `boosterBoxSource.serverUnwrap`）：命中的
	// **原样落地**让玩家自己在消耗品里点开，其余一律展开。装配层从 booster 目录算出来。
	KeepAsIs map[uint32]bool
	// SetEquipmentPool 是 `SetEquipmentReward`（12 个套装盒）的展开池，见 MoonEquipmentSetPool。
	SetEquipmentPool []MoonOathPoolMember
	// Level / Rank 是掷骰用的等级与怪物档位（源副本的等级 + boss 档）。
	Level, Rank byte
	// MaxRows 是**客户端结算面板能吃下的行数上限**（0 = 不限）。
	//
	// 依据：`protocol.ConquestClearReward115` 的 `[8]` 是**座位数**（多人组队每人一组），
	// 每个座位的产物是**变长列表**、服务端上界 `len(...) > 126` 才拒绝；每件产物在客户端
	// 各自一格 ⇒ 这里的上限是 126 件（2026-10-09 纠正，见 next190 §O）。超出的部分必须**记日志**，
	// 不能静默丢（见规划器里的 Skipped 记录）。
	MaxRows int
}

// MoonOathPoolMember 是誓约·星蕴石掷骰池里的一个成员。
//
// Family / Rarity 只用于日志与自检（源的组本身就按稀有度分开，见
// MoonOathEquipmentPool）；掷骰本身按 Weight（源里那一列，实测这批组全是 20）。
type MoonOathPoolMember struct {
	Template uint32
	Weight   uint32
	Family   string // "oath"（誓约）或 "primer"（星蕴石/结晶）
	Rarity   int32
}

// MoonSourceSkip 记录一条**源声明了但本仓发不出去**的产出。
type MoonSourceSkip struct {
	What   string
	Reason string
	Detail string
}

// MoonSourcePlan 是源驱动计划的产物：既有的 MoonRewardPlan（领取事务 / 入库 / WireRows 全复用）
// 外加一份"源声明了什么、发了什么、跳过了什么"的账。
type MoonSourcePlan struct {
	Plan    MoonRewardPlan
	Rolls   int
	Grants  int
	Skipped []MoonSourceSkip
	Summary string
}

// moonRatePermille 是率的量纲（千分比）。见文件头注释里的跨副本依据。
const moonRatePermille = 1000

// moonFamilyOath / moonFamilyEquipment 按条目名判族（小写子串匹配）。
//
// ⚠️ **名字判族有一个已知例外**：`RareEquipmentReward` 的名字里只有 Equipment，
// 但它的组（实测 21470）装的是 `10419544`「Dim Oath Crystal」，图标落在
// `Item/new_equipment/19_OathPrimer/Primer/Primer_00_Basic.img`（与誓约装备同族）。
// 所以「这条声明该不该按誓约族改发装备」**不能只看名字** —— 见 MoonOathDeclaration，
// 它把名字与**组内容的图标**两条判据合起来。
//
func moonFamilyOath(name string) bool {
	l := strings.ToLower(name)
	return strings.Contains(l, "oath") || strings.Contains(l, "crystal")
}

func moonFamilyEquipment(name string) bool {
	l := strings.ToLower(name)
	return strings.Contains(l, "equipment") || strings.Contains(l, "weapon")
}

// moonMultiple 取某类别的倍数。ok=false 表示源里没声明该类别。
func moonMultiple(block catalog.DungeonRewardBlock, kind uint32) (uint32, bool) {
	for _, m := range block.Multiple {
		if m.Kind == kind {
			return m.Normal, true
		}
	}
	return 0, false
}

// moonPickMember 从一组模板里等权抽一个（种子决定；同种子同结果）。
func moonPickMember(templates []uint32, seed []byte) (uint32, error) {
	if len(templates) == 0 {
		return 0, fmt.Errorf("empty Moon source group")
	}
	var raw [8]byte
	copy(raw[:], seed)
	return templates[binary.LittleEndian.Uint64(raw[:])%uint64(len(templates))], nil
}

// moonSourceSink 把"发一件 / 跳过一件"集中到一处，避免计划器与掷骰函数各写一份。
type moonSourceSink struct {
	svc           *Service
	plan          *MoonRewardPlan
	grants        int
	skips         []MoonSourceSkip
	maxRows       int
	rowCapLogged  bool
}

func (g *moonSourceSink) grant(what string, c MoonRewardChoice) bool {
	if g.maxRows > 0 && len(g.plan.Grants) >= g.maxRows {
		if !g.rowCapLogged {
			g.rowCapLogged = true
			g.skips = append(g.skips, MoonSourceSkip{What: what, Reason: "超出结算帧上限",
				Detail: fmt.Sprintf("上限=%d 件（ConquestClearReward115 的每座列表上限），本件及其后同类产出不再入奖单", g.maxRows)})
		}
		return false
	}
	grant, e := g.svc.moonGrant(c)
	if e != nil {
		g.skips = append(g.skips, MoonSourceSkip{What: what, Reason: "不可结算",
			Detail: fmt.Sprintf("%d: %v", c.Template, e)})
		return false
	}
	g.plan.Grants = append(g.plan.Grants, grant)
	g.grants++
	return true
}

func (g *moonSourceSink) skip(what, reason, detail string) {
	g.skips = append(g.skips, MoonSourceSkip{What: what, Reason: reason, Detail: detail})
}

// PlanMoonRewardsFromSource 按源声明产出这一局的翻牌奖单。
//
// 种子前缀一律含 `source身份/run/账号/角色`：同一局重放结果一致、不同局之间随机 ——
// 与 moon_rewards.go 的既有做法保持一致（可回归、不可预测）。
func (s *Service) PlanMoonRewardsFromSource(role Role, run *dungeon.Session, p MoonSourcePolicy) (MoonSourcePlan, error) {
	var out MoonSourcePlan
	if s == nil || run == nil || !MoonRunID(run.RunID) || p.SettlementDungeon == 0 ||
		run.Definition.ID != p.SettlementDungeon || !run.Completed() || role.ConfigVersion != p.Source {
		return out, fmt.Errorf("Moon source reward before owned final")
	}
	if p.Source != s.Catalog.Source.SaveIdentity() {
		return out, fmt.Errorf("Moon source reward source mismatch")
	}
	base := fmt.Sprintf("%s/%s/%d/%d/source", p.Source, run.RunID, role.AccountID, role.ID)
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(base)))
	out.Plan = MoonRewardPlan{Source: p.Source, Run: run.RunID, Rules: digest, Account: role.AccountID, Character: role.ID}
	sink := &moonSourceSink{svc: s, plan: &out.Plan, maxRows: p.MaxRows}

	// ---- 1. 固定产物（源固定组的成员；数量按调用方给的口径）----
	for _, v := range p.Fixed {
		sink.grant("固定产物", v)
	}

	equipMult, hasEquip := moonMultiple(p.Block, 2)
	oathMult, hasOath := moonMultiple(p.Block, 3)

	// ---- 2. 基础产物（逐条 Special：率 千分比，命中从它的组等权抽 1 件）----
	for _, sr := range p.Block.Special {
		name := sr.Name
		if sr.Rate == 0 || sr.Group == 0 || sr.Count == 0 {
			sink.skip(name, "源字段不全", fmt.Sprintf("rate=%d group=%d count=%d", sr.Rate, sr.Group, sr.Count))
			continue
		}
		rate := sr.Rate
		if rate > moonRatePermille {
			rate = moonRatePermille
		}
		seed := sha256.Sum256([]byte(fmt.Sprintf("%s/special/%d", base, sr.Group)))
		if binary.LittleEndian.Uint64(seed[:])%uint64(moonRatePermille) >= uint64(rate) {
			continue // 这一条没中
		}
		// 命中后先抽一个成员，再按「是不是真·客户端可开的罐子」决定原样发还是展开
		//（口径与深渊一致，见 moonDeclarationExpansion 的说明）。
		group, ok := s.Catalog.DropGroupByID(sr.Group)
		if !ok {
			sink.skip(name, "源的组读不到", fmt.Sprintf("group=%d", sr.Group))
			continue
		}
		rows := append(append([]catalog.DropWeight{}, group.Explicit...), group.Smart...)
		members := make([]uint32, 0, len(rows))
		for _, row := range rows {
			if row.Template != 0 {
				members = append(members, row.Template)
			}
		}
		tpl, e := moonPickMember(members, seed[:])
		if e != nil {
			sink.skip(name, "组里没有成员", fmt.Sprintf("group=%d", sr.Group))
			continue
		}
		if exp, handled := s.moonDeclarationExpansion(tpl, sr.Name, p); handled {
			switch {
			case exp.skip != "":
				sink.skip(name, exp.skip, fmt.Sprintf("占位件=%d group=%d", tpl, sr.Group))
			case len(exp.pool) == 0:
				// 展开池为空（目录没装配出来）⇒ **不发达占位件**，宁缺勿编，但要刺眼。
				sink.skip(name, exp.what+" 展开池为空", fmt.Sprintf("占位件=%d group=%d", tpl, sr.Group))
			default:
				s.grantMoonOath(sink, name+exp.what, base, uint64(sr.Group), exp.pool)
			}
			continue
		}
		// 数量就是源声明的 Count（=1）。"700%/300% 的效率"落在**掷骰次数**上，见下面两段：
		// 每件产物在客户端是**独立一格**（业主 + 玩家实证），帧的每座上限是 126 件。
		sink.grant(name, MoonRewardChoice{Template: tpl, Count: sr.Count, Group: sr.Group})
	}

	// ---- 3. 效率产出（源 [reward multiple info] 的倍数：装备 / 誓约·星蕴石）----
	if hasEquip {
		out.Rolls += int(equipMult)
		s.rollMoonSourceEquipment(sink, base, equipMult, p)
	}
	if hasOath {
		out.Rolls += int(oathMult)
		s.rollMoonSourceOath(sink, base, oathMult, p)
	}

	out.Grants = sink.grants
	out.Skipped = sink.skips
	sort.SliceStable(out.Skipped, func(i, j int) bool { return out.Skipped[i].What < out.Skipped[j].What })
	out.Summary = fmt.Sprintf("块=%s 条目=%d 倍数[装备=%s 誓约=%s] 掷骰=%d 实发=%d/%d件上限 跳过=%d",
		p.Block.Contents, len(p.Block.Special),
		multText(hasEquip, equipMult), multText(hasOath, oathMult),
		out.Rolls, out.Grants, p.MaxRows, len(out.Skipped))
	if len(out.Plan.Grants) == 0 {
		return out, fmt.Errorf("Moon source reward produced nothing settleable")
	}
	return out, nil
}

func multText(ok bool, v uint32) string {
	if !ok {
		return "-"
	}
	return fmt.Sprintf("%d", v)
}

// rollMoonSourceEquipment 做 `[reward multiple info]` 类别 2 的 N 次装备掷骰。
//
// 每次掷骰都走**源的掉落/品质判定表**（Roll → Tables.Probability 定出不出、Tables.Rarity
// 定什么品质），池子是源装备组的成员 ∩ 目录里可入包的装备 ⇒ 品质分布就是游戏自己那套率
// （与小深渊同源），本仓不另编概率。
//
// 表里那一掷可能什么都不给，也可能给金币/材料 —— 那不是这一段要发的东西，静默略过。
func (s *Service) rollMoonSourceEquipment(sink *moonSourceSink, base string, rolls uint32, p MoonSourcePolicy) {
	if len(p.RarityPool) > 0 {
		s.rollMoonRarityEquipment(sink, base, rolls, p.RarityPool)
		return
	}
	if len(p.EquipmentPool) == 0 {
		sink.skip("装备掷骰", "源装备组的成员里没有可入包装备",
			fmt.Sprintf("倍数=%d 组模板数=%d", rolls, len(p.EquipmentPool)))
		return
	}
	for i := uint32(0); i < rolls; i++ {
		seed := sha256.Sum256([]byte(fmt.Sprintf("%s/equip/%d", base, i)))
		outcome, e := Roll(s.Catalog, s.Tables, s.Rules, p.EquipmentPool,
			binary.LittleEndian.Uint32(seed[:]), p.Level, p.Rank, 0)
		if e != nil {
			sink.skip("装备掷骰", "掷骰失败", fmt.Sprintf("第%d次: %v", i+1, e))
			return
		}
		for _, a := range outcome.Awards {
			if a.Template == 0 || a.Amount == 0 {
				continue
			}
			if _, _, e := s.moonEquipmentDestination(a.Template); e != nil {
				continue // 掉表里还有金币/材料，它们不是这一段要发的东西
			}
			// 一次掷骰只取一件：源里一格装备产出对应一件（旧实现也是 break 取第一件），
			// 顺带把奖单长度钉在 DecodeMoonReward 的上界内。
			sink.grant(fmt.Sprintf("装备掷骰(第%d次)", i+1), MoonRewardChoice{Template: a.Template, Count: 1})
			break
		}
	}
}

// rollMoonSourceOath 做 `[reward multiple info]` 类别 3 的 N 次誓约·星蕴石掷骰。
//
// 池子是**誓约/结晶装备**（MoonOathEquipmentPool：掉落表里整组都是 `[oath]`/`[primer]`
// 装备的那些组），掷骰按源权重。既然「誓约和星蕴石本质也是装备」，这一段发出去的
// 就该是能穿的装备，而不是那个打不开的「X Oath/Crystal Set」显示盒。
// 取证见 MoonSourcePolicy.OathPool。
func (s *Service) rollMoonSourceOath(sink *moonSourceSink, base string, rolls uint32, p MoonSourcePolicy) {
	if len(p.OathPool) == 0 {
		sink.skip("誓约掷骰", "源里没有「整组都是誓约/结晶装备」的组", fmt.Sprintf("倍数=%d", rolls))
		return
	}
	for i := uint32(0); i < rolls; i++ {
		s.grantMoonOath(sink, fmt.Sprintf("誓约掷骰(第%d次)", i+1), base, uint64(i), p.OathPool)
	}
}

// grantMoonOath 从誓约/结晶装备池按源权重抽一件并登记。
//
// 种子由 base 与 salt 决定（同种子同结果），与其它掷骰同一套做法；
// 权重合计为 0（或超出 uint32）时退化成等权，不编造分布。
func (s *Service) grantMoonOath(sink *moonSourceSink, what, base string, salt uint64, pool []MoonOathPoolMember) {
	if len(pool) == 0 {
		return
	}
	seed := sha256.Sum256([]byte(fmt.Sprintf("%s/oath/%d", base, salt)))
	rng := RNG{binary.LittleEndian.Uint32(seed[:])}
	total := uint64(0)
	for _, m := range pool {
		total += uint64(m.Weight)
	}
	pick := pool[0]
	if total == 0 || total > uint64(^uint32(0)) {
		pick = pool[rng.Next(uint32(len(pool)))]
	} else {
		roll := uint64(rng.Next(uint32(total)))
		acc := uint64(0)
		for _, m := range pool {
			acc += uint64(m.Weight)
			if roll < acc {
				pick = m
				break
			}
		}
	}
	sink.grant(what, MoonRewardChoice{Template: pick.Template, Count: 1})
}

// MoonSourceFixedChoices 从固定组的成员推导固定产物条目。
//
// 数量口径：门票与银币沿用**玩家实测**（60 / 15，见 moon_rewards.go 的同款说明）；其余成员各 1 件。
// 这不是编数：源里根本没有"这次给几张"这种表，而实测值是可回指的 L0′ 依据。
func MoonSourceFixedChoices(members []uint32, observed map[uint32]uint32) []MoonRewardChoice {
	out := make([]MoonRewardChoice, 0, len(members))
	seen := map[uint32]bool{}
	for _, tpl := range members {
		if tpl == 0 || seen[tpl] {
			continue
		}
		seen[tpl] = true
		n := uint32(1)
		if v, ok := observed[tpl]; ok && v > 0 {
			n = v
		}
		out = append(out, MoonRewardChoice{Template: tpl, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Template < out[j].Template })
	return out
}

// MoonSourceEquipmentPool 取"源装备组 ∩ 目录可入包装备"。
//
// `EquipmentCatalog.DropPool()` 的注释写着 Basic 决定成员资格 ⇒ 池里的每一项都一定发得出去，
// 所以做交集是安全的（只会更窄，不会引入发不出去的东西）。
func MoonSourceEquipmentPool(pool []inventory.EquipmentDrop, want []uint32) []inventory.EquipmentDrop {
	if len(pool) == 0 || len(want) == 0 {
		return nil
	}
	set := make(map[uint32]bool, len(want))
	for _, t := range want {
		set[t] = true
	}
	out := make([]inventory.EquipmentDrop, 0, len(pool))
	for _, d := range pool {
		if set[d.ID] {
			out = append(out, d)
		}
	}
	return out
}

// MoonSourceGroupMembers 把组号展开成成员模板（explicit + smart，去重保序）。
func (s *Service) MoonSourceGroupMembers(groups []uint32) []uint32 {
	seen := map[uint32]bool{}
	var out []uint32
	for _, gid := range groups {
		g, ok := s.Catalog.DropGroupByID(gid)
		if !ok {
			continue
		}
		rows := append(append([]catalog.DropWeight{}, g.Explicit...), g.Smart...)
		for _, row := range rows {
			if row.Template == 0 || seen[row.Template] {
				continue
			}
			seen[row.Template] = true
			out = append(out, row.Template)
		}
	}
	return out
}

// moonOathEquipmentType 把 [equipment type] 的原文归到誓约族的哪一支。
//
// 原文实测只有两种：`[oath]`（誓约装备）与 `[primer]`（星蕴石/结晶装备）。
func moonOathEquipmentType(text string) (string, bool) {
	switch text {
	case "[oath]":
		return "oath", true
	case "[primer]":
		return "primer", true
	}
	return "", false
}

// moonEquipmentTypeText 取装备定义里 [equipment type] 的原文（type-6 字符串）。
//
// ⚠️ 这一段实测是**两格**：`` `[oath]` 21 `` —— 原文 + 一个整数。所以判据只看
// `cells[0]`，不能要求恰好一格（第一版写成 len != 1 时整段失效，实测池子恒空）。
func moonEquipmentTypeText(cells []pvf.Token) string {
	if len(cells) == 0 || cells[0].Type != 6 {
		return ""
	}
	return cells[0].Text
}

// MoonOathEquipmentPool 收集「整组成员都是誓约/结晶装备」的掉落组，作为类别 3 的掷骰池。
//
// 为什么不用源里 `[special setinfo reward]` 指向的那个组（21468 / 21470）：那两组装的是
// 「X Oath/Crystal Set」/「Dim Oath Crystal」显示件 —— `[etc]`、没有产出段、玩家打不开；
// 而真正的誓约/结晶是 `equipment/character/common/{oath,primer}/*.equ`（业主口径
// 「誓约和星蕴石本质也是装备，也有品质 4 档」）。详见 MoonSourcePolicy.OathPool。
//
// 判据是**结构性的**（不按组号硬编码）：一个组的成员**每一个**都要能在完整装备目录里
// 解析出 `[equipment type]` = `[oath]`/`[primer]`，有一个不是就整组不算。
// 实测 `etc/dungeondroptablebygroup.etc` 的 1221 个组里恰好 12 个满足
// （21456..21460、21464..21467、21481..21483），合计 100 件、全 115 级、
// rarity 2/3/4/6/8（实测分布 2:1 3:25 4:25 6:25 8:24，oath 48 / primer 52）。
//
// 权重用源里那一列（这批组全是 20 ⇒ 等权）；合计为 0 时由掷骰退化成等权。
func (s *Service) MoonOathEquipmentPool() ([]MoonOathPoolMember, string) {
	if s == nil || s.Equipment == nil {
		return nil, "誓约池：装备目录不可用（Equipment=nil）"
	}
	// 与装备掷骰同源：优先完整目录（选择集只有 rarity ≤ 2 的老货）。
	var src interface {
		Definition(uint32) (inventory.EquipmentDefinition, error)
	} = s.Equipment.Full
	via := "Full"
	if src == nil {
		src, via = s.Equipment, "Selection"
	}
	var out []MoonOathPoolMember
	seen := map[uint32]bool{}
	groups, rejected := 0, 0
	byFamily, byRarity := map[string]int{}, map[int32]int{}
	for _, g := range s.Catalog.DropGroups {
		rows := append(append([]catalog.DropWeight{}, g.Explicit...), g.Smart...)
		if len(rows) == 0 {
			continue
		}
		var members []MoonOathPoolMember
		ok := true
		for _, row := range rows {
			if row.Template == 0 {
				continue
			}
			// 便宜的先判：堆叠物目录里查得到就一定不是誓约装备（那张表只有 stackable）。
			if _, stackable := s.Catalog.Items[row.Template]; stackable {
				ok = false
				break
			}
			def, e := src.Definition(row.Template)
			if e != nil {
				ok = false
				break
			}
			family, isOath := moonOathEquipmentType(moonEquipmentTypeText(def.Fields["[equipment type]"]))
			if !isOath {
				ok = false
				break
			}
			rarity := int32(-1)
			if v := def.Fields["[rarity]"]; len(v) == 1 && v[0].Type == 0 {
				rarity = v[0].Value
			}
			members = append(members, MoonOathPoolMember{
				Template: row.Template, Weight: row.Weight, Family: family, Rarity: rarity,
			})
		}
		if !ok || len(members) == 0 {
			rejected++
			continue
		}
		groups++
		for _, m := range members {
			if seen[m.Template] {
				continue
			}
			seen[m.Template] = true
			out = append(out, m)
			byFamily[m.Family]++
			byRarity[m.Rarity]++
		}
	}
	keys := make([]int, 0, len(byRarity))
	for k := range byRarity {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("rarity%d:%d件", k, byRarity[int32(k)]))
	}
	note := fmt.Sprintf("誓约装备池[%s]（目录=%s 命中组=%d/%d 件=%d 誓约=%d 星蕴石=%d 整组非誓约装备的组=%d）",
		strings.Join(parts, " "), via, groups, len(s.Catalog.DropGroups), len(out),
		byFamily["oath"], byFamily["primer"], rejected)
	if len(out) == 0 {
		return nil, note + " ⇒ 一个成员都没收到（源表变了？）"
	}
	return out, note
}

// MoonOathDeclaration 报告一条 `[special ... reward]` 声明该不该按誓约族**改发装备**。
//
// 判据是**声明名**（源自己的标签名）：含 oath / crystal ⇒ 誓约·星蕴石族，
// 也就是 `SetOathPrimerReward`（它的组 21468 装的正是那 12 个「X Oath/Crystal Set」盒）。
//
// ⚠️ **已知边界（留给业主定）**：`RareEquipmentReward` 的名字里只有 Equipment，
// 但它的组 21470 装的是 `10419544`「Dim Oath Crystal」——
//
//   - 它的 explain 是 “A Common Oath Crystal that provides Common Oath Points.”，
//     与誓约盒（explain 明写 “Unique, Legendary, Epic, or Primeval”）不是一回事；
//   - 源里另有一件**同名装备** `100401592`（`[primer]`、rarity 2），在掉落组 21456。
//
// 今天**不**把它并进誓约族：它是普通（rare）档的结晶，原样按 `[etc]` 发。
// 若业主实机看到它也不对，并进来的判据应该是**组内容的图标**
// （`.../19_OathPrimer/Primer/Primer_00_Basic.img`）—— 但那批「显示件」没有
// `[creation rate]`，`ImportLoot` 不收，目录里只有 `SupplementItemIndex` 补的
// 空壳（`Script.Cells` 为空）⇒ 运行时读不到它们的 `[icon]`；真要按图标判，
// 得让装配层（手上有 archive）预先把这张表算出来再传进策略。
func (s *Service) MoonOathDeclaration(name string) bool {
	return moonFamilyOath(name)
}

// moonEquipmentSlots 是装备部位数（上衣/下装/肩/腰/鞋/项链/手镯/戒指/辅助装备/魔法石/耳环）。
// 实测：`SetEquipmentReward` 声明的 12 个套装盒，对应的真装备组正是「12 块 × 11 件」。
const moonEquipmentSlots = 11

// moonExpansion 是「这条声明的成员该换成什么」的结论。
type moonExpansion struct {
	what string               // 日志后缀（"（套装盒改发装备）" 之类）
	pool []MoonOathPoolMember // 非空 ⇒ 从池里按源权重抽一件
	skip string               // 非空 ⇒ 记跳过、不发
}

// moonDeclarationExpansion 决定一条 `[special ... reward]` 命中后的成员该怎么发。
//
// 口径与深渊对齐（cmd/wireprobe/booster_flow.go 的 boosterBoxSource.serverUnwrap）：
//
//	① 成员是**真·客户端可开的罐子** —— booster 目录里声明了 `[oath item booster]` /
//	   `[lottery ani info]`（即 `p.KeepAsIs`）⇒ **原样落地**，玩家自己在消耗品里点开；
//	② 其余一律**展开**成真物品。源里这几条声明的成员实测都是**占位件**：24 格的最小
//	   脚本、没有 `[creation rate]`、没有产出段，而且名字是**类别名**不是物品名
//	   （"Death in the Shadows Set" / "Normal/Legacy Weapon"）—— 原样落地就是打不开的
//	   死件，正是深渊那次「只出盒子且打不开」的同一个病。
//
// 展开目标按**声明名**（源自己的标签）取，逐条都有取证：
//
//	SetOathPrimerReward   → 誓约/结晶装备池（MoonOathEquipmentPool）
//	RareEquipmentReward   → 同一池里 `[rarity]` 最小的那一档（= 100401592「Dim Oath
//	                        Crystal」，与声明同名），因为这条要的是**普通档结晶**
//	SetEquipmentReward    → 12 套装装备池（MoonEquipmentSetPool）
//	WeaponEquipmentReward → 源里找不到可展开的武器表（成员 10336310「Normal/Legacy
//	                        Weapon」是类别占位件，也不在 booster 目录）⇒ **不落地**并记跳过
//	其余（如 MonsterCard_ExpectationReward_0 的 5 张真·附魔怪物卡）⇒ handled=false，原样发
func (s *Service) moonDeclarationExpansion(tpl uint32, declName string, p MoonSourcePolicy) (moonExpansion, bool) {
	if p.KeepAsIs[tpl] {
		return moonExpansion{}, false // ① 真罐子：原样落地
	}
	switch strings.ToLower(strings.TrimSpace(declName)) {
	case "setequipmentreward":
		return moonExpansion{what: "（套装盒改发装备）", pool: p.SetEquipmentPool}, true
	case "setoathprimerreward":
		return moonExpansion{what: "（誓约族改发装备）", pool: p.OathPool}, true
	case "rareequipmentreward":
		return moonExpansion{what: "（普通档结晶改发装备）", pool: moonMinRarityOathPool(p.OathPool)}, true
	case "weaponequipmentreward":
		return moonExpansion{skip: "源里没有可展开的武器表（成员是类别占位件）"}, true
	}
	return moonExpansion{}, false
}

// moonMinRarityOathPool 取誓约/结晶池里 `[rarity]` 最小的那一档。
//
// 依据：`RareEquipmentReward` 的成员是 `10419544`「Dim Oath Crystal」（explain
// "A Common Oath Crystal that provides Common Oath Points."），源里同名的**装备**是
// `100401592`（`[primer]`、rarity 2）—— 池里 rarity 最小的正是它这一档（实测只有 1 件）。
func moonMinRarityOathPool(pool []MoonOathPoolMember) []MoonOathPoolMember {
	if len(pool) == 0 {
		return nil
	}
	min := int32(-1)
	for _, m := range pool {
		if m.Rarity < 0 {
			continue
		}
		if min < 0 || m.Rarity < min {
			min = m.Rarity
		}
	}
	if min < 0 {
		return nil
	}
	out := make([]MoonOathPoolMember, 0, len(pool))
	for _, m := range pool {
		if m.Rarity == min {
			out = append(out, m)
		}
	}
	return out
}

// MoonEquipmentSetPool 收集「成员数 = 声明成员数 × 部位数」的**全装备**掉落组，
// 作为 `SetEquipmentReward`（12 个套装盒）的展开池。
//
// 取证（2026-10-09 直读内层 PVF）：
//
//   - 声明组 21279 是 12 个 `[etc]` 占位盒，名字就是 12 个**套装名**
//     （"Death in the Shadows Set" … "Magic Domain Set"，与誓约那 12 个盒同序同构）；
//   - 掉落表里恰好有 3 个「12 块 × 11 件」的全装备组（21253 / 21255 / 21257），
//     块序与声明的 12 个盒**逐一对位**（12/12 逐名核对通过，含
//     "Ethereal Orb Arts"↔"Novice Fox"、"Alpha of the Pack Hunt"↔"Pack Omega"、
//     "Magic Domain"↔"Faint Magic Domain"），三组是同一 12 套的 3 个品级档，
//     合计 396 件、全部 115 级。
//
// 判据是结构性的（不按组号硬编码）：成员数 == 声明成员数 × 11，且**每一个**成员都能在
// 完整装备目录里解析出非空的 `[equipment type]`。
func (s *Service) MoonEquipmentSetPool(block catalog.DungeonRewardBlock) ([]MoonOathPoolMember, string) {
	if s == nil || s.Equipment == nil {
		return nil, "套装池：装备目录不可用（Equipment=nil）"
	}
	declared := 0
	for _, sr := range block.Special {
		if !strings.EqualFold(strings.TrimSpace(sr.Name), "SetEquipmentReward") || sr.Group == 0 {
			continue
		}
		if g, ok := s.Catalog.DropGroupByID(sr.Group); ok {
			declared = len(g.Explicit) + len(g.Smart)
		}
	}
	if declared <= 0 {
		return nil, "套装池：本块没有 SetEquipmentReward 声明，拿不到成员数"
	}
	want := declared * moonEquipmentSlots
	var src interface {
		Definition(uint32) (inventory.EquipmentDefinition, error)
	} = s.Equipment.Full
	via := "Full"
	if src == nil {
		src, via = s.Equipment, "Selection"
	}
	var out []MoonOathPoolMember
	seen := map[uint32]bool{}
	groups := 0
	for _, g := range s.Catalog.DropGroups {
		rows := append(append([]catalog.DropWeight{}, g.Explicit...), g.Smart...)
		if len(rows) != want {
			continue
		}
		members := make([]MoonOathPoolMember, 0, len(rows))
		ok := true
		for _, row := range rows {
			if row.Template == 0 {
				ok = false
				break
			}
			if _, stackable := s.Catalog.Items[row.Template]; stackable {
				ok = false
				break
			}
			def, e := src.Definition(row.Template)
			if e != nil || moonEquipmentTypeText(def.Fields["[equipment type]"]) == "" {
				ok = false
				break
			}
			rarity := int32(-1)
			if v := def.Fields["[rarity]"]; len(v) == 1 && v[0].Type == 0 {
				rarity = v[0].Value
			}
			members = append(members, MoonOathPoolMember{Template: row.Template, Weight: row.Weight, Rarity: rarity})
		}
		if !ok {
			continue
		}
		groups++
		for _, m := range members {
			if seen[m.Template] {
				continue
			}
			seen[m.Template] = true
			out = append(out, m)
		}
	}
	note := fmt.Sprintf("套装装备池[声明=%d盒 × %d部位 ⇒ 组大小%d 命中组=%d 件=%d]（目录=%s）",
		declared, moonEquipmentSlots, want, groups, len(out), via)
	if len(out) == 0 {
		return nil, note + " ⇒ 没有匹配的组（源表变了？）"
	}
	return out, note
}

// MoonSourceEquipmentTemplates 收集装备族条目的组的成员（用于和可入包装备取交集）。
func (s *Service) MoonSourceEquipmentTemplates(block catalog.DungeonRewardBlock, declared []uint32) []uint32 {
	seen := map[uint32]bool{}
	var out []uint32
	add := func(tpl uint32) {
		if tpl != 0 && !seen[tpl] {
			seen[tpl] = true
			out = append(out, tpl)
		}
	}
	for _, tpl := range declared {
		add(tpl)
	}
	var groups []uint32
	for _, sr := range block.Special {
		if sr.Group != 0 && moonFamilyEquipment(sr.Name) {
			groups = append(groups, sr.Group)
		}
	}
	for _, tpl := range s.MoonSourceGroupMembers(groups) {
		add(tpl)
	}
	return out
}

// SplitMoonSourceGroups 把 `[normal group index]` 的组号分成「产出池」与「固定产物的组」。
//
// 判据**只看堆叠物目录**（`LootCatalog.Items` 只有 stackable），因此**不依赖装备目录是否装全**：
//
//	组的成员里含堆叠物（门票/银币/通宝罐这类实体物品）⇒ 固定产物的组
//	一个都不含（全是装备模板）                        ⇒ 产出池（供装备掷骰）
//
// ⚠️ 为什么不用"组 ∩ 可入包装备 非空"来判：`Equipment` 为 nil 或目录不全时那个交集恒为空，
// 于是**每一个**组都会被判成固定组 —— 实测（runtime/moonrewardprobe -dungeon 100004137）
// 把 21251 与组 3 的 803 个装备模板全塞进了固定产物。那种退化在运行时是静默的，必须避免。
//
// 不按组号硬编码：21251 / 3 / 21291 只是 100004137 的实测值，判据对任何副本成立。
func (s *Service) SplitMoonSourceGroups(groups []uint32) (pool, fixed []uint32) {
	for _, gid := range groups {
		stackable := false
		for _, tpl := range s.MoonSourceGroupMembers([]uint32{gid}) {
			if _, ok := s.Catalog.Items[tpl]; ok {
				stackable = true
				break
			}
		}
		if stackable {
			fixed = append(fixed, gid)
			continue
		}
		pool = append(pool, gid)
	}
	return pool, fixed
}

// ⚠️ 装备掷骰**不使用源组的成员做池**：`Roll` 是按品级/等级窗口从平台池里挑模板的
// （见 internal/loot/rules.go 的 equipmentCandidates），而实测那 817 个源组模板
// （115 级）并不在平台的可入包装备池里 —— 拿它们做交集会恒为空，装备掷骰一次都发不出
// 且**静默**。所以装备池就用 `Equipment.DropPool()`（旧实现与玩家实机验过的那一套），
// 源提供的是**掷几次**（`[reward multiple info]`）。
//
// `MoonSourceEquipmentPool` 只为"将来平台池真的按源组收窄"那一天留着，并被单测钉住行为。

// MoonRarityWeights 把源的 `[basis of rarity dicision]` 前 9 列，**在池里真有模板的那几档上
// 重归一化**，返回 (档位, 权重)。
//
// 为什么要重归一化（业主 2026-10-09 明确的口径）：源表前两档（rarity 0/1）占了 96.5%，
// 而本副本的池里只有 rarity 2/3/4/6 ⇒ 照原表掷，96.5% 会落到「该档无模板」再退到最近档，
// 实际产出 ≈ 99% 都是稀有档。业主口径是：**把无模板那几档占的比例，按各可用档自身的权重
// 分摊给有模板的几档** —— 数学上就是「在可用档上重归一化」。单机修复允许适度提高产出
//（国服实测确实以稀有档为主，所以这只是一点上浮，不改变体感）。
//
// w[r] = table[r] - table[r-1]（table 是累计阈值，分母 1e6）；可用集 A = {r | len(pool[r]) > 0}；
// W = Σ_{r∈A} w[r]；返回的权重是 w[r]/W。没有可用档、或 W ≤ 0 时返回 nil（调用方记跳过）。
func MoonRarityWeights(table []float64, pool map[int32][]uint32) ([]int32, []float64) {
	if len(table) < 9 {
		return nil, nil
	}
	var tiers []int32
	var weights []float64
	prev := 0.0
	for r := 0; r < 9; r++ {
		mass := table[r] - prev
		prev = table[r]
		if mass <= 0 || len(pool[int32(r)]) == 0 {
			continue
		}
		tiers = append(tiers, int32(r))
		weights = append(weights, mass)
	}
	if len(tiers) == 0 {
		return nil, nil
	}
	total := 0.0
	for _, w := range weights {
		total += w
	}
	if total <= 0 {
		return nil, nil
	}
	for i := range weights {
		weights[i] /= total
	}
	return tiers, weights
}

// rollMoonRarityEquipment 做「按重归一化后的品级权重掷一档 → 该档的池里抽一件」。
//
// 权重口径见 MoonRarityWeights。与深渊的 `Roll`（照原表掷 + 退到最近品级）**不同**是有意为之：
// 深渊那边照原表是国服行为，不动；这里按业主口径做了单机向的上浮。
func (s *Service) rollMoonRarityEquipment(sink *moonSourceSink, base string, rolls uint32, pool map[int32][]uint32) {
	tiers, weights := MoonRarityWeights(s.Tables.Rarity, pool)
	if len(tiers) == 0 {
		sink.skip("装备掷骰", "源 rarity 表不可用，或池里一个模板都没有",
			fmt.Sprintf("列数=%d 池档数=%d", len(s.Tables.Rarity), len(pool)))
		return
	}
	for i := uint32(0); i < rolls; i++ {
		seed := sha256.Sum256([]byte(fmt.Sprintf("%s/equip/%d", base, i)))
		rng := RNG{binary.LittleEndian.Uint32(seed[:])}
		roll := float64(rng.Next(1000000)+1) / 1000000
		rarity := tiers[len(tiers)-1]
		acc := 0.0
		for k, w := range weights {
			acc += w
			if roll <= acc {
				rarity = tiers[k]
				break
			}
		}
		items := pool[rarity]
		if len(items) == 0 {
			sink.skip(fmt.Sprintf("装备掷骰(第%d次)", i+1), "该品级没有模板", fmt.Sprintf("品级=%d", rarity))
			continue
		}
		tpl := items[rng.Next(uint32(len(items)))]
		sink.grant(fmt.Sprintf("装备掷骰(第%d次)", i+1), MoonRewardChoice{Template: tpl, Count: 1})
	}
}

// MoonRewardMaxRows 是**每座**产物行数的硬上限。
//
// 依据：`protocol.ConquestClearReward115` 的编码器对每个在场座位要求
// `0 < len(Rewards[slot]) <= 126`（见 internal/game/protocol/conquest_reward115.go），
// 所以策略的 `MaxRows` 与任何奖单的 `Grants` 都不能超过它。
//
// ⚠️ 2026-10-09 修正：`DecodeMoonReward` 原先用的是**老**月湖策略留下的 16 件上界
// （那时奖单是 `Draws <= 16` 的 Choice 列表）。源驱动路径按 `MaxRows`（=126）记账，
// 而蔚蓝号的效率是「装备 10 / 誓约 4」（沉月湖是 7/3）⇒ 固定 2 件 + 基础产物 + 10 + 4 > 16
// ⇒ 奖单**写完再读回**时被判 `foreign/corrupt Moon reward proof` ⇒ CMD46 整条被拒
// ⇒ **结算面板根本不出现**（业主实机 2026-10-09 21:0x）。沉月湖恰好卡在 16 以内才没露出来。
const MoonRewardMaxRows = 126

// ValidateMoonSourcePolicy 是源驱动策略的**入场前自检**（与旧 ValidateMoonRewards 同一位置调用）。
//
// 为什么要在进门前查：装不出东西的策略必须在这里失败，而不是让玩家打完 Boss 才发现翻牌是空的
// （旧实现就是因为只在"打完"那一层失败，才出过"能进门、领奖报错"的半好状态）。
//
// 判据保持结构性（不需要 run）：身份对得上、声明了内容、行数上限已设、
// 固定产物里至少有一件能按**发奖同源**的判据结算；否则若品级池非空也放行
// （那时装备掷骰还有产出）。
func (s *Service) ValidateMoonSourcePolicy(p MoonSourcePolicy) error {
	if s == nil {
		return fmt.Errorf("Moon source policy without service")
	}
	if p.Source == "" || p.Source != s.Catalog.Source.SaveIdentity() {
		return fmt.Errorf("invalid Moon source policy identity")
	}
	if p.SettlementDungeon == 0 {
		return fmt.Errorf("Moon source policy without settlement dungeon")
	}
	if p.MaxRows <= 0 || p.MaxRows > MoonRewardMaxRows {
		return fmt.Errorf("Moon source policy row budget out of range")
	}
	// 声明了「需要展开」的基础产物却没有展开池 ⇒ **进门就失败**：别让玩家打完 Boss 才发现
	// 少东西（与深渊「booster 目录缺失就硬失败、不退化成把包装丢在地上」同一条理由）。
	for _, sr := range p.Block.Special {
		exp, handled := s.moonDeclarationExpansion(0, sr.Name, p)
		if !handled || exp.skip != "" {
			continue
		}
		if len(exp.pool) == 0 {
			return fmt.Errorf("Moon source policy: declaration %s needs an expansion pool but it is empty", sr.Name)
		}
	}
	if len(p.Block.Special) == 0 && len(p.Block.Multiple) == 0 {
		return fmt.Errorf("Moon source policy has no declared rewards")
	}
	settleable := 0
	for _, c := range p.Fixed {
		if _, e := s.moonGrant(c); e == nil {
			settleable++
		}
	}
	if settleable == 0 && len(p.RarityPool) == 0 && len(p.EquipmentPool) == 0 && len(p.OathPool) == 0 {
		return fmt.Errorf("Moon source policy has nothing settleable")
	}
	return nil
}
