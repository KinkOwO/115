# next79 — 军团频道「这个时间不开门」：官服频道脚本解密与行值复刻

> 问题：用户私服实机军团 tab 全锁，点伊斯大陆弹「这个时间不开门」（dstr 100088632
> "This dungeon isn't open today."，入口门禁⑦ `sub_145695000` 开放日程查表）。
> 诉求：改成周一到周日都能进。
> 最新状态（2026-10-03）：§32完整四阶段、动画后回城、退出队伍已获用户100%确认。后续单机重复挑战/恢复次数为新候选，另见§33。

## 1. 证据源：官服 7101 频道目录抓包解密

既有抓包 `D:\115us\analysis-tools\output\official_20261002-160349.pcapng`（2026-10-02 16:03，
即**同一天官服能进伊斯大陆**的那次连接）里有 4 条与 `3.211.150.198:7101` 的会话——
这就是频道目录协议（私服 7001 端口的同族）。

**解密链**（`analysis-tools/output/decrypt_chscript.py`，与
`internal/channelrefresh/server.go` 的 `handle()/encrypted()` 互为印证）：

| 步骤 | 帧格式 | 内容 |
| --- | --- | --- |
| 客户端 cmd 11（32B body） | 11B 头 `[0]=0,[1]=11,[2:6]=n LE,[10]=1` | 客户端 preface |
| 服务端 cmd 12（36B body） | `[0]=0x7c`（官服魔数，私服用 1，客户端两者都收） | `4B 0 + 32B key`；当日 key = `20261002000014`+18 零；**AES key = key[:16]** |
| 客户端 cmd 9 → 服务端 cmd 10 | body = **zlib(AES-ECB(script))** | 脚本 13568B |
| 客户端 cmd 1 → 服务端 cmd 3 | 同上 | 目录 4576B |

产物：`analysis-tools/output/official_script.bin(.txt)`、`official_directory.bin`。

**目录结构**：`ver u32（官服=2，私服=1 均可）+ 20B 世界名 + count + 44B/行`，且含**两个世界块**：
`cain`（44 行，对应脚本 `[server] 1`）+ `siroco`（50 行，对应脚本 `[server] 3`）。行 tag 为
`#ch.<id>`（私服 `#<id>`，客户端 `Current1451fa5e0` 提取十进制数字，两者兼容）。

## 2. 关键定论

1. **SourceValues 假说被推翻**：官服脚本里**所有军团行（Ispins 86/87、Apocalypse 30/31/32、
   Venus 56/57、FoA 65/66、Dusky 80/81、myres 97/98…）的 11 个规则标量全部为 `0.0`**，
   与私服全 0 完全一致 ⇒ 「标量含开放星期位图」不成立。旧 PVF `etc/channel_info.etc`
   的 11 标量只是旧版区域参数（elven=10、granfloris=5、sky_catle=3 0 0 1…），也非日程。
2. **真正的行差异**（官服 vs 私服旧配置）：

| 字段 | 官服（可进） | 私服旧（弹「不开门」） |
| --- | --- | --- |
| Ispins 行 | **86、87 双行** | 81 单行 |
| Ispins Name | `Ispins` | `Ispins Legion` |
| Ispins Area | **`[ispins_legion]`** | `[none]` |
| `[dungeon]` 块 | `[dungeon]\n  \`[ispins_legion]\` \`Ispins\`\n\n[/dungeon]` | 无（[none] 不发块） |
| 末世录行 | **30、31、32 三行**，Area `[apocalypse]` | 119 单行，Area `[none]` |

3. **推理链**：同客户端、同本地时间，官服可进、私服被门禁⑦拒绝 ⇒ ⑦的判定输入含
   服务端下发的频道行数据；私服所有军团行都用 `[none]` 区域且无 `[dungeon]` 块，
   官服所有军团行都用官方区域键+空 `[dungeon]` 块。⇒ 复刻官服行值。
   （无 IDA 本机，无法进一步定位 `sub_145695000` 读的具体字段；复刻全部行字段是
   覆盖两种可能（Area 键 / [dungeon] 块）的唯一证据一致做法。）
4. 官服行为即「每天开放」（当前版本所有军团日常开放，仅限周奖励次数）⇒ 复刻行值
   即满足「周一到周日都能进」。

## 3. 修改

| 文件 | 内容 |
| --- | --- |
| `configs/channel.local34.json` | Ispins 行 → 86/87 双行（Type 81、`[ispins_legion]`、`Ispins`）；末世录行 → 30/31/32 三行（Type 119、`[apocalypse]`、`Apocalypse`，官服 cain 块行值，同时解 next66 遗留 D1）；新增 `DungeonTitles`（`[ispins_legion]`→`Ispins`、`[apocalypse]`→`Apocalypse`） |
| `internal/channelrefresh/server.go` | `Config.DungeonTitles` + `Load` 校验（非空、≤24、无反引号/换行）；`Script()`：titled 区域发官服形态空 `[dungeon]` 块；同行区域去重（官服每区域一块，Ispins 双行只发一次） |
| `internal/channelrefresh/server_test.go` | 末世录断言改为官服行形态；新增 Ispins 双行 + `[dungeon]` 块唯一性断言 |

不改动：普通频道行（旧 PVF 区域块保持现状，登录/进城已实机验证）；
`[server]` 块头格式（私服 `[server]\n1\n` 与官服 `[server] 1` token 等价，已验证可登录）。

## 4. 验证

- `go test ./internal/channelrefresh` 全过；`go vet` 通过。
- exe 重建：`bin/wireprobe-handoff-source.exe`（2026-10-02 20:08）。
- **待实机**：`停止游戏环境.cmd` → `启动服务端.cmd` → 选角色 → 军团 tab 点伊斯大陆：
  - 期望：不再弹「这个时间不开门」，进入 Ispins 频道（等待区 146）。
  - 若仍弹：⑦的输入不在频道行，下一步抓官服游戏频道内的 NOTI 差异（对照 s1/s4 已解码流）。

## 5. 遗留

- 其余军团（苏醒之森 96/`[foa_legion]`、幽暗岛 91/`[duskyisland_legion]`、次元回廊 84/`[myres_legion]`、
  Venus 99/`[venus_legion]` 等）官服行值已全部解出（§1 产物），私服未实现其协议，暂不加行；
  需要时按同法补。
- 官服脚本 `[server]` 块双世界（cain/siroco）结构：私服单块（ServerID=1）。若日后需要
  世界选择语义再议。

## 6. 第二轮：NOTI708 每周地下城日程覆盖包（2026-10-02 晚）

§4 实机结果：**行值复刻后仍弹「这个时间不开门」** ⇒ ⑦的判定输入不在频道行里。
用户截图给出决定性线索：伊斯大陆面板写着「今日无法进入军团地下城。**(周六、周日、
周一、周二、周三可进入)**」——即客户端本地日程**周四/周五关门**，而抓包当天
2026-10-02 恰是**周五**且官服可进。

### 6.1 定位

推理链：同客户端、同周五，官服能进 ⇒ 官服必有一包**覆盖了客户端本地日程**。
对官服进频道 announce 突发（s1/s4 解码流，`official_20261002-160349_decoded/`）逐 id 查
`analysis/dumps/opcodes.tsv`，唯一日程类候选：

| 帧 | id | 名称 |
| --- | --- | --- |
| s1 帧 8（紧跟 N1370 story digest） | **708** | `ENUM_NOTIPACKET_WEEKLY_DUNGEON_INFO_CONFIG`（每周地下城开放日程配置） |

- 载荷 24B，s1 与 s4 **逐字节相同** ⇒ 静态配置而非会话计算。
- 私服进场帧组从未发过 708 ⇒ 客户端回落本地日程（周四/周五关门）→ 门禁⑦弹窗。
- 用户日志（`runtime/roles_persist_..._next37/events.jsonl`）证实被挡时**不发包**，
  纯客户端判定，与门禁⑦取证（next73）一致。

### 6.2 修改

| 文件 | 内容 |
| --- | --- |
| `cmd/wireprobe/entry_flow.go` | 新增 `weeklyDungeonInfoConfig` 常量（官服 24B 原文，**不解释字段语义**，遵守项目「不猜包」规矩）；`packets()` 在 N1370 之后、N2 之前插入 `{"weekly_dungeon_config_sent", 0, 708, ...}`（对齐官服顺序） |
| `cmd/wireprobe/story_digest_test.go` | 新增 `TestWeeklyDungeonConfigFollowsStoryDigest`：断言 708 存在、位置 1370<708<2、body 与官服抓包逐字节一致 |

### 6.3 官服 24B 原文

```
32 33 36 3f 3e 3b 3a 39 ff ff 00 00 00 00 00 00 00 00 51 fc 40 ed 35 00
```

（前 8 字节疑似星期位图、`ff ff` 疑似全开标记——仅为猜测，代码按原文回放，不按语义构造。）

### 6.4 验证

- 定向 `go test ./cmd/wireprobe -run "TestWeeklyDungeonConfig|TestStoryDigest"` 通过；`go vet` 干净。
- 全量 `go test ./cmd/wireprobe -count=1` 有 3 个失败（`TestAdventureAuditProvenanceAllowanceIsNarrow`、
  `TestPVFCatalogGateRefusesRewardChangesAndDoesNotFallback`、
  `TestEnhancementAuditAllowsOnlyMissingOrdinaryTicketExpirationHeader`），均为 PVF 目录/
  强化审计迁移半成品的**预先存在失败**，与 NOTI708 改动无文件交集。
- exe 重建：`bin/wireprobe-handoff-source.exe`（2026-10-02 20:31）。
- **待实机**：重启服务端 → 军团 tab 点伊斯大陆，期望不再弹「这个时间不开门」。
  若仍弹：下一步对照官服 announce 突发中 708 之外的第二候选（或登录阶段下发的日程数据）。

## 7. 第三轮：N2254 军团状态底座 + 二进制发布链路（2026-10-02 深夜）

### 7.1 二进制发布链路踩坑（假测试）

§6 之后用户复测「锁没了，但点击没反应」。日志排查发现**两次测试全是假测试**：
`启动服务端.cmd`/一键启动器默认跑 `bin/wireprobe-pvf.exe`（`configs/pvf-default.json`
profile 的权威 binary），而两轮修复都只重建了 `wireprobe-handoff-source.exe`——
发布需要 `Build-Server.ps1 -UpdatePVFDefault` 或手动 Copy-Item（服务端运行中文件被锁）。
旧 exe（12:51 构建）重启会读到新 `channel.local34.json`（运行时数据）⇒ 出现
「频道行修复生效（锁消失）但代码修复不生效（无 708、无 Ispins 接入、点击无包）」的假象。
教训已记入项目记忆。

### 7.2 N2254 只发伊斯频道 ⇒ 普通频道军团 tab 门禁静默拦截

新 exe 确认在跑（20:43 会话日志有 `weekly_dungeon_config_sent 708`）后，用户点击伊斯
大陆**依然连 CMD2043 都不发**（events.jsonl 全程无 2043）——纯客户端门禁拦截。

对照官服进频道 announce 突发：**s1 f265 / s4 f257 都带 N2254（272B，世界装载串尾部）**
——普通频道的 announce 也发。用 `cmd/framedump`（临时取证工具，用私服 wire 包解密
官服抓包单帧）提取 f265 全文，与既有 `IspinsEntryCharacterInfo(true,...)` 登录模板
（f257）**逐字节一致**。而私服实现只在 `channelType == 81` 时发送 ⇒ 普通频道客户端
军团状态未装载，入口门禁静默拦截。

### 7.3 修改与验证

| 文件 | 内容 |
| --- | --- |
| `cmd/wireprobe/main.go` | 选角装载串的 N2254 发送条件从 `channelType == 81` 放开为**每次选角都发**（官服双会话实证：普通频道 announce 也带） |

- 官服 f265 原文（272B，与登录模板一致）：flag 布局 101=7f、102=7f、105=7f，其余 00（§6.3 之外的完整取证见 framedump 输出）。
- `go vet` 通过；定向测试通过；exe 已重建并**发布到 wireprobe-pvf.exe**（20:50）。
- **待实机**：重启 → 军团 tab 点伊斯大陆，期望这次点击真正发出 CMD2043 并进入等待区。

## 8. 第四轮：周常账本帧 N1336 / N537（2026-10-02 深夜）

### 8.1 实测现象与甄别

§7 发布后实机复测：军团 tab **完整点亮**（N2254 生效：伊斯大陆条目、入场名望 15,538、
每周挑战 周刊:3/3、奖励次数全部显示），面板上「今日无法进入…(周六~周三可进入)」的
本地日程文本**已消失**（N708 生效），但点击仍弹「这个时间不开门」，且 events.jsonl
仍无 CMD2043——客户端门禁⑦仍在拦截。

对照官服抓包做**成功会话反推**：CMD2043 是从 **s4** 发出的（`session_s4_c2s.txt`
frame 170）。关键甄别：

- **N706 WEEKLY_DUNGEON_INFO（1464B 大表）：s4 里根本没有** ⇒ 排除（s1 有、s4 无，
  与点击成功无关）。
- s4 announce 里我们没发、且名字即周常的静态帧只剩两个：
  **N1336 `WEEKLY_DUNGEON_INOUT_INFO`**（帧 60，16B，
  `00000000 00000000 f4f769b2 3a00 0000`）与
  **N537 `DUNGEON_ENTER_COUNT_INFO`**（帧 91/142 各一次，16B，
  `c40d0000 0500 d5cd28ca 3b00 00000000`）——s1/s4 逐字节一致（静态）。
- N2255/2256（失落之地：伊斯大陆）在 s4 里都在帧 376+，即 CMD2043 **之后**，
  属进本流程（ispins_flow 已实现），不是入场门禁输入。

推理：708 覆盖了「日程显示文本」，但门禁⑦查的周常地下城**账本**（本周出入/入场计数）
从未下发，账本缺失即视为不可入。⇒ 按官服原文回放 N1336 + N537。

### 8.2 修改与验证

