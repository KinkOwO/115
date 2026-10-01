# AGENTS.md — server/

## 2026-10-01：副本结算后回城/进入下个副本已确认

- 用户确认“能离开副本/进入下一个副本了”。CMD72 成功 ACK 恢复公共成功字节，格式为 `01 state option`；客户端先消费成功字节，再由 CMD72 handler 读取 state/option。覆盖反馈异常的部分副本和任务3189关联副本验收，不代表所有副本逐一实测。
- 23:48 手动会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261001_234806_594272_next37`：任务3189完成后，CMD72选择副本 ACK `010101`，随后服务端回复 CMD15 gate ACK；CMD16 请求任务3190，服务端于33ms后返回成功 ACK `01`。用户另确认回城和进入下个副本正常。
- 源码候选 SHA256 `9594b7440046e106337bc5de66277d5931bf3a08202f40bb9d3b6671148cb75b` 纳入本项 confirmed baseline；收口时源码与 PVF 默认入口二进制均为该哈希。专项回归和 `go vet ./...` 通过；全量 Go 测试5项失败在 HEAD overlay 对照中同样复现。存档、schema 与客户端资源未改。协议与 IDA 证据见 `work/dfo-lan/docs/protocol/settlement-exit-envelope-20261001.md`。


## 2026-10-01：弓箭手星座时装礼包漏发已确认

- 用户确认修复。根因是 CMD160 客户端请求含8个选中时装模板；弓箭手第4模板小端首字节 `04` 被旧解析器误当成时装属性条数，ACK和入袋只处理前三件。依据权威 IDB 的原生 writer，为属性段增加模板属于所选列表的边界检查。两职业真实请求端到端回归均为8件，时装背包刷新及 ACK160一致，既有时装和角色存档其他字段保持。
- 源码入口 `server/work/dfo-lan/bin/wireprobe-handoff-source.exe` SHA256 `6b15b723263f28ee50137046529b50d5605f38cdd9797533be88f7b4895ed8ab` 纳入本轮确认。默认 `wireprobe-pvf.exe` 也核对为SHA256 `6b15b723263f28ee50137046529b50d5605f38cdd9797533be88f7b4895ed8ab`；确认范围仅为该礼包弓箭手漏发修复及其它职业回归。
- 全量专项测试和 `go vet ./...` 通过。全量 Go 测试仍有5项失败，修复前代码的 overlay 复核确认同样失败，未新增失败。没有修改 schema、玩家存档数据库或客户端资源；22:37:24实机会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261001_223606_846403_next37` 的角色1 ACK160记录 count=8，8个模板与弓箭手CMD160请求一致，随后角色10第44帧入场预检通过。确认范围限于该礼包的弓箭手漏发及其他职业的专项回归；以前已消耗礼包漏发的5件不自动补发。
- 本次源码文件、协议原生向量、端到端回归、IDB函数索引及交接记录已收口；见 `work/dfo-lan/docs/protocol/archer-avatar-package-20261001.md`。


- **Cera 商城 Cera/金币购买已确认**：用户确认“能购买”。21:53:40 会话 roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261001_213843_140102_next37 记录 SKU3400476 金币订单 applied=true、扣100金币、Cera前后均921780，模板590721400×1到账；同一毫秒记录CMD64响应帧。此前普通Cera SKU3000127 已有扣10及模板10000540×1到账记录。金币修复源码SHA256 c518e50af555178018e85604eb45c814d77f0d108175b8c5170196b5f3e468b5 纳入商城购买confirmed baseline，通过启动游戏.cmd --source-build 使用；默认程序SHA256 ffda6686159700d293a1e9395190f218ce8f094a25208ed84fc7220b94e72263 保持。本次限定普通14列商品购买，扩容、特殊货币及所有商品未逐项验收。go vet通过；全量Go测试仍有改动前已存在的5项失败。无schema、玩家存档或客户端资源修改。详见 work/dfo-lan/docs/protocol/cera-shop-gold-purchase-20261001.md。

