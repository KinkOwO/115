package loot

import "fmt"

// 小深渊（调律之边界）的两条**彼此独立**的掉落路线，各自对应客户端的一条阶梯：
//
//	primer —— 天平**中央的珠子**   → fixed 池（星蕴石/固定奖励）
//	oath   —— **天平本体**         → additional 池（誓约）
//
// 官方口径（业主 2026-10-07 提供）：
//
//	通过天平中央的珠子与天平暗示掉落的星蕴石/誓约稀有度。
//	- 中央珠子暗示星蕴石的稀有度
//	- 天平暗示誓约的稀有度
//
// 客户端把它实现成两条独立的 `[ON DAMAGE]` 阶梯（`primer_00_normal_loop.act` 的 cell 120-290）：
//
//	primer_now == primer_max && oath_now < oath_max -> only_oath_up   （OATH_UP_TO_*）
//	primer_now < primer_max                        -> only_primer_up （PRIMER_UP_TO_*）
//
// 两组动作名各自独立，所以两条线的颜色/特效是**分开**的；顺序固定：先把 primer 顶到
// `primer_max`，再推 oath。
//
// ## 档位值域（八档 ↔ scale_primer.mob 的 Primer_00..07）
//
// 40 normal / 41 rare / 42 unique / 43 legendary / 44 epic / 45 primitive / 70 rainbow1 / 71 rainbow2
//
// 依据：收尾动作是 `[SET GROUP ACTION] end (max-40)`，而 `o:hp_limit` / `o:delay_time`
// 都只有 8 项 ⇒ 合法下标 0..7。primer 的八档与 fixed 池里的八个 tier 标签**一一对应**，
// 且 fixed 池按 tier 汇总的权重恰好等于官方公布的六档 + 两档幸运分布
// （normal 47.0667% / rare 27.2% / unique 20% / legendary 3.6% / epic 1.6% /
//  primeval 0.15% / luck15 0.3333% / luck30 0.05%，合计 1e6）。
//
// oath 只用到其中四档（**没有 rare**）：40 normal / 42 unique / 43 legendary / 44 epic /
// 45 primitive —— 这正是 additional 池那四档誓约盒的档位，逐档对齐。
//
// ## 为什么档位必须在**进本时**先掷
//
// noti 2838 要在天平开场执行 `Primer_Proc.act` 之前到位（见 oath_info.go），而掉落发生在
// **天平死亡**。所以档位先于战斗确定、掉落再按它选池 —— 这样「珠子爬到哪一档」和
// 「发出去的是哪一档」在客户端上是同一个数字（脚本里 `omen_drop_process_rarity ==
// primer_now` 就是靠它对齐），不会出现颜色与实际奖励不符。

// tierGradeValues 是本仓认识的八档及其数值。tier 标签来自 CTP 表（fixed / additional /
// coupon 三张表用同一套标签）。
var tierGradeValues = []struct {
	Tier  string
	Value uint32
}{
	{"normal", 40},
	{"rare", 41},
	{"unique", 42},
	{"legendary", 43},
	{"epic", 44},
	{"primeval", 45},
	{"luck15", 70},
	{"luck30", 71},
}

// TierGradeValue 把 CTP 的 tier 标签翻成客户端阶梯的档位值。未知标签返回 0
// （调用方据此判定「这条线本场没有档位」）。
func TierGradeValue(tier string) uint32 {
	for _, t := range tierGradeValues {
		if t.Tier == tier {
			return t.Value
		}
	}
	return 0
}

// TierForGradeValue 是上一条的反查，供日志与测试用。
func TierForGradeValue(v uint32) string {
	for _, t := range tierGradeValues {
		if t.Value == v {
			return t.Tier
		}
	}
	return ""
}

