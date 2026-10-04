# 装备库「誓约/套装积分」（Set Point / Oath Point）取证笔记 — 2026-10-04

> 用户现象：「誓约没有积分」——装备库 → 誓约页签里，每个誓约那行显示 `0/750次`，
> 右侧「已添加的誓约积分: 0」；用户补充：**装备上之后已装备的誓约积分也是 0**（也是错的）。
> 截图另外给出：`[稀有 I]: 达成750`、`[稀有 II]: 830（需要830）`、
> 「激活需要 装备套装积分 2550 + 誓约积分 1200」、`誓约的庇护 [750]`、
> 顶部筛选含「誓约积分优先」、右下两个按钮「誓约变换 / 统一变换」。

## 1. 客户端文案口径（DSTR，已确认存在）

| DSTR | 文本 | 含义 |
| --- | --- | --- |
| 101038946 | `Convert Oath/Crystal` | 誓约页签的变换按钮 |
| 101038949 | `No Oaths applied` | 没装誓约 |
| **101038950** | **`Oath Points first`** | 排序口径（截图里的「誓约积分优先」） |
| **101039032** | **`Not enough Oath Points to activate the Set effect.`** | 积分不足、套装效果不激活 |
| **101039033** | **`Activated after reaching 2,550 Equipment Set Points`** | 阈值 2550 |
| 101039073 | `Can be distributed after reaching 2,550 Equipment Set Points` | 同上（分配） |
| 101039075 | `%d Set Point Increase` | 积分增量 |
| 101039257 | `Only the effect of the set with the highest Oath Point total will be applied.` | 只取最高那一套 |
| 101039273 | `…increase the Set Points of your equipped items to 2,550 and the Oath Points of the selected Oath to 1,200.` | 两个阈值 |
| 101039327/101039328/101039369 | `…%d Oath Points will be adjusted as Set Points.` / `Point Display Method (Left/Right)…Right - Displays the Set Points and Oath Points registered in the Armory.` | 左/右两种显示口径 |
| 101037312 | `Converted Set Points` | 「转换得到的积分」 |
| 101037515 | `(%d Set Points required until Primeval)` | 距太初档还差多少 |
| 101037516 | `Displays the higher value between Oath Points and Fusion Stone Set Points.` | 与融合石积分取高 |
| 101036141/142/143 | `Increase Oath Points` / `You can obtain Oaths and Crystals in dungeons. Equipping items from the same set adds up their Set Points.` / `Tune your Crystals to increase their levels, and your Set Points will also increase!` | **积分随「等级/调适」增长** |
| 101038673/101038674 | `Oath Point` / `Common Oath Points count toward the one set with the highest amount of Exclusive Oath Points.` | 通用/专属积分 |

## 2. 服务端缺口（本轮定位到的关键）

**决定性线索**：opcode 全表（`analysis/dumps/opcode_name_to_hex.json`，55 条名字含 point 的包）里，
**没有任何 C2S（CMD）包与"套装/誓约积分"相关**——该族只有两个 NOTI：

| opcode | 名称 | 现状 |
| --- | --- | --- |
| **2634 `0x0A4A`** | **`ENUM_NOTIPACKET_CHARACTER_PART_SET_POINT`** | **服务端从未发送**（Go 侧与文档里 0 命中） |
| **2635 `0x0A4B`** | **`ENUM_NOTIPACKET_ACHIEVEMENT_PART_SET_POINT`** | 同上 |
| 2839 `0x0B17` | `ENUM_NOTIPACKET_OATH_SYSTEM_INFO` | 已发，但载荷 **24 字节里只填了 `[0:4) = option`，其余全 0**（客户端只读 15 字节） |
| 2841 `0x0B19` | `ENUM_NOTIPACKET_GET_OATH_EQUIPMENT` | 未实现 |
| 2842 `0x0B1A` | `ENUM_NOTIPACKET_PRIMER_COLLECTION` | 未实现（读长 104B） |
| 2385/2386/2405/2420/2421/2422 | 晶体合成/碎片兑换/赛季誓约/强化誓约/选强化项/升品 | 未实现 |

⇒ 既然**没有查询用的 CMD**，这两个值只可能是**服务端主动推送**；而我们从没发过
⇒ 这是「积分恒 0」的**首选根因**（次选：客户端按实例调适等级自算，而我们的穿戴行没有 `record`）。

**实机会话佐证（`roles_..._20261004_003928_836060_next37/events.jsonl`）**：整个会话里
**客户端一帧都没发过** 2634/2635/2841/2842（`client_frame` 的 id 分布里只有 2127/283/1706/39/38/585/
623/2423/2381/35/1421/390/2178/433…），而**我们发过的誓约族包只有 2839 与 2838**
（`oath_system_info_restored` / `dungeon_oath_selection_restored` / `primer_transform_oath_info` / `oath_system_grades`）
⇒ 誓约窗口里的积分只可能来自**2839**（我们只填了 `[0:4)`）或客户端本地计算（见 §5.0 的 15 字节几何）。

