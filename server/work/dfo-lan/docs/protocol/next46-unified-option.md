# next46 — CMD2377 统一选项与技能锁（外部修复说明书的审核与本仓库落地）

日期：2026-09-21
相关：`analysis/dumps/opcode_name_to_hex.json`、`analysis/dumps/NOTI2827-角色选项默认模板-3539.bin`、
`docs/技能锁修复-补充答复-NOTI2827布局与勘误-20260921.txt`、`internal/game/protocol/unified_option.go`、
`internal/storage/skill_lock.go`、`cmd/wireprobe/unified_flow.go`、`reference/analysis-tools/unified_option_oracle.py`

## 1. 结论摘要

外部反馈分两批：原《技能锁修复-纯提示词分享版》+ 2026-09-21 的《补充答复（NOTI2827 布局 + 两处勘误）》。
**最终状态：接收侧与推送侧都已闭环**（推送侧依赖补充答复随附的 3539 字节客户端模板）。

| 分级 | 内容 |
| --- | --- |
| ✅ 已被本仓库证据证实 | opcode 编号与名称；CMD2377 帧头布局（`+8` 五字节标记、`+13` scope、`+14` subtype、`+15` count、`+19` 起 4 字节条目、0~5 字节尾部填充）；subtype `0x13` 就是技能锁且 `scope=0`；槽值 0 是删除信号；首条目的页边界语义；NOTI2826 是 3648 字节账号级块；"服务端只发 2826、从不发 2827 → 锁丢失"的现象描述 |
| ✅ 已由补充答复补齐并被本仓库验证 | NOTI2827 = 3539 字节整包；技能锁对象在偏移 **2736**（subtype 19）与 **3122**（subtype 20），各 386 字节、需同时填充；锁块内部 `obj+0 valid / obj+1 / obj+2 起 128×u16 / obj+258 起 128×exist`；模板的 subtype→偏移表（客户端 `sub57B0C0` 的 switch） |
| ❌ 已被本仓库证据证伪（对方已确认并作废） | ① 说明书称 `+04` 恒为 `01 00 00 00` —— 22 条真实帧实测有 `0`/`1`/`0xFFFFFFFF`，补充答复亦改口为"不固定，解析时忽略 `+00/+04`"；② 说明书称 `CMD682 = 0x2AA` 且"客户端约 1 秒后发出、服务端可不处理" —— 权威表 `0x2AA` 是 `ENUM_CMDPACKET_STATICS_RUNTIME_TING`，且 next37 完整上行抓包里客户端**从未发出 682**，对方已作废该条 |

## 2. 证据

### 2.1 opcode（权威表 `analysis/dumps/opcode_name_to_hex.json`）

| 编号 | 名称 | 说明书说法 |
| --- | --- | --- |
| `0x949` (2377) | `ENUM_CMDPACKET_SET_UNIFIED_OPTION` | CMD2377 SET_UNIFIED_OPTION ✓ |
| `0x94A` (2378) | `ENUM_CMDPACKET_CLEAR_UNIFIED_OPTION` | 技能锁不使用 ✓ |
| `0xB0A` (2826) | `ENUM_NOTIPACKET_UNIFIED_OPTION_ACCOUNT` | 账号级 3648 字节 ✓ |
| `0xB0B` (2827) | `ENUM_NOTIPACKET_UNIFIED_OPTION_CHARAC` | 角色级 3539 字节，技能锁走这里 ✓ |
| `0xB0C` (2828) | `ENUM_NOTIPACKET_UNIFIED_OPTION` | 本修复不依赖 ✓ |
| `0x178` (376) | `ENUM_NOTIPACKET_CHARACTER_OPTION` | 快捷栏，1028 字节 ✓ |

### 2.2 真实帧（37 条 CMD2377 明文，来自 `runtime/roles_*_next37/events.jsonl`）

固化在 `internal/game/protocol/testdata/native_unified_option_frames36.json`，并作为
`TestDecodeCapturedUnifiedOptionFrames` 的原语向量；也可用
`python reference/analysis-tools/unified_option_oracle.py server/work/dfo-lan/runtime` 复现。

- 22 条 `scope=1 subtype=0x01`（账号级其它选项块，条目 position 最大到 238，远超 128 槽）。
- 13 条 `scope=0 subtype=0x13`（技能锁），其中包含一次完整的"锁 8 个 → 追加 → 解锁 4 个"操作序列（见 2.4）。
- 1 条 48 字节、**不含 `FE` 标记**的帧（`2026-09-20T18:08:52Z`）：CMD2377 至少存在第二种载荷形态，
  本服务端安全拒绝且不建模，测试固化为"只允许恰好一条无标记帧"。

