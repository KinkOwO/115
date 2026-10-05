# mod 开发文档（四层架构）

> 面向**要写 mod 的人**。读这篇就够了；接口的权威清单用 `modkit layers` 打印。
> 架构与实现细节见启动器仓 `docs/modkit.md`。

- 版本：schema 2（四层）
- 引擎：`modkit`（启动器仓 `cmd/modkit`，引擎 `internal/modkit`）
- 服务端宿主：`server/work/dfo-lan/internal/servermod`
- **mod 的唯一家**：服务端模块根的 `mods/`（本机 `server/work/dfo-lan/mods/`）

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
3. 每条 `source` 文件存在，`size` + `sha256` 与实测一致；
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
  "author": "你的名字",
  "description": "一句话说清它改什么",
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

---

## 3. 权限位

| 权限 | 何时必须写 |
| --- | --- |
| `client.file.write` | client 层有 `file.add` / `file.replace` |
| `client.exe.patch` | client 层有 `exe.patch` |
| `exec.script` | pvf 层（要跑 .ps1） |
| `server.hook` | 声明了 server 层 |
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
`modkit install` 落位后还会先跑一次 `go build ./mods/...`，编不过就整体回滚。

### 4.2 三个钩子点

| 钩子 | 什么时候被调用 | 你能用它做什么 |
| --- | --- | --- |
| `server.boot` | 配置与存储就绪、**还没开始监听** | 自检、读配置、记日志、声明内容扩展意图 |
| `console.command` | 启动期一次性命令（`DFO_SERVERMOD_CONSOLE`）或 UI 触发 | 现场动作、打印状态 |
| `reward.script` | 启动装配奖励管线时 | 向奖励管线登记一份事件奖励规则（Lua 脚本）。**这是 mod "发东西"的正路**：复用既有的幂等发放 / 事务 / 邮件语义，不要自己造第二条发放路径 |
| `protocol.response` | 一条 S2C 报文写客户端之前 | **只读观察**（记录/统计），不能改写报文 |

钩子名只能在白名单里；自己发明名字会被 `verify` 拒绝。

### 4.3 五个宿主机操作

| 操作 | 说明 |
| --- | --- |
| `log` | 写服务端日志，自动带 `[mod <id>]` 前缀 |
| `config.read` | 读只读快照（`DFO_*` 环境变量 + main 注入的键） |
| `config.write` | 写**进程内**值，键必须已存在；刻意不落盘 |
| `content.register` | 声明内容扩展**意图**（第一期只登记，等 PVF 扩展点） |
| `console.reply` | 往控制台回一行文本 |

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
| `grant_item(id, count)` | 直接进背包（可堆叠物；`id 0` = 角色金币） |
| `send_mail(subject, body[, attachments])` | 系统邮件；`attachments = { {id=模板, count=数量}, ... }`（≤11 件）。**装备走这条**——附件路径会按装备目录的 reward 规则重建实例 |
| `grant_cera(amount)` | 账号级点券 |

`ctx` 字段：`type` / `level` / `quest_id` / `character_id` / `account_id` / `name`。

**装备必须走 `send_mail` 附件**，不要用 `grant_item`：后者是背包堆叠路径，
装备需要实例化（耐久/属性）。

### 4.5.3 完整示例

`examples/giveaway-random-equipment/` —— 新角色创建时发金币 + 系统邮件
（示例刻意用"必定成功、不依赖内容模板"的发放，便于验证 mod 是否真的注册生效）。
它演示了嵌入 Lua 规则、发邮件带装备附件、以及被管理器勾选/取消勾选。

**规则脚本的失败是可观测的**：模板号填错时奖励管线会记一条 grant/mail 失败日志，
角色创建本身不受影响（奖励是创建之后的可选步骤）。所以池子可以先粗后细。

### 4.5.4 落盘补充口（不重编译加规则）

`<服务端模块>/mods/scripts/*.lua` 会被一并加载，同名时**磁盘优先**。
适合运营侧快速试一条规则。注意它只影响奖励规则，不影响钩子注册。

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


## 5. client 层

三种动作，语义与旧版一致：

- `file.add`：目标**必须不存在**。已存在且同内容 → 幂等跳过；已存在且不同 → 拒绝（要覆盖请用 `file.replace`）。
- `file.replace`：目标**必须存在**；自动备份原文件，卸载时逐字节还原。
- `exe.patch`：**整文件哈希门**——现场 `DFO.exe` 的 sha256 必须等于 `sourceSHA256`，否则整步拒绝；
  然后逐字节核对每处 `before` 再写 `after`。这是设计行为，不是缺陷。

**注意**：client 层会让 modkit 要求游戏退出（装/卸都要求 `DFO.exe` 不在运行）。

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

`size`/`sha256` 用 `modkit verify` 反查：先随便填，`verify` 会报出实测值，填回去即可。
（或 `Get-FileHash <file> -Algorithm SHA256`。）

