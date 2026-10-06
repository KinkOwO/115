# odyssey.hardcore —— 奥德赛强化

> mod id `odyssey.hardcore` · v1.0.0 · 作者 DFO 115us · **服务端层 + 客户端层 mod**（server 层要重编服务端）
>
> **在奥德赛副本内**：怪物血量 ×10、怪物伤害 ×10、禁用消耗品、死亡不可复活、掉落 ×5。
> **普通副本 / 普通角色完全不受影响。**

## 它做什么

只在**奥德赛模式**（DGN 里声明 `[dungeon mode script] arad odyssey` 的副本，运行期就是
`catalog.DungeonDefinition.Odyssey`）内生效，出本/城镇/普通副本一律不受影响：

| # | 规则 | 效果 | 施工点 | 在哪一层 |
| --- | --- | --- | --- | --- |
| ① | 怪物血量 ×10 | 奥德赛副本内四层血量基础值 ×10（保住已受伤比例） | `DifficultyRules.dll` 按 `dungeonIds` 命中后写内存 | **client** |
| ② | 怪物伤害 ×10 | 物攻(`desc+0x398`) / 魔攻(`desc+0x3b8`) 同一倍率 ×10 | 同上 | **client** |
| ③ | 禁用消耗品 | CMD44 一律拒绝：**可以用不了，但可以携带**；城镇照常使用 | `cmd/wireprobe/consume_flow.go` → `odysseyConsumableGate` | server |
| ④ | 死亡不可复活 | CMD41 三级回退（奥德赛测试额度 → 背包复活币 → CERA 扣费）**整条拒掉**，走既有死亡超时/立即判负回城流程 | `cmd/wireprobe/dungeon_revive.go`；死亡侧 `player_death.go` | server |
| ⑤ | 掉落 ×5 | 世界掉落 + 普通小怪专属物品池两档概率各 ×5 | `modpolicy.ConfigureDrops` → `internal/loot/modpolicy_drops.go` | server |

服务端两条规则经 `internal/modpolicy` 挂在**既有业务点**上，不新增玩法表、不新增 JSON 内容源
（根 `AGENTS.md` §0.2 与 `server/AGENTS.md` §0 的单一内容真源要求）。

客户端侧本来就一致：奥德赛进图不下发 NOTI1584（`STACKABLE_DUNGEON_LIMIT`），按官方客户端
实测口径"没有这一帧 = 客户端本地禁用全部消耗品"，药品图标本来就是灰的；服务端这道门挡的是
**权威侧** —— 改过的客户端绕过界面也拿不到药。

---

## 一、客户端两层：插件 + 本 mod 自带的范围规则

`client` 层是**两条 `file.add`**（**没有** `exe.patch`）：

| kind | target | 内容 |
| --- | --- | --- |
| `file.add` | `.115us-mods/DifficultyRules.dll` | 客户端难度引擎（宿主插件通道，`requires: ["qol.client-host"]`） |
| `file.add` | `.115us-mods/rules.d/odyssey.hardcore.json` | 本 mod 自带的**范围规则**：只对奥德赛副本，`percent`/`attackPercent` = 1000 |

所以**用户只装 `odyssey.hardcore` 一个 mod**，客户端侧的血量与伤害倍率就完整到位：
引擎（DLL）与范围（rules.d 里的 JSON）都在包里，不需要用户自己去写任何规则。

### 为什么能做到"只对奥德赛"，而以前的 EXE 补丁做不到

旧版是这个 mod 早期用 `exe.patch` 改 `DFO.exe` 的 HP getter 调用点（3 处、26 字节），
**它是全局的** —— 改的是两个取值调用点，所有副本走同一入口的怪物都会变厚，
与业主"普通角色不受影响"的要求直接冲突；它还要整文件备份/还原、撞 TheMida 完整性风险。

现在的插件按**副本 id** 命中（`dungeonIds`），所以范围是可控的。取证顺序与结论（2026-10-06）：

1. **服务端做不到**：服务端根本不下发怪物血量 —— 全仓 `MaxHP|max_hp|CurrentHP` 零命中，怪物包
   （NOTI29 / `protocol.DungeonMonster`）只有 `entity / template / level / rank / team`。
2. **内容层也没有目标可改**：内层 `Script.inner.pvf`（5,650,173 条目）里 `.tbl` **0** 个，
   `commonmonsterbase`/`baseparameter`/`difficultybonus` **0** 个；客户端 14,643 个 NPK /
   456,188 条目里 `.tbl` **0** 个、血量/难度相关条目 **0** 个。
