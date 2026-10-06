# 奥德赛模式强化（odyssey.hardcore）—— 取证、实现与验证

> 日期：2026-10-06 · 业主口径（本轮答复）：**基础血量 ×10（再叠现有难度缩放）**、
> **副本内禁用任何消耗品（可携带，城镇不受影响）**、**死亡不可复活（复活币与金币复活都禁，直接退出副本/回城）**；
> 交付 = **只做一个 mod 包，需要的服务端支持代码一起做并提交，mod 包不提交**。

## 1. 结论速览

| 需求 | 结论 | 落点 |
| --- | --- | --- |
| 副本内禁用消耗品 | ✅ 服务端可做（权威侧） | `cmd/wireprobe/consume_flow.go` → `odysseyConsumableGate` |
| 死亡不可复活 + 立即回城 | ✅ 服务端可做（一处挡住三级回退；死亡当场判负回城，不等 10 秒） | `cmd/wireprobe/dungeon_revive.go` → `odysseyReviveGate`；`player_death.go` → `deathFailLeave`；`client_dispatch_world.go` case 40 |
| 怪物基础血量 ×10 | ❌ **服务端无通路**，需走内容（PVF）/客户端路线，待业主选 | 见 §4 |

服务端侧的策略入口是新增的 `internal/modpolicy`：server 层 mod 在 `server.boot` 里设置，
**默认全关**（不装 mod = 行为与装之前一致），生效状态由 `cmd/wireprobe/main.go` 打进启动日志。

## 2. 取证（只读，file:line 锚点）

### 2.1 消耗品（CMD44 = `ENUM_CMDPACKET_USE_STACKABLE`）

- 分发：`cmd/wireprobe/client_dispatch_inventory.go:946-965` → `worldState.useStackable`
- 处理：`cmd/wireprobe/consume_flow.go:17`；已有前置门 `forestPotionGate`（苏醒之森每关 8 次）
- 领域校验：`internal/inventory/consumable.go:70-87`（list 只允许 0/7）、`consume.go:18-66`
  （`Kind != "stackable"` 即拒）；**不区分** `[waste]/[material]/[quest]`
- 客户端总闸：NOTI1584 `STACKABLE_DUNGEON_LIMIT`（`internal/game/protocol/stackable_limit.go:5-18`），
  官方客户端实证"**没有这一帧 = 客户端本地禁用全部消耗品**"；奥德赛进图本来就**不发**它
  （只在苏醒之森发 limit=8，`dungeon_flow.go:414-418`）⇒ 界面本来就是灰的，服务端这道挡权威侧
- 拒绝形状：`protocol.UseStackableRefused(r)`（`use_stackable.go:69-73`）

### 2.2 复活（CMD41 = USE_COIN）

- 分发：`client_dispatch_inventory.go:136-157`；错误统一转成
  `booster_action_refused_ack` + `boosterActionRefusal(41)` = `protocol.Refusal(22)`（`dungeon_revive.go:206-211`）
- 三级回退：`dungeon_revive.go:172-188` = 奥德赛测试额度 `pilotRevive` → 背包复活币
  `lifeTokenRevive`（虚拟槽 template1/slot1）→ CERA 扣 15（`lifeTokenCeraCost`）
- 既有校验：`lifeTokenReviveAllowed`（`dungeon_revive.go:28-56`，含源标签 `[cannot use coin map]`）、
  `pilotReviveAllowed`（`odyssey_revive.go:73-101`）
- 死亡侧：CMD40 `player_death.go:16` → `client_dispatch_world.go:386-434` 的 10 秒
  `deathFailTimeout` 补 N33 FAIL_CLEAR + 回城 ⇒ **禁掉复活即等价"死亡直接判负回城"**，无需改死亡流程
- 奥德赛判据：`catalog.DungeonDefinition.Odyssey`（DGN `[dungeon mode script] arad odyssey`，
  `internal/catalog/dungeons.go:287-288`），与准入/难度锁/复活额度同源

### 2.3 怪物血量（关键缺口）

- **服务端不下发怪物血量**：全仓 `MaxHP|max_hp|CurrentHP` 零命中；怪物包只有
  `entity/template/level/rank/team`（`internal/game/protocol/dungeon.go:141-159`、`:232-246`，
  注释明说 record+40 是 team「not HP」）
- 血量由**客户端**按 PVF 算：`Monster/CommonMonsterBaseParameter.tbl`（每级一行，idx5=HP）+
  难度乘子（`MonsterApcDifficultyBonus.tbl`、`SeasonMonsterDifiicultyBonus.tbl`、
  `.dng [monster difficulty bonus]`、`StoryModeDifficultyBonus.tbl`）
