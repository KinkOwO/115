# PVF直读九领域：已确认基线（2026-10-01）

状态：用户于2026-10-01确认“经过确认，都是正常的”。本批九领域直读升级为confirmed baseline，沿用显式profile入口；确认不等于所有源JSON已迁移或默认启动已切换。

## 确认记录

用户手动会话为 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261001_031044_836440_next37`。`gateway.out`记录隔离程序 `.tmp/bin/wireprobe-handoff-source.exe` 与日常客户端 `F:\wip\dof\115US`；程序SHA256为 `95b009aa41e830a70aa1fc76e150b186b7caef49a02842b8e9ffd07a5255e44e`，inner源版本为 `7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80`。

`gateway.err`包含全部九领域的启动校验通过记录，准备耗时37.989秒；随后启用124610个期限模板、1733个外观登记模板、424216条完整装备和599771个奖励分类索引。`events.jsonl`包含角色11/19/20的操作：30条装备槽刷新、15条装备外观刷新、15次名望计算、9次名望同步、3次装备图鉴恢复、2次任务库存提交、3次任务完成及2次副本结算奖励；副本3与5均有入场/房间推进记录。事件计数是服务端记录数，不等同于独立人工操作次数或全功能覆盖数。`client.log`记录 `SUMMARY exit=0x0 normal_run=1`，客户端正常退出。

确认依据为用户反馈与上述运行日志。未迁移的技能/职业投影、普通装备选择目录、商店/礼盒/材料、其它副本源表和嵌入JSON仍按实施计划推进；当前客户端inner跨源兼容和JSON启动门禁移除不在本次确认范围内。未修改数据库结构、存档版本或客户端资源，日常启动方式保持现状。

## 本次范围

| 候选领域 | 已验证内容 |
| --- | --- |
| world / quests / progression | 694区域、236NPC移动、2844任务、150经验阈值；世界62个新增阶段图单独核验 |
| items | 599771个模板；列表ID、路径、分类、堆叠类型/上限完全一致 |
| equipment | 424216个全量装备定义按ID从PVF读取；包括全部属性与名望段逐项完全一致 |
| periods / skins | 124610个期限模板、1733个外观登记模板；127个源缺失skin维持不可登记 |
| journal / create-cost | 5个图鉴分类、9组装备生成成本完整一致 |

本次 `equipment` 指全量穿戴/属性定义，普通和任务的3174行装备选择目录仍读JSON。源物品索引已供给背包补充、箱子结果分类及商城分类；Booster内容表、商店价格/材料、普通掉落、副本/技能和其它嵌入源表尚未迁移。过渡期仍使用导出JSON做启动比对，业务运行复用PVF结果；这不是最终无JSON启动模式。

## 启动

关闭当前游戏会话后，在项目根目录的PowerShell中运行：

```powershell
# 先检查依赖；此命令不启动服务端、客户端或数据库。
./tools/python/python.exe -B ./server/work/dfo-lan/scripts/launch_local.py `
  --repair-profile ./server/work/dfo-lan/configs/pvf-direct-candidate.json --check

# 由用户手动执行原游戏入口，保留其中的场景模式与玩法环境设置。
./启动游戏.cmd --repair-profile ./server/work/dfo-lan/configs/pvf-direct-candidate.json
```

profile选择九个候选领域与 `.tmp/bin/wireprobe-handoff-source.exe` 隔离程序，数据归档为 `server/work/client-build/Script.inner.pvf`，完整SHA256为 `7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80`。这与现有导出规则及存档版本相同；当前客户端inner `be95d64e…` 尚未建立跨源迁移，不应手动改成它或设置来源别名。

已确认隔离程序SHA256为 `95b009aa41e830a70aa1fc76e150b186b7caef49a02842b8e9ffd07a5255e44e`。完整Go测试/vet、Python准备/profile测试及九领域联合准备通过；本次用户实机确认记录见上文。

本机 `launcher.local.json` 的实际客户端为 `F:\wip\dof\115US`，依赖检查通过。该目录的 `Script.pvf` / `sk.dat` 与仓库客户端逐文件SHA256一致；两份EXE的版本均为2.38.2.34，但实际EXE为 `5543c382287bfd5354c3d8c32fd0ce2f1bac572adcc57563c29e6091e332e1cb`，仓库权威EXE为 `1d3948784e5e0f77ed744017bf82bf0c50de9421423f9e59f6f70aed609d8ffa`。只读字节比较发现 `.text` 内4字节差异，文件偏移 `0x7220f48..0x7220f4a` 和 `0x7220f4d`；本批不解释或变更这些既有差异。profile沿用现有日常客户端，实机结果必须注明这一运行身份，不能扩大为仓库原版EXE或新协议已经验收。

若当前环境设置了 `DFO_NPC_PRESENCE_WORLD` 的JSON诊断旁路，PVF世界门禁会明确拒绝。测试时应在当前终端清除此诊断覆盖；不要删除配置文件。

```powershell
Remove-Item Env:DFO_NPC_PRESENCE_WORLD -ErrorAction SilentlyContinue
```

九领域准备约45秒；候选等待ready上限180秒。独立测试GC后heap约1.47GiB，完整游戏服务的峰值和稳定内存还需观察。若来源、字段或资源缺失，网关会在打开玩家数据库前退出，并写明原因。

## 用户手动回归

1. 登录已有角色，检查背包、金币、材料及任务状态；移动城镇、NPC传送，重新选角后再次进入。
2. 检查已有装备和时装显示，换装、脱下再穿回，比较名望与装备耐久；覆盖普通装备、115级装备、宠物/称号等已拥有类型。
3. 使用已有堆叠物，打开已有箱子或领取已有任务奖励，检查结果仍进入正确容器、数量和堆叠上限正常。优先沿用日常操作，不为验证新增奖励模板或修改存档。
4. 查看已有外观登记/选择、装备图鉴与生成成本界面；有原本准备使用的外观道具时检查登记与重登恢复。生成会消耗原有成本，只在本来要执行该操作时验证。
5. 完成一次熟悉的副本，检查掉落、拾取及任务/经验结算；重登确认已有状态保存正确。

复验时请记录操作结果及角色/时间；日志保存在 `server/work/dfo-lan/runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_*_next37/`。服务端 `gateway.err` 应先出现九领域的 `PVF candidate ... verified` 与准备耗时，再出现正常目录装载和业务日志。需据此确认实际运行PVF直读，不能只以程序能启动作为验收。启动编排自身的错误另见`helper.err`。

## 回退

关闭候选会话后，重新使用原 `启动游戏.cmd`，不传 `--repair-profile` 即回到日常程序与JSON模式。profile环境只作用于这次子进程；没有修改日常程序、launcher.local.json或源归档，不需要回滚玩家数据库。若用户另外手动设置了 `DFO_PVF_*` 环境变量，回退时应清除这些候选变量。

已确认程序位于服务端 `.tmp/`，不进入Git；可按需重新编译。2026-10-01已更新CHANGELOG和confirmed baseline并按本任务范围收口；后续迁移顺序与完整未迁移清单见 [实施计划](PVF直读实施计划.md)。
