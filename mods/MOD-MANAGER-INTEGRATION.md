# 启动段 mod 管理器接入文档

> 面向**要把 mod 管理器接进启动器的人**。
> 它讲清：管理器在启动链里插在哪、要读写哪些文件、勾选/禁用怎么落地、以及哪些边界不能碰。
> mod 作者看 `mods/MOD-DEVELOPMENT.md`；架构背景看启动器仓 `docs/modkit.md`。

- 状态：接口已就绪（服务端侧已实现并可测试）；**管理器 UI 已接进启动器「MOD 工具」页**
  （2026-10-06：mod 库列表 / 批量装与卸，以及下半部分的 **Lua 规则脚本**面板）
- 日期：2026-10-06
- 涉及三方：
  - **启动器**（`cmd/launcher` + UI）——管理器的宿主
  - **modkit**（启动器仓 `internal/modkit` + `cmd/modkit`）——装/卸引擎
  - **服务端**（`server/work/dfo-lan/internal/servermod`）——运行期钩子宿主与启用门禁

---

## 1. 管理器在启动链里的位置

```
玩家点「启动游戏」
   │
   ├─ [0] 启动前：读 mod 状态（可勾选/取消）        ← 管理器在这里就把 enabled.json 定好
   │
   ├─ [1] startgate     端口/残留进程检查
   ├─ [2] serverbuild   就地 go build 服务端（已装 server 层 mod 会一起编进去）
   ├─ [3] storage-route 决定存储档
   ├─ [4] dfolauncher launch
   │        │
   │        └─ wireprobe.exe 启动
   │             ├─ loadConfig
   │             ├─ ★ servermod.LoadEnabledList(mods/)   ← 读勾选结果
   │             ├─ ★ mods.RegisterMods()                ← 只让启用的 mod 登记钩子/规则
   │             ├─ prepareRuntime（PVF/存储 + **构造奖励管线**，此时读一遍规则脚本）
   │             ├─ ★ servermod.Boot()                   ← 自检；失败=拒绝启动
   │             └─ 开始监听
   │
   └─ 客户端拉起
```

**顺序是硬的，不能改**：`LoadEnabledList → RegisterMods → prepareRuntime → Boot`
（`cmd/wireprobe/main.go:70-73,88`）。原因写在 `main.go:54-69` 的注释里：
**奖励管线是在 `prepareRuntime` 内部构造的，它构造时会读一遍 mod 提供的规则脚本。**
把注册放到 `prepareRuntime` 之后 ⇒ **规则脚本赶不上那次构造 = 静默不生效**，
而日志里 `servermod: 已装载…` 与 mod 自己的"已登记"都照打（2026-10-06 02:22 现场：
新角色创建后没收到 mod 的邮件）。

**关键判断**：服务端 mod 是**编译进二进制**的，但**启用/禁用是运行期门禁**。
所以管理器的勾选**不需要重编译**——只要改一个文件、重启服务端即可。
（装/卸 mod 才需要重编译，那是 modkit 的职责。）
**例外**：只带 `scripts`（Lua 规则脚本）的 mod **不参与编译**，装完也只要重启服务端。

---

## 2. 三份状态，各管什么

管理器要读写的是这三处，**不要自己发明第四处**：

| 文件 | 归属 | 语义 | 谁写 |
| --- | --- | --- | --- |
| `<服务端模块>/mods/zz_mods_gen.go` | **装没装** | 生成的 import 清单：列进来的 mod 才会被编译进二进制 | 只有 `modkit install/uninstall` |
| `<服务端模块>/mods/<mod-id>/` | **装了哪些** | 每个已装服务端 mod 的 Go 源码（含 `.modkit-owner` 来源标记；**只有声明了 `hooks` 的 mod 才有 `mod.json`**，只带 `scripts` 的不落它 —— `install2.go:472,524-529`） | 只有 `modkit install/uninstall` |
| `<服务端模块>/mods/enabled.json` | **开不开** | **禁用名单**：列在这里的 mod 不登记钩子 | 只有管理器（或 `modkit mods`） |
| `<启动器根>/mods/` | **规则脚本 + mod 库** | 平铺 `*.lua` 是服务端读盘的**第一顺位**；同时也是「MOD 工具」页显示的 mod 库根（`internal/modlib/store.go:60-63`） | 人 / modkit 落位 / 管理器 |
| `<客户端>/.launcher-mods/modkit/registry.json` | 客户端层 mod 的安装凭据 | 逐条还原依据（含 `server.script` 条目的登记哈希） | 只有 `modkit` |

