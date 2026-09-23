# next67 — P2：作战选择 `LEGION_OPERATION_SELECT(2354)` + ACK

> 前置：`next66`（P1 军团入口）。包体契约：`next65-legion-packet-table.md`。
> 妥协台账：`next64` §6.2（本轮新增 **X4**）。
> 状态：**代码完成，待实机**。P1 与 P2 一并等一个实机窗口，不单独验证。

## 1. 目标

服务端能应答 `CMD2354`，让玩家在作战选择界面上的动作被服务端认可。

## 2. 本轮新取证（IDA，`va-decompile/`）

### 2.1 C2S 字段语义由客户端的调用者直接钉死

`sub_1424FE3F0(int a1, int a2, char a3)` 的两个调用点：

```c
sub_14069B580(): return sub_1424FE3F0(107, 1, -1);     // 按钮 C → int32@13 = 1
sub_14069B560(): if (*(a1+328)) return sub_1424FE3F0(107, 2, a2);  // 按钮 B → @13 = 2
```

⇒ C2S 2354 的字段确认：

| 偏移 | 语义 | 值 |
| --- | --- | --- |
| `int32@13` | **动作**（玩家在作战界面选的动作） | `1` 或 `2` |
| `char@17` | 附加字节（UI 直传，语义未定） | `-1` 或按钮参数 |
| `int32@18` | 通道码 | `107` |

这也把 `next65` 里 `int32@13`/`char@17`/`int32@18` 的对应关系从"偏移已知"升级为**语义已知**。

### 2.2 S2C 的结构由消费端揭示

`sub_1424FD320`（2354 handler）读 `u16`（须 107）+ 14 字节结构后交给 `sub_14069A360(a1, a2)`，后者按**首 dword** 分支：

```c
if (*(_DWORD *)a2 == 1) {
    if (*(_BYTE *)(a2 + 5) != 0) { … 关闭/切换界面 … }
    v7 = *(_DWORD *)(a2 + 6);            // 作战 id 走这里
    … sub_1406AFC60(v9, v7);
} else if (*(_DWORD *)a2 == 2) {
    … 前置检查通过后 … sub_1424FE290(107, *(_DWORD *)(a1 + 115));   // ← 客户端自发 CMD2045
}
```

⇒ 结构语义：

| 结构偏移（相对 body `+2`） | 语义 |
| --- | --- |
| `+0` (dword) | **动作**：`1` = 刷新作战界面、`2` = 确认进入 |
| `+5` (byte) | 刷新分支的标志位 |
| `+6` (dword) | 刷新分支的作战 id |

**关键发现**：动作 `2` 的分支里，客户端**自己**会发 `CMD2045 LEGION_ENTER_DUNGEON`（参数 `(107, 选中作战 id)`）——P3 的进入动作不由服务端驱动。

## 3. 实现

| 文件 | 内容 |
| --- | --- |
| `internal/legion/session.go` | `OperationChannelCode = 107`、`DecodeOperationSelect`（三字段解码）、`OperationAck(action)`（`u16(107) + 14 B`，首 dword 回显）、`Session.Action`/`Auxiliary`、`SelectOperation`、`Reset` 一并清理 |
| `internal/legion/session_test.go` | 追加 6 个测试：三字段解码、两个动作各自 round-trip、长度不足拒绝、**`OperationAck` 布局与 107 通道码**、`SelectOperation` 幂等 |
| `cmd/wireprobe/legion_flow.go` | `case legion.CmdOperationSelect`：校验通道码 == 107、要求已走过 2043（无会话即拒绝）、记录动作、回 ack |

## 4. 假设与降级（**已登记 `next64` §6.2 X4**）

- **假设**：S2C 2354 应当**回显**客户端请求的 action；结构 `+6` 的作战 id 留零。
  - 依据：两端用同一组 `1/2` 表达同一组动作；长度与通道码是确定契约。
  - 失败回退：改发**全零结构** —— 消费端首 dword 既非 1 也非 2 时直接 `return`，**安全但无动作**（不会崩）。
- **未做**：`NOTI2896 LEGION_OPERATION`（作战列表，客户端读 `u16(107) + 7 B`）本轮**不主动下发**。
  理由：`2043` 之后客户端的作战列表数据来源未确认（可能来自客户端配置 `apocalypse.ctp` 的
  `[operation data set]`）。实机若出现「列表为空」，则该 NOTI 是下一步。
- **未做**：`0/1/2/4 ↔ 1/2/3/5` 值映射（§6.2 X3）——不影响本轮（客户端发的是 1/2）。

## 5. 实机验证（**与 P1 同一窗口**）

进城镇 → 打开军团入口 → 作战选择界面，关注日志：

| 观察点 | 期望 | 说明 |
| --- | --- | --- |
| `legion_operation_ack` 是否出现 | 出现 | 说明客户端确实发了 2354 且服务端应答 |
| `request_hex` 的 `@13` | `01 00 00 00` 或 `02 00 00 00` | 验证 §2.1 的动作字段 |
| `request_hex` 的 `@18` | `6B 00 00 00`（107） | 验证通道码 |
| 后续是否出现 `legion_refused` id=2045 | 出现即成功 | 动作 2 分支里客户端自发 2045（§2.2），P3 的输入 |
| 界面表现 | 选择被认可 / 无反应 | 无反应且无 `ack` = 客户端没发；有 `ack` 但界面不刷新 = X4 假设不成立，走全零回退 |

**回滚**：删除 `internal/legion/`、`cmd/wireprobe/legion_flow.go`，还原 `main.go` 与
`request_scope.go` 的改动；**无数据库改动**。

## 6. 下一期（P3）输入已就绪

| 项 | 状态 |
| --- | --- |
| `CMD2045 LEGION_ENTER_DUNGEON` | C2S 21 B = `int32@13`(=107) + `int32@17`(作战 id)；由客户端在动作 2 分支自发 |
| `CMD2355 APOCALYPSE_ROLE_SELECT` | C2S 21 B = `int32@13` + `int32@17`(=107 常量) |
| `NOTI2568 PREPARE_LEGION_ENTER_DUNGEON` | S2C body **≥16 B** |
| `NOTI2254 LEGION_ENTRY_CHARAC_INFO` | S2C body **≥256 B**（单人队只填本人槽位，§6.2 T4） |
| 职责语义 | 散兵 / 守卫（`dungeonskillinfo.ctp` 的 `[skirmisher info]` / `[gaurdian]`） |
