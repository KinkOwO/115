# 弓箭手星座时装礼包漏发已修复（2026-10-01）

状态：用户已确认修复并授权提交，2026-10-01 收口。确认源码和默认程序SHA256均为 `6b15b723263f28ee50137046529b50d5605f38cdd9797533be88f7b4895ed8ab`。用户实机会话在22:37:24记录ACK160 count=8，模板与弓箭手原生请求一致。

## 现场与资源

- 会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261001_220404_515843_next37`，同一礼包590723045。
- 22:05:18角色1弓箭手CMD160 category=14、slot=77；请求含8个模板，旧服务端ACK160只列前3件；用户确认背包也只有3件。
- 22:06:00角色20黑暗武士category=15、slot=65；请求含8个模板，ACK列8件。
- 只读当前 `server/work/client-build/Script.inner.pvf` 的 `stackable/cash/590723045.stk`：弓箭手类别14的 `[avatar]` 为517562678、517552724、517572681、517522692、517502715、517512699、517542695、517532681，每件数量1；与实机请求逐项一致。类别15的8件也一致，因此本次不是礼包配置缺失。
- 两条实际C2S明文、模板列表和时间固化在 `internal/game/protocol/testdata/native_booster_avatar_package_20261001.json`。

## 权威IDB

复用已打开的 `client/DFO.exe.i64`，不直接重新打开或覆盖数据库。

- `14573DC60`：`14573DD4B`写u16槽位、`14573DD5D`写u32数量；`14573DD71/14573DD85`分别写两个u8类别；`14573DDB0`或`14573DDF3`逐条写u32选择模板，无独立模板计数。
- `14573DE2B`写u8时装属性条数，`14573DE4E/14573DE5F`逐条写u32模板和u8选项，`14573DE78`写末尾零。
- `14573E120`普通分支同样在 `14573E701/14573E747`写全部u32模板，`14573E776/14573E785`写两个零（无属性选项）。
- `14539C3C0@14539C452`向14573DC60传入属性窗口的2744向量；`14539C820@14539C897..14539C8E4`按模板28字段匹配并更新36字段的选项；`14539A410@14539A55A..14539A5B3`将窗口模板与1379礼包窗口的144字节条目模板匹配。属性条目属于所选时装，不应由任意模板低字节伪造边界。

## 根因与修复

原 `DecodeBoosterUseRequest` 从头扫描所有4字节对齐候选边界，只验证选项数、末尾零与填充。弓箭手第4件517522692的小端字节为 `04 c5 d8 1e`；负载offset20的04被误认成4条属性选项。后续20字节恰好能消耗至合法零尾，提前接受这个边界，留下3个Selections，并误解出4213102789、2650480344、3641911070、517532681作为属性模板。

本次仅在非空Selections的非零属性候选段增加模板身份检查：每个属性模板必须非零且属于前面的选择列表，否则继续寻找真实边界。保留现有零属性、属性选择、无选择属性兼容及扁平列表处理；不按职业或礼包ID硬编码，不加额外奖励、不改ACK格式。发放复用现有扣盒、角色保存和回执事务。

## 验证及存档边界

- 修复前真实弓箭手向量稳定失败（3件+4条伪属性）；修复后两职业均解析为8件、0属性。
- 协议及网关专项通过，含既有单件属性选择/皮肤盒；真实请求端到端用例确认扣1盒、8件保存入时装背包、原有时装和无关角色字段保留、NOTI14完整刷新与ACK160的8件列表一致。
- `go vet ./...`通过；`go test ./...`仍为工程既有5项失败：AdventureAuditProvenanceAllowanceIsNarrow、PVFCatalogGateRefusesRewardChangesAndDoesNotFallback、EnhancementAuditAllowsOnlyMissingOrdinaryTicketExpirationHeader、OdysseyChapterFinalLordDrop、OdysseyCurrencySceneRetryAndPoolIsolation。使用Go overlay将唯一生产代码改动booster.go还原为HEAD，在未覆盖工作区的情况下重跑这5项，全部复现相同失败；本次无新增失败。
- 不改schema、玩家数据库、客户端资源、导出JSON、启动设置。以前已经消耗的盒子及漏发5件不自动回写或补发。

## 候选与手动验收

确认的源码及默认程序SHA256均为 `6b15b723263f28ee50137046529b50d5605f38cdd9797533be88f7b4895ed8ab`。22:37:24实机ACK160 count=8且模板与请求相符；该确认限于该礼包弓箭手漏发及其他职业回归，不扩大为所有礼包逐项验收。

源码候选旧文件SHA256：`c518e50af555178018e85604eb45c814d77f0d108175b8c5170196b5f3e468b5`，精确备份 `.tmp/archer-box/wireprobe-handoff-source.before.exe`。需要回退时关闭会话后复制回 `bin/wireprobe-handoff-source.exe`，无需数据库迁移。
