# modkit-web —— modkit 的简易 Web 页面

一个**独立的小工具**：本地起一个小服务，用浏览器管理一个 mods 目录 ——
分页列表、多选、批量启用/停用/删除、导入/导出 zip（支持单 mod 与多 mod 两种包形态），
以及**安装 / 卸载**（接启动器仓的 `modkit` 引擎，客户端 DLL mod、带 `exe.patch` 的包、
含 `server` 层的 mod 都在这里做）。

> **它不接启动器、也不依赖启动器代码**（不 import 启动器包，两个 Go 模块各自独立编译）。
> 装/卸走的是引擎的**命令行契约**：`modkit verify / plan / install / uninstall`。
> 也就是说：落位、备份、hash 门、逐字节还原、`exe.patch` 这些**只有一份真源**
> ——启动器仓的 `internal/modkit`；本页面只负责把命令跑起来、把原文展示给你。

规则来源：`../MOD-DEVELOPMENT.md`（包结构 / 清单模板 / 权限位 / 层动作）与
`../MOD-MANAGER-INTEGRATION.md`（`enabled.json` 的确切格式与语义、
以及"装/卸归 modkit、启停归管理器"的分工）。

---

## 一、跑起来

**最省事：双击 / 直接运行 `run.cmd`**（默认就是"管 `115\mods`、认两层"；**不会自动打开浏览器** —— 要开就自己加 `--open`，业主 2026-10-06 要求去掉自动弹窗）：

```powershell
cd C:\Game\dof\115us\115\mods\modkit-web
.\run.cmd
# 想换端口或目录就带参数（带参数时不再套用默认值）：
.\run.cmd --addr 127.0.0.1:9000
.\run.cmd --open                                   # 只有显式加 --open 才会打开浏览器
.\run.cmd --mods-dir "C:\Game\dof\115us\115\server\work\dfo-lan\mods" --addr 127.0.0.1:8931
```

`run.cmd` 的逻辑：**优先用同目录下已编好的 `modkit-web.exe`**；没有才去找 Go（先 PATH，
再 `..\..\..\tools\go\bin\go.exe`，再 `C:\Game\dof\115us\tools\go\bin\go.exe`）用 `go run .` 起。

> **本机 `go` 不在 PATH 上**（整合包的 Go 在 `C:\Game\dof\115us\tools\go\bin\go.exe`，在仓库外）。
> 所以手工命令要写全路径：

```powershell
# 直接跑源码
C:\Game\dof\115us\tools\go\bin\go.exe run . --mods-dir C:\Game\dof\115us\115\mods --scan-depth 2 --open

# 或者编一个 exe（*.exe 已被 .gitignore 忽略，不会入库）
C:\Game\dof\115us\tools\go\bin\go.exe build -o modkit-web.exe .
.\modkit-web.exe --mods-dir C:\Game\dof\115us\115\mods --scan-depth 2
```

运行时只绑 `127.0.0.1`，关掉窗口 / Ctrl+C 即停。

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `--mods-dir` | 当前目录下的 `mods/`（没有就用当前目录） | 要管理的 mods 目录 |
| `--scan-depth` | `1` | 认几层（目录与 zip 包都按这个深度找）。`1` = 直接子目录；`2` = 再往下一层（如 `client-mods/*.zip`、`examples/<mod>/`）。界面上指向 `115/mods` 时用 `--scan-depth 2` 才看得到里面的包 |
| `--addr` | `127.0.0.1:8931` | 监听地址，**默认只绑本机** |
| `--open` | 关 | 启动后自动打开浏览器 |
| `--client` | 自动 | DFO 客户端目录。默认读**启动器自己的配置** `<启动器根>/server/launcher.local.json` 的 `client_dir`（相对路径以 `<启动器根>/server` 为基准，与启动器同口径）；读不到再按候选位置探测 |
| `--root` | 自动 | 启动器根或服务端模块根。默认从 `--mods-dir` **向上**找含 `server/work/dfo-lan/go.mod`（`module dfolan`）的那一级 |
| `--modkit` | 自动 | `modkit.exe`（引擎）。默认找**页面同目录** → mods 目录 → `<根>/bin`、`<根>/mods` → PATH；也可用环境变量 `MODKIT_EXE` |
| `--launcher-repo` | 自动 | 启动器仓源码根（缺引擎时用 `go build ./cmd/modkit` 就地构建）。默认在同级目录里按内容探测（有 `cmd/modkit/main.go` + `go.mod` 是 `module dfolauncher`），也可用环境变量 `MODKIT_LAUNCHER_REPO` |
| `--go` | 自动 | 构建引擎用的 Go 工具链。默认 `GO` 环境变量 → PATH → `<启动器根>/../tools/go/bin/go.exe` |
| `--no-engine` | 关 | 关掉安装/卸载（只保留列表 / 导入导出 / 启停）；给"只想整理 mod 目录"的场景 |

