# next69 — `.ctp` 格式取证（两轮：加载链拓扑更正 + 二进制 cell 解码）

> 目标：解开 `contents/2026/apocalypse/etc/apocalypse.ctp` 的记录编码，使 P4 的阶段时限、
> `[allow coin]`、`[operation data set]`、`[reward data]` 等数值有真源，不再写死。
> 状态：**cell 编码 / 串池 / 名字引用 / 记录流 / 尾部列索引已闭环；
> 阶段时钟、门禁时刻已取到真源**；仅剩值区索引头与头部字段语义未定（见 §9.6）。
> 相关台账：`next64` §6.2 **X2**。

## 1. 加载链（**修正版**：两个并列解析器，按资源形态分流）

`sub_1474CDEA0` = `RDARScriptBuilder::build`（错误串实证：`use after RDARScriptBuilder::initialize()`、
`[BUILDER] %s is not exist`、`[BUILDER][LOCALIZE] %s is already localize path(getScript)`）。
它把路径后缀与常量比对，常量已解密确证 = **`.ctp`**（`0x14B278D48`）：

| 分支 | 触发 | 链路 | 结果形态 |
| --- | --- | --- | --- |
| **A 松散/脚本模式** | 后缀 ≠ `.ctp`，或 `dword_14DC91508 != 0` | `sub_1474CA960` → `sub_147C49DA0`(读文件) → `sub_146E96930`(BOM/代码页 → 宽字符) → `sub_1474CA940`(实为 `sub_1474CC1C0` 的 5 行包装) → **`sub_1474D02D0` 文本词法器** | 文本表，vtable `off_14B292410` |
| **B 打包模式** | 后缀 = `.ctp` 且 `dword_14DC91508 == 0` | `sub_147C484A0` → `sub_147C48890`（按名取挂载对象）→ **`sub_1474D11A0`** → **`sub_1474D0C00`（段）→ `sub_1474D0580`（cell）** | 二进制表，vtable `off_14B292468` |
| 取值（两分支共用） | — | `sub_1474CCCD0(table, name, row)` / `sub_1474CCBD0` / `sub_1474CCB60`；行数 `sub_1474D02A0` = `(+88 − +80) / 40` | — |

> **⚠️ 上一轮结论更正**：`.ctp` 后缀命中**分支 B（二进制）**；`sub_1474CA960 → sub_1474CC1C0
> → sub_1474D02D0` 属于**分支 A（文本）**。两者共用表构造器 `sub_1474CC1C0`，但入口与解析器不同。
> 文本词法器的错误串是 `' (at Ln: %d, %d)'`（行/列），**只服务松散模式**；
> 发布版的 `.ctp` 由 `sub_1474D0C00` / `sub_1474D0580` 解析（见 §9）。

副产物：`sub_146E96930` 是**编码转换**——`EF BB BF` → UTF-8（`sub_146E8FF70`）、
`FF FE` → UTF-16LE 直拷（`sub_146E8D510(out, ptr+2, (len−2)/2)`）、无 BOM → 系统代码页
（`sub_146E90480` → `sub_146E904A0(dst, src, CodePage)`）。

## 2. 主解析器揭示的语法（关键）

`sub_1474D02D0` 的循环体把当前字符（u16）拿去 `switch`：

