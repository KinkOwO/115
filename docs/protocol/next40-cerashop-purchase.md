# DFO 本地服 — 对接工作记录：商城购买（CMD64）协议取证与服务端闭环

> 面向后续维护与协议对接。记录 115 级客户端实机 live capture 与 IDA 逆向推导，
> 作为商城购买功能（CMD64 `ENUM_CMDPACKET_BUY_CERASHOP_ITEM`）的权威协议标准。

---

## 1. 协议操作码与语义

| Opcode | 宏名称 | 客户端语义 |
| --- | --- | --- |
| CMD64 (0x0040) | `ENUM_CMDPACKET_BUY_CERASHOP_ITEM` | 玩家在商城（Cera Shop）确认购买购物车商品 |

- **历史问题定位**：客户端点击购买确认后一直卡在 `Processing your purchase... Please wait.` 弹窗。根因为服务端未处理 CMD64，客户端等待应答超时挂起。
- **与 CMD1302 澄清**：历史文档曾将购买误判为 CMD1302；CMD1302 实为登录期 32 字节 UTF-16LE 账号名上报，与商城购买无关。
- **本地余额前置检查**：客户端在发出 CMD64 之前通过 `sub_1408829C0` 进行本地点券充足性检查（账号点券 - 总价 >= 0 才允许发包）。

---

## 2. 请求报文格式（Live Capture + IDA 标定）

发送端为 `sub_146A6ABF0`（购物车结算组包），由 `sub_146A6F8B0` 调用并置窗口为「购买中」状态：

```text
u8  flag                 // 窗口字段 +21748（观察值 0）
u8  mode                 // 模式：1 => 后跟优惠码 wstring (u32 len + bytes) + u8
[u32 len + bytes + u8]   // 仅当 mode == 1
u8  count                // 购物车条目数
count × {
  u8  field36            // 物品结构 +36（观察值 0）
  u8  field72            // 物品结构 +72 低字节（观察值 0）
  u32 goodsId            // 商品目录 ID（如 3000126）
  u32 amount             // 购买数量（原生组包写入常量 1）
  u8  opt1Count, opt1Count × (u32, u8)       // 时装选项表 1 (sub_145F58E70)
  u8  opt2Count, opt2Count × (u32, u16, u16) // 时装选项表 2
}
[trailing bytes]         // 尾随字节（实机含 1 个 0 字节，解码层宽容容错）
```

**实机 live 捕获明文（16 字节）**：
`00 00 01 00 00 3e c7 2d 00 01 00 00 00 00 00 00`

- `flag = 0`
- `mode = 0`
- `count = 1`
- 条目：`field36 = 0`, `field72 = 0`, `goodsId = 0x002DC73E`（3000126），`amount = 1`，`opt1Count = 0`, `opt2Count = 0`，尾随 1 字节 0。

---

## 3. 响应报文格式（IDA 逆向证实）

客户端分发器先读 `u8 成功标志`（`sub_146EA09F0`）。若标志为 0 再读 `u16 错误码`（`sub_146EA1920`）；随后调用 `sub_145267D40` 消费具体分支体。

### 3.1 成功响应（49 字节，成功标志 1 + 分支体）

逐字段读取序列（`sub_145267D40`）：

```text
+00: u8  1 (成功标志)
+01: u8  v56 (子标志，未用，填 0)
+02: u32 v66 (分类目录过滤器，填 0xFFFFFFFF 表示全目录查找)
+06: u32 v63 (goodsId，匹配待结算条目并授予所有权)
+10: u32 v65 (结算比较基准，填 0)
+14: u32 v58 (结算比较基准，填 0，使 v65 == v58 触发完整解锁分支)
+18: u32 v64 (未使用，填 0)
+22: u16 count (结果行数，当前填 0)
+24: u32 v62 (仅当 v65==v58 时读取，填 0xFFFFFFFF 跳过 UI 结果行调用)
+28: u32 v67 (仅当 v65==v58 时读取，填 0)
+32: u32 v60 (所有者条目类型过滤器，填 0xFFFFFFFF)
+36: u32 v68 (未用，填 0)
+40: u32 hi(v61) (高半部，填 0)
+44: u32 lo(v61) (已结算商品数 settled，必须等于购买量，如 1)
+48: u8  v57 (所有者条目标记，填 0)
```

**冻结破局核心**：

