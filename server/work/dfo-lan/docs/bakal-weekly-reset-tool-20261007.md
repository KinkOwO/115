# 当前账号巴卡尔次数恢复工具

根目录`恢复当前账号巴卡尔次数.cmd`由用户手动执行。当前wireprobe在bootstrap.go中使用开发账号`probe`，该入口只恢复此现有账号所有未删除角色的巴卡尔次数。

操作：先运行“停止游戏环境.cmd”，再运行恢复CMD，完成后手动“启动游戏.cmd”。入口检测DFO和本项目wireprobe进程；在线时直接退出，不停止游戏。随后沿现有launch_local.py --storage-only准备存储，只启动存储。使用当前runtime/storage/local.json选择的PostgreSQL或SQLite，调用编译好的统一dfo-tool bakalreset入口。

变更仅将角色JSON中bakal_raid_rewards的clears、rewards设为0。week、待领plans、竞拍、装备/背包/金币、其它角色字段及已有事件回执保留。逐角色复用Store.ApplyGrant的归属校验、锁定与事务；原完整巴卡尔账本保存在管理审计回执before字段。预检所有目标，坏账本拒绝而不覆盖；单个角色数据库写入失败时报告角色，已完成角色保持成功，可重试剩余目标。已是0的角色不产生额外改动或审计。此操作是用户明确要求的管理恢复，不改变PVF每周玩法上限。

独立用法：在server/work/dfo-lan运行`bin/dfo-tool-bakal-reset.exe bakalreset -account probe`只预览；加`-apply`写入。仅匹配已存在的账号，不使用DevelopmentAccount创建账号。恢复CMD的`--help`分支只显示工具帮助。

验证：JSON未知字段、待领计划和竞拍字段保留； malformed/null/负数账本拒绝；临时SQLite双账号/同账号双角色实际ApplyGrant，验证预览只读、仅目标账号恢复、两个审计回执和重复恢复。PostgreSQL沿既有同一Store事务接口，未在玩家库执行实际重置。CMD帮助分支已实际运行验证；恢复写入分支未执行，不自动改当前游戏或玩家库。

工具专项和全仓vet通过；全仓测试仍仅两个既有character兼容失败，未新增失败。已编译`bin/dfo-tool-bakal-reset.exe`，SHA256 `df61c03cab24991b748776736267c69fc61bb31486e4d4be368681d1c2783ead`。运行恢复CMD不需要临时重新编译；未更改游戏默认程序。

合并清理后，恢复CMD在工具exe缺失时自动调用已有便携Go从cmd/dfo-tool构建；工具已存在时直接使用。exe属于本机构建产物，不需要进入Git。
