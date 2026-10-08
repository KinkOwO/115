# PVF 在 mod 中可扩展 —— 方案与取舍

> 业主 2026-10-07 口径：「**PVF 也应该在 mod 中允许被扩展**」。
> 本文只做**方案与证据**，不含实现；实现前需业主选定档位（见 §6）。

## 0. 业主已裁决（2026-10-07）

- **选档：Tier C** —— 要能**新增条目**（真扩展），不只改已有脚本。
- **前置条件：A** —— 允许**就地按本机三件套**（`DFO.exe` + `sk.dat` + `Script.pvf`）实现与验证，不要求先回到官方档位。

## 0.1 Tier C 可行性结论：**服务端路线成立；客户端可见那一半仍缺一块**

### 已实测的哈希流真相（**推翻本文早先的一版说法**）

早先版本写"哈希流 = `(pathHash(path), fileIndex)` 两列、与 `compact.go` 的 `directoryEntry` 同构" —— **实测证伪**。真档（`Script.inner.pvf`）实测：

| 量 | 实测值 |
| --- | --- |
| `hashSize` | 53,780,164（与 `4 + n*8 + 4 + m*4` 严格自洽） |
| `n` | 5,650,173（= 条目数） |
| `m` | 2,144,693（**≠ 条目数**） |
| qword 命中 `pathHash(archivePath)` 集合 | **0 / 200,000**（反向 0 / 200,000） |
| qword 样例 | `0x021f1b4d03ca0019`、`0x006158bf060d78d7`、`0x067a92af067af347` —— 形如**两个 u32 打包** |
| `m` 段取值范围 | min 2403 / **max 494,487,111**（≫ 条目数 5,650,173）⇒ **不是文件下标** |

⇒ 那套 32 位哈希算法与 `m` 段的语义**都不在代码库里**，属**未取证**。

### 但它不挡服务端路线（关键）

服务端的路径查找**只建在文件表上**，哈希流只被当作**偏移与长度**：

- `parse.go:283-287`：`pathIdx` 由文件表逐条填，**哈希流不参与查找**；
- `parse.go:65-87`：哈希流只做**尺寸自洽校验**；
- `l10n.go:394`、`compact.go:155`：只取偏移 / 只累加长度。

⇒ **只要目标内容是"服务端读得到的玩法脚本"，新增条目不必动哈希流**（原样拷贝即可），
也**不需要逆向**。§8.4 第 1~4 步全部落在这一侧 —— 这正是 §0.2"玩法一律从内层 PVF 现场解析"所对应的那半边。

### 仍缺的一块

**客户端可见**的新条目（§8.4 第 5 步）可能需要正确的哈希流才会被原生查找命中 —— **未取证**。
真档里有 5,650,173 组已知输入，可先用字典法试（crc32 各变体 / fnv / djb2 / murmur / xxhash…），
失败再考虑 IDA。**在拿到结论前，不承诺"客户端可见的新增条目"。**

### 其余拼图（这一版仍成立）

| 拼图 | 位置 | 作用 |
| --- | --- | --- |
| `encodePStringBuffer` | `l10n.go` | 重建 `strA`/`strW` 串池（zlib + `decryptPString` + 尺寸异或头） |
| 完整写出骨架 | `l10n.go:360-440` | 已会重写组表/body/头部/串池，并按 `hdr+fileTable+hashes+nameSection+groupBytes+body` 落盘 |
| `decryptProtected` | `crypto.go` | 流密码（加解同路），`iNfO`/`HSrm`/`Gidx`/`mAIn` 四个节段都靠它 |
| chunk 追加机制 | `rewrite.go:16` | 已验证过的"追加进 owning chunk + 重算累计长度" |
| 合成归档构造法 | `compact_test.go:41-96` | 六个节段的可跑样板（含"哈希流可以 n=0,m=0"） |

## 1. 现状：三条路，没一条是"条目级"

