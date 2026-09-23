# next64 — 军团频道 / 末世录（Apocalypse）分期修复计划与评估

> 依据：他人在本地未提交尝试后留下的两份思路文档（`APOCALYPSE-TOWN-OPERATION-20260921.md`、
> `APOCALYPSE-HUD-EXIT-20260921.md`）＋ 本仓库真源资产与现有实现。
> 本报告先落盘评估，再按 §6 分期启动实现。**尚未提交任何分支**（按用户 2026-09-23 指示）。

## 1. 结论

1. **缺口可以补齐**，但性质是**新子系统从零实现**：本仓库对军团/末世录/噩梦循环的覆盖为 **0**
   （除可复用的 CMD37 / CMD72 / CMD2062 骨架）。
2. 思路文档的**包号与关键决策已被我们自己的真源证实**（§3），可作为路线图；
   但它自述「候选修复、待实机验收」，且引用的 `scripts/verify_apocalypse_town.py`、
   `docs/evidence/apocalypse-hud-exit-20260921` 在本仓库不存在，**不能作为实现依据**。
3. 首个 P0 交付物（`.ctp` 导入工具）已落地并跑通：schema 22 个键（含 `[allow coin]`）、
   阶段时钟 `90/300/300/300/600/600` 均已从源数据读出（§4）。
4. **P0 已收口（2026-09-23 下午）**：IDA 9.4 装好并跑完 9 轮取证——14 个 handler 的
   S2C 读取契约、C2S 包体字段、收包链路与游标 API 全部钉死（详见 `next65`）。
   仅剩 2 项不在 P1 阻塞路径上：C2S 13 字节信封归属（待实机一眼）、`.ctp` 变长记录（阻塞 P4/P6）。
5. **实现路线已定**：单人队策略（D2）+ 最小闭环入口（D1）+ 会话态不入库（D3）。
   **所有为此做出的妥协与降级集中在 §6.2 登记**——该节是本计划书的必读部分，
   也是「降级 / 占位 / 未验证」三类项的唯一台账，后续不得把降级当特性。

## 2. 工具链盘点（实测）

| 类别 | 资产 | 状态 |
| --- | --- | --- |
| 静态真源 | `client/DFO.exe.i64`（484 MB，2026-09-21）、`client/DFO.exe`（259 MB） | ✅ 在位，**需 IDA 打开** |
| 反汇编/仿真 | `tools/python`：`pefile 2024.8.26`、`capstone 5.0.9`、`unicorn 2.1.4` | ✅ 可用 |
| 动态注入 | `tools/python`：`frida 17.18.0` + `frida-tools 14.10.4`；`analysis/tools/*.py` 探针（arrow / dungeon_door / odyssey_slot / slot_state）；`server/work/dfo_probe_tools/probe.exe` | ⚠️ 可用但**有风险**：客户端带 Themida（`.themida` 12 MB）＋ BlackCipher(nProtect) 处于激活状态；`analysis/AGENTS.md` 要求不随意 hook。仅限用户手动、单个假设 |
| GUI 反编译 | `D:\tools\ghidra_12.1.3_PUBLIC` + `D:\tools\jdk-21.0.12.1` | ✅ 可用（Ghidra **打不开 .i64**，需对 `DFO.exe` 重新导入分析） |
| 协议/资源 | `analysis/dumps/opcodes.tsv`（5330 条）、`xorstr_map.tsv`（114k）、`dstr_map.tsv`（34k）；`cmd/pvfinspect`（565 万 PVF 条目） | ✅ 可用 |
| 服务端 | `server/work/dfo-lan`（Go 1.26 + PostgreSQL 16.4 + Redis）；`events.jsonl` 会话日志；`dfo_probe_tools/channel_probe.py` | ✅ 可用 |
| 项目 Python | `tools/python`（3.11 便携） | ✅ 可用 |

**没有 IDA 的影响**：`client/DFO.exe.i64` 现在是一块打不开的资产。可选路径：
(a) 用户提供 IDA 安装路径（首选，能直接复用 484 MB 现成 IDB 与既有命名）；
(b) 用 Ghidra 对 `DFO.exe` 重新导入分析（Thenida 虚拟化区段会拖慢，且要重建既有命名）；
(c) 先只用「已导出的 dump + PVF + 实机日志」推进离线部分（本报告 §4 已按此走）。

## 3. 已证实的真源事实（本轮）

### 3.1 包号（`analysis/dumps/opcodes.tsv` 逐条命中）