**找 2634 处理器的快捷路径（给 IDA 侧）**：2610 与 2634/2635 属于同一族
（`EQUIPMENT_SET_JOURNAL_CHARAC_INFO` / `CHARACTER_PART_SET_POINT`），
而 2610 的消费者已知 = `sub_145304380`（读 `0x403C` = 16444 字节，见
`internal/game/protocol/equipment_journal.go` 头注释）。**从 2610 的注册点/id 立即数入手**，
就能找到这条"字符信息族"的注册写法，再据此定位 `0x0A4A` 的处理函数与载荷几何
（`sub_140006430` 那张表里 2839/2841/2382/2383 都能直接看到，但 2610 与 2634 都"不在立即数附近"
—— 两者同族，说明这条族用的是另一种注册写法）。

## 3. 存档现状（角色 1，只读查询）

- 穿戴晶体槽：36=100401633、37=100401592、38=100401632；**誓约核心槽 47=100610065**（有誓约，不是 "No Oaths applied"）。
- **所有穿戴行（含 0..25 的普通装备）都没有实例 `record`** ⇒ 若积分依赖调适等级，则处处为 0。
- state 里没有显式的积分字段（只有 `equipment_journal.counts` 登记、账号选项等）。

## 4. PVF 源侧（已确认存在，待接到执行器）

### 4.1 套装积分表（Service Set Point Table）

```
etc/115lvability/equipmentsetpointtable.lst          ← 索引：<序号> <路径>
etc/115lvability/equipmentsetpointtable/<setid>/0.etc … 20.etc      ← 按「等级/档位」0..20
etc/115lvability/equipmentsetpointtable/<setid>/unique.etc / legendary.etc / epic.etc /
                                              primitive.etc / primitive1..3.etc
etc/115lvability/equipmentsetpointtable/addparameter/<1..35>.etc    ← 追加参数
```

每个 `<level>.etc` 的结构（实读样本 `16201/0.etc`、`16201/20.etc`）：

```
[parameter]
  [index] 0
  [fame value]  200   … 23000        ← 名望
  [skill bonus rate] 4               ← 技能伤害/增益率
  [equipment buff] 960               ← 增益量
[/parameter]
```

⇒ 同一 `(setid, 档位)` 给出该套装的**效果参数**；截图里的「冒险家名望 1650 / 技能伤害 +17% / 增益量 3300」正是这一族字段的合成结果。

### 4.2 誓约/晶体 `.equ` 自身的数值字段（实读样本）

| 模板 | `[rarity]` | `[equipment buff]` | `[fame value]` | `[skill bonus rate]` |
| --- | --- | --- | --- | --- |
| 100401633（青丘 R6） | 6 | **5300** | 2000 | 20 |
| 100401592（R2） | 2 | **4500** | 1400 | 13 |

两者都是 `[grade] 116`、`[equipment type] [primer]`。**注意：与积分表用的是同一组字段名**
（`[fame value]` / `[skill bonus rate]` / `[equipment buff]`），所以"积分"很可能是把这两侧的
数值按套装聚合出来的。

### 4.3 誓约积分表（`etc/115lvability2/equipmentoath/`，本轮新发现）

```
etc/115lvability2/equipmentoath/<setid>_<name>/<tier>/<tier>.etc      ← 每档「需要多少誓约积分」
etc/115lvability2/equipmentoath/<setid>_<name>/<tier>/<rarity>.etc    ← rare/unique/legendary/epic/primitive 的档位参数
etc/115lvability2/equipmentoath/addparameter/<i>.etc                  ← 「oath_expand_i」追加参数
etc/115lvability2/equipmentoath/remainparameter/<i>.etc               ← 剩余参数（140 个）
```

实读样本 `16201_shadow/1/1.etc`（连续 5 个 `[parameter]` 块）：

```
[parameter] [oath point] 750  [basic explain] <37::oath_rare_basic> [/parameter]
[parameter] [oath point] 830  ... [/parameter]
[parameter] [oath point] 910  ... / 990 / 1070 ...
```

**正好对应截图里的「[稀有 I]: 达成750」「[稀有 II]: 830（需要830）」** ⇒ 誓约的档位 = 由
**累计誓约积分**跨过 `[oath point]` 阈值决定，阈值写在源里（不是硬编码）。
`addparameter/<i>.etc` 形如 `[skill bonus rate] i  [equipment buff] i*100`（`oath_expand_ii`），
即"积分/扩张等级 → 增益量"的换算表。

⇒ 到这一步：**客户端算得出的部分（阈值、每档参数、每件 `.equ` 的 buff/fame）源里都有**；
唯一缺的是**"当前累计誓约积分（截图那个 0/750 的 0）"这个数** —— 它只可能来自服务端
（§2 已证：没有任何 C2S 查询包，该族只有 NOTI）。

