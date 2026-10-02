# 服务端架构简化实施计划

状态：批次 1–10 的代码改动和自动化验证已完成；2026-10-01 继续合并装备事件公共流程和连接批量输出，等待用户手动实机验收。

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

截至批次 10 的当前统计（同一口径）：

| 项目 | 当前数量 | 相对基线 |
|---|---:|---:|
| 全部 Go 包 | 81 | -1 |
| `cmd/` 包 | 61 | 0 |
| `internal/` 包 | 20 | -1 |
| 生产 Go 文件 | 598 | -1 |
| 生产代码行（含注释） | 103,221 | -211 |
| 测试文件 | 513 | 0 |
| 内部直接依赖边（不含 `cmd`） | 56 | -1 |

本轮的收益不是追求数字最小化，而是删除一个未接线包、两个无调用 PVF 包装和三条由 `loot` 承担的非掉落职责入口，同时保留有实际规则的领域模块。

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

当前内部包存在 56 条直接依赖边（不含 `cmd`）。主要形状如下：

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

已完成的代码改动：

- 删除 `cmd/wireprobe/character_slot_flow.go`。
- 在 `main.go` 的 CMD295 分支直接执行选择界面 guard 和 `characters.ChangeSlot`。
- 保留原成功/拒绝响应、失败断连、无中途刷新和 `storage` 原子排列逻辑。

批次 2 验收：

- [x] `character_slot_flow.go` 删除，生产代码不再定义 `rosterSlotService` 或 `changeRosterSlot`。
- [x] CMD295 仍只有一个请求入口，且成功/拒绝/断连行为与基线一致。
- [x] `go test ./...` 通过。
- [x] `go vet ./...` 通过。
- [ ] 实机验证由用户手动执行：普通交换、固定栏位移动、非法请求拒绝和存储失败断连。

自动化验证在本批提交前完成；实机验收由用户手动执行后再更新本节和 CHANGELOG。若实机发现协议或客户端消费差异，下一次提交只记录一个新的证据和假设，不回退到增加转发接口。

## 5. 第三批：连接资源生命周期

状态：**代码已完成，等待用户实机验收**。

当前路径：

```text
handleClient
  -> done channel + clientFrames
  -> mailChanges / mailTicker
  -> daily ticker
  -> mine ticker
  -> optional Moon ticker
  -> 多个分散的 defer
```

目标路径：

```text
handleClient
  -> connectionSession
       -> reader stop signal
       -> mail change trigger and ticker
       -> daily / mine / optional Moon ticker
       -> idempotent close
```

本批新增 `connectionSession`，只拥有连接级资源的创建、停止和 reader 退出信号；`worldSession` 继续拥有副本、城镇、角色玩法状态。删除连接时需要理解的清理路径从多个局部 `defer` 收敛为一个所有者，重复关闭也不会 panic。

保持的行为：邮件 2 秒、日切 30 秒、矿区 1 秒、月湖 250 毫秒的触发间隔不变；发送顺序、世界状态退出和客户端帧读取协议不变。

批次 3 验收：

- [x] `done`、连接级 channel 和 ticker 由 `connectionSession` 创建及关闭。
- [x] 连接关闭重复调用安全，未启用月湖时不创建月湖 ticker。
- [x] `go test ./...` 通过。
- [x] `go vet ./...` 通过。
- [ ] 实机验证连接正常退出、异常断连和月湖开关两条路径。

## 6. 第四批：单包连接输出试点

状态：**代码已完成，等待用户实机验收**。

当前路径：

```text
main.go 的 sendPayload 闭包
  -> main.go 自己持有写锁、keys、deadline、preparePackets、writePackets
多个特殊分支
  -> 自己 EncryptPayload / ServerFrame / SetWriteDeadline / io.Copy
```

目标路径：

```text
connectionOutput
  -> 持有连接级写锁、session keys 和 peer 错误上下文
  -> 负责单包 preparePackets + writePackets
  -> 为必须保留原始帧日志的分支提供串行 writeRaw
```

