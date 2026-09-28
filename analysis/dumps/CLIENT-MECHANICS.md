# 客户端通信机制与真值索引（可复用）

> **用途**：新任务直接从这里定位，不必重新逆向一遍。
> **维护**：每次取证有新发现就追加；所有地址都是 115 级客户端 `client/DFO.exe` 的 VA
> （权威 IDB `client/DFO.exe.i64`）。
> **来源**：2026-09-23 军团频道 / 末世录（`analysis/tasks/next64`–`next69`）取证沉淀。

---

## 1. 字符串解密（先用这个，能省一半时间）

客户端所有字符串常量都在 `.rdata` 里加密，代码用 `sub_146E8C7D0(&unk_VA)` 取明文。

| 项 | 值 |
| --- | --- |
| 入口 | `sub_146E8C7D0(加密串VA)` → 明文；实现 `sub_146E8C490`（带 FNV-1a 缓存） |
| 头部 | `byte1 & 0xFE` = 密钥种子；`u16@+2` = 长度码；正文 @+4 |
| 长度 | `length = 2 * (code ^ (seed8 | seed8<<7))`（multiplier = 2） |
| 密钥 | `key = seed8 | 0x9A714CA0` |
| 正文 | 逐 dword：`plain = cipher ^ key; key = (plain + 65599*key) & 0xffffffff`；尾字节用 `263` |
| 编码 | **明文是 UTF-16LE** |
| 现成字典 | `analysis/dumps/xorstr_addr_to_text.json`（114,614 条 VA→明文） |
| 离线解码脚本 | `analysis/tools/pe_decode_xorstr.py`（`decode(raw, 2)` 可复用于任意 VA） |

**实测**：用它解出的 17 个 opcode 枚举名与 `opcodes.tsv` **15/15 完全一致**
⇒ 该 dump 的命名可信，不是猜的。

---

## 2. C2S 发包（客户端 → 服务端）

```c
w = sub_146D74000();            // 取全局 writer
sub_146D746E0(w, opcode);       // 写 opcode（并写 13 字节前导，见 §5）
w = sub_146D74000();
sub_146D75B10(w, &buf, len);    // 追加 len 字节
sub_146D75AF0();                // 发送
sub_140DD8720(opcode);          // 在「期望回包树」登记
```

**期望回包树**：全局 `qword_14E652C80`，以 **opcode 为键的红黑树**（节点 `+0x1c` 是键，
`+25` 是"空/哨兵"标志）。handler 收到包时先 `sub_1403A7590(&qword_14E652C80, &id)` 把它**清掉**；
**清不掉就整个 handler 跳过** ⇒ **服务端必须回同 id 的包**，否则客户端卡在等待态。

---

## 3. S2C 收包与读取游标（**长度是硬契约**）

**唯一绑定点** `sub_146D74A80`（客户端收包循环）：

```c
sub_146EA2160(node + 16);              // 游标指针 = 包节点 + 16
sub_146EA2170(*(u32*)(node + 3) - 16); // 游标长度 = 节点 @+3 的长度 − 16
sub_1459A1BB0(路由对象, a2, 包);          // 按包首字节分派，见 §4
```

⇒ 客户端帧结构：`flag(1B) | id(u16) | 包长(u32) | …padding… | payload@+16`。
**flag==0（通知）时 handler 读到的第 0 字节 = payload 起点** ⇒ 这类 S2C **body 就是
payload，不含任何信封**。⚠️ **flag==1（命令回包）不是**：分派器先替处理器读掉体首的
**u8 状态字节**，处理器从 `body+1` 开始读自己的负载（§4）。

| 读取 API | 语义 | 越界行为 |
| --- | --- | --- |
| `sub_146EA2160(ptr)` | 设游标指针 | — |
| `sub_146EA2170(len)` | 设剩余长度 | — |
| `sub_146EA1540()` | 取游标 | — |
| `sub_146EA09F0(&u8)` | 读 1 字节 | 剩余 <1 → 写 0 |
| `sub_146EA1920(&u16)` | 读 2 字节 | 剩余 <2 → 写 0 |
| `sub_146EA0BA0(&u32)` | 读 4 字节 | 剩余 <4 → 写 0 |
| `sub_146EA0BE0(dst, n)` | 读 n 字节 | 剩余 <n → **`MEMORY[0]=0`，客户端崩** |
| `sub_146EA40F0(n)` | 剩余 >= n ? | — |
| `sub_146EA0340(n)` | 消耗 n 字节 | 剩余 <0 → 从备份缓冲回滚 |

> ⚠️ **`sub_146EA0BE0` 是最容易踩的坑**：它**不是**"把结构推给 UI"，而是**从包体读**。
> 有些资料会把它误读成写出，「回个空包」在这种 handler 上会直接崩客户端。

---

## 4. handler 注册表：**帧首字节决定查哪张表**

| 族 | 注册函数 | 注册表全局 | 查询它的分发 |
| --- | --- | --- | --- |
| CMD | `sub_14599D450(?, id, handler)` → `sub_1459A2FB0(qword_14E6836F8, id, handler)` | `qword_14E6836F8`（惰性建 136 B 表） | **帧首字节 == 1**：`sub_14599D200` → `sub_1459A2D70(表, id, 状态字节, u16 错误码)` |
| NOTI | `sub_14599D5D0(?, id, handler, flags)` → `sub_1459A3DD0(qword_14E683700, id, handler, flags)` | `qword_14E683700`（同样惰性建 136 B 表） | **帧首字节 == 0**：`sub_14599D380` → `sub_1456B8AB0(id, 包)`（通知总线，不查 CMD 表） |

⇒ 分派器 `sub_1459A1BB0(路由对象, a2, 包)` 先看 `*包`（= §3 信封的第 0 个字节）：
`0` 走通知总线、`1` 走 CMD 表，**其它值只记遥测错误，不分发**。
两个数字就是服务端 `wire.ServerFrame(kind, …)` 的 `kind`：
**状态类通知发 kind 0，命令回包发 kind 1**（本仓库已实机的 kind 1 回包：
`booster_open_ack` 160、`settlement_focus_ack` 72、`awakening_completed` 2177）。

⇒ kind 1 的**体首还有一个 u8 状态字节**（`sub_14599D200` 在查表前用
`sub_146EA09F0` 读掉它；只有状态 == 0 时才再读一个 u16 错误码），
处理器签名 `handler(obj, 状态, 错误码)` 就是这两个值：状态为 0 时处理器通常只弹提示
（1565 的 `sub_1444E8D00` 即 `if (!a2) return sub_1444EC790(a3)`）。
所以命令回包的体 = **`u8 1` + 该命令自己的负载**，负载从游标 body+1 开始读
（`protocol.BoosterOpenSuccess` 就是 `add16([]byte{1}, 0)` 起手）。

⚠️ `qword_14E66C090` 不是注册表：`sub_1459A1BB0` 尾部只拿它做耗时遥测
（`sub_1459AC4C0`/`sub_1413085D0`）。

**技巧**：整族注册关系常集中在一个函数里显式列出（军团族见
`sub_1424FE600`），比按 id 逐个搜快得多。
⚠️ 不要用 `find_imm` 对每个 id 全盘扫 `.text`（150 MB，二次复杂度，会超时）——
改「只扫注册表调用点」。

---

## 5. C2S 13 字节前导（信封）

`sub_146D746E0` 在**首次发送**时构造并写入 13 字节：

```c
v25 = 1;  v26 = opcode;  v27 = 0;  v28 = 0;   // 1B + u16 + 8B + 2B = 13
sub_146D75B10(a1, &v25, 13);
```

⇒ **`01 | opcode(u16 LE) | 8 字节 0 | 2 字节 0`**。

**服务端口径**：仓库里 **5 个已实机验证**的协议模块一律 **跳过前 13 字节、从 `p[13:]` 读字段** ——
`internal/game/protocol/` 的 `awakening.go`（`Uint32(p[13:])`，注释明写 "prefix is opaque"）、
`advancement.go`（`p[13] & 0xF`）、`dungeon.go`、`unified_option.go`（`p[13], p[14]`）、
`world.go`。
**唯一残留不确定**：客户端 append 的长度是否已含信封 —— 首次实机看日志 `plain_hex` 一次定论。

---

## 6. `.ctp` 表：两个解析器 + 二进制 cell 编码

`sub_1474CDEA0` = `RDARScriptBuilder::build`，按后缀常量 **`.ctp`**（`0x14B278D48`，明文由 §1 解出）
+ 全局 `dword_14DC91508` **分流成两条完全不同的解析路径**：

| 分支 | 触发 | 链路 | 解析器 |
| --- | --- | --- | --- |
| **A 松散/脚本模式** | 后缀 ≠ `.ctp`，或 `dword_14DC91508 != 0` | `sub_1474CA960` → `sub_147C49DA0`(读文件) → `sub_146E96930`(BOM/代码页→宽字符) → `sub_1474CA940`(=`sub_1474CC1C0` 的 5 行包装) → **`sub_1474D02D0` 文本词法器** | 文本表，vtable `off_14B292410` |
| **B 打包模式** | 后缀 = `.ctp` 且 `dword_14DC91508 == 0` | `sub_147C484A0` → `sub_147C48890` → **`sub_1474D11A0`** → **`sub_1474D0C00`（段）→ `sub_1474D0580`（cell）** | 二进制表，vtable `off_14B292468` |
| 取值（共用） | — | `sub_1474CCCD0(table, L"[列名]", row)` / `sub_1474CCBD0` / `sub_1474CCB60`；行数 `sub_1474D02A0` = `(+88−+80)/40` | — |

**编码转换**（只在分支 A）：`sub_146E96930` — `EF BB BF`→UTF-8、`FF FE`→UTF-16LE 直拷、
无 BOM→系统代码页（`sub_146E90480` → `sub_146E904A0(dst, src, CodePage)`）。

### 6.1 二进制 cell（`sub_1474D0580`：`v = *(u32*)cursor - 1; switch(v)`）

| tag | 载荷 | cell 长 | 语义 |
| --- | --- | --- | --- |
| 1 | 2×u64 | 20 | **名字引用** `(lo,hi)` → 串池字符区间（字符串值） |
| 2 | u32 + 2×u64 | 24 | 带索引的名字引用 |
| 3 | u8 | 5 | 字节/布尔 |
| **4** | u64(f64) | **12** | **float64 —— 玩法数值** |
| 5 | u64(f64) | 12 | float64（第二种编码，实测同为 0.5 / 1.0 等） |
| 6 / 7 | u32 + u8 | 9 | (u32, byte) |
| 其它 | 0 | 4 | **填充 cell** |

### 6.2 记录流（**格式已闭环，两个文件独立验证**）

**记录固定头 = 44 字节**：`u64 名字lo, u64 名字hi, u32 flags, u64 parent, u64 n_cells, u64 n_refs`，
随后 `n_cells × cell` + `n_refs × ref`；`ref = u64 名字lo, u64 名字hi, u64 n, n × u64`。

* **`parent` = 父记录序号（0 基），`-1` = 顶层** —— 这是列归属关系（`[member]` 的
  parent = `[role per member limit]` 的记录号）；
* **文件级布局**：`0x00` 头部 36 字节（`u32 version, u32 record_count, …`）→ `0x24` 记录数组
  → 尾部索引表 → 1 字节 NUL → 串池（ASCII 拼接，字符 0 = 第一个 `[`）；
* **尾部索引表 = 「列名 → 该列所有行号」**：`u64 lo, u64 hi, u64 n, n × u64`；
  `[operation data set]` 的 4 个值就是四个作战块的**记录序号**（10 / 24 / 38 / 52）；
* **校验断言**（两文件均通过）：遍历条数 == 头部 `record_count`（65 / 14）、
  记录区止 == 尾表起点、尾表止 == 池起点、所有名字都能解析成可打印串。

串池与名字解析（`sub_1474CDD90` / `sub_1474D0A00`）：

```c
v4 = *(table + 136);                       // 池（宽字符串，装载时由 sub_1474D0A00 加宽）
lo = min(a,b); hi = max(a,b);
if (hi <= v4[2]) v9 = (char*)v4 + 2*lo;    // 按字符取
assign_wstr(out, v9);                      // 客户端读到 NUL
```

文件里的池 = 尾部 **ASCII 拼接块（无分隔符）**，**基址是第一个 `[`**（`apocalypse.ctp`
= `0x2071` = 头部 `0x1c` 值 + 5）；引用 `(lo, hi)` 是 **0 基、`hi` 独占**，
`hi − lo` = 字符串长度（实测精确吻合），**服务端按区间切即可，不必依赖客户端的 NUL 语义**。

> 头部 `0x14`（cell_end）与 `0x1c`（trailer_end）两个偏移字段实测都**比真实边界小 4 字节**，
> 不要直接用其绝对值；用 `record_count` + 池起点反推更稳。

### 6.3 真源真值（**P3–P6 可直接引用**）

`analysis/tasks/next69-ctp-extract.py` 解出（完整表见 `analysis/tasks/next69-ctp-format.md` §5）：

**阶段时钟（P4）** —— 四条 `[phase info]` 取值完全一致：
`0, 90,1, 300,2, 300,3, 300,4, 600,5, 600`
⇒ **阶段 1..6 = 90 / 300 / 300 / 300 / 600 / 600**，**与作战、难度都无关**。

**四个作战**（`[operation data set]` 记录 10 / 24 / 38 / 52）：

| 列 | 作战① | 作战② | 作战③ | 作战④ |
| --- | --- | --- | --- | --- |
| `[index]` / `[type]` | 1 | 2 | 3 | 5 |
| `[recommend fame]` | 98171 | 105881 | 105881 | 73993 |
| **`[allow coin]`** | **`-1, 8`** | 无此列 | 无此列 | 无此列 |
| `[reward data]` | `normal`, 10421367 | `expert`, 10421369 | `master`, 10421369 | `match`, 10421365 |
| **`[gate schedule]`** | `900,1,0,0,0` | `300,3,1,2,0, 240,3,1,0,0, 180,3,0,0,0` | `300,3,1,2,4, 255,3,1,2,0, 240,3,1,0,0, 180,3,0,0,0` | `900,1,0,0,0` |
| `[gateflow]` | `1,2,3,-1,-1` | `1,3,4,5,-1, 2,4,5,-1,-1, …` | 同作战② | `1,2,3,-1,-1` |

**职责表（P3/P4）** —— `DungeonSkillInfo.ctp`：散兵 `/` 守卫 两职责；
`[gaurdian]` 被动对象 `109133363`、`[role enable/get effect]` 各两个 id、
`[skir chainline effect] = 109133397`、普通攻击 delay `300 ms` / 偏移 `-150,0,0`、
跳跃攻击 delay `50 ms` / 偏移 `-100,0,-35`、`[jump z threshold] = 1`。

**已知 schema**（池实测）：

- `apocalypse.ctp` → **22 个 `[...]` 标签**：`[party waiting area]` `[role per member limit]`
  `[member]` `[keldon xavi final damage rate]`
  `[collaborate attack groggy duration increase per keldon xavi stack]`
  `[guardian hp increase per keldon xavi stack]`
  `[skirmisher monster hp reduction per keldon xavi stack]` `[operation data set]` `[allow coin]`
  `[card Symbol Index]` `[gate schedule]` `[gateflow]` `[index]` `[member limit]` `[phase info]`
  `[recommend fame]` `[reward data]` `[string data]` `[ting reward data]` `[type fixed value]`
  `[type]` `[gate close warning]`；另含 3 档难度名 `normal / expert / master`（`Apocalypse_Difficulty_*`）
  与 3 条 `Contents\2026\Apocalypse\Xui\Animation\ShortcutWarning\Warning_02_0*.ani*`。
- `DungeonSkillInfo.ctp` → **12 标签**：`[skirmisher info]` `[effect]` `[jump attack]`
  `[jump z threshold]` `[nomal attack]` `[delay ani time]` `[relative offset]`
  `[passive object index]` `[gaurdian]` `[role enable effect]` `[role get effect]`
  `[skir chainline effect]` ⇒ **职责 = 散兵 / 守卫**的字段定义在此。

**头部 36 字节**（两文件对照）：

| 偏移 | apocalypse.ctp | dungeonskillinfo.ctp | 含义 |
| --- | --- | --- | --- |
| `0x00` | 1 | 1 | 版本（≠1 拒绝装载） |
| `0x04` | 65 | 14 | **记录数**（与遍历精确一致） |
| `0x0c` | 32 | 32 | 两文件相同，疑为最大列数 |
| `0x14` | 8052 | 1372 | cell 区止 **−4** |
| `0x1c` | 8300 | 1436 | 尾表止 **−4**（池字符 0 = 此值 +5） |

⚠️ **仍未闭环（不影响 P3–P6）**：`flags` 精确语义（观测与「单值列/表列」强相关）；
头部 `0x0c = 32` 与两处 −4 偏差的成因；`[allow coin]` 的 `-1, 8` 两字段含义；
`[string data]` 的 `20:` 前缀。

---

### 6.4 NOTI2657 的分发链（**未闭环，但链条已画出**）

`NOTI2657 LEGION_PHASE_CLEAR_TICK` 的 144 B 块走一条三级链，读它的人要注意
**第二级不是"阶段子系统"本身**：

