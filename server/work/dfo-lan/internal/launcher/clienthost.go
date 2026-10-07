package launcher

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dfolan/internal/wfpisolate"
)

// This file is the Go replacement for the process-hosting half of probe.exe
// (server/work/dfo_probe_tools/probe.cpp, wmain L104-L223): isolate, launch, watch, write
// client.log, same exit codes. The isolation itself is internal/wfpisolate; this file only
// covers "start the client inside that isolation and see it through".
//
// 口径（逐条对齐 probe.cpp）：
//   - 门禁：client_dir\DFO.exe 不是普通文件 -> 退出码 3，**在写 client.log 之前**就返回
//     （probe.cpp L106-L107）；
//   - client.log 每次运行截断重写（std::wofstream::open 默认 trunc），每行是
//     "<GetTickCount64 相对毫秒> <文字>"，UTF-8 + CRLF（L26/L108）；
//   - 命令行：`"<client_dir>\DFO.exe" <payload>`（L135-L136），工作目录 = 客户端目录
//     （L142 的 lpCurrentDirectory = root），环境继承宿主；
//   - 窗口：SW_HIDE + CREATE_NO_WINDOW（L123、L126），进程先挂起、进 Job 之后才 Resume
//     （L126-L144），Job 带 JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE（L121）；
//   - 退出后 TerminateJobObject + 等 2 秒 + 读退出码 + 关句柄 + 记 SUMMARY/JOB_CLOSED
//     （L154），进程自己的退出码不改变宿主退出码 —— 宿主永远回 0（除了上面那几种失败）。
//
// **不搬**的是调试器那一半（L156-L222 的 DEBUG_EVENT 循环、API 断点、EXCEPTION 追踪）：
// 启动链从启动器出发只会用到 interactive-ui 与 trace-root-ui 两种 ui-mode，前者
// `normal=1` 完全不进调试器分支，后者只多一个 DEBUG_ONLY_THIS_PROCESS 却没有断点表
// （L137-L141 + L156/L169）。断点与异常追踪是人工排查用的入口，不属于启动链；要那些
// 能力时应当显式调用 probe.exe（回退路径一行没改）。
const (
	// clientHostMinSeconds / clientHostMaxSeconds 复刻 probe.cpp L108 的
	// std::clamp(_wtoi(argv[3]), 1, 55)：真实调用永远传 55。
	clientHostMinSeconds = 1
	clientHostMaxSeconds = 55

	// exitClientHostMissingExe 是 probe.cpp L107 的返回码 3：看不到 DFO.exe。
	exitClientHostMissingExe = 3
	// exitClientHostJobError 是 L122 的返回码 7：Job 对象建不出来。
	exitClientHostJobError = 7
	// exitClientHostCreateError 是 L142 的返回码 11：CreateProcessW 失败。
	exitClientHostCreateError = 11
	// exitClientHostAssignError 是 L143 的返回码 12：进不了 Job。
	exitClientHostAssignError = 12
)

// ClientHostOptions 是 Go 宿主的输入。位置参数与 probe.exe 的 argv 一一对应，
// 这样调用方（clientrun.go）能够用同一套实参走两条路：
//
//	probe.exe      <client_dir> <client.log> <seconds> <ui-mode> <breakpoints.txt> [payload...]
//	--host-client  <client_dir> <client.log> <seconds> <ui-mode> <breakpoints.txt> [payload...]
//
// BreakpointsFile 只占位（probe.exe 自己也不读 argv[5]，见 clientrun.go 的注释）；
// 宿主不实现断点，所以它既不读也不写。
type ClientHostOptions struct {
	// ClientDir：客户端目录（argv[1] 的绝对化结果）。
	ClientDir string
	// LogPath：client.log（argv[2]）。
	LogPath string
	// Seconds：argv[3]，夹在 1..55。
	Seconds int
	// UIMode：argv[4]（interactive-ui / trace-root-ui / normal-ui / trace-owned-ui……）。
	// 只影响日志措辞：没有调试器就没有断点。
	UIMode string
	// BreakpointsFile：argv[5]，占位保留。
	BreakpointsFile string
	// Args：argv[6:]，按原样接在 DFO.exe 后面（probe.cpp L136 只接了一个）。
	Args []string
	// Env：客户端环境（nil = 继承当前进程）。
	Env []string
	// Console：把同一行日志回显到启动器控制台；nil = 不回显。
	Console interface{ Write([]byte) (int, error) }
	// Wrapper / WrapperDLL：汉化启动宿主（localization-host.exe + localization.dll）。
	//
	// 非空时客户端**不是**由这里直接 CreateProcess，而是交给宿主：argv 变成
	// `<Wrapper> <client_dir>\DFO.exe <WrapperDLL> <payload…>`，宿主自己起客户端并注入 dll，
	// 再把 `READY <客户端 pid>` 写回 stdout（见 clientwrapper.go 的文件头）。
	Wrapper string
	// WrapperDLL 见 Wrapper。
	WrapperDLL string
	// ErrorLog：子进程 stderr 的去处（汉化宿主路径把它接到 helper.err，与 probe.exe 那一路
	// 的 stderr 去处一致）；nil = 不回显、不落盘（尾部仍会随失败错误带出来）。
	ErrorLog io.Writer
}

