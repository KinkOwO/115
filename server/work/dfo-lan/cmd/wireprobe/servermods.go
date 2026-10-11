package main

import (
	"os"
	"path/filepath"
	"strings"
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
