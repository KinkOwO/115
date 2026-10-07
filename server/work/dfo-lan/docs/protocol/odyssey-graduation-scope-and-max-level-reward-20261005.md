# 2026-10-05 奥德赛毕业任务范围（按源收敛）+ 满级礼盒（`[maxlevel reward]`）

状态：**两项均实机确认收口**。已确认基线 `server/work/dfo-lan/bin/wireprobe-pvf.exe` =
`94ba97682b73e48797a0c7d8158f9c976161047f91191356a28a63de1b9530d5`（`-trimpath`，取代 `2dd64fc0…`）。
全程只改 Go 服务端，未动客户端/DLL、未改协议布局、未新增数据库表或存档迁移。
源身份：`server/work/client-build/Script.inner.pvf` = `7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80`。

---

## 一、用户报告与取证结论

### 报告 1「115 级奥德赛毕业角色任务没全清，开槽任务还在」

取证（本地 PG `dfo_lan`，角色 1）：

- 毕业**确实触发过**：`character_events` 有 `odyssey-graduation-v2` 收据，`outcome->quests` = **922 行**；该角色只有 3 条 accepted。症状不是「没跑」。
- **缺口 A（本次修）**：`internal/quest/odyssey_graduation.go` 里 `q.MinimumLevel >= 115 → continue` 是 Go 附加规则，把源 `[quest clear]` 明确标为已清的 **52 条**丢掉。live 源复核：`ClearedAt(115)` = 333 = 259（min<115）+ 52（min≥115）+ 22（不在可玩目录）；这 52 条与 `[branch quest]`≥115 **交集为 0** ⇒ 不是分支排除，是纯服务端平行规则。未接线的 `OdysseyMainlinePlan` 反而没有该过滤（自我矛盾）。
- **开槽任务 649/650/2636**：不在源任何 `[quest clear]` 块，条件是 `[meet npc]`+`[slot expansion]`，无职业/角色类型限制 ⇒ 奥德赛角色照旧可接，槽位已由 `[level action]` 幂等开过。**业主判定保持现状**，不写 Go 特例。
- **缺口 B（仍未修）**：源有 20 条 `[no conditions]`（min 115）定义，角色对其中 10 条库里 **0 行**（从未服务端接受），但客户端毕业后 19 秒主动发 CMD34 交任务，被 `InitialProgress`（`progress.go:226-276` 无该分支）拒 ⇒ 客户端本地持有、永远交不掉。动它属新语义，算 attempt，需客户端/官服佐证。

### 报告 2「奥德赛 115 级只发邮件基础奖励，没有『115级达成礼盒』」

- 礼盒 = **`10362946`**，定义在 **`etc/titlebook.etc` 的 `[maxlevel reward] 10362946 1`**。这条规则**不属于奥德赛**：源语义是「任何角色到达等级上限都发一封系统邮件」。
- 我方 Go 之前对 `titlebook`/`maxlevel` **零引用**，所以只有荣誉邮件 `10420561`（队伍框 + 旅程日志）＝ 用户看到的「基础奖励」。
- 邮件文案同样在源里：`etc/hardcodetexttag.etc` 的 `86_levelup_maxlevel_title` → `Max Level Reward`，`86_levelup_maxlevel_text` → `You are now reached the Level 86. Take this box and I will let you ignite.`
  **源文案仍写 86 级**（旧上限遗留文本）。按「PVF 是唯一内容真源」逐字引用，不自造句子；业主确认「可以了」，故保持源文案。若日后要改成 115 措辞，属本服运营改写，须单独登记为 policy 并注明与源的差异。

---

## 二、源 → reader → 领域规则 → 执行/存档 链

| 层 | 位置 | 内容 |
| --- | --- | --- |
| 源脚本 | `aradodyssey.etc [complete reward info] [reward] 10420561`、`[complete mail text]` | 毕业奖励与邮件文案 |
| 源脚本 | `quest/*.qst` 的 `[quest clear]`、`[branch quest]` | 毕业应清任务集合与分支排除 |
| 源脚本 | `etc/titlebook.etc [maxlevel reward] 10362946 1` | 满级礼盒模板与数量 |
| 源脚本 | `etc/hardcodetexttag.etc` → `String/Etc.uv.str` | 满级邮件标题/正文 |
| reader | `internal/catalog/max_level_reward.go: ImportMaxLevelReward` | 块形状必须是**恰好一对** type-0（模板/数量），模板 ≥2、数量 >0，且模板在 `ItemIndex` 里必须是 `stackable`；文案经 `ParseLocalizedRef` → `LocalizedText` 解析，缺一即拒绝启动而不是猜 |
| reader | `internal/quest/odyssey_graduation.go: GraduationQuestPlan` | `ClearedAt(115)` ∪ epic（`0<min<115`）− `[branch quest]`≥115，按职业/转职过滤；**已删除** `min≥115` 过滤 |
| 目录装载 | `internal/gamedata/source.go: Source.MaxLevelReward` + `catalogs.go` 的 `Catalogs.MaxLevelReward` | 挂在已有 `selected["items"]` 分支内，**未新增 catalog key**，故 `configs/pvf-default.json` 与 `scripts/repair_profile.py` 的域白名单不需要改 |
| 领域服务 | `internal/character/max_level_reward.go: MaxLevelRewardMail / ApplyMaxLevelRewardMail` | 上限取 `GrowthRules.LevelCap`（同一个停止加经验的规则），不再新增 115 常量；`character/exptable.tbl` 的 `Thresholds` 有 150+ 项，**不能**当上限用 |
| 领域服务 | `internal/quest/.../GraduateOdyssey` + `character.OdysseyGraduationReceiptVersion = 3` | 门禁 `state.OdysseyGraduationVersion >= 3` 才跳过 ⇒ 老保存自动重跑 |
| 执行/存档 | `internal/database/odyssey_graduation.go` | 事件键 `odyssey-graduation-v3`；检出旧 `odyssey-graduation-v2` 收据走**补偿**：`ClearQuests`（`INSERT … progress_model 'odyssey-skip-v1' … ON CONFLICT DO NOTHING`）只补缺失行，玩家 in-progress 的 accepted 保留自身交任务与奖励；全新毕业仍用 `CompleteGraduationQuests`（accepted→completed） |
| 执行/存档 | `internal/database/max_level_reward.go: CommitMaxLevelRewardMail` | 事件键 `max-level-reward-mail-v1` = 每角色一次性收据；`insertSystemMailTx` 同事务写邮件，满邮箱（`ErrMailFull`）时收据与邮件一起回滚，下次登录/回城重试 |
| 协议/触发 | `cmd/wireprobe/client_entry.go`（登录，奥德赛分支**之外**）、`cmd/wireprobe/client_dispatch_world.go` 的 `returnedToTown` → `cmd/wireprobe/max_level_reward_flow.go` | 登录即对已达上限的角色补发（老角色无需改档），回城即时补发；不猜包，不主动推 NOTI99，邮件提醒沿用登录时的 mailbox alarm |
| 协议/触发 | `graduateOdysseyAtTown`（仅奥德赛，回城后） | 毕业落库后重发 291/342/21 三帧任务快照 |

