# next49 — `[odyssey enter level]` 只降不升（西海岸 40/0 误拦 20 级奥德赛角色）

日期：2026-09-22
相关：`internal/world/service.go`、`internal/catalog/world.go`、`internal/world/service_test.go`、`cmd/wireprobe/odyssey_enter_level_test.go`、next47/next48（奥德赛门禁落地）

## 1. 结论摘要

20 级奥德赛角色 `gugugaga`（角色 11，DB `state.level=20`，`creation_mode=2`）进不去西海岸（town 40/area 0），客户端提示 DSTR 30069「You must be Level 15 to go to West Coast」。根因是 next47 把 `[odyssey enter level]` 理解成"替换 `[need level]`"，而 40/0 属于 16 个 **odyssey>need** 的区域（need 15 / odyssey 35），replace 语义把 20 级奥德赛角色按 35 级门槛以 code 8 误拦。实机证据表明客户端的有效门槛是 **min(need, odyssey)**：客户端在 20 级就发出了移动请求（服务端收到 CMD 36），且拒绝提示里填的等级是 15（need 值）而不是 35。

## 2. 证据（实机 + 数据闭环）

1. **实机拒绝**：`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260922_191044_617535_next37/events.jsonl` 三次 `area_refused town=40 area=0 reason="destination level requirement not met"`（11:12:38/43/45），每次前面都有 `special_warp_prepared`（NOTI365，角色 11）——客户端完整走完 CMD2261 → NOTI365 → CMD36 流程，即客户端本地门槛 ≤20。
2. **客户端提示填 15**：DSTR 30069 的 `%d` 是客户端自己读出的门槛；next47 已证明奥德赛角色的提示数字跟客户端有效门槛走（斯顿雪域填的是 odyssey 45 而非 need 50）。本次填 15 ⇒ 客户端对 40/0 的有效门槛是 15。
3. **源数据**：`configs/world.generated.json` 40/0 的 definition 为 `[need level] 15 [odyssey enter level] 35`，且该区带 `[phase]` 段（WestCoast_storm / WestCoastAllianceCamp 变体）——高出的 odyssey 值约束的是阶段变体，不是入门。
4. **请求路径**：`odyssey_journal_routes.json` 的 29 个日志节点不含 40/0，实机走的是 `specialWarpPending → teleportTransition` 分支，唯一可能返回 `world.ErrLevel` 的就是 `RequiredLevel(dest, odyssey=true)` 取到 35（DB 核实角色就是 20 级，w.level 无其它写入点）。
5. **min 语义与全部既有证据一致**：43/* need50/odyssey45 → min=45（next47 实机：45 级放行、提示填 45）；40/0 need15/odyssey35 → min=15（本次实机：20 级放行、提示填 15）。16 个 odyssey>need 区在 min 语义下退化为 need，奥德赛角色不会比剧情角色更严——与奥德赛"加速剧情"的设计一致（赫顿玛尔后街 39/2~4 need15/odyssey44 若按 replace 会要求 44 级才能进，显然荒谬）。

## 3. 改动

- `internal/world/service.go`：`RequiredLevel(a, odyssey)` 改为 min 语义——`odyssey && OdysseyMinimumLevel > 0 && OdysseyMinimumLevel < MinimumLevel` 时才用 odyssey 值，否则用 `[need level]`。`RestorationLevel` 本来就是 min，两者现一致（注释已注明）。
- `internal/catalog/world.go`：`OdysseyMinimumLevel` 字段注释更新为 min 语义与两条实机证据。
- 解析、78 区目录补丁、`source.checksum` 均未动；普通角色行为不变（仍只用 `[need level]`）；60 个 odyssey<need 区行为不变。

## 4. 测试

- `internal/world/service_test.go` 新增 `TestOdysseyGateNeverRaisesEntry`：钉死 40/0 need15/odyssey35、odyssey 门槛=15、20 级放行 / 14 级（两模式）仍拒。
- `cmd/wireprobe/odyssey_enter_level_test.go` 新增 `TestOdysseyWestCoastTeleportGate`：复刻实机包形态（39/0 → 40/0、special warp 预备、Flag=5 尾 0），20 级奥德赛角色放行、14 级仍 code 8。
- 既有 `TestOdysseyStormPassJournalTeleportGate`（43/* 45 级放行 / 44 级拒）与全部 30 个包 `go test ./...`、`go vet ./...` 全绿。
- `bin/wireprobe-handoff-source.exe` 已用修复后的源码重建。

## 5. 实机验证（用户操作）

重启服务端后，20 级奥德赛角色 `gugugaga` 从赫顿玛尔点「移动」去西海岸：应直接进城（服务端事件 `area_change_sent town=40`，不再有 `area_refused`）。对照：14 级以下角色仍应被 15 级门槛拦下。

## 6. 回滚

改动均在 Git 内（上述 4 个文件 + 本文档），`git revert` 即可。
