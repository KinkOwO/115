# 交接：启动器侧补「gopath-mod 分片拼装」+ 常量同步

日期：2026-10-05 · 发起：115 主仓库（= 发布源）· 目标仓库：`../115us-dfolauncher`
适用对象：正在改启动器编译链（`internal/toolchain`、`internal/buildenv`、`internal/prebuilt`）的会话

---

## 1. 一句话背景

发布源里的依赖缓存包 `tools/tools-gopath-mod.zip` 已从「缺依赖」补成「**模块图全覆盖**」（62.7 MB），
但**单次 git 推送会被远端断开**，所以仓库里只存它的 4 个分片。
启动器必须能：「**取不到整包 → 取分片 → 拼装 → 按清单校验 → 解压**」，否则玩家端点「从源码编译」在断网时依然失败。

## 2. 发布源（主仓库）已就绪的事实

清单 `tools/manifest.json` 的 `gopath-mod` 条目**仍描述成品**（未改 schema）：

| 字段 | 值 |
| --- | --- |
| `url` | `tools/tools-gopath-mod.zip` |
| `size` | `65731473` |
| `sha256` | `8d35b234865064a9b49ef93b49e7334d5a1e89d866730fc86eb686dbb1f1d9b9` |
| `files` | `387` |
| `check` / `target` | `tools/gopath/pkg/mod` / `tools` |

仓库里**实际存在的是分片**（成品不入库，只在本地/离线包里）：

```
tools/tools-gopath-mod.zip.part01   16,777,216 B
tools/tools-gopath-mod.zip.part02   16,777,216 B
tools/tools-gopath-mod.zip.part03   16,777,216 B
tools/tools-gopath-mod.zip.part04   15,399,825 B
合计                                65,731,473 B   （= manifest 的 size）
```

- **参考实现（语义照抄即可）**：主仓库 `scripts/assemble-gopath-mod.ps1` —— 按名称顺序拼接、
  用 manifest 的 `size`+`sha256` 自校验、**幂等**（已就绪则零写入）、失败给出期望/实际。
- **已实测**：从 **git 对象**（不看工作区）拼出的成品 = `65,731,473 B` / `sha256 8d35b234…`，
  与 manifest 完全一致，387 条目。
- **离线包已自带**：`一键打包.cmd` 现在把 `tools/*.zip` + `tools/manifest.json` 放进离线包，
  且成品由 `$PrebuiltFiles`（工作区带进包）随包分发 —— 所以**拿到离线包的玩家不需要下载/拼装**；
  分片拼装只服务「启动器从发布源下载」这条路。

## 3. 需要在启动器侧做的两件事

### 3.1 常量同步（`internal/buildenv/buildenv.go`）

- `gopathDownloadBytes = 28881300` → **`65731473`**（下载量估算）。
- `gopathDiskMB = 217` **保持**：解压出的只是 `cache/download`（≈66 MB），Go 首次编译再按需解出模块目录，
  总常驻仍是 200 MB 量级。
- 同文件注释里对账那句「下载 67 + 28 + 2 = 97 MB」应改成 **67 + 66 + 2 = 135 MB**（业主概数需重新对一次）。

### 3.2 取包路径支持分片（`internal/toolchain`）

- 位置：`ensureNamed` / `EnsurePackages` / `EnsurePackagesForce` / `RefreshPackagesTo` **共用的下载层** ——
  这样 `--build-server=ensure`、`--build-server=build`、以及「更新时替换旧资源」三条路一起受益。
- 规则（**约定式，不必改清单 schema**）：
  1. 先按清单 `url` 取整包；成功 → 照现有的 size/sha256 校验 + 解压（现状不变）。
  2. 整包取不到（404/不可达）→ 依次取 `url + ".part01"`、`".part02"`…（**两位补零**），
     直到取不到为止（建议加硬上限，如 16 片，防呆）。
  3. 按**名称顺序**拼接；**总字节数必须等于清单 `size`、sha256 必须等于清单 `sha256`**，
     否则明确报错（错误信息里给出期望值与实际值）。
  4. 拼装产物放临时目录即可（解压后丢弃）；愿意的话可缓存，避免每次重下。
- **不变量（别踩）**：`guard_test.go` 断言「开始游戏」路径里不出现任何编译调用 ——
  **拼装只属于 ensure/build 路径**，不得进启动路径。
- 建议测试（可复用 `playersim_test.go` 的假包源harness）：
  - 假源只提供分片 → 断言拼装成功、size/sha256 校验通过、解压后 `tools/gopath/pkg/mod` 存在、`HaveModCache=true`；
  - 只提供 2/4 片 → 明确报错，不静默半成品；
  - 片内容改一位 → 哈希不符、明确报错；
  - 整包存在时行为与今天完全一致（零回归）。

