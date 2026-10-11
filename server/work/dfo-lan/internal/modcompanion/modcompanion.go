// Package modcompanion 是服务端**副本同伴**的可编程入口，供服务端层 mod 使用。
//
// # 它解决的问题
//
// 城镇假人（`internal/modactor`）只能在城镇/区域的场景里站着、走着 —— 副本没有
// `(channel, town, area)` 这个场景键，玩家进本时 peer 会被 `retire`，所以"跟进去"
// 不是把假人搬过去，而是换一种表示：**副本里的同伴是 NOTI29（START_MAP）里的一行
// AIC**。
//
// 这条路径是客户端原生能力，不是本仓发明的：native `145b20dc0` 对
// `SourceIndex == 10000` 有一条独立分支 —— "creates the AIC by Template, applies
// the packet Team and enables following"。官服正是用它把剧情同伴带进下一间房
// （见 `internal/dungeon/session.go` 的 `appendMissingCompanions`）。所以"同伴跟着
// 进本、跟着走、一起打"这件事，**打斗由客户端原生 AIC 完成**，服务端只负责把这一行
// 放进每一间房的怪物表。
//
// ⚠️ 边界（写清楚，别当成缺陷）：服务端**不模拟战斗** —— 没有 HP、没有伤害、
// 没有 AI tick。同伴的技能、攻击、外观全部来自 PVF 的 `aicharacter/<模板>.aic`，
// 所以副本同伴的长相与城镇假人**不一定一致**：城镇看名册里的穿戴，副本看 AIC 资源。
//
// # 默认全关
//
// 没有任何 mod 调用 Configure 时规则是零值 ⇒ `Rows` 返回 nil，服务端行为与加这条
// 能力之前逐字节相同。这与 `internal/modpolicy` 是同一条口径。
package modcompanion

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Companion 是一个随玩家进副本的同伴。
//
// 只有两个字段，因为客户端只认这两样：Template 决定它是什么（外观/技能/等级都从
// 该 AIC 资源读），Rank 决定它用哪一档 AIC 规则（5..8，见 `protocol.StartMap`）。
type Companion struct {
	// Template 是 PVF `list/aicharacter.lst` 里的 AIC 编号（也就是 map 脚本
	// `[ai character]` 行里那个数字）。填 0 或填一个不存在的编号，客户端不会报错，
	// 只是**不出现** —— 所以 mod 侧应当先用 `AICTemplates()` 核对。
	Template uint32 `json:"template"`
	// Rank 沿用 map 脚本的 AIC 档位：5=[normal] 6=[champion] 7=[super champion]
	// 8=[boss]。0 视为 5（普通同伴）。
	Rank byte `json:"rank"`
}

// minRank / maxRank 是 `protocol.StartMap` 对 APC 行接受的档位范围（见该函数的
// `validRank`）。超出这个范围的行会被编码器拒绝，整张怪物表都发不出去。
const (
	minRank byte = 5
	maxRank byte = 8
)

// maxCompanions 是单次注册的同伴上限。
//
// 每个同伴会在**每一间房**占一个实体编号，并且会随 NOTI29 一起下发；AIC 是要客户端
// 真正实例化对象的，几十个同伴不是"人多"，是把客户端的 AIC 预算一次吃光。
const maxCompanions = 8

// Rules 是一次注册的全部内容。零值 = 全关。
type Rules struct {
	// Enabled 为假时 Rows 一律返回 nil。
	Enabled bool `json:"enabled"`
	// Companions 是进副本时注入的 AIC 行。
	Companions []Companion `json:"companions"`
	// IncludeDungeons 非空时**只**给这些副本号注入。
	IncludeDungeons []uint32 `json:"include_dungeons,omitempty"`
	// ExcludeDungeons 是永不注入的副本号，优先于 IncludeDungeons。
	ExcludeDungeons []uint32 `json:"exclude_dungeons,omitempty"`
	// Source 是设置者（mod id）。开启时必须写，避免出现"无主规则"。
	Source string `json:"source"`
}

