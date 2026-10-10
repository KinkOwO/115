# 巴卡尔活首领入口回归：普通服务房与战斗房绑定

2026-10-06。用户反馈arena-retry版本中邪龙、狂龙和精英怪进入后退场，只有冰龙能战斗。新增本地攻略参考：`D:\115us\colg_bakal\巴卡尔团本攻略.md`。攻略用于流程对照，原生PVF和抓包继续定义执行规则。

## 实际运行与原因

运行目录 `20261006_154036_543853_next37` 的server_pid36984；只读进程列表确认实际执行 `wireprobe-bakal-arena-retry-candidate.exe`，创建时间15:40:36。后续minion-presence-final构建时间16:11:19，本会话并未运行它。不要将本会话当成存在状态修复的实机回归。

本次的真实缺陷是上一轮恢复 `bakalLoadedBoss` 出场格点守卫后，漏掉 `enterBakalPortal` 的活首领入口转换。该遗漏由本任务造成，不能以测试直接进入指定Boss格点替代完整入口链验证。

当前原生入口包2062请求的是普通/战后格点；源位置存在活首领时，`LOCATION INFO / SPECIFIC LOCATION XY` 指定实际战斗房。交付源码 `raid_bakal_portal.go:94` 起明确实现这一转换。当前源码原本直接把r.TargetGrid传入Select，故被合法地图校验接受，却落在错误的服务房；恢复后的Boss出场守卫随后正确拒绝在该房生成，产生空战斗场景。

| 首领 | 本会话请求格点 | PVF活首领战斗格点 | 本会话结果 |
|---|---|---|---|
| 邪龙sparazzi | 0,0 | 1,2 | Map100007446，无N2194首领生成 |
| 狂龙hisma | 0,0 | 0,1 | Map100007459，无N2194首领生成 |
| 冰龙skasa | 0,1 | 0,1 | Map100007439，有N2194/109014476 |
| 门将blona | 0,0 | 0,1 | Map100007525，无N2194首领生成 |
| 门将nympha | 0,0 | 0,1 | Map100007487，无N2194首领生成 |
| 门将gerda | 0,0 | 0,1 | Map100007526，无N2194首领生成 |
| 门将basilisk | 0,1 | 0,1 | Map100007484，有N2194/109014480 |

这解释了冰龙可以打而其它入口失败的差异。邪龙/狂龙在本次服务器日志里没有对应动态实体生成，不能把不同房间重复使用entity4096的CMD39全部归到前一只首领身上。取证脚本必须重置每次房间的实体归属。

## 修复

- 2062解码后，根据同源位置表匹配请求的普通格点；如果存在未击杀的当前创建对象，则选择其SPECIFIC格点。二阶段对象继续使用源APPEAR GRID2。
- 已击杀或未创建的源位置继续使用普通格点；不强制所有入口进入Boss房，不绕过源地图存在性和范围校验。
- 保留出场格点守卫、单场击杀去重、小怪存在状态与初始延迟创建、三龙觉醒快照和失败后重新开始修复。
- `bakal_portal_selection` 日志增加requested_grid、resolved_grid、occupied_arena，便于下一次直接核对入口选择。

实际捕获的7组2062向量加入专项测试，走 `enterBakalPortal → Select → loading → N2194` 完整链，验证真实模板全部生成；已击杀门将的普通入口不会重建首领。此前只给bakalDungeonEntry直接传源Boss格点的测试不足以覆盖该回归，本次已补齐。

## 攻略、PVF与当前实现对照

| 攻略规则 | 当前源/证据 | 当前状态 |
|---|---|---|
| 门将房需要实际战斗，击杀后才可进入/穿越 | LOCATION/SPECIFIC XY、注册实体CMD39、源2070返回 | 本轮入口修复；清场返回守卫保留 |
| 三龙觉醒时领域DEBUFF生效 | ON ENTER/HP>0/SET AWAKEN、205～207符号 | 已有觉醒状态保持；BUFF领取/运用仍未完全实现 |
| 固定怪物/守门掉落领域抵抗与增益 | bakalmonster.cos BUFF COUNT/BUFF CHECK/BUFF LIST | source已读取线索，完整选择/授予/时长仍待闭合 |
| 480秒领域抵抗、部分300秒特殊抵抗 | 同源bakalbuff.cos，攻略与当前PVF相符 | 不以攻略另建Go数值表；效果执行未宣称完成 |
| 固定怪物可以按规则再次刷新 | ETC RESERVE CREATE MONSTER及计时/移动事件 | 源INIT首批预留创建已实现；后续复活波次/移动执行仍有缺口 |
| 巴卡尔血量阶段与机制需要三龙配合 | HP UNLOCK GRADE、源第二模板/APPEAR GRID2 | 原有阶段门禁保留；完整伤害报告/所有房内机制不在本轮声称完成范围 |

本次不调整装备/人物伤害、怪物血量倍率、客户端或PVF，不把抵抗BUFF缺失解释成怪物必然自杀。

验证：真实PVF Bakal专项通过，全仓vet通过；全量测试仍仅两项既有character来源兼容失败，无新增失败。未启动/停止用户游戏或访问玩家库，实机仍由用户手动验证。

交付候选 `bin/wireprobe-bakal-live-arena-candidate.exe`，SHA256 `75427e459ed989330ea893143f29a06a2ecf1b4a9aa9700a74aa232526b079b2`。默认与隔离profile同步，25项Python启动/profile检查通过；`launch_local.py --check`确认该候选、客户端D:\115us\DFO、PostgreSQL路线。旧程序保留，用户结束当前游戏会话后运行D:\115us\115-server\启动游戏.cmd测试。
