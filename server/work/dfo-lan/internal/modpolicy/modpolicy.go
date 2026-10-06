// Package modpolicy 是服务端**玩法策略**的可编程入口，供服务端层 mod 使用。
//
// # 为什么需要它
//
// mods/ 的 schema 2 里，**server 层是唯一能改服务端玩法行为的层**，而它的 Go 代码
// 由 modkit 生成 `mods/zz_mods_gen.go` 后**编进服务端二进制**（见
// `server/work/dfo-lan/mods/README.md`）。所以"一个 mod 打开一条模式规则"的正路是：
// mod 在自己的 `Register()` / `server.boot` 里调用本包，把规则设上去。
// mod.json 里没有、也不该有纯数据的玩法开关 —— 那正是根 AGENTS §0.2 与
// server/AGENTS §0「单一内容真源」「禁止把开关当数据」要避免的东西。
//
// # 默认全关
//
// 没有任何 mod 调用 Configure 时，规则都是零值 ⇒ 服务端行为与装 mod 之前一致。
// 生效状态由 `cmd/wireprobe` 在启动装配结束时打进日志（`odyssey mode rules: …`），
// 所以"规则没生效"永远看得出来，不会静默。
package modpolicy

import (
	"fmt"
	"strings"
	"sync"
)

// OdysseyRules 是奥德赛模式（`catalog.DungeonDefinition.Odyssey`，来自 DGN 的
// `[dungeon mode script] arad odyssey`）内的规则。零值 = 全部关闭。
type OdysseyRules struct {
	// BanConsumables：奥德赛副本内禁止**使用**消耗品（仍可携带；城镇不受影响）。
	// 消费点：CMD44 `useStackable` 的 `odysseyConsumableGate`。
	BanConsumables bool

	// BanReviveCoin：奥德赛内禁止复活 —— 一处挡住三级回退（奥德赛测试额度 /
	// 背包复活币 / CERA 扣费）。消费点：CMD41 `useCoinRevive` 的 `odysseyReviveGate`。
	BanReviveCoin bool

	// Source 是设置者（mod id 或配置键）：写启动日志、写拒绝原因，便于排障。
	// 开启任一规则时必填，避免出现"无主规则"。
	Source string
}

// Enabled 报告是否有规则被打开。
func (r OdysseyRules) Enabled() bool { return r.BanConsumables || r.BanReviveCoin }

// String 是给启动日志与诊断的一行摘要。
func (r OdysseyRules) String() string {
	if !r.Enabled() {
		return "关闭（没有 mod 设置模式规则）"
	}
	parts := make([]string, 0, 2)
	if r.BanConsumables {
		parts = append(parts, "副本内禁用消耗品（可携带）")
	}
	if r.BanReviveCoin {
		parts = append(parts, "禁用复活（复活币/测试额度/CERA 三档）")
	}
	src := r.Source
	if src == "" {
		src = "未署名"
	}
	return fmt.Sprintf("开启：%s ← %s", strings.Join(parts, " + "), src)
}

var (
	mu      sync.RWMutex
	odyssey OdysseyRules
)

// Configure 设置奥德赛规则并回显生效值；传零值即撤销。
// 只校验"作者错误"：开启规则必须写明来源。
func Configure(r OdysseyRules) (OdysseyRules, error) {
	r.Source = strings.TrimSpace(r.Source)
	if r.Enabled() && r.Source == "" {
		return OdysseyRules{}, fmt.Errorf("modpolicy: 开启奥德赛规则必须写明 Source（mod id 或配置键）")
	}
	mu.Lock()
	odyssey = r
	mu.Unlock()
	return r, nil
}

// Odyssey 返回当前规则快照（值拷贝，调用方可随意保存）。
func Odyssey() OdysseyRules {
	mu.RLock()
	defer mu.RUnlock()
	return odyssey
}

// Reset 清空全部规则（测试用；生产路径只走 Configure）。
func Reset() {
	mu.Lock()
	odyssey = OdysseyRules{}
	mu.Unlock()
}
