# 进度报告：115us 传送/选图/装备目录修复（paranoiahua / 2026-09-17）

> 本文档记录 **paranoiahua** 一侧在 115us 服务端上的独立工作，供上游合并参考。
> 所有结论都附**实机日志证据**或**可复现命令**；与上游已有修复重复的部分已标注。

---

## 一、已提交到仓库的修复（MR !1，3 个文件 / 41 行）

这三个是**在 `db3613b` 之上新做的**，改动前已确认上游没有对应修复。

### 1. `internal/dungeon/session.go` — 客户端难度是 1 起算

原代码 `r.Difficulty != 0` 直接拒绝。实测客户端请求字节：

```
b0e1f505 01 0000 00 00 ffff ...
   ID     ↑ Difficulty=1
```

而客户端界面显示的正是 **Normal**。即难度从 **1** 开始计（1=普通 2=专家 3=达人 4=王者 5=英雄），
与 `configs/progression.next25.json` 的 `difficulty_rates`（5 档）一致。
原逻辑导致**所有正常选图都被拒**（日志 `dungeon_request_refused / unsupported dungeon option` ×16）。

放宽为 `r.Difficulty > 5` 才拒绝，其余字段仍严格校验。

### 2. `cmd/wireprobe/dungeon_flow.go` — 可进图任务集合漏了 completed

```go
if q.Status == "accepted" && ... { accepted[q.ID] = true }
```

客户端自己的门槛文案是 **"accepted **or** completed prerequisite quests"**，
服务端只认 accepted，导致**已完成的任务副本反而进不去**。

### 3. `internal/world/service.go` — 三处

1. **`WalkableTolerance = 64`**：客户端经传送门/地图传送落地时坐标会稳定偏出
   服务端解析出的可行走矩形，实测偏差 **9 / 11 / 18 / 33** 像素。0 边距把合法的门全挡掉。
   取 64 覆盖这些偏差，且远小于"任意传送"量级，`TestTransitionAuthority`（用 65535 验证）仍通过。
2. **跳过 `unsupported permission` 类 pending**：服务端没实现的条件
   （`[need quest]` / `[level acc enter force level]` / `[event id]`）**由客户端判定**，
   原逻辑把它们当成"该区域不可进入"，会让**整片地图彻底打不开**。
3. **源区域出边枚举不全时放行**：两种情形——`dynamic portal destination`
   （脚本目的地写 `-1 -1`，导入时被丢弃）与该区域**一条门户边都没有**
   （NPC/码头/界面触发的跨区传送）。实测 694 个区域里 **160** 个属前者、**73** 个属后者。
   有门户边的区域仍走严格校验；**排除 `SeriaReturnWarp`**（赛丽亚房间语义是"只能回到保存原点"），
   否则 `TestTransitionAuthority` 的 "remote portal return" 会挂。

**验证**：`go test ./...`（13 个包全绿）、`go vet ./...` 通过；未触碰上游新增的
shop / vault / disjoint / avatar / itemindex / migrate / equip 任何文件。

---

## 二、未提交的数据与配置修复（本地有效，体积大未入库）

| 文件 | 改动 | 效果 |
|---|---|---|
| `configs/dungeons.full.json` | 由 `world.generated.json` 的 3,368 个副本 ID 重新导入 | **11 个副本 / 113 张地图 → 3,200 个副本 / 16,042 张地图** |
| `configs/world.generated.json` | 给全部 694 个区域补上门户边 | 门户边 **约 2,400 → 478,879 条**，跨大陆/赛丽亚房间/码头全部可达 |
| `configs/equipment.current37.json` | 合并高等级装备 | **3,174 → 19,955 项**（新增 16,781 个 100 级以上装备） |
| `configs/loot.next25.json` | `maximum_grade` **20 → 130** | 见 §3.4，这是"怪物卡半空"的根因 |

**注意**：`dungeons.full.json` 与 `world.generated.json` 的 `source.checksum` 必须一致
（都是 `7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80` / 760,530,763 字节），
否则启动报 `dungeon/world source versions differ`。

