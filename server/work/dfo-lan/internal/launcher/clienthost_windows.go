//go:build windows

package launcher

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"dfolan/internal/wfpisolate"
)

// 这个文件是 Go 宿主的 Windows 原生部分：CreateProcessW（挂起起）+ Job Object
// （kill-on-close）+ 保活看护，逐条对齐 probe.cpp L121-L154。
//
// 为什么用原生调用而不是 os/exec：probe 的语义要求「先挂起 → 进 Job → 再 Resume」
// （L126-L144），而 os/exec 不暴露 thread handle，且它内部已经有一个 goroutine 在
// Wait。原生调用能把挂起/恢复/收尸的顺序写得和 probe 一模一样。
//
// 环境块用 syscall 的 CreateProcess 传（x/sys/windows 的封装签名里没有它），
// 其余进程/Job API 走 x/sys/windows。

var procGetTickCount64 = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetTickCount64")

// createUnicodeEnvironment 是 CreateProcessW 的 CREATE_UNICODE_ENVIRONMENT(0x400)。
// x/sys/windows 里常量名随版本变动，这里显式写死数值，避免依赖具体版本导出了哪个名字。
const createUnicodeEnvironment uint32 = 0x00000400

// hostTickCount 复刻 probe.cpp 用的 GetTickCount64（毫秒，系统启动起算）。
func hostTickCount() uint64 {
	millis, _, _ := procGetTickCount64.Call()
	return uint64(millis)
}

// wfpRootForHost 与 wfpisolate.NormalizeRoot 同口径（绝对化 + Clean）：
// 宿主与隔离器必须对"客户端目录"给出同一个答案。
func wfpRootForHost(path string) string { return wfpisolate.NormalizeRoot(path) }

// installHostIsolation 调 wfpisolate 装隔离。want=false 时直接返回"没装"（不报错）。
//
// 返回的 error 只在「确实尝试过、但失败了」时非 nil；调用方必须把它当失败上报，
// 绝不能当成隔离成功（这是安全底线）。
func installHostIsolation(clientDir string, want bool) (wfpInstallation, error) {
	if !want {
		return wfpInstallation{Reason: "调用方要求跳过 Go 隔离"}, nil
	}
	result, err := wfpisolate.Install(wfpisolate.Spec{Root: clientDir, WalkRoot: true})
	installed := wfpInstallation{
		Installed: result.Installed,
		Reason:    result.Reason,
		Apps:      result.Apps,
		Handle:    result.Handle,
	}
	return installed, err
}

// filterSummaryForApps 是 client.log 里 WFP_READY 那一行的后半段。
func filterSummaryForApps(apps int) string { return wfpisolate.FilterSummary(apps) }

// runHostNetSelfTest 是 probe.cpp L125-L130 自检的 Go 版。
func runHostNetSelfTest() hostNetSelfTestResult {
	probe := wfpisolate.RunNetSelfTest()
	return hostNetSelfTestResult{
		Loopback:     probe.Loopback,
		RemoteDenied: probe.RemoteDenied,
		Detail:       probe.Detail,
		Errno:        probe.Errno,
	}
}

// hostProcessSpec 是拉起一个客户端进程的全部输入。
type hostProcessSpec struct {
	Target     string   // <client_dir>\DFO.exe
	Args       []string // 接在 DFO.exe 后面的参数（payload）
	WorkingDir string   // lpCurrentDirectory = 客户端目录（probe.cpp L142）
	Env        []string // nil = 继承宿主
	UIMode     string
	// Timeout 是"等多少秒"（probe.cpp argv[3] 夹出来的 55）。
	Timeout time.Duration
	// WantStdout 为真时给子进程接一根匿名管道当 stdout：汉化启动宿主路径要读它的
	// `READY <pid>`（probe.exe 那一路不接，行为一个字不变）。
	WantStdout bool
	// WantStderr 为真时同样接一根管道（宿主的 stderr → 会话日志）。
	WantStderr bool
}

// hostProcess 是一个已拉起的客户端进程及其 Job。
type hostProcess struct {
	Pid       int
	process   windows.Handle
	thread    windows.Handle
	job       windows.Handle
	created   uint64
	timeout   time.Duration
	unbounded bool // interactive-ui：等客户端自己关，不设上限（probe.cpp L147）
	code      uint32
	closed    bool

	// client 是汉化启动宿主报回来的**游戏客户端**句柄（只有包装路径非 0）。非 0 时
	// watch / exitCode / terminateAndWait 盯的都是它，而不是宿主进程（见 watchTarget）。
	client windows.Handle
	// stdout / stderr 是上面两根管道的读端（父进程侧）。
	stdout *os.File
	stderr *os.File
	// stdoutDone / stderrDone 在对应的转发 goroutine 退出后关闭：关句柄之前必须等它们
	// （对着一个还挂着读操作的句柄关句柄是未定义行为）。
	stdoutDone <-chan struct{}
	stderrDone <-chan struct{}
}

