# modkit-web —— modkit 的简易 Web 页面

一个**独立的小工具**：本地起一个小服务，用浏览器管理一个 mods 目录 ——
分页列表、多选、批量启用/停用/删除、导入/导出 zip（支持单 mod 与多 mod 两种包形态）。

> **它不接启动器，也不依赖启动器代码。** 以后要接进启动器，按下面的
> [API 契约](#五给启动器接入用的-api-契约) 对接即可；数据层可以整体换成启动器里的
> `internal/modkit`（那里的 `Manifest2.Validate` 才是**权威校验器**，这里是按文档实现的同构子集）。

规则来源：`../MOD-DEVELOPMENT.md`（包结构 / 清单模板 / 权限位 / 层动作）与
`../MOD-MANAGER-INTEGRATION.md`（`enabled.json` 的确切格式与语义）。

---

## 一、跑起来

**最省事：双击 / 直接运行 `run.cmd`**（默认就是"管 `115\mods`、认两层、自动开浏览器"）：

```powershell
cd C:\Game\dof\115us\115\mods\modkit-web
.\run.cmd
# 想换端口或目录就带参数（带参数时不再套用默认值）：
.\run.cmd --addr 127.0.0.1:9000
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

### 两个 mods 目录别混（`../README.md` §目录职责）

| 路径 | 是什么 | 该用哪个 depth |
| --- | --- | --- |
| `server/work/dfo-lan/mods/` | **服务端真正加载的目录**（已装 mod、`zz_mods_gen.go`、`enabled.json`） | `1` |
| `115/mods/`（仓库根那个） | **mod 作者工作区**：文档 + `examples/<mod>/` + 可分享的成品包 | `2` |

---

## 二、页面功能

- **分页列表**：每页 10/20/50/100，带总数与页码；列 = 复选框、**mod 名称**（含 id/版本/层）、
  **添加日期**、**启用开关**、**制作人**、大小与相对路径；
- **列表项有两种**：
  - `[已装]`／**mod 目录**（`<mods>/…/<mod>/mod.json`）—— 可以启停；
  - `[包]`／**zip 包**（`<mods>/…/xxx.zip` 里含 mod.json，如 `client-mods/` 里的成品包）
    —— 可以导出、删除，那一行给的是**「导入」按钮**而不是开关：点它把包就地解成目录，
    之后就有真正的启停开关了。同一个 id 的"包"和"已装目录"可以同时存在
    （例如 `pkg/qol.client-host-1.0.0.zip` 与 `qol.client-host/`），
    所以界面与接口都用**唯一键 = 相对路径**来标识一项，而不是 id；
- **多选**：行复选框 + 表头全选；底部显示"已选 N 项"；
- **批量操作**：导入 mod（zip）、导出选中、启用选中、停用选中、删除选中；
- **搜索**：按 id / 名称 / 作者 / 说明过滤（输入即筛，250ms 防抖）；
- **导入前先校验**：选好 zip 后先调 `/api/inspect` 做**干跑校验**，把每个 mod 的
  通过/问题逐条列出来；**全部通过**才允许点"校验通过，导入"（已存在的目录要勾选"覆盖"）；
- **导出**：选中 1 个 → 包里**根目录就是 mod 根**；选中多个 → 包里**每个 mod 一个子目录**（两种都能被本工具重新导入）。

"添加日期"取的是 mod 目录的 mtime（导入/落位时间），没有额外元数据文件。

---

## 三、导入校验规则（对齐 `../MOD-DEVELOPMENT.md`）

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

## 四、zip 的两种形态

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

## 五、给启动器接入用的 API 契约

服务只绑本机，无鉴权（要外露请自己加反代/令牌）。所有响应都是 UTF-8 JSON（导出是 zip 流）。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/` | 返回内置单页 UI（`ui.html`） |
| `GET` | `/api/mods?page=1&size=20&q=` | 分页列表。响应：`{modsDir, note, total, page, size, pages, items:[Mod]}` |
| `GET` | `/api/mods` 的 `Mod.kind` | `installed`（mod 目录，可启停）或 `package`（zip 包，不可启停） |
| `POST` | `/api/inspect` | multipart（字段 `file` = zip）→ **只校验不落盘**：`{candidates:[Candidate]}` |
| `POST` | `/api/import` | multipart（`file`、`dryRun=0/1`、`overwrite=0/1`）→ `{zip, dryRun, imported[], skipped[], candidates[]}`；校验失败返回 400 + `{error, candidates}` |
| `GET` | `/api/export?keys=a,b` | 下载 zip。单个 mod 目录 = 形态 A；单个 zip 包 = **原样字节拷贝**；多个（可混合）= 形态 B（包会解到 `<包名>/` 下，保证外层仍是合法形态 B） |
| `POST` | `/api/delete` | `{"keys":[…]}` → `{done:[…]}`；被依赖时 400 |
| `POST` | `/api/import-local` | `{"keys":[…], "overwrite":bool}` → 把 mods 目录里**已有的 zip 包**就地导入成目录（`[包]` 那行的「导入」按钮走它） |
| `POST` | `/api/enable` | `{"keys":[…]}` → `{done:[…], notes:[…]}`。**对 `package` 项 = 就地导入成目录并启用**（已有同 id 的已装目录就直接启用它），`notes` 里说明发生了什么 |
| `POST` | `/api/disable` | `{"keys":[…]}` → `{done:[…], notes:[…]}`；被**启用中**的 mod 依赖时 400；对 `package` 项**跳过并在 `notes` 说明**（包没有运行状态） |

> **键（key）不是 id**：键 = 相对路径（`qol.client-host` 或 `pkg/x.zip`），因为同一个 mods 树里
> "已装目录"和"它的 zip 包"可能同 id。请求体带 UTF-8 BOM 也能解析。

`Mod`：`{id, name, version, author, description, layers[], enabled, addedAt, dirName, kind, sizeBytes, problems[]}`
（`dirName` 就是接口里的 **key**）

`Candidate`：`{prefix, dirName, manifest, problems[], bytes, exists}`

### 接入建议

1. 把 `store.go` 的三处换成启动器的实现，其余（HTTP 层与页面）不动：
   - `List()` → `internal/modkit` 的 `ListInstalledMods`（字段基本同名，补一个 `addedAt` 即可）；
   - `SetEnabled()` → `SetModEnabled`（同样的"禁用名单"语义与 `updatedBy` 留痕）；
   - `Inspect()/Import()` 的清单校验 → `LoadPackage` + `Manifest2.Validate`（**权威**）。
2. 前端是纯静态 `ui.html`，可以直接嵌进启动器的内嵌浏览器；把 `fetch("/api/…")` 的前缀换成你们的
   内部 IPC/HTTP 路由即可。
3. `enable/disable` **不是热生效**（服务端启动时一次性注册钩子），界面上已经写明"要重启服务端才生效"。

---

## 六、已知边界

- **只写 `enabled.json`**，不碰 `zz_mods_gen.go`、不编译、不触发 modkit 的装/卸流程；
  需要"装进客户端 / 参与服务端编译"的完整流程仍然走 `modkit install`；
- 删除是**直接删文件/目录**（不可撤销；界面里有二次确认），被依赖时拒绝；
- zip 包的"启用"=**就地导入成目录**（这是业主 2026-10-06 反馈"点启用不生效"后定的语义），
  但页面**不会**替你装进游戏客户端 —— 那一步仍然走 `modkit install`（且要求客户端没在运行）；
- **沙箱/权限提醒**：如果 `modkit-web.exe` 放在工作区内、而 `--mods-dir` 指向工作区**外**
  （例如客户端目录），Windows 沙箱可能拒绝写入（表现为 `mkdir … Access is denied`）。
  把页面指向工作区内的目录（如 `115\mods`、`115\server\work\dfo-lan\mods`）不会有这个问题；
- 停用/删除的依赖检查只覆盖**同目录内**已装 mod 的 `requires`；
- 路径与体积都有上限与安全校验（见 §三），但仍建议只绑本机、只对本机可信目录使用；
- 中文文件名在 zip 里的编码沿用 Go `archive/zip` 的默认行为（UTF-8 名字正常）。

---

## 七、测试

```powershell
$go = "C:\Game\dof\115us\tools\go\bin\go.exe"
cd C:\Game\dof\115us\115\mods\modkit-web
& $go vet ./...
& $go test ./... -count=1   # 形态识别、清单校验 10 例、路径穿越、重复 id、导入/导出往返、启停与依赖、zip 包项、删除
& $go build -o modkit-web.exe .
```
