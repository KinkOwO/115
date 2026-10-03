# 机械七战神实验室副本 —— 问题记录与修复手册

> 面向服务端维护者。记录 115 私服实现「机械七战神实验室」副本时遇到的所有问题、根因与最终解法。
> 本文档为运维速查手册，不保证与官方行为一致，只记录本私服已实现/已验证的行为。

---

## 1. 副本背景与正确流程

机械七战神实验室（MeisterLaboratory）是 110 级高级副本，每次进入分两个阶段：

1. **第一阶段（驾车小游戏，约 30 秒）**：控制越野车躲避巨人攻击（按 A 冲刺、B 跳跃、Z 上车），
   打左臂或身体。怪物/玩法数据全部在**客户端本地**（`Contents/2022/MeisterLaboratory/...` 的
   `.tbl` 与 `.dgn` 脚本）。
2. **第二阶段（正式战斗）**：进入实验室与机械七战神战斗。

正确进入流程：NPC「机械七战神库里欧」→ 创建队伍 → 点开始 → 进图先玩小游戏 → 打完进第二阶段。

**本私服的简化实现**：进图（1 阶段营地）后按 Z 上车 → 客户端发 CMD316 → 服务端收到后
**直接推送第二阶段入场序列**（等效"服务端跳过小游戏"），用户直接进入第二阶段战斗，
小地图正常。

---

## 2. 两个阶段的地下城 ID（关键，曾被标反）

来源：`configs/dungeons.full.json` 中各 ID 的 `script` 路径。

| ID | 阶段 | 脚本路径 | 迷宫结构 |
|---|---|---|---|
| `400001392` | **第二阶段战斗** | `meisterlaboratory/dungeon/2phase/meisterlaboratory_2.dgn` | maze0 5×5/20房间 Start[2,4] Boss[2,1] |
| `400001561` | **第一阶段驾车小游戏** | `meisterlaboratory/dungeon/1phase/meisterlaboratory_1.dgn` | maze0、Size[2,1]、Start[0,0]、Boss[1,0]，房间 400004250/400004251 |

**血泪教训**：早期实现误把 400001392 当 1 阶段入场，导致客户端直接进第二阶段、
跳过小游戏、且小地图不显示（副本 HUD 未初始化）。

---

## 3. 已解决问题的清单

### 3.1 客户端不发「开始」命令 → 服务端驱动自动入场

- **现象**：库里欧建队后点「开始」，客户端不发任何命令（闭源客户端无法打补丁）。
- **解法**：镜像沉月湖 moonTick 先例，服务端在**建队后 8 秒**自动推开场序列（`advancedPartyTick`）。
- **代码**：`cmd/wireprobe/advanced_party_flow.go`

### 3.2 `unsupported dungeon option` 每秒报错

- **现象**：自动入场每秒报 `unsupported dungeon option`，`dungeon.Select` 拒绝。
- **根因**：`DungeonSelection.Mode` 传成了建队弹窗的玩法模式（8=机械战神），
  但它是**入场模式**（0=普通、1=地狱派对），`r.Mode > 1` 直接拒绝。
- **解法**：`Mode: 0`。

### 3.3 客户端闪退（`exit_shutdown_signal`）

- **现象**：自动入场后客户端主动发 CMD682 优雅退出。
- **根因**：对客户端**从未发送过**的命令回了 ack（ack1565）+ NOTI1994，状态机冲突。
- **解法**：删掉 ack1565 与 NOTI1994，入场序列对齐沉月湖先例：**无 ack、无 NOTI1994**。

### 3.4 退出队伍无效

- **现象**：自动入场后点「退出队伍」无效。
- **根因**：CMD13 被 `prepared` 守卫永久拒绝（自动入场后 `prepared` 恒 true，且无重置点）。
- **解法**：CMD13 改为**随时允许离队**，只解散队伍通知，不动 `activeDungeon`（副本中继续单人进行）。

### 3.5 库里欧 NPC 对话被拒（`quest 13600 NPC absent`）

- **现象**：每次对话库里欧都报 `quest_interaction_refused "NPC absent from current source area 140/2"`。
- **根因**：库里欧 `100001627` 在服务端 `configs/world.generated.json` 的 140/2 区域脚本中
  **没有 `[NPC]` 定义**（全区域扫描无）。
- **解法**：在 `configs/world.generated.json` 的 `areas["140/2"].map.cells` 追加
  `[NPC] 100001627 [left] 300 250 0`（修改前已备份 `.bak`）。这是数据文件改动，**无需重新编译**。
- **注意**：该文件若被 git 覆盖会退回原状，需重新补丁。

### 3.6 小地图缺失（最曲折，最终根因）

- **现象**：机械战神入场后无小地图，按 N 呼不出；其他副本（含黑鸦）正常。
- **排查过程**（4 轮包序调整全部无效）：
  1. 补发 NOTI3（UserStateDungeon）→ 无效；
  2. NOTI3 移后重排 → 无效；
  3. 整体对齐沉月湖包序（NOTI2×2→NOTI3→NOTI9→NOTI27→NOTI28→NOTI29）→ 无效；
  4. 删掉 NOTI3 → 无效。
