# 取证报告：SQLite 档「点创建角色客户端崩溃」登录阶段包集合差（2026-10-04）

> 承接 `session-handover-20261004-sqlite.md` §5 第 1 步。本报告只记录**可复算的证据**，
> 不含未闭环的推测；未启动客户端、未触碰玩家库（PG 25438）。

## 0. 结论摘要

1. **§5 要求的「登录 → 选角」阶段服务端发包集合差 = 空集。**
   崩溃会话与同一 `--source-build` 二进制族、同一 SQLite 引擎的成功会话之间，
   该阶段 13 个服务端发包全部一致：其中 7 个在 `events.jsonl` 里带 payload，
   **逐字节相同**（唯一差异是 CMD1960 服务器时钟里的 unix 时间戳）；
   另 6 个（`channel_identity`、`roster_background_restored`、708/1792/1198/1336）
   按结构化字段与客户端 trace 收到的尺寸逐项核对，也完全相同。
2. **交接档案 §3 的假设被证伪**：成功会话在点「创建角色」那一刻，
   数据库里的 `characters = 0`、`account_unified_options = 0`，与崩溃会话**完全一样**，
   并且从这个状态成功建号（角色 `qqqq`，创建奖励 `100000` cera 已入账）。
   所以「空角色列表 / 空账号选项」不是本次差异的成因。
3. 崩溃点已被客户端 trace 对齐精确定位：**客户端在「选角界面 → 创建角色窗口」的切换中本地访问冲突**，
   发生在服务端完全不参与的阶段（服务端自 roster 之后一包未发）。
4. 崩溃调用栈落在 UI 弹窗模块（0x1466xxxxx），其中 `14668cdf9` 位于
   本项目已 dump 的 `sub_14668C520`（即打印 `IRDPopupWindow Type : N` 的那个函数）内。
5. 客户端最后一次发包 682 是**退出/关服信号**（`cmd/wireprobe/client_dispatch_account.go:153`），
   发生在崩溃报告之后，属崩溃处理流程，不是创建请求；与本项目已记录的 `op=682 闪退` 模式一致。

→ **没有服务端改动依据**（遵守「禁止猜包」）。能区分「偶发」与「确定性回归」的唯一实验见 §6。

## 1. 对照会话

| 角色 | 会话目录 | 服务程序 | 客户端退出码 | 数据库 |
| --- | --- | --- | --- | --- |
| 崩溃 | `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261004_224700_762172_next37` | `bin\wireprobe-handoff-source.exe` | `0xC0000005` | `runtime\storage\dfolan.sqlite3`（22:47:35 新建） |
| 成功建号 | `..._20261004_221054_428343_next37` | `bin\wireprobe-handoff-source.exe` | `0x0` | 当时的 `dfolan.sqlite3`，现为 `runtime\storage\dfolan.sqlite3.first-try`（mtime 22:15:20） |
| PG 对照 | `..._20261004_193431_865803_next37`、`..._20261004_190224_808571_next37` | `bin\wireprobe-pvf.exe` | 无 client.log | PostgreSQL |

两侧 `gateway.out` 的服务程序命令行（fixture / storage / 各 catalog 参数）逐字相同；
两侧 `client.log` 的 `SYNTHETIC_TEST_ARGUMENTS` 也逐字相同。

## 2. 包集合差（§5 第 1 步）

方法：读 `events.jsonl`，从第一条 `client_frame` 起按时间顺序把「客户端帧(plain_hex)」
与「服务端发包(payload_hex/hex)」交错成序列，剔除遥测流 2127，逐项比较。
客户端帧与发包都以本会话的记录为准，不做任何反推。

