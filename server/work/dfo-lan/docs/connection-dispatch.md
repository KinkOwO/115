# Wireprobe 连接与命令分发

2026-10-03 源码候选；已确认运行程序与实机范围保持。

`main.go` 从 4398 行降到 160 行，只保留参数解析、运行依赖获取、监听端口、日志和频道服务启动。每次接入由 `gameGateway.handleClient` 创建独立 `gameConnection`；共享运行依赖来自 `gatewayRuntime`，角色服务的频道 context 按原方式逐连接复制，选角、技能、装备、商城订单、采样和邮件状态属于连接。

| 文件 | 职责 |
| --- | --- |
| `client_connection.go` | 连接初始化、单一读帧/定时循环、报文输出与事件回调 |
| `client_dispatch.go` | 分发结果、固定阶段顺序、类型门禁与 fixture 应答 |
| `client_dispatch_account.go` | 冒险团/邮件/时间查询、统一设置与手柄选项 |
| `client_dispatch_character.go` | 职业与技能、会话切换、角色列表及角色操作 |
| `client_dispatch_inventory.go` | 商城/礼盒、仓库、物品移动、皮肤、装备和消耗品接线 |
| `client_dispatch_world.go` | 特殊副本、军团、副本会话、城镇与任务接线 |
| `client_entry.go` | 原 SELECT/角色入场流程，当前仍保留整体顺序 |

## 顺序与状态契约

21 个命名阶段沿原 `if` 链顺序执行：7 个类型门禁前阶段、类型门禁、13 个命令阶段。`beforeClientTypeDispatch` 与 `commandDispatch` 使用静态方法表达式，不在每帧创建注册表，也不按 opcode 重新排序。

- `dispatchNext`：继续下一阶段；`dispatchHandled`：处理完本帧；`dispatchClose`：结束连接。它们对应原有落到后续分支、外层 `continue` 和 `return`，内层循环/闭包控制保持。
- `clientRequest` 保存当前帧、明文、校验结果和原帧作用域的错误变量；错误与异步回调引用不跨帧共享。`gameConnection` 保存原连接作用域的可变状态。
- 解密/校验与正文采样保持独立，13 字节无正文请求继续校验。定时触发、请求和写入仍沿既有串行循环/输出锁执行。
- 原类型门禁前的物品路径、重复的 CMD1565/CMD2285 分支优先级保持；本轮不改变这些既有条件。角色入场包、账号/角色存档契约和事件键沿用原实现。
- 继承和增幅书接线测试改为执行真实 `dispatch`，验证校验拒绝及流程拒绝/回包；继承的禁止出站包检查跟随实际物品分发文件，不再要求分支写在 `main.go`。

## 验证

全部分发分支在状态字段与控制返回归一化后 33,614 个 token 一致；读帧/定时/解密/校验/采样循环 1,178 个 token 一致，8 个输出与事件回调逐个一致，21 阶段注册顺序一致。两版隔离回环网关各处理 11 条无正文请求，11 个响应帧及 25 条事件一致；12 组 CLI 子进程对照输出与退出码一致。

新增专项覆盖真实登录响应→时钟→频道身份的发送顺序、同时存在的两个频道连接、共享配置/响应保留、正文采样上限、fixture 回落、门禁前处理、重复分支优先级和发送失败退出。使用内存连接或独立临时工作目录，不读取真实 PVF、玩家数据库或启动游戏客户端。Go 1.26.5 下运行：

```powershell
$env:GOTOOLCHAIN = 'go1.26.5'
go test ./cmd/wireprobe -run '^(TestGameGateway|TestClientDispatch|TestWireprobeConfig|TestPrepareRuntime|TestRunGateway|TestTownArrivalScenesValidated|TestRuntimeCleanup|TestConnection|TestChannel|TestInherit|TestAmplifyGrimoire|TestOnlineTimer|TestSendPacketPlan|TestBodySampler|TestMonsterHistoryLog|TestAwakeningPromote|TestComboRequests)' -count=1
go test ./internal/archtest -count=1
go vet ./...
go build -trimpath -o ../../../.tmp/dispatch/wireprobe-dispatch.exe ./cmd/wireprobe
```

上述专项、架构守卫、vet 和编译通过；按用户要求未运行全量测试。独立候选在根目录 `.tmp/dispatch/`，未替换运行程序。后续可继续拆解 SELECT 入场流程；本轮保持其原有事务、诊断、资源和报文顺序。
