# next61 — 奥德赛创建补给药水（手册 P3 子项 1）

## 1. 真源

`stackable/10417001/10417791.stk`（奥德赛创建礼盒）的完整相关字段：

```
[stackable type] `[booster selection]` 0
[booster select category] 0 0
[stackable]
10417789 1 10417790 1 10418028 30
[/stackable]
```

三行：武器自选盒、初始防具盒、**30 瓶专属恢复药水**。前两项早已实现
（`odysseyWeaponBoxEvent` / `odysseyArmorEvent`），且两者的收据里分别留了
`"selection_settled": false` 与 `"potion_settled": false` —— 也就是当时就明确了
"这两件还没结算"。

## 2. 实现

`cmd/wireprobe/odyssey_rewards.go`：

```go
const odysseyCreatePotionEvent = "odyssey-create-10417791-potion-10418028-v1"
const odysseyCreatePotion      = uint32(10418028)
const odysseyCreatePotionCount = uint32(30)

func applyOdysseyCreatePotion(role, cat catalog.LootCatalog, rules inventory.BagRules) (raw, receipt, error)
func grantOdysseyCreatePotion(ctx, store, cat, rules, role) (Character, bool, error)
```

**用 `Bag.Add` 而不是照 `applyOdysseyWeaponBox` 手写"找一个空格"**：药水是
`[waste]` 可叠加物，角色包里往往已经有几十瓶（初始补给一路给到 73 个）。手写
"找空格"会在每次重试时多占一格，而且 30 个只落一格；`Bag.Add` 会先找同模板行
叠加，找不到才开新格。

**门控**：`isOdysseyRewardRole(role)` + `role.ConfigVersion == odysseySource` +
目录与 bag 规则的 checksum 都必须是真源校验和。

**满包**：`Bag.Add` 返回错误 ⇒ `apply` 返回错误 ⇒ 事务不提交 ⇒ 事件不落库 ⇒
下次登录时选角链再试一次（`odyssey_create_potion_pending` 事件会记下原因）。

## 3. 幂等（为什么只会发一次）

`storage.CommitCharacterEvent` 的流程：

1. `SELECT ... FROM characters ... FOR UPDATE` —— **先锁住角色行**
2. `SELECT model FROM character_events WHERE character_id=$1 AND event_key=$2`
   - 命中且 model 相同 → `tx.Commit()` 后返回 `applied=false`，**不执行 apply**
   - 命中但 model 不同 → 报 `character event model mismatch`
3. 未命中 → `apply(role)` → `INSERT character_events` + `UPDATE characters` → Commit

因为锁在查之前，同一角色的并发提交也会串行化（PG 默认 READ COMMITTED 下，第二个
事务在拿到锁后能看到第一个已插入的记录）。

**实机证据（角色 id=10，2026-09-23）**：

| 检查 | 结果 |
| --- | --- |
| `character_events` 里 `odyssey-create-10417791-potion-10418028-v1` | **恰好 1 行**（at=2026-09-23 01:16:11） |
| 全表 event_key 分组计数 | 500+ 个键**全部 x1** |
| 药水总量 | `slot 3 x73` + `slot 67 x30` = 103（正好 +30） |

## 4. 落格说明（与初始补给不合并）

新 30 瓶落在 **slot 67**（65/66 被银币占）。原因是
`configs/inventory.current37.json` 的 `slots` 只有：

```json
{ "[throw]": [65, 120], "[material]": [121, 176] }
```

**没有 `[waste]` 条目**，于是 `stackableSlotRange` 回落到默认的消耗品栏
`[65, 120]`。而初始补给那 73 瓶在 **slot 3** —— 那是更早的创建流程写的，不在
该范围内，所以 `Bag.Add` 的"同模板且同范围内"条件不成立，不会并入。

要合并成一叠，需要先把 `[waste]` 的真实槽位范围补进规则表，这需要取证
（客户端脚本/IDA 或参考服的明确依据），本批不做。

## 5. 测试

- `cmd/wireprobe/odyssey_create_potion_test.go`（新增）：73 + 30 并入同一叠
  （断言 slot 与 amount=103）、非奥德赛角色拒绝、外来 checksum 目录拒绝。
- `cmd/wireprobe/odyssey_rewards_test.go`（集成测试追加）：发放一次 →
  `applied=true`；重放 → `applied=false` 且合计仍为 30；收据可从库中读回。
- 顺带修复该集成测试写死的 `runtime/swordmaster-pilot-20260916/storage.json`
  路径（目录已不在仓库，测试长期静默 skip），现支持 `ODYSSEY_INTEGRATION_CONFIG`
  覆盖；跑法：

  ```powershell
  $env:ODYSSEY_INTEGRATION='1'
  $env:ODYSSEY_INTEGRATION_CONFIG='../../runtime/storage/local.json'   # 相对包目录
  go test -count=1 -v -run TestOdysseyArmorDatabaseReplay ./cmd/wireprobe
  ```

  测试自建独立 schema 并在结束时 DROP，不触碰真实存档。

## 6. 遗留

- 手册 P3 其余子项：子项 3/4（章节盒 + 七章奖励）、8/9/10（毕业转换 + 等级主线）、
  子项 5（银币/金币掉落显式化，手册自认本来就生效）。
- `[waste]` 槽位范围取证（决定药水是否需要合并成一叠）。
- 另有 4 个文件引用已删除的 `runtime/swordmaster-pilot-20260916/storage.json`：
  `internal/character/odyssey_integration_test.go`、
  `internal/loot/odyssey_currency_integration_test.go`、
  `cmd/wireprobe/fatigue_run_test.go`、`cmd/avatarrestorecheck/main.go`。
  同属"集成/工具入口指了不存在的配置"，可一并收编。
