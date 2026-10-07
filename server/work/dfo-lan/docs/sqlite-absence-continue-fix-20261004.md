# 修复记录：SQLite 档「无法接任务 / 无法穿脱装备」——双引擎「查无此行就继续」被 PG 专有哨兵打断

> 实机症状由业主报告：SQLite 档下能建号、能进城，但**接不了任务、脱不掉装备**。
> 本记录只写已复算的证据；未启动客户端、未访问玩家库（PG 25438）。

## 0. 结论

`internal/database` 里 **41 处**「查不到那一行就继续」的判断仍然只认 PostgreSQL 的哨兵
`pgx.ErrNoRows`；而 SQLite 适配器的 306 处返回值都先经 `storageError` 把驱动哨兵换成了
本包的 `ErrNotFound`。于是同一份控制流在 SQLite 上走进「硬错误」分支，动作被拒。
**`3ec50297` 只把错误文本从 `sql: no rows in result set` 换成了 `stored record not found`，
并没有修行为**——本次才是行为修复。

## 1. 证据：同一操作，PG 与 SQLite 分道扬镳

| 会话 | 引擎 | 任务 3145 / 脱装备 记录 | no-rows 类计数 |
| --- | --- | --- | --- |
| `…_190224_808571` | `wireprobe-pvf.exe`（PG） | `quest_rejected quest=3145 error="quest is completed or requires configuration migration"`（**业务错误**，非 DB 错误） | **0** |
| `…_193431_865803` | `wireprobe-pvf.exe`（PG） | 同上 | **0** |
| `…_221054_428343` | `wireprobe-handoff-source.exe`（SQLite，`3ec50297` 之前） | `quest_rejected`×4、`equipment_move_refused`×3，全部 `sql: no rows in result set` | 34 |
| `…_232103_256813` | 同上（`3ec50297` 之后） | 同一批动作，错误文本变成 `stored record not found` | 27 |

最新一轮 `gateway.err` 里的建号奖励也一样：

```
2026/10/04 23:22:01 reward: grant reward:character_create:newchar.lua:2:item failed: stored record not found
```

同一轮 `events.jsonl` 中被这条路径连带打断的还有（角色 1 与角色 2 各一组）：
`gamepad_options_restore_error`、`entry_stack_slot_error`、`entry_pet_container_error`、
`entry_pet_gear_error`、`entry_creature_loyalty_error` —— 都是「这一行还没有，属于正常」。

## 2. 根因

两处代表性调用点：

* `internal/database/quest.go:63`（接任务）
  ```go
  prior, e := queries.Quest(ctx, sqlcgen.QuestParams{CharacterID: characterID, QuestID: int32(qid)})
  if e == nil { /* 已有行：比对状态 */ }
  if !errors.Is(e, pgx.ErrNoRows) {   // ← SQLite 给的是 ErrNotFound，判定为失败
      return out, e
  }
  ```
* `internal/database/character_event.go:126`（穿脱装备等事件账本）
  ```go
  prior, e := tx.queries().CharacterEventModel(ctx, sqlcgen.CharacterEventModelParams{CharacterID: id, EventKey: key})
  ...
  if !errors.Is(e, pgx.ErrNoRows) {   // ← 第一次移动该 key 必然无行，同样被判失败
      return role, false, e
  }
  ```

`internal/database/driver_errors.go` 的注释早就写明「`isNoRows` 是唯一该知道各驱动怎么写这个
状态的地方」，但该 helper 此前**只有一处使用**（`sqlite.go:159`）——那次重构没把调用点迁过来。
`store.go` 的 `storageError` 也确实只在 SQLite 路径上做映射，PG 路径原样透传 `pgx.ErrNoRows`，
于是「同一个 e」在两个引擎上形状不同，而调用点只认其中一种。

## 3. 改动

| 文件 | 改动 |
| --- | --- |
| `internal/database/driver_errors.go` | 拆成两层：`driverNoRows(err)` = 原始驱动哨兵（`pgx.ErrNoRows` / `sql.ErrNoRows`）；`isNoRows(err)` = `driverNoRows(err) \|\| errors.Is(err, ErrNotFound)`，并把「SQLite 经 storageError 换成 ErrNotFound」写进注释 |
| `internal/database/store.go` | `storageError` 改用 `driverNoRows`（语义不变，只把「谁认识驱动哨兵」收敛到一处） |
| `internal/database/*.go`（22 个文件，41 处） | `errors.Is(x, pgx.ErrNoRows)` → `isNoRows(x)`，**保留原否定形式**（`!isNoRows(x)`）；同时删掉因此不再使用的 `errors` / `pgx` / `database/sql` import |

未改动：SQL / schema、PVF 读取、协议字段、存档格式、玩法规则归属；其它引擎分支未动。

## 4. 验证

新增 `internal/database/sqlite_absence_continue_test.go`（3 个用例，直接锁实机症状）：

* `TestSQLiteAcceptsQuestWithoutAPriorRow`：新角色第一次接任务必须成功落库；
* `TestSQLiteAppliesFirstCharacterEvent`：新幂等键第一次移动必须生效，且重放同一 key 仍幂等；
* `TestIsNoRowsCoversEveryEngineSpelling`：三种拼法（`ErrNotFound` / 两个驱动哨兵 / 包装后的）都认。

**红绿验证**：把 `isNoRows` 临时还原成只认驱动哨兵后重跑，两个 SQLite 用例失败，报错正是实机那句
`got stored record not found`；恢复后全绿。

门禁：

* `go build ./...` = 0
* `go vet ./...` = 0
* `go test ./... -count=1` = 36 个包 `ok`，**0 FAIL**（无 panic / DATA RACE）
* 候选程序（与启动器同参数 `go build -trimpath -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe`）：
  `bin/wireprobe-handoff-source.exe`，35.0 MB，SHA256 `C738A15415B97D3BC5B61AD9A7F2BDDFFEEFE713BEB609C880D71C7C9E38CF50`

## 5. 待业主实机验收

`scripts/启动游戏.cmd --source-build`（会自动重建同一个候选），然后：

1. 接任务（含任务 3145 那条教学链）能否接上；
2. 脱装备 / 穿装备；
3. 退出重进后上面两项的存档是否保持。

判定：`gateway.err` 不应再出现 `stored record not found`；
`events.jsonl` 里不应再出现 `quest_rejected`/`equipment_move_refused` 带该原因。
若仍失败，请把新一轮的 `events.jsonl` 里对应事件与 `gateway.err` 末段发回。

## 6. 边界

未启动客户端、未代替玩家操作；未访问玩家库；未改 PVF/schema/客户端资源；
本次未提交（等业主实机确认后按 §7 收口）。
