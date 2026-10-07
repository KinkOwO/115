# client 层说明（odyssey.hardcore）—— 已改用客户端插件

**本包不再对 `DFO.exe` 做任何字节补丁**（`mod.json` 的 `client.ops` 里没有 `exe.patch`）。
client 层现在是**两条 `file.add`**：

| # | kind | target | source | 内容 |
| --- | --- | --- | --- | --- |
| 1 | `file.add` | `.115us-mods/DifficultyRules.dll` | `client/DifficultyRules.dll` | 客户端难度引擎（宿主插件） |
| 2 | `file.add` | `.115us-mods/rules.d/odyssey.hardcore.json` | `client/rules.d/odyssey.hardcore.json` | 本 mod 自带的**范围规则**：只对奥德赛副本，`percent`/`attackPercent` = 1000 |

DLL 的源文件是 `client-patchs\difficulty\dist\DifficultyRules.dll`（由
`client-patchs\difficulty\build-dll.cmd` 编译），**打包时从那个路径取**；
`build-mod.py` 在该文件缺失时直接报错退出（不会生成一个"装上去但没有倍率"的包）。

> 本目录里只有说明文件 + 一份范围规则 JSON：**没有** `.dll` 二进制副本入库
> （`/.gitignore` 里 `*.dll` 被忽略，见 `client-patchs/AGENTS.md` §1.6「构建产物不入库」）。
> 打包时由 `build-mod.py` 从上面的 dist 路径取，再算 size/sha256 写进 `mod.json`。

> **本文件（`client/PATCH-NOTES.md`）只是包内的普通文件，不生成任何 manifest op**：
> `build-mod.py` 的 doc 阶段把 `client/*.md` 拷进 staging（`build-mod.py:161-174`），
> 于是它随 zip 可读，但**不落位到客户端**、`mod.json` 的 `client.ops` 里也没有它 ——
> modkit 只按 `ops` 干活，所以"包里多一份说明文档"不会多出任何安装动作。

---

## 一、为什么以前用 `exe.patch`

业主口径是"奥德赛模式怪物血量 ×10"。2026-10-06 的取证结论是**服务端做不到、内容层也没有目标**：

1. **服务端不下发怪物血量**：全仓 `MaxHP|max_hp|CurrentHP` 零命中，怪物包
   （NOTI29 / `protocol.DungeonMonster`）只有 `entity / template / level / rank / team`；
2. **内容层没有目标可改**：内层 `Script.inner.pvf`（5,650,173 条目）里 `.tbl` **0** 个，
   `commonmonsterbase`/`baseparameter`/`difficultybonus` **0** 个；客户端 14,643 个 NPK /
   456,188 条目里 `.tbl` **0** 个、血量/难度相关条目 **0** 个；
3. 于是当时只剩 `exe.patch`：把 HP getter 的**两个 flag==1 调用点**改道到一个 16 字节
   trampoline（`mov r10,rcx` / `call getter` / `imul rax,rax,10` / `mov rcx,r10` / `ret`），
   写进 `0x1486786AB` 的函数间 21 字节 `cc` 填充。

那一版**能用但不好**，两个硬伤：

* **补丁是全局的**：改的是两个取值调用点，所有副本走同一入口的怪物都会变厚，
  做不到"只有奥德赛" —— 而业主的需求是**普通角色/普通副本不受影响**；
* **它改的是 `DFO.exe` 文件本身**（26 个字节）：多一个必须备份/还原的现场，
  还撞上 TheMida 节的存在性风险；而且那 2 个调用点所在的函数
  （`0x145A06DB0`）是 6 分支的伤害/招式辅助函数，静态无法排除"怪物打人也变疼"。

## 二、为什么现在不用

2026-10-07 起 `client-patchs/difficulty` 的 **`DifficultyRules` 插件**成熟了：它同样读写怪物血量与
物攻/魔攻字段，但走的是**宿主插件通道**（`qol.client-host` 占住客户端唯一可自动加载的槽位，
插件从 `<客户端>\.115us-mods\*.dll` 被加载），于是：

| | 旧 `exe.patch` | 现在的插件 |
| --- | --- | --- |
| 碰 `DFO.exe` 文件 | 是（26 字节，要整文件备份/还原） | **否**（一个字节都不改） |
| 落位 `dinput8.dll` | 否 | 否 |
| inline 钩子 / 函数改写 | 是（2 处 `call`→`jmp` + 代码洞） | **否**（只做内存读写，不调用任何游戏代码） |
| 作用范围 | **全局**（所有副本） | **按副本 id**（`dungeonIds`，本包 = 真实 PVF 解析出的 56 个奥德赛副本） |
| 能不能只对奥德赛 | 做不到 | **能**，这就是换掉它的主要原因 |
| 改倍率要做什么 | 重新打 EXE 补丁 + 重装 mod | 改 `rules.d\*.json` 后重启客户端 |
| 风险 | TheMida 完整性未知；伤害可能连带变化 | 写前逐字节核对 + 读回校验 + 失败回滚；指纹不符不接管 |

插件纪律（与 `client-patchs/AGENTS.md` §1 一致）：**不覆盖 `DFO.exe`、不落 `dinput8.dll`、
无 inline 钩子、日志只写插件自己目录、指纹不符不接管、写前逐字节核对**。

## 三、范围规则（本包的核心）

`.115us-mods/rules.d/odyssey.hardcore.json` 只有一条规则：

```json
{
  "schema": 1,
  "enabled": true,
  "rules": [
    {
      "id": "odyssey-hardcore-10x",
      "enabled": true,
      "dungeonIds": [100004934, "... 共 56 个 ...", 100004990],
      "percent": 1000,
      "attackPercent": 1000
    }
  ]
}
```

