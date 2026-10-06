# 副本难度（怪物血量 + 怪物伤害倍率）— mod id `difficulty.rules`

## 它做什么

把**启动器内嵌 GM**（`115us-dfolauncher/gm`）里那套「副本难度 → 怪物血量/伤害倍率」的
字段执行逻辑搬进客户端进程。进图后按规则文件里命中的副本 id，对场景里的怪施加：
**四层血量基础值 + 当前血量**一起缩放。

- **血量倍率**（`percent`）：把怪物的**四层血量基础值 + 当前血量**一起缩放，保住已受伤比例。
- **怪物伤害倍率**（`attackPercent`）：把怪物的**物攻（`desc+0x398`）与魔攻（`desc+0x3b8`）**
  按同一倍率缩放；这两个是**受保护 32 位原生整数**（`stored=(v+4)^0x1f2a025c`，
  `guard=stored+v+0xc4`），写入时逐字节核对、连 `guard` 一起重算。

- **只做血量 + 伤害**。移速 / 攻速 / 物防 / 魔防 / 硬直恢复 / 视野 / 好战 / 技能冷却 /
  攻击等待 / 命中抗性全部**没有实现**（源码文末 TODO 列了偏移白名单，刻意不开）。
- **不改 `DFO.exe` 文件**，不做 `exe.patch`。
- **不改任何函数字节**，不做 inline 钩子、不换虚表。
- **不落位 `dinput8.dll`**（那会顶掉汉化槽位）。本 DLL 是
  [`qol.client-host`](../client-host/HOST-README.md) 宿主的插件，落在
  `<客户端>\.115us-mods\DifficultyRules.dll`。
- 全程**只读写内存**，**不调用任何游戏代码**（不当"调用方"，避免和游戏主线程抢状态）。

## 它怎么活

```
<客户端>\ChineseLocalization.dll          ← 宿主（mod qol.client-host）
<客户端>\.115us-mods\DifficultyRules.dll  ← 本插件（mod difficulty.rules）
<客户端>\.115us-mods\rules.json           ← 规则文件（插件首次运行自己生成）
<客户端>\.115us-mods\difficulty-rules.log ← 日志（超过 4 MB 自动改名为 .log.old）
```

`ModStart()` 返回 0，随后起一个自己的线程每 **200 ms** 轮询一次：

1. 逐字节核对客户端原生指纹（14 段代码 + 2 项"旧钩子存在性"检查）；
   **任何一段不符 → 拒绝接管**，一个字节都不改。
2. 读规则文件 → 定位场景 → 取副本 id → 匹配第一条命中规则。
3. 应用/还原血量与怪物伤害。

## 规则文件：`.115us-mods\rules.json`

首次运行自动生成，**默认 `enabled: false`**（不偷偷给玩家加强）。

```json
{
  "schema": 1,
  "enabled": false,
  "rules": [
    {
      "id": "example-mid-dungeon",
      "name": "示例：指定副本 10 倍血量",
      "enabled": false,
      "dungeonIds": [100005014],
      "percent": 1000,
      "attackPercent": 100
    },
    {
      "id": "example-all-dungeons",
      "name": "示例：全部副本 2 倍血量 10 倍伤害",
      "enabled": false,
      "all": true,
      "dungeonIds": [],
      "percent": 200,
      "attackPercent": 1000
    }
  ]
}
```

| 字段 | 说明 |
| --- | --- |
| `schema` | 固定 `1`（只认这一个） |
| `enabled` | 总开关。`false` = 全部保持原版 |
| `rules[].enabled` | 单条开关 |
| `rules[].all` | `true` = 作用于**全部副本** |
| `rules[].dungeonIds` | 命中的副本 id 列表（可用数字或字符串） |
| `rules[].percent` | **血量**倍率，**百分之一倍**：`100` = 1.00 倍，`1000` = 10 倍，`200` = 2 倍（与 GM 侧 `gmrules` 口径一致） |
| `rules[].attackPercent` | **怪物伤害**（物攻+魔攻）倍率，同口径：`1000` = 10 倍。**省略时默认 `100` = 不改伤害**（老规则文件原样可用） |

- **顺序即优先级**：第一条 `enabled` 且命中的规则生效（与 GM 侧 `Impact()` 相同）。
- `rules[].enabled` **省略时默认 `false`**（与 GM 侧 `hpMode` 默认"跟随源"同义：不改）。
- 写了 `enabled: true` 却既没有 `all: true` 又没有 `dungeonIds` → 解析失败（不猜范围）。
- `percent` / `attackPercent` 只有 `enabled: true` 的规则会校验，**超出 `1..100000`（含 `0`）→ 整份配置判为不可用**
  （与旧版 `percent` 同一策略）：插件会**还原已改过的字段并停手**，日志里写明是哪条规则的哪个字段超范围。
  **不会**"用一半"（例如只放大血量、不放大攻击）。停用（`enabled: false`）的规则不校验。
