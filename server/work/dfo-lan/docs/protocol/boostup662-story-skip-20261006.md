# 662 直升后的主线清除与按源补发清券（2026-10-06）

> 状态：**实机收口（v2，业主 2026-10-06「实机OK」）**。业主 2026-10-06 反馈「直升过后的角色 主线没有清除」，
> 并选定口径「直升时批量完成整条主线 + 按源补发 `[quest clear item]` 三张券」；同日晚二次反馈「**还剩 115 级的任务**」
> ⇒ 清除上界由 `min < [goal level]` 放宽为 `min <= [goal level]`（见 §2.4）。
>
> 实机结果（角色 1「Poison」，现役 `164780f1…`）：`character_quests` = **956 行 completed / accepted 0**，
> `22841`、`23028` 均为 `completed` + `progress_model='boost-story-skip-v2'`；
> `character_events` 有 `boost-story-skip-v2`（count=956）与 `boost-story-skip-tickets-v1`（mail_id 5）。
> **诚实结论：本次命中的是全新路径，v1→v2 补偿分支未被实机命中**（原因见 §7）。
>
> 实机入口：**已确认基线 `bin/wireprobe-pvf.exe` = `164780f1c5c3f8c8…`（30,215,680 B，v2，与候选
> `bin/wireprobe-handoff-source.exe` 同一份）**；v1 基准 `36fa2726a8747c8d…` 备份在
> `.tmp/story-skip-20261006/baseline-36fa2726.exe`，更早基准 `c5a5b3840cade8e8…` 备份在同目录
> `baseline-c5a5b384.exe`。启动仍走 `scripts\启动游戏-SQLite.cmd`。


## 1. 症状与根因

实机（任务手册截图，ch10.LAN Normal / Lv 115 / Elvguard）：胶囊直升到 115 后手册仍有

- `In progress (1): [Act] The Legendary Albino Goblin`（等级 5 的 Act 主线）
- `Act (1)` / `Character (1)` / `Side Story (2)`

根因：胶囊链路（`workflow.LootService.UseBoostCapsule` → `character.BoostLevel`）只改等级、
经验、装备解锁位与活动状态，**从不写 `character_quests`**。唯一的历史清除手段是
CMD1422（`ENUM_CMDPACKET_CLEAR_QUEST_TICKET`，8 字节零体）→ `workflow.ClearActQuests`
（`progress_model='act-clear-v2'`），而它按业主 2026-09 裁决**只清已接受的那一条**
（不递归做后继）。115 级角色每点一次只消掉当前任务，客户端随即自动接受后继 ⇒
对走过直升的角色是打地鼠，永远清不完。DB 侧实证：3145/4873/22994 已 completed、
3146 accepted，正是这条循环的中间态。

## 2. 源证据（当前内层 `Script.inner.pvf`，直读）

### 2.1 活动脚本

`live/event/kor/2026/0326_boostup/boostup.evt`：

```
[capsule info]
  [level up]
    [goal level]      115
    [level up table]  1 114 115
  [/level up]
  [quest clear item]  10327301 10327302 10327303
  [/quest clear item]
[/capsule info]
```

`[quest clear item]` 此前**没有被解析**（`internal/boostup` 只取 `[goal level]` /
`[capsule usable level]` 等）。本轮加进 `Catalog.QuestClearItems`，源缺失 → nil（不报错，
免得一张券拖下线整张 662 目录），出现即必须全是正模板。真实导出令牌断言见
`internal/boostup/catalog_test.go`（`US115_TEST_BOOST_SOURCE` 档）：
`QuestClearItems == 10327301/10327302/10327303`。

### 2.2 三张券清的是哪三条任务

`stackable/10327001/{10327301,10327302,10327303}.stk` 各自带 `[any quest clear]`：

