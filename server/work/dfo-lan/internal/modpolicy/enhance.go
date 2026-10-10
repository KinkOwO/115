package modpolicy

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// EnhanceRules 是**装备成长（强化 / 安全强化 / 增幅 / 锻造）成功率**的调参规则。
// 零值 = 不改变（等价于 1.00 倍、且没有覆盖值）。
//
// # 为什么它只能是"后处理"，而不能替代既有成功率表
//
// 四条成功率都有一个既有基值来源：
//
//   - 金币强化（含便携强化器）：`goldRules.Failure.PlayerMeasured.SuccessRatePercentByLevel`
//   - 安全强化：同上表（低等级段）+ `SafePathRules.StageRate` 的失败补正
//   - 增幅：`amplifyUpgradeRules.Official.SuccessRatePercentByLevel`
//   - 锻造：`refineRules.SuccessRatePercentByLevel`
//
// 它们全部来自 `configs/pvf-enhancement-policy.json` 与 `configs/refine.json`
// （服主维护的策略值 —— PVF 里**没有**成功率表，见 `reinforcement_gold.go` 文件头注释）。
// 本规则改的是"这条基值送到掷骰之前，要不要再乘一个系数、或者干脆钉一个值"，
// **不新增一套成功率定义**：Playground 里唯一的数值真源仍是上面的基表。
//
// # 单位与优先级
//
//   - `[X]Percent`：**100 = 1.00 倍**，0（或缺省）= 不改。作用域见常量注释。
//   - `Override`：逐条线的**逐等级显式值**（0..100）。命中就直接用它，**不再乘倍率**。
//   - 优先级：`Override[线][等级]` > 倍率 > 原样。结果一律 clamp 到 `0..100`
//     （掷骰是 `roll < rate` 的百分数口径，超过 100 没有意义）。
//
// # 消费点
//
// `internal/inventory/enhance_modpolicy.go`（作用在**掷骰处**，每次判定现读一遍，
// 所以 mod 在本这 Go 进程里改了之后下一次强化立刻生效，不需要重启服务端）。
type EnhanceRules struct {
	// ReinforcePercent 是**金币强化**（普通强化 / 便携强化器）的成功率倍率：100 = 1.00 倍。
	ReinforcePercent uint32

	// SafePercent 是**安全强化**的成功率倍率：100 = 1.00 倍。
	// 作用在"含失败补正之后"的最终百分数上 —— 这样 ×2 不会连 StageRate 的 cap 一起吃掉。
	SafePercent uint32

	// AmplifyPercent 是**增幅**（含便携增幅器）的成功率倍率：100 = 1.00 倍。
	AmplifyPercent uint32

	// RefinePercent 是**锻造**的成功率倍率：100 = 1.00 倍。
	RefinePercent uint32

	// Override 是逐线逐等级的显式成功率，键是 EnhanceKind、内层键是强化等级的字符串。
	// 命中即采用（不再乘倍率）；值必须是 0..100，越界的条目在 Configure 时整份判失败。
	Override map[string]map[string]int

	// Source 是设置者（mod id 或配置键）：写启动日志、便于排障。
	// 开启任一规则时必填，避免出现"无主规则"。
	Source string
}

// enhanceKinds 是本能力认识的全部线性别。写它之外的 key = 配置写错了。
var enhanceKinds = []string{EnhanceKindReinforce, EnhanceKindSafe, EnhanceKindAmplify, EnhanceKindRefine}

// EnhanceKind 是一条装备成长线的名字（写进 Override 的键）。
const (
	// EnhanceKindReinforce 金币强化（普通强化 / 便携强化器）。
	EnhanceKindReinforce = "reinforce"
	// EnhanceKindSafe 安全强化。
	EnhanceKindSafe = "safe"
	// EnhanceKindAmplify 增幅（含便携增幅器）。
	EnhanceKindAmplify = "amplify"
	// EnhanceKindRefine 锻造。
	EnhanceKindRefine = "refine"
)

// enhancePercentFor 取某条线的倍率。
func (r EnhanceRules) enhancePercentFor(kind string) uint32 {
	switch kind {
	case EnhanceKindReinforce:
		return r.ReinforcePercent
	case EnhanceKindSafe:
		return r.SafePercent
	case EnhanceKindAmplify:
		return r.AmplifyPercent
	case EnhanceKindRefine:
		return r.RefinePercent
	}
	return 0
}

// Enabled 报告是否有任何一条线被打开（≠ 1.00 倍或有覆盖值）。
func (r EnhanceRules) Enabled() bool {
	if r.ReinforcePercent != 0 || r.SafePercent != 0 || r.AmplifyPercent != 0 || r.RefinePercent != 0 {
		return true
	}
	for _, levels := range r.Override {
		if len(levels) > 0 {
			return true
		}
	}
	return false
}

