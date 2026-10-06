# Clone 原生来源关联修复

实机确认构建：`fdd81028`（整合上游前，源码基线`6fc7057`）。本次实机确认复制／解除、实际穿脱及无死亡进图回城正常；套装效果显示问题已搁置。

## 修复方案

Clone 本体保留在穿戴槽，普通外观完整实例保留在装扮背包，以 `CloneSource{Slot, Template}` 建立关联。复制只更新关联；解除复制保留两件物品的位置和内容。来源在背包内移动时跟随，移出或缺失时解除关联。

资格来自同一 PVF Source 的 `[equipment type]` 与 `[item category] clear avatar`。选角、穿脱和切场景同步来源表；mode1重建穿戴后恢复非装扮装备，防止缺席槽被清空。

协议契约：CMD19成功业务体为12字节，撤销使用末尾mode1；NOTI1437采用 `count:u8 + (wornSlot:u8, source:u16 LE)*count`，来源值为背包槽+12，无来源为FFFF。count11清表并读取0..10，再单项更新槽11。

## 存档兼容

旧同槽Group0 Clone＋Group1普通外观经PVF部位校验后，将普通实例移到容量内最小空背包槽并建立关联。保留Record、属性、徽章、期限、锁定和未知JSON字段；容量不足、定义无法校验或关联冲突时拒绝迁移。

迁移在角色事务中保存Before/After回执，重放不重复搬物，schema和存档来源身份不变。`workflow.RollbackCloneAvatarMigration`仅在角色仍等于迁移后状态时恢复，后续玩家变化会阻断回滚。仅切换回旧EXE不等于兼容恢复。

## 验证结果

- Go1.26.8：整合上游`00ebff63`后的提交工作树通过`go build ./...`、`go vet ./...`、`go test ./... -count=1 -json`；3294通过、272跳过，失败集合与上游基线均为空。
- 隔离SQLite／PostgreSQL16迁移与受保护回滚通过，覆盖完整实例、未知字段、幂等、归属校验和后续变化保护。
- 原生向量及真实dispatcher回归通过；真实PVF离线验证装扮槽资格、普通装备／誓约槽拒绝和来源保留。
- 精简后的最终构建已实机确认复制／解除、实际卸下再穿及无死亡进图回城。日志有5次成功移动，撤销来源先于mode1 ACK；卸Clone后的20个非装扮恢复槽与存档一致，来源普通实例保留。

## 未完成项

- 普通Avatar和Clone的套装效果窗口仍可能带出誓约面板；此项已搁置，不采用客户端修改。
- 本次没有副本内复制／解除或换房请求；副本CMD19额外重挂载时机及换房仍待专项实机验证。

## 参考交叉核对

`GF115_CLONE_AVATAR_SHAREABLE_REPRO_GUIDE_20261005.md`的本体／来源分离、仅清关系和来源表语义与本次取证一致。其国服DNF.exe使用16B撤销ACK、count12来源表；本客户端使用12B业务ACK和count11清表＋槽11单项。字段、容量及场景时序按各自构建验证，不直接移植。

## 证据入口

- 协议向量：`internal/game/protocol/clone_avatar_test.go`；存档与dispatcher回归：`internal/workflow/clone_avatar_test.go`、`cmd/wireprobe/clone_town_sync_test.go`。
- 自动验证：原实机任务的忽略目录 `.tmp/clone-native-contract/mr-prepare-20261005/`；上游整合后的源码尚未重新实机验证。
- 最终实机：`runtime/roles_clone_f8_fixedport_20261005_172308_644221_next37/events.jsonl`，复制／解除及穿脱见行208..327，进图回城见行355..426；读回核对见任务`.tmp/clone-native-contract/final-live-audit.json`。
