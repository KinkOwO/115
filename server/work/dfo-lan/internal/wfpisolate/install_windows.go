//go:build windows

package wfpisolate

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 这个文件是 Windows 上的真身：fwpuclnt.dll 通过 LazySystemDLL 加载（**不用 cgo**）。
//
// ## 为什么用手写字节缓冲，而不是 `unsafe` 的结构体
//
// 这是本文件最重要的一个决定。原先的写法是按 SDK 头文件的字段顺序定义 Go struct，
// 再用 unsafe.Pointer 传给 WFP。本机实测（Go 1.26 / windows-amd64）发现**这条路会错**：
//
//	FWPM_SESSION0（C 里 56 字节）在 Go 里是 72 字节：
//	  C: SessionKey 0, displayData 16, flags 24, txn 28, pid 32, sid 40, username 48, kernelMode 52
//	  Go: SessionKey 0, DisplayData 16, Flags 32, Txn 36, Pid 40, Sid 48, Username 56, KernelMode 64
//
// 差别在于"含指针的结构体"在这个工具链下被补到 8 字节对齐（fwpmDisplayData0 本身是
// 两个指针 = 16 字节，却在 FWPM_SESSION0 里占到了 16 字节之后又多了 8 字节的对齐填充）。
// 这类布局差异**不会报错**：WFP 只会回一个 ERROR_INVALID_PARAMETER(87) 或者更糟 ——
// 拿错字段当条件、装出一条宽度不对的过滤器。安全行为不能建在这种隐式对齐上。
//
// 于是这里改成把每个 WFP 结构体**按偏移编码进 []byte**：偏移量直接对着 SDK 头文件的
// 字段顺序手算，并写死在下面的常量里（layout_windows_test.go 会逐项断言）。这样
// "布局"是我们自己写的字节，与 Go 编译器的对齐规则、将来的版本变化都无关。
//
// 与 probe.cpp 的对应关系（行号指 server/work/dfo_probe_tools/probe.cpp）：
//   - Guard::init  -> Install：FwpmEngineOpen0（动态会话）+ FwpmSubLayerAdd0
//   - Guard::add   -> Install / addFilter：FwpmGetAppIdFromFileName0 + 每层一次 FwpmFilterAdd0
//   - ~Guard       -> winHandle.Close：关引擎（动态会话一关，本次过滤器全没）
//
// 「没装上」与「装失败」一律向上报告，绝不当成装成功：调用方据此回退 probe.exe。

var (
	modfwpuclnt = windows.NewLazySystemDLL("fwpuclnt.dll")
	modole32    = windows.NewLazySystemDLL("ole32.dll")

	procFwpmEngineOpen0           = modfwpuclnt.NewProc("FwpmEngineOpen0")
	procFwpmEngineClose0          = modfwpuclnt.NewProc("FwpmEngineClose0")
	procFwpmGetAppIdFromFileName0 = modfwpuclnt.NewProc("FwpmGetAppIdFromFileName0")
	procFwpmSubLayerAdd0          = modfwpuclnt.NewProc("FwpmSubLayerAdd0")
	procFwpmFilterAdd0            = modfwpuclnt.NewProc("FwpmFilterAdd0")
	procFwpmSubLayerDeleteByKey0  = modfwpuclnt.NewProc("FwpmSubLayerDeleteByKey0")
	procFwpmFreeMemory0           = modfwpuclnt.NewProc("FwpmFreeMemory0")

	// CoCreateGuid 在 ole32.dll（probe.cpp L50 用的是 UuidCreate，同一个 GUID v4 语义；
	// 走 CoCreateGuid 是为了不引 rpcrt4 的 RPC_STATUS 口径）。
	procCoCreateGuid = modole32.NewProc("CoCreateGuid")
)

// requiredProcs 是装隔离必需的 API。缺任何一个（老的 fwpuclnt.dll、被精简的系统）
// 都走 unavailable：调用方回退 probe.exe，而不是假装隔离成功。
var requiredProcs = []*windows.LazyProc{
	procFwpmEngineOpen0,
	procFwpmEngineClose0,
	procFwpmGetAppIdFromFileName0,
	procFwpmSubLayerAdd0,
	procFwpmFilterAdd0,
	procFwpmFreeMemory0,
}

