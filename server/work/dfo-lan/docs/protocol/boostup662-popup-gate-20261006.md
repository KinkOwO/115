# 662 直升活动首登弹窗的角色门禁（2026-10-06）

## 0. 现场与结论摘要

业主 2026-10-06 报告：**「奥德赛模式直升活动会弹窗，115级满级角色也会弹窗」**。
截图角色 = `Poison`（本地库 id 1，1 级，奥德赛创建：`creation_options` 的 option[10]=2），
进城即弹「Sky of a Thousand Seas BOOST UP / AUG 4 - NOV 17」窗口（`Cal.xui`）。

结论：**弹窗谓词在客户端，服务端唯一的开关是 NOTI2265 的可用性值**。
- 缺行 = 值 0 = **照弹**；
- 只有「本礼盒值 == 1 **且**它 `[link gift index]` 指向的礼盒也全部 == 1」才不弹。

所以修法不是"不给奥德赛/满级角色发活动行"，而是**把所有行标成已处理**（值 1），
这正是官服那条 2265 用的口径（抓包 10 条连接全是 `{118: 1}`，从不列 117）。
落点：`cmd/wireprobe/boostup_flow.go` 的 `boostGiftOfferSuppressed` +
`boostGiftAvailability`（两个 2265 发送点——进城恢复与 CMD643 领取后回执——都走同一函数）。

---

## 1. 真源（PVF）

`event/eventgift.evt`（导出：`server/work/dfo-lan/.tmp/popup-20261006/00-eventgift.evt.txt`）

| 礼盒 | `[event index]` | `[first login open popup]` | `[first open order]` | `[open check recv reward]` | `[link gift index]` | `[ui file]` | `[level limit]` |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 117 | 10017 | **1** | -1 | 1 | **118** | `Live\Event\Kor\2026\0326_BoostUp\Xui/Cal.xui` | 1 |
| 118 | 10018 | 0 | 0 | 0 | （无） | （无） | 0 |

`live/event/kor/2026/0326_boostup/boostup.evt`：`[goal level] 115`、`[capsule usable level] 115`、
`[level up table] 1 114 115`、`[fame value limit] 63257`、`[event town area] 222 0`。
⇒ 115 既是本活动的终点也是胶囊上限：**已满级/已到终点的角色在这套源里没有可领的东西**。

弹窗主体是 **礼盒 117**（唯一 `[first login open popup] 1` 且自带 `Cal.xui`）。
`[link gift index] 118` 决定了"要两个都标已处理才不弹"这条与门。

奥德赛侧：`boostup.evt` / `eventgift.evt` **都不提奥德赛**，源里没有"奥德赛不参与"的字段。
按 AGENTS.md §0.2 第 4 条，这条记为**业主明确要求的服侧差异**（奥德赛有自己的一条直升线），
不是源驱动规则；等级那条（`>= [goal level]`）是源驱动的。

## 2. NOTI108 不是本缺陷的开关（排除项）

用 `internal/toolcmd/eventinfogate` 的固定记录布局解析官服表
`internal/legion/event_info_official.plain`，四条相关记录**全在官服表里**：

| id | 偏移 | 时间窗 | 记录长度 |
| --- | --- | --- | --- |
| 662 | 4476 | 1785801600..1794905999 | 65 B |
| 665 | 5805 | 1785801600..1794905999 | 65 B |
| **10017** | 1545 | 1785801600..1794905997 | 164 B |
| **10018** | 1709 | 1785801600..1794905996 | 116 B |

⇒ 「108 里有没有 662/10017 行」是**全服共享**的，不能按角色区分，也就不是本次弹窗的开关。
（108 的合并下发是 2026-10-06 早先另一处修复，本缺陷不动它。）

## 3. 客户端链（IDA 9.2 headless，隔离副本 `.tmp/exp37/DFO.exe.i64`）

产物：`analysis/dumps/eventgift-popup{,2,3,4,5,6}/`，脚本 `analysis/dumps/ida_eventgift_popup{,2..6}.py`。

| 环节 | 地址 | 事实（直接反编译） |
| --- | --- | --- |
| NOTI 2265 处理器 | `0x144D49840` | 注册经 `sub_14599D5D0`，`edx=0x8D9` |
| 2265 读取器 | `0x144D499A0` | `u16 count` + 每行 `{u16 礼盒, u8 值}`（`sub_146EA1920`/`sub_146EA09F0`）→ 写进**管理器 +448** 可用性树（键 node+28，值 node+32）。**这是 +448 的唯一写入者** |
| 弹窗登记循环 | `0x144D4ACC0`（调用者 `0x145A21E40`） | 遍历管理器 +32 的 PVF 礼盒表，对每条记录按 +272/+274/+276 决定谓词，经 `sub_144D4B8E0` 注册进首登弹窗登记处（单例 `qword_14E634260`，184 B；`sub_144922980(键=礼盒 u16, 事件号, 谓词, 2)` + `sub_144922330`） |
| 谓词 | `0x144D4AA80` | **本缺陷的判据**，见下 |
| 链接判定 | `0x144D4B4F0` | 礼盒记录的 `[link gift index]` 区间（node[41]..node[42]）**全部**值 == 1 才返回 0 |
| 领取回包写入 | `0x144D49530` → `sub_142AEEA90` | CMD 643 回包把 `{u16 礼盒, u8 值}` 写进同一棵 +448 树 |