```c
// ① handler：从游标读 144 B（长度不够会崩，见 §3）
sub_1424FDF70:
    memset(buf, 0, 144);
    sub_146EA0BE0(buf, 144);
    return sub_142ABF650(qword_14E683C40, buf);

// ② 派发：红黑树查找 + 虚调用（+312 是树根字段，node[5] 是对象）
sub_142ABF650(a1, a2):
    key  = sub_142AB29C0(a1);
    node = rbtree_lower_bound(*(a1 + 312), key);
    obj  = node[5];
    if (obj) return (*(vtable_of(obj) + 1136))(obj, a2);   // slot 142

// ③ 键的算法：返回 7/10/17/20/21/27/28 这类**小整数**，取自一个 locale/language 访问器
sub_142AB29C0(a1):
    switch (WORD1(GetLocaleT(...)[7].mbcinfo)) { case 8: 7; case 9: 10; ... }
```

**关键推断**：键不像玩家 id 或阶段号，像**语言/地区/模式码** ⇒ `+312` 的 map 可能是
「按模式分的处理器表」，`qword_14E683C40` 也可能是通用的消息分发器而不是阶段状态机。
在解出 `vtable+1136` 那一端之前，**不要把 144 B 当阶段状态块使用**（服务端下发会喂错语义）。

**引用计数（`ida_phase_x7c.py`，纯 xrefs，秒级返回）——这两个数字最能说明问题**：

| 锚点 | 引用 | 被引函数数 | 含义 |
| --- | --- | --- | --- |
| `sub_1424FDF70`（2657 handler） | 2 | **1** | 只被注册表 `sub_1424FE600` 引用（登记点），无人调用 |
| `sub_142ABF650`（派发器） | 2 | **1** | **唯一调用者就是 2657 handler** ⇒ 这是一条**专用**入站路径，不是公共设施 |
| `sub_142AB29C0`（算键） | 230 | **193** | 用得这么广 ⇒ 它是**通用的「当前模式/区域」取值器**，不是阶段专用 |
| `qword_14E683C40`（被调对象） | 1547 | **640** | 极中心的全局管理器；`+312` 只是它的一个字段 |

⇒ 结论：**2657 是一条专用通道，但它指向的消费者是「按模式分派」的通用对象**。
所以 144 B 的字段顺序取决于当前模式，必须找到**向 `+312` map 插入对象的地方**（对象工厂/
注册点）才能拿到那一端；只靠猜 144 B 的语义会喂错。

**已证否的探针思路**：全盘扫立即数 `0x470`（1136）或 `312` 定位类 —— 这两个值太常见，
一次扫描命中数百个函数、解出几十个无关函数体，**信噪比极低**（本仓库踩过，见 §9）。
正解是「查派发器的调用者」+「查全局的读者」，用结构关系缩小范围；
而且这类**只查引用、不解伪代码**的探针要在秒级返回，别把解伪代码混进去（会在一个慢函数上卡住）。

---

## 7. MSVC 容器判别（读伪代码时很有用）

| 特征 | 含义 |
| --- | --- |
| `cap >= 8` → 取 `*ptr` | **`std::wstring` 的 SSO**（内联 7 个 wchar，堆则 `cap >= 8`） |
| `*(_QWORD*)x` / `x[1]` 相减 | `vector<T>` 的 `{begin, end}`；`(end-begin)/sizeof(T)` = 元素数 |
| `__wind` / `__unwind` / `__eh34_*` | MSVC 异常处理帧（不是逻辑分支，读时可以跳过） |
| `MEMORY[0] = 0` | 故意写空指针（断言/崩溃路径） |

---

## 8. 服务端实现契约速查（军团族，已实证）

| opcode | 名称 | C2S 布局（@13 起） | S2C body 最小长度 |
| --- | --- | --- | --- |
| 2043 | LEGION_START | `u32@13` | **≥4** |
| 2044 | LEGION_FAIL | `u32@13` | **≥8** |
| 2045 | LEGION_ENTER_DUNGEON | `u32@13`=107、`u32@17`=作战 id | **≥13**（全零 = 成功） |
| 2046 | LEGION_REWARD_END | `u32@13`、`u32@17`、`u8@21` | **≥13** |
| 2354 | LEGION_OPERATION_SELECT | `u32@13`=动作(1/2)、`u8@17`、`u32@18`=107 | **≥16**，首 `u16`=107 |
| 2355 | APOCALYPSE_ROLE_SELECT | `u32@13`=职责值、`u32@17`=107 | 0（裸转发） |
| 2252 | LEGION_BASIC_CLEAR_REWARD（NOTI） | 不发 | **≥7772** |
| 2253 | LEGION_ADDITIONAL_CLEAR_REWARD（NOTI） | 不发 | **≥2405** |
| 2254 | LEGION_ENTRY_CHARAC_INFO（NOTI） | 不发 | **≥256** |
| 2568 | PREPARE_LEGION_ENTER_DUNGEON（NOTI） | 不发 | **≥16** |
| 2657 | LEGION_PHASE_CLEAR_TICK（NOTI） | 不发 | **≥144** |
| 2895 | LEGION_INFO（NOTI） | 不发 | **≥206**，首 `u16`=107 |
| 2896 | LEGION_OPERATION（NOTI） | 不发 | **≥9**，首 `u16`=107 |
| 1474 | DUNGEON_TIMEOUT_TIME（NOTI） | 不发 | 取 handler 参数，不走游标 |
| 2290 / 2293 | VENUS_OPERATION_SELECT / END_AT_PHASE4 | `u32@13`(+`u8@17`) / 空 | — |

细节与证据函数见 `analysis/tasks/next65-legion-packet-table.md`。

### 8.1 军团入口判定链（**实机看不到入口时先看这里**）

入口是否出现/可点，由 **`sub_142510A50` / `sub_142511D10`** 决定（两者尾部都调用
`sub_1424FE550` = CMD2043 的发送函数）。判定顺序与各自的拒绝文案：

| # | 判定 | 不过时 |
| --- | --- | --- |
| ① | `sub_1476E0090(*(a1+128), 解码(word_14982AC40))`：具名条目存在 | 静默 return |
| ② | `sub_145F0B890() != 0` | 静默 return |
| ③ | `sub_145F152A0() != 0`（用 CMD 注册表 `qword_14E66C090`） | 静默 return |
| ④ | `sub_145F147D0()`：**8 个队伍槽全非零** | 串 **532** / **725** 队友未到齐 |
| ⑤ | `(*(modeObj.vt+624))(modeObj, 8) > 0` | 串 **100088500** 本周入场次数已用完 |
| ⑥ | `(*(modeObj.vt+640))(modeObj, 0) > 0` | 串 **100088497** 本周奖励已领完 |
| ⑦ | `sub_145695000(...) == 0` | 串 **100088632** 今天不开放 |

`modeObj = sub_142AB28D0(qword_14E683C40)`（与 §6.4 的 2657 派发共用同一全局）。

**怎么把串 id 变成人话**：`analysis/dumps/dstr_id_to_text.json`（客户端 dstr 表，英文原版）。
例：`t["100088500"]` → `Cannot proceed as all weekly entries have been used.`
**排查时先要弹窗原文** —— 「没反应/进不去」对应到具体哪一道，全靠这张表。

数据侧对照（`configs/*.json`）：入口副本 `100005220` 绑 quest `23099`（该任务
`prerequisites=null`、`jobs=[all]`、`minimum_level=115`）；`apocalypse.ctp` 四个作战
`[recommend fame]` = 98,171 / 105,881 / 105,881 / 73,993 且 `[member limit]=party`。
详见 `analysis/tasks/next73-legion-entry-gate.md`。

### 8.2 军团 S2C 包体结构（消费端反推，**通式 + 各自几何**）

**通式**（7 个 handler 全是这个形状）：默认值写本地缓冲 → `sub_146EA0BE0(buf, N)`
整块覆盖 → 交给某个子系统（虚调用 / 全局单例 / 模式对象）。
⇒ 本地初值只在"读不满"时才有意义。

| opcode | body | 结构闭环度 |
| --- | --- | --- |
| 2895 LEGION_INFO | 206（107 + **204**） | **204 B 偏移全解**；`@3(u32)`/`@7(u32)`/`@11(u64)` 读完被取出送进**模式对象**（`sub_142AC2290/2220/21B0`，形态同 §6.4：模式键查 `+312` 红黑树 → 虚调用 +48/+56/+64）；`@27` 起 **6×12 B 记录**；合计正好 204 ✓ |
| 2896 LEGION_OPERATION | 9（107 + **7**） | **内容读完即弃**（读进本地 `v2/v3/v4` 后再无使用）⇒ 只有长度与首 u16 是契约 |
| 2252 BASIC_CLEAR_REWARD | **7772** | `40×40 B`(0..1599) + `140×44 B`(1600..7759) + 12 B 尾；字段偏移已知，物品语义未知 |
| 2253 ADDITIONAL_CLEAR_REWARD | **2405** | `1 B` + `60×40 B` + `4 B` 尾；字段偏移同 40 字节记录 |
| 2254 ENTRY_CHARAC_INFO | **256** | **7 槽 × 24 B**（`u16@0/u64@2/u32@10/u32@16/u8@20`）+ **88 B 常量填充**（旧文档写"7×24+84=252"不准） |
| 2568 PREPARE_ENTER | **16** | `@0 != 0` 走另一分支；`@0 == 0 && @4 == 1` → toggle `sub_146D0D2A0(obj, @12)`；文案 id 145 / 182 |
| 2657 PHASE_CLEAR_TICK | **144** | **仍阻塞**（见 §6.4） |

**为什么这层能省很多时间**：这些包的**读取长度是硬契约**（§3），
而结构往往由 handler 里的**初始化代码**写死 —— 找 `sub_146EA0BE0(buf, N)` 上面那段
逐字段赋值/清零，就等于拿到一张免费的字段偏移表。详见 `analysis/tasks/next75-legion-s2c-structures.md`。

---

## 9. 易踩的坑（每条都真的踩过）

1. **cmd 与 noti 共享同一 16 位 id 空间、含义完全不同**。例：`0x8CD` 既是 cmd
   `SAVE_TRAINING_ROOM_PRESET` 又是 noti `LEGION_ADDITIONAL_CLEAR_REWARD`。
   按 id 立即数搜"发送点"会把撞号的别的包认成自己的（本仓库曾误判 3 个 sender）。
   **只有 cmd 族的立即数才是 C2S 发送点；NOTI 是纯 S2C，客户端不发。**
2. **`opcodes.tsv` 的 `table_slot_va` 不是 handler 表槽位**，而是「解密后字符串的全局变量地址」。
   早期按它读函数指针会全读到空（表是运行时填充的）。
3. **`sub_146EA0BE0` 是读包体，不是写 UI**（见 §3），长度不够会崩客户端。
4. **S2C 长度是硬契约**，不是"能回个同 id 的空包就行"。
5. **有些 S2C handler 是 toggle**（如 `sub_146D0D2A0` 用 `a1+1416` 做标志），
   条件未闭环时重复下发会把界面**关掉** —— 比不发更糟。
6. **`find_imm` 全盘扫 `.text` 是二次复杂度**，会超时；用注册表调用点定位更高效。
7. **`.ctp` 有两条解析路径，别把后缀分派读反**：`.ctp` 后缀命中的是**二进制**分支
   （`sub_1474D11A0`/`sub_1474D0C00`/`sub_1474D0580`）；文本词法器 `sub_1474D02D0`
   只在**松散模式**（`dword_14DC91508 != 0`）用。两者共用 `sub_1474CC1C0`，很容易误判
   （本仓库为此走了一轮弯路，见 `next69` §0）。
8. **`.ctp` 记录不是「名字 + 值」两段式**：完整记录头是 **44 字节**
   （`名字16 + flags4 + parent8 + n_cells8 + n_refs8`），漏掉 `parent/n_refs` 会在
   第二条记录之后失步（本仓库踩过）。对齐搜索求解「tag→长度」也无效 —— 那条路是错的。
9. **别用「DFS tiling」判记录边界**：浮点 cell 稀疏的文件（如 `dungeonskillinfo.ctp`）
   会退化。正解是按 `record_count` 驱动遍历 + 断言终点 == 尾表起点。
10. **IDA 批处理前先恢复工作副本**：一次运行会在 IDB 旁留下 `.id0/.id1/.id2/.nam/.til`，
    下一次打开会警告 `IDA did not close properly... safer to restart from the packed
    database`，且可能直接失败（`Database is empty` + `internal error 1228`）。
    做法：`rm -rf /d/115us-backup/ida-work && cp <原始备份> ida-work/DFO.exe.i64` 再跑；
    脚本版见 `run-phase-x7b.cmd`（跑前跑后各清一次 `.id0/.id1/.id2/.nam/.til`）。
11. **`ida_search.find_imm` 在 IDA 9.4 返回的是 `(ea, operand)` 元组**，不是裸 ea；
    按 int 比较会抛 `'>=' not supported between instances of 'tuple' and 'int'`。
12. **`ida_ua.generate_disasm_line` 不存在**（`ida_ua` 模块里没有这个名字）。
    要一行反汇编用 `idc.GetDisasm(ea)`；写探针时对两三种拼写都做 fallback 更省事。
13. **拿"函数里出现过某个立即数"当证据是陷阱**：`0x470`、`312` 这类偏移在一个
    259 MB 的 exe 里到处都是。用它筛目标函数会解出几十个无关函数，反而淹掉真信号。
    优先用**结构关系**（谁调用 X、谁读全局 Y、从注册点反查）而不是常量扫描。
14. **"列引用"和"解伪代码"要分成两个探针**：混在一个脚本里，一旦某个被引函数很大，
    Hex-Rays 会卡在那里十几分钟，而**结论只在脚本末尾落盘** ⇒ 中途被打断就一分数据都没有。
    做法：先用只列 xrefs 的探针（秒级）拿到"谁引用谁"，再决定解哪几个函数。
15. **IDA 9.3 的 `idautils` 没有 `FunctionCalls`**（df24 在这一行抛异常，section 4/5 全丢，
    只有前面已落盘的 section 3 有效）。列被调函数要用 `idautils.CodeRefsFrom(head, 0)`
    自己扫函数体内的 `call`，或 `ida_callinfo.guess_func_call(ea)`；写完脚本先 grep 一遍
    再用到的 `idautils.*` 名字是否存在于本机 `python/idautils.py`。
16. **注册表的 `kind` 只证明"客户端注册过这个 id"，不证明服务端发它能被分发到**：
    `df7-registrar-table.json` 按 `{"kind":"noti"|"cmd","handler","site"}` 记了两个注册函数
    （`sub_14599D5D0` / `sub_14599D450`）的插入点。第六轮据此写"1565 同时在两张表里 ⇒
    服务端发 1565 客户端会分发"，**结论对、判据错**：df33/df33b/df33c 查明真正的门是
    **信封首字节**（服务端 `wire.ServerFrame(kind,…)` 的 `kind`）——
    `sub_1459A1BB0` 只在 `*包 == 1` 时才去 CMD 表 `qword_14E6836F8` 查 `sub_1444E8D00`，
    `kind == 0` 走的是通知总线 `sub_1456B8AB0`，那里 1565 挂的是**另一个无关子系统**
    `sub_14399DD70`（读 8 字节 + UI 事件 1636）。所以第五/六轮那种"kind 0 发 1565"的
    回声**从来没进过处理器**，实机表现为「生效中」被 NOTI1546 清掉了但字体不回归。
    ⇒ 判"这帧客户端会不会吃"要连查三层：注册表里有没有 → **哪种 kind 才查这张表** →
    体首有没有被分派器替处理器读掉的前缀字段（§4）。光看注册表会把空转的帧当成已证。
17. **日志采样门可以是功能门**：`cmd/wireprobe/request_scope.go` 的 `retainRequestBody`
    本来只是"未实现命令每条最多留 8 份明文"的取证采样，但分发器复用了它的
    `verified` 结果（`if !verified { continue }`）。于是**已实现**的 C2S 1565 在第 9 次之后
    不解密、不进处理器 —— 实机现象是"第一次能应用、之后点谁都不换"，而客户端侧一切正常。
    凡新增有处理器的 C2S opcode，必须先把它加进 `observedGameRequest`，并用
    `TestSelectSkinRequestIsAlwaysDecrypted` 那类测试钉住；顺带一条：这类"采样影响路由"
    的耦合本身就该拆，别在排障时默认二者无关。
18. **两个 xref 集合求交集证不了"没有读者"**：df30 用「读全局 `qword_14E63AE60` 的函数 ∩
    使用 map 访问器的函数 = 空集」来判 holder 的按槽 map 没人读，这个判据本身是无效的——
    成员函数是**拿对象指针当实参**的（`sub_1447EF380(a, id, slot)`），根本不会去引用那个全局
    变量名。正确做法是 df31 那样**按结构关系全量扫描**：把二进制里所有调用访问器
    `sub_1401C4620` 的 338 个函数都反编译，再按首实参的常量偏移归类。
19. **同名偏移属于不同类**：按 `+120/+136` 归类时会命中 `sub_1448D1560`，它索引 `a1[133]`
    （=+1064），远超声称 336 字节的 holder ⇒ 是另一个类的同偏移，不是读者。用对象**大小**
    （构造里 `sub_146E8BA20(336)`）当边界过滤，别只看偏移数字。
20. **"发这帧会不会引起客户端重发"先看有没有别的帧已经在走同一条链**：判断 CMD1565 回声是否
    成环时，不必新起 IDA 轮——回声收尾的 `sub_1444F1EE0(mgr, 类别)` 与 NOTI1546 处理器收尾调的是
    **同一个函数、同一个实参类别**，而后者上一轮已实机跑过且没有重发风暴。再看链上每个被调者的
    实际语义（`sub_1444E8ED0` 名字叫 "sender" 却是往 `mgr+312` 插 map），**别凭函数名判环**。