| 思路写法 | 真源名（family id） | hex | 备注 |
| --- | --- | --- | --- |
| CMD2043 | `LEGION_START` | 0x07FB | 开始攻坚 |
| CMD2044 / 2046 | `LEGION_FAIL` / `LEGION_REWARD_END` | 0x07FC / 0x07FE | 思路未提，同族 |
| CMD2045 | `LEGION_ENTER_DUNGEON` | 0x07FD | 进入准备房间 |
| CMD2354 | `LEGION_OPERATION_SELECT` | 0x0932 | 作战选择 |
| CMD2355 | `APOCALYPSE_ROLE_SELECT` | 0x0933 | 职责选择 |
| CMD2424 | `LEGION_ACHIEVEMENT_ACTION` | 0x0978 | 军团成就 |
| NOTI2895 / 2896 | `LEGION_INFO` / `LEGION_OPERATION` | 0x0B4F / 0x0B50 | 军团信息/作战下行 |
| NOTI1474 | `DUNGEON_TIMEOUT_TIME` | 0x05C2 | **阶段时限真身** |
| NOTI2657 | `LEGION_PHASE_CLEAR_TICK` | 0x0A61 | **阶段推进真身**（思路没点名） |
| NOTI2568 / 2254 | `PREPARE_LEGION_ENTER_DUNGEON` / `LEGION_ENTRY_CHARAC_INFO` | — | 准备房间 / 入场角色信息 |
| NOTI2252 / 2253 | `LEGION_BASIC_CLEAR_REWARD` / `LEGION_ADDITIONAL_CLEAR_REWARD` | 0x08CC / 0x08CD | 通关奖励 |
| CMD2290 / 2291 / 2292 / 2293 / 2296 | `VENUS_OPERATION_SELECT` / `GET_VENUS_RELIC` / `USE_VENUS_SILK_OF_PRAISE` / `VENUS_END_AT_PHASE4` / `VENUS_EVENT_DUNGEON_ACTIVATION_RELIC` | 0x08F2–0x08F8 | 维纳斯线 |
| NOTI2655 / 2656 / 2664 | `VENUS_LEGION_INFO` / `VENUS_LEGION_OPERATION` / `VENUS_RANKING_EVENT` | — | 维纳斯线 |
| CMD2094–2097、2313–2315、NOTI2401… | `NIGHTMARE_CLOISTER_*` / `U_NIGHTMARE_*` | — | 噩梦循环（截图第十章） |

### 3.2 资源侧

- `Script.inner.pvf`（565 万条）中 `apocalypse` 路径命中 **63,067 条**：
  `contents/2026/apocalypse/**` 34,602、`contents/2026/apocalypse_scenario/**` 23,707、
  `contents/2025/venus/**` 54+、`contents/2026/apocalypse/etc/apocalypse.ctp`（**9,280 B**）、
  `dungeonskillinfo.ctp`。
- 副本 id `100004995 / 100004994 / 100004990` 已在 `configs/dungeons.full.json`。

### 3.3 `apocalypse.ctp` 内部结构（本轮新证）

- 容器：`u32 version=1`、`u32 fields=65`、头部 64 字节、**字符串段声明在 0x206c**
  （schema 字符串紧随其后）；正文为「类型化记录」。
- **schema 22 个键**（实测次序，完整）：`[party waiting area]`、`[role per member limit]`、
  `[member]`、`[keldon xavi final damage rate]`、
  `[collaborate attack groggy duration increase per keldon xavi stack]`、
  `[guardian hp increase per keldon xavi stack]`、
  `[skirmisher monster hp reduction per keldon xavi stack]`、`[operation data set]`、
  **`[allow coin]`**、`[card Symbol Index]`、`[gate schedule]`、`[gateflow]`、`[index]`、
  `[member limit]`、`[phase info]`、`[recommend fame]`、`[reward data]`、`[string data]`、
  `[ting reward data]`、`[type fixed value]`、`[type]`、`[gate close warning]`。
  ⇒ **思路里「复活币由所选作战 `[allow coin]` 决定」这条决策在源里有据**。
- **阶段时限 90/300/300/300/600/600 证实**：数值为 **float64**，在文件中出现 4 组
  （偏移 2408 / 4380 / 6444 / 7928 起），三档难度 `normal` / `expert` / `master` 亦在字符串段内。
  ⚠️ 更正：上一轮我用 float32 扫描得出「时限不成立」，**该结论作废**——值以 double 存储。
- **口径修订（重要）**：正文记录**不是**等宽的 12 字节数组。证据：同一条阶段时钟序列
  在文件不同区域的格点相位不一致（2408 与 4380 落在不同 12 字节格点上），
  说明**记录长度随类型变化**。因此 `BinaryTable` 的正文解码目前是**临时性定宽解码**：
  足以读 schema、定位数值串，**不足以据此推导玩法数值**。

### 3.4 容器普查（离线，`analysis/tasks/next64-ctp-census.py`）

| 结论 | 实测 |
| --- | --- |
| 两个 `.ctp` **共享同一容器形状** | `apocalypse.ctp`：version 1 / fields 65 / data_end 8052 / strings_at 8300，**9280 B**；`dungeonskillinfo.ctp`：version 1 / fields 14 / data_end 1372 / strings_at 1436，**1636 B** |
| 阶段时钟共 **4 组**，全部等于 `90/300/300/300/600/600` | 起始偏移 `0x0969` / `0x111D` / `0x192D` / `0x1EF9`（与三档难度 normal/expert/master 对应，第四组待定） |
| 正文**非等宽** | 12 字节步长在任何起点都无法达到高一致率（最高仅 step=16 的 0.96，属巧合）；两组时钟间距 0x7B4 与 0x810 亦不成等宽关系 |
| 副本侧已索引 | `configs/dungeons.full.json` 里已有 **9 个** apocalypse 副本：`processing_plant 100004918`、`power_control_room 100004994`、`mist_reservoir 100004995`、`magnetic_facility 100005057`、`record_repository 100005111`、`navigation_room 100005112`、`apocalypseantienbi 100005220`（scenario）、`training_room 100005243`、`reward_dummy 100005272` ⇒ **准备房间/阶段房间的地图脚本服务端已具备** |

### 3.5 IDA 侧工具（已就绪并跑通）

