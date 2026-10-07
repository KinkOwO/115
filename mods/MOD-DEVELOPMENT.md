# mod 开发文档（四层架构）

> 面向**要写 mod 的人**。读这篇就够了；接口的权威清单用 `modkit layers` 打印。
> 架构与实现细节见启动器仓 `docs/modkit.md`。

- 版本：schema 2（四层）
- 引擎：`modkit`（启动器仓 `cmd/modkit`，引擎 `internal/modkit`）
- 服务端宿主：`server/work/dfo-lan/internal/servermod`
- **三个 `mods/` 别混**（2026-10-07 订正，旧文"mod 的唯一家 = 服务端模块根 `mods/`"已过时）：

| 路径 | 是什么 | 谁写它 |
| --- | --- | --- |
| `<启动器根>\mods\` | **mod 库根**：既放`.lua` 规则脚本（**平铺**，服务端读盘的**第一顺位**），也是启动器「MOD 工具」页显示的库；`newchar_kit.lua` 这类手工脚本就在这儿 | 人 / modkit 落位 / 管理器 |
| `<服务端模块>\mods\`（本机 `server/work/dfo-lan/mods/`） | **服务端源码 mod**（Go 包 `mods/<id>/`）、生成的 `zz_mods_gen.go`、`enabled.json`；历史位置 `<模块根>\mods\scripts\` 仍被只读兼容 | 只有 `modkit install/uninstall` 与 mod 管理器 |
| `mods/`（**本仓库根**，就是本文件所在的目录） | 示例源码 + 开发文档（给人看的）；`newchar-kit/`、`examples/` 在这儿 | 人手写 |

> 一句话记法：**规则脚本看启动器根的 `mods/`，Go 钩子看服务端模块的 `mods/`。**
> 两者的精确搜索顺序见 §4.5.4。

---

## 0. 五分钟上手

```powershell
# 1) 拿到引擎
#    modkit.exe 由启动器仓构建：go build -o modkit.exe ./cmd/modkit

# 2) 看接口（永远以这个输出为准）
modkit layers

# 3) 校验你的包（不动盘）
modkit verify --mod MyMod.zip

# 4) 预演（只读，告诉你每一步会做什么、哪里会被阻断）
modkit plan --client <客户端根> --mod MyMod.zip --root <启动器根>

# 5) 装（先退游戏）
modkit install --client <客户端根> --mod MyMod.zip --root <启动器根>

# 6) 卸（逐字节还原）
modkit uninstall --client <客户端根> --id my.mod --root <启动器根>
```

`--root` = 启动器根（含 `server/work/dfo-lan`）或服务端模块根本身。只有 server 层需要它。

现成例子：`mods/examples/hello-verify/`（可安装、可验证）。

---

## 1. mod 包长什么样

**只接受带组织文件的 zip。** 目录形态仅开发自测（`--allow-dir`）。

```
MyMod.zip
  mod.json          组织文件，必须在根，必须 schema 2
  README.md         可选
  server/           server 层（清单声明了就必须有）
    mod.go          你的 Go 包；必须导出 func Register()
  pvf/              pvf 层
    check.ps1
  client/           client 层
    myfile.txt
  resource/         resource 层
    a.img