- **Cera 普通购买身份修复已确认**：用户反馈 Cera 点券商品能买。20:51 会话的 20:53:26 日志记录角色13购买SKU3000127，Cera921790→921780，扣10点，模板10000540×1落袋，NOTI14/53及CMD64成功回执发出。商城订单改用存档契约身份，保留PVF目录校验及原子交易；候选源码入口SHA256 ffda6686159700d293a1e9395190f218ce8f094a25208ed84fc7220b94e72263纳入本次普通Cera购买confirmed baseline。背包/金库扩容和全部商城商品未逐项实机确认；金币商品后续修复及确认记录见cera-shop-gold-purchase-20261001.md。原生PVF独立库购买回归和vet通过，全量5项既有失败已在未修改HEAD复现。默认程序实际bc9ce992保持，未操作玩家数据库或客户端资源。详见work/dfo-lan/docs/protocol/cera-shop-save-identity-20261001.md。


- **归档元数据缓存已确认**：用户确认速度提升。17:50会话两类缓存miss/stored，准备50.244秒，角色4的45帧入场预检通过；17:53会话两类缓存hit，元数据2.361秒、联合物品2.647秒、全部准备25.704秒，角色11的49帧预检通过。正式/源码程序均核对为46c349cd8151ea66b9f056ce32f1c9f63368ee4ec6cc448205fbd713a062f7d8，无需替换，纳入confirmed baseline。确认依据用户反馈及上述日志，不扩大为所有玩法逐项验收；不改schema、存档准入/profile/客户端资源。按授权提交本段，再拉取合并上游SHA相关更新，继续其它投影及缓存保留策略。下文候选状态为历史记录。

> 本文件是 `server/` 目录的规则真源与子索引。先读仓库根 `AGENTS.md`，再读本文件。
> 服务端源码位于 `server/work/dfo-lan/`，探针与隔离工具位于 `server/work/dfo_probe_tools/`。

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
4. **禁止把开关当数据。** 玩法行为不留开关（见根 `AGENTS.md` §6 开关原则）；数值差异入口
   必须能追溯到一条明确策略文件，且该文件**不得承载「有哪些内容」的清单**。

**允许保留的 JSON**（仅此三类）：

| 类别 | 例子 | 约束 |
| --- | --- | --- |
| 运维 / 策略 | `pvf-drop-policy.json` 的排除项、各 `pvf-*-policy.json` 的开关与上限 | 不得承载「有哪些内容」的清单；**能自动发现的一律自动发现** |
| 历史基线 | `configs/*.json` 导出物 | 仅在 `DFO_PVF_VERIFY_BASELINES=1` 时作对照，**默认关闭**；不得作为运行期输入 |
| 本地运行配置 | `launcher.local.json`、`pvf-default.json`、`storage/local.json` | 端口 / 路径 / 环境变量，不含游戏内容 |

**已有违规项的收敛方向**（逐项销账，路径与依据写进 `../docs/todo/pvf/PVF单一内容真源改造计划.md`）：

- ✅ **已销账（2026-10-01）**：`attunement_dungeons` 已从 `pvf-content-policy.json` 与
  `pvf-mine-policy.json` 删除——副本范围改由源决定（`etc/rewardboostinfo/**.ctp` 各自声明
  `[dungeon index]`，`loot.ImportAttunementRewards` 自动发现，`Source.Attunement()` 不再收清单）；
  同一批也删掉了 `dungeon_enter_fatigue`（改由 `[use fatigue only start dungeon]` 决定）。
- ⏳ **待收敛**：`pvf-drop-policy.json` 的 `basic_equipment_ids` / `maximum_loot_grade`
  → 从 PVF 装备表自动发现，JSON 只保留排除项（`excluded_loot_ids`）。
  **注意（业主判断，2026-10-01）**：深渊本身是**特殊掉落池**，不能简单套用「PVF 全量装备表」；
  放开范围前必须先弄清深渊自己的池子边界，**不要**为了「走 PVF」而改变掉落行为。

## 0.1 当前确认边界（历史确认记录）

