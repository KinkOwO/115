# 服务端依赖契约（Architecture Contract）

状态：生效中（契约基线）
建立日期：2026-10-01
基线提交：`8e627f5`（`main`，MR !126 合并后；初版建立于 `939c749`）
适用范围：`server/work/dfo-lan/**` 的全部 Go 包

---

## 0. 本文的地位

- 本文是**依赖方向与所有权**的可判定真源。
- `docs/architecture-refactor-plan.md` 是历史进度与逐批交接记录，不是规则真源；两者冲突时以本文为准。
- 本文的规则必须能被自动化检查，见 §6「强制机制」。任何新增的反向依赖都应让检查失败，而不是靠评审记忆。

> 背景：在本文生效前，依赖规则只是口头约定（"消除没有业务含义的转发"），其中 `domain -> storage`、`storage -> domain`、`protocol -> domain` 都被当作可接受的现状。规则不可判定，导致历次精简后依赖形状仍然收敛不了。本文把规则改成**可机械判定**的版本。

---

## 1. 分层

| 层 | 名称 | 包 | 特征 |
|---|---|---|---|
| L0 | 传输原语 | `internal/game/wire` | 帧、加解密、校验和；不含任何游戏事实 |
| L1 | 协议与静态数据 | `internal/game/protocol`、`internal/catalog`、`internal/catalog/pvf`、`internal/derivedcache`、`internal/savecontract` | 字节布局、PVF 归档原语、规则驱动静态目录、磁盘缓存与存档契约原语 |
| L2 | 持久化 | `internal/storage` | SQL、事务、存档读写；游戏事实的搬运者和实现者，不是拥有者 |
| L3 | 业务领域 | `internal/{character,inventory,loot,quest,dungeon,world,cashshop,adventure,legion,npcpresence}` | 拥有各自的游戏规则与状态 |
| L4 | 组合与工具 | `cmd/**`、`internal/{gamedata,managementdata,admin,channelrefresh,workflow}` | 组合根、只读投影、管理、离线工具；`workflow` 承载跨领域编排与事务 |

---

## 2. 依赖规则

### R1 纯基础设施不得依赖领域
`internal/game/wire`、`internal/game/protocol`、`internal/catalog`、`internal/catalog/pvf`、`internal/derivedcache`、`internal/savecontract` **禁止** import 任何 L3 领域包。
理由：协议只描述字节布局，PVF 只描述归档与静态数据原语；一旦它们认识业务类型，布局就会随玩法漂移。

### R2 领域不得依赖持久化实现
L3 领域 **禁止** import `internal/storage`。领域需要的存储能力，由**领域自己声明接口**（如 `character.Store`），由 `storage` 实现，并在 `bootstrap` 注入。
理由：依赖倒置。领域认识具体持久化实现，等于把存储引擎锁死进业务层（也是换库时爆炸半径失控的根因）。

### R3 持久化可实现领域接口，但不得定义游戏事实
`internal/storage` **允许** import 领域包以**实现**领域声明的接口、映射领域类型；但**不得**解释玩法规则、不得成为某条游戏规则的拥有者。
理由：适配器方向是允许的（实现方依赖被实现方），但事实的定义权必须留在领域。

### R4 领域之间的协作必须显式
L3 领域之间 **默认禁止**互相 import。需要另一领域能力时，二选一：
1. **消费者声明小接口**，由提供者在 `bootstrap` 注入；或
2. 把流程放入跨领域编排层（目标 `internal/workflow`，见 §5）。

例外必须登记进 §7 并按计划消除。同一领域的状态与规则先归回领域所有者，避免为了分文件而新建领域或接口。

### R5 禁止依赖环
任意两包之间不得存在互相 import 的环。

### R6 组合与工具层不承载游戏规则
`cmd/**`、`gamedata`、`managementdata`、`admin`、`channelrefresh` 可以 import 任意包；但**不得**成为某条游戏规则的唯一实现。离线导入/审计工具可以依赖领域，领域不得反向依赖它们。

---

## 3. 判断新代码落位的唯一标准

> **这段代码是否需要知道"某个具体游戏事实"？**
> 是 → 属于对应 L3 领域。
> 否（只是字节、文件格式、SQL、事务、装配、投影）→ 属于对应的基础设施或组合层。

不得按 opcode、调用方或临时实现文件名建目录。

---

## 4. 包所有权表