### 4.4 「部位套装」的绑定字段 = `[part set index]`（本轮新发现，实机装备实读）

| 模板 | 部位 | `.equ` 关键字段 |
| --- | --- | --- |
| 100051399 | jacket/cloth | `[part set index] 16298`、`[grade] 3`、`[rarity] 2`、`[fame value] 33` |
| 100101277 | pants/cloth | `[part set index] 16298`（同一套） |
| 100323436 | ring | `[part set index] 16208`、`[grade] 119`、`[rarity] 3`、`[equipment buff] 10240`、`[fame value] 1100` |
| 100401632/633 | primer（晶体） | **无 `[part set index]`**，但有 `[equipment buff] 5300 …`、`[fame value]`、`[skill bonus rate]` |
| 100610065 | oath（誓约核心） | **无 `[part set index]`**，`[equipment buff] 8693`、`[fame value] 5875`、`[skill bonus rate] 50` |
| 109040581 / 100331747 / 109580406 | weapon / title / avatar | 无 `[part set index]`（走各自机制） |

⇒ **两条链在源里就是分开的**：普通装备靠 `[part set index]` 归入
`equipmentsetpointtable/<setid>/…`（套装积分侧）；晶体/誓约核心没有该字段，归
`equipmentoath/<setid>_<name>/…`（誓约积分侧）。服务端若需要"按部位算积分"，
**必须先从 `.equ` 读出 `[part set index]`** —— 这是现有装备目录里**还没有解析**的字段。

> 另注（实机现状，供判断）：用户当前穿的护甲/首饰多为低档老件（jacket/pants `grade 3 rarity 2`、
> `fame 33`），只有戒指 `100323436`（`grade 119 rarity 3`、`[part set index] 16208`）像 115 档；
> 也就是说**即使套装积分链路修好，装备侧的分数也会很低**——与用户报的"誓约积分为 0"是两件事。

**❌ 该假设已被 pt30 否证（2026-10-04）**：`sub_145F06760` 的另外 5 个调用者是**别的 NOTI 的 handler**
（`sub_145304E60` 149B / `sub_145305680` 111B / `sub_14530CD40` 111B / `sub_14530DDB0` 111B /
`sub_14530F540`），它们从**各自载荷**里取值再调 `sub_145F06760`（例：`sub_145305680:163`
`sub_145F06760(v16, *((u32*)v1+3), *((u32*)v1+4))`）⇒ **部位积分是"每部位一条独立通知"**，
2610 尾部与它无关 ⇒ 实现方向 = **发 2634 逐部位**（而不是改 2610 尾部）。

**⚠️ 曾经的假设（保留记录）：2610 尾部 60 字节可能是"6 × 10 字节的部位积分"**

- 2610 handler 已确认：`memset(v7,0,2048*8); memset(&v7[2048],0,60); sub_146EA0BE0(v7,16444);
  sub_140B96DD0(journalMgr, buf)` ⇒ 前 16384 与我们的实现完全一致，**只有尾部 60 字节的语义待定**；
- `sub_145F06760`（把 2634 的 `u16+u32+u32` 应用到"部位"对象）的调用者 =
  `sub_1452C9840`(2634) **+ `sub_145304E60` / `sub_145305680` / `sub_14530CD40` / `sub_14530DDB0` /
  `sub_14530F540`（都在 2610/2609 地址段）**；
- **60 = 6 × 10**，而 2634 的单部位载荷正好是 10 字节（`u16 部位 + u32 + u32`）。

⇒ 若成立，则"部位积分"**本来就在我们已经在发的 2610 里**，我们却按"5 类 × 3 槽 u32 收藏分组"
写（那是既有实现的口径）——**这可能就是用户看到 `0/750次` 的直接原因**，且修法不需要发新包。
反证：既有"收藏"功能（CMD2264）也是按尾部写的，若尾部实为积分，收藏那条链要一起复核。

**历史佐证（CHANGELOG 2026-09-29 那条就把它标成"未闭环"了）**：

> ⚠️ **已知未闭环（本次不含）**：2610 尾部 5×3 的**语义**在两棵树里答案不同 —— 我们按 `!103` 的
> 8 条 2264 实机样本实现为"按类别的收藏分组"；另一棵树实测客户端按 `(0,3,9,12,6)` 分入 5 个
> **互相独立**的集合、假设为"按稀有度的已登记模板"，并观测"**尾部非空才放行制作/兑换窗口**"。
> 本次**不动尾部**，该分歧另行用一次实机点击做 A/B 判定。

⇒ 也就是说：**这个尾部从来没有被证实过**（当时的 A/B 判定一直没做），而它恰好是"积分/已登记模板"
这类内容的强候选。本次修复必须先把 `sub_140B96DD0`（2610 存储）对尾部的解析钉死，再动实现。

**★ 2634 / 2635 载荷几何（已定，pt27 反编译）**