- **2026-10-01 装备调适（CMD2258）已确认**：用户实机确认调适可用（左侧 Tune/Promote 面板按成功刷新并播放调适动画），纳入confirmed baseline。实现为**唯一内容真源 = 内层 PVF**：规则直读 `etc/115lvability/equipmentawakeningoptionsystem.cos`（`[max awakening]`/`[condition]`/`[need materials]`/`[refund materials]`/`[rates]`/`[upgrade result]`）与 `equipmentawakeningoption.lst`（263 条选项），**未新增任何 JSON 链路**。协议按 IDA 闭环：请求体 25 字节（`+13` 模式、`+14..17` 材料组、`+18` 空间、`+19..20` 槽位、`+21..24` 目标模板），应答体 `u8 状态 + u16 结果码`，**状态 1 = 成功**（状态 0 会连面板一起复位——这正是"面板不刷新、无反馈"的根因）。调适阶段落在装备实例行 `+170`（与 `fame.go` 同一映射），升品换模板并清零该字节，其余实例字节与存档未知字段保留；材料 + 金币走同一 PostgreSQL 事务（账号材料仓库优先、背包兜底），按 `(角色, 幂等键)` 防重放。实机（角色 11 / 模板 100261128）连续三次 `0→1→2→3` 成功、`payload_offset=13`，请求 hex 逐字节与实现一致。本轮未做：`mode=1` 初始化/返还、非 100% 成功率、套装积分、CMD2259 转换里的调适联动。取证与清单见 `../analysis/tasks/next148-装备调适-直读PVF实现.md`。
- **2026-10-01 第四批剩余优化已确认**：第四批剩余项已确认：七类确定性投影缓存（装备绑定/掉落/副本/赛季/背景券/传送/终场剧情）及旧缓存保留策略，绑定实际PVF/完整程序/实际输入策略，损坏重建、不可写回退及私有查询索引恢复。424216条装备、18387张地图、七类全部字段和54/63冷热启动一致；全量Go测试/vet、独立PostgreSQL16存档身份迁移回归通过。已提交确认段3def161并以2c24faa合并上游07e1551。用户手动连续两轮启动源码入口并确认：18:21:24冷轮准备48.0247秒，九类缓存miss/stored，角色1第46帧entry_preflight_passed；18:23:49热轮准备14.4253秒，九类缓存hit，角色1第46帧entry_preflight_passed。正式入口与源码入口均核对为SHA256 bc6211802e361f1407a14fd62d7a5730be9ed5250851a7f4c6c48aea6d32310e，现纳入confirmed baseline。热轮相较此前确认热轮23.9059秒快39.44%、累计分配降低61.84%；首次建九文件48.51秒，热堆527.47→536.01MiB，缓存合计约191MiB。实机确认范围为连续两次启动及选角进入前置检查，未扩大为所有玩法逐项验收。 上游包含存档身份契约迁移：新源码启动后旧46c349cd默认程序不能直接作为回退。优先保持源码入口并设置DFO_PVF_CACHE_DIR='-'恢复原生导入；若需撤回本批实现，关闭会话后将.tmp/pvf-phase4c/bin/wireprobe-metadata-sha-compatible.exe复制到源码入口，再继续--source-build。该185ae7e99853d2b4d96a1043c47eb046279cfb6d777e536e7e1df6c1d46ba92f程序来自合并提交2c24faa，含上游身份修复及已确认元数据/物品缓存，不含本批七类投影；54/63离线完整报告与候选一致，未操作玩家数据库。46c349cd精确备份.tmp/pvf-phase4c/bin/wireprobe-handoff-source.confirmed-before.exe仅作迁移前历史快照。 详见../docs/todo/pvf/PVF启动与内存优化实施计划.md。

- 本轮上游同步后的Go1.26全量test/vet（含旧背包兼容）、当前源抽奖/自选原生回归及14项profile检查通过。源码提交不替代新增实机验收。

