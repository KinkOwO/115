# next62 — 章节盒掉落 + 七章奖励补发（手册 P3 子项 3/4）

## 0. 真源

`contents/2026/aradodyssey/etc/aradodysseyjournal.cos`（在 `../client-build/Script.inner.pvf` 内）

| 项 | 值 |
| --- | --- |
| `data_type` | **3（UTF-16）** ⇒ 用 `Archive.ReadText` 解码；`ReadRaw` 单独取字节算哈希 |
| `raw_sha256` | `d4654fa9a50ddd582077f5f7a0a19835ec4fb6b777f032d1a66be7f288eff67e`（与手册 `d4654fa9…` 一致） |
| 结构 | `[chapter]` × 7 → 每章 `[reward]` + 若干 `[node]` → 每个 `[node]` 内若干 `[dungeon]` |

### 0.1 两个必须踩过的坑

1. **`[dungeon]` 有 `[type]` 子字段**。9 个使徒副本长这样：

   ```
   [dungeon]
    [type] `apostle`
    [index] 100004938
    [video] …
   [/dungeon]
   ```

   只匹配「`[dungeon]` 紧邻 `[index]`」会数出 **41** 个副本（12+8+4+8+6+5+7 里漏掉使徒），
   而真值是 **50** —— 与手册的「必须 50 个副本」一致。

2. **副本顺序不是 id 升序**。第 5 章的源码顺序是
   `100004968, 100004967, 100004969, 100004970, 100004971, 100004972`。
   手册说 `Final = 该章最后一个副本`，所以必须按**出现顺序**取，不能排序。

### 0.2 导出结果（7 章 / 50 副本 / 15 模板 / 25 行）

| 章 | 副本数 | Final | `[reward]` 首个（章节盒） | 其余奖励行 |
| --- | --- | --- | --- | --- |
| 1 | 12 | 100004945 | **10417792** | 10419741, 10419342, 10419346 |
| 2 | 8 | 100004953 | **10419742** | 10419745, 10419346 |
| 3 | 4 | 100004957 | **10417793** | 10417796, 10419745, 10419346 |
| 4 | 8 | 100004966 | **10417794** | 10419743 **×2**, 10419745, 10419347 |
| 5 | 6 | 100004972 | **10419744 ×2** | 10419745, 10419347 |
| 6 | 5 | 100004977 | **10417795** | 10419744, 10419745, 10419347 |
| 7 | 7 | 100004990 | **（无，template=0）** | 10419538, 10419539, 10419745 |

- 6 个章节盒的类型是 `[booster selection]`；第 7 章那两盒是普通 `[booster]` ⇒ 该章没有章节盒。
- 数量 >1 的只有 Ch4 `10419743 ×2` 与 Ch5 `10419744 ×2`（= 手册原话）。
- 跨章重复为 0。

## 1. 导出工具

`cmd/odysseychapterimport`（只读）：

```
go run ./cmd/odysseychapterimport -source ../client-build/Script.inner.pvf \
    -chapters configs/odyssey-chapters-candidate.json \
    -drop configs/odyssey-chapter-drop-candidate.json
```

对源逐条硬断言，任一条不满足即 `log.Fatal`：journal sha、7 章、50 副本、跨章不重复、
`Final` == 该章最后一个副本、15 个去重模板。章节盒的判定是「该章 `[reward]` 里**第一个**
`[booster selection]` 模板」，因此第 7 章自然得到 `0`（而不是硬编码特例）。

## 2. 子项 4：七章奖励按进度补发

### 2.1 目录

`internal/catalog/odyssey_chapters.go`：

```go
type OdysseyChapter struct {
    Number   uint8; Dungeons []uint32; Final uint32; Rewards []ChapterReward
}
func LoadOdysseyChapters(path string) (*OdysseyChapters, error)
func (c *OdysseyChapters) ChapterOf(dungeon uint32) (uint8, bool)
func (c *OdysseyChapters) At(number uint8) (OdysseyChapter, bool)
func (c *OdysseyChapters) IsFinal(dungeon uint32) bool
```

`LoadOdysseyChapters` 校验 model / source checksum / `definition_sha256`（钉死 journal）、
7 章、50 副本、`Final != 0` 且等于该章最后一个副本、无跨章重复、每行奖励模板与数量非 0。

### 2.2 发放

`internal/character/odyssey_chapter.go`：

- `ApplyOdysseyChapterReward(role, chapter, line, reward)`：catalog 里**只放这一行**的模板，
  所以过期或被篡改的目录无法让服务端发出日志里没有的东西；非奥德赛角色、越界行号、
  行号与内容错配都拒绝。
- `OdysseyChapterRewards(ctx, role)`：用**已有**的 `odysseyCompleted(role)`（读角色 state 的
  `odyssey_completed_dungeons` + `dungeon_best_times`，都是**服务端自有**成绩，与
  `OdysseyCatchup`/`OdysseyGifts` 同源），凡某章 `Final` 在通关集合里就发该章所有行。
  逐行独立事件键 `odyssey-chapter-reward:<章>:<行>:<模板>` ⇒ 满包只把该行留欠，
  下次登录/通关重试；已发过的行不会重发。