| 动作 | 形态 | 代价 / 硬约束 |
| --- | --- | --- |
| `pvf.replace` | 用包内 `Script.pvf` + `sk.dat` **成对整份替换** | 832 MB 级容器；与 `DFO.exe`/`sk.dat`/`Script.pvf` 三件套版本绑定（安装器有"已知哈希集"门禁） |
| `pvf.merge` | "调用 pvf-kit 套件把 delta 语义合并进客户端现役 PVF" | **盘上没有 pvf-kit 实体**（`layer.go:16/263` 只说"包里自带"）⇒ 内核实际不提供任何 PVF 合并能力；且它是脚本型动作，需 pwsh 7 |
| `pvf.verify` | 只读干跑校验 | 不改盘 |

对照 NPK 侧：`npk.entries` 已经做到**条目级覆盖**（只换指定 img，归档其余字节逐字节保留），并有 `npkdiff` 差分工具、整档快照备份、逐字节还原与五类回归护栏。
**PVF 侧缺的正是这一层。**

## 2. 已有但没产品化的能力（这是本方案的立足点）

### 2.1 写入原语已存在

`internal/catalog/pvf/rewrite.go`：

```go
func (a *Archive) WriteEntryCopy(output, path string, replacement []byte) error
```

注释原文：*"replaces one existing script in a NEW current-format inner archive. Paths/string pools/hash indexes are unchanged. **Append within the owning chunk**, retaining every other entry's original bytes and offsets. It never overwrites a file."*

**能力边界（逐条来自代码，不是推测）**：

| 判据 | 位置 | 含义 |
| --- | --- | --- |
| `format != FormatDFO20260901` | `:20` | 只认**内层**格式 `dfo_20260901_inner` |
| `a.readOnlyView` | `:17` | 服务端日常那条只读视图**不能写** |
| `len(replacement)%5 != 0` | `:20` | PVF 单元 **5 字节**（type + u32），替换体必须是 5 的整数倍 |
| `len(replacement) > 1MB` | `:20` | 单脚本上限 1 MB |
| `!ok \|\| f.DataType != 1` → `existing script required` | `:23-25` | **只能替换已存在的脚本，不能新增路径** |
| 输出 `O_CREATE\|O_EXCL` | `:81` | 永远写**新文件**，不覆盖原件 |

它改写的是 header 的 bodySize、groups 表的 compressedSize/originalSize、以及该文件的记录（原长/新长），其余部分整段照抄 ⇒ **结构上是"单条目追加式覆盖"，与 `npk.entries` 同构**。

### 2.2 反向 wrap 的算法已被吃透，只差产品化

- 生产代码**只有 `UnwrapOuter`（外层→内层）**，没有反向（`unwrap.go:78/99`）。
- 但测试里已经实现过：`unwrap_test.go:142 wrapOuter(inner, keys)` + `:158 aesCBCZeroIVEncrypt(key, buf)`，并有 `TestUnwrapOuterRoundTrip`。
- 外层格式（`unwrap.go:129-193`）：
  - 整份 `Script.pvf` 按 **10 MiB**（`outerBlockSize = 0xA00000`）分块；
  - 每块**前 `0x2800` 字节**用该段 **32 字节段密钥**做 **AES-CBC 零 IV**；
  - 首块前 48 字节再叠一层 `decryptProtected("iNfO", …)`，并验内层魔数 `nkpi`；
  - 段密钥由 `DFO.exe` 内嵌 RSA 私钥解 `sk.dat` 得到（`wrapperKeys`，`:199`）。

⇒ **wrap 是 unwrap 的逐块逆运算**，密钥派生代码现成，缺的只是"写出去"这一半。

### 2.3 内层格式结构（判断"能否新增条目"的依据）

`internal/catalog/pvf/archive.go:18-20` 与 `Archive` 结构体：

```
headerSize    = 0x30   头部
fileItemSize  = 0x18   每条文件记录
groupItemSize = 8      每个组（chunk）
另有：路径表 / 两个串池 strA·strW / 哈希索引 / bodyOff
```

文件记录是**连续表**（`headerSize + index*fileItemSize`）。**在中间插入一条记录会平移其后所有字节**，与"保留其余条目的原始字节与偏移"直接冲突 ⇒ **新增路径 = 重建整个内层布局**，不是小改动（见 §4 Tier C）。

## 3. 关键交互：内层 PVF 会被自动重生成

`internal/launcher/innerpvf.go`：

- 内层归档**不是重新打包的 PVF**，而是客户端 `Script.pvf` **去掉加密**（`:7-9`）。
- 门禁键 = `innerClientInputs = {DFO.exe, sk.dat, Script.pvf}`（`:55-57`），**任意一件变化都会得到不同的内层归档**。

