# next54：缓冲（奶系）直升胶囊使用无效果 —— CMD507 第 5 个字段被通用零校验拒死

> 状态：**2026-10-06 19:04 实机收口（业主确认 OK）**。现役 `bin/wireprobe-pvf.exe` = `2a3962e6a7f2ba32…`
> （30,187,008 B），上一版（2639 行宽修复）备份在 `.tmp/capsule-buffer-20261006/baseline-3b0160a5.exe`。

## 1. 症状

同账号第 4 个角色（`qweqwe`，profession 4，Lv1，赛利亚房间）右键使用
**Sky of a Thousand Seas Starter Boost Capsule「[Buffer Only]」**（奶系专用：
Enchantress / Crusader(M) / Paramedic / Crusader(F) / Muse），确认框输入
`Use on this character` 后**没有任何效果**：不掉等级、不进教学、不弹 2638/2639。

同账号第 3 个角色（`asdasdasd`，profession 11）在同一会话里用**普通**胶囊一切正常。

## 2. 实机取证（`runtime/roles_persist_..._20261006_183702_434609_next37`）

| 时间(UTC) | 角色 | 事件 | 关键内容 |
| --- | --- | --- | --- |
| 10:38:39 | 3 | `boost_gift_inventory` | 67 格 = 模板 **590015870**（CMD14 行 `0001 0043 00 7eed2a23 01`） |
| 10:38:49 | 3 | C2S 507 | `4300 00 00000000 51010000 00000000 …` ⇒ **p[11:15] = 0** |
| 10:38:49 | 3 | `boost_capsule_consumed` + 2638 + 2639 | 直升 + 教学正常 |
| 10:41:36 | 4 | `boost_gift_inventory` | 67 格 = 模板 **590015871**（`7fed2a23`） |
| 10:41:44 | 4 | C2S 507 | `4300 00 00000000 51010000 `**`01000000`**` …` ⇒ **p[11:15] = 1** |
| 10:41:44 | 4 | **`boost_capsule_refused`** | `reason = "unsupported stackable action fields"` |

两帧只差 p[11]，其余 63 字节完全相同 ⇒ 拒绝原因不是槽位、容器、动作号，而是
**通用 CMD507 校验把 p[11:15] 当成必须为 0 的保留字节**。

## 3. 源侧（PVF 直读，`server/work/client-build/Script.inner.pvf`）

| 模板 | `[action type]` | 职业限制 | 说明 |
| --- | --- | --- | --- |
| `stackable/590015001/590015870.stk` | `[boost up mode capsule] 0 10017` | `[usable job] [all]` | 普通胶囊，变体 0 |
| `stackable/590015001/590015871.stk` | `[boost up mode capsule] `**`1`**` 10017` | `[possible job and grow type] [priest] 1 [at priest] 1 [at gunner] 5 [mage] 5 [archer] 1` | 缓冲（奶系）胶囊，变体 1 |
| `stackable/590015001/590015877.stk` | 无 `[action type]`，`[attach type] [trade]` | — | 关卡奖励物品，**不是胶囊**（与 `capsules=2` 一致） |

`live/event/kor/2026/0326_boostup/boostup.evt`：
`[buffer job and grow] 3 5 5 5 14 1 16 1`、`[dual job and grow] 4 1`
⇒ `BufferEligible` 的并集含 `{4,1}`；角色 4 是 `profession=4 / advancement=1`，
解码放行后资格门禁不会二次拒绝（已按 `characters.state` 核对）。

## 4. IDA 闭环（权威 IDB `client/DFO.exe.i64`，工作副本 `.tmp/exp37/`）

发包链按 `analysis/dumps/CLIENT-MECHANICS.md` §2 定位：`sub_146D74000` 取 writer →
`sub_146D746E0(w, opcode)` 写 opcode。脚本 `ida_cmd507_sender.py` 扫该 helper 的全部调用点，
筛出立即数 507 的函数；`ida_cmd507_sender_level2.py` 取二级函数。产物
`analysis/dumps/cmd507-sender/`。

动作 337 分支 `sub_14143F7B0`（`sender_sub_14143f7b0.c:153-176`）按固定宽度写正文：

| 偏移 | 写手（已确认宽度） | 值 |
| --- | --- | --- |
| `+0` | `sub_146D76180` = **u16** | 背包格（v25） |
| `+2` | `sub_146D75CC0` = **u8** | 容器/空间（v34） |
| `+3` | `sub_146D75CE0` = **u32** | 0 |
| `+7` | `sub_146D75CE0` = u32 | 动作 = 337（`sub_14500E540` 读模板 `+2048`） |
| **`+11`** | `sub_146D75CE0` = u32 | **`sub_1421B2820(itemArg)` = `*(u32*)(itemArg+856)`，且客户端要求 `<= 1` 才发帧** |
| `+15` | `sub_146D75CE0` = u32 | 0 |
| `+19` | `sub_146D75B10` | 40 B 零 |

合计 **59 B**（传输层补零到 64），与两帧实测完全一致。⇒ p[11:15] 是**动作参数向量里的第一个参数**，
不是保留字节；它取 0/1 恰好等于源里 `[boost up mode capsule]` 的第一个参数（普通 0 / 缓冲 1）。

## 5. 改动

`internal/game/protocol/boostup_frames115.go`：`DecodeBoostCapsule115` 不再复用通用
`DecodeStackableAction` 的「其余字节必须为 0」，改为胶囊自己的字段掩码
（`+0..2`、`+7..14` 是字段，其余仍必须为 0；长度仍只接受 59/64；动作号仍硬校验 337）。

**不放松通用解码器**：药水 54、皮肤仓 169、幻化栏 101/197、飞艇 206 的零校验保持原样，
避免把未知动作当成合法帧。变体判定仍按模板在源里的变体号（`c.Capsules[模板].Variant`），
**不吃包里的 p[11:15]**，所以这一格填什么都不影响结果，只是不再成为拒绝理由。

## 6. 验证

- 新增 `TestDecodeBoostCapsule115BufferVariant`：两帧实机正文都能解出**完全相同**的请求
  （槽 67 / 容器 0）；通用 `DecodeStackableAction` 仍拒绝缓冲帧（证明是分流不是放水）；
  动作号错、长度 58、p[15] 非零三种情况仍被拒；59 字节原生长度可解。
- `go build ./...` 退出 0；`go vet ./...` 干净；`go test ./... -count=1` 全绿（无失败项）。
- 候选已覆盖现役 `bin/wireprobe-pvf.exe`（`2a3962e6a7f2ba32…`）。
- 实机（`runtime/roles_persist_..._20261006_190433_177502_next37`，业主确认 OK）：C2S 507 明文
  `43000000000000510100000100000000…`（p[11]=1）被接受 → 角色 4 `boost_capsule_consumed` →
  2638 `0001010001` → 2639 四行（32 B = 4+7×4）→ 680 关卡领取；`boost_capsule_refused` 计数 0。

## 7. 未闭环

1. `itemArg+856` 的**写入方**未定位（客户端在哪一步把 0/1 放进物品对象）。当前结论不依赖它：
   本树不消费该字段。
2. 665 挑战 row1/2/3 仍不计数（缺「副本 → 内容号」真源）；官服 2722 尾部字段语义未解；
   第二关副本地图箱按业主判定搁置。