```

**约定即校验**，`verify` 在动盘前查完，任一条不符整包拒绝：

1. 根有 `mod.json` 且 `schema: 2`；
2. 声明了哪一层，包里必须有那个目录；
3. 每条 `source` 文件存在；`size` + `sha256` 是**可选声明**（2026-10-06 起不再阻断安装）：写了就核对，
   与包内实测不符只输出一条**警告**，落位的仍是包内那份实际内容
   （启动器仓 `internal/modkit/package.go` 的 `warn` 段、`internal/modkit/manifest2.go:42-54` 的 `Warnings()`）；
4. 路径必须相对（无绝对路径/盘符/`..`/反斜杠/保留设备名）；
5. 权限位覆盖每条动作（见 §3）；
6. server 层钩子名在白名单内（见 §4.2）；
7. 声明了层却没有动作 → 拒绝。

---

## 2. `mod.json` 完整模板

```jsonc
{
  "schema": 2,
  "id": "my.qol.mod",              // 小写字母/数字/./-，≤64，全局唯一
  "version": "1.0.0",
  "name": "我的体验 mod",
  "author": "你的名字",             // 可选：署名。会显示在 modkit 输出与 mod 管理器里
  "description": "一句话说清它改什么", // 可选：一句话说明；与 author 走同一条展示链
  "permissions": ["server.hook", "client.file.write"],   // 必须覆盖下面所有动作
  "requires": ["other.mod"],       // 可选：必须先装的 mod

  "layers": {
    // ---- server 层：服务端进程行为（编译期注册）----
    "server": {
      "package": ".",              // server/ 内的 Go 包相对路径，默认 "."
      "hooks": [
        { "name": "server.boot", "since": "dev", "note": "启动时读有效配置并记日志" }
      ]
    },

    // ---- client 层：客户端整文件 / EXE 字节补丁 ----
    "client": {
      "ops": [
        { "kind": "file.add",     "target": "mods-demo/a.txt",
          "source": "client/a.txt", "size": 12, "sha256": "<64 hex>" },
        { "kind": "file.replace", "target": "Fonts/x.ttf",
          "source": "client/x.ttf", "size": 1, "sha256": "<64 hex>" },
        { "kind": "exe.patch",    "target": "DFO.exe",
          "sourceSHA256": "<现场 EXE 的 sha256>",
          "patches": [ { "offset": 123, "before": "ddea00", "after": "eb0501" } ] }
      ]
    },

    // ---- pvf 层：Script.pvf / sk.dat ----
    "pvf": {
      "ops": [
        { "kind": "verify", "script": "pvf/check.ps1", "readOnly": true,
          "args": ["-ClientDir", "{client}", "-OutputDir", "{work}"] },
        { "kind": "merge",  "script": "pvf/merge.ps1", "work": "merge",
          "args": ["-ClientDir", "{client}", "-OutputDir", "{work}"],
          "produces": ["Script.pvf", "sk.dat"] },
        { "kind": "replace", "target": "Script.pvf",
          "source": "pvf/Script.pvf", "size": 1, "sha256": "<64 hex>" }
      ]
    },

    // ---- resource 层：NPK ----
    "resource": {
      "ops": [
        { "kind": "npk.add",     "target": "ImagePacks2/new.NPK",
          "source": "resource/new.NPK", "size": 1, "sha256": "<64 hex>" },
        { "kind": "npk.replace", "target": "ImagePacks2/old.NPK",
          "source": "resource/old.NPK", "size": 1, "sha256": "<64 hex>" },
        { "kind": "npk.entries",
          "entries": [ { "archive": "ImagePacks2/sprite.NPK",
                         "entry": "sprite/xxx.img",
                         "source": "resource/xxx.img", "size": 1, "sha256": "<64 hex>" } ] },
        { "kind": "npk.index",   "target": "ImagePacks2/NpkIndex.etc",
          "script": "resource/rebuild-index.ps1", "work": "index",
          "args": ["-ClientDir", "{client}"] }
      ]
    }
  }
}
```

占位符（脚本参数与 `args` 里可用）：`{client}` 客户端根、`{pack}` 包根、`{work}` 本步骤工作目录。

### 2.1 署名与说明（`author` / `description`）

- **填写位置只有一处**：包根的 `mod.json`。两个字段**都可选**，不填不影响校验与安装。
- **传播链**（装完之后包往往已不在盘上，所以引擎会留快照，**但只对带 Go 包的 mod 有效**）：
  - 声明了 `hooks` 的 mod：`modkit install` 把清单**原样**落一份到
    `<服务端模块>/mods/<mod-id>/mod.json`（启动器仓 `internal/modkit/install2.go:472` 的
    `applyServerGoPackage`，写文件在 `:524-529`）——元数据就是这么留下来的；
  - **只带 `scripts` 的 mod 不建 `mods/<mod-id>/`，所以也不会落这份 `mod.json`**
    （`install2.go:472` 的注释写明"声明了 hooks 的 mod 才走这里"）：这类 mod 的名称/版本在
    客户端注册表的**安装记录**里（`id`/`name`/`version`），而 `author`/`description` **不会**被带过去
    —— 注册表条目结构里根本没有这两个字段（启动器仓 `internal/modkit/modkit.go:71-93`）；
  - 因此管理器显示作者/说明时，**真实来源是扫 `<整合包根>\mods\` 下各 mod 的 `mod.json`**
    （启动器仓 `internal/modlib/store.go:190-231`），不是注册表快照；
  - `modkit verify` / `plan` 直接打印清单里的它们；
  - `modkit status` 从注册表读（包删了也还看得到安装记录，但看不到作者/说明）；
  - `modkit mods list [--json]` 也**不输出** `author`/`description`
    （条目结构见启动器仓 `internal/modkit/modsadmin.go:46-67`）——管理器那两列走上面的 store 扫描。
- **展示规则**：说明里的换行在命令行输出里会被压成一行（避免把后续行顶歪）；
  JSON 输出保持原文。
- **兼容**：旧 mod（schema 1，或压根没填这两项）照旧装、照旧跑——缺字段不会被拒装，
  展示侧也不会凭空编造。

---

## 3. 权限位

| 权限 | 何时必须写 |
| --- | --- |
| `client.file.write` | client 层有 `file.add` / `file.replace` |
| `client.exe.patch` | client 层有 `exe.patch` |
| `exec.script` | pvf 层（要跑 .ps1） |
| `server.hook` | server 层写了 `hooks`（Go 钩子，**装完要重新编译服务端**） |
| `server.script` | server 层写了 `scripts`（Lua 规则脚本，只为它写包时**没有 Go 代码**；装完**不重编译**，只要重启服务端） |
| `pvf.merge` | 声明了 pvf 层 |
| `resource.npk` | resource 层有整份 NPK 操作或条目级覆盖 |
| `resource.index` | resource 层有 `npk.entries` 或 `npk.index` |

权限是**给操作者看的**：漏写等于隐瞒，引擎直接拒绝。

---

## 4. server 层：怎么让服务端"真的跑你的代码"

### 4.1 原理

服务端是**就地 `go build`** 的，所以你的 mod 就是**一个参与编译的 Go 包**：

```
你的包 server/  ──modkit install──>  服务端模块 mods/<mod-id>/
                                      mods/zz_mods_gen.go（生成）import 你并调用 Register()
                                                      ↓
                              服务端启动： mods.RegisterMods() → servermod.Boot()
```

签名写错 → 服务端**编不出来**。所以不存在"装了但没生效"的静默失败。
`modkit install` 落位后还会先跑一次静态验证编译，编不过就整体回滚。
**它是逐个已装 mod 目录编译的**（`go build ./mods/<id>`，见启动器仓
`internal/modkit/install2.go:579-611`），**不是** `go build ./mods/...`：后者会把
`examples/` 之类目录里的构建产物一起扫进来，用无关错误把"你的 mod 编不过"这个信号淹掉。

> **两个前提必须先满足**（旧文没写，缺了会在安装期硬失败）：
> 1. **本机要有 Go 工具链**（声明 `hooks` 的 mod 才需要）：找不到就地编译用的 Go 时，
>    `modkit install` 直接报错退出（`install2.go:113-116`、`install2.go:589-592`），
>    `--skip-verify-build` 只是跳过"落位后的静态验证"，并不等于不需要 Go；
> 2. **随包的服务端二进制可能连 mod 宿主都没有**：宿主代码在
>    `server/work/dfo-lan/internal/servermod` 与 `cmd/wireprobe/servermods.go`，只有**按当前服务端源码重编译**
>    的程序才带它。随包的预编译 `wireprobe-pvf.exe` 若没有宿主，装好的 mod 一个都不会被装载，
>    而启动日志里**连 `servermod:` 这一行都不会出现**（判据见 §9）。

### 4.2 三个钩子点

| 钩子 | 什么时候被调用 | 你能用它做什么 |
| --- | --- | --- |
| `server.boot` | 配置与存储就绪、**还没开始监听** | 自检、读配置、记日志、声明内容扩展意图 |
| `console.command` | 启动期一次性命令（`DFO_SERVERMOD_CONSOLE`）或 UI 触发 | 现场动作、打印状态 |
| `reward.script` | 启动装配奖励管线时 | 向奖励管线登记一份事件奖励规则（Lua 脚本）。**这是 mod "发东西"的正路**：复用既有的幂等发放 / 事务 / 邮件语义，不要自己造第二条发放路径 |
| `protocol.response` | 一条 S2C 报文写客户端之前 | **只读观察**（记录/统计），不能改写报文 |

钩子名只能在白名单里；自己发明名字会被 `verify` 拒绝。

### 4.3 六个宿主机操作

| 操作 | 说明 |
| --- | --- |
| `log` | 写服务端日志，自动带 `[mod <id>]` 前缀 |
| `config.read` | 读只读快照（`DFO_*` 环境变量 + main 注入的键） |
| `config.write` | 写**进程内**值，键必须已存在；刻意不落盘 |
| `content.register` | 声明内容扩展**意图**（第一期只登记，等 PVF 扩展点） |
| `console.reply` | 往控制台回一行文本 |
| `reward.register` | 把一份**事件奖励规则**（Lua 脚本）交给奖励管线。`reward.script` 钩子开放给 mod 的就是它（启动器仓 `internal/modkit/layer.go:120-140` 的 `HostOpNames()`） |

（权威清单以 `modkit layers` 的输出为准。）

### 4.4 最小可用模板

**每个 `.go` 文件都必须以 `package modpkg` 开头**（引擎在落位时强制校验）：

- 不能是 `package main` —— Go 不允许 import 程序，会报
  `import "dfolan/mods/x" is a program, not an importable package`；
- 必须**统一**叫 `modpkg` —— 生成的 import 清单用默认 import，
  于是引用的标识符就是包名，不统一就对不上。

```go
// 服务端模块 mods/<mod-id>/mod.go
package modpkg   // ← 固定这个名字，不要改

