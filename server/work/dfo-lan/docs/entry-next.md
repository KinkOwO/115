# 第一阶段完成后的接续点（2026-09-11 00:29 UTC）

第一目标已完成：LanTest01 真实建角、持久化、重开后显示、进入城镇。用户明确反馈“直接进入城镇了  跳过了新手教程”。不再要求重复创建或重启成功窗口。

活动运行 roles_persist_select_actor_town_08：Python exec session22390，gateway29968、probe34020、客户端35368、回环端口49612。动作前核查当前路径/PID。bin/wireprobe-character.exe 来自 bin/wireprobe-town.exe，SHA256 04a21d544e181a2f1fe41341b088f7c7c4eabe016127ca31e277262c61faa940。正常交互模式，无断点，WFP回环限制与Job Object有效。

00:23:18 UTC SELECT 选择数据库角色5 / wireID3 / roster slot2。发送 SELECT success -> USERINFO mode0 -> NOTI24 AREA_USERS。00:24:36 UTC 的 runtime/roles_persist_select_actor_town_08/town_acceptance.json 确认 actor/selfID3、map town38/area0、位置561/234、scene/virtual character存在。用户反馈与原生地图检查共同构成验收；没有保存新的城镇截图。

城镇/地图来自当前PVF导出的 configs/town.generated.json；出生点是独立 town-entry-probe.json 中的本地策略。当前直接进城，教程未实现。后续35/36等请求只看到编号，不能宣称移动/切区已实现。

后续需求保留：独立账号密码登录器、Go按领域分包、PostgreSQL、多人同城与移动、配置驱动。第一目标完成不等于这些功能交付。教程/任务链、完整属性/技能/背包、新建角色自动入场回归亦未完成。

等待视觉验收期间新增 internal/game/protocol/entry_addition.go，是未接入网关的mode1编码器。最小495字节、两个技能树；91字节属性转换用三个原生模拟向量覆盖40字段。Go全部测试、vet通过。来源属性比例和技能三元组映射仍需恢复，不能发送盲零完整状态包。细节见 docs/protocol/entry-userinfo.md，原生模拟脚本 ../dfo_probe_tools/packed_stats_oracle.py。

原始 F:/dnfop/DFO 未改；00:29 UTC 两个主要文件哈希复核一致。PG17端口25438独立运行，七个用户角色保留。runtime/storage/local.json 有秘密，不打印。未来需要重启时只停止核对过路径的owned probe，等待finally清理gateway/Job/WFP；不要杀全部DFO、Go或数据库进程。

画面工具screenshot报0x80004002、树为空pane，不能以shell截图/输入注入绕过computer-use。当前已有用户视觉确认，无需再索要截图。验收索引：../../../outputs/DFO第一阶段验收.json。