装载 `DFO_ODYSSEY_CHAPTERS`；调用点与 `OdysseyGifts` 并列（`cmd/wireprobe/main.go` 选角链、
`cmd/wireprobe/dungeon_flow.go` 通关结算）。

## 3. 子项 3：章节最终领主的章节盒

`internal/loot/odyssey_chapter_drop.go`：

```go
func LoadOdysseyChapterDrop(path string) (*OdysseyChapterDrop, error)
func (d *OdysseyChapterDrop) Enabled() bool
func (d *OdysseyChapterDrop) DropFor(dungeon uint32) (ChapterDropLine, bool)
func (d *OdysseyChapterDrop) Roll(seed, dungeon uint32) ([]Award, uint32, error)
func (d *OdysseyChapterDrop) StorageCatalog(catalog.LootCatalog) catalog.LootCatalog
func (d *OdysseyChapterDrop) BagRules(inventory.BagRules) inventory.BagRules
func (d *OdysseyChapterDrop) ValidateBoxes(*catalog.SelectionBoxes) error
```

- **出厂整表 `enabled=false`**，由 profile 显式开启（手册要求）。
- **禁用行完全惰性**：`Roll` 原样返回种子，**不消耗掷骰**，因此将来开启不会挪动同一次运行里
  任何其它掉落（这条有专门的测试锁住）。
- `ValidateBoxes` 要求启用行的模板**真的是装备自选盒**（查 `SelectionBoxes`），
  禁用行不校验（可以合法地是 `template=0`）。
- 命中点 `internal/loot/session.go` 死亡结算：`monster.Rank == 3` 且该副本是某章 `Final`。
  Session 上的 `ChapterDrop` 由 `dungeon_flow.go` 从 `loot.Service` 复制（与 `Currency` 同处）。
- 装载 `DFO_ODYSSEY_CHAPTER_DROP`；开启时才叠加 `StorageCatalog` / `BagRules`。

## 4. 测试

- `internal/character/odyssey_chapter_test.go`
  - 目录形状：7 章 / 50 副本 / `Final` 一致 / 章节归属 / **第 5 章源码顺序**未被排序破坏
  - 发放：Ch5 首行按数量发 **2 个**（一叠）；非奥德赛角色 / 越界行号 / 不存在的章 /
    与日志不符的行 / 行号与内容错配 全部拒绝
- `internal/loot/odyssey_chapter_drop_test.go`
  - 出厂状态：7 行、全部 `enabled=false`、第 1..6 章有盒、第 7 章 `template=0`
  - 禁用惰性：任何种子、任何行、含非 final 副本，都返回**原种子且无掉落**
  - 启用后：`Final` 上必掉（rate=10000），非 final 仍惰性，`rate=0` 等价于关
  - 坏表拒绝：6 行 / enabled 却无模板 / enabled 却 rate=0 / 校验和错
- `go build`、`go vet`、`go test -count=1 ./internal/... ./cmd/...`（**20 包**）全绿。

## 5. 实机验证要点（已完成）

`test-jh`（id=10）已有 **35 条**通关成绩，`Final` 命中第 1..4 章
（`100004945 / 100004953 / 100004957 / 100004966`）。重启服务端后重选该角色，实际结果：

| 章 | 行数 | 内容 | 落格 |
| --- | --- | --- | --- |
| 1 | 4 | 10417792, 10419741, 10419342, 10419346 | 67 / 68 / 69 / 70 |
| 2 | 3 | 10419742, 10419745, 10419346 | 71 / 72 / (70 叠加) |
| 3 | 4 | 10417793, 10417796, 10419745, 10419346 | 73 / 74 / (72) / (70) |
| 4 | 4 | 10417794, **10419743 ×2**, 10419745, 10419347 | 75 / **76 (x2)** / (72) / 77 |

- `character_events` 里 `odyssey-chapter-reward:*` **恰好 15 条**，`chapters=4`；第 5-7 章 0 条。
- 背包 11 格 / 16 个盒子；跨章共用的 `10419346` 与 `10419745` 各叠成 **3 个**，
  Ch4 的 `10419743 ×2` 落成**一格 2 个** —— 与手册"数量为 2 的行按数量发放"一致。
- 子项 3 的章节盒**按设计未掉落**：出厂 `enabled=false`，且 `channel_probe.py` 不注入
  `DFO_ODYSSEY_CHAPTER_DROP`。要验证需显式设该变量并把某一章改为 `enabled=true`。

## 6. 遗留

- 手册 P3 其余子项：8/9/10（毕业转普通角色 + 按等级主线整理 + 毕业时机）、
  子项 5（银币/金币掉落显式化，手册自认本就生效）。
- `[waste]` 槽位范围取证（决定创建补给药水是否与初始那 73 瓶合并成一叠）。
