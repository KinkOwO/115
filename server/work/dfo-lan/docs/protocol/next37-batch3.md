# 37 轮第三批（进图卡死 / 金币显示诊断 / 装备澄清）

二进制 `b50dc4a3…`。

## 1. 进不去下一个地图/房间——已修，已验证

实机日志（run 215217）：CMD45（移动到下一房间）连续 5 次被拒，reason
`unresolved monster spawn option "[champion]"`。**与本轮任务/金币改动无关**，是既有缺口。

- 运行时真正加载的地下城目录是 **dungeons.next28.json**（113 图，dungeon 1~9），不是
  generated.json（23 图）。原因：`_next37` tag 在 channel_probe 顶部被连续降级到 `_next34`，
  于是命中第 85/88 行把目录换成 next28.json。
- 怪物生成解析器（`internal/dungeon/session.go: fixedMonsters`）只认 `[normal]`/`[boss]`，
  不认 **`[champion]`（精英，rank 1）**。next28 里唯一一只精英在**地图 58570**（dungeon 9
  的一个房间），解析失败 → 整个房间进不去。
- 精英怪那行数据比普通/BOSS 多**一个尾随字段**（map 58570 实测是单个 0）。不消费它会导致
  下一行从错误偏移读起，报 "template-0 invalid source row"。

改动：`[champion]`→rank 1；champion 行后消费一个尾随 0。

**验证**：新增 `TestEveryRuntimeMapParses`，把 next28.json 的 **113 张图全过真实解析器**，
改前 1 张失败（58570），**改后 113/113 全过，0 失败**。next28 / generated / tutorial 三份
目录扫描均无其余未处理的 spawn option。

## 2. 装备穿不上——澄清（非 bug）

用户截图的鞋 `100261068`（"Old Cracked Leather Slippers"）提示框写明
"Explorer Collection - Guide Quest item. Open the Explorer Book to register" ——它是**图鉴
收藏品**，登记用，本就不可穿。

穿装备本身正常：DB 里角色 3 的 worn 槽位已穿上我发的整套弓箭手基础装（10005 coat /
12005 pants / 14005 shoulder / 16005 waist / 18005 shoes / 20006 amulet / 22005 wrist /
24005 ring）。

## 3. 任务金币——已发放并落库，但屏幕不刷新（真缺口）

DB 证据：`character_quest_rewards` 中 **quest 3149 receipt = {"gold": 4300, ...}**，角色金币
1000673 → 1005210。**金币确实发了。**

屏幕数字不动的原因：**钱包金币从未发给客户端显示层**。排查：

- SELECT 响应（`SelectProbeState`）只有 `Cash`（点券/cera），无 gold 字段
- 入场序列（basic/addition/skills/vault/inventory/worn/area/fatigue/…）无任何 money 包
- 副本内拾取金币走 `GoldPickupConfirmed`（CMD39），带**场景对象**、是**增量**——城镇交任务
  没有场景对象，用不了
- 客户端城镇金币计数器只由拾取增量累积，与 DB 总额**脱节**；任务金币是纯 DB 入账，无增量
  推送，因此不显示

**结论**：要显示钱包金币，需要在入场包里定位"金币"字段（`EntryBasicProbe` 里有约 15 个
置零的 u32，其一应是 money），或恢复城镇 money-set 包。属于需原生取证的功能，未实现，
**不猜**（乱填会错位破包，之前吃过亏）。金币不丢，在库里真涨。

## 4. 待续（优先级）

1. 钱包金币显示包（定位 money 字段）——用户最在意
2. 血量 0.0143 倍率根因
3. 51 级+ 的 [seeking]/[hunt monster]/[hunt enemy]/[condition under clear]
4. 装备分解 CMD26、商城 CMD1302、邮件 CMD63/2036
