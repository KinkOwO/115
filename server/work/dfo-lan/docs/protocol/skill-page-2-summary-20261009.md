# 技能页 2（技能类型扩展）2026-10-09 总结

> 详细过程见 `docs/protocol/skill-tree-expansion-20261009.md`（20 轮记录）。
> 本文只留**结论、当前能力、缺口清单**。

## 一、今天做成的事

### 1. 第二技能页从"完全不可用"到"可用"

**根因**：服务端把 USERINFO 里的"技能类型"字节**硬编码成 `0xff`（=未解锁）**，
所以客户端永远认为第二页没买过。

**修复（三块）**

| 块 | 内容 |
|---|---|
| 下发 | **mode0（`UserInfoBasicProbe`）与 mode1（`UserInfoAdditionProbe`）两处都要带**该字节；新增 `State.SkillTreeType`（0=未解锁 / 1=类型1 / 2=类型2）+ `protocol.SkillTreeWireIndex()` |
| 解锁 | 商城专路：product **3000150** / template **821** / **390 Cera**（"Dual Skill Build License"），购买即生效（置 `SkillTreeType=1`） |
| 切换 | `cmd260 CHANGE_ANOTHER_SKILL_TREE` 实现（同号 ACK + 新索引，**实机验证可用**） |

### 2. 放开"只认第一页"的六处硬闸（都是 `req.Tree != 0` 一类）

| 位置 | 命令 | 现状 |
|---|---|---|
| `protocol.DecodeSkillPurchase` / `character.Learn` | cmd29 加点 | ✅ 放开 |
| `protocol.DecodeSkillMove` / `character.MoveSkill` | cmd28 拖放技能 | ✅ 放开 |
| `protocol.DecodeSkillSlotTotal` / `character.MoveSkillTotal` | cmd2179 批量排栏 | ❌ **拒绝**（见 §2） |

### 3. 数据分页（第二页的归第二页）

- `MoveSkill` / `MoveSkillTotal` 写 `SkillSlots[req.Tree]`
- `VariationRestore` 按 `SkillTreeType` 取页（VP 面板）
- 三觉给 TP + 填槽位：**两页都做**
- `ReconcileTechniquePoints` → `techniquePointBalances()`，两页各自 `5 − Evolve`

### 4. 自动加点

- `cmd483` 第二分支的 `ResetAutoSet(…, tree, 7)` **tree 从存档当前页取**（原写死 0）

## 二、定案：`cmd2179` 在第二页受理即崩（五轮实机对照）

| 轮 | 变量 | 结果 |
|---|---|---|
| 一 | 拒 29 + 受理 2179 | 不崩（没加点） |
| 二 | 受理 29 + 拒 2179 | **不崩** |
| 三 | 都受理 | **崩** |
| 四 | 三 + variation `mode` 固定 0 | **崩** |
| 五 | 三 + **去掉 NOTI19**（只回 ACK） | **崩** |

⇒ **与加点、`mode`、NOTI19 全都无关**：`cmd2179`（tree=1）**只要被受理就 0xC0000005**。
服务端只能拒它。

**业主看到的"autoset 排了但不是客户端给的那个栏"**：来源是 `Learn` 里的
`placeNewShortcuts`——它按"把新学的主动技能塞进快捷栏第一个空位"的粗糙规则自己排了一遍，
而客户端预览的那份排列（`cmd2179` 的 pairs）因被拒没能应用。

## 三、第二技能页当前能力

| 能力 | 第一页 | 第二页 |
|---|---|---|
| 购买 / 解锁 / 来回切换 | — | ✅ |
| 加点 `cmd29` | ✅ | ✅ |
| 自定义快捷栏 / 拖技能 `cmd28` | ✅ | ✅ |
| 洗点 `cmd483` | ✅ | ✅ |
| VP（进化/突破） | ✅ | ✅ |
| **autoset 批量排栏 `cmd2179`** | ✅ | ❌ **拒绝** |

## 四、后续可完善清单（扫 `analysis/dumps/opcodes.tsv` 得出）

**技能相关命令：服务端未接的 6 条**

| cmd | 名字 | 为什么值得做 |
|---|---|---|
| **778** | `SKILL_QUICK_SLOT_SORT` **快捷栏排序** | ★ **与今天的 autoset 排栏同族**；客户端若有"排序"按钮，现在点了没反应。而且它可能**揭示客户端的排序规则**——正是 `cmd2179` 缺的那块 |
| **2178** | `SAVE_SKILL_EXPORT_DATA` 技能导出数据 | ★ **实机日志里出现过**（`client_frame 2178`），说明玩家**真的会触发**，现在无响应 |
| **482** | `AUTO_SKILL` 自动技能 | 与 `483 SKILL_INIT` 是一对；需确认它是技能窗口的哪个按钮 |
| **476** | `REQUEST_CHARAC_SKILL_INFO` 请求技能信息 | 客户端主动拉技能数据；未接时可能只在特定场景（他人/APC）暴露 |
| **418** | `REFUND_SKILL` 退点 | `cmd29` 已能退点（`Refund` 字段），这条可能是另一条入口 |
| **332** | `SKILL_COMMAND_ALL_DEFAULT` 指令恢复默认 | 与已接的 `331`（自定义）配套，缺一半 |

**另一笔悬着的账**（业主 21:08 说"先不动"）
`skillRows` 的**栏位起点**：页1 有玩家保存的 `SkillSlots[0]`；页2 空 ⇒ 整套走"源表顺序号"
（`catalog/characters.go:329/340` 里就是 `uint16(len(...))` 自增，**不是从 PVF 读的真实布局**）。
两页起点不同 ⇒ 客户端 autoset 的差分在两页落到不同结果。
候选修法：**解锁时把页1的栏位复制给页2当起点**（只复制摆放、不动加点）。

## 五、方法教训（今天最值钱的几条）

1. **"崩 / 不崩"两个会话的 `client.log` 退出码对比**，是定位闪退最快的办法（一次锁定哪一版引入）。
2. **一次只改一个变量**；五轮实验才把 `cmd2179` 定死，但每轮都排除了一个可能。
3. **别拿包内容当幂等键** —— 客户端在同一会话里会**复用同一个 body**（`cmd260` 的坑）。
4. **别在技能命令里注入"重建角色"级别的包**（mode0/mode1）——技能窗口开着时对象悬垂。
5. **"回退到最小实现"时要区分"新功能"和"正确性修复"** —— 第十四轮把必需的 `tree` 一起退掉了，
   导致后面又花了两轮找回来。