### 2.1 `enabled.json` 的确切格式

```json
{
  "schema": 1,
  "disabled": ["giveaway.random-equipment"],
  "updatedAt": "2026-10-06T02:10:00Z",
  "updatedBy": "launcher-ui"
}
```

**为什么是"禁用名单"而不是"启用名单"**（这是刻意的设计，别改）：

- 新装的 mod **默认启用**（装了就是想用），不需要管理器补写一行；
- 文件缺失、损坏、或新增了 mod，都不会变成"莫名其妙全都不生效"；
- 反过来用启用名单，任何一次写坏都等于"把所有 mod 关掉"，玩家会以为 mod 丢了。

**读失败时的行为**（服务端侧已实现）：
文件不存在 → 全部启用；解析失败 → 全部启用 **并记一条日志**。
绝不把"清单坏了"当成"全都禁用"。

### 2.2 谁负责写

服务端侧只**读**；写只有两个入口：

- 管理器 UI（推荐）——调 `modkit mods enable/disable`（见 §4），不要自己拼 JSON；
- 运维手工编辑（应急）。

两者都走同一份文件，读-改-写 + 原子落盘，避免并发写坏。

---

## 3. 管理器要显示的字段

建议每一个已装 mod 显示这些（都能从现有文件推出来，不需要新接口）：

