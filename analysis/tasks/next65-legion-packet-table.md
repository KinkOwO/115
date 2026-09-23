# next65 — 军团 / 末世录包体字段表（P0 取证产物）

来源：IDA 9.4 + Hex-Rays 对 `client/DFO.exe.i64`（工作副本）的静态分析，
2026-09-23。所有结论都注明证据函数，**未闭环的项明确标「待核」**。

## 0. 三个客户端机制（实现时要对齐的口径）

| 机制 | 函数 | 形态 |
| --- | --- | --- |
| C2S 发包 | `sub_146D74000()`（取 writer）→ `sub_146D746E0(writer, opcode)`（写 opcode）→ `sub_146D75B10(writer, buf, len)`（写包体）→ `sub_146D75AF0(writer)`（发送） | 例：`sub_14069E3B0`（2355） |
| 期望回包登记 | 发包前在以 **opcode 为键的红黑树**（全局 `qword_14E652C80`，比较字段在节点 `+0x1c`）里查一次；发送后调用 `sub_140DD8720(opcode)` 登记 | `sub_1424FE550`（2043）前半段与尾部 |
| handler 注册 | CMD 注册表：`sub_14599D450(registry, id, handler, flags)`；NOTI 注册表：姊妹函数 `sub_14599D5D0(registry, id, handler, flags)`（首次调用时惰性创建 136 B 注册表） | `sub_1424FE600` / `sub_143895CF0` |

## 1. C2S 包体（客户端 → 服务端）

> ⚠️ **族别坑（本轮踩到并更正）**：`opcodes.tsv` 里 **cmd 与 noti 共享同一 16 位数字空间、
> 含义完全不同**（例：`0x8CD` 是 cmd `SAVE_TRAINING_ROOM_PRESET` 同时又是 noti
> `LEGION_ADDITIONAL_CLEAR_REWARD`）。按 id 立即数搜「发送点」会大量误判——
> 第一版 `ida_registry_survey.py` 就是这样把 `sub_1454964F0`(0x8CD)、
> `sub_140B878B0`(0x8CE)、`sub_144267A60`(0x2A9) 错认成 legion 的 C2S。
> **只有 cmd 族的立即数才是 C2S 发送点；NOTI 是纯 S2C，客户端不发。**

| opcode | 名称 | 包体长度 | 字段（偏移 = 包体起点起算） | 证据 |
| --- | --- | --- | --- | --- |
| 0x7FB (2043) | `LEGION_START` | **17** | `int32@13` | `sub_1424FE550`（`r8d=0x11`） |
| 0x7FC (2044) | `LEGION_FAIL` | **17** | `int32@13` | `sub_1424FE340` |
| 0x7FD (2045) | `LEGION_ENTER_DUNGEON` | **21** | `int32@13`、`int32@17` | `sub_1424FE290`（`r8d=0x15`） |
| 0x7FE (2046) | `LEGION_REWARD_END` | **22** | `int32@13`=a1、`int32@17`=a3、`char@21`=a2 | `sub_1424FE4A0`：`int v9 [rsp+2Dh]`=a1、`int v10 [rsp+31h]`=a3、`char v11 [rsp+35h]`=a2 ✅ 闭环 |
| 0x932 (2354) | `LEGION_OPERATION_SELECT` | **22** | `int32@13`=a2、`char@17`=a3、`int32@18`=a1 | `sub_1424FE3F0`：`int v9 [rsp+2Dh]`=a2、`char v10 [rsp+31h]`=a3、`int v11 [rsp+32h]`=a1 ✅ 闭环 |
| 0x933 (2355) | `APOCALYPSE_ROLE_SELECT` | **21** | `int32@13`（入参）、`int32@17` = **107 (0x6B)** 常量 | `sub_14069E3B0`（`mov [rsp+31h], 0x6B`） |
| 0x8F2 (2290) | `VENUS_OPERATION_SELECT` | **18** | `int32@13`=a1、`char@17`=a2 | `sub_141C3CFE0`：`int v8 [rsp+2Dh]`=a1、`char v9 [rsp+31h]`=a2 ✅ 闭环 |
| 0x8F5 (2293) | `VENUS_END_AT_PHASE4` | **13** | **空包体**（只发 13 字节头，无字段） | `sub_141C3CF60`：`sub_146D746E0(w, 0x8F5)` 后直接 `sub_146D75AF0` ✅ |

**发包模板**（所有 legion/VENUS C2S 共用）：