// hostPipe 是一根"子进程写、父进程读"的匿名管道（汉化启动宿主的 stdout / stderr）。
//
// 为什么不用 os/exec 的 StdoutPipe：宿主要**挂起起、先进 Job 再 Resume**（probe.cpp L126-L144
// 的进程语义就靠这个顺序），而 os/exec 不暴露 thread handle，也给不了"进 Job 之后再放行"
// 这个时序。所以这里自己 CreatePipe + 把写端交给 CreateProcessW 继承。
type hostPipe struct {
	readHandle  windows.Handle
	writeHandle windows.Handle
	reader      *os.File
}

// newHostPipe 建一根匿名管道：写端可继承（给子进程），读端不可继承（只归父进程）。
func newHostPipe() (*hostPipe, error) {
	attributes := &windows.SecurityAttributes{
		Length:        uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		InheritHandle: 1,
	}
	var read, write windows.Handle
	if err := windows.CreatePipe(&read, &write, attributes, 0); err != nil {
		return nil, err
	}
	// 读端不能被继承：否则子进程会一直握着写端不肯放手，父进程永远读不到 EOF。
	if err := windows.SetHandleInformation(read, windows.HANDLE_FLAG_INHERIT, 0); err != nil {
		_ = windows.CloseHandle(read)
		_ = windows.CloseHandle(write)
		return nil, err
	}
	return &hostPipe{readHandle: read, writeHandle: write, reader: os.NewFile(uintptr(read), "host-pipe")}, nil
}

// writeHandleOrZero 是给 CreateProcessW 继承的写端（没建管道时是 0，表示"不接"）。
func (p *hostPipe) writeHandleOrZero() windows.Handle {
	if p == nil {
		return 0
	}
	return p.writeHandle
}

// readFile 是父进程这一侧的读端（没建管道时是 nil）。
func (p *hostPipe) readFile() *os.File {
	if p == nil {
		return nil
	}
	return p.reader
}

// closeWrite 关掉父进程这一侧的写端。CreateProcessW 之后必须立刻关：子进程已经拿到自己那份，
// 父进程再留一份就等于让读端永远等不到 EOF。
func (p *hostPipe) closeWrite() {
	if p == nil || p.writeHandle == 0 {
		return
	}
	_ = windows.CloseHandle(p.writeHandle)
	p.writeHandle = 0
}

// closeAll 关掉两端（幂等；失败路径上用完就扔）。
func (p *hostPipe) closeAll() {
	if p == nil {
		return
	}
	p.closeWrite()
	if p.reader != nil {
		_ = p.reader.Close()
		p.reader = nil
		p.readHandle = 0
	}
}