21. **"解除/重置"的默认值是**每个字段自己的**，不是一个全局哨兵**：第五轮因为只看到
    `sub_1444EECA0` case 6 的 else 里写着 `99999999`，就把它当成"客户端的卸下值"用到类别 2 上，
    结果整轮修复对普通分区空转（实机 7 次 `02 … 01` 全被拒）。真正的默认值在**构造函数**里
    （`holder+112 = 1`、`holder+232 = -1`），要按字段查。凡"重置到默认"这类语义，先反编译
    那个对象的 ctor 看每个字段起始值，再决定回什么 id；并且请求里的 id 与默认值相等时仍要过一遍
    归属判断，否则仓库里真有一个同 id 的条目时按钮语义会反过来。
22. **别把「C2S 请求体」读成「玩家的选择」**：同一张 88 字节体可以由**多个内部列表拼出来**，
    服务端原样镜像就会把别的页的值当成本池的选项。第八轮把类别 1 的 20 槽读成一个列表，于是二觉页
    的默认行 100000 被写进 `mgr+296[1]` 那个**抽签池**（`sub_1444EA8A0` 取 `pool[RNG % size]`
    ⇒ 长度 ≥2 就是随机），实机表现为「应用一个新插图后随机勾选自动亮」，并且每来回一次池更长
    （日志里出现过同一 id 在一条请求里出现两次）。判据：**先反编译发送方**（`sub_1444F1090` 及其
    调用者）看清槽位怎么填，再决定服务端存什么；回声可以原样回（客户端拿它跟自己的列表比对），
    但**存储与回推的列表要按客户端自己的过滤器口径清洗**（§15.5；§15.1 那张 switch 顺带复证了
    §13.4 的每分区默认 id）。

23. **"没人读"不等于"不用发"：结论要重做 xref，不能沿用上一轮的搜索半径。** df36 的偏移扫描只覆盖了
    `0x1444E8000..0x1444F2200` 这一段簇内函数，据此写下"`mgr+1152` 无消费者"，服务端就常年发空列表；
    df37 把范围放开到全 executable，才发现唯一读者 `sub_1444EBAD0` 的调用者是簇外的
    `sub_145D451D0`（副本里选插图的那位）。判"某字段无人读"之前，先对**每个**候选访问函数做
    `XrefsTo`，别只做簇内扫描。
24. **不要把客户端的默认行当成协议的哨兵值。** `sub_1444F1B60` / `sub_1444F1BE0` 是在「赋值之后表为空」
    那一刻才补进 30000 / 100000 的，也就是说这两个 id 首先**是可以被玩家勾选的行**，只有单独
    成为表内唯一元素时才兼任未选标记。第十轮把「凡是这两个 id 就剥掉」写进解码口径，等于
    替玩家取消了他真勾过的那一格，回帧再覆盖一次面板的工作列表，表现就是「多选一关一开就没了」。
    判据：同一面板三场实机 body（17:11:02 / 17:12:19 / 17:13:07）里，标记与真实行的**位置关系**
    会变，剥与不剥的差别只在「它是不是该表唯一元素」。
25. **IDA 9.3 的 `idc` 兼容层删掉了三个老接口**，批处理脚本一用就当场 `AttributeError`、整轮白跑：
    `idc.get_segm_qty` / `idc.getnseg`（改用 `ida_segment.get_first_seg()` + `get_next_seg(seg.start_ea)`
    走链）、`idc.bin_search`（自己扫：`ida_bytes.get_bytes` 按 4 MB 分块 + `bytes.find`，读不通会返回
    `None`，要折半重试）。判据是 log 里那句 `module 'idc' has no attribute ...`。
    **每个探针段落外面套 `try/except`、结果先 `dump()` 落盘**，df39 就是靠这条把 §4/§5 救回来的。
26. **不要用 Python heredoc 写 `.cmd` 运行器**：非 raw 字符串里的 `D:\115us\analysis` 会先被 Python
    当转义吃掉（`\115` 是八进制 → `M`，`\a` → BEL），落到磁盘的批处理里路径就成了 `D:Mus` + 控制符，
    报 `missing ...DFO.exe.i64` 而脚本本身看着一切正常。用 Write 工具直接写，或至少 `r"""`；
    写完先 `cat -v` 验一遍反斜杠还在。同理批处理注释只写 ASCII（GBK 码页吃 UTF-8 中文）。
27. **按字节偏移 `+ 328` 扫字段会整轮漏掉 Hex-Rays 的下标拼写**。同一个字段在伪代码里
    可能打印成 `v0[41]`（qword 下标 = 字节偏移 / 8）而不是 `+ 328`，`+N` 的正则一条都抓不到。
    df43 用 `+ 328` 扫过一遍后判「`mgr+328` 网格没人读」并不够，df44 换 `\[\s*41\s*\]`
    对同一批函数再扫一遍，两次都空才把这条写进 §20.4。同理 `+3328` 也会打成 `[416]`/`[417]`。
28. **「这个字节被写成 1」不等于「界面按它显示」**。NOTI1545 的 56 字节节点 `+44/+52` 只有
    一个"读"者，而且是通用节点克隆器 `sub_141FD7060` 的整块拷贝，从不参与比较。
    找谓词要找**比较/分支**（`if ( *(rec+N) == … )`），不要拿赋值当证据。
29. **虚方法不能靠 `XrefsTo(函数)` 找调用方**：`sub_1441D8460` 的 xref 只有一个数据槽
    `0x14A2312B0`（df46），说明它是派发出去的槽位，反搜派发点要跨三张表。定位「界面子对象」
    的 vtable 有更短的路：控件数组由 `sub_148860630(buf, size, count, ctor, dtor)` 分配，
    **ctor 实参**就是行对象构造函数（df48 由 `sub_148860630(window+5384, 400, 5, sub_1441B98C0, …)`
    一步拿到行 vtable `off_14A230C10`，df49 直接反编译槽 +16）。注意 ctor 里 vtable 可能被赋值两次
    （基类 `&off_14A230B78` 后被派生类 `off_14A230C10` 覆盖），要取**最后一次**。
30. **`idat -L` 的日志是 `print` 的目标，也是它的牺牲品**：一个大函数（`sub_1441BB860` 的
    dtor 伪代码）全文打进日志能把真正要看的段落挤到几百行之后。规则：`dump()` 落盘之后
    只 `print(text[:N])`，并在段落里先跑便宜的 xrefs/表扫描、再跑 decompile。
---

## 10. 工具与脚本索引

| 脚本 | 用途 |
| --- | --- |
| `analysis/tools/pe_decode_xorstr.py` | 离线解码 xorstr 字符串（§1），可解任意 VA |
| `analysis/tools/pe_disasm_va.py` | 任意 `.text` 地址的带注释反汇编（RIP 操作数自动用 xorstr 表标注） |
| `analysis/tools/pe_rip_scan.py` | 找代码对某 VA 的 RIP 相对引用（证否过"静态读 handler 表"） |
| `analysis/dumps/ida_decompile_va.py` | **主力工具**：改 `TARGETS` 后 `idat -A -S` 批量出伪代码 |
| `analysis/dumps/ida_ctp_readers.py` | 按解码串反查"谁在用这个列名"（定位读取器/消费者） |
| `analysis/dumps/ida_bind_sites.py` / `ida_cursor_binding.py` | 查全局变量的读写者（定位游标绑定、注册点） |
| `analysis/dumps/ida_sender_callers.py` | 查发包函数的调用者（定位触发时机） |
| `analysis/dumps/ida_skin_dfNN.py` + `run-skin-dfNN.cmd` | **皮肤仓库 NOTI1545/1546/1547/2641 取证轮**（df7 注册表 → df26 渲染几何 → df27–df31 池填充者与「谁读 holder 的槽 map」全量扫描 → df38 页/面板/CMD1565 发送方对照与 opcode 注册全集 → df39 CMD1565 两个组装方（result 恒 0 vs result 2/3 的星星）与收藏链表唯一写者 → df40 `[type]`/`[sub type]` 标签字面量的引用者与 1565 全部组装点 → df41 页 3/7/8 选择形状与 NOTI2641 线上形状 → df42 收藏链表读者 → df43/df44 `mgr+328` 网格与节点标志字节的**否证**（两种拼写各扫一遍）→ df45 窗口 `+3328` 与 `sub_1441EAF20` 的实参 → df46 管理器访问函数与它们的调用者 → df47 星星谓词候选与六个绘制方 → df48 行子对象构造函数（`sub_148860630` 的 ctor 实参）→ df49 `(id, kind)` 落点 `sub_1441E2620`）：一个假设一轮，日志 `analysis/ida-work/dfNN.log`，伪代码 `analysis/dumps/skin-noti/dfNN_<semantic>_<VA>.c`；df31 的 `var_assigns` helper 用来把被提升的指针实参解析回首实参的常量偏移；**df33 系列的落盘点在 `print(text[:4000])` 之前先 `dump()`，所以即使脚本后半段崩了（`f.start_ea` 对 `CodeRefsTo` 返回的 int 不成立）分发链的三张伪代码也已经写完**；df39/df40 进一步把每个探针段落包进 `try/except`（坑见 §9 第 25 条） |
| `sub_148860630(buf, size, count, ctor, dtor)` | 客户端**控件数组分配器**：伪代码里看到它就知道 `buf` 是一排同构子对象，第三个实参是格数、第四个是行构造函数——从行构造函数取 vtable 再取槽，比反搜虚派发点快一轮（§20.1、§9 第 29 条） |
| `analysis/tools/pe_find_plaintext.py` | 在 `client/DFO.exe` 上 mmap 全文件扫明文字符串（**先跑它再为"某标签→int"花 IDA 轮**：`damage font`/`party frame`/`spray`… 命中 0，说明映射不是明文表） |
| `analysis/dumps/ida_phase_x7.py` | NOTI2657 派发链探针（第一版）；常量扫描那半段**信噪比低，已被 x7b 取代** |
| `analysis/dumps/ida_phase_x7b.py` | 结构探针：给定锚点，列 xrefs + 解出这些函数（**慢，会卡在单个大函数上**） |
| `analysis/dumps/ida_phase_x7c.py` | **引用计数探针（推荐先跑）**：只列 xrefs 与"哪些函数同时引用多个锚点"，秒级返回 |
| `analysis/dumps/run-phase-x7b.cmd` / `run-phase-x7c.cmd` | 上两者的批处理入口（跑前跑后清 IDB 解包残留） |
| `cmd/pvfinspect`（Go） | 枚举/导出 PVF 内容（`-find` 会写 `matches.json`，大文件扫完记得删） |
| `cmd/apocalypseimport`（Go） | 导出 `.ctp` 与 schema（早期临时定宽解码，**已被下方 Python 读器取代**） |
| `analysis/tasks/next69-ctp-extract.py` | **`.ctp` 权威读器**：头部 + 记录流 + 尾表 + 池，带 4 条硬断言，可导出 JSON |

**IDA 批处理调用方式**（本机已装在 `D:\tools\ida94`）：

```bash
cd /d/tools/ida94
env -u PYTHONPATH -u PYTHONHOME ./idat.exe -A -L<log> \
  -S"<脚本.py>" "D:\115us-backup\ida-work\DFO.exe.i64"
```

- **必须先清空 `PYTHONPATH`/`PYTHONHOME`**，否则报 `Failed to import encodings module`；
- 只打开**工作副本** IDB（权威 `client/DFO.exe.i64` 不开、不升级）；
- vtable 方法可直接用 PE 读：`off_<VA>` + 8×n。

---

## 11. 物品行「剩余期限」单元格（偏移 56）—— 面板值的校准公式

> 2026-09-23 宠物期限修复的沉淀；行记录本体见 §3 与 `internal/game/protocol/inventory.go`。

| 项 | 值 |
| --- | --- |
| 单元格 | 181 字节物品行 **偏移 56**（u32 LE） |
| 语义 | **剩余秒数**（既不是 Unix 时间戳，也不是天数） |
| 客户端渲染 | `Expires in : %d Day(s)`（dstr **1057**，源文件 `PopupWindow\CNRDItemInfoWindow.cpp`）；不足 1 天走 `Expires in : %d H`（dstr **1058**） |
| 换算 | **面板天数 = round(值 / 86400)**，且**不减当前时间** |
| 值为 0 | 判过期：宠物显示「剩余期限已过。」（历史注释里的 `Past Duration` 是另一条退款文案 dstr 22042，别混用） |
| 声明来源 | 脚本 `[usable period] N`（天）⇒ 下发 `N*86400`；`[expiration date]` 的值是**绝对日期文本**（如 `2020-12-31 06:00:00`，cell `Type=6`），与天数不是一回事 |

**实机校准（单位是怎么定下来的）**：宠物 `100991331`（脚本 `[usable period] 7`）的行填
`math.MaxInt32 = 2147483647`，面板显示「过期时间:24856天」。
`2147483647 / 86400 = 24855.597…` 四舍五入 = **24856**，与面板逐位吻合；
若按「时间戳减当前时间」口径应为 `2038-01-19 − 今天` ≈ **4138 天**，与实测不符 ⇒ 客户端只做整数除法。

**可复用形式**：面对「客户端把某个 u32 渲染成人可读量」的场景，**先用实机读数反解公式**
（拿当前写入值与面板显示值做算术）——一次就能给单位定性，比先猜语义再反复实测快得多，
而且反解出的公式可以直接当验收判据（例如本条的判据是「面板显示 7 天」= `604800`）。

**服务端现状**：`internal/inventory.EquipmentPayload` 只给宠物行（`space 7`；`space 3` 槽 26~29）
填这一格，值来自 `inventory.SetCreaturePeriodSource` 注入的 `[usable period]` 查表换算（启动期由
`cmd/wireprobe` 用全量装备目录安装）；其余写入者是可叠加物（`Bag.Rows`）与商城时效道具。
**装扮的期限走行尾那个 u32**（同一个 `BagEquipment.Period`），不在本次范围内。

---

## 12. 伤害字体「应用」状态在客户端的几何（df25 取证 + df26 修正，未闭环部分已标注）

`sub_142581F20(a, id)` 是一个**裸字段写**：`*(u32*)(a + 232) = id`（dump
`skin-noti/df25_setter_0x142581f20.c`，仅 3 行）。df25 曾按调用点的写法把它读成「写在
**三种不同对象**上」，df26 证明**这是错的**：`sub_143C61150()` 是全局 336 字节对象
`qword_14E63AE60`（构造 `sub_1447E41D0`）的**懒获取器**——`if ( !qword_14E63AE60 ) {…新建…}`
（`df26_holder_sub_143C61150_0x143c61150.c:9-21`）。所以三处调用写的是**同一个对象的同一个字段**：

| 调用点 | 第一个实参 | 证据 |
| --- | --- | --- |
| NOTI1546 category 6 | `sub_143C61150()` ⇒ `qword_14E63AE60` | `rd_reader_1546_select_body_1444eeca0.c` case 6 |
| CMD1565 回声类别 6 | 同上 | `df25_caller_sub_1444EE820_*.c` |
| 皮肤仓库窗口的两个处理函数 | 直接取 `qword_14E63AE60` | `df25_caller_sub_1441D21E0_*.c`、`df25_caller_sub_1441D23A0_*.c` |

⇒ 「服务端只写 `mgr+296[6]`、没人推进渲染持有者」这个 df25 推测**不成立**，发一条
NOTI1546 category 6 就已经把 `*(qword_14E63AE60 + 232)` 写对了。

**那字体为什么在副本里还是默认的**：id 是在**对象创建时快照**进去的，不是每次伤害数字现读。
`sub_1447EB510(obj, …, a8)` 里 `if ( a8 <= -1 ) v91 = *(u32*)(sub_143C61150() + 232); *(u32*)(obj+104) = v91;`
（`df26_holder_sub_1447EB510_0x1447eb510.c:435-441`），而 `sub_1444EAD50` 正是以 `a8 = -1` 调它
（`sub_1447EB510(obj,0,0,0,1234000,0,0,-1)`），调用者是皮肤槽记录池的派发方法 `sub_1441BDCD0`
（见 §13：它是窗口子对象 `+9760` 类 `off_14A230DE8` 的虚表槽 9，不是「伤害数字渲染循环」；
它把记录自带的 id 与默认哨兵 `99999999` 比较来决定走哪个构造器）。
城镇里加载角色时写好的字段，副本重新构造伤害对象时**不会回头问仓库**，所以副本半链必须在
进城加载后把「归属页 + 选中 id」再绝对地推一遍。

**面板自己的口径**：`sub_1441D21E0` / `sub_1441D23A0` 每次都同一条链——
`sub_1444EBC10(mgr, &out, 6)` 取**类别 6 的选中向量**（存在 `mgr+296` 这棵
`map<category, vector<u32>>` 里），再 `sub_142581F20(qword_14E63AE60, *out.begin())`。
⇒ 客户端认定的「当前伤害字体」= `mgr+296[6]` 的**第一个 id**。
「点第二次应用换不动」与本节无关，是服务端 `request_scope.go` 的采样上限把第 9 次起的
C2S 1565 明文丢掉、`verified=false` 后分发器直接 `continue`，已在 `observedGameRequest` 修好。

