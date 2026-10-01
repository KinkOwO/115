# next146 — 存档身份与内层哈希解耦（结构性根治）

> 承接 `next142`（锁自动派生）、`next145`（发布默认程序）。
> 142/145 解决的是「**启动期**能不能起来」，本文解决它们暴露出的**第三类耦合**：
> **存档身份被钉在「每次重打包都会变」的内层哈希上** ⇒ 重生成一次内层 PVF，**全体角色进不去**。
> 2026-10-01 实机现场（玩家报「游戏启动了，但无法进入角色」）暴露，同日止血 + 设计根治。

## 1. 事故现场

玩家侧：**游戏能启动、能登录、能选角色界面，但点进角色失败**。

### 1.1 取证（会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261001_133625_316643_next37`）

`events.jsonl` 的关键序列：

```
character_response                      ← 选角列表 OK（说明能连、能读档）
client_frame id=4 ...                   ← 客户端发「进入角色」
quest_restore_rejected {"error":"quest 3151 requires source migration"}
   ... ×129 → id=2127 (ENUM_CMDPACKET_PROCESS_SCAN)   ← 客户端反复重试 + 刷无应答遥测
```

**判读**：

| 证据 | 含义 |
| --- | --- |
| `character_response` 有 | 存档**读得到**，不是库挂了 / DSN 错 |
| `quest_restore_rejected` | 进角流程走到**任务恢复**这一步被**主动拒绝** |
| 拒绝理由 `requires source migration` | 存档行的 `config_version` ≠ 当次内层 checksum |
| 之后 `id=2127` ×129 | `main.go:4763` 遇拒后 `continue`（不发包）⇒ 客户端重试 opcode 2，再刷 2127 |

### 1.2 根因（第 11 道门禁：数据级）

`internal/quest/progress.go:428`：

```go
if q.ConfigVersion != s.Catalog.Source.Checksum {
    return nil, fmt.Errorf("quest %d requires source migration", q.QuestID)
}
```

而 `role.ConfigVersion = s.Catalog.Source.Checksum`（`internal/character/service.go:154`）——
直读模式下 **= 内层 `Script.inner.pvf` 的 SHA256**。

⚠️ **关键事实：内层归档的哈希不可复现**。用**完全相同的客户端三元组**
（`DFO.exe` + `sk.dat` + `Script.pvf`）重新生成，得到的是**另一个哈希**：

| 批次 | 内层哈希 | 大小 |
| --- | --- | --- |
| 旧（`current37.json` 钉的） | `7ef2db59…d88e80` | 760,530,763 B |
| 新（本次启动用的） | `b2b503b5…33b13e` | 761,764,363 B |

⇒ 只要启动器**重新生成一次内层 PVF**（`ensureInnerPVF`/`forceInnerPVF`/`-UpdatePVFDefault`），
**盘上所有存档行**的 `config_version` 就**全部变成历史值**，进角全被拒。

> 这正是 `next145` 埋的雷：145 让「重编 ⇒ 自动发布 `wireprobe-pvf.exe`」成为常态，
> 而发布若伴随内层重生成，就等于**每次更新都清空一次存档可用性**。

## 2. 止血（本次已落地）

### 2.1 设计决策（用户 2026-10-01 选定）

> 问题：修复范围？
> 用户答复：**「迁移 + 结构性根治（推荐）」**
> 问题：迁移白名单？
> 用户答复：**「仅允许已知的历史内层哈希（推荐）」**

即：**两段式** —— 先做一次**一次性数据迁移**把存量存档救回来（已落地），
再设计**结构性根治**把身份从内层哈希上解耦（本文 §3）。

### 2.2 一次性迁移：`MigrateSourceIdentity`

新增 `internal/storage/source_rebaseline.go`：