| 文件 | 内容 |
| --- | --- |
| `cmd/wireprobe/entry_flow.go` | 新增 `weeklyDungeonInOutInfo`（N1336 16B）与 `dungeonEnterCountInfo`（N537 16B）官服原文常量；`packets()` 在 708 之后、N2 之前插入两帧（官服帧序 708@8 → 1336@60 → 537@91） |
| `cmd/wireprobe/story_digest_test.go` | 新增 `TestWeeklyDungeonLedgerFollowsConfig`：存在性 + 顺序 708<1336<537<2 + 两帧黄金向量 |

- 取证工具：`cmd/framedump`（本次新增，用私服 wire 包解密官服抓包单帧——wiredecode.exe
  的 plain 显示截断，全帧取证用它）。
- `go vet` 通过；定向测试通过；exe 已重建并发布 `wireprobe-pvf.exe`（21:07）。
- **待实机**：重启 → 军团 tab 点伊斯大陆，期望 CMD2043 真正发出并进入等待区。
  若仍拦：下一候选是 s4 announce 与我们发送集合的完整差集里再筛（已知排除 706/2255/2256）。

## 9. 第五轮：N706 WEEKLY_DUNGEON_INFO 大表（2026-10-02 深夜）

### 9.1 §8 排除逻辑的漏洞与 706 复活

§8 发布后实测：708/1336/537/2254 全部确认已发（日志逐一可见），点击**仍**无 CMD2043、
仍弹「这个时间不开门」。

复盘 §8 对 N706 的排除——「s4 announce 里没有 ⇒ 与点击成功无关」——有漏洞：
**s1 和 s4 是同一客户端进程的先后两条连接**（用户先在 s1 进频道、后换到 s4），
N706 的表在 s1 装入后**跨连接保留**到 s4 完全可能。而 706 是整个抓包里唯一一个
名字就叫 `WEEKLY_DUNGEON_INFO`（1464B，大量 `ffffff9c`=-100 条目）的帧。
⇒ 706 重新成为头号候选，按 s1 原文回放。

### 9.2 修改与验证

| 文件 | 内容 |
| --- | --- |
| `cmd/wireprobe/weekly_dungeon_info_generated.go` | 新增（生成自 `D:\115us\analysis-tools\output\gen_706_vector.py`）：N706 官服 s1 帧 78 的 1464B 原文十六进制 + `mustHexDecode` |
| `cmd/wireprobe/entry_flow.go` | `packets()` 在 1336 与 537 之间插入 `{"weekly_dungeon_info_sent", 0, 706, weeklyDungeonInfoTable}`（官服帧序 1336@60 → 706@78 → 537@91） |
| `cmd/wireprobe/story_digest_test.go` | `TestWeeklyDungeonLedgerFollowsConfig` 扩展：706 存在性、顺序、1464B 长度与前 7 字节黄金前缀 |

- IDA 反编译路线受阻：`D:\tools\ida94` 与 `D:\115us-backup\ida-work\DFO.exe.i64` 均
  不在本机（`run-weekly-gate.cmd`/`ida_weekly_gate.py` 已备好，机器可用时跑
  `analysis/dumps/weekly-gate/summary.json` 出门禁⑦真源）。
- `go vet` 通过；定向测试通过。**待发布**（服务端运行中，需 `停止游戏环境.cmd` 后
  Copy-Item 发布 wireprobe-pvf.exe）。
- **待实机**：重启 → 军团 tab 点伊斯大陆。若 706 仍不是门禁⑦的输入，下一步走
  Frida 动态取证（本机已装 `C:\Program Files\Frida`）：hook `sub_145695000`
  （DFO.exe 基址 + 0x5695000）看点击时的实参与返回值，直接定位查表源。

## 10. 第六轮：门禁⑦真源 = ENABLE_CHANNEL_TAB_EVENT_ID 活动查表 → N108 EVENT_INFO 回放（2026-10-02 深夜）

### 10.1 静态逆向（用户提供的去壳客户端 DFO.exe / DFO_KEYFIX.exe）

706 回放后实测仍拦。用户在 D:\115us 放置去壳客户端（246MB，PE 64 位、基址 0x140000000、
.text RVA==文件偏移），静态逆向（capstone + pefile，工具 `analysis-tools/output/disasm.py`）：

1. 全 .text 扫描 dstr 100088632（0x5f73b38）立即数 → 6 命中，门禁区 2 处（0x1425113b8 / 0x142512698）。
2. 反汇编 0x142511340-0x142511440 确定门禁⑦调用链：
   - `mov rdi,[rdi+0x80]` → 频道窗口当前行的**属性容器**
   - `lea rcx,[rip+0x731af88]` → 0x14982C2E0 编码字符串 → 解码 = **ENABLE_CHANNEL_TAB_EVENT_ID**
   - `call 0x1476e0690` → 具名查表(container, name) → eax = 活动 id（未命中返回 -1）
   - `call 0x14021a370` → 取 event mode 单例（0x9c0B，构造 0x1456921b0，位图在 +0x458，memset 0）
   - `call 0x1456958c0` → **活动位图判定**：id ≤ 0x2af7(11000) 测 `[mode+0x458+(id>>6)*8]` 的 `(id&0x3f)` 位；> 11000 走 +0x30 树。返回 0 → 弹「这个时间不开门」
3. 客户端字符串加密完整逆向（0x146e8d090 快路径 / 0x146e8cd50：首 word==0x2CA1 明文标记；
   否则 key0=(hdr[1]&0xFE)|0x9a714ca0、长度=(((h1<<7)|h1)^word_at(+2))*2、+4 起密钥流 XOR：
   dword: plain=key^cipher; key=key*0x1003f+plain；字节尾: plain=(key&0xFF)^cipher; key=key*0x107+plain）。
   工具化于 `analysis-tools/output/lookup_names.py`（解码 0x1476e0690 的 28 个调用点名）。
4. 28 个调用点的名字全部是**频道行字段**（CHANNEL_BASE_IMAGE_INDEX / CHANNEL_DISABLE_MSG /
   REWARD_ITEM_DUNGEON_INDEX / CREATE_LEGION_* …）⇒ 容器 = 当前频道行的键值表，数据源 =
   **legionsystem.cos 的 [ui data][int list]**——服务端仓库已有解析
   （internal/catalog/legion_contents.go，Ispins = 776，末世录 = 1007，见
   configs/legion-contents.generated.json）。
5. ⇒ 门禁⑦真因：**客户端查行内 ENABLE_CHANNEL_TAB_EVENT_ID=776，再查活动 776 是否开启；
   位图默认全 0，开启数据必须来自服务端。** 708/1336/537/706/2254 都不是这个系统的输入。

### 10.2 数据源定位与回放

- 官服 announce（s1/s4 各两次，帧 58/82 与 58/74，字节相同）：**NOTI108 EVENT_INFO**，
  body 2624B **zlib 流**（789c…），解压 6829B 活动日程表。
- 解压后含 **"Ispins Legion Open" = 活动 776**（时间窗 2023-02-10 → 2034-01-04，覆盖抓包日）、
  "Apocalypse Channel" = 1007、"Venus Open"、"Dusky Island Open" 等全部频道开门活动。
  记录布局：`[id u16][3B 标志][名长 u32][名字][pad][start u32][end u32]`（unix 秒）。
- 生成：`analysis-tools/output/gen_108_vector.py`（输入 %TEMP%\n108.hex）→
  `cmd/wireprobe/event_info_generated.go`（2624B 十六进制 + mustHexDecode）。

### 10.3 修改与验证

| 文件 | 内容 |
| --- | --- |
| `cmd/wireprobe/event_info_generated.go` | 新增：N108 官服原文（zlib 流原样，客户端自行解压） |
| `cmd/wireprobe/entry_flow.go` | `packets()` 在 708 与 1336 之间插入 `{"event_info_sent", 0, 108, eventInfoTable}`（官服帧序 708@8 → 108@58 → 1336@60） |
| `cmd/wireprobe/story_digest_test.go` | 新增 `TestEventInfoFollowsWeeklyConfig`：存在性 + 顺序 708<108<1336 + 2624B 长度 + 789c 前缀 + inflate 后 6829B + 包含 "Ispins Legion Open" |

- `go vet` 通过；定向测试通过；全量仅 3 个预先存在的 PVF 审计失败（与本改动无交集）。
- exe 已重建 **并已发布 wireprobe-pvf.exe**（21:43，服务端未运行无锁）。
- **待实机**：启动服务端 → 选角 → 频道窗口军团 tab 点伊斯大陆 → 期望不再弹
  「这个时间不开门」、CMD2043 发出并进入等待区。

## 11. 第七轮：N108 回放回归（进镇卡死）与 N1198 前置修复（2026-10-02 深夜）

### 11.1 回归现象与取证

- 21:43 发布含 N108 的 exe 后，用户实测**进镇后客户端完全冻结**（画面渲染完成、
  弹出本该隐藏的「解放痕迹」获得面板）。
