# PVF直读迁移清单

更新：2026-10-01。当前已启用、能定位PVF真源的数据已完成候选接线：网关54个选择项、63类源投影，以及admin、initialrepair、questrepair和GM查询/发放。用户已确认全量PVF并要求默认启动，54项/63投影升级confirmed baseline；源目录核对、缺失导出JSON准备及离线检查通过，确认依据用户反馈。

游戏源固定为`server/work/client-build/Script.inner.pvf`，SHA256 `7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80`。已有存档继续使用同一来源，不建立checksum别名，不改写角色/任务/物品存档或数据库结构。仓库当前客户端inner `be95d64e…`是另一资源版本，其对接不属于本次同源读取迁移。

## 已迁移的运行数据

| 数据范围 | 原生选择项 / 入口 | 验证范围 |
| --- | --- | --- |
| 职业、世界、任务、经验、技能、起始路线 | characters、world、quests、progression、skills、tutorial | 17职业、694区域、236移动、2844任务、150经验阈值、3224技能、16起始路线；默认技能栏/命令策略独立 |
| 物品索引、完整装备、普通/任务装备范围 | items、equipment、equipment-selection | 599771个LIST绑定、424216完整装备定义、3174选择行和2794掉落池；基础1536个ID为原服务端选取策略 |
| 时限、外观、图鉴、生成成本 | periods、skins、journal、create-cost | 124610期限模板、1733外观、5图鉴分类、9生成成本组；原拒绝/执行边界保持 |
| 价格、材料、商店、现金商城 | prices、materials、item-shops、cashshop | 599682价格、14211材料、527商店/7025商品、17245商城记录/16606实际商品；SHP路由与发布开关独立 |
| 礼盒、Booster、自选箱、抽奖 | boxes、boosters、selection-boxes、lottery | 两个COS材料绑定/54奖励、42504Booster、自选2975/固定2/未解析1、276材料金币池/2477装备池；未解析项不扩大发货 |
| 强化、增幅、附魔、随机词条、盾牌、誓约、金库 | enhancements、random-options、shields、oath-grades、vault | 源券/费用/能力/分组/盾牌窗口/189档位/账号金库60级及40档费用；成功率、槽映射、容量与诊断开关独立 |
| 普通掉落、旁路晶块 | loot、clear-cube | 1022普通物品、1221组、281副本索引、3037源定义；金币/物品兼容公式保持 |
| 城镇、副本、教程、训练、双塔、深渊、迷宫 | town、dungeons、tutorial-dungeons、training-dungeons、dungeon-towers、dungeon-hell、dungeon-maze | 3200副本/18387地图、15教程/4训练场、两塔与55深渊地图、源概率；出生点和已确认概率覆盖独立 |
| 终场、武斗大会、脚本传送、层回访 | dungeon-terminal、dungeon-tournament、script-warps、layer-revisits | 7终场、2武斗图、12脚本路线、1层回访；18字节实机协议记录和缓存恢复策略独立 |
| 天启、调律、黑鸦、铁矿 | apocalypse、attunement、black-purgatory、bleeding-mine | 源CTP/COS、奖励图、装备范围与全部typed有效字段；本服概率、准入及原奖励拒绝规则独立 |
| 奥德赛 | odyssey-growth、odyssey-chapters、odyssey-weapons、odyssey-drop、odyssey-currency、odyssey-routes | 50副本、7章节、85组武器、7行掉落、2币、日志回城引用；已停用章节/概率/rank选币独立 |
| 冒险团、推荐、迷雾、背景券、名望 | adventure、adventure-recommended、season、roster-backgrounds、fame | 原嵌入源目录与派生运行索引直读；等级/商店/推荐/赛季/背景/装备名望源字段核对，外部公式和开关独立 |
| 管理与修复 | admin、initialrepair、questrepair | 共用gamedata/managementdata；check-catalogs在任何存储读取前退出 |
| GM查询和发放 | cmd/gmtool、gmweb.py、代理catalog-metadata | 全部599771个源脚本读取，旧386230个ID、kind/grade/rarity零差异；源部位、等级、实际名称引用直读，代理不需要重复目录JSON |

