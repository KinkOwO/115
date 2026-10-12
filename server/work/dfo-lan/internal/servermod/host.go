// Package servermod 是服务端层的 **mod 宿主**（四层架构里的 server 层）。
//
// # 为什么要一个宿主包
//
// 四层 mod 里只有 server 层会改变**服务端进程本身的行为**。服务端是就地 go build 的
// （启动器 internal/serverbuild），所以服务端 mod 的本质是"一个参与编译的 Go 包 +
// 在这里注册的钩子"——这正是业主定调的 B 路线：
//
//	mod 包（mods/<id>/）──编译进二进制──> 在 init()/Register() 里调用本包注册钩子
//	服务端在固定时机调用这些钩子 ────────> mod 的行为生效
//
// 类型正确性由 Go 编译器保证：钩子签名写错，服务端根本编不出来。所以**不存在**
// "装了但不生效"的静默失败——这是 B 相对"字符串签名匹配"的核心优势。
//
// # 谁调用什么
//
//   - `mods/zz_mods_gen.go`（由 modkit 生成）：import 每个已装 mod，调用它们的 Register()；
//   - mod 的 Register()：调用本包的 RegisterBoot / RegisterConsole 登记回调；
//   - `cmd/wireprobe`：在固定时机调用 Boot / Console / Response 三个触发函数。
//
// # 边界（刻意做小）
//
// 宿主机操作**全部是名字取用、不是类型共享**：这些操作函数是 mod 与服务端之间
// 的稳定契约，加一个等于承诺它的语义长期稳定，所以宁缺勿滥。
//
// 注意：mod 包与内核同属 dfolan 模块，Go 层面**允许**它 import 服务端内部包
// （现成例子：mods/odyssey.hardcore 就 import 了 internal/modpolicy）。那是
// "行为覆盖"的既有通道（见 internal/modpolicy），不是本文件要管的事；本文件管的是
// **钩子点**——即"服务端在什么时机回头调用 mod"。
package servermod

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
)

// Host 操作的名单——与启动器侧 modkit.HostOpNames() 必须一致。
// 两侧任一方新增操作，另一方要同步；不一致时 modkit 的清单校验会先拒绝清单。
const (
	OpLog             = "log"
	OpConfigRead      = "config.read"
	OpConfigWrite     = "config.write"
	OpRegisterContent = "content.register"
	OpConsoleReply    = "console.reply"
	// OpRegisterReward 对应的宿主机操作名（与 modkit.HostOpRegisterReward 同名）。
	OpRegisterReward = "reward.register"
	// OpReplySend 是"按连接发一条报文"的操作名。它只对 protocol.request 钩子开放：
	// 短路的 mod 必须能自己应答客户端，否则钩子等于残废（见 RequestContext.Reply）。
	OpReplySend = "reply.send"
)

// 钩子点名——与启动器侧 modkit.HookPoints 必须一致。
const (
	HookBoot             = "server.boot"
	HookConsoleCommand   = "console.command"
	HookProtocolResponse = "protocol.response"
	// HookRewardScript 由 mod 在启动装配阶段登记事件奖励规则时声明。
	// 它与 modkit 的 HookPoints["reward.script"] 必须同名。
	HookRewardScript = "reward.script"
	// HookProtocolRequest 是**请求侧**接入点：每收到一帧客户端报文、在内置分发之前
	// 调用一次。它给 mod 的是"实现新玩法"的能力——返回 handled=true 即短路，
	// 由 mod 自己经 RequestContext.Reply 应答（见该类型的注释）。
	//
	// 它与 HookProtocolResponse 的分工是刻意的：
	//   - protocol.request  在**请求**入口短路，mod 自己产生应答 → 不走旁路改写；
	//   - protocol.response 在**应答**出口只读观察，保住协议取证链。
	HookProtocolRequest = "protocol.request"
	// HookShutdown 是 server.boot 的**逆操作**：撤销 boot 改过的进程级状态
	// （模式策略、全局表、后台 goroutine 等）。
	//
	// 为什么要有它：启用/禁用已经是运行期门禁，但"关掉一个 mod"如果只让它不再被调用，
	// 它 boot 时改走的东西还留在进程里——玩家看到的是"关了但效果还在"。
	// 只有注册了 shutdown 的 mod，宿主才敢在运行中把它**再打开**（重跑 boot 才有意义）；
	// 没注册的，Reconcile 会拒绝热开并明确写成"需重启"，不假装能热切换。
	//
	// 与 mod.json 的声明核对：本钩子**不参与** CheckDeclarations 的双向比对
	// （它不是功能入口，而是 boot 的收尾），所以 mod 注册它不需要清单里先声明；
	// 等启动器侧 modkit 的 HookPoints 白名单认了这个名字，再纳入核对。
	HookShutdown = "server.shutdown"
)

