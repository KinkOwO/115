# 城镇左上角 "Liberation Trace" 常驻面板：根因与服务端抓手（NOTI 398，confirmed baseline 2026-09-23）

> **状态**：2026-09-23 **confirmed baseline**——实机帧验证通过（会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260923_220820_905214_next37`：`booster_gage_hidden` id 398、14 字节、type 0 下发，**无** CMD 217 溢出），用户实机后指示提交收口。
> **取证真源**：上游 115 级**脱壳镜像** `DFO.exe.bak`（⚠️ 非原版，已知自带一处人工补丁：NOTI 1 频道信息处理器内 `0x1452c804e`），采集日期 2026-09-21；资源真源 `Script.pvf`（`Pvf110.Cli.exe` 只读子命令解包）。
> **⚠️ 本仓复测缺口**：本仓当前无 `client/` 目录（无 `DFO.exe.i64` 与脱壳镜像）、无 `analysis/scripts/capstone-dfo`，**以下 VA 未能在本仓镜像上复测**。按上游静态取证落码，包长 14 是硬约束；实机验收时若出现 CMD 217 溢出且指向 398，立即关开关并回报，不得叠包。

## 0. 摘要

城镇左上角那块没有关闭按钮、常驻遮挡 UI 的 "Liberation Trace" 面板，**服务端无法用任何包把它移除** —— 它是客户端 UI 管理器在构造期无条件建出来的。
**唯一的服务端抓手**是补发一帧 **NOTI 398 `ENUM_NOTIPACKET_BOOSTER_GAGE`**，把最后一个字段 `displayValue` 给 **0**，让客户端自己走到 `SetVisible(false)`。
这条路上最贵、也最容易翻车的点是**包体长度 = 14 字节，不是 18**：按 reader 外形直觉写成两个 `u64` 会让客户端读超 frame，进而吞掉后续 CMD（表现为已知的 CMD 217 wire-overflow）。

## 1. 症状

- 进入城镇后，屏幕**左上角**固定出现一块面板，文案 "Liberation Trace"（月常里程奖励说明）。
- 面板**没有关闭按钮**，玩家无法自行收起，持续遮挡左上角 UI。
- 服务端此前**从未下发过 NOTI 398**（本仓全库 0 处）⇒ 面板停留在初始态，即"永远在"。

## 2. 面板身份（资源侧）

| 项 | 值 |
| --- | --- |
| 宿主窗口 | `live/else/kor/2022/220922_liberationmileage/main.xui` |
| 内嵌面板 | `live/else/kor/2022/220922_liberationmileage/randomboostermileage2.xui` |
| 锚点 | 左上角 `Pos 3,35` |
| 本地化文案 | table 20，键 `random_box_monthly_ui_0..6` |
| 关联货币道具 | `ItemIndex = 10357411` |
| 相关配置 | `Etc/BoosterGage.etc`，其中 `[sub gage] 50`、`[panel show time] 5000` |

## 3. 根因：为什么"关不掉"

1. 面板构建器 = **`sub_1455c6f90`**；唯一调用者 = **`sub_145526430 + 0x2d1e`**。
2. 该调用点的条件里**没有任何服务端数据** —— 只判断刚建出来的 `[rsi+0x1378]` 非空。调用链直通 `wWinMain → InitMain → CNGameCore::Create → initRDAR`，即 **UI 构造期**。
3. `randomboostermileage2.xui` 的**根节点没有 IsVisible 属性** ⇒ 默认永远绘制，且没有可点关闭的控件（体内只有隐藏的 `btn_GetReward`）。

⇒ 结论：**"不发任何包它就也在"是正确预期，不是漏发**。任何"靠不发包让它不出现"的思路都不成立。

## 4. 唯一服务端抓手：NOTI 398

opcode 登记（本仓 `analysis/dumps/opcodes.tsv`）：

```
noti	398	0x018E	ENUM_NOTIPACKET_BOOSTER_GAGE	0x14b07f7c0	0x14ef34120
```

- handler = **`0x1455c6d30`**
- 面板内容/局部可见性刷新器 = **`sub_1455c8d20`**，其**唯一调用者就是 398 handler**（`0x1455c6efd`）
- `sub_1455c8d20` 尾部：

```
0x1455c8ee5  test  r15d, r15d
0x1455c8ee8  setg  dl                    ; dl = (r15d > 0)
0x1455c8eeb  call  qword ptr [rax+0x18]  ; SetVisible(dl)  ← 作用在子控件 [rsi+0x5b8]
```

- `r15d` 的来源：`eax = displayValue / [r14+0x384]`，而 `[r14+0x384]` 取自 `Etc/BoosterGage.etc` 的 `[sub gage] 50`。

⇒ **`displayValue = 0` ⇒ 除法结果 0 ⇒ `SetVisible(false)`**。这是整条链上唯一由服务端数据控制的开关。

## 5. ⚠️ 包体 = 14 字节（本条最贵，请放在实现注释里）

398 handler 的 reader 逐条实测宽度：

| 序 | 指令地址 | 读取器 | 实际宽度 | 备注 |
| --- | --- | --- | --- | --- |
| 1 | `0x1455c6d5e` | `0x146ea09f0` | **1** | u8 增量 A |
| 2 | `0x1455c6d70` | `0x146ea09f0` | **1** | u8 增量 B |
| 3 | `0x1455c6d82` | `0x146ea0be0` | **8** | 可变宽读取器，此处 `edx = 8`（体内 `mov r8d, esi` → `call 0x148aa1e60` memcpy，游标 `+= rsi`） |
| 4 | `0x1455c6d8e` | `0x146ea0ba0` | **4** | 固定 4 字节（`mov eax, dword [rdx]; add rdx,4`） |

⇒ **body = 1 + 1 + 8 + 4 = 14 字节**，字段顺序为 `u8 incA, u8 incB, u64 rawA, u32 displayValue`。

**为什么必须写死 14**：上游第一版取证把第 4 个字段当成 `u64`，得出 18 字节（`u64 + u64 + u8 + u8`）。按 18 发会让客户端**读超出 frame 声明长度**，触发 writer 吞掉后续 CMD 的已知路径（日志里的 `wire-overflow-report-217`）。**症状会表现为"发了 398 之后别的包丢了"**，很容易被误判成协议结构错误。

## 6. 两个 u8 是"增量"，不是"设置值"

```
0x1455c6de0  lea   rsi, [r14+0x3dc]
0x1455c6dfb  add   eax, 0xc4
0x1455c6e00  add   ecx, eax
0x1455c6e02  mov   [rsi+4], ecx      ; 累加，不是赋值
```

`u8@+0` 累加进 `[r14+0x3dc]` 系计数器，`u8@+1` 累加进 `[r14+0x3d4]` 系计数器，累加后与上限 `[r14+0x380]` 比较（`jl` 决定是否继续）。

⇒ 398 本质是"给 booster gage 加进度"的包，**不能清零**。所以修法把它俩**留 0**（加 0），只动第 4 个字段。⚠️ 若以后要做 booster 里程进度功能，注意这两个字节是累加语义，重复下发会叠加。

## 7. 帧序

本仓实现把 398 放在进场组里 **NOTI 124（`enter_gameworld_complete`）之后**、NOTI 37 之前：

```
... story_digest_restored (1370) → enter_gameworld_complete_sent (124) → booster_gage_hidden (398) → entry_experience_restored (37) ...
```

124 之前客户端的面板对象尚未完成初始化，故不放在更早位置。

## 8. 已证 / 未证（不要当成全闭环）

| 项 | 状态 |
| --- | --- |
| 面板由 UI 构造期无条件创建 | ✅ 上游静态已证（`sub_1455c6f90` + 唯一 caller 无条件） |
| NOTI 398 body = 14 字节 | ✅ 上游静态已证（逐条 reader 宽度）；⚠️ 本仓未复测 |
| 两个 u8 为增量语义 | ✅ 上游静态已证 |
| `displayValue=0` ⇒ 走到 `SetVisible(false)` | ✅ 上游静态已证（`sub_1455c8d20` 尾部） |
| **整块面板因此消失** | ⚠️ **上游只有一次实机正证据**：`SetVisible` 打在子控件 `[rsi+0x5b8]` 上，静态无法证明它能收起整个面板；上游 2026-09-22 实机回报"那个窗口消失不见了"，且跨多次进城保持 |
| `[panel show time] = 5000` 的单位与作用 | ❌ 未证（只读到解析进 `[rdi+0x65]`） |
| 两个 u8 的取值域 | ❌ 未证（一律发 0） |

## 9. 被排除的死路（省两轮试错）

1. **NOTI 402 / 426（`INFORM_NOTICE`）与 CMD 469 / 495 与本面板无关。**
   469/495 的机制本身成立（469→A 表 / 495→B 表，`[u8 N][u8 slot]*N` 回灌，极性 `return !contains(slot)`），但：
   - **CMD 469 / 495 在客户端根本没有处理器**（全量 opcode TSV 无行）⇒ 发了没人等、回也回不到代码。
   - 正向反证：构建器 `sub_1455c6f90` 体内 **0 处** contains/mark/notice-slot 接触点。
2. **不要用 exe 字符串反查这个窗口。** DSTR / XORSTR 全量产物里没有 "Liberation Trace" —— 文案在 PVF table 20，运行时才解密。
3. **不要指望"补一个关闭按钮"。** xui 根节点无 IsVisible，面板本体不可服务端移除；能动的只有子控件显隐。

## 10. 本仓实现（2026-09-23）

| 文件 | 改动 |
| --- | --- |
| `internal/game/protocol/booster_gage.go` | `BoosterGage(displayValue uint32) []byte` 封 14 字节（`0,0` + `u64 0` + `u32 displayValue`），注释钉 reader 地址与 14/18 的坑 |
| `internal/game/protocol/booster_gage_test.go` | 钉包长 14、字段位置、displayValue 在尾部、"18 字节是错的"回归 |
| `cmd/wireprobe/entry_flow.go` | `entryPayloads` 新增 `BoosterGage []byte`；`packets()` 在 NOTI 124 之后插一帧 `booster_gage_hidden`（id 398，kind 名沿用上游） |
| `cmd/wireprobe/main.go` | 开关 `-booster-gage-hide`（`flag.Bool`，默认开，env `DFO_BOOSTER_GAGE != "0"` 同步生效）；进场组装处 `plan.BoosterGage = protocol.BoosterGage(0)` |

- **开关形态**：按本仓网关 CLI flag 惯例（对照 `-apocalypse-catalog` / `-shop-release`），默认**开**。
- **回退把手**：`-booster-gage-hide=false` 或 `DFO_BOOSTER_GAGE=0` ⇒ `plan.BoosterGage` 为空 ⇒ `preparePackets` 跳过空 payload ⇒ 等价于修复前行为（面板回到常驻态）。
- 不涉及数据库结构、存档格式与包长变更。

## 11. 复现命令（本仓暂无对应镜像与工具，供有 `client/DFO.exe.i64` 的环境复测）

```bash
# 客户端镜像（脱壳版，只读）
export DFO_EXE=<镜像路径> PY=<repo>/tools/python/python.exe
$PY dfo_dis.py func 0x1455c6f90 400     # 构建器：体内无服务端条件
$PY dfo_xref.py call 0x1455c6f90        # 唯一 caller
$PY dfo_xref.py call 0x1455c8d20        # 唯一 caller = 398 handler
$PY dfo_dis.py func 0x1455c6d30 300     # reader 顺序 u8,u8,u64,u32 与消费路径
$PY dfo_dis.py func 0x146ea0ba0 16      # 宽度=4（钉死 14 字节的关键）
```

## 12. 实机验收判据（由用户操作，不要无人值守起客户端）

1. 进城镇 ⇒ 左上角面板**不再出现**。
2. 会话日志出现该帧（`booster_gage_hidden` / id 398）；**不应**出现 CMD 217 溢出举报。
   - 若出现 217 且指向 398 ⇒ 包长仍然错，立刻 `-booster-gage-hide=false`（或 `DFO_BOOSTER_GAGE=0`）关开关回报，不要继续叠包。
3. 反复进城/退城 ⇒ 面板保持不出现（上游实测跨多次进城保持）。
4. **负结果处置**：若面板仍在 ⇒ 说明该 `SetVisible` 只收起子控件，本问题应归档为**客户端本地缺陷、服务端无解**，并把这次负结果回写进本文档。
