# 查找队伍（Find Party）组队匹配协议取证 — 2026-10-01

> 目标：修复城镇「查找队伍 (J)」窗口显示假行（`No Party name / FULL / 0/0 / Ch.0`）
> 的问题。方法：IDA 9.4 无头探针（`E:\115us\ida-work\r12_probe.py`、`r13_probe.py`，
> 产物 `r12-decomp\`、`r13-decomp\`），复用 0929 轮资产 `r2-maps.json`、
> `r2-decomp\`、`analysis/dumps/opcodes.tsv`。IDB 为部署版 2.38.2.34
> （`E:\115us\DFO.exe.i64`），关键函数沿用 donor 3.25 推导 + r6 字节比对 100% 的结论。

## 一、组队匹配命令族（本轮定盘）

| 包 | 官方名（opcodes.tsv） | 发送方（r2-maps.json） | 反编译 |
| --- | --- | --- | --- |
| C2151 | ENUM_CMDPACKET_EXPANDING_PARTY_MATCHING_USER_INFO | 0x14150A1F0 | r12 |
| C2152 | ENUM_CMDPACKET_EXPANDING_PARTY_MATCHING_JOIN_OR_INVITE | 同上（分叉） | r12 |
| C2153 | ENUM_CMDPACKET_EXPANDING_PARTY_MATCHING_ACCEPT_INVITE | 0x141509EA0 | r12 |
| C411  | ENUM_CMDPACKET_INVITE_MEMBER_FOR_GROUP | 0x142D43800 / 0x142D43C50 | r12 |

- **C2151 与 C2152 共用一个发送函数** `sub_14150A1F0`：模式字段 `*(a1+7920)`
  取 0/2/3 时发 **C2151**，取 1 时发 **C2152**；发送前 `sub_14150D060` 往请求对象
  填 12 个 288B 步长的成员槽、队名字符串（对象+16）、说明字符串（对象+48）、
  各成员物品计数聚合与标志位（对象+1624）。体经请求对象池序列化，静态不可达
  ——与 C697 族同一取证路径，**等实机样本**（三命令已入 `partyEvidenceRequest`
  全量保留白名单）。
- **C2153**（接受匹配邀请）：请求对象直拷 u32 区（378..415+ 连续字段），体较大，
  逐字段语义未定盘。
- **C411**：发送前把 `a1+288` 的 3 槽位映射初始化为 -1，再经对象池提交。

## 二、S→C 等待列表族（布局已还原，构造器已在 `internal/game/protocol/party_notice.go`）

| 包 | 官方名 | 处理器 | 布局（按处理器读取顺序 1:1） |
| --- | --- | --- | --- |
| N645 | ENUM_NOTIPACKET_ENTRY_PARTY_WAIT | sub_144647F50 | `u16 self, u32 count, count × {u16 actor, u8 flagA, u8 state, u32 dungeon, u32 diff}` |
| N646 | ENUM_NOTIPACKET_ENTRY_INTO_PARTY_FINISH | sub_144647CB0 | `u16 actor, u32 A, u32 B`；actor==自己时把 A/B 写进 net+312/+304 两个标量 |
| N647 | ENUM_NOTIPACKET_ENTRY_INTO_PARTY_UPDATE | sub_144647DF0 | `u32 count, count × {s32 key, u8 state}`（同一张排序表的 state 更新） |
| N349 | ENUM_NOTIPACKET_INVITE_MEMBER_LIST | sub_146924D30 | `u32 组id, u8 count, count × {u8 key, 名字}`（名字缺省 dstr 669 "Nameless"） |

**N645 行的两个 u32 已证是（客户端本地副本表的）副本索引与难度索引**，不是副本 id：

- UI 消费 `sub_14464C010(ui, actor, isSelf, flagA, a5, a6)`（r12-decomp/ui_*.c）；
- `a5/a6 == 0xFFFFFFFF` 时整行**直接跳过**（哨兵=无副本）；
- 否则 `sub_145B313A0(net, dungeon, diff, 0xFFFF)`（r13-decomp）按
  `index = dungeon + diff * 难度数` 解析副本对象，越界返回 0（行不渲染）。

因此：**服务端在没有「副本 id → 客户端本地索引」映射事实之前，不能编 N645 行**；
城镇 C12 创建的队伍也没有副本选择。当前 `partyHandle` 对 C2151 回**空等待列表**
（真实状态），并让客户端的请求等待态（发送方 `sub_1467A3620` 注册、
`sub_14667BB90(net,537)` 查询）有应答可收。

## 三、本轮代码变更（2026-10-01）

1. **0929 组队第一期补丁合入当前树**（补丁基于旧谱系，共享文件按净增量重锚）：
   - 新文件：`cmd/wireprobe/party_hub.go`、`party_flow.go`、
     `internal/game/protocol/party_notice.go`、`party_notice_test.go`、本文档前置
     `docs/protocol/party-trade-auction-20260929.md`；
   - `main.go`：`townParties := newTownPartyHub()`、worldSession 字面量
     `parties/channel` 两字段、`defer worldState.partyDisconnect()`、moon 块之后的
     城镇组队分发块（moon 优先，城镇组队仅在无副本时接手）；
   - `world_flow.go`：worldSession 增 `parties/party/partySlot/channel`；
   - `request_scope.go`：`tradeAuctionEvidenceRequest`（C24、C182-190、C334、C411、
     C629、C985/986、C1841-1844）+ 1723（装备继承族补全）。
2. **组队匹配白名单**：C2151/2152/2153 加入 `partyEvidenceRequest`（0929 文档
   声称已加，实际补丁与两条谱系里都没有——本轮补上）。
3. **C2151 应答**：`partyHandle` case 2151 → N645 空等待列表（见第二节边界）。

## 四、如实边界与下一步

1. N645 行渲染需要的（副本索引， 难度索引）映射未定盘 ⇒ 本轮不伪造行，
   查找队伍窗口在空列表下不再被未应答的请求卡住，但**完整招募列表要等实机样本**。
2. C2151/2152/2153 的请求体布局经对象池序列化，静态不可达 ⇒ 白名单已开，
   实机（双开或单开点查找队伍）一次操作即可在 `events.jsonl` 拿到完整明文。
3. C2153/C411 的体语义、N646 A/B 标量、N349 组 id 语义：等样本。
4. 截图里「No Party name」字样来自 N9 解析器的 dstr 626 缺名占位（0929 文档），
   队伍名要在招募广告（C2151 体）定盘后才能服务端供给。
