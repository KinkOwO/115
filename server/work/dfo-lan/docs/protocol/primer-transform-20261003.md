# 装备库「誓约 / 晶体变换」（CMD2381 PRIMER_TRANSFORM）

> 2026-10-03。用户报告「誓约变换点了没反应」。取证 + 实现 + 待实机回归。
> 本文记录**证据链**与**仍未闭环的缺口**，供后续接手直接引用。

## 0. 一句话结论

装备库（军械库）誓约页签的「变换」按钮发的是 **CMD2381 `ENUM_CMDPACKET_PRIMER_TRANSFORM`**，
服务端**此前完全没有实现**（不在 `observedGameRequest`、没有任何 handler）⇒ 客户端发完就卡在
「期望回包树」上，表现就是"点了没反应、什么都没发生"。本轮按 IDA + 实机帧 + PVF 源表实现。

## 1. 报文几何（IDA 定案，权威 IDB 工作副本）

| 方向 | 事实 |
| --- | --- |
| 发送 | `sub_14150C4D0`（与 **2259 同一个**装备库变换窗口发送函数）`0x14150c901`–`0x14150c92d`：`sub_146D746E0(w,2381)` → `sub_146D75B10(w,buf,203)` → 发送 |
| 初始化 | `sub_1404A7B60(buf)` —— 正文几何的唯一真源，逐字节与实机帧吻合 |
| handler | `sub_145287A40`（注册点 `sub_1452A1F90` 的 `0x1452a3cf6 mov edx,94Dh`），固定读 **6 字节** |
| 行↔槽 | `sub_141518820`：**行 0 = 誓约核心 47**，行 k(1..11) = **槽 35+k = 36..46**；`sub_14150B940` 对 space==3 的晶体调 `sub_14150F150(win, slot-35, item, 3, slot, 0)` 复证 |
| 长度 | 客户端 append **203**；实机明文 **208** = 封包层补零到 8 字节边界（2259 的 121→128 同规律） |

正文（不含 13 字节信封）：

```
[0:13]    客户端从不写（穷举发送方全部栈操作数）⇒ 实机 40 2e cf ee / 01 00 00 00 / ac 00 00 00 / 00
          是**栈残留**，服务端整段忽略
[13:17]   u32 = 窗口对象 win+0x19E4（构造器 sub_1414F1A00 写 1，实机 1）
[17:32]   行 0 记录（15B）= 誓约核心槽 47
[32:36]   u32：发送侧 sub_14206BB60 / 回退 sub_145CD4300（实机 1）
[36:201]  11 条 15B 记录 = 晶体槽 36..46
            +0 u8   space（物品所在容器；`sub_1415141E0` 的 mov [rcx],r8d）
            +1 u16  该 space 内槽号（mov [rcx+4],r9d）
            +3 u32  模板（`sub_1421B2820`）
            +7 u32  `*(u8*)((vtbl+1080)(item)+285)`
            +11 u32 `*(u32*)(elem+0x118)`（行重映射；第二遍按它搬行，实机 -1 ⇒ 空转）
[201]     u8 = 0
[202]     u8 = 调用方第 4 实参（`setState` 传 0）
```

回包（kind 1）：`status(1B) + 6B`。`sub_1459A1BB0` 先吃 `status`，**只有 status == 0** 才再读
u16 提示码，之后 handler 固定读 6 字节 ⇒ **正文至少 7 字节**，短了 `MEMORY[0]=0` 崩客户端。

**窗口选择字节必须发 0**（→ 窗口 **2145 = EquipmentTransformWindow** 本体）：
`sub_14151BAE0(win,1)` → `sub_14150F5A0(win,3)` = `setState(3)` → 弹
DSTR **101039076「Oath/Crystal conversion completed! Moving to the Oath settings.」**
并切到窗口 2144（Oath settings）。发非 0 会走窗口 3937（`sub_1404AE520`，弹 DSTR 101038235
「Extraction complete.」，属另一簇窗口）—— 誓约变换会看起来"没生效"。

## 2. 规则真源（唯一内容真源 = 内层 PVF）

`etc/115lvability/equipmenttransformsystem.cos`（10,334 B，sha256 `b064e241…2511c`）：

