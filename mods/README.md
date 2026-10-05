# mods —— mod 示例与开发文档

本目录是**mod 作者的工作区**：示例源码与两份文档。它**不是**服务端加载 mod 的地方。

## 目录职责（两个 mods/ 别混）

| 路径 | 是什么 | 谁会写它 |
| --- | --- | --- |
| **`mods/`**（本目录，仓库根） | **示例源码 + 开发文档**（给人看的） | 人手写 |
| `server/work/dfo-lan/mods/` | **服务端真正加载的目录**：已装 mod 的 Go 源码、生成的 `zz_mods_gen.go`、`enabled.json` | 只有 `modkit install/uninstall` 与 mod 管理器 |

为什么分开：服务端 mod 要参与 Go 编译，必须待在服务端模块里（`server/work/dfo-lan/mods/`，
且其内容会被 `go build ./mods/...` 扫到）。示例源码若放在那里，就会被误当成已安装的 mod
编译进去 —— 所以示例与文档放在这里。

装 mod 用的是**打好的 zip 包**，不是把示例目录拷过去：

```powershell
$mk  = "<启动器仓>\bin\modkit.exe"
$mod = "<某个 mod 的 zip>"

& $mk verify  --mod $mod
& $mk plan    --client <客户端> --mod $mod --root <启动器根>
& $mk install --client <客户端> --mod $mod --root <启动器根>
# 开关（不用重编译，但要重启服务端）
& $mk mods list
& $mk mods enable  --id <mod-id> --by launcher-ui
& $mk mods disable --id <mod-id> --by launcher-ui
```

## 文档

| 文件 | 读者 | 内容 |
| --- | --- | --- |
| [`MOD-DEVELOPMENT.md`](MOD-DEVELOPMENT.md) | **写 mod 的人** | 四层架构、包结构、`mod.json` 模板、权限、钩子、NPK 格式、装/卸语义、游戏内验证清单、常见报错 |
| [`MOD-MANAGER-INTEGRATION.md`](MOD-MANAGER-INTEGRATION.md) | **把 mod 管理器接进启动器的人** | 管理器在启动链的位置、三份状态、`enabled.json` 格式、程序接口、显示字段、边界 |

## 示例

### `examples/hello-verify/` —— 四层结构的可验证样例

演示 server / pvf / client / resource 四层怎么组织，以及"装上去能证明它生效"的最小闭环：

```
server/mod.go          server 层：server.boot + console.command 钩子
pvf/check-pvf.ps1      pvf 层：只读校验 Script.pvf / sk.dat 成对（需 PowerShell 7）
client/…txt            client 层：一个无害的整文件标记（装/卸逐字节可还原）
build-mod.py           打包：算 size/sha256 → 生成 mod.json → 打包 → 自证 verify
build-mod.cmd          ASCII 启动器（自动找 Python）
```

### `examples/giveaway-random-equipment/` —— 事件奖励（"发东西"）

演示怎么往服务端的**事件奖励管线**里追加一条规则（新角色创建时触发），
而不是自己写一条发放路径：

```
server/mod.go                     Register()：登记规则脚本 + boot 自检 + 诊断命令
server/rules/giveaway.lua         规则：on("character_create", …) → grant_item / send_mail
build-mod.py / build-mod.cmd      打包
```

**注意它的发放选择**：示例刻意只发**金币**（`grant_item(0, N)`，角色金币堆）与
**不带附件的系统邮件** —— 这两者"必定成功、不依赖任何内容模板"，
所以能干净地回答"mod 到底注册生效了没有"。

想改成发装备，要注意奖励邮件的装备附件走的是**奖励目录**
（启动日志里的 `loaded equipment catalog: N rows`），
而不是 42 万条的完整穿戴目录；模板不在前者里就会报
`模板 <N> 取不到奖励耐久（equipment definition missing）`。
先确认模板在那份目录里，再放进池子。

## 打包

每个示例自带 `build-mod.py`（逻辑）+ `build-mod.cmd`（ASCII 启动器）：

```powershell
cd mods\examples\hello-verify
.\build-mod.cmd                      # 输出到 .\dist\
python build-mod.py --modkit <modkit.exe>   # 顺便自证 verify
```

`dist/`、`dist-nopvf/` 是**构建产物，不入库**。
