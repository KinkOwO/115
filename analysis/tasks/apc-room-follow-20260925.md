# APC 过房不出现：2026-09-25 取证

状态：未修复。只撤回了 594516b 引入的无效跨房 APC 注入；不宣称队友已能跟随。

## 复现

- 用户实机确认首房能刷出 APC，但副本内走门过房后不出现。例子是 57 级任务 “Retrieve the AT-5T Walker”。
- 该任务对应 quest 3529、dungeon 88 / maze 1；首房 map 92263 有 `[ai character] 27011`，第二房 map 92264 没有 `[ai character]`。
- 首房 92263 的源脚本把这只 APC 标为 `[character] [cinematic] [normal]`，并明确写 `[apc follow type] none`。
- 2026-09-25 14:43 实机会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260925_144309_593823_next37/events.jsonl` 在 92263→92264 过房时，服务端向下一房 NOTI29 塞入 template 27011 / SourceIndex 10000 / team 0 的 APC 行；客户端仍没有显示它。

## 当前客户端 IDA 证据

权威 `client/DFO.exe.i64`，本次活跃 IDA MCP session `3c129d6a`：

- `sub_145B257F0` 分发 NOTI29 的怪物行。APC rank 5..8 分支在 `0x145B25E8A` 检查 SourceIndex；若 `>= 10000`，先取模 10000，再于 `0x145B25EC8` 调用 `sub_145B20DC0`。
- 指令 `0x145B25E92`–`0x145B25EAA` 的常数除法结果已用等价整数运算复核：10000→0、10001→1、20000→0。因此 10000 到达创建函数时已变成 0。
- `sub_145B20DC0` 虽有参数 `a3 == 10000` 的模板直建分支，但正常 NOTI29 调用点无法抵达它。其 0..63 分支先查询当前地图 APC 表；92264 表为空，查索引 0 返回失败。
- `sub_145B20DC0` 的代码调用交叉引用仅见 `0x145B25EC8`。这不能完全排除虚表、间接调用或另一条独立消息路径，但足以否定现有 SourceIndex 10000 方案。

## 实现处置与剩余缺口

- 撤销会话中的 `companions` 跨房克隆逻辑，以及 NOTI29 允许 SourceIndex 10000 的错误门禁；保留原生地图 APC 刷出路径。
- 这不是“已修复”。要实现真正跟随，仍需找到当前 115 客户端接受的跨房 APC 创建/携带路径，或确认该任务资源的 `follow type none` 本来就不要求这只剧情 APC 跟随。不得用其它 SourceIndex、rank、extra row 组合盲试。
- 旧参考实现 `../usdof/server_proto/game/dungeon/helper-dungeon_ally_companions.py` 记录 type-5 APC 跨房克隆曾导致客户端闪退，因此不把它当作 115 客户端的可用事实。
