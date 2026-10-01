# PVF直读第二批：十四领域已确认（2026-10-01）

状态：用户于2026-10-01确认正常，本批十四领域直读及去除所选JSON启动对照升级为confirmed baseline。上一批九领域程序继续保留供回退。

## 本批变化

| 新增领域 | 真源与完整核对结果 |
| --- | --- |
| skills | 各职业自己的skill列表及`.skl`；3224条，与当前生效`skills.next27.json`按职业/技能键逐字段一致，包括技能类型、前置、费用、觉醒和VP字段 |
| prices | 源物品`price/value/add price/add value`；599682条一致；88条非法价格继续缺席并拒绝交易，金币模板0不进入出售表 |
| materials | 物品自己的`[need material]`；14211条的路径、材料模板和数量完整一致；模板0的金币成本保留 |
| boosters | `.stk`的booster/package及智能掉落组；42504条完整奖励池一致，226次智能组替换；不改变既有抽数、权重或空结果行为 |
| tutorial | `etc/tutorial/tutorialflow.etc`；16条路线与现有目录重新投影后的结果一致；教程副本/地图仍由原目录提供 |

技能的旧`skills.release.json`与当前生效`skills.next27.json`有12条技能定义差异；本批以启动编排实际使用的next27核对，未用旧release覆盖已确认技能规则。职业创建、默认技能栏、黑暗武士组合栏策略仍保留原配置。

材料JSON声明的来源是旧外层PVF哈希`2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167`，而角色存档锚点是inner哈希`7ef2db59…`。材料旧加载器不把这一元数据绑定存档；本批完成全部14211条成本/路径严格一致后，直读目录记录实际inner来源。此结果只证明材料投影一致，不添加通用来源别名，不重写存档版本。

礼盒仍报告6159个现有解析器不能投影的booster body；掉落组21469仍因源结构不完整被拒绝。迁移没有为这些条目猜测奖励，也不扩大已实现的开箱范围。选择箱、抽奖表和独立COS礼盒仍在后续清单。

## JSON对照与直读启动分开

`-pvf-verify-baselines`默认true，保留显式审计方式。设为false或`DFO_PVF_VERIFY_BASELINES=0`后，所选十四领域仅导入PVF，不读这些领域的导出JSON做启动对照；预期inner SHA256和角色来源一致性仍强制检查，异常在打开玩家存储前退出。

联合测试先完成十四领域全量对照，再把所选领域全部JSON路径设为不存在，成功准备相同目录并按ID读取装备。来源锚点`characters.skycastle-release.json`、本服策略和未迁移领域仍使用现有配置；这是所选领域去除JSON依赖，不是整个项目已经无JSON。

完整对照准备41.42秒，GC后heap为1641709136字节，约1.53GiB；无导出JSON对照的准备35.02秒。这些是独立测试数据，不是完整服务的RSS或峰值。

## 启动与回退

新profile为`server/work/dfo-lan/configs/pvf-next-candidate.json`，使用`.tmp/pvf-next/bin/wireprobe-handoff-source.exe`，SHA256：

`b8668e5ef9e56ed37f3ba313fb9349625aa64e0cf16dd41b67e13192a85f0db9`

源归档继续为`server/work/client-build/Script.inner.pvf`，完整SHA256：

`7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80`

关闭已有游戏会话后，由用户在项目根目录手动执行：

```powershell
./tools/python/python.exe -B ./server/work/dfo-lan/scripts/launch_local.py `
  --repair-profile ./server/work/dfo-lan/configs/pvf-next-candidate.json --check

./启动游戏.cmd --repair-profile ./server/work/dfo-lan/configs/pvf-next-candidate.json
```

依赖检查已通过，实际客户端仍为`F:\wip\dof\115US`，运行身份与上一批一致。准备阶段未启动客户端、重启服务、修改玩家数据库或覆盖日常exe。

回退上一批已确认直读程序：关闭候选会话后，使用`pvf-direct-candidate.json`；其程序SHA256仍为`95b009aa41e830a70aa1fc76e150b186b7caef49a02842b8e9ffd07a5255e44e`。不传profile则沿用日常启动。两份候选程序都保存在`.tmp`，不入Git。

## 手动回归

1. 已有角色入场，检查装备、金币、材料、任务状态，换装和重新选角。
2. 检查已有技能等级、觉醒技能与快捷栏；沿用原本要执行的学习、退款或组合栏操作，重登确认保存。
3. 用普通NPC出售一件原本准备出售的物品，比较金币；购买原本准备购买的金币/材料商品，检查成本和数量。
4. 打开已有且此前正常的固定内容盒子，核对物品、数量和背包位置；不以本次迁移测试尚未实现的未知箱子。
5. 完成熟悉的普通副本，检查任务/经验、掉落和结算。教程路线可在用户原本计划创建的角色上验证，无需修改已有角色进度。

`gateway.err`应出现十四领域的`PVF ... prepared`、来源和准备耗时；准备好的目录随后由业务加载器复用。记录角色和操作时间，后续结合`events.jsonl`、`gateway.out`及`client.log`验收。

## 尚未接入的商店与后续工作

商店导出器按文件名前缀绑定ID，发现一个ID对应多份源文件，例如`100000375_global_6th.shp`与`100000375_global_6th_seria.shp`。旧导出器最后覆盖其中一项，当前基线绑定后者；`list/itemshop.lst`没有这两份周年旧表的引用。尚缺NPC开店引用/当前客户端消费路径闭环，不能把目录顺序当成正确绑定。因此`shops`没有加入允许的直读领域，商店JSON继续提供现有绑定与报价。

后续优先迁移强化/增幅券与费用、附魔和随机词条、普通装备/掉落投影、主副本及覆盖、特殊奖励和嵌入目录；职业策略分类、商店绑定与同名COS礼盒需要各自补证据。完整清单继续维护于[实施计划](PVF直读实施计划.md)。

本批`go test ./...`、`go vet ./...`、7项Python准备/profile测试、十四领域联合对照与缺失JSON路径测试通过；用户实机确认记录见下文。


## 2026-10-01用户实机确认

用户确认“确认正常，提交并开始接下来的迁移吧”。十四领域直读及去除所选JSON启动对照成为本批confirmed baseline。实际会话`roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261001_042311_786667_next37`运行`.tmp/pvf-next/bin/wireprobe-handoff-source.exe`；十四领域准备32.157秒，技能3224、价格599682、材料14211、礼盒42504及教程16条均从PVF准备。日志有4次商店购买请求/回执/材料更新、1次礼盒打开回执及各容器更新、9次装备移动提交、18次装备槽刷新和技能入场恢复，客户端`SUMMARY exit=0x0 normal_run=1`。事件计数不代表所有规则或教程路线均逐项人工验收。源与程序哈希保持本页记录，存档版本、日常默认及未迁移领域不变。用户反馈与上述日志共同构成本批确认依据。