```c
v = sub_146D74000();              // 取 writer
sub_146D746E0(v, opcode);         // 写 opcode（首包时同时写入 13 字节头，见下）
v = sub_146D74000();
sub_146D75B10(v, &buf, len);      // 追加 len 字节
sub_146D75AF0();                  // 发送
sub_140DD8720(opcode);            // 在「期望回包树」(qword_14E652C80) 里登记
```

### 包体前 13 字节：已锚定（本轮新证据）

原判断「前缀是未初始化栈、来源未知」**已修正**。`sub_146D746E0` 在**首次发送**时显式构造并写入 13 字节：

```c
v25 = 1;  v26 = opcode;  v27 = 0;  v28 = 0;        // [rsp+60h]/[rsp+61h]/[rsp+63h]/[rsp+6Bh]
result = sub_146D75B10(a1, &v25, 13);              // 1 + u16 + u64 + u16 = 13 字节
```

⇒ 线上 13 字节前导 = **`01` + `opcode(u16 LE)` + `8 字节 0` + `2 字节 0`**，是**确定的信封结构**，
不是栈垃圾。同一 writer 只有在 `[writer+1099]==0`（首包）时才写；重复调用会走错误分支返回 0。

**服务端口径（沿用既有先例，本轮已交叉验证）**：仓库中 **5 个已实机验证**的协议模块一律
**跳过前 13 字节、从 `p[13:]` 起读字段**：

| 文件 | 读法 |
| --- | --- |
| `internal/game/protocol/awakening.go` | `Uint32(p[13:])`，注释明写「17 字节栈结构，只有 u32@13 被初始化，prefix 是 opaque，不可信作身份」 |
| `internal/game/protocol/advancement.go` | `p[13] & 0xF` |
| `internal/game/protocol/dungeon.go` | `Uint32(p[13:])` |
| `internal/game/protocol/unified_option.go` | `p[13], p[14]` |
| `internal/game/protocol/world.go` | `Uint32(p[13:])` |

⇒ legion 的 `int32@13` 与 `awakening` 的 `Uint32(p[13:])` **完全同构**（都是「13 字节信封 + 字段」）。
**唯一剩下的不确定**是：客户端 append 的长度是否已含信封（决定服务端是否需要再偏移）。
照 awakening 的先例（客户端 17 字节 = 13 信封 + 4 字段，服务端读 `p[13]`）反推，
legion 也应按 **`p[13]` 直接读**处理。首次实机时用 C2S 日志里的 `plain_hex` 长度一次性定论。

## 2. S2C handler（服务端 → 客户端）

**整族注册关系一次拿全**：`sub_1424FE600` 一个函数里显式列出两个注册表的所有条目
（证据：`registry-expand/register_sub_1424FE600_1424fe600.c`）：

| opcode | 名称 | 注册表 | handler | 读包体？ |
| --- | --- | --- | --- | --- |
| 0x7FB (2043) | `LEGION_START` | CMD | `sub_1424FD900` | ❌ 不读（自建 448/296/280 B 单例） |
| 0x7FC (2044) | `LEGION_FAIL` | CMD | `sub_1424FD290` | ❌ 不读（发 8 B） |
| 0x7FD (2045) | `LEGION_ENTER_DUNGEON` | CMD | `sub_1424FD160` | ❌ 不读（自建 13 B，`v12=108`） |
| 0x7FE (2046) | `LEGION_REWARD_END` | CMD | `sub_1424FD3A0` | ❌ 不读（自建 13 B，`v50[1]=108`） |
| 0x932 (2354) | `LEGION_OPERATION_SELECT` | CMD | `sub_1424FD320` | ✅ 读 **u16，必须 == 107** |
| 0x933 (2355) | `APOCALYPSE_ROLE_SELECT` | CMD | `sub_14069E320` | ❌ 不读（薄转发） |
| 0x8CC (2252) | `LEGION_BASIC_CLEAR_REWARD` | NOTI | `sub_1424FDC30` | ❌ 不读（自建 7772 B 结构） |
| 0x8CD (2253) | `LEGION_ADDITIONAL_CLEAR_REWARD` | NOTI | `sub_1424FDB60` | ❌ 不读（`memset 0x964` + 发 2405 B） |
| 0x8CE (2254) | `LEGION_ENTRY_CHARAC_INFO` | NOTI | `sub_1424FDD50` | ❌ 不读（发 256 B） |
| 0xA08 (2568) | `PREPARE_LEGION_ENTER_DUNGEON` | NOTI | `sub_1424FE000` | ❌ 不读（发 16 B） |
| 0xA61 (2657) | `LEGION_PHASE_CLEAR_TICK` | NOTI | `sub_1424FDF70` | ❌ 不读（发 144 B 零结构） |
| 0xB4F (2895) | `LEGION_INFO` | NOTI | `sub_1424FDDB0` | ✅ 读 **u16，必须 == 107**（发 204 B） |
| 0xB50 (2896) | `LEGION_OPERATION` | NOTI | `sub_1424FDF20` | ✅ 读 **u16，必须 == 107**（发 7 B） |
| 0x5C2 (1474) | `DUNGEON_TIMEOUT_TIME` | NOTI | `sub_143895B00` | ✅ 读**子类 5/6** → 本地化串 91161/91160，转发 2875 |

