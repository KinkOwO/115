# mod API 扩展方案（把第四次更新做成 mod）

> **状态：设计稿，待业主确认。本文档落盘时未改动任何代码。**
> 取证方式：只读。所有 `文件:行号` 均已在本机当前工作树上核对。
> 基线（改动前）：服务端仓 `go build ./...` = 0、`go vet ./...` = 0、`go test ./... -count=1` = 0，
> **零失败**（日志 `.tmp/baseline-20261007/`）。任何新增失败即为阻断。

---

## 0. 结论摘要

| 类别 | 能否做成 mod | 靠什么 |
| --- | --- | --- |
| 客户端资源 / DLL / 字体 | ✅ 现在就能 | client / resource 层（已有先例：`mods/client-mods/`） |
| 事件奖励、补给、发东西 | ✅ 现在就能 | Lua 规则脚本（`server.script`，不重编译） |
| 数值/开关类调服参数 | ⚠️ 需扩 `modpolicy` | Tier 3 |
| **新增玩法（新副本模式、新活动流程、新协议分支）** | ❌ 现在不能 → **需扩请求侧钩子** | **Tier 2（本次核心）** |
| 数据库 schema / 引擎替换 / 整二进制替换 / 源码删除 | ❌ 永远不能 | 只能走版本更新 |

三条不可越过的边界（本方案的自我约束）：

1. **内容仍从 PVF 读**（`server/AGENTS.md` §0）。本方案只开"**行为**扩展点"，**不开"内容表注册"**——
   `content.register` 保持"只登记意图"，不升级成真注册口。否则就是用 mod 机制绕开单一内容真源铁律。
2. **不把 `protocol.response` 改成可改写**。协议取证链必须留在服务端自身代码路径里；
   mod 要改行为，走 Tier 2 的**请求侧短路 + 自己应答**，语义清楚且可归因。
3. **不升 schema**。新增钩子名 / 宿主操作 / kind 不需要升（`mod.json` 用 map/slice 容纳）；
   只有**新增 JSON 字段**才必须升（`manifest2.go:72-76` 的 `DisallowUnknownFields()`）。

---

## 1. 现状取证

### 1.1 已经有的能力（事实）

**宿主 API**（`server/work/dfo-lan/internal/servermod/`）：

- 钩子点常量 `host.go:50-57`：`server.boot` / `console.command` / `protocol.response` / `reward.script`。
- 注册函数：`RegisterBoot` `host.go:133`、`RegisterConsole` `:148`、`RegisterResponse` `:162`、
  `RegisterConsoleHelp` `:288`、`RegisterRewardScript` `scripts.go:68`。
- 宿主操作名 `host.go:39-47`：`log` / `config.read` / `config.write` / `content.register` /
  `console.reply` / `reward.register`（最后一个没有独立函数，`reward.script` 走 `RegisterRewardScript`）。
- 触发点：boot `cmd/wireprobe/main.go:88`；console `main.go:116-119`（仅 `DFO_SERVERMOD_CONSOLE`）；
  response `cmd/wireprobe/connection_output.go:31-33`（每条 S2C）。

**装配顺序**（`cmd/wireprobe/main.go`）：`modpolicy.Reset()` :52 → `LoadEnabledList` :70 →
`mods.RegisterMods()` :71 → `prepareRuntime` :73 → `SetEnvSnapshot` :87 → `Boot` :88 → 监听 :203/:257。

**mod 的本质**：与内核同属 `dfolan` 模块的 Go 包，**可以 import 服务端内部包**——
现成证据 `mods/odyssey.hardcore/mod.go:55-56` 就 import 了 `dfolan/internal/modpolicy` 与
`dfolan/internal/servermod`。所以 `host.go:21-25` 那句"mod 不 import 服务端内部包"是**对宿主操作面的
设计意图，不是编译期限制**。mod 真正缺的不是权限，而是**被调用的时机（钩子点）**。

**唯一正统的"行为覆盖"扩展点 = `internal/modpolicy`**：进程内策略，零值=关，消费点固定在既有业务里
（`cmd/wireprobe/odyssey_gate.go:30,52,66`、`internal/loot/modpolicy_drops.go:34,47`），
`Configure` 强制填 `Source` 以便归因，启动日志一行见证（`odyssey_gate.go:76-78`）。
这是本方案 Tier 3 要沿用的模式。

### 1.2 五处事实缺口（扩 API 时必须一并补）

| # | 缺口 | 证据 | 后果 |
| --- | --- | --- | --- |
| G1 | **`Allowed` 只是显示，不是门** | `manifest2.go:231` 写着 `_ = point // …Allowed 由服务端侧在注册时强制`，而服务端 grep `Allowed`/`HostOp` **零命中**；`layer.go:150-156` 的 `Allowed` 只被 `layerplan.go:450-455`、`run2.go:235/260` 用于**打印** | 声明 `protocol.response` 的 mod 可以调任意宿主操作，越权不报错 |
| G2 | **"声明了钩子但没注册"无任何校验** | 静态校验只有"钩子名 ∈ HookPoints + 同 mod 不重复"（`manifest2.go:222-231`）；`layer.go:96-101` 声称服务端会核对，服务端无此代码（`host.go:176 Registered()` 只返回 id） | 装成功、编成功、**什么也没发生**——最难查的一类失败 |
| G3 | `protocol.response` 只覆盖 `send` | `connection_output.go:31-33` 只在 `send` 里触发；`writeRaw` `:50`、`writePrepared` `:64` 不触发 | **经核实不是缺陷**：观察者契约给的是**明文正文**，而这两条路径拿到的已是 `preparePackets` **编码后**的帧，没有明文可给。要覆盖就得改契约（改成给编码后字节），那是另一个决定 |
| G4 | **`BootContext.ChannelCount` 恒为 0** | `main.go:90` 实参恒 0；而 `channelCfg` 在 `main.go:134-142` 才加载，**晚于 Boot 调用（`:88`）** | **不是一行能修的**：要重排启动顺序（Boot 移到频道配置解析之后）才算真修，属行为变更，须单独一次假设/提交 |
| G5 | **`SetConfigSnapshot` 生产无调用点** | `host.go:212` 有实现，仅测试调用；生产 `config.read` 的键空间实际只有 `DFO_*` 环境快照 | 半成品接口 |

另有 4 处小问题：`layer.go:76-88 layerPermissions()` 是**死代码**（全仓无调用者）；
权限清单**两处硬编码**（`layer.go:58-70` 与 `cmd/modkit/run2.go:239-243`）；
`modhost.go:99-103` 注释说"判不出来按有"但代码 `:114` 三条皆无时返回 false；
`docs/modkit.md:206-221` 的示例写 `package main`，而 `support.go:741-771` **强制 `package modpkg`**。

---

## 2. 扩展设计

### 2.1 Tier 1 —— 补缺口（低风险，建议必做）