3. ⇒ 倍率只能在**客户端进程内**做，而插件通道（`qol.client-host`）提供了"不碰 EXE"的做法。

插件纪律（`client-patchs/AGENTS.md` §1）：**不覆盖 `DFO.exe`、不落 `dinput8.dll`、无 inline 钩子、
日志只写自己目录、指纹不符不接管、写前逐字节核对现场**。对不上就跳过并记日志（宁失效不崩）。

### 范围规则的来源（`dungeonIds` 是怎么来的）

用真实内层 PVF（`server/work/client-build/Script.inner.pvf`，
checksum `b2b503b58ca12ed88befaa2d44b6d222cfb15e94aaff7a7dfa32f6fa0133b13e`）
跑一遍只读解析，取 `contents/2026/aradodyssey/**/*.dgn` 里解析出
`catalog.DungeonDefinition.Odyssey == true` 的**全部**副本：

* 共 **56** 个（源里 3,231 个副本定义）；
* 区间 `100004934..100004957` 与 `100004959..100004990`
  （`100004958` 在内层 PVF 里没有对应的 `aradodyssey` DGN，所以不在列表里）；
* 覆盖 `00_mirkwood` … `54_continental_drift_6` 与 `final_seria`
  （奥德赛五域 + 大陆漂流 + 终章），每个都带 `designatedDifficulty=2`。

完整清单在 `client/rules.d/odyssey.hardcore.json`；这里**没有** `all: true`，
不在列表里的副本插件一条规则都不命中，保持原版。

### 插件怎么合并多份规则文件（决定"谁赢"）

插件按固定顺序读**两处**规则：

1. `<客户端>\.115us-mods\rules.d\*.json` —— 按**文件名升序**遍历；
2. `<客户端>\.115us-mods\rules.json` —— **最后**读。

每个文件内部仍是"第一条 `enabled` 且命中的规则生效"；跨文件是**先遍历到的文件里命中就赢**。
⇒ 本 mod 在 `rules.d` 里的奥德赛规则在奥德赛内**赢过**玩家 `rules.json` 里的通用规则，
其它副本仍按玩家的规则（或原版）。

**为什么这样定优先级**：`rules.json` 是**玩家的**文件，`rules.d` 是**随 mod 分发的**文件。
若让玩家的通用规则（例如"全副本 ×2"）先命中，mod 作者声明的范围就被打穿，
而 mod 作者既不能预期也不能修（他改不了玩家的文件）。让 mod 自带文件先于玩家文件
= "更具体的规则优先"，且不需要引入任何新字段。

`rules.d` 里**单个文件解析失败只跳过它自己**（日志里一行 `[跳过]`），不影响其它文件；
玩家 `rules.json` 缺失/读失败/解析失败仍是旧语义（整份配置不可用 → 还原并停止接管）；
`rules.d` 目录不存在时与旧版行为完全一致。

细节与"为什么不用 EXE 补丁了"写在 [`client/PATCH-NOTES.md`](client/PATCH-NOTES.md)。

---

## 二、配置键（`server/config.json`）

`server/config.json` 随二进制内嵌（`go:embed`），**改了要重新打包 + 重装 mod + 重编译服务端**
（mod 的 Go 代码是编进服务端二进制的；只改客户端侧的 `rules.d` 才只需重启客户端）。

```json
{
  "ban_consumables": true,
  "ban_revive_coin": true,
  "world_drop_percent": 500,
  "monster_item_percent": 500
}
```

| 键 | 类型 | 语义 | 单位 | 默认 / 缺字段 | 兼容别名 |
| --- | --- | --- | --- | --- | --- |
| `ban_consumables` | bool | 奥德赛副本内禁止**使用**消耗品（CMD44 拒；可携带；城镇不受影响） | — | 缺字段 = `true`（本 mod 的语义就是强化） | 无 |
| `ban_revive_coin` | bool | 奥德赛副本内禁止复活（CMD41 三档全拒：奥德赛测试额度 / 背包复活币 / CERA） | — | 缺字段 = `true` | 无 |
| `world_drop_percent` | uint32 | **世界掉落**概率倍率 | **100 = 1.00 倍**（500 = 5 倍） | 缺字段或 `0` = **不改变**（沿用服务端环境变量 `DFO_ORDINARY_WORLD_DROP_PERCENT` 的既有值） | `worldDropPercent` |
| `monster_item_percent` | uint32 | **普通小怪专属物品池**概率倍率 | 同上 | 缺字段或 `0` = **不改变**（沿用 `DFO_ORDINARY_MONSTER_ITEM_DROP_PERCENT`） | `monsterItemPercent` |

