# analysis/dumps/ — 逆向分析核心资产 Dump 库

本目录汇总了从当前 115 级客户端（`client/DFO.exe`）、PVF 资源（`Script.inner.pvf`）与本地化数据（`client/dstr.dat`）中提取的核心数据资产，供逆向工程、IDB 分析及服务端协议实现快速查阅与索引。

---

## 1. 产物总览

| 分类 | 文件名 | 格式 | 记录数 / 大小 | 说明 |
| --- | --- | --- | --- | --- |
| **Opcode** | `opcode_name_to_hex.json` | JSON | 5,330 条 (289 KB) | CMD 与 NOTI 数据包原生名称到 Hex 编号映射字典 |
| **Opcode** | `opcode_table_detailed.json` | JSON | 5,330 条 (1.00 MB) | 含协议族别 (cmd/noti)、Dec/Hex ID、字符串 VA 及分发槽 VA 的完整明细 |
| **Opcode** | `opcodes.tsv` | TSV | 5,330 行 (419 KB) | 适合命令行 `grep` / `awk` 快速过滤的表格 |
| **XORSTR** | `xorstr_addr_to_text.json` | JSON | 114,614 条 (5.36 MB) | 客户端内存 VA（十六进制字符串）到解密后明文字符串映射 |
| **XORSTR** | `xorstr_map.tsv` | TSV | 114,614 行 (4.48 MB) | 包含地址与单行转义明文的 TSV 快速检索表 |
| **XORSTR** | `ida_annotate_xorstr.py` | Python | - | IDAPython 脚本：一键在 IDA 中为所有 xorstr 批量设置 Repeatable Comment |
| **DSTR** | `dstr_id_to_text.json` | JSON | 34,894 条 (1.99 MB) | DSTR 翻译 ID 到本地化明文字符串字典 |
| **DSTR** | `dstr_table_detailed.json` | JSON | 34,894 条 (6.14 MB) | 包含 ID、所属源码 C++ 文件名及翻译内容的明细数组 |
| **DSTR** | `dstr_map.tsv` | TSV | 34,894 行 (3.91 MB) | `id \t source_file \t text` 表格 |
| **DSTR** | `dstr_raw.txt` | TXT | 41,333 行 (1.89 MB) | 解密还原后的完整 Neople 原生 DSTR 本地化文本文件（带注释） |
| **统一选项** | `NOTI2827-角色选项默认模板-3539.bin` | BIN | 3,539 字节 | 客户端角色级统一选项默认块（NOTI2827 整包）。subtype 19 @2736 与 subtype 20 @3122 为 386 字节技能锁对象（初始为空：`valid=0`、128×`0xFFFF`、exist 全 0），其余字节为该客户端默认值 |

---

## 2. 快速使用示例

### A. 查询 CMD / NOTI Opcode

- **通过名称查 Hex 编号**（Python / jq）：

  ```python
  import json
  opcodes = json.load(open("analysis/dumps/opcode_name_to_hex.json", encoding="utf-8"))
  print(opcodes["ENUM_CMDPACKET_SELECT_CHARACTER"]) # 输出: 0x0004
  print(opcodes["ENUM_CMDPACKET_ITEM_USE"])          # 快速定位对应命令
  ```

- **命令行检索**：

  ```bash
  grep -i "MAIL" analysis/dumps/opcodes.tsv
  grep -i "SHOP" analysis/dumps/opcodes.tsv
  ```

### B. 查阅与定位 xorstr 加密字符串

- **在 IDA Pro 中快速定位函数**：
  若在反汇编中看到 `lea rcx, [rip + disp]` 传入的常量地址为 `0x149486510`，直接查表：

  ```python
  import json
  strings = json.load(open("analysis/dumps/xorstr_addr_to_text.json", encoding="utf-8"))
  print(strings.get("0x149486510")) # 输出明文字符串
  ```

- **批量导入 IDA 注释**：
  在 IDA Pro 中执行 `analysis/dumps/ida_annotate_xorstr.py`，即可为 IDB 中所有 114,614 处静态加密字符串打上可重复注释。

### C. 查询 DSTR 本地化翻译

- **查询 PVF 中引用的文本键**：
  PVF 中的 Type 8 cell 常引用 DSTR ID，或当需要查看客户端内部报错、UI 文本时：

  ```python
  import json
  dstr = json.load(open("analysis/dumps/dstr_id_to_text.json", encoding="utf-8"))
  print(dstr.get("0"))     # "You have exceeded the maximum number of robots."
  print(dstr.get("29154")) # "Vanguard"
  ```

