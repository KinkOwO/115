// Package wfpisolate 把「客户端只许走回环」的网络隔离从原生 probe.exe 搬进 Go：
// 用 WFP（Windows Filtering Platform，fwpuclnt.dll）按进程镜像路径装一组阻断过滤器，
// 拦掉非回环的出入站，然后再拉起客户端。
//
// 这一层是 probe.cpp L44-L63 的等价物（Guard 结构 + add()），语义逐条对齐：
//
//   - FwpmEngineOpen0：session.flags = FWPM_SESSION_FLAG_DYNAMIC、名称
//     "DFO isolated startup probe"、RPC_C_AUTHN_WINNT；动态会话的生命周期就是进程，
//     引擎句柄一关，本次装的子层与过滤器全部消失（probe.cpp L46 的析构）。
//   - 子层：UuidCreate 出来的随机 GUID、名称 "DFO probe paths only"、weight = 0xFFFF
//     （最高优先级，probe.cpp L50）。
//   - 过滤器：层 FWPM_LAYER_ALE_AUTH_CONNECT_V4/V6，动作 FWP_ACTION_BLOCK，
//     权重 FWP_UINT8 = 15，两个条件 —— FWPM_CONDITION_ALE_APP_ID 等于该镜像的
//     APP_ID blob，且 FWPM_CONDITION_FLAGS 未置 FWP_CONDITION_FLAG_IS_LOOPBACK
//     （probe.cpp L55-L60，匹配 FWP_MATCH_FLAGS_NONE_SET）。
//
// ⚠️ 已知差异（见 docs/go-launch-migration-plan.md 的 Stage 3「WFP Go 化」）：
// probe.exe 只装 ALE_AUTH_CONNECT_V4/V6 两层，没有装 ALE_AUTH_RECV_ACCEPT_V4/V6，
// 也没有显式的 permit 回环规则（回环是被「FLAGS 不含 IS_LOOPBACK」这个**排除条件**放过的，
// 不是被 permit 规则放过的）。Go 版保持一致 —— 不偷偷加层、也不偷偷放宽。
//
// 调用方必须按「宁可回退 probe.exe，也不让隔离静默失效」处理返回值：
// Install 返回 nil handle 表示隔离**没有**装成功，调用方要么回退 probe.exe，要么报错，
// 绝不能把它当成「隔离已生效」（probe.cpp 在这一步是优雅降级继续跑，见 clienthost.go）。
package wfpisolate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// appIDConditional 复刻 FwpmGetAppIdFromFileName0 编码出来的 APP_ID：路径的 UTF-16LE
// 字节（**含结尾的 U+0000**）做 base64。本机实测：
//
//	FwpmGetAppIdFromFileName0(`C:\Windows\System32\cmd.exe`) -> 98 字节，
//	5C 00 64 00 ... 65 00 00 00，即 "\device\harddiskvolume3\windows\system32\cmd.exe\0"
//	的 UTF-16LE，再 base64 成 XABkAGUAdgBpAGMAZQBcAGgAYQByAGQAZABpAHMAawB2AG8AbAB1AG0A
//	ZQAzAFwAdwBpAG4AZABvAHcAcwBcAHMAeQBzAHQAZQBtADMAMgBcAGMAbQBkAC4AZQB4AGUAAAA=
//
// 正式调用仍然走 FwpmGetAppIdFromFileName0（唯一权威）；这个编码只用于单元测试钉住形状，
// 以及在没有管理员权限、拿不到 WFP 句柄时给日志一个可核对的期望值。
// filterWeight 复刻 probe.cpp 的 FWP_UINT8 = 15（L58）。同一子层里每个层只有一条过滤器，
// 所以这个权重只在「同一个子层里还有别的过滤器」时才有意义；照抄是为了可对比。
const filterWeight uint8 = 15

// subLayerWeight 复刻 probe.cpp L50 的 sl.weight = 0xFFFF：让本次子层优先于其它子层，
// 这样 FWP_ACTION_BLOCK 的终止语义不会被更低优先级的 permit 抢在前面。
const subLayerWeight uint16 = 0xFFFF

// 复刻 probe.cpp L48/L50 的 displayData.name。两个名字都要留住：日志与 WFP 管理界面
// （netsh wfp show filters）里就靠它们认出「这是启动链装的隔离」。
const (
	engineName   = "DFO isolated startup probe"
	subLayerName = "DFO probe paths only"
	filterName   = "DFO probe block non-loopback"
)

// Spec 是一次隔离需要的全部输入。字段名与 probe.cpp 的实参一一对应：
// Root 是 argv[1]（客户端目录），ExtraApps 是 walk 出来的 .exe/.aes 加 probe.exe 自己。
type Spec struct {
	// Root：客户端目录（probe.cpp argv[1]，会先绝对化 + lexically_normal）。
	// 门禁「DFO.exe 不在里面就退出码 3」由宿主负责，Install 只负责网络隔离。
	Root string
	// ExtraApps：除 Root 之外还要拦的镜像（probe.exe 自己、以及别的辅助进程）。
	ExtraApps []string
	// WalkRoot 为真时递归扫描 Root，把 .exe/.aes（大小写不敏感）全部加进过滤器；
	// 等价于 probe.cpp L114-L117 的 recursive_directory_iterator。
	WalkRoot bool
}