```
[need primer materials]     ← CMD2381 用的就是这张
  `rare`      0 25000 | 10401346 5        `unique`    0 30000 | 10401346 6
  `legendary` 0 35000 | 10401346 7        `epic`      0 40000 | 10401346 8
  `primeval`  0 50000 | 10401346 10
[refund primer materials]
  `rare` 0 1 10415190 1 / `rare` 1 0 … primeval 0 1 10415190 100 / `primeval` 1 0
```

列语义由源自身坐实（同文件 `[material list for ui]` 的 `[group] 1` 末项是 `0` 金币占位、
`[group] 2` 末项是 `10401346`；`equipmentsetjournal.cos [create cost]` 同构）：
**`[group] 1` = 金币支、`[group] 2` = 巡礼之印支**；晶体链**没有灵魂项**（装备链才有）。

稀有度名 ↔ 码值：`equipmenttransformsystem.cos [refund materials]` 按 rare/unique/legendary/
epic/primeval 顺序给出的灵魂 `10361512/13/14/15/16`，各自 `.stk [rarity]` 恰为 **2/3/6/4/8**
（源内自证，见 `catalog.TransformRarityName`）。

## 3. 本轮实现（服务端）

| 层 | 位置 | 内容 |
| --- | --- | --- |
| 协议 | `internal/game/protocol/primer_transform.go` | 203..208 字节解析（补零必须为 0）+ 7 字节应答 |
| 规则 | `internal/catalog/equipment_transform_system.go` | 直读三条链的 `[need …]`/`[refund …]`（无 JSON 回落） |
| 领域 | `internal/inventory/primer_transform.go` | 行→槽、按目标稀有度计费、登记+返还被换下去的那件、写穿戴栏 |
| 工作流 | `internal/workflow/item_equipment.go` | `TransformPrimers`（`CommitAccountMaterialEvent`，角色与账号仓同生共死） |
| 网关 | `cmd/wireprobe/primer_transform_flow.go` + `client_dispatch_inventory.go` | 失败也不回 Error（与 2259 同纪律）；成功后补发 NOTI14（穿戴）、NOTI13（背包）、NOTI2610（装备库账本）、账号材料刷新；换掉誓约核心时补 NOTI2839 |

**顺带收敛掉的重复规则（根 AGENTS §0.2）**：CMD2259 的装备变换成本原先**硬编码在 Go 里**
（`walletSoulByRarity` + 常量 `transformGoldCost = 50000`），而源里五档金币是
**25000/30000/35000/40000/50000** —— 旧口径对 rare..epic **一直多扣金币**（只有 primeval 恰好
相等）。本轮两条链共用同一张直读表，常量与映射已删除。

## 4. 未闭环缺口（不得当成已证）

1. ~~**付款方式字段未定论**~~ ✅ **已闭环（2026-10-04，IDA 指令级）**：正文 `+13` **就是支付组号**
   （1 = 金币 / 2 = 巡礼之印），可被 UI 切换、不是常量。见 §4.3。
2. **`[refund primer materials]` 的分支列（0/1）无源内标签**：本轮取**有料的那一支（0）**。
   若实机返还量与玩家预期不符，先怀疑这里。
3. **「军械库 5 件 → 只报 2 件」的过滤判据未闭环**：实机两条非空记录的 `(space 46, slot 0)`
   说明它们来自军械库（非穿戴），但逐件喂行的 `sub_14150B940 / sub_1414F7DA0 / sub_141505960`
   都是**虚函数**（`CodeRefsTo` 全 0），枚举者在军械库列表窗，本轮没追到。
   ⇒ 服务端**不自行筛选**：客户端报什么就按行落什么槽。
4. **成功后必须补发哪些 NOTI 未端到端证实**：handler 内没有任何 C2S 发包（不自回请）；
   面板由 NOTI 承担，已知读长：2839 = 15B（刷窗口 4173）、2842 = 104B（刷 3951/3963）、
   2841 = u16+u32。NOTI2610 的注册点未扫到。本轮的刷新组合是**按 2259 的既有形状推的**，
   需要一次实机确认。
5. **调适（Tune）值与客户端文案的两条细则未实现**：客户端文案 101038948 还写着
   「Tuned Crystals are preserved when converted to the same rarity / If converted to a
   different rarity, the tuning is transferred to a Crystal of the same rarity in another slot,
   and any tuning value that cannot be transferred is refunded as materials.」
   本实现只做到"**调适值留在槽上**"：`equipPrimerCrystals` 除 `Record[2:6]`（模板）外
   **原样保留目标槽的 `Record`**（调适阶段在 `+170`，与 2258 同一映射）。
   未做的两条：①换不同稀有度时把调适值"转移到另一槽的同稀有度晶体"；②转不过去的部分
   "按材料返还"。另外被换下去那件登记进军械库时**只记模板、不带实例 `Record`**，
   所以它的调适值不会跟着走（留在槽上）。⇒ 需要一次实机确认玩家是否在意（当前口径下
   调适值不会凭空消失，只是不会跨槽转移）。
