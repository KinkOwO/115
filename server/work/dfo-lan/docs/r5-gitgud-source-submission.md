# 正式累计 R5 源码与上游集成评审说明

本分支保存正式发布 `2026.10.11-cumulative-r5` 的累计源码，并针对 MR !206 合并上游 main。GUI 版本 2.0.5.0。集成后的服务端经过源码验证，尚未替换已确认的 R5 发布程序，也未完成组合实机验收；MR 保持 Draft。

## 范围与来源

R5 冻结源码提交为 `48b8bbe61046908d3fe38cfe872841fa685fc17c`，原来源上游基线为 `9c99e6510ed5cb882f1603baca70976b4dd9eb15`。本轮合并目标为 `2e7694108c31d19a6fa4625d8c68e7d2e016d275`，提交时还需再次 fetch 核对上游是否变化。使用合并提交保留两侧历史，不 rebase、不强制推送。

累计功能包含 R3 的角色券扩容、精锐全副本、莉尔文 NPC、在线/十日/拾光活动及原有 R2 功能；保留 R4 称号簿 C2423 准备后恢复、签到路径和角色券 cwd 修正。R5 保留武器幻化道具/模具、皮肤登记、背包/邮件/账号金库事务及选角/城镇/副本同步，动画插件版本白名单和 16 GiB 客户端进程组内存配置；服务端上限仍为 4 GiB。

`launcher/` 保存 R5 GUI 和内嵌 GM 源码。R5 `weapon-skin-animation-r1` 和既有 `local-features` 源码在本轮未改变；上游精锐补丁更新按原文件保留。本轮不安装 DLL、不启动游戏、不修改玩家数据。客户端新版本与全部累计玩法组合的实机兼容仍需后续人工验收。

## 冲突处理

本轮有 99 个冲突路径。按双方意图组合服务端实现，同时把正式 R3 到 R5 的差异叠入；不能以旧 R3 候选覆盖 R5。合并后不再声称 Go 生产逻辑与冻结发布包原字节一致。

- 巴卡尔：保留上游 Native 类型/服务与 R5 既有类型，隔离重名；R5 `AddBakalRaidBoss` 与上游通用 `AddRaidBoss` 分开，准备了哪个源规则就走其现有入口。
- 精锐与军团：保留上游原生精锐频道检查及 R5 普通/扩展副本入口；无输出连接的军团待机状态仍初始化。副本加载同时保留上游无缝重试 buff 约束和 R5 武器幻化/称号准备后恢复。
- 装备点数：保留上游按部位套装取最高档的聚合规则，兼容旧 `AbilityGroups` 数据投影；歧义输入拒绝累计，不再保留两套不同聚合算法。
- 森林：采用上游 N31 → CMD46 → N2565 → CMD2062 的结算/切关时序及困难重试状态，保留 R5 SQLite 奖励事件键和重复发奖防护。困难终局检查使用独立临时数据库，验证七种材料真实入账、原物品保留和重复请求不再次发奖。
- 活动与强化：保留上游签到活动投影及 R5 活动标题/永久日期；按合并后的原生强化动作、正式协议向量和 CLI 完整域集合更新旧断言。
- 主线与存档：保留上游根据 PVF 前置关系补足后续主线等级的策略。旧 charactercheck 把后续奖励链固定为 1200 EXP / level 3，现改为从原生任务 3145 → 4873 的关系及等级阈值计算期望；经验、等级、技能点、并发一次入账、重启读取和下游背包不覆盖检查均保留。
- 配置校验：Python 显式 profile 校验识别 R5 已存在的 `DFO_WEAPON_SKIN_ANIMATION=r1`，仍拒绝其它版本、未知键、重复域；默认 profile 保留上游 transform 和 R5 五种累计活动域。

- 编译期 mod 清单：保留上游新增的 actor/enhancement policy 接口；移除生成文件中对四个仓库未跟踪的本机 mod 包的 import/Register 行。该文件自身明确要求提交干净形态，R5 也不自动安装本机样例；不创建假的 mod 包来满足编译。

## 源码验证

合并后的 Go 1.26 `go build ./...`、`go vet ./...`、`go test ./... -count=1 -json` 已运行；完整测试记录为 5435 pass、394 skip、0 fail。跳过包含需要真实资源或显式环境变量的检查，不能把跳过当作游戏验收。最新上游和角色检查调整后，全量重跑也退出 0。

SQLite 标签下 database、character、inventory、workflow 的兼容测试已运行；最初 portable 强化动作旧断言失败；修正并合入最新上游后四包完整重跑退出 0，共 1462 pass、27 skip、0 fail。这些测试使用临时存档，覆盖迁移、资产保留和事务防重；不会读取玩家 SQLite 文件。