// startHostProcess 复刻 probe.cpp L121-L144。
//
// 失败时的返回码与 probe.cpp 完全一致：Job 建不出来 7、CreateProcessW 失败 11、
// 进不了 Job 12。
func startHostProcess(spec hostProcessSpec) (*hostProcess, int, error) {
	job, jobErr := windows.CreateJobObject(nil, nil)
	if jobErr != nil {
		return nil, exitClientHostJobError, fmt.Errorf("CreateJobObjectW: %w", jobErr)
	}
	var limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, setErr := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)),
		uint32(unsafe.Sizeof(limits)),
	); setErr != nil {
		_ = windows.CloseHandle(job)
		return nil, exitClientHostJobError, fmt.Errorf("SetInformationJobObject: %w", setErr)
	}

	// 管道先建好（写端要在 CreateProcessW 之前交给它继承）。只建真正要接的那几根：
	// 多接一根没人读的管道，子进程写满缓冲就会卡在那里。
	var stdoutPipe, stderrPipe *hostPipe
	if spec.WantStdout {
		pipe, pipeErr := newHostPipe()
		if pipeErr != nil {
			_ = windows.CloseHandle(job)
			return nil, exitClientHostCreateError, fmt.Errorf("创建宿主 stdout 管道: %w", pipeErr)
		}
		stdoutPipe = pipe
	}
	if spec.WantStderr {
		pipe, pipeErr := newHostPipe()
		if pipeErr != nil {
			stdoutPipe.closeAll()
			_ = windows.CloseHandle(job)
			return nil, exitClientHostCreateError, fmt.Errorf("创建宿主 stderr 管道: %w", pipeErr)
		}
		stderrPipe = pipe
	}
	discardPipes := func() {
		stdoutPipe.closeAll()
		stderrPipe.closeAll()
	}

	process, thread, pid, createErr := createSuspendedProcess(spec, stdoutPipe, stderrPipe)
	if createErr != nil {
		discardPipes()
		_ = windows.CloseHandle(job)
		return nil, exitClientHostCreateError, createErr
	}
	// 子进程已经拿到（并继承了）写端：父进程立刻关掉自己那一份。
	stdoutPipe.closeWrite()
	stderrPipe.closeWrite()
	if assignErr := windows.AssignProcessToJobObject(job, process); assignErr != nil {
		// 复刻 probe.cpp L143：进不了 Job 就直接杀掉，不留下一个失控的客户端。
		_ = windows.TerminateProcess(process, 99)
		_ = windows.CloseHandle(thread)
		_ = windows.CloseHandle(process)
		_ = windows.CloseHandle(job)
		discardPipes()
		return nil, exitClientHostAssignError, fmt.Errorf("AssignProcessToJobObject: %w", assignErr)
	}
	if _, resumeErr := windows.ResumeThread(thread); resumeErr != nil {
		_ = windows.TerminateJobObject(job, 0xE0000001)
		_ = windows.CloseHandle(thread)
		_ = windows.CloseHandle(process)
		_ = windows.CloseHandle(job)
		discardPipes()
		return nil, exitClientHostCreateError, fmt.Errorf("ResumeThread: %w", resumeErr)
	}
	return &hostProcess{
		Pid:       int(pid),
		process:   process,
		thread:    thread,
		job:       job,
		created:   hostTickCount(),
		timeout:   spec.Timeout,
		unbounded: spec.UIMode == "interactive-ui",
		stdout:    stdoutPipe.readFile(),
		stderr:    stderrPipe.readFile(),
	}, 0, nil
}

