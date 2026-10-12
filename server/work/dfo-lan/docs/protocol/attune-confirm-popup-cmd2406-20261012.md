# 大深渊「选套装·部位」确认弹窗 —— CMD2406 应答接线（2026-10-12）

## 现象（业主报的那句话就是范围）

副本内弹出「Confirm Selected Set/Part」（Part: Accessory / Set: Hideout's Endless Gold，
OK / Reselect 两个按钮），**点 OK 之后弹窗不消失**。这是在副本里的事件触发弹窗，不是城镇开箱子，
也不是礼包/战令的选项盒（前一轮按盒子查过，已排除）。

## 根因

服务端对上行 **CMD2406 `BOUNDARY_OF_ATTUNEMENT_HIDDEN_SELECT`**（就是那一下 OK）**一帧都不回**。
而这个窗的关闭路径**只有**服务端那条回信能走到 —— 客户端的 OK 回调只发不关。

## L0 证据

**实机会话**（2026-10-12）：`04:30:18` 与 `04:42:57` 各有一条 `id=2406, type=1`（CMD 总线）、
正文 16 字节 `02000000 02000000 00000000 00000000`，服务端零应答。同一会话里所有礼包流
（CMD160）正常，说明不是链路或加密问题，是这条命令没接。

**客户端侧（IDA 9.2，权威 IDB 的私有副本；反编译工件在 `analysis/dumps/attune-select/`）**：

1. OK 按钮回调 `sub_1406B30C0`（窗口类 3843 `FortifiedDiscipleOfDoomConfirmWindow` 的
   vtable `off_1492EBF08`+16）**只发包**，发完把「已提交」闩 `win+1816` 置 1 防重复提交；
   函数体内没有任何关窗调用。⇒ 点 OK 之后窗口能消失的唯一途径是收到服务端回信。
2. 唯一入口是注册在 **CMD 表 `qword_14E6836F8`**（注册器 `sub_14599D4D0`）上的同号处理器
   `sub_1406B1860(unsigned __int8 flag, unsigned short)`。
   分发器 `sub_1459A1BB0` **只在信封 kind==1 时**才查这张表（kind==0 走通知表
   `qword_14E683700`）⇒ 回信必须 `kind=1, id=2406`，发成 noti 等于没发。
3. 分发器在把包交给处理器**之前**替处理器读掉体首：`v55[0]=0; sub_146EA09F0(v55,1)` 先读 1 字节
   flag，且**只在 flag==0** 时再用 `sub_146EA1920(&Time_1)` 读一个 u16。这两个值才是处理器的实参
   ⇒ 正文至少 3 字节，短了会让分发器 over-read 到包外。
4. flag 取 **0**：处理器先关旧实例（`sub_1406B4440` → 窗口 vtable+264，要求 `win+1816==0`），
   再**无条件**跑类级收尾 `sub_146694510(mgr, 3843, 1)`。非 0 只跑收尾。0 覆盖两条路。
   类级收尾链是 `sub_146694510 → sub_146694390(mgr, win, r9) → vtable+736`。
5. 这三条命令/通知都属于同一个 240 字节「调律模块」实例，其 init `sub_1406B1520`
   （工件 `module_init_1406B1520.c`）里注册了 noti 2756/2859 与 cmd 2343/2406，
   并把副本 100005068 绑进来 —— 与「大深渊/调律之边界」对上。

## 改了什么

| 文件 | 改动 |
| --- | --- |
| `cmd/wireprobe/attunement_reward.go` | 新增 `attuneConfirmAckBody = {0,0,0}` 与 `answerAttuneConfirm`：回一帧 `kind=1, id=2406, 正文 00 00 00`，并把上行正文原样记日志。上行那两个 u32 是 onOpen 从窗口 `+1768/+1772` 回读的值，语义未取证 ⇒ 只记日志、不参与判定、不据此发放任何东西。 |
| `cmd/wireprobe/request_scope.go` | 2406 同时进 `dungeonRequest`（`dispatchDungeon` 大 switch 的守卫）和 `observedGameRequest`（否则被正文采样门 BodySampleLimit=8 掐掉，`verified` 拿不到）。只登前者会重演实机 2026-10-04 那轮 `case 1654` 永不执行。 |
| `cmd/wireprobe/client_dispatch_world.go` | `dispatchDungeon` 的 switch 里 `case 2406: plan = client.worldState.answerAttuneConfirm(...)`；`e` 保持 nil ⇒ 走通用加密发送，不会套上 `protocol.Refusal`。 |
| `cmd/wireprobe/attunement_reward_test.go` | `TestAttuneConfirmReplyReachesTheCmdHandler`：钉两道门 + 恰好一帧 + `Kind==1/ID==2406` + 正文≥3 且首字节为 0 + 对任意上行正文（含空）**无条件**回。 |

没有加任何开关：机制已闭环 ⇒ 直接回。诊断通道只留给语义未定的取值（noti 2859 的三个 u32）。

验证：`go build ./...`、`go vet ./cmd/...`、`go test ./cmd/wireprobe/`（42.8s，全绿）。

## 实机验收判据

进大深渊（100005068）触发该弹窗，点 OK。日志应出现两行：

```
attune confirm (cmd 2406): body=02000000020000000000000000000000
{"kind":"attune_confirm_ack","id":2406,...}
```

- 弹窗消失 ⇒ 这条线接通，本轮验收通过。
- 弹窗仍在 ⇒ 上面两行是否都出现是第一位的信息：只有一行 = 路由/发送断了；两行都有 =
  收尾链那个第 4 个实参不对，下一轮只换正文形状（flag / u16 的组合），不猜别的包。

**静态定不下来的一处**（所以要一次实机来判）：提交后 `win+1816` 闩已置 1，显式关闭
`sub_1406B4440` 会被闩挡住，真正生效的是类级收尾链 `sub_146694510 → sub_146694390(mgr, win, r9)
→ vtable+736`，而它的第 4 个实参在调用点上不是常量，反编译读不出来。

## 同一条线上仍未接的三处（别当成已完成）

1. **CMD2343**：与 2406 同一个模块注册、同样「发了不回」的形状，尚未接。
2. **noti 2859**（`BOUNDARY_OF_ATTUNEMENT_REWARD`，12 字节 = 3×u32）：默认永不发送，
   只留了诊断通道 —— 三个字段语义没有结论，按「禁止猜包」不默认发。
3. 选完套装后的**奖励物品是否由服务端发放**：未定。

## 对仓内旧文档的更正

`analysis/tasks/next176-大深渊动画接线缺口与掉落错位根因-20261007.md`：

- §Q3 猜 2406 正文是「6 字节（u32+u16）」⇒ 实测 **16 字节**，含两个有意义的 u32。
- 该文暗示 `sub_1406B1860` 是发送 functor ⇒ 它是**接收处理器**（分发器替它读掉 flag 与 u16）。

取证工件（不随本包交付，按路径引用）：`analysis/dumps/ida_attune_select1.py` … `6.py`、
`analysis/dumps/attune-select/**`（probe1..6 的 JSON、各函数反编译 `.c` 与 `.asm.txt`、`.log`）。