- **2026-10-01 本分支同步上游及JSON清理源码收口**：按用户要求合入upstream/main 6eb61fc，merge为05f14ed，保留第三批按需读取/旧背包预热兼容及双方confirmed记录；本批提交抽奖自动发现、只读自选审计和24个无消费者JSON清理。源码候选aabb142d06f3bb6f479c2e7b93302708e3c2f6823d2727d2e0fc1166774bc3be，当前8b源完整抽奖边界/无JSON准备及自选审计通过；没有新的实机确认，运行基线仍按下文对应轮次记录。用户四份配置改动排除提交，默认程序保留，未操作玩家库或客户端。详见../docs/todo/pvf/PVF单一内容真源改造计划.md。
- **2026-10-01 归档元数据缓存候选**：当前内层归档的原始文件记录、路径索引、分组和字符串池接入可失效元数据缓存，仍先验证固定原生文件完整SHA。完整565万目录/池字节、视图生命周期、54/63冷/热启动、物品全字段复测及Go全量测试/vet通过。在物品缓存已命中的基础上，同源单次准备33.14→26.18秒（再快20.98%），归档打开10.14→2.91秒，保留堆基本持平；首次建立两类缓存51.10秒，元数据文件约162MiB。源码46c349cd8151ea66b9f056ce32f1c9f63368ee4ec6cc448205fbd713a062f7d8待用户手动--source-build两轮回归，默认confirmed保持8d6f979a；不改schema、存档准入/profile/客户端资源，其余投影及旧文件保留策略未完成。本段未提交或实机确认。默认runtime/pvf-cache，DFO_PVF_CACHE_DIR或-pvf-cache-dir配置，-同时禁用两类缓存。详见../docs/todo/pvf/PVF启动与内存优化实施计划.md。

- **2026-10-01 第四批联合物品缓存已确认**：用户确认运行正常且第二次启动变快。17:21会话记录miss/stored、准备42.705秒；17:24会话记录hit、联合物品2.430秒、全部准备30.792秒；两次角色1的46帧入场预检及CMD4回执通过，进城体验依据用户反馈，不扩展为所有玩法逐项验收。正式/源码程序均已核对为8d6f979a189eadd96d1e2c45462439d7ecbdd7c85c53ede12aa57720a32b6527，无需替换，纳入confirmed baseline。仅提交本任务文件，继续剩余启动优化；不改schema、存档准入/profile/客户端资源。下文候选状态为历史记录。

- **2026-10-01 第四批联合物品缓存候选**：八个共用投影接入可失效磁盘缓存，原生完整SHA校验仍先执行，绑定完整程序/源/强化策略身份；损坏重建、不可写回退、完整字段及54/63冷/热启动回归、最终全量Go测试/vet通过。同源准备40.49→31.15秒，首次建缓存42.23秒，保留堆基本持平。源码8d6f979a189eadd96d1e2c45462439d7ecbdd7c85c53ede12aa57720a32b6527待用户手动--source-build，默认confirmed保持c6b2bace；缓存默认runtime/pvf-cache，可由DFO_PVF_CACHE_DIR设置，-禁用。本段尚未提交/实机确认，其它投影与旧缓存保留策略未完成；不改玩家数据/schema/profile/客户端资源。详见../docs/todo/pvf/PVF启动与内存优化实施计划.md。

- **2026-10-01 第三批及选角修复已确认**：用户反馈“确认正常，可以提交了，然后开始下一批吧”，选角预热兼容修复及第三批收口。正式/源码程序均已核对为c6b2baceadae5c0a788ef6aab627fcc428c2fafb97014263175035dae0603ed9，无需再次替换。16:57日志确认角色1的46帧入场预检通过、CMD4选角回执和技能/账号恢复发出；进城正常依据用户反馈，不扩展为所有功能已逐项实机验收。保留历史未知背包/任务准入，存档/schema/profile/客户端资源未改；已有全量测试/vet及真实源回归通过。按授权提交本任务文件，继续第四批可失效磁盘派生缓存。

- **2026-10-01 选角预热兼容修复候选**：用户报告第三批后选角无法进游戏。16:44:24日志select_rejected明确为prepare item 10310180: item absent from runtime index；当次使用本地重编译96aca078正式程序。be95原生LIST检查确认10310180无绑定；原流程允许历史未知背包项，新增预热错误地将其作为登录准入检查。已修正为按模板索引类型预热：已知STK及装备读取相应源，旧Bag.Items里的装备也按原生装备查询；未知模板只跳过预取，不删除/迁移存档、不扩大购买发放范围。已接的历史未知任务同样不因可选预取阻止登录；已知源关闭/读取错误仍传播。包含实际10310180、旧背包装备及未知穿戴的保留/状态不变测试和已知源关闭拒绝测试通过，54/63真实准备、全量go test/vet通过。源码修复候选c6b2baceadae5c0a788ef6aab627fcc428c2fafb97014263175035dae0603ed9，待用户手动启动游戏.cmd --source-build重选回归；本次没有确认实机恢复，不升级confirmed。正式程序仍保留96aca078，旧源码已备份，本轮未更改玩家存档/schema/profile或客户端资源。

