package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dfolan/internal/servermod"
)

// 本文件是服务端层 mod（四层架构的 server 层）在主程序侧的接线点。
//
// 真正的钩子宿主在 internal/servermod；mod 源码在模块的 mods/<mod-id>/ 下，
// 由 modkit install 从 mod 包的 server/ 层落位；生成的 mod 清单在同目录的
// zz_mods_gen.go（它提供 RegisterMods()）。

// serverBuildVersion 由构建脚本用 -ldflags -X main.serverBuildVersion=... 注入。
// 没注入时是 "dev"，mod 应当把 "dev" 当作"版本不确定"而不是某个具体版本。
var serverBuildVersion = "dev"

// versionString 返回给 mod 看服务端版本串。
func versionString() string { return serverBuildVersion }

// ModTickIntervalEnvVar 是操作者用来改周期调度的环境变量（单位毫秒）。
const ModTickIntervalEnvVar = "DFO_MOD_TICK_MS"

// defaultModTickInterval 是没配置时的周期。
//
// 1 秒的依据：假人是"走动"这种肉眼观察的东西，比这快没人看得出区别，
// 比这慢走起来会一顿一顿。需要更细节奏的 mod 自己在回调里按 ctx.Seq 分流，
// 而不是把全局周期调小 —— 周期是**所有 mod 共用**的。
const defaultModTickInterval = time.Second

// modTickInterval 读周期配置。写错（非数字、0、负数）时退回默认并记一行日志：
// 一个写坏的环境变量不该让服务端起不来。
func modTickInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv(ModTickIntervalEnvVar))
	if raw == "" {
		return defaultModTickInterval
	}
	ms, err := strconv.Atoi(raw)
	if err != nil || ms <= 0 {
		log.Printf("servermod: %s=%q 不是正整数毫秒，退回默认 %v", ModTickIntervalEnvVar, raw, defaultModTickInterval)
		return defaultModTickInterval
	}
	return time.Duration(ms) * time.Millisecond
}

// startModTicks 起周期调度 goroutine，返回停止函数。
//
// 一个 tick 钩子都没有时不 goroutine 白跑（HasTickHooks 为假直接返回空操作）——
// 绝大多数服根本不装这类 mod，不该为它常驻一个 timer。
//
// 周期漂移用 Ticker 而不是"睡固定时长"：回调本身要花时间，睡固定时长会让
// 周期越跑越长，而 TickContext.SinceLast 会把这个漂移如实交给 mod。
func startModTicks() func() {
	if !servermod.HasTickHooks() {
		return func() {}
	}
	interval := modTickInterval()
	log.Printf("servermod: 周期调度已启动（%s 钩子，周期 %v）", servermod.HookServerTick, interval)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var seq uint64
		last := time.Now()
		for {
			select {
			case <-stop:
				return
			case now := <-ticker.C:
				seq++
				servermod.Tick(servermod.TickContext{Seq: seq, Now: now, SinceLast: now.Sub(last)})
				last = now
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}

// serverModuleDir 返回服务端 Go 模块根（有 go.mod 且 module dfolan 的那个目录）。
//
// 为什么用位置推导而不是编译期常量：服务端二进制通常从模块根运行
// （启动器把工作目录设成模块根），也可能被复制到别处。先用工作目录判断，
// 再退回可执行文件位置（bin/wireprobe-*.exe 的上两级），最后退回相对路径。
func serverModuleDir() string {
	if wd, err := os.Getwd(); err == nil && isServerModuleDir(wd) {
		return wd
	}
	if exe, err := os.Executable(); err == nil {
		root := filepath.Dir(filepath.Dir(exe)) // bin/xxx.exe -> 模块根
		if isServerModuleDir(root) {
			return root
		}
	}
	return "."
}

// serverModsDir 返回服务端 mod 目录（模块根的 mods/）。
//
// 注意：这是**服务端源码 mod**（Go 包）与 enabled.json 的位置；Lua 奖励规则脚本
// 走 rewardScriptsDirs()，两者在 2026-10-06 之后不再是同一个目录。
func serverModsDir() string {
	return filepath.Join(serverModuleDir(), "mods")
}

// RewardScriptsEnvVar 是启动器用来**显式**指定 Lua 规则脚本目录的环境变量。
const RewardScriptsEnvVar = "DFO_REWARD_SCRIPTS_DIR"

// rewardScriptsDirs 返回 Lua 奖励规则脚本的搜索目录（按优先级，存在的才返回）。
//
// 位置口径（业主 2026-10-06 定）：规则脚本与**其它 mod 放一起** —— 直接平铺在
// **mod 库根** `<包根>/mods/`（例：C:\Game\dof\115us\115\mods\newchar_kit.lua），
// 不再进 `mods/scripts/` 子目录。包根 = 服务端模块根往上三级
// （server/work/dfo-lan → 包根）。同时兼容两个历史位置，谁在就用谁：
//
//	<包根>/mods/scripts/      2026-10-06 过渡位置
//	<模块根>/mods/scripts/    最早的位置（启动器旧版落位点）
//
// 奖励管线按**文件名**去重，同名只执行先命中的那份，所以留着旧副本不会重复发。
// 启动器还会用 RewardScriptsEnvVar 显式注入它认定的目录（最准），优先级最高。
func rewardScriptsDirs() []string {
	packRoot := filepath.Join(serverModuleDir(), "..", "..", "..")
	var out []string
	seen := map[string]bool{}
	add := func(dir string) {
		if dir == "" {
			return
		}
		clean := filepath.Clean(dir)
		if seen[clean] {
			return
		}
		if st, err := os.Stat(clean); err == nil && st.IsDir() {
			seen[clean] = true
			out = append(out, clean)
		}
	}
	// ① 启动器显式注入（它知道包根在哪）。
	if env := strings.TrimSpace(os.Getenv(RewardScriptsEnvVar)); env != "" {
		add(env)
	}
	// ② 正式位置：mod 库根，平铺 .lua。
	add(filepath.Join(packRoot, "mods"))
	// ③④ 两个历史位置（只读兼容）。
	add(filepath.Join(packRoot, "mods", "scripts"))
	add(filepath.Join(serverModuleDir(), "mods", "scripts"))
	return out
}

// firstRewardScriptsDir 返回"该把脚本放哪儿"（一个都不存在时给正式位置）。
func firstRewardScriptsDir() string {
	if env := strings.TrimSpace(os.Getenv(RewardScriptsEnvVar)); env != "" {
		return filepath.Clean(env)
	}
	return filepath.Clean(filepath.Join(serverModuleDir(), "..", "..", "..", "mods"))
}

// isServerModuleDir 判定目录是不是服务端 Go 模块根（有 go.mod 且 module dfolan）。
func isServerModuleDir(dir string) bool {
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")) == "dfolan"
		}
	}
	return false
}