// BootContext 是 server.boot 钩子拿到的上下文。
type BootContext struct {
	// Version 是服务端构建版本串（由 main 注入，便于 mod 判断自身兼容性）。
	Version string
	// ChannelCount 是本进程将要开放的频道数（mod 只读）。
	ChannelCount int
}

// ConsoleCommand 是一条控制台命令。
type ConsoleCommand struct {
	// ModID 是发起命令的 mod（由宿主按注册表填）。
	ModID string
	// Name 是子命令名（操作者输入 `mod <mod-id> <name> ...`）。
	Name string
	// Args 是子命令之后的参数。
	Args []string
}

// BootHook 是 server.boot 钩子签名。
//
// 返回 error 表示 mod 自检失败：宿主会**拒绝启动**（fail-closed）。这是刻意的：
// 一个自检不过的 mod 安静地不生效，比启动失败更难查。
type BootHook func(ctx *BootContext) error

// ShutdownReason 说明为什么请 mod 收尾。
type ShutdownReason string

const (
	// ReasonDisabled 是运行中被停用（enabled.json 取消勾选）。
	ReasonDisabled ShutdownReason = "disabled"
	// ReasonStopping 是服务端进程正在退出。
	ReasonStopping ShutdownReason = "stopping"
)

// ShutdownContext 是 server.shutdown 钩子拿到的上下文。
type ShutdownContext struct {
	// Reason 是本次收尾的原因。
	Reason ShutdownReason
	// Version 与 BootContext.Version 同义，便于 mod 判断自己面对的是哪份服务端。
	Version string
}

// ShutdownHook 是 server.shutdown 钩子签名，负责撤销 boot 改过的进程级状态。
//
// 它**不要求**mod 清理自己登记的钩子——那是宿主的事（宿主按 live 集合停止派发）。
// mod 要清的是它 boot 时动过的外部状态：模式策略、注册过的内容、后台 goroutine。
//
// 返回 error：收尾失败不致命（服务端不能因为一个被关掉的 mod 退出），
// 但一定记日志，并把该 mod 标成"boot 副作用可能未撤销"，提示重启收口。
type ShutdownHook func(ctx *ShutdownContext) error

// ConsoleHook 是 console.command 钩子签名。
// 返回 handled=true 表示这条命令由该 mod 处理（宿主不再转发给其它 mod）。
type ConsoleHook func(cmd ConsoleCommand) (handled bool, err error)

// ResponseHook 是 protocol.response 钩子签名。
//
// 这是**只读观察点**：给 mod 一份即将写入客户端的报文副本用于记录/统计。
// 第一期**不允许** mod 改写报文（没有任何返回值能改它）——协议改写必须走
// 服务端自身的代码路径，不能从 mod 侧旁路，否则协议取证链就断了。
type ResponseHook func(conn string, opcode uint16, body []byte)

// RequestContext 是 protocol.request 钩子拿到的上下文。
//
// 它刻意是**只读 + 一个应答口**：mod 要么放行（返回 handled=false，交给内置分发），
// 要么整条接手（返回 handled=true，再用 Reply 自己应答）。
//
// 为什么不给"就地改写 Plaintext"的能力：那等于把重算校验和与重加密搬进宿主，
// 协议真源就会从"服务端自身代码路径 + IDA/实机取证"退化成"某个 mod 的猜测"。
// 要改行为就让 mod 自己接手这条报文——归属清楚、日志可归因、取证链不断。
type RequestContext struct {
	// Conn 是连接标识（peer），用于归因与多连接隔离。
	Conn string
	// Type 是帧类型，ID 是 CMD/NOTI 号。
	Type byte
	ID   uint16
	// Raw 是原始帧（只读）。
	Raw []byte
	// Plaintext 是解密后的正文（只读；已过校验和门时 Verified 为真）。
	Plaintext []byte
	// Verified 表示该帧校验和是否通过。
	Verified bool
	// Reply 是 reply.send 操作：按**本连接**发一条报文。
	//
	// 只有在钩子返回 handled=true 之后才应该用它——放行的报文由内置分发应答。
	Reply func(kind byte, id uint16, payload []byte) error
}