// RunTiers 是一条深渊场次里**两条线各自的档位**。零值表示该线本场没有档位
// （非调律副本、或表里缺这张表）。
type RunTiers struct {
	// Primer 是 fixed 池（星蕴石/中央珠子）的档位：40..45 / 70 / 71。
	Primer uint32
	// Oath 是 additional 池（誓约/天平）的档位：40（本场没拿到誓约）..45。
	Oath uint32
	// PrimerLabel / OathLabel 是**源表自己的档位标签**（CTP 里的 tier 文本）。
	//
	// ⚠️ 大深渊（100005067/68）的 additional 池里有 `voidsoul`（虚无之魂）与
	// `primestella`（太初星蕴石）—— 这两档**不在客户端那 8 档阶梯里**，
	// TierGradeValue 只能返回 0。上一版据此把整条附加奖励跳过，实机表现就是
	// 「动画出了太初特效、地上却什么都没有」。标签留着，掉落阶段就能照源表付。
	// 档位值仍是给客户端看的那个数字（2838），两者不要互相顶替。
	PrimerLabel string
	OathLabel   string
	// Floor / FloorLabel 是本场的**保底档**：入场砝码决定的那个「保证获得对应
	// 稀有度以上的道具」的下界（业主 2026-10-08 口径）。
	//
	// 它由**这张副本自己的 fixed 池**推出来，不是另立的策略表：每张砝码表本来就
	// 是一个独立的 CTP（`unique/legendary/epic.ctp` → 100005066/67/68），
	// 文件里最低的那一档就是它承诺的保底（unique.ctp ⇒ unique，legendary.ctp ⇒
	// legendary，epic.ctp ⇒ epic）。所以**保底是一张表的性质，不是一次掷骰的结果**。
	//
	// 语义（业主定调）：保底只是掷骰的**下界**，掷出来完全可以更高。
	// 因此 PlanRun 掷的是「保底档及以上」那一段权重，而不是把档位钉在保底上。
	// 0 表示这张表没有可比较的保底（例如小深渊，它的 fixed 池从 normal 起）。
	Floor      uint32
	FloorLabel string
}

// PlanRun 在**进本时**预掷两条线的档位。
//
// 两条线各自独立掷一次，谁也不看谁：
//
//	primer：按 fixed 池里**每个 tier 的合计权重**掷（maze 决定用哪一段）；
//	oath  ：先按 [select prob] 掷 additional 的分支，再在该分支内按权重掷条目，
//	        取该条目的 tier —— 分支 1 的唯一一个条目就是「本次没拿到誓约」的空奖，
//	        它的 tier 是 normal ⇒ oath=40，与官方「没拿到誓约」同义。
//
// 掉落阶段用 RollPlanned 按这两个档位选池，保证颜色与奖励一致。
func (a *AttunementRewards) PlanRun(seed, dungeon, maze uint32) (RunTiers, uint32, error) {
	var out RunTiers
	if !a.Enabled() {
		return out, seed, nil
	}
	t, ok := a.byDungeon[dungeon]
	if !ok {
		return out, seed, nil
	}
	rng := RNG{seed}
	if fixed, ok := t.fixedFor(maze); ok && len(fixed.Entries) > 0 {
		out.Floor, out.FloorLabel = fixedFloor(fixed.Entries)
		e, err := pickTierAbove(&rng, fixed.Entries, out.Floor)
		if err != nil {
			return out, seed, err
		}
		out.Primer = TierGradeValue(e)
		out.PrimerLabel = e
	}
	if len(t.Additional) > 0 {
		branch, err := pickAttunementBranch(&rng, t.Additional)
		if err != nil {
			return out, seed, err
		}
		if len(branch.Entries) > 0 {
			e, err := pickAttunement(&rng, branch.Entries)
			if err != nil {
				return out, seed, err
			}
			out.Oath = TierGradeValue(e.Tier)
			out.OathLabel = e.Tier
		}
	}
	return out, rng.Seed, nil
}

// pickTier 在**整张 fixed 表**上按 tier 掷一次，返回该 tier 的标签。
//
// 它等价于「先掷 tier、再在 tier 内掷条目」，因为 fixed 表的权重本来就是按 tier
// 分组、且每组的合计就是该档的概率。
func pickTier(rng *RNG, list []attunementEntry) (string, error) {
	weights := map[string]uint32{}
	order := []string{}
	var total uint32
	for _, e := range list {
		if _, seen := weights[e.Tier]; !seen {
			order = append(order, e.Tier)
		}
		if e.Tier != "" {
			weights[e.Tier] += e.Weight
			total += e.Weight
		}
	}
	if total == 0 {
		return "", fmt.Errorf("attunement fixed table carries no tiered entries")
	}
	roll := rng.Next(total)
	var acc uint32
	for _, tier := range order {
		if weights[tier] == 0 {
			continue
		}
		acc += weights[tier]
		if roll < acc {
			return tier, nil
		}
	}
	return order[len(order)-1], nil
}