- 规则数上限 32（`gmrules.MaxRules`），单条 `dungeonIds` 上限 64 项。
- 改完**存盘即生效**（插件 200 ms 内重载，按"时间戳/大小/内容哈希"三重判定），不用重启游戏。
  规则文件整体删除 → 插件会**重新生成默认文件**（`enabled: false`）并保留原版。

### 行为边界（有意如此）

- **进图后才有怪**：本插件只在战斗场景里动手；大厅/选人/未加载场景一律不碰。
- **规则文件损坏或删除** → 立刻把**已经改过的怪还原成原版**，并停止接管；
  文件修好后自动恢复接管。这与 GM 侧 DLL 执行器的行为一致。
- **只认它自己改过的怪**：本插件为每个怪记住"原始值 / 上次写入值 / 当前倍率"，
  现场值一旦被别的机制改动（原值 ≠ 上次写入值），就**跳过这个怪**、不再碰它。
- **保住"已受伤比例"**：倍率变化时当前血量按 `新上限/旧上限` 同比缩放，
  不是只乘基础值（只乘基础值会让当前血量和上限脱钩 —— 怪会瞬死或无敌）。
- **写前写后都验身份**：节点 key / control / actor 引用 / 虚表 / 虚表
  `+0x12c8` 五条全部复核；任何一条变化就放弃（并回滚已写的部分）。
- **伤害字段逐字节核对**：物攻 / 魔攻是 `stored + guard` 对，读出来先验 `guard = stored + v + 0xc4`；
  对不上就当"这不是我们要的字段"跳过并记日志，绝不硬写。
- **还原是"写回原值"不是"再乘一次反比"**：每个字段都记着 original / applied / percent 三态，
  规则关闭、倍率改回、配置损坏、退出副本时**写回原值**（原值本来就是整数，往返无损）。

### 2026-10-07 修复：血量倍率一直"找到了怪但一个字节都没写"

旧版在算"改后有效上限"时，又拿**新**的基础值调了一次 `hp_layer()`；而 `hp_layer()` 会核对
`DecryptAttr64(*desc) == base`，那一刻内存里还是**旧**值，于是**每一只怪**都在这里被静默丢弃：
现象就是 `tracked` 涨、`monsters` 有值，但 `applied` 恒为 `0`、`failed` 也恒为 `0`、日志一条写入记录都没有。
修法与 Go 侧一致（`monster_runtime.go:349-368`）：只读一次 `hpLayer(old)`，改后用**同一份审计结果**
调 `maxima(new_base)`。自测里的"假内存集成"用例就是照着这个坑写的。

## 日志

`.115us-mods\difficulty-rules.log`（**插件自己所在目录**，遵循 AGENTS §0 第 9 条）。
宿主日志在客户端根 `client-host.log`。

日志里能看到：指纹校验结果、每条规则的加载情况、当前副本 id、命中的规则、
每个怪被改/被还原时的 `层0 旧→新` 与 `当前血量 旧→新`、每个属性字段的 `层0 旧→新`、以及所有跳过原因。
超过 4 MB 自动改名为 `.log.old`。

**实体链诊断**（2026-10-07 加）：每当"链的状态"变化（找不到场景根 / 收下的怪数量变了 /
过滤原因变了 / 副本变了），会打一段 `── 实体链诊断 ──`，里面有：

- `root`、`root->vtable[0xd8]`（期望 `0x144ed9c60`）、`scene`、副本 id；
- `scene+0x108` 的 control 与 **control+8 的计数**（为 0 会明确写"遍历在这里就停了"）；
- `scene+0x110` 的 manager、`manager+0x78` 的哨兵、`*(哨兵)` 的首节点；
- **遍历到的节点数**、收下数、以及 `key>>16 != 3` / `control==0 或 ref<=0x10030` /
  虚表读失败 / 身份校验失败各挡掉多少；
- **身份校验失败分布**：卡在第几步（1 `node+0x10` 低 32 位 key、2 `node+0x20` control、
  3 `control+8` 计数、4 `node+0x28 == actor+0x30`、5 `*actor == vtable`、
  6 `vtable+0x12c8 == 0x145c14140`）；
- **前 3 个节点的** `node / key / control / ref / actor / vtable` 与各自的判定结论；
- 停止原因（走完 / root 为空 / getter 不符 / 计数为 0 / 环路 / 超过 512 项…）。

状态不变时**不会重复打**，正常接管后日志保持干净。

另外每 5 秒（或状态变化时）会写一份
`.115us-mods\difficulty-rules.status.json`（给启动器/GM 读的机器可读状态：
`ready` / `enabled` / `rejectReason` / `dungeonId`（数字或 `null`） / `ruleId` /
`percent` / `attackPercent` / `monsters` / `applied` / `failed` / `tracked` /
`attackApplied` / `attackFailed` / `attackTracked` / `lastError`）。
（旧版有副本 id 时会落盘成 `"dungeonId": ,` —— 整份不是合法 JSON，已修。）

## 怎么开 / 怎么关

