# 誓约系统修复合并说明（2026-09-29）

## 合并目标与范围

- 目标仓库：`fuckworld/115`，目标分支：`main`（整理时基线 `e5cf77fe7e21cf375c3a76d088e1148c77c72ee5`）。
- 来源仓库：`shawang/115`，来源分支：`fix/oath-selection-persistence`。
- 本次代码范围：誓约选项切换、115 级门槛、誓约晶体槽位校验、选项持久化，以及进副本后恢复已选择的选项。
- 现有 `internal/oath/`、`oath_progress.go`、`oath_grade.go`、`configs/oath-grades.json` 等主仓库文件是依赖，不是本次差异，不应重复覆盖。

## 文件清单

以下路径以仓库根目录为基准。`M` 为修改主仓库已有文件，`A` 为新增文件。

| 状态 | 文件 | 合并内容 |
| --- | --- | --- |
| M | `server/work/dfo-lan/cmd/wireprobe/main.go` | 启动时执行誓约选项表迁移；处理 C2S2382；进入城镇和更换槽位 47 装备后刷新 S2C2839。 |
| M | `server/work/dfo-lan/cmd/wireprobe/entry_flow.go` | 城镇入场包序列加入已保存的 S2C2839。 |
| M | `server/work/dfo-lan/cmd/wireprobe/dungeon_flow.go` | 副本加载后按 NOTI13 完整穿戴容器、NOTI14 槽位更新、S2C2839 已选项的顺序恢复状态。 |
| M | `server/work/dfo-lan/internal/inventory/wear.go` | 增加誓约装备和晶体的 115 级门槛，以及晶体槽位和品级校验。 |
| M | `server/work/dfo-lan/internal/storage/oath_options.go` | 从角色穿戴槽位 47 读取誓约核心；按角色与核心保存和读取选项，并在事务中核对等级与装备。 |
| A | `server/work/dfo-lan/cmd/wireprobe/oath_selection_flow.go` | 处理显式选项切换与场景初始化读取；在副本中拒绝切换，进图时重新读取已保存选项。 |
| A | `server/work/dfo-lan/internal/game/protocol/oath_system.go` | 解析当前客户端的 C2S2382 请求，编码 S2C2839 选项状态。 |
| A | `server/work/dfo-lan/docs/protocol/oath-dungeon-selection-20260929.md` | 记录进图选项丢失的抓包、客户端分析和实机验证依据。 |
| A | `server/work/dfo-lan/docs/protocol/oath-merge-guide-20260929.md` | 本合并清单与验收边界。 |

## 合并与数据注意事项

1. 以主仓库 `main` 为基底合并来源分支；保留主仓库 `main.go` 中原有的名望刷新逻辑。本分支已按该基线集成，不需要用本地整个 `server/` 目录覆盖主仓库。
2. `MigrateOathOptions` 在服务启动时创建或兼容已有的选项表。角色 JSON 中的穿戴槽位仍是装备真源；合并不需要清空或替换现有角色存档。
3. 只提交上表文件。`runtime/storage/pgdata/`、`runtime/roles_*/`、本地 PVF 导出、编译产物和其他本地服务端差异不属于本次合并。

## 已验证与待验证

- 已有候选版编译和实机记录：玩家确认穿戴 Nature 誓约进入副本后，誓约窗口能显示城镇中选择的选项。包序和证据见 `oath-dungeon-selection-20260929.md`。
- 本合并文档不代表已完成主仓库合并后的回归。合并后应按仓库规则执行 `go test ./...`、`go vet ./...`，并由玩家手动验证切换、重新登录、进出副本及更换誓约核心。
- Oath Points 显示数值和 Nature 灾难、增益、主动技能、攻击触发火球等战斗效果尚未得到独立实机确认；本次差异不能宣称这些效果已经修复。

## 合并请求建议

标题：`Fix oath option selection, persistence and dungeon restoration`

说明：本次提交只包含上表的誓约相关文件。选项根据角色等级和槽位 47 的誓约核心保存，城镇和副本加载时向客户端恢复。副本内显示已选项已由玩家实机确认；Oath Points 与战斗效果仍需单独验证。