8种已启用嵌入源数据已接入原生候选。旧JSON文件和默认兼容读取分支继续用于回退、历史向量及审计；它们的存在不表示全量原生候选仍以这些导出物启动。

## 保留项与证据边界

| 保留内容 | 保留原因与约束 |
| --- | --- |
| 运行地址、端口、存储连接、输出目录、探针响应/原生向量、账号选项模板 | 运维或客户端协议数据；不属于PVF源表 |
| 建号/出生点/每日疲劳/容器与穿戴布局、默认快捷栏、功能开关、范围白名单 | 现有服务端或客户端布局策略；源属性、物品stack limit、奖励池和价格已从策略中分离 |
| experience/drop/cards兼容计算、强化/增幅概率及失败规则、本服特殊奖励概率 | 已有外部公式或服主规则；不以迁移更换玩法计算 |
| refine锻造公式 | 现有规则继续保留；仅同名表检索未命中不能证明PVF绝对没有该公式。新源规则须补充取证后再实施，不能猜表替换 |
| 527个服务端商店路由 | 202同ID原生绑定、30明确原生ID兼容映射、295既有明确路径。商品/材料/价格来自SHP；295条路径未重新确认为当前客户端原生开店路由，不能在本次迁移中改写ID |
| GM中文译名与固定界面分类标签 | 当前仓库客户端uv多数英文、translate/kor主要韩文，不能等价还原旧中文表；仅作显示覆盖，不能提供物品身份、属性、槽位或存档source |
| GM旧部位缓存与ID拼接名称 | 候选不再使用该缓存。旧392575条部位记录中6561无LIST绑定、12为堆叠物，416部位/244403等级字段不同；663处名称随实际引用修正。这些是明确的查询显示差异；本轮默认切换不更改GM启动入口 |
| 未启用军团/skycastle_scene_routes与GM set/avatar/whitelist资产 | 当前生产入口不消费这些JSON；现成源导入器或资产存在不等于启用功能，本次不新增玩法/历史管理功能 |
| JSONB存档、事务回执、操作备份、测试样本 | 玩家状态或验证证据，不能当成PVF导出规则删除 |
| 离线导出/对照命令与历史数据库验收工具 | 可继续读取旧JSON以复现历史向量或比较输出；全量PVF运行不调用它们。charactercheck等普通数据库命令本次未执行，不声称旧集成检查问题已解决 |

## 候选与确认范围

网关全量profile为`server/work/dfo-lan/configs/pvf-all-candidate.json`，程序在`.tmp/pvf-all/bin/wireprobe-handoff-source.exe`。全部54项开启，`DFO_PVF_VERIFY_BASELINES=0`；源SHA256与策略验证仍强制执行。该profile不改变用户既有玩法开关或客户端资源配置，商城发布模式沿用之前候选值1。

最终全量程序SHA256为`a3ea388ac9a2966f0368e6ede552f3d8559fc10bfba08f24f5158bb583e2d98c`。实际将26个导出JSON参数、存储配置和输出目录均设为不存在，53.26秒完成源准备，报告54项、storage_accessed=false、runtime_started=false，指定输出目录未创建。全量Go测试/vet和20项Python检查通过；时间仅为本次离线准备数据。

之前确认的第四批28选择项/34类源数据保留回退：`pvf-migration-candidate.json`，程序SHA256 `5475dbccdf316f4c582cc2742b22e1f66b5512f22e04997ccb23a031f6e36609`。第五批现已依据用户确认升级默认入口，使用pvf-default.json及bin/wireprobe-pvf.exe。所有原确认程序及逐批候选仍保留。

管理候选程序位于`.tmp/pvf-management/bin/`；admin/initialrepair/questrepair使用`-catalog-source pvf -pvf-archive <inner路径> -pvf-source-checksum <精确SHA256> -check-catalogs`。GM启动准备脚本支持对应参数及`--gmweb-binary`；`--check`只运行候选目录检查，退出前不加载存储。