1. 装宿主：`modkit install --client <客户端根> --mod client-host-1.0.0.zip`。
2. 装本 mod：`modkit install --client <客户端根> --mod difficulty-1.0.0.zip`。
3. 进图前把 `rules.json` 的 `enabled` 改成 `true`，把要用的规则 `enabled` 改成 `true`。
4. **临时关**：把 `rules.json` 的 `enabled` 改回 `false`（存盘后插件会立刻还原并停手）。
5. **彻底卸载**：`modkit uninstall --client <客户端根> --id difficulty.rules`。
   规则文件与日志不会被 mod 框架删除，需要的话自己删。

### ⚠ 卸载后内存里的倍率要**重启游戏**才彻底消失

宿主（`qol.client-host`）用 `GetModuleHandleExW(… PIN)` **只加载不卸载**，
所以卸载 mod 只是删掉了磁盘上的 DLL，**已经加载进进程的那份代码还活着**：

- 卸载 → 磁盘上 DLL 没了 → 但进程里的插件线程还在跑，还会继续按**内存里最后加载的
  那份规则**改血量（`rules.json` 还在的话就按它继续）。
- 想要立刻停止影响：先把 `rules.json` 的 `enabled` 改成 `false`
  （插件会立刻还原它改过的怪并停止接管），**再退出游戏**。
- 想要内存完全干净：**退出并重启客户端**。

## 换客户端版本会怎样

**会失效，但不会崩。** 所有写死的地址（RVA / 全局变量 / 偏移）都先过
`monster_runtime.go:82-121` 那套逐字节指纹校验：

- 镜像基址不是 `0x140000000`（本机 `DFO.exe` 的 PE `ImageBase` 实测值，
  且未置 `DYNAMIC_BASE`）→ 直接拒绝接管；
- 14 段代码里任何一段不符 → 拒绝接管，日志里打印**期望字节 vs 现场字节**；
- 现场存在旧试验钩子（`0x146e922e0` 属性写入原语 / `0x147220fe6` HP 夹取补丁）
  → 拒绝接管（说明旧 GM 实时执行器在同一个进程里干活，两边不能并存）；
- 单个怪的身份链（5 条）在写前写后各验一次，任何一条不符就跳过并回滚。

也就是说：**换了客户端，最坏结果是"这补丁不生效"，不是"游戏崩"**。

## 构建

```powershell
# 1) 编 DLL（x64 / MSVC / 静态 CRT）
client-patchs\difficulty\build-dll.cmd
#    等价于：
#    cl /nologo /LD /O2 /MT /W3 /utf-8 src\difficulty-rules.c ^
#       /Fe:dist\DifficultyRules.dll ^
#       /link /INCREMENTAL:NO /NOIMPLIB /DEF:src\DifficultyRules.def

# 2) 打包成 schema 2 mod
python client-patchs\difficulty\build-mod.py
```

产物：

- `dist\DifficultyRules.dll`（导出 `ModStart` / `ModName`）
- `dist\difficulty-1.0.0.zip`（mod 包，`dist/` 不入库）

## 自测（都不启动游戏）

```powershell
# 1) 纯逻辑自测：把从 Go 搬过来的算法在本进程里跑一遍
#    （属性加解密 / 原生 float 解码 / scaleHP / maxima / rescaleEffectiveHP /
#     rules.json 解析边界 / 规则优先级）。退出码 0 = 全过。
client-patchs\difficulty\selftest\build-selftest.cmd

# 2) 插件 ABI 冒烟：按宿主的方式 LoadLibraryW → GetModuleHandleExW(PIN)
#    → ModName → ModStart，验证导出、返回值与"日志写在插件自己目录"。
client-patchs\difficulty\selftest\smoke-test.cmd

# 3) 包校验（在启动器仓库里跑）
go run ./cmd/modkit verify --mod <...>\dist\difficulty-1.0.0.zip
go run ./cmd/modkit plan --client C:\Game\dof\115us\DFO --mod <...>\dist\difficulty-1.0.0.zip --root C:\Game\dof\115us\115
```

## 为什么必须动客户端（服务端替代不了）

怪物血量存在**客户端**的 actor 对象里，服务端下发的属性包只表达"进入副本时
按什么系数算"。启动器内嵌 GM 的做法本来就是进程内改字段；服务端要等效实现，
得把每个副本每个怪的倍率**逐只**塞进属性协议的 entry-stat 链路，属于另一条
协议取证路线，且对已存在的怪不生效。业主已明确要求"动客户端补丁"
（`client-patchs/AGENTS.md` 的前置条件已满足），所以本补丁按客户端插件形态交付。

## 边界与风险

- 客户端带 **BlackCipher** 反作弊组件；新加载 DLL 是否被拦**没有验证**。
  一旦游戏异常，先卸载补丁再复现，以区分"补丁问题"与"客户端问题"。
- 插件是**进程内原生 DLL**：插件写坏内存 = 游戏崩。所以这里的原则是
  "宁可失效也不崩"：验不到就不写，验到一半就回滚。
- **实机由业主操作**：本目录只交付源码、DLL 与 mod 包，不无人值守启动客户端。