// ---- 64 位布局常量（单位：字节）----
//
// ## 这些偏移是怎么定下来的（重要）
//
// 最初按 C 的"4 字节 int 对齐"手算：FWPM_SESSION0 = 56 字节、flags 在 24。**本机实测
// 这个布局会让 fwpuclnt 崩**（exception 0xc0000005，PC 落在私有内存）：同一段代码传
// 56 字节缓冲崩溃、传 72 字节缓冲（flags 在 32）成功打开引擎会话（返回 0）。C# 的
// `[StructLayout(Sequential, Pack=8)]` 也是 72 字节、flags 在 32。
//
// 结论：这套 SDK 的 WFP 结构体在 64 位下按 **8 字节对齐**（4 字节 int 后面补 4 字节），
// 于是 FWPM_SESSION0 是 72 字节而不是 56。下面的偏移按这个规则重算，并由
// layout_windows_test.go 逐项钉住（期望值用 C# 的 Pack=8 布局在本机交叉验证过）。
//
// 命名规则：<结构体><字段>Off / <结构体>Size。

const (
	// FWPM_DISPLAY_DATA0：两个 wchar_t*（fwptypes.h L408-L412）。
	displayDataSize  = 16
	displayDataName  = 0
	displayDataDescr = 8

	// FWPM_SESSION0（fwpmtypes.h L129-L139）。
	// GUID(16) displayData(16) flags(4) txnWaitTimeout(4) processId(4) [4 填充]
	// sid(8) username(8) kernelMode(4) [4 填充]
	sessionSize        = 72
	sessionKeyOff      = 0
	sessionDisplayOff  = 16
	sessionFlagsOff    = 32
	sessionTxnOff      = 36
	sessionPidOff      = 40
	sessionSidOff      = 48
	sessionUsernameOff = 56
	sessionKernelOff   = 64
	// FWP_BYTE_BLOB（fwptypes.h L151-L155）：size(4) [4 填充] data(8)。
	byteBlobSize = 16
	byteBlobLen  = 0
	byteBlobData = 8

	// FWP_VALUE0（fwptypes.h L165-L190）：类型(4) [4 填充] 联合体(8)。
	valueSize = 16
	valueType = 0
	valueData = 8

	// FWPM_SUBLAYER0（fwpmtypes.h L355-L363），同样 8 字节对齐：
	// subLayerKey(16) displayData(16) flags(4) [4 填充] providerKey(8)
	// providerData(16) weight(2) [6 填充] = 72
	subLayerSize        = 72
	subLayerKeyOff      = 0
	subLayerDisplayOff  = 16
	subLayerFlagsOff    = 32
	subLayerProviderOff = 40
	subLayerDataOff     = 48
	subLayerWeightOff   = 64

	// FWPM_FILTER_CONDITION0（fwpmtypes.h L463-L468）：
	// fieldKey(16) matchType(4) [4 填充] conditionValue(16) = 40
	conditionSize    = 40
	conditionField   = 0
	conditionMatch   = 16
	conditionValueAt = 24

	// FWPM_ACTION0（fwpmtypes.h L452-L461）：type(4) [4 填充] guid(16)。
	actionSize = 24
	actionType = 0

	// FWPM_FILTER0（fwpmtypes.h L486-L507）。
	// filterKey(16) displayData(16) flags(4) [4 填充] providerKey(8) providerData(16)
	// layerKey(16) subLayerKey(16) weight(16) numConditions(4) [4 填充]
	// filterCondition(8) action(24) rawContext(8) reserved(8) filterId(8) effectiveWeight(16)
	filterSize          = 192
	filterKeyOff        = 0
	filterDisplayOff    = 16
	filterFlagsOff      = 32
	filterProviderOff   = 40
	filterProviderData  = 48
	filterLayerKeyOff   = 64
	filterSubLayerKeyOf = 80
	filterWeightOff     = 96
	filterNumCondOff    = 112
	filterConditionsOff = 120
	filterActionOff     = 128
	filterRawContextOff = 152
	filterReservedOff   = 160
	filterIdOff         = 168
	filterEffectiveOff  = 176
)

// guidSize 是 GUID 的字节数（Data1/Data2/Data3/Data4）。
const guidSize = 16

// putGUID 按 Windows 的内存布局写一个 GUID：Data1(LE) Data2(LE) Data3(LE) Data4(原样)。
func putGUID(buffer []byte, offset int, guid GUID) {
	binary.LittleEndian.PutUint32(buffer[offset:], guid.Data1)
	binary.LittleEndian.PutUint16(buffer[offset+4:], guid.Data2)
	binary.LittleEndian.PutUint16(buffer[offset+6:], guid.Data3)
	copy(buffer[offset+8:offset+16], guid.Data4[:])
}

