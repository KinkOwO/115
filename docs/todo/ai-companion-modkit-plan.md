# ACT12 假人整合包 → 115us modkit 方案

> 计划 / 台账类文档，落位按根 `AGENTS.md` §0.4.1。
> 参照对象：`C:\Users\Ricar\Downloads\ACT12假人分享\60ACT12单机假人组队陪玩整合包\60ACT12假人组队整合包`（A12 / 60 级 / C++ 服务端 v89）。
> 源码参照：`60ACT12原版单机整合包.zip` → `ACT12_Server/source/`（383 个文件 / 188 cpp + 160 h），已解压至
> `C:\Users\Ricar\Downloads\ACT12假人分享\a12-src\`（**仓库外的临时目录，不入库**）。
> 本文只做方案与可行性判断，**未改动任何运行路径、未写代码**。
>
> **2026-10-10 补充**：拿到 A12 源码后已按 §1.4 / §2.4 修订 —— 假人模块本身**不在**公开源码中（闭源），
> 但它依赖的**服务端底座 100% 在源码里**，且底座的抽象方式直接证明了 B 路线成立。
>
> **2026-10-10 再补充（状态已变，见 §8）**：M0 内核地基与 M1 第一个 mod **已落地**。
> 上文"未写代码"描述的是本文前七节的成文时点，不是当前状态。

---

## 0. 结论摘要

**可以做成 modkit 系统，但当前一个假人能力都还做不成 mod** —— 缺的不是权限，是**宿主接口**。
必须先给内核补两块地基（周期调度 + 假人实体宿主），之后才能按 ACT12 的模块划分逐个做成独立 mod。

三条关键判断：

1. **ACT12 的假人是「虚拟对端」（virtual_peer），不是「服务端内部实体」** —— 每个假人一个 `bot_account` + 一个真实 UDP 端口 + 完整 wire 握手（`server_log.txt` 中 `[virtual_peer]` 14 502 条 `bound` 记录可证）。这条路在 A12 成立，是因为 **A12 服务端持有战斗权威**。
2. **115us 的 dfolan 不持有战斗权威，且是纯 TCP**（全仓 `net.ListenUDP`/`DialUDP` **零命中**）。所以照搬 virtual_peer 只能换来"会走路的角色"，副本里照样打不出东西 —— **性价比远低于 A12**，不建议作为主路线。
3. **推荐主路线 = 服务端实体注入（lanHub/lanPeer 直造）**：假人是**内存态轻量实体，不落 SQLite**，避免污染玩家存档（§0.4 存档兼容最高优先级），也不需要重建客户端协议栈（避开 §0.2「协议第二真源」红线）。
   **A12 源码佐证（§1.4）**：A12 的队伍 / 摆摊 / PK / 邀请，成员类型清一色是 `TownPresence::Handle`
   而非真实连接 —— 说明"实体句柄"本来就是这类架构的正解，假人拿到句柄即自动获得全部社交资格。

---

## 1. 参照对象解剖：ACT12 到底怎么做的

### 1.1 核心机制：virtual_peer

从 `ACT12_Server/server_log.txt` 与 `ai_companions_status.json` 反推（该包只带运行程序，不带源码）：

```
[17:58:40][INFO][virtual_peer] bound bot_account=593 udp_port=35000 advertised_ipv4=2130706433
          endpoint_bound=true wire_validated=false combat_ready=false
[18:01:35][WARN][ai_peer_route] rejected actor=1576 bot_character=1855 authenticated_human=226
          source_slot=0 source_ipv4=0100007F source_port=5063 reason=no_valid_party_snapshot
[18:00:53][INFO][companions] skill decision actor=1576 job=6 grow=0 skill=1 learned=1
          candidates=3 mp=710 enabled=1 static=1 mode=2 target=5204 combo=0 dx=-182.9 dy=-7.7
