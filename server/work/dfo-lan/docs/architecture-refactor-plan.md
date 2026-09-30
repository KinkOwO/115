# 服务端架构简化实施计划

状态：批次 1、2、3、4、5、6、7、8、9、10 的代码改动和自动化验证已完成，等待用户手动实机验收。

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