| 券模板 | `[any quest clear]` 任务号 | 任务最低等级 | 任务 `[grade]` |
| --- | --- | --- | --- |
| 10327301 | 649 | 60 | `[side]` |
| 10327302 | 650 | 65 | `[side]` |
| 10327303 | 2636 | 90 | `[side]` |

（任务号/最低等级/grade 由 `gamedata.Source` 装载的真实目录读出，取证脚本已用完删除；
`[grade]` 判定来自 `cells(script.Cells, "[grade]")`，`[side]` 不是 `[epic]`。）

结论：**券清的三条是 `[grade] [side]` 的墙任务，不在 epic 主线扫源范围内**，
所以「批量完成主线」和「按源补发三张券」两件事都要做，互不替代。

### 2.3 主线（Act）的源判据

普通模式主线 = 源里 `[grade] [epic]` 的任务（客户端手册的 Act 章节线）。
本树已有的同判据是奥德赛毕业 `GraduationQuestPlan`：单个 `[grade]` 令牌 type 6 文本
`[epic]`，`0 < MinimumLevel < 上限`，职业 `[job]` 允许，`[grow type]` 与转职阶段匹配。

## 2.4 v1 实机证据与 v2 的界（2026-10-06 晚）

v1 实机（角色 1「Poison」，走 `36fa2726…`）：

- `character_events`：`boost-story-skip-v1` 回执 `{"count":896,…}`、
  `boost-story-skip-tickets-v1` `{"mail_id":5}` ⇒ 批量清除与邮寄两条链都确实落地。
- `character_quests`：completed 913 行，**仍剩两条 accepted**：`22841`、`23028`。
- 目录侧（真实 Source 只读探针，用完删除）：这两条都是 `[grade] [epic]`、`MinimumLevel = 115`、
  `[job] [[all]]`，type 分别是 `[clear map]` / `[reach the range]`；源里 min=115 的 epic 共 **60** 条，
  且没有任何 epic 的 min 高于 115。⇒ 缺口就是 v1 那句 `min < goal` 把「目标等级当年章」整批挡掉了。

裁决口径：业主 2026-10-06「还剩 115 级的任务」，与 2026-10-05 那次对奥德赛毕业
「删掉 `min >= 115` 过滤（计划 922 → 974，实机收口）」是同一条裁决口径。
⇒ 上界一律**含等于 `[goal level]`**，不在 Go 里另立「115 级例外」。

> 注意（本轮顺带发现，**未处理**）：当前工作树 `internal/quest/odyssey_graduation.go:36,58`
> 仍是 `MinimumLevel < OdysseyGraduationLevel` 的旧过滤、`odyssey_graduation_version` 仍写 `2`，
> 即 2026-10-05 那次毕业范围修复不在这棵树里（今天 12:34 的整树同步把它冲掉了）。
> 那是另一条链，按业主裁决另行收口，本任务不动它。

## 3. 与 §0.2 的关系：这是一条业主明确要求的服侧偏差

内层 PVF **没有任何字段**表示「普通角色直升后清除主线」：662 只声明了三张史诗/侧传清券，
`[level up table]` 只管等级。因此批量清除是**服侧策略**，按 §0.2 第 4 条单独记录差异：

- 偏差内容：胶囊落地后把 `[grade] [epic]` 且 `0 < MinimumLevel <= [goal level]` 的任务一次性标为完成。
  上界含等于属 v2（§2.4，业主要求），等级/职业/转职/grade 判据仍全部现取于源。
- 判据来源：**不是**新写的 Go 玩法表 —— 等级/职业/转职/grade 全部从同一份只读 Source 现取，
  上限取活动自身的 `[goal level]`（不硬编码 115）。改源字段结果随之改变，
  测试 `TestBoostStorySkipPlanChangesWithSource` 用改 `MinimumLevel`/`Jobs`/goal 三种输入证明。
- 只作用于走过 662 的角色：门禁谓词 `boostStorySkipEligible` =
  活动状态 `activated`（仅由 `loot.UseBoostCapsule` 置位）+ `level >= [goal level]` + 非奥德赛创建。
  奥德赛角色由自己的毕业链（`odyssey-skip-v1` / `[quest clear]` 表）处理，两边不共用收据。