- **现役客户端已把难度缩放 getter 打掉**：`server/work/build_required.py:42-46` +
  `server/reference/analysis-tools/_hppatch.py`（`0x147220F48` `0F86B2000000` → `E9B300000090`）
  ⇒ 业主口径里的"再叠现有难度缩放"在当前客户端上未必成立
- 唯一沾 HP 的服务端代码是只读诊断 `-scale-death-from-hp`（`cmd/wireprobe/config.go:123`）

## 3. 实现

### 3.1 服务端支持（入本次提交）

| 文件 | 作用 |
| --- | --- |
| `internal/modpolicy/modpolicy.go`（新） | 奥德赛规则策略：`OdysseyRules{BanConsumables, BanReviveCoin, Source}`、`Configure/Odyssey/Reset`；开启规则必须写 `Source`（避免无主规则）；**零值 = 全关** |
| `internal/modpolicy/modpolicy_test.go`（新） | 默认全关 / 必须写来源 / 设置-回显-撤销 往返 |
| `cmd/wireprobe/odyssey_gate.go`（新） | `odysseyModeActive()`、`odysseyConsumableGate`（CMD44 拒绝）、`odysseyReviveGate`（CMD41 拒绝）、`logModPolicy()` |
| `cmd/wireprobe/consume_flow.go`（改） | 在 `forestPotionGate` 之后接入消耗品门 |
| `cmd/wireprobe/dungeon_revive.go`（改） | 在 `useCoinRevive` 三级回退**之前**接入复活门 |
| `cmd/wireprobe/main.go`（改） | 启动装配结束时打印 `odyssey mode rules: …`（server/AGENTS §6 的"不许静默"） |
| `cmd/wireprobe/player_death.go`（改） | 把 case 40 里 10 秒定时器的动作抽成 `deathFailLeave(reason)`（N33 FAIL_CLEAR + `leaveDungeon` + 清会话），并新增 `deathFailTimeoutReason=100`；**奥德赛禁复活时 reason=0 立即调用**（业主 2026-10-06「死亡即回城」） |
| `cmd/wireprobe/client_dispatch_world.go`（改） | case 40：`odysseyImmediateDeathFail()` 成立 → 立即 `deathFailLeave(0)`；否则照旧 arm 10 秒定时器。判负/回城只有一份代码，重复调用空操作 |
| `cmd/wireprobe/odyssey_rules_test.go`（新） | 门的行为矩阵：规则关放行、奥德赛副本拦、非奥德赛不拦、只开一条不误伤、nil/无副本不 panic |

### 3.2 mod 包（**不入库**，交付给业主分享）

`mods/odyssey-hardcore/`（作者工作区源码）+ `dist/odyssey.hardcore-1.0.0.zip`：

- `mod.json`：schema 2、id `odyssey.hardcore`、`permissions: [server.hook]`、
  `layers.server.hooks = [server.boot, console.command]`
- `server/mod.go`（package `modpkg`）：`Register()` 问 `servermod.Enabled`；`boot()` 调
  `modpolicy.Configure`；两条诊断命令 `status` / `what`
- `server/config.json`（go:embed）：`{"ban_consumables":true,"ban_revive_coin":true}`；
  两条都关会让服务端拒绝启动（装了个什么都不做的 mod 属配置错误）
- `build-mod.py`：固定时间戳可复现打包 + 从源码推导钩子/权限 + `--modkit` 顺手 verify

## 4. 血量 ×10 的三条候选（待业主选）

1. **PVF 基础血量表 ×10**：唯一真能改血量的点，但**全局**（所有副本变厚），与"只奥德赛"不符；
2. **只改奥德赛 `.dng` 的 `[monster difficulty bonus]`**：最有希望只影响奥德赛，但该段列语义
   **未取证**，且若走的就是被 patch 掉的 getter 则可能完全无效 —— 建议先做这一条的取证；
3. **客户端 EXE getter 补丁**：改动小但全局，且属根 `AGENTS.md` §0 需要业主**明确授权**的客户端补丁路线。

（禁止：把血量倍率写进服务端 JSON/policy —— §0.2 与 server/AGENTS §0 的单一内容真源。）


### 4.1 内容层取证（业主指示「走 PVF 层试试」后实测，2026-10-06）

**结论：怪物血量表不在任何可改的内容归档里，pvf / resource 层都无目标可改。**

| 扫描对象 | 规模 | 结果 |
| --- | --- | --- |
| 内层 `Script.inner.pvf`（= 客户端 `Script.pvf` 剥壳，761,764,363 B） | **5,650,173** 条目 | `.tbl` **0** 个；`commonmonsterbase*`/`baseparameter`/`difficultybonus`/`monsterapc` **0** 个；只有 236 个无关的 `etc/115lvability/equipmentsetpointtable/addparameter`。`monster/` 下 70 万条目形如 `monster/act8/<name>/action`（**不带扩展名**） |
| 客户端全部 NPK（`ImagePacks2` + `SoundPacks`，用框架自带的 `internal/modkit/npk.go` 读索引） | **14,643** 个归档 / **456,188** 条目 | `.tbl` **0** 个；血量/难度相关条目 **0** 个 |

