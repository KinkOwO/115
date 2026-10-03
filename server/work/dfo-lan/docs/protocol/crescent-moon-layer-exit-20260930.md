# Crescent Moon Lake 中场层图选路（2026-09-30）

## 证据

- 用户实机：任务 22993，副本 100004779，击败中场 Boss 后过场退回上一房，再进下一房崩溃。
- 同轮 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260930_021651_699728_next37/events.jsonl`：18:18:02 UTC 的客户端 CMD45 在层图 100008969、坐标 (2,0) 发起 LayerChange；随后服务端 NOTI29 进入 100015646、坐标 (1,0)。之后 100015646 和 100008969 往返两轮；客户端退出码 `0xC0000005`。
- 源 DGN `contents/2025/delezie_of_plague_scenario/dungeon/100004779.dgn`：迷宫从 (0,0) 沿 X 轴到 Boss (5,0)；(2,0) 挂单张层图 100008969。前一格 (1,0) 是 100015646，后一格 (3,0) 是 100015648。任务 22993 的 `[clear map]` 目标为 100008968。
- 当前运行的 `Script.pvf` 和 `sk.dat` 与仓库 `client/` 的对应文件 SHA256 一致；本轮没有发现客户端资源差异。

## attempt 1/3

`layerSequenceExit` 过去选源 DGN 列表中第一个不挂层图的邻格，因此从 (2,0) 选了已访问的 (1,0)。改为在无层图的相邻格中优先选择**唯一未访问**的格；如果不能唯一确定，保留原有选择顺序。仍只使用当前副本的源迷宫和会话访问记录，不写任务号或房间号分支。

定向回归使用捕获的 18 字节 CMD45 换图记录，断言 100008969 → 100015648、(3,0)。此记录只复现已经观测的客户端输入；没有改字段布局或新增包。回滚范围为 `internal/dungeon/scene_transition.go` 与 `internal/dungeon/crescent_moon_layer_exit_test.go` 本次改动。

定向测试、`go test ./...`、`go vet ./...` 均通过；已编译 `bin/wireprobe-handoff-source.exe` 作为手动实机候选。

## Confirmed baseline

用户手动使用源码候选版确认：中场过场后成功进入下一房。验收范围止于进入下一房；后续 Boss、任务目标 100008968 和整条剧情流程尚未据此确认。候选 SHA-256：`C4061CC97F56435A629664FF57D0C5F4E921B52A4C96E70249FFD2234E0C0511`。