- **2026-10-01 第三批完整候选**：第三批实现与离线验证全部完成：物品/技能/任务独立源视图与按需详情、各16MiB/256条缓存，装备32MiB/2048条逐条LRU及8字节原生绑定，选角预热当前角色装备/背包/技能/已接任务。当前be95源全部175554条可用STK、424216条装备、3224条技能、2844条任务详情一致，任务索引/城镇到达/NPC可见性图及进房检查通过；全量Go测试/vet和54选择项/63投影检查通过。源码候选07c8ffbdc2cf4209af91d639650e24fd29e330d86304b134c0e7400cda032fcd，confirmed身份仍72f040e5。本地正式程序已由同期外部构建改为8e1acb8d中间版本，保留该文件，不混作72f确认身份。新功能实机回归待用户手动--source-build，不将离线完成写成实机确认。未改schema、来源规则、profile或客户端资源；第四批磁盘缓存尚未实施。

- **2026-10-01 共享字符串池已确认**：用户反馈“已确认，可以提交，之后应该不会有什么大问题了，把第三批全部完成吧”，共享字符串池优化确认收口。正式及源码程序均核对为72f040e5cbd16ea1f259478fffc250ac6a5df8f8aa5617794b5ecfa819584730，日常根入口直接使用确认版。当前be95源的完整离线对照及GC堆545.78MiB样本保持；确认依据用户反馈，不扩大为逐项实机动态命中。本轮不改schema/profile/客户端资源，仅提交本任务文件；按授权完成第三批余项，第四批磁盘缓存另行实施。

- **2026-10-01 PVF共享字符串池压缩候选**：地图批次以962c9fc提交后继续，64KiB压缩块由所有运行视图共享，解压LRU最多16MiB，父源关闭后装备/MAP仍可读；当前be95全部池字节/5650173目录、地图/NPC/进房及54/63检查、全量Go测试/vet通过。同源GC堆953.31→545.78MiB，准备37.57→38.54秒，峰值约2.6GiB未明显下降。源码72f040e5待用户手动--source-build回归，默认confirmed仍45084387；不改schema/profile/客户端资源，不引入磁盘缓存。详见../docs/todo/pvf/PVF启动与内存优化实施计划.md。

- **2026-10-01 PVF第三批首段已确认**：用户确认并授权提交/继续，地图按需读取收口。正式/源码程序均为450843870979c753949a740b093152d6c8123d550520c101d429afc6bebd4a97，支持现行来源自动派生配置。当前用户生成内层be95d64e；先前原生全量及0.93GiB性能样本属于7ef，确认不扩大为新源逐项取证。日常根启动入口可用；第三批后续及第四批未完成，详见../docs/todo/pvf/PVF启动与内存优化实施计划.md。

- **2026-10-01 PVF第三批首段候选**：MAP脚本按需读取，保留完整副本/地图准入与哈希；原生全量、进房/NPC/沉月湖对照、54/63联合检查及Go全量测试/vet通过。只读GC堆约0.93GiB、峰值2651.37MiB、准备36.80秒。源码45084387，默认confirmed程序仍2e4bd343；待用户手动--source-build回归。同期合入来源自动派生，修复其与只读入口的冲突并保留存档处理；当前空校验profile不兼容旧默认程序，旧回退必须配套旧资源/策略。未访问玩家库或客户端；第三批后续及第四批未完成。详见../docs/todo/pvf/PVF启动与内存优化实施计划.md。

- **2026-10-01 PVF第二批后段已确认**：用户确认并授权继续，复杂物品联合扫描收口。正式与源码程序均为-trimpath构建SHA256 2e4bd343012d5821160f27770a07c382172818d6c3b2bf15d7edae1a808c0678；原bbce为采样构建，非当前发布身份。保持54选择项/63投影与精确7ef存档来源；第三批按需查询后续实施，第四批尚未实施。详见../docs/todo/pvf/PVF启动与内存优化实施计划.md。