```c
// sub_1452C9840 —— 2634 CHARACTER_PART_SET_POINT 的 handler
v6 = 0;  v7 = 0;                              // u16 + u64（紧挨，packed）
result = sub_146EA0BE0(&v6, 10);              // 客户端精确读 10 字节
v1 = sub_145F0BFA0(qword_14E683C08, v6);      // 用 v6(u16) 查"部位"对象
...
return sub_145F06760(v1, (unsigned int)v7, HIDWORD(v7));   // 两个 u32 应用到该部位
// sub_1452C5A10 —— 2635 ACHIEVEMENT_PART_SET_POINT：同形（sub_146EA0BE0(&v1,10)）
```

⇒ **每包一个部位，共 10 字节：`[0:2)` u16 部位号 + `[2:6)` u32 值A + `[6:10)` u32 值B**；
2635 是它的"达成值"变体（走 `sub_145F01800(…,0)` 与 `sub_145BF5620`）。

**值A/值B 的落点已定（pt29）**：`sub_145F06760(部位)` = `sub_147435FF0(部位+128, A, B)`，
而后者就是两句赋值 ⇒ **值A → `部位+1872`、值B → `部位+1876`**（u32 原样存）。
**仍未定**：这两个字段喂给哪个显示（从而知道它们叫 Set Point 还是 Oath Point / 当前值还是达成值），
以及 `qword_14E683C08`（2485 个函数引用的"部位管理器"单例，FNV 哈希表 `u16→部位对象`）里
**合法部位号的枚举**（含不含誓约/晶体那一档）。
**待定**：两个 u32 的语义（Set/Oath？当前/达成？）→ 看 `sub_145F06760` 写部位对象的哪个偏移；
**部位枚举** → 看 `qword_14E683C08` 单例与 `sub_145F0BFA0` 的键表（`sub_145F0BA60` 一侧）。

**2634 / 2635 的处理函数已定位**（pt26：按 `noti_insert_1459A3DD0` / `cmd_insert_1459A2FB0`
两种注册惯用法全量扫描 1071 个注册点、726 个 id 后命中）：

| opcode | 注册点 | **handler** | 说明 |
| --- | --- | --- | --- |
| **2634 `0x0A4A`** | `0x1452fac1f`（`sub_1452F9420`） | **`sub_1452C9840`** | 载荷几何见上（10B/包） |
| **2635 `0x0A4B`** | `0x1452fac36`（`sub_1452F9420`） | **`sub_1452C5A10`** | 同形，达成值变体 |
| 2610 `0x0A32` | `0x14531509b`（`sub_1453140E0`） | `sub_145304380` | 装备库（已知，27 行） |
| 2609 `0x0A31` | `0x1453150b2` | `sub_1453044D0` | 装备技能栏 |
| 2640 `0x0A50` | `0x145315011` | `sub_145310900` | —— |
| 2839/2841/2842 | —— | 不在 insert 表里（在 `sub_140006430` 那张表） | 2839 handler = `sub_1405757F0` |

> 注册函数 `sub_1452F9420` 用的是"**索引**注册"（`sub_1459A3DD0(tbl, index, handler, 0)`），
> 索引与 opcode **不是**连续对应（这也是早先"按立即数扫 opcode"失败的原因）；`0x0A4A/0x0A4B`
> 的索引在 `0x1452fac1f/0x1452fac36` 附近的调用点里给出。
> 辅助函数（pt26/27 反编译）：`sub_1471B49B0`（稀有度**排序**映射 `{0:0,1:1,2:2,3:3,4:6,5:4,6:5,7:7,8:8,*:9}`）、
> `sub_14500CA50`（id→group）、`sub_1401B6FE0/6FC0/7000`（2839 记录字段写入器，写 `+28/+32/+24`）、
> `sub_145CDE2E0`（oath entry get）。

**2839 载荷几何（已定，代码已按此注释 + 测试钉住）**：客户端解析器 `sub_1474AFDC0` 逐字段拷贝
**15 字节**，构造器 `sub_1474A5170` 把对象 `+0..15` 清零：

| 线上偏移 | 目标 | 宽度 | 我们的现状 |
| --- | --- | --- | --- |
| `[0:8)` | `obj+0` | u64 | 低 4 字节 = option（已实机确认） |
| `[8:12)` | `obj+8` | u32 | **恒 0** |
| `[12:14)` | `obj+12` | u16 | **恒 0** |
| `[14]` | `obj+14` | u8 | **恒 0** |

⇒ 我们发的 24 字节里**只有 `[0:4)` 有值，其余 11 字节全 0**。用户看到的「誓约积分 0」很可能
就是这 11 字节之一（誓约积分/套装积分/等级）。

- 存储侧 `sub_14057D140` 用游标读 `sub_145CDE2E0`(u32) / `sub_145CE2370` / `sub_145CE8870` /
  `sub_145CE8980` / `sub_145CE46F0`，并有一段"按 grade 挑誓约物品"的比较
  （`sub_1471B49B0` / `sub_14500CA50`）；2839 handler = `sub_1405757F0`。
