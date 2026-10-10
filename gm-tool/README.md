# DFO GM 工具（独立网页版）

> **2026-10-05：这个独立 GM 工具已经移除。**
>
> 两条入口（`Start-GMWeb.cmd` → `scripts/gmweb.py` → `bin/gmweb.exe`，以及 `GM管理台.cmd` →
> `dashboard/gm_dashboard_proxy.py` + `bin/gmweb.exe`）都跑不起来：`gm-tool\bin\` 下**没有**
> `gmweb.exe` / `admin.exe`，包内那份 `gm-tool\python` 也已在去 Python 那轮删除（Python 运行时只在
> 整合包外的 `..\gm-tool\python`）。因此这些入口与它们的 Python 代理、Dashboard 前端一并删掉了。
>
> **GM 现在只有启动器内嵌的那一个**：启动器仓库 `gm/` → `gmbridge.exe`（Go；PostgreSQL 与 SQLite
> 两条存储档都支持），命令行部分用服务端的 `dfo-tool accountlist` / `cmd/admin` / `dfo-tool setlevel`。
> 下面这些小节是**历史记录**（路径多为旧的 `D:\115us\...`），保留作排查参考，不再维护。

## 2026-10-04 数据库访问收口候选

Dashboard 的管理台邮件列表/发送/撤销与个人仓库发放统一转发到 Go GM 后端的认证接口。Python 不再读取数据库配置、调用 psql 或写 SQL；对应查询由服务端 storage 内的 sqlc 生成方法执行。管理台 `gm_mail` 队列仍独立于游戏 `character_mail`，不将管理台发送视为游戏内投递或领取。

仓库发放复用现有角色/主金库事务，校验账号归属，拒绝超出 uint32 的数量及合并溢出，保留未知物品 JSON 字段。新代理须与包含这些接口的 Go 候选配套使用；旧发布二进制没有这些接口，本轮尚未替换发布程序或进行实机验收。

离线转发回归：`../gm-tool/python/python.exe -m unittest discover -s gm-tool/dashboard -p test_gm_dashboard_proxy.py`（在仓库根执行，不启动网页或数据库）。Python 是 **GM 工具专属依赖**，已从 `tools\` 移到整合包外的 `gm-tool\python`（与 `tools\` 同级；启动游戏不需要它）。数据库集成统一使用显式 `DFO_TEST_POSTGRES_DSN` 和自动清理的隔离 schema。

## 2026-10-03 当前源码：PVF 唯一内容源

GM/admin 源码和 Python 启动器默认使用原生 PVF，拒绝 JSON 内容源；物品分类、可发放集合与完整装备定义不再读取 `items.index.json` 或 `equipment-full.index.json/.data`。中文名称覆盖、背包策略和操作备份继续保留。代理必须取得后端 `/api/catalog-metadata`，接口错误直接报错。

默认存储路径为本仓库 `server/work/dfo-lan/runtime/storage/local.json`；指定 `--storage` 时，从其所在模块派生 `../client-build/Script.inner.pvf` 与 `configs/pvf-drop-policy.json`。资源路径可显式覆盖，`--pvf-source-checksum` 可选，留空时仍校验并使用实际资源哈希；玩家存档身份逻辑保持。

本批候选位于 `server/work/dfo-lan/.tmp/item-equipment-cleanup/gmweb.exe`，未替换 `gm-tool/bin` 发布程序。仓库根目录下可只读检查候选（不读取存储配置、不启动 PostgreSQL、网页服务或浏览器）：

```powershell
../gm-tool/python/python.exe ./gm-tool/scripts/gmweb.py --check --gmweb-binary ./server/work/dfo-lan/.tmp/item-equipment-cleanup/gmweb.exe
```

需要手动回归时使用同目录 `启动GM候选.cmd`，可附加 `--storage` 等参数。本批已完成离线原生目录与发放回归，未增加实机确认范围。以下为旧发布包和早期候选的历史说明，旧 JSON 启动参数不适用于当前源码。

## 2026-10-01 PVF查询候选

源码`server/work/dfo-lan/cmd/gmtool`新增原生源入口；独立程序放在模块的`.tmp/pvf-management/bin/gmweb.exe`，现有发布程序和默认启动参数保持。模块根目录下只检查候选目录（不需要数据库）：

```powershell
../../../../gm-tool/python/python.exe ../../../gm-tool/scripts/gmweb.py --check --catalog-source pvf --gmweb-binary .tmp/pvf-management/bin/gmweb.exe --pvf-archive ../client-build/Script.inner.pvf --pvf-source-checksum 7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80
```

候选索引599771个原生LIST绑定，旧386230个ID、kind/grade/rarity全部一致。部位/最低等级直接读EQU；旧缓存包含过期字段，候选不继续用它决定筛选。名称按脚本实际引用定位，不拼ID；旧中文译名作为外部显示覆盖，当前PVF原文多数为英文。具体663处名称和旧部位/等级差异见`docs/todo/pvf/PVF直读第五批迁移进度.md`，这批还未实机验收。

代理从认证`/api/catalog-metadata`获取后端分类和实际可堆叠集合；PVF路径不依赖items.index、equipment.slots、equipment.current37或loot导出JSON。角色数据、背包/穿戴策略、译名覆盖、操作备份继续保留。当前源码未提供套装/装扮分组JSON消费入口，不扩大历史套装管理功能。


本地兼容服的 GM 操作工具，网页界面，用于**改角色等级、发金币/点券/物品**。
自包含：内含编译好的程序、**38 万件物品库**、**与游戏完全一致的物品中文名**、精简 Python，
拿到包的人**不需要装任何东西**。

---

## 一、前提

- 对方电脑已有 `D:\115us` 环境（游戏 + 服务端 + 数据库），**或**至少有一套能连的 PostgreSQL(25438)。
- 数据库位置默认 `D:\115us\server\work\dfo-lan\runtime\storage\local.json`；路径不同就用 `--storage` 指定。
- 首次使用前先启动一次游戏环境（或数据库已在后台运行）。

## 二、快速开始

1. 把整个 `gm-tool` 文件夹解压到任意位置（例如 `D:\gm-tool`）。
2. 双击 `Start-GMWeb.cmd`。
3. 脚本自动：拉起 PostgreSQL（若未运行）→ 启动网页服务 → 打开浏览器 `http://127.0.0.1:28080`。

