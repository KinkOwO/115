# next69 — `.ctp` 格式规范（**已完全闭环**）

> 目标：解开 `contents/2026/apocalypse/etc/*.ctp`，使 P4 阶段时限、P6 `[allow coin]`、
> P5 `[reward data]`、P3 职责参数等都有**文件真源**，不再写死或转述。
> 状态：**格式 100% 闭环**（用两个文件独立验证：record_count / 遍历终点 / 尾表 / 池全部对齐）。
> 相关台账：`next64` §6.2 **X2**（本轮关闭）。
> 工具：`analysis/tasks/next69-ctp-extract.py`；导出：`next69-apocalypse-ctp.json`、
> `next69-dungeonskillinfo-ctp.json`。

---

## 0. 三轮取证的结论演进（保留，避免后人重走弯路）

| 轮次 | 当时的判断 | 现状 |
| --- | --- | --- |
| ① | `.ctp` 走**文本**词法分析器（`sub_1474D02D0`，`[` = 标签、`/` = 注释） | **部分错**：那是**松散模式（`dword_14DC91508 != 0`）** 的路径；发布版 `dword_14DC91508 == 0` 走**二进制**分支 |
| ② | 记录是「`[u32 kind][变长 payload]`」，用对齐搜索求解 kind→长度 | **错**：对齐搜索 0 解正是因为它不成立；真实结构见 §2 |
| ③ | 记录流 = `名字引用 + 值区`（名字引用锚定扫描） | **对但不完整**：缺了 `flags`/`parent`/`n_refs` 三个字段，导致记录 1 之后失步 |
| ④（本轮） | 完整记录 = `名字(16) + flags(4) + parent(8) + n_cells(8) + n_refs(8) + cells + refs` | ✅ **两个文件精确对齐，record_count 一致，尾表闭环** |

---

## 1. 加载链（两个并列解析器，按资源形态分流）

`sub_1474CDEA0` = `RDARScriptBuilder::build`（错误串实证），按 **路径后缀 + 全局 `dword_14DC91508`** 分流：

| 分支 | 触发 | 链路 | 产物 |
| --- | --- | --- | --- |
| **B 打包（发布版）** | 后缀 = `.ctp` 且 `dword_14DC91508 == 0` | `sub_147C484A0` → `sub_147C48890` → **`sub_1474D11A0`** → `sub_1474D0C00`（段）→ `sub_1474D0580`（cell） | 二进制表，vtable `off_14B292468` |
| **A 松散/脚本** | 后缀 ≠ `.ctp`，或 `dword_14DC91508 != 0` | `sub_1474CA960` → `sub_146E96930`（BOM/代码页 → 宽字符）→ `sub_1474CC1C0` → `sub_1474D02D0`（文本词法器） | 文本表，vtable `off_14B292410` |

`sub_1474CA940` 是 `sub_1474CC1C0` 的包装（文本表的入口），`sub_1474D11A0` 是二进制表的入口。

**服务端只需要分支 B**（发布版 PVF 里的 `.ctp` 都是二进制）。

---

## 2. 文件级布局

以 `apocalypse.ctp`（9280 B）为例；`dungeonskillinfo.ctp`（1636 B）同构。

```
0x00  头部 36 字节
0x24  记录数组（record × record_count）
       └ 记录区止 = 尾表起点
       └ 尾部索引表（trailer）
       └ 1 字节 NUL 分隔
       └ 串池（ASCII 标签，无分隔符；字符 0 = 第一个 '['）
```

### 2.1 头部（36 字节 = 9 × u32，小端）

| 偏移 | apocalypse | dungeonskillinfo | 含义 |
| --- | --- | --- | --- |
| `0x00` | 1 | 1 | **版本**（非 1 拒绝装载） |
| `0x04` | 65 | 14 | **记录数 `record_count`** ✅ 与遍历结果精确一致 |
| `0x08` | 0 | 0 | — |
| `0x0c` | 32 | 32 | 两文件相同，疑为「最大列数/字段数」 |
| `0x10` | 0 | 0 | — |
| `0x14` | 8052 (`0x1F74`) | 1372 (`0x55C`) | cell 区止 **− 4**（实测两文件一致偏差） |
| `0x18` | 0 | 0 | — |
| `0x1c` | 8300 (`0x206C`) | 1436 (`0x59C`) | 尾表止 **− 4**（实测一致偏差） |
| `0x20` | 0 | 0 | — |

> 两处偏移字段都稳定比真实边界小 4 字节，取值时**按 +4 校正**；本工具用
> `record_count` 与池起点反推，不依赖这两个字段。

### 2.2 串池