**另一条更重的 setter（category 2 / 回声 page 2）**：`sub_1447EF2F0(a, id)` 除了
`*(a+112)=id`，还会 `(*(qword_14E683C68 虚表+16))(qword_14E683C68, 178, 2)` **发一个 UI 事件 178**，
再用 `sub_146664340(qword_14E683C68, a+112)` 把值推给全局 UI 管理；
`sub_1447EF3B0(a, id, slot)` / `sub_1447EF380(a, id, slot)` 只是往 `a+120` / `a+136` 的小数组里按
slot 写 id。⇒ 这条链写的是 `+112`，属于 category 2 自己的口径，**不是**换伤害字体缺的那一步（df26 已否定）。

**页号与类别的对应关系（已证部分）**：NOTI1546 category 2 与 category 6 **都**用
`sub_1444EBD90(mgr, 2, id)` 校验归属（`skin-noti/df8_lookup_page_id_1444ebd90.c`：查
`mgr + 16*page + 136` 这排 map），所以类别编号 ≠ 仓库页编号，两者只是碰巧都指向第 2 页。

**`mgr+312` 那个集合**：整个 skin-manager 簇（0x1444E8000–0x1444F2200，113 个函数）里
**只有 `sub_1444EC570(mgr, id)` 读它**，而且它只是一个「这个 id 在不在集合里」的谓词
（`df25_mgr312_sub_1444EC570_*.c`）。写它的两处是 NOTI1546 category 0（先 `sub_1401CA430` 清空再插）
和 CMD1565 回声类别 0 里那条「物品类型 == 2」分支（`sub_1444E8ED0`）。所以它是
「已获得/可应用」集合，不是「当前应用」状态，别把它当选中态用。

---

## 13. 伤害字体的「槽」：池填充者与两条选中类别（df27–df31）

### 13.1 holder 的两张按槽 map 是**只写不读**的死状态

`sub_1447EF380(holder, id, slot)` ⇒ `map(holder+136)[slot] = id`；
`sub_1447EF3B0(holder, id, slot)` ⇒ `map(holder+120)[slot] = id`（`df28_slotmap_*.c`，各 8–35 行）。
df31 把全二进制里**所有 338 个调用 map 访问器 `sub_1401C4620` 的函数**逐个反编译并按首实参的
常量偏移归类（`df31.log` §1，脚本 `ida_skin_df31.py`）：命中 `+120/+136` 的只有三个函数——
两个写入者本身，加上 `sub_1448D1560`；后者索引 `a1[133]`（=+1064）远超声称 336 字节的 holder，
**是另一个类的同名偏移**，不算读者。

⇒ **NOTI1672 的 slot 参数在客户端没有任何消费者**。别为了「按槽下发字体」去发 NOTI1672，
那是无消费者的推测帧。（df30 那句「holder 全局读者 ∩ map 使用者 = 0」本身是无效判据：
holder 的成员函数是拿指针当实参的，不引用 `qword_14E63AE60`；结论靠 df31 的全量扫描才成立。）

### 13.2 池记录 id 的真正来源：`sub_1441E5870`

窗口对象（构造 `sub_1441B8E90`，`df28_ctor_*.c:194/216/246`）里有**三处** `5×400` 记录池
（`+5384`、`+7536`、`+9912`），记录 vptr 统一是 `off_14A230C10`；初始化 `sub_1441BFEC0` 把
`record+40 = -1`、`record+112 = 0`（`df30_eleminit_*.c:25,74`）。填充者是
`sub_1441E5870(a1, a2)`（`df26_holder_sub_1441E5870_*.c`，同一簇的兄弟是 `sub_1441E86C0`）：

| 步骤 | 代码 | 判定 |
| --- | --- | --- |
| 取候选 id | `sub_1444EBAB0(*v60)`，要求 `*(u32*)(rec+8) == 2` | 皮肤注册表里**类别 2 = 伤害字体** |
| 归属校验 | `sub_1444EBD90(mgr, 2, id)` | 必须已在**仓库第 2 页** |
| 写记录 | `*((_DWORD*)v61 - 38) = *v60` ⇒ `record+40 = id` | 渲染循环读的就是这一格 |
| 写类别 | `v72 = 6`，若 `window+2184 == 0` 则 `v72 = 2`；`*((_DWORD*)v61 - 37) = v72` ⇒ `record+44 = v72` | **两条选中类别** |
| 选中态 | `sub_1444EC5B0(mgr, v72, id)` = 「id 在类别 v72 的选中向量里吗」 | 向量存在 `mgr+296`（`map<category, vector<u32>>`） |
| 记录数 | `5 * vectorSize(*(window+112))` | 槽数 × 5 |
| 模式过滤 | `window+2184 == 1` ⇒ 用 `sub_1447EAF60(holder, id)` 过滤；`== 0` ⇒ 从表里剔掉一个 `99999999` | 同一个池在不同模式下取不同的 id |

兄弟函数 `sub_1441E86C0` 结构完全相同，但要求 `rec+8 == 0` 且查 `sub_1444EBD90(mgr, 0, id)`
（`df26_ec570caller_sub_1441E86C0_*.c:456,466`）——那是**第 0 页（边框）**的池。

记录 id 也有一条虚表入口：元素类 `off_14A230C10` 槽 2 = `sub_1441E2620(elem, id, category)`，
类别 5 走 `qword_14E6A7C68` 的 96 字节记录表，类别 9 走注册表 `qword_14E683B38`，其余仍要求
`sub_1444EBAB0(id)` 的 `rec+8 == category`（类别 10 例外），然后写 `+40/+44`（`df30_elemmethod_sub_1441E2620_*.c:141,149,235`）。

### 13.3 CMD1565 的第一字段是**分区的选中类别**，不是仓库页号（实机判决，无需新反编译）

2026-09-27 第三场实机日志（`runtime/roles_persist_..._20260927_035034_350988_next37/events.jsonl`）
把这件事钉死了：同一个面板的两次点击给出

```
02000000 00000000 12000000 …   （id 18）
06000000 00000000 12000000 …   （同一个 id 18）
06000000 00000000 ffe0f505 …   （99999999 = 累计分区点「解除」）
02000000 00000000 01000000 …   （1        = 普通分区点「解除」，第五场 7 次）
```

**同一个皮肤 id 在 2 与 6 两种取值下都出现过** ⇒ 第一字段不可能是"这个 id 所在的仓库页"，
只能是**分区自己的选中类别**。伤害字体面板有两个分区（截图顶部：`普通伤害` / `累计伤害`），
两个分区的列表都取仓库**第 2 页**（§13.2 的填充者对两种模式都调 `sub_1444EBD90(mgr, 2, id)`），
差别只在 `window+2184` ⇒ 记录 `+44` 写 2 还是 6。

回声读取器 `sub_1444EE820`（`df25_caller_sub_1444EE820_*.c:53-55` 先 `v43[0]=10` 再
`sub_146EA0BE0(v43, 88)`，即 `u32 类别, u32 结果, u32 ids[20]`）按 `v43[0]` 分支，
并且**分支号与 NOTI1546 的类别号完全一致**：

| 类别 | 生效动作 | 写的字段 |
| --- | --- | --- |
| `case 2` | `sub_1447EF2F0(holder, v43[2])` | `holder+112` + 向 `qword_14E683C68` 发 **UI 事件 178** 再 `sub_146664340(…, holder+112)` |
| `case 6` | `sub_142581F20(holder, v43[2])` | `holder+232`（§12 里「应用」的那一格） |
| 两者共同 | `LABEL_37` 把值向量插进 `mgr+296[v43[0]]` | ⇒ **`mgr+296[类别]` 就是那个分区的「生效中」来源**（§13.2 的 `sub_1444EC5B0` 成员判定） |

⇒ 服务端的正确做法是**按请求带来的类别原样回**（`u8 类别, u32 id`）。本服务端此前把
所有请求统一回成类别 6，等价于"在 A 分区点、把 B 分区点亮"：`mgr+296[6]` 被写上了
A 分区点的 id，所以 B 分区显示生效中、A 分区永远不亮，而携带 6 的请求（B 分区的点击与
「解除」）被当成未知页丢弃。这就是用户报的"普通伤害不显示生效中 + 解除无效"。
**第六轮补一条同源的判决**：上表最后一行（普通分区的 `02 … 01`）说明「解除」并不共用一个
哨兵，**每个分区点名自己的默认 id**；把它当成全局 `99999999` 会让第五轮的类别回显修复对
普通分区**完全空转**（7 次请求全被"未拥有"拒掉）。默认值的真源见 §13.4。

### 13.4 「解除」= 请求里直接点名**该分区自己的默认 id**，两个类别的卸下语义不同

默认 id 不是猜的，是构造函数写进去的（`df32_holder_ctor_sub_1447E41D0.c`，holder 336 字节，
getter `sub_143C61150()`）：

| 字段 | 构造默认 | 该分区「解除」实际发来的 id（实机） |
| --- | --- | --- |
| `holder+112`（类别 2 渲染的那格） | `*(_DWORD *)(a1 + 112) = 1;`（同文件 `:186`） | **`1`**（`01000000`，第五场 7 次，从未发过哨兵） |
| `holder+232`（类别 6 渲染的那格） | `*(_DWORD *)(v1 + 232) = -1;`（同文件 `:123`） | **`99999999`**（`ffe0f505`，第四场与第五场均是） |

⇒ 两个分区各有各的默认，服务端**必须按类别查默认 id**，不能拿 `99999999` 当全局哨兵。

`sub_1444EECA0` 的两条 case 对"id 不在第 2 页"的处理**不一样**：

| 类别 | 拥有且未过期 | 不拥有（含各自的默认 id） |
| --- | --- | --- |
| `case 2`（`rd_reader_1546_select_body_1444eeca0.c:462-476`） | `sub_1447EF2F0(holder,id)` + `sub_1447EF3B0(holder,id,0)` | `goto LABEL_208` ⇒ 选中向量**置空**（生效中消失），但 **`holder+112` 保持旧值** |
| `case 6`（同文件 `584-606`） | `sub_142581F20(holder,id)` + `sub_1447EF380(holder,id,0)` | else 分支显式 `sub_142581F20(holder, 99999999)` + `sub_1447EF380(holder,99999999,0)` ⇒ **完整卸下** |

⇒ 回 `u8 类别, u32 该分区的默认 id` 是客户端自己走的那条重置路径，不是猜包；类别 6 的卸下是完整的，
类别 2 只能清掉「生效中」标记，`holder+112` 留在旧值（客户端侧的不对称）。

### 13.4b 类别 2 的完整卸下只有一条已证路径：CMD1565 **回声**（第五轮复算旧 dump 提出，第七轮 df33/df33b/df33c 补上"发到哪张表"这半）

2026-09-27 第五轮把上表那句"服务端无法补"**推翻**了：NOTI1546 走不到，但同 opcode 的
回声走到（⚠️ 第五、六轮把它当 88 字节裸负载发，**第七轮才证到它是 kind 1 + `u8 1` 前缀**，
见下面第 4 条与 §4）。三条证据（全部来自已有 dump，不需要新 IDA 轮）：

1. **回声的 case 2 不查拥有**（`df25_caller_sub_1444EE820_0x1444ee820.c:102-107`）：
   `sub_140154010(&v36, 0, &v43[2])` 造出单元素向量，随后 **无条件** `sub_1447EF2F0(holder, v43[2])`
   ⇒ `holder+112` 被改写。`LABEL_37` 再把 `mgr+296[类别] = {v43[2]}`，`LABEL_58` 收尾刷新。
   回**该分区的默认 id** 后向量里只剩一个不属于仓库的 id，面板任何一行都不再命中
   ⇒「生效中」不显示，语义正确。
2. **刷新链真的重建那一层**：`sub_1444F1EE0(mgr, 2)`（`df13_cargo_render_tail_1444f1ee0.c`）→
   `sub_1441ECDB0(window, 2)`（`df12_page_select_1441ecdb0.c:140-143`，向量非空才动手）→
   `sub_1441E0820(window, 2, id)`（`df25_caller_sub_1441E0820_0x1441e0820.c:395-404`）→
   `sub_1447EBED0(..., id, 0)` 用**这个 id** 重建 `window+3680` 的普通伤害字体对象。
3. **没有回环**（这是本轮动手前的唯一门）：链上没有任何发包点 ——
   `sub_1444E8ED0`（§10 df24 已解）**不是**发送器而是往 `mgr+312` 插 map 节点；
   `sub_1441ECDB0` 尾部只把 id 从 `window+3328..3336` 向量里擦掉再调 `sub_1441EC510`。
   更硬的一条：**同一条 `sub_1444F1EE0` 收尾，NOTI1546 处理器每帧已经在调**
   （`handler_noti1546_select_skin_list_1444ed4f0.c:202`），第四轮实机两个分区各回一帧、
   没有重发风暴 ⇒ 回声不会引入新的往返。
4. **收帧方向已证（第六轮的说法判据错了，第七轮补上真链）**：不是"1565 在两张表里所以会分发"，
   而是 df33/df33b/df33c 的三层：`sub_146D74A80` 收包 → `sub_1459A1BB0` **按信封首字节分支**，
   `== 1` 才进 `sub_14599D200` → `sub_1459A2D70(qword_14E6836F8, id, 状态, 错误码)` →
   `handler(obj, 状态, 错误码)` = `sub_1444E8D00`。`== 0` 走 `sub_1456B8AB0` 通知总线，
   那里 1565 是无关子系统 `sub_14399DD70`。**回声必须 kind=1 发，且体首要带被分派器读掉的
   `u8 1` 状态字节**（状态 0 会让 `sub_1444E8D00` 直接进提示分支 `sub_1444EC790`，且那时
   客户端会先读掉一个 u16）。第五、六轮发的是 kind 0 + 无状态字节 ⇒ 两帧里回声那帧**整帧空转**。

⇒ 服务端动作：普通分区的「解除」回**两帧**——先 `NOTI1546 kind 0, u8 2, u32 1`（清标记），
再 `CMD1565 kind 1, u8 1 | u32 2, u32 0, u32 1, …`（把 `holder+112` 写回构造默认并重建那一层）。
第五轮这两帧带的是 `99999999`，实机证明普通分区**从不**发那个值，所以修复空转 ⇒ 第六轮只换 id；
第六轮换了 id 但仍用 kind 0 且缺状态字节 ⇒ 回声依旧不落地（第七轮）。
顺序不能倒：回声的刷新读的是 `mgr+296[2]`，若先回声后 1546，`LABEL_208` 会把刚写进去的向量清空，
`holder+112` 虽然复位但那层对象留在旧值。累计分区仍只回 1546（case 6 自带完整卸下）。
**`id == 1` 只在仓库第 2 页里没有 1 时才算「解除」**：若某账号真有一个 id 1 的字体，这一击就是
普通「应用」，两帧会把默认字体重新点亮成那一行，与按钮语义相反；带归属判断是安全的一侧。

### 13.5 蓝框「9999999」层：现在有了可检验的解释，但仍**未证**

§13.3/§13.4 的修复顺带把**类别 2 那条链第一次真正驱动起来**（`holder+112` + UI 事件 178 +
`sub_146664340` 推给全局 UI 管理）。如果蓝框那一层读的是 `holder+112`，它应当随之改变。
但**没有任何静态证据**把蓝框层钉到 `holder+112` 或某一条池上：三处 `5×400` 池
（`+5384/+7536/+9912`）里哪一处渲染它、当时 `window+2184` 取何值，仍未证。
所以本轮只把它当作**待实机观察的推论**记录，不额外补帧；观察点：两个分区各自应用后，
蓝框层是否分别跟随、城镇与副本是否一致。

## 14. 皮肤仓库的家庭 ↔ 仓库页 ↔ 选中类别全表（df34–df35 + PVF 实测）

伤害字体跑通以后，同一张仓库里还有两类消耗品要用：**边框**与**觉醒插图**。本轮把
"哪个家庭必须发到哪一页、哪一类别" 钉死，判据全部来自已有 dump 与 `Script.inner.pvf`，
没有新增 C2S 尝试。

### 14.1 仓库页号 == 静态注册表记录的 `+8`（家庭类），不是枚举序号

`sub_1444EBD90(mgr, page, id)`（`df14_page_lookup_1444ebd90.c`）取的是
`*(__int64 **)(mgr + 16*page + 136)`，即 **NOTI1545 的页号、`mgr+296` 的选中类别号、
注册表 `+8` 的家庭类号是同一个下标**。两个面板自己的表格填充把这条钉死：

| dump | 页 | 只保留 | 另外单独处理的默认 id |
| ---- | -- | ------ | -------------------- |
| `df22_ui_1441E3F40.c`（边框） | 0 | `rec+8 == 0` | `20000 / 50000 / 60000 / 80000` 挪到列表尾 |
| `df22_ui_1441E4180.c`（觉醒插图） | 1 | `rec+8 == 1` | `30000 / 100000` |
| `df22_ui_1441E3510.c`（伤害字体） | 2 | `rec+8 == 2` | — |

`list/skin.lst` 里这几条默认 id 的路径（`runtime/pvfx/entry.txt` 实测，1318 条）：
`1=DamageFont/default`、`10000=InstantEmoticon`、`20000=PartyFrame/default`、
`30000=SkillCutscene/default`、`40000=WeaponSkin`、`50000=PartyRequestFrame/default`、
`60000=CharacterInfoBG/default`、`70000=Spray`、`80000=PartyListFrame/Default`、
`90000=WeaponEffect`、`100000=SecondAwakeningCutscene/default`、`200000=Teleport`。
**id//10000 是"子家庭序号"，不是家庭类**：0 页收 2/5/6/8 四个块，1 页收 3/10 两个块。

