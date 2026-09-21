# next48 — 奥德赛角色城镇准入：服务端漏读 `[odyssey enter level]`

## 现象

110 级后期主线外的**奥德赛（Arad Odyssey）剧情角色** `test-jh`（角色 10，45 级）在赫顿玛尔后街
（39/5）打开第 1 章「冒险的开始」→「雪山的主人」，点击「移动」按钮后：

- 客户端提示（DSTR 535 `You must be at Level %d to enter %s.`）：
  **「您必须是45级别才能进入斯顿雪域。」**，同一条提示出现两次；
- 剧情链断在这里，无法继续游玩。

## 证据链

来源1 — 服务端会话日志（`runtime/roles_*_20260921_221101_*/events.jsonl`，角色 10 `test-jh`）：

```json
{"kind":"character_mode_projection","character_id":10,"entry_mode_byte":5,"name":"test-jh","odyssey_pilot":true}
{"kind":"world_position_saved","character_id":10,"position":{"town":39,"area":5,"x":651,"y":267},"request":35}
{"kind":"special_warp_prepared","character_id":10,"id":365,"plain_hex":"04010a0000000000"}
{"kind":"area_refused","area":1,"reason":"destination level requirement not met","town":43}
```

两次点击各产生一组「`special_warp_prepared` → `area_refused`」，拒绝原因是
`world.ErrLevel`（`cmd/wireprobe/world_flow.go` 回 code 8，即客户端的原生等级拒绝码）。

来源2 — 客户端本地已放行，服务端拒绝：客户端在发出 `special warp` 预备与 CMD 36 之前自己已做过同级判定
（否则不会发请求）；提示里的 **45** 就是客户端本次判定使用的门槛值。

来源3 — 源数据的两个门槛。`configs/world.generated.json` 的 `43/*`（`map/cataclysm/town/stormpass/`）：

```
[permission]
[need level]           50
[odyssey enter level]  45
[/permission]
```

服务端 `internal/catalog/world.go` 只解析 `[need level]`（50），把
`[odyssey enter level]` 当「已知但忽略」的 permission 标签跳过，于是 45 级奥德赛角色被 50 级门槛拒绝。

来源4 — 客户端存在该字段，并按「是否为阿拉德奥德赛用户」切换：
`analysis/dumps/xorstr_addr_to_text.json` 有 `[odyssey enter level]`（`0x14B340968`）、
`[is arad odyssey user]`（`0x14A878678`）、`[arad odyssey]`、`[odyssey connectable channel type]`。

来源5 — 该门槛就是奥德赛剧情等级阶梯：`internal/character/odyssey_journal_routes.json` 的 29 个日志节点
目标与各城镇的 `[odyssey enter level]` 严格对应且递增——`38/2`=1、`40/4`=35、`41/2`=38、`42/3`=42、
`39/5`=44、**`43/1`=45**、`12/0`=47、`14/2`=56、`22/4`=61、`31/2`=65、`40/7`=73、`140/2`=85、
`168/6`=96、`182/1`=99、`184/2`=100、`203/1`=103、`210/1`=105、`211/1`=106、`219/1`=108、
`224/1`=110、`235/0`=111、`233/1`=113。第 6 个节点正是斯顿雪域，等级 45 —— 与提示里的 45 完全一致。

## 实现

1. `internal/catalog/world.go`
   - `WorldArea` 新增 `OdysseyMinimumLevel`（`odyssey_minimum_level`，省略零值），由 `[permission]` 块里的
     `[odyssey enter level]` 解析；非数值/负数视为该项未解析（pending），**不猜值**。
   - `LoadWorld` 对旧导出文件按需回填：旧目录保留完整 `definition`，不必为了一个新增字段重导 31 MB 配置
     （与 `characters.go` 的既有回填规则一致，`source.checksum` 不变，无存档/配置迁移）。
2. `internal/world/service.go`
   - `RequiredLevel(area, odyssey)`：奥德赛角色在源里定义了 `[odyssey enter level]` 时用它，否则仍用
     `[need level]`。**进入**（CMD 36 传送、日志传送、地图传送点、赛丽亚房间返回、教程落地）走这个门槛。
   - `RestorationLevel(area, odyssey)`：**已经在里面**的位置（登录恢复、CMD 35 位置报告、special warp 预备、
     副本门所在区域）取两个门槛中较小的那个。升级后服务端改用奥德赛门槛也不会把已保存位置的角色挡在门外
     （否则会出现无法登录的死锁）。
   - `ValidatePosition` 增加 `odyssey` 参数；新增 `ValidateRestoredPosition`；`Enter` 增加 `odyssey` 参数。