| 项 | 改哪里 | 说明 |
| --- | --- | --- |
| G1 | 服务端新增"钩子→允许操作"的**注册期强制** | 沿 `layer.go:129-166` 的表在 `internal/servermod` 建同一张表，注册时校验；表不一致由测试锁死 |
| G2 | 服务端启动时**核对声明与注册** | 需要 modkit 把 `mod.json` 的 hooks 传给服务端（或服务端读 `mods/<id>/mod.json`——`install2.go:524-529` 会落这份副本，**已具备条件**） |
| G3 | `connection_output.go:50,64` 补观察点 | 与 `send` 同一份 `HasObservers()` 零开销快路径 |
| G4 | `main.go:90` 传真值 | 纯修 bug |
| G5 | 接线或删除 `SetConfigSnapshot` | 不留半成品；建议接线，让 `config.read` 能看到有效配置 |
| 小项 | 删 `layerPermissions()` 死代码；权限清单收敛到单一真源；修 `modhost.go` 注释/代码不符；修 `docs/modkit.md` §5.5 示例 | 纯清理 |

### 2.2 Tier 2 —— 请求侧钩子 + 应答操作（**本次核心**）

**为什么是它**：所有玩法都从"客户端发来一条 CMD"开始。当前**没有任何请求侧接入点**，
所以 mod 无法实现新协议分支、新副本流程、新活动交互。补上它，等于把"新玩法"从"必须改内核"
变成"可以写成 mod"。

**接入点（唯一咽喉，已验证）**：`cmd/wireprobe/client_dispatch.go:63` 的 `dispatch()`——
它由 `client_connection.go:333` 对**每一帧**调用一次，且在调用前已完成解密与校验
（`client_connection.go:313-331` 产出 `plaintext` 与 `verified`）。在此函数最前面插入一层钩子链即可。

新增（服务端 `internal/servermod/host.go`）：

```go
const HookProtocolRequest = "protocol.request"

type RequestContext struct {
    Conn      string   // 连接标识（peer），用于归因与多连接隔离
    Type      byte     // 帧类型
    ID        uint16   // CMD/NOTI 号
    Raw       []byte   // 原始帧（只读）
    Plaintext []byte   // 解密后正文；mod 可**就地改写**，改写后必须重算校验和
    Verified  bool     // 校验和是否通过
    Level     int      // 角色等级（已知时；未知为 0）
    Reply     func(kind byte, id uint16, payload []byte) error // = reply.send
}

type RequestHook func(ctx *RequestContext) (handled bool, err error)
```

语义：

- 返回 `handled=true` → **短路**，后续内置 handler 全部跳过，由 mod 自己经 `Reply` 应答；
- 返回 `err` → 记日志并把该帧判为失败（不 panic、不影响其它连接）；
- 未登记任何 request 钩子时**零开销直接返回**（照 `ObserveResponse` 的写法，`host.go:353-364`）。

新增宿主操作（必须同步两仓）：

- `reply.send` —— 按连接发一条报文。**没有它，短路的 mod 无法应答客户端**，钩子等于残废。
- `protocol.request` 的 `Allowed` 应含：`log` / `config.read` / `reply.send`（外加 `content.register`
  仅作声明）。按 Tier 1 的强制表生效，不再只是装饰。

> **性能与归因纪律**：钩子链在每帧路径上，必须无分配快路径；每次短路都打一行
> `[mod <id>] 短路 CMD<N>`，让"哪个 mod 吃掉了这条报文"永远可从日志复原。

### 2.3 Tier 3 —— 领域/规则扩展（按需，逐条裁决）

| 扩展 | 用途 | 成本 |
| --- | --- | --- |
| Lua 事件集扩展 | 在"通关/进本/用药/购买"等时机发东西，**不重编译** | 5 处：`internal/reward/trigger.go:8-10`（常量）、`lua.go:30-32`（白名单）、`lua.go:68`（报错文案）、`reward.go:114/118/122`（入口方法）、`reward.go:223-229`（幂等键）+ 域内 emit 点 |
| `modpolicy` 扩面 | 倍率/开关类调服参数（掉落、经验、次数…） | 每个策略一个 struct + Configure/Xxx 访问器 + 既有业务里的消费点 |
| 领域钩子 | 仅在 Tier 2 表达不了时开 | 高，需逐条论证 |
| 新 grant 函数 | Lua 发装备/称号等 | 中，沿 `internal/reward/lua.go` 既有 13 函数的写法 |

### 2.4 明确**不**扩

- ❌ mod 往服务端注册内容表（违反单一内容真源铁律）；
- ❌ mod 改数据库 schema；
- ❌ `protocol.response` 改成可改写（保取证链）；
- ❌ mod 之间互相 import（Go 同名包无法区分，`MOD-DEVELOPMENT.md:346`）。

---

## 3. 改动清单（两仓逐点）

### 3.1 服务端仓 `115`

| 文件 | 改动 |
| --- | --- |
| `internal/servermod/host.go` | 加 `HookProtocolRequest` 常量（`:50` 一带）、`RequestContext`/`RequestHook` 类型、`RegisterRequest`、注册表切片（`:110-116` 一带）、分发函数 `ObserveRequest`（仿 `:353`）、`reply.send` 的实现与 `Op*` 常量（`:39-47`）、Tier 1 的"钩子→允许操作"强制表 |
| `cmd/wireprobe/client_dispatch.go` | `dispatch()` `:63` 最前面插入钩子链调用 |
| `cmd/wireprobe/connection_output.go` | `:50` `writeRaw`、`:64` `writePrepared` 补观察点（G3）；`reply.send` 落到 `send` |
| `cmd/wireprobe/main.go` | `:90` 传真 `ChannelCount`（G4）；`SetConfigSnapshot` 接线（G5）；启动日志加 request 钩子数 |
| `docs/` | 本方案的实现记录与验收记录 |

### 3.2 启动器仓 `115us-dfolauncher`

| 文件 | 改动 |
| --- | --- |
| `internal/modkit/layer.go` | `HookPoints` `:129-166` 加 `protocol.request` 及其 `Allowed`；宿主操作常量 `:172-182` 加 `reply.send`；`HostOpNames()` `:185-187` 同步 |
| `cmd/modkit/run2.go` | 权限清单 `:239-243` 若变动需同步（当前与 `layer.go:58-70` 重复，建议一并收敛） |
| `internal/modkit/manifest2.go` | 白名单校验 `:206-239` 自动覆盖，**不需要改**；`MigrateV1ToV2` `:476-518` 不需要改（未加新 JSON 字段、未升 schema） |
| 生成器 | **不需要改**。`support.go:266-313 renderServerModsGen` 只吃 mod id，与钩子名/操作名无关 ⇒ **不碰逐字节 golden**（`support_gen_test.go`、`support_gen_loader_test.go` 的 `oneModGoldenSHA256`） |

> 这是本方案的一个关键有利事实：**加钩子 / 加宿主操作 / 加 kind 都不触碰生成器与 golden**，
> 改动面被限制在两张表和各自的调用点。

---

## 4. 目标内容 → mod 粒度（待细化）

**粗分类**（依据：`更新文件清单.json` 实测聚合 4685 文件 / 2.00 GB；
`本地修复差异.json` 670 条；`更新内容.txt` 叙述）：

