package launcher

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// This file is the "localization host wrapper" channel between the launcher and this
// orchestrator (2026-10-05): the client is no longer exec'd by the launcher itself, so the
// only way to inject the localization dll is to let **this** program hand the client to the
// launcher's localization host instead of starting DFO.exe directly.
//
// 载体是**环境变量**（启动器下发，这里继承）：
//
//	DFO_CLIENT_WRAPPER      = <客户端 .launcher-mods\localization\localization-host.exe>
//	DFO_CLIENT_WRAPPER_DLL  = <同一个目录下的 localization.dll>
//
// 为什么用环境变量而不是命令行开关：本程序的 argv、client.log 行、退出码与 run.json /
// probe.json 字段都是两个仓库之间钉住的既有契约（见 clientrun.go 顶部），多一个开关就要动
// 参数解析与版本对齐；环境变量是启动器本来就继承给它的通道，对既有契约零影响。
//
// 两个变量都在时，客户端那一步变成：
//
//	<localization-host.exe> <client_dir>\DFO.exe <localization.dll> <payload…>
//
// 宿主自己起客户端并注入 dll，然后把 `READY <游戏客户端 pid>` 写到自己的 stdout 上
// （与启动器 modtool.StartClient 同一份协议：前缀 + 空格 + 十进制 pid）。我们读到的那个 pid
// 就是本次会话的客户端 pid，宿主自己的 pid 另算。
//
// 硬约束（都在下面用代码表达）：
//   - 包装变量存在时**必须**走 Go 宿主：probe.exe 是原生程序，没有"把被拉起的程序换成宿主"
//     这种能力，静默用它等于起一个没汉化的客户端；Go 宿主这条路不可用时**明确报错**；
//   - 等 READY 的上限短且有界（clientWrapperReadyTimeout），超时就杀宿主并把它的 stderr 尾部
//     带进日志与错误 —— 绝不能让"宿主挂住"变成"点开始游戏没反应"；
//   - 宿主报回的 pid 就是客户端 pid：run.json / probe.json 与日志里凡是记录客户端 pid 的地方
//     都用它。

// 两个环境变量的键名。启动器侧的对应常量在 internal/run/run.go（clientWrapperEnvKey /
// clientWrapperDLLEnvKey），改名必须两边同时改。
const (
	clientWrapperEnvKey    = "DFO_CLIENT_WRAPPER"
	clientWrapperDLLEnvKey = "DFO_CLIENT_WRAPPER_DLL"
)

const (
	// clientWrapperReadyTimeout 是"等汉化宿主报出 READY <pid>"的上限。
	//
	// 必须有界：宿主是启动器换上来的一个可执行文件，它挂住就等于整次启动挂住（界面上是
	// "点了开始游戏没反应"）。30 秒足够覆盖"宿主加载 dll 并起客户端"这段；启动器侧等同一行
	// 握手用的是同一个值（modtool 的 wrapperReadyTimeout）。
	clientWrapperReadyTimeout = 30 * time.Second

	// exitClientHostWrapperError 是"汉化宿主没能报出客户端 pid"（READY 超时/无效）的返回码。
	// probe.cpp 没有用过 13；这一档只在启用汉化时可能出现，且调用方会据此直接报错，
	// 不会把它写进 probe.json（那里仍然是 probe.exe 的退出码语义）。
	exitClientHostWrapperError = 13

	// wrapperStderrTailLimit 是失败时随错误带上来的宿主 stderr 尾部字节数。
	wrapperStderrTailLimit = 2048
)

// clientWrapper 是"把汉化 dll 注入客户端"的包装配置。
type clientWrapper struct {
	// Host 是汉化启动宿主（localization-host.exe）的绝对路径。
	Host string
	// DLL 是要注入客户端的汉化 dll（localization.dll）的绝对路径。
	DLL string
}

// Enabled 报告这次要不要走汉化宿主。半套配置（只有一个变量）在 fromEnv 里就被挡住，
// 所以这里"有一个就算启用"只用于错误路径的表达。
func (w clientWrapper) Enabled() bool { return w.Host != "" || w.DLL != "" }

// Argv 是汉化宿主自己的 argv：`<host> <DFO.exe> <localization.dll> <payload…>`。
//
// 位置参数与启动器 modtool.StartClient 改写出来的那一份逐项一致（宿主只认这个顺序）：
// 宿主自己起 DFO.exe、把 dll 注入进去，payload 原样转交给客户端。
func (w clientWrapper) Argv(clientExe string, args []string) []string {
	argv := make([]string, 0, len(args)+3)
	argv = append(argv, w.Host, clientExe, w.DLL)
	return append(argv, args...)
}