3. `cmd/wireprobe`
   - `worldSession.odyssey` 在 `enter()` 时按 `character.CreatedAsOdyssey(role)`（角色自身创建标记 `option[10]=2`）
     落定，`clearSelectedWorld` 清空；
   - `character` 新增 `CreatedAsOdyssey`，`OdysseyRole` 改为「有 `DFO_ODYSSEY_MODE` 就按它、否则回落标记」的薄包装。
     **准入判定用 `CreatedAsOdyssey`**：客户端按角色自身标记判定，而 `DFO_ODYSSEY_MODE` 是启动器的全局档位
     （`115us-dfolauncher` 默认「按角色」不注入该变量，只有开发者页的「强制剧情 / 强制奥德赛」才注入 `0` / `1`；
     旧的 `启动游戏.cmd` 固定 `0`、`启动游戏-奥德赛.cmd` 固定 `1`）。若准入跟着这个全局开关走，强制剧情档下
     45 级奥德赛角色仍会被 50 级门槛拒绝，与客户端提示的 45 自相矛盾——按角色标记判定则两种档位都对齐客户端；
   - 传送路径（`odyssey_teleport.go` 的日志分支与 `teleportTransition`）改用 `RequiredLevel`；
   - `command 35` / `special_warp` / `dungeon gate` 改用 `ValidateRestoredPosition`；教程落地、返回房间用
     `ValidatePosition(odyssey)`。

## 影响面

- **普通角色行为完全不变**（仍只用 `[need level]`）。
- 奥德赛角色的城镇准入与客户端一致：`configs/world.generated.json` 里 78 个区域带
  `[odyssey enter level]`，其中 60 个比 `[need level]` 宽松、16 个更严、2 个相等。
  更严的 16 个（`38/5`、`39/2..5`、`40/0..4`、`40/7`、`41/*`、`42/*`）正是奥德赛剧情尚未到达的等级段，
  客户端本来就会拒绝，服务端拒绝不再与客户端提示矛盾。
- 已保存位置的恢复放宽，不会产生新的登录死锁。

## 测试

- `internal/catalog/world_test.go`：`[permission]` 双门槛解析、非数值门槛不猜值、旧目录回填（含无该标签的区域保持 0）。
- `internal/world/service_test.go`：`43/0..2` 的 `RequiredLevel`/`RestorationLevel`；45 级奥德赛角色可进 43/1、
  44 级被拒、45 级普通角色仍被 `[need level]` 50 拒绝。
- `internal/character/odyssey_test.go`：`CreatedAsOdyssey` 不受 `DFO_ODYSSEY_MODE` 影响，而 `OdysseyRole` 仍按开关走。
- `cmd/wireprobe/odyssey_enter_level_test.go`：复刻本次实机请求（39/5 → 43/1），覆盖两条真实路径——
  `DFO_ODYSSEY_MODE=1` 下的日志传送分支，以及 `DFO_ODYSSEY_MODE=0` 下 `special_warp_prepared` → CMD 36 的
  通用传送；奥德赛 45 级放行 / 44 级 `world.ErrLevel` / 普通角色仍 `world.ErrLevel`。
- `go test ./internal/... ./cmd/...` 与 `go vet ./internal/... ./cmd/...` 通过。
  （`go test ./...` 会踩到 `runtime/update-backup/**` 下的历史源码副本，与本改动无关。）

## 实机验收（2026-09-21，用户操作，已通过）

1. 用 `DFO-115US单机一键启动器.exe`（默认「按角色」档，不注入 `DFO_ODYSSEY_MODE`）重启到源码候选版：
   `server/launcher.local.json` 的 `server_binary` 已指向 `work/dfo-lan/bin/wireprobe-handoff-source.exe`
   （启动器在源码有更新时也可能自动重编该候选版，结果一致）。
2. `test-jh`（45 级奥德赛角色）在 39/5 打开第 1 章「冒险的开始」→「雪山的主人」，点「移动」：
   **直接过图到斯顿雪域（43/1）**，不再出现「您必须是45级别才能进入斯顿雪域」，剧情继续推进。✅
3. 待补回归（玩家推进中顺带观察）：39/5 走路与传送、从斯顿雪域再进城/回城、重登后仍在斯顿雪域；
   以及开发者页「强制剧情」档下的同一路径（准入按角色标记，理论上同样放行）。

## 未闭环点

- 客户端 `[odyssey enter level]` 的**分支实现**尚未静态验证（本机无 IDA/Ghidra）；本轮的对应关系证据是
  「客户端提示填 45 + 日志节点等级阶梯 + 两门槛数据」+ 本次实机通过。若日后出现「客户端放行、服务端仍拒」
  或反之，需要按 `analysis/AGENTS.md` 的门禁回到 IDB 取证。
- `internal/catalog/town.go` 的 `TownArea`（`configs/town.generated.json`，用于初始出生点策略）仍只认
  `[need level]`；本轮不涉及出生点，未改动。
