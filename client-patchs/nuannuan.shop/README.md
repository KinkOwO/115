# 暖暖商城 mod（nuannuan.shop 3.2.1）

把「10月4日版暖暖商城 mod」的 v2 + v3.2 合并成**一个**可一键安装 / 一键卸载的 modkit 包。

## 0. 目录为什么在这里（不要再挪回 `mods\`）

启动器扫库是 `mods\` 下**两层**且「目录里有 `mod.json` 就算一个 mod」（`DefaultScanDepth = 2`）。
源码目录一旦叫 `mods\nuannuan.shop` 且带 `mod.json`，导入时会被判定为「覆盖同名 mod」——
引擎先删整个目录再解压，**源码和已打好的 zip 一起被删光**。
2026-10-11 实测就是这么丢的（5.9GB zip + 打包脚本全没，只能重打）。

所以：**源码在 `client-patchs\nuannuan.shop\`，产物 zip 在 `.tmp\nuannuan-dist\`（已 gitignore），
两者都不在启动器的库目录里。**

## 0.1 ⚠️ 2026-10-11：原始源码目录已被删除，`build-mod.py` 跑不了

`~/Downloads/10月4日版暖暖商城mod\`（v2 + v3.2 原始素材）**已被删除**，
`build-mod.py` 现在会直接报 `FAIL: 源目录不存在：…\暖暖商城_覆盖安装包_v3.2\安装用文件\客户端`。
而且**无法从成品包完整还原**：包内没有作者的 `_payload_manifest.json`，
而 v3.2 的 size/sha256 就是直接采信它的。

**今后改这个包走"派生"路线，不再跑 `build-mod.py`**：

1. 基线是 `.tmp\nuannuan-dist\nuannuan.shop-3.2.1.zip`（成品包本身自洽：每条 op 的
   size/sha256 都在 `mod.json` 里）；
2. 用 `.tmp\hosttest\derive-321.py` 做流式改包 —— 只改 `mod.json`（删/加 op、升 version），
   其余条目逐个复制，**不落盘解压**（5.3GB 约 2.5 分钟）；
3. `modkit verify` 通过后再 `modkit install`（同 id 升级，引擎自动"先还原旧条目再重放"）。

参考实现见 `.tmp\hosttest\derive-321.py`：删 op 用
`[o for o in ops if o["target"] != "<目标>"]`，并从 zip 里一并剔除对应 source 条目。

> ⚠️ 派生时用的是 `compresslevel=1`（内容本就是压缩容器，级别高低几乎不影响体积），
> 包会比原版小几百 MB —— 这是**正常**的：modkit 只校验每条 source 的**内容** sha256，
> 与压缩方式无关。

## 1. 打包（⚠️ 当前不可用，见 §0.1）

```bat
cd /d C:\Game\dof\115us\115\client-patchs\nuannuan.shop
python build-mod.py                :: 全量（v2+v3.2，约 5.9GB）—— 需要原始素材，现已缺失
python build-mod.py --skip-v2      :: 只打 v3.2（约 1.9GB）
python build-mod.py --dry-run      :: 只列清单，不打 zip
```

- v3.2 的 size/sha256 **直接采信作者 `_payload_manifest.json`**（不重算）；
- v2 的 15 个 NPK 现算 sha256，结果缓存在 `.tmp\nuannuan-dist\v2-npk-hashes.json`，重打包不重算；
- 大文件**不落 staging**，直接从 Downloads 源目录流式读进 zip（zip64 + STORED）；
- PowerShell 7 便携版（909 个文件）打进 `tools\pwsh7\`，用来解 `npk.index` 的硬阻塞；
- 自包含 net10 的索引重建小工具打进 `tools\npkidx\NpkIndexTool.exe`（见 §2.5）。

### 1.1 重编索引工具

```bat
cd /d C:\Game\dof\115us\115\client-patchs\nuannuan.shop\idxtool
powershell -ExecutionPolicy Bypass -File publish.ps1
```

需要 .NET 10 **SDK**（编译用，不进包）。没有就先装（免管理员）：

```powershell
iex "& { $(irm https://dot.net/v1/dotnet-install.ps1) } -Channel 10.0 -InstallDir $env:TEMP\dotnet-sdk10 -NoPath"
```

产物写到 `.tmp\npkidx\NpkIndexTool.exe`，`build-mod.py` 从那里取。

## 2.5 为什么 `npk.index` 要一个独立 exe（.NET 版本这个坑）

| 事实 | 说明 |
| --- | --- |
| 索引构建器 `USLocalServer.Pvf.dll` / `USLocalServer.Launcher.Core.dll` | `TargetFrameworkAttribute` 是 **`.NETCoreApp,Version=v10.0`**，依赖 `System.Runtime 10.0.0.0` |
| 本机全局 .NET | 只有 6.0.10 / 7.0.4 / 8.0.8 —— 一加载就 `Could not load file or assembly 'System.Runtime, Version=10.0.0.0'` |
| 包内 pwsh 7.4.6 便携版 | **自包含（self-contained）net8**：目录里有 `hostfxr.dll` / `System.Private.CoreLib.dll` |
| 自包含应用的规矩 | **忽略一切 roll-forward** —— `DOTNET_ROLL_FORWARD=LatestMajor`、改 `pwsh.runtimeconfig.json` 加 `rollForward: LatestMajor` 都实测无效，它永远跑在 8.0.10 |

试过且否决的路：给它配 .NET 10 运行时 + roll-forward（无效，见上）、
换框架依赖型 pwsh + `dotnet.exe pwsh.dll`（可行但要再塞 ~180MB，且让 pwsh 跑在超出两个大版本的运行时上）。

现在走的路：`rebuild-index.ps1` 只做「找工具 → 复制到 `%TEMP%` → 调用 → 透传输出与退出码」，
真正的活由**自包含 net10 的单文件 exe** 干（`idxtool\`，37MB，压缩单文件）。
**目标机不需要装任何 .NET 运行时。**

另外两个必须记住的点：

1. 引擎执行索引脚本时**只把 `.ps1` 抽到 `<workDir>\.modkit-script\`** 再跑，DLL / exe 不会跟着过去
   —— 所以 `mod.json` 里要用 `{pack}` 把包根传进来（`-ToolsDir`、`-PackDir`）；
2. `115\` 树下**加载可执行文件会被系统拦**（`hostfxr.dll` 报 `0x80070005`），
   所以 exe 与 DLL 一律先复制到 `%TEMP%` 再用（pwsh 那份由启动器补丁复制）。

## 2. 安装（启动器）

1. **启动器必须是带「包内 pwsh7 自动复制」补丁的版本**（2026-10-11 重编）。
   旧启动器会在 `npk.index` 那步失败：它确实能找到 `mods\nuannuan.shop\tools\pwsh7\pwsh.exe`，
   但 Windows 对 `115\` 树的可执行文件加载有限制，运行时会报 `hostfxr.dll` 加载失败（`0x80070005`）。
2. 启动器 → **MOD 工具** → 导入 `.tmp\nuannuan-dist\nuannuan.shop-3.2.0.zip`。
3. 装之前关掉游戏 / 服务端 / 启动器。
4. 安装耗时长（解压 ~6GB + 落位 ~6GB + 备份 ~1.3GB + 索引重建连做两次），耐心等。

> ⚠ 选文件时**别让资源管理器停在任何会被删改的目录里**。
> 2026-10-11 那次 `unlinkat ...\dist: The process cannot access the file` 就是目录句柄被
> Explorer / 打开对话框持有，删不掉。

## 3. 配套：6 个服务端开关（不在 mod 里，要在启动器 profile 设）

modkit 没有「改服务端 json」这一层，这 6 个键都在启动器 profile 白名单里：

| 键 | 值 | 作用 |
| --- | --- | --- |
| `DFO_SHOP_OPEN_ALL` | `1` | 商城全开 |
| `DFO_BAKAL_MODE` | `unlimited` | 巴卡尔不限次 |
| `DFO_HELL_PARTY_DROP_PERCENT` | `500` | 深渊掉落 500% |
| `DFO_ATTUNEMENT_QUANTITY_MULTIPLIER` | `5` | 封印数量 ×5 |
| `DFO_ATTUNEMENT_RARITY_MULTIPLIER` | `5` | 封印稀有度 ×5 |
| `DFO_RAID_OPEN_EVENTS_PROBE` | `1` | 团本活动开放 |

改完重启服务端；第一次进游戏会重建内层 PVF，等几分钟。

## 4. 包里装了什么

| 层 | 内容 |
| --- | --- |
| pvf | `Script.pvf`(837MB) + `sk.dat`(2,816B) —— 必须成对 |
| client | `dstr.dat`（replace）；4 个字体（add）；**`dinput8.dll`（replace，扩展按钮）**；`.115us-mods\NuanNuanShop.dll`（add） |
| resource | NPK 新增 17（2 个 dragonrobe4 + v2 的 15 个）/ 替换 5；最后一条 `npk.index` 重建 `ImagePacks2\NpkIndex.etc` |

**`dinput8.dll` 换成带扩展按钮的版本**（`vendor/dinput8.dll`，855,040 B，
详见 [`vendor/README.md`](vendor/README.md)）：

- 它占的是客户端**唯一会被自动加载的槽位**，必然顶掉现有 localization proxy v2
  （107,008 B）—— 卸载时由 modkit `file.replace` 逐字节还原；
- 它自己转发系统 DirectInput，并**主动 `LoadLibrary("ChineseLocalization.dll")`**
  调它的 `StartLocalization`，所以 `qol.client-host` 宿主与 `.115us-mods` 插件链
  一个都不丢（受控加载实测 `CLIENT_HOST started result=0`）；
- 它在 `DFO.exe` 镜像布局校验**不通过时只记一行 `INSTALL rejected image layout`、
  不挂钩**，最坏情况是按钮不生效，游戏照常能进。

**`.115us-mods\NuanNuanShop.dll` 是"保留但不会跑"的**：原包的 `dinput8.dll`（840KB）
改名而来，它**没有 `ModStart` 导出**，宿主日志每次都会写
`插件没有 ModStart 导出，已跳过` —— 也就是说它从装上起就没真正执行过。业主选择保留
（无害），真正提供变身/商城能力的是上面那个新的 `dinput8.dll`。

**不动 `DFO.exe`**（v2 的 10 处补丁没有补丁表，写不成 `exe.patch`，归启动器管）。

## 5. 卸载

启动器 → MOD 工具 → 卸载。逐字节还原（`Script.pvf` 回 `bcf02f20…`、5 个 NPK 回原哈希、
字体与插件 DLL 删除、**`dinput8.dll` 还原成原 localization proxy v2**）。

> 卸完先看一眼 `ImagePacks2\NpkIndex.etc` 有没有残留。**有残留就别启动游戏**，
> 先删掉或重建 —— 索引和 NPK 集合不同源就是启动闪退。

## 6. 待实机取证（装完请确认）

1. **PVF 基线**：现场 `sk.dat` 是 2,560 B，包里是 2,816 B —— 装完服务端日志看 PVF 解析有无报错、条目数是否符合预期。
2. **索引读取路径**：客户端根有一份 `NpkIndex.etc`，`ImagePacks2\` 下原来没有。现在只重建
   `ImagePacks2\` 下那份；若实际读的是根那份，需要再加一条 `npk.index`。
3. **DLL 改名后功能是否还在**：城镇穿齐 8 件第四套龙袍，按 F9/F10 看能不能变身。
4. **扩展按钮到底挂钩了没有**：看客户端根的 **`115-shop-tabs.log`**（这个新 DLL 自己写的）。
   - `INSTALL ok` 一类 → 挂钩成功，进商城看有没有多出来的标签页/按钮；
   - `INSTALL rejected image layout` → 客户端版本对不上，按钮不生效但游戏照常，把日志发我。
5. **卸载后索引是否残留**（见 §5）。
6. **卸完确认 `dinput8.dll` 回到 107,008 B**（原 localization proxy v2）。
