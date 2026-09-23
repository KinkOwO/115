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
sub_1459A1BB0(qword_14E66C090, ...);   // 分发给 CMD 注册表
```

⇒ 客户端帧结构：`flag(1B) | id(u16) | 包长(u32) | …padding… | payload@+16`。
**handler 读到的第 0 字节 = payload 起点** ⇒ 服务端 S2C **body 就是 payload，不含任何信封**。

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

## 4. handler 注册表

| 族 | 注册函数 | 注册表全局 |
| --- | --- | --- |
| CMD | `sub_14599D450(registry, id, handler, flags)` | `qword_14E66C090` |
| NOTI | `sub_14599D5D0(registry, id, handler, flags)`（姊妹函数，首次调用惰性建 136 B 表） | — |

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
| 1 | 2×u64 | 20 | **名字引用** `(lo,hi)` → 串池字符区间 |
| 2 | u32 + 2×u64 | 24 | 带索引的名字引用 |
| 3 | u8 | 5 | 字节/布尔 |
| **4** | u64(f64) | **12** | **float64 —— 玩法数值** |
| 5 | u64 | 12 | 同宽另一类型 |
| 6 / 7 | u32 + u8 | 9 | (u32, byte) |
| 其它 | 0 | 4 | **填充 cell** |

段读器 `sub_1474D0C00(base,end,&cursor,table,rowIdx)` 顺序：`u64,u64`(名字引用) → `u32` →
`u64`(≠−1 时存 row+104) → `u64`(分支 A 的 cell 次数) → `u64`(分支 B 的行数)。
段结构 = **120 B**，cell 条目 = **40 B**。

### 6.2 串池、名字引用与记录流（`sub_1474CDD90` / `sub_1474D0A00`）

```c
v4 = *(table + 136);                       // 池（宽字符串，装载时由 sub_1474D0A00 加宽）
lo = min(a,b); hi = max(a,b);
if (hi <= v4[2]) v9 = (char*)v4 + 2*lo;    // 按字符取
assign_wstr(out, v9);                      // 客户端读到 NUL
```

文件里的池 = 尾部 **ASCII 拼接块（无分隔符）**，**基址是第一个可打印字节**（`apocalypse.ctp`
= `0x2071`，其前一字节为 NUL）；引用 `(lo, hi)` 是 **0 基、`hi` 独占**，
`hi − lo` = 字符串长度（实测精确吻合），**服务端按区间切即可，不必依赖客户端的 NUL 语义**。

**cell 区 = 记录序列**：`20 字节名字引用 + 值区`，下一条名字引用即记录边界。
值区是 u64 序列：标量列 = `0, 值`；表格列 = 索引头 + 12 字节 f64 cell（`u32 tag=4` + `f64`）。

**尾部元数据 = 列索引**：`u64 lo, u64 hi, u64 count, count × u64`（`apocalypse.ctp` = 7 条记录，
恰好 248 字节；`[operation data set]` 的 4 个值就是 4 个作战的行号）。

> 判定记录请用**名字引用锚定**扫描（对每个 4 字节位置的 u64 对做「区间 → 池标签」精确匹配），
> 不要用 `tag 0` 填充感知的 DFS tiling —— 后者在浮点 cell 稀疏的文件上会退化。

### 6.3 已取到真源：阶段时钟与门禁时刻

`analysis/tasks/next69-ctp-extract.py` 解出 **4 条 `[phase info]` 记录、取值完全一致**
（1647 cells：`tag0×1461 / tag3×12 / tag4×174`）：

`90,1, 300,2, 300,3, 300,4, 600,5, 600` ⇒ **阶段 1..6 = 90 / 300 / 300 / 300 / 600 / 600**。

4 条记录的行号 `10 / 24 / 38 / 52` 与尾部列索引 `[operation data set]` 的 4 个值一致
⇒ **这 4 组是 4 个「作战」，不是难度**（`normal / expert / master` 是池里另一维度的命名）；
⇒ **阶段时钟与作战、难度都无关**。

门禁时刻同源可取：`[gate schedule]` 实测 `300 / 240 / 180`，`[gateflow]` 给出阶段间流转。

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

**头部 64 字节**（两文件对照）：

| 偏移 | apocalypse.ctp | dungeonskillinfo.ctp |
| --- | --- | --- |
| `0x00` | 1（版本） | 1 |
| `0x04` | 65 | 14 |
| `0x0c` | 32 | 32 |
| `0x14` | 8052（cell 区止附近） | 1372 |
| `0x1c` | 8300 | 1436 |
| `0x2c` | 20 | 17 |
| `0x34` | 1 | 2 |
| `0x38` / `0x3c` | −1 / −1 | −1 / −1 |

⚠️ **仍未闭环**：

- **值区索引头**：表格列值区开头的若干 u64 语义未定（影响精确取值，不影响阶段时钟/门禁时刻）；
- **头部字段** `65 / 32 / 20 / 1` 的含义；
- 段头（`sub_1474D0C00` 的 `u32` / 首个 `u64(≠−1)`）用途；
- **DFS tiling 仅在浮点 cell 密集时可用**，判定记录请走名字引用锚定路径。

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
   （本仓库为此走了一轮弯路，见 `next69` §1/§9）。
8. **`.ctp` 的 cell 是 `u32 tag` + 变长载荷**，`tag 4` 才是 float64（§6.1）；
   对齐搜索失败是因为 tag→长度表未取全，不是假设不成立。
9. **IDA 批处理前先恢复工作副本**：一次运行会在 IDB 旁留下 `.id0/.id1/.id2/.nam/.til`，
   下一次打开可能报 `Database is empty` + `internal error 1228` 而失败。
   做法：`rm -rf /d/115us-backup/ida-work && cp <原始备份> ida-work/DFO.exe.i64` 再跑。

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
| `cmd/pvfinspect`（Go） | 枚举/导出 PVF 内容（`-find` 会写 `matches.json`，大文件扫完记得删） |
| `cmd/apocalypseimport`（Go） | 导出 `.ctp` 与 schema（当前含临时定宽解码，**仅供参考**） |
| `analysis/tasks/next69-ctp-extract.py` | **解 `.ctp`**：池基址 + 锚点约束 tiling + tag 直方图 + 池标签 + 阶段时钟组（带硬断言） |

**IDA 批处理调用方式**（本机已装在 `D:\tools\ida94`）：

```bash
cd /d/tools/ida94
env -u PYTHONPATH -u PYTHONHOME ./idat.exe -A -L<log> \
  -S"<脚本.py>" "D:\115us-backup\ida-work\DFO.exe.i64"
```

- **必须先清空 `PYTHONPATH`/`PYTHONHOME`**，否则报 `Failed to import encodings module`；
- 只打开**工作副本** IDB（权威 `client/DFO.exe.i64` 不开、不升级）；
- vtable 方法可直接用 PE 读：`off_<VA>` + 8×n。
