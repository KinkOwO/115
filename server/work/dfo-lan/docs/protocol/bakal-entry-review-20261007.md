# 巴卡尔一阶段入口核对与确认范围（2026-10-07）

用户本轮确认：二阶段已完成；退出、换频道、返回角色列表的勾选确认窗已完成；通关竞拍界面仍未接入。此确认不扩大到所有房间出口、完整通关或地图图标清空。确认来自用户反馈，不把当前默认候选指纹当作这些画面对应的实机程序身份。

用户进一步说明一阶段问题为“第一次进入角色人物在最右侧小房间，没有在最中间的小房间”。最新 runtime 仍为 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261007_023721_496480_next37`；没有更晚的 map-reset 实机记录。

## 入口与战斗房证据

- 新加坡时间02:51:20.818：C2062请求地下城100003149、格点2,1；服务端requested/resolved均为2,1，map100007435。
- 02:51:21.307：加载完成，living为空。02:51:23进入2,0，02:51:24回到2,1。
- 02:51:27.376：C45请求1,1；02:51:27.643加载map100007434，living包含entity4096/rank3/template109014482；随后N2194创建包的grid同为1,1。
- 当前源 `contents/2022/bakalraid/etc/bakal.etc` LOCATION24：DUNGEON100003149、LOCATION XY1,5、LOCATION SPECIFIC XY1,1。`bakalmonster.cos`本体模板109014482、POSITION945,311、第二模板109014483、APPEAR GRID2为1,5。
- 既有官服 `captures/20261005-015111` 的C2062时间1791137063201同样请求2,1（此前cinematic-lifecycle取证已记录）。原生入口房与首阶段战斗房不同；目前没有根据将首次入口强制搬到中央，也没有根据在右侧创建本体。

因此用户描述与日志一致。服务端在中央加载时发送首领创建；这不等于客户端画面已验收。如走到中央仍看不到本体，需要用户手动新会话的日志和画面反馈进一步核对。

## 本轮服务端取证完善

`observedGameRequest`复用已有`legion.BakalRequests`，保留656/657/2089/2069/2070/2073/2074/2261/1134每次正文。旧会话2069超过8次只剩帧元信息；现在不再丢失后续战报与传送证据。当前解密与checksum已和日志采样解耦，此修改只影响记录，不称为“8次后玩法失效”修复。

拒绝事件增加当前run、地下城、地图、格点、loaded及living。未改包、出生点、怪物位置、玩法源、PVF、客户端或存档；没有新增或重复维护内容规则。

回归使用023721真实C2062及中央C45，走portal→Select→loading→move→loading→N2194；验证入口无首领、中央创建包与玩家房间一致、只创建一次。显式真实归档Bakal/RaidRecovery专项通过，全仓vet通过。全仓`go test ./...`仍仅两项既有character兼容测试失败：TestEntrySkillsPreservePayloadAcrossProfessionHashChange、TestEntrySkillsRejectDifferentProfessionReference，结果保存在`.tmp/bakal-ui-20261006/entry-evidence-all-tests.txt`。

诊断候选 `bin/wireprobe-bakal-entry-evidence-candidate.exe` SHA256 `3b5c8c7dda3b213a80e902eba97ee1f71340e8f3a716853627cada351fb98a74`。默认profile仍为 `bin/wireprobe-bakal-map-reset-candidate.exe`，未替换或重启用户会话。用户继续运行“启动游戏.cmd”验证地图图标清空；此诊断候选暂未接入默认。

工作区缺少server/AGENTS要求的`scripts/check-commit-hygiene.ps1`，未绕过提交门禁、未暂存或提交用户其它改动。