**推论（本方案最重要的一条）**：

- 若 mod 改**外层** `Script.pvf` ⇒ 键变了 ⇒ 启动器**自动重新生成内层** ⇒ 服务端自动吃到 mod 的改动。
  **这是唯一能让"客户端与服务端同时看到"的路**，也是 Tier A 的依据。
- 若 mod 只改**内层** `Script.inner.pvf` ⇒ 下次客户端三件套任何一件变化（例如装了官方更新）⇒ **内层被重生成，mod 的改动被静默抹掉**。⇒ Tier B 不可作为正式交付形态。

## 4. 三档能力（请业主选一档）

| 档 | 能做到什么 | 需要新写什么 | 风险 |
| --- | --- | --- | --- |
| **Tier A：改已有脚本（客户端可见）** | mod 只带**几 KB 脚本字节**，装时改外层 `Script.pvf` 里的指定脚本；卸载逐字节还原 | ① 生产版 `WrapOuter`（算法已有）；② modkit 新动作 `pvf.entries`（对标 `npk.entries`：整档快照 + 注册表 + 还原）；③ `pvfdiff` 差分工具 | **中**：改长度会移动 10 MiB 块边界与段密钥对应关系，必须实测；整档 832 MB 备份/写入 |
| **Tier B：只改服务端内层** | 只影响服务端规则 | 无（原语现成） | **不可正式交付**：会被 §3 的门禁静默覆盖 |
| **Tier C：新增脚本/路径（真"扩展"）** | mod 能**加**新物品/装扮/商店定义，不必带整份 PVF | 内层格式的**写入器/重排**：文件记录表、路径表、`strA`/`strW` 串池、哈希索引、组表与 body 全部要重建 | **高**：要动 760 MB 容器的整体布局；写错就是全库不可读，需完整往返验证与备份 |

## 5. 无论选哪档都必须一起解决的约束

1. **三件套配套**：wrap/unwrap 都要 `DFO.exe` + `sk.dat`。本机实测 `DFO.exe` 哈希**不等于任何已发布档位**（`01c633df…` vs 安装器要求 `6c78cdf8…`），`Script.pvf` 也小 71 MB ⇒ 生成/回写的**前置条件**要先定：是"就地按本机三件套派生"，还是"必须先回到官方档位"。
2. **替换体限制**：5 字节对齐、≤1 MB、**只能已存在的路径**（Tier C 才能突破）。
3. **备份成本**：整档 832 MB/次（与 NPK 整档快照同类；业主已明确**不做**该侧优化）。
4. **`.gitignore`**：`Script.pvf`/`sk.dat` 体积与敏感性 ⇒ mod 包一律只留本机，不入库（与 NPK/字体同一口径）。
5. **不得静默失败**：装载前的干跑校验（`pvf.verify` 已有）必须在装了 Tier A 之后仍然通过。

## 6. 待业主裁决

1. **选档**：只要 Tier A（改已有脚本）？还是必须包含 Tier C（新增内容）？
   - 若目标是"把龙袍4 的 PVF 定义作为 mod 交付"，那**必须 Tier C**（那是新增条目）。
   - 若目标只是"让 mod 能改 PVF 里的既有规则"，**Tier A 就够**。
2. **是否接受新增一个生产版 `WrapOuter`**（会写回 832 MB 的客户端 `Script.pvf`）。
3. **前置条件**：是否允许在"本机三件套与官方档位不一致"的现状下实现与验证，还是先要求回到官方档位。

## 7. 未取证项（诚实标注）

- 客户端是否**校验** `Script.pvf` 的整体指纹（若校验，Tier A 写回后可能被客户端拒绝）。**未取证**。
- 段密钥条数与 10 MiB 块数的关系：改长度后块数可能超过密钥条数，末段是否按"无密钥即不加密"处理，**只从 `unwrapBlocks` 的 `index < len(keys)` 推断，未实测**。
- `hash indexes` 在内层中的实际用途与是否被读取方校验。**未取证**。

## 8. Tier C 实现规格（内层格式逐字来自 `parse.go` / `rewrite.go` / `l10n.go`）

### 8.1 布局总表