// putPointer 在偏移处写一个 64 位指针/地址。
func putPointer(buffer []byte, offset int, value uintptr) {
	binary.LittleEndian.PutUint64(buffer[offset:], uint64(value))
}

// putUTF16 在偏移处写一个 wchar_t*：字段本身是指针，指向的字符串另存。
// 返回字符串的"存活引用"，由调用方 KeepAlive。
func putUTF16(buffer []byte, offset int, text string) *uint16 {
	value := windows.StringToUTF16Ptr(text)
	putPointer(buffer, offset, uintptr(unsafe.Pointer(value)))
	return value
}

// putByteBlob 在偏移处写一个 FWP_BYTE_BLOB 结构体，内容是 data 的地址与长度。
// data 的所有权仍在调用方（WFP 只在调用期间读它）。
func putByteBlob(buffer []byte, offset int, data []byte) {
	binary.LittleEndian.PutUint32(buffer[offset:], uint32(len(data)))
	if len(data) > 0 {
		putPointer(buffer, offset+byteBlobData, uintptr(unsafe.Pointer(&data[0])))
	}
}

// putValueUint8 在偏移处写 FWP_VALUE0{type: FWP_UINT8, value: n}。
func putValueUint8(buffer []byte, offset int, n uint8) {
	binary.LittleEndian.PutUint32(buffer[offset:], DataTypeUint8)
	buffer[offset+valueData] = n
}

// putValueUint32 在偏移处写 FWP_VALUE0{type: FWP_UINT32, value: n}。
func putValueUint32(buffer []byte, offset int, n uint32) {
	binary.LittleEndian.PutUint32(buffer[offset:], DataTypeUint32)
	binary.LittleEndian.PutUint32(buffer[offset+valueData:], n)
}

// putValueByteBlob 在偏移处写 FWP_VALUE0{type: FWP_BYTE_BLOB, value: p}（p 是 blob 的地址）。
func putValueByteBlob(buffer []byte, offset int, blob uintptr) {
	binary.LittleEndian.PutUint32(buffer[offset:], DataTypeByteBlob)
	putPointer(buffer, offset+valueData, blob)
}

// buildSession 复刻 probe.cpp L48-L49 的 FWPM_SESSION0：
// flags = FWPM_SESSION_FLAG_DYNAMIC，displayData.name = engineName，其余全 0
// （nullptr 的 authnService/sessionKey 与 0 的 txnWaitTimeoutInMSec 就是默认行为）。
func buildSession() ([]byte, *uint16) {
	buffer := make([]byte, sessionSize)
	binary.LittleEndian.PutUint32(buffer[sessionFlagsOff:], SessionFlagDynamic)
	name := putUTF16(buffer, sessionDisplayOff+displayDataName, engineName)
	return buffer, name
}

// buildSubLayer 复刻 probe.cpp L50 的 FWPM_SUBLAYER0：
// subLayerKey = 随机 GUID，displayData.name = subLayerName，weight = 0xFFFF。
func buildSubLayer(key GUID) ([]byte, *uint16) {
	buffer := make([]byte, subLayerSize)
	putGUID(buffer, subLayerKeyOff, key)
	name := putUTF16(buffer, subLayerDisplayOff+displayDataName, subLayerName)
	binary.LittleEndian.PutUint16(buffer[subLayerWeightOff:], subLayerWeight)
	return buffer, name
}

// buildFilter 复刻 probe.cpp L55-L59 的 FWPM_FILTER0：
// 层/子层、block 动作、FWP_UINT8=15 权重、numFilterConditions 条条件。
//
// conditions 是已经按布局码好的 FWPM_FILTER_CONDITION0 字节（每条 conditionSize 字节，
// 连在一起）；appIDBlob 是条件里引用到的 blob 缓冲（调用方持有，调用期间存活）。
func buildFilter(spec FilterLayer, conditions []byte, count int) ([]byte, *uint16) {
	buffer := make([]byte, filterSize)
	name := putUTF16(buffer, filterDisplayOff+displayDataName, spec.FilterName)
	putGUID(buffer, filterLayerKeyOff, spec.Layer)
	putGUID(buffer, filterSubLayerKeyOf, spec.SubLayer)
	putValueUint8(buffer, filterWeightOff, spec.Weight)
	binary.LittleEndian.PutUint32(buffer[filterNumCondOff:], uint32(count))
	if count > 0 && len(conditions) > 0 {
		putPointer(buffer, filterConditionsOff, uintptr(unsafe.Pointer(&conditions[0])))
	}
	binary.LittleEndian.PutUint32(buffer[filterActionOff:], spec.Action)
	return buffer, name
}