- `101038673 Oath Point`（DSTR 指针 `0x605BA51`）被 **6 个函数**引用：`sub_141495D40`、
  `sub_141507550`、`sub_14150E180`、`sub_14151CF40`、`sub_1430A55B0`、`sub_1468A4050`
  ⇒ 这些是积分**显示/格式化**的落点；`101038674 Common Oath Points` 只在 `sub_1430A55B0`；
  `101039257 highest Oath Point total` 在 `sub_1468A4050`；`101038950 Oath Points first`
  在 `sub_140502A40`；`101038949 No Oaths applied` 在 `sub_14151A500`。
- **`2550` 是客户端硬编码**：`0x1411cb6bd sub_141192BC0` 里 `mov cs:dword_14E645CD8, 9F6h`
  （另有 `[rcx+rax*4+9F6h]` 数组读）⇒ 「太初套装积分阈值」不在 PVF 里。
- 2610 的消费者确认 = `sub_145304380`（两处 `mov edx/ecx, 403Ch`）—— 作为找 2634 的"同族注册写法"跳板。
- 已反编译待解读：`sub_1474AFDC0`(2839 解析) / `sub_1474A5170`(2839 ctor) / `sub_14057D140`(2839 存) /
  `sub_14053A280`(窗口 4173 刷新) / `sub_1415D4A00`(装备库誓约 UI) / **`sub_141510130`(积分读取)** /
  `sub_140575610`(CMD2382 誓约系统信息)。


- `101038673 Oath Point`（DSTR 指针 `0x605BA51`）被 **6 个函数**引用：`sub_141495D40`、
  `sub_141507550`、`sub_14150E180`、`sub_14151CF40`、`sub_1430A55B0`、`sub_1468A4050`
  ⇒ 这些是积分**显示/格式化**的落点；`101038674 Common Oath Points` 只在 `sub_1430A55B0`；
  `101039257 highest Oath Point total` 在 `sub_1468A4050`；`101038950 Oath Points first`
  在 `sub_140502A40`；`101038949 No Oaths applied` 在 `sub_14151A500`。
- **`2550` 是客户端硬编码**：`0x1411cb6bd sub_141192BC0` 里 `mov cs:dword_14E645CD8, 9F6h`
  （另有 `[rcx+rax*4+9F6h]` 数组读）⇒ 「太初套装积分阈值」不在 PVF 里。
- 2610 的消费者确认 = `sub_145304380`（两处 `mov edx/ecx, 403Ch`）—— 作为找 2634 的"同族注册写法"跳板。
- 已反编译待解读：`sub_1474AFDC0`(2839 解析) / `sub_1474A5170`(2839 ctor) / `sub_14057D140`(2839 存) /
  `sub_14053A280`(窗口 4173 刷新) / `sub_1415D4A00`(装备库誓约 UI) / **`sub_141510130`(积分读取)** /
  `sub_140575610`(CMD2382 誓约系统信息)。

### 5.1 必须回答的问题清单

1. **谁算积分**：2634/2635 是**服务端算好下发**，还是客户端按 4.1/4.2 自己算？若服务端下发，
   请给出 2634 的**载荷几何**（哪些"部位"、每部位几字节、Set Point 与 Oath Point 各在哪一格、
   何时发）。
2. **每个部位/每件装备贡献多少分**：是 4.2 的 `[equipment buff]`，还是由 4.1 的表按
   `(setid, 调适等级)` 反查？4.1 里的 `0..20` 档位到底对应"调适等级"还是"积分区间"？
3. **750 / 830 / 2550 / 1200 四个数字的来源**（PVF 常量？哪张表？），以及 2839 载荷里
   与誓约积分相关的偏移（我们目前全 0）。
4. **实例 `record` 的角色**：装备/晶体/誓约核心的调适等级若必须落库，`Record` 的哪一偏移
   （2258 那套是 `+170`），谁写、什么时候写。

