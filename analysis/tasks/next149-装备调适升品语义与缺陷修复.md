# next149 — 装备调适「升品」语义闭环与两个缺陷修复

> 承接 `next148`（CMD2258 直读 PVF 落地）。本文记录**升品路径**的实机取证、两个真实缺陷、
> 修复与验证。结论全部有源字段 / 存档字节 / 实机日志三方对照。

---

## 1. 一句话结论

- **升品 = 换模板 + 调适阶段归 0**，之后按**新品质**重新匹配档位（不是"跳到档 5"）。
- 修掉两个缺陷：**升品候选必须跨 `[condition]` 块查**、**落库顺序（先改模板再生成实例行）**。
- 另加**自愈**：读到实例行 `+2` 与 `Template` 不一致时按 `Template` 只修那一处，避免单件装备
  永久锁死。

---

## 2. 源结构（`etc/115lvability/equipmentawakeningoptionsystem.cos`）

```
[max awakening] 3
[infos]
 [info]
  [condition] 115 `rare` 3        ← 品质 + 档位
  [need materials] [group] N      ← 成本（按档位成行：0/1/2/3）
  [refund materials] [rates]
  [upgrade result]                ← 源模板 → 候选目标（**不按档位分块**）
```

两个关键事实（本轮实测确认）：

| 事实 | 证据 |
| --- | --- |
| **成本按"当前档"匹配** | 实机第 4 次面板 = `100000 金币 + Unique Soul×40 + Void Soul×1`，逐字段等于 `[condition] 115 rare 3` 的行 3（源第 153 行） |
| **升品候选跨块** | `100051304` 的升品条目写在 `[condition] 115 rare 1` 块（源第 105 行），而升品动作发生在**阶 3** ⇒ 按块查必失败 |
| **升品是跨品质的** | `100051304`(rare) → `100051275`；`101001152`(epic, rarity 4) → `101001153`(primeval, rarity 8) |

---

## 3. 实机复现与两个缺陷

### 缺陷 A：升品候选按块查 ⇒ 每次升品都被拒

现象：「1–3 次成功，第 4 次无法升品」。日志：

```
equipment_awakening_refused  reason=该装备在阶段 3 没有升品目标（源 [upgrade result] 无此模板）
```

修法：解析时把各块的 `[upgrade result]` **合并成全表**（`Rules.Upgrades`），升品查
`Rules.UpgradeSource(template)`；同源多块时"空候选不覆盖有候选、都非空取并集"。

### 缺陷 B：落库顺序写反 ⇒ 记录与模板不一致，装备被永久锁死

```go
row := EquipmentRow(gear)     // ← 先把旧模板写进实例行 +2
row[170] = newStage
gear.Template = newTemplate   // ← 之后才改模板
```

`EquipmentRow` 会把 `gear.Template` 写进行内 `+2`，所以升品后**行里留旧模板、`Template` 是新模板**。
此后 `ValidateRecord` 的 `equipment instance template mismatch` 会在**读装备那一步**就报错 ——
不仅调适点不动，连 `ENUM_CMDPACKET_GET_USERINFO`（id=8）都会被拒，**整个角色在选角界面消失**：

```
{"error":"equipment instance template mismatch","id":8,"kind":"character_rejected"}
```

修法：**先改 `Template` 再生成行**，写回前 `ValidateRecord()` 兜底；并新增
`healAwakeningRecord()` 自愈（只修行内 `+2`，其余实例字节不动），命中时回执记 `record_healed`。

---

## 4. 实机验证（2026-10-02 00:57–00:58，角色 11 / 100051304）

```
第1次 stage 0→1  稀有灵魂×75  + 金币 150000
第2次 stage 1→2  稀有灵魂×50  + 金币 100000
第3次 stage 2→3  稀有灵魂×50  + 金币 100000
第4次 stage 3→0  upgraded=true target=100051275
      神器灵魂×40 + 虚无之魂×1 + 金币 100000        record_healed=false
第5次面板显示：300000 金币 + Unique Soul(神器灵魂)×35   ← 等于 `[condition] 115 unique` 的行 0
```

⇒ **升级后阶段归 0、按新品质匹配**，与实现一致；`record_healed=false` 说明新代码没再写坏记录。

（材料名对照：`10361512 稀有灵魂`、`10361513 神器灵魂`、`10361514 传说灵魂`、`10361515 史诗灵魂`、
`10361516 太初灵魂`、`10415191 光辉灵魂`、`10400395/10400396 虚无之魂`、`10413522 黑色灾影`。
`[condition] X 5` 档的专属成本是 `800000 金币 + 光辉灵魂×100`。）

---

## 5. 取证资产（本轮新增）

