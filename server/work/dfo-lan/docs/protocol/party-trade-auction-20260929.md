# 组队/邀请/交易/拍卖 协议取证与第一期实现 — 2026-09-29

> 目标：修复 组队、邀请、拍卖、交易。
> 方法：IDA 9.4 无头探针（`E:\115us\ida-work\r1..r6_probe.py`，产物 `r1..r6.out`、
> `r2-decomp\`、`r4-decomp\`、`r5-decomp\`、`party-funcs-325\`），复用既有资产
> `analysis/dumps/opcode_table_detailed.json`（5,330 条 CMD/NOTI 全表）。
> 数据库：donor 2.38.3.25（`E:\115us\DFO.exe.i64`）；关键函数的部署版一致性走
> `bytecmp.py` 64 字节滑窗（探针R6 导出字节）。

## 一、本轮最重要的结构性发现（全新，此前所有文档都没有）

### 1. NOTI 处理器有**两张注册表**（本轮核心发现）

**A. 尾调桩注册表**（r1.out T2a 反汇编实证）——新版命令用：

```asm
mov rcx, cs:qword_14E66C090     ; net 单例
lea r8,  sub_146924880          ; 处理器
xor r9d, r9d
mov edx, 348                    ; NOTI id
add rsp, 28h
jmp  sub_14599D5D0              ; 注册器
```

全库扫描：1,166 个注册点 → **1,134 个 NOTI 处理器**（r2-maps.json）。

**B. 老核心静态注册表 `sub_1452F9420`**（探针R7 顺着 N9 解析器包装器
`0x1452DC3D0/0x1452DC430` → `sub_1459A4410` → 主循环 `sub_1459968C0` 追到）：
一个约 280 项的函数，直接 `sub_1459A3DD0(a1, <id>, <handler>, 0)` 静态注册。
**N9 / N15-18 / N10 / N697/698(S→C) / N1523 等老核心全部在这张表里**（R5 在 A 表里
零命中正是这个原因）。完整清单：`ida-work/old-noti-registrations.txt`。

两条表合并后，目标族处理器全部落位。

### 2. C→S 发送点全图

`sub_14668C520(net, CMD, 0, 0)`（请求包构建器）全库调用点 8,659 个 / 5,713 函数，
恢复 edx 立即数得到 **1,852 个不同 CMD 的发送函数**（r2-maps.json `cmd_senders`）。

### 3. 部署版一致性（探针R6 + bytecmp）

21 个关键函数（N9 解析器 28,156 字节、全部组队/拍卖发送方、邀请处理器、注册器、
请求构建器）对部署版 2.38.2.34 的 DFO.exe 做 64 字节滑窗比对：**全部 100% 命中**
（`bytecmp-party.py` 输出）。donor 库推导的所有布局对部署版原样成立。

### 4. 客户端网络两层模型的边界确认

- **net 层**（`sub_14668C520` 建包）命令的体由调用点随后写字段，可静态还原；
- **请求对象池层**（`sub_144B99B30(pool,族)` + `sub_145C01600` → 目标 vtable+7560 提交）
  序列化在请求对象类内部。C697 族已证走这一层：`_RTDynamicCast(..., &off_14E0A7348)`
  后 `sub_141CBD150(obj, mode)` 只写 `*(u32*)(obj+4)` = 子类型（r4-decomp/t1_*.c）。
  与装备继承（equipment-inherit-chain-20260928.md）结论一致：**这类体的布局以实机
  明文样本定盘，不静态猜测**。

## 二、组队/邀请（第一期已实现 + 待样本项）

### 邀请握手的 S→C 侧已完整闭环（本轮还原）

```
A(队长)邀请 B   →  A 客户端发 C697 族（子类型=请求对象+4，C→S 体待样本）
服务端 → B      →  N697 { u16 actor=A }            「有玩家邀请你」弹窗
                   （名字由客户端按 actor 从本地数据补齐，弹窗文案 dstr 1137，
                    弹窗函数 0x1416D53E0）