---

## 三、★ 对上游有直接价值的四个发现 ★

这几条是排查过程中踩到的**真实坑**，其他人在做同样的导入/修复时很可能遇到。

### 3.1 `list/equipment.lst` **不含 115 级装备**

`cmd/equipmentaudit` 以及依赖它的导入链路都是**按索引遍历**：

```go
idx, _ := catalog.ResolveScript(a, "list/equipment.lst")
rows, _ := catalog.ParseIndex(idx.Cells)        // ← 索引里没有的就永远发现不了
```

实测：**115 级装备的脚本文件存在于 PVF 中**，但**不在这个索引里**。

```
equipment/character/swordman/weapon/ssword/101001153.equ   ← 文件在
list/equipment.lst                                          ← 索引里没有
```

后果：导出结果里 **等级 ≥ 110 的条目只有 2 个**，115 级装备一件都没有 →
服务端 `wearable()` 报 **`equipment definition missing`**（表现为**装备脱下来就穿不回去**）。

**解法**：不要依赖索引，改为**枚举 `Archive.Files()`**（本仓库新增 `cmd/equipmentfull`）。
565 万个文件条目，筛选 `equipment/**/*.equ`。实测导出 **222,436 条候选**，
其中 100 级以上 **16,969 件**（100→7,852 / 105→3,667 / 110→3,363 / **115→2,087**）。

### 3.2 `pvf.File.Path` 是**目录名**，不是完整路径

```go
type File struct {
    Index       int
    Path        string   // "aicharacter/_bizarre/atgunner/mirror_atgunner/action"  ← 目录
    Name        string   // "proc.act"
    ArchivePath string   // "aicharacter/.../action/proc.act"                       ← 完整路径
    ...
}
```

按 `Path` 判断扩展名会得到 `suffix="<none>"` 5,650,015 条（全部），零匹配。
**必须用 `ArchivePath`。**

### 3.3 115 级装备的 `[attach type]` 是 **`[trade]`**，不是 `[free]`

沿用旧目录的 `[free]` 过滤会把 115 级装备**全部排除**：

```json
"[attach type]": [{"type":6,"value":157673527,"text":"[trade]"}]
```

正确的过滤应是"**排除封印类**"，而不是"只要 `[free]`"。

### 3.4 ★ `loot` 的 `maximum_grade` 会让高等级怪物死亡上报**全部被拒** ★

`internal/loot/rules.go:150`：

```go
if level == 0 || rank > 3 || int(difficulty) >= len(r.DifficultyBonus) || uint32(level)+3 > c.MaximumGrade {
    return out, fmt.Errorf("drop source range is not imported")
}
```

`configs/loot.next25.json` 的 `maximum_grade` 是 **20**，于是：

**Lv.82 怪物：`82 + 3 = 85 > 20` → 直接拒绝**，而且在"掷掉落"**之前**就返回错误。

**症状（很隐蔽）**：服务端不崩、界面不报错，只是**默默拒绝**：
- 怪物死亡上报（命令 39）被拒 → 客户端收不到 `monster_death_confirmed`
- → **怪物卡在半空**、不消失
- → **房间不结算、进不了下一张图**、不掉任何东西

实机日志：`dungeon_request_refused × 250`，reason 全是 `drop source range is not imported`。
把 `maximum_grade` 放宽到 130 后，**怪物正常死亡、过图恢复正常**。

> 注：上游文档提到已用 `configs/loot.level150.json` 解决同类问题（**保留原 1022 条定义，扩展为 1023 条**）。
> 本条作为**独立复现**记录：如果 `maximum_grade` 仍是 20，同样的症状会再次出现。

### 3.5 `channel_probe.py` 必须传 `-loot-catalog`，否则掉落服务根本不加载

```go
lootCatalogFile := flag.String("loot-catalog", "", "...")
if *lootCatalogFile != "" { ... 加载 lootService ... }      // 不传就永远是 nil
```