`analysis/dumps/` 下三个脚本：
- `ida_legion_survey.py`（第一遍）——读 `table_slot_va` 指针、列枚举串交叉引用、导两个客户端地址；
- `ida_handler_survey.py`（第二遍）——反编译注册函数、抓每个 opcode 的注册点与槽位写者；
- `ida_decompile_va.py`（通用）——给任意地址出 Hex-Rays 伪代码。

⚠️ 第一遍脚本里的「读槽位指针 = 读 handler」假设**已被证伪**（见 §3.8）：`table_slot_va` 是解密后字符串的全局槽。

产物：`legion-survey/`、`va-decompile/`、`va-listings/`。

**IDA 版本决定**：IDA Pro 9.4 已按用户提供的安装包静默安装到 `D:\tools\ida94`（详见下方状态）。
**开工前必须先备份**——已执行（见 §3.6）。

### 3.6 IDA 环境状态（2026-09-23 实测）

| 项 | 状态 |
| --- | --- |
| 安装 | ✅ IDA Professional **9.4** → `D:\tools\ida94`（BitRock/InstallBuilder 安装包，静默命令 `--optionfile` + `mode=unattended` + `prefix=`，需提权） |
| 可执行 | ✅ `ida.exe`（GUI）、`idat.exe`（无界面）、`idapyswitch.exe`、`idalib`、内置 IDAPython 模块 |
| IDAPython 绑定 | ✅ `idapyswitch -s D:\115us\tools\python\python3.dll` → 绑定项目 Python **3.11.9**（安装包未带运行时，`--install_python` 的内嵌包缺失） |
| 运行环境要求 | ⚠️ 必须先清空 `PYTHONHOME`/`PYTHONPATH`，否则报 `Failed to import encodings module` |
| **授权** | ✅ 已解决：用户放置 `idapro.hexlic` 到 `D:\tools\ida94`，并在 GUI 中接受 EULA（`License not yet accepted, cannot run in batch mode` 因此在批处理下必须由持证人点一次同意）。`kg_patch/` 的 keygen 与替换版 `ida.dll`/`ida32.dll` **未使用** |
| IDB 备份 | ✅ `D:\115us-backup\DFO.exe.i64.orig-20260923`（484,741,229 B，与原件同尺寸）；IDA 工作副本 `D:\115us-backup\ida-work\DFO.exe.i64` —— **权威 IDB 不被打开、不被升级** |
| 一键取证 | ✅ `analysis/dumps/run-legion-survey.cmd`（清空 PYTHON 变量后跑 `idat -A -S ida_legion_survey.py`，输出到 `analysis/dumps/legion-survey/`） |

**授权就绪后立即执行**：`analysis\dumps\run-legion-survey.cmd` → 得到 17 个包的 handler 伪代码、
一层 callee 与两个客户端地址的函数体，即 §5 中「依赖 IDA」的全部条目。

### 3.8 IDA 侧取证第一轮结果（2026-09-23，含一条假设证伪）

IDA 9.4 已可用（EULA 接受后 `idat -A` 正常，Hex-Rays 可用）。三个脚本跑完，**更正 §3.1 的一处误读**：

| 结论 | 证据 |
| --- | --- |
| ✅ 17 个 legion/apocalypse 包的枚举名**各只有一处交叉引用**，全部位于同一函数 `sub_140069BB0`（0x140069BB0，一个巨型静态初始化器） | `legion-survey/survey.json`：每个 opcode `string_xrefs` 恰 1 条，`func=sub_140069BB0` |
| ✅ 初始化模式为 `qword_14EF3CF38 = sub_146E8C7D0(&unk_14B06BF60)`（LEGION_START）等 2429 处赋值 | `legion-survey/registration_140069bb0.c`（2436 行）第 2045-2051 行 |
| ❌ **`opcodes.tsv` 的 `table_slot_va` 不是 handler 表槽位**，而是「解密后字符串的全局变量地址」 | 同上赋值形态：`qword_<slot> = 解码(&<string_va>)` |
| ✅ `sub_146E8C7D0` 是**字符串解码器**，不是包处理注册器 | `va-decompile/packet_registry_lookup_146e8c7d0.c`：`if (*a1 == 11425 /*0x2CA1*/) return a1+2; else return sub_146E8C490(a1, 2);` |
| ✅ 这些枚举串是**加密字符串**（`0x14B06BF60` 原始字节 `00 54 48 2a b1 4c …`，首 u16 ≠ 0x2CA1 ⇒ 走解密分支） | PE 原始字节 + 解码器分支 |
| ✅ 两个客户端地址的伪代码已拿到 | `legion-survey/extra_1406afcc0.txt`（456 B）、`extra_1406aae00.txt`（2594 B） |

**因此 P0 的下一问变成**：「客户端用什么把 opcode id 映射到 handler」。三条待验证路径：
1. 反编译 `sub_146E8C490` 弄清字符串加密 ⇒ 解开 17 个名字（可反向校验 `opcodes.tsv` 的命名是否可信）；
2. 在 IDB 里查 `qword_14EF3CF38…` 这些**解密后全局的读者**——若只有初始化器写、无人读，说明名字只用于日志；若有读者，那就是登记/分发表所在；
3. 直接找按 opcode id（0x7FB / 0x932 / 0x933 / 0xB4F …）比较或索引的分发点，从 id 反查 handler。

工具（均新增，`analysis/dumps/`）：`ida_legion_survey.py`（第一遍：槽位/枚举串/客户端地址）、
`ida_handler_survey.py`（第二遍：注册点与写者）、`ida_decompile_va.py`（通用：给地址出伪代码）。
产物目录：`legion-survey/`、`va-decompile/`。