- **2026-10-01 PVF第二批后段性能候选**：皮肤/普通礼盒/强化/堆叠物名望共用STK扫描，真实完整对照、Go全量测试/vet及54/63联合检查通过；单次只读准备37.02秒、GC后堆约1.39GiB。源码候选SHA256 bbce6de8f663f7bea936d9f1cdc9203b6c13aee82cfaa1579859e59b8f65ed21，默认程序仍为已确认59e14ec1版本。未操作玩家库或客户端，待用户手动回归后收口；第三/四批未实施。详见../docs/todo/pvf/PVF启动与内存优化实施计划.md。

- **2026-10-01 PVF性能优化已确认**：用户反馈“确认没有问题”，归档紧凑目录/固定文件读取/有界缓存与四类物品联合扫描纳入当前默认基线。wireprobe-pvf.exe及源码程序SHA256均为59e14ec18f498f07004b9a797c73e2cdba5f3f8af2245e1401d1953b3105dad3；保持54选择项/63投影及原存档source。业务按需加载、复杂投影合并及磁盘缓存尚未实施，本批已收口。详见../docs/todo/pvf/PVF启动与内存优化实施计划.md。

- **2026-10-01 全量PVF已确认并默认启动**：用户确认54选择项/63源投影，三个根启动入口默认加载configs/pvf-default.json及bin/wireprobe-pvf.exe；源固定为client-build/Script.inner.pvf的7ef校验，存档来源不别名/不改写。源码普通构建保留已确认默认程序，-UpdatePVFDefault显式更新；JSON回退用--json-mode，旧39及逐批程序保留。详见../docs/todo/pvf/PVF直读默认启动确认.md。

- **2026-09-30 黑暗武士组合技能栏已确认**：用户实机确认编辑及重选恢复生效；日志有10次CMD500保存、9次type0/NOTI433回放和两次入场恢复，角色20存档保留最后排列。默认prof9组合技能118～123绑定0～5，六槽只调平存量角色20，无schema或客户端改动。CMD502实机捕获16个零填充字节，已修正并通过PG临时库集成测试；修正后的清空操作尚待再看一条实机日志。旧charactercheck有既有依赖/疲劳检查问题。H格PVF空宏未修。详见work/dfo-lan/docs/protocol/dark-knight-comboset-quickbar-20260930.md。

- **2026-09-30 转职觉醒特效候选**：账号选项1接入NOTI343即时刷新，选角、入场、换装及同场景缓存保持一致。原生reader为1452C9940，注册点1452FA194，bit1/4/5分别对应转职/二觉/三觉。用户确认设置保存，日志确认三觉标志33已发送，各职业动画未逐一验收；不改变真实觉醒、时装或客户端资源。

- **2026-09-29 冒险图鉴引导提交候选**：CMD2139仅接入装备分类的引导物品100261068，扣物、账号集合和回执同事务保存；21651进度依据真实登记恢复。NOTI2425在登录阶段恢复集合，不再依赖CMD35；登记后使用CMD33原位更新任务，21651/21652零奖励使用NOTI1668避免空白结算窗。登记与引导完成有实机日志，最新登录恢复及零奖励通知尚待实机确认；现有全量测试与vet通过，不修改客户端资源，不扩大为完整装备或卡片图鉴已实现。

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
而直读默认档 `configs/pvf-default.json`（17 个键）与 `启动服务端.cmd` 里**一个都没有**
⇒ 玩家走默认入口时**整套玩法静默不生效**：征兆不掷骰（日志 `-omen-rewards is off`）、
隐藏 BOSS 无门禁来源、定盘机关可能打不死（"既不放结束动画也不 DESTROY"）、
**疲劳服务根本没加载**（`if *fatigueRulesFile != ""` 不成立 ⇒ `fatigueService == nil` ⇒ 所有疲劳检查被跳过）。

**教训**：开关的代价不是多打一个 flag，而是「**默认路径悄悄坏掉**，且没有任何人会发现」。