// HostedClient 是一个已经拉起的客户端进程。Run 收尾之后 Pid 仍然可读
// （probe.json / run.json 的 probe_pid 用它）。
type HostedClient struct {
	// Pid 是客户端进程 pid（probe.cpp 记的 ROOT_PID）。
	//
	// 经汉化启动宿主拉起时它是**宿主报回来的那个 pid**（游戏客户端），不是宿主自己的 pid。
	Pid int
	// ExitCode 是客户端自己的退出码（宿主退出码另算）。
	ExitCode int
	// JobClosed 为真表示 Job 对象已按 probe.cpp L154 收掉。
	JobClosed bool
	// Isolated 为真表示这次运行装上了 WFP 隔离（否则是"无隔离"的降级运行）。
	Isolated bool
	// IsolationNote 是隔离的结论（装上了 / 为什么没装上 / 自检结果）。
	IsolationNote string
	// LogPath 是本次写的 client.log。
	LogPath string
	// Wrapped 为真表示这次客户端是经汉化启动宿主拉起的（Pid/WrapperPID 由此而来）。
	Wrapped bool
	// WrapperPID 是汉化启动宿主自己的 pid（只有 Wrapped 为真时有意义；诊断用）。
	WrapperPID int
	// process 是还没收尾的原生进程句柄（Run 内部用）。
	process *hostProcess
}

// probeStyleLog 是 probe.cpp 的 probe_log：一行 "<tick> <文字>"。
// 时间戳用 GetTickCount64（Windows）；别的平台上用进程内的单调时钟，区别只在数字。
type probeStyleLog struct {
	file    *os.File
	started uint64
	console interface{ Write([]byte) (int, error) }
}

// 这一组类型是平台无关的接口面：Windows 上由 clienthost_windows.go 填，
// 别的平台上由 clienthost_other.go 报错（客户端只跑 Windows）。

// wfpInstallation 是一次隔离尝试的结论。Installed 为假时 Handle 必须是 nil，
// 调用方据此回退 probe.exe / 报错，**绝不**能当成"隔离已生效"。
type wfpInstallation struct {
	Installed bool
	Reason    string
	Apps      int
	Handle    wfpisolate.Handle
}

// hostNetSelfTestResult 是自检结论（Windows 上是 wfpisolate.NetSelfTest 的投影）。
type hostNetSelfTestResult struct {
	Loopback     bool
	RemoteDenied bool
	Detail       string
	Errno        int
}

// hostProcessSpec / hostProcess 是平台宿主的输入与句柄（见 clienthost_windows.go）。

// newProbeStyleLog 打开 client.log：截断、创建、写。等价于 std::wofstream::open
// （probe.cpp L108）—— 打不开时 probe 什么都不写、照常拉起客户端。
func newProbeStyleLog(path string, console interface{ Write([]byte) (int, error) }) *probeStyleLog {
	log := &probeStyleLog{started: hostTickCount(), console: console}
	if path == "" {
		return log
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return log
	}
	log.file = file
	return log
}

