# next63 — 满级毕业转普通角色 + 职业等级主线整理（手册 P3 子项 8/9/10）

## 0. 真源

`configs/odyssey-growth-release.json` 的 `definition.cells`（`aradodyssey.etc`，
`definition.sha256 638e71ab…`，无重新导出）。

前置取证脚本：`analysis/tasks/next63-verify-quest-tables.py`（只读）。

### 0.1 四张表实测（与手册逐项对上）

| 表 | 实测 | 备注 |
| --- | --- | --- |
| `[quest clear]` | 11 级（10/17/72/100/102/103/104/105/109/110/115）共 **354** 条 | 跨级重复 id 20 个 ⇒ 并集 **334** |
| `[remove clear quest]` | **10** 条 | 只有 `12911` 真正命中并集，其余 9 条是防御性行 |
| `[show quest]` | **2** 条（26516, 26518） | |
| `[branch quest]` | 80/102/115 三级；每行三元组 `(job, quest, 0)`，`-1`=全职业 | 80: (-1,3868),(11,3869)；102: (-1,12884)；115: (-1,22987) |

关键语义（全部在解析器里硬断言）：

- `12911` 在 `[quest clear]` 102 表内，被移除表救回 ⇒ 102 主线保留；
- `22987`（115 深渊引导）只在分支表 ⇒ 绝不允许进入清除集；
- `12884` 是 102 的分支任务，不在清除表内。

毕业奖励：`[complete reward info] [reward] 10420561`（`stackable/10420001/10420561.stk`，
带 `[booster info]`，与 60/65/90 赠装不重复）。

## 1. 实现

### 1.1 目录层 `internal/catalog/odyssey_quests.go`（新增）

- `OdysseyQuests`：`ClearLevels`（源顺序的 [level] 行）/ `RemoveClear` / `Show` / `Branches`。
- `loadOdysseyQuests(cells)`：解析 + 硬断言（11 级 / 354 条 / 10 / 2 / 3 个分支级含 115、
  12911 在清除集、22987 不在清除集、12911 在移除表）。任何漂移 `log.Fatal` 级拒绝装载。
- `ClearedAt(level)`：≤level 的 `[quest clear]` 并集 − 移除表，升序输出。
- `BranchQuestsUpTo(level, profession)`：`(job==-1 || job==profession)` 的分支行，升序去重。
- `OdysseyGrowth` 新增 `Quests *OdysseyQuests` / `GraduateReward uint32`（均 `json:"-"`）；
  `LoadOdysseyGrowth` 校验毕业盒脚本存在、带 `[booster info]`、模板钉死
  `OdysseyGraduateRewardTemplate=10420561`、不与赠装重复。

**踩坑（本轮实测）**：`[branch quest]` 的 `[level]` 块**没有** `[/level]` 闭合标签，
下一个 `[level]`（或段尾）隐式闭合上一块；按 `[quest clear]` 的写法要求闭合会在 433 处报
"unexpected cell"。

### 1.2 存储层 `internal/storage/quest.go`（+ClearQuests）

- `ClearQuests(ctx, account, characterID, version, ids)`：`INSERT … SELECT … unnest($2::int[])
  … ON CONFLICT (character_id, quest_id) DO NOTHING`，`progress_model='odyssey-skip-v1'`。
- 幂等、绝不改写已存在行（玩家真在做/已做完的任务不动）、JOIN characters 校验归属与
  `deleted_at IS NULL`。回滚：`DELETE FROM character_quests WHERE progress_model='odyssey-skip-v1';`
- ⚠️ SQL 语法为 pgx v5 标准 `unnest` 用法，但本库无先例；实机若报错只会记
  `odyssey_mainline_pending` 事件，不阻断登录链。

### 1.3 策略层 `internal/quest/odyssey_mainline.go`（新增）

- `OdysseyMainlinePlan(g, profession, level)`：纯策略（IO 分离）；
  分支行再过滤一次（剔除任何与清除集重叠的 id，双保险）。
- `Service.OdysseyMainline(ctx, role)`：门禁 `Odyssey==nil || Store==nil ||
  !CreatedAsOdyssey` 全部 no-op；执行时按角色当前等级写清除行，返回 (cleared, branches)。
- `quest.Service` 新增 `Odyssey *catalog.OdysseyGrowth` 字段，`main.go` 构造时挂接。

### 1.4 毕业转换 `internal/character/odyssey_graduate.go`（新增）

- `State.OdysseyGraduated bool`（`omitempty` ⇒ 旧代码读到只忽略，存档不失效）。
- `OdysseyGraduationLevel=115`（`[grow up level on dungeon clear]` 最大目标）。
- `ApplyOdysseyGraduation`：满级 + 奥德赛创建 + 未毕业 ⇒ 落 `odyssey_graduated=true`、
  `CreationMode=0`、`CreationOptions[10]=0`（**原始建号请求包不动**）。