| 区段 | 偏移 | 长度 | 保护 | 备注 |
| --- | --- | --- | --- | --- |
| header | 0 | `0x30` | `decryptProtected("iNfO")` | `[0:4]` magic；`[24:28]` fileCount；`[32:36]` bodySize；`[36:40]` groupCount；`[40:44]` hashSize；`[44:48]` nameSize |
| 文件表 | `0x30` | `fileCount × 0x18` | **明文** | 每条：`[0:4]`nameOffset `[4:8]`pathOffset `[8:12]`chunkIndex `[12:16]`dataOffset `[16:20]`dataSize `[20:24]`dataType |
| 哈希流 | `0x30+fileCount*0x18` | `hashSize` | `decryptProtected("HSrm")` | `[i32 n][n×8B hash][i32 m][m×4B index]`，按 `(hash,index)` 严格有序 |
| 名字段 | 哈希流之后 | `nameSize` | 段自身不保护；**两个池各自**保护 | 前 8 字节前缀 + `strA` 帧 + `strW` 帧 |
| 组表 | 名字段之后 | `groupCount × 8` | `decryptProtected("Gidx")` | `[0:4]`compressedSize（**累计末尾偏移**） `[4:8]`originalSize |
| body | 组表之后 | `bodySize` | 每个 chunk 先 zlib 再 `decryptProtected("mAIn")` | 第 i 组占 `body[g[i-1].compressedSize : g[i].compressedSize)` |

池帧格式（`encodePStringBuffer` 逆向）：`[u32 len(encrypted)^xorConst][u32 len(plain)^len(encrypted)][encrypted=decryptPString(zlib(plain))]`；
`strA` 用 key `stAs` / xor `0xe7adf7ea`（UTF-8），`strW` 用 `stWs` / `0xb8dea7ac`（UTF-16）。

**name/pathOffset 是"魔法偏移"**（`decode.go:52`）：`v&1==1` ⇒ `strW`，字节偏移 `(v>>1)*2`；`v&1==0` ⇒ `strA`，字节偏移 `v>>1`。串是 NUL 结尾。

### 8.2 新增一条脚本要改的四处

1. **串池**：把新路径的 dir 与 name 追加进 `strA`（ASCII）或 `strW`（含非 ASCII），并**重新分配** magic offset（保持各自池内 NUL 结尾）。重建两个池帧（`encodePStringBuffer`），更新 header `nameSize`。
2. **文件表**：追加一条 `0x18` 记录（`dataType=1`、`chunkIndex`=目标 chunk、`dataOffset`=`len(原chunk)`、`dataSize`=新脚本长度）；header `fileCount+1`。
   —— 追加会**平移其后所有区段**，因此整份文件必须重写（与 `WriteEntryCopy` 同样的落盘方式）。
3. **chunk/body**：把新脚本**追加到目标 chunk 尾部**，按 `zlib(新chunk)` → `decryptProtected("mAIn")` 重加密；组表该条 `originalSize=len(新chunk)`、`compressedSize`=新的累计末尾，其后所有组的 `compressedSize` 整体平移；header `bodySize` 同步。
4. **哈希流**：**原样拷贝**（`l10n.go:417` 就是这么做的）。服务端查找不读它，`parse` 只校验尺寸自洽 ——
   不新增条目就永远自洽。**只有在做 §8.4 第 5 步（客户端可见）时才需要动它**，而那时需要先解出那套 32 位哈希（见 §0.1）。

### 8.3 必须保住的不变量（验收判据，不是"看起来对"）

- **除目标条目外，其余每一条的 `(path, dataType, dataSize, 内容)` 逐条不变**；全量条目数 = 原 5,650,173 + 新增数。
- 新条目能被**服务端自己的读取器**读到（`FindFile` + `FileRaw`/`FileText` 返回预期字节）。
- 全部 6 个节段的边界校验（`validateHeader` + `parseDFO20260901` 的区段不等式）通过。
- 组表校验通过：`compressedSize` 严格递增且末值 == `bodySize`。
- 哈希流**逐字节保持原样**（本阶段不允许改写它），且 `parse` 的尺寸自洽校验通过
  （`n`、`m` 与 `hashSize` 的关系不变）。