### 2.2 S2C 读取契约（本轮核心成果，推翻上一版「body 可留空」的结论）

**上一版把 `sub_146EA0BE0` 误读为「把结构推给 UI」，方向搞反了。** 它其实是
**从当前包体游标读 n 字节**。游标 API 家族（全部由本轮反编译确认）：

| API | 语义 | 越界行为 |
| --- | --- | --- |
| `sub_146EA2160(ptr)` | 设置游标指针 | — |
| `sub_146EA2170(len)` | 设置剩余长度 | — |
| `sub_146EA1540()` | 取当前游标 | — |
| `sub_146EA09F0(&u8)` | 读 1 字节 | 剩余 <1 → 写 0 |
| `sub_146EA1920(&u16)` | 读 2 字节 | 剩余 <2 → 写 0 |
| `sub_146EA0BA0(&u32)` | 读 4 字节 | 剩余 <4 → 写 0 |
| `sub_146EA0BE0(dst, n)` | 读 n 字节 | 剩余 <n → **`MEMORY[0]=0`（硬崩）** |
| `sub_146EA40F0(n)` | 剩余 >= n ? | — |
| `sub_146EA0340(n)` | 消耗 n 字节 | 剩余 <0 → 从备份缓冲回滚 |

⇒ **S2C 包体长度是硬契约**：短于客户端要读的长度会让客户端**崩溃**（`MEMORY[0]=0`），
不是"少读点就算了"。每个 handler 的确切读取序列：

| opcode | 名称 | 读取序列 | **服务端 body 最小长度** |
| --- | --- | --- | --- |
| 0x7FB (2043) | `LEGION_START` | `bytes(4)` | **≥ 4** |
| 0x7FC (2044) | `LEGION_FAIL` | `bytes(8)` | **≥ 8** |
| 0x7FD (2045) | `LEGION_ENTER_DUNGEON` | `bytes(13)` | **≥ 13** |
| 0x7FE (2046) | `LEGION_REWARD_END` | `bytes(13)` | **≥ 13** |
| 0x932 (2354) | `LEGION_OPERATION_SELECT` | `u16` == 107 | **≥ 2 且 == 107** |
| 0x933 (2355) | `APOCALYPSE_ROLE_SELECT` | 无 | 0（薄转发） |
| 0x8CC (2252) | `LEGION_BASIC_CLEAR_REWARD` | `bytes(7772)` | **≥ 7772** |
| 0x8CD (2253) | `LEGION_ADDITIONAL_CLEAR_REWARD` | `bytes(2405)` | **≥ 2405** |
| 0x8CE (2254) | `LEGION_ENTRY_CHARAC_INFO` | `bytes(256)` | **≥ 256** |
| 0xA08 (2568) | `PREPARE_LEGION_ENTER_DUNGEON` | `bytes(16)` | **≥ 16** |
| 0xA61 (2657) | `LEGION_PHASE_CLEAR_TICK` | `bytes(144)` | **≥ 144** |
| 0xB4F (2895) | `LEGION_INFO` | `u16`==107 + `bytes(204)` | **≥ 206 且 u16 == 107** |
| 0xB50 (2896) | `LEGION_OPERATION` | `u16`==107 + `bytes(7)` | **≥ 9 且 u16 == 107** |
| 0x5C2 (1474) | `DUNGEON_TIMEOUT_TIME` | 子类取自 handler 参数（非游标） | 无游标约束 |

**已确认的读取点在伪代码中的形态**（供复核）：