* 池的**字符 0 = 第一个 `[`**（apocalypse `0x2071`、dungeonskillinfo `0x5A1`），
  即头部 `0x1c` 值 **+5**；
* 池是无分隔的 ASCII 拼接块（`[a][b][c]…`），引用 `(lo, hi)` 为 **0 基、hi 独占**，
  `hi − lo` **精确等于字符串长度**（服务端可按区间切，不必依赖客户端的「读到 NUL」语义）。

---

## 3. 记录（record）

```
u64 name_lo, u64 name_hi    # 16 B 池引用（列名）
u32 flags                   # 列格式/类型码
u64 parent                  # ★ 父记录的记录序号；-1 = 顶层
u64 n_cells
u64 n_refs
n_cells × cell
n_refs  × ref
```

固定头 **44 字节**（16 + 4 + 8 + 8 + 8），实测在 `[party waiting area]`、
`[phase info]`、`[member]`、`dungeonskillinfo` 全部记录上精确对齐。

### 3.1 语义还原（全部有独立证据）

| 字段 | 语义 | 证据 |
| --- | --- | --- |
| `name` | 列名 | 池区间精确匹配（`hi−lo` = 字符串长度） |
| `flags` | 列格式/类型码（1 = 单值列，2 = 多值/表列） | 与 cell 数量、有无 `refs` 强相关；**精确语义仍待追** |
| **`parent`** | **父记录序号（0 基），-1 = 顶层** | `[member]` 的 parent = 1 = `[role per member limit]`；`[delay ani time]`(parent=2) 的父记录 2 = `[nomal attack]`；`dungeonskillinfo` 全表一致 |
| `n_cells` | 本记录的值个数 | 与解析出的值个数一致 |
| `n_refs` | 子引用个数 | 与解析出的 refs 一致 |

### 3.2 cell 编码（`sub_1474D0580`：`tag = *(u32*)cursor − 1; switch(tag)`）

| tag | 载荷 | cell 长 | 语义 |
| --- | --- | --- | --- |
| 1 | 2 × u64 | 20 | **名字引用**（字符串值，如 `$party$`、`$normal$`、动画路径） |
| 2 | u32 + 2 × u64 | 24 | **带索引的名字引用**（如 `20:$Apocalypse_Difficulty_Name_01$`） |
| 3 | u8 | 5 | 字节/布尔 |
| **4** | u64 (float64) | **12** | **float64 —— 绝大多数玩法数值** |
| 5 | u64 (float64) | 12 | float64（第二种编码；实测 0.5、1.0 等同样语义） |
| 6 / 7 | u32 + u8 | 9 | — |
| 其它 | 0 | 4 | 填充 cell |

### 3.3 ref（子引用）

```
u64 name_lo, u64 name_hi    # 16 B 子列名
u64 n_values
n_values × u64              # ★ 该子列拥有的记录序号（行号）
```

---

## 4. 尾部索引表（trailer）

```
n_entries × ( u64 name_lo, u64 name_hi, u64 n, n × u64 )
```

**语义：列名 → 该列在记录数组中的所有行号。** ✅ 全部逐条核对通过：

| 列名 | 行号 | 对应记录 |
| --- | --- | --- |
| `[party waiting area]` | `[0]` | 记录 0 = `[party waiting area]` ✓ |
| `[role per member limit]` | `[1]` | 记录 1 ✓ |
| `[keldon xavi final damage rate]` | `[6]` | 记录 6 ✓ |
| `[collaborate attack groggy …]` | `[7]` | 记录 7 ✓ |
| `[guardian hp increase …]` | `[8]` | 记录 8 ✓ |
| `[skirmisher monster hp reduction …]` | `[9]` | 记录 9 ✓ |
| `[operation data set]` | `[10, 24, 38, 52]` | 四个作战块的块首记录 ✓ |

`dungeonskillinfo` 尾表 2 条：`[skirmisher info] → [0]`、`[passive object index] → [9]` ✓

---

## 5. 真值表（**P3/P4/P5/P6 可直接引用**）

### 5.1 `apocalypse.ctp` — 全局列

| 列 | 值 |
| --- | --- |
| `[party waiting area]` | `239, 1, 600, 200` |
| `[role per member limit]` | `2`（+ 子引用 `[member] = [2,3,4,5]`） |
| `[keldon xavi final damage rate]` | `0, 0, 10, 10, 0, 0` |
| `[collaborate attack groggy duration increase per keldon xavi stack]` | `0.5` |
| `[guardian hp increase per keldon xavi stack]` | `10` |
| `[skirmisher monster hp reduction per keldon xavi stack]` | `0.5` |

