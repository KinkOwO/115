# 交接任务：定位客户端“更好的装备 ↑”箭头误显示的判定逻辑

> 交接日期：2026-09-21 · 交接方：服务端侧 agent · 接手方：客户端逆向 agent
> 服务端侧已完成取证与三次尝试（全部排除，见下），**本任务只剩客户端一处待定位**。

## 1. 一句话目标

找出 `client/DFO.exe` 中**决定装备栏格子上“↑ 更好的装备”箭头是否显示**的代码，
读出它的判定条件，并说明为什么“重登进城后”与“一次换装/切图后”两种状态下判定结果不同。

## 2. 现象（实机，2026-09-21，玩家操作，零例外）

角色：剑魂（swordman），35 级，角色 id 11。

| 状态 | 背包第一行（8 件可穿装备） | 不可穿的那件 |
| --- | --- | --- |
| **重登进城后**（不做任何操作） | **8 件全部显示 ↑** | `37302` 盗贼双剑（`[usable job]=[thief]`）**不显示** |
| **一次真实换装后** | 只剩**真正比身上强**的那件（`400140161` pants grade 5，对比身上 `400140158` grade 1） | 仍不显示 |
| **切一次地图后** | 同上（正确比较） | 仍不显示 |
| 关闭/打开装备栏 | **无变化** | — |

**结论（服务端侧定性）**：重登后客户端只做“**可穿戴标记**”（能穿就画箭头，与是否更强无关），
一次真实装备操作或区域切换后才进入“**强弱比较**”。触发条件在客户端内部。

背包/身上装备的目录数据（`configs/equipment.current37.json`）已逐件核对：
背包 8 件**全部满足**等级与职业要求，其中 6 件档次（`[grade]`）**低于**身上的同类件 —— 所以“全亮”不是数据问题。

## 3. 已排除的服务端路径（**不要重复**，三次上限已到）

服务端侧在入口帧序列末尾追加过以下帧并发实机验证，**全部无效且已回退**：

| # | 追加内容 | 结果 |
| --- | --- | --- |
| 1 | NOTI23 + NOTI24（复刻一次切图的收尾：`CMD36 → NOTI23 → 重发布 actor → NOTI24 → USERINFO`） | 无效，且使其中一行的箭头消失 |
| 2 | NOTI13 list0 + list3（背包/穿戴快照，即换装应答里那两条） | 无效 |
| 3 | CMD19 确认帧（`ItemMoveSuccess`，源=目标、服务端状态不变） | 无效 |

两次“有效”路径（真实换装、切地图）下发的都是**客户端自己请求的应答**
（`item_move_committed` / `AreaChangeSuccess`），单纯补发通知或确认都不构成触发。

**服务端已确认无缺陷**：穿戴快照完整下发（`worn_equipment_restored`，list 3，1668 字节 / 9 件，
每行 = 181 字节装备行 + 4 字节 Period），槽位↔部位映射与客户端原生表一致
（武器 12、上衣 14、肩 15、裤 16、鞋 17、腰 18、项链 19、手镯 20、戒指 21）。

## 4. 环境与工具（都已就位）

| 资产 | 路径 | 说明 |
| --- | --- | --- |
| **权威 IDB** | `client/DFO.exe.i64` | 484,741,229 字节，2026-09-21 16:50 获取；`client/` 已在 `.git/info/exclude`，**不要入库** |
| xorstr 地址表 | `analysis/dumps/xorstr_addr_to_text.json` | 114,614 条“静态 VA → 明文”，**本任务的主线索来源** |
| opcode 表 | `analysis/dumps/opcode_name_to_hex.json` | 5,330 条 CMD/NOTI 名称↔编号 |
| Ghidra 脚本 | `analysis/tools/ghidra_xorstr_xrefs.py` | 按关键词从 xorstr 表取地址 → 列出引用函数 |
| Ghidra 脚本 | `analysis/tools/ghidra_decompile_xorstr_users.py` | 反编译上述函数并打印伪代码 |
| IDA 脚本（备用） | `analysis/tools/ida_xorstr_xrefs.py` | 同功能；有 Hex-Rays 出伪代码，否则出反汇编 |

注意：IDA Free/Home **不带 Hex-Rays**；若走 IDA 只能看反汇编。Ghidra 需 JDK21（最新版要求）。

## 5. 强线索（xorstr 地址，已确认存在于表中）

**装备比较 / 已装备判定（优先看这几个）**