```go
// 已知的历史内层哈希白名单（只允许这些被重钉，别的来源身份一律不碰）。
var historicalInnerChecksums = []string{
    "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80",
}

// 六个「把内层哈希当身份钉着」的表/列。
var sourceIdentityTargets = []struct{ Table, Column string }{
    {"characters", "config_version"},
    {"character_world", "config_version"},
    {"character_quests", "config_version"},
    {"character_events", "config_version"},
    {"character_map_clears", "source_version"},
    {"character_quest_rewards", "source_version"},
}

func (s *Store) MigrateSourceIdentity(ctx context.Context, current string) (int64, error) {
    if len(current) != 64 { return 0, fmt.Errorf("invalid current source identity") }
    old := whitelist minus current          // 当 current 恰是某个历史值时不误伤
    if len(old) == 0 { return 0, nil }      // 幂等：没有可重钉的行
    var total int64
    for _, t := range sourceIdentityTargets {
        q := fmt.Sprintf(`UPDATE %s SET %s = $1 WHERE %s = ANY($2::text[])`,
            quoteIdent(t.Table), quoteIdent(t.Column), quoteIdent(t.Column))
        tag, err := s.DB.Exec(ctx, q, current, old)
        if err != nil { return total, fmt.Errorf("rebaseline %s.%s: %w", t.Table, t.Column, err) }
        total += tag.RowsAffected()
    }
    return total, nil
}
```

**三条硬约束**（都是踩出来的）：

1. **白名单**，不是「无脑全表 UPDATE」：只重钉 `historicalInnerChecksums` 里的值。
2. **`character_vaults` / `character_secondary_vaults` 显式排除** ——
   它们的 `config_version` = `fda6c33f…` = `vault.generated.json.source_sha256`，
   是**另一套来源身份**（保险箱数据版本，跟内层归档无关）。误伤会把保险箱一起打死。
3. **幂等**：第二遍 `RowsAffected=0`，不发日志、不报错。

### 2.3 接线点（`cmd/wireprobe/main.go:1206`）

放在 **quest 那组 `Migrate*` 之后** —— 因为
`character_quests` / `character_map_clears` / `character_quest_rewards`
是**在那组迁移里才 `CREATE TABLE IF NOT EXISTS` 出来的**：

```go
if characters != nil {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    n, rebaseErr := characters.Store.MigrateSourceIdentity(ctx, characters.Catalog.Source.Checksum)
    cancel()
    if rebaseErr != nil { log.Fatalf("source identity rebaseline: %v", rebaseErr) }
    if n > 0 { log.Printf("source identity rebaselined: %d stored row(s) ...", n) }
}
```

⚠️ **必须放启动期**（`log.Fatalf` 在非启动期 = 「进不去频道」）。

### 2.4 验收

| 项 | 判据 | 结果 |
| --- | --- | --- |
| 单测 | `TestMigrateSourceIdentityRebaselinesOnlyHistoricalInner` |
| ↳ 只钉白名单 | 6 行被重钉，`unknown` 身份**保留** | ✅ |
| ↳ vault 不碰 | `character_vaults` 两行仍 `fda6c33f…` | ✅ |
| ↳ 幂等 | 第二遍返回 0 | ✅ |
| ↳ 拒绝坏输入 | 非 64 hex 的 current 报错 | ✅ |
| 实机库存量 | `characters`12 + `world`12 + `quests`65 + `events`5553 + `map_clears`649 + `quest_rewards`35 = **6,326 行** | ✅ 全转 `b2b503b5…`，0 残留；vault 两组仍 `fda6c33f…` |
| 二进制 | `wireprobe-pvf.exe` = `wireprobe-handoff-source.exe` = `d35e7704…`（26,665,984 B） | ✅ |

## 3. 结构性根治（设计，待实施）

### 3.1 问题重述

现状把**两种语义完全不同的东西**塞进同一个字段：

| 语义 | 应该回答的问题 | 现状值 |
| --- | --- | --- |
| **运行时校验**：这批数据跟当前资源对得上吗？ | 「内层归档是哪一份」 | 内层 SHA256 |
| **存档身份**：这行存档属于哪个「内容世代」？ | 「存档是在哪份用户内容下产生的」 | ❌ 也用了内层 SHA256 |

内层 SHA256 **不是稳定的内容世代标识** —— 它随**每一个字节的重新打包**变化
（压缩器、字典顺序、时间戳都可能影响），但**玩家感知的内容没变**。

### 3.2 方案：身份改用「客户端三元组指纹」

用**决定内容的三个输入文件的指纹**当存档身份，内层哈希降级为**运行时校验专用**：