* `percent` / `attackPercent` 单位 **100 = 1.00 倍**，`1000` = ×10；
* `dungeonIds` 是**真实 PVF 解析**出来的：`contents/2026/aradodyssey/**/*.dgn` 里
  `[dungeon mode script] arad odyssey` 的全部副本（服务端解析为
  `catalog.DungeonDefinition.Odyssey == true`），共 **56** 个，区间
  `100004934..100004957` + `100004959..100004990`（`100004958` 在源里没有这个 DGN）；
* **没有** `all: true` —— 这是"普通角色不受影响"的保证：不在列表里的副本插件一条规则都不命中，
  保持原版（`percent` 走 100）。

### 插件怎么合并多份规则文件（决定"谁赢"）

插件（≥ 本次这一版）按固定顺序读**两处**规则：

1. `<插件目录>\rules.d\*.json` —— 按**文件名升序**遍历；
2. `<插件目录>\rules.json` —— **最后**读。

每个文件内部仍是"第一条 `enabled` 且命中的规则生效"；跨文件是**先遍历到的文件里命中就赢**。
所以本包 `rules.d` 里的奥德赛规则在奥德赛副本内**赢过**玩家 `rules.json` 里的通用规则，
其它副本仍按玩家的规则（或原版）。

**为什么这样定优先级**：`rules.json` 是**玩家的**文件、`rules.d` 是**随 mod 分发的**文件。
如果让玩家的通用规则（例如"全副本 ×2"）先命中，mod 作者声明的范围就被打穿，
而 mod 作者既无法预期也无法修（他不能去改玩家的文件）。让 mod 自带文件先于玩家文件
= "更具体的规则优先"，且**不需要引入任何新字段**。

`rules.d` 里**单个文件解析失败只跳过它自己**（日志里醒目记一行 `[跳过]`），
不影响其它文件；玩家 `rules.json` 缺失/读失败/解析失败仍保持旧语义（整份配置不可用 → 还原并停止接管）。
`rules.d` 目录不存在 = 与旧版行为完全一致。

## 四、装 / 卸

```powershell
# 打包（两个变体：完整包 + rules-only）
python mods\odyssey-hardcore\build-mod.py
python mods\odyssey-hardcore\build-mod.py --rules-only

# 看计划（必须 0 阻断）
<modkit.exe> plan --client C:\Game\dof\115us\DFO --mod "mods\odyssey-hardcore\dist\odyssey.hardcore-1.0.0.zip" --root C:\Game\dof\115us\115

# 安装；server 层是 Go 包，装完要重编服务端
<modkit.exe> install --client C:\Game\dof\115us\DFO --mod "mods\odyssey-hardcore\dist\odyssey.hardcore-1.0.0.zip" --root C:\Game\dof\115us\115

# 一键卸载（client 层两条 file.add 逐字节还原；rules.d 里的 JSON 也会被删）
<modkit.exe> uninstall --client C:\Game\dof\115us\DFO --id odyssey.hardcore --root C:\Game\dof\115us\115
```

> **游戏运行时 modkit 拒绝写入客户端**。要装/卸先关掉 `DFO.exe`。

### ⚠️ 与 `difficulty.rules` 互斥（有意为之）

`difficulty.rules` 与本 mod 都投 `.115us-mods/DifficultyRules.dll`。
两个都装时 modkit 会以「**目标已被 mod difficulty.rules 占用**」**阻断**安装计划。

这是**故意的**：同一个难度引擎只允许一个实例 —— 双份加载会把倍率叠成 ×100。
要换用本 mod，先卸掉 `difficulty.rules`：

```powershell
<modkit.exe> uninstall --client C:\Game\dof\115us\DFO --id difficulty.rules --root C:\Game\dof\115us\115
```

`rules.json`（玩家的通用规则）两者共用，**不需要动**；只装本 mod 时它是 `enabled: false` 的模板，
不会与 `rules.d` 里的奥德赛规则打架。

## 五、怎么确认客户端这一层生效

进游戏后看 `<客户端>\.115us-mods\`：

1. `difficulty-rules.log` 里应有
   `规则已加载：共 2 份文件、N 条规则（… rules.d 指纹 0x…、1 个 .json）`
   与两份文件的逐条清单（`<odyssey.hardcore.json> enabled=1 共 1 条` / `<rules.json> …`）；
   进奥德赛副本后应出现
   `副本 1000049xx 命中规则 odyssey-hardcore-10x（…，来自 odyssey.hardcore.json）→ 血量 10.00 倍 / 伤害 10.00 倍`
   与逐只怪的 `血量已调整` / `属性0x398已调整` 行；
2. `difficulty-rules.status.json` 里 `ready: true`、`enabled: true`、`ruleFile` = `odyssey.hardcore.json`、
   命中时 `percent`/`attackPercent` = 1000；普通副本里 `ruleId` 为空、`percent` = 100；
3. 指纹不符（换了客户端构建）时 `ready: false` + `rejectReason: clientFingerprintMismatch`，
   插件**一个字节都不改**。

## 六、历史（已经不在本包里了）

被替换的那版 `exe.patch`（3 处、26 字节）完整记录在 git 历史里
（`mods/odyssey-hardcore/client/PATCH-NOTES.md` 的 2026-10-06 版本、`build-mod.py` 的
`make_client_ops` / `build_trampoline` / `verify_patch_sites`）。
如果将来还需要 EXE 补丁路线，请从那里取，**不要**在新版本里重新发明一套偏移。
