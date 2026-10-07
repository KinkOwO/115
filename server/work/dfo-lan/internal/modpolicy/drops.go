package modpolicy

import (
	"fmt"
	"strings"
)

// DropRules 是**掉落倍率**规则。零值 = 不改变（等价 1.00 倍）。
//
// 单位与 loot 侧一致：**100 = 1.00 倍，500 = 5 倍**。两个字段各自是 loot 目录里
// 那个由环境变量设进去的兼容倍率的乘数：
//
//   - `WorldPercent`       × `loot.OrdinaryWorldDropPercent`（`DFO_ORDINARY_WORLD_DROP_PERCENT`）
//   - `MonsterItemPercent` × `loot.OrdinaryMonsterItemRate`（`DFO_ORDINARY_MONSTER_ITEM_DROP_PERCENT`）
//
// 服务端在这里只提供"能乘"这个**能力**：乘多少、开不开、对哪个副本/阶段生效，
// 全部由 mod 自己决定。Go 里不写"某副本掉落 ×5"这类内容（见根 AGENTS §0.2）。
//
// 消费点在 loot 的**取用处**（`internal/loot/modpolicy_drops.go`），每次抽取现读一遍，
// 所以 mod 在自己的 `server.boot` 里设置后立刻生效、撤销也立刻生效，
// 不需要重建目录、也不需要重启进程。
type DropRules struct {
	// WorldPercent 是世界掉落（`loot.OrdinaryWorldDropPercent`）的倍率：100 = 1.00 倍。
	// 实际上限按 loot 侧既有口径截断（见 `loot.worldDropMaxPercent`）。
	WorldPercent uint32

	// MonsterItemPercent 是小怪专属物品池（`loot.OrdinaryMonsterItemRate`）的倍率：
	// 100 = 1.00 倍。loot 侧该字段存的是 /10000 口径（百分比 ×100，
	// 见 `loot.ParseMonsterItemDropPercent`），所以这里的 500 表示
	// "把当前小怪专属池概率乘 5"，上限按 `loot.MonsterItemDropDenominator`（10000 = 100%）截断。
	MonsterItemPercent uint32

	// Source 是设置者（mod id 或配置键）：写启动日志、便于排障。
	// 开启任一倍率时必填，避免出现"无主规则"。
	Source string
}

// Enabled 报告是否有倍率被打开（≠ 1.00 倍）。
func (r DropRules) Enabled() bool { return r.WorldPercent != 0 || r.MonsterItemPercent != 0 }

// String 是给启动日志与诊断的一行摘要（风格同 OdysseyRules.String()）。
func (r DropRules) String() string {
	if !r.Enabled() {
		return "关闭（没有 mod 设置掉落倍率）"
	}
	parts := make([]string, 0, 2)
	if r.WorldPercent != 0 {
		parts = append(parts, fmt.Sprintf("世界掉落 ×%.2f（%d/100）", float64(r.WorldPercent)/100, r.WorldPercent))
	}
	if r.MonsterItemPercent != 0 {
		parts = append(parts, fmt.Sprintf("小怪专属物品池 ×%.2f（%d/100）", float64(r.MonsterItemPercent)/100, r.MonsterItemPercent))
	}
	src := r.Source
	if src == "" {
		src = "未署名"
	}
	return fmt.Sprintf("开启：%s ← %s", strings.Join(parts, " + "), src)
}

var drops DropRules

// ConfigureDrops 设置掉落倍率并生效；传零值即撤销（回到 1.00 倍）。
//
// 签名按业主口径固定为无返回值，所以这里**没有 error 通道**：开启倍率却没写
// Source 时不拒绝、照常生效，并在 String() 里显示"未署名"—— 启动日志一定会打印
// 这一行（`cmd/wireprobe` 的 `drop rate rules: …`），不会出现"玩法被改了却在日志里
// 查不出是谁改的"。规则来自 mod 自己的配置，服务端不代它决定任何数值。
func ConfigureDrops(r DropRules) {
	r.Source = strings.TrimSpace(r.Source)
	mu.Lock()
	drops = r
	mu.Unlock()
}

// Drops 返回当前掉落倍率快照（值拷贝，调用方可随意保存）。
func Drops() DropRules {
	mu.RLock()
	defer mu.RUnlock()
	return drops
}
