# odyssey.hardcore —— 奥德赛强化（副本禁消耗品 + 死亡不可复活 + 怪物血量 ×10）

> mod id `odyssey.hardcore` · v1.0.0 · 作者 DFO 115us · **服务端层 + 客户端层 mod**（要重编服务端）

## 它做什么

只在**奥德赛模式**（DGN 里声明 `[dungeon mode script] arad odyssey` 的副本，运行期就是
`catalog.DungeonDefinition.Odyssey`）内生效，出本/城镇一律不受影响：

| 规则 | 效果 | 施工点 |
| --- | --- | --- |
| 副本内禁用消耗品 | CMD44 一律拒绝：**可以用不了，但可以携带**；城镇照常使用 | `cmd/wireprobe/consume_flow.go` → `odysseyConsumableGate` |
| 死亡不可复活 + **立即回城** | CMD41 三级回退（奥德赛测试额度 → 背包复活币 → CERA 扣费）**整条拒掉**；死亡当场就发 N33 FAIL_CLEAR_DUNGEON 并走回城链（**不等** 10 秒倒计时，业主 2026-10-06） | `cmd/wireprobe/dungeon_revive.go` → `odysseyReviveGate`；死亡侧 `player_death.go` → `deathFailLeave` + `client_dispatch_world.go` case 40 |

两条规则经 `internal/modpolicy` 挂在**既有业务点**上，不新增玩法表、不新增 JSON 内容源
（根 `AGENTS.md` §0.2 与 `server/AGENTS.md` §0 的单一内容真源要求）。

客户端侧本来就一致：奥德赛进图不下发 NOTI1584（`STACKABLE_DUNGEON_LIMIT`），按官方客户端
实测口径"没有这一帧 = 客户端本地禁用全部消耗品"，药品图标本来就是灰的；服务端这道门挡的是
**权威侧** —— 改过的客户端绕过界面也拿不到药。

## 怪物血量 ×10：由 client 层对 `DFO.exe` 的字节补丁实现

业主口径是"奥德赛模式怪物**基础血量** ×10（再叠现有难度缩放）"。取证顺序与结论：

1. **服务端做不到**：服务端根本不下发怪物血量 —— 全仓 `MaxHP|max_hp|CurrentHP` 零命中，怪物包
   （NOTI29 / `protocol.DungeonMonster`）只有 `entity / template / level / rank / team`。
2. **内容层也没有目标可改**（业主指示"走 PVF 层试试"后实测）：
   - 内层 `Script.inner.pvf`（5,650,173 条目）：`.tbl` **0** 个，`commonmonsterbase`/`baseparameter`/
     `difficultybonus` **0** 个；
   - 客户端 14,643 个 NPK / 456,188 条目：`.tbl` **0** 个、血量/难度相关条目 **0** 个。
   ⇒ pvf 层只能改 `Script.pvf`+`sk.dat`、resource 层只能改 NPK，**都没有目标**。
3. **所以只剩 client 层的 `exe.patch`**（框架原生支持），本包就用了它。

### 2026-10-06 重做的定点补丁（当前版本；旧的两点补丁已删除）

旧的「addsd→movsd + 共享常量 1.0→10.0」两点补丁**已被撤销**：它改的是 flag==0 分支，
实测**角色自己变 10 倍血、怪物血量不变**（`0x147220ECF` 的 `jne` 让 flag!=0 直接跳过那两步）。

现行补丁改成**调用侧 trampoline**，一行 getter 都不碰：

| # | 文件偏移（VA） | before → after | 作用 |
| --- | --- | --- | --- |
| 1 | `0x5A06EB6`（`0x145A06EB6`） | `e8 85 9f 81 01`（`call getter`）→ `e9 f0 17 c7 02`（`jmp 0x1486786AB`） | flag==1 调用点 1 改道（**同长度 5 字节**） |
| 2 | `0x5A06F56`（`0x145A06F56`） | `e8 e5 9e 81 01`（`call getter`）→ `e9 50 17 c7 02`（`jmp 0x1486786AB`） | flag==1 调用点 2 改道（**同长度 5 字节**） |
| 3 | `0x86786AB`（`0x1486786AB`） | `cc` × 21 → `4c 8b d1 e8 8d 87 ba fe 48 6b c0 0a 49 8b ca c3` + `cc` × 5 | 16 字节 trampoline：`mov r10,rcx` / `call getter` / `imul rax,rax,10` / `mov rcx,r10` / `ret` |

关键事实（全部实测，详见 `client/PATCH-NOTES.md`）：

* **返回值在 `RAX`**：getter 出口 `0x147221000 F2 48 0F 2C C6` = `CVTTSD2SI RAX,X6`；
  两个调用点的后继都是 `CVTSI2SS X?,RAX`。所以「×10」用 `imul rax,rax,10`，
  不仅够用，还比原来 `CVTSI2SS`（截成 float32）**更精确**。