**装/卸要用的这几个路径从哪来，页面顶部会原样写出来**（客户端目录 / 服务端模块根 / 引擎 /
启动器源码 + 各自的来源 + 缺项提示）——出问题时第一眼看这里，不用猜。

### 两个 mods 目录别混（`../README.md` §目录职责）

| 路径 | 是什么 | 该用哪个 depth |
| --- | --- | --- |
| `server/work/dfo-lan/mods/` | **服务端真正加载的目录**（已装 mod、`zz_mods_gen.go`、`enabled.json`） | `1` |
| `115/mods/`（仓库根那个） | **mod 作者工作区**：文档 + `examples/<mod>/` + 可分享的成品包 | `2` |

---

## 二、页面功能

- **分页列表**：每页 10/20/50/100，带总数与页码；列 = 复选框、**mod 名称**（含 id/版本/层）、
  **添加日期**、**启用开关**、**安装/卸载**、**制作人**、大小与相对路径；
- **列表项有两种**：
  - `[已装]`／**mod 目录**（`<mods>/…/<mod>/mod.json`）—— 可以启停、可以安装/卸载；
  - `[包]`／**zip 包**（`<mods>/…/xxx.zip` 里含 mod.json，如 `client-mods/` 里的成品包）
    —— 那一行给的是**「导入」按钮**而不是开关：点它把包就地解成目录（**只是解包，不做安装**），
    之后就有真正的启停开关了；「安装」按钮对 `[包]` 行同样可用（直接以 zip 装），
    **不需要先导入**。同一个 id 的"包"和"已装目录"可以同时存在
    （例如 `pkg/qol.client-host-1.0.0.zip` 与 `qol.client-host/`），
    所以界面与接口都用**唯一键 = 相对路径**来标识一项，而不是 id；
- **多选**：行复选框 + 表头全选；底部显示"已选 N 项"；
- **批量操作**：导入 mod（zip）、导出选中、启用选中、停用选中、删除选中；
- **搜索**：按 id / 名称 / 作者 / 说明过滤（输入即筛，250ms 防抖）；
- **导入前先校验**：选好 zip 后先调 `/api/inspect` 做**干跑校验**，把每个 mod 的
  通过/问题逐条列出来；**全部通过**才允许点"校验通过，导入"（已存在的目录要勾选"覆盖"）；
- **导出**：选中 1 个 → 包里**根目录就是 mod 根**；选中多个 → 包里**每个 mod 一个子目录**（两种都能被本工具重新导入）；
- **安装 / 卸载**（新）：每一行都有这两颗按钮，点完在页面里展开**步骤面板**：
  跑了哪几条命令、每条的命令行与**引擎原文输出**、退出码、是否被阻断、
  以及"含 server 层 ⇒ 要重新编译服务端二进制"这类提示；
- **预演（dry-run）**：工具栏那个勾选框。勾上后「安装 / 卸载」只求解与报告、**不写任何文件**，
  因此**游戏在运行时也能用**（先看清要改什么，再退游戏真做）。

"添加日期"取的是 mod 目录的 mtime（导入/落位时间），没有额外元数据文件。

---

## 三、安装 / 卸载怎么走（重要）

### 3.1 步骤（页面上逐条可见）

| 动作 | 依次执行 | 说明 |
| --- | --- | --- |
| 安装 | `modkit verify --mod <包>` → `modkit plan --client … --mod … --root …` → `modkit install …` | `plan` 退出码 **2 = 计划会阻断**，页面**到此停手**（不执行 install），把 plan 原文（含「冲突：」行）原样显示 |
| 卸载 | `modkit uninstall --dry-run`（先看会还原什么）→ `modkit uninstall` | 逐字节还原：被它替换过的文件从备份还原，它新增的文件删除 |

包从哪来：