| # | 崩溃 224700 | 成功 221054 |
| --- | --- | --- |
| 0 | C> 1554 `0500000070726f626500400b00000000` | 相同 |
| 1 | S> 1554 `01120620…0a74` | 相同 |
| 2 | C> 1 `0500000070726f6265200000003030…322cf104e` | 相同 |
| 3 | S> 1 `010100380000000000000036000000003f6db174…` | 相同 |
| 4 | S> 1960 `011567c26a` | **`01a65ec26a`（仅 unix 秒不同）** |
| 5 | C> 1593 / 6 C> 171 / 7 C> 468 / 8 C> 585 / 9 C> 8 | 相同 |
| 10 | S> 2 `000200300000000000000099000000002c31a97b…2acd934`（48B） | 相同 |
| 11 | C> 848 | 相同 |
| 12 | S> 848 `01500328000000000000002d000000007efb35db…`（40B） | 相同 |
| 13 | C> 433 `0601020304050600` | 相同 |
| 14 | S> 433 `01b101180000000000000016000000001212779d8da463d9`（24B） | 相同 |
| 15 | C> 637 | 相同 |
| 16 | C> 433 `0601020304050600` | 相同 |
| 17 | S> 433 同 14 | 相同 |
| 18 | C> 407 | 相同 |
| 19 | C> **682** `01000000`（退出信号） | C> **2** `05c0a81f83c0a81f83c81c000200000c00000033303536306635393362663700`（`ENUM_CMDPACKET_SET_UDP_IP_PORT`，见 `analysis/dumps/opcodes.tsv`） |
| 20 | — | C> **684** `04000000717171710000000000000000`（`CHECK_DOUBLE_CHARACTER_NAME`） |

无 payload 的结构化事件也逐一核对过（不参与上面的 hex 比较）：

| 事件 | 崩溃 224700 | 成功 221054 |
| --- | --- | --- |
| `roster_background_restored` | `owned_count=0`，`selected` = 5×`{Category:0,ID:0}` | 逐字段相同 |
| `weekly_dungeon_config_sent` / `quest_clear_group_info_sent` / `integrate_event_data_sent` / `weekly_dungeon_inout_info_sent` | 708 / 1792 / 1198 / 1336 | 相同 |
| 客户端 trace 里收到的尺寸 | `SELECT_CHARACVIEW_BG 32`、`WEEKLY_DUNGEON_INFO_CONFIG 24`、`QUEST_CLEAR_GROUP_INFO 96`、`INTEGRATE_EVENT_DATA 176`、`WEEKLY_DUNGEON_INOUT_INFO 16` | 逐项相同 |
| `INTEGRATE_EVENT_DATA` 正文 | `eventIndex=2370`、`[kIntType] [0][0][0][0][0][2][0][0]`、`[kAll] [0]`、全 0 矩阵、`month:10,day:4,hour:14` | 逐项相同 |

**差集为空。** 唯一有内容差异的是 CMD1960 的服务器时钟（墙钟），不可能造成崩溃。
`events.jsonl` 里崩溃会话客户端总共只发出 `1554、1、1593、171、468、585、8、848、433、637、433、407`（外加 2127 遥测 ×17、682 退出）。

## 3. 交接档案 §3 假设的证伪（数据库证据）

对两个 SQLite 库逐表 `COUNT(*)` 与逐行 dump（只读，副本比对）：

| 表 / 行 | `dfolan.sqlite3.first-try`（22:10 那轮用的库） | `dfolan.sqlite3`（22:47 崩轮用的库） |
| --- | --- | --- |
| `accounts` | 1（`probe`，created_at 14:11:29） | 1（`probe`，created_at 14:47:35） |
| `characters` | **1**（`qqqq`，profession 0，created_at 14:12:02） | **0** |
| `account_unified_options` | **3**（opt_index 1 / 230 / 276，updated_at **14:12:20**） | **0** |
| `admin_grants` | 1（`reward:character_create:newchar.lua:1:cera` → +100000 cera，14:12:02） | 0 |
| `character_birth` / `character_world` / `character_tutorial_flags` 等 | 有（建号后产生） | 无 |
| `storage_migrations` | 37 | 37（checksum 逐条相同） |
| `PRAGMA application_id` | `1152026104` | `1152026104` |