### 14.2 家庭由 `.skn` 自己的 `[type]` / `[sub type]` 标签决定（数据驱动，不猜）

`DFO.exe` 里 `damage font`、`party frame`、`skill cutscene`、`spray` … **一个明文字符串都没有**
（mmap 全文件逐串扫过，命中 0；df35 的三个引用函数体里也没有字符串），所以字符串→int 的映射
在加载器里，静态读取代价高。但服务端**不需要那个 int**：把 `configs/skin-storage-items.json`
的 1733 条 `[add skin storage]` 按（`[type]`, `[sub type]`, 目录）分组，标签与目录严格一一对应，
家庭类可以直接从标签读出：

| `[type]` | `[sub type]` | 目录 | 条数 | 家庭类（=页=类别） |
| -------- | ------------ | ---- | ---- | ------------------ |
| `party frame` | — | `partyframe` / `dfo` | 153 | 0 |
| `party frame` | `party request frame` | `partyrequestframe` / `dfo` | 40 | 0 |
| `party frame` | `character info BG` | `characterinfobg` / `dfo` | 28 | 0 |
| `party frame` | `raid party list frame` | `partylistframe` | 3 | 0 |
| `skill cutscene` | — | `skillcutscene` / `dfo` | 476 | 1 |
| `skill cutscene` | `second awakening cutscene` | `secondawakeningcutscene` / `dfo` | 117 | 1 |
| `damage font` | — | `damagefont` / `dfo` | 354 | 2 |
| `instant emoticon` | — | `instantemoticon` / `dfo` | 540 | 3 |
| `weapon skin` | — | `weaponskin` / `dfo` | 2 | 4 |
| `spray` | — | `spray` / `dfo` | 9 | 7 |
| `airship effect` | — | `teleport` / `dfo` | 11 | 8 |

⇒ 结论：**`[type]` 决定页，`[sub type]` 只决定页内哪一个槽**。用标签而不是路径分类，
因为 `skin/dfo/…` 这批搬家文件路径不可靠而标签可靠。

#### 14.2.1 标签 → 整数的加载器已反编译（df40，闭环 §18.6 第 1 条）

`analysis/dumps/skin-noti/df40_loader_sub_147C18830_147c18830.c`：注册表记录字段
`+8` / `+12` 由 `sub_147C18830` 的一个字符串比较阶梯写入，阶梯的分支键本身也是解密字面量
（`[type]` = 0x1491EDFB0，`[sub type]` = 0x14922D290，`[preview]` = 0x14B26F348）。
每个字面量匹配成功跳到的 `LABEL_56x` / `LABEL_61x` 就是该标签的赋值点，逐个已核对
（行 626/660、665/892、697/886、729/880、761/874、793/868、825/867 与 935/969、974/1087、
1006/1081、1038/1075）：

| `[type]` 字面量 | VA | `record+8` | 与独立证据 |
| --- | --- | --- | --- |
| `party frame` | 0x14B389420 | 0 | §14.3 面板页 0 ✔ |
| `skill cutscene` | 0x14B389440 | 1 | §14.4 类别 1 ✔ |
| `damage font` | 0x14B389468 | 2 | 实机页 2 ✔ |
| `instant emoticon` | 0x14B389488 | **3** | 新证 |
| `weapon skin` | 0x14B3894B8 | 4 | `df22_ui_1441E47B0.c` 的 `record+8 == 4` ✔ |
| `spray` | 0x14B3894D8 | **7** | 新证 |
| `airship effect` | 0x14B3894F0 | **8** | 新证 |
| 以上皆不匹配 | — | 10 | 页 10 不在 0..9 的枚举范围内 |

| `[sub type]` 字面量 | VA | `record+12` |
| --- | --- | --- |
| `party request frame` | 0x14B389518 | 0 |
| `character info BG` | 0x14B389548 | 1 |
| `raid party list frame` | 0x14B389578 | 2 |
| `second awakening cutscene` | 0x14B3895B0 | 3 |
| 以上皆不匹配 | — | 4 |

注意 `raid party list frame` 是 `[sub type]`，所以它落在 `record+12 = 2` 而**不是**第 2 页：
它的 `[type]` 仍是 `party frame` ⇒ 页 0，与 §14.3「边框一页四槽」一致，两条证据不冲突。

### 14.3 NOTI1546 类别 0（边框）的帧形状与四个槽的归属

`rd_reader_1546_select_body_1444eeca0.c` case 0：
`u32 A, u32 B, u32 C, u16 n, u32 ids[n]`。A/B/C 各自 `sub_1444EBD90(a1,0,id)`+`sub_1444EC2C0`
过关后**按读入顺序并进同一个向量**（`sub_140154010` 追加到 `v88`），尾部 `LABEL_155` 用该向量
整体替换 `mgr+296[0]`；随后 `sub_1401CA430(a1+312)` 清空"已获得"集，把 `ids[n]` 逐条
（同样按第 0 页归属+期限过滤）喂给 `sub_1444E8ED0`，**n==0 时补一条 `sub_1444E8ED0(a1, 80000)`**。

三条独立证据合出"哪一个家庭走 `ids[]`"：
1. 空列表的默认值 80000 = `Skin/PartyListFrame/Default.skn` ⇒ `ids[]` 是副本队伍列表框那组。
2. `sub_1441E86C0`（`df26_ec570caller_sub_1441E86C0.c:441-497`）在同一个格子上二选一：
   `sub_1441D10E0(a1,2)==1` 时问 `sub_1444EC570(mgr,id)`（`+312` 集合成员），否则问
   `sub_1444EC5B0(mgr,0,id)`（`mgr+296[0]` 向量成员）⇒ 面板里恰好有一个分区读 `+312`。
3. 回声读取器 `df23_sub_1444EE820.c:176-200` 的类别 0 循环：20 个槽里遇到
   `rec+12 == 2` 的 id 就 `sub_1444E8ED0(a1, id)` 并**直接 `goto LABEL_58`**（不写向量、
   不看完剩下的槽）⇒ 家庭类 0 里 `+12==2` 的那一族用集合而不用向量。

⇒ `raid party list frame` = 家庭类 0 的 `+12==2`，三件单选项（队伍信息框 / 组队请求框 /
角色信息背景）落在 A/B/C。**A/B/C 的先后次序无所谓**：客户端只把它们并进一个向量，
渲染方各自按 `+12` 过滤（`df26_ebc10caller_sub_1458D0540.c:75-81` 就是"扫向量、只取 `+12==1`"
的一个消费者实例）。

### 14.4 NOTI1546 类别 1（觉醒插图）：一个向量、两格选项、第二个列表无人读

`rd_reader_1546_select_body_1444eeca0.c` case 1：`u16 n, u32 ids[n], u16 m, u32 ids[m]`。
第一列表逐条 `sub_1473A1120(mgr+1328, 职业, id)` 按职业重映射，再按**第 1 页归属**（`a1+152`）
与期限过滤，进向量；第二列表写 `mgr+1152` 与 `mgr+1176` 两个向量（`rd_reader…:368-371`），
**全 dump 集里除构造器置 0 外没有第三个读者**（`grep "+ 1152\|+ 1176"`）⇒ 服务端发 `m=0`，
并把这条记为缺口。

- 谁用这个向量：`df26_ebc10caller_sub_1444EA8A0.c` —— `mgr+1120 = 30000`，向量非空时
  `= vec[sub_146E9BA90() % size]`，即**在已选集合里随机取一条**当本次觉醒插图。
  所以觉醒插图的"选中"天然是集合而不是单值，两格（一觉 / 二觉）共用 `mgr+296[1]`，
  面板按 `+12` 分格（`df26_ebc10caller_sub_1441ECDB0.c:127` 与 `df22_ui_1441E4180.c`：
  `+12==3` 归二觉格）。
- `second awakening cutscene` = 家庭类 1 的 `+12==3`：由 `df23_sub_1444EE820.c:60-90` 反证，
  回声类别 1 把 20 个槽里 **不满足 `rec+8==1 && rec+12==3`** 的 id 才写进向量，
  而实机 2026-09-27 05:28 二觉「解除」发来的正是 `1, 0, 100000`（100000 = 二觉默认）
  ⇒ 回声把它从向量里剔除 = 卸下，与按钮语义一致。
- 职业重映射表 `mgr+1328` 只有构造器 `sub_147389960(a1+1328)` 与读者，没有任何插入点
  （`grep 1328 *.c`）⇒ 在本构建里它是**恒等映射**，服务端直接发 `skin.lst` 的 id 即可。
  NOTI1545 的两个列表也只有第二个走重映射（`df13_noti1545_body_1444eff40.c:117`），同理。

### 14.5 NOTI1545 对 0/1 页同样成立（整页重建，非增量）

`df13_noti1545_body_1444eff40.c:29-33`：`if (a2 <= 9)` 才处理，目标
`v4 = mgr + 136 + 16*page`，进来第一件事是 `sub_1401DBB80` **把这棵树整个清空**，
再按 `u16 数量` 逐条插入 ⇒ 一帧就是该页的绝对状态，这条对 0/1/2…9 页一律成立。
条目宽度：`page == 4` 是裸 u32，其余页是 `{u32 id, u32 expiry}`（`:41-49`）。
到期语义与第 2 页相同：`sub_1444EC2C0`（`df14_page_flag_1444ec2c0.c`）把"标志位置位且
到期值为 0"读成永久，所以永久发放统一写 `expiry = 0`，不引入时钟域。

### 14.6 仍未证的缺口（不得据此发新帧）

1. ~~类别 0/1 的 C2S 载荷形状没有实机样本~~ **已由 §15 关闭**：发送方 `sub_1444F1090` 与
   2026-09-27 23:24 那 13 条原文逐字节对上。判决同时否定"原样存回整张列表"这一条第八轮口径
   —— 类别 1 的请求体是**两张列表拼的**，原样存会把二觉页的行混进抽签池（§15.3③）。
2. ~~`mgr+1152/+1176` 无读者~~ **判据作废**（§15.4）：flush 发送方 `sub_1444F1410` 读它拼
   槽 0..9。它的**渲染**消费者仍未证 ⇒ 服务端照旧发 `m=0`，不猜内容。
3. 家庭类 0 的 `+12` 具体取值只钉死了"raid party list frame == 2"，其余三族谁是 0/1/≥3
   未证 ⇒ 因为 A/B/C 并进同一向量、客户端按 `+12` 自行路由，这个不确定性对服务端无影响，
   故不为它花 IDA 轮。
4. ~~`instant emoticon` / `spray` / `weapon skin` / `airship effect` 四族的页号没有面板读者
   证据 ⇒ 不发页、不入库显示，只保持消耗与记账。~~ **部分关闭**（§18.2）：武器外观 = 页 4 已由
   `df22_ui_1441E47B0.c` 证死；表情 / 涂鸦 / 飞空艇特效三族仍缺「标签 → `record+8` 整数」的
   加载器证据，等 df40，在此之前照旧不发页。

---

## 15. 觉醒插图的「随机」：类别 1 的 88 字节体是两张 10 槽列表（df36 + 2026-09-27 23:24 实机）

起因（用户实机 2026-09-28）：觉醒插图已能更换，但「应用」一个新插图后面板的**随机勾选自动亮**。

### 15.1 客户端自己的 CMD1565 发送方：`sub_1444F1090(mgr, 类别, 向量)`

`df36_fn_f1090_0x1444f1090.c`。三件事一次钉死：

| 事实 | 代码 |
| --- | --- |
| 向量为空时**按类别**插入该页自己的默认 id：0→20000、1→30000、2→1、6→99999999（3/4/9 填 0） | `case 1LL: *v6 = 30000;` 等，同一张 switch |
| 体是 `u32 类别, u32 结果恒 0, u32 ids[20]`，每个 id 过 `sub_1473A1580(mgr+1328, id)` 逆映射，余格 memset 清零 | `memset(&v24[1],0,84); v24[0]=类别; v24[v7+2]=…` |
| 发出去的确实是 1565 的 88 字节体 | `sub_146D746E0(v15, 1565)` + `sub_146D75B10(v17, v24, 88)` |

⇒ 第六轮从 holder 构造默认外推的「每个分区各自的解除 id」在这里被发送方**独立复证**，
而且是一张覆盖四类目的表。⇒ 第八轮文档里"类别 0/1 的载荷形状无实机样本"这条缺口由发送方
与下面的实机原文同时关闭。

调用者按类别各管一页：`sub_1441D4E20`（伤害字体，类别随 `window+2184` 在 2/6 间取）、
`sub_1441D5DA0`（0）、`sub_1441D7100`（7）、`sub_1441D7740`（8）、`sub_1441D7B70`（4）、
`sub_1441E03D0`（3）、`sub_1444F1410`（**1 = 觉醒插图**）。

### 15.2 类别 1 的向量是**两张 10 槽列表拼出来的**：`sub_1444F1410`

- 槽 0..9 ← `mgr+1176` 那张列表；槽 10..19 ← `mgr+1128` 那张（`*(v31+v20+40) = *(v20+a1[141])`）。
- 发之前先 `sub_1444EBC10(mgr,&out,1)` 取 `mgr+296[1]`，把已经在其中的 id 从 `mgr+1128` 的副本里
  剔掉；只有"剔完还有剩 / 两张列表长度不同 / `mgr+1152` 与 `mgr+1176` 内容不同"才发。
  ⇒ 它是**整状态 flush**，不是"玩家点了哪一格"。调用者 `sub_1441D21E0` 是 UI 事件 2475/2373 的
  处理函数（同一个函数随后还把 `mgr+296[6]` 的第一个 id 提交进 holder+232）。
- `mgr+1128` / `+1152` / `+1176` 三张列表在构造函数里**都是空的**
  （`df13_cargo_ctor_1444e81c0.c:95-102` 六个字段全 0）⇒ 客户端发来的 30000/100000 不是出厂
  预置，是面板把**两页各自的"默认"格**当作该页当前项写进去的。

### 15.3 实机判决（`roles_persist_..._20260927_232446_114792_next37`，13 条 C2S 1565 原文）

| 类别 | 槽 0..9 | 槽 10..19 |
| --- | --- | --- |
| 0（4 条） | `20078 50000 60000` / `20000 50000 60000` | 全 0 |
| 1（头 3 条） | `100000` | `30000` |
| 1（消耗 30117 后） | `100000` | `30000 30117` |
| 1（服务端回推之后） | `100000` | `30000 100000 30117` |
| 1（玩家手动取消其余勾选） | `100000` | `30117` |

⇒ ① 二觉页的行**确实**混在同一条类别 1 请求的槽 0，与 §15.2 的两列表结构逐字节吻合；
② 玩家本来就能把该页列表收回到只剩 `30117` ⇒ 面板支持自主决定池里几项；
③ 槽 10 段里冒出来的那个 `100000` 是服务端上一轮回推把它写进 `mgr+296[1]` 之后，客户端又把它
当本项发回来的 ⇒ **放大是服务端造成的，不是客户端的勾选模型。**

### 15.4 `mgr+296[1]` 是**抽签池**，不是"生效中"标记

`sub_1444EA8A0`：`*(mgr+1120) = 30000;` 若池非空则 `*(mgr+1120) = pool[RNG % size]`
（`df26_ebc10caller_sub_1444EA8A0_0x1444ea8a0.c:15-19`）。⇒ 池 ≥2 项**就是**随机，1 项恒定，
0 项回默认。df36 在 0x1444E8000–0x1444F2200 全簇扫 `+1128/+1136/+1152/+1160/+1176/+1184`
的引用（`df36.log` §2，命中的 14 个函数全是列表增删/比较辅助，见 §15.2），
**没有第二个"随机模式"位**：随机与否只由池的长度决定。

顺带修正 §14.6 第 2 条：`mgr+1152/+1176` **有读者**（§15.2 的 flush 发送方读它拼槽 0..9），
"无读者"判据作废；但它的**渲染**消费者仍未证，所以服务端照旧不填内容。

### 15.5 服务端口径（本轮实装）

真源不在"发哪一帧"，而在"池里放哪些 id"：

1. 存/推的类别 1 池 = 请求列表 **减去 30000**（该页"未选"标记，§15.1）**减去注册表 sub type 为
   second awakening 的行**（客户端回声 case 1 自己就拒存：`df23_sub_1444EE820.c:81`）。
2. 回声**仍原样回请求体**，一个字节都不改：客户端要比对自身那两张列表，改它反而破坏比较。
3. ~~NOTI1546 仍只填**第一个列表**（→ `mgr+296[1]`），第二个列表发 `m=0`：二觉池的落地点只被 flush
   发送方读过，没有渲染证据，不猜。~~ **本条已被 df37 作废**：`mgr+1152` 正是副本内二觉插图的渲染池
   （§16.2/§16.3），第二个列表必须填，见 §16.5 第 3 条。

⇒ 应用一项 = 池 1 项 = 恒定该项、随机不亮；玩家勾第二项才 ≥2 进入随机；解除（列表只剩 30000）
= 池空 = 回默认 30000。

---

## 16. 二次觉醒插图在副本里的真实取值：`mgr+1152` 才是渲染池（df37，2026-09-28）

实机现象：觉醒插图（一觉）能换，二次觉醒分类下的插图在副本中不生效。df37 只读轮
（`analysis/ida-work/df37.log`，dump 前缀 `df37_*`）把这条链闭环了。