B 接受          →  B 客户端发 C697/C698 族（体待样本）
服务端 → A      →  N698 { u8 0(接受), u16 actor=B }  「B 接受了你的邀请」
服务端 → 双方   →  N9 名册
B 拒绝          →  服务端 → A: N698 { u8 非0, u16 actor=B }
踢人            →  服务端 → 被踢者: N10 { 槽位, 1 }；其余成员: N10 { 槽位, 0 }
```

弹窗本身（邀请函数 0x1416D53E0，dstr 1137）已由探针R8 的全库 dstr 扫描定位；
R8 同时证明 dstr 624（"accepted invitation"）在本构建没有直接取用点——
接受提示由 N698 驱动，与上面的链路一致。

### 已证 opcode 表（analysis/dumps/opcode_table_detailed.json）

| 包 | 名字 | 状态 |
| --- | --- | --- |
| C12 | SET_PARTY_INFO | 体布局 moon 已证（u16 + u32名长 + 名 + 30字节尾，尾+9=容量/类型位）；**第一期已实现**（创建） |
| C13 | LEAVE_PARTY | 体全零（moon 同证）；**第一期已实现**（离队+名册广播） |
| C14 | WALKOUT_PARTY_MEMBER | 踢人；目标 actor 预计体首 u16（待样本）；**第一期已实现**（容忍解析） |
| C697/698/699 | ENTRY_INTO_PARTY / FINISH / … | 邀请/入队握手；子类型=请求对象+4 u32（r4）；**C→S 体待实机样本**；S→C 对应 N697/N698 已还原并实现 |
| C411/2152/2153 | INVITE_MEMBER_FOR_GROUP / PARTY_MATCHING | 待样本（白名单已加） |
| C24 | SET_ITEMTRADE_STATE | 交易状态机；**待样本**（白名单已加） |
| C182-190 | AUCTION 族 | 拍卖全套；**待样本**（白名单已加） |
| C334/629/985/986/1841-1844 | 拍卖购买/传说/批量 | 待样本（白名单已加） |
| N9 | PARTY_INFO | 解析器 `sub_1452F2620`（28,156 字节）已整体反编译（r2-decomp/n9_parser_*.c，5,693 行）；字节比对 100% |
| N10 | 踢人/离队通知 | 老核心表槽10；处理器 `sub_1452ECD30`：`u8 槽位, u8 类型`（0=队友被移出 dstr633、1=你被移出 dstr631）。**已实现** |
| N697/698 | ENTRY_INTO_PARTY / FINISH（S→C） | 老核心表槽697/698；`sub_1452E0710`：`u16 actor`（入队请求弹窗）；`sub_1452E1B40`：`u8 结果(0=接受) + u16 actor`。**已实现** |
| N348/349 | INVITE_MEMBER / LIST | 处理器已反编译，布局已还原（r2-decomp/noti348_*.c） |
| N645/646/647 | ENTRY_PARTY_WAIT / FINISH / UPDATE | 处理器已反编译，布局已还原 |
| N15 | CHANGE_ITEMTRADE_ITEM | `sub_1452C7900`：内嵌 181 字节物品行 + u32 槽位 + 多字段（逐行还原见下轮） |
| N16 | CANCEL_ITEMTRADE | `sub_1452C6600`：`u16, u16, u8, u16`（4 个字段的小载荷） |
| N17 | STATE_ITEMTRADE | `sub_1452E8690`：`u16 actor, u8 state`（对方已确认状态） |
| N18 | FINISH_ITEMTRADE | `sub_1452D08C0`：`u16, u16, u16`（3 个 u16 的小载荷） |
| N1523 | PARTY_MEMBER_READY | 老核心表槽1523：`sub_1452CB740`（73 行，待下轮还原） |

### N9 PARTY_INFO 解析器语法（sub_1452F2620，读包序列带字节偏移见 n9-read-offsets.txt）

```
u16 flag           !=0 时进入成员组循环
u16 mode           9999 = 「无队伍/已解散」分支
循环体（每成员组 8 字节头）：
  u16 v809 / u8 idHi / u8 idLo   → 组键 v13 = v801 + (v802<<16)（沉月湖=频道号）
  u8 v906 / u8 memberCount(<8) / u8 v881 / u8 v850
  memberCount<=1 → 内联一条 42 字节成员记录（off12..53，含 u8 名长槽、
                   名长=0 时名取 dstr 626 "No Party name"、u16/u32 属性字段、8×u8 槽数组）
  memberCount> 1 → 跳过内联记录，走 per-slot 变长段（u8 v832 段数；
                   每段 {u8 槽号, u16 actor → v1089[槽], 5×u8, 2×u32, …,
                   u32/槽 → v1084[槽], 内层 count 循环读 u32…}）
后续段：u8 v833 循环（off102..115）、尾部 10×u8+u32（off116..128）、
        v54==3/5 分支、u32 计数 + 每行 u16×2（off131..）等