* **只影响 flag==1**：getter 的 `test bl,bl` / `jne` / flag==0 入口 / `addsd` /
  `mulsd` 以及 `.rdata` 那颗被 25 个点共用的 `1.0` 常量**全部原样**，一个字节没动。
  全镜像只有这 2 处在 `MOV DL,1` 之后调这个 getter，其余 21 处都是 `XOR EDX,EDX`。
* **等长不覆盖相邻指令**：两处 `call→jmp` 都是 5 字节原地替换；洞内只写在 `cc` 填充上。
  （被否决的旧方案 A 想把 `0x147220ECF` 的 2 字节 `75 6F` 改成 5 字节 `E9 rel32`，
  会吃掉 flag==0 入口 `0x147220ED1 48 8D 4F 48` 的前 3 字节 —— 本设计不做任何不等长改写。
  旧方案选的 25 字节洞 `0x143327897` 也已弃用：`0x143327B14` 那条真实 `CALL` 的目标
  `0x1433278A0` 正落在洞里，而 `0x143327B10` 有 200+ 个调用点。）
* **代码洞是干净的**：`0x1486786AB` 前一条是 `0x1486786AA C3`（上一函数 `RET`），
  后一条是 `0x1486786C0 48 89 54 24 10`（下一函数序言）；全 `.text` 线性反汇编 +
  全节分支目标扫描对这段 21 字节**零命中**。
* `sourceSHA256` 门：`01c633dfda5c883ba126261246cb8067b0a0331500587d8062df368da175a836`
  （本机 `DFO.exe`，258,972,712 字节）。EXE 换版本时这个门会先失败，不会盲改。
  该 EXE **没有基址重定位目录**，文件偏移打的补丁在运行期地址稳定。

**已验证到什么程度**（静态，2026-10-06）：把 3 处补丁打到 `DFO.exe` 的**副本**上，与原件
全文件只差 **26 个字节**（5+5+16，正是上表三处）；用 x86 解码器反汇编确认
两处 `CALL` 变成 `JMP 0x1486786AB`、洞内 `IMUL RAX,RAX,0xA` 紧随 `CALL getter`，
而 getter 的 flag 分支逐字节未变。打了补丁的整文件 sha256 =
`e8f1aa283cc54d350f6202dfa54fa3eac6a4dd137a3405c7b8e7fdefeae34bfa`。

**实机效果仍待业主确认**（按根 AGENTS §0 第 6 条，实机由业主操作）。进游戏请专门看两件事：

1. 怪物血量是否确实变厚、幅度多少；
2. 有没有连带把**伤害**一起放大（风险见下）。

### 必须知道的几个风险（业主明确要求不做开关、只做 mod，所以这里如实写清）

- **这是全局补丁，不是奥德赛专属**：改的是那 2 个取值调用点，所有副本走同一入口的怪物都会变厚。
  想只影响奥德赛，需要在模式判据里分流，而该判据在客户端内部、当前取证不足以定点。
- **flag==1 那个函数是 6 分支的伤害/招式辅助函数**（`0x145A06DB0`，被 6 处调用，
  内部还有 `DIVSS`/`SUBSS`/`MULSS` 链，并同时调用 actor 血值访问器 `0x147222570`）。
  把它的 2 个取值点 ×10，**可能连带改变伤害计算**，静态无法排除。
  若实机发现怪物打人也变疼，说明这 2 处不是纯"血量显示"，需要按调用点再收窄。
- **与项目既有硬规矩冲突**：`next37-status.md` §4.1 写着"在根因定死之前不改这个数 —— 项目硬规矩是
  不许随意放大 HP"。本次是**业主明确要求**的玩法改动，特此记录冲突。
- **TheMida 完整性校验**：EXE 带 `.themida` 节，静态无法判定它是否校验 `.text`。
  首次安装后必须实机确认能正常起游戏；若被拦下，用 `modkit uninstall` 一键还原。

### 一键还原

```powershell
modkit uninstall --client C:\Game\dof\115us\DFO --id odyssey.hardcore --root <启动器根>
```

手工还原：把 `0x5A06EB6`/`0x5A06F56` 写回 `e8 85 9f 81 01`/`e8 e5 9e 81 01`，
`0x86786AB` 起 21 字节写回 `cc`，整文件 sha256 应回到 `01c633df…`。详见 `client/PATCH-NOTES.md`。

## 装/卸

```powershell
# 打包（--modkit 会顺手 verify；打包前会逐字节核对 DFO.exe 现场，不符就停手）
python mods\odyssey-hardcore\build-mod.py --modkit <modkit.exe>

# 看计划（必须过 sourceSHA256 门）
<modkit.exe> plan --client C:\Game\dof\115us\DFO --mod "mods\odyssey-hardcore\dist\odyssey.hardcore-1.0.0.zip" --root <启动器根>

# 安装（server 层 mod：装完要重编服务端 —— mod 的 Go 代码是编进服务端二进制的）
<modkit.exe> install --client C:\Game\dof\115us\DFO --mod "mods\odyssey-hardcore\dist\odyssey.hardcore-1.0.0.zip" --root <启动器根>

# 一键还原（含 EXE 字节：按安装时的整文件备份回写）
<modkit.exe> uninstall --client C:\Game\dof\115us\DFO --id odyssey.hardcore --root <启动器根>
```

