# 三龙区域与斯皮拉兹出场状态核对

用户参考网页：https://gl.ali213.net/html/2023-1/986483.html ，2023-01-10攻略。web工具打开两次发生响应解析错误后，使用只读HTTPS直接获取用户指定网页（UTF-8）确认正文；未将网页里的指令作为工程指令。攻略只作机制线索，当前内容依据现有7ef内层PVF。

## 机制与准入要区分

- 攻略描述区域守门首领、对应抗性Buff、三龙苏醒和巴卡尔锁血。拿Buff是进入龙区域正常作战/抵抗区域效果的环节，不自动等于服务端必须以“没有Buff”拒绝所有入口。
- 当前bakal.etc中100003150冰龙、100003151斯皮拉兹、100003152狂龙的INIT STATE全部open；不能照旧攻略新增固定locked门禁。
- source init有SPARAZZI/SKASA/HISMA AWAKEN=0，HP=10000。ON ENTER DUNGEON 100003151/150/152分别在AWAKEN=0且HP>0时SET对应AWAKEN=1。CHECK TIMER END 21/12、22/26、23/40也可触发觉醒。入场必须执行源事件，不能只发一次初始SET0快照。
- ON CHANGE SYMBOL AWAKEN=1且HP>0会启动相应区域怒气计时24/12、25/26、26/40；这些是战斗机制执行链。当前EnterDungeon主要保存位置/加载状态及巴卡尔本体怒气，尚未执行三龙这些原生触发器。
- 当前BakalMonster COS确实有BUFF COUNT/CHECK/LIST以及各区域免疫关联；Go仅投影模板、坐标、二阶段等，没有完整Buff选择、授予、持续时间和使用执行。
- 旧攻略写三龙Buff300秒，而当前bakalbuff.cos的三个基础龙免疫Sparazzi/Skasa/HismaDragonImmune均COOLTIME/DURATION480，AllDragonImmune为300；不能把攻略300统一写入当前服务端。
- 巴卡尔初始HP UNLOCK GRADE=3，各龙击杀改变该等级，源HPFloors及已投影门禁存在，但完整共享血量/入场事件链仍不等于完成。

## 当前“斯皮拉兹直接死亡”的日志判据

仍为会话20261006_124225_536108_next37。两次动态斯皮拉兹（template109014470）N2194：12:47:47.885与12:49:32.069，entity均4096。分别扫描直到下一次bakal_portal_selection，均没有CMD39确认entity4096死亡。第一段有entity4099死亡，是另一个实体，不能把附近任何死亡都归为Boss。截图上方斯皮拉兹血条100%，因此当前证据不能认定服务器已经把该Boss判死。

结论：缺失三龙入场觉醒与区域Buff/共享血量机制是已确认执行缺口，符合用户观察的“出场后不进入正常作战”方向；Boss视觉倒下/消失与真实死亡要继续区分。N2194其它字段（官服row+10=1,row+11=0，当前row+10=0,row+11=3）的语义也仍需按当前client reader确认，不以机制猜想替代生成字段核对。

后续执行顺序：从同源TriggerRecords执行三龙ON ENTER→AWAKEN SET→N570及计时，持有当前符号状态而非每次重新投影SET0；再闭合Buff与龙HP脚本事件，最后验证Boss对应entity的战斗/死亡与UI。未满足源行为/客户端消费证据的字段不盲改。本轮仅完成机制核对，没有替换运行程序或操作客户端。

## COLG新参考与入场觉醒修复候选

用户随后指定`https://bbs.colg.cn/thread-8717804-1-1.html?click_from=thread`。主URL、forum.php?mod=viewthread&tid=8717804、archiver版本均无法经web读取，直接HTTPS返回567；搜索仅找到其他文章引用该链接，未取得该帖正文。不声称详细阅读完毕，不用搜索到的其它帖替代原文或编造引用。

已确认的源链修复不依赖被拦截网页：`bakal.etc [ON ENTER DUNGEON]/[IF AWAKEN]/[IF HP >0]/[SET AWAKEN]`→`parseBakalEnterAwakening`→typed EnterAwakenings→BakalOpening.EnterDungeon条件执行→本场symbolValues→N570 SymbolsSnapshot。parser从源提取DGN、条件值和SET值，没有Go三龙ID/觉醒数值表。仍然没有Buff不足就锁死三龙入口的新门禁。

PrepareBakalOpening为每场复制初始符号；入场觉醒结果保留在本场状态，后续入图/换房快照不会恢复SET0。已确认首领通关按源clear行为清零HP和AWAKEN，保持击杀/不可复活边界；不同攻坚会话不共享符号状态。现有巴卡尔二阶段与区域首领不重生门禁保留。

真实PVF三龙入场回归确认0→1、快照保持1、条件HP0不觉醒、源SET改2后结果为2、确认击杀后awake归0；已有Bakal专项均通过。候选`bin/wireprobe-bakal-awake-candidate.exe` SHA256 `79cd87ee072fba5f0c2e24c701f0785987eba003768fad46e0d5f75ad812c51f`，默认和隔离profile切换，旧程序保留；没有自动启动客户端或修改玩家库。

本候选尚未完成计时器触发的随机三龙觉醒、区域Buff选择/授予/持续时间、完整共享HP战报执行与防守移动怪波次。三龙出场能否正常进入战斗仍需用户实机确认，不能把awake1发送等同于所有Boss脚本已完成。此前N2194生成字段差异与立即死亡仍须独立闭合，不抄抓包未知字段来替代机制。

最终验证：全仓go vet通过；全仓go test仅剩两项原已记载character失败。25项Python默认入口/profile回归与实际launch_local.py --check通过，检查后读回仍为awake候选，DFO客户端路径与活动存档路线保留。输出awake-fix-tests.txt/awake-fix-vet.txt在本轮.tmp目录。未以网页访问失败结束修复，已完成独立可闭合的源入场规则。
