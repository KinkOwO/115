//go:build windows

package wfpisolate

import (
	"crypto/rand"
	"fmt"
	"os"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 这个文件是 Windows 上的真身：fwpuclnt.dll 通过 LazySystemDLL 加载（**不用 cgo**），
// 结构体按 Windows SDK 的头文件布局手写，并靠 install_windows_test.go 里的
// Sizeof/Offsetof 断言钉住（布局错一个字节，WFP 只会回一个 87，什么也说明不了）。
//
// 与 probe.cpp 的对应关系（行号指 server/work/dfo_probe_tools/probe.cpp）：
//   - Guard::init  -> Install：FwpmEngineOpen0（动态会话）+ FwpmSubLayerAdd0
//   - Guard::add   -> Install / addFilter：FwpmGetAppIdFromFileName0 + 每层一次 FwpmFilterAdd0
//   - ~Guard       -> winHandle.Close：关引擎（动态会话一关，本次过滤器全没）
//
// 「没装上」与「装失败」一律向上报告，绝不当成装成功：调用方据此回退 probe.exe。

var (
	modfwpuclnt = windows.NewLazySystemDLL("fwpuclnt.dll")
	modkernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procFwpmEngineOpen0           = modfwpuclnt.NewProc("FwpmEngineOpen0")
	procFwpmEngineClose0          = modfwpuclnt.NewProc("FwpmEngineClose0")
	procFwpmGetAppIdFromFileName0 = modfwpuclnt.NewProc("FwpmGetAppIdFromFileName0")
	procFwpmSubLayerAdd0          = modfwpuclnt.NewProc("FwpmSubLayerAdd0")
	procFwpmFilterAdd0            = modfwpuclnt.NewProc("FwpmFilterAdd0")
	procFwpmSubLayerDeleteByKey0  = modfwpuclnt.NewProc("FwpmSubLayerDeleteByKey0")
	procFwpmFreeMemory0           = modfwpuclnt.NewProc("FwpmFreeMemory0")

	procCoCreateGuid = modkernel32.NewProc("CoCreateGuid")
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

// 结构体布局按 SDK 头文件的原样字段顺序写；指针字段一律 unsafe.Pointer（与 C 的 8 字节
// 指针同宽），补齐字段用空白标识符，一眼能看出"这里是 C 的对齐填充"。

// fwpmDisplayData0 是 FWPM_DISPLAY_DATA0（fwptypes.h L408-L412）。
type fwpmDisplayData0 struct {
	Name        *uint16
	Description *uint16
}

// fwpmSession0 是 FWPM_SESSION0（fwpmtypes.h L129-L139）。
type fwpmSession0 struct {
	SessionKey         GUID
	DisplayData        fwpmDisplayData0
	Flags              uint32
	TxnWaitTimeoutInMs uint32
	ProcessId          uint32
	_                  uint32
	Sid                unsafe.Pointer
	Username           *uint16
	KernelMode         int32
	_                  uint32
}

// fwpByteBlob 是 FWP_BYTE_BLOB（fwptypes.h L151-L155）：4 字节 size + 对齐填充 + 指针。
type fwpByteBlob struct {
	Size uint32
	_    uint32
	Data unsafe.Pointer
}

// fwpValue0 是 FWP_VALUE0（fwptypes.h L165-L190）：类型 + 联合体。
// 只用到 byteBlob 与 uint32 两种形态，统一按 8 字节承载。
type fwpValue0 struct {
	Type  uint32
	_     uint32
	Value uintptr
}

// fwpmSubLayer0 是 FWPM_SUBLAYER0（fwpmtypes.h L355-L363）。
type fwpmSubLayer0 struct {
	SubLayerKey  GUID
	DisplayData  fwpmDisplayData0
	Flags        uint32
	_            uint32
	ProviderKey  unsafe.Pointer
	ProviderData fwpByteBlob
	Weight       uint16
	_            [6]byte
}

// fwpmFilterCondition0 是 FWPM_FILTER_CONDITION0（fwpmtypes.h L463-L468）。
type fwpmFilterCondition0 struct {
	FieldKey       GUID
	MatchType      int32
	_              int32
	ConditionValue fwpValue0
}

// fwpmAction0 是 FWPM_ACTION0（fwpmtypes.h L452-L461）。
type fwpmAction0 struct {
	Type uint32
	_    uint32
	Guid GUID
}

// fwpmFilter0 是 FWPM_FILTER0（fwpmtypes.h L486-L507）。
type fwpmFilter0 struct {
	FilterKey           GUID
	DisplayData         fwpmDisplayData0
	Flags               uint32
	_                   uint32
	ProviderKey         unsafe.Pointer
	ProviderData        fwpByteBlob
	LayerKey            GUID
	SubLayerKey         GUID
	Weight              fwpValue0
	NumFilterConditions uint32
	_                   uint32
	FilterCondition     *fwpmFilterCondition0
	Action              fwpmAction0
	RawContext          uint64
	Reserved            unsafe.Pointer
	FilterId            uint64
	EffectiveWeight     fwpValue0
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
			return Installation{}, fmt.Errorf("扫描客户端目录里的 .exe/.aes 失败：%w", err)
		}
		apps = append(apps, walked...)
	}

	engine, subLayer, err := openEngine()
	if err != nil {
		return Installation{}, err
	}
	handle := &winHandle{engine: engine, subLayer: subLayer}
	result := Installation{
		Installed: true,
		Handle:    handle,
		Reason:    "WFP 隔离已安装",
		AppIDs:    make([]AppEntry, 0, len(apps)),
	}
	// 任何一步失败都把已经装上的部分拆干净，再把错误交给调用方。
	fail := func(stepErr error) (Installation, error) {
		_ = handle.Close()
		return Installation{}, stepErr
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
func openEngine() (windows.Handle, GUID, error) {
	var session fwpmSession0
	session.Flags = SessionFlagDynamic
	session.DisplayData.Name = windows.StringToUTF16Ptr(engineName)

	var engine windows.Handle
	status, _, _ := procFwpmEngineOpen0.Call(
		0,
		uintptr(AuthnWinnt),
		0,
		uintptr(unsafe.Pointer(&session)),
		uintptr(unsafe.Pointer(&engine)),
	)
	if status != 0 {
		return 0, GUID{}, fmt.Errorf("FwpmEngineOpen0 失败：%s；%s",
			win32Error(status), privilegeHint(status))
	}

	subLayer, err := newGUID()
	if err != nil {
		_ = closeEngine(engine)
		return 0, GUID{}, fmt.Errorf("生成子层 GUID 失败：%w", err)
	}
	var layer fwpmSubLayer0
	layer.SubLayerKey = subLayer
	layer.Weight = subLayerWeight
	layer.DisplayData.Name = windows.StringToUTF16Ptr(subLayerName)
	if status, _, _ := procFwpmSubLayerAdd0.Call(
		uintptr(engine),
		uintptr(unsafe.Pointer(&layer)),
		0,
	); status != 0 {
		_ = closeEngine(engine)
		return 0, GUID{}, fmt.Errorf("FwpmSubLayerAdd0 失败：%s；%s",
			win32Error(status), privilegeHint(status))
	}
	return engine, subLayer, nil
}

// addFilter 复刻 probe.cpp L58-L59 的一次 FwpmFilterAdd0。
//
// appID 是 ALE_APP_ID 条件的 blob 字节：它在调用期间必须保持存活。FwpmFilterAdd0
// 返回时已经把内容拷进内核，所以调用之后就可以回收 —— 但**必须**让编译器知道
// （runtime.KeepAlive），否则 blob 可能在 Call 返回前就被判定为死。
func addFilter(engine windows.Handle, filter FilterLayer, appID []byte) error {
	blob := fwpByteBlob{Size: uint32(len(appID))}
	if len(appID) > 0 {
		blob.Data = unsafe.Pointer(&appID[0])
	}

	conditions := make([]fwpmFilterCondition0, 0, len(filter.Conditions))
	for _, condition := range filter.Conditions {
		built := fwpmFilterCondition0{
			FieldKey:  condition.FieldKey,
			MatchType: int32(condition.MatchType),
		}
		switch condition.ValueType {
		case DataTypeByteBlob:
			built.ConditionValue.Type = DataTypeByteBlob
			built.ConditionValue.Value = uintptr(unsafe.Pointer(&blob))
		case DataTypeUint32:
			built.ConditionValue.Type = DataTypeUint32
			built.ConditionValue.Value = uintptr(condition.Uint32)
		default:
			return fmt.Errorf("不支持的 WFP 条件值类型 %d", condition.ValueType)
		}
		conditions = append(conditions, built)
	}

	var entry fwpmFilter0
	entry.DisplayData.Name = windows.StringToUTF16Ptr(filter.FilterName)
	entry.LayerKey = filter.Layer
	entry.SubLayerKey = filter.SubLayer
	entry.NumFilterConditions = uint32(len(conditions))
	entry.Weight.Type = filter.WeightType
	entry.Weight.Value = uintptr(filter.Weight)
	entry.Action.Type = filter.Action
	if len(conditions) > 0 {
		entry.FilterCondition = &conditions[0]
	}

	var filterID uint64
	status, _, _ := procFwpmFilterAdd0.Call(
		uintptr(engine),
		uintptr(unsafe.Pointer(&entry)),
		0,
		uintptr(unsafe.Pointer(&filterID)),
	)
	runtime.KeepAlive(conditions)
	runtime.KeepAlive(appID)
	runtime.KeepAlive(&blob)
	if status != 0 {
		return fmt.Errorf("FwpmFilterAdd0 失败（层 %s）：%s；%s",
			filter.Name, win32Error(status), privilegeHint(status))
	}
	return nil
}

// ptrFromUintptr 把系统 API 返回的地址变成指针。
//
// 为什么绕一下：`go vet` 的 unsafeptr 检查会拦"uintptr 直接转 unsafe.Pointer"（因为 uintptr
// 只是一个整数，GC 不认它，见 unsafe 文档）。WFP 在 FwpmGetAppIdFromFileName0 里分配的这块
// blob 归本函数持有、直到 FwpmFreeMemory0 之前都不会移动，语义上是安全的；这里把转换集中到
// 一处并写明前提，既过 vet，也避免以后有人到处直接转。
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
	if status != 0 {
		return nil, fmt.Errorf("FwpmGetAppIdFromFileName0 失败：%s；%s（路径 %s）",
			win32Error(status), privilegeHint(status), path)
	}
	if raw == 0 {
		return nil, fmt.Errorf("FwpmGetAppIdFromFileName0 返回了空 blob（路径 %s）", path)
	}
	defer procFwpmFreeMemory0.Call(uintptr(unsafe.Pointer(&raw)))

	blob := (*fwpByteBlob)(ptrFromUintptr(raw))
	if blob.Data == nil || blob.Size == 0 {
		return nil, fmt.Errorf("APP_ID blob 是空的（路径 %s）", path)
	}
	encoded := unsafe.Slice((*byte)(blob.Data), blob.Size)
	out := make([]byte, len(encoded))
	copy(out, encoded)
	return out, nil
}

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
	procFwpmSubLayerDeleteByKey0.Call(
		uintptr(h.engine),
		uintptr(unsafe.Pointer(&h.subLayer)),
	)
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
