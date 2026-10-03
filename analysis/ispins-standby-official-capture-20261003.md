# 官服「伊斯大陆待机区入场」实时抓包证据（2026-10-03 02:35）

工具：dfocap.exe（实时解密抓包，GUI 由用户以管理员运行；查询接口 http://127.0.0.1:20241）。
原始数据：`D:\115us\实时抓包\captures\20261003-023535\`（raw/*.bin 全量字节流）；
解析产物：`D:\115us\analysis-tools\output\ispins_switch_frames.json`（1310 帧全量 plain_hex）、
`ispins_switch_timeline.txt`、`ispins_entry_sequences.json`、`ispins_entry_diff.txt`、`ispins_town_vs_standby.txt`。
采集会话：官服，奥德赛角色，登录进镇 → 点伊斯大陆切频道 → 进入待机区站立走动。

## 1. 切频道的完整连接拓扑（conns.jsonl 证据）

| 连接 | 远端 | 说明 |
| --- | --- | --- |
| 57467 → 3.211.150.198:7101 | 登录服务器 | 客户端启动时先连，~12KB 应答 |
| 57472 → 52.23.96.246:10014 | 游戏 channel 服务器 | 登录+选角+城镇入场+站街（帧 1-706） |
| 50061/49684/65299 → 3.211.150.198:7101 ×3 | 登录服务器 | **切频道前客户端短连 3 次查询目标地址**（各发 54B，收 ~3.36KB，连接存活 ~250ms） |
| 61262 → 52.203.207.183:10015 | **军团待机区服务器** | 新连接：完整重登录+选角+待机区入场（帧 707-1310） |

旧连接的收尾：`703 c2s CMD3 EXIT payload=01...`（ExitReason=01，切换语义），无任何「切频道请求」CMD 发给旧 channel。
**目标服务器地址由登录服务器（7101）查询返回**，客户端全程自主完成断开与重连。

## 2. 新连接帧序（帧 707-863）

1. `CHANNELINFO(1, 527B)` → c2s `LOGIN_PRECHECK(1554)` → c2s `LOGIN(1)`
2. 登录 announce 批（帧 711-777）：与首次登录的 announce **完全同构**（含 708/108 EVENT_INFO 2668B/1336/1195 等），
   且含 `2219 SPECIAL_WARP_GATE(144B)`（帧 771，尾部仅 `73ec209b39000000` 一处时间戳，其余全零）与
   **第二个 `CHANNELINFO(1, 112B)`**（帧 773）= 频道身份帧。
   **不重发角色列表**（无 832B USERINFO/CHARAC_MIGRATION 角色列表相）。
3. c2s `SEC_INFO(1593)/CLIENT_SPEC_STATISTIC(171)/SECURITY_STATUS(585)/SET_UDP_IP_PORT(2)` →
   c2s **`SELECT_CHARACTER(4)`**（帧 782，客户端直接选角，无需角色列表）。
4. 入场序（帧 790-863）：`USERINFO(2, 32B 最简)` → `848 CHARAC_MIGRATION(48B)` → `433 COMBO_SKILL_INFO(40B)` →
   `637 RAID_INOUT_SYSTEM(24B, flag=01)` → … 与城镇入场完全同构（STAMINA 1312B、ITEM_LIST 批、
   SKILLINFO、342×2、21、1352、ENTER_GAMEWORLD_COMPLETE(124)），再接 post-entry 批（2828 等，至 ~1048）。
5. 客户端随后 c2s `SET_USER_POSITION(35)`（帧 894）→ `SET_USER_AREA(36)`（帧 919/925，payload 首字段 `0x92`）→
   在待机区内走动（USER_STATE_MOTION/SET_USER_POSITION 持续上报）。

## 3. 关键对比结论（城镇 vs 待机区入场）

- **入场机器完全相同**：ops 仅差 `389 LIMIT_NPC_BUY_ITEM_INFO_ALL`（仅城镇）；
  待机区多的 790/791/793（USERINFO32/CHARAC_MIGRATION/RAID_INOUT）与旧连接角色列表相（帧 84-98）
  **逐字节一致**（`01b2ce8d5940…` / `01000000…7582311745…` / `010038d22c5e4000`）。
- **没有任何帧显式下发「待机区」**：PREV_VILLAGE 双方都是 `f1`=241（奥德赛镇）；WARPGATE 相同；
  客户端落点来自**自身 actor USERINFO（864B，zlib 压缩体内的位置/区域）+ 频道身份（112B CHANNELINFO）**。
- `RAID_INOUT_SYSTEM(637)` flag=`01` + `CHARAC_MIGRATION(848)` flag=`01`：迁移/在册状态，
  登录与入场两处都发。
- 官服**不发送** user_area/town_entry 类推送；客户端自报 `SET_USER_AREA` 首字段 `0x92`（待机区区域号）。
- 登录服务器查询（7101）：c2s 54B（`000b2b0000…`，三次请求结构一致），s2c ~3.36KB 应答含目标服务器地址；
  明文协议与游戏线协议不同（独立握手）。

## 4. 私服实现指向（待实现）

1. 旧会话收到切频道意图（私服表现为 `menu_response(id=3,option=1)`，官服无此帧）时：
   持久化「该角色军团迁移中」状态（模拟 CHARAC_MIGRATION flag=01）。
2. 重连会话（私服客户端重连到 50405 监听器）在 SELECT_CHARACTER 后走「待机区入场」分支：
   - 入场头三帧按官服字节重放（USERINFO32 / CHARAC_MIGRATION / RAID_INOUT flag=01）；
   - actor 的 864B USERINFO 位置体应携带待机区落点（区域 0x92 语义待与 PVF/world 数据核对）；
   - 频道身份（112B CHANNELINFO 等价帧，现私服 channel=87）应改发军团频道身份；
   - **不发送** user_area_sent town=38 / town_entry_probe（官服无此类推送）。
3. 登录服务器查询链路私服已可用（客户端已知重连 50405），无需改动。

## 5. 与既有结论的衔接

- 本次抓包同时验证：708/1198(176B)/1336 announce 序在重登录场景同样成立；
  108 EVENT_INFO 在重登录 announce 为 **2668B 完整表**（私服固定 54B 表仅用于首登录，待机区重连不受影响——
  但若黑屏复发需复查此处）。
- 客户端版本 2.38.2；dfocap opcodes.tsv 覆盖 CMD 2409 / NTF 2922 条。