// Layer 是一个 WFP 层：层 GUID + 要不要在里面装阻断过滤器。
// 层名只为日志与测试可读，不参与 WFP 调用。
type Layer struct {
	Name    string
	Key     GUID
	Install bool
}

// FilterLayer 是一条「层 + 动作 + 条件」的完整规格。AppID 为 nil 表示条件只有
// FWPM_CONDITION_FLAGS（probe.cpp 里不存在这种形态，留作显式表达）。
type FilterLayer struct {
	Name       string
	FilterName string
	Layer      GUID
	SubLayer   GUID
	Action     uint32
	WeightType uint32
	Weight     uint8
	// Conditions 按 probe.cpp 的顺序排列：ALE_APP_ID 在前，FLAGS 在后。
	Conditions []Condition
}

// Condition 是一条过滤器条件（FWPM_FILTER_CONDITION0 的可读投影）。
type Condition struct {
	FieldKey  GUID
	MatchType uint32
	ValueType uint32
	// AppID 非空时这条条件是「ALE_APP_ID 等于该 blob」（FWP_BYTE_BLOB_TYPE）。
	AppID []byte
	// Uint32 是 FWP_UINT32 条件的值（FLAGS 条件：0 表示"不含 IS_LOOPBACK"）。
	Uint32 uint32
}

// Installation 是一次 Install 的结果。Installed=false 时调用方必须回退（或报错），
// Reason 是给日志用的人话。
type Installation struct {
	// Installed 为真时 Handle 非 nil，且过滤器**确实**已经装进引擎。
	Installed bool
	// Reason：没装上时说明为什么（非 Windows、缺 fwpuclnt API、缺管理员权限、
	// FwpmEngineOpen0 失败、某个过滤器装失败……）。
	Reason string
	// Handle：Installed 时待关闭的隔离会话。
	Handle Handle
	// Apps 是实际拿到 APP_ID 并装了过滤器的镜像数（probe.exe 的 guard.filters/2）。
	Apps int
	// Filters 是实际装上的过滤器条数（每个镜像 × 装过滤器的层数）。
	Filters int
	// AppIDs 是镜像路径 -> APP_ID blob，按安装顺序；日志与排查用。
	AppIDs []AppEntry
}

// AppEntry 是一个镜像与它的 APP_ID。
type AppEntry struct {
	Path  string
	AppID []byte
}

// Handle 是一次已生效的隔离。Close 幂等：重复调用不报错，只关一次。
//
// 关闭顺序与析构等价：先删本次装的过滤器（动态会话下由引擎负责，但显式删更可控），
// 再去掉子层，最后关引擎会话。任何一步失败都继续往下走 —— 漏关引擎句柄才是真正
// 会留下残留过滤器的那一条。
type Handle interface {
	Close() error
}

// NetSelfTest 复刻 probe.cpp 的 net_check()（L64-L73）：先确认回环连通，再确认去
// 外部地址被明确拒绝（WSAEACCES）。两个布尔量的口径：
//
//   - Loopback：回环 TCP 连接成功。假 = 隔离把回环也拦了（对启动链是致命的）。
//   - RemoteDenied：非回环连接以 WSAEACCES 失败。假 = 隔离没生效或环境本身拦截了。
//
// 它只是自检：probe.cpp 在自检失败时也只记一行日志、照旧拉起客户端（L129），
// Go 版沿用这个语义 —— 但调用方可以在「隔离装上了、自检却失败」时自行决定更严的策略。
type NetSelfTest struct {
	Loopback     bool
	RemoteDenied bool
	// Detail 是人话结论，用来写 client.log 的那一行。
	Detail string
	// Errno 是外部连接失败时的 Winsock 错误码（0 表示连接居然成功了）。
	Errno int
}

// AppIDPath 是 APP_ID 覆盖到的文件后缀（小写）。probe.cpp L115 只看 .exe 与 .aes。
var appIDPathSuffixes = []string{".exe", ".aes"}

// HasAppIDSuffix 报告这个路径是否会被收进过滤器（大小写不敏感，复刻 L115 的 towlower）。
func HasAppIDSuffix(path string) bool {
	lowered := strings.ToLower(path)
	for _, suffix := range appIDPathSuffixes {
		if strings.HasSuffix(lowered, suffix) {
			return true
		}
	}
	return false
}

// NormalizeRoot 复刻 probe.cpp L106 的 `fs::absolute(argv[1]).lexically_normal()`：
// 相对路径按当前工作目录补全，然后规范化（不吃软链接，只清 . / .. 与重复分隔符）。
// 出错时退回 TrimSpace 后的原串，让 WFP 调用自己去报错。
func NormalizeRoot(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	absolute, err := filepath.Abs(trimmed)
	if err != nil {
		return trimmed
	}
	return filepath.Clean(absolute)
}

