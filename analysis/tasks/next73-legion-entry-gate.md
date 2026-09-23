# next73 — 军团 / 末世录「入口门禁」取证

> 问题：实机看不到**末世录军团本**，反馈猜测"该角色是奥德赛角色，没解锁这个任务"。
> 本文回答「是否有门禁」，并给出客户端那几道判定与它们的拒绝文案。

## 1. 结论先说

**有门禁，一共 7 道，全在客户端 `sub_142510A50` / `sub_142511D10`（也就是发 CMD2043 的那两个函数）里。**
但**没有一道是"奥德赛角色"门禁** —— 数据侧看不到任何 `Odyssey` 相关限制，见 §3。

按代码里的判定顺序：

| # | 判定 | 不过时的表现 | 性质 |
| --- | --- | --- | --- |
| ① | `sub_1476E0090(*(a1+128), 解码串 word_14982AC40)` —— 某个**具名条目**必须存在 | 静默 return（无提示） | 本地数据 |
| ② | `sub_145F0B890() != 0` —— 当前会话/频道有效 | 静默 return | 会话态 |
| ③ | `sub_145F152A0() != 0` —— 频道/模式判定（内部用 CMD 注册表 `qword_14E66C090`） | 静默 return | 会话态 |
| ④ | **`sub_145F147D0() != 0`** —— 要求 **8 个队伍槽全部非零** | 弹 **532**「队友还没全部进入副本」/ **725** | **队伍齐** |
| ⑤ | **`(*(vtable+624))(modeObj, 8) > 0`** | 弹 **100088500**「本周入场次数已用完」 | **周入场次数** |
| ⑥ | **`(*(vtable+640))(modeObj, 0) > 0`** | 弹 **100088497**「本周奖励已领完」 | **周奖励次数** |
| ⑦ | 具名查表 `sub_145695000(...) == 0` | 弹 **100088632**「今天不开放」 | **开放日程** |

全部通过后才执行 `v23 = sub_142AB29C0(qword_14E683C40); sub_1424FE550(v23);`
—— 其中 **`sub_1424FE550` 正是 CMD2043 的发送函数**（`next64` §3.10 已登记），
所以这两个函数就是"军团入口是否出现/是否可点"的判定点，与旧台账 `D1a` 的判断一致。

## 2. 拒绝文案（用客户端自己的字符串表解出来的）

数据源 `analysis/dumps/dstr_id_to_text.json`（客户端 dstr 表，英文原文）：

| 串 id | 原文 | 中文含义 |
| --- | --- | --- |
| 100088500 | `Cannot proceed as all weekly entries have been used.` | **本周入场次数已用完** |
| 100088497 | `Cannot proceed as all weekly rewards have been obtained.` | **本周奖励已领完** |
| 100088632 | `This dungeon isn't open today.` | **今天不开放** |
| 532 | `At least one of your Party Members has not yet joined the dungeon. Please try again later.` | **队友还没全部进本** |
| 725 | `All the Party Members have not joined yet; you cannot enter this dungeon.` | 队友未到齐（另一入口） |

⇒ **实机时只要看到弹窗，对着上表一眼就能定位卡在哪一道门禁。**
这比"没反应"有价值得多：请测试把弹窗原文（或截图）一起回传。

## 3. 「奥德赛角色」这个猜测在数据侧站不住

> **⚠️ 本节已被 `next74` 更正。** 下面第 2 行原来写"任务 23099 没有前置任务"是**错的** ——
> 我只读了 JSON 里的结构化字段 `prerequisites`（它是 null），**漏了脚本体里的
> `[pre required quest]` cell**（那才是真源，`internal/catalog/quests.go:61` 就是读它的）。
> 23099 的真实前置是 **23055**，往上还有一整棵树，见 `next74-legion-entry-prereq-chain.md`。
> 保留下表是因为其余各行仍然成立（尤其是"没有 Odyssey 规则"这一条）。

