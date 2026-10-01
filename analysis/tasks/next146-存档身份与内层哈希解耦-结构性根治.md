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

### 2.1a ⚠️ 首次「手工白名单」实现当场失败（2026-10-01 15:09 实机）

手工白名单上线后，玩家**换客户端**（汉化补丁 → 英文原版）⇒ 内层重生成 ⇒
新哈希 `be95d64ee120248ae503194d2f61743ef74409a8ff999a4a986ca2e0bccf69b0`，
**不在白名单里** ⇒ 迁移**静默不生效** ⇒ 再次「无法进入角色」。

会话取证：`gateway.err` 里 `source=be95d64e…`，且**完全没有任何** `source identity rebaselined` 行；
库内仍是 `b2b503b5…` ⇒ 12 角色全部身份不符。

**教训**：手工白名单在「**每次换客户端都要改代码并重新编译**」的意义上**不可持续**，
而且它的失败是**静默的**（只表现为「进不去角色」，没有任何日志）。这正是 §3 结构性根治要解决的问题；
在根治落地之前，先把白名单改成**自录式**（见 §2.2a）。

### 2.2 一次性迁移：`MigrateSourceIdentity`

> ⚠️ 下方为**首版实现**（手工白名单），已被 §2.2a 的自录式实现取代。保留作为演进记录。

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

### 2.2a ★ 现行实现：**自录式**（取代手工白名单，2026-10-01 15:1x）

手工白名单当场失败（§2.1a）⇒ 改为**自录**。核心思想：

> **凡是本服务端自己用过的 inner 哈希，都自动成为可迁移来源。**

落成一张小表 `source_identity_history(checksum PK, first_seen_at, last_seen_at)`：

```go
func (s *Store) MigrateSourceIdentity(ctx context.Context, current string) (int64, error) {
    if len(current) != 64 { return 0, fmt.Errorf("invalid current source identity") }
    if err := s.migrateSourceIdentityHistory(ctx); err != nil { return 0, err }
    // 1) 自录：当次内层哈希进入历史表（ON CONFLICT 只更新 last_seen_at）。
    s.DB.Exec(ctx, `INSERT INTO source_identity_history(checksum, first_seen_at, last_seen_at)
        VALUES ($1, now(), now())
        ON CONFLICT (checksum) DO UPDATE SET last_seen_at = now()`, current)
    // 2) 取历史值（排除 current），把命中它们的存档行重钉到 current。
    old := /* SELECT checksum FROM source_identity_history WHERE checksum <> current */
    for _, t := range sourceIdentityTargets {
        q := fmt.Sprintf(`UPDATE %s SET %s = $1 WHERE %s = ANY($2::text[])`, ...)
        tag, _ := s.DB.Exec(ctx, q, current, old)
        total += tag.RowsAffected()
    }
    return total, nil
}
```

`bootstrapInnerChecksums` 是**引导值**（`7ef2db59…`、`b2b503b5…`），只为**存量库**服务：
历史表在第一次跑迁移时才创建，若不把那批「迁移上线前就存在」的值同时灌入，就会漏掉第一代存档。
新库不需要它们（启动当场就把 `current` 写进去）。

**为什么安全**：本表**只**记录调用方传入的 `Catalog.Source.Checksum`（内层哈希），
vault 的 `fda6c33f…` 走的是**另一条写入路径**，永远不会进这张表；且 vault 表不在
`sourceIdentityTargets`。双层保证。

**运维含义**：以后换客户端/重生成 inner **不需要改代码重编**——下一次启动自动完成迁移。

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
| 单测（自录版） | `TestMigrateSourceIdentityRebaselinesOnlyHistoricalInner` |
| ↳ 重钉两代历史值 | `historical` + `previous` 各被重钉（10 行） | ✅ |
| ↳ `unknown` 身份保留 | 13 行仍为 `aaaa…` | ✅ |
| ↳ vault 不碰 | `character_vaults` 两行仍 `fda6c33f…` | ✅ |
| ↳ 自录 | `current` 出现在 `source_identity_history` | ✅ |
| ↳ 幂等 | 第二遍返回 0 | ✅ |
| ↳ 拒绝坏输入 | 非 64 hex 的 current 报错 | ✅ |
| 实机（首版白名单） | 存量 6,326 行转 `b2b503b5…`，vault 两组仍 `fda6c33f…` | ✅ |
| 二进制 | `wireprobe-pvf.exe` = `wireprobe-handoff-source.exe` = `d177a63c…` | ✅ |

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