> 取证手段：IDA 工作副本（`D:\ida-work-primer\`）+ 本笔记 §1 的字符串 ID 作锚点；
> 客户端外层 PVF 只读扫描脚本见 `analysis/tmp-primer-transform/scan_points*.py`（gitignored）。

## 6. 诊断实验方案（**不改包长，用现成 `-oath-inject`**）

现成入口 `-oath-inject`（`cmd/wireprobe/oath_probe.go`）本来就能"注入任意 NOTI + 逐字节覆写偏移"，
而且队列**每进一次副本发下一条**（`w.oathNext++`）⇒ 一次服务启动即可逐格扫描。

**为什么低风险**：2839 我们**本来就在发**（登录/入场 + 誓约选择时），载荷长度也**不变**（注入用 256 字节，
客户端只读 15）；历史上唯一一次崩溃是"**给得太短**"（8 字节），与本次做法相反。

**一次启动、六条候选**（`DFO_OATH_INJECT`，环境变量会原样透传给服务端：`launch_local.py`
的 `launch_environment` = `os.environ.copy()` + profile 覆盖）：

```
2839:256:0;8:210;9:4      →  [8:12)  = 1234     （结构体 u32）
2839:256:0;12:77          →  [12:14) = 77       （结构体 u16）
2839:256:0;14:9           →  [14]    = 9        （结构体 u8）
2839:256:0;4:210;5:4      →  [4:8)   = 1234     （u64 的高半）
2839:256:0;15:210;16:4    →  [15:19) = 1234     （**结构体之后的游标字段**）
2839:256:0;19:210;20:4    →  [19:23) = 1234     （同上）
```

> 后两条的理由：客户端存储函数 `sub_14057D140` 在解析完 15 字节结构体后，还用游标继续读
> `sub_145CDE2E0`(u32) / `sub_145CE2370` / `sub_145CE8870` / `sub_145CE8980` / `sub_145CE46F0`
> ⇒ **积分也可能在 15 字节之后**（我们发的 24 字节里那段同样是 0，且 15 字节以下都填过一遍没关系）。

**每条的步骤**：进一次副本（注入发生在副本加载应答）→ 回城 → 打开 装备库 → 誓约页签 →
记录 `0/750次` 与「已添加的 誓约积分」这两个数字（以及右上的 `[稀有 I/II]`）。

**判定表**：

| 观察到 | 结论 |
| --- | --- |
| 某一条候选让 `0/750次` 的分子变成该候选的数值（1234 / 77 / 9） | 该偏移 = **誓约积分**（累加点数）⇒ 服务端按此填写并落库 |
| 让「已添加的誓约积分」变成该数值 | 该偏移 = **本次/该誓约新增的点数**（可能是"折算值"） |
| 让 `[稀有 I/II]` 或激活提示变化 | 该偏移 = **档位/标志位**，不是点数 |
| 四条都毫无变化 | 积分**不由 2839 下发** ⇒ 改查"客户端按装备实例（调适等级）自算"那条路（见 §5.1 第 4 条） |

## 7. 收口版结论与实现方案（2026-10-04，pt27–pt35 + PVF 直读）

### 7.1 客户端要什么（全部为"服务端主动推送"，客户端不回请）

| 包 | 形状 | 作用 | 我们的现状 |
| --- | --- | --- | --- |
| 2610 | 16444B = 2048×(u32 模板,u32 份数) + 尾部 5 组×3 u32 | 装备库登记 + 收藏的**套装号**（`[part set index]`） | ✅ 已发，口径正确 |
| **2841** | `u16 部位 + u32 物品`（6B） | 「把某件誓约装备装进某部位」（截图右侧 3 格） | ❌ 从未发送 |
| **2634** | `u16 键 + u32 A + u32 B`（10B） | **每角色一对 Set Point / Oath Point** → `实体+1872/+1876` | ❌ 从未发送 |
| 2635 | 同形 10B | 达成值变体（走 `sub_145BF5620` 生成 UI 元素） | ❌ 从未发送 |
| 2839 | `u64@0｜u32@8｜u16@12｜u8@14`（15B） | 誓约"选择状态"；记录 +0..+14 **无任何本地写入** ⇒ 服务端独占载体 | ⚠️ 只填 `[0:4)` |

关键判据：2634 的 u16 **不是装备部位**，而是**角色实体键** —— 客户端只在
`sub_145F0BFA0(reg,id) == sub_145F0BA60(reg)`（自身实体）时应用；另外 5 个同族包
（`REQUEST_PEER` / `REMOTE_PARTY_JOIN_REQ` / `RAID_OTHER_CHANNEL_REQUEST_JOIN` /
`EXPANDING_PARTY_MATCHING_*`）把**同一对 `+1872/+1876`** 写在**队友实体**上 ⇒ 它是**每角色**的属性对。

### 7.2 积分数值从哪来（**已拿到，PVF 直读**）

```
etc/115lvability/setpointinfo.cos      816 行 文本  [set point] [table]
    [info] [group] <g> [awakening] <a> [part set index] <p> [value] <v>
etc/115lvability2/oathpointinfo.cos    418 行 文本  [oath point] [table]
    <模板> <awakening> <part set index> <积分>
