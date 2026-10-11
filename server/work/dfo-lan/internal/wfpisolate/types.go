package wfpisolate

// 这个文件是 WFP 常量的**单一来源**：数值全部对着 Windows SDK
// （shared/fwptypes.h、shared/fwpmtypes.h、um/fwpmu.h）抄下来，并逐个标上出处。
// 之所以不引用 golang.org/x/sys/windows 里可能存在的同名常量：WFP 那部分在 x/sys
// 里并不完整（结构体更是没有），混着用反而看不出哪个值是从头里抄的。

// GUID 是 Windows 的 GUID。与 syscall.GUID / windows.GUID 的内存布局完全一致
// （Data1/Data2/Data3 各 4/2/2 字节 + 8 字节 Data4），但自带一个可读的格式。
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// String 是标准 8-4-4-4-12 形式，只用于日志与测试。
func (g GUID) String() string {
	const hex = "0123456789abcdef"
	out := make([]byte, 0, 36)
	appendPair := func(b byte) {
		out = append(out, hex[b>>4], hex[b&0x0f])
	}
	appendUint32 := func(value uint32) {
		for shift := 28; shift >= 0; shift -= 4 {
			out = append(out, hex[(value>>uint(shift))&0x0f])
		}
	}
	appendUint16 := func(value uint16) {
		for shift := 12; shift >= 0; shift -= 4 {
			out = append(out, hex[(value>>uint(shift))&0x0f])
		}
	}
	appendUint32(g.Data1)
	out = append(out, '-')
	appendUint16(g.Data2)
	out = append(out, '-')
	appendUint16(g.Data3)
	out = append(out, '-')
	appendPair(g.Data4[0])
	appendPair(g.Data4[1])
	out = append(out, '-')
	for _, b := range g.Data4[2:] {
		appendPair(b)
	}
	return string(out)
}

// FWP_DATA_TYPE 与 FWP_MATCH_TYPE（fwptypes.h L111-L137、L192-L209）。
const (
	DataTypeUint8      uint32 = 1 // FWP_UINT8
	DataTypeUint16     uint32 = 2 // FWP_UINT16
	DataTypeUint32     uint32 = 3 // FWP_UINT32
	DataTypeByteBlob   uint32 = 13
	MatchEqual         uint32 = 0 // FWP_MATCH_EQUAL
	MatchFlagsNoneSet  uint32 = 8 // FWP_MATCH_FLAGS_NONE_SET
	ActionBlock        uint32 = 0x1001
	SessionFlagDynamic uint32 = 0x00000001 // FWPM_SESSION_FLAG_DYNAMIC
)

// Filtering condition flags（fwpmtypes.h 的 FWP_CONDITION_FLAG_*）。
// IS_LOOPBACK = 0x00000001：probe.cpp 用它做**排除条件**。
const ConditionFlagIsLoopback uint32 = 0x00000001

// 层与条件的 GUID，逐字抄自 um/fwpmu.h：
//
//	FWPM_LAYER_ALE_AUTH_CONNECT_V4      c38d57d1-05a7-4c33-904f-7fbceee60e82  (L455-L462)
//	FWPM_LAYER_ALE_AUTH_CONNECT_V6      4a72393b-319f-44bc-84c3-ba54dcb3b6b4  (L473-L480)
//	FWPM_LAYER_ALE_AUTH_RECV_ACCEPT_V4  e1cd9fe7-f4b5-4273-96c0-592e487b8650  (L419-L426)
//	FWPM_LAYER_ALE_AUTH_RECV_ACCEPT_V6  a3b42c97-9f04-4672-b87e-cee9c483257f  (L437-L444)
//	FWPM_CONDITION_ALE_APP_ID           d78e1e87-8644-4ea5-9437-d809ecefc971  (L1863-L1870)
//	FWPM_CONDITION_FLAGS                632ce23b-5167-435c-86d7-e903684aa80c  (L1779-L1785)
var (
	// LayerALEAuthConnectV4 是出站连接授权层（probe.cpp 装过滤器的两层之一）。
	LayerALEAuthConnectV4 = GUID{0xc38d57d1, 0x05a7, 0x4c33, [8]byte{0x90, 0x4f, 0x7f, 0xbc, 0xee, 0xe6, 0x0e, 0x82}}
	// LayerALEAuthConnectV6 同上，IPv6。
	LayerALEAuthConnectV6 = GUID{0x4a72393b, 0x319f, 0x44bc, [8]byte{0x84, 0xc3, 0xba, 0x54, 0xdc, 0xb3, 0xb6, 0xb4}}
	// LayerALEAuthRecvAcceptV4 是入站接收授权层：**probe.exe 没有装**（已知差异）。
	LayerALEAuthRecvAcceptV4 = GUID{0xe1cd9fe7, 0xf4b5, 0x4273, [8]byte{0x96, 0xc0, 0x59, 0x2e, 0x48, 0x7b, 0x86, 0x50}}
	// LayerALEAuthRecvAcceptV6 同上，IPv6。
	LayerALEAuthRecvAcceptV6 = GUID{0xa3b42c97, 0x9f04, 0x4672, [8]byte{0xb8, 0x7e, 0xce, 0xe9, 0xc4, 0x83, 0x25, 0x7f}}
	// ConditionALEAppID 是「按进程镜像」的条件（probe.cpp 的第一条条件）。
	ConditionALEAppID = GUID{0xd78e1e87, 0x8644, 0x4ea5, [8]byte{0x94, 0x37, 0xd8, 0x09, 0xec, 0xef, 0xc9, 0x71}}
	// ConditionFlags 是流量标志条件（probe.cpp 的第二条条件）。
	ConditionFlags = GUID{0x632ce23b, 0x5167, 0x435c, [8]byte{0x86, 0xd7, 0xe9, 0x03, 0x68, 0x4a, 0xa8, 0x0c}}
)

