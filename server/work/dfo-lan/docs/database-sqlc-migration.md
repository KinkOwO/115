# 数据库访问收口实施记录

本次仍使用 PostgreSQL 16.4；SQLite 实现和玩家数据迁移不在本次范围。
目标是 SQL 集中管理、sqlc 生成方法真正进入执行路径、pgx 和数据库连接仅留在存储基础设施中。
保留现有领域接口和存储文件，不新增 Repository、Adapter 或按表划分的包。

迁移前的主要问题是 Store 公开连接池、通用 db.Tx 将 SQL 能力传给事务回调、Go/工具/Python GM 分散手写 SQL，以及初始化 DDL 和兼容修复没有统一执行记录。收口以现有 database 为持久化边界；不额外保留 db 包，也不为每个生成查询新增一层转发。

本文后半部分保留分批记录，描述各批当时的状态；最终状态和验收证据以最后的审核记录为准。

追加收口：原 38 份迁移文件已合并为 `internal/database/sql/postgres/migrations/0001_initial.sql`。历史文件名作为该文件中的执行分段/台账身份保留，不再对应独立文件；38 段原 SQL 校验和保持一致，既有模块初始化入口、重复修复及可选门禁继续生效。未来新增升级使用新的增量 SQL 文件，不改写已经应用的初始分段。

## 最终边界

- SQL：`internal/database/sql/postgres/{migrations,queries,repairs}`。
- 生成代码：`internal/database/sqlcgen`，仅存储实现可依赖；版本固定为 sqlc 1.31.1。
- 连接池和生成的 Queries 为 Store 私有字段。
- 事务由 database 开始、提交和回滚；跨表回调只取得具体操作能力，不取得任意 SQL 接口。
- 移除 `internal/db` 及 `database/db_adapter.go`，保留已有消费者声明的领域 Store 接口。
- GM、网关和维护工具通过 database 的具体操作访问存档。任意 SQL 的 dbq 作为明确的诊断例外，在 database 内强制只读事务。
- 正常编译使用已提交的生成代码，不要求安装 sqlc；开发时生成并检查一致性。

```text
sqlc.yaml
internal/database/
  store.go / tx.go       # 私有连接、具体事务能力
  migrations.go         # 已有模块初始化与迁移记录
  character_event.go…   # 复用领域持久化文件，组合查询与存档转换
  sql/postgres/
    migrations/
      0001_initial.sql  # 合并初始 schema，保留历史执行分段
    queries/            # sqlc 命名查询，按现有业务组合
    repairs/            # 显式手动维护脚本
    fixtures/           # 隔离测试的数据库故障注入 DDL
  sqlcgen/              # 生成代码，仅 database 使用
scripts/Generate-SQL.ps1
```

业务规则及协议仍在 character / inventory / quest / loot / workflow / wireprobe；这些消费者通过既有领域接口或具体 database 操作读写。database 负责事务原子性、回执、数据库整数/时间/NULL 转换，内部调用 sqlcgen，再由 pgx 执行。领域接口按实际消费声明，不要求新增各领域 store.go；有必要的跨表事务、状态转换和错误处理保留在现有文件。

SQL 位置映射：

| 范围 | 命名查询文件 | 存储实现与入口 |
| --- | --- | --- |
| 账号、建角、角色列表、世界、出生 | core.sql | store / world / birth，网关与 admin |
| 设置、皮肤、技能栏、通知 | settings.sql | 对应已有 database 文件，网关消费 |
| NPC/现金商城、币种、授权、限购 | commerce.sql | shop_purchase / cash_purchase / grant；workflow 与 cashshop |
| 背包及个人/账号/次金库、材料 | inventory.sql | vault / account_vault / account_materials |
| 疲劳、成长、契约、塔 | progression.sql | fatigue / premium / progression_read / tower |
| 任务与目标、奖励 | quests.sql | quest / quest_objective / quest_reward |
| 冒险团、收藏、编队、好感度 | adventure.sql | adventure / adventure_collection / favor |
| 誓约、征兆、奥德赛毕业、矿区、伊斯大陆、黑鸦 | oath.sql / events.sql | 对应已有领域存储文件 |
| 玩家邮件及 GM 独立管理队列 | mail.sql / gm.sql | mailbox / gm_mail，Go GM HTTP；Python 仅转发 |
| 初始化记录及隔离测试种子/断言 | migrations.sql / fixtures.sql | migrations / fixture，仅基础设施消费 |

## 分阶段检查清单

- [x] 1. 接入 sqlc；角色基础、事件回执与设置试点；独立数据库兼容验证。
- [x] 2. 全部读查询、有限动态分支以及 GM / season / 修复工具查询迁移。
- [x] 3. 全部事务读写迁移；具体事务回调替代 db.Tx；限购计数在同一事务中处理。
- [x] 4. 统一 schema 升级生命周期；已有表采用、旧字段和存档修复保留；迁移记录可审计。
- [x] 5. Python GM 数据库访问转到 Go；测试夹具入口独立；连接池私有化；依赖守卫。
- [x] 6. sqlc 一致性、Go 1.26 全量测试 / vet、专用 PG 集成、Python 检查、候选编译。

