# mods —— 服务端层 mod 源码（由 modkit 管理）

本目录下的每个子目录是一个已安装的服务端 mod 的 Go 包，由
`modkit install` 从 mod 包的 `server/` 层落位而来，**不要手改**：
下一次安装/卸载会按注册表重写。

- 每个 mod 目录里的 `.modkit-owner` 是来源标记，卸载靠它确认归属。
- 每个 mod 目录里的 `mod.json` 是**原样搬运**的组织文件副本：
  管理器与 `modkit mods list` 从这里读 名称 / 版本 / **作者 / 说明** / 层 /
  权限 / 依赖 / 钩子。作者与说明写在那份清单里，不要改这个副本。
- import 清单**不在这里**：生成文件是 `cmd/wireprobe/zz_mods_gen.go`，
  它调用各 mod 的 `Register()`；这样即使本目录为空，服务端也编得过。
- 删除某个 mod：用 `modkit uninstall --id <mod-id>`，不要直接删目录。

服务端侧的钩子宿主在 `internal/servermod`（钩子点与宿主机操作的实现）。
