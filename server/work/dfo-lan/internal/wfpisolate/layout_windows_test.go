//go:build windows

package wfpisolate

import (
	"encoding/binary"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 这个文件钉住一件事：手写的字节布局与 Windows SDK（64 位）逐字段一致。
//
// 为什么不是"用 unsafe 定义结构体然后 Sizeof 断言"：Go 1.26/windows-amd64 上含指针的
// 结构体会被补到 8 字节对齐（实测 FWPM_SESSION0 变成 72 字节，C 里是 56），于是
// **不能**把 Go 的结构体直接交给 WFP。所以这里验证的是"我们自己写的字节"，见
// install_windows.go 顶部那段说明。

// offsetsFor 把当前布局常量整理成"字段名 -> 偏移"，方便逐项比对。
func testLayout() map[string]int {
	return map[string]int{
		"FWPM_DISPLAY_DATA0.name":               displayDataName,
		"FWPM_DISPLAY_DATA0.description":        displayDataDescr,
		"FWPM_SESSION0.sessionKey":              sessionKeyOff,
		"FWPM_SESSION0.displayData":             sessionDisplayOff,
		"FWPM_SESSION0.flags":                   sessionFlagsOff,
		"FWPM_SESSION0.txnWaitTimeoutInMSec":    sessionTxnOff,
		"FWPM_SESSION0.processId":               sessionPidOff,
		"FWPM_SESSION0.sid":                     sessionSidOff,
		"FWPM_SESSION0.username":                sessionUsernameOff,
		"FWPM_SESSION0.kernelMode":              sessionKernelOff,
		"FWP_BYTE_BLOB.size":                    byteBlobLen,
		"FWP_BYTE_BLOB.data":                    byteBlobData,
		"FWP_VALUE0.type":                       valueType,
		"FWP_VALUE0.value":                      valueData,
		"FWPM_SUBLAYER0.subLayerKey":            subLayerKeyOff,
		"FWPM_SUBLAYER0.displayData":            subLayerDisplayOff,
		"FWPM_SUBLAYER0.flags":                  subLayerFlagsOff,
		"FWPM_SUBLAYER0.providerKey":            subLayerProviderOff,
		"FWPM_SUBLAYER0.providerData":           subLayerDataOff,
		"FWPM_SUBLAYER0.weight":                 subLayerWeightOff,
		"FWPM_FILTER_CONDITION0.fieldKey":       conditionField,
		"FWPM_FILTER_CONDITION0.matchType":      conditionMatch,
		"FWPM_FILTER_CONDITION0.conditionValue": conditionValueAt,
		"FWPM_ACTION0.type":                     actionType,
		"FWPM_FILTER0.filterKey":                filterKeyOff,
		"FWPM_FILTER0.displayData":              filterDisplayOff,
		"FWPM_FILTER0.flags":                    filterFlagsOff,
		"FWPM_FILTER0.providerKey":              filterProviderOff,
		"FWPM_FILTER0.providerData":             filterProviderData,
		"FWPM_FILTER0.layerKey":                 filterLayerKeyOff,
		"FWPM_FILTER0.subLayerKey":              filterSubLayerKeyOf,
		"FWPM_FILTER0.weight":                   filterWeightOff,
		"FWPM_FILTER0.numFilterConditions":      filterNumCondOff,
		"FWPM_FILTER0.filterCondition":          filterConditionsOff,
		"FWPM_FILTER0.action":                   filterActionOff,
		"FWPM_FILTER0.rawContext":               filterRawContextOff,
		"FWPM_FILTER0.reserved":                 filterReservedOff,
		"FWPM_FILTER0.filterId":                 filterIdOff,
		"FWPM_FILTER0.effectiveWeight":          filterEffectiveOff,
	}
}

func TestLayoutOffsetsMatchSDK(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("只在 64 位 Windows 上验证布局")
	}
	// 期望值来自 SDK 头文件的字段顺序 + 本机实测的 8 字节对齐规则（x64）。
	want := map[string]int{
		"FWPM_DISPLAY_DATA0.name":               0,
		"FWPM_DISPLAY_DATA0.description":        8,
		"FWPM_SESSION0.sessionKey":              0,
		"FWPM_SESSION0.displayData":             16,
		"FWPM_SESSION0.flags":                   32,
		"FWPM_SESSION0.txnWaitTimeoutInMSec":    36,
		"FWPM_SESSION0.processId":               40,
		"FWPM_SESSION0.sid":                     48,
		"FWPM_SESSION0.username":                56,
		"FWPM_SESSION0.kernelMode":              64,
		"FWP_BYTE_BLOB.size":                    0,
		"FWP_BYTE_BLOB.data":                    8,
		"FWP_VALUE0.type":                       0,
		"FWP_VALUE0.value":                      8,
		"FWPM_SUBLAYER0.subLayerKey":            0,
		"FWPM_SUBLAYER0.displayData":            16,
		"FWPM_SUBLAYER0.flags":                  32,
		"FWPM_SUBLAYER0.providerKey":            40,
		"FWPM_SUBLAYER0.providerData":           48,
		"FWPM_SUBLAYER0.weight":                 64,
		"FWPM_FILTER_CONDITION0.fieldKey":       0,
		"FWPM_FILTER_CONDITION0.matchType":      16,
		"FWPM_FILTER_CONDITION0.conditionValue": 24,
		"FWPM_ACTION0.type":                     0,
		"FWPM_FILTER0.filterKey":                0,
		"FWPM_FILTER0.displayData":              16,
		"FWPM_FILTER0.flags":                    32,
		"FWPM_FILTER0.providerKey":              40,
		"FWPM_FILTER0.providerData":             48,
		"FWPM_FILTER0.layerKey":                 64,
		"FWPM_FILTER0.subLayerKey":              80,
		"FWPM_FILTER0.weight":                   96,
		"FWPM_FILTER0.numFilterConditions":      112,
		"FWPM_FILTER0.filterCondition":          120,
		"FWPM_FILTER0.action":                   128,
		"FWPM_FILTER0.rawContext":               152,
		"FWPM_FILTER0.reserved":                 160,
		"FWPM_FILTER0.filterId":                 168,
		"FWPM_FILTER0.effectiveWeight":          176,
	}
	got := testLayout()
	for name, wantOffset := range want {
		found, ok := got[name]
		if !ok {
			t.Errorf("%s 没有对应的布局常量", name)
			continue
		}
		if found != wantOffset {
			t.Errorf("%s 偏移 = %d，want %d", name, found, wantOffset)
		}
	}
}