各阶段是实现顺序，不是缩小交付范围。未勾选项仍属于本次目标。
独立测试 PostgreSQL 的数据位于忽略目录 `.tmp/sqlc-migration/`，与玩家库路径和端口隔离。
不得执行玩家库清理 SQL、自动启动客户端或将离线测试扩大为实机确认。

## 存档与未来 SQLite

保持原 JSON 字段（包括未知字段）、回执键和 model、历史回执字段大小写、软删除、角色编号、邮件共用序列、二进制设置、NULL / 空集合语义。
SQL 方言、pgtype 转换、锁、隔离级别、序列、时间函数、JSONB 运算和 PostgreSQL 运维守卫全部留在 database。
将来替换连接和 SQL / 生成代码以及这些事务实现；领域规则和领域持久化接口应保持稳定。
不提前引入双引擎接口，不声称 PostgreSQL SQL 可以直接由 SQLite 执行。

未来 SQLite 工作清单（本次不实施）：

| 修改范围 | 必须处理的差异 |
| --- | --- |
| sqlc.yaml、queries、schema、sqlcgen | 改 engine/驱动与生成配置；重写 JSONB、类型转换、ANY/unnest、DO/旧表探测及不兼容 DDL；核对目标 SQLite 的 RETURNING/upsert |
| store.go 与 database 类型转换 | 替换 pgx/pgtype/连接池，明确 NULL、时间、二进制、整数范围及 JSON 语义 |
| 事务实现 | 用 SQLite 可实现的写事务序列化替换行锁、FOR NO KEY UPDATE 和 advisory 锁；保持账号限购、角色存档、回执原子性，不能机械删除锁 |
| 迁移与保存编号 | 另建 SQLite 升级脚本和 PG→SQLite 数据转换；保留存档、回执键、外键与玩家邮件共享编号序列；既有 PG 迁移文件不改写 |
| 运维与测试 | 替换 PostgreSQL 启动/可用性探测/管理会话锁；隔离 schema/search_path 改成隔离数据库文件；处理单写者、busy 重试及连接设置 |
| 验证 | 重跑旧存档升级、未知 JSON/大整数/NULL、并发容量/限购、失败回滚和回执重放；再由用户实机验收 |

## 审计发现及本次保留项

本次只迁移数据库访问，不改变玩法规则。既有重复定义继续登记，避免被 SQL 重构掩盖：

- 冒险经验：`etc/adventurersystem/adventurersystem2018.etc` 的 `[adventure exp rate]` 已由 `internal/adventure/rules_native.go` 读取，`internal/database/adventure.go` 仍有 0.3 / 3:10 的校验和运算。当前保留其取整与余数行为；规则归属收敛须另按根 AGENTS §0.2 处理。
- 邮件：领域 mail 已有容量与期限定义，database 的发送路径仍有对应 SQL 常量。迁移时保留现有行为并复核消费链，不以本次 SQL 移动代替玩法来源验证。
- 契约附加值：`character/progression.go`、`character/clear.go` 和任务奖励仍沿用成长契约 20%，`character/learning.go`、`character/skill_variation.go` 仍沿用达人契约有效等级 +5。本次只把独立查询移到事务开始前，未增加或改变数值定义。对应原生脚本/标签的完整来源链本轮尚未取证，不将旧常量声明为新 PVF 真源；后续规则收敛须按根 AGENTS §0.2 补齐。

## 验证记录

- 开始时工作树仅有用户 `.gitignore` 和 `.ignore` 改动，排除本任务。
- 原代码全量 Go 测试通过（初次环境默认 Go 1.27.1）；正式门禁使用 Go 1.26.5 重跑。
- 尚未完成全范围迁移，不升级 confirmed baseline。

### 2026-10-04 第一批已实现

- 49 个命名查询已生成并接入调用路径，SQL 分为 core / settings / commerce 三组。
- 角色基础、建角分配与排序、事件回执、通知、教学标记、传送收藏、装备技能/指令快照、手柄、技能锁、资料皮肤、选角背景、商店计数与记录已接入 sqlc。
- 这批 DDL 从 Go 移到 `migrations`，由 embed 和 sqlc 共用。已有初始化方法及运行开关保持，迁移版本记录和整体初始化收口仍待完成。
- `internal/db/tx.go` 与 `database/db_adapter.go` 已删除。回调改为具体 `*database.Tx`；查询生成类型和驱动字段私有，不提供通用 SQL 方法。
- 事务回调先取得账号 `FOR NO KEY UPDATE` 锁，再锁角色。该锁串行化同账号操作，并兼容外键 key-share 锁。购买计数和写入使用同一事务，回执重放不调用业务回调。
- Tx 中的商店操作和背景授权绑定当前账号/角色，调用方不再传入任意账号编号。
- 原 16 组存储集成测试不再读取玩家 `runtime/storage/local.json`，统一要求显式 `DFO_TEST_POSTGRES_DSN`。
- 依赖测试禁止业务/网关使用 sqlcgen、领域使用 pgx、恢复 generic db 包或在 Tx 上增加通用 SQL/提交能力。