- `[包]` 行 → 就用那个 zip（引擎的正式分发形态）；
- `[已装]` 行 → **先找同 id 的 zip 成品包**（升级时就该装成品包）；找不到才退回**目录形态**
  （`--allow-dir`，引擎会在输出里明说这是"开发自测形态"）。

### 3.2 两道硬护栏（引擎给的，页面照抄展示）

1. **`DFO.exe` 正在运行 ⇒ 拒绝客户端侧写盘**：
   页面在点按钮时就先查一次进程（`snapshotProcesses`，Windows 走 Toolhelp 快照，口径与启动器
   `internal/proc.FindByName` 一致）：要写客户端（含 `client`/`pvf`/`resource` 层）的装/卸被**拒绝**，
   提示"请先退出游戏"；只有 `server` 层的 mod 不写客户端，照常放行。
   引擎里还有同一道门（`InstallLayers`/`UninstallLayers`），所以即使页面漏判，
   引擎也会拒绝，而它的原文会**原样**出现在步骤面板里。
2. **计划被阻断（退出码 2）⇒ 整体拒绝**：典型是"同一个目标文件被两个 mod 声明"、
   "目标已存在且内容不同（`file.add` 不允许覆盖）"、"整文件哈希门不过（不认识的 EXE）"。
   页面不提供"强行覆盖"的开关——要改现场请手工处理后再装。

### 3.3 升级（同 id 换版本）

引擎按"**先按备份还原旧条目，再重放新内容**"处理（这就是为什么同一个 `file.add` 目标
在升级时不再被判成"覆盖别人的文件"）。页面只是把这段说明与原文显示出来；
`exe.patch` 的升级同理：先还原原始 EXE，再过整文件哈希门，卸载时逐字节还原。

### 3.4 含 `server` 层的 mod

落位 = 把包内 `server/` 写进 `<服务端模块>/mods/<id>/` 并重写 `mods/zz_mods_gen.go`，
所以**必须重新编译服务端二进制**才会生效：

- 启动器点「开始游戏」时会就地编译（正常路线，不用你操心）；
- 也可以用 `server/Build-Server.ps1`；
- 页面上会在行内和结果面板里都标出"含 server 层：装/卸后要重新编译服务端"。

**改启用状态不用重编译**（那是运行期门禁，只写 `enabled.json`，重启服务端即可）。

### 3.5 缺引擎时

页面顶部会显示"找不到 modkit 引擎"，并出现一颗**「构建引擎」**按钮：
用 Go 在启动器仓里 `go build -o <页面目录>/modkit.exe ./cmd/modkit`（只在用户没设
`GOPROXY`/`GOPATH`/`GOCACHE`/`GOTOOLCHAIN` 时补默认值，默认值取整合包既有布局
`<整合包>/tools/{gopath,gocache}`）。编好了页面会自己接上，不用重启。

---

## 四、导入校验规则（对齐 `../MOD-DEVELOPMENT.md`）

导入时**先整体校验、再落盘**；任何一份清单不过，整批不落盘（不会留半个 mod）。

| 检查 | 依据 | 报错样例 |
| --- | --- | --- |
| zip 里有 `mod.json` | §1 包结构 | `zip 里找不到 mod.json（形态 A / 形态 B）` |
| `schema` 必须是 2 | §2 模板 | `清单是旧格式 schema=1：请先用 modkit migrate 转成 schema 2` |
| `id` 合法（小写字母/数字/`./-`，≤64） | §2 | `id 非法` |
| `name` / `version` 非空 | §2 | `缺少 name` |
| 至少声明一层，且**声明了就要有动作** | §2 | `client 层声明了但没有任何动作` |
| 声明 server 层必须有 `server/` 目录 | §4 | `声明了 server 层，但包里没有 server/ 目录` |
| 权限位覆盖所有动作 | §3 权限表 | `缺少权限位 client.file.write（理由：client 层有 file.add / file.replace）` |
| 动作引用的 `source` / `script` / `entries[].source` 必须在包里 | §2 | `引用的文件不在包里：client/a.txt` |
| 路径安全（不许绝对路径、不许 `..`） | §8 装/卸语义 | `source 路径不安全` |
| 同一个包里 id 不重复、落位目录名不重复 | — | `同一个包里出现重复 id …` |
| 包体积/条目数上限（zip ≤512MB、≤20000 条目、解包 ≤2GB、单 mod ≤1GB、一次 ≤100 个） | 防 zip 炸弹 | `解包总大小超过上限` |