### 3.6 「玩家改一次 PVF 就得改程序吗」—— 答：**不，现在这个设计是错的**

业主 2026-10-01 15:24 的原问。**正确答案是「不应该」。** 逐层拆开：

#### 3.6.1 现在为什么会「改一次就得改程序」

因为**两道语义不同的门禁被塞进同一个字段**（`config_version`），而它的取值是
**每次打包都会变的内层 SHA256**：

| 场景 | 玩家感知 | 现状行为 | 应该是 |
| --- | --- | --- | --- |
| 服务端自己重打包（同一客户端三元组） | **内容没变** | ❌ 存档全部失效 | ✅ 存档照用 |
| 玩家自己改 `Script.pvf`（加/减/调物品、技能） | **内容变了** | ❌ 存档全部失效 + 要改代码白名单 | ⚠️ 见 §3.6.2 |
| 玩家换客户端版本（`DFO.exe` 变了） | 内容大改 | ❌ 同上 | ⚠️ 见 §3.6.2 |

第一行是**纯 bug** —— 「我用同样的料重做了一遍饭」不该导致「昨天的存档作废」。
本轮止血（§2）已经用**自记录白名单**把第一行压住了，但那是**补丁不是设计**。

#### 3.6.2 玩家改 PVF：分两类，必须区别对待（这是关键）

「玩家改 PVF」根本不是一种情况，硬要一种策略覆盖它必然错：

**(A) 改「数值类内容」—— 应当无条件兼容，不许拒档。**
调掉率、改物品属性、加 buff、改技能倍率、调药水回复量……

- 这类改动**不产生新的存档状态形状** —— 存档里存的还是「角色 ID、任务 ID、物品模板号、数量」，
  这些**键**没变，只是**解析规则**变了。
- 后果是**玩家自己承担**的（他自己改的，装备变强变弱他自己认），
  **服务端没有任何理由替他把档作废**。
- ⇒ **正确做法：这类改动根本不参与身份判定。** 不做校验，不迁移，直接放行。
  所谓「兼容玩家修改 PVF」在这个层面**不是要额外做什么，而是要「不做什么」**。

**(B) 改「结构/目录类内容」—— 该拦，但拦的必须是「我读不懂」，不是「哈希不匹配」。**
删掉某物品模板、改 PVF 目录树、改任务链结构、改客户端资源索引……

- 这类改动会让**存档里已有的引用失效**（挂了一个不存在的模板号）。
- 危险的是**静默数据损坏**：客户端发 `item 12345`，服务端按新表解不出来 → 崩溃 / 静默丢件。
- ⇒ 该拦的是**「按引用查不到」**（`catalog[template]` 不存在），
  **不是**「哈希跟我预期的不一样」。
- 现状的 `q.ConfigVersion != s.Catalog.Source.Checksum` 拦错了对象 ——
  它对**任何**哈希变化都拦，对**真正危险的引用失效**反而不专门拦（靠后续 err 兜）。

#### 3.6.3 结论：门禁分级，不是一刀切

| 层 | 判定对象 | 触发条件 | 现状 | 目标 |
| --- | --- | --- | --- | --- |
| **L1 存档身份** | 这行档属于哪个内容世代 | **客户端三元组指纹**（§3.2） | ❌ 用内层哈希 | ✅ 三元组不变就不拒 |
| **L2 引用完整性** | 存档里的引用在新目录里还解得出吗 | **按 ID 查表成功/失败** | ⚠️ 隐含在 err 里 | ✅ 显式化，缺了就**这条**降级 |
| **L3 运行时校验** | 当前资源是不是我以为的那份 | 内层 SHA256 | ✅ 已有 | ✅ **仅用于启动期自检**，不参与存档门禁 |

⇒ **玩家的 PVF 改动落在 L2**，而 L2 的正确反应是**逐条降级而不是整体拒绝**
（例如「任务 3151 的模板查不到了 ⇒ 丢弃这一条 accepted 记录」，而不是「整个角色进不去」）。

#### 3.6.4 这条原则要写进判据

> **凡是「同样的输入、不同的字节」的产物（压缩包、构建产物），
> 它的哈希只配做「校验」，不配做「身份」。**
> **凡是玩家自己能改的东西，服务端不许拿它的哈希当拒档理由。**

对照现状：内层 PVF 同时犯了这两条。这是 §3 根治的真正目标。

## 3.7 实施记录（2026-10-01 15:30–16:00，已落地）

