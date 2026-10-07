# AGENTS.md — client-patchs/

> 本目录是**客户端补丁 / DLL 的唯一家**。先读仓库根 [`../AGENTS.md`](../AGENTS.md)，
> 再读本文件；根 §0 全部硬约束对本目录完全适用，冲突以根文件为准。
>
> **默认不启用、不随包分发**（根 §0 第 1 条：服务端优先）。只有业主**明确要求**
> 当前任务使用客户端补丁/DLL 时，才在本目录新增或修改，并在交付说明里写清
> "为什么必须动客户端、服务端能不能替代"。

## 1. 硬规则

1. **不覆盖 `DFO.exe`**：本目录现有工具要么改**内存**（运行期钩子/虚表/导出表），
   要么整文件替换**资源**（字体、PVF、NPK）。真要下 EXE 字节补丁，必须
   (a) 单独说明理由与偏移，(b) 先备份原文件，(c) 代码里逐字节核对现场后再生效。
2. **不盲改内存**：任何 inline 跳转/虚表替换/EXE 补丁，**动手前逐字节核对期望字节**，
   地址不可读或字节不符就**跳过并记日志**（换客户端版本时宁可失效也不能崩）。
3. **日志只写 DLL 自己所在目录**（根 §0 第 9 条）：解析自身模块路径，
   不依赖进程当前目录，不往游戏根目录乱写。
4. **实机由业主操作**（根 §0 第 6 条）：补丁准备好后通知业主手动跑；
   不得无人值守启动客户端或代替玩家操作。
5. **交付走 mod 框架**：本目录可安装的东西打成 schema 2 mod 包（client 层 `file.add` /
   `file.replace` / `exe.patch`），必须能过 `modkit verify` / `plan` / `install` / `uninstall`；
   卸载要能逐字节还原。客户端只有**一个**能被自动加载的 DLL 槽位，
   多个客户端 DLL mod 一律走 [`client-host/HOST-README.md`](client-host/HOST-README.md) 的插件通道
   （`client.file.write` + `requires: ["qol.client-host"]`）。
   > **`exe.patch` 没有整文件哈希门**（2026-10-06 起移除）：清单里的 `sourceSHA256` 只是可选声明，
   > 现场 `DFO.exe` 与之不符**只警告、不阻断安装**；能不能打由**逐处 offset 的 `before` 字节比对**决定
   > （启动器仓 `internal/modkit/apply2.go:191-232`）。所以"版本对不对"不再由门挡住，
   > 作者必须自己确认偏移对应的客户端版本。
6. **插件要给机器留状态**：除人读的 `<客户端>\.115us-mods\<插件名>.log` 外，
   尽量再写一份 `<客户端>\.115us-mods\<插件名>.status.json`（`ready` / `enabled` / `rejectReason`
   / 命中 id 等），启动器与 GM 页读它做"生效自证"
   （范例：`client-patchs/difficulty` 的 `difficulty-rules.status.json`）。
   **mod 自带的范围规则放 `<客户端>\.115us-mods\rules.d\*.json`**：插件按**文件名升序**先读
   `rules.d\*.json`、最后读玩家自己的 `rules.json`，所以 mod 自带规则优先于玩家通用规则；
   `rules.d` 里单个文件解析失败**只跳过它自己**（日志一行 `[跳过]`），不影响其它文件。
7. **构建产物不入库**：每个子目录的 `dist/` 由 `.gitignore` 覆盖（`*.dll` / `*.zip` /
   `*.obj` / `*.exp` / `client-patchs/*/dist/`）。中间产物放 `dist/`，不要放在子目录根。
8. **提交前跑门禁**：`powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\check-commit-hygiene.ps1 -All`
   （本机 `pwsh` 不一定在 PATH）。命中任何条目按根 §0.3.1 停止提交并报告业主。

## 2. 目录约定

```
client-patchs/<补丁名>/
    README.md            ← 这个补丁改什么、怎么开/关、怎么回滚、日志在哪
    build-*.cmd          ← 构建（.cmd 保持纯 ASCII + CRLF；中文交给 .ps1/.py 打印）
    build-mod.py         ← 打包成 schema 2 mod（算 size/sha256 写 mod.json）
    src/                 ← 源码（C/CPP/Python）；.def 放这里
    dist/                ← 构建产物（不入库；含打包 staging 与 zip）

client-patchs/tests/     ← 共享的**机制自测**（inline 跳转、虚表替换、IME 门禁复刻、Themida 导入槽），只碰本进程
```

环境：本机 MSVC 在
`C:\Program Files\Microsoft Visual Studio\2022\Enterprise\VC\Auxiliary\Build\vcvars64.bat`
（`cl /LD /O2 /MT`）。DLL 一律 x64、静态 CRT（`/MT`），避免依赖 VC 运行库。

## 3. 现有内容

| 目录 | 是什么 | 状态 |
| --- | --- | --- |
| `fontsize/` | 字体/字号补丁的 Python 工具（改字体文件与 EXE 字节） | 历史补丁，2026-09 |
| `client-host/` | **客户端 mod 宿主**（占住唯一 DLL 槽位，加载 `.115us-mods\*.dll` 插件） | mod `qol.client-host` v1.0.0 |
| `chinese-input/` | 中文输入：诊断探针 + 门禁修复（HKL / IME 文件名） | mod `qol.chinese-input-probe` v1.5.0 |
| `auto-confirm/` | 删角色不用手打确认短语 | mod `qol.auto-confirm` v1.0.0 |

## 4. 已知边界（不要当成已解决）

- 客户端带 BlackCipher 反作弊组件；**新加载 DLL 是否被拦没有验证**。出问题先卸载补丁再复现，
  以区分"补丁问题"与"客户端问题"。
- 本机沙箱下，**放在工作区内的可执行文件写不了工作区外**；`modkit.exe` 要复制到
  `%TEMP%` 之类的工作区外路径再跑（详见 `../analysis/tasks/chinese-input-probe-20261006.md` §2.2）。
- modkit 的 `file.add` 不允许覆盖"已存在且内容不同"的文件，**含 `file.add` 的 mod 无法原地升级
  被改过的文件**；所以补丁的开关文件应由 DLL 首次运行时自己生成，包内只留 DLL。