```bat
Start-GMWeb.cmd --storage D:\对方的\路径\local.json
Start-GMWeb.cmd --check                 :: 只看依赖，不启动
Start-GMWeb.cmd --listen 127.0.0.1:8888 :: 换端口
Start-GMWeb.cmd --no-browser             :: 不开浏览器
```

> 整个文件夹可以随便搬（U 盘、别的盘符、别人的电脑都行）：程序会自己在
> **自己所在的 `configs` 目录**里找物品库/名字表/部位表。

## 三、功能

- 顶部显示账户**点券**余额，可发放（负数即扣减）。
- 角色卡片显示 名字 / **职业中文名** / 等级 / 金币；可**改等级**、**发金币**。
  职业名按角色目录里的 `job` 标签绑定，中文来自客户端文本表（`string/ui.uv.str` 的
  `adv_tab_challenge_category_*`、`string/character.uv.str` 的 `growtype_name_*`）——
  例如 `profession=11` 显示 **女鬼剑士**、`0` 显示 **男鬼剑士**。
  注意客户端把「圣职者」写作 **光职者**，界面跟客户端一致。
- **属性面板（本次新增）**：默认只铺一组**精选核心属性**（HP/MP 上限、物理/魔法攻击力、物理/魔法防御力、
  攻击/施放/移动速度、跳跃力、硬直、负重上限），其余（抗性、恢复速度、物品栏负重等）折叠进
  「展开全部属性」。属性名用**中文**，内部键（`[hp max]` 这种）只放在鼠标悬停提示里；
  对照表里没有的键会标「未汉化」原样列出，不猜也不隐藏。数值一律是存档原值，不做换算。
  中文用词逐条对照客户端 `string/*.uv.str` 原文（实测：`string/equipment.uv.str` 里就有
  「力量」「智力」「体力」「精神」「攻击速度」「施放速度」「移动速度」「跳跃力」「命中率」「回避率」
  「物理防御力」「魔法防御力」「火/冰/光/暗属性抗性」；`string/skill.uv.str` 里有「HP上限」）。
  注意：**键 → 中文名**这张表是工具内置的对照表（客户端文本表里没有键名对照），不是从客户端导出的。
- **发物品**：按中文名 / 英文名 / ID 搜索，选中后选角色、填数量发放。
  **哪些真的能发，界面上直接标出来（本次新增）**：
  - 每件物品带 `grantable`，判定用的是服务端发放路径**同一把尺子**（掉落目录 + 装备目录的
    `Reward` 规则），不是写死的 true；
  - 默认打开「**只看可发放**」；关掉后不可发放的条目会置灰并写明原因，点它直接被挡下并说明，
    不会再出现"挑了、点了执行、才发现发不出去"；
  - 装扮（时装）单独标记为不可发放：服务端的背包层没有装扮容器，发放会塞错容器；
  - 界面上给出「可发放 N 件 / 不可发放 M 件」以及按原因分组的统计。