业主拍板：「目标是根治，需要兼容玩家对 PVF 的修改，而且切换原版客户端 exe 正常也不应该
出现什么无法进入游戏才对，**因为存档的契约是服务端定义的**。」

最后一句是**设计判据**：存档契约由服务端定义 ⇒ 身份必须是**服务端自己的常量**，
与客户端资源（PVF / exe / sk.dat）**一律无关**。

### 3.7.1 落地形态

| 组件 | 内容 |
| --- | --- |
| **新包** `internal/savecontract` | `Identity()` = `SHA256("dfolan-save-contract:v1")` = `c638346f…fc400`；`Generation` 常量；`IsIdentity()` 形状校验；`CatalogIdentity(any)` 存档身份取值入口 |
| **为什么是 64 位 hex** | 现有代码大量 `len(x) != 64` 形状的入参校验（vault / bag / CreateCharacter / CommitCharacterEvent）。让契约身份同形状 ⇒ **那些校验一行不改**，语义却换了 |

### 3.7.2 代码改动（一次性、机械、可复核）

| 面 | 处数 | 做法 |
| --- | --- | --- |
| **读侧身份比较** | 58 处 / 47 文件 | `X.Source.Checksum` → `savecontract.CatalogIdentity(X)`，只改「行含 `ConfigVersion` 且对面是 `Source*`」的比较；vault 与目录自校不动 |
| **写侧身份传参** | 19 处 / 14 文件 | `CommitCharacterEvent(..., <目录身份>, ...)` 的 version 实参 → `savecontract.Identity()` |
| **存档归一迁移** | 1 处 | `MigrateSourceIdentity(ctx, innerChecksum)` → `MigrateSaveIdentity(ctx)`：判据从「白名单字面值」改成「**形状**（`~ '^[0-9a-fA-F]{64}$'`）」⇒ 任何来源的旧身份一次归一，不再有「没记录就静默失效」 |
| **配置清理** | 20 文件 | 92 处硬编码 `7ef2db59…`（含嵌套快照对象 `source.checksum`）清零；**保留** `vault.generated.json` 的 `fda6c33f…`（服务端生成物身份）与自然语言/路径类 source |

### 3.7.3 与旧方案的本质差别

| | 旧（自记录白名单） | 新（契约身份） |
| --- | --- | --- |
| 身份取值 | 当次**内层哈希** | 服务端常量 `c638346f…` |
| 服务端重打包 | 需迁移一次（破坏性改档） | **不动** |
| 玩家改 PVF | 需迁移一次 | **不动** |
| 玩家换 exe | 需迁移一次；换了没记录的版本就**静默失败** | **不动** |
| 迁移语义 | 「追踪」内层哈希 | 「归一」到契约身份 |

### 3.7.4 踩坑（本轮实测）

1. ⚠️ **`source` 字段在 Go 里是 `pvf.ArchiveSnapshot`（对象），不是字符串。**
   第一版清理脚本把 `"source": {"checksum": "…"}` 整块换成 `""` ⇒
   14 个测试 `json: cannot unmarshal string into Go struct field Characters.source` 全红。
   正确做法：**保留对象形状，只清 `checksum`**。
2. ⚠️ **大文件 + 写盘顺序**：`shop-vault-release.json`（179 MB）在 `json.load` 失败前
   已被 `open(w)` 截断成 0 字节。修法：**先解析校验、再写盘**，且用临时文件 + `os.replace` 原子替换。
3. ⚠️ 超过 ~400 MB 的配置（`dungeons.full.json` 742 MB）脚本跳过 —— 它本来就是 `"source": ""`，无需处理。

## 3.8 实施记录（2026-10-01 16:2x–17:1x，已落地）

业主第二轮拍板：**「完整根治：接入 savecontract 契约身份」** + **「先 pg_dump 备份相关 6 张表，再做身份迁移」**。
背景是 15:05 换客户端（汉化 → 英文原版）后内层变 `be95d64e…`，而白名单只认 `7ef2db59…` ⇒
迁移**命中 0 行且静默**，12 个角色再次全部进不去。

### 3.8.1 与旧方案的关键差别：身份取值点