```

**单人形 99 字节块（`protocol.SoloPartyInfo`）继续通过解析器**（既有实机证据）；
**多人形的 per-slot 变长段含按数据的嵌套计数循环，纯静态无法闭环——等实机双开样本**。
样本落地前，成员变更仍按每人单人形 N9 广播（moon 既有模型，见「边界」）。

### N348 INVITE_MEMBER 载荷（处理器 sub_146924880 读取顺序 1:1）

```
u8  kind     100/102 → 整包忽略
u8  keyLo
u32 memberID
u8  extra    (v26 高位)
extra × u8
```
成员名先以 dstr 669（"Nameless"，GameCoreNetwork.cpp）占位——即邀请名单的
「未加载信息」形态。已实现 `protocol.PartyInviteMember115`。

### N645/646/647（处理器 sub_144647F50/CB0/DF0 读取顺序 1:1）

- N645：`u16 self, u32 count, count × {u16 actor, u8, u8, u32, u32}`
- N646：`u16 actor, u32, u32`
- N647：`u32 count, count × {s32 id, u8 state}`

已实现 `protocol.PartyEntryWait115 / PartyEntryFinish115 / PartyEntryUpdate115`
（含单测 `party_notice_test.go`，断言镜像读取顺序）。

## 三、第一期实现（本轮代码变更）

| 文件 | 内容 |
| --- | --- |
| `cmd/wireprobe/party_hub.go` | 城镇组队枢纽：固定槽位（不压缩）、队长=槽位序、按频道分组；创建/加入/离队/踢人（带槽位）/解散/快照广播 |
| `cmd/wireprobe/party_flow.go` | C12 创建（moon 证布局容忍解析）、C13 离队（ack + 双向名册）、C14 踢人（队长校验 + N10 双型通知）、邀请族取证占位；`partyDisconnect` 断线清理 |
| `cmd/wireprobe/world_flow.go` | worldSession 增 `channel/parties/party/partySlot` 字段 |
| `cmd/wireprobe/main.go` | 枢纽构建 + 连接构造 + 分发块（moon 优先，城镇组队其次）+ 断线 defer |
| `cmd/wireprobe/request_scope.go` | `tradeAuctionEvidenceRequest`：C24、C182-190、C334、C411、C629、C985/986、C1841-1844 全量保留 |
| `internal/game/protocol/party_notice.go` | N348/N645/N646/N647/N10/N697/N698 构造器（布局注释直指处理器地址） |
| `internal/game/protocol/party_notice_test.go` | 线格式单测（镜像客户端读取顺序） |

`go build ./cmd/... ./internal/...`、`go vet ./cmd/... ./internal/...`、
`go test ./internal/game/protocol/ ./cmd/wireprobe/` 全部通过。

## 四、如实声明的边界

1. **多人 N9 名册**：每人仍收到单人形（队伍 UI 每人只显示自己）。多人形等实机样本。
2. **邀请握手**：C697/698 体未定盘前只记录不伪造应答——被邀请方还看不到弹窗。
   样本落地后的第二轮实现弹窗 NOTI + C697/698 解码 + join 事务。
3. **交易/拍卖**：本期仅铺好全量取证；状态机/事务在样本落地后实现
   （拍卖需 PostgreSQL 拍卖行表，参照 vault/mail 的存储模式）。
4. ~~NOTI 9/15-18 的老式分发路径未定位~~ 已定位：老核心静态注册表
   `sub_1452F9420`（见第一节）。交易 N16/N18 的小载荷已还原，N15（内嵌
   181 字节物品行）与拍卖族的处理器还原排在样本落地后的第二轮。

## 五、实机双开验证步骤（下一轮的样本来源）

1. 编译候选版 `bin/wireprobe-handoff-source.exe`（`server/Build-Server.ps1`），
   `Start-DFO.cmd --source-build` 拉起双号环境。
2. **一号**：城镇打开组队窗创建队伍（名任意，容量 4）→ 应见 `town_party_created`
   事件（N9 下发）。
3. **二号**：同频道同城进入 → 右键一号角色「邀请入队」→ 无论客户端发 C697 还是
   C12 带 target，事件日志都会以 `plain_hex` 全量保留（`partyEvidenceRequest`）。
4. 一号接收弹窗/二号等待的 S→C 流量在第二轮按 dstr-0x46A/0x270 定位的处理器补实现。
5. 交易：双方面对面开交易窗 → C24 与 N15-18 的第一手样本。
6. 拍卖：打开拍卖行搜索任意关键词 → C186/187；登记/竞拍 → C183/185。
7. 会话日志（`runtime/roles_*/events.jsonl`）交回后按本轮文档实现第二轮。

## 六、探针产物索引

| 产物 | 内容 |
| --- | --- |
| `r2-maps.json` | 1,134 NOTI 处理器全图（A 表）+ 1,852 CMD 发送方全图（本轮核心资产） |
| `r2-decomp/` | 目标发送方/处理器/N9 解析器（5,693 行）反编译 |
| `n9-read-offsets.txt` | N9 读取序列带字节偏移（多人布局还原的起点） |
| `r4-decomp/` | C697 子类型 setter + 提交打包器 |
| `r5-decomp/` / `r5.out` | A 表 1,134 处理器的邀请 dstr 扫描（零命中 → 引出 B 表发现） |
| `r6.out` + `party-funcs-325/` | 21 个关键函数原始字节 → bytecmp 部署版 **21/21 全部 100%** |
| `r7-decomp/` | N9 包装器向上追到主循环；**B 表（老核心静态注册）全文** `old-noti-registrations.txt` |
| `r8-decomp/` + `r8.out` | 24,469 个 dstr 取用点扫描：邀请弹窗 0x1416D53E0、N10 处理器等 |
| `r9-decomp/` | N15/16/17/18（交易）、N697/698（S→C 入队）、N1523 反编译 |
