# 累计 R7 启动器与内嵌 GM 源码

冻结版本：GUI 2.0.7.0，发布标识 `2026.10.11-cumulative-r7`。

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

R7 会实际计算发行文件哈希，记录不一致并继续启动；必须文件、资源解析、存档身份、安装载荷和回退校验仍独立执行。源码已合并最新上游 main，包含 R6 的默示之真谛升级；验证和评审边界见 `../server/work/dfo-lan/docs/r7-gitgud-source-submission.md`。新上游组合仍按 Draft 评审，不以原累计包记录代替合并后实机验收。
