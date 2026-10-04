# CMD41 无复活币时扣 CERA 复活：confirmed baseline（2026-09-24）

状态：用户确认「验证通过」。源码检查通过；本文件只记录已知验收结果，不代填未提供的逐项现场数据。

## 行为基线

- CMD41 按顺序使用奥德赛测试额度、背包复活币、账号 CERA。
- 只有明确识别到奥德赛额度耗尽或复活币堆栈为空才进入下一档；非法请求、禁用复活地图及存储错误直接拒绝。
- CERA 扣款固定 15，使用账号级 `ApplyGrant` 原子调整；余额不足整笔回滚。交易内限制余额在客户端可编码范围。
- 成功下发 ACK 41、复活状态 NOTI 32、余额 NOTI 53。CMD41 失败返回 Refusal(22)。
- 同一会话的同一死亡请求按 frame SHA-256 去重；持久扣款键为 `cera-revive:<run>:<sequence>`。
- 未改数据库结构或 `lifeTokenRevive` 的消费语义。

## 依据

- `analysis/dumps/dstr_id_to_text.json`：dstr 7689 描述无 Life Token 时扣 CERA；dstr 39335 标价 15 CERA；dstr 3015 为 “No more Tokens are available.”
- 协议帧沿用现有 `PlayerDeathState`、`CeraBalance` 与 ACK 41 编码；未新增 opcode。

## 实现与验证

- 实现：`cmd/wireprobe/dungeon_revive.go`、`cmd/wireprobe/main.go`、`cmd/wireprobe/odyssey_revive.go`。
- 错误分类：`internal/inventory/consume.go` 提供空复活币 sentinel；奥德赛额度耗尽使用独立 sentinel。
- 账号扣款上限：`internal/database/grant.go` 在同一数据库事务中应用可选 CERA 余额上限，无 schema 迁移。
- 新增测试：`TestCeraReviveChargesFifteenAndSendsThreeFrames`、`TestCeraReviveIsIdempotentPerDeath`、`TestCeraReviveInsufficientBalanceRefuses22`、`TestReviveFallsThroughTokenToCera`、`TestReviveWithTokenDoesNotChargeCera`、`TestOdysseyCreditsExhaustThenTokenThenCera`、`TestOdysseyCreditsDoNotChargeTokenOrCera`、`TestReviveDoesNotFallThroughUnrelatedErrors`。
- `go test ./...`：21 个含测试包通过；`go vet ./...` 与 `go build ./...` 通过。
- 源码候选程序：`bin/wireprobe-handoff-source.exe`，SHA-256 `DE6F4E8E8FA69D59FA8E191670AAB9A478F9ED83C463D8818135138D1C3932C6`。归档 39 版未更改。