Python `test_repair_profile` 和 `test_restored_activity_profile` 共 19 项通过。历史 Python 游戏启动检查仍依赖已移除的 launch_local.py/stop_environment.py，未用它们证明当前 Go 启动链；wrapper_keys 检查缺少本机 Python pefile 依赖，不列入本轮通过项。

真实 `DFO_PVF_ARCHIVE` 只读使用本机内层归档，执行 `go run ./cmd/dfo-tool charactercheck`，存储始终为 OpenTestFixture 的独立 SQLite 临时库。最新上游整套检查退出 0，角色并发容量、所有权、存档重启、任务链/装备奖励、背包满回滚、掉落翻牌、防重复领奖、装备穿脱及技能存档检查全部通过。首次完整行为检查通过后，仅因隔离目录缺少 runtime 输出目录而退出 1；补建本检出内忽略目录后完整重跑退出 0，没有读取玩家库。

R5 GUI/GM 与武器插件的源代码未发生本轮改动，沿用同一源码的前轮证据：GUI 29 项自检；武器插件编译及 102 组策略向量、原生选择器、四处钩子边界/回滚、宿主未启用或映像不匹配拒绝检查通过。宿主机制检查没有安装进游戏，不代表新上游组合在实机已通过。

原正式 R5 发布另有 4757 pass、319 skip、0 fail、PVF 10 项、五种升级基线备份/安装/重复拒绝/回退记录；冻结源码首次提交为 4753 pass、323 skip、0 fail。两者均不能代替本轮上游集成验证。

## 仍未收敛的 PVF 规则

`internal/legion/forest_rewards.go` 的普通/困难奖励模板及数量仍来自上游既有官方抓包和原服主策略，展示和入库共用该表；涉及森林内容脚本及奖励 `.stk`，完整奖励标签到执行规则的源链仍未闭环。本轮保留兼容，不宣称已完成单一 PVF 真源迁移。

`configs/apocalypse-flip-gear.generated.json` 来源为 `list/equipment.lst` 115 级装备枚举，消费者为启动 bootstrap → `ApocalypseRollFlipGearTiers` → 末世录翻牌；它仍是装备内容快照和上游三档概率策略，尚未改为同一只读 Source 的运行时枚举。删掉会丢失上游既有装备奖励；后续应专门迁移。

## 提交门禁与授权边界

此前冻结 R5 独立分支及 59 项源码例外已获业主“帮我推送吧”授权，作者 Deemo <a641927456@gmail.com>。源码检出没有默认发布 exe 的环境差异按原授权保留，不补 exe 入库。

本轮门禁新增下列 17 项：全部逐项核对与上游原内容一致。按根 AGENTS.md §0.3.1 的二次确认要求，业主 Deemo 于 2026-10-11 明确授权保留清单全部 17 个上游原有文件，原话：“按就保留这17个文件”。本合并提交正文逐项记录路径、条款及此授权；保留原上游内容并推送源码，不视为集成实机验收。

