# Roar Ravine 任务 12430 入场拒绝，2026-09-28

## 实机请求与资源证据

- 用户截图：Roar Ravine，Lv.100；已接取 `[Side Story] Heroes on Top of the World`。
- 会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260928_135611_028309_next37`：角色 11 在 UTC 06:26:28 接取 12430；06:49:10、06:49:12、06:49:13 的 CMD16 均为 `6789d7170000000000ffff000000000000000000000000000000000000000000`。现有原生请求 decoder 解得副本 400001383、Quest=0、Difficulty=0、Party=65535，随后服务端拒绝 `no resolved source maze for requested quest`。
- 从当前 115 资源导出的 `server/work/client-build/Script.inner.pvf` 只读提取 DGN、两张 MAP、QST；四份脚本 SHA-256 均与当前加载配置一致。归档 checksum `7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80`。临时取证存于 `runtime/roar-ravine-entry-20260928/`，不入库。
- DGN `contents/2021/ozma_raid/dungeon/quest/side_quest/400001383_roar_canyon/400001383_roar_canyon.dgn`：SHA-256 `0ca58809c2c0396181eb3ecbdfb5301fb010533719cda4c0eb22a01a96fc6e14`；`[story mode] [quest list] 12430`；唯一 maze 的 `[quest connection] 0 12430 -1`、尺寸 2×1、起点 (0,0)、Boss (1,0)，MAP 为 400004000、400004001。没有 Quest=0 的普通迷宫。
- MAP 400004000 SHA-256 `50923503c061b2c4fd3e716e5def2e29ad568d8d9e052a6212400d27dcfa5466`；400004001 SHA-256 `538a57a7f84bc5e2ea02ae39ae9e9209499c3548979f61bd61d63e895c3a14e0`。两张图已经存在于 `dungeons.full.json`，无需补图或改配置。
- QST `contents/2021/ozma_raid/quest/side_quest/ozma_side_1w.qst` SHA-256 `504b29a7ce4202640df4562e1e8fb4595195fd126789381a09aa97b79af0d30a`：任务 12430 为 `[clear map]`，目标 MAP 400004001，后继任务 12431。

## attempt 1/3：Go 服务端修复与实机确认

原 `dungeon.Select` 只拿请求 Quest 与迷宫 Quest 做精确匹配。本次在既有迷宫选择前解析缺省任务：请求 Quest=0 且副本所有迷宫都有任务绑定时，只允许角色已有权限、解析完整、任务 ID 唯一的源路线。任务权限沿用网关 `acceptedQuestIDs`：配置版本一致且状态为 accepted 或 completed，不扩展未接任务权限。多个不同的有权限任务匹配时拒绝；同一任务的多张迷宫沿用已有选择规则。当前实机场景中 12430 为已接取状态。

显式任务请求仍按原任务校验；副本只要声明任何 Quest=0 的普通迷宫，便保持原路线，即使该普通迷宫尚未解析完整，也不自动换成任务路线。未接任务、未达等级、任务路线不完整的请求仍拒绝。

本次仅改服务端迷宫选择，不新增 opcode、字段、codec 或客户端补丁，不改变数据库、角色存档和任务目标。回滚点是 `session.go` 中 `selectionMazeQuest` 调用及同名新增文件；测试文件 `roar_ravine_entry_test.go` 固化原生实机请求、两张源地图的怪物解析与 StartMap 编码，并覆盖任务归属、等级、普通路线优先、歧义和显式任务边界。

## 验证与基线状态

针对入场与路线边界的回归测试、全量 `go test ./...`、`go vet ./...` 均通过。候选 `bin/wireprobe-handoff-source.exe` 已重新构建，SHA-256 `890E5683152888F0AF2B2EE1ABF9C0119524CD2276F3720F66D1FF4236977C6C`；未覆盖归档 39 基线。Go 编译缓存单独使用忽略的 `runtime/go-cache-roar-ravine/`，避免全局缓存访问拒绝；未修改 Go 工具或全局配置。

构建时原服务端会话仍在运行，该进程不会自动载入新程序；需由用户关闭当前游戏及服务端、重新启动后验证。

**confirmed baseline（attempt 1/3）**：用户已确认重启候选服务端后可以进入 Roar Ravine。候选 SHA-256 `890E5683152888F0AF2B2EE1ABF9C0119524CD2276F3720F66D1FF4236977C6C`。确认范围只包括成功入场；Boss 房、剧情和任务目标推进尚未确认。