| 字段 | 来源 |
| --- | --- |
| id / 名称 / 版本 | 该 mod 的 `mod.json`（装在包里；注册表也存了 id/name/version） |
| **作者 / 说明** | `mod.json` 的 `author` / `description`（**可选**）。**真实来源是扫 `<整合包根>\mods\` 下各 mod 的 `mod.json`**（启动器仓 `internal/modlib/store.go:190-231`）。⚠️ 注册表里**没有**这两个字段（条目结构见 `internal/modkit/modkit.go:71-93`），`modkit mods list --json` 也**不输出**它们（`internal/modkit/modsadmin.go:46-67`）——所以"快照进注册表 / `mods list` 带出"这条旧传播链**不成立**。两个字段都为空 = 作者没填，**不要显示成"未知"以外的假数据** |
| **涉及哪几层** | 注册表 `layers`（`server` / `pvf` / `client` / `resource`） |
| 声明的权限 | `mod.json` 的 `permissions`（展示给玩家看它要动什么） |
| 依赖 | `mod.json` 的 `requires`（未满足时应标灰并说明）。**硬门禁**：`plan` 只看装没装；`install` 还拒绝"依赖被禁用"（见 §5） |
| **是否启用** | `enabled.json` 的 `disabled` 里有没有它 |
| 是否需要重编译 | 有 **Go 钩子**（`layers.server.hooks`）→ 装/卸要重新编译；**只带 `scripts`（Lua 规则脚本）→ 不重编译，只要重启服务端**；**改启用状态不用** |
| 服务端钩子 | `mod.json` 的 `layers.server.hooks`（它会在什么时机被调用） |
| **是否已实际生效（「生效」列）** | **不是**从启动日志推的。判据 = 注册表（装着）+ **服务端二进制里有没有该 modID 字符串**（全文搜）+ **程序 mtime 晚于安装时间**（启动器仓 `internal/modlib/store.go:488-522`），取值 `生效` / `重启服务端后生效` / `未生效（需编译服务端）` / `—`（`store.go:162-175`）。启动日志只作为**补充证据**（见 §6） |

**诚实的提示语**（照抄，别美化）：

- 含 Go 钩子（`hooks`）的 mod：**"改动启用状态后需重启服务端；首次安装/卸载需重新编译服务端"**；
- **只含 `scripts`（Lua 规则脚本）的 mod**：**"不需要重新编译；重启服务端即生效"**
  （脚本不参与编译，服务端启动时读盘）；
- 含 `pvf` 层：**"需要 PowerShell 7；缺失时安装会明确失败"**；
- 含 `exe.patch`：**"修改 DFO.exe 字节；没有整文件哈希门——现场 sha256 与清单声明不符只警告、不阻断；能不能打由逐处 `before` 字节比对决定"**；
- 含 Go 钩子但本机没有 Go 工具链：**"安装会在落位前硬失败（找不到 Go 工具链）"**；
- 预编译服务端程序**可能没有 mod 宿主**，但要分成两件事说（**安装期门禁** vs **日志判据**）：
  - **安装期（启动器 v1.7.6 起，硬门禁；`dbb9970` 实现、`2000645` 随 1.7.6 发布）**：
    `modkit install` 在**写盘之前**探测目标服务端，
    判据 = `<模块根>/cmd/wireprobe/servermods.go` / `internal/servermod/` / `mods/zz_mods_gen.go`
    三者任一存在即算"源码树有宿主"。
    - **源码树也没有宿主 ⇒ 直接拒绝安装 server 层 mod**（报「计划被阻断：目标服务端的源码树里没有
      mod 宿主…」），并给出两条出路：① 用启动器「更新」拿带宿主的新服务端包；
      ② 有 Go 工具链时点「编译服务端」重编当前源码。**一个字节都不写**（连 modkit 状态目录都不建）；
    - **源码有宿主、已编译产物里搜不到宿主标记** ⇒ **只提示**「装完要重新编译服务端」，**不阻断**。
    （启动器仓 `internal/modkit/modhost.go:233-263` 的 `hostGate`，接在 `install2.go:108`
    写盘之前的宿主门禁位置。管理器应当把这段输出原样展示，不要自己再判一次。）
  - **日志判据（运行期）**：这时装好的 mod 一个都不会装载，**启动日志里连 `servermod:` 都不出现** ——
    管理器要提示"换成按当前源码编译的服务端"。
  - **发布包时间线**：2026-10-07 起重打的包已带宿主（`tools/tools-server-bin.zip` 85 条目 /
    `tools/tools-server-src.zip` 1973 条目），但那批包在分支 `mr/packages-20261007`、**尚未合并上游**；
    在此之前发布的包没有宿主。

---

## 4. 程序接口（管理器调这些）

### 4.1 装 / 卸（已有）

```powershell
modkit install   --client <客户端> --mod <mod.zip> --root <启动器根> [--dry-run] [--pwsh <路径>] [--timeout <分钟>] [--skip-verify-build] [--allow-dir]
modkit uninstall --client <客户端> --id <mod-id> --root <启动器根> [--dry-run] [--force]
modkit plan      --client <客户端> --mod <mod.zip> --root <启动器根> [--allow-dir]   # 退出码 2 = 会阻断
modkit verify    --mod <mod.zip> [--allow-dir] [--json]
modkit status    --client <客户端>
modkit layers                                        # 四层接口/钩子点/宿主机操作（权威清单）
```

真实的 flag 清单（启动器仓 `cmd/modkit/run2.go:70-227`，`verify` **没有** `--client`）：
`verify` = `--mod` / `--allow-dir` / `--json`；`plan` = `--client` / `--mod` / `--root` / `--allow-dir`；
`install` = `--client` / `--mod` / `--root` / `--dry-run` / `--pwsh` / `--timeout` /
`--skip-verify-build` / `--allow-dir`；`uninstall` = `--client` / `--id` / `--root` / `--dry-run` / `--force`。

### 4.2 勾选 / 取消勾选（管理器必须用这个）

```powershell
modkit mods list                          --root <启动器根> [--json]
modkit mods enable  --id <mod-id>         --root <启动器根> [--by launcher-ui]
modkit mods disable --id <mod-id>         --root <启动器根> [--by launcher-ui]
```

- 退出码：`0` 成功；`1` 参数/IO 错误；`2` 目标 mod 不存在或依赖不满足；
- `mods list --json` 输出每个已装 mod 的
  `id/name/version/layers/enabled/hooks/permissions/requires/dir/needsRebuildOnChange/drift`
  （`hooks`/`permissions`/`requires`/`drift` 为空时省略；字段定义见 `internal/modkit/modsadmin.go:46-67`），
  管理器直接渲染成勾选列表即可，**不必自己解析 enabled.json**；
  顶层另有 `moduleDir`/`modsDir`/`enabledFile`/`count`/`note`；
  ⚠️ **`author` / `description` 不在这个输出里**（条目结构里没有这两个字段）——
  管理器要显示作者/说明，得另外扫 `<整合包根>\mods\<名字>\mod.json`（见 §3 的说明）；
- **`drift` 非空 = 该 mod 落位目录里没有 `mod.json`**：此时名称退化成 id，
  层/权限/钩子/依赖都不可知（`modsadmin.go:152-166`）。管理器要把 `drift` 原文显示出来，
  而不是把空字段当成"作者没写"；
- 落位目录里**可能有**也可能**没有** `mod.json`：只有声明了 `hooks` 的 mod 会落它
  （`internal/modkit/install2.go:472,524-529`），**只带 `scripts` 的 mod 不落** ——
  所以只带脚本的 mod 在这里就会报 `drift`，这是**正常现象**，不是安装坏了。

> 这三个子命令是给管理器用的稳定接口。管理器**不要**直接改 `enabled.json`
> （绕过依赖检查、也容易写坏 JSON）。

### 4.4 Lua 规则脚本（管理器要能管这个目录）

服务端启动时会读 **`<启动器根>\mods\*.lua`**（mod 库根，**平铺**）当奖励规则
（见 `MOD-DEVELOPMENT.md` §4.5.4），这是**不重新编译**就能加一条规则的正式入口。
管理器应当能管这个目录，但必须守住下面四条：

| 规则 | 为什么 |
| --- | --- |
| 文件名必须是**平铺 `*.lua`**（不许带目录、不许上跳） | 服务端是按平铺 `*.lua` 枚举脚本的，带目录的名字它读不到 —— 那种"写了却没生效"最难查 |
| **由 mod 安装落位的脚本只读**（改/删都拒绝，提示去那个 mod 的页面处理） | 它是那个 mod 的内容；手工改会让注册表里的登记哈希对不上（页面会显示"现场已被改动"）。**注意：卸载那个 mod 不会删这份脚本**，它会变成手工脚本（§9 第 5 条） |
| 保存时要带上"打开时那份内容的哈希" | 两个人同时编辑时，后点保存的不该悄悄盖掉前一个 |
| 每次都提示「改完要**重启服务端**才生效」 | 脚本是启动时一次性读进 Lua state 的，无法热摘；**但不需要重新编译**（与 `server.hook` 不同） |

事实来源（不要自己发明）：

- **这个目录里有什么** = `<启动器根>\mods\*.lua`（平铺，忽略子目录与点开头文件）；
- **是谁落的位** = modkit 注册表（`<客户端>\.launcher-mods\modkit\registry.json`）里
  `kind = "server.script"` 的条目（`target` = `mods/<名>`，**没有 `scripts/` 子目录**）；
  读不到注册表就**不猜**，归属显示为空并给出原因；
- **会不会盖住某个 mod** = 各已装 mod 落位目录里的 `mod.json` 的 `server.scripts`
  （同名时服务端**磁盘优先**，实现已落地：`reward_flow.go:128` 的
  `compositeScriptFS{first: foldScriptFS(onDisk), second: bundled}`；
  覆盖时日志会打 `reward scripts: 磁盘脚本 <名> 覆盖内置同名规则…`，见 `reward_flow.go:155`），
  页面要点名是哪个 mod 被盖住；

启动器里的落地（供其它宿主参考）：数据层 `internal/modlib/scripts.go`，
RPC 在 `cmd/launcher/modlib.go`：

```text
mods.scripts        无参                        → {dir, moduleDir, items[], note, restartHint}
mods.scriptRead     {name}                      → {name, body, sha256}
mods.scriptSave     {name, body, sha256}        → {name, path, created}（sha256 = 打开时那份的哈希）
mods.scriptDelete   {name}                      → {name}
mods.scriptImport   无参（弹文件选择框）        → {name, path, created} | {cancelled:true}
```

`items[]` 每项：`{name, sizeBytes, sizeText, modTime, sha256, owner, ownerNote, providedBy[], readOnly}`。
`owner` 非空即只读；`providedBy` 非空表示"这份脚本会盖住这些 mod 自带的同名规则"。

### 4.3 在游戏/服务端里核验

```powershell
# 服务端启动期一次性命令（结果进日志；服务端没有可交互 stdin）
$env:DFO_SERVERMOD_CONSOLE = "help"                        # 列出所有 mod 声明的命令
$env:DFO_SERVERMOD_CONSOLE = "<mod-id> status"             # 问某个 mod 自己
```

---

## 5. 依赖与顺序

- **`requires` 是硬门禁**，而且是**两段**判据（别只实现一段）：
  - **`plan` 阶段只看"装没装"**：缺了就在计划里给 `冲突：依赖未安装：X` 并 **exit 2**
    （启动器仓 `internal/modkit/layerplan.go:369-373`）；
  - **`install` 在写盘前再判一次**，并**额外拒绝"依赖装了但被 `enabled.json` 禁用"**
    ——那种情况同样不生效，报 `依赖 mod X 当前被禁用：请先在 MOD 列表里启用它`
    （启动器仓 `internal/modkit/install2.go:97-102,296-327`，用例 `requires_gate_test.go:101-106`）；
  - 被依赖的 mod **不允许卸载**（`internal/modkit/uninstall2.go:66-68`），禁用方向也拦
    （启动器仓 `internal/modkit/modsadmin.go:281-298`）。
- modkit **不做加载顺序拓扑**：所有启用的 mod 都按 `Register()` 的稳定顺序注册，
  它们之间不应互相 import（同名包 `modpkg`，Go 里无法区分）；
- ⚠️ **同时装 ≥2 个 server 层 mod：已支持**（启动器仓 commit `0bd67dc`，2026-10-07 03:03，
  **已在远端 `fork/master`**；版号仍是 1.7.7、exe 已重出 —— `version.json` 的
  `exe_size 64764416 → 64770048`）：
  - 生成器在 **≥2 个 mod** 时给**每条 import 一个显式别名**（`mod_<清洗后的 id>`，撞名追加 `_2`/`_3`），
    `RegisterMods()` 按别名逐条调用 ⇒ 不再有 `modpkg redeclared in this block`；
    **0 个 / 1 个 mod 的产物逐字节不变**（已装 1 个 mod 的机器上那份加载器不会被动到）；
  - 依据：`internal/modkit/support.go:196` 的 `serverModGoImportAlias()`、`:266-313` 的
    `renderServerModsGen()`；回归 `internal/modkit/support_gen_loader_test.go`（2/3 个 mod 逐字节 golden +
    临时模块里真跑 `go build ./mods/`，含旧的默认 import 形态必红的反向证据）。
  - **历史版本**：`0bd67dc` 之前那一版 1.7.7（commit `8c87358`）**只能装 1 个**，
    第二个会让 `mods/zz_mods_gen.go` 报 `modpkg redeclared in this block`，
    由安装期 `go build ./mods/` 拦下并回滚（`install2.go:633-645`）。按 `version.json`
    的 `exe_sha256` 可以区分手上那份 exe 是哪一版。
  - 与 `layers.server.scripts`（只带 Lua 规则脚本）无关：那条路径不进加载器，装几个都不冲突。
- 需要在 mod 之间协作时，走 `servermod` 的钩子与宿主机操作，不要直接耦合。

---

## 6. 服务端侧契约（启动日志是**补充**证据，不是「生效」列的判据）

> ⚠️ 先纠正一条口径：**「生效」列的判据不是启动日志**，是"注册表 + 服务端二进制全文搜 modID + mtime"
> （见 §3 的字段表与 §8）。启动日志回答的是另一个问题：**这次启动到底加载了什么**。

服务端启动时会打这些日志，管理器可以只读地抓取/展示：

```
servermod: 已装载 2 个服务端 mod：a.mod, b.mod；boot 钩子 1 个；console 钩子 1 个；奖励规则脚本 1 份
servermod: 已禁用 1 个 mod（giveaway.random-equipment）
servermod: 启用清单：没有 enabled.json（按全部启用处理）
servermod: mod 提供的奖励规则脚本 1 份：giveaway.random-equipment:giveaway-random-equipment.lua
reward rules enabled (embedded scripts + 1 mod script(s))
[mod a.mod] 启动自检通过；服务端版本=dev 频道数=0
```

- 格式串来自 `internal/servermod/host.go:461-488`（`Description()`）与 `cmd/wireprobe/main.go:94-105`；
- **一行 `servermod:` 都没有** ⇒ 这份服务端二进制**没有 mod 宿主**（预编译包可能就没有，
  现场记录见启动器仓 `internal/modlib/store.go:101-108`）。此时装好的 mod **一个都不会装载**，
  但它仍可能出现在「生效」列里 —— 这正是需要管理器额外提示的场景。
  ⚠️ 注意这**不是**安装期那道门禁（那道在 `install2.go:103-113`，见 §3 与 §7 第 5 条）：
  安装期看的是**源码树**，**源码有宿主只提示"装完要重新编译"、不阻断**；这里看的是**跑起来的二进制**。
- `reward rules enabled (embedded scripts + N mod script(s))` 是**构造顺序的见证**（`reward_flow.go:50-61`）：
  `+0` 而上面又打了"已装载 1 个 mod" ⇒ 规则**静默失效**（注册晚于管线构造，见 §1）。

**注意**：被禁用的 mod **不会出现在"已装载"里**（它 Register() 时就返回了）。
如果管理器要显示"已安装但已禁用"，那份信息从 `mods/` 目录 + `enabled.json` 取，
不要从服务端日志推。

`boot` 钩子返回 error → **服务端拒绝启动**（fail-closed，见 `mods/MOD-DEVELOPMENT.md` §4）。
管理器应当把启动失败的原样日志展示给玩家，不要吞掉。

> 完整的"三处自证"（MOD 页「生效」列 / 服务端日志 / 客户端插件日志 `*.log` 与 `*.status.json`）
> 写在 `mods/MOD-DEVELOPMENT.md` §9，管理器可以直接把那一节抄进帮助页。

---

## 7. 边界与硬约束（不要碰）

1. **不要手改 `zz_mods_gen.go`**：它是生成物，下次 install/uninstall 会覆盖。
   管理器若要显示"装了哪些"，读 `mods/` 目录或注册表；
2. **不要在管理器里做"启用=重编译"**：勾选改的是运行期门禁，重编译是装/卸的事；
   把两者混在一起会让玩家点一下等一次 `go build`；
3. **不要绕过 modkit 直接往 `mods/` 写源码**：那样 `enabled.json`、来源标记与
   生成的 import 清单会不同步，之后 `uninstall` 会认不出来；
4. **不要在游戏运行时改客户端文件**：`modkit` 已经有 `DFO.exe` 在跑就拒绝的硬门，
   管理器应当把这条错误原样显示；
5. **PVF 层需要 PowerShell 7**：管理器应在"装之前"提示，而不是等安装失败；
   **声明 Go 钩子（`hooks`）的 mod 需要本机 Go 工具链**：缺失时 `install` 在落位前就硬失败
   （启动器仓 `internal/modkit/install2.go:113-116`）；
   **目标服务端的源码树必须有 mod 宿主**：缺了会被**安装期硬门禁**拒绝并在写盘前中止
   （`internal/modkit/modhost.go:233-255`，接在 `install2.go:103-113`）——
   管理器要把报错里的两条出路原样展示（「更新」拿新包 / 「编译服务端」重编源码）；
   源码有宿主但产物里搜不到宿主标记时**只提示"装完要重新编译"，不阻断**（`modhost.go:256-262`）；
6. **服务端 mod 的启用状态改动需要重启服务端**：奖励规则脚本在启动时一次性
   加载进 Lua state，无法热摘。管理器要么提示重启，要么帮玩家重启；
7. **客户端插件自己的开关不归管理器管**：客户端 DLL mod 的 `.115us-mods\*.ini`
   （如中文输入的 `fix_cancel` / `ime_bridge`）是**插件首次运行时自己生成**、由玩家手改的；
   管理器不要读改写它、也不要把它当成 mod 的组成部分 —— 它不在包里，
   `uninstall` 也不会删它（卸载后残留需手工清理）。管理器若要展示，只读、只提示路径；
8. **客户端 DLL 插件有宿主依赖**：除了宿主本身，其它客户端 DLL mod 都声明
   `"requires": ["qol.client-host"]`。管理器禁用/卸载**宿主**前必须先处理依赖它的插件
   （`modkit` 会拒绝：`依赖未安装` / 被依赖），否则插件留在 `.115us-mods\` 里不会生效，
   玩家会以为"勾了却没反应"；
9. **落盘的 Lua 规则脚本"卸载不删"**（2026-10-06 业主定调）：卸载带 `server.script` 的 mod 时
   脚本留在 `<启动器根>\mods\` 变成手工脚本，只摘注册表条目
   （启动器仓 `internal/modkit/uninstall2.go:123-124,169-172`）。管理器**不要**把"卸载成功"
   显示成"规则已停止生效"——它下次启动服务端照样加载。要真停，得在 §4.4 的面板里删那份 `.lua`；
   管理器也不该替玩家自动删（那会让"卸载 mod 不影响脚本"这条承诺失效）。

---

## 8. 建议的管理器交互流程

```
打开管理器
  └─ modkit mods list --json          → 渲染列表（勾选态来自 enabled）
       ├─ 勾选变化 → modkit mods enable/disable --id X --by launcher-ui
       │              └─ 若含 Go 钩子：提示「重启服务端后生效」
       ├─ 「安装」按钮 → 文件选择 → modkit verify → modkit plan
       │              ├─ exit 2 → 显示 plan 里的「冲突：」行，不放行
       │              └─ exit 0 → modkit install
       ├─ 「卸载」按钮 → modkit uninstall --dry-run → 确认 → 真卸
       │              └─ 若含 server.script：提示「规则脚本已保留为手工脚本，仍在生效」
       └─ 「详情」    → 读 <整合包根>\mods\<名字>\mod.json（层/权限/依赖/钩子/作者）
