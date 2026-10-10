# Boost 662 教程装备变换 confirmed baseline

业主确认原话：“已确认教程推进，可提交”。确认范围为本角色在115 Boost教程第十步中确认装备变换后，穿戴装备实际替换及教程正常推进；不扩大为全部职业、普通装备库请求头语义、所有生成/变换窗口或费用规则的全面验收。

确认程序为源码入口 `bin/wireprobe-handoff-source.exe`，31,027,200字节，Go1.26.0，SHA256 `8d3b2d50f57d5ac8f43501b26dce978113d84c8ffd88405e4b922a430befc65b`。默认 `bin/wireprobe-pvf.exe` 与39归档未替换；复验由用户手动运行仓库根 `scripts/启动游戏-SQLite.cmd --source-build`。旧源码候选精确备份 `.tmp/boost-transform-routing/wireprobe-handoff-source.before.exe`，SHA256 `6b93bfdc34c8ffdbeee6a91c7ec82595c8068018d064e5f8382cde3840118325`。

## 实机证据

会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261010_040737_176050_next37/`，北京时间2026-10-10 04:16:11：

| events.jsonl行 | 事实 |
| --- | --- |
| 229 | 角色3确认CMD2259；旧代码所谓panel为46，header[12]=0，目标槽14/模板100051289。 |
| 230 | 现有变换事务applied=true，槽14从100051288变为100051289，gold=0、付款方式1。 |
| 231..234 | 成功应答后刷新穿戴N14、背包N13及16444字节图鉴N2610。 |
| 235 | 补发5字节N2638，kind=boost_equipment_mission_progress。 |

教程可见推进以业主反馈确认；日志补充实际换装事务与进度刷新证据，不把仅发包记录当作全部UI验收。

## 真源与兼容

源为 `live/event/kor/2026/0326_boostup/boostup.evt [mission][type] transform equip journal or equip item`，内层PVF SHA256 `4d8c0c82192eb72806d4bec803f5ab638e91e24401d38110cb78fcb09f69e37f`。`boostup.Load/Parse -> Catalog.Steps -> boostJournalSwap`读取当前任务和已领取状态，复用 `workflow.ItemService.TransformEquipment` 原子事务及 `reconcileBoostEquipment/WearRequirement`；没有复制任务序号、模板或分组内容表。

旧门禁把历史采样u32@0=36当固定教学窗口身份，本次原生46证明它不成立。权威IDB与只读分析副本SHA256一致；IDA构造器/发送分支及原始实机向量证据见[取证记录](boostup662-transform-routing-20261010.md)。普通路径历史分派保持；前13字节完整语义继续未闭环。

不改PVF、客户端资源、DLL、IDB、schema、SQLite角色/物品或回执格式；现有存档兼容。此前误生成的两件装备保留，不自动删除。教学免单特例与源[discount cost]的收敛仍列在[PVF迁移计划](../../../../../docs/todo/pvf/PVF单一内容真源改造计划.md)，本轮没有改费用。

## 验证与提交边界

Go1.26.0专项 `go test ./cmd/wireprobe -run TestBoostJournal -count=1` 通过；全量 `go build ./...`、`go vet ./...`、`go test ./... -count=1` 均退出0，无失败集合。源任务变化/步骤迁移及未激活、未领取、越界、毕业、活动关闭等边界回归通过，原始128字节确认请求走实际网关变换分支。

提交前已按根要求检查origin/fork，但当前仓库仅配置gud；沿用当前实际上游 `git fetch gud`，HEAD与gud/main为0/0，无需合并。同步后重跑全量门禁；只暂存本任务源码、测试、CHANGELOG、交接及取证/基线/待办记录。本地二进制、runtime、日志、PVF、存档和其他工作区文件不入库。