### 16.1 副本内"用哪张插图"由 `sub_145D451D0(a1, a2)` 决定，有三个来源

| 条件 | 取值 | 证据 |
| ---- | ---- | ---- |
| `sub_145CD39F0(a1,1) == a2`（一觉动画位） | `sub_1444EA8A0(mgr)` → `mgr+296[1][RNG % size]`，空则 30000 | `df36_caller_ea8a0_0x145d451d0.c:35-50` |
| 二觉门 `sub_145CF7C40(a1,a2)` 成立，且 vtable `+4832==3` 且 `+4848==3` 且 `a2==247` | **硬编码 100000** | 同上 `:53-61` |
| 二觉门成立的其他情况 | `sub_1444EBAD0(mgr)` | 同上 `:62-63` |

`sub_145CF7C40` 的门（`df37_gate_0x145cf7c40.c`）：`sub_145CD39F0(a1,2)==a2`，或
记录 `+12==1` 且 vtable`+4832==4` 且 `!+10096` 且 `a2==300`，或 vtable`+4832==10` 且
`a2==261`。`a1` 是角色对象，`a2` 是要播的动画/技能位；`mgr` 由 `sub_140764510()` 取，
就是 `qword_14E638F28` 那个 1472 字节单例（`df37_fn_getmgr_0x140764510.c`）。

### 16.2 `sub_1444EBAD0(mgr)` 逐条语义（`df37_fn_ebad0_0x1444ebad0.c`）

1. 先置默认：`v1 = 100000`。
2. 读 `mgr+1152` / `mgr+1160`（begin/end）；**非空**则
   `v1 = *(mgr+1152 + 4 * (sub_146E9BA90() % size))` —— 与一觉 `sub_1444EA8A0`
   同一个 RNG、同一套"池长度决定随机"语义：长度 1 = 固定，长度 ≥2 = 随机，长度 0 = 默认 100000。
3. `sub_143CA0E80()` 为真时直接返回 100000（与 EA8A0 里那个把结果压回 30000 的全局同源，
   是一个"整族关闭"的开关，服务端不参与）。

`df37.log` §1：**EBAD0 的调用者只有一个** —— `sub_145D451D0`。所以 `mgr+1152` 没有别的读者，
也没有进城快照：每次播插图都是**实时读**。

### 16.3 `mgr+1152` 的唯一写入者是 NOTI1546 类别 1 的第二个列表

`df13_noti1546_body_1444eeca0.c:368-371`（case 1LL）：帧的第二个列表被**同时**赋给
`mgr+1152` 和 `mgr+1176`；第一个列表在 LABEL_155 替换 `mgr+296[1]`。第二个列表的读取循环
只要求 `(int)id > 0`，**不做注册表归属校验**。

⇒ 服务端一直发 `m=0`（`skin_cargo.go` 里那句"no consumer of those two vectors exists"）
就等于**从不投递二觉池**，副本里只能退回硬编码 100000。这就是现象的真源。

CMD1565 的客户端回声（`sub_1444EE820` case 1，`df23_sub_1444EE820.c:64-110`）只写
`mgr+296[1]`，并且主动跳过注册表 `+8==1 且 +12==3`（二觉类）的槽——**回声里没有任何路径写
`mgr+1152`**。所以二觉池只能靠 NOTI1546 的第二个列表填。

### 16.4 88 字节体是**按位置**分的两张列表，各有自己的"未选"标记

| 列表 | 槽位 | 空列表时客户端自插的标记 | 证据 |
| ---- | ---- | ---- | ---- |
| `mgr+1176`（面板里的二觉工作集） | 0..9 | **100000** | `df37_writer_0x1444f1be0.c:14-21` |
| `mgr+1128`（面板里的一觉工作集） | 10..19 | **30000** | `df37_writer_0x1444f1b60.c:12-21` |

两张列表各自最多 10 项（`sub_1444E9290` / `sub_1444E9240` 都带 `size < 0xA` 的门，
`df37_writer_0x1444e9240.c` / `df37_writer_0x1444e9290.c`）。发送方 `sub_1444F1410`
（`df36_fn_f1410_0x1444f1410.c:103-131`）把 `mgr+1176` 铺进槽 0..9、`mgr+1128` 铺进槽 10..19，
再交给 `sub_1444F1090(mgr, 1, vec)`。实机 13 条类别 1 请求（§15.3）与此完全吻合：
槽 0 恒为 100000，一觉选项只出现在槽 10 之后，从未反过来。

⇒ 修正 §15.5 第 1 条的做法：不该"把二觉 id 从一觉池里剔掉"这种按内容猜，而是**按槽位取**；
100000 与 30000 一样是"未选"标记，不是玩家选项。

### 16.5 服务端口径（本轮实装）

1. 类别 1 的请求体按位置拆：`slots 0..9` = 二觉列表，`slots 10..19` = 一觉列表，各自丢 0 值。
2. 两个标记 id（30000 / 100000）都不入库、不下发：它们只代表"该页没选"。
3. NOTI1546 类别 1 的两个列表都填：第一列表 → `mgr+296[1]`（一觉渲染池），
   第二列表 → `mgr+1152`+`mgr+1176`（二觉渲染池）。
4. 落库仍用 `character_skin_selection_list(character_id, category=1, skin_key)` 一张表，
   读回时按 `.skn` 的 sub type 分类（`catalog.IsSecondAwakeningCutscene`，目录里 117 条
   second awakening 行全部带该标签；30000/100000 不在目录中，按 id 丢）——**零 schema 变更**，
   旧行自愈。
5. 二觉池同样遵守"长度决定随机"：只有一项就固定，勾选第二项才随机；这与 §15.4 一觉一致。

**仍未证**：面板二觉页的"生效中"勾选态读的是哪张向量（若读 `mgr+296[1]` 并按注册表 sub type
过滤，则二觉选中态可能只在本次会话的工作列表里，重启后靠 §16.5 第 3 条恢复，不影响副本渲染）；
`sub_145D451D0` 后半段那条 `sub_144507FB0` 覆盖分支（走 `sub_1444EA9D0` / `sub_1444EBB90`
这两组按键的向量）在什么场景命中，未查。

---

## 17. 类别 1 的多选为什么会丢：默认行的双重身份（df36/df37 复读 + 2026-09-28 实机）

用户回报「一觉与二觉插图无法选取应用多个，关闭皮肤仓库后重新打开便失效」。本轮没有花新的 IDA
轮次，也没有花 C2S 次数——答案已在 §15 与 §16 的 dump 里，缺的是把两处放在一起读。

### 17.1 请求体是两张工作集的拼接，不是一张选择表

`sub_1444F1410`（CMD1565 的发送方）对类别 1 做的三件事，按 dump 顺序：

1. 把 `mgr+1176` 与 `mgr+1128` 各自排序，前者写进槽 0..9、后者写进槽 10..19；
2. 复制一份 `mgr+1128`，减去 `mgr+296[1]`（服务端上一帧刚覆盖过的那张选中向量），
   所以**已经生效的一觉行不会再出现在请求里**——请求体是「面板当前显示的行」减去「已生效的行」；
3. 三份内容（左集合、右集合、选中向量）任一与上一次不同才发包。

⇒ 槽位有确定含义：0..9 属于二觉页，10..19 属于一觉页。把 20 个槽当成一张去零列表合并，
就把两页的勾选混成了一页（§15.5 第 3 条已因此划废，本轮按位置分表）。

### 17.2 30000 / 100000 是行，不是哨兵

`sub_1444F1B60`（写 `mgr+1128`）与 `sub_1444F1BE0`（写 `mgr+1176`）的形状一样：
先从向量逐个 insert，**insert 完发现表空**才补进各自默认行的第一个元素。也就是说：

- 表里只有默认行 ⇒ 客户端自己的语义是「这张表没选任何东西」；
- 默认行与真实行并列 ⇒ 它就是那一格被勾上了，玩家要它进抽签池。

实机三场把这条钉死：17:11:02 是 `{30117}` / `{100047}`（各一件，无标记），
17:12:19 是 `{30000,30117}` / `{100000,100047}`（标记与真实行并列），17:13:07 是三行且另一页
只剩标记。若按「见标记即剥」处理第二场，服务端存下的就是 `{30117}` / `{100047}`，玩家勾的
那一格被替他取消了。

### 17.3 丢失为什么表现为「重开才失效」

回声（CMD1565 原样回发）先执行客户端要求的删除，NOTI1546 类别 1 再把两张列表整表覆盖回去：
第一列表覆盖 `mgr+296[1]`，第二列表同时覆盖 `mgr+1152` 与 `mgr+1176`。面板重开时
的勾选态来自这几张工作列表，而不是来自服务端的存储，所以存储被剥短的那一刻界面看不出来，
要等下一次重开（或下一帧覆盖）才显出「刚才的多选没保住」。

**修复口径（服务端）**：分表后每张表只在「唯一元素恰是它的默认行」时判为空，其余原样保留；
跨表迁移的二觉行（从一觉槽位里被识别出来）**替换**目标表的默认行而不是并列，因为玩家从没
在二觉页勾过那一格，它出现在那里只是 §17.1 第 2 条那次减法留下的位移。

**仍未证**：默认行在面板上的「生效中」高亮究竟读 `mgr+296[1]` 还是读两张工作列表（本轮不改
回帧形状，只保证存储与回帧一致）；两张列表各自 10 槽的上限对服务端存储有无约束（客户端截断
发生在发送方，服务端收到什么就存什么）。


---

## 18. 剩余家庭的页号与 CMD1565 的两个发送方（df38 + df39，未闭环部分已标注）

用户口径：按伤害字体 / 觉醒插图那条路子修 表情、角色装饰、变装，并查武器外观「复制进仓库后
不实时生效 + 无法应用」，最后修各分类的星星收藏（概要）。本节只收 df38/df39 已闭合的事实。

### 18.1 皮肤管理器注册了自己的全部 opcode（df38 §3）

`sub_1444E81C0`（ctor，1472 字节，全局 `qword_14E638F28`）末尾逐个登记：
NOTI **1545 / 1546 / 1547 / 1671 / 1672 / 2243 / 2426 / 2641**，CMD **1565 / 2039 / 2140**。
⇒ 表情快捷键（2039/2243）、涂鸦（2140）、收藏（2641）、以及尚未读过的 **2426** 都挂在同一个
对象上，家庭之间共用页表与选中表，不是各自一套管理器。

### 18.2 页 / 面板 / 发送方对照（df38 §1，只列有面板读者者）

| 页=类别 | 面板填充函数 | CMD1565 发送方 | 备注 |
| --- | --- | --- | --- |
| 0 边框 | `sub_1441E86C0` | `sub_1441D5DA0`×2 | 已实装 |
| 1 觉醒插图 | `sub_1441E9650` | `sub_1444F1410` | 已实装 |
| 2 / 类别 6 伤害字体 | `sub_1441E5870`、`sub_1441E3510` | `sub_1441D7100`(7) 等 | 已实装 |
| 3 表情 | `sub_1441DBFE0`、`sub_1441E7BE0` | `sub_1441E03D0` | NOTI1546 类别 3 = 4 个 u32，逐个数过页 3；页号 = `instant emoticon`（§14.2.1） |
| 4 武器外观 | `sub_1441E47B0` | `sub_1441D7B70`×2 | 页内条目只有裸 u32（无期限列） |
| 7 涂鸦 | `sub_1441EA4F0` | `sub_1441D7100` | 页 7 网格无 `record+8` 过滤、无默认行；页号 = `spray`（§14.2.1） |
| 8 飞空艇特效 | `sub_1441E4600`(页 8)、`sub_1441EB0F0` | `sub_1441D7740`×2 | 页号 = `airship effect`（§14.2.1） |
| 9 | `sub_1441ECBC0` + 填充 `sub_1441E36F0` | 见 §18.3 | NOTI1546 类别 9 = 4 个 u32 + 一张 id 集合，**不做页校验** |
| 5、6 | 无面板读者证据 | — | 不发帧 |

武器外观 == 页 4 有两条独立证据：`df22_ui_1441E47B0.c` 枚举页 4 且只保留
`sub_1444EBAB0(0x9C40)`（`Skin/WeaponSkin/default.skn`）那条记录的 `record+8 == 4`；
`0x9C40` 是所有 dump 里唯一硬编码的注册表 id。

### 18.3 result 字段是**命令种类**，不是状态码（df39 §4，本轮最重要的一条）

同一个 88 字节体有两个不同的组装函数：

- `sub_1444F1090(mgr, 类别, 向量)`：`memset(&v24[1], 0, 84)` 之后再没写过槽 1，
  ⇒ **result 恒为 0**。它在向量为空时替该页补上自己的默认行（0→20000、1→30000、2→1、
  6→99999999、3 与 9 各补四个 0、4 补一个 0），每个 id 先过 `sub_1473A1580(mgr+1328, id)`
  的按职业重映射。这就是「应用 / 打开页」那一路。
- `sub_1444F0FE0(mgr, 类别, id, flag)`：`v12[1] = (flag != 0) + 2` ⇒ **result 只能是 2 或 3**，
  同样过按职业重映射。它唯一的调用者 `sub_1441DD510` 遍历面板 +3280/+3288 之间的 48 字节节点，
  节点 +36 当类别、+32 当 skin id，`sub_141FB6530(node+16)` 提供 flag，
  且先用 `sub_1444EB840(职业, 类型)` 计数、**≥10 就拒**并发 101037008 那条提示（节点 +40 置位时
  走 101036910），都通过才发一帧。⇒ 这一路就是**星星收藏 / 概要**，每类上限十个。

实机判决：`roles_persist_..._20260928_025044_035087_next37` 里那五条类别 4 请求不是同一种点击
——19:37:50 与 20:56:19 是 `result 0`（应用），19:38:23 连续两条是 `result 2`（星星），
五条全部被服务端拒收（`skin_selection_unsupported_category`）。

### 18.4 收藏列表客户端只会**接收**，不会自己填（df39 §5）

`sub_1444E8DF0`（往 `mgr+8` 那条 32 字节节点链表插 `{页, 子键, id}`）在全 executable 里只有一个
调用者：NOTI2641 的读取器 `sub_1444ED1B0`。⇒ 概要里的收藏内容只能由服务端下发；玩家点星星只是
发 §18.3 那帧 result 2/3，界面不会自己长出一行。收藏帧形状：先清表，再按页 0..9 各读
`u32 count` + `count × u32 id` 插 `{页,-1,id}`，最后 4 组 `{ -1, j }`，每个 id 过按职业重映射。

### 18.5 服务端本轮口径（武器外观）

`selectSkin` 收到 `category 4 && result 0` 就转给 `syncSkin`（存皮肤 + 回 NOTI1546 类别 4 +
重发 opcode 2 外观块），`result 2/3` 仍按未支持记录，因为那是收藏开关、不是穿戴。
`main.go` 里第二段 `frame.ID == 1565 → syncSkin` 分支依旧不可达（第一段两条路径都 `continue`），
本轮**不动那个共享文件**，改在 `selectSkin` 单点分流。

### 18.6 仍未证（不得据此发新帧）

1. ~~注册表把 `[type]` / `[sub type]` 标签写成 `record+8` 整数值的那个**加载器还没反编译出来**。~~
   **已闭环（df40）**：阶梯与赋值点逐个核对见 §14.2.1，
   `instant emoticon` = 页 3、`spray` = 页 7、`airship effect` = 页 8，未匹配 = 10。
2. ~~类别 9 的 `result 1`（实机两条）既不是 §18.3 的两个发送方能产生的值，发送方未点名~~
   **已闭环（df41 §4 + df42 §5）**：第三个组装函数 `sub_1444F1310(mgr, result, 类别, 向量)`
   （`df41_composer_F1310.c:18-19` 写 `v14[1]=a2` 即 result 槽、`v14[0]=a3` 即类别槽，其余
   80 字节是 20 个 id 且逐个过 `sub_1473A1580(mgr+1328, id)` 的按职业重映射）的四个调用者里，
   `sub_1444F0B40`、`sub_1444F1C40` 都以 **(result 1, 类别 9)** 发帧
   （`df42_cat9_sender_F0B40_1444f0b40.c:79`、`df42_cat9_sender_F1C40_1444f1c40.c:131`），
   `sub_1444F0D00` 以 (0, 9) 发（同目录 :27）。⇒ 实机那两条 result 1 的发送方就是这一族，
   result 取值集合扩大为 **{0, 1, 2, 3, 4}**，其中 1 只出现在类别 9。
3. ~~NOTI2426（`sub_1444EDD60`）从未读过，是否就是概要页的推送未知~~
   **已闭环（df41 §5 + df42 §1/§4）**：`sub_1444EDD60` 先读一个模式字，模式 <2 时把每行
   （27 字节、以 u16 键开头）交给 `sub_1444E9010`，模式 == 2 时在 `mgr+1440` 那棵红黑树里按
   u16 键删除（`df41_handler_NOTI_2426_1444edd60.c:30-48`、`:50-130`）。而 df42 §1 对管理器
   区间 96 个函数逐个检查了「谁遍历 `mgr+8` 收藏链表」，`sub_1444EDD60` 命中数 **0**，
   `sub_1444E9010` 的唯一调用者也只有它自己 ⇒ **2426 写的是 `mgr+1440` 那张 u16 键表，
   与概要/收藏无关，已排除**。收藏帧只有 NOTI2641 一个来源（§18.4）。