// Validate 校验"作者错误"：未知线别、越界倍率、越界显式值、无名来源。
//
// 与掉落倍率不同：这里**允许**失败并返回 error —— 成功率一旦写错，玩家会在
// 完全不知情的情况下损失材料与金币，宁可起不来也不要悄悄用错值。
func (r EnhanceRules) Validate() error {
	if r.Source == "" && r.Enabled() {
		return fmt.Errorf("modpolicy: 开启装备成长成功率调整必须写明 Source（mod id 或配置键）")
	}
	for kind, levels := range r.Override {
		if !isEnhanceKind(kind) {
			return fmt.Errorf("modpolicy: 未知的装备成长线别 %q（可用：%s）", kind, strings.Join(enhanceKinds, " / "))
		}
		for level, percent := range levels {
			if percent < 0 || percent > enhanceMaxPercent {
				return fmt.Errorf("modpolicy: %s 第 %s 级的成功率 %d 超出 0..%d", kind, level, percent, enhanceMaxPercent)
			}
		}
	}
	for _, kind := range enhanceKinds {
		if p := r.enhancePercentFor(kind); p > enhanceMaxPercentMultiplier {
			return fmt.Errorf("modpolicy: %s 的倍率 %d 超出上限 %d（100 = 1.00 倍）", kind, p, enhanceMaxPercentMultiplier)
		}
	}
	return nil
}

func isEnhanceKind(kind string) bool {
	for _, k := range enhanceKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// enhanceMaxPercent 是掷骰用百分数的上限（100% = 必成）。
const enhanceMaxPercent = 100

// enhanceMaxPercentMultiplier 是倍率上限：1000 = 10 倍，与掉落倍率同一口径。
// 成功率本身会被 clamp 到 0..100，超过 10 倍只是让更多等级顶到 100，没有现实意义。
const enhanceMaxPercentMultiplier uint32 = 1000

// SuccessPercent 算某条线某等级的**实际**成功率。
//
// 优先级：Override 命中 → 倍率 → 原样（base）。
// 零值规则（没有任何倍率也没有覆盖）时**逐位返回 base**，不做任何乘法与截断，
// 这样"没有 mod"和"有 mod 但全关"两种情况下服务端行为与加这条能力之前完全一致。
func (r EnhanceRules) SuccessPercent(kind string, level, base int) int {
	if !r.Enabled() {
		return base
	}
	if levels := r.Override[kind]; levels != nil {
		if v, ok := levels[fmt.Sprint(level)]; ok {
			return clampPercent(v)
		}
	}
	multiplier := r.enhancePercentFor(kind)
	if multiplier == 0 || base <= 0 {
		// base <= 0 时乘任何倍数都是 0 —— "倍率只能放既有成功率，不能凭空开池"。
		return clampPercent(base)
	}
	// 全程 uint64 中间量避免溢出；负值（异常基值）按 0 处理，不让它绕过 clamp。
	scaled := uint64(base) * uint64(multiplier) / 100
	if scaled > enhanceMaxPercent {
		scaled = enhanceMaxPercent
	}
	return int(scaled)
}

func clampPercent(v int) int {
	if v < 0 {
		return 0
	}
	if v > enhanceMaxPercent {
		return enhanceMaxPercent
	}
	return v
}

// String 是给启动日志与诊断的一行摘要（风格同 DropRules.String()）。
func (r EnhanceRules) String() string {
	if !r.Enabled() {
		return "关闭（没有 mod 调整装备成长成功率）"
	}
	parts := make([]string, 0, 5)
	for _, kind := range enhanceKinds {
		if p := r.enhancePercentFor(kind); p != 0 {
			parts = append(parts, fmt.Sprintf("%s ×%.2f（%d/100）", enhanceKindLabel(kind), float64(p)/100, p))
		}
		if n := len(r.Override[kind]); n > 0 {
			levels := make([]string, 0, n)
			for level := range r.Override[kind] {
				levels = append(levels, level)
			}
			sort.Strings(levels)
			parts = append(parts, fmt.Sprintf("%s 覆盖 %d 级（%s）", enhanceKindLabel(kind), n, strings.Join(levels, ",")))
		}
	}
	src := r.Source
	if src == "" {
		src = "未署名"
	}
	return fmt.Sprintf("开启：%s ← %s", strings.Join(parts, " + "), src)
}

func enhanceKindLabel(kind string) string {
	switch kind {
	case EnhanceKindReinforce:
		return "强化"
	case EnhanceKindSafe:
		return "安全强化"
	case EnhanceKindAmplify:
		return "增幅"
	case EnhanceKindRefine:
		return "锻造"
	}
	return kind
}

var (
	enhanceMu sync.RWMutex
	enhance   EnhanceRules
)

// ConfigureEnhance 设置装备成长成功率规则并生效；传零值即撤销。
//
// 与 ConfigureDrops 不同，它有 error 通道：规则写错值直接拒绝，
// 调用方（mod 的自检）应当让服务端起不来，而不是带着一个错的成功率开门营业。
func ConfigureEnhance(r EnhanceRules) (EnhanceRules, error) {
	r.Source = strings.TrimSpace(r.Source)
	if err := r.Validate(); err != nil {
		return EnhanceRules{}, err
	}
	enhanceMu.Lock()
	enhance = r
	enhanceMu.Unlock()
	return r, nil
}

// Enhance 返回当前规则快照（值拷贝，调用方可随意保存）。
//
// 注意：`SuccessPercent` 在快照上调，不要在持锁路径上算 —— 掷骰处每次拿一份快照，
// 这样"读到一半被另一个 goroutine 改掉"不可能发生。
func Enhance() EnhanceRules {
	enhanceMu.RLock()
	defer enhanceMu.RUnlock()
	return enhance
}