补充说明：

* **键名兼容**：掉落两键的第一版用的是 camelCase。两个键同时出现时以 **snake_case 为准**，
  只写旧键的老配置照旧生效（`server/mod.go` 的 `loadConfig`）。
* **掉落倍率的向后兼容语义**：缺字段或写 `0` = **不改变**，绝不会因为这份配置没写新字段
  就把掉率变成 0。布尔规则相反 —— 缺字段按"开"兜底（这是强化 mod）。
* **两条布尔规则不能同时为 `false`**：那样等于"装了个什么都不做的 mod"，
  `server.boot` 会返回 error 让服务端**拒绝启动**（配置错误不该静默）。
* 掉落倍率的消费点在 loot 的**取用处**（`internal/loot/modpolicy_drops.go` 的世界掉落 /
  小怪专属池），每次抽取现读一遍 —— boot 之后立刻生效，不需要重建目录。

### 客户端侧的数值（另一处，不走 config.json）

`percent` / `attackPercent`（血量 / 伤害倍率，同样 **100 = 1.00 倍**）写在
`<客户端>\.115us-mods\rules.d\odyssey.hardcore.json`。

* 想换倍率：**改这个 JSON，重启客户端**即可（插件每 200 ms 快照一次配置，内容是热读的）；
* 想换范围：改 `dungeonIds`（`all: true` = 全部副本，**本 mod 不用它**）；
* 这个文件是 mod 落的位，`modkit uninstall` 会把它一起删掉；
  插件自己生成的玩家文件是 `rules.json`（`enabled: false` 模板），卸载时**不动**它。

---

## 三、生效自证（三层证据）

### 1. 服务端启动日志（三行，缺一不可）

```text
[mod odyssey.hardcore] 已登记：奥德赛模式规则（副本内禁用消耗品 + 死亡不可复活）+ 掉落倍率（来自 config.json），boot 时生效
[mod odyssey.hardcore] 策略已生效（服务端版本=…）：开启：副本内禁用消耗品（可携带） + 禁用复活（复活币/测试额度/CERA 三档） ← odyssey.hardcore
odyssey mode rules: 开启：副本内禁用消耗品（可携带） + 禁用复活（复活币/测试额度/CERA 三档） ← odyssey.hardcore
drop rate rules: 世界掉落 ×5.00 + 小怪专属池 ×5.00 ← odyssey.hardcore
```

* 第 1 行 = `Register()` 被调到了；
* 第 2、3 行 = 策略真的设上去了（由 `cmd/wireprobe/main.go` 在启动装配结束时打印）；
  第 3 行是 `odyssey mode rules:` 这一行本身；
* 第 4 行 `drop rate rules: …` 是掉落倍率的自证（数值来自 `config.json`，
  缺字段时显示"不改变"）。

> 装了这个 mod 但两条布尔规则都关掉 → 服务端**拒绝启动**并指出原因，不会静默跑一个空 mod。

### 2. 客户端（`<客户端>\.115us-mods\`）

看 `difficulty-rules.log` 与 `difficulty-rules.status.json`：

```text
==== DifficultyRules（副本难度 · 怪物血量倍率）启动 ====
客户端原生指纹校验通过（14 段逐字节 + 2 项旧钩子存在性检查）
规则已加载：共 2 份文件、N 条规则（rules.json 哈希 0x…；rules.d 指纹 0x…、1 个 .json）
  <odyssey.hardcore.json> enabled=1 共 1 条
    [0] id=odyssey-hardcore-10x percent=1000 (10.00 倍) attackPercent=1000 (10.00 倍) 副本数=56 …
  <rules.json> enabled=0 共 1 条
副本 100004934 命中规则 odyssey-hardcore-10x（…，来自 odyssey.hardcore.json）→ 血量 10.00 倍 / 伤害 10.00 倍
```

以及 `difficulty-rules.status.json`：

* `ready: true`（指纹通过）、`enabled: true`（有可用规则）；
* `ruleFiles` / `rules`（加载了几份文件、几条规则）、`ruleFile`（命中规则来自哪份文件）；
* 奥德赛副本内 `percent` / `attackPercent` = 1000；**普通副本里规则不命中，
  `ruleId` 为空、`percent` = 100** —— 这就是"普通角色不受影响"的直接证据。