// clientWrapperFromEnv 读启动器下发的包装变量。两个都没有 = 没启用汉化（返回零值，不报错）；
// 只有一个 = 半套配置，**明确报错**（静默按"没启用"处理会起一个没汉化的客户端，
// 而那正是这次要消灭的现象）。
func clientWrapperFromEnv(env *childEnv) (clientWrapper, error) {
	if env == nil {
		return clientWrapper{}, nil
	}
	host := strings.TrimSpace(env.Get(clientWrapperEnvKey))
	dll := strings.TrimSpace(env.Get(clientWrapperDLLEnvKey))
	switch {
	case host == "" && dll == "":
		return clientWrapper{}, nil
	case host == "" || dll == "":
		return clientWrapper{}, fmt.Errorf(
			"汉化注入配置不完整（%s=%q，%s=%q）：两个变量必须同时给；"+
				"请在启动器「MOD 工具」页重新启用一次汉化，或先关闭汉化再启动",
			clientWrapperEnvKey, host, clientWrapperDLLEnvKey, dll)
	}
	return clientWrapper{Host: host, DLL: dll}, nil
}

// clientLaunchRoute 是"这次客户端谁拉"的分流结果。
type clientLaunchRoute struct {
	// Wrapper 非零表示经汉化启动宿主拉起客户端。
	Wrapper clientWrapper
	// UseProbe 为真表示走 probe.exe 回退（只有没有包装变量时才可能为真）。
	UseProbe bool
}

// planClientLaunch 决定这次客户端怎么拉，并把"包装变量带来的硬约束"表达成明确错误。
//
//   - 有包装变量（启用汉化）：**必须**走 Go 宿主（UseProbe 恒为假）。被 DFO_FORCE_PROBE_EXE
//     要求回退时直接报错 —— probe.exe 是原生程序，没有"把被拉起的程序换成宿主"的能力，
//     顺着它走只会得到"汉化看起来开着、文本却不生效"；
//   - 没有包装变量：与既有行为逐字一致（DFO_FORCE_PROBE_EXE 走 probe.exe，否则首选 Go 宿主，
//     装不上隔离再按原有规则回退）。
func planClientLaunch(env *childEnv) (clientLaunchRoute, error) {
	wrapper, err := clientWrapperFromEnv(env)
	if err != nil {
		return clientLaunchRoute{}, err
	}
	forced := forceProbeFallback(env)
	if wrapper.Enabled() && forced {
		return clientLaunchRoute{}, fmt.Errorf(
			"已启用汉化（%s=%s），但 %s=1 要求走 probe.exe：probe.exe 是原生程序，"+
				"无法把客户端交给汉化启动宿主（%s 注入不了）。请取消 %s，或在启动器里先关闭汉化再启动。",
			clientWrapperEnvKey, wrapper.Host, probeFallbackEnvKey, clientWrapperDLLEnvKey, probeFallbackEnvKey)
	}
	return clientLaunchRoute{Wrapper: wrapper, UseProbe: forced}, nil
}

// wrapperHostFailure 造出"包装变量存在、但这次没能用汉化宿主拉起客户端"的明确错误。
//
// 这里**不**回退 probe.exe（原因见文件头）：回退只会静默给出一个没汉化的客户端，
// 所以宁可直接失败并把三件事说清 —— 原因、宿主路径、客户端目录。
func wrapperHostFailure(wrapper clientWrapper, clientDir string, cause error) error {
	reason := "Go 客户端宿主不可用"
	if cause != nil {
		reason = cause.Error()
	}
	return fmt.Errorf("已启用汉化，但这次无法用汉化启动宿主拉起客户端（不回落 probe.exe：它无法注入 dll）：%s\n"+
		"汉化宿主：%s\n客户端目录：%s\n"+
		"可在启动器「MOD 工具」页重新启用一次汉化（会补齐资源），或先关闭汉化再启动。",
		reason, wrapper.Host, clientDir)
}

