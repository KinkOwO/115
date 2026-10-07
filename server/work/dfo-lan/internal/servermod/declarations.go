package servermod

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 本文件实现「**声明与注册的一致性核对**」。
//
// # 为什么需要它
//
// modkit 侧只校验"钩子名在 HookPoints 白名单里"（internal/modkit/manifest2.go），
// 安装期只保证"这个 mod 包能编译"（install2.go 的 verifyServerBuilds）。
// 于是一种最坏的失败是**完全静默**的：mod 在 mod.json 里声明了 server.boot，
// 却在 Go 里忘了调 RegisterBoot —— 装成功、编成功、启动成功，**什么也没发生**。
// 玩家和操作者都看不出区别，只有把 mod 源码读一遍才发现。
//
// 所以服务端在启动时按 mods/<id>/mod.json **反查**一遍：声明了就必须真的注册了。
// 口径沿用既有的 fail-closed：宁可起不来，也不要静默半残（见 host.go 的 Boot 注释）。
// 唯一的"关掉"方式是把该 mod 写进 mods/enabled.json —— 那本来就是停用 mod 的正规入口，
// 而且它由启动器的「MOD 工具」页管理，不需要服务端能起来。

// DeclarationProblem 描述一个 mod 的声明与注册不一致。
type DeclarationProblem struct {
	ModID string
	// Missing 是"声明了却没注册"：该钩子永远不会被调用，mod 的这部分是死的。
	Missing []string
	// Extra 是"注册了却没声明"：功能上能用，但 mod.json 少写，审计/权限口径对不上。
	Extra []string
}

// String 生成一行可读的问题描述（供启动日志）。
func (p DeclarationProblem) String() string {
	parts := make([]string, 0, 2)
	if len(p.Missing) > 0 {
		parts = append(parts, "声明了却没注册："+strings.Join(p.Missing, ", "))
	}
	if len(p.Extra) > 0 {
		parts = append(parts, "注册了却没声明："+strings.Join(p.Extra, ", "))
	}
	return fmt.Sprintf("mod %s：%s", p.ModID, strings.Join(parts, "；"))
}

// RegisteredHooks 返回某个 mod **实际**注册的全部钩子点（稳定排序）。
//
// reward.script 没有独立的注册函数（它由 RegisterRewardScript 满足），
// 所以这里按"该 mod 有没有登记过奖励规则脚本"来判定。
func RegisteredHooks(modID string) []string {
	seen := map[string]bool{}

	mu.RLock()
	for _, b := range boots {
		if b.modID == modID {
			seen[HookBoot] = true
		}
	}
	for _, c := range consoles {
		if c.modID == modID {
			seen[HookConsoleCommand] = true
		}
	}
	for _, r := range responses {
		if r.modID == modID {
			seen[HookProtocolResponse] = true
		}
	}
	for _, r := range requests {
		if r.modID == modID {
			seen[HookProtocolRequest] = true
		}
	}
	mu.RUnlock()

	// 脚本注册表用的是自己的锁（见 scripts.go 的 scriptMu）。
	scriptMu.RLock()
	for _, s := range modScripts {
		if s.ModID == modID {
			seen[HookRewardScript] = true
			break
		}
	}
	scriptMu.RUnlock()

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// modDeclaration 是 mod.json 里本核对需要的那一片。
//
// 刻意只解析这两个字段：modkit 的清单是 schema 2 的完整对象，而服务端只需要
// "这个 mod 声明了哪些钩子点"。用 DisallowUnknownFields 之外的全宽松解析，
// 未来 modkit 加字段不会让服务端解析失败。
type modDeclaration struct {
	ID     string `json:"id"`
	Layers struct {
		Server *struct {
			Hooks []struct {
				Name string `json:"name"`
			} `json:"hooks"`
		} `json:"server"`
	} `json:"layers"`
}

// declaredHooks 读一个 mod 目录下的 mod.json，返回它的 id 与声明的钩子点。
//
// 目录里没有 mod.json 时返回 ok=false：那不是"服务端源码 mod"。
// modkit 只给**带 hooks 的** mod 落这份文件（install2.go 的 applyServerGoPackage），
// 只带 Lua 脚本的 mod 不建目录。
func declaredHooks(modDir string) (modID string, hooks []string, ok bool, err error) {
	raw, readErr := os.ReadFile(filepath.Join(modDir, "mod.json"))
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return "", nil, false, nil
		}
		return "", nil, false, readErr
	}
	var decl modDeclaration
	if unmarshalErr := json.Unmarshal(raw, &decl); unmarshalErr != nil {
		return "", nil, false, fmt.Errorf("解析 %s 失败：%w", filepath.Join(modDir, "mod.json"), unmarshalErr)
	}
	if decl.Layers.Server == nil {
		return decl.ID, nil, true, nil
	}
	for _, h := range decl.Layers.Server.Hooks {
		if name := strings.TrimSpace(h.Name); name != "" {
			hooks = append(hooks, name)
		}
	}
	return decl.ID, hooks, true, nil
}

// CheckDeclarations 核对 modsDir 下每个"带 mod.json 的 mod 目录"的声明与注册。
//
// 被禁用的 mod 直接跳过：它的 Register() 本来就会在开头 return，
// 不注册任何钩子是**预期行为**，不该报成问题（见 enabled.go 的口径）。
//
// 返回的问题按 mod id 稳定排序；错误只来自"读得到文件但解析不了"这类硬故障。
func CheckDeclarations(modsDir string) ([]DeclarationProblem, error) {
	if strings.TrimSpace(modsDir) == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var problems []DeclarationProblem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		modDir := filepath.Join(modsDir, e.Name())
		modID, declared, ok, declErr := declaredHooks(modDir)
		if declErr != nil {
			return nil, declErr
		}
		if !ok {
			continue
		}
		if modID == "" {
			modID = e.Name()
		}
		if !Enabled(modID) {
			continue
		}
		actual := RegisteredHooks(modID)
		missing := difference(declared, actual)
		extra := difference(actual, declared)
		if len(missing) == 0 && len(extra) == 0 {
			continue
		}
		problems = append(problems, DeclarationProblem{ModID: modID, Missing: missing, Extra: extra})
	}
	sort.Slice(problems, func(i, j int) bool { return problems[i].ModID < problems[j].ModID })
	return problems, nil
}

// difference 返回 a 里有、b 里没有的元素（去重 + 稳定排序）。
func difference(a, b []string) []string {
	in := make(map[string]bool, len(b))
	for _, s := range b {
		in[s] = true
	}
	seen := make(map[string]bool, len(a))
	out := make([]string, 0, len(a))
	for _, s := range a {
		if in[s] || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