`[member]` 四行（parent = 1，即 `[role per member limit]`）：

| 记录 | 即 `[role per member limit]` 的子项 `[2,3,4,5]` | 值 |
| --- | --- | --- |
| 2 | 2 | `1, 1, 1` |
| 3 | 3 | `2, 2, 1, 1, 1` |
| 4 | 4 | `3, 2, 2, 1, 2` |
| 5 | 5 | `4, 2, 2, 1, 2` |

⇒ **4 档队伍规模**（首列 1..4 = 人数），`[role per member limit]` = 2 与
`dungeonskillinfo` 的**两个职责（散兵/守卫）** 吻合。

### 5.2 `apocalypse.ctp` — 四个「作战」（`[operation data set]`）

`[operation data set]` 在记录 10 / 24 / 38 / 52 出现，各带一组同名子列；`parent` 字段把子列归属到对应作战块。

| 列 | 作战① (行 10) | 作战② (行 24) | 作战③ (行 38) | 作战④ (行 52) |
| --- | --- | --- | --- | --- |
| `[index]` | 1 | 2 | 3 | 5 |
| `[type]` | 1 | 2 | 3 | 5 |
| `[card Symbol Index]` | 7 | 0 | 0 | 0 |
| `[member limit]` | `$party$` | `$party$` | `$party$` | `$party$` |
| `[recommend fame]` | 98171 | 105881 | 105881 | 73993 |
| **`[allow coin]`** | **`-1, 8`** | —（无此列） | — | — |
| `[type fixed value]` | 5 | 5 | 5 | 5 |
| `[reward data]` | `$normal$`, `10421367`, 1, **1000000**, 1, 1 | `$expert$`, `10421369`, 1, 1000000, 2, 1 | `$master$`, `10421369`, 1, 1000000, 1, 1 | `$match$`, `10421365`, 1, 1000000, 1, 1 |
| `[ting reward data]` | `$normal$`, 10421367, 1, 1000000, 1, 0 | `$expert$`, …, 2, 0 | `$master$`, …, 3, 0 | `$match$`, …, 3, 0 |
| `[string data]` | `Apocalypse_Difficulty_Name_01/Info_01/Explain_01` | `_02` | `_03` | `_01`（仅 Name） |
| **`[gate schedule]`** | `900, 1, 0, 0, 0` | `300,3,1,2,0, 240,3,1,0,0, 180,3,0,0,0` | `300,3,1,2,4, 255,3,1,2,0, 240,3,1,0,0, 180,3,0,0,0` | `900, 1, 0, 0, 0` |
| `[gate close warning]` | — | `270`/`Warning_02_01.ani`, `210`/`Warning_02_02.ani` | `285`/`_01`, `270`/`_02`, `210`/`_03` | — |
| `[gateflow]` | `1,2,3,-1,-1` | `1,3,4,5,-1, 2,4,5,-1,-1, 3,2,3,4,5, 4,5,-1,-1,-1` | 同作战② | `1,2,3,-1,-1` |
| **`[phase info]`** | `0, 90,1, 300,2, 300,3, 300,4, 600,5, 600` | 同左 | 同左 | 同左 |

**要点（对实现的直接影响）**

1. **阶段时钟与作战、难度都无关**：四组 `[phase info]` 取值完全一致
   ⇒ 阶段 1..6 = `90 / 300 / 300 / 300 / 600 / 600`（首项 `0` 是阶段 0 的占位）。
   **P4 可直接按此实现**，不再是转述值。
2. **作战只有 4 个**，`index/type` = `1, 2, 3, 5`（**没有 4**）；难度名来自
   `[string data]`（`_01 normal` / `_02 expert` / `_03 master`）与 `[reward data]` 首列
   （`normal/expert/master/match`）—— 即**难度是另一维度**，作战① = normal、② = expert、
   ③ = master、④ = `match`（可能是「匹配/挑战」模式）。
3. **`[allow coin]` 只在作战① 出现，值 `-1, 8`**（`flags=2`）。其余作战**没有该列**。
   ⇒ P6 的「作战级投币限制」要按列存在性处理，不能假定每作战都有。
4. **门禁时刻有真源**：作战① = 900 s 开 1 门（无关闭警告）；作战② = 300 / 240 / 180 s
   三段、最多 3 门；作战③ = 300 / 255 / 240 / 180 s 四段；配合 `[gate close warning]`
   提前警告（270 / 210 / 285 秒）与对应 `.ani` 路径。
5. **奖励物品 id 真源**：`10421367`（normal）/ `10421369`（expert、master）/ `10421365`（match），
   其后为数量与金币（`1000000`）。

