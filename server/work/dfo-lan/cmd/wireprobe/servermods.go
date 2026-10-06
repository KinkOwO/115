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

// serverModsDir 返回服务端 mod 目录（模块根的 mods/）。
//
// 为什么用位置推导而不是编译期常量：服务端二进制通常从模块根运行
// （启动器把工作目录设成模块根），也可能被复制到别处。先用工作目录判断，
// 再退回可执行文件位置（bin/wireprobe-*.exe 的上两级），最后退回相对路径。
func serverModsDir() string {
	if wd, err := os.Getwd(); err == nil && isServerModuleDir(wd) {
		return filepath.Join(wd, "mods")
	}
	if exe, err := os.Executable(); err == nil {
		root := filepath.Dir(filepath.Dir(exe)) // bin/xxx.exe -> 模块根
		if isServerModuleDir(root) {
			return filepath.Join(root, "mods")
		}
	}
	return "mods"
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
