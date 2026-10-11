package wfpisolate

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// 规格构造是这一层唯一能在没有管理员权限时验证的部分，所以钉得细一点：
// 层、条件、动作、权重、APP_ID 编码、错误分支、Close 幂等。

// BuildFilterLayers 必须给出 probe.cpp L55-L60 那两条条件的**原样组合**：
// ALE_APP_ID 相等 + FLAGS 未置 IS_LOOPBACK，动作 FWP_ACTION_BLOCK，权重 FWP_UINT8=15。
func TestBuildFilterLayersMatchesProbeConditions(t *testing.T) {
	subLayer := GUID{Data1: 0x11223344, Data2: 0x5566, Data3: 0x7788, Data4: [8]byte{1, 2, 3, 4, 5, 6, 7, 8}}
	appID := AppIDBytes(`C:\Game\dof\115us\DFO\DFO.exe`)

	layers := BuildFilterLayers(subLayer, appID)
	if len(layers) != 2 {
		t.Fatalf("过滤器条数 = %d，want 2（V4/V6 各一条，与 probe.cpp L57 一致）", len(layers))
	}
	wantLayers := []GUID{LayerALEAuthConnectV4, LayerALEAuthConnectV6}
	for index, layer := range layers {
		if layer.Layer != wantLayers[index] {
			t.Errorf("第 %d 条的层 = %s，want %s", index, layer.Layer, wantLayers[index])
		}
		if layer.SubLayer != subLayer {
			t.Errorf("第 %d 条的子层 = %s，want %s", index, layer.SubLayer, subLayer)
		}
		if layer.Action != ActionBlock {
			t.Errorf("第 %d 条的动作 = 0x%X，want FWP_ACTION_BLOCK 0x%X", index, layer.Action, ActionBlock)
		}
		if layer.WeightType != DataTypeUint8 || layer.Weight != 15 {
			t.Errorf("第 %d 条的权重类型/值 = %d/%d，want FWP_UINT8/15", index, layer.WeightType, layer.Weight)
		}
		if layer.FilterName != FilterName {
			t.Errorf("第 %d 条的显示名 = %q，want %q", index, layer.FilterName, FilterName)
		}
		if len(layer.Conditions) != 2 {
			t.Fatalf("第 %d 条的条件数 = %d，want 2", index, len(layer.Conditions))
		}
		first, second := layer.Conditions[0], layer.Conditions[1]
		if first.FieldKey != ConditionALEAppID || first.MatchType != MatchEqual ||
			first.ValueType != DataTypeByteBlob {
			t.Errorf("第 %d 条的第一个条件 = %+v，want ALE_APP_ID/EQUAL/byteBlob", index, first)
		}
		if !reflect.DeepEqual(first.AppID, appID) {
			t.Errorf("第 %d 条的 APP_ID 与输入的 blob 不一致", index)
		}
		if second.FieldKey != ConditionFlags || second.MatchType != MatchFlagsNoneSet ||
			second.ValueType != DataTypeUint32 || second.Uint32 != ConditionFlagIsLoopback {
			t.Errorf("第 %d 条的第二个条件 = %+v，want FLAGS/NONE_SET/uint32=IS_LOOPBACK", index, second)
		}
	}
}

