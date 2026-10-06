# MOD 框架能力边界 + 文档一致性审计（2026-10-07）

> 只读取证，未改动任何文件、未启动游戏或服务端。
> 证据索引在 §7；括号里的 `文件:行号` 均指向本机当前工作树。

## 0. 一句话结论

- **玩家无源码自制 mod**：**客户端层 / 资源层 ✅ 成立**；**Lua 规则脚本 ❌**（随包与远端发布的服务端程序**都没有 mod 宿主**，落盘成功但无人读取 ⇒ 静默失效）；**Go 钩子 mod ❌**（安装期静态验证编译失败并回滚）。
- **文档与事实**：**22 条不一致（4 致命 / 9 困惑 / 9 文案）**，其中 3 条会让 mod 作者"以为做成了、其实没生效"。
- **框架代码本身没问题**：本机当前树完整可用（带宿主的二进制、已装的 server 层 mod、已生效的客户端插件通道都在）。缺的是**把发布包按当前树重打**。

## 1. 能力边界矩阵

前提：玩家只有「启动器 + 整合包」，**没有服务端源码、没有 Go 工具链**。

| 类型 | 能做到什么 | 卡在哪一步 | 结论 |
| --- | --- | --- | --- |
| ① 只改客户端文件（`file.add`/`file.replace`/`exe.patch`） | 完全可行；本机实机已装 3 个 client 层 mod | 仅要求先退游戏；`file.add` 不覆盖"已存在且内容不同" | ✅ |
| ② 只放 Lua 规则脚本（`server.script`） | modkit 会落到 `<启动器根>\mods\<名>.lua`，启动器还注入 `DFO_REWARD_SCRIPTS_DIR` | **随包服务端不认识这个变量**（解析在 `cmd/wireprobe/servermods.go:81`，该文件不在随包源码里），且随包 `buildRewardService` 不传 `Scripts` ⇒ 只剩内嵌规则集。**零报错** | ❌ 静默失效 |
| ③ 改服务端玩法（Go 钩子 `server.hook`） | — | `modkit install` 的静态验证编译（`install2.go:250-257,579-611`）必失败：mod 必 `import "dfolan/internal/servermod"`，随包没有该包 ⇒ 回滚报错；缺 Go 更早失败（`install2.go:589-592`）。**更隐蔽**：即使跳过校验装上，「编译服务端」也能成功（`./cmd/wireprobe` 不 import `dfolan/mods`），但服务端永不调 `mods.RegisterMods()` | ❌ |
| ④ 改内容数据（NPK / PVF） | **NPK 条目级或整包替换可行**（`layer.go:291-298`） | **PVF 层要 pwsh 7**；本机 PATH / `C:\Program Files\PowerShell\7` / `115\tools\pwsh7` 三处皆无 ⇒ 含 pvf 层的 mod 一律装不上（明确报错） | ⚠️ 资源能 / PVF 卡住 |
| ⑤ 编译链本身（玩家自己编服务端） | **已具备**：`serverbuild.Build` 用包内 `tools/go` 跑 `go build -trimpath ./cmd/wireprobe`（`serverbuild.go:161-238`），编译前重算 mod 加载器（`:184`）；`tools\` 下 `tools-go.zip` / `tools-gopath-mod.zip` / `tools-server-src.zip` 已就位 | 能编，但**输入陈旧** ⇒ 编出来仍无宿主 | ⚠️ 工具齐全、原料过期 |

## 2. 发布链的断点（最关键）

### 2.1 远端发布包也没有 mod 宿主

`https://gitgud.io/fuckworld/115`（启动器 update 主端点）的 `tools/tools-server-bin.zip`
= **24,060,992 B / sha256 `e2cc9b3a9e51bd9f98b4ad3af7d4152a3ee3a6967f44836eea6c7700d83c32d7` / 85 文件**（已下载核验，与 manifest 声明一致）。包内 `work/dfo-lan/bin/wireprobe-pvf.exe`（28,633,600 B）：