```

| 事实 | 证据 |
| --- | --- |
| 每个假人有独立 bot 账号与角色 ID | `bot_account=593` / `bot_character=1855` |
| 每个假人绑定独立 UDP 端口，35000 起递增 | `udp_port=35000…`，`advertised_ipv4=2130706433`(=127.0.0.1) |
| 真的走 wire 握手，不是内存伪造 | `wire_validated` / `[virtual_peer] wire handshake confirmed` |
| 三态生命周期：端点绑定 → 握手校验 → 可战斗 | `endpoint_bound` / `wire_validated` / `combat_ready` |
| 有独立的 peer 路由与拒绝原因 | `[ai_peer_route] … reason=no_valid_party_snapshot` |
| 可靠传输自建 | status 里 `reliable_acks` / `rejected_peer_frames` / `received_relay_frames` |
| 规模：名册 4800 / 33 区域 / 8 频道 | `pool started regions=33 channels=8 bots=4800` |
| 技能是**逐个取证逐个支持**的 | status 里逐技能力计数：`upper_slash_wire` / `close_punch_wire` / `crash_low_kick_wire` / `moonlight_wire` / `dark_nail_spawn_wire` / `gunner_prepare_wire` … |

**这就是 A12 作者的路线：服务端内置一个"无头客户端"，走真协议、真账号、真 UDP。** 代价是要重建整套客户端侧状态机，收益是假人天然享有"正常玩家"的一切路径（被看见、被组队、能进本、能战斗）。

### 1.2 模块划分（可直接照搬的组织方式）

| 模块 | 配置 | 职责 | 我们的对应可行性 |
| --- | --- | --- | --- |
| `companions` | `ai_companions.ini [companions]` | 城镇/街道/入口假人、闲置走动、组队跟随、进本、移动/攻击/技能、召唤、复活 | M1/M2/M3 |
| `pvp` | `ai_companions.ini [pvp]` | 独立 PK 假人池（34 名 / 10~60 六档），血量/伤害/攻速/硬直可调 | M4 |
| `chatter` | `features.ini [chatter]` | 入口招募、街道收材料、全服对骂、趣味双人对话 | M5 |
| `fake_market` | `fake_market.ini` | 摆摊（含独立控制器进程）+ 拍卖行供货 | M6 |
| 名册 roster | `Generation` / `AppearanceSeed` / `roster.cache` | 带签名的名册缓存（10 MB），改代号即重建 | M1 |
| 评分/其它 | `features.ini [score]` | 最低 S / 自动 3S | 不在本方案范围 |

### 1.3 值得照搬的三条工程做法

1. **配置分三层、热读分级**：总开关（`features.ini`）× 模块开关 × 参数（`ai_companions.ini`）；普通开关约 1 秒重读、PK/喊话约 5 秒、人数/外观需重启重建。**改了没生效是假人系统最常见的投诉**，这层设计直接解决它。
2. **状态外置为 JSON**：`ai_companions_status.json` / `fake_market_status.json` / `npc_chatter_status.json`，每个子系统dump 自己的计数与 `error`，排查时看文件不看日志。
3. **名册缓存带签名**：`RosterCache=1` + `roster.cache`，签名含版本/PVF/配置，避免每次启动重建（重建耗时实测 67 秒：`roster catalog cache_hit=0 profiles=4800 elapsed_ms=67375`）。

### 1.4 源码底座解剖（2026-10-10 补充，本节最重要）

#### 先说一个事实更正：**假人模块本身不在源码里**

`a12-src/` 里 grep `virtual_peer|companion|fake_market|npc_chatter|bot_account|ai_pvp` **全部零命中**。
这份源码是"原版整合包"的 A12 服务端源码（`source/README.md` 自称"专供公开发布"），
而假人系统是作者在 v89 二进制里加的**闭源扩展**。所以：

> **能抄的是底座，不是假人本身。** 但底座的设计恰好是"为假人预留座位"的，价值极高。

#### 决定性发现：`TownPresence::Handle` 是所有社交系统的公共抽象

`session/town_presence.h` 与 `party_registry.h` / `private_store.h` / `pvp_registry.h` 对照：

| 子系统 | 成员/主体类型 | 出处 |
| --- | --- | --- |
| 城镇存在 | `TownPresence::Connection`（只有 `ActorId` + `State` + `Pending` 队列） | `town_presence.h:26-41` |
| 队伍 | `std::array<std::weak_ptr<TownPresence::Connection>, 4> Members` | `party_registry.h:47` |
| 摆摊 | `std::weak_ptr<TownPresence::Connection> Player` + `TownPresenceState Owner` | `private_store.h:21-23` |
| PK 房间 | `std::array<TownPresence::Handle, 8> Members` + `std::array<TownPresenceState, 8>` | `pvp_registry.h:33-34` |
| 队伍/PVP 邀请 | `std::weak_ptr<TownPresence::Connection> Source, Target` | `party_registry.h:50`、`pvp_registry.h:47` |

**结论：假人只要拿到一个 `TownPresence::Handle`，就自动获得组队 / 摆摊 / PK / 被浏览的全部资格，
不需要真实 socket。** 这是 B 路线（服务端实体注入）的**直接证据**，而且比我原方案里零散的
"Actor 六件套"更本质 —— 正确的内核改造目标是**把我们的 `lanPeer` 提升为同等级的公共抽象**，
而不是给 mod 开一组动作接口。

配套的另外三条：

- **广播出口由 TownPresence 代持**：`Send(target, packet)` / `Broadcast(channel, packet)` / `SendToArea`
  （`town_presence.h:59,62,72`）。假人没有连接，正是靠这个被别人"看见"。完全对应我 M0 里写的
  "必须由服务端代持一个广播出口"。
- **外观是一段字节**：`TownPresenceState::Identity` 是 `std::vector<uint8_t>`（`town_presence.h:15`），
  另有 `Refresh(handle, identity)` 可热换外观 —— 对应我们的 NOTI2 info + addition 两片。
- **邀请必须有超时**：`Invitation{... Expires}` + `ExpireInvitations()`（Party 与 Pvp 各一套，
  `party_registry.h:51,59`、`pvp_registry.h:50,52`）。假人"自动接受邀请"必须配超时清理，
  否则会堆积僵尸邀请。

#### `TownPresenceState` = 我们 `ActorSpec` 的现成模板

```cpp
struct TownPresenceState {
    uint8_t  Channel, Town, Area, Direction;   uint16_t X, Y;
    int64_t  PrivateOwner;                     std::vector<uint8_t> Identity;  // 外观字节
    int64_t  AccountId, CharacterId;           uint16_t Slot, Fatigue;
    uint8_t  Stamina = 100;                    bool VillageSiegeGhost;
    std::string Name;                          PeerAddress Network;            // 可为无效
};
```

对照我们的 `lanPeer`：**缺 `Identity` 外部注入口、缺 `Name`、缺"无连接"标记**。M0 要补的就是这三样。

#### 底座清单（A12 有、我们要不要抄）

| 底座 | A12 源码 | 我们的现状 | 对方案的影响 |
| --- | --- | --- | --- |
| 城镇存在 `TownPresence` | `session/town_presence.{h,cpp}`（459 行 cpp） | 有 `lan_hub.go`（等价物，但在 `package main`） | **M0 核心**：提升为公共抽象 |
| 队伍 `PartyRegistry` | `session/party_registry.{h,cpp}` + `party_dungeon/room/run/settlement.cpp` | 未实现 | M2 有完整设计可抄 |
| 摆摊 `PrivateStores` | `session/private_store.h` + `open/browse/buy/registry.cpp` | 只有 NPC 商店 | **M6 工作量大幅下调** |
| PK `PvpRegistry` | `session/pvp_registry.{h,cpp}` + `pvp_battle/invitation/relay/settlement.cpp` | `internal/pvp/room.go` 有类型但无实例 | M4 有完整设计可抄 |
| peer 中继 `PeerRelayServer` | `net/peer_relay.{h,cpp}`、`net/peer_address.h` | **纯 TCP，无 UDP** | 见下方风险 |
| 喊话 | `handlers/megaphone.cpp`（53 行，`sendMegaphone(type, space, slot, text)`） | 无 | M5 有参照但仍需 115 包号取证 |
| 战斗 | `handlers/combat.cpp`（536 行，**战斗在服务端**） | 战斗不在服务端 | 差异根源，见 §2.3 |
| 子系统装配 | `GameServices`：Characters / Parties / Town / Mentors / Guilds / Stores / Pvp / Relay（`game_services.h:56-65`） | 全在 `package main` 的 `gameGateway` | 装配模式可参照 |

#### opcode 参照表（**A12 包号，仅供对照，严禁直接搬到 115**）

```
请求侧   SET_PARTY_INFO=12  LEAVE_PARTY=13  WALKOUT_PARTY=14  CHAT=17
         MAKE_PVP_ROOM=53  ENTER_PVP_ROOM=54  SET_PVP_SEAT=55  SET_PVP_READY=56
         SET_PVP_TEAM_MODE=57  DIE_PVP=58  PVP_TIMEOUT=59  END_PVP_RESULT=60  SET_PVP_MAP=62
         BUY_PRIVATE_STORE=85  ENTER=86  EXIT=87  CREATE=88  REMOVE=89  COMPLETE=90
