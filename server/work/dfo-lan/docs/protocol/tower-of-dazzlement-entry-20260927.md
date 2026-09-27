# Tower of Dazzlement 入场取证，2026-09-27

## 用户实机拒绝

- 客户端 CMD16 `plain_hex=b11d00000000000000ffff...` 请求副本 7601；服务端报 `no resolved source maze for requested quest`。选图截图是 Tower of Dazzlement（Basement Floor 1）。此时 `dungeons.full.json` 已有 7601 DGN，但其 2×1 迷宫标为 `unsupported room specification`，没有载入任何房间 MAP。

## 当前 115 资源闭环

- `etc/towerofdazzlement.etc` 的 `[base layer dungeon index]` 是 7601，`[dungeon list]` 列 33 个遭遇副本：7601–7620、7651–7660、7701–7703。前 20 个与最后 3 个是 2×1，中央 10 个是 1×1。`list/map.lst` 中 `map/towerofdazzlement/*.map` 恰好 56 张，每张有 `[dungeon]` 归属和 `[type]` 普通／Boss。7601 对应的两张图是 `dazzlement01.map`、`dazzlement01_1.map`。
- 当前客户端 IDB 只读反编译 `sub_1471E9060`：原生怪物阶级仅识别 `[normal]`、`[champion]`、`[super champion]`、`[boss]`。`sub_1471C18C0` 将未识别阶级初始化为 0。7601 普通图的 `[named]` 因此保留 rank 0；服务端解析器按这一行为接受该标记。没有修改客户端。
- 新增从当前 `Script.inner.pvf` 导出的 33 DGN／56 MAP 覆盖层；加载时检查源 checksum、DGN SHA、MAP 归属、房间数量与坐标，并在校验完成后附加房间。33 个副本均能静态 `Select`、编码起始地图；56 张图均通过怪物行解析。

## attempt 1/3：实机确认的入场基线

- 此次只补源 MAP 和 `[named]` 的当前客户端解析行为，不新增塔专属协议包，也不把迷眩之塔的 33 个遭遇 ID 猜成 7 层固定进度。迷眩之塔的奖励、下一层选择、每日或每周规则与悲叹之塔不同；当前尚未走通对应的客户端通知和结算路径。
- 用户手动实机确认：7601 能进首房、进入下一房间、击败 Boss，并出现结算。当前出现的是普通副本结算；不能据此认定迷眩之塔专属结算、奖励、进度或每日次数已经实现。后续要针对这些功能继续分析实机 CMD 与客户端原生处理。
- `go test ./...`、`go vet ./...` 均通过。实机候选 `bin/wireprobe-handoff-source.exe` SHA-256 `C2F959D025EC73DB48328EED7EE0AEF0C3ADE2D49672B8FA2B84F43DCF6D199F`。首层的入场 MAP 450001、Boss MAP 450002 均由源 `list/map.lst` 和 `[dungeon]` 归属解析，实际入场及换房已由用户确认。