| 特征串 | 远端包 | 本机当前树编译产物 |
| --- | --- | --- |
| `internal/servermod` / `RegisterMods` / `zz_mods_gen` | **0 / 0 / 0** | 160 / 2 / 1 |
| `DFO_REWARD_SCRIPTS_DIR` / `enabled.json` / `odyssey.hardcore` | **0 / 0 / 0** | 1 / 3 / 3 |
| `unlock_equip_slots`（10-06 待遇 API） | **0** | 2 |
| `jackc/pgx/v5` | **0**（⇒ 是 10-05 之后编的） | 0 |

⇒ 它是「**10-05 PostgreSQL 移除之后、10-06 mod 宿主进树之前**」的版本。

### 2.2 本地 `tools\` 两个包更旧

- `tools-server-bin.zip` 36,760,064 B（本机 manifest 登记 `e1b63235…`／23,960,838 B／84 文件 —— 比远端还旧一版）。
- `tools-server-src.zip` 11,185,853 B / 1878 条目：顶层只有 `cmd`/`internal`/`scripts`/`go.mod`/`go.sum`/`sqlc.yaml`；`mods/*`=0、`internal/servermod/*`=0、`cmd/wireprobe/servermods.go`=0；仍带 `internal/database/sql/postgres/**` 与 `go.mod` 的 `pgx v5.7.6`；包内 `internal/reward/lua.go` 时间戳 2026-10-04 17:11、`reward_flow.go` 2026-10-04 20:41，且其 `buildRewardService` 调 `reward.New` **不传 `Scripts`**，注释原文：*"Its rules are embedded in the binary, so there is no external path to resolve."*

### 2.3 覆盖风险：一次「更新/自检」能让 server 层 mod 静默全灭

- **常规启动**：只比对"版本 ID"、**根本不看盘**（`internal/prebuilt/prebuilt.go:274-286`）⇒ 玩家自己编的程序不会被碰 ✅；
- **自检 / 更新页**：会替换受管文件，而 `server-bin` 包的 check 路径正是 `server/work/dfo-lan/bin/wireprobe-pvf.exe` ⇒ **本地带宿主版本会被换成发布的无宿主版本**，之后所有 server 层 mod 静默失效。

## 3. 文档 vs 事实：22 条不一致

### 3.1 致命（4 条）

