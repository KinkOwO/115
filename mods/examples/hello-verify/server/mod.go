// hello-verify —— 四层架构的**可安装、可验证**示例 mod（server 层）。
//
// 它演示 server 层的三件事：
//  1. server.boot 钩子：自检 + 读有效配置 + 记日志（游戏/服务端启动就能在日志里看到）；
//  2. console.command 钩子：一次性命令，用 DFO_SERVERMOD_CONSOLE 触发，结果进日志；
//  3. 宿主机操作的最小用法：log / config.read / config.write / console.reply。
//
// 编译期注册：本包由服务端模块 mods/zz_mods_gen.go import 并调用 Register()。
// 签名写错 → 服务端编不出来（这是 B 路线的核心保障）。
//
// 注意：boot 回调返回 error = 服务端拒绝启动。别拿它做可选检查。

package modpkg

import (
	"fmt"
	"sort"
	"strings"

	"dfolan/internal/servermod"
)

// modID 必须与 mod.json 的 id 逐字符一致（日志与注册表靠它归因）。
const modID = "demo.hello-verify"

// Register 由 modkit 生成的 mods/zz_mods_gen.go 调用。
func Register() {
	servermod.RegisterBoot(modID, boot)
	servermod.RegisterConsole(modID, console)
	servermod.RegisterConsoleHelp(modID, "status | env <KEY> | flag <KEY> <VALUE> | mods")
}

// boot 在"配置与存储就绪、还没开始监听"时执行。
//
// 只做只读自检 + 日志：这样玩家在服务端启动日志里就能确认 mod 真的被加载了。
func boot(ctx *servermod.BootContext) error {
	servermod.Logf(modID, "启动自检通过；服务端版本=%s 频道数=%d", ctx.Version, ctx.ChannelCount)
	servermod.Logf(modID, "服务端层已生效（如果能看到这行，说明 mod 被编译进二进制并执行了 Register()）")

	// 读几个既有的运营开关，确认 config.read 通路可用。
	// 键空间 = 启动时的 DFO_* 环境变量快照；没有的键不存在，不报错。
	for _, key := range []string{
		"DFO_SHOP_OPEN_ALL",
		"DFO_BAKAL_MODE",
		"DFO_HELL_PARTY_DROP_PERCENT",
		"DFO_ATTUNEMENT_QUANTITY_MULTIPLIER",
	} {
		if v, ok := servermod.ConfigRead(key); ok {
			servermod.Logf(modID, "有效配置 %s=%s", key, v)
		}
	}

	// 声明内容扩展"意图"（第一期只登记，随启动日志输出）。
	// 真正的内容扩展能力要等服务端 PVF 侧扩展点落地——mod 不应另造内容真源。
	if err := servermod.ContentRegistrar(modID, "hello-verify.trace",
		"示例：只声明意图，不改任何内容表"); err != nil {
		return err
	}
	return nil
}

// console 处理启动期一次性命令（DFO_SERVERMOD_CONSOLE="demo.hello-verify status"）。
//
// 返回值：handled=true 表示这条命令由本 mod 认领。
func console(cmd servermod.ConsoleCommand) (bool, error) {
	switch cmd.Name {
	case "status":
		mods := servermod.Registered()
		servermod.ConsoleReply(fmt.Sprintf("[%s] 运行中；已装载服务端 mod：%s",
			modID, strings.Join(mods, ", ")))
		return true, nil

	case "mods":
		mods := servermod.Registered()
		sort.Strings(mods)
		servermod.ConsoleReply(fmt.Sprintf("[%s] 共 %d 个服务端 mod：%s",
			modID, len(mods), strings.Join(mods, ", ")))
		return true, nil

	case "env":
		if len(cmd.Args) != 1 {
			return true, fmt.Errorf("用法：env <KEY>")
		}
		key := cmd.Args[0]
		v, ok := servermod.ConfigRead(key)
		if !ok {
			servermod.ConsoleReply(fmt.Sprintf("[%s] %s 不在有效配置快照里", modID, key))
			return true, nil
		}
		servermod.ConsoleReply(fmt.Sprintf("[%s] %s=%s", modID, key, v))
		return true, nil

	case "flag":
		if len(cmd.Args) != 2 {
			return true, fmt.Errorf("用法：flag <KEY> <VALUE>（KEY 必须在有效配置快照里）")
		}
		key, val := cmd.Args[0], cmd.Args[1]
		if err := servermod.ConfigWrite(key, val); err != nil {
			return true, err
		}
		got, _ := servermod.ModValue(key)
		servermod.ConsoleReply(fmt.Sprintf("[%s] 已设置 %s=%s（仅本次进程；改配置要写启动档）",
			modID, key, got))
		return true, nil
	}
	// 不是我的命令：返回 false 让别的 mod 有机会认领。
	return false, nil
}