// RequestHook 是 protocol.request 钩子签名。
//
// 返回 handled=true 表示这条报文由该 mod 接手，内置分发表不再看它。
// 返回 error 表示处理失败：宿主记日志并把它算作该帧失败，不影响其它连接。
type RequestHook func(ctx *RequestContext) (handled bool, err error)

type bootReg struct {
	modID string
	order int
	fn    BootHook
}

type consoleReg struct {
	modID string
	fn    ConsoleHook
}

type responseReg struct {
	modID string
	fn    ResponseHook
}

type requestReg struct {
	modID string
	fn    RequestHook
}

type shutdownReg struct {
	modID string
	fn    ShutdownHook
}

var (
	mu        sync.RWMutex
	seq       int
	boots     []bootReg
	consoles  []consoleReg
	responses []responseReg
	requests  []requestReg
	shutdowns []shutdownReg

	// live 是运行期门禁的唯一判据："这个 mod 现在在不在跑"。
	//
	// 它与 enabled.json 的差别就在于**迁移可能被拒绝**：清单说启用，但这个 mod
	// 的 boot 没有配 shutdown，宿主不敢重跑 boot，于是它"启用而未生效"。
	// 首次登记时按清单初始化（见 markRegisteredLocked），之后只由 Reconcile 改。
	live map[string]bool

	// dirtyBoot 记下"关掉时没能撤销 boot 副作用"的 mod（没有注册 shutdown 的）。
	// 它决定了热开方向要不要被拒绝：脏状态下重跑 boot 等于把状态改两遍。
	dirtyBoot map[string]bool

	// lastNote 是最近一次状态迁移的说明，随 mods-status.json 落盘，供界面显示。
	lastNote map[string]string

	// modValues 是 config.write 的进程内落地位置：键空间由白名单约束。
	modValues = map[string]string{}
	// configKeys 是 host 在启动时注入的"有效配置只读快照"。
	configKeys = map[string]string{}

	// registeredIDs 记录已注册的 mod id，用于诊断输出。
	registeredIDs = map[string]bool{}

	// consoleOut 是 console.reply 的落点（默认标准输出）。
	consoleOut = func(s string) { fmt.Fprintln(os.Stdout, s) }
)

// markRegisteredLocked 登记"这个 mod 存在"，并在首次登记时按清单决定它的初始 live 状态。
//
// on 必须由调用方在**拿 mu 之前**从清单算好：Enabled() 用的是另一把锁，
// 在持 mu 时调用会把锁序变成 mu→enabledMu，与 Reconcile 的 enabledMu→mu 相反。
//
// 调用方必须已持有 mu。
func markRegisteredLocked(modID string, on bool) {
	registeredIDs[modID] = true
	if live == nil {
		live = map[string]bool{}
	}
	if _, seen := live[modID]; !seen {
		live[modID] = on
	}
}

// RegisterShutdown 登记一个 server.shutdown 钩子（boot 的逆操作）。
//
// 一个 mod 最多登记一个：boot 的副作用只有它自己知道怎么撤销，登记两份只会让
// 收尾顺序变成谜。重复登记且函数不同 = 作者错误，直接 panic（与其余 Register 一致）。
func RegisterShutdown(modID string, fn ShutdownHook) {
	if fn == nil {
		panic("servermod: RegisterShutdown 收到 nil 回调")
	}
	if !validModID(modID) {
		panic("servermod: 非法 mod id: " + modID)
	}
	on := Enabled(modID)
	mu.Lock()
	defer mu.Unlock()
	for _, s := range shutdowns {
		if s.modID == modID {
			panic("servermod: mod " + modID + " 重复登记 server.shutdown 钩子")
		}
	}
	shutdowns = append(shutdowns, shutdownReg{modID: modID, fn: fn})
	markRegisteredLocked(modID, on)
}