## 4. 同一仓库里另一件需要口径确认的事

`../115us-dfolauncher/bin/server/work/dfo-lan/bin/` 下有启动器自带的旧副本：

```
wireprobe-pvf.exe  36,888,576 B  C32C46C30AAB22D6…
dfolauncher.exe    24,857,600 B  44CC249ED83F52F2…
```

它们是 **PostgreSQL 移除之前**的构建。2026-10-05 **21:07:58-59** 游戏树里的同名文件曾被整体还原成这两个哈希
（把含「PG 移除 + 誓约直入修复」的发布版换掉了；详见主仓库提交 `c7c9cc7f` 与 `CHANGELOG`）。
游戏树现在已是含修复的那对（`08E62F97…` / `1B18A69D…`）。
**这一侧的副本要不要一起刷新，请与业主确认** —— 主仓库这边不会去动你们的仓库。

## 5. 验收口径（主仓库已做到的对照）

- 干净环境 + 只用 `go` / `gopath-mod` / `server-src` 三个包 + **全断网** `GOPROXY=off`：
  `go build ./...` **exit 0**、`go vet ./...` **exit 0**、
  `go build -trimpath -o bin/wireprobe-pvf.exe ./cmd/wireprobe` **exit 0**（28,627,968 B）、
  `go build -trimpath -o bin/dfolauncher.exe ./cmd/dfolauncher` **exit 0**（16,474,624 B）。
  （与主仓库内构建 28,633,600 B 的差异只来自源码包无 `.git` ⇒ 无 `vcs.revision` 元数据。）
- 启动器侧建议验收：断网（或临时屏蔽发布源）时 `--build-server=ensure` → `--build-server=build` 仍成功。

## 6. 主仓库本次相关提交（便于你们对照）

```
9f46d144  补全 gopath-mod 依赖缓存（模块图 51 个模块全覆盖）+ 分片存储 + 编译时自动拼装
35f6d704 / c341d960 / 96705938 / 2777f332   gopath-mod 分片 1/4 … 4/4（逐片提交逐片推送）
34872d9f  离线包加回 tools（自带编译能力），打包前自动拼装依赖缓存整包
c8058562  本交接文档
81fb00cc  重打 configs 发布包（按跟踪集）+ 清掉 27 个陈旧产出物
```

## 7. 顺带交接：4 个本地化 patch 的字节（本仓库已不再分发）

背景：按业主定调「这边不要」，本仓库把这 4 个本地化 patch 从 `configs/` **删除**，并**按当前跟踪集重打**了
`tools/tools-server-configs.zip`（条目 69 → 68，sha256 `6e5b2483…` → `8161129f…`，见主仓库提交 `81fb00cc`）。
它们**从未被正式跟踪过**（git 历史里没有「加入」记录），过去只是被卷进了 configs 包；启动器侧对它们的
引用与通配均为 **0**。

| 文件 | 大小 | sha256 |
| --- | --- | --- |
| `cera-contract-format.patch.json` | 215 B | `d7ad544396c965220ab5cde842017b6bb80432530de3931054608ef7820cc6a8` |
| `character.names.patch.json` | 9,178 B | `df7c090caae3fa79e186546efdf03350083da81595249e95ca813881532a1708` |
| `item.names.client.patch.json` | 71,362 B | `82d425429af107fd9cb9fbf802b62c4023c346067ed66d0bac31baff5418c884` |
| `pvf.localization.patch.json` | 14,148 B | `d179b2599270c808058065ee4d17f0584bd5dedd16fd68bd772df0c19f1e2969` |

**取字节的两条路**（两条路的内容已逐字节核对一致）：

1. **从 git 历史里的旧 configs 包取出**（推荐：任何 clone 都能取，不依赖额外载体）：

   ```powershell
   git show c8058562:tools/tools-server-configs.zip > old-configs.zip
   # 解开后取 configs/ 下的这 4 个 *.patch.json
   ```

   注意：`c8058562` 是重打之前最后一个含它们的提交；**`81fb00cc` 起的新包不再含它们**。
2. 业主机上留了一份稳定副本：`<主仓库>\.tmp\localization-patches-handover\`
   （`.tmp/` 被 `.gitignore` 覆盖，不入库）。

**给接收方的建议**：这 4 个是**本地化工具链的产出物**（不是输入）。最稳妥的做法是让那套工具
（启动器仓库的 `tools/translate_*.py`、`pack_localization*.py` 等）**自己携带或重新生成**，
而不是长期靠人肉搬运；若确定要纳入版本管理，请放在工具自己所属的仓库里（本仓库口径为「这边不要」）。