已通过：Go 1.26.5 全量 `go test ./...`（启用独立 PG 集成）、`go vet ./...`、sqlc 1.31.1 生成一致性检查、任务文件 `git diff --check`、网关候选构建。
追加的专用回归涵盖：旧 schema 存档无损采用、未知 JSON、角色编号与 64 位排序序号、并发建角容量、账号跨角色限购（金币/材料两条事务）、回滚、回执重放、装备两组 NULL/二进制快照、手柄部分回退、通知/教学/收藏空集合语义、收藏第二行失败整体回滚、背景重复解锁与到期视图不改写选择。

测试库进程使用 `.tmp/sqlc-migration/pgdata`，只监听 `127.0.0.1:25439`，用户 `sqlc_test`，数据库 `postgres`；不访问端口 25438 的玩家库。
候选 `.tmp/sqlc-migration/wireprobe-sqlc.exe` SHA256：`bcfe4f62e8f1f7a08f6a2eae2b45955a6a35dbd66b2e6996e687031f0ce64948`。
候选尚未部署或实机验收；未提交用户 `.gitignore` / `.ignore`。

后续继续：皮肤和统一选项、各领域读写与事务的剩余 SQL、网关/GM/工具的直连、Python GM、初始化生命周期、私有连接池、维护脚本与测试夹具、最终全范围审核及 charactercheck。当前 Store.DB 仍公开，剩余 SQL 仍较多，不能将本批标记为整体完成。

开发命令（在模块根、PowerShell 7 下执行）：

```powershell
./scripts/Generate-SQL.ps1          # sqlc 1.31.1 生成
./scripts/Generate-SQL.ps1 -Check   # 在忽略目录生成并比较，不改写源码
$env:GOTOOLCHAIN = 'go1.26.5'
$env:DFO_TEST_POSTGRES_DSN = 'postgres://sqlc_test@127.0.0.1:25439/postgres?sslmode=disable'
go test ./...
go vet ./...
```

### 2026-10-04 后续迁移进展

当前 SQL 输入共 127 个命名查询，分为 core / settings / commerce / inventory / progression 五组；DDL 已抽取 22 份。该数量仅描述当前覆盖，不代表全量完成。

- 皮肤单项、列表、槽位、收藏、仓库及统一选项已接入生成方法，保留 NULL、空集合、账号哨兵、角色所有权和二进制布局语义。
- 账号材料及材料事件事务已接入生成方法。GM 列表和发放历史、网关 season / lottery、开发角色修复入口不再直接查询数据库。
- 疲劳日切、房间记账、付费 run 判定、药剂恢复和副本统计已接入 progression.sql。日期参数在 SQL 中转换，生成类型和数据库日期格式留在 database；药剂最后使用时间采用值及存在标志，避免向业务暴露 pgtype。
- 出生表与世界位置 DDL 已抽取。出生初始化仍补齐缺失记录，保留已记录阶段；特殊频道位置仍只在进入时记审计行，会话内移动不写库，删除原本不可达的特殊频道保存 SQL。
- 契约列表、有效性、续期、商城和角色事件内的契约操作、账号货币读写已接入 commerce.sql。保持既有锁顺序，未把独立契约激活的首次并发续期问题误记为已修复。
- GM 发放完整事务已接入 sqlc。原发放回放及零点券发放会在持有事务时再次申请池连接；改为在同一事务内读取余额，专用单连接池测试证明该路径无需第二条连接。
- 游戏会话 PostgreSQL advisory guard 和月湖待发奖 run 查询也已接入生成方法。guard 的连接仍在释放时关闭，防止带会话锁的连接回到池中。

追加独立 PG 回归已通过：并发重复房间只收费一次、消耗截断、付费 run 的零疲劳续房、新 run 拒绝、药剂库存失败回滚/冷却/日限、倒退时钟/日切、32 位以上怪物经验、出生阶段与大 dungeon 编号、位置乐观冲突/普通及特殊频道隔离/定向污染清理；GM 单连接池回放与角色发放、回滚后事件键复用、扣款越界拒绝；契约到期边界、续期、回执重放和无效回执整体回滚。

尚未完成：仓库有限表分支、任务及旧任务修复、邮件、冒险团与部分玩法存档、商城剩余 SQL、source rebaseline、Python GM、统一迁移台账、私有连接池、诊断/测试夹具入口及最终 charactercheck。上述范围全部继续保留在当前迁移目标中。

本批验证：Go 1.26.5 无缓存全量 `go test -count=1 ./...`（显式独立 PG DSN）和 `go vet ./...` 通过；sqlc 1.31.1 `Generate-SQL.ps1 -Check`、任务文件 `git diff --check` 通过；本批 9 个完成迁移的存储文件已无 Exec / Query / QueryRow 调用。候选重新构建为 `.tmp/sqlc-migration/wireprobe-sqlc.exe`，SHA256 `d0115709b2a2b8efc8dcc380197dec0c5669f4ceb5c60babd27125d3e88b27bf`，替代上述第一批本地候选；未部署、未访问玩家库、未变更 confirmed baseline。charactercheck 的完整命令验证仍待专用夹具入口收敛后执行，不能用其包测试代替该门禁。

### 2026-10-04 任务、金库及商城已迁移

当前共 175 个命名查询、29 份 SQL 初始化/修复文件。新增 quests.sql，金库查询复用 inventory.sql，商城查询复用 commerce.sql，没有新增领域持久化包。