| 面 | 旧 | 新 |
| --- | --- | --- |
| 目录侧身份取值 | `X.Source.Checksum`（内层哈希） | `X.Source.SaveIdentity()`（= `savecontract.Identity()`） |
| `pvf.ArchiveSnapshot` | 只有 `Checksum` | 新增方法 `SaveIdentity()`（任何快照可用，**调用点不需要新增 import**） |
| Odyssey 家族 | `s.Odyssey.Source` / `catalog.OdysseySource` 当身份 | 身份比较改 `savecontract.Identity()`；**目录自校仍用真哈希** |
| 迁移判据 | 手工白名单（`historicalInnerChecksums`） | **形状**：`~ '^[0-9a-fA-F]{64}$'` AND `<> current` |
| 迁移日志 | `if n > 0` 才打（**静默失败**） | 无论 0 与否都打，且带 `contract` 与 `inner archive` 两个值 |

### 3.8.2 分级（哪些改、哪些**不许**改）

| 层 | 语义 | 本次处置 |
| --- | --- | --- |
| **L1 存档身份** | 这行档属于哪个契约世代 | 全部改读 `SaveIdentity()` / `savecontract.Identity()` |
| **L3 目录自校** | 这份目录/记录是不是从这份归档来的 | **保留真哈希**（`a.Snapshot().Checksum != index.Source.Checksum`、`route.Source`、`scene.Source`、`overlay.Source`、`doc.Source` …） |
| **L4 vault 身份** | 保险箱生成物（`vault.generated.json` 的 `fda6c33f…`） | **一律不动** |

⚠️ 最危险的错误是把 L3 与 L1 混为一谈：`odyssey_teleport.go` 的
`routes.Source.Checksum != catalog.OdysseySource` 若改成 `!= savecontract.Identity()`，
会因「快照 checksum 是哈希、身份是常量」而**恒为真** ⇒ 传送门全部拒绝（本轮实测踩过一次）。

### 3.8.3 代码改动清单

| 面 | 处数 | 说明 |
| --- | --- | --- |
| `pvf.ArchiveSnapshot.SaveIdentity()` | 1 | `internal/catalog/pvf/archive.go` 新增方法 |
| 读侧身份比较 | ~95 | `ConfigVersion` 相邻比较、Odyssey 家族、`s.Learning.Source`、`wear`/`knight_deck`/`reinforcement` 等 |
| 写侧身份实参 | ~25 | `CommitCharacterEvent` / `CommitCharacterPremiumEvent` / `CommitQuestReward` / `AcceptQuest(Groups)` / `RecordQuestMapClear` / `MapClear` / `ClearActQuests` / `CompleteQuestObjective` / `LoadWorld` / `CompletedQuestIDs` / `CreateCharacter` |
| 服务端生成收据 | ~14 | `ClearReceipt` / `FinishReceipt` / `seekingGrantReceipt` / `CardPlan` / `MoonRewardPolicy` / `BoxReceipt` / 商城 `out.Source` / `session.Currency.Source`：写入与比较同时改 |
| 目录身份令牌（string） | 4 | `BleedingMineRewards.Source`、`BlackPurgatoryRewards.Source`、`npcpresence.Index.Source`、`quest.Index.Source` 构造处 |
| 删除「跨目录同源」假门禁 | ~12 | `s.Rules.Source` / `s.BagRules.Source` / `s.Shields.Source != role.ConfigVersion`（契约模型下无意义，且会锁死玩法） |
| 迁移实现 | 1 | `MigrateSourceIdentity` → `MigrateSaveIdentity`（形状判据 + 幂等） |
| 接线与日志 | 1 | `cmd/wireprobe/main.go`：无论 0 与否都打 `save identity normalized: N row(s) -> contract … (inner archive …)` |

### 3.8.4 踩坑（本轮实测，务必记住）

1. **批量文本替换必须按语义分流，不能按变量名。** 第一版脚本把「`X.Source.Checksum` 全部换成 `SaveIdentity()`」
   连**赋值左侧**和 **L3 自校**一起改坏 ⇒ 50+ 测试红。处置：全量反向还原，改用两类精确规则
   （只动与 `ConfigVersion` 相邻的、只动 `Commit*/Accept*/MapClear/…` 实参的）。
2. **`odysseySource()` 是双用令牌**（既给目录快照当校验值，又当运行时身份）。整体改成返回契约身份
   ⇒ 测试立即报 `full equipment source mismatch`。正确做法：令牌保持真值，**只在身份比较处**换常量。
3. **测试夹具 = 存档行。** 生产代码要求 `role.ConfigVersion == Identity` 后，fixture 里任何
   `ConfigVersion: "version"` / 哈希都会让功能被拒 ⇒ 期望通过的夹具必须改；`stale`／expected-reject 的用例保留。
