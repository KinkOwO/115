package launcher

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	memoryKernel       = windows.NewLazySystemDLL("kernel32.dll")
	processMemoryInfo  = memoryKernel.NewProc("K32GetProcessMemoryInfo")
	globalMemoryStatus = memoryKernel.NewProc("GlobalMemoryStatusEx")
)

type memoryLogWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *memoryLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

type processMemoryCounters struct {
	Size, Faults                                                    uint32
	PeakWorkingSet, WorkingSet, PeakPagedPool, PagedPool            uintptr
	PeakNonPagedPool, NonPagedPool, Pagefile, PeakPagefile, Private uintptr
}

type systemMemoryStatus struct {
	Size, Load                                                         uint32
	TotalPhysical, AvailablePhysical, TotalPagefile, AvailablePagefile uint64
	TotalVirtual, AvailableVirtual, AvailableExtendedVirtual           uint64
}

func readMemoryUsage(process windows.Handle) (private, working, available uint64, err error) {
	var p processMemoryCounters
	p.Size = uint32(unsafe.Sizeof(p))
	if ok, _, e := processMemoryInfo.Call(uintptr(process), uintptr(unsafe.Pointer(&p)), uintptr(p.Size)); ok == 0 {
		return 0, 0, 0, fmt.Errorf("GetProcessMemoryInfo: %w", e)
	}
	var s systemMemoryStatus
	s.Size = uint32(unsafe.Sizeof(s))
	if ok, _, e := globalMemoryStatus.Call(uintptr(unsafe.Pointer(&s))); ok == 0 {
		return 0, 0, 0, fmt.Errorf("GlobalMemoryStatusEx: %w", e)
	}
	return uint64(p.Private), uint64(p.WorkingSet), s.AvailablePhysical, nil
}

func configureMemoryJob(job windows.Handle, budget uint64) error {
	if uint64(uintptr(budget)) != budget {
		return fmt.Errorf("memory budget requires a 64-bit launcher")
	}
	var limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE |
		windows.JOB_OBJECT_LIMIT_PROCESS_MEMORY | windows.JOB_OBJECT_LIMIT_JOB_MEMORY
	limits.ProcessMemoryLimit = uintptr(budget)
	limits.JobMemoryLimit = uintptr(budget)
	_, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)))
	return err
}

// Start suspended so the gateway cannot allocate before the hard limit applies.
// Every failure reaps only the process we just created; there is no unprotected
// fallback, nor any process-name based termination.
func startMemoryBoundedChild(cmd *exec.Cmd, budget uint64, label string) (*childProcess, error) {
	sink := cmd.Stderr
	closeSink := func() {}
	if sink == nil {
		sink = io.Discard
	} else if file, directFile := sink.(*os.File); directFile {
		// startSession closes its file on return from ready. Keep a duplicate of
		// the SAME file object so monitoring continues and shares the write offset
		// with the child's stderr, rather than overwriting/interleaving records.
		var duplicate windows.Handle
		if err := windows.DuplicateHandle(windows.CurrentProcess(), windows.Handle(file.Fd()),
			windows.CurrentProcess(), &duplicate, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
			return nil, fmt.Errorf("复制内存监测日志句柄失败：%w", err)
		}
		logFile := os.NewFile(uintptr(duplicate), file.Name())
		sink = logFile
		closeSink = func() { _ = logFile.Close() }
	} else {
		// exec's stderr copier and the monitor must share a safe writer.
		sink = &memoryLogWriter{w: sink}
		cmd.Stderr = sink
	}
	handedOver := false
	defer func() {
		if !handedOver {
			closeSink()
		}
	}()
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("创建内存保护失败：%w", err)
	}
	if err := configureMemoryJob(job, budget); err != nil {
		windows.CloseHandle(job)
		return nil, fmt.Errorf("设置内存保护失败：%w", err)
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED | createNoWindow
	if err := cmd.Start(); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	fail := func(err error) (*childProcess, error) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("内存保护未生效，已取消本次启动：%w", err)
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|
		windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, uint32(cmd.Process.Pid))
	if err != nil {
		return fail(err)
	}
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		windows.CloseHandle(process)
		return fail(err)
	}
	if _, _, _, err := readMemoryUsage(process); err != nil {
		windows.CloseHandle(process)
		return fail(err)
	}
	if err := resumeOwnedProcess(uint32(cmd.Process.Pid)); err != nil {
		windows.CloseHandle(process)
		return fail(err)
	}
	fmt.Fprintf(sink, "MEMORY_GUARD active pid=%d scope=%s hard_commit_mib=%d low_memory_mib=%d system_pressure_action=warn_only policy=local-stability-20261008-r1\n", cmd.Process.Pid, label, budget>>20, systemLowMemoryBytes>>20)
	child := &childProcess{cmd: cmd, done: make(chan error, 1)}
	finished, monitorDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(monitorDone)
		watchOwnedMemory(cmd, process, budget, label, sink, finished)
	}()
	go func() {
		err := cmd.Wait()
		close(finished)
		<-monitorDone
		closeSink()
		_ = windows.CloseHandle(process)
		_ = windows.CloseHandle(job)
		child.done <- err
	}()
	handedOver = true
	return child, nil
}