- 任务接受、替代前置分组、放弃、会面/用物/地图目标、批量剧情跳过、完成奖励、跨角色完成查询和旧零进度修复均使用 sqlc。原有角色所有权及软删除过滤差异保持，没有在本次 SQL 移动中扩大过滤条件。
- NPC 到达和单目标击杀的旧模型 SQL 移入 `0023_quests.sql`，原两份只存 SQL 常量的 Go 文件删除。剧情误插入清理仍先审计再删除，与上述模型迁移保持原执行顺序和同一次多语句事务；原模型回归读取同一 SQL 文件。
- 任务模型和用物回归不再要求玩家配置文件，全部转为显式独立 DSN 及私有 schema。
- 主/次金库改用固定命名查询和具体容器选择，不再拼接表名。个人存取、两库移动、账号金库排序/开通/升级/跨库移动及账本已接入 sqlc；保持每种事务既有锁顺序、回执冲突校验和未知 JSON。
- 次金库历史容量补齐的可选调用保留；SQL 仍条件检查旧 `character_cargos` 存在，只提升容量不删物品。后续统一迁移生命周期不能仅按文件序号自动启用该可选修复。
- 商城剩余查询全部迁移：历史金币与礼包存档修复候选、订单摘要/回执、三种金库升级、库存发放和签收。既有金币模板与历史礼包模板、子礼包清单属于原修复输入，本批未新增或改写内容规则；继续保留原 JSON 合并函数，避免丢失未知物品/背包字段。
- 追加专用 PG 回归覆盖前置分组、最大 uint32 进度、批量失败整体回滚、旧地图回执不能推进重新接受的任务、奖励失败与回放、旧误插入精准清理及审计幂等；主/次/共享仓库隔离、登录不自动开通、流水失败回滚全部存档、跨库回放、旧容量与物品保留、商城三种升级目标及回放不重复扣款。

范围审核：本批 8 个任务/仓库/商城生产存储文件均无 Exec / Query / QueryRow 调用。剩余直接执行 SQL 的生产存储文件为 adventure、adventure_collection、favor、bleeding_mine、ispins_weekly、black_purgatory / read、oath_options / progress、omen_state、odyssey_graduation、mailbox、tower_progress / grief、source_rebaseline，以及基础设施 migrations.go。Python GM、工具与测试夹具、连接池私有化、版本台账仍未完成；整体目标保持。

本批门禁：Go 1.26.5 无缓存全量测试（显式独立 PG）、vet、sqlc 1.31.1 生成一致性和任务 diff 检查均通过。迁移 SQL 增补历史来源注释后，三组旧任务修复回归复查通过，候选重新构建为 SHA256 `9d98babd3ebd2bc9c2deebc5c0d0e8537dde62e97126562832f15e7ca5ad5499`。候选仍仅位于忽略目录，没有发布；存档 schema 和修复只在专用测试库验证，玩家库及 confirmed baseline 保持。

### 2026-10-04 生产查询与迁移台账收口

当前 SQL 输入共 263 个命名查询、37 份初始化 SQL（含迁移台账 bootstrap）。生产存储文件中的业务查询均已使用生成方法；剩余直接执行仅为迁移文件执行、隔离夹具的 schema 创建/删除，以及明确的只读 dbq 诊断。连接池仍公开，库外集成测试尚待转入独立夹具；不能据此宣称全目标完成。

- 冒险团资料、收藏、职业编队、精英使用、好感度日切与赠礼、伊斯大陆周回执、奥德赛毕业、征兆状态、誓约进度/选项、矿区编队、塔进度/旧悲叹数据均迁入生成查询。保持旧模型、数字宽度、空集合、日期格式、所有权及软删除差异。
- 黑鸦入场/返还/通关/掉线恢复、奖单冻结和待补领奖单读取已接入 sqlc；奖单与次数阶段仍同一事务。待领奖单先读取最多 16 条，再按创建时间调用领域解码；连接在回调前释放，数据库读取失败不执行部分回调，回调错误仍立即停止。
- 邮件收件人、收件箱、通知高水位、容量、附件序列、发送/系统投递/领取/状态修改全部使用 mail.sql。保留 `mail-send-v1` 回执的 `MessageID` / `RecipientID` 大小写、系统发件人 NULL、nil 编号列表读取全部、非 nil 空列表读取零条、领取后软删除和 15 天期限。
- 存档身份修复使用六个固定查询，去除表/列拼接和标识符工具；保留逐表提交、失败时返回已处理行数，以及不触碰个人金库身份的边界。该修复按当次契约参数执行，不变成一次性的 schema 升级。
- `migrations.go` 为全部模块共用生命周期：先获取事务级 advisory 锁，再执行/校验台账，文件与成功记录一同提交。首次接入旧库实际执行原有 additive SQL 后才记录，不以“表已存在”跳过兼容升级。校验和统一 LF，已应用文件改写会明确拒绝。既有皮肤、任务模型、次金库容量和旧塔补齐仍在每次原调用时重跑并累计 runs；出生补齐保持每次调用；可选修复不会按文件 glob 自动启用。
- 网关原来无条件初始化的 18 个步骤移到 `InitializeGame`，保持原顺序。依赖原生目录准备的其它模块初始化与显式容量修复仍留在原内容门禁之后，共用上述迁移执行器。
- dbq 的 pgx 移入 database.DiagnosticQuery，强制只读事务、传播结果/输出错误，并关闭专用连接以清除会话状态。业务、网关和生产工具全部禁止在 database 外导入 pgx；生成类型仍只在 database 内部。
- charactercheck 通过 `OpenTestFixture` 自建随机 schema，只读取显式 `DFO_TEST_POSTGRES_DSN`，不读取玩家配置。夹具提供具体造数/断言方法，SQL 与生成类型留在 database，Reopen 保持同一隔离范围，Close 删除自身 schema；其它生产入口禁止使用该夹具。
- 完整 charactercheck 揭示旧夹具未更新：补齐选角所需设置/任务/契约表，按目标任务检查重登恢复，排除钱包/复活币与宠物容器的发放模板；同日额度变化和已付费 run 零疲劳续房断言对齐已有行为。生产数值/内容规则未因这些夹具修正改变。
- 完整命令实际命中池耗尽死锁：经验事务持有角色锁时，成长契约读查询另申请连接，其它并发事务已占满池并等待角色锁。怪物/通关/任务奖励、学习与重置的独立契约查询前移到事务前，回调只消费已读标志；契约有效性以请求预读时点判断，计算和已保存回执保持。新增回归禁止经验回调在持有事务时读取契约。

