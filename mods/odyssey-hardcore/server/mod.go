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
//  3. **掉落倍率（本 mod 用来把爆率设成 ×5）**：数值来自本 mod 的 config.json
//     （`world_drop_percent` / `monster_item_percent`，单位 **100 = 1.00 倍**，500 = 5 倍；
//     第一版用的 `worldDropPercent` / `monsterItemPercent` 仍作为兼容别名被识别，
//     两键同时出现时 snake_case 优先），
//     由 `modpolicy.ConfigureDrops` 写进服务端策略。服务端只提供"能乘"这个能力：
//     它不会自己决定任何副本/阶段的掉率，乘数一律由 mod 配置给。
//     缺字段或写 0 = 不改变（沿用服务端环境变量的既有值），所以这份配置向后兼容。
//     消费点在 loot 的**取用处**（`internal/loot/modpolicy_drops.go` 的世界掉落 /
//     小怪专属物品池），每次抽取现读一遍 —— boot 之后立刻生效，不需要重建目录。
//     启动日志里对应 `drop rate rules: …` 一行。
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

	// 掉落倍率：单位与 loot 侧一致，**100 = 1.00 倍**，500 = 5 倍。
	// 缺字段或写 0 = 不改变（沿用服务端环境变量 DFO_ORDINARY_WORLD_DROP_PERCENT /
	// DFO_ORDINARY_MONSTER_ITEM_DROP_PERCENT 的既有值）—— 向后兼容，不会因为
	// 这份配置没有新字段就把掉率变成 0。键名与同文件的 ban_consumables 同为 snake_case。
	WorldDropPercent   uint32 `json:"world_drop_percent"`
	MonsterItemPercent uint32 `json:"monster_item_percent"`

	// 兼容别名：本能力第一版用的是 camelCase 键。已经照那一版写好的配置不能失效，
	// 所以两个键都认；两个键同时出现时以 snake_case 为准（优先级见 loadConfig）。
	WorldDropPercentAlias   uint32 `json:"worldDropPercent"`
	MonsterItemPercentAlias uint32 `json:"monsterItemPercent"`
}

// loadConfig 读内嵌配置；布尔规则缺字段时按"业主口径"兜底为开（本 mod 的语义就是强化）。
// 掉落倍率**没有兜底值**：缺字段/0 = 不改变，这是文档里承诺过的向后兼容语义。
func loadConfig() (rulesConfig, error) {
	cfg := rulesConfig{BanConsumables: true, BanReviveCoin: true}
	if len(defaultConfig) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(defaultConfig, &cfg); err != nil {
		return cfg, fmt.Errorf("解析 mod 内嵌 config.json 失败：%w", err)
	}
	// 键名兼容：snake_case（推荐，与 ban_consumables 同风格）优先，camelCase 别名兜底
	// —— 只写旧键的老配置照旧生效；两个都写时听 snake_case 的。
	if cfg.WorldDropPercent == 0 {
		cfg.WorldDropPercent = cfg.WorldDropPercentAlias
	}
	if cfg.MonsterItemPercent == 0 {
		cfg.MonsterItemPercent = cfg.MonsterItemPercentAlias
	}
	// 清掉别名，免得下游拿到未解析的原始值。
	cfg.WorldDropPercentAlias, cfg.MonsterItemPercentAlias = 0, 0
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
	servermod.Logf(modID, "已登记：奥德赛模式规则（副本内禁用消耗品 + 死亡不可复活）+ 掉落倍率（来自 config.json），boot 时生效")
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
	// 掉落倍率：同样只把"本 mod 的配置值"设上去，服务端不代它决定任何数值。
	// 0（缺字段）= 不改变；两个字段都可以单独用。
	modpolicy.ConfigureDrops(modpolicy.DropRules{
		WorldPercent:       cfg.WorldDropPercent,
		MonsterItemPercent: cfg.MonsterItemPercent,
		Source:             modID,
	})
	servermod.Logf(modID, "掉落倍率已生效（服务端版本=%s）：%s", ctx.Version, modpolicy.Drops())
	return nil
}

// console 提供两条启动期一次性诊断命令
// （DFO_SERVERMOD_CONSOLE="odyssey.hardcore what"）：回答"装没装、会做什么、现在开着什么"。
func console(cmd servermod.ConsoleCommand) (bool, error) {
	switch cmd.Name {
	case "status":
		servermod.ConsoleReply(fmt.Sprintf("[%s] 当前模式规则：%s；当前掉落倍率：%s", modID, modpolicy.Odyssey(), modpolicy.Drops()))
		return true, nil
	case "what":
		servermod.ConsoleReply(fmt.Sprintf(
			"[%s] 奥德赛副本内：① 禁止使用消耗品（可携带、城镇不受影响）；"+
				"② 死亡无法复活（奥德赛测试额度/背包复活币/CERA 三档全禁），走既有死亡超时判负回城。"+
				"③ 掉落倍率：把**世界掉落**与**普通小怪专属物品池**两档概率各乘一个倍数"+
				"（100 = 1.00 倍，500 = 5 倍），倍数写在 config.json 的 world_drop_percent / "+
				"monster_item_percent（旧键 worldDropPercent / monsterItemPercent 仍兼容）；"+
				"缺字段或写 0 = 不改变。它在 loot 的取用处现算，启动日志里对应 `drop rate rules: …`。"+
				"怪物血量 ×10 由 client 层对 DFO.exe 的两处字节补丁实现（HP getter 乘数 1.0→10.0，"+
				"全局生效，不是奥德赛专属）。", modID))
		return true, nil
	}
	return false, nil
}