4. **配置里的内层哈希不能裸清。** 清空 `configs/*.json` 的 `source`/`checksum` 会让
   `len(Source.Checksum) != 64` 与快照比对直接失败。本轮**已全部还原**（`git diff -- configs/` 为空）；
   清零必须与「留空即派生」的加载器改造**同时**做。
5. **删门禁要连带删变量**（`source := s.Catalog.Source.Checksum` 会变成未使用变量 ⇒ 编译失败）。
6. **「双用令牌」的构造处绝不能改成契约身份。** `BleedingMineRewards.Source` / `BlackPurgatoryRewards.Source` /
   `npcpresence.Index.Source` / `quest.Index.Source` 既**被启动期**校验（`mine.Source != c.Source.Checksum`，
   `cmd/wireprobe/main.go:1028`）**又被运行期**当身份比较。把构造处改成 `SaveIdentity()` ⇒ 启动即
   `log.Fatal("赤红铁矿奖励表与当前角色配置版本不一致")`（启动器表现为「启动脚本异常退出: exit status 1」）。
   **正确做法：令牌构造保持真哈希，只改运行期身份比较**（`role.ConfigVersion != savecontract.Identity()`）。
   判据：**一个字段若同时出现在「目录/记录之间的校验」和「与存档行的比较」中，它就是双用的**，
   改之前先把两类读者都列出来。
7. **与上游合并时，上游新增的代码也可能含「目录身份比较」。** 本轮实测：上游 `internal/character/prewarm.go`
   的 `role.ConfigVersion != s.Learning.Source.Checksum`，以及 `prewarm_test.go` 用任意哈希当 `ConfigVersion`。
   合并后必须**重跑全量测试**，并 grep 一遍 `ConfigVersion\s*(!=|==)\s*\S*\.Source\.Checksum`，
   把上游新代码里的这类比较一并换成 `SaveIdentity()`（注意区分：`learning_catalog.go` 的**加载期**校验
   `c.Source.Checksum != source` 属于 L3，必须保持真哈希）。

### 3.8.5 验收

| 项 | 判据 | 结果 |
| --- | --- | --- |
| `go build` / `go vet` / `go test -count=1 ./internal/... ./cmd/...` | 无错误、无告警、全绿（31 包） | ✅ |
| 迁移单测 | `TestMigrateSaveIdentityNormalizesAnyHistoricalIdentity`（形状归一 / 非 hex 保留 / vault 不动 / 幂等 / 坏输入） | ✅ |
| 数据迁移 | 6 张表 **6,364 行** → `c638346f…fc400` | ✅ |
| vault 保护 | `character_vaults` 12 行、`character_secondary_vaults` 3 行仍为 `fda6c33f…` | ✅ |
| 幂等 | 第二遍 0 行 | ✅ |
| 备份 | `runtime/backup-identity-20261001/identity-tables.sql`（2.1 MB，8 表） | ✅ |
| 程序发布 | `bin/wireprobe-pvf.exe` = `bin/wireprobe-handoff-source.exe`，SHA256 `CEB3B12A806D17BB62512E3961CB0F14E87821F1D04C32F8CC02368F1C5206E1`（旧件备份 `.previous-20261001-170522`） | ✅ |
| 实机进角链 | **待用户手动验证**：选角 → 进城 → 换装 | ⏳ |

### 3.8.6 遗留（明确不做/待办）

1. **6 个大配置**（`items.index.json` 66 MB、`dungeons.full.json` 294 MB、`equipment-full.index.json` 51 MB、
   `booster-catalog.json` 84 MB、`shop-next-candidate.json` 163 MB、`shop-vault-release.json` 163 MB）
   里的内层哈希**未清**（需流式替换）。当前不影响运行（加载器容忍），但与 §3.7.2 的「92 处清零」仍有差距。
2. **`configs` 清零必须配套加载器「留空即派生」**（`world.go` / `equipment_full.go` / `town.go` 等十余处
   `len(Source.Checksum) != 64` / `!= source`），属**独立**改造，不能与数据迁移混做。
3. `character.State.SourceSHA256`（职业文件来源 `prof.RawSHA256`）**未纳入**本轮：它是**单文件内容哈希**，
   与内层归档无关（重打包不变、改职业文件才变）；若玩家改职业文件导致觉醒/转职被拒，按
   「L2 逐条降级」另行处理。
4. `Generation` 仍为 1；将来改存档 schema 时按 `savecontract.Generation` 规则 bump 并写定向迁移。

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