### 5.3 `dungeonskillinfo.ctp` — 职责（散兵 / 守卫）

| 记录 | 列 | parent | 值 |
| --- | --- | --- | --- |
| 0 | `[skirmisher info]` | — | 子列：`[effect]`→8、`[jump attack]`→5、`[jump z threshold]`→1、`[nomal attack]`→2 |
| 1 | `[jump z threshold]` | 0 | `1` |
| 2 | `[nomal attack]` | 0 | 子列：`[delay ani time]`→3、`[relative offset]`→4 |
| 3 | `[delay ani time]` | 2 | `300` |
| 4 | `[relative offset]` | 2 | `-150, 0, 0` |
| 5 | `[jump attack]` | 0 | 子列：`[delay ani time]`→6、`[relative offset]`→7 |
| 6 | `[delay ani time]` | 5 | `50` |
| 7 | `[relative offset]` | 5 | `-100, 0, -35` |
| 8 | `[effect]` | 0 | 8 个空串（占位） |
| 9 | `[passive object index]` | — | 子列：`[gaurdian]`→10、`[role enable effect]`→12、`[role get effect]`→11、`[skir chainline effect]`→13 |
| 10 | `[gaurdian]` | 9 | `109133363` |
| 11 | `[role get effect]` | 9 | `109133366, 109133361` |
| 12 | `[role enable effect]` | 9 | `109133367, 109133368` |
| 13 | `[skir chainline effect]` | 9 | `109133397` |

⇒ **职责 = 散兵（skirmisher）/ 守卫（gaurdian）**，各自的被动对象索引、普通攻击
（delay 300 ms、偏移 `-150,0,0`）、跳跃攻击（delay 50 ms、偏移 `-100,0,-35`）、
跳跃 z 阈值 `1`、启用/获得效果 id 全部有真源。**P3 的职责选择与 P4 的战斗参数直接可用。**

---

## 6. 服务端读器实现要点（Go）

* 只需实现**分支 B**（二进制表），无视文本分支；
* 全部小端；头部 36 字节；记录固定头 44 字节；
* 用 `record_count` 驱动遍历并**断言**：遍历条数 == `record_count`、
  记录区止 == 尾表起点（这是格式校验的最强断言，两文件均通过）；
* 池引用用 `(min, max)` 区间切片（`hi` 独占），比客户端「读到 NUL」更强；
* **不要**依赖头部 `0x14` / `0x1c` 两个偏移字段的绝对值（实测各差 4），
  或统一 +4 校正；
* 拒绝策略：版本 ≠ 1、cell 载荷越界、`n_cells/n_refs` 超阈值、
  记录名无法解析（应视为格式漂移）⇒ 拒绝装载，不猜。

---

## 7. 仍未闭环（不影响 P3–P6）

| 项 | 说明 |
| --- | --- |
| `flags` 精确语义 | 观测到与「单值列 / 表列」强相关（1 / 2），但未从客户端代码确认 |
| 头部 `0x0c = 32`、`0x14/0x1c` 的 −4 偏差成因 | 疑与某个 4 字节前缀/对齐有关；不影响解析（已用 `record_count` 与池起点替代） |
| 文本分支（松散模式）的值语法 | 三个子解析器已导出未读；发布版不走此路径 |
| `[allow coin]` 的 `-1, 8` 字段语义 | 值有真源（作战① 独有），但两字段含义未从客户端消费点确认 |
| `[string data]` 的 `20:` 前缀 | `tag2` 的 u32 字段语义未确认（观测值 20） |

---

## 8. 复现命令

```bash
# 解出 .ctp 并打印真值表
python analysis/tasks/next69-ctp-extract.py \
  --file <apocalypse.ctp.bin> --out analysis/tasks/next69-apocalypse-ctp.json
```

导出 `.ctp` 原始字节（PVF 内）：
`go run ./cmd/pvfinspect -source ../client-build/Script.inner.pvf -files "<路径>" -output <目录>`

---

## 9. 证据文件索引

| 文件 | 内容 |
| --- | --- |
| `analysis/tasks/next69-ctp-extract.py` | 权威读器 + 硬断言 + JSON 导出 |
| `analysis/tasks/next69-apocalypse-ctp.json` | 65 条记录 + 7 条尾表项 + 池文本 |
| `analysis/tasks/next69-dungeonskillinfo-ctp.json` | 14 条记录 + 2 条尾表项 |
| `analysis/dumps/va-decompile/ctp_*.c` | 加载链全部伪代码（分派/装载/段读器/cell 读器/池/尾表） |
| `analysis/dumps/CLIENT-MECHANICS.md` §6 | 客户端机制速查（面向其他开发） |