追加独立 PG 测试通过：邮件发送后回执失败整体回滚、领取第二行失败回滚首行、历史附件容量及已领取释放、255 封并发容量、nil/空编号列表、系统邮件 NULL 与空附件、过期/保留状态、黑鸦冻结重放/次数恢复/补领顺序和上限、任意旧身份及个人库身份隔离、迁移失败无残留 DDL/记录、并发首次采用、校验和拒绝、初始化后再导入旧任务/塔存档、显式可选容量修复、夹具重开与 schema 清理、dbq 写入拒绝与错误传播。

本批生产查询迁移后的 Go 1.26.5 无缓存全量测试（显式独立 PG）、vet、sqlc 1.31.1 生成一致性、diff 检查已通过。后来修正事务回调二次申请连接后，character / workflow 专项及依赖守卫已通过；最新全量门禁与完整 charactercheck 仍在推进，不把较早的绿色结果当作最新全范围证明。上节候选已落后于当前源码，尚未发布或实机验收。

继续保留全部剩余目标：`gm-tool/dashboard/gm_dashboard_proxy.py` 的 psql 查询/写入转到 Go（管理台 gm_mail 与玩家 character_mail 语义区分），库外集成测试夹具与连接池私有化、维护 SQL 脚本归位、生成/构建门禁和最终全范围审核；SQLite 不在本次实现范围。

### 2026-10-04 GM、维护 SQL 与库外夹具继续收口

- 当前共 266 个命名查询、38 份初始化 SQL。新增 gm.sql 与 `0037_gm_mail.sql`；原 Python 管理台 gm_mail 数据、状态、索引及过滤/200 条倒序列表保持。仅 Go GM 入口显式初始化这张管理表，游戏初始化不自动开启它。账号与指定角色校验使用参数化查询，发送保留中文、引号、分隔符与换行；撤销只允许 unread/read。
- 新增四个认证 HTTP 接口 `/api/mail/list`、`/api/mail/send`、`/api/mail/revoke`、`/api/vault/send`，保留管理台响应字段及 ok 标志。管理台邮件继续是独立队列，不变成 character_mail 或实际游戏投递。Python 移除全部 psql、连接配置、DDL 和 SQL，统一走原代理；修正 POST 在 token 刷新重试时重新读取已消费请求体的问题。
- 主金库发放复用 CommitVaultMove，在同一角色/金库锁下读、合并、保存；原 Python 忽略 account 参数且读写分离，改为实际校验归属，防止并发覆盖。保留 Template/Amount 历史大小写与未知字段，拒绝 uint32 越界与合并溢出，不增加游戏物品规则表。
- 专用 PG 与 HTTP 回归覆盖旧管理队列采用、不可撤销状态、并发撤销仅一次、异常账号/角色/数量无写入、中文/引号/换行标题、认证/方法/JSON 拒绝、12 次并发仓库合并无丢失、满格/溢出回滚及大整数未知字段保留。Python 三个离线回归覆盖路由/过滤转发、错误不重试及 token 刷新保留原 POST body；不启动 GM 页面或玩家库。
- 7 份 character/workflow 集成测试改用 OpenTestFixture；不再读取玩家配置或自行公开池建 schema，重开通过夹具保持隔离。新增启用的旧搬运夹具修正存档身份与内容哈希混用。历史 captured-autoset 用显式 DFO_TEST_SKILL_CAPTURE_FILE 选择已存在的 before-state 输入，本机该历史 runtime 文件缺失，当前明确跳过；其它技能/觉醒/增益/奥德赛与搬运集成已启用并通过。
- charactercheck 当前已验证任务奖励和副本结算，旧任务检查遗漏现有金币奖励所需 Inventory：保留无执行器拒绝后配置原生 inventory，并验证源金币只结算一次及职业过滤。后续 loot/card 余额断言改为检查开始时的真实余额加本次源奖励，继续验证并发/重放只发放一次，不将较早阶段零余额当作全命令前提。
- 旧 `scripts/migrations/dark_knight_combo_slots_v1.sql` 原字节移动到 `internal/database/sql/postgres/repairs/`，原协议文档引用同步更新。它是手动维护/修复例外，保留原事务、表锁、完整审计与拒绝覆盖条件，未运行或自动纳入初始化，也不作为 sqlc 查询输入。
- Build-Server.ps1 增加可选 -CheckSQL 调用固定版本生成一致性门禁；普通构建继续使用提交的生成代码，不要求安装 sqlc，不自动生成或连接数据库。