- **写坏的代价是不可读**，所以实现必须：只写新文件（`O_CREATE|O_EXCL`）、绝不就地改原件、先在小样本上往返、再上全量。

### 8.4 实施顺序（每步都要能独立验证）

1. 造**小型合成归档**（照 `compact_test.go:72-92` 的写法）覆盖六个节段 → 写"新增条目"→ 用自己的解析器读回，断言 §8.3 全部成立。
2. 在**真实内层 PVF 的副本**上加一条无害新条目 → 解析器全量遍历 + 与原档逐条比对（除新条目外零差异）。
3. 让**服务端真的加载**它（`DFO_PVF_ARCHIVE` 指向副本）→ 启动日志与目录解析正常。
4. 才做 modkit 的 `pvf.add` / `pvf.entries` 动作 + 整档快照备份 + 逐字节还原 + 安装/卸载往返。
5. 最后才做"改外层 `Script.pvf`"（需要新写 `WrapOuter`）——客户端可见的那一半。

## 9. 业主 2026-10-07 追加口径：pvf 层改「目录树 = 解包片段」+ 覆盖/织入

> 原话：「PVF 层的加载应该是根据 pvf 层的目录结构加载解包文件，为 pvf 片段」
> 　　　「并且需要用 mod 的 pvf 覆盖或着织入」

### 9.1 形态

`pvf/` 目录**就是**一棵 PVF 子树的镜像：**相对路径 = PVF 里的路径**，每个文件 = 一个 **PVF 片段**。
不再在清单里列 `entries`（目录结构本身就是清单，少一处能对不上的地方）。

```
pvf/
  equipment/character/archer/avatar/belt/117530000.equ   ← 该路径已存在 ⇒ 覆盖
  pvfmod/selfcheck/probe.txt                             ← 该路径不存在 ⇒ 织入（新增）
```

### 9.2 落位语义（同一次装载内逐片段判定）

| 判据 | 动作 | 依据 |
| --- | --- | --- |
| 路径**已存在**于目标 PVF | **覆盖**（只换这一条，其余条目字节保留） | 服务端 `Archive.WriteEntryCopy`（已实测） |
| 路径**不存在** | **织入**（新增路径 + 串池 + 文件记录 + chunk 追加） | 服务端 `WriteEntryAdded` / modkit 流式 `AddPVFEntry`（已实测，真档 9.13 秒） |

判定必须在**装载时**读目标 PVF 现况得出，不能靠 mod 自己声明 —— 声明会与现况漂移。

### 9.3 两个方向的容器难题都已闭环

| 需要 | 状态 |
| --- | --- |
| 让**服务端**看到（内层归档） | `AddPVFEntry`（modkit 流式，726 MB / 9.13 秒）+ `WriteEntryAdded`（服务端），**两套独立实现产出逐字节相同**（`9f9cb5c1…`） |
| 让**客户端**看到（外层 `Script.pvf`） | **`WrapOuter` 已实现并证明正确**：把现网内层重新加壳，**逐字节还原出现网外层**（761,764,363 字节 / 73 段，1.43 秒）。加壳只有 AES 一层（`iNfO` 属内层格式自身，不属外层包裹） |

⇒ 「改外层 → 客户端与服务端同时看到」这条路**容器层已经通了**（启动器的内层重生成门禁会自动从新外层重做内层）。

### 9.4 仍未闭环的一项（必须先定，才能动手）

**「解包文件」的形态是什么？** 这决定要不要写一个 PVF 文本 ↔ 5 字节单元的编解码器：

- **A) PVF 文本**（`[tag] 值 \`串\``，即 `pvfinspect` 导出的 `.txt` 那种）——
  对 mod 作者最友好（可读可 diff），但要实现**文本 → 单元**的编码器，并处理和 `strA`/`strW` 串池的往返
  （新串要进池、拿到 magic 偏移）。
- **B) 原始单元字节**（5 字节 × N，即 `pvfinspect` 导出的 `.bin`）——
  改动最小（现有写入器直接吃这种），但 mod 作者要先用工具导出再改。

**未定之前不开工**：A 与 B 的工作量差一个数量级，且 A 会改变 mod 的编写方式。

### 9.5 对 modkit 动作设计的影响