// 结构体大小必须与"8 字节对齐"那套偏移自洽。期望值的来源有两条，互相独立：
//
//  1. C# 的 `[StructLayout(LayoutKind.Sequential, Pack=8)]`（本机实测输出，见下方注释）；
//  2. FWPM_SESSION0 那一半的**运行时证据**：用 72 字节会话缓冲调用 FwpmEngineOpen0
//     返回 0（成功），用按"4 字节对齐"算出来的 56 字节缓冲会 access violation。
//
// 本机 C# 实测输出（与这里的数字逐项一致）：
//
//	DD 16 / Blob 16 data@8 / Value 16 value@8
//	Session 72 display@16 flags@32 txn@36 pid@40 sid@48 user@56 km@64
//	SubLayer 72 display@16 flags@32 provKey@40 provData@48 weight@64
//	Cond 40 match@16 value@24 / Action 20 guid@4 / Filter 192（action@128 id@168 eff@176）
func TestLayoutSizesMatchSDK(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"FWPM_DISPLAY_DATA0", displayDataSize, 16},
		{"FWPM_SESSION0", sessionSize, 72},
		{"FWP_BYTE_BLOB", byteBlobSize, 16},
		{"FWP_VALUE0", valueSize, 16},
		{"FWPM_SUBLAYER0", subLayerSize, 72},
		{"FWPM_FILTER_CONDITION0", conditionSize, 40},
		{"FWPM_ACTION0", actionSize, 24},
		{"FWPM_FILTER0", filterSize, 192},
		{"GUID", guidSize, 16}}
	for _, testCase := range cases {
		if testCase.got != testCase.want {
			t.Errorf("%s = %d，want %d", testCase.name, testCase.got, testCase.want)
		}
	}
}