4. 变装（costume）、宠物（creature）、阴影（shadow）在 1733 条 `[add skin storage]` 模板里
   **一条都没有**（实测计数见 §18.7），所以「变装系统」这一分类没有消耗品入口，
   服务端拿不到可入库的 skin id ⇒ 不是接线缺失，是数据源缺失，记为证据缺口。
5. ~~页 3 / 7 / 8 的条目内期限列是否存在~~ **已闭环**：`df13_noti1545_body_1444eff40.c:47-57`
   是 `if (a2 == 4) { 读 4 字节 } else { sub_146EA0BE0(&v32, 8) 读 8 字节 }`，
   ⇒ **只有页 4 是裸 u32，其余 0..9 页（含 3/7/8）都是 `u32 id + u32 期限`**；
   第二个列表的 id 还要过 `sub_1473A1120(mgr+1328, 职业, id)` 的按职业重映射（同文件 :117）。
   ~~这三页仍未做的是**实机回归**（服务端还没为它们发过页）。~~
   **本轮服务端已按 §19 的形状为页 3 / 7 / 8 发帧，待实机回归。**

### 18.7 `[add skin storage]` 模板按 `.skn` 标签的实测分布（`configs/skin-storage-items.json`，1733 条）

instant emoticon 540、skill cutscene 476 + second awakening cutscene 117、damage font 354、
party frame 153 + party request frame 40 + character info BG 28 + raid party list frame 3、
airship effect 11、spray 9、weapon skin 2。
⇒ 本轮真正要新增的家庭只有 **instant emoticon（540）**、**spray（9）**、**airship effect（11）**
三族，外加 2 条走消耗品发进来的 weapon skin；其余都已实装或无数据源。

## 19. 页 3 / 7 / 8 的选择形状、星星收藏的线上形状（df41 + df42，本轮实装的依据）

用户口径的最后一批：表情（页/类别 3）、涂鸦（7）、飞空艇特效（8）按伤害字体那条路子接通，
外加「各分类的星星收藏 → 概要」。本节只收 df41/df42 已闭合、且服务端本轮据以下发包的事实。

### 19.1 三个组装函数把 88 字节的 result 槽写成三种不同的命令（补 §18.3）

| 组装函数 | result 槽写法 | 发送方 | 语义 |
| --- | --- | --- | --- |
| `sub_1444F1090(mgr, 类别, 向量)` | `memset(&v24[1],0,84)` 后再不写 ⇒ 恒 **0** | 各面板的 应用 / 整页 flush | 穿戴 |
| `sub_1444F1310(mgr, result, 类别, 向量)` | 由调用者传入（`df41_composer_F1310.c:18-19`） | `sub_1444F0B40`、`sub_1444F1C40` 传 **1**，`sub_1444F0D00` 传 **0**，类别都是 **9** | 类别 9 专有，本轮未实装 |
| `sub_1444F0FE0(mgr, 类别, id, flag)` | `v12[1] = (flag != 0) + 2` ⇒ **2 或 3** | 唯一调用者 `sub_1441DD510` | 星星收藏开 / 关 |
| `sub_1444F1880(mgr, 类别, id)` | `v8[1] = 4`（`df41_composer_F1880.c:23`），`a3 == -1` 时**整个不发** | 唯一调用者 `sub_1441D5DA0`（边框面板） | 从已获取集合里抹掉一个 id |

`sub_1444F1310` 与 `sub_1444F1090` 一样，把向量的每个 id 先过
`sub_1473A1580(mgr+1328, id)` 的按职业重映射、最多取 20 个（同文件 :26-33）。
⇒ 服务端解 CMD1565 时 result 只能按 {0,1,2,3,4} 分流，**不能当状态码**；本轮只对 0（穿戴）与
2/3（星星）发包，1 与 4 只回 echo。

### 19.2 星星一族的入口与上限（df41 §5）

`sub_1441DD510`（唯一被 `sub_1441DBB60` 调用）遍历面板 +3280..+3288 的 48 字节节点：+36 = 类别、
+32 = skin id、`sub_141FB6530(node+16)` = 当前星状态，先用 `sub_1444EB840(职业, 类型)` 数该组已有
多少颗，**≥10 直接拒**并发 101037008 那条提示（节点 +40 置位时走 101036910），通过才发一帧。
⇒ 每类上限十个是**客户端自己先拦**的，服务端只需在同一口径上拒绝，不要指望收到第 11 条。

### 19.3 表情是**位置性**的，涂鸦 / 飞空艇特效是**单值**的（df41 §1/§2/§3）

* 类别 3 的选中向量只有一个读者 `sub_1444EC930`，它把整条向量原样交给聊天频道
  `sub_14668C520(qword_14E683C78, 34, &v24, 0)`（`df41_reader_ebc10_sub_1444EC930_1444ec930.c:107`）；
  发送方 `sub_1441E03D0` 从 `panel+4176` 按步长 120 取**四格**。
  ⇒ 表情必须按槽位存、按槽位回，同一个 id 占两格是合法状态，不能折成集合。
* 同一条链上 NOTI1546 的 `case 3` 是**固定四轮**（`df40_core_sub_1444EECA0.c:479-556`：
  `v40 = 4` 起步，每轮读一个 u32；id 为 0 ⇒ 追加 0；id 非 0 但**不在 `a1+184` 那棵页 3
  归属树里 / 已过期 ⇒ 走 LABEL_132，一个字节都不追加**）。
  ⇒ 服务端若把一个账号不拥有的 id 塞进中间槽，客户端会把那一格**整格吃掉**、后面的槽位左移，
  四格就错位。所以发包前必须逐格做归属过滤（不拥有 ⇒ 写成 0），不能指望客户端兜底。
* NOTI1546 的 `case 7` / `case 8` 各只读**一个 u32**，随后
  `sub_1444EBD90(mgr, 页, id)` 判归属、`sub_1444EC2C0` 判期限，**任一不通过就 `goto LABEL_208`
  而 LABEL_208 只是释放临时向量**（`df40_core_sub_1444EECA0.c:612-628`、`:775`、`:447-466`）
  ⇒ 这两族只能通过 NOTI1546 **选中**，**清场做不到**：清空得靠 CMD1565 的回帧（echo）。
* 页 3 / 7 / 8 的网格填充函数（`sub_1441E7BE0`、`sub_1441EA4F0`、`sub_1441EB0F0`）里
  **没有任何内置默认行的 id 字面量**（对比类别 0→20000、1→30000、2→1、6→99999999，
  df41 §3 的三个 dump 中 20000/30000/99999999 一个都不出现）
  ⇒ 这三族的页帧只带账号注册过的 skin，服务端不补默认行，客户端也不会自己长出一行。

### 19.4 NOTI2641 的线上形状：**组下标就是注册页号**（df41 §5，本轮收藏帧的依据）

`sub_1444ED1B0`：先把 `mgr+8` 那条 32 字节节点链表整条清空并复位计数（`:40-56`），再

```
do { u32 count; for (i=0;i<count;++i) { u32 id; id = sub_1473A1120(mgr+1328, 职业, id);
     插入 { +0 = v4(组下标), +4 = -1, +8 = id, +12 = count }; } } while (++v4 < 10);
for (j=0;j<4;++j) { u32 count; count × u32 id ⇒ 插入 { -1, j, id, count }; }
```

⇒ **前十组没有页号字段**，组 i 就是注册表页 i（§14.2.1 的 0..9 阶梯），服务端发十组即可；
节点第四个字段是该组的 count，不是期限。后四组的键是 `{ -1, j }`，语义未证 ⇒ 本轮**发空计数**，
不发明 id。清空 = 十组全发 count 0（reader 解析前就清表，所以全空帧是安全的抹除手段）。
收藏表唯一写入者 `sub_1444E8DF0` 的唯一调用者仍是这个 reader（df42 §5 再次数过：callers=1），
⇒ 概要内容只能由服务端下发，玩家点星星本身不会让界面长出一行，必须回 NOTI2641。

### 19.5 收藏分组键取值：页号来自请求类别，只有类别 6 需要折叠

§18.2 的对照里类别与页号一一对应（伤害字体是唯一例外：类别 2 与 6 共用页 2），
星星帧 `sub_1444F0FE0` 传的第一个字就是**类别**，⇒ 服务端按 `page = category` 落位、
仅把 6 折成 2，其余不猜。类别 ≥10（未匹配标签，§14.2.1）与 id 0 一律拒收只回 echo，
不发收藏帧。

### 19.6 仍未证（本轮未据此发包）

1. 概要页**渲染收藏行**的那个读取方没有点名：df42 §1 把管理器区间 96 个函数全反编译，
   其中遍历 `mgr+8` 收藏链表的有 16 个；§2 逐个列了它们的调用者，能追到界面侧的两条是
   `sub_1444EB5F0`（六个网格填充者共用，但函数体只是拼 100002241 / 100002242 / 100002250
   三条提示串并把计数 +1900 格式化进去，
   `df42_mgr_walk_sub_1444EB5F0_1444eb5f0.c:30-84`）与 `sub_1444E9910`
   （六个调用者全在 `0x141FD7480`~`0x141FD9500` 这段 UI 模块里，df42 §2，本轮**未反编译**）
   ⇒ **收藏表能收到、能计数，但概要界面把星画在哪一格仍未证**，
   实机以「概要页有没有行」为准。
2. 后四个交叉组 `{ -1, j }` 的含义未知 ⇒ 只发 count 0。
3. 副本内是否需要重发 NOTI2641 未证：收藏表挂在 `qword_14E638F28` 这个单例上，
   而它的 ctor `sub_1444E81C0` 有 8 个调用点（df42 §4：`0x14075CB00`、`0x14075EC90`、
   `0x140764510`、`0x140766270`、`0x14087D030`、`0x141543E80`、`0x141566440`、`0x141566A10`），
   本轮**没有逐个确认这些点里哪几个是进副本**，所以只在**进城**与**切换之后**发帧，
   副本内不重发；若实机表现为出副本后概要空，就说明 ctor 在切图时被重建，届时补重发。

---

### 19.7 星星/概要刷新：CMD1565 回帧的**唯一作用**就是重绘那一页（df50 + df51，实机 2026-09-28）

实机回报「加入收藏已生效，但星星不是点击点亮、再点取消；概要也不随各分类应用的各种样式实时更改」。
本轮把回帧的作用范围钉死，并据此把两帧的**先后**改了过来（服务端 `cmd/wireprobe/skin_family_flow.go`
`skinFavoriteAnswer`）。

1. **`sub_1444EE820`（CMD1565 接收核）对 result ≠ 0 什么都不做，只留一次重绘**：
   `if (!v43[1]) switch (v43[0]) {…}` 整段只在 result 为 0 时跑（`df40_core_sub_1444EE820.c`、
   `rd_cmd1565_select_skin_else_1444ee820.c`）；result 2/3（星星）落到
   `LABEL_58: sub_1444F1EE0(a1, v43[0])`。而 `sub_1444F1EE0(mgr, 类别)` 只做一件事——
   `sub_14667BB90(qword_14E683C78, 730, 0)` 取皮肤仓库窗口，再
   `sub_1441ECDB0(win, 页)` + `sub_1441C4010/C4080` 重建那一页（`df13_cargo_render_tail_1444f1ee0.c`）。
   ⇒ **回帧 = 按管理器现状重绘该页**，它自己不写任何状态；帧尾没有任何发包调用，
   上一轮担心的「回声再触发 1565 回环」在这条路径上不成立。
2. **所以两帧的顺序就是结果**：NOTI2641 是 `mgr+8` 收藏表唯一写者（§18.4），回帧是重绘触发器。
   先回帧后发表 ⇒ 重绘读到的是点击之前的表，星星与概要停在旧状态，直到别的东西再重绘一次。
   服务端原来正是这个顺序（`skin_selection_echo_only` 排在 `skin_favorites_restored` 前面）。
3. **`sub_146ECFD90` 不是纯读勾选位**（df50，`df50_getter_Fd90.c`）：
   `return *(u8*)(w+141) && dword_14DC6B63C == *(u32*)(w+496);`
   ⇒ 全局 `dword_14DC6B63C` 与 `w+496` 不等时恒 false。行的复选框由 `sub_1441DBF10` 经子控件
   vtable+16 刷新，取值来自 `sub_141FB6530(child)` = `*(u8*)(child+140)`；`sub_1441DCE30` 对 9 个行各刷一次。
4. **`child+140` 不是星星（否证）**：邻域内唯一写它的是 `sub_141FB65B0`，它把
   `a2+12/+16/+20/+24/+28` 五个 u32 求和后与全局 `qword_14E650BF8` 的容量比，
   并把 `a2` 的四个字段写进 `child+124..136`（`df51_write140_sub_141FB65B0.c`）——这是**物品格记录**的
   数量/堆叠口径，不是收藏位。⇒ 星星的存储态不在这里。
5. **仍未证（不据此发包）**：`*(u8*)(w+141)` 的写入方、`dword_14DC6B63C` 的身份（df51 §2 的读写分类
   把比较指令误判成写，只有 `sub_14674E900`/`sub_14674EA50` 在 UI 模块外，未反编译确认）、
   以及「概要页那四个分类的**已应用预览**是不是也从 `mgr+8` 画」——df47 的六个绘制方全部只调
   `sub_1444EC320`（对 `mgr+8` 的存在性查询，`df14_cargo_small_1444ec320.c`），
   只能证明**概要里的行**由收藏表决定，不能证明那四个预览槽同源。
   df51 §3（扫 `byte ptr [reg+8Ch]/[+8Dh]` 写入点）因 `idc.find_func_start` 不存在整段没跑，
   下一轮用 `ida_funcs.get_func` 重跑再判。


## 20. 「最近获得」的唯一数据源：NOTI1547 → `mgr+1096` → 五行格条（df43–df49，本轮实装的依据）

起因（用户实机 2026-09-28）：消耗品入库并应用的伤害字体 / 觉醒插图从不出现在
recently acquired 分类里。§8 与 `docs/protocol/skin-cargo-scaffolding-20260926.md` 早就把
NOTI1547 的字节形状钉死了，但当时记的是「`mgr+1096` 向量的消费者未追到 ⇒ 不发帧」；
本轮把这条消费链从 reader 一路追到界面，并据此第一次发出这帧。

### 20.1 消费链（每一环都只有一条路径，没有分叉）

| 环节 | 函数 | 事实 | dump |
| --- | --- | --- | --- |
| 读取器 | `sub_1444ED400` | 解析前先 `v0[138] = v0[137]` ⇒ **进 loop 就整表清空**，一帧就是全量而非追加；随后 `u8 count`，每条 `{u8 kind, u32 id}`，落成 8 字节元素 `{u32 id@0, u32 kind@4}`（`HIDWORD(v5)=v7` 是 kind、`LODWORD(v5)=v4` 是 id，**线上顺序与内存顺序相反**） | `df38_handler_NOTI_1547_0x1444ed400.c`、`df43_mgr_sub_1444ED400_*.c` |
| 存放 | `qword_14E638F28 + 1096/1104/1112` | begin / end / cap 三指针的 `std::vector` | 同上 |
| 唯一读者 | `sub_1444EB880(mgr, out)` | 把整条向量复制进 `out`；`XrefsTo` 全库**只有一个调用者** | `df46_accessor_sub_1444EB880_0x1444EB880.c` |
| 唯一调用者 | `sub_1441EAF20` | 「最近获得」那 5 格条的重绘（`off_14A230CD8` 槽 0，条对象嵌在窗口 `+5232`） | `df44_recent_caller_sub_1441EAF20_0x1441EAF20.c` |

`sub_1441EAF20` 的形状：先把 `strip+200` 起步、步长 50 的 5 个子控件各 `vtable+16(this, 0)`
清一遍，再取 `sub_1444EB880` 的副本，**从 `end-8` 往前**走（`v12 -= 2`）、`v11 < 5`，
即**向量尾部 = 显示在第一格 ⇒ 线上顺序必须是「旧 → 新」**。每格做四件事：
`(*(row+48))[2](1)` 显示、`(*(row+0))[2](row, id, kind)` 填内容、
`(*(row+232))[2](owned)`、`(*(row+200))[2](0)` 与 `(*(row+8))[2](0)` 复位，其中
`owned = (id ∈ 窗口 +3328..+3336 的 u32 向量)`。

行子对象（400 字节 ×5）不是靠反搜派发点找到的：df48 从窗口 ctor `sub_1441B8E90`
里读到 `sub_148860630(window+5384, size=400, count=5, ctor=sub_1441B98C0, dtor=sub_1441BB860)`
（`window+5384` 正是 `strip+152`），再在 `sub_1441B98C0` 里取它给 `row+0` 装的
**最后一次**赋值 `off_14A230C10`（先 `&off_14A230B78` 后 `off_14A230C10`，基类被派生类覆盖），
槽 +16 = `sub_1441E2620` = `(id, kind)` 的落点。

### 20.2 `kind` 的真身：它必须等于该皮肤的注册表家庭类（= 仓库页号），否则整格被拒

`sub_1441E2620`（`df49_rowslot2_0x1441E2620.c`）的通用分支：

```c
v36 = sub_1444EBAB0(id);                    // 静态皮肤注册表，§14.2.1
if (kind == 4) v36 = sub_1444EBAB0(40000);  // Skin/WeaponSkin.skn 哨兵，其 +8 == 4
if (!v36 || (kind != 10 && *(DWORD*)(v36 + 8) != kind)) return 0;   // ← 整格拒画
*(DWORD*)(row + 40) = id;  *(DWORD*)(row + 44) = kind;
```