// line 写一行：与 probe.cpp L26 的 `probe_log << elapsed << " " << s << std::endl` 同形。
// UTF-8（probe 的 wofstream 挂了 codecvt_utf8_utf16）、CRLF（文本模式）。
func (l *probeStyleLog) line(text string) {
	if l == nil {
		return
	}
	entry := fmt.Sprintf("%d %s", hostTickCount()-l.started, text)
	if l.file != nil {
		_, _ = l.file.WriteString(entry + pythonTextNewline)
	}
	if l.console != nil {
		_, _ = l.console.Write([]byte(entry + "\n"))
	}
}

// close 关掉 client.log（幂等）。
func (l *probeStyleLog) close() {
	if l == nil || l.file == nil {
		return
	}
	_ = l.file.Close()
	l.file = nil
}

// startHostedClient 是 probe.cpp 从 Guard 装好之后那一半：拉起客户端、看护到退出、
// 写 client.log、收 Job。返回退出码就是 **probe.exe 的退出码语义**（0/3/7/11/12）。
//
// isolate 为真时先装 WFP 隔离：装不上**不**阻止启动（probe.cpp L120 的
// WFP_NOT_AVAILABLE_RUNNING_WITHOUT_ISOLATION 就是优雅降级），但结论必须写进日志与
// 返回值，绝不允许"装不上却当成隔离成功"。
func startHostedClient(options ClientHostOptions, isolate bool) (*HostedClient, int, error) {
	clientDir := options.ClientDir
	if clientDir != "" {
		clientDir = wfpRootForHost(clientDir)
	}
	target := filepath.Join(clientDir, "DFO.exe")
	// probe.cpp L106-L107：看不到 DFO.exe 就在这里返回，**不写 client.log**。
	if !regularFile(target) {
		return nil, exitClientHostMissingExe, nil
	}

	// 汉化启动宿主那一对文件（启用汉化时由启动器经环境变量下发）：同样在写 client.log 之前
	// 就先判在不在 —— 宿主缺失是"这次根本起不了带汉化的客户端"，不该留下半份现场。
	wrapper, err := resolveWrapperTargets(options.Wrapper, options.WrapperDLL)
	if err != nil {
		return nil, exitClientHostWrapperError, err
	}

	log := newProbeStyleLog(options.LogPath, options.Console)
	defer log.close()

	hosted := &HostedClient{Pid: 0, LogPath: options.LogPath}

	// 隔离：与 probe.cpp L119-L120 同样的两行结论。
	result, installErr := installHostIsolation(clientDir, isolate)
	hosted.Isolated = result.Installed
	switch {
	case result.Installed:
		// WFP_READY 那一行的 filters= 是**条数**（镜像数 × 装过滤器的层数），
		// 与 probe.exe 的 guard.filters 同口径。
		hosted.IsolationNote = fmt.Sprintf("WFP_READY %s", filterSummaryForApps(result.Apps))
		log.line(hosted.IsolationNote)
		// 自检与 probe.cpp L129-L130 同形：PASS 一行，FAILED 一行带错误码。
		selfTest := runHostNetSelfTest()
		log.line(selfTest.Detail)
		hosted.IsolationNote = selfTest.Detail
	case installErr != nil:
		hosted.IsolationNote = "WFP_NOT_AVAILABLE_RUNNING_WITHOUT_ISOLATION reason=" + installErr.Error()
		log.line(hosted.IsolationNote)
	default:
		hosted.IsolationNote = "WFP_NOT_AVAILABLE_RUNNING_WITHOUT_ISOLATION"
		log.line(hosted.IsolationNote)
	}
	// 探针在拉起客户端之前先把附加参数记一行（L136）。
	for _, arg := range options.Args {
		if arg == "" {
			continue
		}
		log.line("SYNTHETIC_TEST_ARGUMENTS " + arg)
	}
	// 记录这次走的哪条路（Go 版/无隔离），方便从 client.log 直接判断。
	log.line("HOST " + wfpIsolationLine(hosted))
	if wrapper.Enabled() {
		// 汉化那一档也要留下现场：client.log 是唯一能证明"这次到底有没有注入 dll"的地方。
		log.line("WRAPPER_HOST " + wrapper.Host + " dll=" + wrapper.DLL)
	}

	seconds := options.Seconds
	if seconds < clientHostMinSeconds {
		seconds = clientHostMinSeconds
	}
	if seconds > clientHostMaxSeconds {
		seconds = clientHostMaxSeconds
	}

	spec := hostProcessSpec{
		Target:     target,
		Args:       options.Args,
		WorkingDir: clientDir,
		Env:        options.Env,
		UIMode:     options.UIMode,
		Timeout:    time.Duration(seconds) * time.Second,
	}
	// 汉化的 stderr 尾部（失败时随错误带出来）。宿主在启动阶段短暂附加，输出很少，所以只留
	// 最后 wrapperStderrTailLimit 字节。
	var wrapperTail *wrapperStderrTail
	if wrapper.Enabled() {
		// 客户端交给汉化宿主：argv 变成 `<host> <DFO.exe> <dll> <payload…>`，宿主自己起客户端
		// 并注入 dll。它的 stdout 要读 `READY <pid>`，stderr 按 probe.exe 那一路的去处落盘。
		//
		// 目标与参数取自同一个 argv：**第一项是程序自己**（buildClientCommandLine 会把它拼成
		// 命令行第一段），所以参数只能取它后面的部分 —— 两处各拼一次会让宿主收到
		// `[host host DFO.exe dll payload]`，于是它把自己又起了一遍（2026-10-05 实测踩到）。
		argv := wrapper.Argv(target, options.Args)
		spec.Target = argv[0]
		spec.Args = argv[1:]
		spec.WantStdout = true
		if options.ErrorLog != nil {
			spec.WantStderr = true
		}
	}

	process, code, err := startHostProcess(spec)
	if err != nil {
		if process != nil && process.Pid > 0 {
			log.line(fmt.Sprintf("ROOT_PID %d", process.Pid))
		}
		log.line(fmt.Sprintf("CREATE_ERROR %v", err))
		return nil, code, err
	}
	hosted.process = process
	hosted.Pid = process.Pid
	log.line(fmt.Sprintf("ROOT_PID %d", process.Pid))
	if wrapper.Enabled() {
		// stderr 先接上（握手失败时它就是唯一的原因来源）：实时落 helper.err，同时留一份尾部。
		if process.stderr != nil {
			wrapperTail = &wrapperStderrTail{limit: wrapperStderrTailLimit}
			process.drainStderr(io.MultiWriter(options.ErrorLog, wrapperTail))
		}
		pid, readyErr := process.awaitWrapperClient(options.Console)
		if readyErr != nil {
			log.line("WRAPPER_ERROR " + readyErr.Error())
			// 先杀宿主（它 Job 里的客户端也一并收掉），再关句柄 —— closeAll 内部会等两个管道
			// 读者退出，所以下面读 wrapperTail 是安全的。
			process.terminateAndWait(2 * time.Second)
			process.closeAll()
			tail := wrapperTail.String()
			for _, line := range strings.Split(tail, "\n") {
				if line = strings.TrimRight(line, "\r"); line != "" {
					log.line("WRAPPER_STDERR " + line)
				}
			}
			if tail != "" {
				return nil, exitClientHostWrapperError, fmt.Errorf("%w；汉化启动宿主 stderr 尾部：%s", readyErr, tail)
			}
			return nil, exitClientHostWrapperError, readyErr
		}
		// 宿主报回的是**游戏客户端**的 pid：从此 watch / 退出码都盯着它（宿主可能在初始化完成后
		// 就自己退出，等它等于把一次还在跑的游戏判成已结束）。
		hosted.Wrapped = true
		hosted.WrapperPID = process.Pid
		hosted.Pid = pid
		if attachErr := process.attachClient(pid); attachErr != nil {
			// 拿不到客户端句柄（权限/反作弊保护）时**不**把启动判失败：游戏可能真的在跑。
			// 退回"等宿主进程"，并把这次降级如实写进 client.log。
			log.line(fmt.Sprintf("WRAPPER_ATTACH_ERROR %v", attachErr))
		}
		log.line(fmt.Sprintf("WRAPPER_READY client_pid=%d", pid))
	}
	// probe.cpp L145-L146 的两行：启动链只会走到 normal（没有调试器）。
	log.line("NORMAL_RUN no_debugger no_breakpoints")
	if hostUIModeIsInteractive(options.UIMode) {
		log.line("INTERACTIVE_RUN until_client_closes")
	}

	exitCode, timedOut := process.watch()
	if timedOut {
		log.line("TIMEOUT")
	}
	log.line(fmt.Sprintf("NORMAL_BEFORE_CLEANUP exit=%s", hostHexExitCode(exitCode)))
	process.terminateAndWait(2 * time.Second)
	finalCode := process.exitCode()
	hosted.ExitCode = int(finalCode)
	hosted.JobClosed = process.closeAll()
	log.line(fmt.Sprintf("SUMMARY exit=%s normal_run=1", hostHexExitCode(finalCode)))
	if hosted.JobClosed {
		log.line("JOB_CLOSED")
	}
	if result.Installed && result.Handle != nil {
		if closeErr := result.Handle.Close(); closeErr != nil {
			log.line("WFP_CLOSE_ERROR " + closeErr.Error())
		} else {
			log.line("WFP_DYNAMIC_SESSION_CLOSED")
		}
	}
	return hosted, 0, nil
}

