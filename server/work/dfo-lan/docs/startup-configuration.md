# Wireprobe 启动配置

`cmd/wireprobe/config.go` 的 `Config` 是 Go 网关启动参数的唯一声明处。95 个既有参数的名称、类型、默认值、环境变量别名和帮助文本在同一结构体中声明；`main.go` 只读取类型化字段。

## 启动装配（2026-10-03 源码候选）

`main` 解析参数并调用 `runGateway`。`bootstrap.go` 的 `prepareRuntime(Config)` 负责既有目录准备、策略校验、数据库迁移及服务接线，返回类型化 `gatewayRuntime` 与清理函数；`runGateway` 再打开监听端口、记录文件并进入原有连接分发。`main.go` 从 5825 行降到 4398 行。

- 装配按原顺序执行，迁移与存档身份归一位置保持。自动补全的路径及 `DFO_SKILL_CATALOG` 覆盖保存在返回的 `runtime.config`，连接消费装配后的配置；调用者的 `Config` 值保持。
- `pvf-check-catalogs` 完成报告后返回空 runtime，提前退出，仍在运行规则安装、存储、监听之前。失败与检查模式自行清理已获取资源；正常模式由 `runGateway` 持有清理函数。
- 启动错误向上返回，`main` 在清理完成后记录错误并退出。目录资源、数据库池和管理锁按注册的逆序只关闭一次；`PrepareCatalogs` 返回部分结果及错误时也释放已打开目录。准备期的数据库超时 context 在准备函数退出时取消。
- 城镇场景白名单在打开监听端口前校验；world 与 quest 同时启用时不能传入 nil，合法空 map 继续允许。连接中原来的进程级 `log.Fatal` 已移除。
- 当前连接分发保留原局部变量和作用域，尤其每连接的角色 context 隔离；之后拆分连接流程时再收敛服务引用。依赖注入库暂未引入，目录读取、校验与迁移仍需要显式编排。

新增启动专项使用临时工作目录、临时文件和本机回环监听端口，覆盖配置/报文 fixture 保留、错误返回、端口释放、白名单缺失与合法空表，以及并发重复清理。按用户要求不运行全量测试，也未执行真实 PVF 完整检查或玩家数据库迁移。

```powershell
$env:GOTOOLCHAIN = 'go1.26.5'
go test ./cmd/wireprobe -run '^(TestPrepareRuntime|TestRunGateway|TestTownArrivalScenesValidated|TestRuntimeCleanup|TestWireprobeConfig|TestConnection(Session|Output)|TestInherit|TestAmplifyGrimoire)' -count=1
go test ./internal/archtest -count=1
go vet ./...
go build -trimpath -o ../../../.tmp/bootstrap/wireprobe-bootstrap.exe ./cmd/wireprobe
```

旧源码与新源码的连接分发在排除前移的白名单校验后 35,896 个 token 一致；启动装配在归一化错误返回、资源所有权和策略类型提取后 9,886 个 token 一致。12 组旧/新 CLI 子进程对照覆盖帮助、非法参数、检查模式参数校验、模板/fixture 缺失及监听后的错误，输出与退出码保持。候选保存在根目录 `.tmp/bootstrap/`，confirmed runtime baseline 及实机确认范围保持。

## 读取顺序

1. `Config` 的默认值。
2. 已登记的 54 个 `DFO_*` 环境变量。
3. 命令行中显式指定的参数。

Koanf v2.3.7 将三层已完成类型转换的值合并到 `Config`。只覆盖显式传入的命令行参数；空字符串和 `false` 也属于显式覆盖。每次解析使用独立的 `flag.FlagSet`，保留标准库的单/双横线、`-h`、重复参数和位置参数行为。

运行 profile 仍由现有 Python 启动器读取：`launch_local.py` / `repair_profile.py` 将 `configs/pvf-default.json` 等本地运行配置转换为环境变量，网关再消费这些值。本批没有新增配置文件入口或自动推导环境变量名称；未声明的环境变量不会覆盖启动参数。其它运行期诊断环境变量继续由原消费者读取。

## 兼容规则

- 普通布尔环境变量只有字符串 `1` 表示开启；`envmode:"not-zero"` 的选项只有字符串 `0` 表示关闭。字符串 `true` 不自动解释成 `1`。
- 空环境变量保留默认值。非法或溢出的整数环境变量回到默认值。
- `envmode:"byte"` 将环境变量值限制在 0..255；显式命令行整数继续沿用原 `flag.Int` 行为，不新增截断或范围门禁。
- 誓约保底默认值继续引用原 `oathDefaultProgressClears` 和 `oathDefaultProgressDungeons` 常量。
- 原 PVF 检查参数组合在目录准备前校验，错误语义保持。
- Koanf 只管理启动配置。游戏内容继续走既有内层 PVF 准备路径，存档身份、协议布局和事务边界保持。

## 维护与验证

新增或调整既有运维参数时修改 `Config` 对应字段及标签，业务编排直接使用字段，不再增加散落的 `flag.*` 指针或环境变量转换函数。当前解析器只支持 `string`、`bool`、`int`；新增类型需明确实现转换并测试。

`testdata/config_legacy.json` 是改造前解析器的独立输出对照，包含默认值、全部环境变量别名、全部显式参数覆盖、空/非法环境值、当时的默认 PVF profile 和位置参数七组向量。`config_help.json` 用 JSON 字符串保留原帮助文本，程序名称归一化为 wireprobe。修改这些对照必须说明行为变更，不能按新解析器的输出直接重生成。

本轮按用户要求不运行全量测试。Go 1.26.5 下的验证命令：

```powershell
$env:GOTOOLCHAIN = 'go1.26.5'
go test ./cmd/wireprobe -run '^(TestWireprobeConfig|TestConnection(Session|Output)|TestInherit|TestAmplifyGrimoire)' -count=1
go test ./internal/archtest -count=1
go vet ./...
go build -trimpath -o .tmp/koanf/wireprobe-config.exe ./cmd/wireprobe
```

独立候选位于 `.tmp/koanf/`，没有替换日常入口或已确认二进制。源码验证不扩大现有实机 confirmed baseline。
