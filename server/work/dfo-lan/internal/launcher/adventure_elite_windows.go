//go:build windows

package launcher

import (
	"bytes"
	"debug/pe"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var eliteKernel = windows.NewLazySystemDLL("kernel32.dll")
var eliteAlloc = eliteKernel.NewProc("VirtualAllocEx")
var eliteFree = eliteKernel.NewProc("VirtualFreeEx")
var eliteRemoteThread = eliteKernel.NewProc("CreateRemoteThread")
var eliteModuleHandle = eliteKernel.NewProc("GetModuleHandleExW")
var eliteInJob = eliteKernel.NewProc("IsProcessInJob")
var eliteThreadExit = eliteKernel.NewProc("GetExitCodeThread")

const eliteInjectTimeout = 30 * time.Second

func eliteModules(pid uint32) ([]windows.ModuleEntry32, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPMODULE|windows.TH32CS_SNAPMODULE32, pid)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ModuleEntry32{Size: uint32(unsafe.Sizeof(windows.ModuleEntry32{}))}
	if err = windows.Module32First(snapshot, &entry); err != nil {
		return nil, err
	}
	var entries []windows.ModuleEntry32
	for {
		entries = append(entries, entry)
		err = windows.Module32Next(snapshot, &entry)
		if err == windows.ERROR_NO_MORE_FILES {
			return entries, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

func eliteFindModule(pid uint32, path string, deadline time.Time) (windows.ModuleEntry32, error) {
	return eliteFindModuleSnapshot(pid, path, deadline, eliteModules)
}

// A just-resumed process can have an empty loader snapshot. Retry only the
// documented transient snapshot errors, within the existing injection deadline.
func eliteFindModuleSnapshot(pid uint32, path string, deadline time.Time, snapshot func(uint32) ([]windows.ModuleEntry32, error)) (windows.ModuleEntry32, error) {
	for {
		entries, err := snapshot(pid)
		if err == nil {
			for _, entry := range entries {
				loaded := windows.UTF16ToString(entry.ExePath[:])
				if strings.EqualFold(filepath.Clean(loaded), filepath.Clean(path)) {
					return entry, nil
				}
				if strings.EqualFold(filepath.Base(loaded), filepath.Base(path)) {
					return windows.ModuleEntry32{}, fmt.Errorf("同名 DLL 已从另一目录加载：%s", loaded)
				}
			}
		} else if err != windows.ERROR_BAD_LENGTH && err != windows.ERROR_PARTIAL_COPY && err != windows.ERROR_NO_MORE_FILES {
			return windows.ModuleEntry32{}, err
		}
		if time.Now().After(deadline) {
			return windows.ModuleEntry32{}, fmt.Errorf("等待远程模块超时：%s", filepath.Base(path))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func eliteCallRemote(process windows.Handle, address, argument uintptr, deadline time.Time) (uint32, bool, error) {
	thread, _, err := eliteRemoteThread.Call(uintptr(process), 0, 0, address, argument, 0, 0)
	if thread == 0 {
		return 0, true, fmt.Errorf("CreateRemoteThread：%w", err)
	}
	defer windows.CloseHandle(windows.Handle(thread))
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 0, false, fmt.Errorf("注入时间已耗尽")
	}
	event, err := windows.WaitForSingleObject(windows.Handle(thread), uint32(remaining/time.Millisecond)+1)
	if err != nil {
		return 0, false, err
	}
	if event != windows.WAIT_OBJECT_0 {
		return 0, false, fmt.Errorf("远程初始化未完成：wait=%d", event)
	}
	var code uint32
	ok, _, err := eliteThreadExit.Call(thread, uintptr(unsafe.Pointer(&code)))
	if ok == 0 {
		return 0, true, err
	}
	return code, true, nil
}

// Only a client owned by this launcher's Job can be injected. Neither process
// enumeration nor an externally supplied PID is accepted by the launch script.
func injectAdventureElite(pid int, dll, expectedExe string, job uintptr) error {
	if runtime.GOARCH != "amd64" || pid <= 0 || job == 0 {
		return fmt.Errorf("注入需要 x64 启动器及当前会话 Job")
	}
	rva, err := eliteModStartRVA(dll)
	if err != nil {
		return err
	}
	dll, err = filepath.Abs(dll)
	if err != nil {
		return err
	}
	process, err := windows.OpenProcess(windows.PROCESS_CREATE_THREAD|windows.PROCESS_QUERY_INFORMATION|
		windows.PROCESS_VM_OPERATION|windows.PROCESS_VM_WRITE|windows.PROCESS_VM_READ|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("打开本次客户端：%w", err)
	}
	defer windows.CloseHandle(process)
	var inJob uint32
	ok, _, err := eliteInJob.Call(uintptr(process), job, uintptr(unsafe.Pointer(&inJob)))
	if ok == 0 {
		return fmt.Errorf("检查会话 Job：%w", err)
	}
	if inJob == 0 {
		return fmt.Errorf("客户端不属于本次启动会话")
	}
	var machine, nativeMachine uint16
	if err = windows.IsWow64Process2(process, &machine, &nativeMachine); err != nil {
		return err
	}
	if machine != 0 || nativeMachine != pe.IMAGE_FILE_MACHINE_AMD64 {
		return fmt.Errorf("客户端必须是原生 x64")
	}
	var exeName [32768]uint16
	length := uint32(len(exeName))
	if err = windows.QueryFullProcessImageName(process, 0, &exeName[0], &length); err != nil {
		return err
	}
	expectedExe, err = filepath.Abs(expectedExe)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Clean(windows.UTF16ToString(exeName[:length])), filepath.Clean(expectedExe)) {
		return fmt.Errorf("客户端路径与本次启动目标不符")
	}
	deadline := time.Now().Add(eliteInjectTimeout)
	loadLibrary := eliteKernel.NewProc("LoadLibraryW")
	if err = loadLibrary.Find(); err != nil {
		return err
	}
	// LoadLibraryW can be forwarded into KernelBase. Resolve its actual local
	// provider, find that same provider remotely, and use its RVA (not local VA).
	var provider windows.Handle
	ok, _, err = eliteModuleHandle.Call(windows.GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS|windows.GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
		loadLibrary.Addr(), uintptr(unsafe.Pointer(&provider)))
	if ok == 0 {
		return err
	}
	var providerName [32768]uint16
	n, err := windows.GetModuleFileName(provider, &providerName[0], uint32(len(providerName)))
	if err != nil || n == 0 || int(n) >= len(providerName) {
		return fmt.Errorf("系统加载器路径无效：%v", err)
	}
	remoteProvider, err := eliteFindModule(uint32(pid), windows.UTF16ToString(providerName[:n]), deadline)
	if err != nil {
		return err
	}
	offset := loadLibrary.Addr() - uintptr(provider)
	if offset+16 > uintptr(remoteProvider.ModBaseSize) {
		return fmt.Errorf("系统加载器 RVA 超出模块")
	}
	remoteLoader := remoteProvider.ModBaseAddr + offset
	localBytes, remoteBytes := make([]byte, 16), make([]byte, 16)
	var count uintptr
	if err = windows.ReadProcessMemory(windows.CurrentProcess(), loadLibrary.Addr(), &localBytes[0], 16, &count); err != nil || count != 16 {
		return fmt.Errorf("读取本地加载器：%v", err)
	}
	if err = windows.ReadProcessMemory(process, remoteLoader, &remoteBytes[0], 16, &count); err != nil || count != 16 {
		return fmt.Errorf("读取远程加载器：%v", err)
	}
	if !bytes.Equal(localBytes, remoteBytes) {
		return fmt.Errorf("远程系统加载器现场不匹配")
	}
	path, err := windows.UTF16FromString(dll)
	if err != nil {
		return err
	}
	size := uintptr(len(path) * 2)
	allocation, _, err := eliteAlloc.Call(uintptr(process), 0, size, windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if allocation == 0 {
		return fmt.Errorf("分配远程 DLL 路径：%w", err)
	}
	completed := true
	defer func() {
		// Do not free a path that a timed-out thread may still read. The caller
		// terminates this newly created Job on failure, reclaiming its memory.
		if completed {
			eliteFree.Call(uintptr(process), allocation, 0, windows.MEM_RELEASE)
		}
	}()
	if err = windows.WriteProcessMemory(process, allocation, (*byte)(unsafe.Pointer(&path[0])), size, &count); err != nil || count != size {
		return fmt.Errorf("写入远程 DLL 路径：%v", err)
	}
	_, completed, err = eliteCallRemote(process, remoteLoader, allocation, deadline)
	if err != nil {
		return err
	}
	module, err := eliteFindModule(uint32(pid), dll, deadline)
	if err != nil {
		return err
	}
	if rva >= module.ModBaseSize {
		return fmt.Errorf("ModStart RVA 超出远程 DLL")
	}
	// ModStart has the same x64 calling convention as a thread entry; its unused
	// parameter is zero. Success means all expected sites matched and changed.
	for {
		code, _, err := eliteCallRemote(process, module.ModBaseAddr+uintptr(rva), 0, deadline)
		if err != nil {
			return err
		}
		if code == uint32(windows.ERROR_BUSY) && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if code != 0 {
			return fmt.Errorf("DLL ModStartInjected 拒绝初始化：code=%d；查看 DLL 目录状态文件", code)
		}
		return nil
	}
}