| 路径 | 类别与保留影响 |
| --- | --- |
| `mods/modkit-web/run.cmd` | 环境不匹配：保留上游工具路径：configure-env 依赖仓库内 tools 清单；另三个脚本含父目录 tools、旧安装路径或 PATH/已编译程序等选择。当前隔离检出不具备所有预设路径，直接运行相关分支可能找不到依赖。 |
| `scripts/build-game.ps1` | 环境不匹配：保留上游工具路径：configure-env 依赖仓库内 tools 清单；另三个脚本含父目录 tools、旧安装路径或 PATH/已编译程序等选择。当前隔离检出不具备所有预设路径，直接运行相关分支可能找不到依赖。 |
| `scripts/configure-env.ps1` | 环境不匹配：保留上游工具路径：configure-env 依赖仓库内 tools 清单；另三个脚本含父目录 tools、旧安装路径或 PATH/已编译程序等选择。当前隔离检出不具备所有预设路径，直接运行相关分支可能找不到依赖。 |
| `scripts/start-modkit.ps1` | 环境不匹配：保留上游工具路径：configure-env 依赖仓库内 tools 清单；另三个脚本含父目录 tools、旧安装路径或 PATH/已编译程序等选择。当前隔离检出不具备所有预设路径，直接运行相关分支可能找不到依赖。 |
| `server/work/dfo-lan/configs/apocalypse-flip-gear.generated.json` | 需确认：服务端 JSON / 生成表：上游末世录翻牌消费该装备池（维纳斯使用独立的 venus-flip-gear.generated.json）；源为 list/equipment.lst 的 115 级装备枚举，服主三档概率策略保留。并非已迁移成运行时 PVF 直读。 |
| `server/work/dfo-lan/configs/character-rules.jobs-release.json` | 需确认：历史角色策略 JSON：上游恢复的 max_characters/initial_level/all_jobs_pilot 角色试运行策略。当前源码无直接命名引用；configs/README 的旧删除记录与上游恢复文件不一致。 |
| `server/work/dfo-lan/configs/pvf-all-candidate.json` | 需确认：候选 profile JSON：上游已有显式候选启动档，定义候选 binary、PVF 域集合及 policy 路径；不是新导出玩法整表。依赖候选 exe 和本地归档，不可作为当前源码检出的默认可运行档。 |
| `server/work/dfo-lan/configs/pvf-boxes-candidate.json` | 需确认：候选 profile JSON：上游已有显式候选启动档，定义候选 binary、PVF 域集合及 policy 路径；不是新导出玩法整表。依赖候选 exe 和本地归档，不可作为当前源码检出的默认可运行档。 |
| `server/work/dfo-lan/configs/pvf-cashshop-candidate.json` | 需确认：候选 profile JSON：上游已有显式候选启动档，定义候选 binary、PVF 域集合及 policy 路径；不是新导出玩法整表。依赖候选 exe 和本地归档，不可作为当前源码检出的默认可运行档。 |
| `server/work/dfo-lan/configs/pvf-characters-candidate.json` | 需确认：候选 profile JSON：上游已有显式候选启动档，定义候选 binary、PVF 域集合及 policy 路径；不是新导出玩法整表。依赖候选 exe 和本地归档，不可作为当前源码检出的默认可运行档。 |
| `server/work/dfo-lan/configs/pvf-content-candidate.json` | 需确认：候选 profile JSON：上游已有显式候选启动档，定义候选 binary、PVF 域集合及 policy 路径；不是新导出玩法整表。依赖候选 exe 和本地归档，不可作为当前源码检出的默认可运行档。 |
| `server/work/dfo-lan/configs/pvf-direct-candidate.json` | 需确认：候选 profile JSON：上游已有显式候选启动档，定义候选 binary、PVF 域集合及 policy 路径；不是新导出玩法整表。依赖候选 exe 和本地归档，不可作为当前源码检出的默认可运行档。 |
| `server/work/dfo-lan/configs/pvf-enhancement-candidate.json` | 需确认：候选 profile JSON：上游已有显式候选启动档，定义候选 binary、PVF 域集合及 policy 路径；不是新导出玩法整表。依赖候选 exe 和本地归档，不可作为当前源码检出的默认可运行档。 |
| `server/work/dfo-lan/configs/pvf-item-shops-candidate.json` | 需确认：候选 profile JSON：上游已有显式候选启动档，定义候选 binary、PVF 域集合及 policy 路径；不是新导出玩法整表。依赖候选 exe 和本地归档，不可作为当前源码检出的默认可运行档。 |
| `server/work/dfo-lan/configs/pvf-migration-candidate.json` | 需确认：候选 profile JSON：上游已有显式候选启动档，定义候选 binary、PVF 域集合及 policy 路径；不是新导出玩法整表。依赖候选 exe 和本地归档，不可作为当前源码检出的默认可运行档。 |
| `server/work/dfo-lan/configs/pvf-next-candidate.json` | 需确认：候选 profile JSON：上游已有显式候选启动档，定义候选 binary、PVF 域集合及 policy 路径；不是新导出玩法整表。依赖候选 exe 和本地归档，不可作为当前源码检出的默认可运行档。 |
| `server/work/dfo-lan/configs/pvf-odyssey-candidate.json` | 需确认：候选 profile JSON：上游已有显式候选启动档，定义候选 binary、PVF 域集合及 policy 路径；不是新导出玩法整表。依赖候选 exe 和本地归档，不可作为当前源码检出的默认可运行档。 |

四个脚本在当前隔离检出中引用仓库内 tools，相关分支不能直接运行；建议保留上游原文件，路径兼容另行处理；build-game/start-modkit/run.cmd 另有 PATH、显式 Go 或已编译程序等选择，命中机械检查不表示脚本在所有环境都无法运行。候选 profile 仅供显式候选选择，不替换 R5 默认发布程序、不新增 JSON 内容回退。历史角色试运行策略没有接入默认档。

不新增上传玩家数据库、日志、缓存、生成 exe/DLL、PVF/密钥或完整资源 payload；上游历史已有文件沿合并历史保留。本轮没有启动游戏、覆盖 D:\115us 的程序或配置。
