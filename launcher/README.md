# 累计 R5 启动器与内嵌 GM 源码

冻结版本：GUI 2.0.5.0，发布标识 `2026.10.11-cumulative-r5`。

在 Windows 上安装 .NET Framework 4.x 编译器和 WPF 组件，先在 `server/work/dfo-lan/` 编译：

```powershell
go build -o bin/gmtool-local.exe ./cmd/gmtool
go build -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe
```

再从仓库根执行：

```powershell
pwsh -NoProfile -File launcher/Build.ps1 -Backend server/work/dfo-lan/bin/gmtool-local.exe -Server server/work/dfo-lan/bin/wireprobe-handoff-source.exe
& './launcher/dist/DFO-115US本地版登录器.exe' --self-test --evidence './launcher/self-test'
```

产物留在未跟踪的 `dist/`。源码检出不包含完整游戏运行包、资源、玩家数据库或默认服务端程序。正常启动需要与该冻结版本对应的运行包与 release manifest；不要把新编译程序直接覆盖到另一版本的运行包中。

此源码分支尚未与最新上游 main 合并；服务端验证和评审边界见 `../server/work/dfo-lan/docs/r5-gitgud-source-submission.md`。