import (
    "fmt"

    "dfolan/internal/servermod"
)

const modID = "my.qol.mod"

func Register() {
    servermod.RegisterBoot(modID, boot)
    servermod.RegisterConsole(modID, console)
    servermod.RegisterConsoleHelp(modID, "status | env <KEY> | flag <KEY> <VALUE>")
    // 只在你确实要观察报文时才登记（它有运行期开销）
    // servermod.RegisterResponse(modID, observe)
}

func boot(ctx *servermod.BootContext) error {
    // 自检失败请 return error —— 服务端会拒绝启动（fail-closed）
    servermod.Logf(modID, "启动自检通过，服务端版本=%s", ctx.Version)
    if v, ok := servermod.ConfigRead("DFO_SHOP_OPEN_ALL"); ok {
        servermod.Logf(modID, "%s=%s", "DFO_SHOP_OPEN_ALL", v)
    }
    return nil
}

func console(cmd servermod.ConsoleCommand) (bool, error) {
    switch cmd.Name {
    case "status":
        servermod.ConsoleReply(fmt.Sprintf("[%s] 运行中；服务端版本见启动日志", modID))
        return true, nil
    case "env":
        if len(cmd.Args) != 1 {
            return true, fmt.Errorf("用法：env <KEY>")
        }
        v, ok := servermod.ConfigRead(cmd.Args[0])
        servermod.ConsoleReply(fmt.Sprintf("%s=%s（存在=%v）", cmd.Args[0], v, ok))
        return true, nil
    case "flag":
        if len(cmd.Args) != 2 {
            return true, fmt.Errorf("用法：flag <KEY> <VALUE>")
        }
        if err := servermod.ConfigWrite(cmd.Args[0], cmd.Args[1]); err != nil {
            return true, err
        }
        servermod.ConsoleReply(fmt.Sprintf("已设置 %s=%s（仅本次进程）", cmd.Args[0], cmd.Args[1]))
        return true, nil
    }
    return false, nil // 不是我的命令，让别的 mod 试
}

func observe(conn string, opcode uint16, body []byte) {
    // 只读！改不了报文。
    _, _, _ = conn, opcode, body
}
```

**注意**：`RegisterBoot` 的回调里 `return error` = 服务端起不来。别拿它做可选检查——
可选信息请只记日志。

**固定包名的约束**：`mods/` 下每个 mod 都叫 `modpkg`，所以**不要**在 mod 之间互相 import
（Go 里同名包无法区分）。mod 之间的协作请走 `servermod` 的钩子与操作，不要直接耦合。

---

---

## 4.5 发东西（奖励规则）——mod 最常用的能力

服务端已有一条成熟的**事件奖励管线**，mod 应当复用它而不是自己发道具：

```
character.Service.Create ──新角色提交成功──> reward.Service.CharacterCreate
                                                  │
                                     Lua 规则 on("character_create", ...)
                                                  │
                              grant_item / send_mail(带附件) / grant_cera
                                                  │
                        CommitCharacterEvent（稳定幂等键）/ CommitSystemMail
```

它自带：**幂等键**（事件重放不重复发）、事务、背包满转邮件、装备实例重建。
自己写一条发放路径，等于把这些全部重写一遍，还引入第二套内容真源（违反 AGENTS §0.2）。

### 4.5.1 怎么做

mod 在自己的 `Register()` 里交一份 Lua 规则给管线：

```go
package modpkg

import (
    _ "embed"
    "dfolan/internal/servermod"
)

const modID = "my.reward.mod"

//go:embed rules/my-rule.lua
var rule []byte

