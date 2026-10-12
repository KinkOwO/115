package main

// DFO_* 环境变量审计（2026-10-12 事故的防护网）。
//
// ★ 事故：一个**遗留在用户级环境变量**里的 `DFO_ORACLE_ETC`（整块 21 字节覆盖）把 685 面板
// 费用的真值整块盖掉，于是「改代码 / 改数值 / 重登」全都不生效，看起来像客户端 bug，
// 白查了一整天。环境变量**不在仓库里**，`grep` / `git diff` / `git status` 都查不到它。
//
// ⇒ 从此每次启动把这批开关报一行（含**启动器 profile 注入的那些** —— 它们同样是
// 「在代码之外生效」的东西，值不值得看由人判断，不该静默）。长值只打印长度（免得长 hex 刷屏），
// 并按名字排序，便于与上一次启动逐行对比。

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

// auditDebugEnv 列出当前生效的 DFO_* 环境变量（升序，长值折叠成 `<N chars>`）。
func auditDebugEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || !strings.HasPrefix(k, "DFO_") {
			continue
		}
		if len(v) > 24 {
			out = append(out, fmt.Sprintf("%s=<%d chars>", k, len(v)))
			continue
		}
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

// logDebugEnvAudit 启动时报出所有生效的 DFO_* 开关；一个都没有就保持安静。
//
// ⚠️ 收尾纪律：临时的探针用完要清掉 —— 清没清干净启动日志里一眼可见（这一行会变短或消失）。
func logDebugEnvAudit() {
	set := auditDebugEnv()
	if len(set) == 0 {
		return
	}
	log.Printf("note: %d DFO_* environment variable(s) are set in this process; they come from the "+
		"launcher profile or from the shell and they override in-code defaults. If a change "+
		"\"does not take effect\", look here first — a leftover probe value can silently win. "+
		"Clear a leftover in a NEW shell (POSIX `unset`, Windows "+
		"`[Environment]::SetEnvironmentVariable('<VAR>',$null,'User')`): %v", len(set), set)
}