```
2043: sub_146EA0BE0(&v27, 4);        2044: sub_146EA0BE0(v5, 8);
2045: sub_146EA0BE0(v11, 13);        2046: sub_146EA0BE0(v50, 13);
2252: sub_146EA0BE0(v6, 7772);       2253: sub_146EA0BE0(v4, 2405);
2254: sub_146EA0BE0(v1, 256);        2568: sub_146EA0BE0(v19, 16);
2657: sub_146EA0BE0(v1, 144);
2895: sub_146EA1920(&v6); if (v6==107) { … sub_146EA0BE0(&v9, 204); }
2896: sub_146EA1920(&v1); if (v1==107) { return sub_146EA0BE0(&v2, 7); }
2354: sub_146EA1920(&v10); if ((u16)v10 == 107) { … }
```

### 2.3 收包链路（游标从哪来）

唯一绑定点是 **`sub_146D74A80`**（客户端收包循环，与发包器 `sub_146D74xxx` 同族）：

```c
sub_146EA2160((void *)(v59 + 16));            // 游标指针 = 包节点 + 16
sub_146EA2170(*(_DWORD *)(v59 + 3) - 16);     // 游标长度 = 节点 @+3 的 dword - 16
sub_1459A1BB0(qword_14E66C090, v64, v59, v60);// 分发给 CMD 注册表（qword_14E66C090）
```

⇒ **客户端帧结构**：`flag(1B) | id(u16) | 包长(u32) | …padding… | payload@+16`。
游标长度 = 包长 − 16，所以 **handler 读到的第 0 字节就是 payload 起点**。

⇒ 对服务端 S2C 的含义：**回包的 body 就是 payload，不含任何客户端信封**；
上表的「最小长度」直接就是服务端要发足的长度。

**14 个 handler 全部确认完毕**（上一版的 5 处「待核」已清零）。

### 2.4 handler 细节（证据）

- `sub_1424FD320`（2354）：`if (a2 != 0) { sub_146EA1920(&v10); if ((u16)v10 == 107) { 读 14 B → 打开作战选择 UI } }`
  —— 与 C2S 2355 包体里的常量 `107` **配对**，即「客户端带 107 请求 → 服务端回包也要带 107」。
- `sub_1424FDF20`（2896）：同样 `sub_146EA1920(&v1)` + `if (v1 == 107)` → 读 7 B。
- `sub_1424FDF70`（2657 阶段推进）：`memset(buf,0,144) → sub_146EA0BE0(buf,144) → sub_142ABF650(qword_14E683C40, buf)`
  —— **读 144 字节阶段状态块，再交给阶段状态子系统**。
- `sub_1424FDC30`（2252 基础奖励）：`memset 7772` → 读 7772 B → 切成 `40×40 B + 140×44 B` 交给 `sub_142AB28D0`。
- `sub_1424FD160`（2045）：读 13 B 到 `v11`（`*(u32*)v11=0; v12=108; v13=-1; v14=0` 是**写入本地缓冲后的初值/覆盖**），
  再 `sub_14668C520(..., 2875, 本地化串(725|101037339))` 推 HUD 文案；`v12=108 (0x6C)` 与 C2S 2355 的 `107 (0x6B)` 同族。
- `sub_1424FD900`（2043）：读 4 B，再惰性创建 448/296/280 B 单例并 `sub_1420A10F0(..., -1)`。

> 上一版把这里的 `sub_146EA0BE0(x, n)` 写成「自建 n 字节结构并推给 UI」，与 §2.2 的实测实现
> （`memmove(dst, cursor, n)`）相反，已按 fix-at-root 更正：**都是从包体读**。

## 3. S2C handler 的行为特征（重要：影响服务端要发什么）

请求-应答型 handler 开头是同一套动作，**先清 ack 树，再从游标读固定长度**：

```c
result = sub_1403A7590(&qword_14E652C80, &id);   // ① 从「期望回包树」里清掉这个 id
if (result) { … }                                 // ② 清掉之后才继续（清不掉就整个跳过）
<读固定长度>                                      // ③ 见 §2.2 的契约表
```

| handler | 行为 | 含义 |
| --- | --- | --- |
| `sub_1424FD900`(2043 LEGION_START) | 读 4 B；惰性建 448/296/280 B 单例并逐个 `sub_1420A10F0(..., -1)` | 启动攻坚界面/状态机，**载荷读完即弃** |
| `sub_1424FD320`(2354 OPERATION_SELECT) | 读 u16（须 107）；`v6 = 255; sub_146EA0BE0(&v5, 14)` 再读 14 B；`sub_140696290()` → `sub_14069A360(..., &v5)` | 打开作战选择界面 |
| `sub_1424FD160`(2045 ENTER_DUNGEON) | 读 13 B；`*(_DWORD*)v11 = 0; v12 = 108; v13 = -1; v14 = 0`；`sub_14668C520(..., 2875, 本地化串(725/101037339))` | 准备房间 UI + HUD 文案 |
| `sub_143895B00`(1474 TIMEOUT) | `(a1, a2, a3)`：`a2==0` 且 `a3==5/6` → 串 91161/91160 → 转发 `2875` | **取字段自 handler 参数，不走游标** |