// 常量必须与 Windows SDK 头文件逐字一致（都是 enum 推导出来的数值，写错一位 WFP 只会
// 回一个 87，什么也说明不了）。
func TestWFPConstantsMatchSDK(t *testing.T) {
	cases := []struct {
		name string
		got  uint32
		want uint32
	}{
		{"FWP_UINT8", DataTypeUint8, 1},
		{"FWP_UINT16", DataTypeUint16, 2},
		{"FWP_UINT32", DataTypeUint32, 3},
		{"FWP_BYTE_BLOB_TYPE", DataTypeByteBlob, 13},
		{"FWP_MATCH_EQUAL", MatchEqual, 0},
		{"FWP_MATCH_FLAGS_NONE_SET", MatchFlagsNoneSet, 8},
		// FWP_ACTION_BLOCK = FWP_ACTION_FLAG_TERMINATING(0x1000) | 1
		{"FWP_ACTION_BLOCK", ActionBlock, 0x1001},
		{"FWPM_SESSION_FLAG_DYNAMIC", SessionFlagDynamic, 0x00000001},
		{"FWP_CONDITION_FLAG_IS_LOOPBACK", ConditionFlagIsLoopback, 0x00000001},
		{"RPC_C_AUTHN_WINNT", AuthnWinnt, 0x0A},
	}
	for _, testCase := range cases {
		if testCase.got != testCase.want {
			t.Errorf("%s = 0x%X，want 0x%X", testCase.name, testCase.got, testCase.want)
		}
	}

	// 层/条件 GUID 的字符串形式直接与 um/fwpmu.h 的注释比对。
	guids := map[string]string{
		"FWPM_LAYER_ALE_AUTH_CONNECT_V4":     "c38d57d1-05a7-4c33-904f-7fbceee60e82",
		"FWPM_LAYER_ALE_AUTH_CONNECT_V6":     "4a72393b-319f-44bc-84c3-ba54dcb3b6b4",
		"FWPM_LAYER_ALE_AUTH_RECV_ACCEPT_V4": "e1cd9fe7-f4b5-4273-96c0-592e487b8650",
		"FWPM_LAYER_ALE_AUTH_RECV_ACCEPT_V6": "a3b42c97-9f04-4672-b87e-cee9c483257f",
		"FWPM_CONDITION_ALE_APP_ID":          "d78e1e87-8644-4ea5-9437-d809ecefc971",
		"FWPM_CONDITION_FLAGS":               "632ce23b-5167-435c-86d7-e903684aa80c",
	}
	values := map[string]GUID{
		"FWPM_LAYER_ALE_AUTH_CONNECT_V4":     LayerALEAuthConnectV4,
		"FWPM_LAYER_ALE_AUTH_CONNECT_V6":     LayerALEAuthConnectV6,
		"FWPM_LAYER_ALE_AUTH_RECV_ACCEPT_V4": LayerALEAuthRecvAcceptV4,
		"FWPM_LAYER_ALE_AUTH_RECV_ACCEPT_V6": LayerALEAuthRecvAcceptV6,
		"FWPM_CONDITION_ALE_APP_ID":          ConditionALEAppID,
		"FWPM_CONDITION_FLAGS":               ConditionFlags,
	}
	for name, want := range guids {
		if got := values[name].String(); got != want {
			t.Errorf("%s = %s，want %s", name, got, want)
		}
	}
}

// 已知差异必须钉在数据里：probe.exe 只装 ALE_AUTH_CONNECT_V4/V6，没有装接收层。
// 谁要改口径，就必须先改这条测试（而不是在别处顺手加一层）。
func TestOnlyConnectLayersAreInstalled(t *testing.T) {
	install := map[string]bool{}
	for _, layer := range FilterLayers() {
		install[layer.Name] = layer.Install
	}
	for _, name := range []string{"FWPM_LAYER_ALE_AUTH_CONNECT_V4", "FWPM_LAYER_ALE_AUTH_CONNECT_V6"} {
		if !install[name] {
			t.Errorf("%s 应当装过滤器（probe.cpp L57）", name)
		}
	}
	for _, name := range []string{"FWPM_LAYER_ALE_AUTH_RECV_ACCEPT_V4", "FWPM_LAYER_ALE_AUTH_RECV_ACCEPT_V6"} {
		if install[name] {
			t.Errorf("%s 不该装过滤器：probe.exe 没有装接收层（已知差异，不能悄悄补上）", name)
		}
	}
	if got := FilterSummary(12); got != "filters=24 NON_LOOPBACK_BLOCKED IPV4_IPV6" {
		t.Errorf("FilterSummary(12) = %q，want 24 条（与真实会话 client.log 的 filters=24 同形）", got)
	}
}