⇒ mod 的 `pvf` 层只能改 `Script.pvf` + `sk.dat`，`resource` 层只能改 NPK；两者都**没有**血量表这个目标。
附带缺口：仓库 Go 侧**没有 PVF 写入器**（`internal/catalog/pvf` 只有 `UnwrapOuter` 剥壳）；
回封（每 10 MiB 前 0x2800 字节 AES-256-CBC，密钥来自 `DFO.exe` 内嵌 RSA 私钥解 `sk.dat`）的算法
只存在于参考 Python `server/work/build_required.py`，未纳入 mod 安装管线。

⇒ HP ×10 剩下**唯一**的 mod 路线是 `client` 层的 **`exe.patch`**（改客户端 EXE 的血量计算）：
框架支持、项目已有同类先例（`build_required.py` 就在 patch HP getter `0x147220F48`），
但要把"×10"对准需要 IDA 级定位（本机没有 IDA；`docs/protocol/next37-status.md` 自述 HP 公式未闭环）。

### 4.2 顺带修掉的 modkit 真 bug（关键路径）

装第二个**服务端** mod 时，生成的 `mods/zz_mods_gen.go` 把两个包都按约定包名 `modpkg` 裸 import
⇒ `modpkg redeclared`，**服务端编不过**（安装时的静态验证是逐包编译，拦不住）。修法（启动器仓）：

- `internal/modkit/support.go`：生成文件改为**唯一别名** `modpkg0/modpkg1…`，并在每条 import 后写 mod id 注释；
- `internal/modkit/install2.go`：静态验证在逐包编译之后**追加聚合包** `go build ./mods`
  （就是 `zz_mods_gen.go` 所在的包，服务端正是通过它的 `RegisterMods()` 拉起所有 mod）；
- `internal/modkit/layer2_test.go`：断言改为别名化 import/调用 + 不得出现裸 import。

端到端复验：两个服务端 mod 同时安装 → `go build ./...` = 0；`Register() → servermod.Boot()` 日志
`[mod odyssey.hardcore] 策略已生效：…` ✔。

## 5. 验证（都在隔离 worktree：基线 = 合并提交 `bc3a8db9`）

> 为什么用隔离树：主工作树里有**其它会话未完成的半成品**
> （`internal/dungeon/raid_boss.go`、`raid_stage.go` 引用了不存在的 `Rank`/`Bakal`），
> 全模块 `go build ./...` 在主树里必然失败，与本任务无关。

- `go build ./...` = **0**、`go vet ./...` = **0**
- `go test ./internal/modpolicy/ -count=1` = **ok**（3/3 PASS）
- `go test ./cmd/wireprobe/ -run 'Odyssey|Revive|Consume|Stackable|Forest'` = **ok**
- 全量 `go test ./... -count=1` = **44 包 ok / 0 FAIL**（基线 43 包 + 新包 1）
- mod 包：`build-mod.py --modkit` → **`modkit verify` 通过**（3976 B，
  sha256 `bbf57a2c…`，permissions `server.hook`，hooks `server.boot, console.command`）
- 新增测试：`TestOdysseyImmediateDeathFail`（判据矩阵）与
  `TestOdysseyDeathFailLeaveSendsFailAndReturnsToTown`（立即发 FAIL_CLEAR + 回城链 + 清会话 + 重复调用空操作）
  —— 均 PASS；既有死亡/失败/回城测试（`boss_flow_test.go` 等）不受影响；
- **mod 端到端**（隔离树里把 mod 落位 + 重写 `zz_mods_gen.go`）：`Register()` → `servermod.Boot()`
  → 日志 `[mod odyssey.hardcore] 策略已生效（服务端版本=verify）：开启：副本内禁用消耗品（可携带） +
  禁用复活（复活币/测试额度/CERA 三档） ← odyssey.hardcore`，断言 PASS

## 6. 未闭环 / 风险

- **血量 ×10 未实现**（§4，等业主选路线）；
- 死亡即回城已按业主要求做成**当场判负**（`deathFailLeave(0)`），不再等 10 秒；其它模式仍是既有 10 秒倒计时；
- CMD507 族（疲劳药水/扩容券/背景券/幻化栏/胶囊）**未纳入**消耗品禁令（另一条 opcode，
  以城镇动作为主）；若要一并禁需单独口径；
- 实机未验证（根 AGENTS §0 第 6 条：实机由业主操作）；
- 判据只认**副本**的奥德赛标记（`Definition.Odyssey`），存档层的"奥德赛角色"
  （`character.OdysseyRole`）不参与 —— 奥德赛角色打普通副本不受影响。
