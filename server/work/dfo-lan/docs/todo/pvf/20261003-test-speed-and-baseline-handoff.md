# 交接：测试提速与旧基线收敛（2026-10-03）

> 范围：`server/work/dfo-lan` 的**测试提速**与**旧 JSON 基线收敛**。未改协议/SQL schema/玩家存档/客户端资源；未替换运行程序；未跑整包全量回归（按业主要求）。
> 相关 CHANGELOG：仓库根 `CHANGELOG` 2026-10-03 条目。

## 1. 背景与根因

原生模式（设 `DFO_PVF_CORE_TEST_ARCHIVE`）下 `go test ./...` 既慢又大量失败：

1. **慢**：每个测试包进程各自校验 726 MiB 归档；联合物品扫描（`ImportItemBasics`）逐条解码约 **599,771** 条脚本；UTC-16 文本解码占 ~45% CPU。
2. **失败**：内层归档当前为 **8b2a**，而大量测试/夹具仍对照旧源 **7ef2**（`characters.skycastle-release.json`、`items.index.json`、`equipment-full`、各 overlay JSON 等）。

## 2. 已完成（按提交）

| 提交 | 内容 |
|---|---|
| `2d7b7c1` | 共享缓存归档（`OpenReadOnlyCached` + 包级 `TestMain`/`sync.Once`）+ 定向导入（`LoadNativeLootTables` / `LoadNativeDungeons`） |
| `967a0f9` | gamedata 去冗余旧角色锚点（解锁 ~38 个 FAIL）+ `DerivedCacheDir` 共享 |
| `de3ccb7` | inventory equipment parity 抽样 |
| `7ac2d81` | managementdata parity → `DFO_PVF_VERIFY_BASELINES=1` 门禁 |
| `cbfb569` | catalog/pvf + catalog 重 parity 抽样；删除 3 个纯旧基线对照测试 |
| `03fa1dc` | runtime-details 逐条 item 循环抽样（93s→13s） |
| `39e7409` | `scripts/test_local.ps1` 单一入口（`-Full` 开关） |
| `12fea7b` | **读取提速**：并行联合扫描 + UTF-16 单趟解码 + 稳定测试缓存身份 + 持久派生缓存 |
| `57f67e0` | dungeon 4 个 overlay 测试改原生 `Import*/Apply*` |
| `b8a85ac` | 删除旧基线 `items_test.go` |
| `69e58c2` | gamedata 10 个：原生源锚点/当前源断言/删旧基线 |
| `d47b436` | quest `TestSeekMeetBossClearRequiresOwnedSourceAndItems` 改原生 `ImportQuests` |
| `60350a1` | CHANGELOG 收口 |

## 3. 核心机制（供后续维护）

- **并行联合扫描**：`internal/catalog/item_basics.go` `ImportItemBasics` → `prefetchItemScripts`（分批 1024、worker = GOMAXPROCS，并行 `ReadRaw`+`TokensFromRaw`）+ **串行按序消费**（保持 map/消费者写入顺序与错误语义）。生产启动准备同享。
- **UTF-16 解码**：`internal/catalog/pvf/decode.go` `decodeUTF16LE` 单趟直接编码 UTF-8（ASCII 快路径 + 手动 BMP/代理对 + `sync.Pool` 缓冲），未配对代理项产出 U+FFFD（同 `utf16.Decode`）。
- **测试缓存身份**：`internal/gamedata/derived_cache.go` `derivedParserIdentity` 支持 `DFO_PVF_CACHE_IDENTITY` 覆盖；`scripts/test_local.ps1` 设 `dfo-lan-test-cache-v1`，使解析投影跨编译复用。**生产仍用可执行哈希。**
- **抽样开关**：`DFO_PVF_ARCHIVE_FULL_SWEEP=1`（恢复重 parity/穷举全量）。
- **单一入口**：`pwsh -File scripts/test_local.ps1 [-Full] [pkg] [-Run Test]`。

## 4. 效果与验证证据

- 联合扫描 `TestJointComplexItemsLocalArchiveParity`：**52s → 40s(冷) / 21s(暖)**。
- cashshop 66→6s；inventory 98→28s；managementdata 129→25s；runtime-details 93→13s。
- 原生模式原报告 15 个 FAIL 全部转 PASS（dungeon 4 / gamedata 10 / quest 1）。
- `go build ./...`、`go vet ./...` 通过；`internal/catalog/pvf` 包测试通过（解码正确性）；联合扫描连续 3 次结果一致（`scripts=599771`）。

## 5. 已知限制 / 风险

- **未跑 `-race`**：本机无 gcc（`go test -race` 报 `-race requires cgo`）。并行扫描的并发正确性靠代码审查（只读查找 + `chunk` 互斥 + `sync.Pool`）与重复运行验证，**建议在带 C 编译器的环境补一次 `go test -race ./internal/catalog ./internal/gamedata`**。
- **未跑整包全量回归**：转绿依据是逐项定向测试；建议方便时补一次带 env 的 `go test ./...` 确认零失败。
- **冷态仍慢**：`TestDerivedItemCache`(125s)、`TestProjectionCaches`(104s)、`TestPVFDerivedCacheCombined`(91s) 是**故意冷启动**验缓存语义的测试，未接持久缓存。
- **workspace 并发**：会话期间另一 agent 在改 `cmd/wireprobe/*`（`client_connection*`/`client_dispatch*` 等新文件 + `main.go`/`go.mod`）。本批未触碰这些文件；提交时注意区分。

## 6. 下一步待办（建议顺序）

1. **补 `-race` + 全量原生套件确认**（见 §5）。
2. **继续 §0 内容真源收敛**（业主最初目标，oracle 方案 Batch 2–5）：
   - world / quest / progression 测试由旧 JSON 基线迁原生；
   - items.index.json(66MB) / booster-catalog.json(84MB) / equipment-full/*(363MB) / quests.generated.json(27MB) 的**工具/测试消费者**迁 PVF 直读，可回收 ~500MB+。
3. **冷缓存测试提速收尾**（§5 三个）。
4. **生产启动测量**：并行+解码优化后复测服务端准备耗时（此前记录 14–50s）。

## 7. 关键文件索引

- 读取/扫描：`internal/catalog/item_basics.go`、`internal/catalog/pvf/decode.go`、`internal/catalog/pvf/view.go`、`internal/catalog/pvf/read.go`
- 测试支持：`internal/catalog/native_test_support.go`、`internal/catalog/pvf/archive_sampling_test.go`、`internal/gamedata/catalogs_core_test.go`（`prepareCatalogsForTest` / `itemSample` / `testPVFCacheDir`）
- 缓存：`internal/gamedata/derived_cache.go`、`internal/gamedata/projection_cache.go`
- 运行器：`scripts/test_local.ps1`
- 迁移台账：`docs/todo/pvf/PVF单一内容真源改造计划.md`
