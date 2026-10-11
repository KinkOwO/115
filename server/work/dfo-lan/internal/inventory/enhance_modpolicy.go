package inventory

import (
	"dfolan/internal/modpolicy"
)

// 这里把 mod 的**装备成长成功率**接到四条既有的掷骰消费点上。
//
// # 为什么又是一个"取用处"薄层
//
// 与 `internal/loot/modpolicy_drops.go` 同一套道理：服务端层 mod 的生效点是
// `server.boot`，它跑在 `prepareRuntime` **之后**，而四条成功率表的加载都在更早的
// 装配里完成。若在加载时就把倍率乘进表，boot 里设的规则会静默漏掉。
// 所以一律在"这一次掷骰"里现读 `modpolicy.Enhance()`。副作用是它天然支持热更新：
// mod 改了配置之后，下一位玩家点强化就生效，不需要重启服务端。
//
// # 零值语义（重要）
//
// 没有任何 mod 设置规则时 `modpolicy.EnhanceRules.SuccessPercent` 原样返回 base，
// 连 clamping 都不做 ⇒ 与加这条能力之前**逐位相同**（同一 seed 的结果不许变，
// 这一条由 `enhance_modpolicy_test.go` 钉死）。
//
// # 它不是第二套成功率表
//
// 基值仍然来自各自的既有来源（PVF 无关、`configs/pvf-enhancement-policy.json` 与
// `configs/refine.json` 的服主策略段）。这里只做"后处理"：乘一个系数，或者按
// mod 给的逐等级表取一个显式值。Go 里不写"某等级应该是百分之几"这类内容。

// EnhanceKind 沿用 modpolicy 的四个线别常量，落在本包名下方便调用点少写一串 import。
const (
	enhanceKindReinforce = modpolicy.EnhanceKindReinforce
	enhanceKindSafe      = modpolicy.EnhanceKindSafe
	enhanceKindAmplify   = modpolicy.EnhanceKindAmplify
	enhanceKindRefine    = modpolicy.EnhanceKindRefine
)

// enhanceSuccessPercent 是四条线唯一的收口：把"表里的基值"交给当前 mod 规则，
// 得到本次掷骰真正使用的百分数。
//
// base 是既有成功率（0..100），level 是当前强化等级（安全强化的补正已经算在 base 里）。
// 返回值已 clamp 到 0..100，调用方可以直接 `roll < rate` 使用。
func enhanceSuccessPercent(kind string, level, base int) int {
	return modpolicy.Enhance().SuccessPercent(kind, level, base)
}
