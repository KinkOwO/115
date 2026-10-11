package modpkg

// giveaway.random-equipment —— 新角色创建时发一笔金币 + 一封系统邮件。
//
// # 它怎么工作（不造第二套发放路径）
//
// 服务端已经有一条成熟的事件奖励链路：
//
//	character.Service.Create ──新角色提交成功──> reward.Service.CharacterCreate
//	                                                  │
//	                                    Lua 规则 on("character_create", ...)
//	                                                  │
//	                              grant_item(0, N) / send_mail(subject, body)
//	                                                  │
//	              CommitCharacterEvent（稳定幂等键）/ CommitSystemMail
//
// 本 mod 做的就是**往这条链路追加一条规则脚本**（rules/giveaway.lua）。
//
// # 为什么发金币而不是发装备
//
// 目的是**验证"mod 能不能注册并生效"**，所以发放通道必须"必定成功、
// 不依赖任何内容模板"。发装备要过"奖励目录"（启动日志里那行
// `loaded equipment catalog: 3174 rows`，注意它不是 42 万条的完整穿戴目录），
// 模板号不在里面就发不出来 —— 那会把"mod 有没有生效"和"模板号抄得对不对"
// 搅在一起，排查成本翻倍。
//
// 而 id 0 是角色**金币堆**（inventory.Bag.Add 的 id==0 分支，不查目录、
// 不查装备），一定发得出去；`send_mail(subject, body)` 不带附件时也不涉及
// 任何内容模板。两者都已被内嵌的 newchar.lua 在多个角色上实证。
//
// # 启用/禁用
//
// Register() 开头就问 servermod.Enabled(modID)：管理员在 mod 管理器里取消勾选后
// （写入 mods/enabled.json），本 mod 不会把自己的规则脚本登记进去。**改启用状态后
// 需要重启服务端** —— 奖励脚本在服务端启动时一次性加载进 Lua state，无法热摘。
//
// # 怎么确认它真的生效（三层证据，逐层收紧）
//
//  1. 启动日志：`[mod giveaway.random-equipment] 已启用…` —— 证明 Register() 被调到；
//  2. 启动日志：`reward rules enabled (embedded scripts + 1 mod script(s))`
//     —— 证明规则脚本**赶在奖励管线构造之前**进了管线（这一行是构造顺序的见证）；
//  3. 建一个新角色 → 邮箱收到「mod 生效验证」→ 证明事件触发与发放执行都通了。

import (
	_ "embed"
	"fmt"

	"dfolan/internal/servermod"
)

// modID 必须与 mod.json 的 id 逐字符一致。
const modID = "giveaway.random-equipment"

// ruleScript 是规则脚本内容。go:embed 让它随二进制走，不依赖运行期文件。
//
//go:embed rules/giveaway.lua
var ruleScript []byte

// Register 由 modkit 生成的 mods/zz_mods_gen.go 调用。
//
// 阶段约束：此时**配置与存储都还没就绪**（本函数在 prepareRuntime 之前跑），
// 所以这里只登记钩子与规则，不要读配置、不要碰存储。见 servermod 包注释。
func Register() {
	if !servermod.Enabled(modID) {
		// 被管理员禁用：连规则都不登记，等于这个 mod 不存在。
		servermod.Logf(modID, "已被禁用（mods/enabled.json），本次不加载任何规则")
		return
	}

	if err := servermod.RegisterRewardScript(modID, "giveaway-random-equipment.lua", ruleScript); err != nil {
		// 登记失败是作者错误（重名/内容为空），必须让启动失败而不是静默不生效。
		panic("giveaway-random-equipment: 登记奖励规则失败: " + err.Error())
	}
	servermod.RegisterConsoleHelp(modID, "status | what")
	servermod.RegisterConsole(modID, console)
	// boot 钩子在"配置与存储已就绪、还没开始监听"时跑，用来在启动日志里
	// 留下最后一道证据：规则脚本确实进了管线。
	servermod.RegisterBoot(modID, boot)
	servermod.Logf(modID, "已启用：新角色创建时发 %d 金币 + 一封系统邮件（规则 %s）",
		goldPerCharacter, "giveaway-random-equipment.lua")
}

// boot 是启动自检：在日志里确认规则脚本已被奖励管线接收。
//
// 返回值恒为 nil —— 本 mod 没有"必须阻止启动"的前置条件。
// （boot 返回 error 会让服务端拒绝启动，不要拿它做可选检查。）
func boot(ctx *servermod.BootContext) error {
	names := servermod.RewardScriptNames()
	found := false
	for _, n := range names {
		if n == modID+":giveaway-random-equipment.lua" {
			found = true
		}
	}
	if !found {
		// 这种情况说明规则登记与管线构造的顺序又反了。明确报错，别静默。
		return fmt.Errorf("[%s] 启动自检失败：规则脚本没有进入奖励管线（当前管线里的 mod 脚本：%v）",
			modID, names)
	}
	servermod.Logf(modID, "启动自检通过（服务端版本=%s）；奖励管线已接收 mod 规则脚本",
		ctx.Version)
	return nil
}

// goldPerCharacter 与 rules/giveaway.lua 里的 GOLD 保持一致（仅用于日志展示；
// 真正的数值以脚本为准 —— 那里才是执行的地方）。
const goldPerCharacter = 1000000

// console 提供两条启动期只能跑一次的诊断命令
// （DFO_SERVERMOD_CONSOLE="giveaway.random-equipment what"），
// 用来回答"这个 mod 到底装没装、会做什么"。
func console(cmd servermod.ConsoleCommand) (bool, error) {
	switch cmd.Name {
	case "status":
		servermod.ConsoleReply(fmt.Sprintf(
			"[%s] 已启用；规则脚本已登记 giveaway-random-equipment.lua", modID))
		return true, nil
	case "what":
		servermod.ConsoleReply(fmt.Sprintf(
			"[%s] 新角色创建时：grant_item(0, %d) 发金币 + send_mail 发一封无附件系统邮件。"+
				"两者都不依赖内容模板，所以只要事件触发就一定发得出。", modID, goldPerCharacter))
		return true, nil
	}
	return false, nil
}