// WalkApps 复刻 probe.cpp L114-L117 的递归扫描：Root 下所有 .exe / .aes 的**绝对路径**，
// 顺序与 filepath.WalkDir 的字典序一致，且跳过无法读取的目录（probe 的迭代器会在
// 权限不足时抛异常，这里用跳过换稳定：宁可少拦一个读不到的目录，也不要整个隔离装不上）。
//
// 只收普通文件（复刻 L115 的 is_regular_file），目录与重解析点不跟。
func WalkApps(root string) ([]string, error) {
	var apps []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		if !HasAppIDSuffix(path) {
			return nil
		}
		apps = append(apps, path)
		return nil
	})
	if err != nil {
		return apps, err
	}
	return apps, nil
}

// BuildFilterLayers 复刻 probe.cpp L53-L61 的 add()：把一个 APP_ID 变成一条过滤器规格。
//
// appID 是 FwpmGetAppIdFromFileName0 的产物（UTF-16LE + 结尾 NUL 的 base64 字节，
// 见 AppIDBytes）。返回的规格里每条过滤器的 Conditions 顺序、权重、动作都与 probe 一致。
func BuildFilterLayers(subLayer GUID, appID []byte) []FilterLayer {
	layers := []FilterLayer{}
	for _, layer := range FilterLayers() {
		if !layer.Install {
			continue
		}
		layers = append(layers, FilterLayer{
			Name:       layer.Name,
			FilterName: filterName,
			Layer:      layer.Key,
			SubLayer:   subLayer,
			Action:     ActionBlock,
			WeightType: DataTypeUint8,
			Weight:     filterWeight,
			Conditions: []Condition{
				{
					FieldKey:  ConditionALEAppID,
					MatchType: MatchEqual,
					ValueType: DataTypeByteBlob,
					AppID:     appID,
				},
				{
					// FWP_MATCH_FLAGS_NONE_SET + 0：只匹配"FLAGS 里没有
					// IS_LOOPBACK 这一位"的流量，即非回环 —— 这就是 probe.cpp
					// 放过回环的全部机制（没有 permit 规则）。
					FieldKey:  ConditionFlags,
					MatchType: MatchFlagsNoneSet,
					ValueType: DataTypeUint32,
					Uint32:    ConditionFlagIsLoopback,
				},
			},
		})
	}
	return layers
}

// FilterLayers 是 probe.cpp L57 那一对层，外加**没有**装的接收层（Install=false）。
//
// 把没装的层也列出来是故意的：这一行就是「已知差异」的可执行证据 —— 谁想改口径
// 必须在这里显式把 Install 改成 true，而不是在别处顺手加一层。
func FilterLayers() []Layer {
	return []Layer{
		{Name: "FWPM_LAYER_ALE_AUTH_CONNECT_V4", Key: LayerALEAuthConnectV4, Install: true},
		{Name: "FWPM_LAYER_ALE_AUTH_CONNECT_V6", Key: LayerALEAuthConnectV6, Install: true},
		{Name: "FWPM_LAYER_ALE_AUTH_RECV_ACCEPT_V4", Key: LayerALEAuthRecvAcceptV4, Install: false},
		{Name: "FWPM_LAYER_ALE_AUTH_RECV_ACCEPT_V6", Key: LayerALEAuthRecvAcceptV6, Install: false},
	}
}

// FilterSummary 是日志与测试要的那一句「装了什么」：条数算得出来，口径写死在这。
// probe.exe 对应的输出是 `WFP_READY filters=<N> NON_LOOPBACK_BLOCKED IPV4_IPV6`
// （L119），N = 镜像数 × 2。
func FilterSummary(apps int) string {
	return fmt.Sprintf("filters=%d NON_LOOPBACK_BLOCKED IPV4_IPV6", apps*installedLayerCount())
}

// installedLayerCount 是真正装过滤器的层数（当前为 2：V4/V6 的 ALE_AUTH_CONNECT）。
func installedLayerCount() int {
	count := 0
	for _, layer := range FilterLayers() {
		if layer.Install {
			count++
		}
	}
	return count
}

// UnavailableError 覆盖「这台机器/这个平台根本没法装隔离」的三种情形，
// 调用方一律回退 probe.exe。Reason 是给日志的人话。
func UnavailableError(reason string) error {
	return &unavailableError{reason: reason}
}

type unavailableError struct{ reason string }

func (e *unavailableError) Error() string { return e.reason }

// IsUnavailable 报告 err 是不是「没法装隔离（应当回退 probe.exe）」而不是「装失败了」。
// 两者在调用方那里的处置不同：前者回退，后者同样回退但日志要写明是失败。
func IsUnavailable(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*unavailableError)
	return ok
}