指纹不符（换了客户端构建）时 `ready: false` + `rejectReason: clientFingerprintMismatch`，
插件一个字节都不改。

### 3. 游戏里

* 奥德赛副本内：怪物明显变厚、打人更疼；吃药被拒（库存不减，界面本来就是灰的）；
  死亡后复活被拒、走判负回城；
* 普通副本/城镇：血量伤害与原版一致，吃药正常、复活正常。

### 诊断命令（启动期一次性）

```powershell
$env:DFO_SERVERMOD_CONSOLE = "odyssey.hardcore what"    # 或 status
```

---

## 四、兼容性（必读）

### ⚠️ 与 `difficulty.rules` 互斥（有意为之）

| mod | client 层投的文件 |
| --- | --- |
| `difficulty.rules`（通用难度 mod） | `.115us-mods/DifficultyRules.dll` |
| `odyssey.hardcore`（本 mod） | `.115us-mods/DifficultyRules.dll` **+** `.115us-mods/rules.d/odyssey.hardcore.json` |

两个都装时 modkit 会把计划里的 DLL 那一步判为
「**目标已被 mod difficulty.rules 占用**」并**阻断**安装。

这是**故意的**：同一个难度引擎只允许一个实例 —— 双份加载会把倍率叠成 ×100。
**二选一**：只装 `odyssey.hardcore` 就只要奥德赛那套（血量 ×10 + 伤害 ×10 都按副本 id 命中）；
只装 `difficulty.rules` 就按玩家自己的 `rules.json` 来（含 `all: true` 的全局规则）。

换用的顺序（**先关游戏**）：

```powershell
<modkit.exe> uninstall --client C:\Game\dof\115us\DFO --id difficulty.rules --root C:\Game\dof\115us\115
<modkit.exe> install   --client C:\Game\dof\115us\DFO --mod "mods\odyssey-hardcore\dist\odyssey.hardcore-1.0.0.zip" --root C:\Game\dof\115us\115
```

`rules.json`（玩家的通用规则）两者共用，换 mod **不需要动它**；
只装本 mod 时它是 `enabled: false` 的模板，不会与 `rules.d` 里的奥德赛规则打架。

### 客户端的玩家规则文件 `rules.json`（中性模板）

`<客户端>\.115us-mods\rules.json` 是**玩家自己**的规则文件，插件首次运行时会生成一份
`enabled: false` 的模板（带两条示例规则，都是 `enabled: false`）。
想自己加强某些副本就在这个文件里改（字段与单位见下），改完重启客户端即可：

```json
{
  "schema": 1,
  "enabled": false,
  "rules": [
    { "id": "example-mid-dungeon", "name": "示例：指定副本 10 倍血量",
      "enabled": false, "dungeonIds": [100005014], "percent": 1000, "attackPercent": 100 },
    { "id": "example-all-dungeons", "name": "示例：全部副本 2 倍血量 10 倍伤害",
      "enabled": false, "all": true, "dungeonIds": [], "percent": 200, "attackPercent": 1000 }
  ]
}
```

* `enabled`（顶层）= 这份文件的总开关；规则自己的 `enabled` 缺省 = **停用**（默认安全）；
* `all: true` = 全部副本（**本 mod 不用它**）；也可写 `dungeonIds`；
* `percent` / `attackPercent` 单位 **100 = 1.00 倍**，合法范围 1..100000；
  省略 `attackPercent` = 100（不改伤害）；
* 一条规则既没 `all: true` 也没有 `dungeonIds` → 整份配置判不可用（必须能判定作用范围）。

---

## 五、装 / 卸

```powershell
# 打包（默认完整包；--modkit 会顺手 verify）
python mods\odyssey-hardcore\build-mod.py
python mods\odyssey-hardcore\build-mod.py --rules-only

# 看计划（必须 0 阻断；DLL 那一步被 difficulty.rules 占用就会阻断，见 §四）
<modkit.exe> plan    --client C:\Game\dof\115us\DFO --mod "mods\odyssey-hardcore\dist\odyssey.hardcore-1.0.0.zip" --root C:\Game\dof\115us\115

# 安装（server 层 mod：装完要重编服务端 —— mod 的 Go 代码是编进服务端二进制的）
<modkit.exe> install --client C:\Game\dof\115us\DFO --mod "mods\odyssey-hardcore\dist\odyssey.hardcore-1.0.0.zip" --root C:\Game\dof\115us\115

# 一键还原（client 层两条 file.add 逐字节还原；server 层删目录并重写 import 清单）
<modkit.exe> uninstall --client C:\Game\dof\115us\DFO --id odyssey.hardcore --root C:\Game\dof\115us\115
```