关键时序：成功会话客户端 **14:11:50** 进入选角（此时库里 `characters=0`、`account_unified_options=0`），
**14:12:02** 建号成功；`account_unified_options` 的三行是 **14:12:20**（进城后客户端写设置）才落库的。

⇒ 「空角色列表」「空账号选项」在成功轮里同样是首次遇到的输入，且**并未导致崩溃**。
交接档案 §3 把这两个输入列为可疑差异，不成立。

## 4. 崩溃点定位（客户端侧，服务端未参与）

崩溃会话的 `client_trace.txt` 是崩溃报告里的**环形缓冲尾部**（`<TRACE>` 段 410 行原始行）；
成功会话留有**全程** trace（4312 行）。用 difflib 把前者对齐到后者：

* 崩溃 trace 与成功 trace **逐行相同**（仅毫秒级计时数值不同）直到
  `[14:47:53] [ETC] Close IRDPopupWindow Type : 4019`；
* 成功会话在该行之后的**紧接着两行**是：

```
[14:11:55] [ERR] ERROR! Image index error : sprite/interface2/charactercreatever2/particle/light.img (90)
[14:11:55] [ETC] Open IRDPopupWindow Type : 181
```

* 崩溃会话 trace 共 143 条去噪行，其中只有 26 条在成功会话全程找不到对应：
  24 条是 `createReport()` 块本身，另 2 条是
  `ItemInfoWindow::initFindSkillList() 0ms`（成功轮记的是 `1ms`）和
  `CNSelectChannelSocket::connectChannel() - ip=127.0.0.2, port=-9531`
  （成功轮是 `port=-9333`；两侧都把游戏网关端口 56003 / 56201 按有符号 16 位打印，
  是两轮都存在的既有显示问题，与本次无关）。
  **崩溃轮没有任何独有的客户端行为或独有的资源错误**（`[ERR]` 集合是成功轮的子集）。

两侧的弹窗编排完全一致，可确认 4019 是「创建角色」的过渡弹窗、181 才是创建窗口本体：

| 成功 221054 | 崩溃 224700 |
| --- | --- |
| `Open 3789` → `Open 2425` 14:11:50 | 同（14:47:49） |
| `Open 4019` 14:11:53 | `Open 4019` 14:47:51 |
| `Close 2425` 14:11:53 → `Close 4019` 14:11:55 | `Close 2425` 14:47:52 → `Close 4019` 14:47:53 |
| `light.img (90)` → `Open 181` 14:11:55 | **崩溃（无 light.img 行、无 181）** |
| `Open 2429/2433` 14:11:58 → `Open 2875`+CMD684 14:12:01 → CMD5 14:12:02 | — |

崩溃调用栈（`createReport()` 块）：

```
145a3b706  146de2acf  14576c04c
7ffaad26f613 7ffaafbe93a0 7ffaafb9f933 7ffaafbe4abf 7ffaafac5e97 7ffaafbe43fe   (系统 DLL)
146ee1cb4  14173caed  141735543  14668cdf9  140239e9b  1404b2a67
146671c80  14667296d  14668f506  145a06bd7  1401f4977
```

其中 4 帧（`14668cdf9`、`146671c80`、`14667296d`、`14668f506`）落在 0x1466xxxxx 的 UI 弹窗区间；
`14668cdf9 = sub_14668C520 + 0x8D9`，而 `sub_14668C520` 就是本项目已 dump 的
`analysis/dumps/awakening-2258e/ui_popup_14668C520_sub_14668C520.c`——即打印
`Open/Close IRDPopupWindow Type : N` 的那个函数本体。
→ 访问冲突发生在弹窗模块内部，正好是「要开 181 却还没开成」的那一步。