- `analysis/dumps/ida_awakening_2258f.py` / `awakening-2258f/`：从 `[condition]` 等字符串反查引用者；
- `analysis/dumps/ida_awakening_2258g.py` / `awakening-2258g/`：定位到客户端解析器
  **`sub_1477E07E0`**（`EquipmentAwakeningOptionSystem`，读 `EquipmentAwakeningOption.lst`），
  它按 `[max awakening] → [infos] → [info] → [condition] → [need materials] → [refund materials]
  → [rates] → [upgrade result]` 的**同一顺序**解析 ⇒ 两边同源，客户端没有隐藏规则；
- `analysis/tools/pe_scan_awakening_labels.py`：在 `client/DFO.exe` 里按明文/常量搜这些标签，
  输出 VA（`[max awakening]`=`0x14B2FC1F0`、`[need materials]`=`0x14B2CF9C8` …）。

---

## 6. 未做 / 边界

1. **`GET_USERINFO` 单件坏装备 ⇒ 拒整个角色**（本轮踩到）。业主 2026-10-02 决定**暂不改**
   （属于既有门禁行为），后续若要加固，方向是"逐件降级：丢弃/修正坏件并记警告，而不是拒角色"。
2. `mode=1`（初始化/返还）仍未实现（源 `[refund materials]` 已读齐，缺客户端表现证据）。
3. 非 100% 成功率仍未处理（本版本源里全是 100%）。
4. **星蕴石 / 誓约（`[primer]`，`[equipment awakening option]=1`）尚未实机验证** —— 走同一套
   规则（Table1 有 6 档 0..5），修好升品后应可用，但需要一次实测确认。
5. 人工改存档的教训：**先备份该行、改完立刻做一致性校验**（本轮因手工改动制造了不一致，
   导致全部角色在选角界面消失）。

---

## 7. 实机验收（2026-10-02，业主确认）

| 项 | 结果 |
| --- | --- |
| 调适全链路（稀有 → 史诗） | ✅ `100051304`(rare 2) → `100051275`(unique 3) → `100051276`(legendary 4) → `100051277`(epic 5)，每轮 `0→1→2→3` 后升品；**第 9 次及以后不再被拒** |
| 星蕴石 | ✅ 可调适且升品正常（option = `EquipmentAwakening_Table1.etc`，6 档 0..5）；顺带确认服务端按 `space=3` 查穿戴槽即可命中 |
| 誓约（primer） | ⛔ 客户端**没有** Tune/Promote 按钮（业主核实）⇒ **不是服务端缺口**，不实现 |
| 「同一会话第 9 次被拒」 | ✅ 根因见 §8，已根治 |

## 8. 「同一会话第 9 次调适被拒」的根因（已根治）

`cmd/wireprobe/main.go` 原先把**帧校验和判定** `verified` 写在 `retainRequestBody()` 分支**内部**：

```go
if len(frame.Raw) >= wire.ClientHeaderSize && retainRequestBody(frame.ID, bodySamples) {
    // ...
    verified = wire.Checksum(append(append([]byte{}, frame.Raw[11:13]...), p...)) == frame.Raw[7]
}
```

而 `retainRequestBody` 对**未登记**在 `observedGameRequest` 的命令有 `BodySampleLimit = 8` 的
**日志正文采样配额**（`request_scope.go`）。CMD2258 当时没登记 —— 同族的 **2259 登记了**：

- 同一会话前 8 帧：配额未耗尽 ⇒ 算 checksum ⇒ 一切正常；
- **第 9 帧起**：`retainRequestBody` 返回 false ⇒ `verified` 恒 false ⇒ 业务分发被
  `equipment_awakening_rejected`（"请求校验失败"）挡在 wire 层；
- **重进客户端** ⇒ 新会话、配额重置 ⇒ 又能 8 次。

⇒ 与业主描述的现象逐字吻合（"调适 8 次后无法继续；重进后又能，但仍然 8 次"）。

修法（两处，互为保险）：

1. **解耦**：`verified` **始终**计算（所有 `type==1` 帧都解密 + 校验），只有
   `hex`/`plain_hex`/`checksum_ok` 的**记录**受采样配额约束；
2. 把 `2258` 登记进 `observedGameRequest`（与 2259 同族），并加回归测试
   `TestAwakeningPromoteIsExemptFromTheBodySampleCap`。

同类先例：**CMD2329** 在 2026-09-27 因同一机制失效（当时只补了豁免，没解耦）⇒ 这次的解耦让
"漏登记就静默失效"这类问题不再可能发生。

## 9. 收口结论

调适（CMD2258）在 115 客户端上**全部闭环**：档位推进、跨品质升品、材料与金币结算、全链路
`rare → unique → legendary → epic`、星蕴石调适均实机通过；誓约无客户端入口，非服务端范围。
唯一仍未做的是 `mode=1` 初始化/返还与非 100% 成功率（本版本源里全为 100%）。
