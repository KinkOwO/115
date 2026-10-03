# 副本经验与难度接线（2026-10-03，源码候选）

状态：已确认。用户手动确认副本不同难度获得的经验不同；confirmed baseline 更新为本任务独立候选。实机确认范围只记为本次用户所测路径，不推定所有地图、等级段和模式均已实测。

## 问题与证据

`ProgressionService.Monster` 和 `ClearWithTowerRewards` 原来分别调用
`GrowthMonsterGain(..., 0)`、`GrowthDungeonClear(..., 0, rank)`。
副本会话已保留实际 `Difficulty`，但两处始终使用经验表首列。

当前内层 PVF SHA256 为 `4d8c0c82192eb72806d4bec803f5ab638e91e24401d38110cb78fcb09f69e37f`。
直接读取 `etc/(r)serverparameter.etc`，原始条目 SHA256 为
`dabfa8887674c92d418e8ead8f538b550fac7e8865e077a7ad18f21a211f10ab`，
`[dungeon difficulty exp bonusrate]` 依次为 `1.30 2.00 2.50 3.00 4.00`。
此次导出只作取证，不作为运行输入；运行仍由 native PVF progression 导入。

权威 IDB 复制到本任务工作目录后由 IDA 分析：标签 `0x14B3018B0` 在
`0x1478128D6` 引用，所属读取器 `0x147812210`，分支在对象字节偏移1656
顺序读取五个 float32（`sub_14709BDB0`）。未打开或修改权威 IDB。
精简证据见 `analysis/tasks/dungeon-experience-difficulty-20261003.json`；已补入函数索引。

普通 CMD16 的1起算边界见 `internal/dungeon/session.go` 现有 Normal 实机记录；
2026-10-02 副本11又实际报告难度0，见 `analysis/tasks/monster-drop-rate-audit-20261002.md`。
因此本服成长经验沿用普通掉落/翻牌已确认的边界：0和1对应首列，2..5对应后续列。
季节经验使用独立源表，保留其原有原始难度索引。
网络检索仅作版本线索：[DFO难度调整](https://www.dfoneople.com/news/updates/221/System-Upgrade/Dungeon/Dungeon-Difficulty-Related)、
[普通副本更新](https://www.dfoneople.com/news/updates/4890/Content/Normal-and-Special-Dungeon)。
参考端 `ServerS4A21/Server/DfoServer/GameWorld/DungeonExperienceDefinition.cs`
也读取同名表，但其编号和规则不用于替代115证据。

## 实现与存档边界

统一 `growthDifficultyIndex` 将会话难度映射为PVF列索引，随后交给原公式。
击杀、通关基础与基于基础计算的评级经验均受相应系数影响。
系数来自PVF，未新增运行倍率常量、开关或游戏内容JSON。
现有 reference90-solo-v1 兼容公式、等级差惩罚、成长契约、零权重、APC/演出对象排除保持。
这不是115官方私有服务端完整经验公式的确认。

无schema、存档身份、SQL、客户端/DLL或PVF修改。
保留原怪物与通关幂等键，已有已结算事件不会被重新计算或补发经验。
公式辅助函数继续接受0起算列索引；只在事务入口转换一次。

## 验证

- 新事务级测试修复前复现难度2..5全部只获得130，修复后依次获得200/250/300/400。
- 难度0/1各130；通关基础同档，评级10%随基础变化；结算怪物总量与已提交击杀一致。
- 击杀及通关重放不重复支付，背包与未知存档字段保留；难度6/255拒绝且无写入。
- Go 1.26.0 `go test ./...`、`go vet ./...`及最终新增边界专项通过。
- 独立候选的当前PVF完整54域准备通过，报告 `runtime_started=false`、
  `storage_accessed=false`；按启动编排补齐 `-equipment-wear-rules configs/equipment-wear.current35.json`。
- `go run ./cmd/charactercheck` 在临时schema因 `account_unified_options` 缺表失败；
  HEAD原代码overlay复现相同错误，未扩大本次修改为既有检查程序修复。

## Confirmed baseline

独立候选 `.tmp/difficulty-exp-20261003/wireprobe-difficulty-experience.exe`，
SHA256 `6cce9f0462d25093fabec8c932a43d4e21bf4f90ee2b25b6d9741dfb9bcc6e0f`。
profile及 `启动验证.cmd` 位于同目录，继承默认PVF profile的完整环境。
准备检查使用本任务缓存目录，避免影响日常派生缓存。
用户确认不同难度经验不同；根启动入口修复为通过 `%~dp0profile.json` 绝对路径加载候选profile。
候选、profile 与启动入口在独立 `.tmp/difficulty-exp-20261003/` 目录。默认程序未替换，
此次变更无schema、数据库、存档格式或客户端资源改动。