// APP_ID 编码：路径的 UTF-16LE 字节 + 结尾 U+0000，再做标准 base64。
//
// 这里的期望值是**本机实测**的 FwpmGetAppIdFromFileName0 返回值（见 spec.go 顶部记录）：
// 注意 API 返回的不是输入路径，而是内核规范化后的 `\device\harddiskvolumeN\...`。
// 所以 AppIDBytes 只负责"把给定路径按同一格式编码"，规范化由 API 负责 —— 正式安装
// 一律调用 API，这个函数只用于钉住形状与日志核对。
func TestAppIDBytesMatchMeasuredFormat(t *testing.T) {
	cases := []struct {
		name string
		path string
		want string
	}{
		{
			"cmd.exe（实测原值）",
			`\device\harddiskvolume3\windows\system32\cmd.exe`,
			"XABkAGUAdgBpAGMAZQBcAGgAYQByAGQAZABpAHMAawB2AG8AbAB1AG0AZQAzAFwAdwBpAG4AZABvAHcAcwBcAHMA" +
				"eQBzAHQAZQBtADMAMgBcAGMAbQBkAC4AZQB4AGUAAAA=",
		},
		{
			"explorer.exe（实测原值）",
			`\device\harddiskvolume3\windows\explorer.exe`,
			"XABkAGUAdgBpAGMAZQBcAGgAYQByAGQAZABpAHMAawB2AG8AbAB1AG0AZQAzAFwAdwBpAG4AZABvAHcAcwBcAGUA" +
				"eABwAGwAbwByAGUAcgAuAGUAeABlAAAA",
		},
	}
	for _, testCase := range cases {
		got := string(AppIDBytes(testCase.path))
		if got != testCase.want {
			t.Errorf("%s：AppIDBytes(%q)\n got %s\nwant %s", testCase.name, testCase.path, got, testCase.want)
		}
	}

	// 任意路径（含中文）的往返：解回 UTF-16LE 必须与原串 + U+0000 相同。
	path := `C:\游戏\115us\DFO\DFO.exe`
	raw, err := base64.StdEncoding.DecodeString(string(AppIDBytes(path)))
	if err != nil {
		t.Fatalf("AppIDBytes 不是合法 base64：%v", err)
	}
	if len(raw)%2 != 0 {
		t.Fatalf("APP_ID 字节数 = %d，应当是偶数（UTF-16）", len(raw))
	}
	units := make([]uint16, 0, len(raw)/2)
	for index := 0; index < len(raw); index += 2 {
		units = append(units, uint16(raw[index])|uint16(raw[index+1])<<8)
	}
	if len(units) != len([]rune(path))+1 || units[len(units)-1] != 0 {
		t.Fatalf("解码后的码元 = %v", units)
	}
	if got := string(runesFromUnits(units[:len(units)-1])); got != path {
		t.Errorf("往返后的路径 = %q，want %q", got, path)
	}

	// 结尾一定是 UTF-16LE 的 U+0000（两个 0 字节）——这是 WFP 认的 APP_ID 形状。
	tail := raw[len(raw)-2:]
	if tail[0] != 0 || tail[1] != 0 {
		t.Errorf("APP_ID 结尾 = %v，want 00 00", tail)
	}
}

// base64Encode 必须与标准库逐字节一致（自己写是为了让"无换行"这件事在代码里可见）。
func TestBase64EncodeMatchesStandardLibrary(t *testing.T) {
	for length := 0; length < 40; length++ {
		data := make([]byte, length)
		for index := range data {
			data[index] = byte(index*7 + length)
		}
		if got, want := string(base64Encode(data)), base64.StdEncoding.EncodeToString(data); got != want {
			t.Fatalf("长度 %d：base64Encode = %q，want %q", length, got, want)
		}
	}
}

// runesFromUnits 只用于测试里把 UTF-16 码元读回来。
func runesFromUnits(units []uint16) []rune {
	out := make([]rune, 0, len(units))
	for index := 0; index < len(units); index++ {
		unit := units[index]
		if unit >= 0xd800 && unit <= 0xdbff && index+1 < len(units) {
			low := units[index+1]
			if low >= 0xdc00 && low <= 0xdfff {
				out = append(out, rune(0x10000+(uint32(unit)-0xd800)<<10+(uint32(low)-0xdc00)))
				index++
				continue
			}
		}
		out = append(out, rune(unit))
	}
	return out
}

// 后缀判据（probe.cpp L115 的 .exe / .aes，大小写不敏感）。
func TestHasAppIDSuffix(t *testing.T) {
	yes := []string{"DFO.exe", `C:\a\b\CEF.EXE`, "BlackCipher64.aes", `x\y\Z.Aes`}
	no := []string{"DFO.exe.bak", "Script.pvf", "sk.dat", "aes", "readme.txt", ""}
	for _, path := range yes {
		if !HasAppIDSuffix(path) {
			t.Errorf("%q 应当被收进过滤器", path)
		}
	}
	for _, path := range no {
		if HasAppIDSuffix(path) {
			t.Errorf("%q 不该被收进过滤器", path)
		}
	}
}

