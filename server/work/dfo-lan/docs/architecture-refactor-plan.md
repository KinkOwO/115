# 服务端架构简化实施计划

状态：批次 1 基线已记录；批次 2 待实施。  
基线提交：`9fc9a0617d3fc16fa43f8bb4bc1fcd31be4c7990`（2026-09-30）  
当前分支：`refactor/go-server-dedup-20260930`

这份文档是后续架构改动的持续交接文档。它记录当前真实职责、依赖方向、每批改动的边界和验收条件。每完成一批，更新本文件的状态、提交号、验证结果和仍然存在的风险；不要把未验证的目标架构写成已经完成。

## 1. 改动约束

- 保持现有客户端行为、协议字段和玩家存档兼容。
- 只改 Go 服务端；不通过客户端 DLL 或补丁弥补服务端缺口。
- 一次只推进一个可回滚的批次；每批独立提交并推送。
- 不把运行时数据库、日志、角色会话目录或本地二进制放进提交。
- 修改协议或业务后，在 `server/work/dfo-lan/` 执行 `go test ./...` 和 `go vet ./...`。
- 真实客户端只能由用户手动操作。自动化测试通过不等于实机验收通过。
- 有依赖的批次只能在前置批次合并后推进；回滚按依赖的逆序进行。

## 2. 当前基线

### 2.1 规模

以基线提交 `9fc9a06` 统计：

| 项目 | 数量 |
|---|---:|
| Go module | 1（`dfolan`） |
| 全部 Go 包 | 82 |
| `cmd/` 包 | 61 |
| `internal/` 包 | 21 |
| 生产 Go 文件 | 599 |
| 生产代码行（含注释） | 103,432 |
| 测试文件 | 513 |
| `cmd/wireprobe/main.go` | 5,671 行 |

这些数字是定位复杂度的基线，不是重构必须达到的配额。删除一个包只有在职责更集中、依赖更简单时才算收益。

### 2.2 主要职责

| 模块 | 当前实际职责 | 本轮原则 |
|---|---|---|
| `cmd/wireprobe` | 进程启动、连接读写、请求分发、各功能流的运行编排 | 保留运行协调；逐步删除纯转发和重复装配 |
| `internal/game/protocol` | 客户端协议请求/通知的字段编码解码 | 保留协议事实，不把业务规则塞进 codec |
| `internal/game/wire` | 加解密、帧、校验和、底层线格式 | 保留传输格式实现，不新增同义传输层 |
| `internal/character` | 角色创建、列表、角色状态、技能、成长和角色相关操作 | 角色规则与角色操作的主要所有者 |
| `internal/inventory` | 背包、穿戴、通用物品状态和装备操作 | 通用物品操作的主要所有者；后续从 `loot` 收回交叉职责 |
| `internal/loot` | 掉落生成、掉落实例、拾取和掉落去重 | 保留掉落来源与拾取准入，不包揽全部物品操作 |
| `internal/dungeon` | 副本会话、房间、门、清场和结算 | 保留副本状态机和时序 |
| `internal/world` | 城镇、区域跳转、传送和位置保存 | 保留场景几何和转移规则 |
| `internal/quest` | 任务链、目标推进、任务奖励 | 保留任务规则；事务意图后续显式化 |
| `internal/storage` | PostgreSQL/Redis、存档、锁、事务和缓存 | 保存数据与原子性所有者，不解释玩法字符串 |
| `internal/catalog` / `catalog/pvf` | 规则目录和 PVF 解析 | 保留来源、解析和快照边界 |
| `internal/cashshop` | 商城报价/购买试点和支付账本 | 后续明确是否扩展为交易所有者，避免万能支付层 |
| `internal/game/profileskin` 等小模块 | 共享协议/存储数据或诊断读模型 | 只有删除后依赖更清楚才合并，不做按数量清理 |

### 2.3 当前依赖方向

当前内部包存在 57 条直接依赖边（不含 `cmd`）。主要形状如下：

```text
cmd/wireprobe
  -> character / inventory / loot / quest / dungeon / world / cashshop
  -> game/protocol -> profileskin / rosterbg / adventure
  -> game/wire
  -> storage

character -> storage, inventory, progression, dungeon, catalog, protocol
quest     -> storage, character, inventory, dungeon, progression, catalog, protocol
loot      -> storage, inventory, cashshop, dungeon, catalog, protocol
storage   -> adventure, profileskin, rosterbg
```

这不是要求把所有箭头排成纯单向分层；游戏规则之间确实存在协作。后续只消除没有业务含义的转发、隐式事务和反向解释，不把有实质行为的深模块强行搬进 `game`。