本批把最常用的单包响应路径交给 `connectionOutput`，并将 CMD433/848 的预构建响应接入同一写锁。协议 payload 仍由调用方构造，整组入场预编码和逐包发送仍保留原路径，避免把两种客户端时序错误合并。

复杂度减少在于：单包响应不再同时理解连接写锁、密钥、编码、deadline 和写失败日志；特殊分支也不会绕过连接级串行写入。

批次 4 验收：

- [x] 单包响应由 `connectionOutput.send` 统一编码和写入。
- [x] CMD433/848 的原始响应日志仍保留，并通过同一连接锁写入。
- [x] 入场整组预编码和原有逐包输出路径保持不变。
- [x] `go test ./...` 通过。
- [x] `go vet ./...` 通过。
- [ ] 实机验证登录、选角、CMD433/848 和定时通知没有响应交错。

## 7. 第五批：角色背景恢复操作

状态：**代码已完成，等待用户实机验收**。

当前路径：

```text
main.go 的 sendRosterBackgrounds 闭包
  -> storage.RosterBackgrounds
  -> protocol.RosterBackgroundRestore
  -> sendPayload
```

目标路径：

```text
roster_background_flow.go:restoreRosterBackgrounds
  -> 读取账号背景状态
  -> 编码 NOTI1759
  -> 交给连接输出所有者发送
main.go
  -> 只在选角列表刷新后的三个入口调用该操作
```

本批把一个会随角色列表、背景选择和角色创建一起变化的完整操作放回背景功能文件。主循环不再了解背景存储表和 NOTI1759 的组装细节；连接写锁和编码仍由 `connectionOutput` 负责。

批次 5 验收：

- [x] `main.go` 不再定义背景恢复闭包。
- [x] 背景状态读取、协议编码和恢复事件由 `restoreRosterBackgrounds` 负责。
- [x] 背景券事务和角色列表刷新顺序未改变。
- [x] `go test ./...` 通过。
- [x] `go vet ./...` 通过。
- [ ] 实机验证登录恢复、背景选择后恢复和创建/删除角色后的列表刷新。

## 8. 第六批：任务目标持久化试点

状态：**代码已完成，等待用户实机验收**。

当前路径：

```text
quest.Service.MeetNPC
  -> quest.Service.Store.DB.Exec
       -> 直接理解 character_quests 表、角色归属、状态和进度模型
```

目标路径：

```text
quest.Service.MeetNPC
  -> storage.Store.MarkMeetNPCQuest
       -> 统一执行账号/角色归属和 accepted/version/model 条件
```

本批只移动 SQL 所有权。任务包继续负责目录匹配、NPC 目标校验和进度模型选择；存储包负责表结构、归属条件和影响行数语义。没有改变事务边界、schema、返回错误或进度值。

批次 6 验收：

- [x] `quest` 不再直接访问 `Store.DB` 完成 meet-NPC 更新。
- [x] SQL 条件、影响行数检查和错误文本保持一致。
- [x] `go test ./...` 通过。
- [x] `go vet ./...` 通过。
- [ ] 实机验证远程 NPC 任务和普通位置 NPC 任务的推进与拒绝路径。

## 9. 第七批：通用背包移动归 inventory

状态：**代码与自动化验证已完成，等待用户实机验收**。

当前路径：

```text
CMD19 quickslot_flow.go
  -> 复制 loot.Service
  -> loot.Service.MoveStack
  -> inventory.Bag.MoveStackRequest
  -> storage.Store.CommitCharacterEvent
```

目标路径：

```text
CMD19 quickslot_flow.go
  -> inventory.MoveStack
       -> inventory.Bag.MoveStackRequest
       -> storage.Store.CommitCharacterEvent
```