// HasBoot 报告某个 mod 是否登记了 server.boot 钩子。
func HasBoot(modID string) bool {
	mu.RLock()
	defer mu.RUnlock()
	for _, b := range boots {
		if b.modID == modID {
			return true
		}
	}
	return false
}

// HasShutdown 报告某个 mod 是否登记了 server.shutdown（boot 的逆操作）。
func HasShutdown(modID string) bool {
	mu.RLock()
	defer mu.RUnlock()
	for _, s := range shutdowns {
		if s.modID == modID {
			return true
		}
	}
	return false
}

// RegisterBoot 登记一个 server.boot 钩子。mod 在自己的 Register() 里调用。
//
// modID 用 mod 自身常量传进来（引擎的 import 清单已经保证了"哪个包属于哪个 id"，
// 这里再显式传一次是为了审计日志能直接指认来源）。
func RegisterBoot(modID string, fn BootHook) {
	if fn == nil {
		panic("servermod: RegisterBoot 收到 nil 回调")
	}
	if !validModID(modID) {
		panic("servermod: 非法 mod id: " + modID)
	}
	on := Enabled(modID)
	mu.Lock()
	defer mu.Unlock()
	seq++
	boots = append(boots, bootReg{modID: modID, order: seq, fn: fn})
	markRegisteredLocked(modID, on)
}

// RegisterConsole 登记一个 console.command 钩子。
func RegisterConsole(modID string, fn ConsoleHook) {
	if fn == nil {
		panic("servermod: RegisterConsole 收到 nil 回调")
	}
	if !validModID(modID) {
		panic("servermod: 非法 mod id: " + modID)
	}
	on := Enabled(modID)
	mu.Lock()
	defer mu.Unlock()
	consoles = append(consoles, consoleReg{modID: modID, fn: fn})
	markRegisteredLocked(modID, on)
}

// RegisterResponse 登记一个 protocol.response 观察钩子。
func RegisterResponse(modID string, fn ResponseHook) {
	if fn == nil {
		panic("servermod: RegisterResponse 收到 nil 回调")
	}
	if !validModID(modID) {
		panic("servermod: 非法 mod id: " + modID)
	}
	on := Enabled(modID)
	mu.Lock()
	defer mu.Unlock()
	responses = append(responses, responseReg{modID: modID, fn: fn})
	markRegisteredLocked(modID, on)
}

// RegisterRequest 登记一个 protocol.request 钩子。
//
// 钩子按**注册顺序**依次询问，第一个返回 handled=true 的接手这条报文
// （顺序稳定 = 谁的日志在前谁先看到，多 mod 行为可复现）。
func RegisterRequest(modID string, fn RequestHook) {
	if fn == nil {
		panic("servermod: RegisterRequest 收到 nil 回调")
	}
	if !validModID(modID) {
		panic("servermod: 非法 mod id: " + modID)
	}
	on := Enabled(modID)
	mu.Lock()
	defer mu.Unlock()
	requests = append(requests, requestReg{modID: modID, fn: fn})
	markRegisteredLocked(modID, on)
}

