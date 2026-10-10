package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/modpolicy"
)

// 这里把 mod 的**掉落倍率**接到 loot 的既有消费点上。
//
// # 为什么在"取用处"算，而不是在目录构造处算
//
// 服务端层 mod 的生效点是 `server.boot`，它跑在 `prepareRuntime` **之后**
// （见 `cmd/wireprobe/main.go` 的四步顺序注释），而 loot 目录
// （`catalog.LootCatalog`，含 `OrdinaryWorldDropPercent` / `OrdinaryMonsterItemRate`）
// 是在 `prepareRuntime` 里构造的。若把倍率在构造时乘进目录字段，
// boot 里设的规则就会**静默漏掉**（日志显示"已生效"，实际一次都没乘）。
//
// 所以倍率一律在"这一次抽取"里现读现算：`internal/loot` 导入
// `internal/modpolicy` 是叶子方向（modpolicy 只依赖 fmt/strings/sync），不成环。
// mod 在 boot 里设置后立刻生效，撤销（传零值）也立刻生效。
//
// # 零值语义
//
// 没有任何 mod 设置（倍率 0）时 `scaleDropPercent` **原样返回 base**，
// 连截断都不做 ⇒ 与加这条能力之前逐位相同（同一 seed 的 Awards/NextSeed 都不变，
// 见 `modpolicy_drops_test.go` 里钉死的改动前黄金值）。

// worldDropMaxPercent 是世界掉落倍率的既有上限（loot 侧 100 = 1.00 倍）；
// 与 `world_drop.go` 的校验、`threshold := total * percent / 100` 换算同口径。
const worldDropMaxPercent uint32 = 10000

// effectiveWorldDropPercent 是世界掉落的**实际**倍率 = 目录里的环境变量值 × mod 倍率。
func effectiveWorldDropPercent(c catalog.LootCatalog) uint32 {
	return scaleDropPercent(c.OrdinaryWorldDropPercent, modpolicy.Drops().WorldPercent, worldDropMaxPercent)
}

// effectiveMonsterItemRate 是小怪专属物品池的**实际**速率 = 目录里的环境变量值 × mod 倍率。
//
// 上限取 `MonsterItemDropDenominator`（10000 = 100%）：目录里存的是 /10000 口径
// ——`ParseMonsterItemDropPercent` 把 0..100 的百分比 ×100 存进来，日志也写
// `rate=%d/10000`，而 `monster_items.go` 的既有校验正是 `> MonsterItemDropDenominator` 才报错。
// 所以"上限 100"指的是**环境变量**的单位（0..100），换算到该字段就是 0..10000。
//
// ⚠️ 别再"照字面"把这个上限改成 100：默认 10% 存成 1000，一乘 5 会被截成 100（=1%），
// ×5 会变成掉率降到 1/10。上限只能跟 `MonsterItemDropDenominator`（=10000）对齐。
func effectiveMonsterItemRate(c catalog.LootCatalog) uint32 {
	return scaleDropPercent(c.OrdinaryMonsterItemRate, modpolicy.Drops().MonsterItemPercent, MonsterItemDropDenominator)
}

// scaleDropPercent 按 /100 口径做整数缩放（100 = 1.00 倍），全程 uint64 中间量避免溢出，
// 超上限时**截断到 limit**（不是溢出，也不是取模回绕）。
// multiplier == 0（没有 mod 设置）时原样返回 base —— 这就是"零值完全不改变服务端行为"。
func scaleDropPercent(base, multiplier, limit uint32) uint32 {
	if multiplier == 0 {
		return base
	}
	scaled := uint64(base) * uint64(multiplier) / 100
	if scaled > uint64(limit) {
		return limit
	}
	return uint32(scaled)
}

// ordinaryDropPoolsEnabled 报告"小怪专属池 / 世界掉落至少有一档开着"。
// 用**取用处**的实际倍率判断：base 为 0 时任何倍率仍是 0
// —— 倍率只能放大既有速率，不能凭空把一个关掉的池子打开。
func ordinaryDropPoolsEnabled(c catalog.LootCatalog) bool {
	return effectiveMonsterItemRate(c) > 0 ||
		c.WorldDrop != nil && effectiveWorldDropPercent(c) > 0
}