本批将普通背包堆叠移动的持久化操作归入 `inventory`。它不负责掉落来源、掉落会话或副本奖励；`loot` 继续拥有拾取和掉落规则。删除的复杂度是 `loot.Service` 不再为了一个通用背包操作被复制到调用方，也不再把背包目录、事件幂等和存档写入隐藏在掉落服务中。槽位校验、事件模型、回执内容、`InventoryRestore` 校验和 quickslot 回包保持不变。

批次 7 验收：

- [x] 删除 `loot.Service.MoveStack` 及其旧文件。
- [x] `inventory.MoveStack` 直接拥有普通背包移动的事件提交和回执。
- [x] quickslot 流程只负责识别请求、准备目录/规则和组装客户端回包。
- [x] 背包移动集成回归测试随操作移动到 `internal/inventory`。
- [x] `go test ./...` 通过。
- [x] `go vet ./...` 通过。
- [ ] 实机验证快捷栏放入、移出、堆叠合并和旧客户端重试行为。

## 10. 第八批：NPC 商店交易归 inventory

状态：**代码与自动化验证已完成，等待用户实机验收**。

当前路径：

```text
CMD21/CMD22 shop_flow.go
  -> worldSession.loot
  -> loot.Service.Buy/Sell
  -> inventory.Bag.Buy/Sell + storage.Store
```

目标路径：

```text
CMD21/CMD22 shop_flow.go
  -> worldSession.shop
  -> inventory.ShopService.Buy/Sell
       -> inventory.Bag.Buy/Sell
       -> storage.Store
```

本批把 NPC 金币/材料商店的报价解析、限购记录、支付、背包变更和幂等收据统一交给 `inventory.ShopService`。`loot` 不再持有商店目录、价格目录或材料目录，因此不会同时解释掉落和商店交易。Cera 购买继续由 `cashshop` 负责，因为它使用账号点券账本和独立的商品交付协议；两条支付路径不互相调用。

批次 8 验收：

- [x] 删除 `loot.Service.Buy/Sell` 及其旧商店文件。
- [x] `inventory.ShopService` 直接拥有 NPC 商店交易和事件模型。
- [x] `worldSession` 只保存专用的 NPC 商店服务，`loot.Service` 不再携带商店配置。
- [x] 买入、卖出、材料支付、并发超卖和跨重启事件键回归测试随商店所有者移动。
- [x] `go test ./...` 通过。
- [x] `go vet ./...` 通过。
- [ ] 实机验证 NPC 商店买入、卖出、材料支付和重复请求回放。

## 11. 第九批：删除未接线骨架和 PVF 空包装

状态：**代码与自动化验证已完成，等待最终启动/导入回归**。

当前路径：

```text
未接线的 internal/oath
  -> 只有包内测试，没有生产调用

pvf.Open / pvf.OpenBytes
  -> pvf.LoadArchive / pvf.OpenArchive
```

目标路径：

```text
已删除 internal/oath

所有 PVF 读取方
  -> pvf.LoadArchive
```

本批删除没有生产引用的 `internal/oath` 骨架，以及没有仓内调用的 `pvf.Open`、`pvf.OpenBytes` 包装函数。保留 `internal/storage` 中已经接入的 oath 选项/进度存储、`internal/inventory` 的装备规则和 `cmd/wireprobe` 的实际协议流程；保留 `pvf.LoadArchive`、`ReadScript`、`ResolveScript` 和 `pvfpatch` 的独立输出能力。删除后包图少一个未接线模块和两个重复入口，不改变服务端启动、协议、存档或 PVF 导入结果。

批次 9 验收：

- [x] `internal/oath` 没有生产调用，已整体删除。
- [x] `pvf.Open`、`pvf.OpenBytes` 没有仓内调用，已删除。
- [x] PVF 解析、catalog 导入和 `cmd/pvfpatch` 仍使用 `LoadArchive` 路径。
- [x] `go test ./...` 通过。
- [x] `go vet ./...` 通过。
- [ ] 实机验证不涉及本批删除；需在最终全量回归中确认启动和导入命令。