```

状态展示的唯一事实来源：

- **装没装** → `mods/` 目录（服务端层）+ 注册表（客户端层）；
- **开不开** → `enabled.json`；
- **是否生效** → **注册表（装了）+ 服务端二进制里有没有该 modID + 程序 mtime**（§3）；
  启动日志是补充证据（§6），**不能**当作「生效」列的判据；
- **只带 `scripts` 的 mod** → `重启服务端后生效`（不参与编译，不受 mtime 判据管）。

---

## 9. 未闭环（写明，不假装）

1. `modkit mods list/enable/disable` **已实现**：`list` 支持 `--json`，`enable/disable` 需 `--id`
   （可带 `--by` 记录调用者），`--root` 可省略或直接 `cd` 到服务端模块根；退出码 0 / 1 / 2 与 §4.2 一致。
   ⚠️ **订正（2026-10-07 核对）**：`mods list` 的输出里**没有** `author` / `description`
   （`internal/modkit/modsadmin.go:46-67` 的条目结构没有这两个字段），
   旧文"会按 §3 的字段表打出名称/作者/说明三行"不成立；作者/说明由启动器的 mod 库扫描
   （`internal/modlib/store.go:190-231`）另取；
2. ~~管理器 UI 本身未做~~ —— **2026-10-06 已交付**：启动器「MOD 工具」页 = mod 库管理器
   （列表 / 分页 / 导入导出 zip / 删除 / 批量安装卸载，实现在启动器仓 `internal/modlib`），
   同页下半部分是 **Lua 规则脚本**面板（§4.4）。**仍未接进页面**的是运行期启用/停用
   （`enabled.json`）：业主 2026-10-06 定的界面是"安装状态只做展示 + 批量装与卸"，
   开关仍只有 `modkit mods enable/disable` 与手工编辑那份文件两个入口；
3. mod 的**加载顺序**没有显式依赖排序（只有 `requires` 存在性检查）；
   - **同时装 ≥2 个 server 层 mod：已支持**（启动器仓 commit `0bd67dc`，版号仍 1.7.7、
     已推到 `fork/master`）：≥2 个 mod 时生成器给每条 import 一个显式别名
     （`mod_<清洗后的 id>`），按别名调用各自的 `Register()`。
     `0bd67dc` 之前那一版 1.7.7（`8c87358`）只保证 1 个，第二个会被安装期拦下并回滚 ——
     见 §5 与 `MOD-DEVELOPMENT.md` §4.8；
4. 启用/禁用**不能热生效**（需重启服务端）——这是奖励脚本一次性加载带来的边界，
   要热摘需要把 Lua state 做成可重建的，属独立工作；
5. **落盘规则脚本没有"停用"开关**：服务端无条件加载 `<启动器根>\mods\*.lua`，`enabled.json`
   管不到它（那只管编译进去的 mod 的 `Register()`）。管理页给的是**删除**，不做
   "重命名成 `.lua.off` 式停用"——那会发明第二套启用状态，与 §2 的"三份状态"口径冲突。
   **卸载 mod 也不删这些脚本**（§7 第 9 条），所以"卸载后规则还在生效"是设计行为；
6. 规则脚本的**编辑是纯文本**：不校验 Lua 语法（语法错只在服务端启动时以
   `reward rules disabled` 出现），也不做版本历史与回滚。