// Registered 返回已注册的 mod id（稳定排序），供启动日志与状态查询。
func Registered() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registeredIDs))
	for id := range registeredIDs {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// SetEnvSnapshot 由 main 在装配完成后注入"环境变量只读快照"。
//
// 为什么用环境变量而不是服务端 Config 结构体：mod 的 config.read/config.write 走的是
// **名字取用**（避免 mod 依赖服务端内部类型）。环境变量是这套服务端既有、稳定、且
// mod 与操作者都能看懂的名字空间（`DFO_*`），因此拿它当只读快照最合适。
//
// 只有 `DFO_` 前缀的键进快照：其余环境变量（PATH 等）与内容无关，不给 mod 看。
func SetEnvSnapshot(environ []string) {
	mu.Lock()
	defer mu.Unlock()
	for _, kv := range environ {
		i := strings.IndexByte(kv, '=')
		if i <= 0 {
			continue
		}
		key := kv[:i]
		if !strings.HasPrefix(key, "DFO_") {
			continue
		}
		configKeys[key] = kv[i+1:]
	}
}

// SetConfigSnapshot 由 main 在装配完成后注入"有效配置只读快照"。
// 键空间就是这里给的集合：mod 只能读到这里列的键。与 SetEnvSnapshot 累加。
func SetConfigSnapshot(kv map[string]string) {
	mu.Lock()
	defer mu.Unlock()
	if configKeys == nil {
		configKeys = map[string]string{}
	}
	for k, v := range kv {
		configKeys[k] = v
	}
}

// Boot 在服务端完成配置与存储初始化、开始监听之前调用。
//
// 只跑 live（= 清单里启用）的 mod；被禁用的 mod 的钩子本来就不该动全局状态。
// 任何一个 mod 的 boot 钩子返回错误 → 整体返回错误 → 服务端拒绝启动。
// 这是"宁可起不来也不要静默半残"的口径。
func Boot(ctx *BootContext) error {
	mu.RLock()
	list := make([]bootReg, 0, len(boots))
	for _, b := range boots {
		if live[b.modID] {
			list = append(list, b)
		}
	}
	mu.RUnlock()
	if len(list) == 0 {
		return nil
	}
	sort.Slice(list, func(i, j int) bool { return list[i].order < list[j].order })
	for _, b := range list {
		if err := b.fn(ctx); err != nil {
			return fmt.Errorf("mod %s 的 %s 钩子自检失败：%w", b.modID, HookBoot, err)
		}
		log.Printf("servermod: mod %s 已就绪（%s）", b.modID, HookBoot)
	}
	return nil
}

// runBootHook 重跑某个 mod 的**全部** boot 钩子（按登记顺序）。
//
// 给 Reconcile 在运行中"打开一个 mod"时用。它刻意不整体替换 Boot()：
// 启动期一个钩子失败要拒绝启动，运行期一个钩子失败只要把这个 mod 退回关闭状态，
// 两者处置不同，但**跑的是同一份 mod 代码**。
func runBootHook(modID string, ctx *BootContext) error {
	mu.RLock()
	list := make([]bootReg, 0, 2)
	for _, b := range boots {
		if b.modID == modID {
			list = append(list, b)
		}
	}
	mu.RUnlock()
	sort.Slice(list, func(i, j int) bool { return list[i].order < list[j].order })
	for _, b := range list {
		if err := b.fn(ctx); err != nil {
			return fmt.Errorf("mod %s 的 %s 钩子失败：%w", modID, HookBoot, err)
		}
	}
	return nil
}

// runShutdownHook 请某个 mod 撤销它的 boot 副作用（没有注册 shutdown 钩子时是空操作）。
func runShutdownHook(modID string, ctx *ShutdownContext) error {
	mu.RLock()
	list := make([]shutdownReg, 0, 1)
	for _, s := range shutdowns {
		if s.modID == modID {
			list = append(list, s)
		}
	}
	mu.RUnlock()
	for _, s := range list {
		if err := s.fn(ctx); err != nil {
			return fmt.Errorf("mod %s 的 %s 钩子失败：%w", modID, HookShutdown, err)
		}
	}
	return nil
}

// ShutdownLive 在服务端退出前请所有仍在跑的 mod 收尾（ReasonStopping）。
//
// 单个 mod 收尾失败只记日志、不阻断退出：进程正在关闭，把玩家卡在这里没有意义。
// 与 Reconcile 共用一把串行锁，所以不会和运行中的状态迁移交叉跑钩子。
func ShutdownLive(version string) {
	transitionMu.Lock()
	defer transitionMu.Unlock()
	for _, modID := range liveModIDs() {
		// 先停止派发，再收尾——与 disableMod 同一条顺序纪律：收尾期间不该还有
		// 报文回调进这个 mod。进程本来也要退出了，但重复调用（defer + 显式收尾）
		// 必须只会收尾一次，live 置假就是那个闩。
		setLive(modID, false)
		if err := runShutdownHook(modID, &ShutdownContext{Reason: ReasonStopping, Version: version}); err != nil {
			log.Printf("servermod: mod %s 退出收尾失败：%v", modID, err)
		}
	}
}

// Console 把一条 `mod <id> <name> ...` 命令分发给已登记的 mod。
// 返回 handled=false 表示没有 mod 认领（调用方可以打印帮助）。
//
// 被停用的 mod 不再被派发；但如果操作者点的**就是**这个 mod 的命令，这里明确报
// "已停用"而不是"未被认领"——后者会让人以为命令名打错了，白查一轮。
func Console(modID, name string, args []string) (bool, error) {
	mu.RLock()
	list := make([]consoleReg, 0, len(consoles))
	var stopped string
	for _, c := range consoles {
		if modID != "" && !strings.EqualFold(c.modID, modID) {
			continue
		}
		if !live[c.modID] {
			if stopped == "" {
				stopped = c.modID
			}
			continue
		}
		list = append(list, c)
	}
	mu.RUnlock()
	if stopped != "" && len(list) == 0 {
		return true, fmt.Errorf("mod %s 已停用（enabled.json 的禁用名单），命令 %q 没有派发", stopped, name)
	}
	for _, c := range list {
		handled, err := c.fn(ConsoleCommand{ModID: c.modID, Name: name, Args: args})
		if err != nil {
			return true, fmt.Errorf("mod %s 的 %s 命令 %q 执行失败：%w", c.modID, HookConsoleCommand, name, err)
		}
		if handled {
			return true, nil
		}
	}
	return false, nil
}

// ConsoleHelp 汇总所有已登记 mod 声明的命令名，供 `mod help` 与启动日志使用。
//
// 命令名从**注册时**抓取：注册的闭包不认识"命令表"这种概念，所以这里让 mod 在
// RegisterConsoleHelp 里显式声明（见那个函数）。空表示没有 mod 声明用途。
func ConsoleHelp() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(consoleHelp))
	for _, h := range consoleHelp {
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}