## 12. 批次索引与完成状态

每批合并前都要更新本文件；“已完成”只代表代码和自动化检查完成，实机状态单独标注。

| 批次 | 改动 | 依赖 | 状态 | 验收重点 |
|---|---|---|---|---|
| 01 | 职责基线与回归清单 | — | 已完成 | 基线统计、职责和依赖图可追查 |
| 02 | CMD295 角色栏位调整试点 | 01 | 已完成；待实机 | 选择界面 guard、拒绝码和原子保存保持不变 |
| 03 | 在现有运行包内收拢连接/会话生命周期 | 02 | 已完成；待实机 | 连接资源和清理只有一个运行所有者 |
| 04 | 整理连接输出、协议与编码组合 | 03 | 已完成；待实机 | 保留逐包流式发送和整组预编码两种行为 |
| 05 | 整理分发和功能入口，缩小 `main.go` 的协调范围 | 03、04 | 已完成；待实机 | 规则回到真实领域所有者，`game` 只做运行协调 |
| 06 | 明确 SQL、锁和事务意图 | 01，顺序上晚于 05 | 已完成；待实机 | 不破坏锁顺序、收据和存档兼容 |
| 07 | 将通用物品操作从 `loot` 收回 `inventory` | 05、06 | 已完成；待实机 | 不形成 `inventory -> cashshop -> inventory` 循环 |
| 08 | 明确 NPC/Cera/商城交易所有者 | 04、06、07 | 已完成；待实机 | NPC 归 `inventory`，Cera 仍归 `cashshop` |
| 09 | 删除无用 oath 骨架、PVF 无调用中间层和重复启动装配 | 06–08 | 已完成；待最终回归 | 启动装配与外部 `pgdata` 仍需回归 |
| 10 | 全量回归和长期依赖检查 | 01–09 | 自动化已完成；待实机 | 行为、存档、协议和依赖规则都有可重复检查 |

## 13. 第十批：全量回归和长期依赖检查

状态：**自动化检查已完成，等待用户手动实机验收**。

本批没有新增运行代码，只把前 1–9 批的结果收口为可重复检查，并更新当前规模和依赖边。目标是确认删除和归属调整没有留下隐式旧入口。

已执行并通过：

```text
go list ./...
go test ./...
go vet ./...
git diff --check
```

已执行的残留检查：

- `internal/oath` 不再存在，也没有 Go 生产引用。
- `pvf.Open`、`pvf.OpenBytes` 不再存在或被调用；PVF 读取统一走 `LoadArchive`。
- `loot.Service.Buy`、`loot.Service.Sell`、`loot.Service.MoveStack` 不再存在；NPC 商店和普通堆叠移动分别由 `inventory.ShopService`、`inventory.MoveStack` 负责。
- 当前内部直接依赖边为 56 条，包图没有新增 `inventory -> cashshop -> inventory` 环。

仍需用户手动验收的行为：

- 登录、选角、断线和月湖开关下的连接资源清理与通知顺序。
- CMD295 栏位交换、背景恢复，以及 CMD433/848 的连接输出不交错。
- MeetNPC 任务推进、快捷栏堆叠移动、NPC 买卖和重复请求回放。
- 服务端启动、PVF 导入、已有 PostgreSQL 存档读取和重启后事件恢复。

完成这些实机项目后，再由用户明确确认是否更新 `CHANGELOG` 和 confirmed baseline；本批不自行宣称实机验收通过。

## 14. 每批 MR 模板

```text
标题：refactor(server): <一个批次的单一架构目标>

问题：当前哪个调用路径重复、转发或隐藏了所有权。
改动：本批具体删除/移动/合并了什么。
保持：协议、存档、锁顺序、失败响应和实机行为。
验证：go test ./...；go vet ./...；必要的集成/实机证据。
风险与回滚：本批独立回滚方式，以及依赖它的后续批次。
```

