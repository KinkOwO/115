# 服务端命令入口

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

原 `go run ./cmd/<工具名> <参数>` 统一改为 `go run ./cmd/dfo-tool <工具名> <参数>`；原独立工具构建改为构建 `dfo-tool` 并在执行时传工具名。59 个工具名保留，完整清单按 PVF、目录导出、审计、维护、协议分组显示在 `-h` 中。旧路径不保留空壳入口。历史协议记录中的旧调用方式按此转换。

实现与原有测试位于 `internal/toolcmd/<工具名>`。新增工具应注册到统一入口，不能继续添加独立 `cmd` 目录。工具仅在选中后注册/解析自身参数；相对输入输出路径仍以工作目录为基准。

这是入口合并，不是工具行为重写：维护工具保留原来的迁移、临时 schema 和 `-apply` 语义，PVF patch 工具保留原来的资源写入能力。帮助命令不执行工具、不读取存档或修改资源。`avatarrestorecheck`、`dump781x`、`odysseyaudit` 仍为历史固定输入诊断，仅在对应环境中显式执行；不参与日常启动。

参数式协议工具的调用位置保持：

```powershell
go run ./cmd/dfo-tool dump342 <key.bin> <stream.bin>
go run ./cmd/dfo-tool framedump <label> <key.bin> <stream.bin> <all|id|offset:size> [header]
go run ./cmd/dfo-tool loginchannel <input.bin> <output.bin> <channel-type>
go run ./cmd/dfo-tool protocolfixture <output.bin> [login|characters|name|characters-row]
```

`framedump` 的 `<label>` 为原入口保留的位置参数。存储检查仍需用户明确选择环境后执行，例如 `go run ./cmd/dfo-tool charactercheck`；本次整理的自动验证不运行玩家库检查或客户端。