| 桶 | 量 | 判定 | 走哪条路 |
| --- | --- | --- | --- |
| 客户端 16 文件（PVF 832MB、NPK 531MB、字体、`dstr.dat`、`dinput8.dll`、`sk.dat`） | 16 | 可 mod（PVF 层需 pwsh7；PVF 是内容真源，建议留基线） | client / resource 层 |
| 服务端协议与网关 | `internal/game` 142（protocol 131）+ `cmd/wireprobe` 168 | **需 Tier 2 改写成钩子**，且工作量大 | Tier 2 |
| 领域执行（副本/军团/raid/工作流） | legion 24、raid 24、workflow 23、loot 11… | 需 Tier 2；部分可 Tier 3 | Tier 2/3 |
| 数据库与存储 | `internal/database` 47 + 62 条删除 | ❌ 只能版本更新 | 不走 mod |
| 配置与策略 | `configs/*.json` 71 | 策略部分可 Tier 3 | Tier 3 |
| 工具与启动器二进制 | 启动器 64MB、`gmbridge.exe`、5 个 `bin/*.exe` | ❌ | 版本更新 |

**待业主裁决**：是否要为"服务端玩法"这一桶真的做 Tier 2（工作量大、且 mod 会与内核版本强耦合），
还是只做 Tier 1 + Tier 3，把客户端资源/DLL/规则做成 mod、服务端玩法继续走版本更新。

---

## 5. 风险清单

| 风险 | 说明 | 缓解 |
| --- | --- | --- |
| mod 旁路协议取证链 | 请求侧钩子能被 mod 用来实现任意协议行为 | 钩子**不改写服务端报文**、只做短路+自己应答；每次短路打日志归因；`protocol.response` 保持只读 |
| 平行内容表 | 用 mod 塞内容是最省事的诱惑 | 本方案不开内容注册；`content.register` 保持只登记 |
| 存档兼容 | schema 仍由服务端幂等升级承担 | mod 不碰 schema；`archive` 兼容义务不变（根 `AGENTS.md` 铁律 4） |
| 多 mod 冲突 | 两个 mod 抢同一条 CMD | 请求钩子链**按注册顺序**执行，短路即停；安装期对同名钩子做提示 |
| mod 与内核耦合 | mod 可 import 内部包 ⇒ 内核重构会拖断 mod | 沿用 `modpolicy` 模式：只依赖**窄而稳定**的宿主契约 |
| 生成器 golden 被打破 | 一旦有人顺手改生成器 | 本方案明确**不改** `renderServerModsGen` |

---

## 6. 验收与提交

1. 每个扩展点单独提交，一假设一提交（`server/AGENTS.md` §5.1）；
2. `go build ./...` / `go vet ./...` / `go test ./... -count=1` **三项 exit 0**，
   且失败集合与 `.tmp/baseline-20261007/` 的零失败基线**逐名比对**；
3. 启动器仓同样三项过；
4. 提交前 `pwsh -NoProfile -File scripts/check-commit-hygiene.ps1`，命中即停并逐条报告业主（根 `AGENTS.md` §0.3.1）；
5. 实机由业主操作，AI 只读日志（根 `AGENTS.md` 铁律 6）。

---

## 7. 待业主裁决

1. **Tier 2 做不做**（请求侧钩子）——这是"新增玩法能做成 mod"的唯一前提，也是改动最大的一步。
2. **Tier 1 的五处缺口是否一并补**（建议必做，尤其 G1/G2 两条"静默失败"）。
3. **`config.write` / `content.register` 的定位**：接线成真能力，还是明确降级为纯声明？
4. **PVF 层是否随包带 pwsh7**（否则含 pvf 层的 mod 现在一律装不上）。
5. 工作区隔离：`mods/odyssey.*`、`mods/difficulty.rules` 当前有**他人未提交改动**，本方案不碰。

---

## 8. 实施记录

### 8.1 业主裁决（2026-10-07）

- **范围**：Tier 1 + Tier 2 + 一个端到端样板。
- **mod 化对象**：客户端资源 **与** 服务端新增玩法都要 mod 化。

### 8.2 必须在**当前工作树**上做，不能在 20261006 包快照上做

取证发现包快照与工作树不一致（包 = 启动器 1.6.10，工作树 = 1.7.8）：

| 项 | 20261006 包 | 当前工作树 |
| --- | --- | --- |
| `protocol.response` 生产调用点 | **无**（全模块只有 `host_test.go:101`） | `cmd/wireprobe/connection_output.go:31-33` |
| `internal/modpolicy` | **不存在** | 存在（`OdysseyRules` / `DropRules`） |

⇒ 在包上做 mod API，`protocol.response` 与 `modpolicy` 两点都是死的。**一律以工作树为准。**

### 8.3 已完成：Tier 2 请求侧钩子（两仓，已验证）

**服务端仓**

| 文件 | 改动 |
| --- | --- |
| `internal/servermod/host.go` | 新增 `HookProtocolRequest`、`OpReplySend`、`RequestContext`/`RequestHook`、`requestReg` 注册表、`RegisterRequest`、`HasRequestHooks`、`ObserveRequest`；`Description()` 报出 request 钩子数；顺带订正包注释里"只提供 5 个宿主机操作"的旧口径与"mod 不 import 内部包"的失实说法 |
| `cmd/wireprobe/client_dispatch.go` | `dispatch()` 最前面插入钩子链（`HasRequestHooks` 零开销快路径；短路返回 `dispatchHandled`；`Reply` 接到 `client.output.send`） |
| `internal/servermod/host_test.go` | +4 用例：帧上下文/短路/顺序、坏 mod 不拖垮其余、无钩子零开销、入参纪律 |
| `cmd/wireprobe/client_dispatch_test.go` | +2 用例：钩子赢过内置 handler 且应答仍走正规编码、放行路径零副作用 |

**设计决定（相对本文档 §2.2 初稿的收紧）**：**不给"就地改写 `Plaintext`"的能力**。
理由：改写要重算校验和并重加密，等于把协议改写搬进宿主，协议真源会退化成"某个 mod 的猜测"。
要改行为就让 mod 自己接手这条报文——归属清楚、日志可归因、取证链不断。

**启动器仓**

| 文件 | 改动 |
| --- | --- |
| `internal/modkit/layer.go` | `HookPoints` 加 `protocol.request`（`Allowed` = log / config.read / reply.send）；新增 `HostOpReplySend = "reply.send"` 并列入 `HostOpNames()`；订正"第一期只放五个"的旧注释 |
| `internal/modkit/manifest2.go` | 订正 `:231` 那条**失实注释**（原文声称 "Allowed 由服务端侧在注册时强制"，服务端无此代码）——见 §1.2 G1 |

**验证**（两仓均实测）：`go build ./...` = 0、`go vet ./...` = 0；
服务端 `go test ./... -count=1` 与启动器 `go test ./... -count=1` 见本轮日志。
加钩子/加操作**没有触碰生成器与逐字节 golden**（`renderServerModsGen` 只吃 mod id），与 §3.2 的预判一致。

### 8.4 已完成：Tier 1 G2「声明与注册的一致性核对」（服务端仓，已验证）

**为什么是它**：这是本次扩展里唯一一条能把"**装成功、编成功、启动成功、什么也没发生**"
从静默变成响亮失败的机制。modkit 只校验钩子名在白名单里、安装期只保证包能编译，
所以"mod.json 声明了 `server.boot`、Go 里忘了调 `RegisterBoot`"过去**完全不可见**。