一次同步事故中把旧版 `channel_probe.py` 覆盖到新服务端上，
**漏了 `-loot-catalog` / `-loot-rules`**，结果同样是"怪物卡半空"。
上游版（`db3613b`）已包含这两行，**恢复上游版本即修复**。

---

## 四、其他排查结论（供参考）

| 现象 | 结论 |
|---|---|
| 任务门槛任务写不进 | 服务端任务目录 2,844 个里**有 18 个门槛任务不存在**（3428/3449/3478/5562/…/32103），这些门**永远开不了**；但 `protocol.CompletedQuests` 只校验 `id != 0 && id < 40000 && !dup`，**不查目录成员**，所以可以注入 |
| 区域 `[need quest]` 误收集 | 权限块里的数字**不全是任务 ID**（`[need level]` 是等级、`[condition]` 是条件数）。全量收集会写进 115 个误编号，需要按段名区分 |
| 装备写在 `equipment` 字段 | **`inventory.equipment` 是背包的"装备栏"，`inventory.worn` 才是"已穿戴"**（`internal/inventory/bag.go`）。写错会导致：属性和荣誉值不生效 + 背包装备栏被占满 → 客户端报 `target inventory is full` |
| `jsonb_set` 静默无效 | 新号没有 `inventory` 字段时，`jsonb_set(state,'{inventory,worn}',...)` **不报错也不写入**。需要先建 `{"version":"ordinary-bag-v1", ...}` 容器 |
| 客户端属性上限 | `internal/character/detail.go` 会把属性 **×10** 再打包：uint16（攻击/防御/攻击速度/施放速度）**属性值上限 6,553.5**；int16（硬直/跳跃力/抗性）**上限 3,276.7**。超限 → `entry_addition_error` → **角色进不去游戏** |
| 转职 | `detail.go:76` `if s.Advancement != 0` 直接报 `unsupported initial skill state`，注释说明"只导入了 17 个职业的未转职初始技能表"。**直接改 `advancement` 会让角色进不去游戏**；真转职需要导入 `[growtype N]` 技能表 + 改 `learning_catalog.go:90` |
| 奥德赛装备隔离 | 服务端里 **Odyssey 没有任何独立背包/装备容器**（全局只有 `world.go:72` 的 `[odyssey enter level]`）。所谓"奥德赛装备独立"是客户端行为，服务端不区分 |
| 地面掉落标签 | 未涉及；上游仍在追踪原生标签管理器路径 |

---

## 五、新增工具

| 工具 | 用途 |
|---|---|
| `cmd/equipmentfull` | **不依赖 `list/equipment.lst` 索引**，改为枚举 `Archive.Files()` 导出装备目录。支持 `-min-level`。这是绕过 §3.1 的唯一可靠方式 |
| `cmd/equipmentaudit`（已有） | 按索引遍历，**会漏掉 115 级装备** |

---

## 六、建议的下一步（上游）

1. **把 `equipmentfull` 的枚举方式并入正式导入链路**，或修复 `list/equipment.lst` 的解析
   —— 否则任何"按索引"的导入都会漏掉 115 级装备。
2. **复核所有 `maximum_grade` 类阈值**（loot 及其他目录），确认没有"等级上限 < 实际内容等级"的隐性拒绝。
3. **`channel_probe.py` 的 flag 清单加入同步检查**（本地环境与仓库版本不一致时容易漏参数）。
4. `internal/world/service.go` 的三处放宽**是权宜之计**：真正的根因是**导入器没有解析
   `[pathgate pos]`**（城镇门户定义的第二处来源），以及 **walkable 矩形与客户端落点存在系统性偏差**。
   上游若能从导入器层面解决，本 MR 的第 3 条可以回退。

---

## 七、本机环境说明（与上游不同）

- 本项目在 `D:\115us\upstream\repo`（浅克隆 `fuckworld/115` @ `db3613b` + 本侧改动）
- 客户端 `D:\115us\client`，数据库沿用 `D:\115us\server\work\dfo-lan\runtime\storage\`
- 未修改上游的 `runtime/storage/pgdata`、日志或任何大二进制
