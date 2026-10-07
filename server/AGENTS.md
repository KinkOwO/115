# AGENTS.md — server/

## 0. 单一内容真源铁律（2026-10-01 业主定调，最高优先级）

> **所有游戏内容数据只有一个来源：内层 PVF**（`server/work/client-build/Script.inner.pvf`）。
> 服务端已切换为 **PVF 直读**（`configs/pvf-default.json` + `bin/wireprobe-pvf.exe`）；
> `configs/*.json` 导出物**只是历史基线**，不再是运行期数据源。

**禁止**（无业主明确指令不得触犯）：

1. **禁止新增「导出 JSON → 服务端读取」的链路。** 新内容（奖励表、掉落组、装备、任务、
   商店、副本参数、玩法定义…）一律从 PVF 现场解析；需要新解析器就在 `internal/catalog/`
   或对应包内写 PVF 解析，**不要**新增 `configs/*.generated.json` 或类似导入器
   （`cmd/*import` 只保留给历史基线，不再扩写）。
2. **禁止把 PVF 已能提供的数据搬进 JSON 再读回来。** 判据很简单：**PVF 里读得到 ⇒ 不许写 JSON。**
3. **禁止新增「JSON 回落」。** `pvfCoreCatalogs.load*()` 里除 `if c.x != nil` 之外不得引入
   新数据源；已有回落随任务逐步删除——直读失败要**显式报错**，不要静默换源（静默换源正是
   此前「直读模式下玩法整片失效」却查不出来的原因）。
4. **禁止把开关当数据。** 玩法行为不留开关（见本文件 §6 开关原则）；数值差异入口
   必须能追溯到一条明确策略文件，且该文件**不得承载「有哪些内容」的清单**。
5. 禁止将工作日志写入`AGENTS.md`里，这里不是写日志的地方。

**允许保留的 JSON**（仅此三类）：

| 类别 | 例子 | 约束 |
| --- | --- | --- |
| 运维 / 策略 | `pvf-drop-policy.json` 的排除项、各 `pvf-*-policy.json` 的开关与上限 | 不得承载「有哪些内容」的清单；**能自动发现的一律自动发现** |
| 历史基线 | `configs/*.json` 导出物 | 仅在 `DFO_PVF_VERIFY_BASELINES=1` 时作对照，**默认关闭**；不得作为运行期输入 |
| 本地运行配置 | `launcher.local.json`、`pvf-default.json`、`storage/local.json` | 端口 / 路径 / 环境变量，不含游戏内容 |

## 1. 执行模型与技术栈

- **服务端主体**：Go 1.26（模块根目录位于 `server/work/dfo-lan/`，通过 `go.mod` / `go.sum` 管理依赖）。
- **服务启动编排**：**仓库内 Go 启动器**（`server/work/dfo-lan/bin/dfolauncher.exe`，源码 `cmd/dfolauncher` + `internal/launcher`）。2026-10-05 起启动链**不再需要 Python**：`launch_local.py`、`channel_probe.py`、`bootstrap_local.py`、`stop_environment.py` 已删除，入口只走 `scripts\storage-route.ps1` → `dfolauncher launch`。
- **数据持久化**：PostgreSQL 16.4 便携版（端口 25438），连接配置 `runtime/storage/local.json`，数据目录 `runtime/storage/pgdata/`。

## 2. 服务入口与端点约定

| 端点 / 入口               | 作用                                                         |
| ------------------------- | ------------------------------------------------------------ |
| `127.0.0.1:7001`          | Channel 频道目录与刷新服务（HTTP / 专有协议）                |
| `127.0.0.2:<动态端口>`    | Game 游戏接入网关（TCP，由 probe 协同引导连接）              |
| `scripts/启动游戏.cmd`     | 玩家与完整测试入口（需管理员权限，自动拉起存储、服务与客户端） |
| `scripts/启动服务端.cmd`   | 纯服务端调试入口（`storage-route.ps1 server-*` → `dfolauncher launch --server-only`）     |
| `scripts/停止游戏环境.cmd` | 安全关闭客户端、游戏服务、PostgreSQL (做 checkpoint) |
| `server/Build-Server.ps1` | 服务端编译脚本（执行测试、vet 并编译候选版）                 |

## 3. 目录职责（`server/work/dfo-lan/`）