仍待完成：其余库外集成测试中的 SQL 与夹具迁移、Store.DB 私有化、完整 charactercheck 最终 PASS、最新全量门禁/候选以及要求逐项审核。已通过的较早全量结果不代表这些剩余项完成，整体目标保持。

本批追加：sqlc 输入现为 269 个命名查询、38 份初始化 SQL，另有 1 份手动 repairs SQL。已迁入夹具的库外测试共 13 份；商城购买、伊斯大陆无限次数异常结算、奥德赛装备/货币、疲劳进房和恢复药剂也已在显式独立 PG 上启用验证。剩余库外直接池/SQL 使用位于 knight_deck、combo_skill、oath_direct_entry、monster_death_advancement、shop_gold / source_identity / integration / pilot、unseal、odyssey_graduation 这 10 份测试；它们仍须迁移，连接池不能提前视为私有。

charactercheck 已实际通过拾取、翻牌和软删除阶段。翻牌补齐原生装备目录，按冻结奖单逐模板验证数量，保留原 EXP 与已有背包；重开 payload 与提交后的实际 payload 相等，不再使用过期的固定 367 字节。学习退款旧检查硬编码技能 46 为拖动源，而前面的检查已从源布局发现实际拖动技能；现在比较退款前除已退款技能之外的完整布局，同时保持 SP=80、仅退款一次、初始等级拒绝等断言。最终完整命令仍在运行，尚未记录整体 PASS。

Python 离线验证当前共 11 项通过（服务端存储启动 5、GM 存储启动 3、Dashboard 转发 3）。旧 GM --check 测试仍断言检查 PostgreSQL 在线；已按现有原生目录只读检查行为修正 mock，验证只执行 -check-catalogs、不探测或启动存储、不打开浏览器。本机便携 Python 的 pefile/cryptography 从已有忽略目录 `.tmp/config-cleanup/pydeps` 加载，未修改便携环境。

当前 269 查询版本的 Go 1.26.5 无缓存全量测试（显式独立 PG）、vet、sqlc 1.31.1 一致性和任务 diff 检查通过；13 份夹具改动及学习退款布局修正均在该版本验证范围中。重新生成的独立网关候选 SHA256 `12e47656f8ae59869d343e6ea0ccbdb66be8948f9d7a5092e56fe2d067faa0a7`，GM 候选 `cacae11c0e25e2425492402cb2d13992c1e55be7201584eeae65826b345097cc`，均位于 `.tmp/sqlc-migration`，未发布。之后只修改 GM 文件头说明，候选不作为最终整项交付；剩余迁移完成后仍要重建并审核。

完整 charactercheck 本轮最后一次启动的执行会话为 89377，输出 `.tmp/sqlc-migration/charactercheck.log`，与其它已终止会话区分。最近确认该会话仍运行，专用 PostgreSQL 的 6 条应用连接均 idle/ClientRead，没有事务锁等待；按同一执行会话继续观察，不因读取超时重复启动。尚无最终 POSTGRES_CHARACTER_CHECK_PASS，不把包测试、较早阶段 PASS 或进程运行状态当作完整门禁完成。


### 2026-10-04 最终边界审核（随后完成验收，见下方终态）

实现范围已按目标逐项审核，阶段 2–5 完成。sqlc 输入现为 280 个命名查询、38 份迁移/初始化 SQL、1 份手动修复 SQL，另有 2 份隔离夹具故障注入 DDL；全部 SQL 位于 database/sql/postgres。最后 10 份库外测试已迁入夹具：knight_deck、odyssey_graduation、combo_skill、oath_direct_entry、monster_death_advancement、unseal 及 4 份 shop 测试，专项实际启用专用 PG 和原生 PVF 后通过，保留原并发、存档、幂等与回滚断言。

Store.db、Store.queries 和 Tx 内部驱动全部私有。AST 守卫覆盖生产与测试，拒绝库外 pgx/database/sql/生成代码依赖、公开 Store/Tx 字段、通用 SQL/提交方法、公开函数参数或返回生成/驱动类型，以及业务调用 dbq 诊断例外。Fixture 只能由显式测试 DSN 构造，除 charactercheck 外不允许生产入口使用。

当前直接数据库执行位置仅为 migrations.go（迁移 DDL 文件）、fixture.go（自己拥有的 schema 和 CHECK 故障注入）、diagnostic.go（只读 dbq）。database 包内升级/兼容性测试故意保留原始 DDL/造数，验证旧 schema、校验和、真正数据库约束失败等行为；这不向业务开放 SQL 能力。历史 gm-tool 备份与 server/reference 不作为本轮运行代码迁移对象；Python 日常编排仍负责 PostgreSQL 进程/可用性与配置管理，不执行业务 SQL。