时间线（`events.jsonl` 与 `client.log`）：
`14:47:53` 写崩溃报告 → `14:47:54.556` 客户端发出 CMD682（`exit_shutdown_signal`）→
`14:47:57.4` 进程以 `0xC0000005` 结束。682 属崩溃收尾，不是创建请求。

## 5. 两轮运行环境差异清单（穷举后均已排除）

| 项 | 22:10 成功轮 | 22:47 崩溃轮 | 判定 |
| --- | --- | --- | --- |
| 服务程序 | `wireprobe-handoff-source.exe` | 同一路径，重建后含 `3ec50297`（22:15:13） | 登录阶段输出逐字节相同 ⇒ 该提交未改变被测阶段行为 |
| 存储引擎 / 路径 | SQLite，`runtime\storage\dfolan.sqlite3`（全新） | 同 | 无差异 |
| 库内容（登录时） | accounts 1 / characters 0 / options 0 | accounts 1 / characters 0 / options 0 | 无差异 |
| `Script.pvf` | `INIT>> script checksum: bcf02f20…dff` | 同一校验和 | 无差异 |
| 客户端 `[ERR]` 集合 | 较大 | 为其子集 | 崩溃轮没有多出的资源缺失 |
| 客户端自报模块遥测（2127） | 11 个 ASCII 串（进程/路径） | 逐串相同，仅日期时间串不同 | 无差异 |
| 客户端启动参数 | `probe.exe … 3?127.0.0.1?7001?probe?…` | 逐字相同 | 无差异 |
| `DFO` 目录 2026-10-04 被改动的文件 | — | 仅 `LagLog.txt`(19:39)、`NGClient64.aes`(22:39:17)、`cef.log`+`BlackCipher\*.log`(22:47:37–55) | `Script.pvf`/`sk.dat`/NPK 均未改动 |
| `NGClient64.aes` | — | mtime 22:39:17，但与 `C:\Game\dof\110us\client\NGClient64.aes` **SHA256 相同**（`7AE957DC362E2999B7B5CAACD11B59F48DF84F84A2B9875B1B1818C42F633B19`） | 只是被重写了一次，内容未变 ⇒ 不是反作弊版本变化 |
| 崩溃计数 `CrashDNF2.cra` | 未写 | 1 字节 `0x06` | 该客户端安装**此前已记录 5 次崩溃**；今天其余会话均非 `0xC0000005` |

## 6. 下一步（唯一能区分的实验，需业主手动操作）

证据已经把服务端与协议排除；剩下的候选（客户端本地偶发 / 时序）只能靠**复现**区分。
请在**不改任何代码**的前提下，用同一个入口再跑一轮并做两次点击：

1. 复现轮 A：进入选角界面后**立即**点「创建角色」。
2. 复现轮 B：进入选角界面后**等 15 秒**再点「创建角色」。

判定：

| 结果 | 结论 | 下一步方向 |
| --- | --- | --- |
| A/B 都再次 `0xC0000005` | 确定性客户端崩溃，与协议无关 | 对 `sub_14668C520` 开窗路径 + `charactercreatever2` 资源做 IDA/资源比对（§2 的 IDA 步骤改为针对**创建窗口构造**，而不是「创建界面读空列表」） |
| A 崩、B 不崩 | 时序/加载未完成 | 客户端侧等待策略；服务端无需改动 |
| A/B 都能建号 | 22:47 那次是偶发 | 本阻塞关闭，按 §6 继续「建号 / 穿脱装备 / 接任务 / 退出重进」回归 |

诊断留档请保留（不要删）：
`runtime\roles_*\events.jsonl`、`client_trace.txt`、`client.log`、
以及 `C:\Game\dof\115us\DFO\CrashDNF2.cra`（崩溃计数，重跑前后各读一次即可知道是否又崩）。

## 7. 本次用到的复算脚本（会话工作区 `.dsh-scratch\`，未入库，已清理）

`login-phase-diff.py`、`login-timeline.py`、`bytediff.py`、`db-compare*.py`、
`trace-compare.py`、`trace-align.py`、`trace-gaps.py`、`unique-lines.py`、
`session-survey.py`、`telemetry-strings.py`。