// hostUIModeIsInteractive 复刻 probe.cpp L137：只有 interactive-ui 是"等客户端自己关"。
func hostUIModeIsInteractive(uiMode string) bool {
	return uiMode == "interactive-ui"
}

// hostHexExitCode 复刻 probe.cpp 的 hx()：退出码在日志里是 0x…（大写十六进制）。
func hostHexExitCode(code uint32) string {
	return fmt.Sprintf("0x%X", code)
}

// HostClientArityError 是 `--host-client` 参数不够时的错误（调用方回退出码 2）。
type HostClientArityError struct{ Got int }

func (e *HostClientArityError) Error() string {
	return fmt.Sprintf("--host-client 至少需要 5 个位置参数（客户端目录、client.log、秒数、ui-mode），只给了 %d 个", e.Got)
}

// ParseHostClientArgv 把 `--host-client` 后面的位置参数解析成 ClientHostOptions。
// 形参与 probe.exe 的 argv 一一对应（args[0] = client_dir … args[3] = ui-mode，
// args[4] = probe 的 breakpoints.txt 占位，args[5:] = 接给客户端的参数）。
//
// 秒数解析失败时退回 55（probe.cpp 用 _wtoi，非法输入就是 0，随后被 clamp 到 1；
// 启动链真正的调用永远传 55，所以这里取"真实调用值"作为兜底而不是 1）。
func ParseHostClientArgv(args []string) (ClientHostOptions, error) {
	if len(args) < 4 {
		return ClientHostOptions{}, &HostClientArityError{Got: len(args)}
	}
	options := ClientHostOptions{
		ClientDir: args[0],
		LogPath:   args[1],
		Seconds:   55,
		UIMode:    args[3],
	}
	if seconds, err := strconv.Atoi(args[2]); err == nil {
		options.Seconds = seconds
	}
	if len(args) > 4 {
		options.BreakpointsFile = args[4]
	}
	if len(args) > 5 {
		options.Args = append([]string{}, args[5:]...)
	}
	return options, nil
}

// RunClientHost 是 Go 宿主的对外入口（`dfolauncher --host-client` 用）。返回的就是
// probe.exe 口径的退出码；error 只有在"确实起不来"时才非 nil。
func RunClientHost(options ClientHostOptions) (int, error) {
	_, code, err := startHostedClient(options, true)
	return code, err
}

// wfpIsolationLine 是调用方写进 helper.out 的那一行，说明本次走的是哪条路。
// 两条路都要有明确记录 —— 日志是唯一能证明"这次到底有没有隔离"的地方。
func wfpIsolationLine(hosted *HostedClient) string {
	if hosted == nil {
		return ""
	}
	if hosted.Isolated {
		return "WFP 隔离：Go 版（internal/wfpisolate）；" + hosted.IsolationNote
	}
	return "WFP 隔离：未安装（Go 版不可用）；" + hosted.IsolationNote
}
