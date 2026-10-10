# 服务端命令入口

2026-10-06 新增 `go run ./cmd/dfo-tool botclient -h`：通过服主本机 Windows 身份，明确列出/选择活动源码候选连接、签发两分钟一次性凭据和读取同账号其它角色身份。默认不启用服务端通道，须在用户手动启动源码候选前设置 `DFO_BOT_CONTROL=1`；DLL 通信候选已完成离线互通，服务端邀请/未发布快照准备已接线，原生列表 hook 和实际 bot 执行尚未接入。操作和测试边界见 [独立通道说明](../docs/bot-client-channel.md)。新增实现仍在 `internal/toolcmd`，当前工具清单以 `dfo-tool -h` 为准；下方裁减数量描述当时的历史范围。

`cmd` 只保留四个可执行程序入口：

| 入口 | 用途 |
|---|---|
| `wireprobe` | 游戏网关；日常仍由 Python 启动器编排 |
| `admin` | GM 命令行管理，保持现有构建与调用方式 |
| `gmtool` | GM Web 服务，保持现有构建与调用方式 |
| `dfo-tool` | PVF 检查/导出、审计、存储维护和协议离线诊断 |

在 `server/work/dfo-lan` 下运行：

```powershell
$env:GOTOOLCHAIN = 'go1.26.5'
go run ./cmd/dfo-tool -h
go run ./cmd/dfo-tool pvfinspect -h
go run ./cmd/dfo-tool charactercheck -h
go build -trimpath -o bin/dfo-tool.exe ./cmd/dfo-tool
./bin/dfo-tool.exe pvfaudit -h
```

bot 基础攻击的只读取证复用 `skillaudit`，不新增命令入口：

```powershell
go run ./cmd/dfo-tool skillaudit -basic-attacks -source ../client-build/Script.inner.pvf -output .tmp/bot-v1/basic-attack-sources.json
```

该输出保留 `.chr` 攻击相关引用及其依赖 cells/哈希，不能作为运行配置或证明职业已可攻击。当前实现与未闭环项见 [bot 队友检查点](../docs/bot-team-v1.md)。

原 `go run ./cmd/<工具名> <参数>` 对仍保留的工具改为 `go run ./cmd/dfo-tool <工具名> <参数>`；原独立工具构建改为构建 `dfo-tool` 并在执行时传工具名。当前只保留20个工具，完整清单按 PVF、目录导出、审计、维护、协议分组显示在 `-h` 中。旧路径不保留空壳入口，已删除的39个工具名直接报未知命令，不再提供旧导出行为。删除与保留依据见 [工具裁减记录](../../../../docs/todo/server-tool-pruning-20261003.md)。

实现与原有测试位于 `internal/toolcmd/<工具名>`。新增工具应注册到统一入口，不能继续添加独立 `cmd` 目录。工具仅在选中后注册/解析自身参数；相对输入输出路径仍以工作目录为基准。

保留工具的执行行为保持：维护工具沿用原来的迁移、临时 schema 和 `-apply` 语义。帮助命令不执行工具、不读取存档或修改资源。固定输入诊断 `avatarrestorecheck`、`dump781x`、`odysseyaudit` 及资源写入工具 `pvfpatch` 已删除；运行PVF资源和既有历史证据保持。

参数式协议工具的调用位置保持：

```powershell
go run ./cmd/dfo-tool framedump <label> <key.bin> <stream.bin> <all|id|offset:size> [header]
go run ./cmd/dfo-tool loginchannel <input.bin> <output.bin> <channel-type>
go run ./cmd/dfo-tool protocolfixture <output.bin> [login|characters|name|characters-row]
```

`framedump` 的 `<label>` 为原入口保留的位置参数。存储检查仍需用户明确选择环境后执行，例如 `go run ./cmd/dfo-tool charactercheck`；本次整理的自动验证不运行玩家库检查或客户端。