| 包 | 层 | 拥有者（职责） | 允许 import | 禁止 import |
|---|---|---|---|---|
| `internal/game/wire` | L0 | 帧、加解密、校验和 | 仅标准库 | 任何 `internal/*` |
| `internal/game/protocol` | L1 | 协议请求/通知的字段编解码 | `game/wire`（如需） | 任何 L3 领域 |
| `internal/catalog/pvf` | L1 | PVF 归档、token、列表、路径原语 | 仅标准库 | 任何 `internal/*` |
| `internal/catalog` | L1 | 规则驱动静态目录与索引 | `catalog/pvf` | 任何 L3 领域 |
| `internal/derivedcache` | L1 | 磁盘派生缓存原语（哈希/失效/读写） | 仅标准库 | 任何 `internal/*` |
| `internal/savecontract` | L1 | 存档契约版本与身份（与客户端资源解耦） | 仅标准库 | 任何 `internal/*` |
| `internal/storage` | L2 | SQL、事务、锁、存档；实现领域声明的接口 | L3 领域（仅为实现接口）、`catalog`、`catalog/pvf` | 定义游戏规则 |
| `internal/character` | L3 | 建角、列表、角色状态、技能、经验与奖励成长、资料皮肤与账号选角背景 | `game/protocol`、`catalog`、`catalog/pvf`、自声明接口 | `storage`、其他领域（§7 例外除外） |
| `internal/inventory` | L3 | 背包、穿戴、通用物品状态、装备图鉴制作/变换/分解、时装与徽章操作、消耗品/宠物喂养与光辉礼盒目录/开启/奖励修复 | 同上 | 同上 |
| `internal/loot` | L3 | 掉落生成、掉落实例、拾取、去重 | 同上 | 同上 |
| `internal/quest` | L3 | 任务链、目标推进、任务奖励 | 同上 | 同上 |
| `internal/dungeon` | L3 | 副本会话、房间、门、清场、结算 | 同上 | 同上 |
| `internal/world` | L3 | 城镇、区域跳转、传送、位置保存 | 同上 | 同上 |
| `internal/cashshop` | L3 | 商城报价、购买、账号点券账本 | 同上 | 同上 |
| `internal/adventure` | L3 | 冒险团/成长规则 | 同上 | 同上 |
| `internal/legion` | L3 | 军团/末世录请求状态、阶段门控与入场计划；共享运行由 partyrun 持有 | 同上 | 同上 |
| `internal/npcpresence` | L3 | NPC 相位/可见性规则 | 同上 | 同上 |
| `internal/gamedata` | L4 | 全量静态数据只读投影/装配 | L1–L3 | 被 L3 依赖 |
| `internal/managementdata` | L4 | 管理端只读目录投影 | L1–L3 | 被 L3 依赖 |
| `internal/admin` | L4 | GM/管理操作 | L1–L3 | 被 L3 依赖 |
| `internal/channelrefresh` | L4 | 频道目录刷新 | L1–L3 | 被 L3 依赖 |
| `internal/workflow` | L4 | 跨领域编排与事务（持有 storage 句柄，调用领域纯逻辑） | L1–L3、`storage` | 被 L3 依赖；承载游戏规则（只编排，不定事实） |
| `cmd/wireprobe` | L4 | 组合根 + 请求分发（目标；当前仍含运行编排） | 全部 | 承载游戏规则 |
| 其余 `cmd/*` | L4 | 离线导入/审计/修复/检查工具 | 全部 | 成为运行时依赖 |

---

## 5. 目标形态（阶段性）

1. `cmd/wireprobe` 的 65 个 `*_flow.go` 中的跨领域编排，收敛到目标 `internal/workflow`；`main.go` 收敛为组合根 + 分发。**已起步（2026-10-01）**：新增 `internal/workflow`，首个切片为快捷栏堆叠移动（`workflow.MoveStack`），由 workflow 持有事务、调用 inventory 的纯背包变换。
2. 领域存储能力全部改为「领域声明接口 + bootstrap 注入」。
3. `game/protocol` 与 `catalog` 不再认识任何领域类型。

以上均为目标，未完成前按 §7 例外清单管理，不得写成已完成。

---

## 6. 强制机制

**已实现**：`internal/archtest/contract_test.go` 的 `TestDependencyContract`，纳入 `go test ./...`。

- **数据源**：用标准库 `go/build` 遍历 `internal/` 与 `cmd/`，解析每个包的非测试 import；不引入新依赖。
- **规则**：
  - 对 §2 R1 的每个包，断言其 import 中不含任何 L3 领域包。
  - 对每个 L3 领域包，断言其 import 中不含 `internal/storage`。
  - 对每个 L3 领域包，断言其与其他 L3 领域的 import 边全部出现在**允许清单**中。
  - 断言不存在 import 环（由各规则组合保证）。
- **允许清单 = §7 的例外清单**。测试对每条例外 fail；例外消除后必须同步删除对应条目（否则 §6 的"过期条目"检查会失败）。**只减不增**：新增例外必须改本文并留评审记录。

---

## 7. 例外清单（baseline，只减不增）

以下为 `8e627f5` 时点的现存违例（初版基于 `939c749`；合并 MR !126 后由守卫复核，补齐 `cashshop → inventory`）。它们当前**允许存在**，但必须按「目标」列逐步消除。任何**新增**违例不得加入本表，应直接按 §2 修正。

