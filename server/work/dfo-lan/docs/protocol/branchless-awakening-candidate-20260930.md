# 无转职职业觉醒修复候选（2026-09-30）

状态：attempt 1/3，已完成源码、配置及存储验证，待用户手动实机验收；未升级 confirmed baseline。

2026-09-30 用户暂无实机测试条件，授权先提交并推送本次实现。代码与PG验证结果保持，实机验收延期，候选状态及confirmed baseline不变。

## 问题与证据

09-26交接包记录了黑暗武士adv=0发送CMD2177 stage1被拒的实机证据。本机当前树只具备导出器和学习层修复，`WireAdvancement`、`ApplyAwakening`仍拒adv=0，实际加载配置job9/10的`awakening_skills`仍为空。交接包是另一工作目录的候选，不能视为本机部署记录。

本轮从本机`server/work/client-build/Script.inner.pvf`只读提取两个角色条目及typed tokens：

- 归档SHA-256：`7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80`，与生效角色及技能目录source checksum一致。
- `character/swordman/dsswordman.chr`原始条目SHA-256：`9db9a315ccbef3a0f0804eafdab52bd614bc7296e3fe717c5422c7d7f311a34d`。
- `character/mage/creatormage.chr`原始条目SHA-256：`87295166915198758ebdc175f51e3b9d9afb67728b2b6620c7023348b61ba2fb`。
- 两条目仅有`[growtype 1]`，`[max grow count] 1`，授权位于该段的`[awakening 1..3]`。wire编码沿用既有客户端取证：低4位转职分支、高位觉醒阶段，不改协议布局。

| 职业 | 一觉源授予 | 二觉源授予 | 三觉源授予 |
| --- | --- | --- | --- |
| job9 黑暗武士 | 263×1 | 256×1、255×1 | 269×1 |
| job10 缔造者 | 273×1、268×1 | 274×1、261×1、262×1 | 407×1 |

提取文件位于模块`.tmp/awakening-source-20260930/`，未纳入Git。授予由typed tokens解析，不按技能矩阵擅自扩充免费技能。

## 实现与兼容

- `WireAdvancement`允许adv=0与stage1/2/3，分别编码`0x10/0x20/0x30`，保留adv和stage范围检查。
- `ApplyAwakening`要求职业无`AdvancementGrowth`且具有`AwakeningSkills[0]`，普通未转职职业继续拒绝。等级50/75/100、顺序、源版本和授权有效性校验保留。
- 定点补齐`characters.skycastle-release.json`的job9/10授权表。启动器实际加载该文件；没有整体重导配置，source checksum、raw hash、其它职业及组合槽位均保持。
- 黑暗武士二觉源授予255的前置为81×1，该前置可能尚未购买。旧`Learn`遍历全体known技能的前置链，导致此后任何加点都被拒。当前仅拒绝本次变更技能自身前置不足，以及本次降级前置导致的依赖不足；历史缺口不阻塞无关操作。没有替玩家购买前置或增加SP，退款仍不得低于源授予floor。
- 沿用既有角色事件事务、觉醒保存字段和三觉VP初始化。无schema变更，无玩家存档批量修改，无客户端/PVF/sk.dat/DLL改动。

## 验证

- 接入前，新回归测试复现wire误拒及运行配置空授权；前置修复前，真实目录中的job9二觉255→81缺口复现无关64技能学习被拒。
- 接入后，实际目录job9/10一至三觉逐阶段授予、重复幂等、入场基础/技能/附加信息生成、未知存档字段及三觉VP保留通过。
- 普通职业adv=0、缺授权、跳阶、等级不足、源不匹配拒绝；新学习前置和退款依赖保护测试通过。
- `go test ./...`及`go vet ./...`通过，使用完整Go1.26工具链。仓库`tools/go`缺少标准库源码，验证使用`D:/ProgramFiles/Go/bin/go.exe`及模块`.tmp/awakening-review-go-cache`。
- `AWAKENING_INTEGRATION=1 go test ./internal/character -run TestBranchlessAwakeningAndLearningPersistence -count=1 -v`通过：真实PG独立临时schema覆盖两个职业一至三觉落库，黑暗武士二觉后购买64、请求重放不重复扣SP、重读角色和入场投影、未知字段与config_version保留。测试结束删除该schema；未写玩家schema。

## 部署及回退

确认客户端及所有wireprobe网关均未运行后，已备份旧exe并将当前Go源码构建部署至默认启动的`bin/wireprobe-handoff-source.exe`；没有自动启动服务或客户端。

- 候选SHA-256：`68348C816BE876C70BD3E2E1C11C9FCCABFD4F47B6FC4EB3715CF0361663D925`，24,763,904字节。
- 改前exe SHA-256：`A9EC4F0FE15AE2668BC0269B849C1D8D611F6EA3444B21257ACDE0741E8A97C6`。
- 模块`.tmp/awakening-deploy-20260930/`保留`wireprobe-handoff-source.before.exe`、`characters.skycastle-release.before.json`、独立候选exe及`deployment.json`；复制前后均核对SHA-256。
- `server/launcher.local.json`仍使用原本的`work/dfo-lan/bin/wireprobe-handoff-source.exe`，下次用户手动启动会加载新候选。

改前配置备份：模块`.tmp/awakening-deploy-20260930/characters.skycastle-release.before.json`。回退候选时先关闭游戏及网关，再恢复本轮旧exe和配置；不要回退其它功能的配置或覆盖玩家存档。成功觉醒形成的adv=0/stage>0存档旧exe会拒绝，因此成功验收后若需回退，必须另行评估存档兼容，不直接降级旧exe。

## 用户手动验收

1. 使用根`启动游戏.cmd`手动启动，登录未觉醒黑暗武士，依次完成一觉、二觉、三觉；等级分别须满足50/75/100。
2. 检查觉醒弹窗结束、对应源授予出现；二觉后可普通加点，三觉VP初始化正常。
3. 重选角色或重新登录，检查阶段及技能保留，同时确认已有组合技能栏排列不受影响。
4. 有缔造者角色时同样验证；没有时只标代码及PG覆盖，不标缔造者实机通过。

日志核对：`awakening_completed`和技能/角色刷新，检查是否还有`awakening_refused`、`skill_refused`；若出现CMD1881或技能栏不更新，再取当前用户手动日志和原生消费路径，不叠加无依据包。可选择觉醒技能索引字段语义仍未单独取证。沿用09-26既有attempt 1/3方案，本轮未新增布局/codec假设；用户确认前保持候选状态。