## 3. 第一批：职责基线与回归清单

状态：**已完成**（本文件）。

本批建立了以下可复查基线：

- 记录包、文件、代码量和内部依赖数量。
- 记录各主要模块的实际职责，而不是按 `network/protocol/transport/service` 等名称猜职责。
- 保留当前协议、存档和运行约束。
- 确定后续批次必须遵守的单批提交、测试、推送和回滚规则。

批次 1 的验收：

- [x] 文档位于仓库 `server/work/dfo-lan/docs/`，可随代码一起评审。
- [x] 基线提交、分支和统计口径已写明。
- [x] 现有职责与依赖方向有具体文件范围可追查。
- [x] 后续批次有明确入口、依赖和行为验收条件。

## 4. 第二批：CMD295 角色栏位调整试点

当前代码路径：

```text
CMD295
  -> cmd/wireprobe/main.go
  -> character_slot_flow.go:rosterSlotService（只做 selected-character guard + 转发）
  -> character.Service.ChangeSlot
  -> protocol.DecodeCharacterSlot
  -> storage.Store.ChangeCharacterSlots
```

目标路径：

```text
CMD295
  -> cmd/wireprobe/main.go（会话条件、成功/拒绝应答、断开决策）
  -> character.Service.ChangeSlot（角色操作）
  -> protocol.DecodeCharacterSlot
  -> storage.Store.ChangeCharacterSlots（锁、排列、原子保存）
```

本批只删除没有独立行为的 `rosterSlotService` 和 `changeRosterSlot` 转发层，把调用直接放回已有的 CMD295 分支。角色规则仍归 `character`，排列算法和存档事务仍归 `storage`。

必须保持的行为：

- 只有角色选择界面允许修改栏位；已选角色请求继续拒绝。
- 密文、校验和或字段解码失败继续走原拒绝路径。
- 存储操作失败继续发送原拒绝码 19，并结束当前连接，避免客户端本地栏位领先存档。
- 成功只发送原成功字节，不在多格拖拽过程中刷新角色列表。
- 角色 ID、固定栏位、排序和整组原子保存逻辑不改。

批次 2 验收：

- [ ] `character_slot_flow.go` 删除，生产代码不再定义 `rosterSlotService` 或 `changeRosterSlot`。
- [ ] CMD295 仍只有一个请求入口，且成功/拒绝/断连行为与基线一致。
- [ ] `go test ./...` 通过。
- [ ] `go vet ./...` 通过。
- [ ] 实机验证由用户手动执行：普通交换、固定栏位移动、非法请求拒绝和存储失败断连。

## 5. 后续批次

每批合并前都要更新本文件；下表中的“完成”只有在代码、自动化检查和必要的实机证据齐全后才能勾选。

| 批次 | 改动 | 依赖 | 验收重点 |
|---|---|---|---|
| 03 | 在现有运行包内收拢连接/会话生命周期 | 02 | 账号、角色、频道、密钥、计时器和清理只有一个运行所有者 |
| 04 | 整理连接输出、协议与编码组合 | 03 | 保留逐包流式发送和整组预编码两种已验证行为 |
| 05 | 整理分发和功能入口，缩小 `main.go` 的协调范围 | 03、04 | 规则回到真实领域所有者，`game` 只做运行协调 |
| 06 | 明确 SQL、锁和事务意图 | 01，顺序上晚于 05 | 不破坏账户/角色锁顺序、收据和存档兼容 |
| 07 | 将通用物品操作从 `loot` 收回 `inventory` | 05、06 | 不形成 `inventory -> cashshop -> inventory` 循环 |
| 08 | 明确 NPC/Cera/商城交易所有者 | 04、06、07 | 报价、扣款、交付和提交前编码在一个可理解的交易路径中 |
| 09 | 删除无用 oath 骨架、PVF 无调用中间层和重复启动装配 | 06–08 | 直接 GM 启动路径和外部 pgdata 策略保持不变 |
| 10 | 全量回归和长期依赖检查 | 01–09 | 行为、存档、协议和依赖规则都有可重复检查 |

## 6. 每批 MR 模板

```text
标题：refactor(server): <一个批次的单一架构目标>

问题：当前哪个调用路径重复、转发或隐藏了所有权。
改动：本批具体删除/移动/合并了什么。
保持：协议、存档、锁顺序、失败响应和实机行为。
验证：go test ./...；go vet ./...；必要的集成/实机证据。
风险与回滚：本批独立回滚方式，以及依赖它的后续批次。
```

每次推送前执行 `git status --short`，只提交本批文件；不要把下一批的预备性重命名或兼容别名混进来。