- pvf 层**不再需要**清单里的 `entries`；只需声明"本 mod 提供 pvf 片段树"，目录承载内容。
- `plan` 仍要逐片段列出**覆盖 / 织入**（操作者看得懂的关键信息，也是"声明给操作者看"的口径）。
- 备份与还原：整档快照 + 逐字节还原（与 NPK 侧同一套）。
- 权限：新增片段树会**改 PVF 容器**，需要比现有 `exec.script`（脚本执行）更贴切的声明；命名待一并裁决。

## 10. mod 侧接线：逐处落点与两处**必须先解决**的冲突

容器层已全部实现并对着真档验过（`pvftree.go` / `pvfapply.go` / `pvfadd.go` / `pvfedit.go` / `pvfwrap.go` / `pvfinstall.go`）。
剩下的是把它接进清单/计划/注册表。四处落点（行号以 2026-10-07 工作区为准）：

| 处 | 文件 | 要做什么 |
| --- | --- | --- |
| 1 | `internal/modkit/layer.go` | 新增 `PVFKindTree = "tree"`，并加进 `PVFOpKinds`（否则校验直接拒，见下"fail-closed"） |
| 2 | `internal/modkit/manifest2.go` `validatePVF` | 新增 `case PVFKindTree`：要求 `script/args/work/touches/produces` **全空**（片段树不跑脚本、不产出中间件）；树的落地位置固定是包内 `pvf/` |
| 3 | `internal/modkit/manifest2.go` 权限映射（`:161` 附近） | 现在 pvf 层**一刀切**同时要 `pvf.merge` + `exec.script`；片段树不执行任何脚本，应把 `exec.script` **收窄到 merge/verify** |
| 4 | `internal/modkit/layerplan.go` `planPVF`（`:524`） | 新增 `case PVFKindTree`：遍历 `Manifest2.PackagePath()/pvf`，**逐片段**出一行 `覆盖 / 织入` |
| 5 | `internal/modkit/apply2.go` `applyPVFLayer`（`:478`） | 新增 `case PVFKindTree`：调 `InstallPVFTree`，按它返回的备份路径与两个 sha 登记一条 `Entry`（卸载走既有 `revertEntry` 的文件还原分支即可） |

### 10.1 冲突一：`assertPVFPairing` 会误拒片段树

`apply2.go:554` 的 `assertPVFPairing` 要求 `Script.pvf` 与 `sk.dat` **成对**替换，判据是条目里出现了 `script.pvf` 就必须同时出现 `sk.dat`。

**片段树恰恰只改 `Script.pvf`、不动 `sk.dat`** —— 它是在**现役外层上就地织入**，
并且用客户端**当前那把 `sk.dat`** 做剥壳/加壳，所以 `sk.dat` 依然配套，配对规则在这里不适用。

**修法**：把配对检查**限定到 `pvf.replace` 那条动作**（它才是"整容器替换、必须成对"的原意），
不要让片段树登记的条目卷进去。**不要**放宽成"任何 pvf 条目都不查配对" —— 那会把 replace 那道保护也拆掉。

### 10.2 冲突二：计划阶段拿不到 root

`planPVF(client, reg)` 的签名里**没有 root**（`layerplan.go:524`），而逐片段判定覆盖/织入
需要读**内层归档** `<root>/server/work/client-build/Script.inner.pvf`（见 §0.1：哈希流那套 32 位
哈希未取证，只能走"文件表 + 串池"这条确定性路径）。

而 `BuildLayerPlanWithOptions` **是**拿到 root 的（`:369`，`planServer(root, reg)` 就在用它），
所以把 root 透给 `planPVF` 是**改动最小**的做法；别为了省一个参数去"计划期临时剥壳" ——
那要多写 726 MB，计划不该有那种代价。

### 10.3 fail-closed：没接线之前必须保持"拒绝"

只要第 1 处没做，`pvf.tree` 就是**未知 kind**，声明它的 mod 会在**校验阶段被明确拒绝**。
这是刻意的：半接线的动作（能过校验但不做任何事）正是 AGENTS §6 里
「默认路径悄悄坏掉、且没有任何人会发现」那一类，比"没做"危险得多。
`internal/modkit/pvfinstall_test.go` 里的端到端用例（真客户端副本上装/卸）可以直接复用为
接线后的验收判据。
