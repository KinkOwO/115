# next66 — P1：军团频道入口 + `LEGION_START(2043)`

> 分期依据：`analysis/tasks/next64-legion-apocalypse-plan.md` §6（P1）。
> 包体契约：`analysis/tasks/next65-legion-packet-table.md`（C2S §1、S2C §2.2、收包链路 §2.3）。
> 状态：**代码完成，待实机**（按提交纪律，实机通过前不提交）。

## 1. 本轮目标与边界

| 项 | 内容 |
| --- | --- |
| 目标 | 服务端能正确应答 `CMD2043 LEGION_START`，让客户端进入城镇作战选择界面 |
| 范围内 | `internal/legion`（会话态 + 2043 编解码）、`cmd/wireprobe` 接线、C2S 原始 body 记录 |
| 范围外 | 2044/2045/2046/2354/2355 的实际行为（P2–P5）；频道入口的暴露条件（D1，待实机观察） |
| 未实现包的处理 | **显式拒绝并记录**，不返回编造数据去清客户端的等待态 |

## 2. 真源依据（全部来自 `next65`，非推测）

| 项 | 结论 |
| --- | --- |
| C2S 2043 布局 | 17 字节 = 13 字节信封 + `u32@13`（证据 `sub_1424FE550`，`r8d=0x11`） |
| S2C 2043 契约 | 客户端 handler `sub_1424FD900` 执行 `sub_146EA0BE0(&v27, 4)` ⇒ **body ≥ 4 B**；短了客户端写空指针崩溃 |
| S2C body 形态 | 裸 payload，**不含信封**（仓库既有 `protocol.Refusal` 亦如此） |
| 时序要求 | 客户端发包后登记「期望回包树」，handler 先查树——**清不掉就整个跳过** ⇒ 必须回同 id 包 |
| 13 字节信封结构 | `01 | opcode(u16 LE) | 0×10`（`sub_146D746E0` 首次发送时写入） |

## 3. 实现清单

| 文件 | 内容 |
| --- | --- |
| `internal/legion/session.go` | 包文档（D2/D3 决策 + §6.2 妥协索引）、opcode 常量、`EnvelopeSize`、`Requests`、`DecodeStart`、`StartAck`、`Session`（`Begin`/`Reset`） |
| `internal/legion/session_test.go` | 8 个测试：信封不泄漏进字段、**接受未定论的更长 body 并回报真实长度**（X1）、长度不足拒绝、`StartAck` 满足 4 B 硬契约、`Begin` 幂等、`Reset`、`Requests` 不误吞撞号 opcode |
| `cmd/wireprobe/legion_flow.go` | `legionSession.handle`（城镇 + 已选角 + 非副本三守卫；2043 应答；其余显式拒绝）、`legionRequestBody` |
| `cmd/wireprobe/main.go` | 连接作用域 `legionState`；`frame.Type==1 && legion.Requests(id) && bootstrapped && verified && worldState != nil` 分支；**无论成功/拒绝都记录 C2S 原始字节与长度** |
| `cmd/wireprobe/request_scope.go` | `retainRequestBody` 纳入 `legion.Requests` ⇒ 家族 6 个包体全部保留（不采样） |

## 4. 关键设计（决策依据见 `next64` §6.1 / §6.2）

- **单人队（D2）**：`Session.PartySize` 恒为 1，且刻意具名——让这个妥协在每个使用点可见。
- **不入库（D3）**：会话态只在内存；连接重建即重置。不改数据库、不动存档兼容。
- **不解释未知字段**：2043 的 `u32@13` 语义未取证，代码只**记录**不解释（`StartRequest.Argument` 的注释明确禁止编造语义）。
- **未实现即拒绝**：`default` 分支返回错误，日志出 `legion_refused`。这是刻意的——用编造数据回包能把客户端的等待态清掉，但会让后续排障失去真值。

## 5. 实机验证（**请用户操作**）

```
停止游戏环境.cmd  →  启动服务端.cmd
```

然后选角色进城镇，尝试触发军团入口（具体入口位置未知，这正是 D1 要观察的），关注日志：

| 观察点 | 期望 | 说明 |
| --- | --- | --- |
| `legion_start_ack` | `request_bytes` + `request_hex` + `plain_hex` | **这是 X1 的定论依据** |
| `request_bytes` 的值 | 17 或 30 | 17 = 信封含在客户端 append 内；30 = 另有 13 字节信封在外。**两种都已被代码接受**，看日志即可定论 |
| `legion_refused` | 若出现，记下 `id` | 说明玩家触发了 P2–P5 的包（好东西：这就是下一期的输入） |
| 客户端表现 | 出现作战选择界面 / 无响应 | 无响应时看有没有 `legion_start_ack`：有 ack 但界面没出 = 入口条件问题（D1）；没有 ack = 客户端没发 |

**回滚**：删除 `internal/legion/`、`cmd/wireprobe/legion_flow.go`，还原 `main.go`（去掉 import、`legionState` 声明、处理分支）与 `request_scope.go` 的一行改动即可；**无数据库改动**。

## 6. 遗留（P2 起）

1. **X1**：C2S 信封归属——本轮日志一眼定论（见 §5）；
2. **D1**：军团入口暴露条件未取证（触发点 `sub_142510A50` / `sub_142511D10`）；
3. **P2 输入已就绪**：2354 的 C2S 字段（`int32@13`/`char@17`/`int32@18`）与 S2C 契约（首 u16=107 + ≥16 B）都已闭环；
4. **作战选择值映射 `0/1/2/4 ↔ 1/2/3/5`** 未取证（§6.2 X3）——P2 首轮可能被拒，属预期。