// WalkApps 复刻 probe.cpp L114-L117 的递归扫描：只收普通文件、只认 .exe/.aes、
// 目录本身不进列表，顺序稳定（字典序）。
func TestWalkAppsRecursesAndFilters(t *testing.T) {
	root := t.TempDir()
	write := func(relative, body string) {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("DFO.exe", "a")
	write("Script.pvf", "b")
	write("CEF.exe", "c")
	write("BlackCipher/BlackCipher64.aes", "d")
	write("BlackCipher/readme.txt", "e")
	if err := os.MkdirAll(filepath.Join(root, "CEF.exe.d"), 0o755); err != nil {
		t.Fatal(err)
	}

	apps, err := WalkApps(root)
	if err != nil {
		t.Fatalf("WalkApps: %v", err)
	}
	var relative []string
	for _, app := range apps {
		rel, relErr := filepath.Rel(root, app)
		if relErr != nil {
			t.Fatal(relErr)
		}
		relative = append(relative, filepath.ToSlash(rel))
	}
	want := []string{"BlackCipher/BlackCipher64.aes", "CEF.exe", "DFO.exe"}
	sorted := append([]string{}, relative...)
	sort.Strings(sorted)
	if strings.Join(relative, ",") != strings.Join(want, ",") {
		t.Errorf("WalkApps = %v，want %v", relative, want)
	}
}

// NormalizeRoot 复刻 fs::absolute().lexically_normal()：相对路径按当前目录补齐，
// . / .. / 重复分隔符清掉，空串保持空串。
func TestNormalizeRoot(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ in, want string }{
		{"", ""},
		{"   ", ""},
		{`C:\Game\dof\115us\DFO`, `C:\Game\dof\115us\DFO`},
		{`C:\Game\..\Game\dof\115us\DFO\`, `C:\Game\dof\115us\DFO`},
		{"relative/dir", filepath.Join(cwd, "relative", "dir")},
	}
	for _, testCase := range cases {
		if got := NormalizeRoot(testCase.in); got != testCase.want {
			t.Errorf("NormalizeRoot(%q) = %q，want %q", testCase.in, got, testCase.want)
		}
	}
}

// Install 的错误分支：Spec 里的 Root 不存在也不该 panic，要么"装上了"（有权限时
// 也会因为拿不到 APP_ID 而失败），要么给出可读的错误 —— 两者都不能是"静默成功"。
func TestInstallNeverReportsSilentSuccess(t *testing.T) {
	spec := Spec{Root: filepath.Join(t.TempDir(), "does-not-exist"), WalkRoot: true}
	result, err := Install(spec)
	if result.Installed {
		if result.Handle == nil {
			t.Fatal("Installed=true 时 Handle 不能是 nil（否则隔离无法拆除）")
		}
		if closeErr := result.Handle.Close(); closeErr != nil {
			t.Errorf("Close: %v", closeErr)
		}
		if secondErr := result.Handle.Close(); secondErr != nil {
			t.Errorf("Close 必须幂等，第二次返回 %v", secondErr)
		}
		return
	}
	if err == nil {
		t.Fatal("没装上时必须给出错误（调用方据此回退 probe.exe）")
	}
	if result.Handle != nil {
		t.Error("没装上时不该有 Handle")
	}
	if result.Reason == "" {
		t.Error("没装上时必须写明原因")
	}
}

// IsUnavailable 区分"这个平台/这台机器装不了（该回退）"与别的一般错误。
func TestIsUnavailable(t *testing.T) {
	if IsUnavailable(nil) {
		t.Error("nil 不该算 unavailable")
	}
	if !IsUnavailable(UnavailableError("非 Windows")) {
		t.Error("UnavailableError 应当被认出")
	}
	if IsUnavailable(os.ErrNotExist) {
		t.Error("普通错误不该算 unavailable")
	}
	if got := UnavailableError("因为没权限").Error(); got != "因为没权限" {
		t.Errorf("UnavailableError.Error() = %q", got)
	}
}
