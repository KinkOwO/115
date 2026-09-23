# AGENTS.md — server/

> 本文件是 `server/` 目录的规则真源与子索引。先读仓库根 `AGENTS.md`，再读本文件。
> 服务端源码位于 `server/work/dfo-lan/`，探针与隔离工具位于 `server/work/dfo_probe_tools/`。

## 0. 当前基线与交付边界

- **2026-09-23 邮箱移植基线**：普通邮箱功能及后续刷新、领取崩溃、收件角色和金币显示修复已完整移植；原分支用户确认本轮修复完成，上游移植分支实机待验证。特殊付费、时装和宠物邮件仍未开放。
- **2026-09-23 技能指令改键基线**：CMD331 自定义指令快照按角色 JSON 状态持久化，NOTI19 重建技能列表时覆盖对应技能的源指令；保存成功后立即刷新 NOTI19。实机确认保存后技能窗口即时更新，关闭重开及重选角色仍回显新键位。CMD332 全部重置语义仍待单独取证。详见 `work/dfo-lan/docs/protocol/next51-skill-command-customizing.md`。

- **归档基准服务**：`bin/wireprobe-dungeon39.exe` 为前一阶段验收通过的 39 版服务程序，已实机验证进城、装备显示、重登保留；日常测试默认以此为稳定基准。
- **源码候选服务**：`bin/wireprobe-handoff-source.exe` 为当前源码编译版（已补齐入城 NOTI14 装备外观刷新逻辑，通过 `go test` 与 `go vet`）。2026-09-20 实机确认：默认启动链用 `configs/characters.skycastle-release.json` 这一代角色目录时，建号会按 `option[8]` 落账转职，并按源 `[create equipment list]` 投影初始穿戴（见 `work/dfo-lan/docs/protocol/next45-creation-equipment.md`）。
- **技能锁（统一选项）基线**：CMD2377 接收侧（解析 + `character_skill_locks` 持久化 + 幂等整体替换）与 NOTI2827 推送侧（内置客户端 3539 字节角色选项块，两个 386 字节锁对象在偏移 2736/3122，作为入口帧序列**最后一帧**发送）均已落地；2026-09-21 实机确认：锁定后回角色选择重进**锁正常回显且不闪退**。`-unified-charac-template` / `-skill-lock-offset` 仅用于不同客户端版本。详见 `work/dfo-lan/docs/protocol/next46-unified-option.md`。
- **世界区域等级门禁（奥德赛）基线**：next47 落地——源 `[odyssey enter level]`（双标签区的奥德赛入区门槛，如风云径 43/* need50/odyssey45）由 `catalog/world.go` 解析为 `OdysseyEnterLevel`，全部入门校验（`ValidatePosition`/`Transition[Strict]`/`Enter` 与 6 个 cmd 调用点）按逐角色 `character.OdysseyRole` 模式判定（剧情角色用 `[need level]`；奥德赛角色取两者较小值——next49 实机更正：`[odyssey enter level]` 只降不升，40/0 西海岸 need15/odyssey35 时客户端对 20 级奥德赛角色放行且提示填 15）；78 个双标签区已逐行补丁进 `configs/world.generated.json` 的 `odyssey_enter_level` 扁平字段（`source.checksum` 7ef2db59… 不变、694 区不变）。2026-09-21 实机确认：45 级奥德赛角色传送 43/1 成功、城内位置上报全通过、零 `area_refused`，用户确认"修好了"。详见 `work/dfo-lan/docs/protocol/next47-odyssey-world-level.md` 与 `next49-odyssey-gate-min-semantics.md`。
- **交付工具**：`server/Build-Manifest.ps1` 重生成 `package-manifest.json` / `MANIFEST.sha256`（文本按 LF 归一化；`-WhatIfOnly` 只报差异）；`work/dfo-lan/cmd/initialrepair` 给修复前的角色一次性补 `option[8]` 转职落账与源初始穿戴（默认预览、`-apply` 落盘、幂等键 `creation-equipment-repair-v1`，仅限开发账号）。2026-09-20 实机确认：老角色修复后的穿戴与当天新建的同分支角色完全一致。
- **当前核心目标**：跑通主干玩法闭环（接任务 → 进图 → 战斗 → 拾取 → 通关 → 结算 → 回城 → 提交任务 → 奖励/升级 → 下一任务）。
- **未完成系统**：商城、邮件、装备分解、独立局域网账号登录器、多人共享战斗同步等属于后续独立子系统，严禁发送虚假通用成功包冒充实现。

## 1. 执行模型与技术栈

- **服务端主体**：Go 1.26（模块根目录位于 `server/work/dfo-lan/`，通过 `go.mod` / `go.sum` 管理依赖）。
- **服务启动编排**：Python 3.11.9 便携版（`tools/python/python.exe`），调用 `launch_local.py` 与 `channel_probe.py`。
- **数据持久化**：PostgreSQL 16.4 便携版（端口 25438），连接配置 `runtime/storage/local.json`，数据目录 `runtime/storage/pgdata/`。
- **缓存与会话**：Redis 5.0.14 便携版（端口 26388），纯内存配置 `runtime/storage/redis.conf`。

## 2. 服务入口与端点约定

| 端点 / 入口               | 作用                                                         |
| ------------------------- | ------------------------------------------------------------ |
| `127.0.0.1:7001`          | Channel 频道目录与刷新服务（HTTP / 专有协议）                |
| `127.0.0.2:<动态端口>`    | Game 游戏接入网关（TCP，由 probe 协同引导连接）              |
| 根目录 `启动游戏.cmd`     | 玩家与完整测试入口（需管理员权限，自动拉起存储、服务与客户端） |
| 根目录 `启动服务端.cmd`   | 纯服务端调试入口（调用 `launch_local.py --server-only`）     |
| 根目录 `停止游戏环境.cmd` | 安全关闭客户端、游戏服务、PostgreSQL (做 checkpoint) 与 Redis |
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
| `internal/storage/`             | PostgreSQL 数据库事务 (pgxpool)、角色存档持久化、Redis 会话  |
| `internal/catalog/`             | 游戏规则驱动目录与静态数据索引解析                           |
| `configs/`                      | 导出的全量 JSON 规则配置（任务、地图、装备、掉落等）         |
| `scripts/`                      | 本地启动与初始化脚本（`launch_local.py`、`bootstrap_local.py`） |
| `runtime/storage/`              | 本地存储集群：`pgdata/`、`redis.conf`、`local.json`（严禁入库） |
| `runtime/roles_*/`              | 运行会话追踪日志（`run.json`、`events.jsonl`、`helper.err`） |
| `reference/analysis-tools/*.py` | 分析辅助脚本                                                 |

## 4. 开发与构建规范

1. **测试先行**：修改协议或业务逻辑后，必须在 `server/work/dfo-lan/` 下执行：

   ```powershell
   go test ./...
   go vet ./...
   ```

2. **候选版隔离**：源码编译输出为 `bin/wireprobe-handoff-source.exe`。**严禁直接覆盖原 39 版归档基线 `wireprobe-dungeon39.exe`**。只有经过实机完整回归确认后，方可升级基准。
3. **验证候选版**：关闭已有游戏会话后，通过带参数启动测试源码候选版：

   ```powershell
   powershell -NoProfile -ExecutionPolicy Bypass -File ./Start-DFO.cmd --source-build
   ```

4. **数据库集成检查**：运行 `go run ./cmd/charactercheck` 检验角色存储与 schema 兼容性。

## 5. 变更事务与数据安全

1. **一次假设、一次 commit**：每次协议 A/B 测试、功能补齐或状态机修复单独 commit，不堆叠未提交改动。
2. **存档向后兼容**：PostgreSQL 角色数据是玩家核心资产，数据库变更必须支持已有角色无损升级，严禁随意删除已初始化的 `pgdata/`。
3. **工作区隔离与忽略规则**：
   - 严禁提交 `server/work/dfo-lan/runtime/storage/pgdata/`
   - 严禁提交 `server/work/dfo-lan/runtime/storage/*.log`、`local.json`
   - 严禁提交动态会话目录 `server/work/dfo-lan/runtime/roles_*/`
   - 严禁提交本地编译的中间文件或未授权的大型二进制