6. `[0:13]` 栈残留的具体来源、`win+0x19E4`（= 支付组号，见 §4.3）之外的窗口字段语义：未定
   （都不影响服务端）。
   ⚠️ 另有一条实机新观察（2026-10-04 00:43:11 会话）：**`+202`（第 4 实参 / `CallerArg`）出现了 `1`**
   —— 此前样本恒为 0（IDA 静态看也是 `sub_14150C4D0(a1, mode, 0, 0)` 的常量 0），而这一帧
   `caller=1` 且 `rows=[0 2] templates=[100401633 100401632]`（两行、目标跨两个槽）。
   疑与"单件变换 vs 整合变换（`[conversion group]` 那条链，文案 101039323）」有关，**语义未闭环**；
   服务端当前**忽略该字段**（只记日志、按行执行），待取证后再决定是否分支。
7. 3937/2145 的**类名**没取到（IDA 传给 `__RTDynamicCast` 的 `off_` 不是 vtable 起点）；
   「2145 = EquipmentTransformWindow」是**行为链证据**（发送方唯一上层调用者就是它的 setState）。

### 4.1 失败分支的形状（**已取证，但本轮故意不用**）

分派器只在 `status == 0` 时才再读 u16 提示码，所以失败应答是 **9 字节**
`{status=0, code=0, payload[0..6)}`（`code = 0` ⇒ `sub_146ADFC80` 不弹提示），
`payload[4]==0 && payload[5]==0` ⇒ `sub_14151BAE0(win, 0)` = `setState(4)` = 窗口的失败态。

**本轮一律回成功形状（7 字节）**，理由是 2259 的既有教训（该链上"先成功再 Error"实测
`exit=0xC0000005`）：这条链的失败分支**从未在实机跑过**，不能拿玩家的客户端当实验场。
代价是"没换成功"时客户端仍会弹成功提示 —— 正常路径下客户端自己会先过滤
（文案 101038947「Not enough Oaths/Crystals registered/owned.」），所以预期不会出现；
真要改，先做一次失败态实机取证，再动这里。

### 4.2 单帧即执行（**已用实机事件日志闭环**）

2259 的「变换 → 确定」是**同一份正文发两次**（客户端本地弹确认框，服务端按指纹识别第二次）。
2381 不是：`runtime/roles_..._20261004_002343_168831_next37/events.jsonl` 里 5 次点击各对应
**恰好一帧 `client_frame id=2381 type=1`**（16:25:47 / 16:25:57 / 16:26:05 / 16:26:14 / 16:26:31），
每帧之后紧跟 `primer_transform_done` + `primer_transform_opened`(2381) + 刷新包
（`_worn_refreshed` id14 / `_inventory_refreshed` id13 / `_journal_refreshed` id2610 /
账号材料与灵魂恢复包），**没有任何第二帧或追加请求** ⇒ 单帧即执行的口径成立 ✔。
（最后一次的正文首 13 字节是 `feffffff…`，正是那个"客户端从不写"的栈残留区，
两种取值都被同一套解析接受 —— 也顺带复证了 `[0:13]` 必须忽略。）

### 4.3 ✅ 支付组号闭环（2026-10-04，IDA 指令级）

正文 `+13` **就是支付组号**（1 = 金币 / 2 = 巡礼之印 `10401346`×N），**可被 UI 切换、不是常量**：

| 环节 | 地址 | 证据 |
| --- | --- | --- |
| 写入正文 | `0x14150c8cb` | `mov eax,[rbx+19E4h]` → `mov [var_203],eax`（`var_203` = 正文 +13） |
| 字段本体 | `win+0x19E4` | 构造器 `0x1414f30fb` 置 1；玩家点 `win+0x12E0` 上的开关 → `sub_141503B70` 写 `(旧值 == 1) + 1`（写入点 `0x141503bba`）⇒ **1 ↔ 2 切换** |
| 唯一性 | — | 0x1414E0000–0x141530000 全函数逐指令位移扫描：`win+0x19E4` 的写者**只有**构造器与 `sub_141503B70` 两处，其余 56 处全是读/比较（`pt07_displacement_scan.json`） |
| 取值口径 | `.cos` 解析器 `sub_14769EAF0` | 直接读 `[need primer materials]` 的 `[group]` 列编号并原样上传 ⇒ **1 = 金币、2 = 巡礼之印**；`[primer material list for ui]` 同列同构 |
| 与 2259 的关系 | — | 两个 opcode 由**同一个发包器、同一条指令**写同一字段 ⇒ 同义，不存在"只给 2259 用" |