| 事实 | 证据 |
| --- | --- |
| 入口副本 `100005220` 绑任务是 `quests=[23099]` | `configs/dungeons.full.json` → `dungeons.100005220.mazes[0].quest = 23099` |
| ~~任务 23099 没有前置任务~~ | ❌ **错**：结构化字段 `prerequisites=null` 不等于没有前置。真实前置见 `next74` |
| 任务 23099 只要 **115 级 + 任意职业** | 同上：`minimum_level=115`、`maximum_level=10000`、`jobs=["[all]"]`；脚本体里 `[level] 115`、`[job] [all]`、`[grade] [epic]`、`[difficulty] G`、`[cant giveup] 1` |
| 四个**阶段副本没有任务门禁** | `100004994/100004995/100005057/100005111/100005112` 全部 `quests=[0]` |
| 所有末世录副本 `Odyssey=false` / `Tutorial=false` | 同上，`MinimumLevel=115`、`BasisLevel=145` |

任务 23099 的脚本体：`contents/2026/apocalypse_scenario/quest/apocalypseantienbi.qst`，
接取/完成 NPC `[npc index] 100002999`，`[dungeon info] 100005220`。

**所以"奥德赛角色接不了这个任务"没有数据依据。** 真要说差别，奥德赛角色的等级/名声/队伍状态
可能与普通角色不同，但那属于 ④⑤⑥ 三类门禁的输入，不是一条独立规则。

## 4. 名声：另有一条独立要求

`apocalypse.ctp` 的 `[recommend fame]`（`configs/apocalypse.generated.json` → `operations[].recommendFame`）：

| 作战 | `[index]` | `[member limit]` | `[recommend fame]` |
| --- | --- | --- | --- |
| ① | 1 | party | **98,171** |
| ② | 2 | party | **105,881** |
| ③ | 3 | party | **105,881** |
| ④ | 5 | party | **73,993** |

截图里 VENUS 那行显示 `需要高级名声 41,929(17,804)` —— 括号内是当前值。
**如果末世录按同一口径展示，门槛（73,993 起）比 VENUS 高得多。**
请测试顺带确认该角色的**高级名声**数值。

另注意 `[member limit]` 四个作战全是 **`party`** —— 与门禁 ④（8 个队伍槽全非零）互相印证：
**末世录原版是队伍内容，单人是否过得去取决于我们伪造的 single-party 能否让那 8 个槽都"有人"。**

## 5. 周计数（⑤⑥）的来源还没定位

已排除：末世录目录下只有三个非美术数据文件 ——
`contents/2026/apocalypse/etc/{apocalypse.ctp, dungeonskillinfo.ctp, dungeonspecialkeyinfo.cos}`，
前两个我们已完整解析，**都不含周计数**。

旁证：截图里 VENUS 显示 `每周参赛人数 3/3`、`每周奖励计数 1/1`，而**服务端对 VENUS 一行代码都没有**
（`grep -i venus` 只命中我们为对照而声明的两个 opcode 常量）⇒
**这些计数很可能是客户端本地表给的，不是服务端下发**（即：不是"我们没发 NOTI2895"导致的）。

⇒ 待办：定位 `(*(vtable+624))(modeObj, 8)` / `+640` 这两个 getter 的**数据来源**
（`modeObj = sub_142AB28D0(qword_14E683C40)`，同一个全局在 X7 里出现过）。
在拿到之前，不要臆断"补一个包就能过门禁"。

## 6. 下一步（按性价比排序）

1. **实机收集弹窗**：把这 7 道门禁的文本对上号，就能立刻确定卡点（成本最低、信息量最大）。
2. 确认该角色的**高级名声**与**队伍状态**（④与名声是最可能的两处）。
3. 定位 ⑤⑥ 两个 getter 的数据源（IDA：`sub_142AB28D0` → 类 → vtable slot 78/80）。
4. 若确认卡在 ④：检查我们 `-solo-party-bootstrap` 伪造的队伍是否让 8 个槽全非零
   —— 这是**服务端唯一能主动改善**的一条。

## 7. 复现

```bash
# 反编译入口判定点 + 一层 callee（产物 analysis/dumps/legion-entry-gate/）
analysis\dumps\run-legion-entry.cmd

# 在 PVF 里找数据文件（示例：末世录 etc 目录）
cd server/work/dfo-lan
go run ./cmd/pvfinspect -source ../client-build/Script.inner.pvf \
    -find "contents/2026/apocalypse/etc" -prefix -output runtime/pvf-guide
# 注意：-find 会写一份很大的 matches.json，用完删掉
```

拒绝文案的对照表在 §2；`analysis/dumps/sender-callers/` 里已有这两个函数的调用者伪代码。
