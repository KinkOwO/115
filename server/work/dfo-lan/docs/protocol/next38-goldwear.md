# next38 — 金币显示刷新 + 装备穿戴修复（自查记录）

二进制：bin/wireprobe-dungeon37.exe（原地替换，旧版备份为 wireprobe-dungeon37.pre-goldwear.exe）
SHA256：c58951ccaa85207024529c28b56b85e49d14d5c9c9b58236a24616860a036cc7
go test ./... 全绿，go vet 干净，launch_local.py --check 通过（PG 在线）。

## 本轮根因与修复（均已自查，不需用户先测）

### 1. 任务金币"没有增加"——根因是显示不刷新，不是没入账
- 事实核对：角色 6666（id=3，弓箭手 prof16，adv0，lvl11）DB 金币=1,005,210。
- character_quest_rewards 显示任务 3149 正确入账 4300 金币（exp 22000，无物品）。
- `[gold reward table]` 是 questparameter.etc 的真实源表（200 级档：[9]=360…），配合 [difficulty] 权重与等级惩罚，与经验用同一公式；3149 的 4300=penalty100×((1200×360/100)/100)，是源数据正确结果，非捏造。
- **Bug（Gap 1）**：cmd/wireprobe/quest_flow.go 的 finishQuest 只有 `len(Items)>0` 时才重发背包包（NOTI13，其中 slot0/template0 行携带金币余额）。3149 的 [job] 物品成长型不匹配未转职弓箭手 → 无物品 → 不发 NOTI13 → 屏幕金币不刷新（重登才会显示）。用户看到的正是"金币没增加"。
- **修复**：改为 `len(Items)>0 || Gold>0` 都重发 NOTI13。入城本身已在 NOTI13 携带金币行（Bootstrap→InventoryRestore(bag.Rows())，实测入城包 row0 amount=当时金币），拾取路径也已重发（loot_flow NOTI14 id14）。

### 2. 鞋子/自己的装备无法穿戴
- **Bug**：internal/inventory/wear.go 的 wearable() 用 `Catalog.Basic()` 作穿戴门槛。Basic 是"掉落池"规则（要求 [free] 附着 且 稀有度≤1），不是"能否穿戴"规则。于是角色已拥有、且满足等级/职业/成长型的装备被拒穿：
  - [trade delete] 绑定鞋（探险家收集鞋 100261068，实测 [shoes]/[trade delete]/rarity0/lvl1/[all]）；
  - 每一件稀有度2的普通掉落。
- **修复**：穿戴改用结构性 `Reward()`（校验附着/稀有度/类型/耐久），保留槽位匹配与 WearableBy（等级/职业/成长型）。掉落池仍由 Basic 构建，故掉落不受影响、也不会开放到别职业能穿。
- **实测（真实目录+角色）**：100261068 现可穿到 slot17（原被拒）；14005 控制项 OK；稀有度2 lvl15 肩甲对 lvl11 弓箭手仍按等级正确拒穿。新增回归测试 wear_equip_fix_test.go。

## 已核实为"正确/非服务端问题"（无需改动）
- 进图卡住/进不去：上一轮 [champion] 生成解析修复，113/113 运行地图全解析（allmaps37_test）。本轮 events.jsonl 里 6 次 dungeon_request_refused 全部就是旧版 [champion] 报错。
- 技能栏：服务端对 prof16/adv0 下发 26 个可学技能（6 初始 + 学习目录 ForAdvancement(0)），不是 2 个；客户端 Script.pvf 校验为已恢复的 35 版（BA2C94FE…）。服务端与 PVF 均正确，若客户端仍显示少，属客户端按等级门槛/PVF 技能树渲染，需在客户端确认。
- 任务奖励物品：按源 [reward int data] 的 `[job]<职业><成长型><数量>` 过滤。弓箭手物品奖励成长型均为 1-5（转职子职业），未转职正确不发；通用物品（如 3146 的 20002/24002/22002）正常入包并已在 DB 确认。

## 未做（需要动态证据或属独立特性，避免瞎猜）
- 怪物血量 0.0143 倍率根因（需副本内动态追踪）。
- 副本结算/翻牌（CMD46 → NOTI34/37/35）与卡片（CMD69-71）细节。
- 邮件/商店/分解/时装等命令。