```

实读：`setpointinfo` 的 `group 55/awakening 0/-1 → 65`、`55/1/-1 → 75`、`55/2/-1 → 85`、
`55/3/-1 → 95`、`group 51/0/-1 → 115`；`oathpointinfo` 的 `100313750 0 -1 265`、
`100610095 0 -1 355`、`100313752 0 16201 265`、`100610042 0 16201 355`。
=> **未调适（awakening 0）也有分** ⇒ 服务端可直接按"装备/晶体的模板 + 调适档位 + 套装号"查表求和。

档位效果表：`equipmentsetpointtable/<setid>/<row>.etc` 的 `[basic explain]` 里写着档位阈值
（`1.etc → set750`、`10.etc → set1700`、`13.etc → set2000`、`14.etc → set2100`、`15.etc → set2200`），
行内是该档的套装效果参数（fame / skill bonus rate / equipment buff / equipment damage）。
`2550` 是客户端硬编码（`dword_14E645CD8`）。

### 7.3 最后一个未知：2634 的 u16 键

- 客户端自身键 = `sub_146E920A0(&dword_14EF2CA00, &v16)` 推出的
  `v16 = (*dword_14EF2CA00 ^ 0x1F2A025C) - 4`（u16 使用），并与成对全局
  `dword_14EF2CA04 == v16 + *dword_14EF2CA00 + 196` 做校验；
- **静态侧没有部位号的枚举表**（`reg+72` 的有序链表由 `sub_145F0AB10` 运行期创建 272 字节对象，
  全簇 300 个函数 + FNV 素数全站点扫描都没找到静态 u16 表）⇒ **服务端不能自造键**；
- 待定：`dword_14EF2CA00` 的写者（若来自服务端下发的 actor id，就能算出同一个键）。
  ⇒ 若静态不可得，用**低风险实机差分**收尾：2634 的键不匹配时客户端**直接忽略**
  （handler 里先查表再判 `== 自身`），所以"发错键"不会崩、不会污染，只是没反应 ——
  可以拿服务端自己分配的 actor id 试。

### 7.4 实现方案（下一步）

1. `internal/catalog/` 新增两份源表读者：`SetPointInfo`（按 `group/awakening/part set index`）
   与 `OathPointInfo`（按 `模板/awakening/part set index`），走 `gamedata.Source` + `PrepareCatalogs`，
   **纯 PVF 直读、无 JSON 回落**；
2. 聚合角色总分：按客户端文案（"Only the effect of the set with the highest **Set Point total**"
   / 101038674 的 Common vs Exclusive）取"最高那一套/誓约"的合计；
3. 推 **2634**（`u16 键 + u32 SetPoint + u32 OathPoint`）：入场、换装、变换/登记后各一次；
4. 视需要补 **2841**（誓约装备 ∈ 部位）与 **2839** 尾部字段（语义仍未定，暂不动）。

### 7.6 ⚠️ 实机更正（2026-10-04）：装备库→誓约 页签**不由 2634 驱动**

用户截图（装备库窗口）显示的两处数字**仍为 0**，且**与 2634 的字段顺序无关**：

- 左栏每个誓约一行：`和谐之青丘誓约 0/750次` + 小字「誓约积分」；
- 右栏（装备栏→誓约）：3 格（幻影 / 凝华 / 集聚）+「**已添加的 誓约积分 : 0**」；
- tooltip：`[稀有 I]：达成750`、`所需装备套装积分 2550`、`所需誓约积分 1200`、`誓约的庇护 [750]`。

两次实机（`A=0/B=670` 与对调后的 `A=670/B=0`）该界面**都是 0** ⇒ 这一页**既不读 `实体+1872`
也不读 `+1876`**。同一批次里 `NOTI2634` 确实生效（角色面板另一处的数字随它变化），说明包与键都对，
**但用户要修的就是这一页** —— 因此 §7.5 的"完成"口径需要按此更正。

**已排除**：`2839` 只填 `[0:4)`、`2841`（键 `0xFFFF`，4 件穿戴誓约装备）实机发送后**该界面无变化**
（2841 的发送代码已按 §0.3 回滚，几何留在本文档）。

**待定的三个候选机制**（交给 pt5x 取证，不再猜键）：
1. 客户端**自己现算**：读"已登记（2610）/已装备"的誓约装备 + `oathpointinfo.cos`/`setpointinfo.cos`
   ⇒ 若是，则"已添加"的 0 说明**输入集合为空**（用户登记表里确实没有誓约装备：只有
   `100323436/100354160/600/617/628`）；
2. 由某个**服务端报文字段**驱动（最可能是 `2839` 那 15 字节里除 `[0:4)` 之外的载体，
   或 `2634` 之外的另一个 noti）；
3. 存在**"已添加"的累加点数池**（例如每次「誓约变换」CMD2381 往池里加分）⇒ 若成立，则
   本服务端的变换实现**漏了加分**这一步。

### 7.7 顺带定位到的"输入集合为空"（2026-10-04 第二轮）

§7.6 候选 1 说"该页读的是**登记在装备库里的东西**"，那么**登记表里必须真有誓约/晶体**它才可能非 0。
用户 07:31 登记的却是**普通装备**（`disjoint_journal_added added=2` / `added=1`，见
`equipment-journal-handoff-20261004.md` §1 的会话 `..._20261004_152634_119390_next37`）；
而 `oathpointinfo.cos` 只对**誓约/晶体**模板有行 ⇒ **即使该页的逻辑完全正确，它也只会显示 0**。

⇒ 这条给下一次实机的**最小验证步骤**（不需要改代码、不需要发新包）：

1. 用 CMD26 分解一件**晶体**（如 `100401592` 那种 `[primer]`）或一件**誓约核心**（`[oath]`），
   让它在图鉴里出现（登记成功时日志有 `disjoint_journal_added added≥1`、随后 `equipment_journal_restored`）；
2. 打开 装备库 → 誓约页签，看每行 `?/750次` 与「已添加的 誓约积分」；
3. 判定：**出现非 0** ⇒ 该页就是"按登记表现算"，本服务端无需再发包（剩下的只是 counts/套装号口径）；
   **仍是 0** ⇒ 把候选 2/3 交给 IDA（重点：`sub_14151A500` / `sub_1468A4050` / `sub_141510130`
   这几个读 `101038673 Oath Point` 的显示函数，看它们从哪里取输入）。



| 落地 | 位置 | 说明 |
| --- | --- | --- |
| 源表读者 | `internal/catalog/point_rules.go`（+ `_test.go`） | 分段解析 `[set point] [table]` / `[oath point] [table]` / `[grade list]` / `[min oath point]`；真实内层归档装载计数 **set=88 / grades=21 / oath=239 / minOath=1200** |
| 查表 | `SetPointFor` / `OathPointFor` / `GradeFor` | `[add parameters]` 段的裸 `[part set index]` 与数字列表不会污染结果（有专门测试） |
| 聚合 | `PointRules.OathPoints` | 精确行 →（套装号≠-1 时）通用行 →（源里没有该档位时）**回退 0 档基础分** →（模板,档位）唯一行；**多行且都非 -1 时跳过**（归属不确定就不瞎算）。`SetPoints` 仍是记录在案的缺口（类别号映射未定），恒返回 `(0,0)` |
| 调适档位 | `oathPointItems` 读实例行 `Record[170]` | 与名望计算 `internal/character/fame.go` 同一格（181 字节记录 +170 = 实例阶段）；无实例 record 的老件按 0 档（源里 0 档同样有分） |
| 接入 | `Source.PointRules()` → `Catalogs.Points`（与 `transform` 同域）→ `ItemService.Points` | 装载失败**显式报错**，不静默降级 |
| 推送 | `cmd/wireprobe/oath_point_flow.go` + `client_entry.go` + `entry_flow.go` | 入场先发 **NOTI2841**（`u16 键 + u32 模板`，6 B/件：把穿戴的誓约装备归位；用户选择"先试一下"的实验项，语义未完全闭环），再发 **NOTI2634**（10 B：`u16 键 + u32 OathPoint + u32 SetPoint`）；两者都**排在所有帧之后**（`actor_appearance_ready` 会重建实体）；积分表未装载时不发 2634，不用 0 冒充 |
| 候选键 | **已收窄为 `oathPartSetPointKey = 0xFFFF`** | 实机用"每把键带可区分数值"的探针跑一次，誓约页签出现 **670**（= 下标 0 的值）⇒ 键确认为 `0xFFFF`（客户端"自身部位对象"用印章解码明文建键，初始化明文即 0xFFFF，`sub_14005D4C0`/`sub_14023D2F0`）。临时探针（258 键 + 高位指纹）已按 §6 删除 |

**角色 1 的预期值（离线按真实源表算）**：晶体 `100401633`(125) + `100401592`(45) +
`100401632`(85) + 誓约核心 `100610065`(455) = **710**（第一档阈值 750 ⇒ 显示应接近 `710/750`）。
该值已由 `TestOathPointsRealCharacterOne` 从真实源导出断言（不是手算）。

**★ 实机验证（2026-10-04，用户回报）**：用候选键 `[0xFFFF, WireID, 0, 2, 100]` 各发一条
（载荷统一为 `A=0 / B=670`），用户观察：
- 角色/装备面板的「**套装积分**」显示 **670** ✔ ⇒ **推送生效、键命中、写入路径正确**；
- 装备库→誓约 页签的「**已添加的 誓约积分**」与每行 `?/750次` 进度**仍为 0** ✗。

⇒ 两条硬结论：
1. **载荷字段顺序与我最初的假设相反**：`u32 A`（落 `实体+1872`）= **OathPoint**（誓约页签读它），
   `u32 B`（落 `实体+1876`）= **SetPoint**（"套装积分"读它）。修法 = 两个值对调。
2. **装备库那一页不是读角色实体这对值**（它读"登记在装备库里的内容" —— 客户端文案 101039328
   `… registered in the Armory` 一致），因此对调后仍需单独确认它由什么驱动（见 §7.6）。

`"id":2634` / `"id":2635` / `"id":2841` **0 命中**（服务端历史上一次都没发过这三个包），
同一检索下 `"id":2839` 有 12 条命中 ⇒ 日志确实记录了誓约族出站帧，不是漏记。
配合"客户端不自己算总分"的反编译结论 ⇒ **「誓约积分恒 0」的根因就是这三个包从未发送**。