**验证建议**：先拿"改动可见但风险低"的条目试（加载图 / 光效 / 无关紧要的图标），
不要一上来就换 `interface/windowcommon.img` 这类核心 UI。

---

## 8. 装/卸/回滚语义（必须知道）

- **顺序**：server → pvf → client → resource。任何一步失败 → 逆序回滚本次已落地部分。
- **先备份 + 先写注册表日志，再动现场**：中断也能还原。
- **卸载逐字节核对**：现场必须是"我们装的状态"或"本来就是原状"，否则**保留现场并拒绝**
  （`--force` 才强行用备份还原）。
- **同目标冲突**：两个 mod 声明同一目标 → 后者被阻断，绝不静默覆盖。
- **依赖**：`requires` 里的 mod 必须先装；被依赖的 mod 不允许卸载。
- **幂等**：重复安装同一包是安全的（已就绪的步骤会跳过）。
- **审计**：`<client>\.launcher-mods\modkit\audit.jsonl` 全程可归因到 mod id。

---

## 9. 在游戏里怎么验证（清单）

### 9.1 client 层

装完 → `<客户端根>\<target>` 出现该文件；卸完 → 消失（原本不存在）或逐字节还原（原本存在）。
`modkit status --client <客户端根>` 应显示 `[一致]`。

### 9.2 server 层

1. `modkit install` 成功 → 服务端模块 `mods/<mod-id>/` 有你的源码，`mods/zz_mods_gen.go` 里
   import 了你并调用 `Register()`；
2. **重新编译服务端**（启动器的"编译服务端"，或 `server/Build-Server.ps1`）；
3. 启动服务端 → 日志里应出现：
   - `servermod: 已装载 N 个服务端 mod：<你的 id>；boot 钩子 1 个`（`servermod.Description()`）
   - `[mod <你的 id>] 启动自检通过…`（你 `boot` 里写的日志）
4. 想验 `console.command`：启动前设 `DFO_SERVERMOD_CONSOLE="<mod-id> status"`，
   服务端起来后会执行一次并把结果写进日志；
   `DFO_SERVERMOD_CONSOLE=help` 会列出所有 mod 声明的命令。

### 9.3 pvf 层

`modkit status` 显示 `[一致]`；`Script.pvf` / `sk.dat` 的 sha256 与你清单里写的一致。
要确认游戏可用：进游戏后内容仍正常（PVF 语义合并的效果由内容本身决定）。

### 9.4 resource 层

- `modkit status` → `[一致]`；
- 归档大小会变（载荷长度变了），这是正常的；
- 进游戏看目标条目是否生效（图像条目 → 视觉可见）。

---

## 10. 常见报错与处置

| 现象 | 原因 | 处置 |
| --- | --- | --- |
| `声明了 client 层但缺 client/ 目录` | 包里没有该层目录 | 补齐目录（哪怕只放一个文件） |
| `缺少 size/sha256 双重校验` | 清单没写 | 用 `Get-FileHash` 与文件大小填上 |
| `清单 permissions 缺少 ...` | 漏权限位 | 按 §3 补齐 |
| `未知钩子点 "xxx"` | 自创钩子名 | 用 `modkit layers` 里的名字 |
| `整文件哈希门不过` | 你的 `DFO.exe` 与清单要求的不是同一份 | 用现场 EXE 的 sha256 重算 `patches` 的 `before/after` |
| `目标已存在且内容不同（file.add…）` | 想新增但目标已存在 | 改用 `file.replace` |
| `请先退出游戏` | `DFO.exe` 在运行 | 退游戏再装/卸 |
| `找不到 pwsh 7` | pvf 层需要 pwsh | 装 PowerShell 7，或 `--pwsh` 指定，或放 `<客户端工作区>\tools\pwsh7\pwsh.exe` |
| `服务端 mod 目录已存在且不属于本 mod` | 有同名目录但无来源标记 | 人工确认后移走；引擎不覆盖不认识的目录 |
| `server 层静态验证编译失败` | 你的 Go 代码编不过 | 按报错改；安装已自动回滚 |
| `计划被阻断`（exit 2） | 冲突/缺依赖/缺前置 | 看 `plan` 输出的 `冲突：` 行 |

---

## 11. 发布前自检

```powershell
modkit verify  --mod MyMod.zip                 # 结构 + 哈希 + 权限 + 钩子名
modkit plan    --client <客户端> --mod MyMod.zip --root <启动器根>   # 应 0 阻断
# 装到干净客户端验证一遍，再 uninstall，确认现场逐字节回到原状
modkit status  --client <客户端>               # 装完应全 [一致]
```

写 mod 的三条纪律：

1. **不动 PVF 之外的内容真源** —— 内容定义在 PVF；mod 不要往 Go 里塞平行内容表；
2. **不改写协议报文** —— `protocol.response` 是只读观察点；
3. **自检失败就报错** —— 宁可服务端起不来，也不要静默半残。
