# next148 — 装备调适（CMD2258）直读 PVF 实现与取证

> 任务：在 `main` 上实现 **115 级装备调适**服务端闭环，规则数据**只走 Go 直读内层 PVF**，
> 不新增、不依赖任何导出 JSON（业主 2026-10-01 定调）。
> 本文是取证链 + 实现落点 + 实机验证清单的合并记录。

---

## 1. 一句话现状

- **协议、规则、业务、接入都已在源码里落地**；`go build ./internal/... ./cmd/...`、
  `go vet`、`go test -p 1`（全量）通过。
- **尚未实机**：客户端每次调适的 25 字节 `request_hex` 与实际阶段值要靠实机日志核对（见 §7）。
- 起点很重要：**附件《装备调适逻辑说明》引用的源码路径在本仓库不存在**
  （`internal/game/protocol/equipment_awakening.go` / `internal/inventory/set_points.go` /
  `internal/loot/transform.go` 在 `git rev-list --all --objects` 里 0 命中），
  所以本轮是**从零实现**，那份文档只作线索。

---

## 2. Opcode 身份

`analysis/dumps/opcodes.tsv`：

```
cmd   2258   0x08D2   ENUM_CMDPACKET_EQUIPMENT_AWAKENING
```

⇒「装备调适」= `EQUIPMENT_AWAKENING`（同一编号的 NOTI 是另一件事，别混）。

---

## 3. IDA 取证链（脚本与产物都在 `analysis/dumps/`）

| 脚本 | 产物 | 结论 |
| --- | --- | --- |
| `ida_awakening_2258.py` | `awakening-2258/` | 注册点 `sub_14000A060`：`sub_14599D450(qword_14E66C090, 2258, sub_140B899B0, 0)` ⇒ **S2C handler = `sub_140B899B0`**；发送器 `sub_140B8AD40`：`sub_146D746E0(w,2258)` → `sub_146D75B10(w, buf, 25)` ⇒ **请求负载固定 25 字节** |
| `ida_awakening_2258b.py` | `awakening-2258b/` | 两个发包点的字段偏移（见 §4） |
| `ida_awakening_2258c.py` | `awakening-2258c/` | 目标模板生成（`sub_1480A6620`，其它 UI 路径共用） |
| `ida_awakening_2258d.py` | `awakening-2258d/` | ACK 语义：`sub_140B899B0(handler, 状态, 错误码)`，`sub_1414921F0(ui, 4)` = 成功收尾、`(ui, 3)` = 失败收尾 ⇒ **体首状态 0 = 成功** |

IDA 调用方式与工作副本约定见 `analysis/dumps/CLIENT-MECHANICS.md` §10
（`D:\tools\ida94\idat.exe -A -S<script> D:\115us-backup\ida-work\DFO.exe.i64`，先清 `PYTHONPATH/PYTHONHOME`）。

---

## 4. CMD2258 协议表

**C2S**（`sub_140B8AD40` 发送 25 字节；字段偏移来自两个原生发包点）

| 偏移 | 宽度 | 含义 | 证据 |
| --- | --- | --- | --- |
| `+13` | u8 | 模式：`0` = 调适、`1` = 初始化 / 返还 | `sub_141491D10` 写 `BYTE5(v13[1]) = 0`；`sub_141491BE0` 写 `1` |
| `+14..17` | u32 | 材料组（源 `[need materials]` 的 `[group] N`） | `sub_141491D10`：`= [a1+996]`（初始化路径恒 `0xFFFFFFFF`） |
| `+18` | u8 | 装备空间：`0` = 背包、`3` = 穿戴栏 | 两个发包点都写 `[a1+56]` |
| `+19..20` | u16 | 装备槽位 | 两个发包点都写 `[a1+60]` |
| `+21..24` | u32 | 目标装备模板（阶段 < 上限时原生写 `0xFFFFFFFF`） | `sub_141491D10`：`= 目标 id`；`sub_141491BE0`：`0xFFFFFFFF` |

> ⚠️ 服务端拿到的 `p` 与这 25 字节**同口径**（既有 5 个实机验证模块同样从 `p[13:]` 读字段，
> 见 `CLIENT-MECHANICS.md` §5）。实现里保留一条**带信封**的兼容读法：
> 若长度 ≥ 38 且首三字节是 `01 | D2 08`，字段整体后移 13 字节（结构自校验，不是猜包）。
> 请求体前 13 字节的内容服务端不解释，只记录 `payload_offset` 便于实机核对。

**S2C**（客户端 `sub_140B899B0(handler, 状态, 错误码)`）