### 3.10 handler / sender 映射已建立（P0 核心成果，2026-09-23）

**三个可复用的客户端机制**（全部来自 IDA 伪代码，脚本见 §3.11）：

| 机制 | 函数 | 形态 |
| --- | --- | --- |
| 字符串解密（枚举名） | `sub_146E8C7D0` → `sub_146E8C490` | FNV-1a 缓存 + 滚动密钥 XOR（`k = plain + 65599*k`，尾部字节 263）；明文 **UTF-16LE**；头部 `byte1&0xFE` 为种子、`u16@+2` 编码长度 |
| **handler 注册** | `sub_14599D450(registry, id, handler, 0)` | 例：`sub_14599D450(qword_14E66C090, 0x7FB, sub_1424FD900, 0)` |
| **C2S 发包** | `sub_146D746E0(writer, id)` + `sub_146D75B10(writer, buf, len)` + `sub_146D75AF0(writer)` | 例：2355 的包体 21 B，`int@+13`=入参，`int@+17`=107 |

**枚举名可信度已验证**：用上述算法离线解出 17 个包的字符串，**15/15 与 `opcodes.tsv` 完全一致**
（`analysis/tools/pe_decode_xorstr.py`，产物 `decoded-names-legion.json`）。

**映射表（地址簇交叉验证：同族的 handler 与 sender 落在相邻区间）**

| id | 包名（cmd） | 客户端 handler（S2C） | 客户端 sender（C2S） |
| --- | --- | --- | --- |
| 0x07FB 2043 | `LEGION_START` | `sub_1424FD900` | `sub_1424FE550`（另有 `sub_142808870`） |
| 0x07FC 2044 | `LEGION_FAIL` | `sub_1424FD290` | `sub_1424FE340`、`sub_14237E310` |
| 0x07FD 2045 | `LEGION_ENTER_DUNGEON` | `sub_1424FD160` | `sub_1424FE290` |
| 0x07FE 2046 | `LEGION_REWARD_END` | `sub_1424FD3A0` | `sub_1424FE4A0` |
| 0x0932 2354 | `LEGION_OPERATION_SELECT` | `sub_1424FD320` | `sub_1424FE3F0` |
| 0x0933 2355 | `APOCALYPSE_ROLE_SELECT` | `sub_14069E320` | `sub_14069E3B0`（21 B 包体已解） |
| 0x05C2 1474 | `DUNGEON_TIMEOUT_TIME` | `sub_143895B00`（子类 5/6 → 本地化串 91161/91160 → 转发 NOTI 2875） | `sub_143895FA0` |
| 0x0A08 2568 | `PREPARE_LEGION_ENTER_DUNGEON` | 待找（NOTI 注册表） | `sub_144267A60`、`sub_1449597C0` |
| 0x08CD 2253 | `LEGION_ADDITIONAL_CLEAR_REWARD` | 待找 | `sub_1454964F0`、`sub_145497B40` |
| 0x08CE 2254 | `LEGION_ENTRY_CHARAC_INFO` | 待找 | `sub_140B878B0` |
| 0x08F2 2290 | `VENUS_OPERATION_SELECT` | `sub_141C3CC10` | `sub_141C3CFE0` |
| 0x08F5 2293 | `VENUS_END_AT_PHASE4` | `sub_141C3CB80` | `sub_141C3CF60` |
| 0x0B4F/0x0B50/0x0A61 | `LEGION_INFO` / `LEGION_OPERATION` / `LEGION_PHASE_CLEAR_TICK` | **待找**（NOTI 注册表） | 无（S2C） |

**结论**：P0 的「包体字段表」已具备入手点——5 个 legion CMD handler（`0x1424FD1xx-3xx`）与
相应 sender 的伪代码都已导出（`va-decompile/`、`registry-expand/`），可直接读字段顺序与宽度。

### 3.11 本轮脚本清单（`analysis/dumps/`）

| 脚本 | 作用 |
| --- | --- |
| `ida_legion_survey.py` | 第一遍：槽位/枚举串交叉引用、客户端地址 |
| `ida_handler_survey.py` | 第二遍：注册点与写者（发现 2429 处字符串初始化） |
| `ida_decompile_va.py` | 通用：给地址出 Hex-Rays 伪代码（改 `TARGETS`） |
| `ida_dispatch_survey.py` | 第三遍：解密器 + 名字全局读者 + opcode id 立即数分布 |
| `ida_registry_survey.py` | 第四遍：`sub_14599D450` 注册点 / `sub_146D746E0` 发包点 |
| `ida_registry_expand.py` | 第五遍：批量导出相关函数伪代码 + 建表 |
| `../tools/pe_decode_xorstr.py` | 离线解密枚举串（校核命名） |
| `../tools/pe_disasm_va.py` / `pe_rip_scan.py` | 无 IDA 时的 capstone 反汇编 / RIP 引用扫描 |

### 3.12 无 IDA 期间的静态替代路径（保留备查）

在授权未就绪时，用 PE + capstone 做了两件事，**结论一正一反**：