- **校准结论**（黑鸦实测有小地图）：普通 CMD16 入场与黑鸦入场都**不带 NOTI3**。
- **真正根因**：入场用的 `advancedPartyDungeon = 400001392`（第二阶段），客户端跳过小游戏
  直接进 2phase，副本 HUD（含小地图）错乱。
- **最终解法**：入场改用 `400001561`（1 阶段），Z 上车后服务端切 2 阶段（见 3.7），小地图正常。

### 3.7 小游戏卡住 → 服务端跳过小游戏（当前方案）

- **现象**：1 阶段营地 Z 上车后按任何键无反应；上车约 6 秒后客户端发一次 CMD316，
  此后零网络帧直到主动退本。
- **排查**：CMD316 是上车时唯一出现的非心跳帧；尝试回 ack316{01} 仍不启动
  （小游戏为客户端深度脚本化玩法：怪物表在本地 tbl、`[pathgate object]` 服务端从不 spawn、
  `[custom frame index]` 特殊 UI，服务端无参照样本无法驱动小游戏本体）。
- **最终解法**：`meisterVehicleHandle` 收到 CMD316 后**不回简单 ack**，而是直接
  `prepareDungeonEntry(400001392)` 推第二阶段入场序列，等效"服务端跳过小游戏"。
  `plan[0]` 是给 CMD16 的 ack（客户端未发过），必须丢弃，否则触发状态机冲突闪退。
- **代码**：`cmd/wireprobe/advanced_party_flow.go`

---

## 4. 关键协议速查

| 项 | 说明 |
|---|---|
| CMD12 | 建队（`DecodeAdvancedPartyCreate`：名字@2、容量@at、难度@at+5、玩法模式@at+9） |
| CMD13 | 离队（全零体）——**随时允许**，不动 activeDungeon |
| CMD15 | dungeon gate 许可（非零 requested 走世界地图/控制板直选） |
| CMD16 | 选图确认（普通入场入口） |
| CMD316 | 机械战神上车就绪/下车清理子命令（首字节 02=上车、03=下车；客户端实际发送，可安全应答） |
| CMD1565 | 皮肤/开始共用帧（勿对未发送命令回 ack） |
| CMD682 | 客户端 `exit_shutdown_signal`（收到 ack 冲突时的优雅退出信号） |
| NOTI2 | 角色资料 ×2（basic + addition） |
| NOTI3 | 用户状态（UserStateDungeon）——普通/黑鸦入场都**不带**，勿乱加 |
| NOTI9 | 队伍名册 / solo party bootstrap |
| NOTI14 | 穿戴外观 |
| NOTI27 | EnterDungeonSelection（选图/场景切换） |
| NOTI28 | DungeonInfo（ID、Difficulty、Maze 索引、Boss 等） |
| NOTI29 | StartMap（当前房间+怪物） |
| NOTI1994 | BlackPurgatoryEntryInfo（黑鸦专属，机械战神不用） |

**已验证的小地图正常入场包序**（服务端直推，无 ack）：
`NOTI2×2 → NOTI14 → NOTI9 → NOTI27 → NOTI28 → NOTI29`（对齐沉月湖/黑鸦，无 NOTI3、无 ack、无 NOTI1994）。

---

## 5. 涉及文件

| 文件 | 改动 |
|---|---|
| `cmd/wireprobe/advanced_party_flow.go` | 建队/离队/自动入场/`meisterVehicleHandle`（CMD316 切 2 阶段） |
| `cmd/wireprobe/dungeon_flow.go` | 普通入场序列（NOTI9 条件放开） |
| `cmd/wireprobe/main.go` | `advancedPartyHandle`/`meisterVehicleHandle` 接线、`advancedPartyTick` |
| `cmd/wireprobe/world_flow.go` | `advancedParty` 状态字段 |
| `configs/world.generated.json` | 140/2 补库里欧 `[NPC]`（注意 git 覆盖风险，有 `.bak`） |

---

## 6. 部署流程（务必执行）

1. 改代码后必须手动重新编译：
   `go build -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe`（在 `dfo-lan` 目录）
2. 用户手动重启 `启动服务端.cmd`。
3. 数据文件（world.generated.json）改动无需编译，直接重启即可。
4. 新会话日志在 `runtime\roles_persist_..._next37\events.jsonl`，用
   `dungeon_start_map_sent` / `meister_vehicle_ready_ack` / `advanced_party_*` 定位事件。

---

## 7. 遗留问题 / 未解决项

- **驾车小游戏本体未实现**：当前是"服务端跳过"方案，小游戏玩法（驾车躲避巨人）不可玩。
  若官方服务端有样本可抓包逆向，可另行实现；怪物表在客户端本地 `.tbl`，服务端无法直接驱动。
- **「跳过继续作战」弹窗**：截图里库里欧头顶的"跳过继续作战"是 NPC 提示文字，
  **不是可点击弹窗**，用户看不到，此路不通。
- **小游戏→二阶段的过渡**：当前方案依赖用户手动按 Z 上车触发 CMD316；
  若未来想自动衔接，可在 `advancedPartyTick` 里固定延时后主动切 2 阶段。
- **1 阶段通关（若实现小游戏）后的衔接流程**未验证。
- 早期遗留：库里欧区域映射曾刷 `quest_interaction_refused`，修复后仍有
  `quest is completed or requires configuration migration`（入场前 NPC 交互），属已知未完全处理项。