```
kind = 1（命令回包）
体 = u8 状态 + u16 结果码
状态 = 1（非 0）= 成功 → sub_141491360 → sub_1414921F0(窗口 3580, 3)：**只刷新调适面板**
状态 = 0        = 失败 → 按 u16 码弹提示 → sub_1414921F0(窗口 3580, 4)：刷新 + 复位
结果码只在状态 0 时被读；0 = 无文案；1/3/119/217 各有原生物品提示
```

⇒ 服务端成功应答 = `01 00 00`（状态 1 = 成功），失败 = `00 00 00`。
这与姊妹协议 CMD2259 的 `EquipmentCraftReply`（`out[0] = 1 // 成功前缀`）同一约定。

### 4.1 实机结论（2026-10-01 22:46，角色 11 / 100261128 鞋子）

`request_hex = 0000803f00000000d0fae94601000100000003110008ddf90500000000000000`
（38 字节；前 13 字节是客户端 writer 里的残留、服务端不解释）：

| 字节 | 值 | 解读 |
| --- | --- | --- |
| `[13]` | `00` | 模式 = 调适 |
| `[14..17]` | `01 00 00 00` | 材料组 1 |
| `[18]` | `03` | 穿戴栏 |
| `[19..20]` | `11 00` | 槽位 17 |
| `[21..24]` | `08 dd f9 05` | 模板 100261128（= 当前模板） |

三次调适 `stage 0→1→2→3` 全部成功（`equipment_awakening_committed`，`payload_offset=13`），
装备实例 `+170` 与右侧面板同步上涨 ⇒ **协议读法确认**。

**当时的体验缺陷**：左侧调适面板不刷新、也没有成功提示。
根因是应答状态字节发了 `0`（当时误判"0 = 成功"）：客户端据此走
`sub_1414921F0(window, 4)`（刷新 + **复位**）而不是 `(window, 3)`（只刷新
—— `sub_1414928D0` 会**重查材料并刷新面板按钮/材料行**）。
改为 `01 00 00` 后由用户复验。

---

## 5. PVF 直读源与结构

唯一真源（`server/work/client-build/Script.inner.pvf`）：

```
etc/115lvability/equipmentawakeningoptionsystem.cos   规则表
etc/115lvability/equipmentawakeningoption.lst         选项索引（263 条）
etc/115lvability/equipmentawakeningoption/**          各选项的阶段加成（[parameter][level]）
equipment/**/*.equ 的 [equipment awakening option] N  装备 → 选项 ID
```

`equipmentawakeningoptionsystem.cos` 结构（`internal/catalog/equipment_awakening.go` 的解析对象）：

```
[max awakening] 3                       阶段上限
[infos]
 [info]
  [condition] 115 `rare` 0              等级 / 品质名 / 条件档
  [need materials]
   [group] 1                           材料组 1（基础材料）
    0 2 0 150000 10361512 75           阶段 项目数上限 (模板 数量)…
   [/group]
   [group] 2                           材料组 2（额外催化剂 10401346）
   [/group]
  [/need materials]
  [refund materials]                   返还（mode=1 / 转换降品用）
  [rates]                              各阶段成功率（本版本全 100）
  [upgrade result]                     阶段 3 的升品映射：源模板 候选数 目标…
 [/info]
[/infos]
```

关键判据（**踩过的坑**）：

1. **"项目数"是上限，不是相等**：源里数量为 0 的项被整对省略 ——
   实测 `[condition] 115 epic 5` 的 `3 3 0 800000 10415191 100` 声明 3 项却只带 2 对。
   解析器按 `实际 ≤ 声明` 校验（超出才报错）。
2. **`[condition]` 第三个数与成本行阶段是两个维度**：前者只有 0/1/2/3/5，
   后者恒为 0..3；每个材料行都必须有对应成功率（原生测试以此断言）。
3. **太初（primeval）在源里只有 `[condition] 115 primeval 0`**，且 `[upgrade result]`
   **全部候选数为 0**（例如 `117010253 0`）⇒ 如实反映"不可升品"，不做猜测性兜底。
4. 品质名表与装备源 `[rarity]` 数值的对应：`common…primeval` = `0..8`
   （与 `internal/inventory/reinforcement_gold.go` 同序；`117010253` 的 `[rarity] 8` = primeval）。

---

## 6. 实现落点