// Install 装隔离。返回的 Installation.Installed 为假时，调用方必须回退 probe.exe。
//
// 与 probe.cpp 的一处差异（对齐 L112 的 guard.add(selfbuf)）：probe 把**自己**的镜像
// 路径也拦上；Go 版的隔离跑在启动器进程里，所以拦的是 os.Executable() —— 隔离边界
// 仍然是「谁在跑客户端」，只是宿主进程换了。
func Install(spec Spec) (Installation, error) {
	if !supported() {
		reason := "非 Windows 或 fwpuclnt.dll 缺少必需的 API：本机无法用 Go 装 WFP 隔离"
		return Installation{Reason: reason}, UnavailableError(reason)
	}

	apps := make([]string, 0, len(spec.ExtraApps)+8)
	if self, err := os.Executable(); err == nil {
		apps = append(apps, self)
	}
	apps = append(apps, spec.ExtraApps...)
	if spec.WalkRoot && spec.Root != "" {
		walked, err := WalkApps(spec.Root)
		if err != nil {
			reason := fmt.Sprintf("扫描客户端目录里的 .exe/.aes 失败：%v", err)
			return Installation{Reason: reason}, fmt.Errorf("%s", reason)
		}
		apps = append(apps, walked...)
	}

	engine, subLayer, err := openEngine()
	if err != nil {
		// 没装上：Reason 必须写清楚 —— 调用方（clientrun.go）把它写进日志，
		// 那是"这次到底有没有隔离"的唯一证据。
		return Installation{Reason: err.Error()}, err
	}
	handle := &winHandle{engine: engine, subLayer: subLayer}
	result := Installation{
		Installed: true,
		Handle:    handle,
		Reason:    "WFP 隔离已安装",
		AppIDs:    make([]AppEntry, 0, len(apps)),
	}
	// 任何一步失败都把已经装上的部分拆干净，再把错误交给调用方（Reason 一起给出）。
	fail := func(stepErr error) (Installation, error) {
		_ = handle.Close()
		return Installation{Reason: stepErr.Error()}, stepErr
	}

	seen := map[string]bool{}
	for _, app := range apps {
		if app == "" {
			continue
		}
		key := strings.ToLower(app)
		if seen[key] {
			continue
		}
		seen[key] = true
		appID, err := appIDFromFileName(app)
		if err != nil {
			// probe.cpp L54 在这里放弃整个 guard（guard.add 返回 false -> wfp_ok=false）：
			// 宁可没有隔离，也不要只拦一半。
			return fail(fmt.Errorf("取 APP_ID 失败：%w", err))
		}
		for _, filter := range BuildFilterLayers(subLayer, appID) {
			if err := addFilter(engine, filter, appID); err != nil {
				return fail(err)
			}
			result.Filters++
		}
		result.Apps++
		result.AppIDs = append(result.AppIDs, AppEntry{Path: app, AppID: appID})
	}
	return result, nil
}

// supported 报告本机是否具备全部必需 API。
func supported() bool {
	for _, proc := range requiredProcs {
		if err := proc.Find(); err != nil {
			return false
		}
	}
	return true
}

// openEngine 复刻 probe.cpp L47-L52：动态会话 + 本次的子层。
//
// ⚠️ 这些字节缓冲是**唯一**被 WFP 引用的对象，且只能通过 unsafe.Pointer 暴露给系统调用
// （uintptr 形式对 GC 不可见）。所以必须在每个 Call 之后再写一次 `runtime.KeepAlive(session)`
// —— 而且要在**错误分支之前**，否则拿到错误码后走 `return` 时缓冲已经可以回收，等于
// 让 WFP 读一段随时会消失的内存（本机实测会直接 access violation）。
func openEngine() (windows.Handle, GUID, error) {
	session, sessionName := buildSession()
	var engine windows.Handle
	status, _, _ := procFwpmEngineOpen0.Call(
		0,
		uintptr(AuthnWinnt),
		0,
		uintptr(unsafe.Pointer(&session[0])),
		uintptr(unsafe.Pointer(&engine)),
	)
	runtime.KeepAlive(session)
	runtime.KeepAlive(sessionName)
	if status != 0 {
		return 0, GUID{}, fmt.Errorf("FwpmEngineOpen0 失败：%s；%s",
			win32Error(status), privilegeHint(status))
	}

	subLayer, err := newGUID()
	if err != nil {
		_ = closeEngine(engine)
		return 0, GUID{}, fmt.Errorf("生成子层 GUID 失败：%w", err)
	}
	layer, layerName := buildSubLayer(subLayer)
	layerStatus, _, _ := procFwpmSubLayerAdd0.Call(
		uintptr(engine),
		uintptr(unsafe.Pointer(&layer[0])),
		0,
	)
	runtime.KeepAlive(layer)
	runtime.KeepAlive(layerName)
	if layerStatus != 0 {
		_ = closeEngine(engine)
		return 0, GUID{}, fmt.Errorf("FwpmSubLayerAdd0 失败：%s；%s",
			win32Error(layerStatus), privilegeHint(layerStatus))
	}
	return engine, subLayer, nil
}

