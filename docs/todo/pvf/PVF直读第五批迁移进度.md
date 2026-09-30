# PVF直读第五批：天启与调律源表候选（2026-10-01）

第四批28个选择项/34类源数据已由用户确认正常。confirmed baseline 为 `pvf-migration-candidate.json`，程序SHA256 `5475dbccdf316f4c582cc2742b22e1f66b5512f22e04997ccb23a031f6e36609`。本批新增两项仅为离线候选，继续实施其余迁移。

| 选择项 | PVF来源与完整核对 |
| --- | --- |
| `apocalypse` | `contents/2026/apocalypse/etc/apocalypse.ctp` 和 `dungeonskillinfo.ctp`，主表65记录、4操作、6阶段共2190秒、职责14记录；原始字段、奖励位置、源哈希与运行投影一致 |
| `attunement` | 通过 `etc/rewardboostinfo/**/*.ctp` 的 `[dungeon index]` 源字段定位4个已启用副本，126奖励模板、5优惠券行；源路径/哈希/概率/隐藏原始字段完整一致 |

调律副本选择 `[100005066,100005067,100005068,100005014]` 置于 `pvf-content-policy.json`。策略不记录脚本路径、源概率或奖励池；文件绑定由PVF声明决定，重复声明拒绝加载。使用无整库切片复制的归档遍历，避免复制全部约565万条记录。现有调参、隐藏表中间字段及Omen启用边界保持；每次运行装载深复制奖励表，调参不改变准备好的源目录。天启时钟与运行操作继续使用原验证器。两个离线导出命令也复用同一源解析器。

源仍为 `server/work/client-build/Script.inner.pvf`，SHA256 `7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80`。角色存档来源门禁保持，未修改协议、客户端资源、数据库结构或玩家存档。

联合30选择项/36类源数据测试将全部所选导出JSON路径设为不存在，准备成功，约47.38秒；两个CTP完整对照及调参副本隔离检查通过。全量Go测试、vet和7项Python profile测试通过。时间是离线准备测试结果，不代表实机启动或内存上限。

独立profile：`server/work/dfo-lan/configs/pvf-content-candidate.json`。

隔离程序：`server/work/dfo-lan/.tmp/pvf-content/bin/wireprobe-handoff-source.exe`。

SHA256：`fcefe7737c198e1425a4d87c03affc7122f7828d1615412835751ed1beafcf2d`。

只读 `launch_local.py --repair-profile configs/pvf-content-candidate.json --check` 通过；未启动客户端、服务或连接玩家数据库。第四批及之前回退程序保持。Odyssey、其它副本覆盖、特殊奖励、嵌入目录、商品绑定、职业源/策略拆分和GM查询继续按实施计划推进。
