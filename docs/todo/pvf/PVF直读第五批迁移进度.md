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


## 后续奥德赛五项（离线候选）

奥德赛五项直读已完成离线候选，合计35选择项/41类有效源投影。成长源50通关/50准入副本、3赠品及毕业礼盒10420561与毕业主线完整一致；章节7章/50副本/15奖励模板、7行章节掉落、2种货币及85组创建武器选项完整一致。章节2/7禁用、章节概率和货币概率 `[1000,10000,10000,10000]` 保留为独立策略，奖励模板与最终副本由PVF推导。35项缺失所选JSON联合准备、完整源对照、全量Go测试/vet及8项Python测试通过。profile为`pvf-odyssey-candidate.json`，隔离程序SHA256 `9c1b9722733ef93db5f57d11fc25fa415dfd01e5f16c36b6bdea89c97ff26f3a`。确认范围仍为第四批28项；所有旧候选程序/策略保持可回退。

| 选择项 | 源字段与核对 |
| --- | --- |
| `odyssey-growth` | `contents/2026/aradodyssey/etc/aradodyssey.etc`：50成长/50准入副本、3级别赠品、毕业礼盒10420561、毕业任务计划及6份关联源脚本全部一致 |
| `odyssey-chapters` | `aradodysseyjournal.cos`：7章、50副本、15奖励模板，原顺序和最终副本一致 |
| `odyssey-drop` | 从源章节奖励及共享物品索引推导首个选择箱和最终副本；7行一致，章节2及7继续停用 |
| `odyssey-currency` | 两种币的 `.stk` 全部原始Token、源哈希、Grade/Rarity/StackLimit一致；权重0，不进入普通随机掉落池 |
| `odyssey-weapons` | 10417789脚本的85组类别及装备选项；共享选择箱源解析器，沿用原武器奖励启用开关 |

新增 `pvf-odyssey-policy.json` 保存章节启用/概率、已确认货币概率与按怪物rank选币策略，以及原成长证据目录的额外2个物品ID（10418028、10418036）。赠品、毕业物品、章节最终副本及选择箱模板不保存在策略中；所有脚本路径/哈希/属性从PVF读取。货币rank选币是现有服务器策略，未据此宣称源官方掉率已恢复。原 `pvf-content-policy.json` 没有加入新字段，因此旧30项程序的严格策略解析仍可使用。

新的35项profile为 `server/work/dfo-lan/configs/pvf-odyssey-candidate.json`，对应程序 `.tmp/pvf-odyssey/bin/wireprobe-handoff-source.exe`；旧30项profile和程序仍保留。35项源装配耗时约49.94秒；完整奥德赛对照约15.63秒。两个离线导出器共享运行源解析。只读启动依赖检查通过，未启动客户端、服务或修改玩家数据库。本批尚未实机确认，剩余特殊奖励、副本覆盖及其它目录继续迁移。


## 无色小晶块源覆盖候选

新增clear-cube选择项，3037无色小晶块从共享PVF索引及原始脚本直读，完整源Token/哈希对照一致。保留原存储覆盖中Grade/Rarity/Weight为0的最小投影，原源值仍在ScriptRecord中；不进入普通掉落池，不改变分解或技能消耗公式。总计36选择项/42类有效源投影，缺失所有所选导出JSON联合准备约42.10秒通过，混用存储来源拒绝。全量Go测试/vet、8项Python测试及只读依赖检查通过。新profile为`pvf-cube-candidate.json`，隔离程序SHA256 `c6e29f3cc4c8b375ee9ecbc7781553134b005c6b1d03cc94980b83ffddd1261b`；第五批尚未实机，确认范围仍为第四批28项。

新增选择项`clear-cube`只替换原`clear-cube-source.json`的来源。源脚本`stackable/material/cubepiece_clear.stk`中Grade=5/Rarity=1完整保留；现有业务投影仍为0/0，不因迁移将其纳入随机候选。源哈希仍由原覆盖验证器强制校验。新程序位于`.tmp/pvf-cube/bin/wireprobe-handoff-source.exe`，所有原profile/程序/策略文件保持。特殊奖励与其它剩余目录继续实施。