// createSuspendedProcess 是 probe.cpp L135-L142 的 CreateProcessW：
// 命令行 `"<target>" <args...>`，工作目录 = 客户端目录，窗口隐藏 + CREATE_NO_WINDOW
// + CREATE_SUSPENDED。
//
// 命令行用 syscall.EscapeArg 逐个转义。payload 里只有 `?` 与数字，不需要引号，
// 所以产出与 probe 的手工拼接逐字节相同。
//
// stdoutPipe / stderrPipe 非 nil 时把它们的写端接给子进程（汉化宿主路径要读 `READY <pid>`）；
// 两个都为 nil 时与 probe.cpp 完全一致（不传句柄、不继承 —— probe.cpp 的 bInheritHandles 是
// FALSE，那样连标准句柄都不给，子进程的输出直接丢）。
func createSuspendedProcess(spec hostProcessSpec, stdoutPipe, stderrPipe *hostPipe) (windows.Handle, windows.Handle, uint32, error) {
	target, err := syscall.UTF16PtrFromString(spec.Target)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("可执行路径无法转 UTF-16：%w", err)
	}
	commandLine := buildClientCommandLine(spec.Target, spec.Args)
	line, err := syscall.UTF16PtrFromString(commandLine)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("命令行无法转 UTF-16：%w", err)
	}
	var directory *uint16
	if spec.WorkingDir != "" {
		if directory, err = syscall.UTF16PtrFromString(spec.WorkingDir); err != nil {
			return 0, 0, 0, fmt.Errorf("工作目录无法转 UTF-16：%w", err)
		}
	}
	// 环境块：nil 表示继承宿主（probe.cpp L142 也是 nullptr）。
	// 用 unsafe.SliceData 取首元素地址，避免为一整块环境做一次"取址即逃逸"的拷贝。
	var environment *uint16
	var environmentBlock []uint16
	if len(spec.Env) > 0 {
		environmentBlock = utf16EnvironmentBlock(spec.Env)
		environment = unsafe.SliceData(environmentBlock)
	}

	var startup syscall.StartupInfo
	startup.Cb = uint32(unsafe.Sizeof(startup))
	startup.Flags = uint32(windows.STARTF_USESHOWWINDOW)
	// probe.cpp L141：interactive-ui / normal-ui / trace-root-ui 会把窗口显示出来；
	// 启动链默认 interactive-ui，所以这条分支就是常态。
	switch spec.UIMode {
	case "interactive-ui", "normal-ui", "trace-root-ui", "trace-ui":
		startup.ShowWindow = windows.SW_SHOWNORMAL
	default:
		startup.ShowWindow = windows.SW_HIDE
	}

	// 汉化宿主路径：把管道的写端当标准输出/错误交给子进程。带了 STARTF_USESTDHANDLES 就必须
	// 同时把三个标准句柄都填上（没给的那个用 0 = 不接），否则 CreateProcessW 会抱怨参数不对。
	stdoutWrite, stderrWrite := stdoutPipe.writeHandleOrZero(), stderrPipe.writeHandleOrZero()
	inheritHandles := stdoutWrite != 0 || stderrWrite != 0
	if inheritHandles {
		startup.Flags |= windows.STARTF_USESTDHANDLES
		startup.StdInput = 0
		startup.StdOutput = syscall.Handle(stdoutWrite)
		startup.StdErr = syscall.Handle(stderrWrite)
	}

	var info syscall.ProcessInformation
	// CREATE_UNICODE_ENVIRONMENT 必须带：上面传的是 UTF-16 环境块。
	// 少了它 CreateProcessW 直接报 ERROR_INVALID_PARAMETER(87)（实测：
	// NO_WINDOW+UTF16 环境块 → "The parameter is incorrect."；加上 0x400 → OK；
	// 2026-10-05 业主实机「创建客户端进程失败、退出码 1」的根因）。
	// environment == nil 时该标志无副作用（Windows 忽略它）。
	flags := uint32(windows.CREATE_SUSPENDED | windows.CREATE_NO_WINDOW | createUnicodeEnvironment)
	if createErr := syscall.CreateProcess(
		target,
		line,
		nil,
		nil,
		inheritHandles,
		flags,
		environment,
		directory,
		&startup,
		&info,
	); createErr != nil {
		return 0, 0, 0, fmt.Errorf("CreateProcessW: %w", createErr)
	}
	runtime.KeepAlive(commandLine)
	runtime.KeepAlive(environmentBlock)
	return windows.Handle(info.Process), windows.Handle(info.Thread), info.ProcessId, nil
}

// buildClientCommandLine 复刻 probe.cpp L135-L136：`"<exe>"` 后面接参数。
func buildClientCommandLine(target string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, syscall.EscapeArg(target))
	for _, arg := range args {
		parts = append(parts, syscall.EscapeArg(arg))
	}
	return strings.Join(parts, " ")
}

// utf16EnvironmentBlock 把 KEY=VALUE 列表编成 CreateProcessW 要的 UTF-16 环境块：
// 每项以 U+0000 结尾，整块再加一个 U+0000。顺序保持调用方给的原样。
func utf16EnvironmentBlock(entries []string) []uint16 {
	units := make([]uint16, 0, 1024)
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		for _, r := range entry {
			if r < 0x10000 {
				units = append(units, uint16(r))
			} else {
				value := r - 0x10000
				units = append(units, uint16(0xd800+(value>>10)), uint16(0xdc00+(value&0x3ff)))
			}
		}
		units = append(units, 0)
	}
	if len(units) == 0 {
		return nil
	}
	return append(units, 0)
}

// watchTarget 返回"本次要等谁退出"。
//
// 有汉化宿主报回的客户端句柄时是**客户端**，不是宿主进程：宿主只在启动阶段附加，初始化完成后
// 自己就可能退出 —— 等它等于把一次还在跑的游戏判成已结束（会话就此收尾，网关与隔离都被拆掉）。
func (p *hostProcess) watchTarget() windows.Handle {
	if p == nil {
		return 0
	}
	if p.client != 0 {
		return p.client
	}
	return p.process
}

// attachClient 打开汉化宿主报回来的游戏客户端进程，之后 watch / exitCode / terminateAndWait
// 盯的都是它。只申请"等待 + 读退出码"所需的最小权限（OpenProcess 的权限面越小越好）。
func (p *hostProcess) attachClient(pid int) error {
	if p == nil {
		return fmt.Errorf("内部错误：没有进程句柄")
	}
	if pid <= 0 {
		return fmt.Errorf("客户端 pid 无效：%d", pid)
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("OpenProcess(%d)：%w", pid, err)
	}
	p.client = handle
	return nil
}

