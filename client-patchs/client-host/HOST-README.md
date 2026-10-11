# 客户端DLL框架（ChineseLocalization.dll）— mod id `qol.client-host`

## 它解决什么问题

客户端自带的 `dinput8.dll` 是汉化代理，它**只**加载一个固定名字的文件：

```
<客户端目录>\ChineseLocalization.dll      ← 唯一可自动加载的槽位
```

而 modkit 的 client 层规定「同一目标路径被两个 mod 声明 → 后者被阻断」，
所以第二个客户端 mod 没有地方落位。本宿主把那唯一的位置变成**插件目录**：

```
<客户端目录>\ChineseLocalization.dll       ← 本宿主（mod id: qol.client-host）
<客户端目录>\.115us-mods\*.dll             ← 各客户端 mod 的插件（file.add 落位）
```

宿主只做几件事：开日志、按上面的顺序枚举插件/依赖、加载并（在独立线程里）调用它们、
把加载结果登记下来供 `GetPlugin` 查询。

## 插件 ABI

```c
/* 插件可选导出（C 链接，x64 无名字修饰）： */
__declspec(dllexport) DWORD WINAPI ModStart(void);          /* 宿主调用；0 = 成功 */
__declspec(dllexport) const char *WINAPI ModName(void);     /* 日志里显示的可读名字 */
```

- `ModStart` 也可能被写成 `StartLocalization`——宿主两个名字都认，
  这样同一个 DLL 既能当插件，也能直接占那个唯一槽位（便于单独调试）。
- **1.1.0 起两个都没有也不算失败**（增强 A）：照样登记进宿主，日志写
  `已加载（无 ModStart/StartLocalization 入口点，仅 DllMain）`。
  注意这不改变执行时机——`DllMain` 在 `LoadLibrary` 时就已经跑完了，
  A 只是把状态从"跳过"改成"已加载"，**不会额外执行任何代码**。
- 宿主**只加载、不卸载**：插件一律 `GetModuleHandleExW(… PIN)` 固定到进程结束。
- 一个插件失败不影响其它插件（各自线程、各自日志）。

### 加载顺序（1.1.0）

1. **子目录依赖**：`.115us-mods\**\*.dll`（深度上限 3）**同步**加载，
   **只 `LoadLibrary`、不调用入口点**（增强 B）。子目录里的东西一律当依赖，
   根层重名打架的问题因此消失。
2. **清单声明项**：按 `plugins.json` 的顺序，且 `deps` 里的先加载（增强 C）。
3. **清单没声明的**：按**文件名字典序**补上（旧行为，`autoDiscover` 可关）。

### `plugins.json`（可选，增强 C）

放在 `.115us-mods\plugins.json`。**没有它时行为与 1.0.0 完全一致**
（单层自动发现 + 递归依赖），所以升级宿主不会改变现有插件的加载方式。

```json
{
  "recurse": true,
  "autoDiscover": true,
  "plugins": [
    { "name": "SomeBase",  "path": "SomeBase.dll" },
    { "name": "SomeMod",   "path": "SomeMod.dll", "deps": ["SomeBase"] },
    { "name": "PureDep",   "path": "dep/PureDep.dll", "entrypoint": false },
    { "name": "NotNow",    "path": "NotNow.dll", "enabled": false }
  ]
}
```

