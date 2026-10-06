package modpkg

// odyssey.hardcore —— 奥德赛模式强化：副本内禁用消耗品 + 死亡不可复活。
//
// # 它怎么生效（不造第二套玩法表）
//
// 服务端侧唯一能改玩法的地方是 `internal/modpolicy`：一份**进程内策略**，
// 消费点固定在既有业务里（CMD44 的 `odysseyConsumableGate`、CMD41 的
// `odysseyReviveGate`）。本 mod 在 `server.boot` 里把业主口径设上去。
// 不装 mod（或在管理器里禁用）时策略保持零值 ⇒ 服务端行为与装 mod 之前一致。
//
// # 规则与判据
//
//  1. **副本内禁用消耗品**：CMD44 一律拒绝（可以携带；城镇不受影响）。
//     客户端侧本来就一致：奥德赛进图不下发 NOTI1584，界面本来就是灰的，
//     服务端这道挡的是权威侧 —— 改过的客户端绕过界面也拿不到药。
//  2. **死亡不可复活**：CMD41 的三级回退（奥德赛测试额度 / 背包复活币 /
//     CERA 扣费）在入口处整条拒掉，不留"换一档还能复活"的缝；随后走既有
//     死亡超时流程（10 秒 → N33 FAIL_CLEAR + 回城）判负回城。
//
// 怪物血量 ×10 由 **client 层**对 `DFO.exe` 的两处同长度字节补丁实现（不在服务端，也不在这个
// Go 包里）：服务端根本不下发怪物血量（怪物包只有 entity/template/level/rank，见
// `internal/game/protocol/dungeon.go`），而血量表既不在 `Script.pvf` 也不在任何 NPK 里，
// 所以只能改客户端 EXE 的 HP getter 乘数（1.0 → 10.0，见 mod.json 的 client 层与 README）。
// 注意：这条是**全局**的（所有副本怪物都变厚），不是奥德赛专属。
//
// # 启用/禁用
//
// `Register()` 先问 `servermod.Enabled(modID)`（管理器写 `mods/enabled.json`）。
// 改动启用状态后**必须重启服务端**：mod 的 Go 代码是编进服务端二进制的。
//
// # 怎么确认真的生效（三层证据）
//
//  1. 启动日志 `[mod odyssey.hardcore] 已登记…` —— 证明 Register() 被调到；
//  2. 启动日志 `[mod odyssey.hardcore] 策略已生效…` 与
//     `odyssey mode rules: 开启：…（← odyssey.hardcore）` —— 证明策略真的设上去了
//     （`cmd/wireprobe/main.go` 在启动装配结束时打印，见 server/AGENTS §6 的教训）；
//  3. 游戏里：奥德赛副本内吃药被拒、死亡后复活被拒。

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"dfolan/internal/modpolicy"
	"dfolan/internal/servermod"
)

// modID 必须与 mod.json 的 id 逐字符一致。
const modID = "odyssey.hardcore"

// config.json 随二进制走（go:embed），所以改数值要重装这个 mod。
//
//go:embed config.json
var defaultConfig []byte

type rulesConfig struct {
	BanConsumables bool `json:"ban_consumables"`
	BanReviveCoin  bool `json:"ban_revive_coin"`
}

// loadConfig 读内嵌配置；缺字段时按"业主口径"兜底为开（本 mod 的语义就是强化）。
func loadConfig() (rulesConfig, error) {
	cfg := rulesConfig{BanConsumables: true, BanReviveCoin: true}
	if len(defaultConfig) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(defaultConfig, &cfg); err != nil {
		return cfg, fmt.Errorf("解析 mod 内嵌 config.json 失败：%w", err)
	}
	return cfg, nil
}

// Register 由 modkit 生成的 mods/zz_mods_gen.go 调用。
//
// 阶段约束：此时配置与存储都还没就绪（本函数在 prepareRuntime 之前跑），
// 所以只登记钩子，不读配置、不碰存储（见 servermod 包注释）。
func Register() {
	if !servermod.Enabled(modID) {
		servermod.Logf(modID, "已被禁用（mods/enabled.json），奥德赛模式规则本次不生效")
		return
	}
	servermod.RegisterConsoleHelp(modID, "status | what")
	servermod.RegisterConsole(modID, console)
	servermod.RegisterBoot(modID, boot)
	servermod.Logf(modID, "已登记：奥德赛模式规则（副本内禁用消耗品 + 死亡不可复活），boot 时生效")
}

// boot 在"配置与存储已就绪、还没开始监听"时把策略设上去。
//
// 返回 error 会让服务端拒绝启动 —— 这里刻意严格：装了这个 mod 却因为配置
// 把两条规则都关掉，属于作者的配置错误，不该静默变成"装了个没用的 mod"。
func boot(ctx *servermod.BootContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("[%s] %w", modID, err)
	}
	applied, err := modpolicy.Configure(modpolicy.OdysseyRules{
		BanConsumables: cfg.BanConsumables,
		BanReviveCoin:  cfg.BanReviveCoin,
		Source:         modID,
	})
	if err != nil {
		return fmt.Errorf("[%s] %w", modID, err)
	}
	if !applied.Enabled() {
		return fmt.Errorf("[%s] config.json 把两条规则都关了：装上这个 mod 却什么都不做，请改配置或卸载它", modID)
	}
	servermod.Logf(modID, "策略已生效（服务端版本=%s）：%s", ctx.Version, applied)
	return nil
}

// console 提供两条启动期一次性诊断命令
// （DFO_SERVERMOD_CONSOLE="odyssey.hardcore what"）：回答"装没装、会做什么、现在开着什么"。
func console(cmd servermod.ConsoleCommand) (bool, error) {
	switch cmd.Name {
	case "status":
		servermod.ConsoleReply(fmt.Sprintf("[%s] 当前模式规则：%s", modID, modpolicy.Odyssey()))
		return true, nil
	case "what":
		servermod.ConsoleReply(fmt.Sprintf(
			"[%s] 奥德赛副本内：① 禁止使用消耗品（可携带、城镇不受影响）；"+
				"② 死亡无法复活（奥德赛测试额度/背包复活币/CERA 三档全禁），走既有死亡超时判负回城。"+
				"怪物血量 ×10 由 client 层对 DFO.exe 的两处字节补丁实现（HP getter 乘数 1.0→10.0，"+
				"全局生效，不是奥德赛专属）。", modID))
		return true, nil
	}
	return false, nil
}