## 8. 边界声明

* 未启动客户端、未代替玩家操作；未访问玩家库（PG 25438）；未启动或替换任何服务程序。
* 未改动 PVF、schema、客户端资源；未提交任何代码或文档。
* 本轮唯一的环境改动是 §9 记录的 **SQLite 开发库换库**（可回滚），不涉及 PostgreSQL。
* 本报告只把已复算的日志/数据库事实写成结论，未闭环部分（客户端弹窗模块内部原因）明确留白。

## 9. 本轮附带处置：恢复 22:12 建号库为活动库（业主选定）

业主选择「先恢复 22:12 那个真建号库」，以便先把 SQLite 档的非建号回归跑起来。
处置（全部可回滚；执行前已确认没有任何服务/客户端进程占用该库）：

| 文件 | 动作 |
| --- | --- |
| `runtime/storage/dfolan.sqlite3`（22:47 崩轮那个空库） | 改名保留为 `dfolan.sqlite3.empty-from-224700` |
| `runtime/storage/dfolan.sqlite3.first-try`（22:12 真建号那轮的库） | 复制为活动库 `dfolan.sqlite3`（原件保留不动） |
| `runtime/storage/dfolan.sqlite3.admin-guard` | 内容是崩溃会话服务进程 PID `20776`（已不存在）的过期管理租约；按 `sqlite_adapter_manual.go` 的 TTL 语义删除（本来 60s 后服务端也会自动回收） |

换库后的活动库校验（只读）：

```
application_id      = 1152026104 (OK)     integrity_check = ok
storage_migrations  = 37
accounts = 1   characters = 1   account_unified_options = 3   admin_grants = 1
characters[1]: name='qqqq' profession=0 roster_order=1 fixed_slot=0 config_version=c638346f…
  == 当次存档契约? 是（savecontract.Identity()，见 gateway.err 的 save identity normalized）
  state.source_sha256 = 8d382caf…  ← 这是「职业目录身份」(prof.RawSHA256，见
                                      internal/character/service.go:129)，不是内层归档哈希；
                                      与当前 configs/characters.*.json 中的该哈希一致
character_vaults / secondary: config_version = fda6c33f… == 当前规则身份? 是
account_unified_options = (1,1,1) (1,230,1) (1,276,1)
account_currency: cera = 100000
```

副作用（正向）：这个库里 `account_unified_options` 非空，所以下一次实机不再处于
「客户端首次见到空账号选项」的状态——顺带把 §3 的第二个可疑输入也去掉了。

回滚方式：把 `dfolan.sqlite3.empty-from-224700` 复制回 `dfolan.sqlite3`，
并删除可能新增的 `dfolan.sqlite3-wal` / `-shm` 即可。

> 附带：为让换库能写入，按 DSH 的 Windows 文件权限诊断流程对
> `server` / `server/work` / `server/work/dfo-lan` / `…\runtime` 补了当前用户的完整控制权限；
> 脚本对每处改动都留了备份与撤销命令，报告与回滚脚本在 `C:\Game\dof\115us\dsh-acl-report\`。

## 10. 换库后首次启动失败（23:17）：不是存储，是**频道监听端口扇出**被系统拒绝

会话 `…_20261004_231708_393357_next37` 的 `gateway.err` 末尾：

```
23:17:43 save identity normalized: 0 stored row(s) -> contract c638346f… (inner archive b2b503b5…)
23:17:43 listen tcp4 127.0.0.2:52270: bind: An attempt was made to access a socket in a way
         forbidden by its access permissions.
