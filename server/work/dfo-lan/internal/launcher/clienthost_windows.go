//go:build windows

package launcher

import (
	"fmt"
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

	process, thread, pid, createErr := createSuspendedProcess(spec)
	if createErr != nil {
		_ = windows.CloseHandle(job)
		return nil, exitClientHostCreateError, createErr
	}
	if assignErr := windows.AssignProcessToJobObject(job, process); assignErr != nil {
		// 复刻 probe.cpp L143：进不了 Job 就直接杀掉，不留下一个失控的客户端。
		_ = windows.TerminateProcess(process, 99)
		_ = windows.CloseHandle(thread)
		_ = windows.CloseHandle(process)
		_ = windows.CloseHandle(job)
		return nil, exitClientHostAssignError, fmt.Errorf("AssignProcessToJobObject: %w", assignErr)
	}
	if _, resumeErr := windows.ResumeThread(thread); resumeErr != nil {
		_ = windows.TerminateJobObject(job, 0xE0000001)
		_ = windows.CloseHandle(thread)
		_ = windows.CloseHandle(process)
		_ = windows.CloseHandle(job)
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
	}, 0, nil
}

// createSuspendedProcess 是 probe.cpp L135-L142 的 CreateProcessW：
// 命令行 `"<target>" <args...>`，工作目录 = 客户端目录，窗口隐藏 + CREATE_NO_WINDOW
// + CREATE_SUSPENDED。
//
// 命令行用 syscall.EscapeArg 逐个转义。payload 里只有 `?` 与数字，不需要引号，
// 所以产出与 probe 的手工拼接逐字节相同。
func createSuspendedProcess(spec hostProcessSpec) (windows.Handle, windows.Handle, uint32, error) {
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
		false,
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

// watch 复刻 probe.cpp L146-L153 的等待循环：每 200ms 问一次"退出了没有"，
// interactive-ui 不设上限（客户端开着就一直等），其余模式到点就停（宿主随后收 Job）。
// 返回 (退出码, 是否超时)。
func (p *hostProcess) watch() (uint32, bool) {
	deadline := p.created + uint64(p.timeout.Milliseconds())
	for {
		event, _ := windows.WaitForSingleObject(p.process, 200)
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
	_, _ = windows.WaitForSingleObject(p.process, uint32(timeout/time.Millisecond))
}

// exitCode 读进程退出码（还在跑就是 STILL_ACTIVE）。
func (p *hostProcess) exitCode() uint32 {
	if p == nil || p.process == 0 {
		return p.code
	}
	var code uint32
	if err := syscall.GetExitCodeProcess(syscall.Handle(p.process), &code); err != nil {
		return p.code
	}
	p.code = code
	return code
}

// closeAll 关掉线程/进程/Job 三个句柄（probe.cpp L154 的三个 CloseHandle + JOB_CLOSED）。
// 返回 Job 句柄是否真的被关掉（那就是日志里的 JOB_CLOSED）。
func (p *hostProcess) closeAll() bool {
	if p == nil || p.closed {
		return false
	}
	p.closed = true
	closed := false
	if p.thread != 0 {
		_ = windows.CloseHandle(p.thread)
		p.thread = 0
	}
	if p.process != 0 {
		_ = windows.CloseHandle(p.process)
		p.process = 0
	}
	if p.job != 0 {
		_ = windows.CloseHandle(p.job)
		p.job = 0
		closed = true
	}
	return closed
}