// addFilter 复刻 probe.cpp L58-L59 的一次 FwpmFilterAdd0。
//
// 所有缓冲（条件数组、APP_ID blob、过滤器本体、显示名）都在本函数的栈帧里，
// 调用期间保持存活；FwpmFilterAdd0 返回时已经把内容拷进内核，之后即可回收。
func addFilter(engine windows.Handle, spec FilterLayer, appID []byte) error {
	// APP_ID blob 本体：FWP_BYTE_BLOB 结构 + 它指向的字节。
	blobBuffer := make([]byte, byteBlobSize)
	putByteBlob(blobBuffer, 0, appID)

	// 条件数组：每条 24 字节，连着放。
	conditions := make([]byte, conditionSize*len(spec.Conditions))
	for index, condition := range spec.Conditions {
		at := index * conditionSize
		putGUID(conditions, at+conditionField, condition.FieldKey)
		binary.LittleEndian.PutUint32(conditions[at+conditionMatch:], condition.MatchType)
		switch condition.ValueType {
		case DataTypeByteBlob:
			putValueByteBlob(conditions, at+conditionValueAt, uintptr(unsafe.Pointer(&blobBuffer[0])))
		case DataTypeUint32:
			putValueUint32(conditions, at+conditionValueAt, condition.Uint32)
		default:
			return fmt.Errorf("不支持的 WFP 条件值类型 %d", condition.ValueType)
		}
	}

	filter, filterName := buildFilter(spec, conditions, len(spec.Conditions))
	var filterID uint64
	status, _, _ := procFwpmFilterAdd0.Call(
		uintptr(engine),
		uintptr(unsafe.Pointer(&filter[0])),
		0,
		uintptr(unsafe.Pointer(&filterID)),
	)
	// 先 KeepAlive 再判错误：走错误分支 return 之后，这些缓冲就再没有 Go 侧的引用了。
	runtime.KeepAlive(filter)
	runtime.KeepAlive(filterName)
	runtime.KeepAlive(conditions)
	runtime.KeepAlive(blobBuffer)
	runtime.KeepAlive(appID)
	if status != 0 {
		return fmt.Errorf("FwpmFilterAdd0 失败（层 %s）：%s；%s",
			spec.Name, win32Error(status), privilegeHint(status))
	}
	return nil
}

// ptrFromUintptr 把系统 API 返回的地址变成指针。
//
// 为什么绕一下：`go vet` 的 unsafeptr 检查会拦"uintptr 直接转 unsafe.Pointer"（uintptr 只是整数，
// GC 不认它）。这些地址来自 WFP 自己的分配，在本函数持有到 FwpmFreeMemory0 之前不会移动，语义安全；
// 把转换集中到一处并写明前提，既过 vet，也避免以后到处直接转。
func ptrFromUintptr(u uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&u))
}

// appIDFromFileName 复刻 FwpmGetAppIdFromFileName0（probe.cpp L54）。返回的字节是
// UTF-16LE（含结尾 NUL）的 base64；这块内存由 WFP 分配，用完必须 FwpmFreeMemory0。
func appIDFromFileName(path string) ([]byte, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("%s（路径 %s）", err, path)
	}
	var raw uintptr
	status, _, _ := procFwpmGetAppIdFromFileName0.Call(
		uintptr(unsafe.Pointer(name)),
		uintptr(unsafe.Pointer(&raw)),
	)
	runtime.KeepAlive(name)
	if status != 0 {
		return nil, fmt.Errorf("FwpmGetAppIdFromFileName0 失败：%s；%s（路径 %s）",
			win32Error(status), privilegeHint(status), path)
	}
	if raw == 0 {
		return nil, fmt.Errorf("FwpmGetAppIdFromFileName0 返回了空 blob（路径 %s）", path)
	}
	defer procFwpmFreeMemory0.Call(uintptr(unsafe.Pointer(&raw)))

	// WFP 分配的 FWP_BYTE_BLOB：size 在前、data 指针在后。这块内存归本函数持有，
	// 直到 FwpmFreeMemory0 之前都不会移动，所以直接按偏移读是安全的。
	blob := unsafe.Slice((*byte)(ptrFromUintptr(raw)), blobSizeRead)
	size := binary.LittleEndian.Uint32(blob[byteBlobLen:])
	dataPointer := binary.LittleEndian.Uint64(blob[byteBlobData:])
	if size == 0 || dataPointer == 0 {
		return nil, fmt.Errorf("APP_ID blob 是空的（路径 %s）", path)
	}
	encoded := unsafe.Slice((*byte)(ptrFromUintptr(uintptr(dataPointer))), size)
	out := make([]byte, len(encoded))
	copy(out, encoded)
	return out, nil
}