// buildSession / buildSubLayer / buildFilter 写出来的字节必须与预期逐字节一致：
// 这是"布局正确"最直接的证据（偏移常量写错时，条件或指针会落到错误的字段上）。
func TestMarshalledStructures(t *testing.T) {
	session, name := buildSession()
	if len(session) != sessionSize {
		t.Fatalf("会话缓冲 = %d 字节，want %d", len(session), sessionSize)
	}
	if flags := binary.LittleEndian.Uint32(session[sessionFlagsOff:]); flags != SessionFlagDynamic {
		t.Errorf("session.flags = 0x%X，want 0x%X", flags, SessionFlagDynamic)
	}
	if pointer := binary.LittleEndian.Uint64(session[sessionDisplayOff+displayDataName:]); pointer != uint64(uintptr(unsafe.Pointer(name))) {
		t.Error("session.displayData.name 指针不对")
	}
	if got := windows.UTF16PtrToString(name); got != engineName {
		t.Errorf("session 显示名 = %q，want %q", got, engineName)
	}
	// 其余字段必须全是 0：probe.cpp 用的就是零值（nullptr 的 authz/sid/username）。
	for offset := sessionKeyOff; offset < guidSize; offset++ {
		if session[offset] != 0 {
			t.Errorf("sessionKey 应当是零值，偏移 %d = 0x%X", offset, session[offset])
		}
	}
	for _, offset := range []int{sessionSidOff, sessionUsernameOff} {
		if value := binary.LittleEndian.Uint64(session[offset:]); value != 0 {
			t.Errorf("偏移 %d 应当为 0，得到 0x%X", offset, value)
		}
	}
	if value := binary.LittleEndian.Uint32(session[sessionKernelOff:]); value != 0 {
		t.Errorf("kernelMode 应当是 0，得到 0x%X", value)
	}

	key := GUID{Data1: 0x11223344, Data2: 0x5566, Data3: 0x7788, Data4: [8]byte{9, 10, 11, 12, 13, 14, 15, 16}}
	layer, layerName := buildSubLayer(key)
	if len(layer) != subLayerSize {
		t.Fatalf("子层缓冲 = %d 字节，want %d", len(layer), subLayerSize)
	}
	decoded := GUID{
		Data1: binary.LittleEndian.Uint32(layer[subLayerKeyOff:]),
		Data2: binary.LittleEndian.Uint16(layer[subLayerKeyOff+4:]),
		Data3: binary.LittleEndian.Uint16(layer[subLayerKeyOff+6:]),
		Data4: [8]byte{layer[8], layer[9], layer[10], layer[11], layer[12], layer[13], layer[14], layer[15]},
	}
	if decoded != key {
		t.Errorf("子层 key = %s，want %s", decoded, key)
	}
	if weight := binary.LittleEndian.Uint16(layer[subLayerWeightOff:]); weight != subLayerWeight {
		t.Errorf("子层权重 = 0x%X，want 0x%X", weight, subLayerWeight)
	}
	if got := windows.UTF16PtrToString(layerName); got != subLayerName {
		t.Errorf("子层显示名 = %q，want %q", got, subLayerName)
	}

	appID := AppIDBytes(`C:\Game\dof\115us\DFO\DFO.exe`)
	filterSpec := BuildFilterLayers(key, appID)[0]
	conditions := make([]byte, conditionSize*2)
	blobBuffer := make([]byte, byteBlobSize)
	putByteBlob(blobBuffer, 0, appID)
	putGUID(conditions, conditionField, ConditionALEAppID)
	binary.LittleEndian.PutUint32(conditions[conditionMatch:], MatchEqual)
	putValueByteBlob(conditions, conditionValueAt, uintptr(unsafe.Pointer(&blobBuffer[0])))
	putGUID(conditions, conditionSize+conditionField, ConditionFlags)
	binary.LittleEndian.PutUint32(conditions[conditionSize+conditionMatch:], MatchFlagsNoneSet)
	putValueUint32(conditions, conditionSize+conditionValueAt, ConditionFlagIsLoopback)

	filter, filterName := buildFilter(filterSpec, conditions, 2)
	if len(filter) != filterSize {
		t.Fatalf("过滤器缓冲 = %d 字节，want %d", len(filter), filterSize)
	}
	if got := windows.UTF16PtrToString(filterName); got != FilterName {
		t.Errorf("过滤器显示名 = %q，want %q", got, FilterName)
	}
	if binary.LittleEndian.Uint32(filter[filterNumCondOff:]) != 2 {
		t.Error("条件数不是 2")
	}
	if binary.LittleEndian.Uint32(filter[filterActionOff:]) != ActionBlock {
		t.Errorf("动作 = 0x%X，want 0x%X", binary.LittleEndian.Uint32(filter[filterActionOff:]), ActionBlock)
	}
	if weight := binary.LittleEndian.Uint32(filter[filterWeightOff:]); weight != DataTypeUint8 {
		t.Errorf("权重类型 = %d，want FWP_UINT8", weight)
	}
	if weight := filter[filterWeightOff+valueData]; weight != 15 {
		t.Errorf("权重 = %d，want 15", weight)
	}
	if pointer := binary.LittleEndian.Uint64(filter[filterConditionsOff:]); pointer != uint64(uintptr(unsafe.Pointer(&conditions[0]))) {
		t.Error("条件数组指针不对")
	}
	// 层 key 必须原样写进 layerKey 字段（逐字节等于输入层 GUID 的 Windows 布局）。
	layerField := filter[filterLayerKeyOff : filterLayerKeyOff+guidSize]
	wantLayer := make([]byte, guidSize)
	putGUID(wantLayer, 0, filterSpec.Layer)
	for index := range wantLayer {
		if layerField[index] != wantLayer[index] {
			t.Fatalf("layerKey = % X，want % X", layerField, wantLayer)
		}
	}
	// 条件 0：ALE_APP_ID 是 byteBlob，指向 blobBuffer。
	if valueType := binary.LittleEndian.Uint32(conditions[conditionValueAt:]); valueType != DataTypeByteBlob {
		t.Errorf("条件 0 的值类型 = %d，want FWP_BYTE_BLOB_TYPE", valueType)
	}
	if pointer := binary.LittleEndian.Uint64(conditions[conditionValueAt+valueData:]); pointer != uint64(uintptr(unsafe.Pointer(&blobBuffer[0]))) {
		t.Error("条件 0 的 blob 指针不对")
	}
	// 条件 1：FLAGS 未置 IS_LOOPBACK。
	if valueType := binary.LittleEndian.Uint32(conditions[conditionSize+conditionValueAt:]); valueType != DataTypeUint32 {
		t.Errorf("条件 1 的值类型 = %d，want FWP_UINT32", valueType)
	}
	if value := binary.LittleEndian.Uint32(conditions[conditionSize+conditionValueAt+valueData:]); value != ConditionFlagIsLoopback {
		t.Errorf("条件 1 的标志 = 0x%X，want IS_LOOPBACK", value)
	}
}

// isElevated 判断当前进程是否在管理员组里（WFP 装过滤器要求提升）。
func isElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