### 2.3 组结构与两份客户端块的交叉印证

客户端块由"按 subtype 排列的定长对象"组成，对象形状 = `valid(1B) + 1B + N×u16 + N×exist(1B)`，
即长度 `2 + 3N`。两条独立路径得到同一形状：

- 账号块 `templates/account-options-current.bin`（3648 字节，原生构造器 dump）实测组：
  `N=286`（= `AccountOptions()` 的 `index < 286`，860 字节）、`N=85`（257）、`N=157`（473）、`N=157`（473）。
- 角色块 `templates/unified-charac-options-current.bin`（3539 字节，随补充答复提供）按 subtype 表实测：
  `3→N=157, 4→157, 5→173, 6→173, 10→1, 12→10, 13→3, 14→3, 15→3, 16→7, 17→7, 18→6, 19→128, 20→128,
  23→2, 24→1, 25→1`。
- 这些 N 与我们从客户端 `etc/unifiedoption/unifiedoption.ctp` 字符串池切出的组键数**逐项吻合**
  （`seal skill normal` = 128 键 = `S100…S163 + S200…S263`，`system charac normal` = 173，warp gate = 10 等）。
  两份来源互相印证，因此 3539 模板与偏移可以用。

### 2.4 实机抓包验证（2026-09-21 02:09，玩家操作）

玩家做了一次完整操作：一次锁 8 个技能 → 追加锁（含一个比现有都小的技能 ID）→ 解锁 4 个。
服务端留下 12 条 `subtype=0x13` 帧，服务端合并结果、`character_skill_locks` 落库与
`reference/analysis-tools/unified_option_oracle.py` 复算**三方一致**：

| 帧 | 客户端条目 | 合并结果 |
| --- | --- | --- |
| 1 | `(0,3)` | `[3]` |
| 2 | `(0,3) (1,68)` | `[3 68]` |
| 3 | `(0,3) (1,31) (2,68)` | `[3 31 68]` |
| 4 | `(0,3) (1,9) (2,31) (3,68)` | `[3 9 31 68]` |
| 5 | `(0,3) (1,9) (2,31) (3,68) (4,109)` | `[3 9 31 68 109]` |
| 6 | `(0,1) (1,3) (2,9) (3,31) (4,68) (5,109)` | `[1 3 9 31 68 109]` |
| 7 | `… (4,58) …` | `[1 3 9 31 58 68 109]` |
| 8 | `… (3,20) …` | `[1 3 9 20 31 58 68 109]` |
| 9–12 | 逐个移除 9 / 31 / 68 / 3 | `[1 3 20 31 58 68 109]` → `[1 20 58 109]` |

结论：

- 客户端**每一帧都整页重建** page 0（首条目 `position=0`，条目数 = 当前锁定集合大小，紧凑升序），
  包括"插入一个更小 ID 导致整页重排"和"删除导致后续槽前移"两种情形。因此"首条目落在页边界即整页重建"
  在本客户端**成立且是常态**。
- 本轮**未出现** `value=0` 的删除条目（本客户端用重建帧表达删除）。"槽值 0 视为删除"的分支按说明书保留，
  目前未被实测覆盖，但合成帧已覆盖（见 3 的单元测试）。
- 回归：`TestCapturedSkillLockSession` 用这 12 条真实帧断言逐帧集合。

## 3. 本仓库已实现