| 尝试 | 工具 | 结论 |
| --- | --- | --- |
| 从磁盘 PE 读 handler 表槽位取值 | 直接读 `client/DFO.exe` | ❌ 不可行：槽位 VA 落在 `.data` 的**未初始化尾部**（VirtualSize `0x1760378` > SizeOfRawData `0x9d2000`），指针值只在运行时写入，文件里没有 |
| 扫 `.text` 找 RIP 相对引用以定位注册点 | `analysis/tools/pe_rip_scan.py` | ⚠️ 35 条候选、**0 条通过校验** ⇒ 注册代码不是逐槽 RIP 引用（应为「取表基址一次 + 索引存储」），纯静态无法定位 handler。**要读 handler 必须 IDA 或运行时内存** |
| 反汇编 `.text` 内的已知地址 | `analysis/tools/pe_disasm_va.py` | ✅ 可行，且**证实了思路的一条关键决策**（见下） |

**客户端世界态门禁已证实（`0x1406afcc0`，capstone 反汇编，无需 IDA）**：

```
0x1406afcc9  mov  rcx, [rip + 0xdfbc3c0]
0x1406afcd0  call 0x1459a90f0        ; 取当前 world state
0x1406afcd5  cmp  eax, 1             ; ← 只接受 state == 1
0x1406afcd8  jne  0x1406afd1b        ;    否则直接返回（不做后续处理）
0x1406afcea  cmp  dword [rbx + 0x5e8], 1
0x1406afcf1  je   0x1406afd1b        ; ← 已打过标记则跳过（一次性）
0x1406afcf6  call 0x1406af0d0        ; “界面/对象就绪”检查（检查 +0x5f0/+0x600/+0x610/+0x680/+0x688/+0x690 与 vtable+0x298）
0x1406afd04  mov  dword [rbx + 0x5e8], 1   ; 落标记
0x1406afd16  jmp  0x1406afa80        ; 进入真正的处理
```

⇒ **思路「原客户端要求 world state=1（城镇）、副本态提前返回」得到独立证实**，
并且给出了「等待界面就绪」的具体判据函数（`0x1406af0d0`）与一次性标记位置（`+0x5e8`）。
产物：`analysis/dumps/va-listings/1406afcc0.txt`、`1406af0d0.txt`、`1459a90f0.txt`、`1406aae00.txt`。

**仍未闭环**：13 个包的 handler 体（reader 字段/时序）——只能靠 IDA（有授权）或运行时内存。
`0x1406aae00` 反汇编出来是「0x158 大小单例的惰性初始化」样板，没直接看到复活币文案分支，
需要顺着 `[allow coin]` 的消费点继续追（同样依赖更深的分析能力）。

## 4. 本轮新增代码（P0.1 交付物，未提交）

| 文件 | 作用 |
| --- | --- |
| `internal/catalog/pvf/table.go` | `Archive.BinaryTable(path)`：解码 `data_type=4` 二进制表；断言版本、字符串段、schema 非空；暴露 `TableCell` / `Schema` / `SchemaIndex` / `Kind`；显式注释正文解码为临时定宽、不可用于推导玩法数值 |
| `cmd/apocalypseimport/main.go` | 只读导出工具：`-source <pvf> -output <dir>` → `apocalypse-table.json`；硬断言 sha256、必需 schema 键（含 `[allow coin]`）、阶段时钟序列 |

实测输出（`go run ./cmd/apocalypseimport -source ../client-build/Script.inner.pvf`）：

```
apocalypse.ctp sha256=d560876f683d921ba8bab42ac268c51e2a704ba1dabaf1236ad7eeca78a8dba7 bytes=9280 version=1 fields=65
cells=666 (provisional fixed-stride decode of the body)
schema tags=22
phase clock groups=[[90 300 300 300 600 600] [90 300 300 300 600 600]]
```

（`go vet ./internal/... ./cmd/...` 全绿。）

## 5. 取证缺口（P0 状态：**已收口**，2026-09-23 下午）

| 缺口 | 状态 | 结论 / 证据 |
| --- | --- | --- |
| `CMD2043/2045/2354/2355/2046/2044` 的 C2S 字段 | ✅ **闭环** | 全部为「13 字节信封 + 字段」，字段偏移逐条钉死（见 `next65` §1） |
| 13 个 handler 的 S2C 读取 | ✅ **闭环（重大修正）** | **全部读包体**，且**长度是硬契约**（读不够客户端 `MEMORY[0]=0` 崩溃）；逐包最小长度见 `next65` §2.2 |
| 收包链路 / 游标来源 | ✅ **闭环** | 唯一绑定点 `sub_146D74A80`：游标 = (节点+16, @节点+3 长度−16)（`next65` §2.3） |
| 客户端世界态门禁 `0x1406afcc0` | ✅ **闭环** | `cmp eax,1 / jne exit` + 一次性标记 `+0x5e8` —— 思路那条决策**成立** |
| 复活币文本 `0x1406aae00` | ✅ 已导出 | `va-listings/` 有反汇编 + IDA 伪代码 |
| 作战选择值映射 `0/1/2/4 ↔ 1/2/3/5` | ⏳ 未取证 | 不在 P1/P2 阻塞路径上（P2 再取） |
| **C2S 是否把 13 字节信封算在 body 内** | ⏳ **仅剩这一项，待实机** | 两种读法差 13 字节；有 5 个已实机验证模块按 `p[13]` 读。第一次进频道看 `plain_hex` 即定论；读错只影响取值、不会崩客户端 |
| `.ctp` 正文变长记录编码 | ⏳ 未闭环 | 阻塞 P4/P6 的数值取值；**不阻塞 P1/P2/P3** |
| 队伍/军团身份构造 | 📐 设计决策 | 单机按「单人队伍」建模（本报告 §6.1） |

## 6. 分期计划

