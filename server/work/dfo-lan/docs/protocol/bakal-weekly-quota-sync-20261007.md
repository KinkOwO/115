# 巴卡尔建团窗口周次数同步（2026-10-07）

用户截图显示Weekly Entry/Reward Count均为2/1，但当前角色建团被“普通巴卡尔本周通关次数已用完”拒绝。已定位为客户端缺少原生周记录，而非需要清空角色存档。此候选为attempt1/3，待用户手动实机验证。

## 当前原生契约与显示根因

- 原生Kind0 N1434 `RAID_WEEKLY_CLEAR_INFO`注册于144CD49D0，消费144CE0120：u8顶层数量；每行u8 raid kind、三个u32、u8子行数量；子行u8 key与三个u32。开头清除客户端quota map，之后存入记录，不需要角色实体字段。
- 当前建团窗口1420B2C90的1420B4246..4282：144CF3C60取得PVF的周入场上限，144CF16E0(kind,1)取得已用次数，二者相减。后者读取quota节点+2c，即每行第二个u32；记录缺失返回ffffffff/-1，导致1-(-1)=2。
- 奖励字段1420B523A..5257：144CF3590(kind,1)取得源奖励上限，144CF3450(kind,1,0)读取key0第二个u32的已用奖励次数，相减。缺行同样返回-1。
- 此字段语义为“已用”，不能发送剩余0，否则客户端会反算成剩余1。
- 官服`captures/20261005-015111/frames.jsonl`时间1791136454121的Kind0 N1434，完整reader消费796B（采集800B的尾4B为补齐）。raid8的53B行，顶层三个数均0；子key0/1/2各三个数均0。逻辑原生向量保存在`internal/game/protocol/testdata/raid_weekly_clear_info115.hex`。

协议构造按源RaidID生成单行、子key0/1/2保留原生布局；只写已闭合的顶层第二字段和key0第二字段。其它字段保留原生零，不推测其它次数/模式语义。正常/困难建团窗口的这两个getter均查询raid8的共享字段，本轮不新增困难玩法或另一套计数规则。

## 同一源与同一账本

`contents/2022/bakalraid/etc/bakal.etc`的WEEKLY CLEAR COUNT与WEEKLY REWARD COUNT由已有catalog读取。客户端分母继续来自PVF；后端门禁继续使用同一rules。

新增只读`workflow.BakalWeeklyUsage`统一查询当前角色`bakal_raid_rewards`的clears/rewards。BakalRaidAdmission也调用该函数，保持原门禁语义。周键继续沿原有database.IspinsWeekStart；旧周投影为0，既有字段/待领计划/未知存档字段保持，不落库、不迁移、不改变冻结/授予/回执行为。

注意现有账本的clears/rewards是原实现的两种独立计数（未获资格的冻结推进clears、资格冻结预留rewards）；本轮没有将它们改成另一种“总通关数”定义，也没有复制PVF的周上限到Go或JSON内容表。窗口忠实显示同一账本，角色已被clears门禁拒绝时，入场字段同时显示耗尽。

## 同步时序

- 仅选中巴卡尔频道82的角色发送N1434，不虚构其它raid的计数。该包会替换raid缓存，因此不为无存档证据的其它raid生成空状态表。
- 选角成功重置本会话同步标记；场景就绪C35后发送。若玩家先请求建团/编队/费用/成员资料，也在对应处理前恢复。
- 成功通关、奖励事务提交且回营/结束包发送后，再用最新w.role恢复计数；失败结算不制造新的已用数。
- 后续城镇活动只在role或计数改变时发送，避免每秒重复发布。跨周由同一投影恢复；失败发送不缓存成功状态，可重试。
- 同一次分发的显示同步与建团门禁共用now，避免周边界两次取时导致一帧内不一致。

新事件`bakal_weekly_quota_synchronized`记录角色、raid kind、已用clear/reward、源上限和plain_hex，便于实机核对。服务端仍保留源周门禁；目标是让窗口与拒绝原因一致。

## 验证与交付

回归覆盖：原生raid8逻辑行逐字节一致；两个getter字段的已用值；未知字段不猜填；缺省/旧档/跨周投影与准入一致且不改存档；发送失败重试；未建团就能同步；结算后账本变化刷新；重新选同一角色重新恢复；其它频道不发送。

显式真实归档catalog/legion/protocol/workflow/wireprobe Bakal/RaidWeekly/RaidRecovery专项全部通过，全仓vet通过。普通全量仍仅两个既有character兼容失败：TestEntrySkillsPreservePayloadAcrossProfessionHashChange、TestEntrySkillsRejectDifferentProfessionReference，无新增失败。结果在`.tmp/bakal-ui-20261006/weekly-quota-native-tests.txt`及`weekly-quota-all-tests.txt`。Python25项启动检查与launch_local.py --check通过，原客户端D:\115us\DFO与PostgreSQL路线保持。

候选`bin/wireprobe-bakal-weekly-quota-candidate.exe`，SHA256 `8177f587d1b949c50b61c34cc307df8e767136cae51c98e63dcb618df813c93b`。默认与既有隔离profile已接线，保留前一版inventory-restore和map-reset程序，未重启运行中的服务/游戏。NPC加载后库存同步包含在此候选；Final Strike自然结束的未闭合项继续按既有记录，不宣称本轮修复终幕。

用户关闭当前会话后手动“启动游戏.cmd”，使用当前角色进入巴卡尔频道再打开建团窗：已耗尽的入场次数应显示剩余0/1，不再出现2/1；奖励次数按实际rewards账本投影，源门禁仍一致拒绝耗尽角色。未修改客户端/PVF/玩家库。scripts/check-commit-hygiene.ps1仍缺失，未绕过提交门禁或提交用户其它改动。
