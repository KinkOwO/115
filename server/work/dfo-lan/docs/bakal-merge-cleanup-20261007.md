# 巴卡尔合并前清理（2026-10-07）

本轮只清理可重建产物、补充合并清单和恢复CMD的缺文件构建路径。未暂存、提交、合并、启动/停止游戏，未修改玩家存档；其它工作区功能改动保持。

清理期间最后复查发现已有快照提交`b4a4446c`（chore: 同步工作区快照（bakal 团本、venus、sqlite 双引擎、启动器与工具链）），其中已包含文件清单和恢复CMD。本助手没有执行git add/commit或重写该提交；之后新增的清理报告/记录仍留在工作区。

## 已删除

共27个文件、978,113,827字节（约933MiB）：25份已无运行脚本/配置消费者的旧wireprobe-bakal候选exe；`.tmp/bakal-fix-20261006/wireprobe-bakal-fix.exe`；7MB重复PVF导出`.tmp/bakal-ui-20261006/final-scene-source.txt`。这些文件本来已被Git忽略，清理主要释放本地空间，不能把其大小报成仓库提交体积减少。

精确删除路径与大小保存在本机`.tmp/bakal-merge-cleanup-plan-20261007.json`。删除前检查绝对路径位于本项目模块内，只删除指定文件，未递归删除目录。

## 保留的程序和证据

- 当前默认及正在运行的`wireprobe-bakal-weekly-quota-candidate.exe`（8177f587）。
- 回退`wireprobe-bakal-inventory-restore-candidate.exe`、`wireprobe-bakal-map-reset-candidate.exe`。
- 已确认来源`wireprobe-bakal-live-arena-candidate.exe`、`wireprobe-bakal-recovery-candidate.exe`，以及runtime/baselines下两份精确快照。
- 用户要求的恢复工具`dfo-tool-bakal-reset.exe`与根恢复CMD；新CMD在工具缺失时用便携Go从统一cmd/dfo-tool构建，避免合并/新检出后依赖一个不入库的私有exe。
- `.tmp/bakal-fix-20261006/dfolauncher-bakal-fix.exe`仍由该目录的“启动巴卡尔候选.cmd”引用，保留隔离入口及profile。
- 未闭合Final Strike/NPC问题所需的独立取证、反汇编及精简源导出保留；没有把整个.tmp目录或runtime日志删除。

所有bin/.tmp程序、取证中间产物、动态runtime日志和玩家库仍排除合并。

## 哪些测试应合并

[专用文件清单](bakal-merge-file-manifest-20261007.md)列出28个专用Go源码、26个回归测试和4份原生向量/说明。它们共约533KiB，测试代码不是临时产物。

26份`*_test.go`覆盖PVF读取/模式边界、原生包字节、活首领入口、房间路线、二阶段、同房传送、失败重开、预算、恢复等待、图标清空、库存同步、周次数、奖励事务/竞拍后端，以及恢复工具保存兼容性。保留这些测试，防止再次出现空房、重复生成、坏回包或存档损失。竞拍测试证明后端事务，并不表示竞拍UI已接入。

原生fixtures：`internal/legion/testdata/bakal_wire_fixtures.json`、`bakal_refusals.json`、`internal/game/protocol/testdata/raid_weekly_clear_info115.hex`及其来源说明。拒绝记录是原始失败取证，保留为审查依据；不参与运行内容读取。完整Script.inner.pvf等归档不入库，原生集成测试由DFO_PVF_CORE_TEST_ARCHIVE显式提供。

## 共享文件需按改动块审查

巴卡尔还接入以下共享路径，不能只复制清单中的新文件，也不能把整份混合修改都视为巴卡尔改动：

- `cmd/wireprobe/{bootstrap,client_connection,client_dispatch,client_dispatch_world,client_entry,consume_flow,dungeon_flow,dungeon_revive,world_flow,request_scope}.go`。
- `internal/gamedata/{source,catalogs,catalogs_content}.go`及已有原生内容门禁测试。
- `internal/dungeon/{session,completion}.go`；`internal/game/protocol/{dungeon,legion_portal115,unassigned_monster115,use_stackable,player_death}.go`；`internal/workflow/item_consumables.go`。
- `cmd/dfo-tool/main.go`与`cmd/README.md`的bakalreset注册/说明；根“恢复当前账号巴卡尔次数.cmd”。
- `configs/pvf-default.json`的bakal-raid领域/候选路径，及Python启动profile相关测试。

当前共享文件同时有SQLite、维纳斯、装备/存档等其它修改；本轮未覆盖或回滚，合并时需分别审核。本清单不是可直接git add -A的授权。

## 部署与真实阻塞项

当前测试profile引用`bin/wireprobe-bakal-weekly-quota-candidate.exe`，bin已忽略。新检出可按当前profile用`go build -trimpath -o bin/wireprobe-bakal-weekly-quota-candidate.exe ./cmd/wireprobe`构建；或按Build-Server.ps1构建标准handoff-source程序并用`启动游戏.cmd --source-build`。不要为解决本地路径而提交exe、改写玩家库或覆盖39版归档。

`runtime/storage/pgdata/postgresql.conf`在清理开始时已跟踪且有本地修改，最后快照复查后仍被跟踪。后续合并需从提交范围排除此运行目录文件；它不因.gitignore自动变成未跟踪。本轮保留本机文件，不调整已有快照或索引。

全仓Go测试仍有两个既有失败：TestEntrySkillsPreservePayloadAcrossProfessionHashChange、TestEntrySkillsRejectDifferentProfessionReference。保留测试，不能靠删掉用例让CI变绿。此前真实归档Bakal/RaidWeekly/RaidRecovery专项、全仓vet和25项启动检查通过；本轮清理后再次验证专项和CMD帮助入口。

项目要求的`scripts/check-commit-hygiene.ps1`缺失，GIT-MANAGEMENT.md也不存在。本轮不提交；真正提交前仍需补齐卫生检查或由维护者处理流程缺口。

确认范围保持：二阶段及退出勾选窗来自用户确认；NPC库存、周次数同步尚无本轮新实机验收；Final Strike自然结束、通关竞拍UI仍未完成。合并说明应保留这些边界，不能把清理完成写成整个巴卡尔已完成。