> 注：2354 的 `sub_146EA1920` + `sub_146EA0BE0(&v5, 14)` 两次读共 16 B，
> 说明该 handler 实际需要 **≥16 字节**（§2.2 表里按已确认的最小值 2 记，实施时按 16 发更稳）。

**推论（已被 §2.2 修正）**：对 2043/2044/2045/2046/2354 这几个请求-应答型 CMD，
服务端回包的**载荷内容**无关紧要（handler 读完即弃、不解释语义），
但**载荷长度是硬约束**（读不够会 `MEMORY[0]=0` 崩溃）。所以正确口径是
「**发足长度、内容可零**」，而不是上一版写的「body 可以留空」。

另注：客户端反复出现 **13/14 字节的「UI 消息结构」**与常量
**107(0x6B)/108(0x6C)/255**；C2S 2355 包体里的 `@17 = 107` 与 2045/2046 handler 里的
`v12 = 108` 是同一族常量，可能是「消息/阶段类型码」，值得在 P3/P4 继续追。
**2354/2895/2896 三个 S2C 包的首个 u16 必须等于 107**，这是目前唯一的载荷取值约束。

## 4. P0 收口状态与下一步

### 已闭环（本轮清零）

| 项 | 结论 |
| --- | --- |
| 14 个 handler 是否读包体 | 全部**读**，读取序列与最小长度见 §2.2 |
| C2S 2046 / 2354 字段偏移 | 已闭环（`int32@13` + `int32@17` + `char@21` / `int32@13` + `char@17` + `int32@18`） |
| VENUS 2290 / 2293 C2S | 已闭环（18 B 带 2 字段 / 13 B 空包体） |
| 误判的 4 个「sender」 | 已剔除（cmd/noti 撞号，与 legion 无关） |
| 游标 API 家族 + 绑定点 | 已确认（§2.3，唯一绑定点 `sub_146D74A80`） |
| 13 字节前导 | 结构已锚定（`01`+`opcode u16`+`10×0`，由 `sub_146D746E0` 首次发送时写入）；服务端口径沿用 `p[13]`（5 个已实机验证先例） |

### 仅剩 1 项待实机（不阻塞 P1 开工）

**C2S 载荷里「13 字节信封 + 字段」还是「字段本身也在 13 之前」**——两种读法在
`p[13]`/`p[26]` 之间，需要一次实机 C2S 抓包（服务端已有 `plain_hex` 事件机制，
第一次进军团频道就会打印真实长度与字节）。**风险可控**：读错位置只会导致
「字段值不对」，不会崩客户端（C2S 方向客户端不校验）。

### P1 可以直接用的输入

1. **C2S**：2043 读 `u32@13`（军团入口参数）、2354 读 `int32@13`/`char@17`/`int32@18`、2355 读 `u32@13` + 常量 `107@17`；
2. **S2C**：回包 body 必须发足 §2.2 的长度，**2354 / 2895 / 2896 的首个 u16 = 107**，其余内容可零；
3. **时序**：客户端发包后会登记「期望回包树」(`sub_140DD8720`)，handler 收到时先查树
   (`sub_1403A7590`)——**清不掉就整个 handler 跳过**。⇒ 服务端**必须回同 id 的包**，
   否则客户端状态机会卡在等待态（这解释了「进不去/无响应」类现象）。

## 5. 复现路径

| 产物 | 来源脚本 |
| --- | --- |
| `va-decompile/*.c` | `ida_decompile_va.py`（改 TARGETS 后 `idat -A -S` 运行） |
| `registry-expand/register_sub_1424FE600_1424fe600.c` | `ida_registry_expand.py`（整族注册关系） |
| `cursor-binding/*.c` + `cursor-bind-sites/*.c` | `ida_cursor_binding.py` / `ida_bind_sites.py`（游标 API 与绑定点） |
| `dispatch-survey/string_decoder_146e8c490.c` | `ida_dispatch_survey.py`（枚举名解密器） |
| `decoded-names-legion.json` | `pe_decode_xorstr.py`（离线解码校验命名，15/15 命中） |