⇒ **`kind` 不是标志位，是家庭类**：取值必须等于 `record+8`（§14.1/§14.2.1 里
「NOTI1545 页号 == `mgr+296` 选中类别号 == 注册表 `+8`」同一个下标），
`10` 是「不校验家庭类」的通配。图标与文字**一律从注册表记录取**
（`+352` 名字串、`+384` 图标号、`+388/392/396` 颜色、`+12` 子家庭决定用 `row+96` 还是 `row+128`
那个子控件），服务端只提供 `(id, kind)`。
`row+44` 存的是 kind，行的其余虚方法（槽 0 = `sub_1441C72A0`，3618 字节）后续按它路由，
所以**不能图省事发 10**，要发该皮肤自己那一页的页号。

武器外观（页 4）是唯一换命名空间的族：kind 4 的显示名改走
`sub_14021BE90(qword_14E683B30, id, 1)`（item 表），⇒ **页 4 的 id 是武器 item template**，
与 NOTI1545 页 4 那条「只有 `page==4` 是裸 u32」的列表同一个空间。

### 20.3 两个本轮用不上的特殊 kind

* `kind == 5`：`id` 是**下标**不是皮肤 id，用它索引一张 96 字节步长的全局数组
  （`qword_14E6A7C68`..`qword_14E6A7C70`，越界即 `return 0`），名字取 `*(rec+56)`，
  并额外走一次 `qword_14F1C39C8` 的取图链。
* `kind == 9`：查的是**另一张注册表** `sub_140283D60(qword_14E683B38, id, 1)`，
  名字 `+496`、图标 `+352/+368`，完全不碰 `sub_1444EBAB0`。

§14.2.1 的标签阶梯只给出 `party frame`=0、`skill cutscene`=1、`damage font`=2、
`instant emoticon`=3、`weapon skin`=4、`spray`=7、`airship effect`=8、其余=10；
5 与 9 对应的 `[type]` 字面量仍未名，而 `configs/skin-storage-items.json` 的 1733 条
`[add skin storage]` 全落在那七个族里 ⇒ 本轮不为 5/9 发任何 id。

### 20.4 否证：两条曾经被怀疑的来源都不是（省掉两轮实装）

* **NOTI1671 的 `mgr+328` 桶网格没有任何界面读取方**。df43 用 `+ 328` 拼写、df44 用
  qword 下标 `v0[41]` 拼写，对管理器区间、面板区间、星星区间加全库 107 个单例引用者
  各扫一遍，命中只剩 reader/writer 自己（`df44_grid_sub_1441E7BE0/1441E86C0/1441E9650.c`、
  `df44_grid_sub_1444ED400/1444EE1C0.c`）。
* **NOTI1545 节点的标志字节 `+44` / `+52` 不参与判定**：唯一的写者是页框 reader，
  唯一的"读"者是 56 字节节点的通用克隆器 `sub_141FD7060`（整块拷贝，从不比较，
  `df43_star_sub_141FD7060_*.c`）⇒「stamp 写 1」不能当界面显示的依据。

### 20.5 df45 的一处误判要更正：窗口 `+3328` 不是「最近获得 id 表」

df45 把 `window+3328/+3336` 记成 recent-id 向量。复读
`df45_recentvec_sub_1441D8460_0x1441D8460.c:174-274`（开窗时用实参向量整体重填）与
`df45_recentvec_sub_1441ECDB0_0x1441ECDB0.c`（**入参是类别号**：先
`sub_1444EBC10(mgr, out, 类别)` 取该类别**选中向量**，按类别 switch 把元素交给
`sub_1441E0820(window, 类别, id)` 做预览，再把该向量首元素从 `+3328` 中删除并
`sub_1441EC510(window)` 刷新页签）后，它的身份是**本次开窗的选中/待确认 id 向量**，
与 `mgr+1096` 是两套状态。`sub_1444F1EE0(mgr, 类别)` 才是它的服务端触发方：
先 `sub_14667BB90(qword_14E683C78, 730, 0)` **按窗口号 730 找到皮肤仓库窗口**，
类别 10 时对 0..9 全刷、否则只刷那一类（`df13_cargo_render_tail_1444f1ee0.c`）。
⇒ 这一族的数据源仍是 NOTI1546（选中表），本轮**不为 `+3328` 发新帧**；
`sub_1441EAF20` 里那个 `owned` 布尔的含义（该格是否已在本次开窗的选中集合里）按此理解。

### 20.6 服务端据此实装（本轮）

1. **入场**推一帧全量 NOTI1547；**CMD507 action 169 注册成功**后重推全表；
   **武器复制成功**后重推全表。帧是绝对状态，所以三处都发整表而不是新增那一条。
2. `kind` 取该皮肤家庭的页号：`party frame`0 / `skill cutscene`1 / `damage font`2 /
   `instant emoticon`3 / `spray`7 / `airship effect`8，武器外观固定 4。
   家庭没有已证页号的（`SkinFamilyUnknown`）不进表。
3. `id`：非页 4 用 `account_skin_cargo.skin_key`（= `list/skin.lst` id，§14.2.1 注册表认识它），
   页 4 用武器 item template。
4. 顺序：账号皮肤按 `unlocked_at` 升序（同一 `skin_key` 取最早一次），再拼该角色
   `WeaponSkins` 的追加顺序；超过 `u8` 上限 255 时丢表头（最旧），保证尾部是最新。
   `account_skin_cargo.unlocked_at` 早已存在 ⇒ **零迁移**，不改表。
5. 上一轮复制路径发的 `{Kind: 0, SkinID: template}` 按 §20.2 的谓词必然 `return 0`
   （武器 template 查不到皮肤注册表），是错的，改成 `{Kind: 4, SkinID: template}`。

### 20.7 仍未证（不据此发包）

1. 「最近获得」在服务端侧有没有保留期（例如只记 N 天）没有证据；本实现把整张
   `account_skin_cargo` 当最近列表下发，只受 255 条上限约束。
2. 武器外观 id 在存档里**没有时间戳**（`WeaponSkins` 只是 id 数组），所以它与账号皮肤
   混排时的先后是**实现选择**而不是客户端事实。
3. `kind` 5 / 9 的 `[type]` 字面量仍未名（§20.3）。
4. 行子控件 `+8 / +48 / +200 / +232` 各自是什么控件、以及点击一格走的是行的哪个槽
   未证 ⇒ 本轮只填内容，不假设点击能代替 CMD1565 的应用。

## 21. 表情快捷键的上行帧：CMD 1551 体 = `u32 表情皮肤 id, u32 0`（实机 2026-09-28 + df52）

用户实机回报「表情可以正确配置，但按快捷键后角色头顶不出表情」。本场取证把上行帧钉死了，
回包半**未闭环**，按硬约束记成缺口，不据此发包。

1. **上行帧就是 1551**：`roles_persist_..._20260928_092434_869967_next37` 里用户按了 5 次快捷键，
   5 个时间戳（01:25:56 / 01:26:04 / 01:26:29 / 01:26:55 / 01:27:00）上**各自只有一帧** CMD 1551
   （整帧 21 字节 = 13 头 + 8 体），前后 2 秒内没有任何别的未登记 opcode；本场与前面 10 场的
   opcode **差集为空** ⇒ 1551 不是这场新冒出来的，而是之前几场也按过。服务端对 1551 零处理
   （`grep -rn '1551' cmd internal` 无命中），帧到即丢。
2. **体首 u32 是表情皮肤 id**：实机值 `0x27C3 = 10179`，正是 `configs/skin-storage-items.json`
   里 `skin_type == "instant emoticon"` 的皮肤 id（模板 10325568 →
   `skin/instantemoticon/211021_legendofbits_emotion_04.skn`）。它不是字符串 id
   （`analysis/dumps/dstr_id_to_text.json` 里没有 10179）。第二个 u32 恒 0，语义未证。
3. **上下行不是同一个形状，不能原样回声**：客户端 1551 的接收处理器挂在 NOTI 表
   （`sub_14599D5D0(qword_14E66C090, 1551, sub_14449CDB0, 0)`，`df38_registrar_0x14449caf0.c:58`）。
   df52 反编译 `sub_14449CDB0`（`df52_handler_1551.c`）：读 `u8, u8, u32`，再把后两个写进 496 字节
   单例 `qword_14E659EA8` 的 `+64+8*i` 与 `+68+8*i`（`i` = 第一个 u8）。把上行体按这个读法解就是
   `i=0xC3=195`，而该表只有 `(496-64)/8 = 54` 格 ⇒ 越界，说明**下行体是另一套字段**（一格一个
   `{标志, 值}`），不是上行的回声。
4. **表情族另外两帧的读者形状**（df38 dump，本轮未反编译 2345 的消费者）：
   `sub_1444E8CC0(a1, 状态, 错误码)`（CMD 2039）**一个体字节都不读**——状态非 0 只发界面事件
   `sub_146694510(qword_14E683C78, 2345, -1, 0, 1)`，状态 0 才走 `sub_1444EC790(错误码)` 弹错误文本。
   `sub_1444ECFF0`（NOTI 2243）读 `u8 选择子`（0/1 二选一）+ `u8 条数` + 每条 25 字节，写进
   `mgr+1200`（容量 4）与 `mgr+1360`（容量 2）两条向量 ⇒ 那是**四格快捷表情配置**本身，
   与 §19.3「发送方 `sub_1441E03D0` 从 `panel+4176` 按步长 120 取四格」对得上。
5. **缺口（本轮不发包）**：① 没有任何证据说明 2039 就是 1551 的回包——实机 8 场里客户端从未
   发过 2039，而 2039 在 CMD 表（kind 1 = 命令回包）却没人请求它，两种可能都还在；
   ② 界面事件 2345 的消费者没点名，不能断言它就是头顶气泡；③ 上行体第二个 u32 语义未证；
   ④ 五种按键体完全相同，无法区分「按不同格子会不会变」——需要只按同一格与只按另一格各一次
   再抓一场。df53 两条路：找 2345 的注册/消费方；找客户端 1551 的**发送方**，看它发完挂了
   什么等待态（`sub_14449CAF0` 是那个 496 字节单例的 ctor，也是 1551 的登记点，从它的调用者
   往下找最快）。
6. **实机判决（attempt 1/3 否证，2026-09-28）**：换候选后四格各按一次**无气泡**，而那一场的日志
   证明帧发出去了（六次按键各配一条 `emote_use_acknowledged`，`skin_id` 依格为 10179 / 10175 / 10176，
   没有一次被采样门吃掉）⇒ 否证的是「2039 是 1551 的回包」这个假设本身，不是接线。支持这个判决的
   独立理由见上面第 5 条②：`sub_1444E8CC0` 不读体，2039 表达不了「哪个角色放哪个表情」，而气泡必须
   点名角色。服务端已回滚该回包，只留 `emote_use_observed` 观测与 1551 的解密白名单。
7. **一条不能当结论用的阴性**：df54 想按「发包组装方」定位 1551 的发送方（`sub_146D746E0` 写 opcode，
   调用者 2837 个；`sub_146D74000` 取写包对象，调用者 2918 个），在每个调用者机器码里扫小端
   `0F 06 00 00`，命中 0。**探针缺阳性对照**：没扫已知由 `sub_1444F0FE0` 写入的 1565（`1D 06 00 00`），
   因此无法区分「1551 不走这条路」与「扫描本身坏」（opcode 若以 u16 立即数编进 `mov cx, 60Fh`，
   4 字节模式压根不存在）。df55 必须先把 1565 / 1592 当对照加进同一张表，再谈阴性。

## 22. 武器幻化两步流程的可表达性：消耗品带不动「哪把武器」（PVF 直读 + df55，2026-09-28）

用户给的口径是：林纳斯复制出的武器外观**先进背包消耗品栏**（钢制模具出的可交易、普通模具出的绑定），
右键使用才永久登记进该角色的皮肤仓库武器页。本轮要回答的是「本 115 客户端能不能表达这件事」。

1. **客户端登记哪个皮肤，只看道具定义里的一个静态 u32**：谓词 `sub_1444EC1B0`
   （`df34_cand_0x1444ec1b0.c`）取 `sub_145ABB480(template)` 的**道具定义**，要求
   `*(u32*)(def+2048) == 169`，再 `v6 = *v5` 取 `def+2056` 参数向量的**第一个也是唯一一个** u32 当皮肤 id。
   ⇒ 该 id 来自静态定义表，**没有任何按道具实例覆盖的入口**。
2. **本 PVF 武器族的 `[add skin storage]` 只有两个模板，参数都是 40000**：
   `stackable/10308001/10308358.stk` 与 `10308800.stk`，均为
   `[stackable type] [waste]` + `[action type] [add skin storage] 40000` +
   `[action target] [character]` + `[action expiration info] [unlimit]`；
   `40000` = `skin/weaponskin/baseform.skn`，也正是页 4 面板填充方唯一硬编码的注册记录（`0x9C40`，§18.2）。
   对照：伤害字体 `10160911 → 3`、即时表情 `10325568 → 10179`（同一条解析路径，参数就是各自皮肤 id）。
3. **NOTI1545 有两张表，只有第二张按职业重映射**（df55，`df55_noti1545_reader.c`）：读取器
   `sub_1444EFF40(mgr, 页)` 先清该页容器，再读 `u16 第一条表计数` + 条目，然后读
   `u16 第二条表计数` + 每条 8 字节。关键分支：
   * **第一条表**：`if (a2 == 4) { sub_146EA0BA0(&v32); HIDWORD(v32)=0 }` ⇒ 页 4 每条只读**裸 u32**
     （没有期限列，与 §18.2 一致），且**不做任何重映射**，id 原样进容器；
   * **第二条表**：每条 8 字节，且 `v15 = sub_1473A1120(a1 + 1328, *(u32*)(v13+7656 取职业), v32)`
     ⇒ **这一张才过按职业重映射**（§18.3 提到的那条）。
   `sub_1473A1120` 全库只有 4 个调用者：`sub_1444ED1B0`(2641)、`sub_1444EE820`(1565 回帧)、
   `sub_1444EECA0`(1546)、`sub_1444EFF40`(1545 第二条表)；`sub_1473A1580`（1565 组装侧那版）只有
   `sub_1444F0FE0/F1090/F1310/F1410` 四个组装方。
4. **结论（本轮判责）**：面板枚举的是**第一条表**，页 4 的 id 在其中**原样存取、不重映射**。
   所以「右键使用一件消耗品 ⇒ 武器页长出**被复制那把武器**那一行」在本构建里没有通路：
   道具能带的只有静态的 `40000`，而 `40000` 展开不出武器模板。服务端现有的一步式
   （CMD1592 直接把武器模板记进 `Bag.WeaponSkins` 并按页 4 下发）是本客户端**唯一能正确显示**的形状，
   实机截图里同一角色同时有「先驱者的玄机弓」与「旅行专家的玄机弓[决斗场]」两行即为证。
   ⇒ 不改一步式实现、不为凑流程发明字段；把「本构建不可表达」作为规格差记录，并留一条实机判决。
5. **一条能直接判掉的实机实验**（不改代码）：用 GM 工具给角色发 `10308358` 与 `10308800` 各一件，
   右键使用一次，看 ① 客户端有没有发 CMD507 动作 169；② 服务端从参数向量读到的皮肤 id 是不是 40000；
   ③ 武器页长出的是哪一行、显示什么名字。若它登记的仍是 40000，则第 4 条结论由实机闭合；
   若客户端另有取 id 的来源（例如道具实例字段），那才是本清单缺的那一环，届时按新证据重开实现。
6. **实机判决已回（2026-09-28 11:0x）**：用 GM 工具把 `10308358` 与 `10308800` 各发一件到角色背包，
   **两件都无法右键使用**，而且那一场 `events.jsonl` 里 **CMD507 一条都没有**（263 个事件，507 计数 0）
   ⇒ 客户端在本地就拒了，连上行都没发。先排除「期限格」这条老坑：背包材料行由
   `internal/inventory/bag.go:285/309` 的 `protocol.OrdinaryItem(slot, template, amount, ExpireTime)` 编码，
   其中 `ItemPeriodForWire` 会把「存储在 `configs/skin-storage-items.json` 里且期限为 0」的模板抬成
   `MaxItemPeriod`（`internal/game/protocol/inventory.go:34`），而这两个模板正是该表的条目，
   白名单在 `cmd/wireprobe/main.go:517` 的 `ConfigureSkinStoragePeriods(templates)` 装好 ⇒ 走到的偏移 56
   不是 0，第八轮那条「剩余期限已过」的形状在这里不成立。
7. **真正拦住它的是「你已经拥有这个皮肤」**：谓词 `sub_1444EC1B0` 查的容器是
   `a1 + 16 * (记录+8 家族类号) + 136`，而 NOTI1545 的读取器 `sub_1444EFF40` 写的正是同一个容器
   （`v4 = 16 * 页 + a1 + 136`，df55）⇒ 那是**归属页容器**，不是静态注册表。找不到条目时 `v12 = 0`
   直接 `return 0`。所以这两个道具的使用门是「角色页 4 里已经有皮肤 40000」，
   它**不可能**是「把某把武器的外观授予你」的入口 —— 与第 2、3 条互相独立地指向同一个结论：
   本构建的两步幻化没有通路，服务端现有的一步式（CMD1592 直接记武器模板）是唯一能正确显示的形状。
