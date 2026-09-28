# 新手教程副本末房首领校验（2026-09-28）

## 现场证据

- 本地会话角色 8 通过教程 CMD15/CMD16 进入源教程副本 7112。
- 末房切图通知记录角色位于 `(4,0)`、地图 `53126`。客户端随后发送 CMD117，目标实体为 `0x100e`。
- `configs/tutorial-dungeons.current36.json` 中副本 7112 的 Maze 0 将 Boss 坐标设为 `(4,0)`；对应房间记录 `boss=false`。末房通知中的实体 `0x100e` 是该房间 Rank 3 怪物。
- 服务端日志拒绝 CMD117：`boss check target is not a source boss in this room`。该分支因此没有进入 `completeDungeon`，也没有发出通关 NOTI31。

## 修复

教程副本使用源 Maze 的 Boss 坐标判定末房；仍要求 CMD117 目标匹配当前房间实际生成的 Rank 3 首领。普通副本仍要求源房间的 Boss 标记与 Maze 坐标同时匹配。

## 行为边界

通过通关校验后，服务端发送现有通关和结算通知。返回城镇仍由客户端在结算界面发送 CMD72 选项 2，或发送现有 CMD42 离开请求后触发；此修复不添加未经原生证据确认的自动跳城通知。
