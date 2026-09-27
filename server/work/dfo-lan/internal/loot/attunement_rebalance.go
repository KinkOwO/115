package loot

import (
	"fmt"
	"sort"
)

// 本私服的「掉落调参层」。
//
// 官方那张表是按长期反复刷取的生态设计的：1710 场的社区样本里，小深渊 fixed 池的
// 普通 + 稀有占 74%，而征兆在低档位的「无事发生」高达 90%。本服务器是单人模拟端，
// 玩家能刷的次数远少于官方生态 —— 照搬会让「刷了很多场却什么都没拿到」变成常态。
//
// 两条变换都是**可解释的比例运算**，不是随手改常数：
//
//	OmenHalveIdle     每行「① 无事发生」的份额减半；腾出的份额在「② 再激活更高
//	                  品质 1 个」与「③ 结算奖励并重置」之间对半分。
//	FixedTiltPercent  把 fixed 池里「普通 / 稀有」两档的权重各减去这个百分比，
//	                  减掉的权重按高档**现有的比例**补给神器 / 传说 / 史诗 / 太初。
//
// 第二条里「按现有比例」很关键：四档被放大**同一个倍数**，所以稀有度阶梯的形状
// （谁比谁稀有多少）原样保留，变的只是「好东西的总量」。若改成等分或越稀有分得
// 越多，神器与太初的相对关系就被改写了，那是另一回事。
//
// 倾斜幅度刻意做成一个可调的百分比，而不是写死的「减半」：减半会把神器推到 49%，
// 那等于把「稀有」这一档废掉。25% 的效果是高档总量从 25.4% 抬到 43.9% —— 玩家
// 明显更常看到好东西，而梯度还在。
//
// 变换后仍然满足加载期的全部不变量（每份 drop list 与每组 selectProb 都恰好等于
// 1e6），所以它可以在表校验通过之后安全地就地改写，而且当场复核一遍。
//
// ⚠️ 这是**与官服的显式差异**：默认关闭，必须显式打开，并在启动日志里把改前 /
// 改后的关键数字打出来 —— 否则以后有人拿这份表去对官方数据，会一头雾水。
type Rebalance struct {
	// OmenHalveIdle 见文件头。
	OmenHalveIdle bool
	// FixedTiltPercent 见文件头。0 = 固定池不动；必须小于 100。
	FixedTiltPercent uint32
}

// Enabled 报告这份配置是否要改动任何东西。
func (r Rebalance) Enabled() bool { return r.OmenHalveIdle || r.FixedTiltPercent > 0 }

// fixed 池的档次分组。
//
// 「降」的一侧只有普通与稀有 —— 它们在大深渊的三张表里**一条都没有**（那三张表
// 的池子本身就是按难度给的 unique/legendary/epic），所以这条变换天然只作用于
// 小深渊，不需要额外的副本白名单。
//
// luck15 / luck30 两边都不属于：它们是「幸运事件」的档，权重必须原样保留。
var (
	fixedDemotedTiers = map[string]bool{"normal": true, "rare": true}
	fixedRaisedTiers  = map[string]bool{"unique": true, "legendary": true, "epic": true, "primeval": true}
)

// ApplyRebalance 就地改写表，并**重新校验**权重不变量。
//
// 返回 (改写的征兆行数, 改写的 fixed 段数) 供启动日志使用。
//
// 它**不是幂等的**：削减是有方向的，跑两次会削两次。调用方只在启动时调一次。
func (a *AttunementRewards) ApplyRebalance(r Rebalance) (int, int, error) {
	if a == nil || !r.Enabled() {
		return 0, 0, nil
	}
	if r.FixedTiltPercent >= 100 {
		return 0, 0, fmt.Errorf("fixed tilt %d%% would empty the common tiers; use 1..99", r.FixedTiltPercent)
	}
	rows, groups := 0, 0
	for i := range a.Tables {
		t := &a.Tables[i]
		if r.OmenHalveIdle {
			n, err := t.halveOmenIdle()
			if err != nil {
				return rows, groups, fmt.Errorf("dungeon %d omen rebalance: %w", t.Dungeon, err)
			}
			rows += n
		}
		if r.FixedTiltPercent > 0 {
			n, err := t.tiltFixedCommon(r.FixedTiltPercent)
			if err != nil {
				return rows, groups, fmt.Errorf("dungeon %d fixed rebalance: %w", t.Dungeon, err)
			}
			groups += n
		}
		// 不变量当场复核：改写只允许「重新分配同一个 1e6」，不许多也不许少。
		for _, f := range t.Fixed {
			if err := checkAttunementEntries(f.Entries,
				fmt.Sprintf("dungeon %d fixed maze %d (rebalanced)", t.Dungeon, f.Maze)); err != nil {
				return rows, groups, err
			}
		}
		for j, c := range t.Coupons {
			if uint64(c.ObtainProb)+uint64(c.DropProb) > attunementWeightSpace {
				return rows, groups, fmt.Errorf("dungeon %d coupon %d spends %d of the %d space after rebalance",
					t.Dungeon, j, uint64(c.ObtainProb)+uint64(c.DropProb), attunementWeightSpace)
			}
			if len(c.Entries) > 0 {
				if err := checkAttunementEntries(c.Entries,
					fmt.Sprintf("dungeon %d coupon %d (rebalanced)", t.Dungeon, j)); err != nil {
					return rows, groups, err
				}
			}
		}
	}
	return rows, groups, nil
}