- **按 Neople C++ 源码模块查找字符串**：

  ```bash
  grep "CNSelectCharacterModule.cpp" analysis/dumps/dstr_map.tsv
  ```

---

## 3. 逆向技术背景与解密链条备忘

1. **Opcode 注册机制**：
   - 客户端在 `0x140069bb0`（Command 表初始化）和 `0x140075000`（Notification 表初始化）中，按顺序对每个数据包结构调用 `0x146e8c7d0` 解密名称，并存入全局表槽位（CMD 表基址 `0x14ef38f60`，NOTI 表基址 `0x14ef334b0`）。
2. **xorstr 加密算法**：
   - 数据头格式：`[x, x|..., len_low, len_high]`，通过 `key = (header[1] & 0xfe) | 0x9a714ca0` 派生初始密钥，循环内迭代 `key = (key * 0x1003f + v) & 0xffffffff`。
3. **dstr.dat 加密算法**：
   - 文件大小 `N`，有效密文长度为 `N - (N & 0xFF)`。
   - 双层 AES-256-CBC 解密（IV 均为 16 字节 `0x00`）：
     - Layer 1 Key: `9D6C4A333560167E8D276B81E32B537867E862341A1D3E6E9955E48819F4C899`
     - Layer 2 Key: `C50796A39913B6D5156B6C651CE1F1F82542953F338D7BE87121E82CB45A810E`
   - 解密后文件头为 `SC01` + 4 字节原大小。后续数据以 `0x5819af17` 为初始种子进行 CBC 式 4 字节连续 XOR 解扰，还原为原生 UTF-8 格式文本。
4. **统一选项块（UNIFIED_OPTION）布局**：
   - CMD2377 `SET_UNIFIED_OPTION` 一帧携带一个选项块：`+8` 五字节 `FE FF FF FF FF` 标记、`+13` scope、`+14` subtype、`+15` count、`+19` 起 `(u16 position, u16 value)` 条目；`+00`/`+04` 在不同 subtype 下取值不同，**不可当常量校验**（当作常量会漏帧）。
   - 账号块 NOTI2826 为 3648 字节（对应客户端 `sub_14757AE50`）、角色块 NOTI2827 为 3539 字节（`sub_14757B0C0`）；块内对象形状统一为 `valid(1B) + 1B + N×u16 + N×exist(1B)` = `2 + 3N` 字节。
   - 角色块 subtype→偏移（取自客户端 switch 表）：`19→2736`、`20→3122`（各 386 字节，均为技能锁对象，需同时填充同数据，服务端推送时作为进城帧序列的最后一帧）。
   - 服务端实现与验收记录见 `server/work/dfo-lan/docs/protocol/next46-unified-option.md`。

---

## 4. 客户端通信机制与真值索引（可复用，**新任务先看这个**）

见 [`CLIENT-MECHANICS.md`](./CLIENT-MECHANICS.md)。2026-09-23 军团频道 / 末世录取证的沉淀，
内容覆盖：

| 章节 | 内容 |
| --- | --- |
| §1 | **字符串解密**（算法 + `pe_decode_xorstr.py`；已用 17 个枚举名反向校验 dump 命名可信） |
| §2 | C2S 发包器 + 「期望回包树」（**不回同 id 包客户端会卡住**） |
| §3 | S2C 读取游标 API 全家 + **长度硬契约**（短了客户端崩） |
| §4 | handler 注册表（CMD/NOTI 两个 helper；别用 `find_imm` 全盘扫） |
| §5 | C2S 13 字节前导的确切结构 + 服务端 `p[13:]` 口径的 5 个先例 |
| §6 | **`.ctp` 脚本表加载链 6 层** + 主解析器字符分派 + 两个已知 schema（21/12 标签） |
| §7 | MSVC 容器判别（`wstring` SSO、`vector` 边界、异常帧、`MEMORY[0]=0`） |
| §8 | 军团/末世录 **包契约速查表**（C2S 布局 + S2C 最小长度） |
| §9 | **7 条已踩过的坑**（cmd/noti 同号不同义、`table_slot_va` 不是 handler 槽、toggle 语义…） |
| §10 | 工具与脚本索引 + IDA 批处理调用方式（含必清 `PYTHONPATH`） |