// awaitWrapperClient 等汉化宿主报出游戏客户端 pid（读它的 stdout），上限 clientWrapperReadyTimeout。
//
// 返回的 pid 是**客户端**的 pid；读者 goroutine 的结束信号存进 p.stdoutDone，closeAll 会等它。
func (p *hostProcess) awaitWrapperClient(sink io.Writer) (int, error) {
	if p == nil || p.stdout == nil {
		return 0, fmt.Errorf("汉化启动宿主没有可读的 stdout（内部错误）")
	}
	pid, done, err := awaitWrapperReady(p.stdout, sink, clientWrapperReadyTimeout)
	p.stdoutDone = done
	return pid, err
}

// drainStderr 把子进程的 stderr 转到 sink（后台，直到管道断开）。收尾时 closeAll 等它退出。
func (p *hostProcess) drainStderr(sink io.Writer) {
	if p == nil || p.stderr == nil || sink == nil {
		return
	}
	done := make(chan struct{})
	p.stderrDone = done
	go func() {
		defer close(done)
		_, _ = io.Copy(sink, p.stderr)
	}()
}

// watch 复刻 probe.cpp L146-L153 的等待循环：每 200ms 问一次"退出了没有"，
// interactive-ui 不设上限（客户端开着就一直等），其余模式到点就停（宿主随后收 Job）。
// 返回 (退出码, 是否超时)。
func (p *hostProcess) watch() (uint32, bool) {
	deadline := p.created + uint64(p.timeout.Milliseconds())
	for {
		event, _ := windows.WaitForSingleObject(p.watchTarget(), 200)
		if event == windows.WAIT_OBJECT_0 {
			return p.exitCode(), false
		}
		if !p.unbounded && hostTickCount() >= deadline {
			return p.exitCode(), true
		}
	}
}

// terminateAndWait 复刻 probe.cpp L154 的 TerminateJobObject + WaitForSingleObject(2s)。
func (p *hostProcess) terminateAndWait(timeout time.Duration) {
	if p == nil || p.job == 0 {
		return
	}
	_ = windows.TerminateJobObject(p.job, 0xE0000001)
	// 等的是 watchTarget：有汉化宿主时 p.process 是宿主（可能早就退出），等它等于没等。
	_, _ = windows.WaitForSingleObject(p.watchTarget(), uint32(timeout/time.Millisecond))
}

// exitCode 读进程退出码（还在跑就是 STILL_ACTIVE）。有客户端句柄时读的是**客户端**的退出码
// （probe.json 的 probe_returncode 口径与无包装路径一致）。
func (p *hostProcess) exitCode() uint32 {
	if p == nil {
		return 0
	}
	target := p.watchTarget()
	if target == 0 {
		return p.code
	}
	var code uint32
	if err := syscall.GetExitCodeProcess(syscall.Handle(target), &code); err != nil {
		return p.code
	}
	p.code = code
	return code
}

// closeAll 关掉管道读端、线程/进程/客户端/Job 句柄（probe.cpp L154 的三个 CloseHandle +
// JOB_CLOSED）。返回 Job 句柄是否真的被关掉（那就是日志里的 JOB_CLOSED）。
//
// 关读端之前先等两个转发 goroutine 退出（有界）：对着一个还挂着读操作的句柄关句柄是未定义行为。
func (p *hostProcess) closeAll() bool {
	if p == nil || p.closed {
		return false
	}
	p.closed = true
	awaitReader(p.stdoutDone, hostReaderGrace)
	awaitReader(p.stderrDone, hostReaderGrace)
	if p.stdout != nil {
		_ = p.stdout.Close()
		p.stdout = nil
	}
	if p.stderr != nil {
		_ = p.stderr.Close()
		p.stderr = nil
	}
	closed := false
	if p.thread != 0 {
		_ = windows.CloseHandle(p.thread)
		p.thread = 0
	}
	if p.process != 0 {
		_ = windows.CloseHandle(p.process)
		p.process = 0
	}
	if p.client != 0 {
		_ = windows.CloseHandle(p.client)
		p.client = 0
	}
	if p.job != 0 {
		_ = windows.CloseHandle(p.job)
		p.job = 0
		closed = true
	}
	return closed
}

// hostReaderGrace 是"等管道转发 goroutine 收尾"的上限：宿主已经被杀，管道写端随之关闭，
// 读者正常都会立刻返回；等不到也不该把收尾挂住（真的卡住的读者由进程退出收拾）。
const hostReaderGrace = 2 * time.Second