附件期限沿用 `inventory.GrantExpireTime`（永不过期哨兵），与项目既有发放惯例一致；因此 tooltip 不显示「剩余 30 天」，这是**有意**偏离源显示、为避免「发放物被判过期不可用」（见 `internal/inventory/awards.go` 取证注释）。

---

## 三、验证

代码级：
- `TestGraduationPlanFollowsSourceLevel115Clears`：plan 中 `min≥115` 的条数 **== 52**，并锁 22833/22906/22957/22984/22997/23028 在册。
- `TestGraduateOdysseyReceiptVersionGate`：version 0 的老保存必须重跑；Apply 后（version 3）`applied=false, err=nil`。
- `TestOdysseyGraduationCompensatesLegacyReceipt`（真 PG）：v2 收据 + 玩家 accepted 的 22835 ⇒ 只补缺失行、22835 仍 accepted/`player-active`、存档 `inventory` 原样保留、收据数 2、重放 no-op。
- `TestMaxLevelRewardFollowsSourceBlock`（真归档 `7ef2db59…`）：`template=10362946 count=1`、标题/正文由源解析、模板在源索引里确实是 `stackable`、索引缺件即拒绝。
- `TestApplyMaxLevelRewardMailGates`：低于上限、存档契约不符、无源规则、无上限配置四种情况都拒绝；成功路径不改写存档、附件 = `10362946×1`。
- `TestMaxLevelRewardMailPostgres`（真 PG）：发信成功且**发件人/正文等于源文案**、8 并发只 1 封、114 级不发且不留收据、满邮箱整笔回滚后仍可补发。
- `go build ./...`、`go vet ./...`、全量 `go test ./...` 全绿；PG 测试用独立 schema，跑完 `DROP SCHEMA … CASCADE`，未留残留。

实机（会话 `runtime/roles_persist_…_20261005_202222_585605_next37`，角色 1）：
- `gateway.err:34` `PVF max level reward prepared: template=10362946 count=1 source=7ef2db59…`
- `gateway.err:287` `20:23:22 满级礼盒已投递：character=1 template=10362946`（副本 100004990 结算回城触发）
- `events.jsonl` 20:23:22 出现 `odyssey_graduation_triggers`(291)/`odyssey_graduation_completed_quests`(342)/`odyssey_graduation_available_quests`(21)
- `character_mail` id=6：`Max Level Reward` / 源正文 / 附件 `Template 10362946 ×1`，`status=2`、`claimed=true`、已删除 ⇒ **玩家已领取并消耗**；同批 id=4 为奥德赛荣誉邮件（`10420561`），两者互不干扰
- `character_events`：`max-level-reward-mail-v1` 1 行、`odyssey-graduation-v3` 1 行（`outcome.quests` = 974）
- `character_quests`：completed = **974**（原 922 + 52）；`characters.state.odyssey_graduation_version = 3`、`odyssey_graduated = true`
- 本次角色无 v2 收据，走的是 `CompleteGraduationQuests` 正常路径（其 22835 属毕业主线，已一并完成）；v2→v3 补偿分支由上述 PG 集成测试覆盖。

业主结论：**「我已实机验证 完美」「可以了」**。

---

## 四、仍未闭环

- **缺口 B**：`[no conditions]` 的 10 条客户端本地持有任务，仍无服务端可验证条件，需客户端/官服佐证后再动（算 attempt）。
- **源文案「Level 86」**：按源逐字保留。若业主改为 115 措辞，须登记为本服运营 policy 并写明与源差异。
- 开槽任务 649/650/2636：业主判定保持现状（源如此），不加 Go 特例。
- 礼盒 30 天显示：本服统一用永不过期哨兵，不还原源的 30 天倒计时。