// consoleHelp 记录每个 mod 声明的命令用途（mod 在注册时调用 RegisterConsoleHelp）。
var consoleHelp []string

// RegisterConsoleHelp 让 mod 声明"我认领哪些子命令、分别做什么"，供 `mod help` 列出。
//
// 为什么不让引擎从回调里推：回调是个不透明函数，引擎无法知道它认领哪些名字。
// 显式声明一次，操作者就能在服务端日志里看到"这个 mod 能干什么"，
// 而不必去读 mod 源码。
func RegisterConsoleHelp(modID, usage string) {
	if !validModID(modID) {
		panic("servermod: 非法 mod id: " + modID)
	}
	usage = strings.TrimSpace(usage)
	if usage == "" {
		return
	}
	on := Enabled(modID)
	mu.Lock()
	defer mu.Unlock()
	consoleHelp = append(consoleHelp, modID+": "+usage)
	markRegisteredLocked(modID, on)
}

// RunConsoleOnce 执行一次"启动期命令"（由 main 从环境变量/参数取到后调用一次）。
//
// # 为什么是"一次性"而不是交互式控制台
//
// 服务端由启动器以无控制台方式拉起（`proc.CommandContext` + 隐藏窗口），
// 所以它没有可交互的 stdin。硬加一个读 stdin 的循环，在玩家那台机器上等于
// 挂一个永远阻塞的 goroutine，还会和"隐藏窗口"的取向冲突。
//
// 一次性执行则完全确定：操作者/脚本在启动前设好命令，服务端起来就执行一次、
// 把结果写进日志。既是验收通路，也能用在自动化里。
func RunConsoleOnce(spec string) error {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil
	}
	fields := strings.Fields(spec)
	if len(fields) == 0 {
		return nil
	}
	// 语法：`<mod-id> <name> [args...]`；第一个字段为 "help" 时列出已登记命令。
	if strings.EqualFold(fields[0], "help") {
		for _, h := range ConsoleHelp() {
			log.Printf("servermod: mod 命令 %s", h)
		}
		if len(ConsoleHelp()) == 0 {
			log.Print("servermod: 当前没有 mod 声明任何控制台命令")
		}
		return nil
	}
	modID := fields[0]
	name := "run"
	args := []string{}
	if len(fields) > 1 {
		name = fields[1]
		args = fields[2:]
	}
	handled, err := Console(modID, name, args)
	if err != nil {
		return err
	}
	if !handled {
		return fmt.Errorf("mod 命令未被认领：mod=%s name=%s（可用 modkit layers 查看引擎接口；"+
			"用 DFO_SERVERMOD_CONSOLE=help 列出已登记命令）", modID, name)
	}
	return nil
}