| 文件 | 职责 |
| --- | --- |
| `internal/catalog/equipment_awakening.go` | 直读解析：`ImportEquipmentAwakeningRules` / `ParseEquipmentAwakeningRules` / `ImportEquipmentAwakeningOptions` + 查询 API（`Info` / `Group` / `Row` / `Refund` / `Rate` / `Upgrade`） |
| `internal/game/protocol/equipment_awakening.go` | `DecodeEquipmentAwakening`（含带信封兼容）+ `EquipmentAwakeningReply/Success/Failure` |
| `internal/inventory/equipment_awakening.go` | `PlanAwakening`（纯判定）+ `ApplyEquipmentAwakening`（事务/幂等）+ `SetEquipmentAwakeningRules`（启动期注入） |
| `internal/gamedata/source.go` | `Source.EquipmentAwakening()` / `Source.EquipmentAwakeningOptions()` |
| `cmd/wireprobe/pvf_awakening.go` | 启动期准备与安装（`preparePVFEquipmentAwakening` / `installEquipmentAwakening`） |
| `cmd/wireprobe/equipment_awakening_flow.go` | CMD2258 业务流程与增量行刷新 |
| `cmd/wireprobe/main.go` | 分发分支（`frame.ID == protocol.EquipmentAwakeningOpcode`） |

**存档与状态落点**：装备实例行（181 B）的 **`+170`（0xAA）** 就是调适阶段 ——
`internal/character/fame.go` 早就按 `rules.Awakening[template][Record[170]]` 算名望
（注释里的客户端映射 `14576D8B0`）。调适只动这一字节；升品时同时改模板并清零该字节，
其余实例字节（强化/增幅/附魔/品级/融合…）与存档未知字段一律保留。

**材料来源**：`10361512..10361516` 与 `10415191` 同时是**账号共享材料仓库**的固定格
（`account_materials.go` 的 375..379 / 光辉灵魂 1 号格），所以整笔事务走
`Store.CommitAccountMaterialEvent`（角色 + 账号材料同一事务、`(角色, key)` 幂等）。
其余材料（`10400396` / `10401346` / `10415195`）走背包扣减。

**明确拒绝（不静默兜底）**：

- `mode = 1`（初始化 / 返还）：源 `[refund materials]` 已读齐，但客户端表现未取证 ⇒ 先拒绝；
- 非 100% 成功率：失败分支（扣料但阶段不动）缺客户端展示证据 ⇒ 拒绝；
- 阶段/品质/材料组在源里没有对应条目：拒绝并在日志里列出可用阶段/材料组。

---

## 7. 实机验证清单（需要业主手动操作）

准备：按 §2 的命令重新编译并**发布 exe**（启动器跑的是 `bin/wireprobe-pvf.exe`）。

1. **启动日志**应出现：
   `PVF equipment awakening prepared: max=3 conditions=21 stage-tables=… upgrade-sources=… options=263`
2. 用一件 **115 级、rarity 2/3/4/6** 的装备打开调适界面，点一次「调适」：
   - 期望：装备阶段 +1，材料/金币按源扣除，界面不卡；
   - 日志 `equipment_awakening_committed` 里核对 **`stage_before` / `stage_after` / `material_group` /
     `payload_offset` / `request_hex`** —— 尤其 `payload_offset` 必须是 13（若出现 26，说明该帧带信封，
     协议读法需要按实机结论收紧）。
3. 把装备调到阶段 3 再点一次：期望走**升品**（模板变化、阶段归 0），或对太初装备收到
   「升品候选为空」的拒绝提示。
4. 材料不足 / 金币不足 / 装备不在对应槽位：期望拒绝且**不扣任何东西**（同一事务回滚）。
5. 若调适后名望变化异常，按 `docs/交接-深渊与身份修复-20261001.md` §2 的命令取 `gateway.err` 里的
   `equipment_awakening_*` 事件。

> 实机确认后按根 `AGENTS.md` §0.7 收口：更新 `CHANGELOG` 与 confirmed baseline，再提交本轮文件。

---

## 8. 待办 / 边界（本轮未做）

1. `mode = 1`（初始化 / 返还）：源已读齐（`[refund materials]`），缺客户端表现证据。
2. **套装积分**：`etc/115lvability/setpointinfo.cos` + `equipmentsetpointtable.lst`
   已随名望链路直读（`internal/character/fame_native.go`），但 **`internal/inventory/set_points.go`
   在本仓库不存在** —— 附件文档描述的那套"套装档位名望"仍是空白。
3. **装备转换（CMD2259）里的调适联动**：`etc/115lvability/equipmenttransformsystem.cos`
   已确认可直读（UTF-16LE，`a.ReadText` 能解），但"转换保留/返还调适阶段"未实现。
4. 调适选项表（`equipmentawakeningoption/**`）目前只做**可读性核对**（启动日志报告 263 条），
   其阶段加成（`[skill bonus rate]` / `[equipment buff]`）尚未写入战斗属性包。