// fixedFloor 取一张 fixed 池的**保底档**：池里最低的那个「六档稀有度」。
//
// 只看 40..45（normal..primeval）：70/71 是「神秘的幸运」那条**独立加成轴**，
// 与稀有度不是一个量纲，不参与比较（它们在任何保底之下都照样可以出）；
// 不在八档阶梯里的标签（voidsoul / primestella）也没有可比大小。
//
// 池里一个可比较档位都没有时返回 0 = 「这张表没有保底」，掷骰与旧行为逐字节相同。
func fixedFloor(entries []attunementEntry) (uint32, string) {
	var floor uint32
	var label string
	for _, e := range entries {
		v := TierGradeValue(e.Tier)
		if v < 40 || v > 45 {
			continue
		}
		if floor == 0 || v < floor {
			floor, label = v, e.Tier
		}
	}
	return floor, label
}

// pickTierAbove 在**保底档及以上**的档位里按权重掷一次。
//
// 保底是下界而不是结果（业主 2026-10-08 定调）：掷出来完全可以更高。
// 已知四张源表的 fixed 池本来就只写保底档及以上的条目，所以对当前表这是**恒等变换**
// —— 它存在的意义是把「保底」变成机制而不是巧合，源表将来多写一条低档时不会静默发错档。
//
// 保底为 0（表里没有可比较档位）时直接退回 pickTier，分布与旧行为完全相同。
//
// 遇到既不是六档稀有度、也不是两档幸运的标签时**报错**，不做「算不算达标」的猜测：
// 那个标签意味着源表换了一套量纲，猜错的代价是发出低于保底的奖励。
func pickTierAbove(rng *RNG, list []attunementEntry, floor uint32) (string, error) {
	if floor == 0 {
		return pickTier(rng, list)
	}
	eligible := make([]attunementEntry, 0, len(list))
	for _, e := range list {
		switch v := TierGradeValue(e.Tier); {
		case v == 70 || v == 71:
			// 独立加成轴：不受稀有度保底约束。
			eligible = append(eligible, e)
		case v >= 40 && v <= 45:
			if v >= floor {
				eligible = append(eligible, e)
			}
		default:
			return "", fmt.Errorf("attunement fixed pool tier %q is not comparable with the guaranteed floor %d", e.Tier, floor)
		}
	}
	if len(eligible) == 0 {
		return "", fmt.Errorf("attunement fixed pool has nothing at or above the guaranteed floor %d", floor)
	}
	return pickTier(rng, eligible)
}

// pickWithinTier 在给定 tier 的条目里按权重掷一个。池内权重是**组内相对值**，
// 所以这里按组内合计归一化，不能沿用「百万空间」的读法。
func pickWithinTier(rng *RNG, list []attunementEntry, tier string) (attunementEntry, error) {
	var in []attunementEntry
	var total uint32
	for _, e := range list {
		if e.Tier == tier {
			in = append(in, e)
			total += e.Weight
		}
	}
	if len(in) == 0 {
		return attunementEntry{}, fmt.Errorf("attunement tier %q carries no entry", tier)
	}
	roll := rng.Next(total)
	var acc uint32
	for _, e := range in {
		acc += e.Weight
		if roll < acc {
			return e, nil
		}
	}
	return in[len(in)-1], nil
}

// RollPlanned 按**预掷的档位**选池，是掉落阶段真正走的入口。
//
// 与旧的 Roll 相比只有一处不同：fixed 用 `tiers.Primer` 选定 tier 再在组内掷，
// additional 用 `tiers.Oath` 选定条目 —— 于是「客户端显示的那一档」与
// 「服务端发的这一件」永远是同一档。档位为 0（该线本场没有）时那条线不发东西。
func (a *AttunementRewards) RollPlanned(seed, dungeon, maze uint32, tiers RunTiers) ([]Award, uint32, error) {
	if !a.Enabled() {
		return nil, seed, nil
	}
	t, ok := a.byDungeon[dungeon]
	if !ok {
		return nil, seed, nil
	}
	rng := RNG{seed}
	var out []Award
	if tiers.Primer != 0 || tiers.PrimerLabel != "" {
		if fixed, ok := t.fixedFor(maze); ok {
			e, err := pickWithinTier(&rng, fixed.Entries, plannedTier(tiers.Primer, tiers.PrimerLabel))
			if err != nil {
				return nil, seed, err
			}
			out = append(out, Award{Template: e.Item, Amount: 1})
		}
	}
	if tiers.Oath != 0 || tiers.OathLabel != "" {
		branch, e, err := oathEntryByTier(t, plannedTier(tiers.Oath, tiers.OathLabel))
		if err != nil {
			return nil, seed, err
		}
		for i := uint32(0); i < branch.DropCount; i++ {
			// DropCount 在本表恒为 1；多份时按**同一档**重复发，不再二次掷骰，
			// 否则「显示一档、掉了另一档」会从这里漏回来。
			out = append(out, Award{Template: e.Item, Amount: 1})
		}
	}
	return out, rng.Seed, nil
}