func resumeOwnedProcess(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	var thread windows.ThreadEntry32
	thread.Size = uint32(unsafe.Sizeof(thread))
	for err := windows.Thread32First(snapshot, &thread); err == nil; err = windows.Thread32Next(snapshot, &thread) {
		if thread.OwnerProcessID != pid {
			continue
		}
		handle, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, thread.ThreadID)
		if err != nil {
			return err
		}
		count, err := windows.ResumeThread(handle)
		windows.CloseHandle(handle)
		if err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("unexpected primary thread suspend count %d", count)
		}
		return nil
	}
	return fmt.Errorf("找不到本次启动进程 %d 的挂起线程", pid)
}

func watchOwnedMemory(cmd *exec.Cmd, process windows.Handle, budget uint64, label string, sink io.Writer, finished <-chan struct{}) {
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	started := time.Now()
	var peak uint64
	var lastLog time.Time
	var pressure memoryPressureGuard
	for {
		select {
		case <-finished:
			fmt.Fprintf(sink, "MEMORY_GUARD ended scope=%s peak_private_mib=%d\n", label, peak>>20)
			return
		case <-tick.C:
			private, working, available, err := readMemoryUsage(process)
			if err != nil {
				// Wait may already be collecting an exited process.
				var code uint32
				if windows.GetExitCodeProcess(process, &code) == nil && code != 259 {
					return
				}
				fmt.Fprintf(sink, "MEMORY_GUARD stopped scope=%s reason=measurement_failed error=%v\n", label, err)
				_ = cmd.Process.Kill()
				return
			}
			if private > peak {
				peak = private
			}
			if time.Since(lastLog) >= 2*time.Second {
				fmt.Fprintf(sink, "MEMORY_GUARD sample scope=%s seconds=%.1f private_mib=%d working_mib=%d system_available_mib=%d\n", label, time.Since(started).Seconds(), private>>20, working>>20, available>>20)
				lastLog = time.Now()
			}
			decision := pressure.observe(time.Now(), private, available, budget)
			if decision.event != "" {
				fmt.Fprintf(sink, "MEMORY_GUARD %s scope=%s reason=system_memory_pressure private_mib=%d system_available_mib=%d low_seconds=%.1f\n", decision.event, label, private>>20, available>>20, decision.lowFor.Seconds())
			}
			if decision.stopTrigger != "" {
				fmt.Fprintf(sink, "MEMORY_GUARD stopped scope=%s reason=memory_pressure trigger=%s private_mib=%d system_available_mib=%d low_seconds=%.1f\n", label, decision.stopTrigger, private>>20, available>>20, decision.lowFor.Seconds())
				_ = cmd.Process.Kill()
				return
			}
		}
	}
}