| # | 文档说 | 事实 | 影响 |
| --- | --- | --- | --- |
| F2 | `MOD-DEVELOPMENT.md:440`、`mods/README.md:97`、`newchar-kit/README.md:46`：卸载会"按注册表哈希撤掉/删除"脚本 | **用户卸载一律保留脚本**（`internal/modkit/rewardscripts.go:34-35,384-388`、`uninstall2.go:123-124,169-172,187-201`）；只有安装失败/升级的内部回滚才按哈希删（`rewardscripts.go:390-393`） | 以为卸了就停，脚本仍在 `<启动器根>\mods\`，下次启动照样加载 → **静默继续生效** |
| F3 | 只说"同名时磁盘优先"（`MOD-DEVELOPMENT.md:415`、`MOD-MANAGER-INTEGRATION.md:170`） | `compositeScriptFS{first: bundled, second: onDisk}`（`cmd/wireprobe/reward_flow.go:99-101,124-129`）⇒ **内嵌优先**；内嵌集 = `internal/reward/scripts/{level,newchar,quest}.lua`（`reward.go:30`）。全套文档**零处**提到内嵌集 | 作者把脚本命名成 `newchar.lua` → 一条规则都不跑，日志仍打"已装载" ⇒ **静默失效** |
| F7 | `MOD-MANAGER-INTEGRATION.md:31-34` 把 `LoadEnabledList`/`RegisterMods` 画在 `prepareRuntime` **之后** | 实际顺序 `LoadEnabledList → RegisterMods → prepareRuntime → Boot`（`cmd/wireprobe/main.go:70-71,73,88`）；`main.go:56-60` 注释明说放在后面会导致"规则脚本赶不上管线构造 = 静默不生效"（10-06 现场） | 照文档实现的集成会静默丢掉所有规则脚本 |
| F4 | `MOD-DEVELOPMENT.md:488-489,644`、`odyssey-hardcore/client/PATCH-NOTES.md:7`、同目录 README `:64-65,107`：`exe.patch` 有整文件哈希门 | 哈希门已移除（`manifest2.go:261-263`、`layerplan.go:602-609`、`apply2.go:210-213`、`run2.go:147-153`），改为逐字节 `before` 校验 + 警告 | 作者以为"版本不对会被门挡住"，实际会照落 |

### 3.2 困惑（9 条，摘要）

| # | 主题 | 要点 |
| --- | --- | --- |
| F6 | 注册表没有 author/description | `modkit.go:71-89`、`modsadmin.go:47-67`；`mods list --json` 不带（`mods.go:184-193`）。真实来源是 `internal/modlib/store.go:190-228` 扫 `<整合包根>\mods`。`MOD-DEVELOPMENT.md:145-150` 与 `MOD-MANAGER-INTEGRATION.md:96,135-137,276-277` 的传播链不成立 |
| F8 | `size/sha256` 现为**可选**声明，不符只 warning（`package.go:376-392`） | `MOD-DEVELOPMENT.md:59,63` 仍说"任一条不符整包拒绝" |
| F9/F10 | `MOD-MANAGER-INTEGRATION.md:101,107`"有 server 层就要重编译" | 对 **scripts-only** mod 是错的（`layer.go:375-377`、`install2.go:250,263-264`） |
| F16 | `MOD-DEVELOPMENT.md:202`"五个宿主机操作" | 实际 **6 个**（漏 `reward.register`，`layer.go:186`） |
| F17 | `MOD-DEVELOPMENT.md:361-363` 的 `:vault-1`/`:vault-2` 幂等键 | **不存在**；个人金库走 `database/vault.go:111`，签名里没有 key |
| F18 | `uninstall --force` = "强行用备份还原" | 实际只是取消跳过（`uninstall2.go:103-109`），`apply2.go:605-607` 不看 Force ⇒ 效果是"整次卸载报错中止" |
| F19 | `MOD-DEVELOPMENT.md:189` 的 `go build ./mods/...` | 实际是**逐个 mod 目录**编译（`install2.go:593-609`） |
| F20 | `MOD-DEVELOPMENT.md:9`、`mods/README.md:3`："mods/ 不是加载 mod 的地方 / mod 唯一家在服务端模块" | 已过时：`<启动器根>/mods` 既是脚本第一顺位目录，又是 MOD 页的库（`modlib/store.go:60-63`） |
| F21 | `MOD-DEVELOPMENT.md:145` 与 `:438` 内部矛盾 | 只有带 `hooks` 才落 `mods/<id>/mod.json`（`install2.go:438,524-529`） |

### 3.3 文案（9 条，摘要）

- `MOD-DEVELOPMENT.md:641,644` 写了两条**不存在**的报错（`缺少 size/sha256 双重校验`、`整文件哈希门不过`）；`:583`"verify 会报出实测值"不成立（`cmd/modkit/run2.go:94-108` 不打印 Warnings）。
- `cmd/modkit/main.go:66,70` 的 `verify --client`、`uninstall --purge` **不存在**。
- `MOD-DEVELOPMENT.md:383-384,390,396-397` 说 `grant_item` 只能发堆叠物（装备必须走 `send_mail`）—— 现在它走装备分支并实例化（`internal/inventory/awards.go:49-91`、`equipment.go:424`），`newchar_kit.lua:54,294` 就是这么用的；与 `newchar-kit/README.md:35-40` 自相矛盾。
- `mods/README.md:74-78`"邮件发装备必须先在奖励目录里"是旧口径（`bootstrap.go:932-956`、`reward_flow.go:544-566` 已放开），与 `newchar-kit/README.md:29-32` 冲突。
- `mods/README.md:88-94` 把 newchar-kit 写成两个脚本（实际 `mod.json` 只声明 `server/rules/newchar_kit.lua`）；`newchar-kit/README.md:11` 的 `66e6236e…` 属于 `variants/original/newchar_kit.lua`，当前脚本是 `cba54ad3…`。

### 3.4 六条"点名核实"的结论

1. **脚本落位**：文档说 `<服务端模块>/mods/scripts/<名>`（4 处）；实际**落**在 `<启动器根>/mods/<名>`（`rewardscripts.go:73-89`、`install2.go:457-458`），服务端读取顺序 `DFO_REWARD_SCRIPTS_DIR → <包根>/mods → <包根>/mods/scripts → <模块根>/mods/scripts`（`servermods.go:63-90`）。**卸载不删脚本**（见 F2）。
2. **Lua API**：13 个函数**逐一对应、无缺无多**（`internal/reward/lua.go:64,77,93,124,156,165,175,185,200,210,221,242,258`）；事件 3 个、ctx 字段、附件 ≤11、各项钳制范围全部与文档一致（唯一偏差见 §3.3 的 `grant_item`）。
3. **`requires` 是硬门禁**：plan 阶段 `冲突：依赖未安装：X` + exit 2（`layerplan.go:369-373`）；install 写盘前再判一次，**额外拒绝"依赖被禁用"**（`install2.go:97-102,296-327`）；卸载/禁用方向也拦（`uninstall2.go:66-68`、`mods.go:281-298`）。文档没写"plan 只看装没装、install 还看启用状态"。
4. **MOD 页两列**：列头 `安装状态 / 生效 / 制作人`（`index.html:860-862`）；取值 = 已安装·未安装（`app.js:481-490`）与 生效 / 重启服务端后生效 / 未生效（需编译服务端）/ —（`app.js:492-521`、`modlib/store.go:164-175`），判据是**注册表 + 服务端二进制全文搜 modID + mtime**（`store.go:488-522`），**不是**启动日志。`MOD-MANAGER-INTEGRATION.md:100,103` 两列都写错。
5. **modkit flags**：verify `--mod/--allow-dir/--json`；plan `--client/--mod/--root/--allow-dir`；install `--client/--mod/--root/--dry-run/--pwsh/--timeout/--skip-verify-build/--allow-dir`；uninstall `--client/--id/--root/--dry-run/--force`。文档**没有**写不存在的 flag（只是漏写若干）。
6. **Go 工具链 / 随包二进制**：文档**都没写** —— ① 装 `server.hook` mod 在无 Go 工具链时是**安装期硬失败**（`install2.go:113-116`）；② 预编译包可能**连 mod 宿主都没有**（`modlib/store.go:101-108` 有现场记录），此时启动日志连 `servermod: 已装载` 都不会有，而文档把"是否生效"指向启动日志（`:103`）。

## 4. Lua 能力面（玩家现在能做什么）

**13 个函数**（不在表里的一律不存在）：`on`(64) `grant_item`(77) `send_mail`(93，附件≤11) `grant_cera`(124) `unlock_equip_slots`(156) `expand_bag`(165) `expand_avatar`(175) `grant_revive_coin`(185) `grant_pet`(200) `grant_pet_item`(210) `expand_vault`(221) `grant_account_material`(242) `unlock_skins`(258)。

**3 个事件**：`level_up` / `quest_complete` / `character_create`（其它名字 `on()` 直接 RaiseError，`lua.go:67-70`）。
**ctx**：恒有 `type/level/quest_id/character_id/account_id/name`；仅 `character_create` 额外 `profession`/`advancement`（`lua.go:293-304`）。
**幂等键**：`reward:<event>:<脚本文件名>:<判别值>` + `:item`/`:mail`(`:mail:%d`)/`:cera`/`:state`/`:account-vault`/`:account-material`。**判别值是脚本文件名，不是 mod id** ⇒ 跨 mod 同名脚本互相顶掉。

**做不了（举 3 个）**：① 改掉落倍率/掉落表（函数表闭合、事件表闭合、钩子白名单闭合在 `layer.go:129-166` 的 4 个点；连 `config.write` 都只写进 `modValues`，**没有任何服务端代码读它**，`host.go:401-414`）；② 击杀/进副本/商城购买时发东西（只有 3 个事件）；③ 改 S2C 报文（`protocol.response` 是只读观察点，无返回值可改，`host.go:88-92`）。

## 5. 最小改动清单（按优先级）

| # | 改哪里 | 为什么 | 影响面 |
| --- | --- | --- | --- |
| **P0** | 按当前树重打 `tools\tools-server-bin.zip`（含宿主的 `wireprobe-pvf.exe` + `dfolauncher.exe` + `probe.exe` + 干净形态 `mods/`）与 `tools\tools-server-src.zip`（含 `internal/servermod/**`、`cmd/wireprobe/servermods.go`、`mods/*`），并同步 `tools\manifest.json` 的 `size/sha256/files` | 玩家拿到的服务端**没有 mod 宿主**，这是"自制 mod"成立的唯一硬前提；顺带甩掉 pgx/PG 残留 | 全体玩家的服务端程序；需实机验收 |
| **P0.5** | **发布**：推 `RicardoLz/115` 不够 —— 启动器 update 端点是**先 `fuckworld/115`、后 `RicardoLz/115`**，前者有 manifest 时玩家永远读不到新包 | 不解决发布路径，P0 等于没做 | 需要业主/维护者协调，或改端点顺序 |
| **P1** | modkit 加"**宿主探测**"：本机服务端源码/二进制不具备该层宿主时**拒绝或明确告警**（判据：`cmd/wireprobe/servermods.go` 是否存在、`go.mod` 是否 import `dfolan/mods`、启动日志有无 `servermod: 已装载`） | 把"装成功→编成功→什么也没发生"从静默变响亮 | 只影响校验与提示 |
| **P1.5** | 编译成功的**本地服务端二进制**记为玩家产物，让自检/更新**不再替换**（或至少告警） | 否则 P0 做完，玩家自编版本仍会被更新冲掉（§2.3） | 启动器资源闸门 |
| **P2** | 把"掉落/经验倍率"这类**运维参数**开成 mod 可提供的策略覆盖（只放数值、不放内容清单，守 §0.2） | 玩家最想要的一类 mod；`config.write` 目前是装饰性的 | 触及玩法数值 |
| **P3** | 修 §3 的 22 条文档不一致；补"**怎么自证生效**"一节（服务端日志 `servermod: 已装载 N 个…`、`modkit status`、MOD 页"生效"列）+ 最小脚本模板 + "别用 `level.lua`/`newchar.lua`/`quest.lua`"警告 | 玩家目前没有任何自检手段；§3.1 的三条致命不一致会直接导致静默失败 | 纯文档 |
| **P4** | 随包带 pwsh7（或给 pvf 层换自带解释器） | 否则含 PVF 层的 mod 一律装不上 | 所有 pvf 层 mod |

## 6. 待业主决策

1. **同名脚本优先级**：内置 vs 磁盘 —— 建议按文档改成**磁盘/mod 优先**（内置退化为兜底），与"发不发由 mod/脚本决定"一致；当前实现是内置优先（F3）。
2. **P0 的发布路径**：主端点 `fuckworld/115` 的更新权不在本项目手上 —— 请维护者同步，还是改 `launcher.settings.json` 的主端点？
3. **P0 现在开工吗**（会先备份两个 zip + manifest，改完给新旧 size/sha256 对照与解包验证）。
4. **P2 的范围**：先开哪几个倍率（掉落 / 经验 / …）。

## 7. 证据索引（关键文件）

- 预编译包与发布：`tools\manifest.json`、`tools\tools-server-bin.zip`、`tools\tools-server-src.zip`、`https://gitgud.io/fuckworld/115/-/raw/main/tools/manifest.json`
- 启动器：`internal/prebuilt/prebuilt.go`、`internal/resources/*`（版本 ID 闸门）、`internal/modkit/{manifest2,layer,layerplan,install2,uninstall2,apply2,package,rewardscripts}.go`、`internal/modlib/{store,scripts,ops,mods}.go`、`cmd/modkit/{main,run2}.go`、`internal/appui/ui/{index.html,app.js}`
- 服务端：`cmd/wireprobe/{main,servermods,reward_flow,odyssey_gate}.go`、`internal/servermod/*`、`internal/reward/{reward,lua,trigger,scripts}/*`、`internal/modpolicy/{modpolicy,drops}.go`、`internal/loot/{world_drop,monster_items,session,modpolicy_drops}.go`、`mods/README.md`、`mods/zz_mods_gen.go`
- 客户端插件通道：`client-patchs/AGENTS.md`、`client-patchs/client-host/HOST-README.md`
- 现场实机证据：`DFO\.launcher-mods\modkit\registry.json`（4 个已装 mod）、`DFO\client-host.log`（插件启动 → 返回 0）、`server\work\dfo-lan\mods\zz_mods_gen.go`（已 import `odyssey.hardcore`）