语义审核已对照原 bootstrap，18 个无条件初始化步骤顺序一致；内容依赖初始化与显式次金库修复保持原门禁。金币/材料购买的限购计数和写入绑定同一个 Tx，账号锁先于角色锁，回执重放跳过回调。原 additive 升级先执行再记录，失败时 DDL/台账一起回滚；四种重复修复、出生补齐、来源身份修复及 GM 独立管理队列保持各自生命周期。

已通过最新 sqlc 1.31.1 一致性、存储包全量专用 PG、AST 守卫、10 份库外专项、伊斯周回执专项与 Python 11 项检查。完整原生扩展全量 Go 的首次运行发现疲劳药剂夹具混用原生 shop 来源和旧 clear-cube JSON/global 来源；修正为同档原生 ImportClearCube 和运行时相同的来源绑定，保留并发/每日限制/未知存档/材料投递全部断言，仅修改测试。首次失败日志保留，最终原生网关复查和完整 charactercheck 仍待记录终态，不能提前标记阶段 6 或目标完成。

四个 Go 1.26.5 -trimpath 隔离候选均编译成功，位于忽略目录 .tmp/sqlc-migration，未部署：
- wireprobe-sqlc.exe：103fb28b40ae7ed5a5fc8ea5d4dbad304f5919861694e5706b421d1496d9272c
- gmweb-sqlc.exe：8ce41ed1d77029d05189035a8b3d0393e7f743579221ca04585a8bce22078853
- admin-sqlc.exe：9a5bb04d0fb5166e3db85fcde3d4cc3236a76b9cb5cdc1bf1a87b2924dc8ed7f
- dfo-tool-sqlc.exe：41946219c33116f9ab424c411ef86da4bcabbdb0ea7ff5c3bc614ff60343bbe9

生产源码在这些候选编译后未变；后续仅修改测试/守卫/本文。玩家库、客户端、默认程序与 confirmed baseline 未触碰，用户 .gitignore/.ignore 未暂存。没有执行手动 repair SQL，SQLite 实现和数据转换仍明确排除。


### 2026-10-04 验收终态与范围

六个阶段全部完成，本次数据库访问迁移完成。最终 SQL 输入为 281 个命名查询；最后新增的 FixtureFatigueUsage 将旧药剂种子的 used/used_max 同时恢复，原 charactercheck/module 的只写 used 种子继续保留，避免改变另一种前置条件。最新真实 PVF 疲劳专项完整通过，随后整个原生网关包也通过；两个中间失败日志保留，未通过改断言掩盖问题。

| 要求 | 当前证据 |
| --- | --- |
| 查询、目录、驱动与依赖边界 | sqlc.yaml / SQL 输入 / database 实际调用链；AST 守卫覆盖 internal 和 cmd 的生产及测试；生产直连仅三个已说明例外 |
| 限购及跨表事务 | 两种支付路径语义审核；专用 PG 并发账号限购、回放/回滚、邮件/仓库/存档测试 |
| 旧玩家存档及初始化兼容 | 旧 schema 采用、未知 JSON/NULL/二进制/编号保留、迁移失败原子回滚、并发采用、checksum 拒绝、重复修复与可选门禁回归 |
| 全量 Go 1.26.5 | go test -p 2 -count=1 ./...，显式独立 DSN，并启用伊斯周回执集成；exit 0。日志 final-go-test-green.log 覆盖当前 go list ./... 的全部 51 个包，集合一致 |
| 静态检查 | go vet ./... exit 0，final-go-vet.log；git diff --check 无错误 |
| 当前生成代码 | sqlc 1.31.1 Generate-SQL.ps1 -Check 通过，281 查询版本一致 |
| 原生网关全包 | 显式独立 DSN + DFO_PVF_CORE_TEST_ARCHIVE，go test ./cmd/wireprobe -run '^Test' -count=1，exit 0 / 162.756s；final-native-wireprobe-green.log |
| 完整角色存储命令 | go run ./cmd/dfo-tool charactercheck，exit 0，最终 POSTGRES_CHARACTER_CHECK_PASS；final-charactercheck.log。此后仅新增另一路测试造数方法和相应断言，该命令实际调用的只写 used 方法及生产路径未改 |
| Python | 服务端存储启动 5、GM 存储启动 3、Dashboard 代理 3，共 11 项离线测试通过 |
| 候选 | 四入口最新 281 查询版本 Go 1.26.5 -trimpath 编译成功，未部署，见下方 SHA256 |
| 资源/玩家隔离 | 专用 PG 25439 最后核对零临时 schema、零其它客户端连接，再校验自身 PID19960/端口后关闭；忽略目录数据保留。没有玩家库、客户端、默认程序或 confirmed baseline 写入；没有暂存/提交文件 |

所有日志与候选均在忽略目录 .tmp/sqlc-migration。用户 .gitignore 与 .ignore 改动保持，手动 repair SQL 当前 Git blob 与原 HEAD 文件一致，未执行。