// oathEntry 在 additional 里找出**属于给定档位**的那一支与那一个条目。
//
// 不按「分支下标」而按档位反查，是因为档位就是客户端真正消费的那个数字：
// 分支 1（98.52%）的唯一条目 tier 是 normal ⇒ 对应「本次没拿到誓约」；
// 分支 2（1.48%）的四个条目 tier 是 unique/legendary/epic/primeval。
// 任一档位都只会命中一处，所以这个反查是确定的。
// plannedTier 取「这一场该按哪个 tier 标签选池」：档位能反查就用档位（它才是客户端消费的数字），
// 反查不到（voidsoul / primestella 这类不在 8 档阶梯里的源标签）就退回源表标签。
func plannedTier(grade uint32, label string) string {
	if t := TierForGradeValue(grade); t != "" {
		return t
	}
	return label
}

// oathEntryByTier 按**tier 标签**在 additional 里反查分支与条目。
//
// 与按档位反查等价（标签 ↔ 档位是一一对应），区别只在它同时也认
// `voidsoul`（虚无之魂）/ `primestella`（太初星蕴石）这类非客户端档位的源标签。
func oathEntryByTier(t *attunementDungeon, want string) (attunementAdditional, attunementEntry, error) {
	if want == "" {
		return attunementAdditional{}, attunementEntry{}, fmt.Errorf("attunement oath tier is empty")
	}
	for _, branch := range t.Additional {
		for _, e := range branch.Entries {
			if e.Tier == want {
				return branch, e, nil
			}
		}
	}
	return attunementAdditional{}, attunementEntry{}, fmt.Errorf("attunement oath tier %q has no entry", want)
}

func oathEntry(t *attunementDungeon, grade uint32) (attunementAdditional, attunementEntry, error) {
	want := TierForGradeValue(grade)
	if want == "" {
		return attunementAdditional{}, attunementEntry{}, fmt.Errorf("attunement oath tier %d is outside the eight tiers", grade)
	}
	for _, branch := range t.Additional {
		for _, e := range branch.Entries {
			if e.Tier == want {
				return branch, e, nil
			}
		}
	}
	return attunementAdditional{}, attunementEntry{}, fmt.Errorf("attunement oath tier %q has no entry", want)
}

// AnimationGrade 把本场档位（40..45 / 70 / 71）翻成客户端**掉落演出**那一格的值。
//
// ## 刻度怎么来的（实测，2026-10-08）
//
// 客户端四格演出的存入顺序（`sub_140657920`）= [Unique, Legendary, Epic, Primeval]
// （模块 dword 偏移 58/80/102/124，步长 88 字节）；**索引 = 值 − 40**：
//
//	42 -> EpicDrop   （实机：业主看到「史诗」那一格）
//	40 -> 最低那格    （实机：业主看到低档）
//	43/44/45 以前恒为 Primeval（索引 3/4/5，越界兜底）
//
// 而我们自己的档位阶梯是 40..45 = normal..primeval，**比演出表晚两格** ——
// 这正是「大深渊每次都播太初」的成因：大深渊的保底就是传说(43)，索引 43−40=3
// 恰好是最后一格 Primeval，44/45 更是直接越界兜底。
//
// ⇒ 要让演出显示本场档位，必须 **−2** 再夹到 [40, 43]。
//
// ok=false 表示这一档在演出表里**没有对应格**（normal / rare）：四格演出从「独有」起，
// 低档本来就没有演出。调用方应当**不发**这一帧，而不是拿最近的一格顶上 —— 那会让
// 演出说谎（与「动画和掉落绑定」正好相反）。
func AnimationGrade(tier uint32) (uint32, bool) {
	switch {
	case tier >= 42 && tier <= 45:
		return tier - 2, true
	case tier == 70 || tier == 71:
		// 「神秘的幸运」是独立加成轴，取最高那格。
		return 43, true
	default:
		return 0, false
	}
}

// AnimationSlot 是 AnimationGrade 那个值的名字（客户端四格），只用于日志与验收。
func AnimationSlot(v uint32) string {
	switch v {
	case 40:
		return "UniqueDrop"
	case 41:
		return "LegendaryDrop"
	case 42:
		return "EpicDrop"
	case 43:
		return "PrimevalDrop"
	}
	return ""
}