- **角色修改（本次新增，五张卡）**：选中角色后，右侧面板依次出现
  1. **改等级**：填目标等级 → 同时把 `experience` 设成该等级的**门槛**（门槛来自服务端自己的
     成长目录 `configs/progression.next25.json` + 规则 `configs/experience.compat90.json`，不是本工具编的公式）；
     可勾选"同时把技能点按该级重算为满值"。界面写明：服务端规则上限 **150**（硬上限），
     客户端上限 **115**，超过 115 会不会出问题由使用者自己决定。
  2. **技能点 SP / TP**：两个数组（协议里的两条技能树账本）分别可填；可一键"按当前等级重算 SP 为满值"。
  3. **背包与装备**：列出 `inventory` 的 `items` / `worn` / `equipment` 三个数组，物品行改**数量**（填 0 即删除该行），
     装备行只改**耐久**；`template` 与 `slot` 一律不给改，只改已有行、不会新建。
  4. **任务一键完成**：把该角色 `character_quests` 里 `status='accepted'` 的行改成 `'completed'`，
     **只改状态、绝不插入新任务**；执行前必须勾选二次确认，执行后显示影响行数。
  5. **转职 / 觉醒**：**禁用**（卡片里写清了原因与代码位置）。服务端在
     `internal/character/detail.go:76`、`internal/character/progression_state.go:23`、
     `internal/character/learning_catalog.go:90` 三处硬拒绝 `advancement != 0`，
     改大它会让角色卡在选人界面进不去，所以不提供这个按钮。
- **改存档的安全措施**：每次写操作前把该角色当前 `state` 原样备份到 `backups/`
  （文件名带角色 id 与时间戳，备份失败则整笔拒绝），并在 `admin_grants` 里留一行审计
  （`grant_id` 幂等键 + `request` 里记录"改了什么"）。角色在线上时必须**重新选择角色**才生效。
- **筛选**：
  - **按类型**：消耗品 / 材料 / 任务 / 副职业 / 徽章 / 装备 / 其它
  - **按部位**：武器 · 称号 · 上衣 · 头肩 · 下装 · 鞋 · 腰带 · 项链 · 手镯 · 戒指 · 辅助装备 · 魔法石 · 辅助武器 · 耳环 · 时装（按真实部位细分为 时装·帽子/上衣/… ） · 其它装备（顺序即装备栏顺序）
  - **按等级**：≤20 / 21-50 / 51-80 / 81-100 / 101-110 / 111-115 / 116+
  - **按品级**：普通 / 高级 / 稀有 / 神器 / 史诗 / 勇者 / 传说 / 神话 / **太初**
  - 四组条件可与关键词**任意叠加**；带"清除筛选"、命中条数与品级色标；
    页面地址栏也会带上筛选条件，可直接收藏或转发。
- 发放记录（审计）保留最近 100 条。

> 角色在线上时，需要**重新选择角色**（小退再进）才能看到变化。
> 一键完成任务请**先把角色切换到别的角色**（或退回选人界面）再执行，否则存档可能覆盖掉这次改动。

## 四、物品名为什么和游戏一模一样

物品中文名来自**客户端当前汉化补丁里真正加载的文本表**（`Script.pvf` → 内层 PVF 的
`string/equipment.uv.str`、`string/stackable.uv.str` 等），一共 **418,909 条**，
覆盖物品库里全部 386,230 件物品，**一件不漏**。

优先级：**客户端文本表 → 旧汉化兜底表 → 英文名**。

> 对照示例：`101001153` 游戏里叫 **太初之星 - 短剑**（旧流水线误译成"原始的星辰 - 短剑"）；
> 品级 8 游戏里叫 **太初**（旧表误作"原始的"）。现在两边一致。

## 五、目录结构

```
gm-tool/
  bin/gmweb.exe            网页 GM 服务（新）
  bin/admin.exe            命令行 GM 工具（可选）
  bin/gmweb.exe.bak_*      历史版本备份
  configs/items.index.json     38 万件物品库
  configs/names.client.json    客户端 PVF 文本表导出的显示名（41.9 万条）
  configs/equipment.slots.json 装备 id → 部位/最低等级（13.9 万件）
  configs/equipment.current37.json / loot.next25.json / characters.next25.json / ...
  python/                  GM 专属 Python 3.11 便携版（本目录缺 python.exe 时用整合包外的 ../gm-tool/python；启动游戏不需要它）
  scripts/gmweb.py         启动脚本
  Start-GMWeb.cmd          双击入口
```