| 路径                            | 职责                                                         |
| ------------------------------- | ------------------------------------------------------------ |
| `cmd/wireprobe/`                | 主服务网关与分发器（`main.go`、`*_flow.go` 编排各业务流）    |
| `internal/game/wire/`           | 底层封包格式、校验和计算、Blowfish 等加解密协议实现          |
| `internal/game/protocol/`       | 客户端/服务端协议编解码、原生测试向量测试                    |
| `internal/character/`           | 角色属性、基础数值、成长公式、技能树、疲劳保存与日切         |
| `internal/inventory/`           | 背包、穿戴校验 (wear)、装备属性与状态恢复                    |
| `internal/loot/`                | 掉落池计算、掉落物生成、拾取事务与去重                       |
| `internal/quest/`               | 任务链、任务目标推进（NPC 对话、范围到达、通关检查等）与奖励 |
| `internal/dungeon/`             | 副本会话状态机、房间切换、门控制、怪物清场与通关结算         |
| `internal/world/`               | 城镇场景、区域跳转、传送逻辑与位置保存                       |
| `internal/database/`             | PostgreSQL 数据库事务 (pgxpool)、角色存档持久化  |
| `internal/catalog/`             | 游戏规则驱动目录与静态数据索引解析                           |
| `configs/`                      | 导出的全量 JSON 规则配置（任务、地图、装备、掉落等）         |
| `scripts/`                      | PVF 导出/审计等开发工具与 `Generate-SQL.ps1`；**启动编排已全部收进 Go**（`cmd/dfolauncher` + `internal/launcher`） |
| `runtime/storage/`              | 本地存储：`pgdata/`、`local.json`（严禁入库） |
| `runtime/roles_*/`              | 运行会话追踪日志（`run.json`、`events.jsonl`、`helper.err`） |
| `reference/analysis-tools/*.py` | 分析辅助脚本                                                 |

## 4. 开发与构建规范

1. **测试门禁**：修改协议或业务逻辑后，在 `server/work/dfo-lan/` 执行 `go test ./...` 与 `go vet ./...`。
2. **数据库集成**：`go run ./cmd/dfo-tool charactercheck` 校验角色存储与 schema 兼容性。
3. **候选隔离**：源码编译输出 `bin/wireprobe-handoff-source.exe`，**严禁直接覆盖 39 版归档基线 `wireprobe-dungeon39.exe`**；实机完整回归确认后方可升级基准。
4. **实机回归**：关闭已有游戏会话后 `./scripts/启动游戏.cmd --source-build`，由用户手动操作。

## 5. 变更事务与数据安全

1. **一次假设、一次 commit**：每次协议 A/B 测试、功能补齐或状态机修复单独 commit，不堆叠未提交改动。
2. **存档向后兼容**：PostgreSQL 角色数据是玩家核心资产，数据库变更必须支持已有角色无损升级，严禁随意删除已初始化的 `pgdata/`。
3. **工作区隔离与忽略规则**：
   - 严禁提交 `server/work/dfo-lan/runtime/storage/pgdata/`
   - 严禁提交 `server/work/dfo-lan/runtime/storage/*.log`、`local.json`
   - 严禁提交动态会话目录 `server/work/dfo-lan/runtime/roles_*/`
   - 严禁提交本地编译的中间文件或未授权的大型二进制

## 6. 开关原则（2026-10-01 业主定调）

> **开关只用于本地调试；一旦确认有效，就移除开关、变成默认行为。**

1. **「玩法是否开启」不是开关。** 征兆系统、隐藏 BOSS 门禁、定盘机关兜底判死、疲劳规则……
   这类「不补就没功能」的东西一律**直接默认生效**，代码里不留 flag/env 入口
   （需要临时关闭时改代码，而不是加开关）。
2. **只有「玩家体验上的数值差异」才保留入口。** 例如掉落调参
   （`DFO_ATTUNEMENT_REBALANCE` / `-attunement-fixed-tilt`）、`DFO_SHOP_RELEASE`、
   **`DFO_FATIGUE_FREE`（疲劳消耗总开关，业主 2026-10-01 按玩家反馈要求；默认关 = 保留消耗，
   打开后进本与房间两处一起归零 —— 只关一处会卡在加载界面）** ——
   这类开合属于业主的经营决策，开关留在 profile（`configs/pvf-default.json`）里。
3. **诊断入口可以留。** `-omen-hold` / `-omen-info` / `-maze-force` 这类**只用于复现与取证**的入口保留，
   但帮助文本里必须写明是诊断用。

**为什么**（2026-10-01 深渊失效排查的教训）：那批深渊 MR 的功能全靠
`DFO_OMEN_REWARDS=1` / `DFO_OMEN_STATE=1` / `DFO_SCALE_DEATH_FROM_HP=1` / `-fatigue-rules` 开启，
而直读默认档 `configs/pvf-default.json`（17 个键）与 `scripts/启动服务端.cmd` 里**一个都没有**
⇒ 玩家走默认入口时**整套玩法静默不生效**：征兆不掷骰（日志 `-omen-rewards is off`）、
隐藏 BOSS 无门禁来源、定盘机关可能打不死（"既不放结束动画也不 DESTROY"）、
**疲劳服务根本没加载**（`if *fatigueRulesFile != ""` 不成立 ⇒ `fatigueService == nil` ⇒ 所有疲劳检查被跳过）。

**教训**：开关的代价不是多打一个 flag，而是「**默认路径悄悄坏掉**，且没有任何人会发现」。