func Register() {
    if !servermod.Enabled(modID) { return }   // ← 见 4.6
    if err := servermod.RegisterRewardScript(modID, "my-rule.lua", rule); err != nil {
        panic("登记奖励规则失败: " + err.Error())
    }
}
```

`mod.json` 里声明钩子 `reward.script`。

### 4.5.2 Lua 规则可用的 API

| 函数 | 作用 |
| --- | --- |
| `on("character_create", fn)` | 注册新角色创建事件（还有 `level_up`、`quest_complete`） |
| `grant_item(id, count)` | 直接进背包：`id 0` = 角色金币；**是堆叠物就按堆叠发，否则按装备发**（服务端取 PVF 里的装备定义并**实例化**，写进普通装备栏；宠物装备走宠物装备栏）。2026-10-06 起**装备也能直接发** |
| `send_mail(subject, body[, attachments])` | 系统邮件；`attachments = { {id=模板, count=数量}, ... }`（≤11 件）。附件同样"是堆叠物按堆叠发、否则按装备发"，**不再要求模板先在奖励目录/掉落池里** |
| `grant_cera(amount)` | 账号级点券 |

**角色待遇**（2026-10-06 新增）：这些是**存档字段、不是物品**，`grant_item` / `send_mail` 碰不到它们，
所以单开一组能力。语义一律"只升不降/累加"，**没调就不碰**，绝不会把玩家已有的待遇改小：

| 函数 | 作用 |
| --- | --- |
| `unlock_equip_slots(mask)` | 扩展装备栏挂锁：把 mask **按位或**进 `expand_equip_flags`。位：`support=1`、`magic stone=2`、`aura skin=8`、`earring=16`、`creature skin=32`（**五个全开 = 59**） |
| `expand_bag(tier)` | 背包扩容**档位**（客户端按 `40+8×档位` 算容量）：只升不降，超上限钳到 **2**（源商城只卖到 2 档） |
| `expand_avatar(tier)` | 时装栏扩容档位：只升不降，钳到协议上限 |
| `grant_revive_coin(count)` | 复活币：累加，到顶停在 `MaxUint32`（不回绕成 0） |
| `grant_pet(template)` | 宠物**本体**：落宠物容器 `0..139`（不是普通装备栏）。模板必须是 `[creature]`、**不能是宠物蛋**、一次 1 只 |
| `grant_pet_item(template, count)` | **宠物用品**（饲料/改名卡）：落 `376..431`，按堆叠规则合并。模板必须是 `[feed]` / `[creature]` 类堆叠物 |
| `expand_vault(space, slots)` | 金库容量：`space` **2**=金库1、**45**=金库2（`8..264`、步长 16）、**12**=账号金库（8 的倍数、≤320）。只升不降 |
| `grant_account_material(template, count)` | **账号材料仓**：按模板累加，落库时映射到仓库固定槽位。模板不在服务端映射里的会被拒并点名 |
| `unlock_skins()` | **皮肤仓库全解锁**（账号级，幂等）。清单由服务端按**运行期皮肤目录**现算，不写死在脚本里 |

**宠物装备不用这两条**：`[artifact *]` 跟其它装备一样走 `send_mail`，服务端领取时会放进宠物栏 `320..375`。

**落库分派**：角色 state 那部分（挂锁/档位/复活币/宠物）是一笔角色事件事务（键后缀 `:state`）；
账号金库容量（键后缀 `:account-vault`）、账号材料仓（键后缀 `:account-material`）各走自己的表与幂等键；
皮肤仓库走 `ON CONFLICT DO NOTHING`，**没有键后缀**。所以其中一项失败不会连累其它项，重放也不会互相吞掉。

> **个人金库没有幂等键**（旧文写的 `:vault-1` / `:vault-2` **不存在**）：金库1/金库2 都走
> `internal/database/vault.go` 的 `GrantVaultSlots(account, id, version, initial, slots, secondary)`
> —— 签名里根本没有 key，它靠"只升不降"天然幂等（`cmd/wireprobe/reward_flow.go:275-302`）。

一个 handler 里这几条与发放一样是**先收集、事件处理完一次落库**（键后缀 `:state`，与发放同一套
稳定键语义，重放不会重复改）。越界入参（mask > 255、count ≤ 0、未知 space）会报错并记一条日志，不静默截断。

`ctx` 字段：`type` / `level` / `quest_id` / `character_id` / `account_id` / `name`，
以及**新角色创建时才有的职业信息** `profession`（基础职业号）/ `advancement`（转职号）。> **职业字段是"可选存在"的**（2026-10-06 起，服务端 `internal/reward` 的 `Recipient`
> 带 `HasProfession`）：只有 `character_create` 事件会填它，`level_up` / `quest_complete`
> 一律**不写进 ctx**。所以规则要写成"拿不到就跳过/走默认"，而不是把 0 当成"没职业"——
> **基础职业 0（鬼剑士）是合法值**：
>
> ```lua
> local prof = tonumber(ctx and ctx.profession)
> if prof == nil then return end          -- 拿不到职业：本段跳过
> local adv = tonumber(ctx and ctx.advancement) or 0
> ```
>
> 判断"输出/辅助"必须**两个一起看**（同一基础职业里既有输出也有辅助，例：女神枪手(5)
> 转职 1 = 漫游枪手（输出）、转职 5 = 协战师（辅助））。

**装备两条通道都能发**（2026-10-06 起）：`grant_item` 与 `send_mail` 附件走的是**同一套分流** ——
是堆叠物就按堆叠发；否则按**装备**发，服务端从 PVF 取耐久与部位并**实例化**。
旧文说"装备必须走 `send_mail`、`grant_item` 只能发堆叠物"**已过时**：
`internal/inventory/awards.go:49-91` 就是 `grant_item` 的装备分支（走 `AddEquipment` 实例化、
宠物装备走 `AddPetGear`），`mods/newchar-kit/server/rules/newchar_kit.lua` 也直接在用它。

**能不能发是服务端能力，发什么由你（mod 脚本）决定**（2026-10-06 定调）。具体到两条通道：

| 通道 | 服务端保证 | 你要负责 |
| --- | --- | --- |
| `grant_item(id, n)` | `id 0` = 角色金币；其余必须是**堆叠物**（物品索引里 `kind=stackable`）。**一次调用是一整批**：里面有一个号不合格，整批（连金币）都不会发，日志里会点名那个模板号 | 池子里的模板号必须真实存在且是堆叠物 |
| `send_mail(..., 附件)` | 每个附件：是堆叠物就按堆叠发；否则按**装备**发 —— 只要 **PVF 里有这件装备的定义**（不必在掉落池/奖励选集里），服务端就能取到耐久与部位并实例化。每封 ≤ 11 件 | 模板号要对；装备附件的 `count` 必须是 1 |

**报错一定会带上模板号**（`模板 <N> …`），所以池子里哪个号错了，看服务端日志一眼就能定位 ——
这是刻意的：规则的池子通常是一串手抄的模板号，没有号就没法查。

**发不出去的三种情况**（都会明确报错、不影响角色创建本身）：
模板号在 PVF 里根本不存在；不是堆叠物却用 `grant_item` 发；装备附件 `count != 1`。

### 4.5.3 完整示例

`examples/giveaway-random-equipment/` —— 新角色创建时发金币 + 系统邮件
（示例刻意用"必定成功、不依赖内容模板"的发放，便于验证 mod 是否真的注册生效）。
它演示了嵌入 Lua 规则、发邮件带装备附件、以及被管理器勾选/取消勾选。

**另一个真实例子**：[`../newchar-kit/`](../newchar-kit/)（整合包自带）—— 一整套出厂补给，
**当前版只有一个脚本** `server/rules/newchar_kit.lua`（`mod.json` 的 `layers.server.scripts` 就声明这一项），
**没有 Go 代码**、不参与编译。要看"职业判定怎么写、邮件怎么按 11 件切分、装备怎么发"，看它比看示例更直接。
（它另有一份**不参与安装**的"两层两个脚本"备用变体在 `variants/two-layer/`，只有把 `mod.json` 的
`scripts` 换过去才会启用 —— 别把备用变体当成当前形态。）

**规则脚本的失败是可观测的**：模板号填错时奖励管线会记一条 grant/mail 失败日志，
角色创建本身不受影响（奖励是创建之后的可选步骤）。所以池子可以先粗后细。

### 4.5.4 落盘补充口（不重编译加规则）与内置兜底规则集

**先把"谁赢"说清楚**（旧的"只说磁盘优先、不提内置"是 22 条不一致里最坑的一条）：

- 规则来源有两份，合成一个只读视图交给奖励管线：
  ① **内置规则集**（编译进二进制，`internal/reward/scripts/level.lua`、`newchar.lua`、`quest.lua`）；
  ② **磁盘/mod 脚本**（下面这四层目录）。
- **同名时磁盘/mod 脚本胜出，内置集只是"兜底"**：磁盘上没有同名脚本时才轮到内置那份
  （启动器仓 `internal/modkit/rewardscripts.go:26-35`、`:432-436` 的 `ProvidedBy` 注释口径）。
- ⚠️ **实现核对状态（2026-10-07）**：文档按**磁盘优先 + 上面的四层搜索顺序**写，
  但**已提交的 `fork/main` 基线还没有这一版实现** —— 它的 `rewardScriptFS()` 仍是
  "`compositeScriptFS{first: bundled, second: <模块根>/mods/scripts}`"（**内置在前**），
  且 `cmd/wireprobe/servermods.go` 里**没有** `rewardScriptsDirs()`、**不认** `DFO_REWARD_SCRIPTS_DIR`。
  带多目录 + 环境变量的那一版目前只在**工作区**（另一个任务正在把它带进 fork）。
  **本文档不迁就旧实现**，按已定口径写"磁盘优先"；实现落地后本节即为事实。
  在它落地前，请按 §9 的自证手段（尤其 `reward rules enabled …` 与 `reward scripts: 磁盘规则脚本目录 …` 两行）
  确认现场到底是哪一份在跑，并**别用 `level.lua` / `newchar.lua` / `quest.lua` 这三个名字**
  （旧实现下会静默被内置顶掉）。

**磁盘脚本的搜索顺序**（多个目录合成一个视图，按**文件名**去重、同名只执行先命中的那份）：

```
① 启动器注入的 DFO_REWARD_SCRIPTS_DIR（启动器把 <启动器根>\mods 的绝对路径挂进服务端进程环境）
② <包根>\mods\            ← 正式位置：mod 库根，平铺 *.lua（2026-10-06 业主口径）
③ <包根>\mods\scripts\    ← 2026-10-06 过渡位置
④ <模块根>\mods\scripts\  ← 最早的位置（启动器旧版落位点）
```

（`cmd/wireprobe/servermods.go` 的 `rewardScriptsDirs()`；①优先于②，②③④只读兼容。
"包根"= 服务端模块根往上三级，本机就是整合包根 `C:\Game\dof\115us\115`。）

> **命名警告**：别把自己的脚本叫成 `level.lua` / `newchar.lua` / `quest.lua`（内置规则集的名字），
> 除非你**确实要覆盖内置规则**。叫了这三个名字，在"磁盘优先"落地后就是**静默顶掉内置那一份**；
> 在它落地前则是你那份被内置顶掉、一条都不跑。两种情况都很难查。

**它也是"只带规则、不带代码"的 mod 的落位目标**（2026-10-06 起）。这样的 mod
不需要写任何 Go、也不需要重新编译服务端 —— 清单里声明 `scripts` 就行：

```json
{
  "schema": 2,
  "id": "myserver.rules",
  "version": "1.0.0",
  "name": "我的规则",
  "permissions": ["server.script"],
  "layers": {
    "server": { "scripts": ["server/rules/newchar-gift.lua"] }
  }
}
```

- `scripts` 的每一项是**包内相对路径**（必须在 `server/` 目录下）、必须是 `.lua`；
- **落位位置 = `<启动器根>\mods\<文件名>`**（**启动器根**的 `mods/`，与各 mod 库目录同级平铺，
  不再有 `scripts/` 子目录 —— 启动器仓 `internal/modkit/rewardscripts.go:73-89`）。
  落位文件名校验很严：必须平铺、不许带目录、不许以 `.` 开头：服务端就是按平铺 `*.lua` 枚举脚本的，
  带目录的名字它读不到（那种"装了却没生效"必须在落位前报出来）；
- **文件名是平铺的**：同一个 mod 内不能重名、跨 mod 也不能同名 —— 后者会被 `plan` 拦下；
- 一份清单里 `hooks` 与 `scripts` 可以同时写；**只写 `scripts` 时包里不需要 `.go` 文件**，
  装的时候不建 `mods/<mod-id>/`、不改加载器、也不跑编译；
- 权限位要 `server.script`（和 Go 钩子的 `server.hook` 分开：后者要重编译，前者不要）；
- **卸载 mod 不删脚本**（2026-10-06 业主定调，旧文"按注册表逐份核对哈希撤掉"不对）：
  用户主动卸载时脚本**一律留在原地**变成"手工脚本"，只摘掉注册表条目
  （启动器仓 `internal/modkit/rewardscripts.go:31-35,384-389`、
  `internal/modkit/uninstall2.go:123-124,169-172,187-201`）。
  只有**安装失败/升级的内部回滚**才按登记的哈希删（现场被改过就保留现场，`rewardscripts.go:390-393`）；
- 改完要**重启服务端**才生效（脚本是启动时一次性读进内存的，无法热摘）。

> **想停用一份已落盘的规则脚本怎么办？** 只有一条路：**把 `<启动器根>\mods\<名>.lua` 删掉**
> （或在启动器「MOD 工具」页下半部分「Lua 规则脚本」里删）。`enabled.json` **管不到它** ——
> 那份清单只管"哪些编译进去的 mod 在启动时调 `Register()`"，而磁盘 `*.lua` 是服务端**无条件**读盘的。

这一块在启动器「MOD 工具」页里也能直接管（页面下半部分「Lua 规则脚本」）：列出 / 新建 /
编辑 / 删除 / 导入 `.lua` / 打开目录。**由 mod 安装落位的脚本在那页是只读的** ——
要改就卸载那个 mod（手工改会让它的还原凭据失效）。

---

## 4.6 启用 / 禁用（mod 管理器）

管理员可以在启动器的 mod 管理器里勾选/取消勾选。**这是运行期门禁，不是编译期裁剪**：

- 装/卸决定"哪些被编译进二进制"（要重编译）；
- 开/关决定"这次启动让哪些真正注册钩子"（**只要重启服务端**）。

**mod 的 `Register()` 必须在开头就问**：

```go
func Register() {
    if !servermod.Enabled(modID) { return }   // ← 被禁用就直接返回
    ...
}
```

漏了这一行，你的 mod 会**无视管理员的勾选**一直生效——这是作者侧最常见的错。

清单文件：`<服务端模块>/mods/enabled.json`（**禁用名单**，缺省=全部启用）。
mod 只读；写只有两个入口：管理器 UI、或 `modkit mods enable/disable`。

> 完整的管理器接入契约（管理器要读什么、调什么、显示什么）见
> [`MOD-MANAGER-INTEGRATION.md`](MOD-MANAGER-INTEGRATION.md)。

**边界**：奖励规则脚本在服务端启动时一次性加载进 Lua state，**无法热摘**。
所以改启用状态后必须重启服务端；管理器应当提示或代劳。

**注意 `enabled.json` 管不到"只带脚本的 mod"**（§4.5.4）：那份清单只决定"哪些编译进去的
mod 在启动时调 `Register()`"，而 `<启动器根>\mods\*.lua` 是服务端**无条件**读盘的。
要停掉一条落盘规则，只能删掉/移走那个 `.lua`（管理页给的就是删除，不做"重命名式停用"——
那等于发明第二套启用状态）。


## 5. client 层

三种动作，语义与旧版一致：

- `file.add`：目标**必须不存在**。已存在且同内容 → 幂等跳过；已存在且不同 → 拒绝（要覆盖请用 `file.replace`）。
- `file.replace`：目标**必须存在**；自动备份原文件，卸载时逐字节还原。
- `exe.patch`：**没有整文件哈希门**（2026-10-06 起移除，旧文"整文件哈希门"已过时）。
  清单里的 `sourceSHA256` 只是**可选的前置声明**：现场 `DFO.exe` 的 sha256 与它不符时
  **只记一条警告、不阻断安装**，能不能打由**逐处 offset 的 `before` 字节比对**决定 ——
  任一处对不上，那一步才失败（启动器仓 `internal/modkit/manifest2.go:261-265`、
  `internal/modkit/layerplan.go:588-609`、`internal/modkit/apply2.go:191-232`）。
  为什么要改：这个门挡住的从来不是"补丁打错位置"，而是"这个 EXE 我不认识"；
  真正的定位正确性由逐字节 `before` 证明。**报错一定指出是哪一处补丁、期望字节与现场字节**。

**注意**：client 层会让 modkit 要求游戏退出（装/卸都要求 `DFO.exe` 不在运行）。

### 5.1 客户端 DLL：只有一个槽位，多个 DLL 走插件通道

客户端**只有一个**能被自动加载的 DLL 槽位（115us 整合包里的 `dinput8.dll` 代理会
`LoadLibraryEx` 客户端根下的 `ChineseLocalization.dll`，再调它的 `StartLocalization`）。所以：

1. **第一个**客户端 DLL mod 直接占这个槽位（`file.add ChineseLocalization.dll`），
   而且它应当只干一件事：把 `<客户端>\.115us-mods\*.dll` 按文件名顺序加载起来 ——
   这就是宿主 [`qol.client-host`](../../client-patchs/client-host/HOST-README.md)；
2. **其它**客户端 DLL mod 一律当**插件**投放（`file.add .115us-mods/<你的>.dll`），
   并在 `mod.json` 里声明 `"requires": ["qol.client-host"]`：宿主没装时 `plan` 会直接挡下
   （`依赖未安装`），这是设计行为；
3. 插件导出约定（宿主按名解析；缺 `ModStart` 时回退 `StartLocalization`，两个都没有只记日志不崩）：

   | 导出 | 必需 | 说明 |
   | --- | --- | --- |
   | `DWORD WINAPI ModStart(void)` | 是 | 插件入口。宿主在**自己的线程**上调用它 → 插件要干重活请自建线程，别阻塞宿主 |
   | `const char *WINAPI ModName(void)` | 否 | 展示名，宿主日志里会打出来 |
   | `DWORD WINAPI StartLocalization(void)` | 否 | 兼容名（早期代理就是按这个名字调的） |

4. **日志**：每个插件写**自己所在目录**（解析自身模块路径，不依赖进程当前目录、
   更不要写游戏根目录）；宿主日志固定写在客户端根 `client-host.log`；
5. **改内存的纪律**（inline 跳转 / 虚表槽替换 / 改导入表）：动手前**逐字节核对期望字节**，
   地址不可读或字节不符就**跳过并记日志** —— 客户端版本一变，宁可失效也不能崩。
   细则见 [`client-patchs/AGENTS.md`](../../client-patchs/AGENTS.md)，可复跑的机制自测在
   [`client-patchs/tests/`](../../client-patchs/tests/)（inline 跳转 / 虚表替换 / 门禁复刻 / Themida 导入槽）；
6. **开关文件由插件首次运行时自己生成**，不要放进包里：`file.add` 不允许覆盖
   「已存在且内容不同」的文件，把 ini 放包里会让**下一次升级被 `plan` 阻断**；
7. 现成例子：中文输入 [`qol.chinese-input-probe`](../../client-patchs/chinese-input/PROBE-README.md)、
   删角色免打字 [`qol.auto-confirm`](../../client-patchs/auto-confirm/AUTO-CONFIRM-README.md)；
   可直接分享的包在 [`../mods/client-mods/`](client-mods/README-安装与分享.md)。

---

## 6. pvf 层

- `verify`：只读干跑校验，脚本退出码必须为 0；
- `merge`：跑合并脚本 → 按 `produces` **成对**替换 `Script.pvf` + `sk.dat`
  （只登记一个会被拒绝：客户端与内层归档必须匹配）；
- `replace`：直接用包内（或脚本产物）的 `Script.pvf`/`sk.dat` 替换。

脚本 **必须 pwsh 7**（`-NoProfile -NonInteractive -ExecutionPolicy Bypass`）。
引擎找 pwsh 的顺序：`--pwsh` 指定 → PATH → `<客户端工作区>\tools\pwsh7\pwsh.exe`。
本机若没装 pwsh 7，含 pvf 层的 mod 会安装失败 —— 这是明确报错，不是静默跳过。

---

## 7. resource 层与 NPK

### 7.1 NPK 容器格式（已用真实客户端归档实测）

```
0x00  16B  magic "NeoplePack_Bill\x00"
0x10  u32  条目数 N
0x14  N × 264B 索引，每条：
        0..3    u32 数据段偏移（绝对）
        4..7    u32 数据段长度
        8..263  256B 条目名 —— XOR 加密：
                  prefix = "puchikon@neople dungeon and fighter "
                  key[i] = prefix[i] (i < len(prefix))，其余 = "DNF"[(i-len(prefix)) % 3]
                  decoded[j] = raw[8+j] ^ key[j]
