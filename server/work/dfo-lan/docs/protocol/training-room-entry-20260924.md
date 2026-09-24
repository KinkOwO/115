# 修炼场入口修复候选（2026-09-24）

状态：**attempt 1/3，用户实机确认修复通过**。此记录和候选版构成训练场入口的确认基线。

## 现象与依据

- 训练场 5000 家族的 `.dgn` 缺 `[basis level]`，旧解析器将其列入 skipped，不能建立副本会话。
- 门请求 `CMD15(5000)` 需要 ACK15 后的 NOTI27 才能把客户端抬入选图模块；随后 `CMD16` 走现有 ACK16、NOTI28、NOTI29 入图链。
- 门显示由客户端资源的 quest 10100 条件控制。本文不更改任务脚本或角色存档。

## 本次改动

1. `[minimum required level]` 保持必填；缺 `[basis level]` 时采用 `[recommended level]` 首个正整数，仍缺失则采用最低等级。训练场标为不耗疲劳。
2. 从 `server/work/client-build/Script.inner.pvf` 按原目录相同的 SHA-256 `7ef2db59…44d88e80` 导出四个副本和七张地图到 `configs/dungeons.training-room.json`。当前 `runtime/pvf_source/Script.inner.pvf` 为另一份 `be95d64e…` 快照，因此未混用。启动时按校验值合并 overlay，拒绝覆盖原副本或地图。
3. 对训练场的 CMD15 发 ACK15、NOTI27；训练场的 CMD16 按已见的 Mode/Flag 范围和单人字段校验，进入现有副本入场帧。CMD2062 的同目标路径也走训练场选择。

## 代码检查

- `go test ./...`：通过。
- `go vet ./...`：通过。
- `go build -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe`：通过；未覆盖归档 39 版。
- 用户确认实机修复通过。
- 候选程序 SHA-256：`873A9F9FE63906B9F5A8F6696750FBC79D7FA9EA992B5C4D02519BA09B75686E`。
- overlay 与 `dungeons.full.json` 校验值相同，四个副本 ID 和七张地图 ID 均未与基目录冲突。
- 当前 `runtime/pvf_source/Script.inner.pvf` 的快照校验值虽不同，但这四个 `.dgn` 与七张地图的脚本 SHA-256 逐项和 overlay 一致。

## 回滚

只回退本次 `internal/catalog/dungeons.go`、`internal/dungeon/training_room.go`、`cmd/wireprobe/dungeon_flow.go`、`cmd/wireprobe/main.go` 的改动及训练场 overlay；不改数据库，也不触碰其他工作区文件。