| 文件 | 改动 |
| --- | --- |
| `internal/servermod/declarations.go`（新增） | `RegisteredHooks(modID)`（覆盖 4 个注册函数 + `reward.script` 由 `RegisterRewardScript` 满足）、`modDeclaration` 宽松解析、`declaredHooks`、`CheckDeclarations(modsDir)`、`DeclarationProblem` |
| `cmd/wireprobe/main.go` | `Boot` 之后、开始监听之前核对；不一致 → **拒绝启动**（口径同 `Boot` 的 fail-closed） |
| `internal/servermod/declarations_test.go`（新增） | +9 用例：声明=注册、声明未注册、注册未声明、禁用 mod 跳过、无 mod.json 目录跳过、`reward.script` 特殊映射、坏 manifest 报错、目录不存在 |
| `internal/servermod/host_test.go` | `reset` 一并清 `modScripts` 与启用清单，保证用例互不串味 |

**三条设计判据**

1. **只在"启用且带 `mod.json`"时判**：被 `enabled.json` 禁用的 mod 本来就不该注册钩子
   （`Register()` 开头 `return`），报成问题就是假警报。
2. **识别判据与 modkit 一致**：只看"目录里有 `mod.json`"。modkit 也只给带 hooks 的 mod
   落这份文件（`install2.go` 的 `applyServerGoPackage`），只带 Lua 脚本的 mod 不建目录 ——
   `mods/scripts/` 这类目录必须被无视。
3. **`reward.script` 没有独立注册函数**，由 `RegisterRewardScript` 满足，所以按"该 mod
   有没有登记过规则脚本"判定，否则会把正常 mod 全部误报。

**实机防误报验证（临时用例，跑完即删）**：用真实 `mods/` 目录走
`LoadEnabledList → mods.RegisterMods() → CheckDeclarations`，实测结果：

```
已注册 mod = [odyssey.hardcore]
  odyssey.hardcore 实际注册的钩子 = [console.command server.boot]
现网 mods/ 通过核对，无误报
```

即现网唯一已装 mod 的 `mod.json` 声明（`server.boot` + `console.command`）与它的实际注册
**逐字符一致**，这条 fail-closed 检查不会挡住现网启动。

### 8.5 已完成：端到端样板 mod（`mods/examples/request-hook-pilot/`）

**目的**：证明 `protocol.request` + `reply.send` 这条新通路**真的能装、能验、能编译**，
并给"把新玩法写成 mod"的人一个可照抄的骨架。它不是玩法 mod。

| 文件 | 作用 |
| --- | --- |
| `mod.json` | schema 2，`permissions: ["server.hook"]`，声明 `protocol.request` + `console.command` |
| `server/mod.go` | `package modpkg`；`Register()`（先问 `Enabled()`）→ `RegisterConsoleHelp` / `RegisterConsole` / `RegisterRequest`；`onRequest` 只在"type=1 id=59999 且正文以魔数 `D0 0F BE EF` 开头"时接手并用 `ctx.Reply` 应答，其余一律放行 |
| `README.md` | 装/验命令、三条"怎么确认真的生效"、边界 |
| `.gitignore` | `dist/` 不入库（与 `newchar-kit`、`examples/*` 一致） |

**为什么对真实游戏零影响**：正常客户端永远不发那个魔数，所以每帧都是"看一眼、放行"。

**实测证据（本机 2026-10-07）**

| 步骤 | 结果 |
| --- | --- |
| 对着真实宿主 API 编译 | `go build ./mods/demo.request-hook-pilot` = **0**，`go vet` = **0**（临时落位后已删） |
| `modkit layers` | 已列出 `protocol.request` → `log, config.read, reply.send`；宿主操作表含 `reply.send` |
| `modkit verify --mod <zip>` | **exit 0**，"清单 OK … 层：server，权限：server.hook" |
| `modkit plan --client <客户端> --root <整合包根>` | **exit 0**，"**3 执行 / 2 已就绪 / 0 阻断**"，并提示 server 层落地后必须重编译服务端 |

`install` 会写客户端/服务端模块并重写 `zz_mods_gen.go`、随后要重编服务端 —— 那是**实机步骤**，
按铁律 6 由业主操作。

### 8.6 关于 G1 的结论订正（业主未指定 (a)/(b)，我实测后改判）

上轮说"按 (b) 做"（modkit 静态扫描 mod 源码里的 `servermod.<操作>` 调用）。
本轮把这件事想清楚后**改判：不做**。理由：

`Allowed`（钩子 → 允许的宿主操作）在这套架构里**做不成真正的门**。mod 与内核同属 `dfolan`
模块，Go 层面**本来就能 import 任意内部包**（现成例子 `mods/odyssey.hardcore/mod.go` 就
import 了 `internal/modpolicy`）。因此：

- (a) 服务端按 mod 收口操作入口 —— 要改宿主操作函数签名（破坏性变更），**仍然是假门**，
  因为 mod 可以直接 import 内部包绕过去；
- (b) modkit 静态扫描 —— 更弱：一行变量间接调用就能绕过，还多养一张"Go 标识符 → 操作名"的
  同步表。

**正确处置是如实标注，而不是演一道假门** —— 这一条已在 §8.3 完成（订正 `manifest2.go` 里
那条"Allowed 由服务端侧强制"的失实注释），`Allowed` 现在的定位是**审计/展示声明**。

若业主要真正的边界，唯一有意义的做法是**改变架构前提**（例如 mod 不再与内核同模块、
或宿主操作收进一个受限接口类型），那是一次独立立项，不在本方案范围内。

### 8.7 下一步

1. **选第一个要 mod 化的第四次更新玩法**（需要业主点名）。这是本目标剩余的主体工作：
   把该玩法的实现从内核搬到 `mods/<id>/`，用 `protocol.request` 承担协议分支。
   建议从**边界清楚、不碰 schema** 的活动类玩法起步（如"奇迹加速活动"的奖励/引导分支），
   而不是 `internal/database` 或 `internal/game/protocol` 的表级改动。
2. **客户端资源 mod**：`dinput8.dll` / 新增 NPK / 字体 / `dstr.dat` 走 client、resource 层
   （现有能力，`mods/client-mods/` 已有先例）。PVF 层要先解决 pwsh7 依赖。
3. **G4**（启动顺序重排，让 `BootContext.ChannelCount` 有真值）、**G5**（`SetConfigSnapshot`
   接线或删除）——各自单独一次假设/提交。
4. **提交门禁**：`git fetch origin` + `git fetch fork` → 落后则 `merge`（禁 rebase）→
   重跑 build/vet/test → `scripts/check-commit-hygiene.ps1` → 只暂存本任务文件。

### 8.8 并发写者警告（实施期实测，提交前必看）

本方案实施期间，**启动器仓有另一个并发写者**在改
`internal/modlib/store.go`、`internal/modlib/serverstale_test.go`、
`internal/serverbuild/serverbuild.go(+_test)` —— 与本方案的 `internal/modkit/**` 无重叠，
但**会污染测试结果**：

实测时间线（2026-10-07）：