⇒ 服务端实现（`primerTransformPayOption`）**直接采信 `+13` 的 1..2**，域外值回落金币并记日志。
产物：`analysis/tmp-primer-transform/ida/pt10_log_sec1.txt`、`pt10_setter_19E4_141503B70*.c`、
`pt11_ctx_toggle_call_141505e73.dis`、`pt12_cos_reader_full_14769eaf0.c`、`pt07_displacement_scan.json`。

**仍未闭环**（不影响计费，记录备查）：`map<group, 材料id>` 两棵树的**填充者**没定位
（只确认了做键与读键两端）；`win+0x19E4` 的写者扫描范围只覆盖 0x1414E0000–0x141530000。

### 4.4 ⚠️ 实机缺陷与修复：**凭空生成晶体**（2026-10-04，用户实机）

**现象**：用户点「变换」可以成功，但**晶体越变越多**，反复点后 11 个晶体槽全满。

**取证**（`runtime/roles_..._20261004_002343_168831_next37/gateway.err`，5 次点击）：

| 点击 | 客户端行 | 本文件旧实现的结果 |
| --- | --- | --- |
| 1 | `rows=[0 1] templates=[617 592] spaces=[46 46]` | 金币 -60000，槽 36←617、37←592 |
| 2 | `rows=[0 2] templates=[600 592] spaces=[3 46] slots=[36 0]` | -55000，槽 **38←592（新造）** |
| 3 | `rows=[0 3 4] templates=[617 617 592]` | -95000，槽 39/40 各多一件 |
| 4 | `rows=[5 6 7]`（全是 46/0） | -95000，槽 41/42/43 |
| 5 | `rows=[8 9 10]` | -95000，槽 44/45/46 ⇒ **11 格全满** |

**根因**：旧实现只把 `journal.Counts[target] > 0` 当"配方解锁"，换上去的目标是**新造**的
（`equipPrimerCrystals` 直接写 `Template`），从不扣掉那件实物；客户端看到军械库仍"有货"
就继续报下一格 ⇒ 每次点击都是净增一件。

**修复口径**：**变换 = 把一件"拥有但没穿上"的实物搬到目标槽**。每行必须先找到一件来源，
找不到就跳过：

| 来源（按优先级） | 消耗方式 |
| --- | --- |
| 背包物品 / 背包装备列表 | 该件 −1（`Amount == 1` 时删条目） |
| 军械库登记 `counts[target]` | 登记 −1（下限 0，保留 0 值条目） |

⚠️ **来源不取"另一个穿戴槽"**（同日收紧）：把已穿戴的晶体搬到别的槽既不是变换，而且
**会变成刷材料的漏洞** —— 搬位会顶掉目标槽那件，那件按源表"登记 + 返还材料"，
于是两块晶体来回搬就能无限刷返还材料（每次只花金币）。所以只认"未穿戴的实物"；
被换下去的那件（`from`）仍可以是穿戴中的，这正是客户端文案
「Crystals in the Armory, the inventory, or equipped slots」的后半句。
（同轮踩到并绕开的坑：清空穿戴槽必须**删条目**，`ReadBag` 明确拒绝 `Template == 0` 的 worn 行。）

同一份请求内两行**不能共用同一件实物**（`primerSourceClaim`），plan 与 prepare 两遍用同一套
规则。被换下去的那件照旧"登记 + 返还材料"（客户端文案 101038948 说的就是这一件）。

**实机序列的修复后表现**（`TestPrimerTransformFlowDoesNotMultiplyCrystals`，真实 PostgreSQL）：
第 1 轮把军械库那件搬上装备栏，第 2..6 轮**全部拒绝**（`目标 … 没有可消耗的实物`）⇒
**不再扣费、不再新增晶体**，日志逐行写明原因。

## 5. 复现与验证