| 字符码 | 字符 | 分支处理 |
| --- | --- | --- |
| `0` | NUL | **结束**（`return 1`） |
| `9` / `13` / `32` / `44` | `\t` `\r` 空格 `,` | 跳过 |
| `10` | `\n` | 记录一行（往 `a1+40` 的 vector 追加 `{offset, lineNo}`） |
| `43` `45` `48`–`57` | `+` `-` `0`–`9` | `sub_1474CF1D0` → **数字** |
| `47` | `/` | `sub_1474CEDB0` → **注释** |
| `60` | `<` | `sub_1474CFAA0` → **`<...>` 标注** |
| `91` | `[` | `sub_1474CFDF0` → **`[...]` 标签** |
| `96` | `` ` `` | `sub_1474CF6E0` → **反引号字符串** |
| 其它 | — | `sub_1474CF080` → 标识符/关键字 |

**⇒ 这套语法属于 PVF script 的文本语法**（标签、注释、数字、字符串、反引号串），
对应**分支 A**（松散模式的 `.ctp` 脚本源）。标签名与 schema 吻合（`[party waiting area]`、
`[phase info]`、`[allow coin]` 等），说明**发布版的二进制 `.ctp` 是这些脚本的编译产物**。

### ⚠️ 方向修正（两轮之后的最终口径）

- 第一轮把 `sub_1474D02D0` 当成了 `.ctp` 的解析器 —— **错**，它是分支 A（文本源）的词法器；
- 第二轮据此推出「`.ctp` 是文本、二进制假设作废」—— **也错**。`sub_1474CDEA0` 的后缀分派+
  `dword_14DC91508` 门控证明：**发布版 `.ctp` 走二进制分支 B**（§9），文本分支只用于松散模式；
- **`[u32 tag][payload]` 的 cell 假设成立**，只是 tag→长度表此前没取到（现已从 `sub_1474D0580` 取全，见 §9.1）。

## 3. 解析后的运行时结构

| 结构 | 形态 |
| --- | --- |
| 表对象 | 136 字节；`+40` 与 `+96` 两个 `map<wstring, …>`（分别是"行表"与"列表"） |
| 值索引 | **16 字节条目** `{u32 kind; u32 pad; u64 index}`（`>> 4` 计数，见 `sub_1474CB530`） |
| 节点树 | **120 字节/节点**（`/120` 计数）：`+0` 串指针/内联、`+16` 长度、`+24` SSO 容量、`+32` 节点类型、`+40` 子 map、`+56` 子 vector、`+104` 链接 |
| 类型池 | `sub_1474CCD60` 按 kind 选池：**kind 0/5 → 40 B 元素；kind 1 → 48 B；kind 2/3/6/7/9 → 32 B**；`case 4` 缺失（值内联） |

## 4. 已拿到的确定事实（可直接用于文档/工具）

- **表名真源**：`Contents/2026/Apocalypse/Etc/DungeonSkillInfo.ctp`
  （解密串 `0x1492E08D0`）；`.ctp` 后缀常量在 `0x14B278D48`。
- **`apocalypse.ctp` schema = 21 个标签**（字符串段 8300..9280 实测）：
  `[party waiting area]`、`[role per member limit]`、`[member]`、
  `[keldon xavi final damage rate]`、`[collaborate attack groggy duration increase per keldon xavi stack]`、
  `[guardian hp increase per keldon xavi stack]`、`[skirmisher monster hp reduction per keldon xavi stack]`、
  `[operation data set]`、`[allow coin]`、`[card Symbol Index]`、`[gate schedule]`、`[gateflow]`、
  `[index]`、`[member limit]`、`[phase info]`、`[recommend fame]`、`[reward data]`、
  `[string data]`、`[ting reward data]`、`[type fixed value]`、`[type]`、`[gate close warning]`
- **`dungeonskillinfo.ctp` schema = 12 个标签**：`[skirmisher info]`、`[effect]`、`[jump attack]`、
  `[jump z threshold]`、`[nomal attack]`、`[delay ani time]`、`[relative offset]`、
  `[passive object index]`、`[gaurdian]`、`[role enable effect]`、`[role get effect]`、
  `[skir chainline effect]` ⇒ **职责 = 散兵 / 守卫**的字段定义就在这里。
- **头部（64 字节）字段值**（`apocalypse.ctp` / `dungeonskillinfo.ctp` 对照）：
  `0x00`=1 / 1（版本）、`0x04`=65 / 14、`0x0c`=32 / 32、`0x14`=8052 / 1372（数据区止）、
  `0x1c`=8300 / 1436（字符串区始）、`0x2c`=20 / 17、`0x34`=1 / 2、`0x38`/`0x3c`=-1 / -1。

## 5. 仍未闭环的部分（更新版，详见 §9.6）

| 缺口 | 说明 |
| --- | --- |
| ~~文件字节 → 宽字符流的转换~~ | **已澄清**：发布版 `.ctp` 不转换——它走**二进制分支 B**（§9.1）；转换只发生在松散模式的文本分支 A（`sub_146E96930`） |
| ~~尾部元数据数组结构~~ | **已闭环**：`(名字引用) + count + count×u64` 的列索引（§9.4） |
| ~~组 → 难度 的对应~~ | **已澄清**：4 组是 4 个**作战**（行号与 `[operation data set]` 一致），难度是另一维度（§9.3） |
| 值区**索引头**语义 | 表格列值区开头的若干 u64（§9.6）—— 影响"精确取值"，但**不影响**阶段时钟/门禁时刻的可读性 |
| 头部字段语义 | `65 / 32 / 20 / 1` 未定 |
| 松散模式的值文本语法 | `sub_1474CFDF0`/`CF1D0`/`CF6E0` 未读；仅在需要重建"脚本源"时才有意义 |

## 6. 对 P4/P6 的影响（更新版）

- **阶段时钟已有真源**：4 个作战一致 = `90/300/300/300/600/600`，由
  `analysis/tasks/next69-ctp-extract.py` 从 `apocalypse.ctp` 直接解出（`[phase info]` 记录）。
  **P4 可以按这个值实现**，并在代码注释里标注真源 + 提取脚本。
- **门禁时刻也有真源**：`[gate schedule]` 实测 `300 / 240 / 180`，`[gateflow]` 给出阶段间流转 ⇒
  P4 的"门/阶段推进"不再全靠猜。
- **`[allow coin]` 的行号可读**（标量列值区 = `0, 值`），但该列在 `apocalypse.ctp` 里存的是
  **行号**而非开关值——真正的开关语义要连带 §9.6 的索引头一起定，**在闭环前不当已还原**。
- `.ctp` 是**独立于 PVF `data_type` 的另一套容器**（自带 64 字节头 + 记录流 + 池 + 尾索引），
  服务端要读它，需要在 `internal/catalog/pvf` 之外**新写一个 `.ctp` 读器**（可照 §9.1/§9.2/§9.4 实现，
  并用 `next69-ctp-extract.py` 的输出做对照测试）。

## 7. 下一步（按优先级，第三轮更新）

1. **闭合 §9.6 的值区索引头**：把"表格列值区头几个 u64"的语义定下来 —— 它决定精确取值
   （`[allow coin]` 开关、`[reward data]` 奖励表）。入手点：`sub_1474D0C00` 里
   `u32 → node+32` 与 `u64(≠−1) → node+104` 的用途；
2. 然后按 §9.1/§9.2/§9.4 给服务端写 `.ctp` 读器（Go），以 `next69-ctp-extract.py` 输出为准做对照；
3. 头部 4 个未知字段（65 / 32 / 20 / 1）随后自然可解释。

---

## 8. 追加进展（同日，含一条实测反证）

### 8.1 内容读取链已追到终点

`sub_147C49DA0`（② 读入）内部结构：

- **只做路径解析与错误上报**，两条路径（`dword_14DC91508 == 0` 与 `!= 0`），
  最终都落到 `sub_147C4A1F0(...)` 拿"文件对象"；
- 内容由 `sub_146F35360(obj, out)` 取出，而它的实现只有一句：

  ```c
  sub_146E8D340(a2, (void *)(a1 + 16));   // 复制 obj+16 处的字符串
  return a2;
  ```

  ⇒ **内容字符串存在文件对象的 `+16`，转换不在这一层**。

**下一步点**：反编译 **`sub_147C4A1F0`** —— 它是"取文件对象"的入口，
转换（解码/解压/编码扩展）应在这里或其下游。

### 8.2 实测反证：不能把"字节直接当 wchar 读"

对 `apocalypse.ctp` 全文的编码特征扫描：

| 特征 | 出现次数 |
| --- | --- |
| `5B 00`（UTF-16 的 `[`） | **0** |
| ASCII `[` | 22（**首个在 0x2071**，即字符串段 8300 起） |
| UTF-16 CRLF（`0D 00 0A 00`） | **0** |
| ASCII CRLF（`0D 0A`） | **0** |
| `00 00 00 00` | 1146 |

data 区按 u16 读 = `[4, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0, 0]`，是控制字符而非文本。

⇒ 若把文件字节直接当宽字符流喂给 §2 的 lexer，**第一个 `0` 就会让它 `return 1`（结束）**，
不可能解析出 21 个标签。**转换确实存在**，必须先在 `sub_147C4A1F0` 处闭环。

### 8.3 沉淀产出（其他开发可直接复用）

- **`analysis/dumps/CLIENT-MECHANICS.md`**（新建，已随 README §4 挂索引）：
  字符串解密、C2S 发包器 + 期望回包树、S2C 游标 API 与**长度硬契约**、handler 注册表、
  13 字节前导、`.ctp` 加载链与两个 schema、MSVC 容器判别、军团族包契约速查、
  **7 条已踩过的坑**、工具与脚本索引 + IDA 批处理调用方式。
- **文档仓库**（`analysis/tasks/`，随 git 提交）：`next64`（分期 + 妥协台账）、
  `next65`（包体字段表）、`next66/67/68`（P1/P2/P3）、`next69`（本文件）。

## 9. 追加进展：二进制 `.ctp` 格式已解码（同日第二轮，IDA 实证）

### 9.1 cell 编码（**tag → 载荷长度**，来自 `sub_1474D0580`）

```c
v8 = *(u32*)(base + cursor) - 1;   // 读 tag，减 1 进 switch
switch (v8) { case 0: … case 6: … }
```

| tag | 载荷 | cell 总长 | 语义 |
| --- | --- | --- | --- |
| 1 | 2 × u64 | 20 | **名字引用**（`(lo,hi)` → 串池区间） |
| 2 | u32 + 2 × u64 | 24 | 带索引的名字引用 |
| 3 | u8 | 5 | 字节 / 布尔 |
| **4** | u64（f64 位型） | **12** | **float64 —— 玩法数值在这儿** |
| 5 | u64 | 12 | 同宽另一类型（整型/引用） |
| 6 / 7 | u32 + u8 | 9 | （u32, byte）对 |
| 其它 | 0 | 4 | **填充 cell**（占位，可自我对齐） |

段读器 `sub_1474D0C00(base,end,&cursor,table,rowIdx)` 的顺序：
`u64, u64`（→ 名字引用，经 vtable[0] 解析）→ `u32` → `u64`（若 ≠ −1 存到 row+104）→
`u64`（分支 A：调 `sub_1474D0580` 这个次数）→ `u64`（分支 B：行数）。
段结构体 = **120 字节**；cell 条目 = **40 字节**（`sub_1474D02A0` = `(+88−+80)/40`）。

### 9.2 串池与名字解析（`sub_1474CDD90`）

```c
v4 = *(table + 136);              // 池容器 {ptr, ?, len, cap}
lo = min(a,b); hi = max(a,b);
if (hi <= v4[2]) v9 = (char*)v4 + 2*lo;   // ← 按字符取，*2 说明池是宽字符
assign_wstr(out, v9);                     // 读到 NUL 为止
```

- 文件里的池 = 尾部 **ASCII 拼接块（无分隔符）**，**基址 `0x2071`**（该字节前一个字节是 NUL 分隔符）；
- 引用 = `(lo, hi)`，**0 基、`hi` 独占** ⇒ `hi − lo` **正好等于字符串长度**（实测：
  `(0,20)`=`[party waiting area]`、`(51,82)`=`[keldon xavi final damage rate]`(31)、
  `(82,149)`=`[collaborate…stack]`(67)）——这比客户端自己的"从 min 读到 NUL"更强，
  服务端可直接按区间切；
- 客户端用 `pool + 2*lo` 说明装载时池被**加宽为 UTF-16**（`sub_1474D0A00` 把「游标之后的
  剩余内容」整体转宽，`sub_146E8FEF0`）；
- `apocalypse.ctp` 池 = 975 字节，含 **22 个 `[...]` 标签** + 3 档难度名
  （`Apocalypse_Difficulty_Name_01 normal` / `_02 expert` / `_03 master`）+ 3 条动画路径。

### 9.3 阶段时钟：**4 个作战，取值一致**（§9.3 修订）

用 `analysis/tasks/next69-ctp-extract.py` 按**名字引用锚定**解析（不依赖易退化的 tiling）：
`[phase info]` 共 4 条记录，全部是

```
[0.0, 90.0, 1.0, 300.0, 2.0, 300.0, 3.0, 300.0, 4.0, 600.0, 5.0, 600.0]
```

即 **(90,1)(300,2)(300,3)(300,4)(600,5)(600)** = 阶段 1..6 时限 `90/300/300/300/600/600`，
首项 `0.0` 是阶段 0 的占位。

4 条记录的行号 `10 / 24 / 38 / 52` 与尾部元数据 `[operation data set]` 的 4 个值**完全一致**
⇒ **这 4 组是 4 个「作战」（operation），不是 3 档难度**（池里的 normal/expert/master 是另一维度）。
⇒ **阶段时钟与作战、难度都无关** = `90/300/300/300/600/600`，P4 可直接使用并有真源。

同一记录族还给出门禁数据（`[gate schedule]` 实测 `300,240,180` 三个开放时刻；
`[gateflow]` 给出阶段间流转）。

### 9.4 记录流与尾部元数据（**已闭环**）

**cell 区 = 记录序列**，每条记录 = `20 字节名字引用 + 值区`，**下一条名字引用即记录边界**：

```
[20B: u64 lo, u64 hi(名字区间)] [值区]
```

- 值区是 u64 序列：**标量列** = `0, 值`（如 `[allow coin]` → `0, 17`）；
  **表格列** = 索引头 + 若干 12 字节 f64 cell（`u32 tag=4` + `f64`）；
- 名字引用扫描**不依赖 tiling**：把每个 4 字节位置上的 u64 对拿去做「区间 → 池标签」精确匹配，
  `apocalypse.ctp` 命中 **116 处**，且分布严格重复（每行一套列顺序）⇒ 可交叉验证；
- `dungeonskillinfo.ctp` 同样命中 25 条记录（此前 DFS tiling 在该文件退化，无法解析）。

**尾部元数据 = 列索引**（`0x1F78..0x2070`，248 字节），记录格式：

```
u64 lo, u64 hi(名字区间), u64 count, count × u64
```

- `apocalypse.ctp` = **7 条记录，恰好占满 248 字节**，每条 `hi − lo` 都等于标签长度
  （脚本输出 `ok` 逐条校验）：

| 名字 | values |
| --- | --- |
| `[collaborate attack groggy duration increase per keldon xavi stack]` | 1 个（7） |
| `[guardian hp increase per keldon xavi stack]` | 1 个（8） |
| `[keldon xavi final damage rate]` | 1 个（6） |
| **`[operation data set]`** | **4 个（10, 24, 38, 52）** |
| `[party waiting area]` | 1 个（0） |
| `[role per member limit]` | 1 个（1） |
| `[skirmisher monster hp reduction per keldon xavi stack]` | 1 个（9） |

⇒ 这几个值就是各列的**行号**（`[operation data set]` 的 4 个行号 = `[phase info]` 4 条记录的行号）。

### 9.5 本轮新增可复用产物

| 产物 | 用途 |
| --- | --- |
| `analysis/tasks/next69-ctp-extract.py` | 解 `.ctp`：池基址 + **名字引用锚定的记录解析**（不依赖 tiling）+ 尾部列索引（逐条 span 校验）+ 阶段时钟；带硬断言 |
| `analysis/tasks/next69-apocalypse-ctp.json` / `next69-dungeonskillinfo-ctp.json` | 两个 `.ctp` 的结构化导出（含 `records` / `trailer` / `phase_groups`） |
| `analysis/dumps/va-decompile/ctp_*.c` | 二进制分支全部函数的伪代码（cell/段/池/解析器/名字解析） |

### 9.6 仍未闭环

| 缺口 | 说明 |
| --- | --- |
| 值区**索引头**语义 | 表格列值区开头的几个 u64（如 `[phase info]` 的 `10, 0, 12, 0, 0, 0`）含义未定；当前实现按"能走 12 字节就走 f64 cell，否则吃 8 字节"的规则读，实测能对齐到记录尾 |
| 头部字段语义 | `0x00`=1、`0x04`=65、`0x0c`=32、`0x14`=8052（cell 区止）、`0x1c`=8300；65 / 32 / 20 / 1 未定 |
| 段头（`sub_1474D0C00` 的 `u32` / 首个 `u64(≠−1)`） | 读了但用途未追 |
| tiling（DFS） | 仅用于 tag 直方图；**浮点 cell 少时会退化**（`dungeonskillinfo.ctp` 只有 3 个 float），
判定记录请用名字引用锚定路径 |