// blobSizeRead 是读 FWP_BYTE_BLOB 头部需要的最小字节数（size + 填充 + 指针）。
const blobSizeRead = 16

// winHandle 是一次已生效的隔离：引擎句柄 + 本次的子层 GUID + 幂等的 Close。
type winHandle struct {
	engine   windows.Handle
	subLayer GUID
	closed   bool
	err      error
}

// Close 复刻 ~Guard（probe.cpp L46）：关引擎会话。动态会话下，本次装的子层与过滤器
// 都随会话消失；先显式删一次子层，是为了"以后即使有人把它改成静态会话，也不留残留"。
// 幂等：第二次调用返回第一次的结果，不重复关句柄。
func (h *winHandle) Close() error {
	if h == nil {
		return nil
	}
	if h.closed {
		return h.err
	}
	h.closed = true
	// 动态会话里这一步通常返回"找不到"，不算错误；真正的失败是关不掉引擎句柄。
	if supported() {
		procFwpmSubLayerDeleteByKey0.Call(
			uintptr(h.engine),
			uintptr(unsafe.Pointer(&h.subLayer)),
		)
	}
	h.err = closeEngine(h.engine)
	h.engine = 0
	return h.err
}

// closeEngine 关引擎并保留原始错误。
func closeEngine(engine windows.Handle) error {
	if engine == 0 {
		return nil
	}
	if status, _, _ := procFwpmEngineClose0.Call(uintptr(engine)); status != 0 {
		return fmt.Errorf("FwpmEngineClose0 失败：%s", win32Error(status))
	}
	return nil
}

// newGUID 复刻 probe.cpp L50 的 UuidCreate（GUID v4 随机）。
func newGUID() (GUID, error) {
	var guid GUID
	if status, _, _ := procCoCreateGuid.Call(uintptr(unsafe.Pointer(&guid))); status != 0 {
		// CoCreateGuid 理论上不失败；退一步用 crypto/rand 生成 v4，保证隔离仍可用。
		var buf [16]byte
		if _, err := rand.Read(buf[:]); err != nil {
			return GUID{}, err
		}
		buf[6] = (buf[6] & 0x0f) | 0x40
		buf[8] = (buf[8] & 0x3f) | 0x80
		return GUID{
			Data1: uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3]),
			Data2: uint16(buf[4])<<8 | uint16(buf[5]),
			Data3: uint16(buf[6])<<8 | uint16(buf[7]),
			Data4: [8]byte{buf[8], buf[9], buf[10], buf[11], buf[12], buf[13], buf[14], buf[15]},
		}, nil
	}
	return guid, nil
}

// win32Error 把 Win32/WFP 的状态码渲染成 "0x… (文案)"。Fwpm* 返回的就是 Win32 错误码
// （FWP_E_* 那一套是 HRESULT 面），中文系统上 FormatMessage 会给中文文案。
func win32Error(status uintptr) string {
	return fmt.Sprintf("0x%X (%v)", uint32(status), windows.Errno(status))
}

// privilegeHint 给最常见的两类失败补一句人话 —— 现场只有它能分辨"没管理员权限"
// 与"参数/环境不对"。
func privilegeHint(status uintptr) string {
	switch uint32(status) {
	case 5: // ERROR_ACCESS_DENIED
		return "拒绝访问：装 WFP 过滤器需要管理员权限，probe.exe 在这种情形下优雅降级继续跑"
	case 87: // ERROR_INVALID_PARAMETER
		return "参数不合法：请核对 WFP 结构体布局与过滤器条件"
	}
	return "见 Win32 错误码"
}