最新候选：
- wireprobe-sqlc.exe：a156a4bc9eca60f5930e125290ca72fbc89d48be853e25d171adee0cd7749f80
- gmweb-sqlc.exe：eafe99e9192e4121cf6e247850646d2b3b9aef7937cb368a6c70a2565d8552dc
- admin-sqlc.exe：b6d3909c862ef92a569a87ae25af88685070f77ff0a75da488a3c2dea7d3b22b
- dfo-tool-sqlc.exe：b71c873e87d25221b39873eaf0f4d370cf6f76330475fbfeece7743e1dbcf3be

额外扩展验证的限制必须区分：给整个项目设置真实 PVF archive 后的首轮全量原生资源审计没有整体通过，final-go-test.log 除已修复的疲劳夹具外，还报告 TestNativeDungeonsCurrentArchiveFingerprint 与 TestPVFScenesLocalArchive 的完整指纹/历史 hell-party 投影差异，以及 gamedata 包 10 分钟超时。这些内容解析/配置文件本轮 git diff 为空，未改资源或历史期望值，不能将该扩展资源审计宣称为 PASS。数据库迁移要求的标准全量 Go、原生网关全包、专用 PG 兼容/事务和完整 charactercheck 均已通过；完整内容指纹/投影核对属于另行资源审计，本文保留具体失败与日志。历史 captured-autoset 的外部 before-state 文件仍缺失，按既有显式输入门禁跳过；不声称补齐了该历史输入或新增实机验收。

后续 SQLite 仍按前文清单处理 SQL/生成/连接、锁和写事务序列化、迁移及玩家数据转换、编号、时间/NULL、运维和隔离测试。当前不提供双引擎支持，不执行 PostgreSQL→SQLite 切换，不以离线门禁替代用户手动实机回归。


### 2026-10-04 初始迁移文件合并

按用户要求，migrations 从 38 个 SQL 文件减少到 1 个 0001_initial.sql，约 27 KB。合并仅改变 SQL 存放方式，sqlc 的 schema 输入路径保持；生成代码一致性检查通过，281 个查询及生成文件没有变化。

执行器按明确的 -- migration / -- end migration 注释边界取得原 SQL，保持旧台账名称及 checksum。禁止把整份 initial 当作一个模块迁移执行，缺失分段、未闭合/嵌套/重复边界明确报错；新版本增量仍可以使用普通 SQL 文件。原升级入口和初始化顺序不变，重复修复不会因合并变成一次性，可选次金库容量及 GM 管理表不会自动启用。

新增兼容回归冻结合并前全部 38 份 SQL 的 LF 归一化 SHA256，独立核对备份原字节与新分段一致；在专用 PG 采用旧台账并保留 runs=7，验证角色 ID/编号、bytea、未知大整数 JSON 不变，且 core 初始化不会提前建 GM/任务/冒险团/金库表。迁移失败原子回滚、并发采用、checksum 拒绝、重复修复、可选容量和历史任务模型修复专项全部通过。

本轮最新全量 Go/vet、完整 charactercheck 及隔离候选的终态在验证后追加。此前章节的 38 文件数量与候选哈希属于合并前版本；玩家库、内容配置、客户端、默认程序及 confirmed baseline 未改，未暂存/提交用户工作区。


合并版本验收完成：Go 1.26.5 全量无缓存测试（显式专用 PG，包含伊斯周回执）覆盖全部 51 包并 exit 0，go vet ./...、sqlc 1.31.1 Generate-SQL -Check、git diff --check 全部通过。完整 go run ./cmd/dfo-tool charactercheck 终态 exit 0 / POSTGRES_CHARACTER_CHECK_PASS；所有临时 schema 和连接清理后，核对本轮自建测试服务 PID24336/端口25439并关闭。SQLC 生成代码未改，历史 38 个迁移段 checksum 和台账继续保持，目录实查仅 1 份初始 SQL。

本轮隔离候选位于 .tmp/migration-consolidation（均 Go 1.26.5 -trimpath 构建，未部署）：
- wireprobe.exe：e97bd5aa01ca5dcdfd4f125b1ab644fd9ff031fd941ad7a5586442ab020456f2
- gmweb.exe：bdc27c87a349c7304ba43f2543d6d384aa0f91b821fb735d165d77a0433b06c6
- admin.exe：7a65bdfb5ffcb0bfde7d2b41061e01629c3265c1048df3a772fe14090a24c44b
- dfo-tool.exe：4a0ef4bd55f233aa1b887198168528b344a0f837e9e9c85c588dd48cc14c52c7

本轮日志为 .tmp/migration-consolidation/{go-test.log,go-vet.log,charactercheck.log}。前述额外 PVF 全资源指纹审计限制仍按原记录保留；本轮没有改资源或相关历史快照。

目录命名收口：源码包改为 internal/database，Go 包名改为 database；运行数据目录 runtime/storage、配置字段与命令行参数保持原有名称。17 个 SQL 文件逐一校验内容未变。

命名调整验证：Go 1.26.5 全量测试（含独立 PostgreSQL 集成测试）、go vet、sqlc 生成一致性检查及真实 PVF charactercheck 全部通过；四个入口程序已构建至 .tmp/database-rename，未替换运行程序。验证日志位于同目录，独立测试库已关闭。