| 期 | 目标 | 主要产物 | 验收 |
| --- | --- | --- | --- |
| **P0** ✅ | 取证闭环 | `.ctp` 容器解码工具、包体字段表、客户端门禁、handler/sender 映射 | **已收口**：`next64` + `next65` 两份报告 |
| **P1** ← 现在 | 军团频道入口 + `LEGION_START(2043)` | `internal/legion/` 会话态、C2S 分发（2043）、S2C 回包（≥4 B） | 实机：能进入城镇作战选择界面 |
| **P2** | 作战选择 `LEGION_OPERATION_SELECT(2354)` + ACK | 映射表校验、`phase0` 授权 | 实机：选择被服务端认可 |
| **P3** | 准备房间 `LEGION_ENTER_DUNGEON(2045)` + `APOCALYPSE_ROLE_SELECT(2355)` + `PREPARE_LEGION_ENTER_DUNGEON(2568)` / `LEGION_ENTRY_CHARAC_INFO(2254)` | 准备房间、职责同步、`phase1` 授权 | 实机：准备房间可见、职责可选 |
| **P4** | 阶段推进与计时（`LEGION_PHASE_CLEAR_TICK(2657)` + `DUNGEON_TIMEOUT_TIME(1474)` + CMD37 挂钩） | 阶段时钟**改为从 `.ctp` 读真源**（阶段 1..6 = 90/300/300/300/600/600，见 `next69` §5.2），领主死亡/阶段残留修复 | 实机：首战房间通过 |
| **P5** | 失败/结算/奖励（2044 / 2046 / 2252 / 2253 / 2895 换图） | 死亡动画、结算与军团奖励 | 实机：完整一轮 |
| **P6** | 撤退与死亡回城（CMD72 末世录分支 + 作战级复活币限制） | 撤退续进、`[allow coin]` 限制 | 实机：撤退与复活各一次 |

### 6.1 P1 实施设计（决策已定，2026-09-23）

**已具备的实现输入**（全部来自 `next65`，非推测）：

| 方向 | 契约 |
| --- | --- |
| C2S 2043 | `LEGION_START`，17 B，`u32@13` = 入参 |
| S2C 2043 | 回同 id，**body ≥ 4 B**（内容可零） |
| 时序 | 客户端发包后登记「期望回包树」；handler 先查树，**清不掉就整个跳过** ⇒ 服务端必须回同 id 包，否则卡等待态 |

**三项决策（已拍板）**：

| 决策点 | **决定** | 生效范围 |
| --- | --- | --- |
| **D1 军团频道入口怎么暴露** | **先不动频道配置**，采用「收到 2043 即视为已进入军团上下文」的最小闭环；入口暴露条件留到实机观察后补 | P1 |
| **D2 队伍身份** | **单人队**（成员数 = 1，不引入新表；`LEGION_INFO`/`LEGION_OPERATION` 只填自己） | P1–P6 |
| **D3 会话态存放** | 进程内 `legion.Session`（选中作战 / 职责 / 阶段），**不入库**；若 P5 奖励需持久化再单开表 + 迁移脚本 | P1–P6 |

**P1 交付物清单**：`internal/legion/{session.go,session_test.go}`、`cmd/wireprobe/legion_flow.go`（2043 分发 + 回包）、任务文档 `next66-*.md`、C2S `plain_hex` 日志（用于定论 §5 最后一项）。

**回滚**：只新增文件 + 在 C2S 分发处挂一个 `case`，删除即可恢复；无数据库改动。

### 6.2 单人队策略：妥协与降级登记表（**必读，随 git 提交**）

> 本节是 D1/D2/D3 三项决策的**代价台账**。凡是「与原版语义不一致」或「暂不实现」
> 的项都在此列明，避免后续把降级当特性、或把未验证当已完成。
> 状态口径：**已实现** / **降级实现**（行为与原版不同）/ **占位**（结构对、内容空）/ **未验证**。

