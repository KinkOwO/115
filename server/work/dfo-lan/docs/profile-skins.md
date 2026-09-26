# 角色背景皮肤登录恢复

范围：恢复当前 US 客户端 category 0 的公共默认组队框、申请框和角色背景，防止 CharBG_Ani 在缺失选择状态时残留 XUI 默认 NEW 动画。原生皮肤拥有与选择状态和 NOTI1759 偏好角色页是独立机制。

## 状态与接口

本地状态按角色隔离在 character_profile_skins，使用带版本的 JSONB 快照保存永久拥有项和三个选择 ID。它是本次初始化修复的存储策略，不宣称已经实现官服付费皮肤、限时皮肤或完整账号共享机制。

首次登录只在快照缺失时授予 PVF Skin.lst 中公共默认 20000、50000、60000；已有快照不重置。RestoreProfileSkins 在事务内锁定归属正确且未删除的角色，插入缺省后从表中回读、校验并提交。独立表避免角色 State JSON 的其他写入者丢失皮肤状态。损坏记录不被静默覆盖；恢复失败中止本次入场准备并记录 profile_skin_restore_error，不发送伪造成功状态。

ProfileSkinRestore 从回读快照编码 category 0 的 NOTI1545 和 NOTI1546。entryPayloads 在进入城镇完成前先发送拥有列表，再发送选择列表；客户端只接受已经拥有的选择 ID。每次选角都会重新发送该类别完整状态，从而覆盖上一角色的客户端缓存。

当前 US reader 的最大类别/全量标志为 10，官服抓包中此批全量标志为 11；不能直接回放官服全量包。默认选择正文 15 字节，拥有正文 5+8*N 字节，第二拥有集合计数为零。

## 实现索引

| 模块 | 职责与位置 |
|---|---|
| 领域 | internal/profileskin/state.go：State、Defaults、Validate，公共初始状态及已拥有选择约束 |
| 持久化 | internal/storage/profile_skin.go：MigrateProfileSkins、RestoreProfileSkins，增量建表、归属与事务回读 |
| 协议 | internal/game/protocol/profile_skin.go：ProfileSkinRestore，当前 US 类别 0 编码 |
| 登录 | cmd/wireprobe/main.go：迁移与选角状态恢复；entry_flow.go：先拥有后选择的发送顺序 |
| 回归 | protocol/profile_skin_test.go、storage/profile_skin_test.go、cmd/wireprobe/profile_skin_test.go |

## 验收边界

自动测试覆盖当前类别格式、发送顺序、未拥有选择拒绝、数据库持久化、并发幂等、角色隔离和损坏拒绝。限时到期换算、皮肤选择请求、非默认皮肤授予与官服共享策略不在本次实现范围。存储集成测试需 CASH_INTEGRATION=1 与可用数据库。