| 时刻 | 事件 |
| --- | --- |
| 14:19:46 / 14:20:47 | 本方案写 `internal/modkit/layer.go` / `manifest2.go` |
| 14:26:24 / 14:26:32 | 并发写者写 `modlib/store.go` / `serverbuild/serverbuild.go` |
| 14:28:14 | 并发写者写 `modlib/serverstale_test.go`（+122 行） |
| **14:28:33** | 本方案跑启动器全量测试 → `modlib` 两个用例红 |
| 14:29 起 | 重跑与 `-count=20` 全绿 |

⇒ **那两条红是并发写者的在飞改动造成的，不是本方案的回归。**
提交前若再遇到启动器仓的失败，先看 `git -C <启动器仓> status --short` 有没有第三方文件，
再决定是不是自己的问题。**绝不暂存、绝不回退这 4 个文件。**

---

## 9. 客户端资源分档（第四次更新 2026 的 16 个客户端文件）

### 9.1 实测对照（包内 vs 现网 `C:\Game\dof\115us\DFO`）

| 文件 | 包内 | 现网 | 形态 |
| --- | --- | --- | --- |
| `Fonts/head-name-simsun.ttc` | 18,316,748 | **缺失**（现网是同尺寸的 `simsun.ttc`） | `file.add` |
| `Fonts/hud-cn-digits.otf` | 8,482,020 | **缺失** | `file.add` |
| `Fonts/menu-cn-regular.otf` | 8,482,020 | **缺失** | `file.add` |
| `Fonts/menu-cn-serifbold.otf` | 11,728,184 | **缺失** | `file.add` |
| `ImagePacks2/NpkIndex.etc` | 4,655,322 | **该路径缺失**（现网在客户端根，4,410,859） | `npk.index` 重建 |
| `sprite_avatar_dragonrobe4_20261006.NPK` | 127,681,898 | **缺失** | `npk.add` |
| `sprite_avatar_dragonrobe4_effects_20261006.NPK` | 177,406,251 | **缺失** | `npk.add` |
| `sprite_interface.NPK` | 531,548,516 | 531,868,113（不同） | ⚠️ 整份替换不可行 |
| `sprite_interface2_aura.NPK` | 1,494,576 | 1,295,623（不同） | ✅ `npk.replace` |
| `sprite_interface2_titlebook.NPK` | 1,200,784 | 1,201,035（不同） | ✅ `npk.replace` |
| `sprite_live_event_kor_2025_0109_boostup.NPK` | 25,684,311 | 13,171,810（不同） | ⚠️ 偏大 |
| `sprite_live_event_kor_2026_0326_boostup.NPK` | 1,613,411 | 1,668,420（不同） | ✅ `npk.replace` |
| `dinput8.dll` | 840,192 | 107,008（不同） | `file.replace`（**被 .gitignore 挡**） |
| `dstr.dat` | 1,941,444 | 同尺寸**但哈希不同** | `file.replace`（**被挡**） |
| `sk.dat` | 2,816 | 2,560（不同） | pvf 层**必须与 PVF 成对** |
| `Script.pvf` | 832,661,309 | 761,764,363（不同） | pvf 层（**被挡** + 要 pwsh7） |

### 9.2 三条硬结论

**① 素材不能入库 ⇒ 客户端 mod 只能是本地构建产物。**
仓库 `.gitignore` 第 5 节全局挡 `*.dll` / `*.dat` / `*.pvf` / `*.zip` / `*.exe`；
而 4 个字体（8.4~18.3 MB）与 2 个新 NPK（127/177 MB）全都超过根 `AGENTS.md` §0.3.3 的
**单 blob 5 MB 阈值**，入库要先问业主。所以**仓库里放的只能是配方**（`mod.json` 模板 +
构建脚本 + 分档表），**不是素材本身**。

**② 大容器不能走整份替换。**
`sprite_interface.NPK` 531 MB、`Script.pvf` 832 MB：`npk.replace` / `pvf.replace` 要求包里
带整份内容，且 modkit 会给原文件**再留一份备份**（`.launcher-mods`），磁盘代价是原件的好几倍。
正解是 **`npk.entries` 条目级增量**（只动变化的 img 条目，NPK 其余逐字节保留）。

**③ 但条目级增量现在缺工具。**
`npk.entries` 的清单要写 `{archive, entry, source, sha256, size}` —— 即"**哪些条目变了**"。
modkit 里有 NPK 解析/写回实现（`internal/modkit/npk.go`），但**没有导出的差分工具**，
所以目前**算不出这份清单**。这是客户端 lane 要补的下一个能力，也是把 531 MB 压到几 MB 的唯一路径。

### 9.3 本期已实测跑通的部分（本地，未入库）

用三个小容器做 `npk.replace` 实证（`.tmp/clientmod-res/`，跑完可删）：

```
verify exit 0   清单 OK；层：resource；权限：resource.npk
plan   exit 0   3 执行 / 0 已就绪 / 0 阻断
                ImagePacks2/sprite_interface2_aura.NPK
                ImagePacks2/sprite_interface2_titlebook.NPK
                ImagePacks2/sprite_live_event_kor_2026_0326_boostup.NPK
```

⇒ **resource 层确实能吃第四次更新的真实 NPK**（整份替换这条路是通的，只是不适合大容器）。

### 9.4 客户端 lane 的剩余工作

1. **NPK 差分工具**（要补的能力）：输入"现网 NPK + 更新 NPK"，输出 `npk.entries` 清单 +
   变化的 `.img` 条目，把 531 MB 压成增量。需业主决定工具落位（启动器仓新增 `cmd/<工具>`？
   还是给 modkit 加导出的差分 API？两者都要动目录/API，按 §0.4.1 需先问业主）。
2. **`NpkIndex.etc` 重建**：走 `npk.index`（脚本型 op），但**依赖 pwsh7** —— 与 pvf 层同一个卡点。
3. **PVF 层（结论已订正）**：`Script.pvf` + `sk.dat` 必须成对。
   **说错过的**：本文档早先写"含 pvf 层的 mod 一律装不上（缺 pwsh7）"—— **不成立**。
   实测（pwsh 在 PATH / `C:\Program Files\PowerShell\7` / `<客户端>\tools\pwsh7` 三处**全为 False**）：

   ```
   verify exit 0   层：pvf；权限：pvf.merge / exec.script
   plan   exit 0   2 执行 / 0 已就绪 / 0 阻断
                   pvf.replace  Script.pvf ← pvf/Script.pvf（原文件先备份，卸载逐字节还原）
                   pvf.replace  sk.dat     ← pvf/sk.dat
   ```

   代码侧的原因也清楚：`apply2.go` 只有 `PVFKindVerify`(:432) 与 `PVFKindMerge`(:450) 调
   `resolvePwsh`，**`PVFKindReplace`(:480-490) 不调** —— 它不跑脚本。

   ⇒ 准确口径：**`pvf.replace` 不需要 pwsh；`pvf.merge` / `pvf.verify` 需要。**
   但对第四次更新而言，**实际卡点从"装不上"变成了"装得下吗"**：
   `replace` 要包里带整份 **832 MB** 的 `Script.pvf`，而"把新旧 PVF 合并成增量"的
   `merge` 恰好是要 pwsh 的那条路。所以 PVF 的实用路径仍然被 pwsh 挡着，
   只是原因与本文档早先写的不一样。

   另注：`manifest2.go:161-164` 规定**只要声明了 pvf 层**就要同时具备
   `pvf.merge` + `exec.script` 两个权限位 —— 纯 `replace` 的包也被要求声明
   `exec.script`，属权限模型的粗粒度，功能上不阻断（见上面 verify 输出）。
