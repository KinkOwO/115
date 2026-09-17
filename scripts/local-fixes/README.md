# 本侧修复的可复现脚本

这里是把 `docs/进度-20260917-paranoiahua.md` 描述的改动**重放**到任意环境的脚本。

上游只需在**自己的 configs 目录**上运行它们，就能得到与本侧一致的配置，
不需要接收任何 261MB 级的生成物或本机专属文件。

---

## 用法总览

```bash
# 1) 服务端源码的 3 个修复（也可直接用 MR 的 diff）
python port_fixes_to_upstream.py

# 2) configs 的全部改动（副本/门户边/装备目录/掉落等级/掉落类别）
python apply_local_config.py --configs <目标configs目录> --dry     # 先看会改什么
python apply_local_config.py --configs <目标configs目录>           # 实际执行

# 3) 装备目录合并（需要先用 equipmentfull 导出 equipment-high.json）
go build -o bin/equipmentfull ./cmd/equipmentfull
./bin/equipmentfull -source <内层PVF> -output <configs>/equipment-high.json -min-level 100
python remerge_equipment.py --a-configs <目标configs目录> --high <configs>/equipment-high.json

# 4) 启动脚本接线（**必做**，否则副本仍是 11 个）
python fix_launch_wiring.py --channel-probe <dfo_probe_tools/channel_probe.py>
```

---

## 各脚本说明

| 脚本 | 作用 | 幂等 |
|---|---|---|
| `port_fixes_to_upstream.py` | 把 3 个源码修复按精确字符串替换打到源码上 | ✓ |
| `apply_local_config.py` | 补门户边 / loot 等级 / 掉落类别 / 装备目录合并 / 校验 dungeons.full 与 world 校验和一致 | ✓ |
| `remerge_equipment.py` | 把 `equipment-high.json` 合并进 `equipment.current37.json`（**纯加法**） | ✓ |
| `fix_launch_wiring.py` | 修 `channel_probe.py` 的副本目录接线 | ✓ |

---

## ★ 必读：四个"改了不生效"的坑 ★

这几条是本侧排查中最耗时的部分，**不做就会以为改好了、其实没生效**。

### 坑 1：`channel_probe.py` 有 tag 降级链，`_next37` 会被降成 `_next35`

```python
candidate37 = tag.endswith('_next37')
if candidate37: tag = tag[:-7] + '_next36'
candidate36 = tag.endswith('_next36')
if candidate36: tag = tag[:-7] + '_next35'
```

而所有把 `-dungeon-catalog` 指到 `dungeons.full.json` 的分支条件都是
`_next28`.. `_next34`，**不包含 `_next35`**。于是最后生效的是默认值
`configs/dungeons.generated.json`（**只有 11 个副本**）。

**症状**：`dungeon_request_refused` / `reason="dungeon absent from imported source"`。
**修法**：把默认值那一行改成 `dungeons.full.json`（`fix_launch_wiring.py` 做这件事）。

### 坑 2：`list/equipment.lst` **不含 115 级装备**

`cmd/equipmentaudit` 以及依赖它的导入链路都按这个索引遍历：

```go
idx, _ := catalog.ResolveScript(a, "list/equipment.lst")
rows, _ := catalog.ParseIndex(idx.Cells)     // 索引里没有的就永远发现不了
```

实测：115 级装备的 `.equ` 文件**确实在 PVF 里**，但**不在索引里**。后果：

- 导出结果里等级 ≥110 的条目**只有 2 个**，115 级装备一件都没有
- 服务端 `wearable()` 报 `equipment definition missing`
- 表现为**装备脱下来就穿不回去**

**修法**：改用枚举 `Archive.Files()`（见 `cmd/equipmentfull`），不依赖索引。

### 坑 3：`pvf.File.Path` 是**目录名**，不是完整路径

```go
type File struct {
    Path        string   // "aicharacter/_bizarre/.../action"   ← 目录
    Name        string   // "proc.act"
    ArchivePath string   // "aicharacter/.../action/proc.act"  ← 完整路径
}
```

按 `Path` 判断扩展名会得到 `suffix="<none>"` 5,650,015 条（全部文件）、零匹配。
**必须用 `ArchivePath`。**

### 坑 4：`loot` 的 `maximum_grade` 会让高等级怪物死亡上报**全部被拒**

```go
// internal/loot/rules.go:150
if level == 0 || rank > 3 || int(difficulty) >= len(r.DifficultyBonus) ||
   uint32(level)+3 > c.MaximumGrade {
    return out, fmt.Errorf("drop source range is not imported")
}
```

当 `loot.next25.json` 的 `maximum_grade = 20` 时，**Lv.18 以上怪物一律被拒**，
且这个判断在**掷掉落之前**。

**症状（极隐蔽）**：服务端**不崩、界面不报错**，只是默默拒绝
→ 客户端收不到 `monster_death_confirmed`
→ **怪物卡在半空 / 房间不结算 / 进不了下一张图 / 什么都不掉**。

实机日志：`dungeon_request_refused × 250`，reason 全是 `drop source range is not imported`。
**修法**：`maximum_grade` 放宽到 ≥130（纯放宽，不影响低等级行为）。

### 坑 5（附带）：装备写入的字段是 `worn` 不是 `equipment`

```go
type Bag struct {
    Equipment []BagEquipment `json:"equipment"`  // 背包里的"装备栏"
    Worn      []BagEquipment `json:"worn"`       // ← 真正"已穿戴"
}
```

写错字段的后果：属性和荣誉值不生效 + 背包装备栏被占满 →
客户端报 `target inventory is full`。

另外：`jsonb_set(state,'{inventory,worn}',...)` 在 `inventory` 不存在或为 null 时
**静默无效**（不报错、不写入），新号必须先建
`{"version":"ordinary-bag-v1", ...}` 容器。

---

## 其他实测结论（供参考）

| 现象 | 结论 |
|---|---|
| 客户端难度 | **1 起算**（1=普通…5=英雄）；服务端原要求 `==0`，导致正常选图全被拒 |
| 属性上限 | `internal/character/detail.go` 会把属性 **×10** 再打包：uint16 类（攻击/防御/攻击速度/施放速度）**上限 6553.5**；int16 类（硬直/跳跃力/抗性）**上限 3276.7**。超限 → `entry_addition_error` → **角色进不去游戏** |
| 转职 | `detail.go:76` `if s.Advancement != 0` 直接报 `unsupported initial skill state`（注释说明只导入了 17 个职业的未转职初始技能表）。**直接改 `advancement` 会让角色进不去游戏** |
| 奥德赛装备隔离 | 服务端里 **Odyssey 没有任何独立背包/装备容器**；所谓"奥德赛装备独立"是客户端行为 |
| 装备掉落 | `internal/loot/rules.go:215` 的 `if !enabled["equipment"]` 会整段跳过装备分支；需在 **实际生效的那份** rules 文件（本侧是 `drop.current36.json`）的 `supported_kinds` 里加 `"equipment"` |
| 装备职业过滤 | `equipmentCandidates` **不做职业过滤**，会掉出别的职业的装备（原版 DNF 亦如此） |