// resolveWrapperTargets 校验汉化启动宿主与 dll 都在盘上，返回它们；两个都没给时返回零值。
//
// 启动器在启动前已经把这些文件备好了（见启动器 internal/modtool.EnsureWrapper），这里再核一遍
// 是因为本程序也可能被别的入口直接调用（两条路线脚本 / 手工命令行）：宿主缺一个文件就**明确
// 报错**，绝不"看起来启用了汉化、其实什么也没注入"。
func resolveWrapperTargets(host, dll string) (clientWrapper, error) {
	wrapper := clientWrapper{Host: strings.TrimSpace(host), DLL: strings.TrimSpace(dll)}
	if !wrapper.Enabled() {
		return clientWrapper{}, nil
	}
	if wrapper.Host == "" || wrapper.DLL == "" {
		return clientWrapper{}, fmt.Errorf("汉化注入配置不完整（宿主=%q，dll=%q）：两个都要给；"+
			"请在启动器「MOD 工具」页重新启用一次汉化，或先关闭汉化再启动", wrapper.Host, wrapper.DLL)
	}
	for _, path := range []string{wrapper.Host, wrapper.DLL} {
		if !regularFile(path) {
			return clientWrapper{}, fmt.Errorf("汉化启动宿主文件不存在：%s；"+
				"请在启动器「MOD 工具」页重新启用一次汉化（会补齐资源），或先关闭汉化再启动", path)
		}
	}
	return wrapper, nil
}

// parseWrapperReady 解析汉化宿主写在自己 stdout 上的那一行：`READY <游戏客户端 pid>`。
// 前缀与启动器 modtool.StartClient 一样是大小写敏感的 ASCII（那是既有协议，不在这里放宽）。
func parseWrapperReady(line string) (int, error) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "READY ") {
		if trimmed == "" {
			return 0, fmt.Errorf("汉化启动宿主没有报出 READY <pid>（stdout 是空的）")
		}
		return 0, fmt.Errorf("汉化启动宿主没有报出 READY <pid>：%q", trimmed)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(trimmed, "READY ")))
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("汉化启动宿主报出的客户端 pid 无效：%q", trimmed)
	}
	return pid, nil
}

// awaitWrapperReady 从汉化宿主的 stdout 读一行 READY，上限 timeout；宿主的其余 stdout 原样
// 转发给 sink（与 probe.exe 那一路的去处一致：helper.out）。
//
// 返回的 pid 是**游戏客户端**的 pid（不是宿主自己的）。返回的 done 在读者 goroutine 退出后
// 关闭：调用方在杀宿主、关句柄之前必须等它 —— 对着一个还挂着读操作的句柄关句柄是未定义行为。
func awaitWrapperReady(stdout io.Reader, sink io.Writer, timeout time.Duration) (int, <-chan struct{}, error) {
	type outcome struct {
		pid int
		err error
	}
	done := make(chan struct{})
	ready := make(chan outcome, 1)
	go func() {
		defer close(done)
		reader := bufio.NewReaderSize(stdout, 4096)
		line, err := reader.ReadString('\n')
		pid := 0
		if err == nil {
			pid, err = parseWrapperReady(line)
		}
		ready <- outcome{pid: pid, err: err}
		if sink != nil {
			// 握手之后的 stdout 不再有含义，但它是宿主的输出，照既有规则落进 helper.out。
			_, _ = io.Copy(sink, reader)
		}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case got := <-ready:
		if got.err != nil {
			return 0, done, got.err
		}
		return got.pid, done, nil
	case <-timer.C:
		return 0, done, fmt.Errorf("汉化启动宿主在 %s 内没有报出 READY <pid>", timeout)
	}
}

// awaitReader 等一个"父子管道读者"收尾（有界）。超时不是错误：调用方只是要避免在读者还挂在
// 句柄上时关句柄，等不到就照旧往下走（真的卡住的读者由进程退出收拾）。
func awaitReader(done <-chan struct{}, timeout time.Duration) {
	if done == nil {
		return
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
}

// wrapperStderrTail 保留子进程 stderr 的最后 N 字节：宿主只往 stdout 报一行握手结果，
// 它为什么没报只能看 stderr —— 失败时要把它带进 client.log 与错误信息。
//
// 写入发生在转发 goroutine 里、读取发生在收尾（那两个 goroutine 已经退出）之后，
// 所以调用方必须**先**关管道读者再读它（见 startHostedClient 的失败分支）。
type wrapperStderrTail struct {
	buf   []byte
	limit int
}

func (t *wrapperStderrTail) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if t.limit > 0 && len(t.buf) > t.limit {
		t.buf = t.buf[len(t.buf)-t.limit:]
	}
	return len(p), nil
}

func (t *wrapperStderrTail) String() string {
	if t == nil {
		return ""
	}
	return strings.TrimSpace(string(t.buf))
}