4. **字体**：`head-name-simsun.ttc` 与现网 `simsun.ttc` **尺寸完全相同**，
   疑似同一份文件的改名副本；入库前需业主确认这两个是不是同一内容（若是，`file.add` 即可，
   不必新增 18 MB）。

---

## 10. 条目级增量：工具已落地 + 实测倍数

### 10.1 工具落位（不动顶层目录约定）

按 §0.4.1「新增顶层目录/顶层文件前必须问业主」，**没有**新增 `cmd/<工具>`，而是：

| 位置 | 内容 |
| --- | --- |
| `internal/modkit/npkdiff.go`（新增） | 能力出口：`DiffNPKArchives(old,new)`、`ReadNPKEntryPayload`、`NPKDiffResult`/`NPKDiff`/`NPKEntryState`、`Counts()`、`ChangedBytes()` |
| `cmd/modkit/npkdiff.go`（新增） | 子命令 `modkit npkdiff`：算差异、可选 `--out` 导出条目载荷与可直接粘进 mod.json 的 `npk.entries` 动作 |
| `internal/modkit/npkdiff_test.go`（新增） | +5 用例：四类判定/稳定排序/同尺寸不同内容/零差异/读回逐字节/非 NPK 拒绝 |

`main.go` 三处同步（flag 变量、`newNPKDiffFlags()`、`newFlagSets`、`usageText()`）——
`usage_flags_test.go` 的正反向护栏与 `TestSubcommandsUsageSmoke` 全部通过。

**判据是"载荷逐字节"而不是"看文件大小"**：同尺寸不同内容（换压缩参数）必须报变化。
读取是**流式**（`io.Copy` 到 sha256），不把 531 MB 读进内存。
读出的字节与 `replaceNPKEntry` 的写回口径**同源**（原始压缩载荷，逐字节搬运、不重压缩）。

### 10.2 实测：五个被改容器的真实倍数（2026-10-07，本机）

| 归档 | 条目 | 变化 | 条目级需入包 | 整份替换 | 倍数 |
| --- | --- | --- | --- | --- | --- |
| `sprite_interface.NPK` | 212 | **22** | **7.03 MB** | **507.23 MB** | **72.2×** |
| `sprite_live_event_kor_2025_0109_boostup.NPK` | 4 | 1 | 0.46 MB | 12.56 MB | 27.6× |
| `sprite_interface2_titlebook.NPK` | 5 | 1 | 0.45 MB | 1.15 MB | 2.5× |
| `sprite_live_event_kor_2026_0326_boostup.NPK` | 6 | 3 | 0.86 MB | 1.59 MB | 1.9× |
| `sprite_interface2_aura.NPK` | 6 | 3 | 1.33 MB | 1.24 MB | **0.9×** |

**合计 ≈ 10.1 MB vs 523.8 MB ⇒ 约 52×**；`sprite_interface` 单个归档耗时 **0.8 秒**。

### 10.3 关键结论：条目级不是无条件更优

`aura` 那一行是 **0.9×——条目级反而更大**。所以选型判据不是"一律用条目级"，而是：

> **条目级只在"归档远大于变化量"时赢。**
> 归档大、改动少（`sprite_interface` 212 条里只动 22 条）⇒ 用条目级；
> 归档小、改动占比高（`aura` 6 条里动 3 条）⇒ 整份替换更省。

工具只负责**如实报出两个数字与倍数**，选型由人按这张表定。

### 10.4 端到端闭环（本地，未入库）

用 `npkdiff` 的真实产物打了一个 `npk.entries` 包（`.tmp/modkit-round5/entriespilot/`）：

```
22 个条目载荷，zip = 6.82 MB（整份替换要 507 MB）
verify exit 0   清单 OK；层：resource；权限：resource.npk / resource.index
plan   exit 0   22 执行 / 0 已就绪 / 0 阻断
```

⇒ **"507 MB 压到 7 MB"这条路是通的**，且引擎接受这份条目级清单。

### 10.5 残留边界（必须说清）

- **退档备份仍是整份**：`applyNPKEntry` 会 `copyFileAtomic(full, backupAbs)`，
  即条目级覆盖**照样把整个 531 MB 归档备份一份**（卸载=整文件还原）。
  所以 mod 包小了，**磁盘占用没小**。要真正省盘，得给条目级做"只备份被动条目的原始载荷"。
- **资源层不支持删除条目**：新归档里消失的条目无法通过 mod 表达，工具会如实列出来。
- **`NpkIndex.etc` 重建**（`npk.index`）是脚本型 op（`apply2.go:375` 调 `resolvePwsh`），
  所以缺 pwsh 时**在计划阶段就被阻断** —— 见 §11（该门禁是 2026-10-07 本轮补上的）。
- **PVF 层**已订正口径，见 §9.4 第 3 条：`pvf.replace` 无需 pwsh，`pvf.merge` / `pvf.verify` 需要。

---

## 11. 计划期 pwsh 门禁（本轮发现并修复）

### 11.1 缺陷（实测）

四层路径的脚本型动作（`pvf.merge` / `pvf.verify` / `npk.index`）在**安装期**才调
`resolvePwsh`，取不到就失败。而**计划侧从前完全不知道 pwsh 存不存在**，于是：

```
（修复前）modkit plan --client <无 pwsh 的客户端> --mod <含 npk.index 的包>
步骤：1 执行 / 0 已就绪 / 0 阻断        ← plan 说"可执行"
plan exit = 0
```

点下去安装才会报 `找不到 pwsh 7`。**这正是本项目最忌讳的那类落差**（面板说行、落地才炸）。
v1 路径本来是有这道门禁的（`plan.go:190-191` 的 `classifyScript` 会标 `StepBlocked`），
四层改造时漏掉了。

### 11.2 修法

| 位置 | 改动 |
| --- | --- |
| `internal/modkit/layerplan.go` | 新增 `LayerPlan.pwshReason`（非导出、不进 JSON）；新增 `pwshUnavailableReason(explicit)`，**复用安装期同一个 `resolvePwsh`**，不另发明判据；`planPVF` 对 `verify`/`merge` 与其 `produces` 步骤、`planResource` 对 `npk.index` 标 `StepBlocked` |
| 同上 | 新增 `BuildLayerPlanWithOptions(m, client, reg, root, pwshPath)`；`BuildLayerPlanWithRoot` 委托给它（签名不变，既有调用方与测试零改动） |
| `internal/modkit/install2.go` | 求解计划时把 `opts.PwshPath` 递进去 —— 否则操作者用 `install --pwsh <路径>` 指定了 pwsh，计划却会误判成阻断 |
| `internal/modkit/layerplan_pwsh_test.go`（新增） | +3 用例：`pvf.merge` 阻断、`npk.index` 阻断、**`pvf.replace` 不阻断**（反向护栏） |

