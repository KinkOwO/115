# mods —— 服务端层 mod 源码（由 modkit 管理）

**这是服务端真正加载 mod 的目录。** 本目录下的每个子目录是一个**已安装的服务端 mod
的 Go 包**，由 `modkit install` 从 mod 包的 `server/` 层落位而来。

> 示例源码与开发文档**不在**这里 —— 它们在**仓库根的 `mods/`**
> （[`../../../mods/`](../../../mods/)）。分开的原因：本目录会被 `go build ./mods/...`
> 扫到，示例源码放这里会被误当成已装 mod 编译进去。

**不要手改这里的东西**：下一次安装/卸载会按注册表重写。每个 mod 目录里的
`.modkit-owner` 是来源标记，卸载靠它确认归属。

## 本目录里的文件

| 路径 | 作用 |
| --- | --- |
| `<mod-id>/` | 已装 mod 的 Go 源码（+ 落位时的 `mod.json` 副本 + `.modkit-owner`） |
| `zz_mods_gen.go` | **生成物**：import 所有已装 mod 并提供 `RegisterMods()`，由服务端主程序调用 |
| `enabled.json` | **禁用名单**：列在这里的 mod 这次启动不注册任何钩子（管理器读写） |
| `scripts/` | 落盘补充口：`*.lua` 会被并入奖励管线（不重编译就能加规则） |
| `zz_mods_test.go` | 带 `modtest` 标签的整链自证（`go test -tags modtest ./mods/`） |

文档与示例：[仓库根 `mods/README.md`](../../../mods/README.md) ·
[`MOD-DEVELOPMENT.md`](../../../mods/MOD-DEVELOPMENT.md)（写 mod）·
[`MOD-MANAGER-INTEGRATION.md`](../../../mods/MOD-MANAGER-INTEGRATION.md)（接管理器）

## 它怎么生效

服务端是就地 `go build` 的，所以服务端 mod 就是"参与编译的 Go 包"：

```
mods/<mod-id>/                         ← 本目录（mod 的 Go 源码）
mods/zz_mods_gen.go                    ← 生成的 import 清单，调用各 mod 的 Register()
internal/servermod/                    ← 服务端侧的钩子宿主（钩子点、宿主机操作、启用门禁）
```

## 每个 mod 必须提供

```go
package modpkg   // ← 固定包名，不能是 main（Go 不允许 import 程序）

func Register()  // 由 zz_mods_gen.go 调用；内部往 internal/servermod 注册钩子
```

`Register()` 开头必须判断启用状态，否则会无视管理器的勾选一直生效：

```go
func Register() {
    if !servermod.Enabled("my.mod") { return }
    ...
}
```

## 增删 mod 与开关

- 装：`modkit install --client <客户端> --mod <mod.zip> --root <启动器根>`；
- 卸：`modkit uninstall --client <客户端> --id <mod-id> --root <启动器根>`；
- **开关**（不用重编译）：`modkit mods list|enable|disable --id <mod-id> --root <启动器根>`；
- **不要直接删目录** —— 那样会留下过期的 import 清单，服务端编不过。

装/卸之后都要**重新编译服务端**（启动器的"编译服务端"，或 `server/Build-Server.ps1`），
新二进制才会生效。**改开关不需要重编译，但要重启服务端**。

## 接口清单

`modkit layers` 打印引擎认识的四层接口、服务端钩子点与宿主机操作（权威清单）。
设计与约定见启动器仓 `docs/modkit.md`。