> **游戏运行时 modkit 拒绝写入客户端**。要装/卸血量/伤害那一层，先关掉 `DFO.exe`；
> 打包（`build-mod.py`）不受影响，它只读核对，不改任何文件。

> ⚠️ **它是 server 层 mod，所以受"加载器"那条限制**：当前已发布版启动器（**v1.7.7**，仓
> `115us-dfolauncher` HEAD `8c87358`）生成的 `mods/zz_mods_gen.go` 对所有 mod 用**默认 import**，
> 所以**同时装 ≥2 个 server 层 mod 会在安装期被拦下并回滚**
> （报「生成的 mod 加载器编译失败」；根因是 `modpkg redeclared in this block`）。
> **下一版起**（启动器仓工作区那条**尚未提交/发布**的修复）会给每条 import 生成按 mod id 的
> 显式别名，届时支持多个共存。依据与两段口径见
> [`../MOD-DEVELOPMENT.md`](../MOD-DEVELOPMENT.md) §4.8。
> 只带规则脚本的 mod（`server.script`）不受这条限制。

卸载后规则立刻回到"全关"（`internal/modpolicy` 是进程内策略，零值 = 原行为），但**同样要重启服务端**。

## 六、变体与构建开关

```powershell
python build-mod.py                 # 默认：server（③④⑤）+ client（插件 + 奥德赛范围规则）
python build-mod.py --rules-only    # 只出 server 层，不碰客户端（不含 DLL / rules.d）
```

| 产物 | 内容 | 什么时候用 |
| --- | --- | --- |
| `dist/odyssey.hardcore-1.0.0.zip` | server(2 hooks) + **client(2× file.add)** | 默认交付：五条需求全上 |
| `dist/odyssey.hardcore-1.0.0-rules-only.zip` | 只 server(2 hooks) | 你暂时不想在客户端装插件时；或只想在服务端禁药/禁复活 + 掉落 ×5 |

两个变体**同 id**，装其中一个之前请先卸掉另一个
（`modkit uninstall --client … --id odyssey.hardcore --root …`）。
`--rules-only` 变体**不含** `client/DifficultyRules.dll` 与 `client/rules.d/odyssey.hardcore.json`，
也不声明 `requires: qol.client-host`（用不到宿主通道就不挂这条依赖）。

### `build-mod.py` 的硬依赖

完整包在打包时会**强制检查**这两个源文件，缺一个就报错退出（不会生成一个
"装上去但没有倍率"的包）：

1. `<仓库根>\client-patchs\difficulty\dist\DifficultyRules.dll`
   —— 先跑 `client-patchs\difficulty\build-dll.cmd` 生成；
2. `<本目录>\client\rules.d\odyssey.hardcore.json`
   —— 本 mod 的范围规则（随包分发，`dungeonIds` 见 §一）。

DLL **不入库**（`.gitignore` 里 `*.dll` 被忽略），打包时从上面的 dist 路径取并算 size/sha256。

## 七、边界（有意为之）

* **不**碰消耗品的拾取/携带/掉落：只禁"副本内使用"；
* **不**碰 CMD507 族（疲劳药水/扩容券/背景券/幻化栏/胶囊）：那是另一条 opcode，城镇动作为主，
  奥德赛副本内用不到；要一起禁需要单独口径；
* 死亡流程只在**奥德赛 + 规则开**时改：其它模式/规则关时仍是既有的「10 秒倒计时 → 判负回城」；
  判负与回城复用同一份代码（`deathFailLeave`），重复调用是空操作，不会重复发包；
* 判据用的是**副本**的奥德赛标记（`Definition.Odyssey`），与既有复活额度、准入、难度锁同源；
  存档层面的"奥德赛角色"标记（`character.OdysseyRole`）**不**参与判定 —— 奥德赛角色打普通副本
  不受影响，非奥德赛角色也进不了奥德赛副本；
* 客户端倍数只作用在**怪物**上（血量四层基础值 + 当前血量、物攻/魔攻两项原生整数属性）；
  其余白名单偏移（移速/攻速/硬直/物防/魔防）本版**刻意不启用**。