通知侧   PARTY_IPINFO=11  CHAT=12  MEGAPHONE=130
```

#### 顺带印证：Tick 是子系统级标配

`PlayerTrade` / `PrivateStores` / `PvpRegistry` **都**有 `void Tick(GameServices&, const TownPresence::Handle&)`。
A12 自己也是靠 tick 驱动这些子系统的 —— M0 的 `RegisterTick` 不是可选项。

---

## 2. 我们这边的能力盘点

### 2.1 已有（可以直接用）

| 能力 | 位置 | 说明 |
| --- | --- | --- |
| 城镇实体骨架 `lanPeer`/`lanHub` | `cmd/wireprobe/lan_hub.go:28,81` | 建 actor / 放置 / 移动广播 / 离场，NOTI 2、22、23、24、6 全套 |
| 外观字节合成 | `EntryBasicProbe` / `EntryAddition`（`client_entry.go:299`、`internal/character/service.go:394`） | 可为虚构角色现算 info + addition 两片 |
| 可见性判定 | `visible()` `lan_hub.go:66` | 同频道 + 同 town + 同 area + 非 private |
| 全量 PVF 解析 | `internal/gamedata/source.go:44`、`catalogs.go:353` | 54 个直读域，含职业/转职/成长/装备 424 216 条 |
| 技能学习目录 | `internal/character/learning_catalog.go:19` | `[type]`/`[required level]`/`[pre required skill]` —— **名册技能选择的真源** |
| 副本内 APC 通道 | `protocol/dungeon.go:269-279`（`SourceIndex==10000` 走原生 AIC + 跟随） | 已实机验证，见 adventure-elite |
| 决斗场房间 | `internal/pvp/room.go` | Create/Join/Seat/Ready 齐全（**但 `Manager` 无生产实例**） |
| 服务端 mod 框架 | `internal/servermod/host.go` | boot / console / request / response / reward.script 五个钩子 |

### 2.2 缺失（必须补，否则做不成）

| 缺什么 | 影响 | 证据 |
| --- | --- | --- |
| **周期调度钩子** | 假人需要持续驱动，框架只有启动期一次性回调 | `internal/servermod` 内 grep `Tick|Timer` 仅一处注释；`console.command` 也是启动跑一次（`main.go:133`） |
| **假人实体宿主接口** | mod 不能造实体、不能广播 | `lanHub` 在 `package main`，Go 不允许 import main 包 |
| **广播出口** | `reply.send` 只对当前连接、只在单帧内有效 | `host.go:129-132` |
| **队伍服务** | 组队未实现 | `request_scope.go:83-95` 原文 "None of them is implemented"；只有 `SoloPartyInfo`（1 人队） |
| **战斗权威** | 技能/伤害/命中全在客户端 | 全仓无 damage/hit 结算代码 |
| **聊天编解码** | 喊话无从下手 | `internal/game/protocol` 无 chat/shout/whisper 文件 |
| **个人摆摊** | 玩法缺失，但 A12 有完整设计可抄（§1.4） | 只有 NPC 商店 `shop_flow.go` |
| `pvp.Manager` 实例 | PK 池接不进去，但 A12 有完整设计可抄（§1.4） | 全仓 `NewManager(` 零命中 |

### 2.3 与 A12 的决定性差异

| 维度 | ACT12 | 115us dfolan |
| --- | --- | --- |
| 传输 | TCP + **UDP peer + relay**（606/562/5555） | **纯 TCP**（UDP 零命中） |
| 战斗权威 | **在服务端**（技能决策、伤害、怪物 AI） | **不在服务端**（客户端自算，服务端只收死亡/拾取/结算上报） |
| 假人形态 | 独立账号 + 独立角色 + 落库 | 建议：**内存态实体，不落 SQLite** |
| 队伍 | 已实现（有 party snapshot） | 未实现 |

> **这条差异是路线选择的根**：A12 造 virtual_peer 是因为它有能力让这个 peer 真正"打"；我们造 virtual_peer，它只能"站着走"，
> 却要付出重建整套客户端握手/心跳/状态机的代价，还要为假人在玩家存档库里建号（触碰 §0.4 存档兼容红线）。**不划算。**

---

## 3. 路线对比

| 路线 | 做法 | 优点 | 缺点 | 结论 |
| --- | --- | --- | --- | --- |
| **A. 虚拟对端**（照搬 A12） | 服务端内置无头客户端，TCP 握手登录进城 | 假人天然走正常玩家路径，被看见/组队/进本全免费 | 要重建客户端状态机；要为假人建账号角色（污染存档库）；副本战斗仍打不出来（我们无战斗权威）；触碰 §0.2 第二真源红线 | ❌ 不推荐 |
| **B. 服务端实体注入** | 内核暴露 Actor 宿主，mod 造内存态假人 | 轻量、可控、不落库、不碰存档、不需重建协议 | 假人"不是真角色"，队伍/进本/战斗需逐项补服务端侧语义 | ✅ **推荐主路线** |
| **C. 客户端 DLL 原生 APC**（沿用 adventure-elite） | 客户端注入，用游戏自带 AIC | **已实机验证**：五房通关、跨门跟随、击杀、拾取、Boss、翻牌 | 只能陪副本；做不了城镇街道假人/摆摊/PK 池；需 MSVC x64 + MinHook；与 §0.1「服务端优先」相悖，需业主明确要求 | ⚠️ 副本战斗的**现成答案**，作为 M3 的选项之一 |

**推荐组合：城镇与跟随走 B；副本内"打"暂时借 C（或先只做跟随不打）；组队先补内核队伍服务。**

---

## 4. 分阶段方案

### M0 —— 地基（**内核改动，不是 mod**，必须先做）

| # | 改动 | 文件 |
| --- | --- | --- |
| M0.1 | 新增 `RegisterTick(modID, fn func(time.Time))` + `RegisterShutdown(modID, fn func())`；无注册者时零开销 | `internal/servermod/host.go`、`declarations.go` |
| M0.2 | 钩子常量 `server.tick` / `server.shutdown`，并**同步登记到启动器仓** `internal/modkit/layer.go` 的 `HookPoints`、`HostOps` | 两仓成对演进，否则 modkit 拒绝声明 |
| M0.3 | 新建 `internal/modactor`：进程内单例 + 零值=关 + `Source` 强制署名 + 消费点固定在既有 lanHub 路径 + 启动日志一行见证（**照抄 `internal/modpolicy` 的既有模式**） | 新包 |
| M0.4 | 宿主接口：`RegisterActor(spec) (ActorID, error)` / `MoveActor` / `SetActorArea` / `RemoveActor` / `ActorPose` / `ListActors` / `RefreshIdentity` | `internal/servermod/host.go` |
| M0.4b | **（2026-10-10 修订，比 M0.4 更本质）** 把 `lanPeer` 提升为**公共抽象**而非一组动作接口：允许存在"无连接"的 peer（Send 出口由 hub 代持，对应 A12 `TownPresence::Send`），并让它能被队伍/摆摊/PK 直接引用（对应 A12 清一色 `TownPresence::Handle`）。ActorSpec 字段照 §1.4 的 `TownPresenceState` 模板补齐 **`Identity` 注入口 / `Name` / 无连接标记** 三样 | `cmd/wireprobe/lan_hub.go` |
| M0.5 | 权限位 `server.tick` / `server.actor`，并在 `manifest2.go` 的 need 推导里登记（**声明/审计性质，不是安全门** —— mod 与内核同模块，本就能 import 任意内部包） | 启动器仓 |

**验收**：`registerActor` 出来的假人能被同频道同区域的真人客户端看见（NOTI 24 里出现），移动能被广播（NOTI 22），撤掉能发 NOTI 6；无 mod 注册 tick 时启动日志无新增、CPU 无变化。

### M1 —— `companion.town`：城镇/街道/入口假人（第一个真 mod）

- **名册生成必须由 PVF 驱动**（§0.2）：职业/转职/合法等级取自 `gamedata.Source` 的 Characters 域，技能取自 `learning_catalog`；**禁止硬编码职业清单或 ID 表**。
- 配置模型照搬 ACT12：`Generation`（改值重建）+ `AppearanceSeed` + 每区域人数上限 + 闲置走动。
- 假人**不落 SQLite**，进程内名册，重启即重建（可选带签名缓存，参考 A12 的 67 秒重建代价）。

**验收**：城镇站得住、能被看见；`IdleWander` 走动不穿模不越界（用 `World/Area` 的可行走矩形校验）；改 `Generation` 能撤回重建；关总开关能全部撤掉且不残留 actor。

### M2 —— `companion.party`：组队跟随（**前置是内核队伍服务**）

服务端目前完全没有队伍服务，这是**内核工作，不是 mod**。补完 `DecodePartyCreate115`（`party_create115.go:30`）到真实队伍状态机后，mod 侧才谈得上：自动接受邀请 → 跟随真人移动/换区/回城 → 离队归位。

**内核队伍服务照 A12 `PartyRegistry` 抄**（§1.4），四条不能省：

- 成员用**实体句柄**（A12 是 `weak_ptr<TownPresence::Connection>`），不是连接 —— 否则假人进不来；
- 4 人上限 `Capacity=4`，`Leader` 席位独立；
- **邀请带超时 + `ExpireInvitations()`**，假人自动接受后必须有清理，否则堆僵尸邀请；
- 进本要 `EntrySnapshot` + `BeginSelection` + `LockEntry` 三段锁（A12 `party_registry.h:35-40`），
  假人随队进本走同一条锁，不另开后门。

**验收**：真人邀请假人 → 假人自动接受 → 真人换区假人跟上 → 真人回城假人归位 → 解散队伍假人结束跟随；
邀请超时后无残留。

### M3 —— `companion.dungeon`：副本内行为（**需业主裁决**）

- **保守版（推荐先做）**：只做跟随/位移/过门，**不做攻击**。风险低、不碰战斗协议。
- **进取版**：让假人放技能 —— 需要战斗包取证，受 §0.1 **C2S 三次上限**约束，且 A12 的经验是"技能必须逐个取证逐个支持"（它的 status 里每个技能一个计数器）。
- **借道版**：直接沿用 adventure-elite 客户端 APC（已验证五房通关）—— 但这是 DLL，需业主明确要求才动（§0.1）。

### M4 —— `companion.pvp`：PK 假人
前置：`internal/pvp.Manager` 接进 `package main` 并暴露实例。之后照搬 A12 的参数模型（`BotHpPercent` / `HumanDamagePercent` / `AttackIntervalMs` / `HitStunMs` / `BotSkillsEnabled`）。

> **2026-10-10 下调工作量**：A12 的 `PvpRegistry` 是完整参照（`pvp_registry.h`）：8 座房间、成员用句柄、
> `RelayActive[2]` 中继位、`Deadline` 超时、`Settle`/`CheckWinner`/`AdvanceRelay` 结算与接力、邀请带超时。
> 我们的 `internal/pvp/room.go` 已有 Create/Join/Seat/Ready，只差"接进 main + 成员改句柄"。
> **从"缺设计"降为"照设计实现 + 115 包号取证"**。

### M5 —— `town.chatter`：喊话
服务端**完全没有**聊天编解码与分发，属全新内核能力 + 全新包号取证。A12 只提供了很薄的参照
（`handlers/megaphone.cpp` 仅 53 行：`sendMegaphone(type, space, slot, text)`，REQ `CHAT=17` / NOTI `MEGAPHONE=130`），
**包号必须按 115 重新取证**。建议排在实体类之后独立立项。

### M6 —— `market.stall`：摆摊 / 拍卖供货
服务端只有 NPC 商店。

> **2026-10-10 下调工作量（下调幅度最大的一项）**：A12 的 `PrivateStores` 是**完整可抄的状态机**
> （`session/private_store.h`）：`Store{Player(句柄), Owner(状态), Title, Items[{Slot, Price, Item}], Open,
> NextPermissionCheck}` + `Browser{Player, Character, Seller}` 浏览态，配 `Create/Complete/Browse/
> LeaveBrowser/Buy/Close/List/Tick` 八个动作。
> **从"全新玩法"降为"照设计实现 + 115 包号取证"**（A12 包号见 §1.4，不可直接搬）。
> 仍然不建议与 M1~M3 并行 —— 不是因为难，而是因为它要碰背包与金币事务。

---

## 5. mod 包结构（以 `companion.town` 为例）

```
mods/companion.town/
  mod.json                 id/version/name/author/description/permissions/requires/layers
  server/
    mod.go                 package modpkg；Register() 开头先问 servermod.Enabled(modID)
    roster.go              名册生成（gamedata.Source 驱动）
    behavior.go            闲置走动 / 跟随决策
    config.json            //go:embed，随二进制走
    rules/*.lua            可选，不参与本模块核心
  build-mod.py / build-mod.cmd
  README.md
```

`mod.json` 关键片段：

```json
{
  "schema": 2,
  "id": "companion.town",
  "version": "0.1.0",
  "name": "城镇假人",
  "permissions": ["server.hook", "server.actor", "server.tick"],
  "layers": {
    "server": {
      "package": ".",
      "hooks": [
        {"name": "server.boot", "note": "生成名册并注册假人"},
        {"name": "server.tick", "note": "驱动走动与跟随"},
        {"name": "server.shutdown", "note": "撤回全部假人"},
        {"name": "console.command", "note": "status/what 诊断"}
      ]
    }
  }
}
```

> 注意 `manifest2.go` 用 `DisallowUnknownFields()`：新增 JSON 字段必须升 schema，
> 但**新增钩子名/操作名不用**（进 slice）。这条既有事实让 M0 的成本可控。

---

## 6. 需要业主裁决

1. **主路线**：确认 B（服务端实体注入）为主，还是坚持照搬 A12 的 A（虚拟对端）？
2. **M3 副本战斗**：保守版（只跟随不打）/ 进取版（服务端发技能帧，受三次上限约束）/ 借道版（adventure-elite 客户端 DLL，需业主明确要求）？
3. **范围**：先做到 M1（城镇假人）就收口，还是一路推到 M4/M5/M6？M5/M6 是全新内核能力，量级完全不同。
4. **假人是否落库**：本方案默认**不落 SQLite**（内存态）。若业主要求假人有持久背包/装备/等级，需另议存档隔离方案。

---

## 7. 风险与未闭环

| 项 | 说明 |
| --- | --- |
| **NOTI 24 的 flag A/B/C 语义未确认** | `docs/protocol/area-users.md:16`；做 M1 时只能沿用 `0/1/1`，假人外观显示可能有偏差 |
| **发布链缺 mod 宿主** | `MOD-FRAMEWORK-AUDIT-20261007.md:53-66`：远端 `tools-server-bin.zip` 里 `internal/servermod`/`RegisterMods` 命中 **0/0** —— 玩家侧装了 Go 钩子 mod 会静默失效。M0 之前必须先修发布链，否则做完也发不出去 |
| **两仓同步** | 加钩子必须同时改 `115` 与 `115us-dfolauncher`，漏一边 modkit 就拒绝声明 |
| **`ChannelCount: 0` 恒零**（`main.go:90`）与 `SetConfigSnapshot` 无生产调用 | 假人需要知道频道拓扑，M1 前要修 |
| **PVF 驱动自检** | 名册若出现任何硬编码职业/等级/技能清单，即违反 §0.2，需在当轮主动提示 |
| **PeerAddress 是否必需，待取证**（2026-10-10 新增） | A12 的 `TownPresenceState::Network` 是 `PeerAddress{InnerIp, OuterIp, Mtu, Port, Nat 1..4}`，队伍进本与 PK 都要 `SendNetwork`/`RefreshNetwork` 交换它（`party_registry.h:41`、`pvp_registry.h:20`）。我们**纯 TCP、无 UDP**，M0 的假人可以先留空 Network；但**若 115 的队伍进本/PVP 也要求 peer 地址**，假人就必须伪造一个 —— 这是 M2/M4 的实际卡点，动手前必须先用 v1 事件流取证确认，不要凭 A12 推断 |
| **本方案未做任何协议取证** | 全部结论来自只读调研；M3 一旦涉及战斗包，必须按 §0.1 记 attempt 并遵守三次上限 |
| **A12 源码只是参照，不是事实标准** | 版本不同（A12/60 级/C++ vs 115/115 级/Go），按根 `AGENTS.md` §0.2：opcode、字段布局、动作枚举**必须**以 115 客户端 IDA 与实机证据为准；§1.4 的 opcode 表仅供对照，严禁直接搬 |

---

## 8. 实现记录（2026-10-10 落地：M0 + M1）

M0 内核地基与 M1 第一个 mod 已实现。**全部改动都在服务端 Go 侧，没有碰客户端 DLL/补丁**
（根 `AGENTS.md` §0.1 服务端优先）。门禁实测：`go vet` 退出 0、`go test ./cmd/... ./internal/... ./mods/... -count=1` 全绿。

### 8.1 M0 —— 内核地基（**不是 mod**，是让 mod 能做成的前提）

| 改动 | 文件 | 做了什么 |
| --- | --- | --- |
| 周期调度钩子 | `internal/servermod/host.go` | 新增 `HookServerTick = "server.tick"` / `HookShutdown = "server.shutdown"`；`RegisterTick` / `RegisterShutdown` / `Tick()` / `Shutdown()` / `HasTickHooks()`；`TickContext{Seq, Now, SinceLast}`；钩子 panic 被接住并记日志（tick 在服务端自己的 goroutine 里，带崩代价不成比例） |
| 声明核对补全 | `internal/servermod/declarations.go` | `RegisteredHooks` 认得这两个新钩子，否则"声明了却没注册"的 fail-closed 核对会误报 |
| **场景实体宿主接口** | `internal/modactor/modactor.go`（新包） | `Host` 接口（`Spawn`/`Move`/`Relocate`/`Despawn`/`Pose`/`List`/`Scenes`）+ 进程内单例 `Install`/`Uninstall` + `Spec`/`Pose`/`Snapshot`/`Scene`。**照抄 `internal/modpolicy` 的模式**：没有宿主时一律返回 `ErrNoHost`（明确报错，不静默空转） |
| 宿主实现 | `cmd/wireprobe/lan_actor.go`（新文件） | 把 `lanHub` 包成 `modactor.Host`。假人 = **`send == nil` 的 `lanPeer`**：在花名册里所以 `visible()` 认它（既有的 `introducePeers` / NOTI 24 自动带上它），没有 `send` 所以所有发给它的推送被既有代码跳过。actor id 从 `0x8000` 起分配并跳过已占用（真实玩家的 WireID 是数据库分配的小整数） |
| 外观编码 | 同上 | **复用** `protocol.UserInfoBasicProbe` / `UserInfoAdditionProbe`，不自己拼字节 —— 让每个 mod 自己拼一段 NOTI 2 载荷等于把协议真源拆成 N 份（§0.2）。顺带给 protocol 导出了 `MaxEquippedAppearanceSlot()`（槽位上界是客户端表的性质，不该出现第二份常量） |
| 接线 | `cmd/wireprobe/main.go`、`servermods.go` | Boot 之前 `installActorHost(prepared.hub)`（hub 在 `prepareRuntime` 里建，装早了拿不到）；`defer` 顺序刻意做成"先停 ticker → 再 Shutdown → 再卸宿主"；周期来自 `DFO_MOD_TICK_MS`（默认 1000ms），**没有 tick 钩子就不起 goroutine** |
| 启动器同步 | `115us-dfolauncher/internal/modkit/layer.go` | `HookPoints` 加 `server.tick` / `server.shutdown`；`HostOps` 加 `actor.spawn`/`actor.move`/`actor.despawn`/`actor.list`。**两仓必须成对** —— 漏一边 modkit 直接拒绝声明 |

**关键设计取舍：为什么加了 `Scenes()`**

假人要有用就得站在有人的地方，而城镇/区域编号来自 PVF、mod 拿不到那份目录。让 mod 在配置里写死
一组坐标，等于逼作者先去做一次协议/资源取证（§0.2、§0.1）。所以给了一个只读的
"现在哪些场景有真实连接"查询，mod 就能写出**不需要任何取证**的行为。这是 §1.4 里
`TownPresence::Handle` 那条结论的落地：正确的内核目标不是"给 mod 开一组动作接口"，
而是让"无连接实体"成为一等公民。

### 8.2 M1 —— `town.companion`（第一个真 mod，完整 mod 包）

产物在**包根** `mods/town.companion/`（源形态）+ `dist/town.companion-1.0.0.zip`（可分发的 zip）：

```
mods/town.companion/
  mod.json            组织文件（schema 2，声明 4 个钩子：boot/tick/shutdown/console）
  README.md           说明书
  build-mod.py        打包 + 调 modkit 校验
  dist/*.zip          正式分发包
  server/
    mod.go            Go 包（编译进服务端）
    config.json       行为参数（go:embed，改了要重装）
    roster.json       名册资源（随包分发，运行时从 mod 目录读，改完约 10 秒热重载）
```

行为：有人进城 → 假人出现在他所在的场景 → 人换区跟着过去 → 每个人在锚点附近来回巡逻。
服务端退出时全部撤走。**不落库**：不写 SQLite、不占角色槽、不进账号。

modkit 校验实测通过（用**当前源码**编的 modkit；`.tmp/modkit-new.exe` 是改 `layer.go` 之前编的，
它会以"未知钩子点 server.tick"拒绝——这正是 §7「发布链缺 mod 宿主」那条风险的现场复现）：

```
清单 OK：town.companion（城镇假人（会走路的陪玩））v1.0.0
层：server    权限：server.hook
```

### 8.3 有意没做的（对应 §7 的边界）

- **不打架 / 不组队 / 不说话** —— 三条各自对应一次协议取证，而"站着陪你"本身不需要它们；
- **不照搬 A12 的 virtual_peer** —— 见 §0 第 2 条；
- **没动 `zz_mods_gen.go`** —— 它由 modkit 生成，提交形态必须干净。装 mod 走 `modkit install`。
  当前工作区里这份是"本机装了两个 mod"的形态（别人的改动），按仓库规矩**不该入库**。

### 8.4 待业主裁决（M2 及以后）

§6 的四个决策点里，"主路线的实体注入"已按推荐执行；剩下三个仍然有效，且现在多了一个
明确的**卡点前置**：`PeerAddress` 是否必需（§7）必须先取证，否则 M2/M4 会卡住。