- **不碰 CMD1422 的 accepted-only 口径**：普通角色的手动清券路径与业主 2026-09 的裁决保持不变。

## 4. 实现链（源脚本 → reader → 领域规则 → 执行/存档/协议）

| 环节 | 位置 |
| --- | --- |
| 源脚本 | `boostup.evt [capsule info] [quest clear item]`；任务 `[grade]/[job]/[grow type]` 与 `[quest clear]` 表不变 |
| reader | `internal/boostup/catalog.go` `Catalog.QuestClearItems`（新增）；`gamedata.Source` 任务目录（既有） |
| 领域规则 | `internal/quest/boost_skip.go` `Service.BoostStorySkipPlan(role, goal)` |
| 执行与存档 | `internal/database/boost_story_skip.go` `CommitBoostStorySkip`，`progress_model`/事件键 `boost-story-skip-v2`；检测到只带 `boost-story-skip-v1` 收据的老存档时**按 v2 补跑一次**（回执写 `compensated_from`），复用既有 `CompleteGraduationQuests`（插入 completed，冲突时**只**把 accepted 改成 completed，所以 v1 已写的行不改写、不降级） |
| 补发清券 | `cmd/wireprobe/boostup_flow.go` `boostStorySkipTickets`，事件键 `boost-story-skip-tickets-v1`，系统邮件附件 ×1 每张 |
| 协议 | 新写入后 `actQuestRefresh` → NOTI291（进行中触发）/342（完成位图）/21（可接清单）；邮件走既有 NOTI262/领取链 |
| 调用点 | ① 胶囊落地（`useBoostCapsule`，`applied` 分支，接在解锁三连之后）；② 进城登录钩子（`client_entry.go`，排在 `questService.Active` **之前**，让本版本之前直升的角色下一次进城即被修好） |

存档兼容（§0 铁律 4）：无 schema 变更；两张收据都是 `character_events` 现有表的行，
`character_quests` 只新增/改写 `status`。重放只花一次索引 SELECT；事务失败不留半成品，
下一次进城重试。已完成的行不会被降级（`ON CONFLICT … WHERE status='accepted'`）。
v1→v2 不要求玩家重吃胶囊：只持有 v1 收据的角色在下一个进城钩子按 v2 补跑一次，
补跑只写「仍 accepted 的行」和「缺失的行」，v1 已经写成 completed 的行原样保留
（`progress_model` 仍是 `boost-story-skip-v1`）；
三张清券**不**跟着升版（`boost-story-skip-tickets-v1` 已发过就不再发，避免二次发放）。

## 5. 测试

- `internal/quest/boost_skip_test.go`
  - `TestBoostStorySkipPlanSortedAndValid`：计划全部落在 `(0, goal]`、有序去重、每个 id 都在源目录里；
    并钉住「计划里存在 min==goal 的 epic」以及实机剩下的 22841/23028 必须被清（§2.4）；
    goal=0 与源身份不符时拒绝。
  - `TestBoostStorySkipPlanChangesWithSource`：改 `MinimumLevel`（含 min==goal 要被收、min==goal+1 要被拒）/
    `Jobs`/goal 后结果随源变（§0.2.5）。
  - `TestBoostStorySkipCommitIsIdempotent`：首跑写入、连跑两次都是 `applied=false`；
    accepted 行变 completed+`boost-story-skip-v2`，带其它 model 的 completed 行不动；
    收据恰好一行；行数等于计划长度。
  - `TestBoostStorySkipCompensatesLegacyReceipt`：预置 v1 收据 + 一条 v1 写过的 completed +
    一条仍 accepted 的 min==goal epic ⇒ 下一次跑 `applied=true`，该 accepted 变 v2 completed，
    v1 那行不被改写，v1/v2 收据各恰好一行，v2 回执带 `compensated_from=boost-story-skip-v1`，
    再重放 `applied=false`。
