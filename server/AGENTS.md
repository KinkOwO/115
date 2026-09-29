# AGENTS.md — server/

> 本文件是 `server/` 目录的规则真源与子索引。先读仓库根 `AGENTS.md`，再读本文件。
> 服务端源码位于 `server/work/dfo-lan/`，探针与隔离工具位于 `server/work/dfo_probe_tools/`。

## 0. 当前候选边界

- **2026-09-29 冒险团及特殊副本候选**：冒险团信息、成长、商店、角色设置、推荐计数与迷雾誓约已接入；矿区和黑鸦小队接入频道、准入、编队或建队、入场与奖励存档。黑鸦入场、通关及自动翻牌材料领取已有本地实机证据；精锐真实资料加载已接线，随行战斗未完整验收。玩法信息页暂缓，矿区重新探索及图鉴碎片未接入。领主史诗10%、神话0.1%、腐蚀产物1%为本服暂定值。上游移植保留装备图鉴、设置、普通直达和物品期限行为；不包含汉化MOD、自动拾取、运行数据库及测试账号调整。

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
