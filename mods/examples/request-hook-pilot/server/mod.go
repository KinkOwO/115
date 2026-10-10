// request-hook-pilot —— **请求侧钩子（protocol.request）**的可装可验样板。
//
// # 它演示什么
//
// 在 protocol.request 出现之前，服务端层 mod 只能"在启动时登记策略"或"在应答出口
// 只读观察"，**没有任何请求侧接入点** —— 所以"新增一个玩法"只能改内核、不能写成 mod。
// 这个钩子把那个口子打开了：
//
//	客户端一帧报文
//	    ↓ 网关解密 + 判校验和（client_connection.go 的读循环）
//	    ↓ 【本钩子在这里被问一次】—— 拿到与内置 handler 完全一致的正文
//	    ├─ 返回 false → 放行给内置分发表（一切照旧）
//	    └─ 返回 true  → 内置分发表全部跳过，由 mod 用 ctx.Reply 自己应答
//
// # 为什么它对真实游戏零影响
//
// 它只认**带魔数标记**的帧，而正常客户端永远不会发那个标记。所以装上它以后，
// 每一帧都走"看一眼、放行"，游戏行为与不装时逐字节一致。要改成真玩法，
// 把 probeMarker 判据换成你自己的业务判据即可。
//
// # 写作纪律（踩过就知道疼的两条）
//
//  1. **只看不改**：RequestContext 刻意不提供"就地改写正文"的能力。要改行为就
//     整条接手 —— 旁路改写会让协议真源从"服务端代码路径 + IDA/实机取证"退化成
//     "某个 mod 的猜测"。
//  2. **短路必须能应答**：返回 handled 之后内置分发不再说话，所以要么用 ctx.Reply
//     发出应答，要么这条报文就石沉大海（客户端会卡在等待态）。ctx.Reply 走的仍是
//     服务端正规编码（校验和 + 加密），不是裸包。
//
// 编译期注册：本包由服务端模块 mods/zz_mods_gen.go import 并调用 Register()。
// 钩子签名写错 → 服务端根本编不出来，所以不存在"装了但没生效"的静默失败
//（mod.json 声明与注册是否一致另有启动期核对，见 internal/servermod/declarations.go）。

package modpkg

import (
	"bytes"
	"fmt"
	"sync/atomic"

	"dfolan/internal/servermod"
)

// modID 必须与 mod.json 的 id 逐字符一致（日志与注册表靠它归因）。
const modID = "demo.request-hook-pilot"

// probeCMD 是样板认领的报文号。刻意选一个内置分发表不认领的值。
const probeCMD uint16 = 59999

// probeMarker 是"这条报文归我管"的魔数。正常客户端不会发它。
var probeMarker = []byte{0xD0, 0x0F, 0xBE, 0xEF}

var (
	seenFrames   atomic.Int64
	handledFrame atomic.Int64
)

// Register 由 modkit 生成的 mods/zz_mods_gen.go 调用。
//
// 开头先问 Enabled()：被 mods/enabled.json 禁用时**一个钩子都不注册**，
// 这样启动期的"声明与注册一致性核对"才不会把它误报成故障。
func Register() {
	if !servermod.Enabled(modID) {
		servermod.Logf(modID, "已被禁用（mods/enabled.json），请求侧钩子本次不注册")
		return
	}
	servermod.RegisterConsoleHelp(modID, "status | reset")
	servermod.RegisterConsole(modID, console)
	servermod.RegisterRequest(modID, onRequest)
	servermod.Logf(modID, "已登记 protocol.request 钩子（只认带魔数标记的帧，其余一律放行）")
}

// onRequest 是请求侧钩子本体。
//
// 返回值语义：
//   - (false, nil) = 放行，交给内置分发表（绝大多数帧走这条）；
//   - (true,  nil) = 我接手了，内置分发表不再看这条报文；
//   - (_,    err)  = 处理失败：宿主记一行日志后**继续问下一个钩子与内置分发**，
//     所以出错不会把这一帧吞掉（但仍会被算作一次失败处理）。
func onRequest(ctx *servermod.RequestContext) (handled bool, err error) {
	seenFrames.Add(1)

	// 只看不改：不满足判据就原样放行。
	if ctx.Type != 1 || ctx.ID != probeCMD || !bytes.HasPrefix(ctx.Plaintext, probeMarker) {
		return false, nil
	}
	// 校验和没过：这条帧本来就不该被业务处理，放行让内置路径去拒绝/记录。
	if !ctx.Verified {
		return false, nil
	}

	handledFrame.Add(1)
	servermod.Logf(modID, "接手 CMD%d（连接 %s，正文 %d 字节）", ctx.ID, ctx.Conn, len(ctx.Plaintext))

	if ctx.Reply == nil {
		// 宿主没给应答口 = 引擎接线坏了。宁可报错让它响，也不要静默吞掉这一帧。
		return true, fmt.Errorf("宿主没有提供 reply.send 入口")
	}
	// 应答正文。真实玩法应当用 internal/game/protocol 里的编码器产出这段字节；
	// 样板只演示"mod 自己应答"这条路走得通。
	if err := ctx.Reply(0, probeCMD, []byte("pilot: handled by request hook")); err != nil {
		return true, fmt.Errorf("应答 CMD%d 失败：%w", probeCMD, err)
	}
	return true, nil
}

// console 处理启动期一次性命令（DFO_SERVERMOD_CONSOLE="demo.request-hook-pilot status"）。
func console(cmd servermod.ConsoleCommand) (bool, error) {
	switch cmd.Name {
	case "status":
		servermod.ConsoleReply(fmt.Sprintf("[%s] 已观察 %d 帧，接手 %d 帧（判据：type=1 id=%d 且正文以魔数开头）",
			modID, seenFrames.Load(), handledFrame.Load(), probeCMD))
		return true, nil
	case "reset":
		seenFrames.Store(0)
		handledFrame.Store(0)
		servermod.ConsoleReply(fmt.Sprintf("[%s] 计数已清零", modID))
		return true, nil
	}
	// 不是我的命令：返回 false 让别的 mod 有机会认领。
	return false, nil
}