导入落位：**多 mod 形态**用子目录名（嵌套时取最后一段）；**单 mod 形态**用清单里的 `id` 当目录名。
目标目录已存在时默认**跳过**；勾选"覆盖"才会先删后写。

> 注意：**这里的校验只是"导入（解包）"的前置**，权威校验器在引擎里
> （`internal/modkit` 的 `Manifest2.Validate`）——「安装」那一步会再跑一次 `modkit verify`，
> 用的是同一个真源。

## 五、zip 的两种形态

```
形态 A（单 mod，zip 根就是 mod 根）        形态 B（一组 mod，每个一个子目录）
pkg.zip                                  pkg.zip
├── mod.json                             ├── one/
├── client/…                             │   ├── mod.json
└── server/…                             │   └── client/…
                                         └── two/
                                             ├── mod.json
                                             └── client/…
```

同一个包里**混用两种形态会被拒绝**（避免歧义）。导出同上：选 1 个走 A，选多个走 B。

---

## 六、给启动器接入用的 API 契约

服务只绑本机，无鉴权（要外露请自己加反代/令牌）。所有响应都是 UTF-8 JSON（导出是 zip 流）。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/` | 返回内置单页 UI（`ui.html`） |
| `GET` | `/api/mods?page=1&size=20&q=` | 分页列表。响应：`{modsDir, note, total, page, size, pages, items:[Mod], env:Env}` |
| `GET` | `/api/mods` 的 `Mod.kind` | `installed`（mod 目录，可启停/可装）或 `package`（zip 包，可装/可导出） |
| `POST` | `/api/inspect` | multipart（字段 `file` = zip）→ **只校验不落盘**：`{candidates:[Candidate]}` |
| `POST` | `/api/import` | multipart（`file`、`dryRun=0/1`、`overwrite=0/1`）→ `{zip, dryRun, imported[], skipped[], candidates[]}`；校验失败返回 400 + `{error, candidates}` |
| `GET` | `/api/export?keys=a,b` | 下载 zip。单个 mod 目录 = 形态 A；单个 zip 包 = **原样字节拷贝**；多个（可混合）= 形态 B（包会解到 `<包名>/` 下，保证外层仍是合法形态 B） |
| `POST` | `/api/delete` | `{"keys":[…]}` → `{done:[…]}`；被依赖时 400 |
| `POST` | `/api/import-local` | `{"keys":[…], "overwrite":bool}` → 把 mods 目录里**已有的 zip 包**就地导入成目录（`[包]` 那行的「导入」按钮走它） |
| `POST` | `/api/enable` | `{"keys":[…]}` → `{done:[…], notes:[…]}`。**对 `package` 项 = 就地导入成目录并启用**（已有同 id 的已装目录就直接启用它），`notes` 里说明发生了什么 |
| `POST` | `/api/disable` | `{"keys":[…]}` → `{done:[…], notes:[…]}`；被**启用中**的 mod 依赖时 400；对 `package` 项**跳过并在 `notes` 说明**（包没有运行状态） |
| `POST` | `/api/install` | `{"key":"<相对路径>", "dryRun":false}` → `OpResult`。依次跑 `verify`→`plan`→`install`，页面据此渲染步骤面板 |
| `POST` | `/api/uninstall` | `{"key":"<相对路径>", "dryRun":false}` → `OpResult`。先 `uninstall --dry-run` 预览再真卸 |
| `POST` | `/api/engine/build` | 无请求体 → `{ok, exe, repo, goExe, run, output, error, envAdded}`。就地构建 `modkit.exe` |

> **键（key）不是 id**：键 = 相对路径（`qol.client-host` 或 `pkg/x.zip`），因为同一个 mods 树里
> "已装目录"和"它的 zip 包"可能同 id。请求体带 UTF-8 BOM 也能解析。

`Mod`：`{id, name, version, author, description, layers[], enabled, addedAt, dirName, kind, sizeBytes, problems[]}`
（`dirName` 就是接口里的 **key**）

`Candidate`：`{prefix, dirName, manifest, problems[], bytes, exists}`

`Env`：`{modsDir, launcherRoot(+来源), moduleDir, clientDir(+来源), modkitExe(+来源),
launcherRepo(+来源), goExe, clientRunning, clientProcs[], problems[]}`

`OpResult`：`{action, dryRun, ok, blocked, key, id, package, packageForm, steps[], notes[],
needsRebuild, error}`；
`steps[]` 里每一步是 `{name, detail, run:{cmd, output, exitCode, spawnErr}, ok, blocked, note}`
——`run.output` 就是**引擎原文**，页面不加工。

**返回码**：`200` = 引擎跑过了（成功 / 被阻断 / 失败都在 `steps` 与 `error` 里，页面照渲染）；
`400` = 请求本身不对；`409` = 前置检查拒绝（缺客户端目录/引擎、或游戏在跑要写客户端）。
`409` 的响应体同样是完整的 `OpResult`（**带 steps**），所以被拒时也能把"检查了哪几步"显示出来。

### 接入建议

1. 把 `store.go` 的三处换成启动器的实现，其余（HTTP 层与页面）不动：
   - `List()` → `internal/modkit` 的 `ListInstalledMods`（字段基本同名，补一个 `addedAt` 即可）；
   - `SetEnabled()` → `SetModEnabled`（同样的"禁用名单"语义与 `updatedBy` 留痕）；
   - `Inspect()/Import()` 的清单校验 → `LoadPackage` + `Manifest2.Validate`（**权威**）。
2. **装/卸**：直接调 `InstallLayers`/`UninstallLayers`（或继续用 `cmd/modkit` 的 CLI 契约），
   把 `Emit`/`Lines` 的输出喂给同一份步骤面板即可——本页面的 `engine.go` 就是"CLI 契约"版实现。
3. 前端是纯静态 `ui.html`，可以直接嵌进启动器的内嵌浏览器；把 `fetch("/api/…")` 的前缀换成你们的
   内部 IPC/HTTP 路由即可。
4. `enable/disable` **不是热生效**（服务端启动时一次性注册钩子），界面上已经写明"要重启服务端才生效"。

---

## 七、已知边界

- **装/卸只走引擎**：页面不自己写客户端/服务端文件，也不提供"忽略冲突强行装"的开关；
- **引擎是外部可执行文件**：没编出来时页面只能给出构建入口（需要 Go 与启动器仓源码都在本机）；
- 删除是**直接删文件/目录**（不可撤销；界面里有二次确认），被依赖时拒绝；
- zip 包的"启用"=**就地导入成目录**（业主 2026-10-06 反馈"点启用不生效"后定的语义）；
- **沙箱/权限提醒**：如果 `modkit-web.exe` 放在工作区内、而 `--mods-dir` 指向工作区**外**
  （例如客户端目录），Windows 沙箱可能拒绝写入（表现为 `mkdir … Access is denied`）。
  把页面指向工作区内的目录（如 `115\mods`、`115\server\work\dfo-lan\mods`）不会有这个问题；
- **装/卸与真实客户端/服务端目录**：页面只调用引擎，引擎按注册表（`<客户端>/.launcher-mods/modkit/`）
  判断"装没装、能不能还原"。**不是用 modkit 装的 mod 卸不掉**（引擎会明说"尚未通过 modkit 安装"）；
- 停用/删除的依赖检查只覆盖**同目录内**已装 mod 的 `requires`；
- 路径与体积都有上限与安全校验（见 §四），但仍建议只绑本机、只对本机可信目录使用；
- 中文文件名在 zip 里的编码沿用 Go `archive/zip` 的默认行为（UTF-8 名字正常）。

---

## 八、测试

```powershell
$go = "C:\Game\dof\115us\tools\go\bin\go.exe"
cd C:\Game\dof\115us\115\mods\modkit-web
& $go vet ./...
& $go test ./... -count=1   # 形态识别、清单校验 10 例、路径穿越、重复 id、导入/导出往返、启停与依赖、zip 包项、删除
                            # + 引擎层：路径来源（读启动器配置）、游戏在跑就拒绝、预演放行、计划被阻断就停手、
                            #   装/卸的命令行与顺序、缺引擎的提示、同 id 优先用成品包
& $go build -o modkit-web.exe .
```

装/卸的真路径（真的写客户端 / 真的编译服务端）需要**游戏没在运行**，而且会改盘，
所以用例里把"引擎子进程"与"进程探测"换成假的（`Server.Run` / `Server.Procs`），
只测页面自己的判决与展示；引擎那一侧的落位/还原/升级语义由
**启动器仓 `internal/modkit` 的用例**负责（`layer2_test.go`、`upgrade2_test.go`、`guard_test.go`）。
