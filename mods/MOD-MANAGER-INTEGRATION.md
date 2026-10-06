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
   │             ├─ prepareRuntime（PVF/存储）
   │             ├─ ★ servermod.LoadEnabledList(mods/)   ← 读勾选结果
   │             ├─ ★ mods.RegisterMods()                ← 只让启用的 mod 登记钩子
   │             ├─ ★ servermod.Boot()                   ← 自检；失败=拒绝启动
   │             └─ 开始监听
   │
   └─ 客户端拉起
```

**关键判断**：服务端 mod 是**编译进二进制**的，但**启用/禁用是运行期门禁**。
所以管理器的勾选**不需要重编译**——只要改一个文件、重启服务端即可。
（装/卸 mod 才需要重编译，那是 modkit 的职责。）

---

## 2. 三份状态，各管什么

管理器要读写的是这三处，**不要自己发明第四处**：

| 文件 | 归属 | 语义 | 谁写 |
| --- | --- | --- | --- |
| `<服务端模块>/mods/zz_mods_gen.go` | **装没装** | 生成的 import 清单：列进来的 mod 才会被编译进二进制 | 只有 `modkit install/uninstall` |
| `<服务端模块>/mods/<mod-id>/` | **装了哪些** | 每个已装服务端 mod 的 Go 源码（含 `.modkit-owner` 来源标记） | 只有 `modkit install/uninstall` |
| `<服务端模块>/mods/enabled.json` | **开不开** | **禁用名单**：列在这里的 mod 不登记钩子 | 只有管理器（或 `modkit mods`） |
| `<客户端>/.launcher-mods/modkit/registry.json` | 客户端层 mod 的安装凭据 | 逐条还原依据 | 只有 `modkit` |

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
| **作者 / 说明** | `mod.json` 的 `author` / `description`（**可选**）。服务端层从落位目录的 `mods/<id>/mod.json` 读（`mods list --json` 已带出）；客户端层从注册表快照读（`modkit status`）。两者都为空 = 作者没填，**不要显示成"未知"以外的假数据** |
| **涉及哪几层** | 注册表 `layers`（`server` / `pvf` / `client` / `resource`） |
| 声明的权限 | `mod.json` 的 `permissions`（展示给玩家看它要动什么） |
| 依赖 | `mod.json` 的 `requires`（未满足时应标灰并说明） |
| **是否启用** | `enabled.json` 的 `disabled` 里有没有它 |
| 是否需要重编译 | 有 `server` 层 → 装/卸要重新编译；**改启用状态不用** |
| 服务端钩子 | `mod.json` 的 `layers.server.hooks`（它会在什么时机被调用） |
| 是否已实际生效 | 服务端启动日志里的 `servermod: 已装载 …`（见 §6） |

**诚实的提示语**（照抄，别美化）：

- 含 `server` 层的 mod：**"改动启用状态后需重启服务端；首次安装/卸载需重新编译服务端"**；
- 含 `pvf` 层：**"需要 PowerShell 7；缺失时安装会明确失败"**；
- 含 `exe.patch`：**"修改 DFO.exe 字节；整文件哈希门不过会拒绝安装"**。

---

## 4. 程序接口（管理器调这些）

### 4.1 装 / 卸（已有）

```powershell
modkit install   --client <客户端> --mod <mod.zip> --root <启动器根> [--dry-run]
modkit uninstall --client <客户端> --id <mod-id> --root <启动器根> [--dry-run] [--force]
modkit plan      --client <客户端> --mod <mod.zip> --root <启动器根>   # 退出码 2 = 会阻断
modkit verify    --mod <mod.zip>
modkit status    --client <客户端>
modkit layers                                        # 四层接口/钩子点/宿主机操作（权威清单）
```

### 4.2 勾选 / 取消勾选（管理器必须用这个）

```powershell
modkit mods list                          --root <启动器根> [--json]
modkit mods enable  --id <mod-id>         --root <启动器根> [--by launcher-ui]
modkit mods disable --id <mod-id>         --root <启动器根> [--by launcher-ui]
```

- 退出码：`0` 成功；`1` 参数/IO 错误；`2` 目标 mod 不存在或依赖不满足；
- `mods list --json` 输出每个已装 mod 的
  `id/name/version/author/description/layers/enabled/hooks/permissions/requires/dir/needsRebuildOnChange/drift`
  （`hooks`/`permissions`/`requires`/`author`/`description`/`drift` 为空时省略），
  管理器直接渲染成勾选列表即可，**不必自己解析 enabled.json**；
  顶层另有 `moduleDir`/`modsDir`/`enabledFile`/`count`/`note`；
- **`drift` 非空 = 该 mod 目录里没有 `mod.json`**：此时名称退化成 id，
  作者/说明/层/权限/依赖都不可知。管理器要把 `drift` 原文显示出来，
  而不是把空字段当成"作者没写"；
- `author` / `description` 来自该 mod 落位目录里的 `mod.json`（`modkit install` 落的原样副本），
  两个字段都是 `omitempty`：作者没填就是**键不出现**，管理器按"未填写"显示即可，
  不要回退去猜包名或作者名。

> 这三个子命令是给管理器用的稳定接口。管理器**不要**直接改 `enabled.json`
> （绕过依赖检查、也容易写坏 JSON）。

### 4.4 Lua 规则脚本（管理器要能管这个目录）

服务端启动时会读 `<服务端模块>/mods/scripts/*.lua` 当奖励规则（见 `MOD-DEVELOPMENT.md`
§4.5.4），这是**不重新编译**就能加一条规则的正式入口。管理器应当能管这个目录，
但必须守住下面四条：

| 规则 | 为什么 |
| --- | --- |
| 文件名必须是**平铺 `*.lua`**（不许带目录、不许上跳） | 服务端是按平铺 `*.lua` 枚举脚本的，带目录的名字它读不到 —— 那种"写了却没生效"最难查 |
| **由 mod 安装落位的脚本只读**（改/删都拒绝，提示去卸载那个 mod） | 它是那个 mod 的内容；手工改会让注册表里的还原凭据（哈希）对不上 |
| 保存时要带上"打开时那份内容的哈希" | 两个人同时编辑时，后点保存的不该悄悄盖掉前一个 |
| 每次都提示「改完要**重启服务端**才生效」 | 脚本是启动时一次性读进 Lua state 的，无法热摘；**但不需要重新编译**（与 `server.hook` 不同） |

事实来源（不要自己发明）：

- **这个目录里有什么** = `<服务端模块>/mods/scripts/*.lua`（平铺，忽略子目录与点开头文件）；
- **是谁落的位** = modkit 注册表（`<客户端>\.launcher-mods\modkit\registry.json`）里
  `kind = "server.script"` 的条目（`target` = `mods/scripts/<名>`）；读不到注册表就**不猜**，
  归属显示为空并给出原因；
- **会不会盖住某个 mod** = 各已装 mod 落位目录里的 `mod.json` 的 `server.scripts`
  （同名时服务端**磁盘优先**，页面要点名是哪个 mod 被盖住）。

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

- `requires` 里的 mod **必须先装**；被依赖的 mod **不允许卸载**（modkit 会拒绝）；
- modkit 目前**不做加载顺序拓扑**：所有启用的 mod 都按 `Register()` 的稳定顺序注册，
  它们之间不应互相 import（同名包 `modpkg`，Go 里无法区分）；
- 需要在 mod 之间协作时，走 `servermod` 的钩子与宿主机操作，不要直接耦合。

---

## 6. 服务端侧契约（管理器据此显示"是否真的生效"）

服务端启动时会打这些日志，管理器可以只读地抓取/展示：

```
servermod: 已装载 2 个服务端 mod：a.mod, b.mod；boot 钩子 1 个；console 钩子 1 个；奖励规则脚本 1 份
servermod: 已禁用 1 个 mod（giveaway.random-equipment）
servermod: 启用清单：没有 enabled.json（按全部启用处理）
servermod: mod 提供的奖励规则脚本 1 份：giveaway.random-equipment:giveaway-random-equipment.lua
[mod a.mod] 启动自检通过；服务端版本=dev 频道数=0
```

**注意**：被禁用的 mod **不会出现在"已装载"里**（它 Register() 时就返回了）。
如果管理器要显示"已安装但已禁用"，那份信息从 `mods/` 目录 + `enabled.json` 取，
不要从服务端日志推。

`boot` 钩子返回 error → **服务端拒绝启动**（fail-closed，见 `mods/MOD-DEVELOPMENT.md` §4）。
管理器应当把启动失败的原样日志展示给玩家，不要吞掉。

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
6. **服务端 mod 的启用状态改动需要重启服务端**：奖励规则脚本在启动时一次性
   加载进 Lua state，无法热摘。管理器要么提示重启，要么帮玩家重启；
7. **客户端插件自己的开关不归管理器管**：客户端 DLL mod 的 `.115us-mods\*.ini`
   （如中文输入的 `fix_cancel` / `ime_bridge`）是**插件首次运行时自己生成**、由玩家手改的；
   管理器不要读改写它、也不要把它当成 mod 的组成部分 —— 它不在包里，
   `uninstall` 也不会删它（卸载后残留需手工清理）。管理器若要展示，只读、只提示路径；
8. **客户端 DLL 插件有宿主依赖**：除了宿主本身，其它客户端 DLL mod 都声明
   `"requires": ["qol.client-host"]`。管理器禁用/卸载**宿主**前必须先处理依赖它的插件
   （`modkit` 会拒绝：`依赖未安装` / 被依赖），否则插件留在 `.115us-mods\` 里不会生效，
   玩家会以为"勾了却没反应"。

---

## 8. 建议的管理器交互流程

```
打开管理器
  └─ modkit mods list --json          → 渲染列表（勾选态来自 enabled）
       ├─ 勾选变化 → modkit mods enable/disable --id X --by launcher-ui
       │              └─ 若含 server 层：提示「重启服务端后生效」
       ├─ 「安装」按钮 → 文件选择 → modkit verify → modkit plan
       │              ├─ exit 2 → 显示 plan 里的「冲突：」行，不放行
       │              └─ exit 0 → modkit install
       ├─ 「卸载」按钮 → modkit uninstall --dry-run → 确认 → 真卸
       └─ 「详情」    → 读 mod.json（层/权限/依赖/钩子）
```

状态展示的唯一事实来源：

- **装没装** → `mods/` 目录（服务端层）+ 注册表（客户端层）；
- **开不开** → `enabled.json`；
- **是否生效** → 服务端启动日志。

---

## 9. 未闭环（写明，不假装）

1. `modkit mods list/enable/disable` **已实现**：`list` 支持 `--json`，`enable/disable` 需 `--id`
   （可带 `--by` 记录调用者），`--root` 可省略或直接 `cd` 到服务端模块根；退出码 0 / 1 / 2 与 §4.2 一致。
   2026-10-06 实测 `modkit mods list --root <115 仓>` 会按 §3 的字段表打出
   **名称 / 作者 / 说明**三行（`author` / `description` 取自各 mod 的 `mod.json`）；
2. ~~管理器 UI 本身未做~~ —— **2026-10-06 已交付**：启动器「MOD 工具」页 = mod 库管理器
   （列表 / 分页 / 导入导出 zip / 删除 / 批量安装卸载，实现在启动器仓 `internal/modlib`），
   同页下半部分是 **Lua 规则脚本**面板（§4.4）。**仍未接进页面**的是运行期启用/停用
   （`enabled.json`）：业主 2026-10-06 定的界面是"安装状态只做展示 + 批量装与卸"，
   开关仍只有 `modkit mods enable/disable` 与手工编辑那份文件两个入口；
3. mod 的**加载顺序**没有显式依赖排序（只有 `requires` 存在性检查）；
4. 启用/禁用**不能热生效**（需重启服务端）——这是奖励脚本一次性加载带来的边界，
   要热摘需要把 Lua state 做成可重建的，属独立工作；
5. **落盘规则脚本没有"停用"开关**：服务端无条件加载 `mods/scripts/*.lua`，`enabled.json`
   管不到它（那只管编译进去的 mod 的 `Register()`）。管理页给的是**删除**，不做
   "重命名成 `.lua.off` 式停用"——那会发明第二套启用状态，与 §2 的"三份状态"口径冲突；
6. 规则脚本的**编辑是纯文本**：不校验 Lua 语法（语法错只在服务端启动时以
   `reward rules disabled` 出现），也不做版本历史与回滚。