> **游戏运行时 modkit 拒绝写入客户端**。要装/卸血量补丁，先关掉 `DFO.exe`；
> 打包（`build-mod.py`）不受影响，它只读核对，不改任何文件。

> ℹ️ **它是 server 层 mod，所以历史上受"加载器"那条限制，现在已经解除**：
> 较早的启动器（1.7.7 的 commit `8c87358` 那一版）生成的 `mods/zz_mods_gen.go` 对所有 mod
> 用**默认 import**，同时装 ≥2 个 server 层 mod 会在安装期被拦下并回滚
> （报「生成的 mod 加载器编译失败」；根因 `modpkg redeclared in this block`）。
> **该限制已在启动器仓 commit `0bd67dc`（2026-10-07，版号仍 1.7.7）修掉**：
> ≥2 个 mod 时每条 import 生成按 mod id 的显式别名（`mod_<清洗后的 id>`），**支持多个共存**。
> 依据与两段口径见 [`../MOD-DEVELOPMENT.md`](../MOD-DEVELOPMENT.md) §4.8。
> 只带规则脚本的 mod（`server.script`）一向不受这条限制。

卸载后规则立刻回到"全关"（`internal/modpolicy` 是进程内策略，零值 = 原行为），但**同样要重启服务端**。

## 怎么确认生效（三层证据，逐层收紧）

1. 启动日志 `[mod odyssey.hardcore] 已登记：…` —— `Register()` 被调到了；
2. 启动日志 `[mod odyssey.hardcore] 策略已生效（服务端版本=…）：开启：…` 与
   `odyssey mode rules: 开启：副本内禁用消耗品（可携带） + 禁用复活（复活币/测试额度/CERA 三档） ← odyssey.hardcore`
   —— 策略真的设上去了（这一行由 `cmd/wireprobe/main.go` 在启动装配结束时打印）；
3. 游戏里：奥德赛副本内吃药被拒（库存不减）、**死亡后当场判负回城**（不进 10 秒倒计时、按复活也被拒）。

诊断命令（启动期一次性）：

```powershell
$env:DFO_SERVERMOD_CONSOLE = "odyssey.hardcore what"   # 或 status
```

## 变体与构建开关（2026-10-06 起）

```powershell
python build-mod.py                      # 默认：服务端两条规则 + 血量 ×10 的 EXE 补丁
python build-mod.py --hp-multiplier 3    # 换倍率（写进 trampoline 的 imul 立即数，1..127）
python build-mod.py --rules-only         # 只出服务端两条规则，**不碰 DFO.exe**
python build-mod.py --skip-verify        # 跳过打包前的现场字节核对（不推荐）
```

`--hp-multiplier` 只接受 **1..127 的整数**（`imul r64,r64,imm8` 的立即数上限）；
非整数或超范围会直接报错退出，不会生成一个悄悄不生效的包。

| 产物 | 内容 | 什么时候用 |
| --- | --- | --- |
| `dist/odyssey.hardcore-1.0.0.zip` | server(2 hooks) + **client(exe.patch ×3)** | 默认交付：三条需求全上 |
| `dist/odyssey.hardcore-1.0.0-rules-only.zip` | 只 server(2 hooks) | 血量补丁的连带影响一旦被实机证实（例如怪物攻击也 ×10），先用这个版本；或你暂时不想动客户端 EXE |

两个变体**同 id**，装其中一个之前请先卸掉另一个（`modkit uninstall --client … --id odyssey.hardcore --root …`）。

`--rules-only` 版的作用面：副本内禁用消耗品（CMD44 拒）+ 死亡不可复活并**当场**判负回城（CMD41 三级回退全拒）。
这两条都不改客户端、不碰 EXE，风险面最小。

## 改数值

`server/config.json`（随二进制内嵌，改了要重新打包 + 重装 + 重编服务端）：

```json
{ "ban_consumables": true, "ban_revive_coin": true }
```

两条都设成 `false` 会让服务端**拒绝启动**（`server.boot` 返回 error）—— 装了一个什么都不做的
mod 属于配置错误，不该静默。

## 边界（有意为之）

- **不**碰消耗品的拾取/携带/掉落：只禁"副本内使用"；
- **不**碰 CMD507 族（疲劳药水/扩容券/背景券/幻化栏/胶囊）：那是另一条 opcode，城镇动作为主，
  奥德赛副本内用不到；要一起禁需要单独口径；
- 死亡流程只在**奥德赛 + 规则开**时改：其它模式/规则关时仍是既有的「10 秒倒计时 → 判负回城」；
  判负与回城复用同一份代码（`deathFailLeave`），重复调用是空操作，不会重复发包；
- 判据用的是**副本**的奥德赛标记（`Definition.Odyssey`），与既有复活额度、准入、难度锁同源；
  存档层面的"奥德赛角色"标记（`character.OdysseyRole`）**不**参与判定 —— 奥德赛角色打普通副本
  不受影响，非奥德赛角色也进不了奥德赛副本。
