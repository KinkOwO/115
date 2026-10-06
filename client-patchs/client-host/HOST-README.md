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

宿主只做三件事：开日志、按文件名字典序枚举插件、每个插件起一个线程调用它。

## 插件 ABI

```c
/* 插件必须导出（C 链接，x64 无名字修饰）： */
__declspec(dllexport) DWORD WINAPI ModStart(void);   /* 宿主在独立线程调用；0 = 成功 */
/* 可选： */
__declspec(dllexport) const char *WINAPI ModName(void);  /* 日志里显示的可读名字 */
```

- `ModStart` 也可能被写成 `StartLocalization`——宿主两个名字都认，
  这样同一个 DLL 既能当插件，也能直接占那个唯一槽位（便于单独调试）。
- 插件按**文件名字典序**加载，顺序稳定可预期。
- 宿主**只加载、不卸载**：插件一律 `GetModuleHandleExW(… PIN)` 固定到进程结束。
- 一个插件失败不影响其它插件（各自线程、各自日志）。

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

## 怎么装

```powershell
modkit install --client <客户端根> --mod client-host-1.0.0.zip --root <启动器根>
# 依赖它的客户端 mod 必须后装（清单 requires: ["qol.client-host"]）
```

卸载宿主前必须先卸载依赖它的 mod（modkit 会拒绝：被依赖的 mod 不允许卸载）。

> 顺带一条安装语义：`file.add` 要求目标**必须不存在**（已存在且内容不同 → 拒绝），
> 所以宿主本身也走"先卸后装"的升级路径，不能原地覆盖被改过的 `ChineseLocalization.dll`。

## 边界与风险

- 宿主不改 `DFO.exe`、不改任何函数体；它只在那个 DLL 槽位里存在，卸载即逐字节删除。
- 插件是**进程内原生 DLL**：插件崩 = 游戏崩。写插件要像写驱动一样保守
  （参考 `client-patchs/chinese-input`：包装后原样转发、日志只写自己目录、绝不改返回值语义）。
- 客户端有 BlackCipher 反作弊组件。本机既有的 dinput8 代理已经属于"外置 DLL"这一类，
  但**新加载 DLL 仍有可能被拦**；一旦游戏异常，先卸载本 mod 再复现一次以区分责任。