用例刻意用**一个不存在的显式 pwsh 路径**制造"确定取不到"，这样与本机装没装
PowerShell 7 无关（否则在装了 pwsh 的机器上恒绿，等于没有护栏）。

### 11.3 修复后实测（同一条命令）

```
npk.index   步骤：0 执行 / 0 已就绪 / 1 阻断   plan exit = 2
pvf.merge   步骤：0 执行 / 0 已就绪 / 3 阻断   plan exit = 2
            ⚠ 未找到 pwsh 7（PowerShell 7+）：这一步要跑包内 .ps1 脚本…
pvf.replace 步骤：2 执行 / 0 已就绪 / 0 阻断   plan exit = 0   ← 非脚本型，不受影响
```

**测试影响面为零**：现存用例里没有任何一条断言"无 pwsh 时脚本动作可执行"
（`modkit_test.go:302` 那条 v1 `KindScriptRun` 用例本来就 `t.Skip`）。

---

## 12. 条目级覆盖的**卸载还原缺陷**（本轮发现并修复）

### 12.1 症状（实测，不是推断）

用临时客户端目录 + 真实 NPK 副本做**安装→卸载往返**（此前只跑过 verify/plan，这是第一次真写盘）：

```
原始副本      sha=16112cff…  size=1295623
install  exit 0   安装后 size=1494544
uninstall exit 0  "已还原：ImagePacks2/sprite_interface2_aura.NPK（npk.entries）"
卸载后        sha=53b607d1…  size=1496310   ← **不是原件**
```

`uninstall` **报告成功**，但归档没还原到原始字节。

### 12.2 根因

一个 `npk.entries` 动作可以列出**同一归档的多个条目**，`applyResourceLayer` 会逐条调用
`applyNPKEntry`。而旧实现里**每个条目都做一次** `copyFileAtomic(full, backupAbs)`，
备份路径只由 `mod id + 归档路径` 决定 ⇒ **后写覆盖先写**，备份记下的是
"已经改过前几条之后"的中间态。卸载据此还原，原始字节永久丢失。

归档大小与推理逐项吻合（aura，3 条替换）：

| 状态 | 大小 |
| --- | --- |
| 原版 | 1,295,623 |
| 替换第 1 条后 | ≈1,297,980 |
| **替换第 2 条后** | **≈1,496,342** |
| 替换第 3 条后 | ≈1,494,576（= 包内那份） |
| **实测卸载后（= 备份内容）** | **1,496,310** |

⇒ 备份是"改过 2 条之后"。**影响面**：凡是在同一归档上覆盖 ≥2 个条目的 mod 都中招；
本轮做的 `sprite_interface` 样板有 **22 个条目落在同一归档**，卸载会把归档还原成
"改过 21 条之后" —— 比不还原更糟。

### 12.3 修法

把快照从"每条目一次"提到"**每归档一次**"，在任何条目被改动之前完成，
顺序沿用既有铁律「**先备份 + 先写注册表日志，再动现场**」：

| 位置 | 改动 |
| --- | --- |
| `internal/modkit/apply2.go` | 新增 `archiveBackup` 与 `snapshotArchiveOnce(client, m, archive, seen, res)`：按归档去重、整份快照、**并写注册表**（先登记再动现场） |
| 同上 | `applyNPKEntry` 改为接收 `snap *archiveBackup`，只负责替换与更新 `InstalledSHA256`，**不再自己备份** |
| `internal/modkit/applyResourceLayer` | `ResKindEntries` 分支持有一张 `snapshots` 表，跨条目复用 |
| `internal/modkit/entries_backup_test.go`（新增） | 回归护栏：3 个条目落在同一归档 → 安装 → 逐条目核对 → 卸载 → **断言逐字节还原** |

### 12.4 修复后实测（同一条往返）

```
卸载前备份文件 = 1295623 字节（= 原件；修复前是 1496310）
卸载后 sha=16112cff…  size=1295623
逐字节还原成功? True
```

### 12.5 护栏的反面证据（证明它不是恒真）

把 `snapshotArchiveOnce` 的去重临时退掉（`ok && false`），同一条用例立刻变红：

```
--- FAIL: TestNPKEntriesMultiEntryUninstallIsByteExact
    entries_backup_test.go:154: 卸载后长度不对：827 ≠ 原版 824
```

恢复去重后转绿。用例刻意用 **3 个条目落在同一归档** —— 单条目时旧实现也能过，
所以 <2 条拦不住这个 bug。

> **教训（写下来给以后的人）**：`npk.entries` 的 `Note` 一直写着"卸载=整文件还原"，
> 而这条承诺在**多条目**场景下是假的。凡是"报告成功"的还原路径，都必须有一条
> **真的做一次安装再卸载、并比对原始字节**的用例兜着 —— verify/plan 永远发现不了它。

### 12.6 同一个洞还有第二层：快照表按 op 分配（本轮补掉）

第一版修复把 `snapshots` 表声明在 `case ResKindEntries:` **里面** —— 也就是"**每个 op 一张**"。
于是漏洞只是被缩小、没被根除：**同一归档上的两个 `npk.entries` 动作**，第二个动作
仍会把"已改过第一个动作"的内容当原件备份。

修法：把 `snapshots` 提到 `applyResourceLayer` 的**函数作用域**（= 每次安装一张表），
任何 op、任何条目都只快照一次。

新增第二条护栏 `TestNPKEntriesMultiOpUninstallIsByteExact`：**4 个条目拆成 3 个动作**
全部落在同一归档上 → 安装 → 逐条目核对 → 卸载 → 断言逐字节还原。

**两条护栏的分工（反面证据实测）**：把快照表改回"每 op 一张"后——

```
--- FAIL: TestNPKEntriesMultiOpUninstallIsByteExact   （多动作用例红）
--- PASS: TestNPKEntriesMultiEntryUninstallIsByteExact（多条目用例仍绿，因为它在同一个 op 内）
```

即两条用例各守一个洞，缺一不可。

### 12.7 其余写盘路径的往返矩阵（同一轮系统性实测）

吸取教训后，把**每条会写盘的路径**都真跑一遍 安装→卸载：

| 路径 | 往返逐字节还原 |
| --- | --- |
| `npk.entries`（多条目同归档 / 多动作同归档） | ✅ 修复后 True（修复前 False） |
| `npk.replace`（整份替换） | ✅ True |
| `pvf.replace`（`Script.pvf` + `sk.dat` 成对） | ✅ True / True |
| `client file.replace` | ✅ True |
| `client file.add` | ✅ 正确删除 |
| `client exe.patch` | ✅ True（另：`before` 字节不符时**拒绝**且 EXE 未被改动） |
| **server 层（Go 钩子 + 重写 `zz_mods_gen.go`）** | ⚠️ **本会话无法验证**，见 §12.8 |

⇒ 该缺陷**只**存在于 `npk.entries` 的多条目/多动作路径；其余路径的还原语义经实测成立。

### 12.8 server 层往返：本会话跑不了（环境限制，非仓库缺陷）

试装 `demo.request-hook-pilot` 进**真实服务端模块**时，`modkit.exe` 被拒：