随后是各条目载荷，连续打包（同载荷可被多条索引共享，即别名条目）
```

### 7.2 条目级覆盖（`npk.entries`）

- **只替换已存在的条目**，不新建条目（新建需要作者提供完整条目元数据，本期不支持）；
- 载荷是**原始压缩字节**，引擎不重新压缩；只换目标条目，其余条目逐字节保留；
- 改名会重排载荷并重建索引 —— 所以**没有"新图必须装得进老格子"的限制**
  （这是相对旧汉化 modtool 的关键改善）；
- 目标条目不存在 → 整包拒绝并回滚。

### 7.3 怎么做一次真正的"图像替换"（作者视角）

`.img` 是 `Neople Img File` 版本化容器，**帧结构与压缩枚举没有完整逆向**，
所以**不要手写 `.img`**。正确做法是把游戏已有的条目取出来改：

1. 用你惯用的 NPK/img 工具从**你自己客户端**的归档里导出目标 `.img`
   （也可用仓库外 `analysis-tools` 里的工具集）；
2. 用图像工具改它（**保持尺寸与格式一致**）；
3. 存回 `.img`；
4. 把新 `.img` 放进 mod 包的 `resource/`，清单写：

```jsonc
{ "kind": "npk.entries",
  "entries": [ { "archive": "ImagePacks2/sprite.NPK",
                 "entry": "sprite/nowloading_default.img",   // 必须与归档里的名字逐字符一致
                 "source": "resource/nowloading_default.img",
                 "size": <新文件字节数>, "sha256": "<新文件 sha256>" } ] }