// AuthnWinnt 是 RPC_C_AUTHN_WINNT（rpcdce.h），probe.cpp L49 传给 FwpmEngineOpen0 的
// authnService。0x0A 是 WinNT 认证，FwpmEngineOpen0 只接受这一个值。
const AuthnWinnt uint32 = 0x0A

// 引擎/子层/过滤器的显示名在这里统一（spec.go 里的常量是它们的唯一来源）。
const (
	EngineName   = engineName
	SubLayerName = subLayerName
	FilterName   = filterName
)

// AppIDBytes 把路径编成 WFP 的 APP_ID blob：路径的 UTF-16LE 字节 + 结尾 U+0000，
// 再做 base64（**标准 base64，带 = 填充**，不含换行）。
//
// 这不是"猜"的格式：FwpmGetAppIdFromFileName0 本机实测返回的就是它，见 spec.go 顶部
// 那段实测记录。正式安装仍然调用该 API，这个函数只用于钉住形状与日志核对。
func AppIDBytes(path string) []byte {
	encoded := make([]byte, 0, len(path)*2+8)
	for _, unit := range utf16LEUnits(path) {
		encoded = append(encoded, byte(unit), byte(unit>>8))
	}
	// 结尾的 U+0000。
	encoded = append(encoded, 0, 0)
	return base64Encode(encoded)
}

// utf16LEUnits 把字符串切成 UTF-16 码元。非 ASCII 走标准代理对，与 Windows 的
// UTF-16 编码一致；非法 UTF-8 字节按 U+FFFD 处理（不会出现在真实路径里）。
func utf16LEUnits(text string) []uint16 {
	units := make([]uint16, 0, len(text))
	for _, r := range text {
		switch {
		case r < 0x10000:
			units = append(units, uint16(r))
		default:
			value := r - 0x10000
			units = append(units, uint16(0xd800+(value>>10)), uint16(0xdc00+(value&0x3ff)))
		}
	}
	return units
}

// base64Encode 是标准 base64（RFC 4648，带填充）。自己写是为了让"没有换行、没有 URL
// 变体"这件事在代码里看得见（单元测试与 encoding/base64 逐字节对照）。
func base64Encode(data []byte) []byte {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	out := make([]byte, 0, (len(data)+2)/3*4)
	for index := 0; index < len(data); index += 3 {
		var chunk [3]byte
		copied := copy(chunk[:], data[index:])
		out = append(out, alphabet[chunk[0]>>2])
		out = append(out, alphabet[(chunk[0]&0x03)<<4|chunk[1]>>4])
		if copied > 1 {
			out = append(out, alphabet[(chunk[1]&0x0f)<<2|chunk[2]>>6])
		} else {
			out = append(out, '=')
		}
		if copied > 2 {
			out = append(out, alphabet[chunk[2]&0x3f])
		} else {
			out = append(out, '=')
		}
	}
	return out
}