// Configure 设置副本同伴规则并回显生效值；传零值即撤销。
//
// 只校验"作者错误"：开启规则必须署名、至少一个同伴、模板非 0 且不重复、档位在
// 5..8、数量不超上限。校验放在注册时而不是注入时，是为了让"配置写错了"在启动日志里
// 立刻可见 —— 注入发生在玩家进本那一刻，那里报错只会表现为"点了没反应"。
func Configure(r Rules) (Rules, error) {
	r.Source = strings.TrimSpace(r.Source)
	if !r.Enabled {
		// 撤销：连**全局状态**一起清掉。只返回零值而留着上一份注册，
		// 会让"关掉之后副本里还有同伴"变成一处静默的不一致。
		mu.Lock()
		current = Rules{}
		mu.Unlock()
		return Rules{}, nil
	}
	if r.Source == "" {
		return Rules{}, fmt.Errorf("modcompanion: 开启副本同伴必须写明 Source（mod id）")
	}
	if len(r.Companions) == 0 {
		return Rules{}, fmt.Errorf("modcompanion: 开启了副本同伴却没有给 companions（要么给名单，要么 enabled=false）")
	}
	if len(r.Companions) > maxCompanions {
		return Rules{}, fmt.Errorf("modcompanion: 同伴最多 %d 个（给了 %d 个）", maxCompanions, len(r.Companions))
	}
	seen := map[uint32]bool{}
	out := make([]Companion, 0, len(r.Companions))
	for _, c := range r.Companions {
		if c.Template == 0 {
			return Rules{}, fmt.Errorf("modcompanion: 同伴缺 template（AIC 编号来自 PVF list/aicharacter.lst）")
		}
		if seen[c.Template] {
			// 注入按模板去重（`appendMissingCompanions` 的判据就是模板），
			// 重复项会被静默吃掉，所以这里直接拒绝，别让作者以为"写了两个"。
			return Rules{}, fmt.Errorf("modcompanion: 同伴 AIC 模板 %d 重复", c.Template)
		}
		seen[c.Template] = true
		if c.Rank == 0 {
			c.Rank = minRank
		}
		if c.Rank < minRank || c.Rank > maxRank {
			return Rules{}, fmt.Errorf("modcompanion: AIC %d 的档位 %d 超出 %d..%d", c.Template, c.Rank, minRank, maxRank)
		}
		out = append(out, c)
	}
	r.Companions = out
	r.IncludeDungeons = dedupIDs(r.IncludeDungeons)
	r.ExcludeDungeons = dedupIDs(r.ExcludeDungeons)
	mu.Lock()
	current = r
	mu.Unlock()
	return r, nil
}

// dedupIDs 排序去重，让"同一份配置"永远回显同一个形状（日志/测试可比对）。
func dedupIDs(in []uint32) []uint32 {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[uint32]bool, len(in))
	out := make([]uint32, 0, len(in))
	for _, id := range in {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

var (
	mu      sync.RWMutex
	current Rules
)

// Current 返回当前生效的规则（零值 = 全关）。
func Current() Rules {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Enabled 报告是否有 mod 注册了副本同伴。
func Enabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return current.Enabled && len(current.Companions) > 0
}

// Reset 清空注册（测试用；生产路径只走 Configure 传零值）。
func Reset() {
	mu.Lock()
	current = Rules{}
	mu.Unlock()
}

// AppliesTo 报告这个副本号该不该带同伴。
func (r Rules) AppliesTo(dungeonID uint32) bool {
	if !r.Enabled || len(r.Companions) == 0 {
		return false
	}
	for _, id := range r.ExcludeDungeons {
		if id == dungeonID {
			return false
		}
	}
	if len(r.IncludeDungeons) == 0 {
		return true
	}
	for _, id := range r.IncludeDungeons {
		if id == dungeonID {
			return true
		}
	}
	return false
}

// String 是给启动日志与诊断的一行摘要。
func (r Rules) String() string {
	if !r.Enabled || len(r.Companions) == 0 {
		return "关闭（没有 mod 设置副本同伴）"
	}
	parts := make([]string, 0, len(r.Companions))
	for _, c := range r.Companions {
		parts = append(parts, fmt.Sprintf("AIC %d/档%d", c.Template, c.Rank))
	}
	scope := "全部副本"
	if len(r.IncludeDungeons) > 0 {
		scope = fmt.Sprintf("仅副本 %v", r.IncludeDungeons)
	}
	if len(r.ExcludeDungeons) > 0 {
		scope += fmt.Sprintf("（排除 %v）", r.ExcludeDungeons)
	}
	return fmt.Sprintf("开启：%s，范围=%s ← %s", strings.Join(parts, " + "), scope, r.Source)
}