## 六、命令行版（可选）

```bat
:: 看角色列表（注意：admin.exe 只支持发放，不支持改等级/技能点/背包/任务 ——
:: 那四项在网页界面的「角色修改」区里，见第三节）
bin\admin.exe -storage D:\115us\server\work\dfo-lan\runtime\storage\local.json -characters

:: 发放
bin\admin.exe -storage ... -grant-id gm001 -reason "测试" -character 4 -gold 1000000 -cera 10000
```

`gmweb.exe` 还可直接带参数跑（供脚本/别的启动器调用）：

```bat
bin\gmweb.exe -storage <local.json> -listen 127.0.0.1:28080 ^
  -loot-catalog configs\loot.next25.json -bag-rules configs\inventory.current37.json ^
  -equipment-catalog configs\equipment.current37.json -item-index configs\items.index.json
:: 不带这些参数也行，程序会在自己旁边的 configs 里自动找
```

## 七、数据来源与重建

- **物品名 / 品级名**：客户端内层 PVF 的 `string/*.uv.str`。
- **部位 / 最低等级**：服务端装备目录的 `[equipment type]` / `[minimum level]` 真实字段
  （逐行枚举 19,955 行，22 种取值一个不漏）。
- 重建方式（需要一份解密后的客户端内层 PVF，参见仓库 `reference/analysis-tools/unwrap_pvf.py`）：

```bat
bin\gmweb.exe -build-data -client-pvf <客户端内层PVF> -equipment-slots configs\equipment.slots.json
```

## 八、已知限制（诚实说明）

- **转职 / 觉醒不能改**：服务端在三处硬拒绝 `advancement != 0`
  （`internal/character/detail.go:76` 的 `initialSkills()` 会让角色卡在选人界面进不去；
  `internal/character/progression_state.go:23` 的 `ApplyGain()` 让进阶职业拿不到经验；
  `internal/character/learning_catalog.go:90` 的 `Cost()` 让进阶职业学不了技能）。
  界面里这一项显示为禁用 + 原因，不提供按钮。
- **改等级不改属性**：`level` 与 `experience` 成对写，但 `attributes`（HP/MP/攻防…）保持原样。
  这是刻意的 —— 不动 `attributes` 就不会越过服务端 `EntryAddition` 的原生边界校验。
- **等级上限**：服务端经验规则是 150（硬上限），本包对应客户端是 **115**。
  超过 115 会不会出问题由使用者判断；公开资料记录过超过上限会导致服务端崩溃/假死、
  物品全红、拿不到 SP。
- **背包只能改已有行**：不新增物品、不改 `template`/`slot`。公开资料记录过往背包里塞时装
  会导致炸服，所以这里不提供新增行。
- 装备 258,820 件里，**80,356 件**能定位到具体部位；剩 **178,464 件**归「其它装备」
  （主要是 500–516 段时装与旧版道具，两份装备目录里都没有它们）。
- 等级只对 **82,766 件**装备已知，其余没有来源 → **按等级筛选时会被排除**（不是当成 0 级）。
- 物品库里大部分装备**不在服务端现役目录**里，发放时服务端会拒绝——这是既有行为。
  现在这类条目会直接标成「不可发放」并给出原因，不用等到执行才发现。
  当前物品库 386,230 件的实际判定是：**可发放约 6.4 万件 / 不可发放约 32.2 万件**，
  原因分布（界面「不可发放的原因」里实时显示）：
  - **装扮（时装）约 16.7 万件** —— 服务端的背包层没有装扮容器，发放会塞错容器，工具直接挡下；
  - **约 15.5 万件** —— 服务端装备目录里没有这件装备（目录只覆盖能从客户端内层 PVF
    还原出必要字段的那部分）；
  - 约 100 件 —— 装备目录里缺 `[durability]` / `[attach type]` / `[rarity]` 等字段；
  - 1 件 —— 物品库里的 `template=0`，不是有效物品。
- 装扮（时装）**发不出去**：服务端的背包层没有装扮容器（`internal/inventory` 里没有
  `Avatars`/`AddAvatar`），发放会塞错容器。界面把这一整类标成不可发放并说明原因。
- 工具只监听 `127.0.0.1`，**无登录认证**，请只在本人电脑上使用。
