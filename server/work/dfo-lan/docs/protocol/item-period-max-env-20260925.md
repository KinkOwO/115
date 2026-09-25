# 带期限物品最大期限开关（attempt 2/3）

状态：通用实现已完成代码检查，待用户手动实机回归。590012183 是已确认的样例，代码中没有该模板的单项分支。

## 真源与范围

- 当前客户端对应的内层 PVF 和原始导出 PVF 中，`stackable/590012001/590012183.stk` 都声明 `[expiration date] 2025-04-29 09:00:00`。原会话 `client_trace.txt` 的该物品为 `period(0)`；用户实机确认下发最大值后不再显示过期，礼盒能够使用。
- `cmd/itemperiodimport` 遍历源 PVF 的 `list/stackable.lst` 与 `list/equipment.lst`，只按 `[expiration date]`、`[usable period]`、`[period]`、`[usable datetime]` 标记导出模板 ID。原始配置源共 124,610 个模板，包含 590012183。用当前客户端对应的内层 PVF 独立重导，得到完全相同的 124,610 个 ID。
- `configs/item-period-tags.json` 带源 PVF 校验值。启用时必须与角色目录的配置源一致，否则服务端拒绝启动；不使用未知来源的期限模板表。

## 开关行为

设置 `DFO_MAX_ITEM_PERIOD=1` 并启动源码候选版时：

- 上述 PVF 标记模板的普通道具、装备、装扮和宠物下发期限为 `2147483647`；旧存档中为 0 或过期值的实例也适用。
- 未列入表但存档中有非零期限的实例，也按带期限物品下发最大值。
- 服务端已有的契约、礼盒和袖珍罐实例期限检查同步遵循开关。
- 只改发往客户端的期限和使用判定，不批量改数据库存档，也不修改客户端 PVF。未设置或设为 `0` 时沿用原有行为。

该开关应在启动服务端的管理员环境中设置，启动后检查日志 `maximum item period enabled for 124610 PVF templates`。启动器会把自身环境传给服务端。需要关闭并重启会话；在运行中修改环境变量不会更新已启动的进程。

在已提权的 PowerShell 中可用：

```powershell
$env:DFO_MAX_ITEM_PERIOD = '1'
.\启动游戏.cmd --source-build
```

重导配置表：在 `server/work/dfo-lan` 中运行 `go run ./cmd/itemperiodimport -source ../client-build/Script.inner.pvf -output configs/item-period-tags.json`。

## 验证边界

代码测试覆盖开关关闭、PVF 标记实例的零/旧期限、仅存档有期限的实例、普通装备记录复制、宠物与装扮的附加期限行、来源校验失败以及重复模板拒绝。用户仍需手动测试至少一个非 590012183 的带期限物品；若失败，只记录该物品的客户端 trace、服务端事件和源脚本，再取证具体消费路径。

`go vet ./...` 通过。`go test ./...` 中本功能相关包通过；`internal/catalog` 的两条 NPC 好感度源解析测试（`TestParseNPCFavorRulesFromSource`、`TestParseBwangaFavorProfileFromSource`）失败，其文件未被本改动修改，需由对应功能线另行处理。
