# 巴卡尔Final Strike、怒气突跳与指挥官显示核对（2026-10-07）

用户本轮实机反馈：最终Boss死亡后动画一直循环，点击Skip才能回营地；右上进度条一次上涨约50%～70%；NPC下方次数截图均为0。两张截图为用户提供的画面证据。最新会话为`roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261007_125124_732486_next37`，使用默认map-reset程序，不是未接线的entry-evidence诊断候选。

## 怒气确有突跳，来源为三路漏防

新加坡时间12:57:20，N570符号200（BAKAL ANGER）同一tick从1400→3400→5400→7400。此前原生位置11、22、51各有进攻怪，随后分别发送N2286移除行。不是单个伤害报告或普通秒增长跳6000，而是三个路线末端的事件同刻执行。

当前`contents/2022/bakalraid/etc/bakal.etc`正常模式分别定义：timer1/11、1/22、1/51到期，执行`[INCREASE BAKAL ANGER] location 2000`及`[DELETE MONSTER] location`。源失败阈值是8000。因此每路25个百分点，三路同刻合计75个百分点，本次从17.5%→92.5%。用户所述大幅上涨有日志证据；截图99%也与随后仍有怒气增长相容。

此前口头把6000说成60个百分点，是误按10000满值换算，已明确更正为当前源8000满值下的75个百分点。没有修改源惩罚或添加平滑显示规则。

`deleteEventMonster`删除实际placement同时取消对应location的timer，已有击杀不会继续沿相同预约链移动到末端计罚。本次三个末端各有实际移除事件；不能据“大幅涨条”将源漏防惩罚当成无怪计罚缺陷。

新增原生回归执行INIT三条预约及四次60秒移动，在末端前核验placement存在，随后怒气增量扣除当秒速率后严格等于源EscapeAnger之和6000，三只源对象随后消失。测试不复制生产规则。

## Final Strike自然结束未验收

- 12:58:48.949，二阶段100003149/map100007763的实体4097死亡，清场。
- 12:58:48.959，发布BAKAL PHASE2及final selection。
- 12:58:49.403，真实加载100003165/map100007771，实体4096为源rank3模板109015394。
- 12:59:51.792，最终场景实体4096死亡；12:59:52.625，发回城N3/N23/N24、N2285回营、N13背包、N588通关、N574结束。
- 用户确认这段场景死亡及回营是点击Skip后的结果。不能把这些包记为自然动画结束成功。

当前源`100003165_last.dgn`起点0,0/map100007771、boss格1,0/map100007772、hunt boss109015394，与实际选图一致；`bakal_3phase_dummy.mob`及`proc_dummy.act`生成本地动画对象。`passiveobject/manager/3phase_manager.obj`的`o:last_action_cool=300000`，`manager_start.act`达到这一dungeonTime后才进入lastaction。该本地循环参数为5分钟；raid源在本体clear时另设`SET TIMER28/56/180`，为服务端结算兜底。

本次在场约63秒后Skip，因此尚没有超过180秒的自然等待日志，不能认定服务端永远不结束，也不能否定用户看到自然结束异常。当前服务端在最终演员死亡或源结算期限到达后进入奖励冻结/回营链；不能无原生依据改成强制杀演员、复制未知剧情包或修改客户端资源。

新增真实源回归：三龙clear后本体clear确实进入Final；SettlementTimer期限前不settle、到期可settle；奖励准备后Tick发布结算并结束。此检查证明源状态机期限逻辑，不代替客户端自然画面或真实存档事务验收。自然交互/终幕结束的确切缺口继续取证，未发布动画修复。

## NPC次数与reader核对

本次N2288：12:53:15开局`020202020219000000ffffffff`；后续逐次授予；12:58:48为`04040303031900000000000000`，表示5个源指挥官库存分别4/4/3/3/3，并非全部0。

当前客户端2288注册链为`144cd49d0 → 144cde360 → 142542f70`：reader整块读13B；consumer将body0..4五个u8写入库存+10e4/+10ec/+10f4/+10fc/+1104，body+5为u32 used kind（25表示无使用），body+9为u32目标。现布局和日志字节对应，库存没有被误当作NPC编号或u32数组。

因此截图全0与服务端已发送库存不一致，当前不能宣称NPC显示正常。用户回答此前没有注意到是否仅最终场景变0，此边界尚无实机信息。继续核查客户端缓存/初始化/显示时序；没有证据直接改布局或以叠包试探替代闭环。NPC键位配置提示与剩余次数是不同信息，不将快捷键未设置当作库存0的结论。

## 验证与交付范围

新增`internal/legion/bakal_final_review_test.go`。显式真实归档Bakal/RaidRecovery专项全部通过，全仓vet通过。全量仍仅两项既有character测试失败：TestEntrySkillsPreservePayloadAcrossProfessionHashChange、TestEntrySkillsRejectDifferentProfessionReference；没有新增失败，结果位于`.tmp/bakal-ui-20261006/final-review-native-tests.txt`与`final-review-all-tests.txt`。源脚本、客户端和玩家存档未修改；没有新增进度条平滑、替换指挥官规则或改变最终动画结束行为。默认map-reset程序保持。三个问题中，怒气突跳已解释；自然动画结束与NPC画面仍未修复，不能写成confirmed baseline。

取证输出在`.tmp/bakal-ui-20261006/`：final-actor-actions、final-actor-definition、final-objects及144cde360/142542f70反汇编；临时脚本和大输出不进入Git。
