# 服务端工具实际裁减（2026-10-03）

第一轮 `af21aaca` 将62个cmd目录合并为4个入口，仍有59个工具实现。本轮基于该提交在独立worktree删除39个工具、42个Go文件、3,187行代码（工作树原始字节93,138），保留20个工具。删除包含实现和统一CLI注册，不另建归档副本或兼容入口。

## 启动器实际消费链

只读核对相邻 `../115us-dfolauncher` 提交 `4e3bb21` 的源码：

| 证据 | 实际行为 | 对服务端工具的依赖 |
|---|---|---|
| `internal/serverbuild/serverbuild.go:105` | 在服务端模块执行 `go build -trimpath -o … ./cmd/wireprobe` | 只构建游戏网关 |
| `internal/run/run.go:707` | 调用服务端 `scripts/launch_local.py`，后者调用 `channel_probe.py` | 没有调用dfo-tool子命令 |
| `build/pack-gm.ps1:8,17` | 在启动器自己的 `gm` 模块构建 `./cmd/gmtool` | 不读取本仓库GM工具源码 |
| `internal/update/follow.go:292` | 本仓库 `cmd/gmtool` 不在启动器服务端更新范围 | 内嵌GM独立维护 |

没有把启动器自身的 `cmd/launcher`、`cmd/healthcheck` 或 `gm/cmd/gmtool` 误算成本仓库命令。本轮不修改启动器仓库；本仓库 `admin/gmtool` 仍服务独立GM工作流，不因内嵌GM独立就删除。Go生产依赖核对：`wireprobe/admin/gmtool` 的依赖集合不包含任何 `internal/toolcmd` 包。

## 删除依据与范围

扫描两仓库当前执行源码/构建脚本，以及本仓库受版本管理的脚本、源码和文档；排除统一注册表自身、交付清单和工具自引用。文档与源码注释中的历史产物来源按来源说明处理，不把它们视为可执行调用。没有发现以下工具的当前外部执行调用。

| 类别 | 删除工具 | 原用途及退休原因 |
|---|---|---|
| 旧内容JSON/索引导出 | `apocalypseimport`、`attunementimport`、`boosterexport`、`catalogimport`、`equipmentfullimport`、`equipmentjournalimport`、`equipmentwearimport`、`hellpartyimport`、`itemperiodimport`、`itemshopimport`、`legionimport`、`lootimport`、`lotterycatalog`、`npcteleportimport`、`oathgradeimport`、`odysseychapterimport`、`progressionimport`、`questcatalog`、`questequipmentimport`、`randomoptionimport`、`selectionboximport`、`shopimport`、`shopprices`、`skinstorageimport`、`towerdazzlementimport`、`towergriefimport`、`tutorialimport`、`worldcatalog` | 主要为旧configs导出或历史对照文件生成入口；运行内容已由 `gamedata.Source/PrepareCatalogs` 原生准备，默认启动不执行这些生成器。保留现有reader、领域模型和历史测试输入，不重建另一套运行JSON |
| 固定输入/专题调查 | `avatarrestorecheck`、`dump342`、`dump781x`、`odysseyaudit`、`odysseygrowthaudit`、`questaudit` | 历史pilot库、固定会话、特定Opcode/奥德赛调查；通用脚本取证用pvfinspect，帧解密用framedump。既有生成的确认向量与取证记录保留 |
| 重复或无当前工作流 | `equipmentaudit`、`pvflist`、`pvfpatch`、`shopaudit`、`towncatalog` | 旧装备字段导出有equipmentfull/equipfields；文件查找/列表及脚本单项导出用pvfinspect；商城字段调查用同一只读脚本工具；城镇点位用townprobe。PVF写副本工具没有当前脚本调用，删除入口而不修改资源，也不声称只读工具可替代其写入功能 |

三个退休导出器自带的测试文件同步删除：`equipmentwearimport/main_test.go`、`questequipmentimport/main_test.go`、`shopimport/main_test.go`，仅覆盖对应CLI参数/JSON写入/文件原子替换。物品、装备选集、商城、技能、领域规则及存档测试不删除或迁为假通过。

源码仍有历史名称的来源注释，例如weekly difficulty向量的 `dump781x`、旧商城导出与奥德赛章节JSON来源。这些只是既有证据/历史输入的出处，没有导入或执行上述包。旧协议文档记录不批量重写，旧命令不再作为当前操作教程；如需审计已退休实现，可从 `af21aaca:server/work/dfo-lan/internal/toolcmd/<名称>` 查看。

## 保留清单：20个工具

这些工具也不是启动器依赖；保留依据是独立的开发/维护工作流，不以“注册表引用自身”或“有个测试文件”当必要性证明。

| 工具 | 保留用途/证据 |
|---|---|
| `pvfaudit`、`pvfinspect` | 统一源/历史对照及selection scope审计；任意PVF脚本、原始字节、类型化cells取证，避免再添领域专用导出器 |
| `charactercheck`、`storagecheck`、`dbq` | 临时schema整栈存档回归、PG迁移/连通检查、显式SQL诊断；不参与自动启动，不在本轮自动执行 |
| `initialrepair`、`questrepair` | 角色初始化/任务修复的既有预览、显式apply与目录检查工作流 |
| `framedump`、`loginchannel`、`protocolfixture` | 通用协议离线帧解密、登录频道类型实验、合成测试夹具生成；不跑客户端 |
| `audit36`、`questchain` | 任务支持范围/奖励执行缺口审计、指定角色条件下的任务前置图诊断，不等同于泛用原始脚本导出 |
| `npcpresenceaudit`、`dungeonscenesaudit`、`townprobe` | 任务NPC/剧情图存在性、终场场景跨脚本关联、城镇出生点/可行走区域诊断 |
| `equipfields`、`shieldaudit`、`skillaudit` | 模板ID到原生字段定位、骑士盾牌窗口与职业绑定、技能学习元数据诊断 |
| `equipmentfull`、`dungeonimport` | `scripts/local-fixes/apply_local_config.py:134,146` 仍给出这两个导出工具作为缺失历史输入的手动操作步骤，暂不删；该脚本没有自动调用它们。导出路径仍须显式指定，产物不成为运行内容回退 |

## 验证与边界

Go1.26.5全量 `go test -count=1 ./...`、`go vet ./...` 及CLI/网关构建通过；真实CLI的20个保留帮助命令、39个退休命令拒绝、3类参数错误出口和4组确定性协议夹具逐字节回归通过。帮助与退休命令拒绝在空目录执行，没有创建输出文件。

生产网关整理前后按相同toolchain和 `-trimpath -buildvcs=false` 构建（排除两个工作树dirty状态的VCS元数据差异），两个SHA256均为 `a949651ae252b687d4b3ca88492c646a642f102f072b0e064a267a45bec11ed3`。这证明本批入口删除未改变网关构建内容，不是新增实机验收。

本轮没有启动客户端、监听或玩家数据库，没有改PVF、数据库schema、玩法规则或存档身份；正式bin保持。confirmed baseline继续沿用既有实机范围。源码/接口验证不冒充游戏实机验收。主目录并行奖励/依赖改动不纳入本轮提交。