- `OdysseyRole` 先判毕业再判启动器档位 ⇒ 毕业角色无论 `DFO_ODYSSEY_MODE` 都不是奥德赛角色。
- `OdysseyMember = CreatedAsOdyssey && !OdysseyGraduated`；`world_flow.go` 准入改用它。
- 奖励：`OdysseyGraduateBoxCatalog` 只放毕业盒一个模板；
  `ApplyOdysseyGraduationReward` 用**毕业标记**判定（非活动角色），满包只欠盒子、不阻碍毕业。
- 事件键：`odyssey-graduate-v1` / `odyssey-graduate-reward-v1`，各自 `CommitCharacterEvent` 幂等。

### 1.5 接线（子项 10）

- `cmd/wireprobe/main.go` 选角链：Catchup → Gifts → ChapterRewards 之后追加
  OdysseyGraduate → OdysseyGraduationReward → OdysseyMainline（questService 挂 Odyssey 时）。
  位置在 `EntryBasicProbe` 与 `worldState.enter` 之前 ⇒ 本次登录即按普通角色投影。
- `dungeon_flow.go` 副本结算**不调用**毕业（等级门槛切换会拒掉通关后的移动）。
- `channel_probe.py` 未改（不引入 `odyssey_mode` 标识符；主线/毕业数据随
  `DFO_ODYSSEY_GROWTH` 走，无新环境变量）。

## 2. 测试

- `internal/catalog/odyssey_quests_test.go`：表形状（11/354/10/2/3+毕业盒模板）、
  `ClearedAt` 语义（333 条 = 并集 334 − 12911；低级子集；升序确定性）、
  `BranchQuestsUpTo`（-1 全职业行、职业 11 的 3869、115 行 12884/22987、源顺序）。
- `internal/quest/odyssey_mainline_test.go`：满级计划 333 条 / 分支 3 条且与清除集无交、
  拒绝（nil 目录、超毕业等级）、执行层门禁 no-op。
- `internal/character/odyssey_graduate_test.go`：毕业流（标记/创建标记清零/请求包不动/
  模式覆盖下非奥德赛判定/盒目录唯一/发放 1 个 10420561）与拒绝（未满级/普通角色/重复毕业/
  未毕业无盒/赠装不重复/模板钉死）。
- `go vet` 0；`go test -count=1 ./internal/... ./cmd/...` **20 包全绿**。
- 二进制部署确认：五个串 `odyssey-graduate-v1` / `odyssey-graduate-reward-v1` /
  `odyssey_mainline_applied` / `odyssey_graduation_pending` / `odyssey-skip-v1` 各 grep 命中 1。
- `bin/wireprobe-handoff-source.exe`、`bin/admin.exe` 已重新构建。

## 3. 实机验证要点（待用户操作）

1. `停止游戏环境.cmd` → `启动服务端.cmd`（重启才加载新二进制）。
2. **毕业路径**：重选一个等级 ≥115 的奥德赛创建角色 ⇒ 会话日志应出现
   `odyssey_graduated` → `odyssey_graduate_reward_granted`（背包多一个 10420561 盒）→
   `odyssey_mainline_applied`（带 `cleared`/`branches`）。名单 mode 应变 0。
3. **非毕业路径**：低等级奥德赛角色重选 ⇒ 只有 `odyssey_mainline_applied`
   （`cleared` 为新增行数，重复登录为 0，幂等）。
4. DB 复核（另起调用复读）：
   - `SELECT event_key FROM character_events WHERE event_key LIKE 'odyssey-graduate%';`
   - `SELECT count(*) FROM character_quests WHERE progress_model='odyssey-skip-v1';`
5. 毕业角色进副本/城镇应走普通 `[need level]` 门槛（原奥德赛门槛不再适用）。

## 4. 回滚

- 无数据库结构变更（`character_quests` 只写行）；`ClearQuests` 行可精确撤销：
  `DELETE FROM character_quests WHERE progress_model='odyssey-skip-v1';`
- `odyssey_graduated` 是 `omitempty` 新字段，旧二进制读到只忽略，存档不失效。
- 源码按本文 §1 清单还原；二进制回退上一版 `bin/wireprobe-handoff-source.exe`。

## 5. 遗留

- 手册 P3 子项 5（银币/金币掉落显式化，手册自认本就生效）。
- `[waste]` 槽位范围取证（创建补给药水 10418028 与初始 73 瓶分两叠问题）。
- `ClearQuests` 的 `unnest` SQL 无集成测试覆盖（PG opt-in 场景），首次实机登录即验证。