| # | 项 | 原版语义 | 我们的做法 | 性质 | 影响期 | 补齐路径 |
| --- | --- | --- | --- | --- | --- | --- |
| T1 | 队伍成员 | 军团 = 多成员队伍，队伍级信息下发 | 伪造「单人队」，成员数恒为 1，只含本人 | **降级实现** | P1–P6 | 若将来要做多人，需引入队伍表 + 成员同步；当前不预留代码结构以外的抽象 |
| T2 | 进入人数门槛 | 原版可能有人数下限（未取证） | 不做人数校验，来者即放行 | **降级实现** | P1 | 实机时观察客户端是否本地校验；若本地校验，则人数由客户端决定、服务端无从干预 |
| T3 | 职责分配 | `APOCALYPSE_ROLE_SELECT` 职责 = 散兵/守卫（`dungeonskillinfo.ctp` 的 `[skirmisher info]`/`[gaurdian]`），多人互斥互补 | 只记录本人职责，不做队友互斥/互补校验 | **降级实现** | P3 | 语义锚点已有（.ctp 12 键），若要还原互斥规则需解码该表 |
| T4 | 准备房间成员列表 | `LEGION_ENTRY_CHARAC_INFO`(2254) 下发多名入场角色（**客户端读 ≥256 B**） | 只填本人槽位，其余槽位留零 | **占位** | P3 | **最可能需要实机调试的点**：若客户端按非零槽位数渲染成员列表，空槽可能显示异常 |
| T5 | 通关奖励口径 | `LEGION_BASIC_CLEAR_REWARD`(2252, ≥7772 B) / `LEGION_ADDITIONAL_CLEAR_REWARD`(2253, ≥2405 B) 按队伍结算 | 按单人结算，长度发足、内容为零 | **占位** | P5 | 奖励数值需按玩家实机反馈校准；`.ctp` 的 `[reward data]` 是取值真源，但需先解变长记录 |
| T6 | 阶段协作难度 | 末世录 7 个阶段房间按多人协作设计 | 不调整难度，按原版数值放行 | **未验证** | P4 | **最大的未验证风险**：单人可完成性未知。若某阶段硬性要求多人（如同时守多门），需在 P4 之后补「难度降级」决策 |
| T7 | 阶段状态块 | `LEGION_PHASE_CLEAR_TICK`(2657) 的 144 B 阶段状态块可能含多人进度 | 按单人进度填 | **未验证** | P4 | 需在实机确认客户端是否会因进度字段不符而卡阶段 |
| D1a | 入口暴露条件 | 军团入口在客户端的显示条件（频道 `Type`？NPC？条件判断？） | **不动频道配置**，只保证「收到 2043 就正确回包」 | **未验证** | P1 | 触发点已定位（`sub_142510A50`/`sub_142511D10`），实机看不到入口时按这两个函数反推条件 |
| D3a | 会话持久化 | 官服会话在服务端，断线可续 | 会话态仅在内存，**服务端重启即丢失**，玩家掉出攻坚 | **降级实现** | P1–P6 | 单机可接受；若要做「断线续进」，加表 + 平滑迁移（符合根 `AGENTS.md` §4 存档兼容） |
| X1 | C2S 13 字节信封归属 | —（客户端实现细节） | 按 `p[13]` 读字段（沿用 5 个已实机验证模块的口径） | **未验证** | P1 | 首次实机的 `plain_hex` 一眼定论；读错只影响取值、不崩客户端 |
| X2 | `.ctp` 记录编码 | 阶段时限、`[allow coin]`、`[reward data]`、职责参数等真源 | ✅ **全部取到真源**（见 `next69` §5 真值表） | **已闭环** | P3/P4/P5/P6 | **2026-09-23 第四轮（闭环）**：格式 100% 解出并用两个文件独立验证 —— 记录头 44 B（`名字16 + flags4 + parent8 + n_cells8 + n_refs8`）+ cells + refs；`parent` = 父记录序号；尾表 = 「列名 → 行号」；池字符 0 = 首个 `[`。断言：遍历条数 == 头部 `record_count`（65/14）、记录止 == 尾表起、尾表止 == 池起。真值：阶段时钟 `0,90,1,300,2,300,3,300,4,600,5,600`（四作战一致）；`[allow coin]` 仅作战① = `-1, 8`；`[gate schedule]` 分作战 900 vs 300/240/180 vs 300/255/240/180；`[reward data]` 物品 10421367/10421369/10421365；职责表散兵/守卫参数齐。仍未定：`flags` 精确语义、头部 `0x0c=32` 与两处 −4 偏差、`[allow coin]` 两字段含义 |
| X3 | 作战选择值映射 | 作战号的真源 = `apocalypse.ctp` 的 `[operation data set]` 块 `[index]`：**`1 / 2 / 3 / 5`**（没有 4） | 服务端**不再猜映射**：CMD2045 的作战号直接按源表校验（`catalog.Operation(id)`），未声明的 id 明确拒绝并记日志 | **部分闭环**（值域已确证；客户端发出时机仍待实机） | P2/P4 | **2026-09-23 第四轮更新**：早期「`0/1/2/4 ↔ 1/2/3/5`」的映射假设**作废** —— 那是从思路文档转述的、无源依据。现在源表声明 4 个块，客户端在确认时把作战号读自自身界面状态（`sub_1424FE290(107, *(a1+115))`）。拒绝时的日志会打印客户端实际发的 id，一次实机即可确认 |
| X4 | CMD2354 回包内容 | 客户端按 14 字节结构**首 dword** 分支：`1`=刷新作战界面（用 `+6` 的 dword）、`2`=确认进入（客户端随后自发 2045） | **回显客户端请求的 action**，`+6` 的 dword 留零 | **未验证** | P2 | 两端都用 `1/2` 表达同一组动作（`sub_14069B580` 发 1、`sub_14069B560` 发 2），回显是最小假设；实机若拒绝则改发全零结构（handler 提前 return，安全但无动作） |
| X5 | `NOTI2568 PREPARE_LEGION_ENTER_DUNGEON` | 16 字节结构：`@0` 结果码（`0`=OK）、`@4` 模式（`1`/`2` 触发 UI 切换）、`@12` 参数 | **本轮不主动下发** | **有意不做** | P3 | 结构已从客户端初值常量（`00000000 00000000 ffffffff 00000000`）确证，但投递方 `sub_146D0D2A0` 是 **toggle**（`a1+1416` 已置位即走"关闭"分支）——条件未闭环时重发会**把界面关掉**。实机若准备房间不出现再补 |
| X6 | `NOTI2254 LEGION_ENTRY_CHARAC_INFO` | 256 字节 = **7 × 24 字节成员槽位**（7×24=168）+ 84 字节 `0xFF` 尾部哨兵；消费端只做整体拷贝存储 | **本轮不主动下发** | **有意不做** | P3 | 槽位字段语义未闭环：客户端默认构造器 `sub_1424FD050` 显示每槽清 `u16@0 / u64@2 / dword@10 / dword@16 / byte@20`，但**没有写者**可对照，无法确定哪个字段是角色 id / 名字 / 职业。单人队要填"本人"槽位，填错后果未知 |
| X7 | `NOTI2657 LEGION_PHASE_CLEAR_TICK` 的 **144 B** | 服务端驱动「阶段推进」的载体 | **不发**（阶段推进暂不实现） | **阻塞 P4 驱动** | P4 | **2026-09-23 新登记**：handler `sub_1424FDF70` 只有三行 —— `memset(buf,0,144) → sub_146EA0BE0(buf,144) → sub_142ABF650(qword_14E683C40, buf)`，即**从包体读 144 B**（早前误读成"客户端自建 144 B"）。下游 `sub_142ABF650` 是「按 `sub_142AB29C0(obj)` 得到的键做 map 查找 → 对命中对象发起 vtable+1136 虚调用」，144 B 结构体由**被调类**解释。**未解出该结构前不得下发**。下一步探针：定位向 `obj+312` 的 map 插入点（对象池注册处），取到类 → 读 vtable slot 142；或反向搜客户端**填充** 144 B 结构的写者 |
| X8 | `NOTI1474 DUNGEON_TIMEOUT_TIME` | 早前以为是「阶段时限下发」 | **不发**（仅登记语义） | **语义已澄清** | P4 | **2026-09-23 新登记**：handler `sub_143895B00(a1, a2, a3)` 的两条分支是**超时提示文案** —— `a2==0 && a3==5` → 本地化串 `91161`，`a3==6` → `91160`，都经 `sub_14668C520(..., 2875, ...)` 转发；`a2!=0` 走 UI 设置分支。**阶段时限不在这个包里**：时限来自 `.ctp`，客户端自己就能读（它读同一张表）⇒ **服务端无需下发时钟** |

