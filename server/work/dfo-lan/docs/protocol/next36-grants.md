# 36 轮：点券充值 / 金币 / 道具发放

编写时间：2026-09-11。**已实现并在真实库上验证通过。**

这三样是服务端侧的运营发放，背后没有客户端命令，所以不需要协议取证——但需要和任务奖励同级的保证：单事务、幂等键、审计留痕、账号归属。

## 1. 之前的状态

`cmd/admin`、`internal/httpapi`、`internal/account` 三个目录都是空的（架构文档里规划过，从未实现）。具体缺口：

| 项 | 之前 |
|---|---|
| 金币 | 只能靠掉落和任务奖励产生，没有发放入口 |
| 点券 | **完全没有持久化**。SELECT 响应里的 Cash 字段取自 `select-world-probe.json` 的固定 `"cash": 0`，所以无论如何都是 0 |
| 道具 | 只能靠掉落和任务奖励 |
| 审计 | 无 |

## 2. 现在

### 2.1 点券落地

新增账号级账本 `account_currency(account_id, cera, updated_at)`，`cera >= 0` 由 CHECK 约束保证。

点券是**账号级**的，因为客户端是从账号的 SELECT 响应里读它，不是从角色状态。`cmd/wireprobe` 的 SELECT 分支现在从账本读取余额填入 `profile.Cash`（此前恒为配置里的 0），并记 `cera_restored` 事件。**所以充值要重启服务端后、重新选择角色才能看到。**

### 2.2 发放与审计

`admin_grants(grant_id PK, account_id, character_id, request, receipt, operator, reason, created_at)`。

`grant_id` 就是幂等键。实现上有一个关键点：**审计行的插入本身就是幂等闸门**，不是"先查有没有、再插入"——后者在并发下会漏（两个请求都通过检查）。现在用 `INSERT … ON CONFLICT (grant_id) DO NOTHING` 抢占，冲突的那一方会等待持有者的事务结果，因此同一个 id 永远只有一次支付；若持有者回滚，id 重新可用。

这一点是被测试抓出来的：并发用例第一版直接撞了主键冲突。

金币走背包自己的货币路径（`Bag.Add(…, 0, amount)`，与金币掉落同一条），道具走共享的 `inventory.Awarder`——所以背包满会**整笔拒绝**，不会只发一半；源目录里不存在的物品 ID 一律拒绝，不凭空造物品。

### 2.3 命令行

```
admin -account probe -grant-id <唯一键> -reason <原因> [-operator <人>] \
      [-character <id>] [-cera <±数额>] [-gold <数额>] [-item 6003x5,20002]

admin -balance        # 查点券余额
admin -history        # 查发放审计
```

`-grant-id` 与 `-reason` 是必填：前者防重复支付，后者进审计。点券可为负数用于扣回，但扣到负数会**整笔拒绝**而不是截断到 0。金币和道具需要 `-character`；点券是账号级，可以不带角色。

凭据只从 `runtime/storage/local.json` 读取，不打印、不进交付物。

## 3. 真实库验证

对 LanTest01（角色 id=3）实发一笔：

```
-grant-id verify-36-lantest01 -cera 10000 -gold 1000000 -item 6003x5
→ cera_balance 10000, gold 31 → 1000031, 道具 6003×5 进槽位 65
```

原样再跑一次同一条命令：**提示已发放过、返回原收据、点券仍为 10000**（不是 20000），审计只有一行。

这笔发放留在库里了，你测试时应当能看到；不想要可以用 `-cera -10000` 配一个新的 grant-id 扣回。

## 4. 回归测试

`charactercheck` 新增 `GRANT_STORAGE_PASS`，覆盖：

- 同一 grant id 的 **12 个并发请求只支付一次**
- 点券不可被扣成负数，且被拒绝的扣款不改变余额
- 正常扣款生效
- 另一个账号不能对本账号的角色发放
- 源目录里不存在的物品被拒绝
- 只有真正生效的发放留审计行；**被拒绝的发放不留痕，其 id 可重新使用**（也已断言）

## 5. 已知边界

- 在线角色的背包与余额是内存态，发放后需重新选择角色才可见。工具本身会提示这一点。
- `character.State.CurrencySlot2` 是另一个货币槽（走 NOTI37），语义未取证，本轮**没有触碰**。
- 点券上限按线上字段裁到 uint32。
- 发放不产生邮件通知——邮件协议仍未取证（见 `next36-protocol-recovery.md` 第 5 节）。