| 位置 | 内容 |
| --- | --- |
| `internal/game/protocol/unified_option.go` | `DecodeUnifiedOption`（帧布局 + 标记/长度/填充校验，**不校验 `+00`/`+04`**）、`CompactSkillSlots`/`SkillIDsFromSlots`、`MergeSkillLocks`（增量合并、槽值 0 = 删除、首条目落在 0/64 时先重建该页）、`EncodeSkillLockBlock`（386 字节对象）、`UnifiedCharacOptions`（3539 字节整包：内置客户端模板 + 同时填充 subtype 19/20 两个锁对象）、`UnifiedCharacOptionsFrom`（不同客户端版本时替换模板/偏移）、`CharacOptionsTemplate` |
| `internal/game/protocol/templates/unified-charac-options-current.bin` | 客户端 3539 字节角色选项默认块（`go:embed`），sha256 `dd5ccdc7a84f66f799594edd1e8ceddfa957fdebc7d566e955d178038236f816` |
| `internal/storage/skill_lock.go` | `character_skill_locks` 表（`IF NOT EXISTS` 幂等迁移）、`SkillLocks`、`CommitSkillLocks`（角色行锁 + `character_events` 幂等键 + 先删后插整体替换；重放同一帧不重复应用；拒绝 0/越界/重复/超 128） |
| `internal/character/unified_option.go` | `SaveSkillLocks`（把一帧合并进该角色的集合） |
| `cmd/wireprobe/main.go` | CMD2377 按 subtype 分流：`0x13` 落库并记 `skill_lock_saved`，其它 subtype 仍客户端自管；启动执行 `MigrateSkillLocks`；入口处准备 NOTI2827 载荷 |
| `cmd/wireprobe/entry_flow.go` + `unified_flow.go` | NOTI2827 作为**入口帧序列的最后一帧**（`packets()` 末尾，排在 NOTI14 外观刷新之后）；默认使用内置模板与偏移 2736，`-unified-charac-template` / `-skill-lock-offset` 仅作为不同客户端版本的逃生口 |
| `reference/analysis-tools/unified_option_oracle.py` | 取证/验证脚本：从任意 `events.jsonl` 提取 2377 明文、按 subtype 分类、逐帧合并，`--expect` 可比对玩家实际锁定的技能 |

测试：`unified_option_test.go`（真实帧向量、合并/重建/删除/跨页、锁块编码/往返/上限、
内置模板的空对象与两处填充、subtype 18 不被触碰）、`unified_flow_test.go`（内置模板与覆盖路径）、
`skill_lock_test.go`（隔离 schema 集成：替换语义、幂等重放、非法集合拒绝）、
`worn_display_handoff_test.go`（"2827 可作最后一帧，但必须排在 NOTI14 之后"）。

## 4. NOTI2827 推送规格（已落地）

1. **整包**：复制客户端 3539 字节默认块，只改两个锁对象，其余字节保持模板默认值。
2. **两个锁对象**：subtype 19 @ **2736**、subtype 20 @ **3122**（`2736 + 386 = 3122`，相邻），
   填**相同**的锁数据。只填 2736 时重选角色/重启后锁会不全；subtype 18 @ 2716 只有 20 字节，不是锁块。
3. **对象内部**：`obj+0 = 1`、`obj+1 = 0`、`obj+2 + 2*slot` = 槽值（page0 = 技能 ID，page1 = ID−512，
   空槽 `0xFFFF`）、`obj+258 + slot` = exist(1)。
4. **发送时机**：进城整组数据帧的**最后**（快捷栏 NOTI376 之后、所有数据帧之后）。放早会在进城后
   约 0.3~1 秒崩溃，与内容是否正确无关。
5. **NOTI376 内也带一份技能 ID 列表（偏移 260，`0xFFFF` 终止）**：补充答复称其非决定性；本仓库暂未实现
   NOTI376 的技能列表部分。

## 5. 实机验收步骤（由玩家操作，服务端只读日志）

前置：`启动服务端.cmd`（默认加载源码版 `bin/wireprobe-handoff-source.exe`；2827 默认启用）。

1. 进游戏，在技能面板一次锁 8~20 个技能，再追加锁 2~3 个、解锁 2~3 个。
2. 回角色选择再进（或重启服务端后重进），确认：
   - **锁全部保留**（这是本轮修复的目标）；
   - 新锁/解锁的增删正确；
   - **客户端不闪退**（2827 时机正确的重要判据）。
3. 服务端侧应留下 `skill_lock_saved`（含 `applied/count/locks`）与 `entry_skill_lock_prepared`
   （含 `count/bytes`，`bytes` 应为 3539）。
4. 如需再次核对合并算法：

   ```powershell
   python server\reference\analysis-tools\unified_option_oracle.py server\work\dfo-lan\runtime --expect <你实际锁定的技能ID>
   ```

## 6. 与项目硬规则的一致性

- 3539 模板来自客户端本身，偏移来自客户端 `sub57B0C0` 的 switch 表（非推测），且与我们从
  `unifiedoption.ctp` 独立解析出的组键数互相印证后才接入。
- 说明书被证伪的两条（`+00/+04` 常量、`CMD682`）都未进入代码：解析从 `+14` subtype 开始，不读 `+00/+04`。
- 数据库为新增表 + `IF NOT EXISTS` 迁移，老角色存档不受影响（锁集合默认空）。
- 2827 放在帧序列末尾由代码固定，测试断言防止回归。