// ObserveResponse 把一条即将写入客户端的报文副本交给已启用（live）的响应观察者。
//
// 这个函数在每条 S2C 报文上都会被调用一次，所以它必须**极便宜**：
// 没有 live 观察者时直接返回，不做任何分配。被停用的 mod 不进这里（派发门禁）。
func ObserveResponse(conn string, opcode uint16, body []byte) {
	mu.RLock()
	if len(responses) == 0 {
		mu.RUnlock()
		return
	}
	list := make([]responseReg, 0, len(responses))
	for _, r := range responses {
		if live[r.modID] {
			list = append(list, r)
		}
	}
	mu.RUnlock()
	for _, r := range list {
		r.fn(conn, opcode, body)
	}
}

// HasObservers 报告是否有 **live** 的响应观察者（调用方据此决定要不要复制 body）。
func HasObservers() bool {
	mu.RLock()
	defer mu.RUnlock()
	for _, r := range responses {
		if live[r.modID] {
			return true
		}
	}
	return false
}

// HasRequestHooks 报告是否登记了任何 **live** 的请求钩子。
//
// 每帧都会问一次，所以它必须极便宜：没有钩子时调用方直接跳过，不做任何分配。
func HasRequestHooks() bool {
	mu.RLock()
	defer mu.RUnlock()
	for _, r := range requests {
		if live[r.modID] {
			return true
		}
	}
	return false
}