| VA | 明文 |
| --- | --- |
| `0x1491ADF70` | `is same my equipped item group` |
| `0x1491ADF40` | `is my equipped item` |
| `0x1491ADEE0` | `is equipped` |
| `0x1491AE1A8` | `is equipped slot` |
| `0x1491AB210` | `equip specific level` |
| `0x1491AB120` | `upgrade inven` |

**箭头资源（格子上那个 ↑ 的可能资源名）**

| VA | 明文 |
| --- | --- |
| `0x14974BBB0` | `spec_slot_arrow` |
| `0x1492F60D8` | `up_arrow` |
| `0x1492F60F8` | `down_arrow` |
| `0x1492BD070` | `panel_arrow` |
| `0x1491CD848` | `arrow` |

**对比箭头 / 名望（tooltip 侧的“对比”箭头，与格子箭头可能同源）**

| VA | 明文 |
| --- | --- |
| `0x1491F5C98` | `conArrowRoot` |
| `0x1491F5CC0` | `numArrow` |
| `0x1491F5CE0` | `arrowKey_%d` |
| `0x1491F5D28` | `conArrowDodgeRoot` |
| `0x1491F5D58` | `arrowDodge_%d` |
| `0x1491DC488` | `buffRateArrow` |
| `0x1491DC4D8` | `fameText` |
| `0x1491DC6C0` | `fame_txt` |

**名望（玩家提到箭头“等级与名望都比身上低”时用的比较量，值得确认它是否参与判定）**

| VA | 明文 |
| --- | --- |
| `0x1491AAA08` | `fame` |
| `0x1491AB060` | `fame to stat` |
| `0x1491D33D0` | `[min fame]` |
| `0x1491D33F0` | `[max fame]` |

> 注意这些 VA 是**静态数据地址**（xorstr 解密前的存储位置），不是明文出现处 ——
> 在 IDB 中跳到该地址，看它的 xref，才是使用它的代码。

## 6. 建议的第一步

1. 用 IDB 打开（Ghidra 用 Ghidra 脚本 / IDA 用 IDA 脚本）；
2. 先跑 xref 脚本，看第 5 节那批 VA 的引用函数分布 —— 预期 `is same my equipped item group`、
   `is equipped slot`、`spec_slot_arrow` 会指向**同一批 UI 函数**；
3. 对其中一个函数下断/读伪代码，确认它是“格子绘制”还是“tooltip 比较”；
4. 明确回答两个问题：
   - **箭头是按什么算出来的**（档次 `[grade]`？名望？攻击/防御？装配位置？）；
   - **重登路径下客户端缺哪一步**，导致它退化成“只标可穿戴”。

## 7. 期望产出

- 命中函数的地址 + 伪代码（或关键汇编）；
- 判定条件的自然语言描述；
- 如果可修复：明确是**服务端补数据**（哪种数据、哪个包、哪个字段）还是**必须客户端补丁**。
  若判定为客户端补丁，**先不要动手**：按项目规则需用户明确要求才可新增/修改客户端 DLL/补丁。

## 8. 约束（项目硬规则，务必遵守）

- **服务端优先**：本任务只做**分析取证**；未经用户明确要求，不得新增/修改/启用客户端 DLL 或补丁。
- **禁止猜测**：结论必须有 IDA/IDB 证据 + 实机对照；不得凭“名字像”下结论。
- **实机由用户操作**：不得无人值守启动客户端；需要实机验证时先通知用户。
- **保护工作区**：`client/DFO.exe.i64` 等大资产不入库；`client/` 已被 `.git/info/exclude` 排除。
- 分析完毕**正常保存并关闭 IDB**，避免损坏或进程锁死。

## 9. 参考文档与代码（服务端侧，供对照）

| 位置 | 内容 |
| --- | --- |
| `server/work/dfo-lan/docs/protocol/next47-relog-upgrade-arrow-client-side.md` | 本问题的现象、三次失败尝试、定性结论 |
| `server/work/dfo-lan/cmd/wireprobe/entry_flow.go` | 入口帧序列（含 NOTI13 list0/list3、NOTI14 外观刷新、末帧 NOTI2827） |
| `server/work/dfo-lan/cmd/wireprobe/equipment_flow.go` | 换装（CMD19）应答：`item_move_committed` + list0 + list3 + NOTI14 |
| `server/work/dfo-lan/cmd/wireprobe/world_flow.go` | 区域切换（CMD36）应答序列：`36 → NOTI23 → actor → NOTI24 → USERINFO` |
| `server/work/dfo-lan/internal/inventory/equipment.go` | `EquipmentRow`：181 字节装备行的字段布局（**不含等级/名望等属性**，客户端比较时用的是本地 PVF 目录） |
