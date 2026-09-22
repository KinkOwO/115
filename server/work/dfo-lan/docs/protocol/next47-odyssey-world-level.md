# next47 — 奥德赛角色世界区域等级门禁（[odyssey enter level]）

日期：2026-09-21
相关：`internal/catalog/world.go`、`internal/world/service.go`、`cmd/wireprobe/{world_flow,odyssey_teleport,menu_flow,special_warp,dungeon_flow,tutorial_flow}.go`、`configs/world.generated.json`、`docs/protocol/`（next37 实机日志）、`.workbench/travel-level-gate/HANDOFF.md`（根因取证工作间）

## 1. 结论摘要

45 级奥德赛角色点「传送」去 Storm Pass（风云径，town 43）无任何反应。根因**不是协议/通道问题，而是服务端世界区域等级门禁少了半条规则**：源地图 `[permission]` 段对奥德赛模式角色单列 `[odyssey enter level] 45`，与 `[need level] 50` 并存；服务端目录解析只读 `[need level]`，把 45 级奥德赛角色按 50 级门禁以原生等级拒绝（code 8）挡回，客户端收到等级拒绝后无可见提示 → 表现为"点了没反应"。

| 项 | 值 |
| --- | --- |
| 现象 | 45 级奥德赛角色传送 43/* 被拒（`area_refused reason="destination level requirement not met"` ×2，2026-09-21 实机日志） |
| 根因 | `[odyssey enter level]` 标签未解析、未参与入门校验 |
| 修复 | 目录解析 + `RequiredLevel(odyssey)` 语义 + 全调用点逐角色模式判定 + 78 区运行时目录补丁 |
| 影响面 | 610 个带等级标签区域中的 **78 个双标签区**（60 区 odyssey<need / 16 区 odyssey>need / 2 平）；532 个仅 need 区行为不变 |
| 存档兼容 | DB 全部 7 角色逐一审计：仅角色 7（qwes，奥德赛，45 级）受新规则影响且满足门槛（存档 38/1）；**无现存存档会被软锁定** |

## 2. 证据（根因闭环，详见工作间 HANDOFF.md §2）

1. **实机拒绝**：`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260921_211918_342173_next37/events.jsonl` 两次 `area_refused town=43 area=1`；同会话 `special_warp_prepared`（NOTI365 已发）证明客户端完整走完 CMD2261（预备）→ NOTI365（开船动画）→ CMD36（43/1）流程，服务端以 **code 8（原生等级拒绝）** 回包——协议通道正常，纯业务门禁拒绝。
2. **客户端自身判定**：45 级奥德赛角色在该区域**未置灰**且正常下发请求 → 客户端入区门槛 ≤45，与服务端 50 不一致。
3. **源数据**：`configs/world.generated.json` town 43 三个区域（43/0、43/1、43/2）的 `definition` 均同时含 `[need level] 50` 与 `[odyssey enter level] 45`（正好等于角色等级）。
4. **客户端二进制**：`analysis/dumps/xorstr_addr_to_text.json` 含 `[odyssey enter level]` XORSTR（115 客户端确实解析此标签）。
5. **DB 排除缓存假设**：PG 25438 确认角色 7 就是 45 级（`state->level`），且 `quest_flow.go`/`settlement_flow.go` 会刷新 `w.level`。
6. **模式判定**：角色 entry 事件 `entry_mode_byte: 5`（客户端自报奥德赛）；服务端统一走 `character.OdysseyRole(role)`（env `DFO_ODYSSEY_MODE` 优先，否则建号请求 `Options[10]==2`）。
7. **语义取证（关键）**：全量 78 个双标签区中 **16 个 odyssey>need**（如 38/5 need50/odyssey114、40/7 need15/odyssey73、39/2~4 need15/odyssey44）→ 该标签**不是"放宽值"，而是按模式各自生效的门槛**。规则只能取：奥德赛角色用 `[odyssey enter level]`（若标注），否则 `[need level]`。

## 3. 改动清单

### 3.1 目录解析（`internal/catalog/world.go`）

- `WorldArea.OdysseyEnterLevel uint32`（`json:"odyssey_enter_level,omitempty"`，0 = 未标注），字段顺序落在 `MinimumLevel` 之后（Go 重生成同位）。
- `parseWorldAreas` 新增 `[odyssey enter level]` 解析：单数值 cell ≥0 才入账；多 cell/非数值/负数 → `Pending "conditional odyssey level rule requires interpretation"`（与 `[need level]` 的条件式处理同构）。
- `WorldArea.RequiredLevel(odyssey bool) uint32`：`odyssey && OdysseyEnterLevel > 0 → OdysseyEnterLevel`，否则 `MinimumLevel`。

### 3.2 门禁语义（`internal/world/service.go`）

- `ValidatePosition(odyssey, level, p)`、`Transition(odyssey, level, old, r)`、`TransitionStrict(odyssey, level, old, r)`、内部 `transition(…, strict)` 全部增加 `odyssey bool` 首参；等级判断统一 `dest.RequiredLevel(odyssey)`。
- `Enter(ctx, account, id, odyssey, level, spawn)` 同样按模式校验（入城落点 + 读档后二次校验）。

### 3.3 调用点（`cmd/wireprobe`，全部逐角色模式，不硬编码）

| 文件 | 位置 | 说明 |
| --- | --- | --- |
| `world_flow.go` | 新增 `worldSession.odyssey()` 助手 | `character.OdysseyRole(w.role)` |
| `world_flow.go` | `enter()` → `Enter(…, character.OdysseyRole(role), …)`；CMD35 位置上报 `ValidatePosition(w.odyssey(), …)` | 逐角色：奥德赛角色在 odyssey 区内的位置上报不再被 need 门槛误拒 |
| `odyssey_teleport.go` | `areaTransition`（journal 分支 + `TransitionStrict`/`Transition`）、`teleportTransition`（`dest.RequiredLevel(w.odyssey())` + `ValidatePosition`） | 传送流（CMD2261/365/36）全链路 |
| `menu_flow.go` | `returnDestination` → `Transition(character.OdysseyRole(w.role), …)` | 赛丽亚房返回 |
| `special_warp.go` | `prepareSpecialWarp` | 特殊传送预备 |
| `dungeon_flow.go` | `dungeonGate` | 副本门 |
| `tutorial_flow.go` | `settleTutorialReturn` | 教学返回 |

> 注：交接初稿在部分调用点硬编码 `false/true` 字面量；已按 HANDOFF §4.2 统一改为逐角色 `character.OdysseyRole(w.role)`。否则奥德赛角色（如 45 级 qwes）进入 43/* 后，其 CMD35 位置上报会按 need=50 被整片拒绝，造成"进得去、站不住"。

### 3.4 运行时目录补丁（`configs/world.generated.json`）

`LoadWorld` 只读 JSON 扁平字段、不重解析 `definition`，因此把 78 个双标签区的值补丁进文件：

- 取值与解析器同逻辑（definition 中 `[odyssey enter level]` 单数值 cell）：恰好 **78** 区；43/0、43/1、43/2 = 45。
- 脚本 `.workbench/travel-level-gate/patch_world_catalog_v2.py`（HANDOFF §4.3 逐行算法，CRLF 稳健，无正则偏移）：key 行精确匹配 → 区内首个 `"minimum_level"` 行后插入 `"odyssey_enter_level": V,`（对齐 Go 重生成位）→ 断言行下为 `"kind"` 行。
- **三重验证**（全部通过后才落盘）：① JSON 可解析、78 处取值与 definition 计算一致、area 总数 694 不变；② 剔除 78 个插入行后行序列与原文完全一致（逐字节）；③ `source.checksum` = `7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80` 不变（变更会触发 `world/character source versions differ` 门禁）。
- **严禁整体重序列化**：文件为 Go `MarshalIndent` 输出、100% CRLF、6 空格缩进、含 HTML 转义；补丁只插行不改任何既有字节。
- 备份：`.workbench/travel-level-gate/world.generated.json.bak`（原始）与 `world.generated.json.pre-patch2.*.bak`（补丁前第二份；脚本曾误生成在 `configs/` 内，已移回工作间）。

### 3.5 测试

- `internal/world/service_test.go`：既有 16 处 `Transition` 调用补 `false` 首参；新增 `TestTransitionOdysseyLevelGate`（双标签区 need50/odyssey45：odyssey@44 拒 / odyssey@45 过 / 非 odyssey@45 拒 / 非 odyssey@50 过；无 odyssey 标签区两模式同用 need）。
- `internal/catalog/world_test.go`（新增）：
  - `TestParseWorldAreasOdysseyEnterLevel`：单值解析进字段、`RequiredLevel` 语义；条件式（多 cell）落 `Pending` 且不产生门槛。
  - `TestWorldCatalogOdysseyEnterLevel`（金标准）：运行时目录恰好 78 个双标签区、43/* = 50/45、area 总数 694、`source.checksum` 钉死——同时守护本次补丁不被后续重生成意外覆盖。
- `go test ./...` 全绿（30 包，含 `cmd/wireprobe` 21.7s）、`go vet ./...` 通过；`Build-Server.ps1` 重建 `bin/wireprobe-handoff-source.exe`。

## 4. 实机验证步骤（用户操作）

前置：游戏进程已停；用**一键启动器**（或 `启动游戏-奥德赛.cmd`）重新拉起，服务端用源码候选版 `bin/wireprobe-handoff-source.exe`。

1. **主验证**：45 级奥德赛角色（qwes，存档 38/1）从 38/1 点「传送」→ 风云径（Storm Pass，town 43）：应**开船动画走完、进城成功**（服务端事件 `area_change_sent town=43`；不再有 `area_refused`）。
2. **区内活动**：进城后正常移动/站立（CMD35 位置上报不再被 need=50 整片拒绝）。
3. **对照（剧情角色）**：剧情角色（如 35 级 Tiandi@40/4）经同一入口进 43 仍应被原生等级拒绝（need=50，客户端会置灰或不发请求）。
4. **返回**：从 43/* 经赛丽亚房/传送点返回 38/1 正常（`returnDestination` 核对模式门槛）。
5. **重登口径**：在 43/* 退出重登：入城落点校验按 odyssey=45 通过（`Enter` 的 `odyssey` 参数生效）。
6. **回归对照**：另一剧情角色（35 级 Tiandi）常规行走/传送/副本门不受影响。

若第 1 步仍被拒或客户端行为异常：读 `runtime/roles_*/events.jsonl` 的 `area_refused` reason；若出现"16 个 odyssey>need 区在奥德赛下连 45 级也不应进入"的动态证据，说明客户端另有剧情门禁，停止扩展并回查（HANDOFF §5，本任务不占用 C2S 盲试额度）。

## 5. 回滚方式

- 代码：`git revert`（涉及 world.go、service.go、6 个 cmd 文件、两个测试文件）。
- 目录数据：用 `.workbench/travel-level-gate/world.generated.json.bak` 或 `world.generated.json.pre-patch2.*.bak` 恢复 `configs/world.generated.json`（sha256 `C94B481169970F06E390BB27D70AAF86D56A085E42EE99A6A892EF2889233F8D` 为原始态指纹）。
- 候选 exe 重新 `Build-Server.ps1` 即可回到旧行为；归档基线 `wireprobe-dungeon39.exe` 从未被触碰。