每次推送前执行 `git status --short`，只提交本批文件；不要把下一批的预备性重命名或兼容别名混进来。


## 15. 2026-10-01：合并实际重复流程

本批基线：`4a3e67c`。目标是删除重复实现，而非仅移动文件。

- 附魔、增幅书、增幅券、材料增幅、锻造、装备继承和固定强化券共七条路径，统一使用包内 `commitEquipmentEvent` 完成角色事件提交、收据 JSON 编码、持久化收据读取及 WireID 恢复。
- 请求 guard、业务算法、事件 key/model 和收据结构保留；强化券仍检查原收据与请求一致。重放依然读取数据库中的旧收据，不重新执行随机结果或扣料。
- 金币强化的账号材料事务、商店交易及其他不同事务没有接入该函数。
- `main.go` 的 13 处预编码批量输出和四处原始帧输出接入现有连接输出所有者；删除 11 处重复 deadline 设置。编码时机、包顺序和逐包日志回调保留，批量、单包、原始帧共享连接写锁。
- 新增收据重放、提交/业务/编码/读取/解码失败测试，以及批量输出与原始帧并发时的字节顺序和回调顺序测试。

生产 Go 代码净减少 **80 行**（相对本批基线，含新增公共函数，不含测试及文档）；七处事件流程收敛为一处。该数字与移动文件分别统计，不宣称大规模精简已经完成。

验证：`go test ./...`、`go vet ./...`、`git diff --check`。实机仍待用户手动验证，未部署二进制，未升级 confirmed baseline。无数据库 schema、存档格式、协议布局、客户端或 DLL 改动。

后续候选：快捷栏的背包依赖仍借用 loot，需结合真实装配及目录覆盖关系单独整理；booster/皮肤流程暂未证明可共用同一事务和回包语义，不强行合并。本批先落实重复已确认的前两项。


## 16. 与原生 PVF 主干合并

目标主干：`b1c9d32`。两处文本冲突已按职责合并：

- `main.go` 保留主干 `pvfCatalogs.prices != nil` 启用条件和 `pvfCatalogs.loadShopPrices`，价格写入本分支的 `inventory.ShopService`，不恢复已删除的 `loot.Service.Prices`。
- `CHANGELOG` 同时保留原生 PVF 默认启动确认记录与本分支精简候选记录。
- 自动合并的 PVF archive 保留主干迭代/缓存管理能力，继续使用 `LoadArchive`，不恢复无调用旧包装。

合并不发布或替换已确认的默认程序，不新增玩家存档迁移；主干已有 PVF confirmed baseline 沿用，精简候选不据源码合并升级为实机确认。验证包括全量 Go test/vet、默认启动及频道身份10项Python测试和暂存差异检查。

## 16. MR !123 整合至 !126

2026-10-01 将 `refactor/server-flow-simplification` 合入精简候选，保留两个分支历史。重叠装备查找、刷新与事务收据采用 !126 实现；纳入 !123 的逐包发送、公共行构造、会话事件键和解密白名单整理，保留双方回归测试。单包、预编码批量、原始帧继续使用 !126 的统一连接锁。全量 `go test ./...`、`go vet ./...` 通过；未部署、未启动客户端、未操作玩家数据库，confirmed baseline 不扩大。

## 17. 2026-10-02：MR !127 消除领域到持久化的依赖

本批起点为 `5b6a868`，已包含上游 `c80c432`。目标是改变所有权和调用方向，保持现有功能行为。