| 字段 | 默认 | 含义 |
| --- | --- | --- |
| `name` | 文件名主干 | 日志与 `GetPlugin()` 用的名字 |
| `path` | — | 相对 `.115us-mods` 的路径，`/` 会自动换成 `\` |
| `enabled` | `true` | `false` = 整条跳过（不 `LoadLibrary`） |
| `entrypoint` | `true` | `false` = 只 `LoadLibrary`、不调 `ModStart`（纯依赖） |
| `deps` | `[]` | 依赖的其它插件 `name`，保证**先于**本项就绪 |

解析是宿主内建的极简 JSON 读取器：只认上面这几个字段，其它字段整对跳过；
文件超过 1 MB 或解析不出形状就当没有清单，回退旧行为。

### 插件之间怎么拿到对方（增强 D）

宿主额外导出（见 `src/ChineseLocalization.def`）：

```c
HMODULE    WINAPI GetPlugin(const char *name);  /* 按 name 或文件名主干查，找不到返回 NULL */
DWORD      WINAPI GetPluginCount(void);
const char *WINAPI GetPluginNameAt(DWORD index);
DWORD      WINAPI GetHostVersion(void);         /* 1.1.0 => 0x00010001 */
```

插件里这么用：

```c
HMODULE host = GetModuleHandleW(L"ChineseLocalization.dll");
typedef HMODULE (WINAPI *GetPluginFn)(const char *);
GetPluginFn get = (GetPluginFn)GetProcAddress(host, "GetPlugin");
HMODULE other = get ? get("SomeBase") : NULL;
```

**要在自己的 `ModStart` 里可靠拿到对方，必须声明 `deps`**：依赖是同步加载的，
能保证先于依赖方就绪；没声明 `deps` 的两个插件各跑各的线程，
互相之间没有顺序保证，`GetPlugin` 可能返回 `NULL`。

## 日志与状态（排查按这三处看）

- **宿主**：`<客户端目录>\client-host.log`（固定在客户端根；路径由宿主解析自身模块目录得到）。
  它回答的是"插件被枚举到了没有、`ModStart` 返回几"；
- **插件人读日志**：`<客户端>\.115us-mods\<插件名>.log`（**插件自己所在目录**，按 AGENTS §1.3）；
- **插件机读状态**：`<客户端>\.115us-mods\<插件名>.status.json`。
  建议每个插件都写一份（`ready` / `enabled` / `rejectReason` / 命中的 id 等），
  这样启动器「MOD 工具」页与 GM 页可以**不猜**地显示"这个插件到底接管了没有"。
  范例：`client-patchs/difficulty` 的 `difficulty-rules.log` + `difficulty-rules.status.json`
  （状态里还带 `ruleFile` / `ruleFiles` / `rules`，即"命中规则来自哪份文件"）。

> **mod 自带的范围规则**（2026-10-07 起）：插件从 `<客户端>\.115us-mods\rules.d\*.json`
> （按**文件名升序**）先读，最后才读玩家自己的 `rules.json` —— 所以 mod 分发的规则
> 在各自的范围内**优先于**玩家的通用规则。`rules.d` 里**单个文件解析失败只跳过它自己**
> （日志一行 `[跳过]`），不影响其它文件；目录不存在时行为与旧版完全一致。
>
> 边界（以 `client-patchs/difficulty/src/difficulty-rules.c` 为准）：单条规则的 `dungeonIds`
> 上限 **256**（`MAX_RULE_IDS`，`:234`）、规则**总条数**上限 32（`MAX_RULES`，`:220`）、
> `rules.d` 来源文件数上限 32（`MAX_RULE_FILES`，`:252`）；任一越界 = **整份配置判不可用**
> （不"用一半"）。合并顺序 `:270-277,1784`、升序枚举 `:1616-1663`、坏文件跳过 `:1699-1705`。

## 怎么装

```powershell
modkit install --client <客户端根> --mod client-host-1.1.0.zip --root <启动器根>
# 依赖它的客户端 mod 必须后装（清单 requires: ["qol.client-host"]）
```

卸载宿主前必须先卸载依赖它的 mod（modkit 会拒绝：被依赖的 mod 不允许卸载）。

> 顺带一条安装语义：`file.add` 要求目标**必须不存在**（已存在且内容不同 → 拒绝），
> 所以宿主本身也走"先卸后装"的升级路径，不能原地覆盖被改过的 `ChineseLocalization.dll`。

## 边界与风险

- 宿主不改 `DFO.exe`、不改任何函数体；它只在那个 DLL 槽位里存在，卸载即逐字节删除。
- 插件是**进程内原生 DLL**：插件崩 = 游戏崩。写插件要像写驱动一样保守
  （参考 `client-patchs/chinese-input`：包装后原样转发、日志只写自己目录、绝不改返回值语义）。
- 1.1.0 的 B 会把**子目录**里的 dll 也 `LoadLibrary` 进来（只加载、不调入口点）。
  插件自带的第三方运行时若放在 `.115us-mods\<插件名>\` 下会因此被加载——这是想要的效果，
  但也意味着子目录里不该放"暂时不想启用"的 dll，用清单的 `enabled:false` 或
  挪到 `.115us-mods` 之外的目录更稳妥。
- `MAX_PLUGINS` 仍是 **64**（依赖 + 插件合计上限）；依赖不再占用入口点调用，但占这个名额。
- 客户端有 BlackCipher 反作弊组件。本机既有的 dinput8 代理已经属于"外置 DLL"这一类，
  但**新加载 DLL 仍有可能被拦**；一旦游戏异常，先卸载本 mod 再复现一次以区分责任。