- 冻结会话（runtime `..._20261002_214529_318852_next37/events.jsonl`）：announce 于
  13:49:04.153 发出（含 708→108→1336），客户端反应 31/143/433/67/707/**217** 后
  彻底静默直到用户杀进程。**CMD217 = `ENUM_CMDPACKET_OVERFLOW_INFO`**，plain
  `006c000000000000`（0x6c=108，报告的就是 NOTI108 溢出），服务端把它记为
  `unimplemented_sample` 无应答。
- 对照：健康会话（21:20 旧 exe）与官服抓包中**从未出现 CMD217**；健康会话在
  announce 后立即开始 CMD2127 心跳。

### 11.2 根因假说与处置

- 官服 announce 帧序（s1）：708@8 → **1198@15** → 108@58 → 108@82(重复) → 1336@60。
  **NOTI1198 INTEGRATE_EVENT_DATA（176B）在 108 之前**——客户端吸收 108 表前需要
  1198 建立事件容器状态，缺失则报 CMD217 溢出并冻结（H1，头号嫌疑）。
- 向量：s1 帧 15（offset 2795）/ s4 帧 15（offset 2794，**framedump 用十进制 offset**，
  首次误用 2795 对 s4 取到错帧 id=49156）提取 176B，**两 session 字节相同**。
  布局要点：u32 0x0942 开头、+4 flag 1、+25 = 2、+0xA5 抓包时间戳 0x3f1095d69f、
  其余全零；不做语义解读（项目规矩）。
- 生成：`analysis-tools/output/gen_1198_var.py` → entry_flow.go 内
  `var integrateEventData = mustHexDecode(...)`（手写字节行首次写错总数/偏移，
  改为脚本生成 + 测试断言双保险）。

### 11.3 修改与验证

| 文件 | 内容 |
| --- | --- |
| `cmd/wireprobe/entry_flow.go` | 新增 `integrateEventData`（176B 官服原文）；`packets()` 在 708 与 108 之间插入 `{"integrate_event_data_sent", 0, 1198, integrateEventData}`（官服帧序 708@8 → 1198@15 → 108@58） |
| `cmd/wireprobe/story_digest_test.go` | `TestEventInfoFollowsWeeklyConfig` 扩展：1198 存在性 + 顺序 708<1198<108<1336 + 176B 逐字节断言（02@+25、ts@+0xA5） |

- `go vet` 通过；定向测试通过（测试同时证明 mustHexDecode 字符串与抓包原件逐字节一致）。
- exe 已重建并发布 wireprobe-pvf.exe（22:12，服务端未运行无锁；Select-String 确认
  两 exe 均含 1198 常量）。
- **待实机**：`启动服务端.cmd` → 进镇不卡死 → 军团 tab 点伊斯大陆。
- **若仍卡死**：备选 A 回滚 108（恢复可玩）；B 调整 108 位置；C 解压 6829B 表仅保留
  频道开门活动（Ispins 776 / Apocalypse 1007 / Venus / Dusky Island / Forest of
  Awakening / DefaultEvent ENTER_*）后重压缩（H2：表内 web 弹窗事件（libcef）在私服
  环境挂起）。

## 12. 第八轮：H1 被否定 → 方案 C 过滤表（2026-10-02 深夜）

- 22:14 实测：1198 已按官服序（708→1198→108→1336→706）发出，客户端**仍回 CMD217
  （0x6c=108）后冻结**（runtime `..._221448_775749_next37`，14:16:41.358 后静默）。
  H1（缺 1198 容器前置）否定。
- 表结构完全解剖（`walk_108_records.py`）：全局头 u16 计数=76；记录 =
  `[id u16][flags 3B][name_len u32][名字][pad][25B tail]` + 可选参数串；tail 内含
  `[start u32][end u32]`（unix 秒，Ispins 2023-02-21→2033-12-31）。74 条可验证 +
  2 条空/短名（id 621 @0x0fd2 空名）。官服 s1 帧 58/82 两帧 108 字节相同（非分段）。
- **方案 C 实施**（`gen_108_filtered.py`）：保留频道行引用的全部 7 个频道开门活动
  （legionsystem.cos `[ui data][int list]` = 621/627/642/680/681/776/1007），
  记录字节**原样**复制（每条尺寸精确 = 9+名长+25，无参数串），仅改计数头 76→7 并
  重压缩（zlib level 6，789c）。6829B→356B（解压）/ 2624B→202B（压缩）。7 条时间窗
  全部覆盖 2026-10-02。附带收益：web 弹窗/商店/日历类活动全部移除，进镇弹出的
  「解放痕迹」获得面板应一并消失。
- 修改：`event_info_generated.go` 重写为过滤表（含推导注释）；`story_digest_test.go`
  断言更新（202B/356B/计数 7/六个开门活动名）。`go vet`+测试通过；发布
  wireprobe-pvf.exe（22:26）。
- **待实机**：进镇不卡死 + 军团 tab 点伊斯大陆。若 7 条仍溢出，说明私服客户端的
  108 解析器与官服版本不兼容（版本差异），下一步仅保留 776/1007 两条或彻底换思路。

## 13. 第九轮：真根因 = 阶段放错 —— 108/1198 属于登录洪流（2026-10-02 深夜）

### 13.1 决定性证据（用户提供的完整协议解析目录）

用户提供 `D:\115us\DFO-115US-抓包协议完全解析`（主文档 + 附录A/B + 21:18 完整会话
frames.jsonl，26MB 含 plain_hex）。其第 6 章明确 EVENT_INFO(0x006C) 与
INTEGRATE_EVENT_DATA(0x04AE=1198) 属于「登录后 1 秒内的账号级数据洪流」（选角之前）。

对 `20261002-211855_.../frames.jsonl` 逐帧过滤 op∈{1,4,108,1198,124,1759,708,1336,706,537,782}
按时间排序，**三条业务连接帧序完全一致**：

```
S2C 1(CHANNELINFO,527) → C2S 1(LOGIN) → S2C 1759(SELECT_CHARACVIEW_BG) →
S2C 708 → S2C 1198 → S2C 108(len=2668) → S2C 1336 → S2C 1(len=112) →
C2S 782 → S2C 782(32B) → C2S 4(SELECT_CHARACTER) →
S2C 706 → S2C 108(第二次,len=2668) → S2C 4(STAMINA) → S2C 537 →
S2C 124(ENTER_GAMEWORLD_COMPLETE) → S2C 537
```

关键结论：

1. **官服把 708/1198/108/1336 放在登录洪流（选角前），不是选角后的进镇 announce**。
   前几轮把它们放进 announce ⇒ 阶段放错。客户端的 108 处理器要求事件容器在登录阶段
   建立，选角后单独收到 108 就报 CMD217 溢出并冻结——这解释了第七、八轮两次修复
   （补 1198、过滤表到 7 条）均无效。
2. **官服 108 发两次**：登录洪流一次 + 选角后（706 之后）一次。第二份依赖第一份
   建立的容器。
3. 16:03 抓包的 s1 帧 7-60（1370@7/708@8/1198@15/108@58/1336@60）实为登录洪流，
   之前误标为「频道进入 announce」——第二/四轮的插帧位置注释全部基于这个误标。

### 13.2 修改

| 文件 | 内容 |
| --- | --- |
| `cmd/wireprobe/entry_flow.go` | 新增 `loginFloodPackets()`（708→1198→108→1336 官服洪流序）；`packets()` 移除 708/1198/1336，108 挪到 706 之后（对齐官服选角后序 706→108→537，作为"第二份"） |
| `cmd/wireprobe/main.go` | 每连接守卫 `loginEventFloodSent`；在 CMD8(mode2) 角色列表应答 + `restoreRosterBackgrounds`（1759）之后、选角请求之前，`sendPlan(loginFloodPackets())` 发送洪流一次（对齐官服 1759→洪流→SELECT 顺序；两处 bootstrapped=false 重置点均紧跟连接关闭 return，无需重置守卫） |
| `cmd/wireprobe/story_digest_test.go` | 重写：`TestLoginEventFloodOrder`（洪流 4 帧顺序 + 708/1198/1336 黄金体 + 108 表内容）；`TestWeeklyDungeonLedgerFollowsConfig`（announce 序 1370<706<108<537<2 + 706/537 黄金体 + 1198/1336 不回流 announce）；`TestWeeklyDungeonConfigLeftTheAnnounce`（708 不回流）；`TestEventInfoSecondCopyRidesAnnounce`（announce 恰一份 108 且与洪流表逐字节一致） |

### 13.3 验证与发布

- `go vet` 通过；定向测试（TestLoginEventFlood/TestWeeklyDungeon/TestEventInfo/TestStoryDigest）全过。
- exe 重建并发布 `wireprobe-pvf.exe`（22:45，服务端未运行无锁）。
- **待实机**：`启动服务端.cmd` → 登录选角（期望洪流在选角界面阶段发出）→ 进镇
  不卡死 → 军团 tab 点伊斯大陆不弹「这个时间不开门」、CMD2043 发出进入等待区。
- **若仍冻结**：备选 = 登录洪流改用完整官服表（21:18 抓包 108 body 2652B，可从
  frames.jsonl 提取 plain_hex）；或回滚 108 保可玩再深挖客户端 108 解析器版本差异。

## 14. 第十轮：阶段修复后仍 CMD217 → 真根因二 = 私服客户端 108 不吃 zlib，改发裸表（2026-10-02 深夜）

### 14.1 实测与对照

- 22:45 版实测：**点击「游戏开始」无反应**（客户端卡在选角屏，连 CMD782/CMD4 都不发），
  退出时弹「公告/结束比赛…」窗（截图取证）。
- runtime `..._224712_933446_next37`：登录 → CMD8 → NOTI2+1759 → **洪流
  （708/1198/108/1336）** → 客户端 848/433/637/2127/433 → **CMD217** → 静默关闭。
- 对照健康会话（22:28 版）：同一位置后客户端发 407 → 心跳 → **782 → CMD4（选角成功）**。
  ⇒ 洪流四帧中有一个让客户端在选角阶段就卡死。

### 14.2 决定性证据：私服客户端的 zlib 语义与官服客户端不同

1. 官服 21:18 抓包（官服客户端）里 108/2826/1336/537 等**大量配置帧都是 zlib 体**。
2. 我们私服一直发的 NOTI2826（UNIFIED_OPTION_ACCOUNT）是 **3648B 裸模板**
   （`internal/game/protocol/account_options.go` 直接写原始字段），私服客户端消化正常；
   官服抓包里同帧是 144/208B zlib。⇒ **私服客户端对官服压缩帧吃的是裸体**。
3. 检查全部健康会话日志：我们发给私服客户端的帧**没有任何一个是 zlib 体**（无 plain_hex
   以 789c 开头）——**唯一发过 zlib 体的帧就是 108，唯一触发 CMD217 的帧也是 108**
   （四轮复现：全量表 21:43 / +1198 22:12 / 过滤表 22:26 / 登录洪流 22:45）。
4. CMD217 = `ENUM_CMDPACKET_OVERFLOW_INFO` 语义吻合：zlib 头 `78 9c` 被私服客户端
   当裸表头解析，记录数读成 0x9c78（u16 LE，40056 条）→ 解析器越界 → 报「溢出」。

### 14.3 修改

| 文件 | 内容 |
| --- | --- |
| `cmd/wireprobe/event_info_generated.go` | 108 体改为**解压后的裸表**（356B，计数 7，记录字节与官服解压内容逐字节一致）；生成脚本 `analysis-tools/output/gen_108_raw.py`（inflate 现有 zlib 向量 + 断言） |
| `cmd/wireprobe/story_digest_test.go` | 断言改为：356B 裸表、**禁止 789c 前缀**（防回归 zlib）、计数 7、六活动名；移除不再使用的 zlib/io 导入 |

不动：洪流位置（1759 后选角前）与 announce 第二份 108（706 之后）保持第九轮布局。

### 14.4 验证与发布

- `go vet` + 定向测试通过；发布 wireprobe-pvf.exe（22:55）。
- **待实机**：`启动服务端.cmd` → 选角点「游戏开始」有反应 → 进镇不卡死 → 军团 tab
  点伊斯大陆不弹「这个时间不开门」、CMD2043 发出进入等待区。
- **若仍 217**：说明私服客户端 108 裸表布局也与官服解压表不同（版本差异），下一步
  反汇编私服客户端（去壳 DFO.exe 已在 D:\115us）108 处理器定真格式；或回滚 108 保可玩。

## 15. 第十一轮（2026-10-02 23:31）：裸表仍 CMD217 → 静态 RE 穷尽 → 黑盒差分实验

### 15.1 实测结论

第十轮发布（22:55，登录洪流 708→1198→108(裸表356B)→1336 + announce 第二份裸表）实测
**仍在选角屏卡死**。五轮字节级官服复刻全部触发 CMD217（21:43 / 22:12 / 22:26 / 22:45 / 23:00
前后），否定一切"位置/编码/前置/表大小"假说：**私服客户端的 108 解析器与官服当前版本格式不同**。

### 15.2 静态 RE 取证（本窗口，工具全部落 analysis-tools/output/）

对去壳私服 DFO.exe（.text VA 0x140001000 size 0x9186000）：

- **event-mode 类**：对象 0x9c0B；单例槽 [rip+0x14E635248]；位图区 +0x458（memset 0x560，
  172 qword 覆盖 id 0..11007）；id 阈值 0x2AF7(10999)：≤走位图、>走 +0x30 std::map。
- **锚点全灭**：单例 getter 全内联（槽引用 11158 处）；opcode 分发无 cmp 立即数链（11 处无
  聚类）、无数据段跳转表（.rdata 16723 项长龙 0x1491889a0 是 CRT _initterm 初始化表）；
  NOTI 注册走通用 pubsub `0x14668e570`（edx 类型值 12/231/232 被注册成百次，按运行时
  string pool 名键控，静态不可枚举）；protobuf 仅 66 消息无 EVENT_INFO；CMD217 发送方
  0xD9+0x6C 扫描仅 1 误报（fld 操作数字节）。
- 结论：常规静态锚点已穷尽，转黑盒。

### 15.3 黑盒差分实验设计（本轮实现）

1. **登录洪流永久去掉 108**（保 708→1198→1336）→ 登录阶段恒健康；本服务端唯一 108
   是 announce 探针。
2. **announce 的 108 改为每次进镇轮换下一个变体**，main.go 在 announce 构建前
   `advanceEventInfoProbe()` 并记录 `event_info_probe_selected`（含变体名与字节数）。
3. 变体表（`cmd/wireprobe/event_info_probe.go`）：

| 变体 | 体 | 说明 |
| --- | --- | --- |
| V0_skip_no_108 | 无帧 | 纯基线；此轮出现 217 = 非 108 原因，整轮作废 |
| V1_raw_empty_table | `0000`（2B） | 最小裸表 count=0 |
| V2_zlib_empty_table | `789c6360000000020001`（10B） | 官方容器最小内容：区分"容器被拒"与"内容被拒" |
| V3_raw_ispins_only | `0100`+52B 记录（54B） | count=1 仅 Ispins 记录，从过滤表切片 |
| V4_raw_filtered7_known_bad | 356B 裸表 | 已知必死（第 9/10 轮），阳性对照 |
| V5_zlib_official_2624_known_bad | 2624B 官方 zlib | 已知必死（第 7-11 轮），阳性对照 |

   V4/V5 若不再触发 217 ⇒ 轮换机制本身坏了（announce 没送达），当轮数据作废。
4. 判读：`event_info_probe_selected` 之后紧跟 CMD217 = 该变体失败；干净进镇 = 该变体通过。
   Ispins 门禁在空表变体下必然锁 —— 本实验目标是**解析通过**而非门禁解锁。

### 15.4 修改与验证

| 文件 | 内容 |
| --- | --- |
| `cmd/wireprobe/event_info_probe.go`（新） | 探针变体表 + 轮换计数器 + `advanceEventInfoProbe`/`currentEventInfoProbe`（互斥锁保护） |
| `cmd/wireprobe/event_info_generated.go` | 追加 `eventInfoOfficialZlibHex`（2624B 官方 zlib 体，V5 数据源） |
| `cmd/wireprobe/entry_flow.go` | `loginFloodPackets` 移除 108；`packets()` 的 108 条目改 `event_info_probe_sent` + 探针体（skip 变体由 preparePackets 丢弃空载荷） |
| `cmd/wireprobe/main.go` | announce 构建前轮换探针并记录 `event_info_probe_selected` |
| `cmd/wireprobe/story_digest_test.go` | 洪流断言改 708→1198→1336 且禁 108；新增 TestEventInfoProbeRidesAnnounce / Rotation / Bodies；ledger 序列改 706→(108探针)→537 |

`go vet` 通过；探针/storydigest 全部测试通过（3 个 PVF 目录门测试失败为工作区既有问题，
与本轮无关）。发布 wireprobe-pvf.exe（23:31）。

### 15.5 实机操作（待用户执行）

`启动服务端.cmd` → **连续 6 次登录**（每次选同一角色进镇；若卡死/被踢就退出客户端重登，
不要重启服务端）→ 6 个变体各测一次。之后读 events.jsonl：每条
`event_info_probe_selected` 的变体名 + 其后 60 秒内有无 CMD217。

- V0 卡死 ⇒ 问题根本不在 108（本轮全部结论作废，重查）。
- V1/V2/V3 有通过项 ⇒ 私服 108 真格式浮出（容器/记录布局），据此重做正式向量。
- V1..V5 全灭但 V0 活 ⇒ 客户端对任何非空 108 都拒 ⇒ 回滚 108 保可玩，Ispins 挂起。


## 16. 第十二~十五轮（2026-10-03）：108 固化 → 342 战线 → 待机区入场官服复刻

黑盒差分实验出结果后一路推进，本节只记结论链，细节散见各轮提交与抓包文档：

1. **V3 探针转正（第十二轮）**：V3（裸表 count=1 仅 Ispins 记录）通过且伊斯大陆界面可开。
   探针轮换机制整体删除（event_info_probe.go 删除），108 固化为 54B 表（id 776，时间窗
   2023-02-10..2034-01-04），测试改 TestEventInfoTableRidesAnnounce / TestEventInfoTableBody。
   发布 00:22。
2. **门禁弹窗与 342 战线（第十三轮）**：「必须完成任务『熄灭火焰的时间』」弹窗排查中确认：
   342 不能 zlib 压缩（私服 2.38.2 客户端 zlib 342 直接闪退，回滚裸发）；342/21/2310 必须
   在 actor_appearance_ready 之后、NOTI2827 之前发（g_QuestManager115 未初始化会静默丢插
   入）。02:17 版本后门禁弹窗消失，频道可点。
3. **待机区入场官服复刻（第十四~十五轮）**：dfocap.exe 实时解密抓官服待机区入场
   （analysis/ispins-standby-official-capture-20261003.md，1310 帧基准）。私服 Type 81 分支：
   落点 146/0 (562,234) 会话内位置（不写城镇存档，world_flow.go enter()），区内移动按军团
   频道会话处理；N2254/N733 撞装载期硬崩 → 改 pending 标志、入场后首帧 c2s 触发、N733
   待机不发。两次实测仍闪退 → 根因另见 §17。

## 17. 第十六轮（2026-10-03 03:52）：待机区闪退根因闭环 = 快捷栏双行 + N2254 待机体

### 17.1 取证（client_trace + 实时抓包 + psql 三方闭环）

- 032305 会话 client_trace.txt（12.4MB）：findItemSlot fail - multiple slot issues in
  the inventory - itemIndex : 10418028 共 **79,374 次**。城镇会话 140-320 次/秒（会话只活
  11 秒，用户点了切换频道，并非自愈）；第一次待机 19:25:01-13；第二次 19:25:16 起 112 次/秒
  稳定跑满 11 分钟至 19:36:04 崩（尾部 Close IRDPopupWindow Type:2875 → GameSocket
  disconnected）。待机区场景从未可交互（零移动帧），城镇里循环与可玩性共存。
- psql 实证 char 7 inventory.items 两行：slot 3 ×12 + slot 75 ×40（template 10418028，
  [waste]，新年自动恢复药水）。10418028 无显式 [stack limit] → stackLimitFor =
  MaxInt32，合并 52 合法。
- 成因链：用户把 12 个药水拖上快捷栏（CMD19 65→3，MoveStackable 整堆搬）后，odyssey
  发放 ×30 走 Add 只认类型段 65..120 → 段内另起 75 行 → 同模板双行。SweepStackSlots 的
  row.Slot <= 8 豁免使 belt 脏行永久幸存；addStackable/AddMailItem 同病。
- 排除项：op36 FATIGUE 非杀手（客户端在 op36 前已冻死，官服也发）；客户端资源齐全；
  优雅退出（1356+1402+op3）是用户点切换频道，非崩溃。

### 17.2 N2254 待机体逐字节对齐

官服待机入场 N2254（ispins_switch_frames.json 帧 331/997，272B）vs 模板 diff：
run 模板 + 四 stage 旗标全开（偏移 2/4/6/8）+ 偏移 21/117 = 0x7f（语义未解，官服原文），
nonce 5B 外与 run 基底零差异；vs login 模板差 0..8 五个旗标。私服原发 login 模板（旗标
全零）—— 待机场景拿到了一个它从未进入的军团状态。

### 17.3 修改

| 文件 | 内容 |
| --- | --- |
| internal/inventory/bag.go | SweepStackSlots：belt 行不再整体豁免，同模板段内有行且能吸收时并入（期限不同不并，段内满上限保留剩余），唯一 belt 堆仍受保护；Add 合并扫描加 r.Quick(row.Slot) 臂 |
| internal/inventory/shop.go | addStackable 同加 belt 臂 |
| internal/inventory/mailbox.go | AddMailItem 同加 belt 臂 |
| internal/legion/ispins.go | 新增 IspinsStandbyEntryCharacterInfo（run 模板+全旗标+21/117=0x7f） |
| cmd/wireprobe/main.go | 待机入场 N2254 补发（first_post_entry_frame 触发）改用待机体 |
| internal/inventory/quickslot_duplicate_test.go（新） | belt 双行清扫合并 + 幂等 + 唯一 belt 堆保护；Add/Buy/AddMailItem 三路径并进 belt 堆 |
| internal/legion/ispins_test.go | TestIspinsStandbyEntryCharacterInfo（vs run 恰差 6 偏移、vs login 恰差 5） |

数据无需手工修复：SweepStackSlots 在每次入场前经 CommitCharacterEvent(stack-slot-resweep)
持久化执行，char 7 下次登录自动合并为 75 行 ×52、belt 3 行删除。

### 17.4 验证

go vet + inventory/legion/wireprobe 定向测试全过（PVF 审计 4 失败为既有）。发布
wireprobe-pvf.exe（03:52）。**用户实测：待机区进入成功，不再闪退、不再回城镇**（本节
根因确认闭环）。待机区内长时间停留稳定性与开作战流程待用户后续实机验收。

## 18. 第十七轮（2026-10-03 05:16）：781 0xff 理论的提出与证伪

### 18.1 现象

§17 修复后待机区可进入，但「创建队伍」面板显示 **每周参赛人数 2/3、每周奖励计数
0/1**，四个子地下城各 0/1，建队被客户端本地拦截（无 CMD2043 上行）。

### 18.2 排查过程（本轮全部证伪，记录以避免重走）

- **N706 周本表尾部五字节**：s1 新鲜态 `8c/03/03/96/01` vs switch 耗尽态
  `9c/00/00/00/00`（_diff706.py，仅差 5 字节）；私服已发 s1 新鲜版、0x2de=00 —— 排除。
- **N781 0xff 门控理论**：「0xff = 0x68 + 完成本周场数」假说。私服抓包 041758 三版演化
  idx1199=0x6a → idx1748=0x68 → idx2281=0x69（_check_state.py），三版实测**全部被拦截**；
  官服侧 s1 城镇版在 0x61/0x34 间浮动、s4 战斗中变 0xe7（_evolve.py 解析 _dump781_out.txt
  16 帧头 31 行结构）——0xff 是会话内浮动值而非周状态门控，理论**证伪**。05:16 发布的
  0xff=0x69 版本无效。
- **N2255/N2256（STOLEN_LAND_ISPINS_INFO/OPERATION）**：switch 待机连接中同样不出现
  （_switch2255.py），与私服一致 —— 排除。
- 637/537/1336 应答、781 记录数组、706 全表：私服与官服 s4 逐字节一致 —— 排除。

### 18.3 方法论收获

官服对照样本要选对状态：s4（10-02 全程作战、可建队）vs switch（10-03 本周打满、被拦）
是唯一一对「可/不可」差分样本；s1 是城镇会话，与待机阶段不可直接比。

## 19. 第十八轮（2026-10-03）：真根因 = 待机入场 N2254 五旗标是周状态载体

### 19.1 根因

§17.2 按 switch 抓包（帧 331/997）复刻的「run 模板 + 四 stage 旗标全开 + 21/117=0x7f」
待机体，实际是官服**本周已打满**状态的形态。而官服 s4 会话（10-02，唯一待机后成功建队
并全程作战的样本）的待机 announce **帧 257** 是**五个 head u16（偏移 0/2/4/6/8）全零 +
21/117=0x7f**——与 login 帧同形。五个 u16 是本周 stage 完成标记：全开 = 客户端面板显示
「参赛 2/3、奖励 0/1」并本地拦截建队；全零 = 新鲜周状态，可建队。

排除法收敛证据：私服待机包集合与官服 s4 在 706（新鲜尾）、781（记录数组+0xff=0x69）、
637/537/1336 上全部逐字节一致，**唯一**周状态差异即 N2254 五旗标。

§17.2「login 模板不能用于待机入场」的判定是 findItemSlot 崩溃时代的误判（当时待机入场
因物品堆叠 bug 必崩，与 N2254 形态无关）。

### 19.2 修复

| 文件 | 内容 |
| --- | --- |
| internal/legion/ispins.go | `IspinsStandbyEntryCharacterInfo`：改用 login 形态——`IspinsEntryCharacterInfo(false, [4]bool{}, nonce)` 后 `copy(p[:10], zeros)`（run 基底自带 offset 0 入场标记，必须显式清零）+ 21/117=0x7f；0..255 与 login 基底完全一致 |
| internal/legion/ispins_test.go | `TestIspinsStandbyEntryCharacterInfo`：断言五旗标全零、vs run 基底恰差 [0 21 117]、vs login 基底 0..255 零差异 |
| cmd/wireprobe/main.go | 待机入场触点过时注释更新（§17 结论已被本轮推翻） |

### 19.3 构建管道修复（附带）

Build-Server.ps1 `go test ./...` 失败：`runtime/update-backup/` 下 26 个历史备份含旧
模块拷贝（无独立 go.mod）被主模块扫描。修复：每个备份根写 stub go.mod（module backup），
Go 将其视为独立模块排除；备份内容不动。

### 19.4 验证

`go test ./internal/legion/ -count=1` 全过；全量 Build-Server.ps1（test+vet+build）
通过后发布 wireprobe-pvf.exe。待用户实测：待机区面板应显示可建队的新鲜周状态（不再
「奖励 0/1」拦截）。**用户实测（05:54 会话）：面板变为「参赛 3/3、奖励 1/1」，计数
问题闭环。**

## 20. 第十九轮（2026-10-03 06:03）：待机区久置断线 + 军团选角门禁回归

用户实测报告两个新问题（§19 修复生效后）：

### 20.1 问题① 待机区久置断线

**证据**（052757 run peer 49475，21:31:05–21:45:31，14.5 分钟）：

- 21:31:05 进待机区，客户端每秒 ~5 次 op=2127 PROCESS_SCAN 心跳
- **21:31:20 心跳戛然而止**（服务端从未应答 2127，与官服一致——官服也不应答，但官服
  客户端全程不停发）
- 14 分钟无任何 s2c 下行后，21:45:27 客户端发 623+35，21:45:31 爆发清理帧序列
  （495/1566/744/194/1402...）+ op3 EXIT——弹「请检查网络连接」断线

**官服对照**（switch 抓包 f645/f697 + s4 帧 435-835）：官服每 **30 秒**发一条 **s2c
NOTI1719 SEC_PING_CHECK**（16B 常量体 `00000000e16ba3763f00000000000000`），客户端回
CMD1706（全零，无需应答）。待机区单人闲置时 1719 几乎是唯一下行流量；我们从未周期发送
→ 客户端超时。

**修复**（main.go 选角完成锚点）：启动 30s ticker goroutine 发 NOTI1719（16B 官服常量
体），发送失败（连接断）即退出；事件 `sec_ping_check_sent` 便于观测。1706 不在
`relevantRequest` 白名单，走 BodySampleLimit 采样，无副作用。

### 20.2 问题② 军团频道选角门禁回归

**现象**：城镇 → 频道选择 → 军团频道 → 选角界面选 115 级角色 → 弹「必须完成任务
『熄灭火焰的时间』才能进入 Ispins : 被掠夺的土地频道」→ 退回城镇（<110 级提示等级
不够，正常）。

**根因**：门禁在**选角界面**评估，此时连接刚建立、342（已完成任务列表）要等选角后才
发（switch f129，t+7.1s）；第十六轮的 342 时序修复只覆盖城镇进镇流程。官服在**登录
洪流**里、选角前发 **NOTI1792 QUEST_CLEAR_GROUP_INFO**（switch f12 / s4 帧 9，
96B，byte-identical：count=17 清任务组表）——这是选角门禁唯一的任务完成数据源，我们
从未实现。

**修复**（entry_flow.go）：`loginFloodPackets` 插入 1792（708 → **1792** → 1198 →
1336，对齐官服 switch f11→f12→f18 顺序），body 官方原文回放
（`questClearGroupInfo`）。`TestLoginEventFloodOrder` 更新断言 1792 位置与 96B
golden body。

### 20.3 发布与验证

定向测试 + go vet 过；06:02:57 构建、复制 wireprobe-pvf.exe、服务端重启监听正常。
待用户实测：① 待机区放置 >14 分钟不断线（events 里 `sec_ping_check_sent` 每 30s 一
条）；② 军团频道选角不再弹任务门禁，直接进待机区。

## 21. 第二十轮（2026-10-03 06:26）：待机区建队（CMD12 → NOTI9）

§19 修复后用户进入组队环节：点「组队」输队名点确定，**UI 毫无反应**。排查定位到
服务端对 c2s op=12（PARTY_CREATE）无应答。

### 21.1 官服抓包定位（时间戳因果对齐）

用户提示 10-02 官服抓包含成功建队数据。对 s4 会话（唯一建队并全程作战的样本）做
tshark 包重组 + c2s/s2c 时间线合并（seq 排序去重传，s2c 102674B 精确匹配锁定 stream 4）：

| 时刻 | 方向 | 帧 | 内容 |
| --- | --- | --- | --- |
| +2.4s | s2c | 257 等 | 待机入场帧组（N2254 全零旗标） |
| **+14.742s** | **c2s** | **125** | **op=12 建队请求，48B** |
| **+15.231s** | **s2c** | **354** | **id=9 应答，208B，含队名「111」** |
| +20.5s | s2c | 374 起 | N2254 旗标 1 + id=9 更新 + CMD2043 开战应答 |

应答就是**单独一帧 NOTI9**（0.5s 内唯一相关下行），客户端收到后直接进已建队状态，
随后 CMD2043 开战——无其它握手帧。排除了 announce 期 op=13 招募板帧（95-103）与
入场期 op=14（帧 284）。

私服客户端实测（05:54 run events.jsonl 两次 op=12，21:57:10 / 21:58:32）所发请求
与官服帧 125 **逐字节一致**（48B；此前「16B」是 plain_hex 显示截断的误判）。

### 21.2 字节契约

- 请求 48B：`[u16 0][u32 名长=3][队名][u32 容量=4][5零][u8 类型=0x0b][u16 模式=1][1,1,2,4][7×4][ff×4][零尾]`
- 应答 208B：黑鸦 NOTI9 同语法（p[0:2]=1、p[2:4]=party_id=9999、名长@14、队名@18，
  **其后字段名字相对偏移**，black_purgatory.go RE 注释为证）+ 伊斯语义（容量 4、
  类型 0x0b、模式 1）+ 尾部 80B 成员描述组；`01 01 02 04 07 07 07 07` attr 块三处
  出现位置与黑鸦一致。

### 21.3 实现（官服模板 verbatim + 队名拼接）

与 637/781/782 verbatim 应答先例同思路（属应答帧，不违反入场序重放护栏）：

| 文件 | 内容 |
| --- | --- |
| `internal/game/protocol/ispins_party.go`（新） | `ispinsStandbyPartyReplyTemplate`（官服 s4 帧 354，208B）+ `DecodeIspinsStandbyParty`（名长/容量4/类型0x0b/模式1 校验，拒截断/容量1/类型7/名含 NUL）+ `IspinsStandbyPartyReply`（头 14B + 本连接队名 + 尾段拼接） |
| `cmd/wireprobe/ispins_flow.go` | `ispinsStandbyPartyHandle`：Type 81 + 已选角 + CMD12 才接手；成功置 `soloPartyReady`（同黑鸦：选图 Party==1 归一化 65535） |
| `cmd/wireprobe/main.go` | moonHandle 分支后接线；拒绝路径回 `protocol.Refusal(8)` 并记 `ispins_party_request_rejected` |
| `cmd/wireprobe/ispins_party_test.go`（新） | 4 测试：队名「111」时拼接结果 == 官服帧 354 golden；换名头尾恒等；官服请求 48B golden 解码；门禁（城镇频道/CMD36 不接手） |

拒绝路径备选方案（黑鸦式 110B 自有语法，moon_solo_party.go q[69]/mode 双写位歧义）
被否——官服 208B 是此客户端此流程的实证有效字节串。

### 21.4 发布

go vet 干净、4 个新测试过、全包回归仅 3 个既有 PVF 审计失败（与本无关）。
06:26 停旧服（PID 29284）→ 复制 wireprobe-pvf.exe → 重启（06:27:59 run 目录）。

待用户实测：待机区点「组队」→ 输队名 → 确定，应出现队伍窗口（events 里
`{"kind": "伊斯待机区队伍创建", "id": 9, ...}`）。若队伍成员列表显示异常，迭代方向：
官服 208B 尾部 80B 成员描述组含官服会话角色数据（如职业 0xed），考虑替换为本角色数据。

### 21.5 首测闪退与修复（2026-10-03 06:36 发布）

**首测结果（06:31 会话）**：CMD12 应答已送达（events 有「伊斯待机区队伍创建」），
但客户端 **1.2 秒后闪退**——发 op=682 退出信号（官服全部抓包中 op=12 后从无 682）。
client-direct.out CrashDump：`0xc0000005` 空指针违例 @0x0000000000000000。

**根因（全量明文对比官服 15 个 NOTI9 帧后定位）**：官服建队应答帧 354（224B 帧
= 208B body）比稳态更新帧 375+（192B 帧）多出的正是 0x84 起的 32B **成员块**，
其中 0x86 的 u16 = 创建者 actor。官服帧内嵌 `ed 00`=237——这是官服会话自身的
路由前缀（s4 全部 NOTI22/546/23 帧都以 `ed 00` 打头，yan55 本人）。verbatim 回放
该值 → 私服客户端在本地 actor 表查 237 无此人 → 空指针闪退。

**修复**：`IspinsStandbyPartyReply` 增加 actor 参数，把本地 `w.role.WireID` 写入
成员块（名字相对 q[113]，名长 3 时绝对 0x86）。黑鸦先例同构：BlackPurgatorySoloParty
的 NOTI9 也显式传本地 actor（q[71]），从来不做 verbatim。成员块其余字段
（名望 75 e5、等级/职业展示 9c 00/15 01 等）暂保持官服原文，若队伍窗口显示
异常再逐项本地化。

**验证**：5 个测试过（新增 TestIspinsStandbyPartyReplySplicesLocalActor 护栏：
actor 7 写入 0x86、换名平移、拒 0/65535）；vet 干净；06:36:54 发布、06:37:02
服务端重启。待用户复测。

### 21.6 二测仍闪退（先卡死很久）→ §21.5 根因证伪，真根因 = 布局不兼容（2026-10-03 06:49 发布）

**二测结果（06:40 会话）**：actor 修复后仍闪退，且节奏变为**先卡死很久才崩**——
events 时间线 op=12 → NOTI9 应答（成员块已带本地 actor 7：`01 00 07 00 …`）→
1.2s 后 op=682 退出。无新 CrashDump，但卡死现象本身是新证据。

**§21.5「actor 0xed 查无此人」单一理论证伪**：actor 已正确本地化仍崩，说明
成员块内容不是（唯一）问题所在。

**真根因：官服 208B 帧布局是官服新版客户端的 NOTI9 布局，2.38.2 读取器不兼容**。
证据链：

1. 黑鸦/月环族在本客户端实证可解析的同族 NOTI9（1452F2620 写入器）都是 110B
   级布局，其 **p[4:6] 是成员数槽位（恒为 1）**；
2. 官服 208B 帧同一位置的值是 `69 00` = **105** —— 2.38.2 读取器把它当成员数，
   按 105 个成员解析 208B body，必然越界；越界读出垃圾长度 →
   `ReadLengthPrefixedBlob`（0x146D77F50 → memcpy 0x146EA0BE0）巨量拷贝——
   这正是用户看到的「先卡死很久」；拷贝耗尽后空指针崩溃（CrashDump 0xc0000005
   @0x0）→ 客户端发 op=682 退出（官服 op=12 后从无 682）；
3. 两次崩溃节奏与 actor 值无关（06:32 verbatim actor=237、06:41 本地 actor=7，
   同为收帧 ~1.2s 后 682）；
4. 第一次崩溃调用栈顶 0x146ea0c30 紧邻 146D77F50 的 memcpy 目标
   （0x146EA0BE0 一带），与「巨量拷贝」吻合。

多数官服包在 2.38.2 上 verbatim 可用（637/781/782/N2254/706/1792 均已实证），
但队伍 NOTI9 是例外——新版客户端改了布局。

**修复（黑鸦族原生语法承载伊斯语义）**：

- `protocol/ispins_party.go`：删除官服 208B verbatim 模板；`IspinsStandbyPartyReply`
  重写为 2.38.2 原生黑鸦族布局（110B = 107+名长）：p[0:2]=1、p[2:4]=9999、
  **p[4:6]=1（成员数槽，官服帧在此放 105 即崩）**、p[6:8]=channel、名长@14、名@18、
  q=p[len(name):] 名字相对；q[20]=4（容量）、q[25]=5（难度，同黑鸦发送器）、
  q[29]/q[95]=0x0b（军团普通队伍，双写）、u16 q[30]=1（模式）、q[46:54]=attr
  `01 01 02 04 07 07 07 07`、q[69]=1、u16 q[71]=本地 actor、尾段扩展类型 0；
- `cmd/wireprobe/ispins_flow.go`：handler 照黑鸦建队先例（black_purgatory_flow.go
  100-119）在 NOTI9 前先发队长资料两个 op=2 包（EntryBasicProbe/EntryAddition，
  成员列表显示数据源），并补 `w.characters` 守卫；
- `cmd/wireprobe/ispins_party_test.go`：官服 golden 用例删除（布局已证伪），
  新增 `TestIspinsStandbyPartyReplyNativeGrammar`（成员数槽=1、伊斯语义字段、
  拒绝路径）+ gate 测试成功路径（3 包、110B、soloPartyReady）；官服帧 hex
  保留仅作语义参照。

**验证**：3 个测试过；vet 干净；06:49 发布、服务端重启（PID 41364）。待用户三测。

### 21.7 三测通过：建队+进图成功，转入结算环节（2026-10-03 07:00 用户实测）

三测实证：待机区建队 → 输队名确定 → 队伍 UI 正常 → CMD2043 开战 → CMD2047 选作战 →
CMD2045 进图（背叛者宅邸 100006476）→ boss 战可打。黑鸦族原生 NOTI9 语法方案闭环，
§21.6 根因结论确认。剩余问题：boss 死亡无结算 + 撤退死按钮 → 见 §22。

## 22 阶段结算：ArenaBoss 房间归属 + CMD72 三发送器（2026-10-03 07:14 发布）

**实测取证（run 065220_293353）**：完整流程事件齐全（ispins_started → operation →
stage_entered monsters:1/maze:0/map:100006476/dungeon:100002987 → monster_death_confirmed
→ omen_clear），但**没有** dungeon_clear_enabled —— activeDungeon 永不 Completed()。
两个拒绝事件给出直接证据：

1. `dungeon_request_refused` id=117 reason="boss check target is not a source boss
   in this room"（22:55:22.81，op=117 body `07 00 00 10 ...` actor=7 target=4096）
2. `dungeon_request_refused` id=72 reason="unsupported exit source"（5 次，op=72 body
   `01 02 00 c52024763f 00...` —— 撤退按钮发送器）

**根因 1（结算缺失）**：源迷宫 100002987/maze0 的 boss 坐标在 (0,0)/100006472，
Start 在 (1,1)/100006476；官服 s4 帧 451 实证也是进 (1,1) 开打（N28 帧 449 仍回
Boss=(0,0)），即**军团阶段本的 boss 战就发生在进图房间，源 [boss map] 是未使用元数据**。
而 BossCheck 的守卫 `s.Room.Boss && position == s.Maze.Boss` 两边全假 → 拒绝 →
completionTarget 恒 0 → 无任何结算路径。官服对照：s4 的 CMD117（帧 337 target=255）
被官服受理，N115（帧 495）回显该 target；且 s4 四个阶段只有前两个发 CMD117，
后两个无 CMD117 也照常结算（4 条 N31）⇒ 官服对进图房间有死亡驱动的结算兜底。

**修复（Session.ArenaBoss 机制）**：
- `internal/dungeon/session.go`：Session 加 `ArenaBoss bool`（只放宽房间归属守卫，
  目标仍须为房内真实源领主 rank3/APC team≠0）；
- `internal/dungeon/completion.go` 三处：BossCheck 主循环 bossRoom 置真、
  [MERGE-20260927-BOSS-ID-SKEW] 错位兜底守卫放行、tryComplete 加死亡驱动结算分支
  （`ArenaBoss && Loaded && roomEnemiesDead() && reportableDisplayBoss()!=0`，覆盖
  无 CMD117 的阶段）；
- `cmd/wireprobe/ispins_flow.go` enterIspinsStage 置 `s.ArenaBoss = true`；
  completeIspinsStage 链尾按官服帧 495 补 N115（boss_check_confirmed，回显
  CompletionTarget，位置在 N31 家族之后、客户端 CMD2046 之前）。

**根因 2（撤退死按钮）**：CMD72 有三种发送器，第三个字节（source）不同：
- `01 02 01`：结算面板（边界之调律 RE 先例，原解码器唯一接受的形态）；
- `01 02 00`：副本内撤退对话框（2.38.2 实测 5 次，被拒 → 撤退死按钮）；
- `01 01 02`：官服伊斯客户端奖励领取后的退出（s4 帧 498，紧跟 CMD2046 帧 496）。
且官服 s4 帧 342/402/498 与私服实测 5 帧的 16B 体在 p[3:8] 逐字节一致携带常量
token `c5 20 24 76 3f`（跨新旧客户端相同 ⇒ 客户端常量或对某服务端帧的回执），
原「padding 全零」检查同样会拒。

**修复**：
- `internal/game/protocol/cards.go` DecodeSettlementExit：source ∈ {0,1,2}；
  p[3:8] 允许已知常量 token（p[8:] 仍须全零）；
- `cmd/wireprobe/card_flow.go` settlementExit：伊斯分支（w.ispins != nil &&
  activeDungeon != nil）绕过通用翻牌/结算（官服 s4 整场无 69/70/71，奖励由
  N2256/N2252 承载），复用 leaveDungeon 回进本前位置（待机区），只发 ACK72
  （主循环按 settlement_exit_ack 清 activeDungeon）；撤退不算通关，run.cleared
  不动，重新 CMD2043 即整场重开（与官服「撤退作废本次作战」语义一致）。

**测试**：`TestIspinsArenaBossCompletion`（dungeon 包：未置位必拒的回归护栏、
CMD117 驱动、死亡驱动、假目标拒绝）+ `TestIspinsSettlementExitSources`
（protocol 包：三种 source + token 常量 + padding 拒绝）；既有 SettlementExit/
SourceBoss/Odyssey/Dungeon22 套件全过；vet 干净；07:13 发布、服务端重启。

**待用户四测**：①杀 boss 看结算面板（预期 N31 链 + N115，事件依次
dungeon_clear_enabled/ispins_*）；②结算后领奖退场（CMD2046 → CMD72）应回待机区；
③撤退按钮应直接退回待机区。已知风险：结算后客户端若发 op=46 通用结算请求，
会走 generic dungeonResult（经验保存+N26），与官服无 N26 有偏差，实测观察。

## §23 四测闪退：伊斯上下文的 op=46 走 generic 结算族（2026-10-03 07:35 发布）

四测结果（run 20261003_071548_040041，UTC 23:18:33-34）：ArenaBoss + CMD72 修复
全部生效——boss 死亡后死亡驱动结算 fired，伊斯链完整发出
（N31→2256→2252→2255→2253→2254→N115 `01010010` 回显 target 4096）。但客户端在
死亡瞬间闪退：

1. 23:18:33.65 服务端发出死亡批次（39/38/37 ack + 伊斯全链）；
2. 客户端正常回包：op=585、op=117（`07 00 00 10` actor=7 target=4096）、op=283、
   **op=46（141B 通用结算请求）**；
3. 23:18:33.678 服务端按 generic dungeonResult 应答 op=46，发出
   **N34 dungeon_play_result、N37 dungeon_clear_experience、N26/N35
   dungeon_clear_reward、N261 eplp_rechallenge(09)、N19 skill_state_restored、
   N2758 skill_preset_restored、N29 skill_variation_response、N21
   clear_available_quests**；
4. 23:18:34.865 客户端 op=682 崩溃退出（收包 1.2 秒后）。

**官服对照（推翻 §22 的预判）**：s4 c2s 明文存档里**伊斯每阶段都有 op=46**
（帧 338/393/442/489，141B，与私服客户端逐帧同形），紧跟在 op=39 死亡报告 +
op=585/op=117 之后——「官服伊斯无 CMD46」是错的。但官服 s2c 全流**没有 kind=1
id=46 应答**，结算区间（s2c 468-498：38→2204→2201→279→N31→…→N115→ACK2046→
ACK72）里 N34/N37/N26/N261/N19/N2758/N29/N21 **一个都没有**。即官服对伊斯阶段的
op=46 完全静默，结算链在死亡时刻已由服务端推出（s2c 468 死亡确认后直接跟 N31），
不等待 op=46。op=585/283/1654 官服同样无应答，私服无 handler 恰好一致。

**根因**：generic dungeonResult 的 8 个通用结算包在伊斯上下文属于「官服从不发
的帧」——与 §21.6 NOTI9 布局异常同一教训：客户端在军团场景的状态机没有为这些
包准备解析路径，收包即崩。

**修复**：`cmd/wireprobe/settlement_flow.go` dungeonResult 入口整包吞掉——
`w.ispins != nil && w.activeDungeon != nil` 时直接 `return nil, nil`（静默无应答，
与官服逐帧一致）；非伊斯会话的 generic 路径不变（普通副本结算仍走 N34/N261 等）。
守卫放在 dungeonResult 而非 main.go 分发处，覆盖所有调用点且可单测。

**测试**：`TestIspinsSwallowsGenericPlayResult`（settlement_retry_test.go：伊斯会话
吞包静默、非伊斯会话保持 generic 拒绝/路径）；既有
TestIspinsArenaBossCompletion/TestIspinsSettlementExitSources/TestSettlementRetry
全过；vet 干净；07:35 发布（停服→Copy-Item→launch_local --server-only+4 环境变量）。

**待用户五测**：杀 boss → 结算面板出现不闪退 → CMD2046 领奖 → CMD72 退场回待机区
→ 下一阶段；撤退按钮同样验证。

## §24 五测闪退：死亡批次 + 结算链布局全量修正（2026-10-03 07:46 发布）

五测结果（run 20261003_072701_155719，UTC 23:29:33-34）：op=46 拦截**已生效**
（客户端发 585/117/283/46 后服务端零应答），但**仍崩**——23:29:34.391 op=682 退出。
§23 的「generic 8 包是崩溃源」结论被推翻：op=46 只是必要非充分，真正的崩溃源在
死亡批次与结算链本身的布局偏差。

### 24.1 官服 s4 stage0 结算链全量（帧 468-496）

```
468  N38   16B  ff000000 00000000 5849ff0b3e 000000      ← boss 死亡确认
469  N2204 32B  01000000 90000000 cd020000 01000000 ...  ← 成就
470  N2201 32B  90000000 00000000 01000000 cd020000 ...  ← 成就
471  N279  16B  e8030000 d52fe4f4 33000000 00000000      ← 成就
472  N31   16B  938c0000 9fcb4206 45000000 00000000      ← 阶段 token
473  N2256 16B  0b0001d6 f7020140 00000000 00000000      ← 作战通知
474-477 N2168×4 48B                                       ← 物品台账
478  N14   192B                                           ← 物品更新
479  N2252 64B  789cedce...                                ← 基础奖励(zlib)
480-482 N2168×3 48B
483-487 N14×5  192B
488  N2    480B 00010003 56000000 04000000 ... yan55...   ← actor 信息
489  N2253 64B  789c63dc...                                ← 追加奖励(zlib)
490  N2254 272B 01000100... idx21/53/85=7f idx117=39     ← 入场信息(旗标[1,1,0,0,0])
491  N9    176B 01000f27 6900...                          ← 队伍更新
492  N2255 88B  0000ff02 00000003 ...                     ← 阶段信息
493  N1658 EMPTY                                           ← 请求结算信息
494  N2168 48B
495  N115  16B  0101ff00 aa53062f42 00000000 00000000     ← boss check 确认
496  ACK2046 32B                                           ← 领奖应答
```

### 24.2 私服五测死亡批次 vs 官服差异

| 帧 | 官服 s4 | 私服五测 | 性质 |
|---|---|---|---|
| 39-ack | **不发** | `01`(1B) | 多余 |
| N38 | 16B `<u32 entity> 00000000 <5B token> 000` | ~200B 掉落实体语法 | **布局错** |
| N37 | **kind=0 全流为零**（只有 ACK@458） | 32B 经验包主动推送 | 多余 |
| N31/N2256/N2252 | 逐字节同 | 逐字节同 | ✓ |
| N2255 位置 | 2254/N9 之后 | 2252 后立即发 | 顺序错 |
| N2254 idx21/53/85 | 0x7f | 0x00 | 值错 |
| N2254 idx117 | 0x39 | 0x7f（模板自带，错） | 值错 |
| N115 | 16B `01 01 <u16> <5B token> 0000...` | **4B** `01010010` | **布局错** |

缺失帧（2204/2201/279/2168×8/14×6/N2/N9）私服未实现，整组跳过——这些是成就/物品/
队伍台账，官服新客户端有但 2.38.2 客户端不一定要求，先不补。

### 24.3 修复（四项）

1. **死亡批次 ispins 分支**（dungeon_flow.go monsterDeath）：`w.ispins != nil &&
   w.activeDungeon != nil` 时跳过 39-ack、N37、generic N38，只发 16B 官服形 N38
   （`IspinsMonsterDeathConfirmed(r.Entity)` = `<u32 entity> <4B零> <5B token
   5849ff0b3e> <3B零>`），然后走 completeDungeon()。

2. **结算链重排**（ispins_flow.go completeIspinsStage）：从
   `31→2256→2252→2255→1658→2253→2254→115` 改为
   `31→2256→2252→2253→2254→2255→1658→115`，对齐官服帧序（2253/2254 在 2255 之前）。

3. **N2254 阶段标记位**（legion/ispins.go IspinsEntryCharacterInfo）：有阶段已结算
   时覆盖 `p[21]=p[53]=p[85]=0x7f, p[117]=0x39`（官服帧 490 形态）；基线（无阶段
   结算）与待机/登录帧保持原模板行为（已实机验证可建队可进图，不动）。

4. **N115 16B 官服形**（protocol/boss_check.go 新增 IspinsBossCheckConfirmed）：
   `01 01 <u16 target> <5B token aa53062f42> <7B零>`。BossCheckConfirmed（4B）
   保持不变供普通副本/流血矿使用，避免回归。

### 24.4 N38/N115 的 5B token

官服四阶段 N38 token 各不相同（stage0=5849ff0b3e、stage1=b44eb20244、
stage2=1214196242、stage3=b15bb2b139），N115 token 也不同且与同阶段 N38 不同
（stage0=aa53062f42、stage3=ed9b7cfd35）。形态像 per-stage nonce，语义未破译。
本轮用 stage0 官方值回放（崩溃发生在 stage0）；若玩家推进到 stage1+ 再闪退，按
同法补各阶段 token 表。

### 24.5 发布

定向测试（Ispins/BossCheck/MonsterDeath/Settlement）全过；vet 干净；
07:45:37 构建 wireprobe-handoff-source.exe → Copy-Item wireprobe-pvf.exe →
launch_local --server-only + 4 环境变量（DFO_SHOP_OPEN_ALL/DFO_MAX_ITEM_PERIOD/
DFO_QUEST_VISIBLE_NPC_RELAX/DFO_QUEST_NPC_DISTANCE_MULTIPLIER）。

**待用户六测**：杀 stage0 boss → 结算面板出现不闪退 → CMD2046 → CMD72 退场 →
stage1；若 stage1 闪退，查 N38/N115 token 是否需要 stage1 值。

## 25. 第二十四轮后续（2026-10-03 08:00）：六测仍闪退 → 根因 = N1658 空包被 preparePackets 静默丢弃

六测（07:48 run，§24 发布后）结果：op=46 已静默、N38 常量体/链序/N115 16B 全部
按 §24 生效，但客户端收链后仍只发 585/117/46，1.2s 后 op=682 闪退，始终没发 CMD2046。

### 25.1 定位

events 里结算链为 31→2256→2252→2253→2254→2255→115——**缺 1658**。源码里
completeIspinsStage 明明有 `{"ispins_req_dungeon_clear_info", 0, 1658, []byte{}}`。
追发送路径：dungeon 请求分支（main.go ~4353）先 `preparePackets` 再 `writePrepared`，
而 **preparePackets（entry_flow.go）对 `len(p.Payload)==0` 一律 `continue`**——空包
被静默丢弃，连事件都不记（事件从 writePrepared 回调记）。exe 时间戳（07:45:37）
晚于源码改动（07:43:08），排除部署过期。

官服对照（s4 帧 468-498 全链明文）：**帧 493 = N1658，size=16、body=0、cs=EMPTY**
——官服真的发 16B 纯头空包（四个阶段各一条：493/588/671/780），位置在 N2255 之后、
N2168/N115 之前。原始字节 `00 7a 06 10 00 00 00 18 00 00 00 18 00 00 00 00`，
checksum 位 [11]=0x18 = 我们 `Checksum(空)` 的精确值。

附带核实（排除两个假偏差）：① N115 target：官服帧 495 = `ff 00`（官服 boss 实体
255），我们 = `00 10`（本会话 boss 实体 4096，客户端 CMD39/117 均上报 0x1000）——
各自回显会话实体，正确；② 官服 CMD117 `ed 00 ff 00…` vs 我们 `07 00 10 00…`
= actor 前缀（237 vs 7）+ boss 实体，一致。

### 25.2 修复

| 文件 | 内容 |
| --- | --- |
| entry_flow.go preparePackets | 跳过条件从 `len==0` 改为 `p.Payload == nil`：nil = 占位符（保持旧跳过语义），非 nil 空 slice = 真空包，放行产出 16B 纯头帧 |
| ispins_flow.go | 1658 包保持 `[]byte{}`（非 nil 空）不变，语义由 preparePackets 新规则激活 |
| account_vault_flow.go:166 | 唯一另一处 `[]byte{}` 字面量（ACK20 占位）改为 nil，保持旧跳过行为不变 |
| packet_plan_test.go | 新增 TestPreparePacketsEmptyBodyDelivery：`[]byte{}` 产出 16B 帧、id 字节 7a 06、checksum 0x18、nil 仍被跳过 |

### 25.3 风险审计

全仓 outboundPacket 空体字面量仅 account_vault 一处（已改 nil 保持行为）；
entryPayloads.InformNotice 系 `make([]byte, 1+n)` 恒 ≥1B 不受影响。定向测试
（PreparePackets/SendPacketPlan/Ispins/AccountVault/Vault）全过；全包测试仅
3 个既有 PVF 审计失败（Adventure/PVFCatalogGate/Enhancement，与本无关）；
vet 干净。

### 25.4 发布

08:00:44 构建 wireprobe-handoff-source.exe → Copy-Item wireprobe-pvf.exe
（服务端进程已退出，无文件锁）。

**待用户七测**：杀 stage0 boss，事件里应出现
`{"kind":"ispins_req_dungeon_clear_info","id":1658,"plain_hex":""}`，链完整后
客户端应发 CMD2046 而非 op=682。若仍闪退，下一偏差候选：官服链中我们仍未发的
N2204/N2201/N279（死亡批次后、N31 前）与 N2168×N/N14×5/N2/N9（奖励台账/队伍
更新，§24 有意跳过），以及 stage1+ 的 per-stage N38/N115 token。

## 26. 第二十五轮后续（2026-10-03 08:19）：七测仍闪退 → 根因 = 结算链辅助包整组缺失，全链对齐官服

### 26.1 七测定位（08:01 run）

§25 修复生效：events 里 1658 已发出（16B 纯头）。但仍闪退——客户端
39→585→117→46 后 1.2s op=682，未发 CMD1654/CMD2046。逐字节对照官服 s4
stage0 全链（帧 468-496）：我们发的主链包（N38/N31/N2256/N2252/N2253/N2254/
N2255/N1658）与官服全部一致（2252/2253 解压后 byte-equal），**剩余偏差 =
官服链里整组未实现的辅助包**。

### 26.2 官服链精读结论（s4 帧 468-496 + 四阶段全量）

- 官服链：N38→N2204→N2201→N279→N31→N2256→N2168×4→N14→N2252→N2168×3→
  N14×5→N2→N2253→N2254→N9→N2255→N1658(空)→N2168→N115→ACK2046→N292→ACK72→…
- 辅助包 shape：N2204=32B、N2201=32B、N279=16B、N2168=48B、N14=192B
  （`space+count+189B 行`，count 为官服会话累计值）
- N2256 官服只在 stage 0/1/3 出现，**stage2 无**（帧 656→657 直接 2168）
- CMD117/N115 只在 stage0/3（c2s 帧 337/488）；token per-stage：
  stage0=aa53062f42、stage3=ed9b7cfd35；stage1/2 不发
- N38 尾 5B token per-stage：5849ff0b3e / b44eb20244 / 1214196242 / b15bb2b139
- **N2（内嵌官服角色名 yan55）与 N9（队伍名 111、actor ed）刻意不发**：
  verbatim 回放会在本地 actor 表查无此人（§21.5 闪退先例），本地构造留待下轮
- 52 个辅助包 body 已批量导出存档 `analysis/tasks/next79-aux-bodies.json`
  （长度全部校验通过）

### 26.3 修复（全链 verbatim 对齐）

- `cmd/wireprobe/ispins_flow.go`：新增 `ispinsAuxPacket` 类型 +
  `decodeHexOrDie` + `appendIspinsAux` + 四张 per-stage 辅助表
  （pre31/pre2252/pre2253/pre115，52 个官服 body verbatim 回放）；
  `completeIspinsStage` 链重排为：
  pre31 → N31 → N2256(stage≠2) → pre2252 → N2252 → pre2253 → N2253 →
  N2254 → N2255 → N1658(空) → pre115 → N115(仅 stage0/3)
- `protocol/dungeon.go`：`IspinsMonsterDeathConfirmed` 加 stage 参数 +
  `ispinsDeathTokens` 四阶段表
- `protocol/boss_check.go`：`IspinsBossCheckConfirmed` 加 stage 参数；
  stage1/2 返回 (nil, nil) 不发
- `cmd/wireprobe/dungeon_flow.go`：ispins 死亡分支传 `w.ispins.stage`
- 新增 `cmd/wireprobe/ispins_settlement_test.go`：表 shape 与 stage0 帧数
  组合（pre31=3、pre2252=2168×4+14×1、pre2253=2168×3+14×5、pre115=1×2168）、
  per-stage N38 token、N115 stage 门（stage1/2 nil、stage0/3 token 各异）
- 踩坑：第一次手粘生成的表 N14 hex 全部损坏（奇数长度），包 init 时
  `decodeHexOrDie` panic 立刻抓获；改为 `regen_aux_tables.py` 从
  next79-aux-bodies.json 程序化直写 ispins_flow.go（带逐项长度断言）
- N14 奖励物品仅 verbatim 回放，不入库（v1 限制，记后续事项）

### 26.4 发布

`go test ./cmd/wireprobe -run TestIspins` ok；`go vet` 干净；protocol /
legion 包测试 ok。08:19:44 构建 wireprobe-handoff-source.exe → Copy-Item
wireprobe-pvf.exe（服务端未运行，无文件锁）。

**待用户八测**：杀 stage0 boss，events 应依次出现
`ispins_legion_field_object`/`ispins_star_cluster`/`ispins_lag_statistics`/
`ispins_training_counter`/`ispins_reward_item_granted`，客户端应发
CMD1654→CMD2046（结算面板→翻牌→退出）而非 op=682。若仍闪退，下轮候选 =
N2/N9 本地 actor 构造（§21.5/§21.6 教训：需按黑鸦族原生语法本地拼，禁 verbatim）。

## 27. 第二十六轮后续（2026-10-03 08:46）：八测仍闪退 → 根因 = 链尾段缺 N2/N9，本地构造补齐

### 27.1 八测定位（08:21 run）

§26 修复生效：整链 26 包 3ms 内发完（38→2204→2201→279→31→2256→2168×4→14→
2252→2168×3→14×5→2253→2254→2255→1658→2168→115），客户端随后 585→117→**2060**→46
（14ms 后），1.2s 后 op=682。**CMD1654（进结算面板）从未发出**。

官服时间轴定量分析（map_official_settlement_times.py，把 s4 TCP 流字节偏移
映射到 tshark 包时间戳，相对 c2s op=39 死亡上报）：
- 官服 N38+链前段 +297ms 到 → 客户端 +310ms 发 585/117/46 突发（反应式）
- N1658 +595ms；**链尾段（N2 起，TCP 重传补洞）+1103ms 到**
- **1654 +1113ms（收到链尾段后 10ms）**；N115 +1401ms；2046 +12699ms

⇒ 客户端进结算面板由**链尾段**触发。官服链尾 = N2(488,480B)→N2253→N2254→
N9(491,176B)→N2255→N1658→N2168→N115。我们链里缺 **N2 与 N9** 两帧——客户端等
不到 N2，改发官服从未出现过的 op=2060（疑似查询缺失信息），无人应答后 682 退出。

### 27.2 官服帧结构精读（为什么不能 verbatim）

- **N2 480B（帧 488）**：op=2 mode0 count1，行上下文 `{03,56}`（进图版是
  {01,56}）。两处内嵌官服角色名 yan55（名长 u32@28/名@32；第二块 actor u16@165/
  名长@167/名@171），第二块名后紧跟迷你角色行 `10 35 73 00 01 04 …`
  （职业/转职/等级0x73=115/PvP/**状态**）。四阶段帧（488/583/666/772）除模板
  偏移 416..419（2B per-stage nonce + u16 进度 3/7/11/15=4*stage+3）外逐字节
  一致。回待机版帧 507 与 488 仅差 1 字节：迷你行状态位 01→00（结算态标志）。
  verbatim 回放内嵌 yan55 + actor 0xed → §21.5 本地查无此人闪退。
- **N9 176B（帧 491）**：官服新版布局，p[4:6]=0x69=105——正是 §21.6 证伪的
  成员数槽（2.38.2 读取器按 105 个成员越界解析→卡死+闪退），严禁 verbatim。
  各官服 N9 帧是演化帧（计数器@12/24-27/42-43、状态@153、5B token@163-167），
  黑鸦族旧语法无对应槽位，无法忠实映射。

### 27.3 修复（本地构造，非 verbatim）

- **N2**：新增 `protocol/ispins_settlement.go`（由 `analysis/tasks/
  regen_settlement_n2.py` 从官服存档 next79_settlement_tail_bodies.json /
  next79_n2_n9_all.json 程序化直写，带模板结构断言——沿用 §26 手粘损坏教训）。
  `IspinsSettlementCharacterInfo(name, actor, stage)`：官服 480B 模板 + 两处
  名字块按本地角色名重排（op=2 行读取器为游标式顺序解析，docs/protocol/
  entry-userinfo.md「307 + UTF-8 name bytes」证明行宽随名字变化，名后字段随
  长度平移）+ 第二块 actor 槽换 w.role.WireID + 每阶段 4 字节补丁（官方帧
  416..419 verbatim）。名字与官服等长（5）时布局与官服帧完全同构。
- **N9**：结算链内官服帧为新版布局不可回放；本地改发 `protocol.SoloPartyInfo(
  w.role.WireID)`——进图时客户端已接受的同型帧（八测 events 帧 591
  solo_party_initialized 99B），重申作战中队伍状态，最小状态变更。
- `cmd/wireprobe/ispins_flow.go` completeIspinsStage 插入两包：
  **N2 于 pre2253 辅助包后、N2253 前（官服帧 488 位）；N9 于 N2254 后、N2255
  前（官服帧 491 位）**。
- 新增回归测试 `TestIspinsSettlementCharacterInfoRebuild`：476B 长度、两名字块
  本地化（3 字符名整体平移 -2：actor@163/名@169/行@172）、迷你行状态 01、
  per-stage 补丁、5 字符名与官服帧同构、非法入参拒绝。
- 顺手修复 ispins_settlement_test.go 既有 `:=` 重声明编译错。

### 27.4 发布（08:46:30）

`go test ./cmd/wireprobe -run Ispins` 全绿；`go vet` 干净；protocol 包 ok。
三个 PVF 目录门禁测试失败为运行环境 PVF/JSON 快照漂移（quest7 等级、票据
过期日），与本轮无关。构建 wireprobe-handoff-source.exe → Copy-Item
wireprobe-pvf.exe（服务端未运行）。

**待用户九测**：杀 stage0 boss，events 应出现 `ispins_settlement_character_info`
（476B）与 `ispins_settlement_party_steady`（99B）两个新事件，客户端应发
CMD1654（进结算面板）→CMD2046（翻牌领奖）而非 op=2060/op=682。若仍闪退，
下轮候选：① N9 改发 IspinsStandbyPartyReply（存 CMD12 队名）替代 SoloPartyInfo；
② N2 名字改 5 字符定长填充对照官服帧逐字节排查；③ 对照官服 488 全帧与我们
N2 的逐字节 diff 复核重排假设。

## 28. 第二十七轮后续（2026-10-03 13:42）：九测仍闪退 → 根因 = 结算 N2254 旗标错位（§24 写错位置），修正为官服四阶段实测形态

### 28.1 九测定位（13:31 run）

§27 两包已送达（events 1271 N2 476B 本地名 "001" 可见、1274 N9 99B），但客户端
反应与八测完全一致：585→117→2060→46，1.37s 后 op=682；1654 仍未发。
**八/九两测 CrashDump callstack 逐字节相同**（eip=0x146ea0c30、
checksum=0x000100062c33d494、同 10 帧）——崩溃源与 N2/N9 无关（八测根本没有
这两包也一样崩），两包既未引发也未阻止崩溃。

### 28.2 整链逐字节对照（九测 28 包 vs 官服帧 488-495）

- **byte-equal**：N2255（88B）、N1658（空）、N2253（解压后 2405B）、
  N2252（§24 已验）、辅助包表（§26 verbatim 生成）
- **by-design 差异**：N2（本地名/actor 重排）、N9（黑鸦族语法替代）、
  N115（target 回显我方 boss 0x1000 vs 官服 0xff，§22 语义一致）
- **意外差异 = N2254：10 字节**——53/69/85/93/94/117 旗标 + 256-259 nonce

官服全部 7 帧 id=2254（257/374/490/585/668/777/813）旗标槽对照：
- 登录/待机 257：21/117=7f（§19 形态 ✓）
- 374（待机建队后）：全零
- **四阶段结算 490/585/668/777：形态完全一致 = 21=7f、69=7f、93=39、94=32、
  117=00，53/85=00**；仅 256-259 每帧 nonce 不同（490=6b70269c…）
- 813（终局）：仅 165=7f

⇒ §24 的「idx21/53/85=0x7f、idx117=0x39」是错位写法（把 69 的 7f 写到了 53/85，
把 93/94 的 39 32 写到了 117）。§19 已实证客户端主动读这些旗标做 UI 门禁
（全旗形态曾阻断建队），错形结算旗标同理阻断结算面板——1654 不发的最解释。

### 28.3 修复

- `internal/legion/ispins.go` IspinsEntryCharacterInfo anyCleared 覆盖块改为
  官服实测形态：`p[21], p[69] = 0x7f, 0x7f; p[93], p[94] = 0x39, 0x32`
  （117/53/85 保持基底 00）。nonce 256-259 维持 374 基底值不动（官服每帧
  各异、无跨帧校验证据，最小变更）。
- 新增 `TestIspinsEntryCharacterInfoSettlementFlags`：四种累计旗标组合 +
  未结算基线全零形态。
- 待机/登录形态不受影响（空旗标不触发覆盖块，既有测试守卫）。

### 28.4 发布（13:42:16）

legion + wireprobe Ispins 测试 ok、vet 干净。待用户十测。

**十测预期**：杀 stage0 boss → N2254 现为官服 490 逐字节同形（除 nonce）→
客户端应发 CMD1654 进结算面板 → CMD2046 翻牌。若仍闪退，剩余候选按序：
① N2254 nonce 改官服 per-stage 值（490/585/668/777 的 256-259）；
② N2 改 5 字符定长名填充逐字节对照；③ 结算链分段延迟（官服链头→客户端
burst→链尾的时序，我们 3ms 灌完与官服 +310ms/+1103ms 不同）；
④ RE op=2060（0x146ea0c30 崩溃点上游 handler）。

## 29. 2026-10-03：十测仍闪退，当前客户端 N2252 读取器拒绝压缩体

### 29.1 本轮证据与纠正

十测会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261003_134327_943975_next37`：
13:47:18 发结算链，客户端 585→117→2060→46，13:47:19.725 发 682 退出。
`client-direct.out` 明确记录 `0xc0000005`，栈顶 `146EA0C30`，上层返回地址
`1424FDD06 → 1459A3C7C`。前几轮关于辅助包、N2/N9、旗标为闪退根因的判断
均未获得实机确认；不能继续把待测假设写成已确认根因。

本轮直接读取 **D:/115us/client/DFO.exe** 的 PE 指令，与既有 IDB 分析脚本
`analysis/dumps/ida_legion_noti_consumers.py` 的 handler 地址交叉核实：

- N2252 handler `1424FDC30`：`1424FDCF7 mov edx,0x1e5c`，
  `1424FDD01 call 146EA0BE0`，返回地址恰为 CrashDump 的 `1424FDD06`。
  直接读取 **7772B**，handler 内没有 zlib 解压。
- N2253 handler `1424FDB60`：`1424FDBCB mov edx,0x965`，
  `1424FDBD5 call 146EA0BE0`，直接读取 **2405B**。
- reader `146EA0BE0`：比较剩余字节与 edx，不足跳 `146EA0C30`；该处是
  `mov dword ptr [0],0`，主动触发访问违例。这是明确的短包断言，
  **不是推测的 actor 空指针或巨量 memcpy**。
- 十测 N2252 实际仅 **54B**（`789c...` zlib 流），N2253 也是 **71B** 压缩流。
  `completeIspinsStage → preparePackets → EncryptPayload → ServerFrame`
  没有解压步骤；本地固定长 reader 必然拒绝。
- `D:/115us/DFO.exe`（此前反汇编工具的默认目标）与正在运行的
  `D:/115us/client/DFO.exe` **不是同一版本**：前者同地址 `146EA0C30`
  甚至落在另一条指令中间。官服压缩体相等不能证明本地兼容。
- CMD2060 真名为 `ENUM_CMDPACKET_INSPECT_ACCOUNT_GUIDE_QUEST`，并非已证明
  的“缺 N2 查询”。应停止沿这个未经证实的解释补包。

资源先验检查：本地 EXE、sk.dat、外层 Script.pvf、服务端内层 Script.inner.pvf
SHA256 全部与 `Script.inner.manifest.json` 匹配，无资源快照漂移证据。
外层/内层哈希不同是解码前后差异，不是资源缺失。

可复跑的只读取证脚本：`analysis/tasks/next79-native-reader-evidence.py`；
结果 `next79-native-reader-evidence.json`，含当前 EXE 哈希、三段原生指令、
7772/2405 长度立即数与主动空地址写入的机器码断言。不打开或修改权威 IDB。

### 29.2 修复边界

`internal/legion/ispins.go` 两个奖励构造函数返回原始固定长数据，删除 zlib
封装及仅供这两个函数使用的压缩 helper。四阶段奖励内容、链序、旗标、
nonce 和既有辅助包全部保持本轮起点行为；不新增猜包、DLL 或客户端补丁。
仅修正已证明不兼容的 S2C 奖励体封装，不涉及 C2S 猜包尝试。
不改数据库/schema、玩家存档或客户端运行资源；既有奖励入库限制仍保留。

### 29.3 验证与发布

- `TestIspinsRewardPayloads` 改为直接校验输出体 7772/2405B，测试中不再
  先解压来掩盖发送错误。
- 新增 `TestIspinsSettlementNativeRewardDelivery`：四阶段真实 `ConfirmDeath`
  触发完成，运行 `completeIspinsStage → preparePackets`，验证发出的
  奖励体达到 native reader 长度，解密后的帧仍携带完整原始体。
- 用 Go overlay 只恢复本轮前的压缩行为，同一回归 **失败**：
  `stage 0 NOTI2252 body=54, native reader requires 7772`；修复后四阶段通过。
- Ispins / LegionReward / PreparePackets 专项测试通过；`go vet ./...` 通过。
- `go test ./...` 有 4 项失败：wireprobe 的 Adventure/PVFCatalogGate/
  Enhancement 三项，以及 cashshop 的 `TestShopPilotPVFCurrentCatalog`
  （`empty delivery 3400013`）。四项都在修复前 overlay 中原样复现，
  不扩大本轮范围。日志在仓库外 `analysis-tools/output/next79-*`。
- 构建 `wireprobe-handoff-source.exe` 并复制到默认 `wireprobe-pvf.exe`，
  两者 SHA256 = `e71d3e7efeb50f1abd6aad98f12f0dd0296e714ecce4f8366eb721cfdf978ac2`。
  发布时无游戏/服务端进程，原 39 基准保留。
- 14:02:25 通过 `launch_local.py --server-only` 准备服务端，PID 36608，
  已确认 7001 就绪；新日志会话尾名 `20261003_140225_417856_next37`。
  沿用默认入口的四个环境变量，没有启动或操作客户端。

**待用户十一测**：从默认入口进入伊斯大陆，击杀 stage0 Boss。日志应见
N2252 未压缩 7772B、N2253 未压缩 2405B；验收为出现结算面板、能领奖及
退出/进下一阶段。若仍闪退，先读新的 CrashDump 返回地址和实际 reader，
不重复猜 nonce/延迟/补辅助包。当前只确认旧崩溃触发点被修正，未宣称实机已修好。

## 30. 十一测：N2252 已跨过，崩溃移至 N2 装备行；同时补变更作战 action4

### 30.1 两个问题的实际日志

用户反馈：“还是打完 Boss 就卡死”“选择变更作战点击确定没有回到选择其他难度”。
会话 `20261003_140426_861308_next37`，客户端进程 32440；此次使用原生启动器，
没有 `client-direct.out`，完整客户端诊断在 **client_trace.txt**（不能把缺 stdout 当成缺 CrashDump）。

结算时间 14:07:08，退出 14:07:09.397。服务端 N2252=7772B、N2253=2405B。
客户端明确记录 N2252 已接收 `(Size:7776)`（含加密对齐），继续接收 N14 和
N2 `(Size:480)` 后发生异常；**未接收到后续 N2253**。
新调用栈：`146EA0C30 ← 146D77F77 ← 1459A024D ← 14563990B ←
14563F0E7 ← 145637BEB ← 1459A3C7C`。
这与 §29 的 `1424FDD06` 奖励 reader 栈不同，证明旧触发点已跨过，
并不代表整体结算已验收。

直接反汇编当前 client/DFO.exe：N2 基础角色行 `14563EC60` 先读取160B，
`14563F0E2` 调用装备读取器 `145639840`，其 `145639906` 调用
`1459A0220` 读取长度前缀字段，最终在 fixed-copy 的短包断言崩溃。
§27 用新版官服角色模板只重排名长/actor，未重建本地装备行语法，
因而不能兼容本地客户端。此前“补 N2 会让客户端进入结算”的推断不能当事实使用。

变更作战日志：14:06:26 客户端 CMD2047 body：
`0000000000000000000000000004000000ffff00000000000000000000000000`。
服务端立即 `ispins_refused: ispins operation variant 4 unknown`。
用户随后仍按原确认进图，说明旧确认态并未清除。

### 30.2 原生 action4 应答闭环

当前原生发送函数 `1425311F0` 在 body@13 写 action u32、@17 写 argument u16，
使用 opcode2047，发送19B；实机经 transport/padding 后为32B。action4、argffff
已由用户本次操作动态命中。

ACK2047 注册点 `1425314CC` 指向 **142530950**：成功字节已由公共分派消费；
该 handler 直接读取19B结构。首 u32 action 判别允许1、2、4；`142530A1E`
检查4后获取窗口 **0x284=644**，`142530A58 →14251DCA0` 清理三个作战选择控件。
结构+6 flag=0 进入选择路径，恢复 mode1/选择面板；argffff 与既有 variant-A
的未指定参数语法一致。此处依据本地 reader 构造，未猜未知 opcode或布局。

### 30.3 修复与验收边界

- 结算 N2：停止在生产链调用 `protocol.IspinsSettlementCharacterInfo` 官服模板；
  改用当前 `w.characters.EntryBasicProbe/EntryAddition`，直接投影当前角色的存档。
  这两个编码器已在同一客户端建队、进图接受，避免继续拼新版模板。
  旧模板保留为历史调查资产，并标 Deprecated；旧拼接测试改名明确只是历史模板测试。
- CMD2047：增加严格 action4/ffff 解码；按本地19B ACK结构返回成功＋action4，
  重置 confirmed=false，保留通关进度。下一次 action1 也清除旧确认；
  只有重新 action2 确认后才能 CMD2045 进图。
- 本轮修的是“变更作战确定后重新进入选择流程”。**不把 counter/token 猜成难度值**，
  也不宣称难度与战斗数值映射已实现；若返回选择界面后新选择仍被覆盖，
  下一步须读取新操作向量及本地选择表，不恢复硬编码试探。
- 奖励原始体、链序、旗标、辅助包与既有奖励入库限制保持；无DLL/客户端资源/
  数据库结构/玩家存档改动。action4 新运行路径记 **attempt1/3（已有动态请求＋静态reader证据）**。

### 30.4 回归、发布与下一测

四阶段完成回归现在同时检查 N2 两包来自本地角色原生编码器、以及7772/2405B
奖励包的实际加密传输；变更作战回归覆盖 live action4、非法字段拒绝、清确认而
保留进度、禁止直接进图、重新选择→确认成功。
通过 overlay 恢复本轮前 N2模板＋缺 action4 分支，两项回归分别失败为
“必须使用本地角色资料”和“variant4 unknown”；修复后全部专项通过。
`go vet ./...` 通过；全量 Go 测试仍仅 §29 已对照确认的4项失败，没有新增失败。
日志 `analysis-tools/output/next79-native-character-reset-*` 和 `next79-pre-native-character-check.log`。

两个 EXE 已重建并发布，SHA256：
`b3fc95409ac600108030221ea69fdb0d5c83bf7180559394b703be4cb1aa82c9`。
扩展 `next79-native-reader-evidence.py/json` 收录本地装备reader、2047 writer/ACK控制流；
函数索引更新，不直接修改权威 IDB。

**待用户十二测**：先确认“变更作战→确定”能重新显示难度选择面板，再重新选择、
确认、进图击杀 stage0 Boss，观察结算/领奖/退场。若再崩，先查 client_trace.txt
最后接收包与新调用栈，依次修真实不兼容reader，不把任一修复候选写成实机成功。

14:20:46 启动纯服务端，14:22:03 完成源数据加载，PID21500、7001已就绪，
会话尾名 `20261003_142046_845846_next37`。没有启动或操作客户端；待用户手动验收。

### 30.5 用户实机确认与收口

用户确认变更作战、Boss结算、奖励发放生效；14:23会话前两阶段均发1654→2046，
第一阶段72回待机成功，基线SHA256 `b3fc95409ac600108030221ea69fdb0d5c83bf7180559394b703be4cb1aa82c9`。
本次确认已写CHANGELOG和 `docs/protocol/next79-confirmed-baseline-20261003.md`。
第二阶段正式回城仍被拒，是新的focus清理缺陷，继续§31，不能扩大已确认范围。

## 31. 第二阶段回城无反应：focus ACK 错误触发服务端会话清理

### 31.1 本地会话证据

用户截图停在死亡之森(stage1)结算后的“是否继续/返回城镇”按钮，客户端仍存活。
14:23会话事件：

- 14:28:22.624 stage1 CMD2046结束领奖，服务端正常应答及N2255 wait2。
- 14:28:23.065 CMD72 `02020100000000000000000000000000`（state2 focus，option2城镇），
  服务端发正确字节 `010202`，但事件名错误为 **settlement_exit_ack**。
- `main.go` 发送后回调按此事件名清 `activeDungeon`、completionSent、翻牌状态。
- 14:28:24.038 真正 CMD72 `01020100000000000000000000000000`（state1确认）
  已无activeDungeon，落通用翻牌门禁，拒绝 `cards before owned settlement`。
- stage0只发state1，因而正常回城；第二阶段先focus再确认，才出现问题。

协议真源沿用已确认 `docs/protocol/settlement-exit-envelope-20261001.md`：
当前CMD72 handler消费 success/state/option，state!=1只改按钮状态，state1才执行离场。
这不是ACK字节缺失，也不需要新增回城包；错误位于服务端发送后状态清理的事件标记。

### 31.2 最小修正

`card_flow.go settlementExit` 的Ispins分支先构造 **settlement_focus_ack**，
state2直接返回此包；只有state1通过leaveDungeon、回城帧准备完成后，
才命名为settlement_exit_ack，由既有发送成功回调清会话。
ACK内容与回城路由顺序不改；普通副本、撤退和已有军团阶段进度保持。
只修状态生命周期，不涉及数据库/玩家存档/客户端补丁，不猜新协议字段。

### 31.3 验证与候选

新增 `TestIspinsSettlementFocusThenTownExit`：四阶段真实boss死亡完成，
重复focus两次均只发010202 focus ACK并保留副本/完成标记，随后state1产生
010102正式ACK以及NOTI3/23/24回到原待机区；只有正式ACK带退出清理事件名。
相关Ispins/SettlementExit/CardTransport/LegionReward专项通过；vet通过。
overlay仅恢复focus包的旧事件名，新回归立即失败，证明可抓住本次提前清理缺陷。
外部日志：`analysis-tools/output/next79-pre-focus-fix-check.log`。

源码候选已构建 SHA256：
`925f6b74998725998891346d1fac881fe51c33df212d5a924d0f3d9211b2bda3`。
已确认基线单独commit **0acf471**，未夹带本轮回城候选或其它用户改动。
全量 `go test ./...` 仍仅4项既有失败（Adventure/PVFCatalogGate/Enhancement/ShopPilot），
其余通过，没有新增失败，完整日志 `analysis-tools/output/next79-focus-exit-go-test.log`。

用户要求“更新完告诉我，我来重启服务端”。按此要求已复制候选到默认wireprobe-pvf.exe，
两者SHA256均为上述925f6b74；发布时旧服务端已退出，无需结束客户端。
没有启动服务端，由用户自行重启；没有改数据库和客户端资源。回城候选尚未实机确认。

### 31.4 用户确认

用户确认四个副本地图均可战斗、完成结算。14:41会话stage1在14:50:02聚焦后
14:50:05正式回城成功，§31修复纳入925f6b74 confirmed baseline。
最终动画后右上角回城及CMD13离队仍有问题，另行§32，不扩大确认范围。

## 32. 最终动画后回城与退出队伍

### 32.1 动态证据

会话 `20261003_144132_451428_next37`：四阶段均进图并CMD2046领奖结束。
最后stage3在14:52:57领奖后CMD191 state0开始动画，14:54:10 state1结束动画。
`ispinsStoryPause` 在state1中立即执行 `w.ispins=nil`，但玩家此时仍在最后副本，
activeDungeon还在。14:54:24/25右上角CMD72 focus/正式返回城镇依次为
`02020100000000000000000000000000` / `01020100000000000000000000000000`，
两者均误入普通cardsReady并拒绝 `cards before owned settlement`。
ESC菜单在14:54:47正常回城，是另一路菜单离场，说明目的城镇路由本身有效。

14:54:54 CMD13 LEAVE_PARTY body为8个零。主循环的伊斯队伍handler只接CMD12，
CMD13被记unimplemented_sample、没有任何队伍解除应答；这与用户“退出队伍无效”吻合。

### 32.2 修正与本地语法

- run增加storyFinished，将动画完成与副本离场分开；CMD191结束动画只设置标记，
  保留w.ispins，直到正式CMD72通过leaveDungeon预检、准备退出ACK和回城帧时再释放。
  聚焦仍保留activeDungeon/挑战状态。ESC回城后留有挑战标记时，CMD13同样可以清理。
- 现有ispinsStandbyPartyHandle同时接CMD12/13；严格只接受空/8零离队请求，
  只在本伊斯频道、已选角且不在副本内处理，副本内拒绝而保留队伍/挑战状态。
  成功发送通用NOTI9 party-gone，清soloPartyReady及剩余Ispins记录；可以重新建队。
- 解除包复用已验证本地 `BlackPurgatoryPartyGone` 编码器，与Moon族同形，
  12B：`0100 0f27 0100 <本连接channel2B> 00 03 01 00`。伊斯建队同用本地party9999。
  不复制官服新版NOTI9、不新增猜测的ACK13。
- 本地1452F2620头部读取两个u16＋每块u16/6字节；p9 action3在1452F2AA4跳过
  资料段、1452F2F05跳过成员资料，1452F40FA..4132把8个成员槽置ffff解除成员。
  证据收录扩展的next79-native-reader-evidence.py/json；完整asm留仓库外output。
- 无数据库结构/玩家存档/客户端资源/DLL变更；没有调整奖励内容或难度数字。

### 32.3 回归与发布

新增最终全流程回归：最后奖励CMD2046→CMD191开始/结束→CMD72 focus→正式回城→
成功离队→重新建队；检查动画完成时仍拥有结算，只有正式离场释放终局记录，
通用NOTI9本地解除体逐字节匹配。另覆盖无效长度/非零选项/副本内离队拒绝、其它频道不接手。
此前四阶段focus→回城回归及全部Ispins/SettlementExit/CardTransport/LegionReward专项仍通过。
overlay仅恢复“动画末尾清ispins”和“只接CMD12”后，两项新增回归均失败；修正后通过。
go vet ./...通过；全量go test ./...仍仅4项既有失败，无新增失败。
日志 `analysis-tools/output/next79-finale-party-exit-*` / `next79-pre-finale-party-check.log`。

四阶段/阶段间回城确认已单独commit **d37c53f**；只暂存本任务伊斯分支及确认记录，
没有夹带card_flow.go内其它workflow/疲劳/塔修改和CHANGELOG已有其它95行变更。
本轮候选及默认EXE均已更新为SHA256：
`de16748a507b07ae269206f71fc5a5265e87353aa9a74f0f60f26907c92c5686`。
发布时无服务端/客户端进程；遵循用户此前要求由用户自行重启服务端，没有代启动客户端。

**待用户实机**：四阶段打完最后动画，直接用右上角返回城镇；回待机区退出队伍，
确认队伍窗口关闭，并可重新创建队伍。也可用ESC回城再退出队伍验证另一入口。
本轮候选尚未实机确认，不扩大925f6b74基线。

### 32.4 用户确认

用户确认100%完整走通，de16748a纳入最新confirmed baseline；已更新CHANGELOG和
next79-confirmed-baseline-20261003.md，收口本轮。恢复次数提议单独§33，不扩大实机确认范围。