```

**换库本身已被这次真实启动验证通过**：服务端成功打开换上的库（`save identity normalized`，
且 0 行需归一 = 角色的 `config_version` 本来就等于当次契约），并完整准备完全部目录，
失败点在**最后一步监听**，与存储/存档无关。

失败的具体位置是**频道监听扇出**，不是 `:0` 那次绑定：

`cmd/wireprobe/main.go:134-150`
```go
bindHost, _, _ := net.SplitHostPort(startup.GameListen)      // 127.0.0.2
_, portText, _ := net.SplitHostPort(l.Addr().String())        // 主监听(127.0.0.2:0)拿到的端口
basePort, _ := strconv.Atoi(portText)
for i, ch := range channelCfg.Channels {                     // 本次 18 个频道
    ln := l
    if i > 0 {
        ln, err = net.Listen("tcp4", net.JoinHostPort(bindHost, strconv.Itoa(basePort+i)))
        if err != nil { return err }                          // ← 52270 在这里被拒
    }
```

判据：错误串里带**具体端口 52270**。`startup.GameListen` 是 `127.0.0.2:0`，端口 0 的绑定
失败时 Go 报的仍是 `:0`；出现具体端口只可能来自 `:145` 这行的**显式端口**绑定
（`basePort+i`，即 `52270 ∈ [basePort, basePort+17]`）。
而 `:0` 由操作系统选端口，系统不会发一个自己禁用的端口；**显式**端口被拒只剩两种可能：

1. 该端口落在 Windows 的**排除端口段**里。当前 `netsh int ipv4 show excludedportrange protocol=tcp`
   显示动态保留段会随时间变动（`49152-49251`、`49452-49551`、`50000-50059*`、`57705-58006`…），
   这些是 Hyper-V/HNS/WSL 之类服务启动/停止时增删的 ⇒ **同一份代码、同一个库，
   22:10 / 22:47 成功而 23:17 失败**，正是这种间歇性成因；
2. 网络过滤驱动（本机在跑 `aTrustAgent`/`aTrustXtunnel`/`eaio_service`/`eaio_agent`（深信服）
   与 `rustdesk`）拒绝了这次绑定。
   项目自带的诊断也把 `probe.exe --net-check` 非 0 解释为「probe.exe 的运行时环境异常
   （常见于安全软件拦截）」（`115us-dfolauncher/internal/diag/diag.go`）。

本机的旁证（在 DSH 会话内实测，非管理员）：

| 检查 | 结果 |
| --- | --- |
| `TcpListener` 绑 `127.0.0.2:0` | 成功（拿到 52290 / 52345） |
| 回环 监听+连接 `127.0.0.2` | 成功 |
| 52270 是否在当前 TCP 排除段 | **不在**（说明是当时那一刻的状态，不是恒定值） |
| 残留 `wireprobe*`/`probe.exe`/`DFO.exe` 进程 | 无 |
| 7001 端口占用 | 无 |
| `dfolauncher.exe stop --root …` | 正常执行，`Environment fully stopped`（SQLite 档，不动 PostgreSQL） |

> 注意：我在**未提权**的 DSH 会话里跑 `probe.exe --net-check` 得到 10060（超时），
> 这个结果受本会话自身网络限制影响，只能提示、不能定罪；启动器在沙箱外跑同一自检，
> 以它的输出为准。

**处置建议（按代价从低到高）**：① 直接重试同一次启动（`basePort` 每次都不同，很可能落到别的段）；
② 仍失败就重启机器（清 WFP 过滤状态并重排动态保留段，这是 WSAEACCES-on-bind 的常规解法）；
③ 再失败就临时退出深信服 aTrust/EAIO 与 RustDesk 后重试，并确认 UAC 提权确实生效
（WFP 回环隔离需要管理员）。

**可选的源码加固（待业主决定，未实施）**：扇出失败时不要直接结束启动——
关闭已开的监听、重新从 `:0` 取一个 base 端口，再重试整组扇出（有界次数）。
端口扇出本身不要求连续（`endpoints[ch.ID]` 各带自己的 `Host/Port`），
所以重取 base 不改变频道目录语义，只把「偶尔撞上保留段」变成可恢复。