- `cmd/wireprobe/boostup_story_skip_test.go`：`boostStorySkipEligible` 四种边界（未吃过胶囊 / 未到 goal / 奥德赛 / 目录缺失）。
- `internal/boostup/catalog_test.go`：真实导出令牌下 `[quest clear item]` 三张券。
- 门禁（v2 本轮实际执行）：`go build ./...` 退出 0、`go vet ./...` 无输出、
  `go test ./... -count=1` 全绿（43 个包 `ok`，失败集合为空，与既有空基线逐名一致）；
  `go test ./internal/quest/ -run BoostStorySkip -count=1 -v` 四个用例全 PASS。


## 6. 未闭环项

1. **券的使用帧**：三张券的 `[any quest clear]` 在 CMD507 上的 `[action type]` 未知
   （本机 live 证据只覆盖 337/54/101/169/197/206）。按 §0.1/§0.3 不猜包：本轮只按源补发券，
   使用行为留待实机抓一次帧（预计 attempt 1/3）。若客户端在用券时直接发 CMD1422
   （该帧零体、不带任务号，清除范围本来就是纯服侧策略），则现有 1422 路径即其落点，
   无需新代码 —— 这也要实机证据确认，不作为本轮结论。
2. **实机验收**：已收口（业主 2026-10-06「实机OK」）—— v1（count=896 + 三张券）与 v2（count=956、
   22841/23028 转 completed、accepted 归零）两次实机均落地；`Side Story` 是否随券清除仍未观察，
   因为**券的使用帧还没抓过**（见第 1 条与 §7）。
3. 2638/2639/2722 是否也要按角色抑制（上一轮遗留，与本任务无关）。

## 7. v1→v2 补偿分支：实机未命中（诚实结论，2026-10-06 收口时补）

本次实机走的是**全新毕业式路径**（一次事务写 956 行），不是补偿路径：

- 只读复核 `runtime/storage/dfolan.sqlite3`：`character_events` 里**已无** `boost-story-skip-v1` 行，
  `character_quests` 956 行的 `progress_model` **全部**是 `boost-story-skip-v2`（没有 v1 残留），
  `completed_at` 全部等于本次时刻；而 `characters.created_at` 与 `boostup-capsule` 事件的
  `created_at`（`1791271411729002` / `1791271443559004`）与 v1 那次实机**逐位相同**
  ⇒ 同一个角色、同一份库文件，不是新存档。
- `boost-story-skip-tickets-v1` 的 `created_at = 1791293423114982`，与 v2 收据
  （`1791293423114016`）同一毫秒 ⇒ 券收据也是本次新写的（`mail_id` 又回到 5），
  说明取证（≈21:10，当时明确读到 `boost-story-skip-v1 count=896` + 913 completed）与实机（≈21:30）
  之间，该角色的 `character_quests` / `character_events` 被**外部清过**。
- 代码侧核对：全树 `DELETE FROM character_quests` 只有 `AbandonQuest`（且 `status='accepted'` 才删），
  没有任何路径删已完成行或删 `character_events` 行 ⇒ 不能把这条数据变化算成我方行为，
  也不能据此改口说补偿分支「已验证」。
- 结论：补偿分支的正确性目前**只有单测** `TestBoostStorySkipCompensatesLegacyReceipt` 背书。
  要拿实机证据必须**留一个带 v1 收据、且期间不清任务数据**的角色（与 2026-10-05 毕业那次同一教训：
  见 `docs/protocol/` 与 [[project-odyssey-graduation-quest-gap]] 的同类记录）。
- 另记：当天内层源身份由上午的 `7ef2db59…` 变成 `c638346f…`（角色 `config_version`、两张收据的
  `config_version` 都随之更新），这也是 §2.4 里「奥德赛毕业那次修复不在当前工作树」的同源现象。