```

`size`/`sha256` 怎么填：这两项是**可选**的，不填也能装/卸（不填就少一道"包内内容与声明不符"的警告）。
要填就用 `Get-FileHash <file> -Algorithm SHA256` 与文件字节数。
⚠️ **`modkit verify` 不会替你打印实测值**（它不输出 Warnings，启动器仓 `cmd/modkit/run2.go:70-109`），
所以别指望"先随便填、让 verify 报出真值再抄回去"。

**验证建议**：先拿"改动可见但风险低"的条目试（加载图 / 光效 / 无关紧要的图标），
不要一上来就换 `interface/windowcommon.img` 这类核心 UI。

---

## 8. 装/卸/回滚语义（必须知道）

- **顺序**：server → pvf → client → resource。任何一步失败 → 逆序回滚本次已落地部分。
- **先备份 + 先写注册表日志，再动现场**：中断也能还原。
- **卸载逐字节核对**：现场必须是"我们装的状态"或"本来就是原状"，否则**保留现场并拒绝**。
- **`uninstall --force` 的真实语义**（旧文"强行用备份还原"**不对**）：
  `--force` 只是**取消那道"保留现场并跳过"的判断**，让流程继续走还原。
  而还原入口本身**照样会核对**现场哈希（既不是我们装的、也不是原始的就直接报错），
  所以实际效果是：**整次卸载报错中止**，而不是把被第三方改过的文件强行抹回去
  （启动器仓 `internal/modkit/uninstall2.go:103-109`、`internal/modkit/apply2.go:602-607`）。
  **正常卸载不需要 `--force`**，它只在"现场被别人改过、你确认要按注册表继续"时用。
- **规则脚本不随卸载删除**：只摘注册表条目（见 §4.5.4）；`--force` 也不改这一条。
- **同目标冲突**：两个 mod 声明同一目标 → 后者被阻断，绝不静默覆盖。
- **依赖是硬门禁**：`requires` 里的 mod 必须先装；**`plan` 只看"装没装"**
  （缺了给 `冲突：依赖未安装：X` 并 exit 2，启动器仓 `internal/modkit/layerplan.go:369-373`），
  **`install` 在写盘前还会再判一次，并额外拒绝"依赖装了但被 `enabled.json` 禁用"**
  （报 `依赖 mod X 当前被禁用：请先在 MOD 列表里启用它`，启动器仓 `internal/modkit/install2.go:97-102,296-327`）；
  被依赖的 mod 不允许卸载（`uninstall2.go:66-68`）。
- **幂等**：重复安装同一包是安全的（已就绪的步骤会跳过）。
- **审计**：`<client>\.launcher-mods\modkit\audit.jsonl` 全程可归因到 mod id。

---

## 9. 怎么自证生效（装完就看这三处）

> 这一节回答作者最常问的那句话：**"我装上了，可它到底跑没跑？"**
> 三处证据都要看，缺一处就可能把"静默失效"当成"已生效"。

### ① 启动器「MOD 工具」页的「生效」列

列表有 `安装状态 / 生效 / 制作人` 三列（启动器仓 `internal/appui/ui/index.html:860-862`）。
「生效」列**不是**从启动日志推的，也不是你手填的，判据是后端算出来的三条
（启动器仓 `internal/modlib/store.go:488-522`）：

1. 该 mod 在**注册表**里（装了）；
2. **服务端二进制里有这个 mod 的 id 字符串**（`bytes.Contains` 全文搜 `modID` ——
   mod 的 `modID` 常量会以字符串进二进制）；
3. 服务端程序的 **mtime 晚于**该 mod 的安装时间。

取值只有四种（`internal/modlib/store.go:162-175`）：

| 显示 | 含义 |
| --- | --- |
| `生效` | 只有客户端层 / 规则脚本那类不靠编译的东西 |
| `重启服务端后生效` | 只带 Lua 规则脚本：不参与编译，但服务端启动时才读盘 |
| `未生效（需编译服务端）` | 带 Go 钩子但二进制里没有它（没编译，或被预编译包覆盖了） |
| `—` | 没装 |

### ② 服务端启动日志

启动后按顺序在日志里找这几行（**都是启动期一次性打印的**）：

```text
servermod: 已装载 N 个服务端 mod：<你的 id>；boot 钩子 M 个；…
servermod: 启用的名单：没有 enabled.json（按全部启用处理）      # 有 disabled 条目时另有一行"已禁用 N 个 mod"
servermod: mod 提供的奖励规则脚本 N 份：<mod-id>:<脚本名>
reward rules enabled (embedded scripts + N mod script(s))     # 规则管线构造时**实际看到**几份 mod 脚本
[mod <你的 id>] 启动自检通过…                                  # 你自己在 boot 里写的日志
odyssey mode rules: 关闭 / 开启：… ← <mod-id>
drop rate rules: 不改变 / 世界掉落 ×5.00 … ← <mod-id>
```

- `servermod: 已装载 …` 来自 `internal/servermod/host.go` 的 `Description()`（`:461-488`）；
- **一行 `servermod:` 都没有**（连 `未装载` 都没有）⇒ 这份服务端二进制**根本没有 mod 宿主**：
  换用按当前服务端源码编译的程序（见 §4.1 的两个前提）；
- `reward rules enabled (embedded scripts + N mod script(s))` 是**构造顺序的见证**：
  若它是 `+0` 而上面又打了"已装载 1 个 mod"，就是"登记了但没赶上管线构造"——规则**静默失效**
  （`cmd/wireprobe/reward_flow.go:50-61`，现场见 `cmd/wireprobe/main.go:54-69` 的注释）；
- `策略已生效…` 不是引擎打的，是**mod 自己**在 `boot` 里用 `servermod.Logf` 写的
  （例：`server/work/dfo-lan/mods/odyssey.hardcore/mod.go:142`）。所以别把它当通用判据 ——
  你自己的 mod 想有这一行，就得自己在 `boot` 里打。

### ③ 客户端插件日志

客户端 DLL 插件的日志**写在插件自己所在目录**，即
`<客户端>\.115us-mods\*.log`；机器可读状态在同目录 `<客户端>\.115us-mods\*.status.json`
（例：`difficulty-rules.log` / `difficulty-rules.status.json`）；宿主日志固定在客户端根
`client-host.log`（见 `client-patchs/AGENTS.md` §1.3 与各插件自己的 README）。
`status.json` 里看 `ready` / `enabled` / 命中的 `ruleId` / `rejectReason` 这几项就够了。

> 三处的分工：**MOD 页说"装没装、编没编"；服务端日志说"这次启动加载了什么"；
> 客户端日志说"插件进游戏后干了什么"。**

### 4.5.5 最小规则脚本模板（可直接照抄）

只带规则、不带 Go 的 mod，`mod.json` 最小形态：

```json
{
  "schema": 2,
  "id": "myserver.rules",
  "version": "1.0.0",
  "name": "我的规则",
  "permissions": ["server.script"],
  "layers": { "server": { "scripts": ["server/rules/my-rule.lua"] } }
}
```

`server/rules/my-rule.lua` 骨架（**文件名别用 `level.lua` / `newchar.lua` / `quest.lua`**，见 §4.5.4）：

```lua
-- 文件名就是幂等键的一部分：reward:<事件>:<本文件名>:<判别值>
-- 所以同一个文件里注册多条规则时，靠"行号/判别值"区分，改文件名等于换键（会重发一次）。