谓词 `sub_144D4AA80(礼盒号)` 的返回值（`true` = 提示该礼盒）：

```
礼盒不在 PVF 表 或 其 [link gift index] 区间为空
    ⇒ 返回 (可用性值 != 1)      // 缺行时值取 0 ⇒ 恒为 true（照弹）
礼盒在表内且有链接区间
    ⇒ 仅当 可用性值 == 1 且 区间内每个礼盒值 == 1 时返回 false（不弹）
      其余情况返回 true
```

⇒ **省略 2265 的行 = 弹窗保留**；只有把 117 与 118 都标成值 1 才关掉。
这也解释了实机现象：领完两个礼盒后不再弹（同一谓词），以及官服为什么只需一条 `{118:1}`。

旁证（不参与本缺陷）：`sub_144D4B660`（调用者 `0x145B719A0`）里有硬编码的
`取等级(vtable+2744) < 115` 判定——客户端在**另一条路径**（选角侧）确实按 115 挡，
但首登弹窗谓词 `sub_144D4AA80` 不带等级判定，所以服务端必须自己挡。

证据分级：谓词、读取器、登记循环的分支与树写入是**直接反编译**；
「+272 = `[first login open popup]`、链接区间 = `[link gift index]`」是**按值对应推断**
（117 与 118 恰好只在这几个布尔字段上不同，且弹窗确实只跟 117 走），
未逐个追 PVF 解析器 `sub_147C03830` 的字段写入点。

## 4. 官服对照

`D:\DNF115\123\pathgate-work\cap43\full\20261002-211855_...\frames.jsonl`，op 2265 共 10 条，
全部相同：`0100 7600 01 ...` = count 1 / 礼盒 118 / 值 1，**从不列 117**。
修复前我方两条 2265 = `{117:0, 118:0}`（对每个角色，无门禁）。

## 5. 改动

`cmd/wireprobe/boostup_flow.go`
- 新增 `boostGiftOfferSuppressed(c, role)`：`character.CreatedAsOdyssey(role)`（只看角色自身创建标记，
  不吃 `DFO_ODYSSEY_MODE` 覆盖，与进城准入同源）或 `state.level >= c.GoalLevel`（`[goal level]`，源驱动）。
- `boostGiftAvailability` 在被抑制的角色上把所有礼盒行写成「已处理」（值 1）。
- 帧形状、opcode、发送时机一字未动；报价内角色（1..114 级普通角色）逐字节与修复前一致。

测试：`cmd/wireprobe/boostup_flow_test.go`
`TestBoostGiftOfferSuppressedForOdysseyAndMaxLevel` 钉住 1/114/115/奥德赛 四种报价、
`DFO_ODYSSEY_MODE` 两个覆盖方向、以及"报价内角色仍按自身领取状态出行"。

## 6. 未闭环 / 需业主裁决

1. **CMD643 领取路径未加同样门禁（业主 2026-10-06 裁决：保持现状）**：奥德赛或 115 级角色若仍从别处点到
   礼盒 117，服务端照发胶囊（沿用 2026-10-04 已实机确认的"上限含 115，未上过课的角色仍可进教学"判定）。
   报价显示"已处理"与领取仍会成功并存，是本次明确接受的口径；领取后的 2265 回执同样走
   `boostGiftAvailability`，被抑制角色保持全行已处理，不会出现"领完反而又弹一次"。
2. NOTI2638（训练进度）/2639（名单）/2722（665 挑战）仍按现有口径下发，未按角色抑制；业主只报了弹窗。
3. 首登弹窗登记处（`qword_14E634260`）的"每角色只弹一次"去重存储与 `sub_144D4B8E0` 里
   注册对象 +16 那棵树的关系没有继续追——不影响本门禁结论。
4. **实机验收（业主 2026-10-06「实机测试OK」）**：会话
   `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261006_195540_767216_next37`
   里 `character_id=1`（奥德赛 1 级 Poison）与 `character_id=2`（115 级 Duyao）各收到一条
   `boost_gift_states_restored id=2265 plain_bytes=8`，两类角色进城均不再弹窗；现役基线
   `bin/wireprobe-pvf.exe` = `c5a5b384…`（旧基线备份 `.tmp/popup-20261006/baseline-2a3962e6.exe`）。