1. `lo(v61)` 即已结算数必须等于购买数量（1），客户端 `CeraShopManager::onRecvPurchase` 匹配待结算表后清除记录并从购物车移除。
2. `v65 == v58` 成立，触发读取 `v62`、`v67` 并调用 `sub_1408874E0` -> `sub_1408873C0` 释放 Processing 处理中弹窗锁。

### 3.2 失败响应（20 字节）

```text
+00: u8  0 (失败标志)
+01: u16 code (错误码，小端)
+03: u8  v56 (0)
+04: u32 v58 (0)
+08: u32 hi(v61) (0)
+12: u32 lo(v61) (0)
+16: u32 v60 (0)
```

错误码定义与客户端 dstr 对照：

- `1`：`CeraShopFailMalformed`（dstr 264: Transaction information error.）
- `11`：`CeraShopFailNotEnoughCera`（dstr 258: Not enough CERA.）
- `20`：`CeraShopFailUnavailable`（dstr 253: Unavailable for purchase.）
- `25`：`CeraShopFailInternal`（dstr 263: Billing server error.）

---

## 4. 商品目录数据（真源）

- 真实商品目录来源于客户端 `Script.pvf` 中的 `etc/(r)cerashop.etc`。
- 通过 `server/work/dfo-lan/scripts/export_cerashop_catalog.py` 从解包文本导出 `configs/cerashop.json`。
- 共解析出 9 个核心商品区段（`item`, `item second`, `creature`, `item etc`, `avatar dye`, `package related` 等），共计 **1362** 个有效商品。
- 关键抽查验证商品：`3000126` = `Fatigue Recovery Potion(30)`，单价 50 点券，template 10000541，数量 1。

---

## 5. 存储架构与数据库事务

采用纯增量 DDL，保持存档向前兼容：

```sql
CREATE TABLE IF NOT EXISTS cerashop_owned(
  account_id bigint NOT NULL REFERENCES accounts(id),
  goods_id   bigint NOT NULL,
  template   bigint NOT NULL,
  owned      bigint NOT NULL DEFAULT 0 CHECK(owned >= 0),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (account_id, goods_id)
);

CREATE TABLE IF NOT EXISTS cerashop_purchases(
  id          bigserial PRIMARY KEY,
  account_id  bigint NOT NULL REFERENCES accounts(id),
  goods_id    bigint NOT NULL,
  template    bigint NOT NULL,
  amount      bigint NOT NULL,
  unit_price  bigint NOT NULL,
  total_price bigint NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);
```

扣点与购买在单一事务内闭环：

1. `account_currency` 扣减点券（带有 `cera >= totalPrice` 的乐观互斥锁，余额不足直接回滚并返回 `ErrInsufficientCera`）。
2. `cerashop_owned` 累加玩家已购条目数量。
3. `cerashop_purchases` 写入审计流水。
4. 提交事务，返回更新后的余额与已购数量。

---

## 6. 服务端接入与流控

1. `cmd/wireprobe/request_scope.go`: 添加 64 至 `observedGameRequest` 白名单，保证不被采样丢弃。
2. `cmd/wireprobe/main.go`:
   - 增加 `-cerashop-catalog` 命令行参数（默认 `configs/cerashop.json`）。
   - 启动期调用 `s.MigrateCeraShop(ctx)` 与 `cerashop.LoadCatalog(...)`。
   - 在 CMD44 之前接入 `frame.ID == 64` 分支，单商品完成扣款与 ACK 发送，多商品返回礼貌拒绝（`CeraShopFailMalformed`）。
   - **商城内模态与背包刷新冲突规避**：客户端在商城全屏界面（CEF / UI 模态）下，主城物品管理器与槽位指针处于非活跃状态。此时若直接下发 NOTI 14（`UPDATE_ITEM_LIST`），客户端底层 `sub_1452E9810` 会触发 `0xC0000005` 内存访问越界崩溃，导致弹窗永久冻结在 `Processing your purchase... Please wait.`。因此，CMD 64 购买时仅响应 CMD 64 S2C ACK 解锁弹窗，将背包同步标记为 `pendingInventorySync = true`，待玩家关闭商城返回城镇并发出移动包（`CMD 35` / `CMD 36` / `CMD 623`）时，在城镇上下文平滑下发全量 NOTI 14 同步背包，彻底避免卡死并实现出商城即入包。