```
save_identity = H( DFO.exe.sha256 ‖ sk.dat.sha256 ‖ Script.pvf.sha256 )
```

**为什么是这三个**（不是内层哈希）：

- `ensure_inner_pvf.py` 的缓存判据本来就是这个三元组（docstring：「yes | yes | yes → reuse」）——
  **内层归档是这三者的纯函数**，三元组变 ⇒ 内层该变；三元组不变 ⇒ 内容语义不变（即便字节不同）。
- 玩家更新客户端（换了 `DFO.exe` / `sk.dat` / `Script.pvf`）⇒ **应当**算新世代，旧档该迁移。
- 服务端自己重打包（同一三元组）⇒ **不应**算新世代，旧档**必须**继续可用。

### 3.3 迁移路径（三阶段，可逐段实机）

| 阶段 | 动作 | 风险 |
| --- | --- | --- |
| **P1 双列共存** | 新增 `content_version` 列（= 三元组指纹），`config_version` 保留；写入时**两列都写**；**读取时以 `config_version` 为准**（行为不变） | 无（纯增量） |
| **P2 切换读侧** | 校验改读 `content_version`；`config_version` 退化为**观测列**；保留 `MigrateSourceIdentity` 兜底老档 | 中：需回归至少一条完整进角链 |
| **P3 清理** | `config_version` 从校验路径彻底移除（或改名 `inner_checksum_at_write` 明确其审计语义） | 低（此时已无人读） |

### 3.4 必须一起改的点（不能只改一处）

⚠️ **这类门禁是「逐个文件各写一遍」的**（`next145 §7.1` 的教训），
`ConfigVersion` 相关比对散布在 **12 个包、约 207 处**：

```
cmd/wireprobe        32    internal/character    23    internal/storage      21
internal/inventory   16    internal/loot         11    internal/quest         9
其余 cmd/*           6 处 + internal/{world,catalog,cashshop} 各 1
```

⇒ 根治**必须**走 P1「双列共存」把它们**平滑**引过去，不能一次性全改
（一次性全改 = 207 处同时风险暴露，违反 AGENTS.md §0.4 存档兼容最高优先级）。

### 3.5 待决问题（实施前需定）

1. **三元组指纹的算法/编码**：`H(a‖b‖c)` 还是 `H(JSON{a,b,c})`？是否要带算法前缀（`sha256:`）？
2. **`character_vaults` 是否也切换**？它现在用 `vault.generated.json` 的 sha256，
   语义上也是「内容世代」，理论上同构；但保险箱生成物由服务端产出，**可能**比客户端三元组更贴切。
3. **降级行为**：P2 后若 `content_version` 为空（老行），是**回填**还是**按 `config_version` 放行一次**？
4. 是否需要 `cmd/storagecheck` 增加一条「身份分布」审计（现在只能手写 SQL）。

## 4. 教训

1. **「启动成功」≠「能进游戏」**：本次前面 10 道门禁全过（能起服、能登录、能看到选角），
   第 11 道在**点进角色**才炸。验收必须包含**完整进角链**，不能止于「服务端起来了」。
2. **不可复现的哈希不能当身份**：凡是「同样的输入、不同的字节」的产物（压缩包、构建产物），
   它的哈希只配做**校验**，不配做**身份/主键/版本世代**。
3. **白名单迁移 > 全表迁移**：一次 `UPDATE ... WHERE col = ANY(whitelist)`
   比 `UPDATE ... SET col = current` 安全一个数量级 —— 后者会误伤一切「恰好用同名字段」的别的身份。
4. **枚举消费点再改语义**：`next145 §7` 的 wear 规则、本文的 vault 列，
   都是「同名字段、不同来源」。改一个字段的**含义**前，必须先把它的**所有读者**列出来。

## 5. 相关

- `next141-内层PVF自动生成与哈希门禁-设计.md`（四态门禁 + 自愈）
- `next142-PVF校验锁自动派生-根治.md`（三个运行时哈希门禁的放开）
- `next145-直读模式重编后发布PVF默认程序-根治.md`（发布默认程序；本文 §1.2 的雷就是它埋的）
- `internal/storage/source_rebaseline.go` / `source_rebaseline_test.go`