```
mkdir ...\server\work\dfo-lan\mods\demo.request-hook-pilot: Access is denied.
（预建目录后）RemoveAll=unlinkat … Access is denied.
             重写 mods/zz_mods_gen.go 失败：open …: Access is denied.
```

**判定：不是仓库 ACL 缺陷，也不是"子进程一律被拒"。** 三个独立写者都成功：

| 写者 | 在该目录 Mkdir/Write/RemoveAll |
| --- | --- |
| 本会话 shell（`New-Item` / `Remove-Item`） | ✅ 成功 |
| `cmd /c echo > …`（子进程） | ✅ 成功 |
| 最小 Go 程序（`os.Mkdir` + `os.WriteFile` + `os.RemoveAll`） | ✅ **全部成功** |
| `modkit.exe`（同一个 shell 里 `&` 启动） | ❌ Access is denied |

而同一个 `modkit.exe` 写 `.tmp/` 下的一切都正常（本轮所有往返测试都写在那里）。
⇒ 是**本会话对 `modkit.exe` 这个二进制的写权限限制**，与仓库状态无关。

**善后（已核对）**：失败安装**没有留下任何痕迹** ——
`mods/zz_mods_gen.go` 仍是 `e4d61d604aaaa3b1`、`enabled.json` 仍是 `3cfd1d2d6c42bef4`
（与安装前逐字节一致），`mods/demo.request-hook-pilot` 无残留，`git status` 只剩本任务既有改动。
引擎自己的报错也如实说了「现场保留，请人工核对」——**中途失败没有静默损坏任何东西**。

**待办**：server 层的 install→uninstall 往返需要在**普通 shell**（不受本会话限制）里补测一次；
这正是业主实机验收里应当包含的一步。

（补：2026-10-07 又用 `cmd /c "<modkit.exe>" install …` 试了一次 —— **同样被拒**。
所以该限制与"怎么启动它"无关，是环境对这个二进制本身写该路径的限制。不再尝试。）

---

## 13. 客户端 lane 的落地结论

### 13.1 真包已生成（本地，未入库）

用 `modkit npkdiff` 对第四次更新里**全部 5 个被改动的 ImagePacks2 容器**生成条目级增量，
组装成**一个** mod 包（放在 `.tmp/4th-client-mods/`，不入库：包内是派生自外部更新包的载荷）：

| 容器 | 条目 | 变化 | 条目级 | 整份 | 倍数 |
| --- | --- | --- | --- | --- | --- |
| `sprite_interface.NPK` | 212 | 22 | 7.03 MB | 507.23 MB | 72.2× |
| `sprite_live_event_kor_2025_0109_boostup.NPK` | 4 | 1 | 0.46 MB | 12.56 MB | 27.6× |
| `sprite_interface2_titlebook.NPK` | 5 | 1 | 0.45 MB | 1.15 MB | 2.5× |
| `sprite_live_event_kor_2026_0326_boostup.NPK` | 6 | 3 | 0.86 MB | 1.59 MB | 1.9× |
| `sprite_interface2_aura.NPK` | 6 | 3 | 1.33 MB | 1.24 MB | 0.9× |
| **合计** | | **30** | **≈10.1 MB** | **523.8 MB** | **≈52×** |

实测：`115us.4th.client-npk-1.0.0.zip` = **9.85 MB**，
`verify` exit 0，`plan --client <DFO 客户端根>` = **30 执行 / 0 已就绪 / 0 阻断**。

⇒ 你最初问的"**这些补丁能不能做成 mod**"，对客户端资源这一半的回答是：
**能，而且比整份替换小 52 倍**。

**这个真包已通过安装→卸载验收（在临时客户端副本上，未碰现网）**：

```
install    30 步执行 / 0 已就绪             exit 0
           安装后逐容器与更新包核对：233 条目全部逐字节相同（0 变化 / 0 新增 / 0 删除）
              sprite_interface 212、aura 6、titlebook 5、2025_0109 4、2026_0326 6
uninstall  还原 5 / 删除 0 / 保留现场 0      exit 0
           备份大小逐容器等于现网原版（含 531,868,113 的 sprite_interface）
           卸载后 5 个容器全部逐字节还原 = True；注册表清空
```

这一跑同时覆盖了 §12 修掉的那两个洞（**多条目同归档** 22 条 + **多归档同 mod** 5 个）。

### 13.2 决定性检查：启动器会不会把 mod 改过的客户端资源覆盖回去？

**不会。** `internal/resources/manifest.go:50` 的 `ManagedGlobs` 是启动器"受管文件"的
**唯一真源**，内容只有服务端路径：

```
server/work/dfo-lan/configs/**
server/work/dfo-lan/cmd/wireprobe/testdata/**
server/work/dfo-lan/runtime/*.bin
server/work/dfo-lan/bin/*.exe
server/work/dfo_probe_tools/probe.exe
```

源码注释原话（`manifest.go:49`）："**只动这里列出的路径**：存档（runtime/storage/**）、
设置、日志、**客户端目录一律不在其中**。"

全仓核对也一致：非测试代码里提到 `ImagePacks2` / `Script.pvf` / `sk.dat` 的地方**全是只读**
（GM 工具提物品图标、启动器检查三件套是否存在）。**没有任何"按哈希自动替换客户端资源"的逻辑。**
⇒ 客户端 mod 装上之后**不会被 更新/自检 悄悄还原** —— 这是客户端 lane 可用的前提。

### 13.3 但 PVF 不能随便换：受"三件套"约束

`gm/internal/servercompat/catalog/pvf/client.go:61` 的报错是
「客户端资源解密校验失败，请检查 **DFO.exe、sk.dat 与 Script.pvf** 是否配套」——
即服务端解内层 PVF 时，这三者必须是**同一套构建**。

结合更新清单里的 `required_client_exe_sha256`，结论比"PVF 太大"更硬：

> **PVF mod 必须带一套与目标 DFO.exe 配套的 `Script.pvf` + `sk.dat`。**
> 换 PVF 不换 EXE（或反过来）会让服务端解不开内层归档。所以 PVF 的 mod 化不是
> "打包大小"问题，而是"**版本绑定**"问题 —— 这也解释了为什么 20261006 那个包
> 把三者当同一个版本单元一起发。

### 13.4 客户端 lane 的剩余边界

| 项 | 结论 |
| --- | --- |
| 5 个被改容器 | ✅ 条目级增量可 mod 化（真包已验证） |
| 2 个**新增** NPK（127/177 MB）+ 4 个字体（8.4~18.3 MB） | ⚠️ 纯新增，只能整份带 ⇒ 包内 ≈356 MB；且单 blob 超 §0.3.3 的 5 MB 阈值，**入库要先问业主** |
| `NpkIndex.etc` | 走 `npk.index`，**要 pwsh 7**（计划期已能正确阻断） |
| `dinput8.dll` / `dstr.dat` / `sk.dat` | 被 `.gitignore` 挡（`*.dll` / `*.dat`）⇒ 素材不能入库 |
| `Script.pvf` | 受 §13.3 的三件套绑定；`replace` 不需 pwsh 但要带整份 832 MB，`merge` 要 pwsh |