### 7.1 纯基础设施 → 领域（违反 R1）

**已全部消除（2026-10-01，本分支）**：E01–E04 已按下列目标移回领域，`internal/game/protocol` 与 `internal/catalog` 不再 import 任何 L3 领域；`archtest` 允许清单中的对应条目已删除。

| # | 边 | 处理 |
|---|---|---|
| E01 | `game/protocol` → `adventure` | `SeasonLevelHistory`/`SeasonOathHistory` 移入 `internal/adventure/season_wire.go` |
| E02 | `game/protocol` → `profileskin` | 已消除；资料皮肤状态与恢复现归 `character/profile_skin.go` |
| E03 | `game/protocol` → `rosterbg` | 已消除；背景状态、选择/恢复/解码现归 `character/roster_background.go` |
| E04 | `catalog` → `rosterbg` | 已消除；背景 PVF 投影现归 `character/roster_background_catalog.go` |

### 7.2 领域 → 持久化（违反 R2）

**已全部消除（2026-10-02，本分支）**：五个领域的生产代码均不再 import `internal/storage`，对应守卫允许条目全部删除。领域声明自己的消费接口或状态投影；跨领域事务由 `workflow` 持有持久化实现，`storage` 通过类型别名和适配器实现领域契约，不引入共享 model 包。

| # | 边 | 目标 |
|---|---|---|
| E11 | `character` → `storage` | **已消除（2026-10-01，本分支）**：角色、疲劳和栏位类型归 `character`，领域声明窄 `Store` 接口，`storage` 以别名和适配方法实现 |
| E12 | `inventory` → `storage` | **已消除（2026-10-01，本分支）**：装备、商店和金库事务迁入 `workflow`；inventory 只接收消费投影，金库状态归 inventory、storage 以别名兼容 |
| E13 | `loot` → `storage` | 已消除：事务、收据与恢复编排迁入 `workflow`；掉落领域只保留规则和角色投影 |
| E14 | `quest` → `storage` | **已消除（2026-10-01，本分支）**：quest 声明 Store 接口并拥有任务状态类型；storage 以别名实现，位置检查使用 quest 自有消费投影 |
| E15 | `world` → `storage` | **已消除（2026-10-01）**：`world` 声明 `Store` 接口（`LoadWorld`），`storage` 实现并注入；`WorldPosition`/`WorldState`/`WorldReturn` 类型归 `world`，`storage` 以类型别名复用（迁移期）。见提交"world Store 倒置" |
| E16 | `cashshop` → `storage` | **已消除（2026-10-01，本分支）**：现金订单/回执类型归 cashshop，storage 以别名实现持久化；金库购买编排迁入 workflow |

> 说明：`internal/storage` 依赖领域类型和消费接口属于 R3 允许方向（实现方依赖被实现方），保留在例外之外；但不得解释玩法规则。`cmd/*` 工具 import `storage`（`gmtool`/`charactercheck`/`storagecheck`/`initialrepair`/`questrepair`/`avatarrestorecheck`）属 R6 工具层，允许。

### 7.3 领域 ↔ 领域（违反 R4）

| # | 边 | 目标 |
|---|---|---|
| E21 | `character` → `adventure`、`dungeon`、`inventory` | `progression` 的成长规则已归 character，原依赖删除；其余移入 `workflow` 或改为消费者接口 |
| E22 | `quest` → `character`、`dungeon`、`inventory` | `progression` 依赖已由奖励消费者接口消除；其余同上 |
| E23 | `loot` → `dungeon`、`inventory` | `adventure`/`cashshop` 依赖已迁入 workflow 或消费者接口；其余同上 |
| E24 | `legion` → `dungeon` | **已消除（2026-10-01，本分支）**：legion 接收选择校验器，由组合层注入 dungeon 校验 |
| E25 | `cashshop` → `inventory` | 商城发货写背包；改为消费者接口或移入 `workflow` |

当前剩余 9 条领域间允许边。2026-10-02 领域归并删除 `progression`、`profileskin`、`rosterbg` 三个目录；不保留转发门面。后续消耗品与光辉礼盒目录、抽取、进度及奖励修复归现有 inventory.ItemService，事务归 workflow.ItemService；loot 继续拥有副本掉落包装展开（RewardBoxSource/OpenRewardBoxes）与掉落恢复。本批改变职责所有者，没有消除上述 9 条实际领域依赖。军团与 NPC 相位各自具有独立状态职责，保留其领域。

---

## 8. 变更流程

1. 新增/修改依赖关系前，先按 §3 判断落位，再对照 §2 与 §4。
2. 若确需新增例外，必须：改本文 §7、说明目标与消除计划、在提交信息中标注。
3. 消除例外时，同步从 §7 和 `archtest` 允许清单删除。
4. 守卫测试随 `go test ./...` 执行；改动后的测试门禁见 `server/AGENTS.md` §4。
