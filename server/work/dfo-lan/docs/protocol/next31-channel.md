# 31版频道选择回归修复

用户现象：30版能显示 Select Server，但连接黑屏后返回，未到角色列表。

## 确认原因

30版运行 `manual_20260911_054519_420817_next30` 仅出现频道刷新，游戏网关没有收到连接。原客户端记录 `[ SERVER_GROUP_TYPE_CAIN, 0 ch ] has no data in DB.`。

1. 原 `1451fa5e0` 从目录名称提取数字，`#LAN Local` 得到0；配置频道是1。目录改成由实际ID生成的 `#1`，友好名称保留在脚本中。原函数执行回归证明旧名称0、新名称1和23。
2. 目录 `first` 在此客户端映射到 Cain 枚举0；`144d98940` 将它转换为脚本服务器键1。旧脚本使用服务器0。只读原客户端表快照证明旧 `{server_id:0,channels:[1]}`；新配置使用服务器1。

## 原客户端验证

31版运行目录：`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260911_055222_318423_next31`。

- 原客户端 10:52:36Z：`CHANNEL>> [ SERVER_GROUP_TYPE_CAIN, 1 ch (type:2) ]`，此前“no data in DB”错误消失。
- `runtime/channel-table-before31.json`、`channel-table-after31.json` 分别保存服务器表修正前后的快照。
- `runtime/channel-runtime31.txt` 为该次频道日志摘录。
- 当前该运行使用游戏地址127.0.0.1:52452；下次启动端口动态分配。
- 这证明频道目录和配置成功匹配；点击选服到角色列表仍需本轮实机结果，不把列表读取成功等同于完整登录成功。

## 其他

保留30版全部任务/结算/金币/技能修复，只更换频道目录与31版频道配置。根目录 `Start-DFO.cmd` 已切换31。

同时修复怪物观察器启动时读模块列表的竞态：客户端PID写出时进程仍处于启动阶段，可能返回WinError299。增加模块就绪重试。31版观察器已记录 `observer_started` 且无启动错误。仅使用ReadProcessMemory，不在客户端写入数据或设置断点。
