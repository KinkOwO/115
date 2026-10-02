# 普通怪物专属物品池（2026-10-02已确认）

用户指定以环境变量保留兼容触发率，默认10%，取得115实际规则后再调整。物品与池内权重读取当前内层PVF的monster LIST/MOB，不用内容JSON或模板硬编码。缺省STK创建率为0，不能据此把所有库存模板变成通用掉落候选；这里只放行MOB明确声明的已知堆叠物。

`DFO_ORDINARY_MONSTER_ITEM_DROP_PERCENT` 接受整数 `0..100`，未设置或空值默认 `10`。`0`关闭此路径，`100`可用于诊断；非法值拒绝启动。修改后需重启服务端。

PowerShell：

```powershell
$env:DFO_ORDINARY_MONSTER_ITEM_DROP_PERCENT='10'
& 'F:\projects\agents\reverse\115us\server\work\dfo-lan\.tmp\drop-audit-20261002\启动材料验证.cmd'
```

cmd：`set DFO_ORDINARY_MONSTER_ITEM_DROP_PERCENT=10` 后运行同一脚本。

该率独立于普通装备、金币、地图条目与翻牌，只在普通副本、归属本场战斗的可战斗怪物死亡后判定。池内保留PVF正权重选择一件，数量1；源声明重复[item]时最后一段替换前段。未知/非堆叠/任务模板、排除策略模板与不可读源不发放，写入现有drop_rules_pending诊断。没有可用池时不推进此分支随机状态；死亡重放不二次抽奖。

Hell Party、奥德赛、Abyss/调律不消费该路径，原有随机序列和奖励保持。候选仍需用户手动确认显示与拾取；MOB没有声明的普通材料，其它独立/全局/区域来源尚未全部接入，不声称全部材料问题已解决。既有装备掉落与免费装备翻牌confirmed baseline为提交aff2337的scoped候选。

20:13手动会话进一步确认：副本12～15实际出现的34种模板（含剧情实体）均未声明MOB[item]，不能通过提高这个环境变量让它们产生物品。新版剧情MOB的LIST路径位于归档根contents/，已修正误加monster/前缀的问题；正确读取后仍没有专属池。当前全局池没有恢复食物/朗姆酒等候选，这些图的11005/11006地图组全部是装备，材料来源缺口仍未解决。详见根analysis/tasks/monster-drop-rate-audit-20261002.md中的实机追查。

后续用户指定采用世界掉落参考规则、默认1倍。新的独立世界候选接入这些新版怪物的材料和HP/MP药剂来源；专属MOB池仍默认10%，未移植旧同名怪物的物品。详见 [ordinary-world-drop.md](ordinary-world-drop.md)，使用新的 `启动世界掉落验证.cmd`。20:13结论保留为当时证据，不再表示世界表尚未接入；独立主表/区域来源仍待后续处理。