// halveOmenIdle 把每行「① 无事发生」减半，腾出的份额补给 ② / ③。
//
// 行 0 是例外：官方那一行的 [drop prob] 本来就是 0（没有「结算」这回事），所以它
// 腾出的份额只能全部并入 ②。这也意味着「拿到第一个征兆」会明显变快。
func (t *attunementDungeon) halveOmenIdle() (int, error) {
	changed := 0
	for i := range t.Coupons {
		c := &t.Coupons[i]
		spent := uint64(c.ObtainProb) + uint64(c.DropProb)
		if spent > attunementWeightSpace {
			return changed, fmt.Errorf("coupon %d already spends %d of the %d space", i, spent, attunementWeightSpace)
		}
		idle := uint32(attunementWeightSpace) - uint32(spent)
		if idle <= 1 {
			continue // 这一行已经几乎没有「无事」可减
		}
		freed := idle - idle/2 // 减半腾出的量（idle 为奇数时向上取，保证真的减掉一半）
		if c.DropProb == 0 {
			c.ObtainProb += freed
		} else {
			// 对半分；奇数多出的那 1 给 ③（结算），让玩家的反馈周期短一点。
			toDrop := (freed + 1) / 2
			c.ObtainProb += freed - toDrop
			c.DropProb += toDrop
		}
		changed++
	}
	return changed, nil
}

// tiltFixedCommon 把一段 fixed 池的普通 / 稀有各减去 tilt%，减掉的权重补给高档。
//
// 取整用**最大余数法**：直接四舍五入会让这一段的合计偏离 1e6，而 1e6 正是加载期
// 用来证伪「列读错了」的那条不变量 —— 调参不该把它弄坏。
func (t *attunementDungeon) tiltFixedCommon(tilt uint32) (int, error) {
	changed := 0
	for i := range t.Fixed {
		f := &t.Fixed[i]

		// ① 降：普通与稀有的权重各减去 tilt%（逐条向下取整，宁可少削不多削）。
		var freed uint64
		for j := range f.Entries {
			e := &f.Entries[j]
			if !fixedDemotedTiers[e.Tier] {
				continue
			}
			cut := uint32(uint64(e.Weight) * uint64(tilt) / 100)
			freed += uint64(cut)
			e.Weight -= cut
		}
		if freed == 0 {
			continue // 这一段没有普通 / 稀有（大深渊的三张表就是如此），或幅度太小
		}

		// ② 补：按四档现有的权重比例分配。
		type slot struct {
			idx    int
			weight uint64
		}
		var upper []slot
		var total uint64
		for j := range f.Entries {
			if !fixedRaisedTiers[f.Entries[j].Tier] {
				continue
			}
			upper = append(upper, slot{j, uint64(f.Entries[j].Weight)})
			total += uint64(f.Entries[j].Weight)
		}
		if total == 0 {
			// 没有任何高档可补：把减掉的权重原样还回当前最大的那条，
			// 保证这一段仍然恰好等于 1e6。宁可等于没改，也不能让表变得不合法。
			max := 0
			for j := range f.Entries {
				if f.Entries[j].Weight > f.Entries[max].Weight {
					max = j
				}
			}
			f.Entries[max].Weight += uint32(freed)
			changed++
			continue
		}

		assigned := make([]uint32, len(upper))
		type remainder struct {
			k int
			r uint64
		}
		rems := make([]remainder, len(upper))
		var used uint64
		for k, u := range upper {
			product := freed * u.weight
			assigned[k] = uint32(product / total)
			rems[k] = remainder{k, product % total}
			used += uint64(assigned[k])
		}
		// 余数从大到小派发，合计精确等于 freed。
		sort.SliceStable(rems, func(x, y int) bool { return rems[x].r > rems[y].r })
		for n := uint64(0); n < freed-used; n++ {
			assigned[rems[n%uint64(len(rems))].k]++
		}
		for k, u := range upper {
			f.Entries[u.idx].Weight += assigned[k]
		}
		changed++
	}
	return changed, nil
}

// FixedTiers 汇总一段 fixed 池里每个档次标签的权重合计。
//
// 同一档次可能由多条条目承载 —— 小深渊的普通档就有三条（其中一条是那个 2000 的
// 「尾巴」）—— 所以对数要看合计而不是逐条。给启动日志与测试用。
func (a *AttunementRewards) FixedTiers(dungeon, maze uint32) map[string]uint32 {
	if a == nil {
		return nil
	}
	t, ok := a.byDungeon[dungeon]
	if !ok {
		return nil
	}
	f, ok := t.fixedFor(maze)
	if !ok {
		return nil
	}
	out := map[string]uint32{}
	for _, e := range f.Entries {
		out[e.Tier] += e.Weight
	}
	return out
}