- E11：Character、栏位、疲劳状态归 character；角色、进度、疲劳服务使用领域消费接口，SQL 查询移入 storage 原样适配。typed nil 接口仍按旧构造函数返回 nil。
- E12：装备操作、NPC 买卖和金库事务归 workflow；inventory 保留纯规则、角色消费投影、金库状态。PremiumReader 保留原查询、两秒超时和忽略查询错误的行为。
- E13：拾取、翻牌、黑鸦/月湖奖励恢复、分解、装备制作/变换、物品使用及礼盒修复的持久化编排归 workflow；loot 保留 Plan/Prepare/Apply 规则和角色投影。恢复查询原样移入 storage，事务键、model、收据校验和错误回滚顺序保留。
- E14：任务状态和 Store 接口归 quest；完成、清任务、主线、寻物与库存进度事务归 workflow，奖励公式经消费回调接入原实现。Premium 回调仍在原奖励计算阶段调用。
- E16：现金订单/回执类型归 cashshop，storage 保留类型别名；现金金库购买跨领域编排归 workflow。
- E24：legion 入口消费校验函数，不再直接引用 dungeon。
- 另消除 quest→progression、loot→adventure/cashshop。守卫允许清单只减不增；剩余 E21 的 4 条、E22 的 3 条、E23 的 2 条和 E25 的 1 条，共 10 条领域间依赖。这些包含实际背包计算、场景状态和角色规则，仍需逐条拆分，不能据此宣称 MR 的全部目标完成。

验证使用 `GOTOOLCHAIN=go1.26.0`：全仓编译、`go vet ./...`、依赖守卫和相关领域测试通过。全量 `go test ./...` 保留以下 5 项既有失败；本批此前以起点源码隔离复现相同断言，仅补齐起点未声明的库存接口类型以使其可编译：

- `cmd/wireprobe.TestAdventureAuditProvenanceAllowanceIsNarrow`
- `cmd/wireprobe.TestPVFCatalogGateRefusesRewardChangesAndDoesNotFallback`
- `cmd/wireprobe.TestEnhancementAuditAllowsOnlyMissingOrdinaryTicketExpirationHeader`
- `internal/loot.TestOdysseyChapterFinalLordDrop`
- `internal/loot.TestOdysseyCurrencySceneRetryAndPoolIsolation`

未修改这些测试断言或对应业务实现。数据库集成测试沿用已有显式启用门禁，本批没有连接玩家 PostgreSQL。实机仍待用户手动验收：装备操作及重复请求、NPC 买卖与金库转移、任务完成/奖励、拾取和翻牌、黑鸦/月湖重选角色恢复。没有部署二进制或启动客户端，不升级 confirmed baseline。

用户原有 `.gitignore`、PostgreSQL/Redis 配置与 `scripts/launch_local.py` 内容保持不变，不纳入本批提交。

## 18. 合入最新上游并保持边界

完成第 17 节后保存重构提交 `e5784cf`，再次 fetch 发现上游已推进到 `9fa5bfe`。本批合入其完整历史，保留结算 ACK、商城 CERA/金币与身份修复、魔法封印解除、装备调适、时装分解/徽章重铸和对应测试；只适配调用与所有权，不另外修改上游功能行为。

新增的装备调适、时装徽章重铸与分解事务仍归 workflow，领域保持 storage-free。校验、随机调用、事务、收据读取和响应构造的先后顺序按上游保留；商城订单新增的金币字段和校验保留，上游存档身份修复同步应用到此前迁移的工作流。

合并后 Go 1.26 全仓编译、`go vet ./...`、架构守卫、身份口径守卫通过。最终 `go test ./...` 仅剩第 17 节中的 3 项 wireprobe 既有失败，其他包通过；上游修复使原来的 2 项 loot Odyssey 测试恢复通过。装备迁移收尾后另跑 inventory/workflow 定向测试与全仓 vet，均通过；最终删除一处未使用的中间 helper 后再通过 vet。`git diff --check` 通过。

本分支未部署或启动客户端，未操作玩家数据库。上游已有 confirmed 记录随合并保留，不把本批边界重构扩大为实机已验收。仍剩 10 条领域间依赖，后续范围见第 17 节。