**纪律（不可逾越，来自根 `AGENTS.md`）**：

- **C2S 三次上限**：同一 opcode / 同一功能最多 3 次试包，每次留可回滚记录并写 `attempt N/3`；
  第 3 次仍未闭环即停止，回报证据缺口。
- **实机由用户操作**：不无人值守启动客户端。
- **验证通过前不提交**：代码改完只到「编译 + 测试 + 待实机」。
- **不猜包**：上表 T4/T6/T7/X1/X2/X3 六项在取证/实机确认前，不得按推断写死行为。

**提交边界**（本轮约定）：`analysis/tasks/*.md`、`internal/**`、`cmd/**` 等源码与文档入库；
`analysis/dumps/` 下的反编译产物（`va-decompile/`、`registry-expand/`、`noti-*/`、
`cursor-binding/`、`sender-callers/` 等 28 MB 中间产物）**不入库**，只保留可复现的脚本。

## 7. 风险

**单人队策略引入的降级风险统一登记在 §6.2**，本节只列不因决策而改变的固有风险：

- 思路文档未实机验收；本轮已修正它一条结论（时限），其余决策仍需独立复核。
- 客户端有 Themida + BlackCipher，动态注入风险高；取证优先静态（IDA）与服务端日志。
- 军团是多人语义，单机需自造队伍身份，可能牵动 `lan_hub` / `soloPartyBootstrap` 的既有假设
  （受影响的具项见 §6.2 T1/T2）。
- **S2C 长度是硬契约**：body 短于客户端读取长度会让客户端 `MEMORY[0]=0` 崩溃（`next65` §2.2）。
  这是本子系统最容易被踩的坑——「回个空包」在其它模块可能可行，这里不行。
- 手册截图自述的边界要继承：可进入 ≠ 全流程还原；周常次数受单字节 255 上限约束；
  噩梦循环仅挑战模式「已最新验收」。

## 8. 工作切分（离线可做 / 依赖 IDA）

| 事项 | 归属 | 状态 |
| --- | --- | --- |
| `.ctp` 容器解码（header + schema + 时钟定位） | 离线 | ✅ 已完成（P0.1） |
| `.ctp` 正文变长记录编码 | 离线可推 + IDA 复核 | 🔄 进行中：先用 **`dungeonskillinfo.ctp`（1636 B、无浮点、12 键）** 当对照组解出记录规则，再用 apocalypse.ctp 验证；IDA 到位后按 reader 复核 |
| legion/apocalypse 副本与地图盘点 | 离线 | ✅ 9 个副本已索引（§3.4） |
| 作战/奖励/门禁取值（schema 22 键） | 离线 | 🔄 待正文编码闭环 |
| IDA 取证脚本 | 离线 | ✅ `analysis/dumps/ida_legion_survey.py` |
| 13 个包体的 reader 字段与时序 | **依赖 IDA** | ⏸ 阻塞 |
| 作战值映射 `0/1/2/4 ↔ 1/2/3/5` | 半依赖（源表可先离线找） | ⏸ 待 IDA 复核 |
| `0x1406afcc0` / `0x1406aae00` 客户端分支 | **依赖 IDA** | ⏸ 阻塞 |
| `internal/legion` 骨架（会话态/队伍身份） | 离线设计 | ⏸ 待 P0 闭环，避免按猜包定架构 |

## 9. 复现命令

```bash
cd server/work/dfo-lan
export GOROOT=D:/115us/tools/go GOPATH=D:/115us/tools/gopath GOCACHE=D:/115us/tools/gocache CGO_ENABLED=0
go vet ./internal/... ./cmd/...
go run ./cmd/apocalypseimport -source ../client-build/Script.inner.pvf -output runtime/apocalypse
```