```powershell
# 直读准备（不碰存储、不启动监听）
cd server/work/dfo-lan
$env:DFO_PVF_CATALOGS='journal,create-cost,transform'
& ..\..\..\tools\go\bin\go.exe run ./cmd/wireprobe `
  -pvf-archive ..\client-build\Script.inner.pvf -pvf-check-catalogs
# 期望：PVF equipment transform system prepared: need=5/2/5 refund=10/10/10
```

端到端回归（需要专用测试库，不碰玩家库）：

```powershell
# 先建一个一次性库（例：dfo_lan_selftest_primer），跑完即删
$env:DFO_TEST_POSTGRES_DSN='postgres://dfo_owner:<pw>@127.0.0.1:25438/dfo_lan_selftest_primer?sslmode=disable'
& ..\..\..\tools\go\bin\go.exe test ./cmd/wireprobe/ -run TestPrimerTransform -count=1 -v
```

`TestPrimerTransformFlowIntegration` 覆盖：解码（203 字节）→ 计划 → PostgreSQL 事务 →
回包逐字节 `01 00 00 00 00 00 00` → 刷新包（NOTI14 / NOTI13 / NOTI2610，2610 恰 16444 字节）
→ 落库核对（金币 100000→65000、槽 36 换成目标、旧件进装备库、微光灵魂进账号仓）。
`TestPrimerTransformRejectsPrimevalInEarlySlot` 钉住「太初晶体只能进 44..46」的守卫。

单测：`go test ./internal/game/protocol/ ./internal/catalog/ ./internal/inventory/`。

实机（**由用户手动操作**）：装备库 → 誓约页签 → 点「变换」。日志关键字：
`primer transform: window_key=… rows=… templates=…` 与 `primer transform DONE: pairs=… gold=…`。

## 6. 变更域名的连带影响（已处理，供后续加域参考）

`transform` 是新增的 PVF 候选域，它有三份必须同改的清单：

| 位置 | 作用 |
| --- | --- |
| Go `internal/gamedata/catalogs.go` 的 `SupportedDomains` | 运行期白名单（少一处 → `domain is not enabled`） |
| Python `scripts/repair_profile.py` 的 `allowed` | **启动脚本的 profile 校验**（少一处 → `Invalid PVF candidate domains` → 启动脚本 `exit status 1`） |
| `configs/pvf-default.json` 与 `docs/repair-profile.example.json` | 实际选中的域（少一处 → 该功能在默认启动下不准备） |

`scripts/test_repair_profile.py` / `test_pvf_default_launch.py` 的**域数量断言**（现为 55）是
漂移哨兵：加域时必须一起改，否则本机测试直接红。

## 7. 实机排查决策树（第一次实机回归时按这个走）

按症状定位，**每一步都先看日志再动代码**（日志关键字都在 `runtime/roles_*/gateway.err` 与
`events.jsonl`）：

| 症状 | 先看什么 | 结论 / 下一步 |
| --- | --- | --- |
| 点「变换」完全没反应，日志里**没有** `primer transform:` 行 | `events.jsonl` 里有没有 `"id":2381` 的 `client_frame` | 有帧 = 帧到了但被门禁挡下（查 `verified`；2381 已在 `observedGameRequest`）；没帧 = 客户端没发，问题在客户端侧（界面/前置条件），与本次服务端改动无关 |
| 有 `primer transform: … rows=[…]` 但随后 `REFUSED` | REFUSED 的原因文本 | 逐条：未在装备库登记 / 不是晶体·誓约 / 太初晶体只能进 44..46 / 付不起（金币或材料） |
| `DONE` 了但回执里 `skipped` 非空 | `pairs` 与 `skipped` | 金币是**逐件**判的（见下）；`skipped` 里的模板多半是"**没有可消耗的实物**"（军械库登记为 0、背包也没有）——这是**正常跳过**，不扣费、不生成 |
| 点了没变化（日志里是 `REFUSED: … 没有可消耗的实物`） | 军械库登记 / 背包里到底有没有那件 | 该行没有真实来源 ⇒ 设计上就该跳过；如果玩家明明有那件却报这个，先看它是不是**只穿在身上**（穿戴中的不作为来源，见 §4.4） |
| `DONE` 了但客户端**不弹窗、不切 Oath settings** | 回包字节（`primer_transform_opened` 的 `plain_hex`）与窗口字节 | 回包应为 `01 00 00 00 00 00 00`。若不是 ⇒ 用 `DFO_PRIMER_TRANSFORM_WINDOW=1`（窗口 3937）对照一次；仍不对就回到 IDA 复核 handler 分支 |
| 弹窗正常但**晶体没进槽 / 进错槽** | `DONE` 行的 `pairs`（row/slot/from/to）与 `primer_transform_worn_refreshed`(NOTI14) 是否发出 | pairs 对但客户端没变 ⇒ NOTI14 内容/时机问题（`WornSpaceUpdate` 走 0..47，晶体槽在内）；pairs 的 slot 与预期不符 ⇒ 记录行↔槽映射需按记录的 `(space,slot)` 重新判定 |
| 装备库/背包数字没跟着变 | 有没有 `primer_transform_journal_refreshed`(2610) / `_inventory_refreshed`(NOTI13) | 2610 正文必须恰好 16444 字节；缺包 ⇒ 补发逻辑没走到（账本为空时按设计不发） |
| 客户端**崩溃**（`0xC0000005`） | 立刻停手、保存本次 `events.jsonl` | 这是几何/窗口选择错误或失败分支问题的典型表现。**不要连点重试**；回来按 IDB 复核 `sub_145287A40` 的两支分支与应答长度（7B 成功 / 9B 失败） |

**金币按"逐件"判**：`PreparePrimerTransform` 用**剩余金币**（`live.Gold - 已累计`）逐件比，
不够的那件只进 `Skipped`，不把整批回滚 —— 否则"够换便宜那件、不够换贵那件"会表现成
"点了没反应"（`TestPrimerTransformSkipsOnlyThePricierUnfundedRow` 钉住这条）。2259 那条链
仍是最后一次性比总额（本轮未改，属既有行为）。

诊断开关（都走 profile/命令行，不需要重编）：

- `-primer-transform observe`（`DFO_PRIMER_TRANSFORM_APPLY=observe`）：只记日志、不动存档，
  用来先确认"帧到没到、行/模板是什么"，再切回 `apply` 真正执行；
- `-primer-transform-window` / `-primer-transform-variant`：换窗口字节对照（见上表第 3 行）。

## 8. 相邻发现：源里有、执行器里没有的三条规则（**本轮未改，按 §0.2 主动上报**）

读 `etc/115lvability/equipmenttransformsystem.cos` 时顺带发现的、与本次改动相邻但**没有被任何
执行器消费**的源规则。它们不影响 2381 的闭环，但属于"源已有定义 ⇒ 不应另立 Go 常量"的收敛范围：

| 源段 | 现状 | 影响与建议 |
| --- | --- | --- |
| `[refund materials]`（**装备变换 CMD2259 的返还**） | CMD2259 现在的实现：换下去的源装备**登记进装备库**，但**不返还任何材料** | 源按源装备稀有度返 1 件灵魂（primeval 还多返 10403609 太初星辉）。若不返，玩家每次变换比官服少拿一件灵魂。**属于玩家可见的数值差异**，需一次实机口径确认（`[refund materials]` 的分支列语义同样未闭环）再决定是否接；接的话与 2381 共用同一张直读表 |
| `[need amalgamation materials]` / `[refund amalgamation materials]`（**融合石变换**） | 无执行器（融合石相关只有穿戴/分解） | 融合石变换链完全没接。接入顺序：先按 IDA 定出对应 CMD（`amalgamationstoneconversion.cos [conversion group]` + `[item shop] 100000936`），再复用本文件的直读表 |
| `[tradable soul info]`（**灵魂结晶登记**） | 无执行器 | `10403160..10403164` 是"用 500 ×灵魂 → 可交易结晶"的登记；`500` 的语义源里无标签 ⇒ 未闭环，不接 |

另外两条**已在本轮收敛/保留**的记录，便于后续复查：

- ✅ **已收敛**：CMD2259 的 `[need materials]` 成本口径（原先硬编码 `walletSoulByRarity` +
  常量 50000 金币，源里是 25000/30000/35000/40000/50000）—— 两条链现共用同一份直读表。
- ⏳ **保留**：`internal/inventory/equipment_journal_operations.go` 的 `costGroupFor`
  （把变换目标映射到 `[create cost]` 档位的那段回退逻辑）在本轮之后**已无生产消费者**
  （只剩测试引用）。它写的是"变换要走 `[create cost]`"的旧结论，与现在的源口径冲突；
  建议下次清理时连同 `TestCostGroupFor*` 两条用例一起删（本轮为控制改动面保留）。