最终远端复核又发现上游新增 `7d4f5c0`（时装皮肤开孔修复），继续合入该 11 文件更新。开孔事务按同样边界迁至 workflow，原协议、道具规则、时间戳位置、事件 key/model、收据与回包顺序保留。最后一次全仓编译、vet、架构/身份守卫通过；全量测试仍仅上述 3 项 wireprobe 既有失败，其余通过。四个用户原有改动再次核对内容哈希一致，未纳入提交。

上游持续更新，本轮同步最终截止 `647c3fd`（徽章合成修复），合入其完整历史并将新增合成事务适配进 workflow。领域、道具规则及协议与上游一致；随机调用仍只发生在事务回调，事件键的 JSON 字段顺序、摘要、model、收据 DeepEqual 检查和 ACK/NOTI13 顺序保留。适配完成后再次全仓编译、vet、架构/身份守卫及全量测试，仍仅上述 3 项 wireprobe 既有失败，其余包通过；差异检查通过。继续保留 10 条领域间例外，本 MR 整体尚未完成。

## 19. 领域归并与文件收口

MR !127 已合并，本批从 main `052532c` 开始，分支 `refactor/domain-consolidation`。用户要求先按职责收拢领域和文件，再处理剩余跨领域依赖。

- inventory：装备制作/变换/分解的规则、计划与回执从 loot 迁入同一装备图鉴操作文件；时装分解/开孔、徽章合成/镶嵌的服务准备逻辑并入现有对应 Bag 规则文件，不保留 loot 转发门面。
- character：经验、任务奖励与通关成长公式归入 growth_rules/growth_rewards；资料皮肤状态与恢复合并，账号选角背景状态/报文与目录/PVF 投影合并。删除 progression、profileskin、rosterbg 三个目录，背景券嵌入数据保持同字节。
- workflow：7 个物品操作事务归 ItemService，按装备与时装/徽章合并为 2 文件；奖励事务、礼盒事务与背包移动/排序分别归并。cmd 的时装/徽章分发也合并，main 注入独立 ItemService，并在监听前同步最终 Catalog/BagRules/Equipment，保留上游目录扩充的装配结果。
- 架构契约与守卫同步：删除 character→progression 允许边，剩余 9 条。纠正 legion 的职责记录为军团/末世录请求、阶段门控与入场计划；它与 NPC 相位各有独立状态职责，保留目录。

Go 文件由 1437 减至 1408，其中生产文件 795→772，测试文件 642→636。inventory 迁入的 41 个操作函数、成长的 14 个公式函数和归并的 14 个事务函数，经标识归一化后的词法比对保持一致。装备迁移测试保留 17 项测试和 101 个断言调用，其他测试随领域迁移保留断言。SQL、schema、协议布局、JSON 存档字段、事件 key/model 和随机调用点未改。

验证使用 Go 1.26：全仓 vet、架构守卫、存档身份守卫与迁移领域/工作流测试通过。全量 go test ./... 留下 4 项既有失败：

- cmd/wireprobe.TestAdventureAuditProvenanceAllowanceIsNarrow
- cmd/wireprobe.TestPVFCatalogGateRefusesRewardChangesAndDoesNotFallback
- cmd/wireprobe.TestEnhancementAuditAllowsOnlyMissingOrdinaryTicketExpirationHeader
- internal/cashshop.TestShopPilotPVFCurrentCatalog

前三项为第 17/18 节已有失败；商城项在本批起点 052532c 的原始源码隔离目录重现同一 pvf_catalog_test.go:194 / empty delivery 3400013 断言。隔离目录仅提取起点 Go 源码与嵌入文件，通过 configs junction 使用同一只读配置；未改 baseline 源码或断言。其余包通过，没有新增测试失败。git diff --check 通过。

后续先梳理 loot 的消耗/礼盒奖励混合职责，再评估 character/quest/loot/cashshop 的剩余 9 条依赖；不通过复制规则或新增短转发文件消除统计边。没有连接玩家库、部署二进制或启动客户端，不扩大 confirmed baseline。四个用户原有配置改动保持原哈希，不纳入提交。