// ObserveRequest 把一条**客户端请求**交给所有已启用（live）的请求钩子。
//
// 返回 handled=true 表示某个 mod 接手了这条报文：调用方**必须跳过内置分发**。
// 钩子返回 error 时记一行日志并继续问下一个——一个 mod 出错，不该让别的 mod
// 与内置分发跟着一起失效。
//
// 短路一定打日志：否则"哪条报文被哪个 mod 吃掉了"在排障时无从复原。
func ObserveRequest(conn string, typ byte, id uint16, raw, plaintext []byte, verified bool,
	reply func(kind byte, id uint16, payload []byte) error) bool {
	mu.RLock()
	if len(requests) == 0 {
		mu.RUnlock()
		return false
	}
	list := make([]requestReg, 0, len(requests))
	for _, r := range requests {
		if live[r.modID] {
			list = append(list, r)
		}
	}
	mu.RUnlock()
	if len(list) == 0 {
		return false
	}
	ctx := &RequestContext{
		Conn: conn, Type: typ, ID: id, Raw: raw, Plaintext: plaintext,
		Verified: verified, Reply: reply,
	}
	for _, r := range list {
		handled, err := r.fn(ctx)
		if err != nil {
			Logf(r.modID, "处理请求 type=%d id=%d 失败：%v", typ, id, err)
			continue
		}
		if handled {
			Logf(r.modID, "短路请求 type=%d id=%d（连接 %s）", typ, id, conn)
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 宿主操作：mod 只能通过这 5 个名字取用服务端能力
// ---------------------------------------------------------------------------

// Logf 是 log 操作的实现：写入服务端日志，并统一带上 mod 前缀便于归因。
func Logf(modID, format string, args ...any) {
	log.Printf("[mod %s] %s", modID, fmt.Sprintf(format, args...))
}

// ConfigRead 是 config.read 操作的实现：读"有效配置只读快照"。
func ConfigRead(key string) (string, bool) {
	mu.RLock()
	defer mu.RUnlock()
	v, ok := configKeys[key]
	return v, ok
}

// ConfigWrite 是 config.write 操作的实现：写**进程内**的 mod 值空间。
//
// 刻意不落盘：mod 不应该靠改服务端配置文件来生效（那会让"这次启动做了什么"
// 无法从注册表与日志复原）。键必须是已登记的键，否则拒绝——防止 mod 凭空造配置项。
func ConfigWrite(key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("config.write 的键不能为空")
	}
	mu.Lock()
	defer mu.Unlock()
	if _, known := configKeys[key]; !known {
		return fmt.Errorf("config.write 拒绝未登记的键 %q：只允许改启动快照里已有的键", key)
	}
	modValues[key] = value
	return nil
}

// ModValue 读回 config.write 写入的进程内值（供 mod 自己与诊断使用）。
func ModValue(key string) (string, bool) {
	mu.RLock()
	defer mu.RUnlock()
	v, ok := modValues[key]
	return v, ok
}

// ConsoleReply 是 console.reply 操作的实现：往控制台回一行文本。
func ConsoleReply(line string) { consoleOut(line) }

// ContentRegistrar 是 content.register 操作在第一期的**能力声明口**。
//
// 第一期不提供"往服务端目录里注册新内容"的通用钩子——服务端的内容真源是 PVF
// （根 AGENTS §0.2），mod 不应该绕过它造平行内容表。这里只登记 mod 声明的内容
// 扩展意图（名字 + 说明），随启动日志输出，供操作者与后续版本核对。
//
// 真正的"扩展内容"能力要等明确的 PVF 侧扩展点（例如新的脚本标签）落地后再开，
// 否则就会变成"mod 各自往 Go 里塞一张表"的第二真源。
func ContentRegistrar(modID, name, note string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("content.register 的 name 不能为空")
	}
	mu.Lock()
	contentClaims = append(contentClaims, contentClaim{modID: modID, name: name, note: note})
	mu.Unlock()
	Logf(modID, "声明内容扩展意图：%s（%s）", name, note)
	return nil
}

type contentClaim struct {
	modID, name, note string
}

var contentClaims []contentClaim

// ContentClaims 返回已声明的内容扩展意图（诊断用）。
func ContentClaims() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(contentClaims))
	for _, c := range contentClaims {
		out = append(out, fmt.Sprintf("%s: %s（%s）", c.modID, c.name, c.note))
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// 诊断
// ---------------------------------------------------------------------------

// Description 生成一段启动日志，列出本进程实际装载了哪些 mod 与钩子。
//
// "装载"（编进二进制并登记了钩子）与"在跑"（live）分开写：只写前者就会出现
// "日志说已装载 1 个 mod，但它的规则一份都没生效"这种对不上的现场。
func Description() string {
	mu.RLock()
	if len(registeredIDs) == 0 {
		mu.RUnlock()
		return "servermod: 未装载任何服务端 mod"
	}
	ids := make([]string, 0, len(registeredIDs))
	for id := range registeredIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var liveIDs, offIDs []string
	nBoot, nConsole, nResp, nReq, nShutdown := 0, 0, 0, 0, 0
	for _, r := range boots {
		if live[r.modID] {
			nBoot++
		}
	}
	for _, r := range consoles {
		if live[r.modID] {
			nConsole++
		}
	}
	for _, r := range responses {
		if live[r.modID] {
			nResp++
		}
	}
	for _, r := range requests {
		if live[r.modID] {
			nReq++
		}
	}
	for _, r := range shutdowns {
		if live[r.modID] {
			nShutdown++
		}
	}
	for _, id := range ids {
		if live[id] {
			liveIDs = append(liveIDs, id)
		} else {
			offIDs = append(offIDs, id)
		}
	}
	mu.RUnlock()

	// 脚本注册表用的是自己的锁（见 scripts.go 的 scriptMu），必须在放掉 mu 之后再问。
	scriptMu.RLock()
	nScript := len(modScripts)
	scriptMu.RUnlock()

	var b strings.Builder
	fmt.Fprintf(&b, "servermod: 已装载 %d 个服务端 mod", len(ids))
	if len(liveIDs) > 0 {
		fmt.Fprintf(&b, "（在跑 %d：%s）", len(liveIDs), strings.Join(liveIDs, ", "))
	} else {
		b.WriteString("（一个都没在跑）")
	}
	if len(offIDs) > 0 {
		fmt.Fprintf(&b, "；停用 %d：%s（钩子不派发）", len(offIDs), strings.Join(offIDs, ", "))
	}
	fmt.Fprintf(&b, "；生效钩子：boot %d、console %d、response %d、request %d、shutdown %d、奖励脚本 %d",
		nBoot, nConsole, nResp, nReq, nShutdown, nScript)
	return b.String()
}

func validModID(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '-':
		default:
			return false
		}
	}
	return true
}