## 无数据库检查

以下在`server/work/dfo-lan/`执行。依赖检查只查看路径/端口，不启动环境：

```powershell
../../../tools/python/python.exe scripts/launch_local.py --repair-profile configs/pvf-all-candidate.json --check --server-only
```

完整源准备可直接使用网关入口；此例读取独立穿戴策略，JSON基线审计关闭，退出前不安装运行全局、创建捕获目录、监听端口或访问存储：

```powershell
$profile = Get-Content -LiteralPath configs/pvf-all-candidate.json -Raw | ConvertFrom-Json
foreach ($entry in $profile.environment.PSObject.Properties) {
    Set-Item -LiteralPath ('Env:' + $entry.Name) -Value $entry.Value
}
./.tmp/pvf-all/bin/wireprobe-handoff-source.exe -pvf-check-catalogs -equipment-wear-rules configs/equipment-wear.current35.json
```

上述环境设置仅作用于当前PowerShell进程，需在独立检查终端执行。正式候选启动仍使用现有launch_local及用户既有客户端/存储配置；只读目录准备不代替用户手动登录、穿戴、技能、副本、奖励、商店、存档重选和GM筛选验收。详细逐域源哈希与差异见《PVF直读第五批迁移进度》及实施计划。

## 2026-10-01：全量PVF已确认并作为默认启动

用户确认“已确认，将pvf模式作为默认启动项”，本批54选择项/63类源投影升级为confirmed baseline。默认profile为server/work/dfo-lan/configs/pvf-default.json，正式程序为server/work/dfo-lan/bin/wireprobe-pvf.exe，SHA256 a3ea388ac9a2966f0368e6ede552f3d8559fc10bfba08f24f5158bb583e2d98c，与已确认的全量隔离程序逐字节相同。来源仍为server/work/client-build/Script.inner.pvf及精确7ef SHA256，不别名、不改写玩家存档。确认依据用户反馈，本轮未新增实机会话日志，不扩大为逐项客户端动态命中。

启动服务端.cmd、启动游戏.cmd、启动游戏-奥德赛.cmd通过共用launch_local默认加载全量PVF配置；分别保留仅服务端、剧情模式0及奥德赛模式1。默认JSON基线审计关闭，源身份/策略校验仍强制。独立profile显式保持既有奥德赛武器奖励发布值1，使挂载不依赖旧武器盒导出JSON是否存在。客户端路径、channel identity、存储配置及其它玩法开关保持；客户端/存储单独模式不要求本地PVF。

编排按选中的characters/dungeons领域跳过旧JSON告警或副本文件门禁，其余JSON模式/未选领域继续原检查。PVF选中而程序不自报pvf-catalogs能力时拒绝启动，不能静默退回JSON。外层等待从30秒调整为PVF 210秒，覆盖内层180秒源准备；JSON等待仍30秒。停止环境识别wireprobe-pvf.exe。Build-Server首次构建补齐默认PVF程序，已有确认程序在普通构建时保留；-UpdatePVFDefault显式发布后续已验收构建。

显式--repair-profile仍可选择逐批隔离/修复配置；--json-mode使用launcher.local.json原server_binary并清除继承的DFO_PVF_*，与repair-profile互斥。--source-build在默认PVF模式下改用源码程序并保留源配置，需要重建有全部选择项的版本；--source-build --json-mode为旧JSON源码路径。旧39程序、原源码程序、各批隔离程序和JSON文件均保留。GM启动入口仍保持独立选择，不在本次三个入口切换范围。

全量go test ./...及go vet ./...通过；启动/profile/inner-PVF/channel identity共25项Python检查与5项GM检查通过。模拟就绪在40秒到达仍成功，缺失旧JSON与不支持PVF程序拒绝/跳过边界均验证。默认服务端、剧情、奥德赛及显式JSON四组实际--check均通过，均不启动服务或客户端、不操作玩家数据库。