on("character_create", function(ctx)
  -- ctx 恒有：type / level / quest_id / character_id / account_id / name
  -- 仅 character_create 额外有：profession（基础职业号）/ advancement（转职号）
  -- 注意：基础职业 0（鬼剑士）是合法值，所以判"拿不到职业"必须用 nil，不能拿 0 当哨兵。
  local prof = tonumber(ctx and ctx.profession)
  if prof == nil then
    return                     -- 拿不到职业：本段跳过（level_up / quest_complete 永远拿不到）
  end
  local adv = tonumber(ctx and ctx.advancement) or 0

  -- ① 金币：id 0 = 角色金币（一定成功，适合做"装上了没有"的哨兵）
  grant_item(0, 100000)

  -- ② 堆叠物：按堆叠发
  grant_item(10000000, 10)

  -- ③ 装备：grant_item 也能发（服务端取 PVF 定义并实例化）；附件路径同理
  grant_item(100261128, 1)
  send_mail("Starter Kit - 1/1", "welcome", { { id = 100261128, count = 1 } })

  -- ④ 角色待遇（存档字段，不是物品）：只升不降 / 累加
  unlock_equip_slots(59)       -- 五个扩展位全开
  expand_bag(2)                -- 背包扩容档位
  grant_revive_coin(100)       -- 复活币

  -- ⑤ 账号级（各走自己的事务与幂等键）
  expand_vault(12, 320)        -- 账号金库
end)
```

**报错一定带模板号**（`模板 <N> …`），所以池子里哪个号错了看服务端日志一眼就能定位。
一个号不合格时 `grant_item` 那**一整批**都不会发（连金币），把哨兵金币和风险项分两次调用更稳。

---

## 10. 在游戏里怎么验证（分层清单）

### 10.1 client 层

装完 → `<客户端根>\<target>` 出现该文件；卸完 → 消失（原本不存在）或逐字节还原（原本存在）。
`modkit status --client <客户端根>` 应显示 `[一致]`。

### 10.2 server 层

1. `modkit install` 成功 → 服务端模块 `mods/<mod-id>/` 有你的源码，`mods/zz_mods_gen.go` 里
   import 了你并调用 `Register()`；
2. **重新编译服务端**（启动器的"编译服务端"，或 `server/Build-Server.ps1`）；
3. 启动服务端 → 对照上面的 §9 ② 逐行核对；
4. 想验 `console.command`：启动前设 `DFO_SERVERMOD_CONSOLE="<mod-id> status"`，
   服务端起来后会执行一次并把结果写进日志；
   `DFO_SERVERMOD_CONSOLE=help` 会列出所有 mod 声明的命令。

### 10.3 pvf 层

`modkit status` 显示 `[一致]`；`Script.pvf` / `sk.dat` 的 sha256 与你清单里写的一致。
要确认游戏可用：进游戏后内容仍正常（PVF 语义合并的效果由内容本身决定）。

### 10.4 resource 层

- `modkit status` → `[一致]`；
- 归档大小会变（载荷长度变了），这是正常的；
- 进游戏看目标条目是否生效（图像条目 → 视觉可见）。

---

## 11. 常见报错与处置

| 现象 | 原因 | 处置 |
| --- | --- | --- |
| `声明了 client 层但缺 client/ 目录` | 包里没有该层目录 | 补齐目录（哪怕只放一个文件） |
| `client 层动作 #N：…size/sha256 与清单声明不符` | 只是**警告**（不阻断） | 要么按包内实际内容落位（无视它），要么重算 `size`/`sha256` 填对 |
| `清单 permissions 缺少 ...` | 漏权限位 | 按 §3 补齐 |
| `未知钩子点 "xxx"` | 自创钩子名 | 用 `modkit layers` 里的名字 |
| `整文件哈希与清单声明不同：现场 … ≠ 声明 …` | 只是**警告**（不再阻断） | 补丁能不能打由逐处 `before` 决定；要么用现场 EXE 重算 `patches` 的 `before/after` |
| `补丁 #N 现场字节不匹配` | 逐字节 `before` 校验没过（**这才是真正的门**） | 你的 `DFO.exe` 与清单要求的不是同一份：重算 `patches` 的 `before/after` |
| `目标已存在且内容不同（file.add…）` | 想新增但目标已存在 | 改用 `file.replace` |
| `请先退出游戏` | `DFO.exe` 在运行 | 退游戏再装/卸 |
| `找不到 pwsh 7` | pvf 层需要 pwsh | 装 PowerShell 7，或 `--pwsh` 指定，或放 `<客户端工作区>\tools\pwsh7\pwsh.exe` |
| `服务端 mod 目录已存在且不属于本 mod` | 有同名目录但无来源标记 | 人工确认后移走；引擎不覆盖不认识的目录 |
| `server 层静态验证编译失败` | 你的 Go 代码编不过 | 按报错改；安装已自动回滚 |
| `本 mod 声明了 server 层的 Go 钩子…但找不到 Go 工具链` | 声明 `hooks` 但本机没 Go | 装 Go，或改用只带 `scripts` 的规则脚本 mod（不需要 Go） |
| `依赖 mod X 当前被禁用：请先在 MOD 列表里启用它` | 依赖装了但被 `enabled.json` 禁用 | 先在 MOD 列表启用依赖，再装 |
| `计划被阻断`（exit 2） | 冲突/缺依赖/缺前置 | 看 `plan` 输出的 `冲突：` 行 |

---

## 12. 发布前自检

```powershell
modkit verify  --mod MyMod.zip                 # 结构 + 包内哈希 + 权限 + 钩子名
modkit plan    --client <客户端> --mod MyMod.zip --root <启动器根>   # 应 0 阻断
# 装到干净客户端验证一遍，再 uninstall，确认现场逐字节回到原状
modkit status  --client <客户端>               # 装完应全 [一致]
```

写 mod 的三条纪律：

1. **不动 PVF 之外的内容真源** —— 内容定义在 PVF；mod 不要往 Go 里塞平行内容表；
2. **不改写协议报文** —— `protocol.response` 是只读观察点；
3. **自检失败就报错** —— 宁可服务端起不来，也不要静默半残。
