# 巴卡尔阶段切换、传送、作战元信息与邪龙空房（2026-10-07）

## 已定位与修复

用户011723会话：首次入场100003149格点2,1，随后移动到1,1才创建109014482。这与官服20261005-015111的2062（1791137063201，目标2,1）相符。原生入口与中央战斗格是不同房间，不能凭截图把出生点强制搬到首领身上。

01:26:49的C45目标仍1,1，Record为`010000000505c80280010000000000000000`，被普通相邻检查拒绝。源DGN [move map even enemy]允许此图有敌人时移动。新增窄路由：拥有同一raid/DGN、同格点、Record前四字节为1、源允许有敌移动，复用场景模式0的N29和原生传送坐标，不清空／重生当前首领，不扩大普通相邻移动规则。

随后2070持续收到“raid stage warp outside cleared owned combat”，共37次。源`contents/2022/bakalraid/monster/bakal/action/proc.act`在HP阈值和HP UNLOCK GRADE条件成立时切换，首个actor仍活着；第二模板／格点来自bakalmonster.cos。新增原生PROC阈值解析，网关验证第一阶段源arena、所属rank3 actor、源解锁条件，再接受ACT的2070阈值报告。阶段切换发送ACK2070、源HP／PHASE更新与真实N29，等待既有加载握手后才创建第二模板。旧actor仅退役，不虚构C39奖励。重复同阶段2070只ACK，不重置HP／PHASE／房间，不重播开场。普通未清怪2070返回仍受原门禁约束。

发现N578攻坚队资料一直保持State0、phaseff、开始时间0，虽然N574已active。当前144CDBF80分别读取其state到+2c、开始值到+38；官服active模式2有state2/phase0/Unix开始时间。已按准备1、作战2、结束0同步mode2资料，并发送当前run开始时间；下一run时间和状态从新源实例获得。它与N574缓存不同。退出警告由客户端状态和弹窗流程决定，不能复制伊斯N2254或伪造窗口包。此状态遗漏确定修复；截图所示勾选警告是否完整出现仍待实机验证，不宣称已强制实现二次确认。

## 021400会话邪龙区域没有出口

第一run1791310522292684700于02:19:17进入布洛娜100003154 arena0,1，02:19:26.498确认实体4096死亡且alive=null/cleared=true，02:19:30发送2070返回，02:19:32进入邪龙路线；02:19:43.779重新请求已击杀的布洛娜arena0,1。旧入口仅抑制boss重生，却继续让玩家进入这个空arena，之后没有2070请求，02:20:38攻坚失败。

修复明确源状态：目标是具有SPECIFIC XY的已击杀arena且尚未源CREATE复活时，转到LOCATION普通XY。布洛娜为0,1→0,0，不凭空生成出口／复活首领。其它此类区域同源映射适用，巴卡尔主本双阶段不走这个兜底。真正源再创建会撤销击杀抑制，原活arena绑定随之恢复。原生回归覆盖该捕获路径。

## 重开、确认范围与测试纠正

用户确认再次点击开始没有重复生成。实际测试仍使用recovery候选63e34574，当前cinematic/lifecycle修复此前尚未发布。确认范围只有失败重开不重复生成，不能扩展成全部出口、弹窗或完整通关。冻结快照见restart-confirmed文档。

本轮发现需要原生归档的用例依赖`DFO_PVF_CORE_TEST_ARCHIVE`，裸go test会跳过这些用例。已显式设置为实际Script.inner.pvf，重跑catalog/legion/wireprobe/protocol Bakal专项（含前两轮预算和恢复）、结果全部通过，输出boss-native-tests.txt。此前普通测试通过不等于这些原生用例已经执行，验收说明以此纠正。全仓普通test/vet另行保留，不将两类检查混淆。

新增回归：首领活着时进入二阶段、第二地图真实加载、重复2070幂等、同格点传送保留actor、上局已击杀门将在新局只生成一次、mode2作战元信息起止、已击杀布洛娜重进arena转普通路线。进房日志附DGN/map/grid/location、实际placements、living和defeated，后续缺图标／缺实体可逐项对应，当前没有证据证明随机再创建本身必然重复或丢失。

竞拍UI仍未开放，玩家库／客户端／PVF未修改，游戏操作仍由用户手动完成。

交付：bin/wireprobe-bakal-cinematic-lifecycle-candidate.exe，SHA256 a170aadf68b92568a66fe9b36df652ac906b2524b871d48d2cff5a4e9d565b92。默认／隔离profile接线，10+15启动测试和launch --check通过；客户端D:\115us\DFO与PG路线保持。显式归档Bakal/恢复专项通过，普通全仓test仅两项既有character失败，全仓vet通过。没有替用户重启游戏，未用新程序冒充已确认recovery快照。
