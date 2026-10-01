# PVF直读第三批：强化、增幅与附魔已确认（2026-10-01）

状态：用户已确认正常并要求提交，第三批升级confirmed baseline。第二批十四领域已确认并提交为`28c866b`，其程序与profile保留。本批通过一个`enhancements`选择项接入六类源数据；合计15个选择项、20类源数据，不能将其称为20个独立开关或全项目无JSON。

## 迁移结果

| 原JSON | PVF真源与核对结果 |
| --- | --- |
| reinforcement-tickets | `.stk`的`[equipment reinforcement ticket]`及全部条件；1196种，券的目标等级和概率直读 |
| amplify-tickets | `.stk`的`[equipment amplify reinforcement ticket]`及全部条件；1629种 |
| amplify-grimoire | `.stk`的`[amplification random value]`；433种，随机数值/权重、路径及期限标记一致 |
| enchant-beads | `[enchant waste]`物品的`[monster card id]`；4846种，保留宝珠到怪物卡的完整映射 |
| reinforcement-gold | `etc/upgrade.etc`；255级材料表、普通/100级费用与稀有度/等级权重、安全强化源表及条件一致 |
| amplify-upgrade | `etc/amplifyupgrade.etc`；255级普通材料/金币表、安全增幅武器/非武器表及条件一致 |

旧普通强化券JSON来源为`566b87b7…`，增幅书为`5681103c…`，另外四类为旧外层`2429b15a…`，不是当前存档inner来源。逐字段原始比较仅有926处差异：PVF普通强化券存在`[expiration date]`，旧JSON完全没有这类字段。直读目录保留这些源日期；既有用券业务使用玩家物品实例`ExpireTime`校验期限，期限模板分类由已确认的`periods`源目录提供，不从这个脚本日期扣券或改写实例。审计只允许补入旧普通券缺少的日期头，已有日期变化及其它字段变化仍拒绝；日志明确报告补入数量。除此以外六类有效类型字段完整一致，未添加通用来源别名或重写存档版本。

两种券仍受现有已验证的等级、装备类别和特殊条件边界约束；来源迁移不开放尚未支持的专用券。增幅书Golden/Pure和宝珠运行索引沿用现有装载逻辑，规则整批校验成功后才发布，导入与比对不修改运行规则或玩家存储。

## 保留的独立策略

新增`server/work/dfo-lan/configs/pvf-enhancement-policy.json`，仅保存现有纯净增幅书白名单、可选材料和容器位置、显示别名、最高执行等级、普通强化/增幅成功率与失败策略、安全强化成功率/阶段补正等。材料脚本路径从共享PVF索引读取，不保留在策略文件。源费用、源保护表、原物品条件与券自带概率不再依赖这六个JSON。

现有锻造`refine.json`继续保留，尚未定位其公式对应的权威源表。增幅连续失败补正等原来未实现的业务也不因迁移而开启。仅作诊断、没有进入原运行类型的原始矩阵列不被宣称为新增业务已实现。

## 离线验证

- 六类有效字段审计与运行激活通过，普通券补入926个原始期限头。强化115级史诗费用仍为非武器+0：147750、武器+0：177300、武器+15：8155800；增幅+9金币为50000。
- 前十四领域加`enhancements`联合准备通过。将全部所选导出JSON路径设为不存在后仍成功，保留角色来源锚点和独立策略。联合准备约34～39秒，未打开数据库；这不是完整服务RSS或启动耗时。
- 全量`go test ./...`、`go vet ./...`及8项Python准备/profile测试通过；来源不符、未知策略源字段、截断矩阵和规则未完整时拒绝加载。
- 新程序与启动依赖检查见下文；已确认程序保留，不覆盖日常exe、不启动客户端或修改玩家数据库。

## 启动与回退

Profile：`server/work/dfo-lan/configs/pvf-enhancement-candidate.json`，源为`server/work/client-build/Script.inner.pvf`，SHA256：

`7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80`

新程序：`server/work/dfo-lan/.tmp/pvf-enhancement/bin/wireprobe-handoff-source.exe`，SHA256：

`9372b04936353219a0aa1c428cf61c4f30ef463e7d282022f20229fc322005db`。

候选设`DFO_PVF_VERIFY_BASELINES=0`，所选20类源数据从PVF准备。其它目录、角色锚点、服务器策略和运维配置仍按原逻辑读取。

关闭已有会话后，由用户在项目根目录执行：

```powershell
./tools/python/python.exe -B ./server/work/dfo-lan/scripts/launch_local.py `
  --repair-profile ./server/work/dfo-lan/configs/pvf-enhancement-candidate.json --check

./启动游戏.cmd --repair-profile ./server/work/dfo-lan/configs/pvf-enhancement-candidate.json
```

回退已确认第二批：关闭候选后使用`pvf-next-candidate.json`。对应程序SHA256仍为`b8668e5ef9e56ed37f3ba313fb9349625aa64e0cf16dd41b67e13192a85f0db9`；不传profile沿用日常默认。候选二进制和临时审计报告位于`.tmp`，不入Git。

## 手动检查

1. 已有角色入城，检查装备、金币、材料、技能和任务，换装及重选角色恢复。
2. 按原本计划使用已支持的普通强化券、增幅券、增幅书和附魔宝珠，检查成功/失败结果、扣除数量、装备属性及重登保存。
3. 按原本计划进行普通金币强化/增幅；如计划使用安全路径，核对武器/非武器的材料与金币成本。概率与失败惩罚沿用原规则。
4. 回归上一批已有的商店、礼盒和普通副本操作，确认原功能继续正常。

`gateway.err`应出现`PVF enhancements prepared`，数量为1196/1629/433/4846/255/255。记录角色、操作时间，再结合`events.jsonl`及客户端日志确认。用户于2026-10-01确认正常并要求提交。第三批15个选择项/20类源数据升级confirmed baseline，程序SHA256继续为`9372b04936353219a0aa1c428cf61c4f30ef463e7d282022f20229fc322005db`，使用`pvf-enhancement-candidate.json`。本次确认依据用户反馈；没有新增会话日志可引用，不扩展为逐职业或全部特殊券验收。

后续继续迁移随机词条、骑士盾牌、普通装备选择/掉落、副本与特殊奖励、嵌入目录和GM源查询。商店同ID多脚本与独立礼盒同名COS的绑定证据仍单独处理；完整清单见[实施计划](PVF直读实施计划.md)。


### 后续候选边界

后续六项已完成源码接线和完整离线核对，仍属候选：随机词条17组、骑士盾牌25面、誓约/引子189件、账号金库40档、掉落1022种及装备选择3174行/2794行掉落池。金库客户端容量及存档来源保留，装备基础白名单1536个ID和掉落排除6013保留为原策略；任务新增装备从PVF任务推导。尚未为这六项部署或确认实机。
